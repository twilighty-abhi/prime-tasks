// Vikunja is a to-do list application to facilitate your life.
// Copyright 2018-present Vikunja and contributors. All rights reserved.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package models

import (
	"testing"

	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/xorm"
)

// F9 (prime-tasks): permission matrix for project access. It pins today's
// behaviour so later sharing fixes change rows here instead of silently
// widening or narrowing access.

type matrixGrantSource string

const (
	matrixGrantNone       matrixGrantSource = "no grant"
	matrixGrantUser       matrixGrantSource = "direct user share"
	matrixGrantTeam       matrixGrantSource = "team share"
	matrixGrantParentUser matrixGrantSource = "parent project user share"
	matrixGrantParentTeam matrixGrantSource = "parent project team share"
)

type matrixAction string

const (
	matrixReadProject      matrixAction = "read project"
	matrixCreateTask       matrixAction = "create task"
	matrixDeleteTask       matrixAction = "delete task"
	matrixCreateView       matrixAction = "create view"
	matrixManageLinkShares matrixAction = "manage link shares"
	matrixRemoveShare      matrixAction = "remove a share"
)

var matrixPermissionNames = map[Permission]string{
	PermissionRead:  "read",
	PermissionWrite: "read & write",
	PermissionAdmin: "admin",
}

var matrixActions = []matrixAction{
	matrixReadProject,
	matrixCreateTask,
	matrixDeleteTask,
	matrixCreateView,
	matrixManageLinkShares,
	matrixRemoveShare,
}

type matrixFixture struct {
	// project the actions run against: the shared project, or its child for parent grants
	projectID int64
	taskID    int64
}

// setupPermissionMatrix creates a fresh parent/child project pair owned by user1,
// a task in each, a user share for user3 on both (the share "remove a share" targets),
// and grants user2 access according to source and permission.
func setupPermissionMatrix(t *testing.T, source matrixGrantSource, permission Permission) matrixFixture {
	owner := &user.User{ID: 1, Username: "user1"}

	db.LoadAndAssertFixtures(t)
	s := db.NewSession()
	defer s.Close()

	parent := &Project{Title: "matrix parent"}
	require.NoError(t, parent.Create(s, owner))
	child := &Project{
		Title:           "matrix child",
		ParentProjectID: &parent.ID,
	}
	require.NoError(t, child.Create(s, owner))

	sharedProjectID := parent.ID
	target := parent
	if source == matrixGrantParentUser || source == matrixGrantParentTeam {
		target = child
	}

	task := &Task{
		Title:     "matrix task",
		ProjectID: target.ID,
	}
	require.NoError(t, task.Create(s, owner))

	for _, p := range []*Project{parent, child} {
		other := &ProjectUser{
			Username:   "user3",
			ProjectID:  p.ID,
			Permission: PermissionRead,
		}
		require.NoError(t, other.Create(s, owner))
	}

	switch source {
	case matrixGrantNone:
	case matrixGrantUser, matrixGrantParentUser:
		share := &ProjectUser{
			Username:   "user2",
			ProjectID:  sharedProjectID,
			Permission: permission,
		}
		require.NoError(t, share.Create(s, owner))
	case matrixGrantTeam, matrixGrantParentTeam:
		team := &Team{Name: "matrix team"}
		require.NoError(t, team.CreateNewTeam(s, owner, true))
		member := &TeamMember{
			TeamID:   team.ID,
			Username: "user2",
		}
		require.NoError(t, member.Create(s, owner))
		share := &TeamProject{
			TeamID:     team.ID,
			ProjectID:  sharedProjectID,
			Permission: permission,
		}
		require.NoError(t, share.Create(s, owner))
	default:
		t.Fatalf("unknown grant source %q", source)
	}

	require.NoError(t, s.Commit())

	return matrixFixture{
		projectID: target.ID,
		taskID:    task.ID,
	}
}

func runMatrixAction(t *testing.T, s *xorm.Session, action matrixAction, f matrixFixture) bool {
	doer := &user.User{ID: 2, Username: "user2"}

	var (
		can bool
		err error
	)
	switch action {
	case matrixReadProject:
		can, _, err = (&Project{ID: f.projectID}).CanRead(s, doer)
	case matrixCreateTask:
		can, err = (&Task{ProjectID: f.projectID}).CanCreate(s, doer)
	case matrixDeleteTask:
		can, err = (&Task{ID: f.taskID}).CanDelete(s, doer)
	case matrixCreateView:
		can, err = (&ProjectView{ProjectID: f.projectID}).CanCreate(s, doer)
	case matrixManageLinkShares:
		can, err = (&LinkSharing{ProjectID: f.projectID}).CanCreate(s, doer)
	case matrixRemoveShare:
		can, err = (&ProjectUser{
			ProjectID: f.projectID,
			Username:  "user3",
		}).CanDelete(s, doer)
	default:
		t.Fatalf("unknown action %q", action)
	}
	require.NoError(t, err)
	return can
}

func TestProjectPermissionMatrix(t *testing.T) {
	readOnly := map[matrixAction]bool{
		matrixReadProject:      true,
		matrixCreateTask:       false,
		matrixDeleteTask:       false,
		matrixCreateView:       false,
		matrixManageLinkShares: false,
		matrixRemoveShare:      false,
	}
	readWrite := map[matrixAction]bool{
		matrixReadProject:      true,
		matrixCreateTask:       true,
		matrixDeleteTask:       true,
		matrixCreateView:       false,
		matrixManageLinkShares: false,
		matrixRemoveShare:      false,
	}
	admin := map[matrixAction]bool{
		matrixReadProject:      true,
		matrixCreateTask:       true,
		matrixDeleteTask:       true,
		matrixCreateView:       true,
		matrixManageLinkShares: true,
		matrixRemoveShare:      true,
	}
	nothing := map[matrixAction]bool{
		matrixReadProject:      false,
		matrixCreateTask:       false,
		matrixDeleteTask:       false,
		matrixCreateView:       false,
		matrixManageLinkShares: false,
		matrixRemoveShare:      false,
	}

	// Rows are upstream behaviour. A prime-tasks fix that changes access edits or adds rows here.
	rows := []struct {
		source     matrixGrantSource
		permission Permission
		want       map[matrixAction]bool
	}{
		{
			source: matrixGrantNone,
			want:   nothing,
		},
		{
			source:     matrixGrantUser,
			permission: PermissionRead,
			want:       readOnly,
		},
		{
			source:     matrixGrantUser,
			permission: PermissionWrite,
			want:       readWrite,
		},
		{
			source:     matrixGrantUser,
			permission: PermissionAdmin,
			want:       admin,
		},
		{
			source:     matrixGrantTeam,
			permission: PermissionRead,
			want:       readOnly,
		},
		{
			source:     matrixGrantTeam,
			permission: PermissionWrite,
			want:       readWrite,
		},
		{
			source:     matrixGrantTeam,
			permission: PermissionAdmin,
			want:       admin,
		},
		{
			source:     matrixGrantParentUser,
			permission: PermissionRead,
			want:       readOnly,
		},
		{
			source:     matrixGrantParentUser,
			permission: PermissionWrite,
			want:       readWrite,
		},
		{
			source:     matrixGrantParentUser,
			permission: PermissionAdmin,
			want:       admin,
		},
		{
			source:     matrixGrantParentTeam,
			permission: PermissionRead,
			want:       readOnly,
		},
		{
			source:     matrixGrantParentTeam,
			permission: PermissionWrite,
			want:       readWrite,
		},
		{
			source:     matrixGrantParentTeam,
			permission: PermissionAdmin,
			want:       admin,
		},
	}

	for _, row := range rows {
		name := string(row.source)
		if row.source != matrixGrantNone {
			name += "/" + matrixPermissionNames[row.permission]
		}
		t.Run(name, func(t *testing.T) {
			f := setupPermissionMatrix(t, row.source, row.permission)
			for _, action := range matrixActions {
				want, has := row.want[action]
				require.True(t, has, "row has no expectation for %q", action)
				t.Run(string(action), func(t *testing.T) {
					s := db.NewSession()
					defer s.Close()
					assert.Equal(t, want, runMatrixAction(t, s, action, f))
				})
			}
		})
	}
}
