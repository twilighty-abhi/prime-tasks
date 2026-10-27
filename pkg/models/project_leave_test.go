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
)

// F4 (prime-tasks): leaving a project.
func TestLeaveProject(t *testing.T) {
	owner := &user.User{ID: 1, Username: "user1"}
	leaver := &user.User{ID: 2, Username: "user2"}

	// setup creates parent "leave parent" and child "leave child", both owned by user1.
	setup := func(t *testing.T) (parent, child *Project) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		parent = &Project{Title: "leave parent"}
		require.NoError(t, parent.Create(s, owner))
		child = &Project{Title: "leave child", ParentProjectID: &parent.ID}
		require.NoError(t, child.Create(s, owner))
		require.NoError(t, s.Commit())
		return parent, child
	}
	shareWithUser := func(t *testing.T, projectID int64, permission Permission) {
		s := db.NewSession()
		defer s.Close()
		require.NoError(t, (&ProjectUser{Username: leaver.Username, ProjectID: projectID, Permission: permission}).Create(s, owner))
		require.NoError(t, s.Commit())
	}
	shareWithTeam := func(t *testing.T, name string, projectID int64) {
		s := db.NewSession()
		defer s.Close()
		team := &Team{Name: name}
		require.NoError(t, team.CreateNewTeam(s, owner, true))
		require.NoError(t, (&TeamMember{TeamID: team.ID, Username: leaver.Username}).Create(s, owner))
		require.NoError(t, (&TeamProject{TeamID: team.ID, ProjectID: projectID, Permission: PermissionRead}).Create(s, owner))
		require.NoError(t, s.Commit())
	}
	leave := func(t *testing.T, a *user.User, projectID int64) error {
		s := db.NewSession()
		defer s.Close()
		err := LeaveProject(s, a, projectID)
		if err != nil {
			_ = s.Rollback()
			return err
		}
		require.NoError(t, s.Commit())
		return nil
	}
	canRead := func(t *testing.T, projectID int64) bool {
		s := db.NewSession()
		defer s.Close()
		can, _, err := (&Project{ID: projectID}).CanRead(s, leaver)
		require.NoError(t, err)
		return can
	}

	for _, permission := range []Permission{PermissionRead, PermissionWrite, PermissionAdmin} {
		t.Run("direct share/"+matrixPermissionNames[permission], func(t *testing.T) {
			parent, _ := setup(t)
			shareWithUser(t, parent.ID, permission)
			require.NoError(t, leave(t, leaver, parent.ID))
			assert.False(t, canRead(t, parent.ID))
			db.AssertMissing(t, "users_projects", map[string]any{"user_id": leaver.ID, "project_id": parent.ID})
		})
	}

	t.Run("team share names the team", func(t *testing.T) {
		parent, _ := setup(t)
		shareWithTeam(t, "leave team", parent.ID)
		err := leave(t, leaver, parent.ID)
		require.Error(t, err)
		require.True(t, IsErrCannotLeaveProject(err), "got %v", err)
		assert.Equal(t, []string{"leave team"}, err.(*ErrCannotLeaveProject).Teams)
		assert.Contains(t, err.(*ErrCannotLeaveProject).HTTPError().Message, `the team "leave team"`)
		assert.True(t, canRead(t, parent.ID))
	})

	t.Run("parent user share names the parent project", func(t *testing.T) {
		parent, child := setup(t)
		shareWithUser(t, parent.ID, PermissionWrite)
		err := leave(t, leaver, child.ID)
		require.Error(t, err)
		require.True(t, IsErrCannotLeaveProject(err), "got %v", err)
		assert.Equal(t, []string{"leave parent"}, err.(*ErrCannotLeaveProject).ParentProjects)
		assert.Contains(t, err.(*ErrCannotLeaveProject).HTTPError().Message, `the parent project "leave parent"`)
		assert.True(t, canRead(t, child.ID))
	})

	t.Run("parent team share names the team", func(t *testing.T) {
		parent, child := setup(t)
		shareWithTeam(t, "parent team", parent.ID)
		err := leave(t, leaver, child.ID)
		require.True(t, IsErrCannotLeaveProject(err), "got %v", err)
		assert.Equal(t, []string{"parent team"}, err.(*ErrCannotLeaveProject).Teams)
	})

	t.Run("direct share plus team share removes only the direct share", func(t *testing.T) {
		parent, _ := setup(t)
		shareWithUser(t, parent.ID, PermissionWrite)
		shareWithTeam(t, "backup team", parent.ID)
		require.NoError(t, leave(t, leaver, parent.ID))
		db.AssertMissing(t, "users_projects", map[string]any{"user_id": leaver.ID, "project_id": parent.ID})
		assert.True(t, canRead(t, parent.ID), "team access stays")
	})

	t.Run("owner cannot leave", func(t *testing.T) {
		parent, child := setup(t)
		err := leave(t, owner, parent.ID)
		require.True(t, IsErrCannotLeaveProject(err), "got %v", err)
		assert.True(t, err.(*ErrCannotLeaveProject).Owner)
		// The owner of the parent reaches the child through the parent.
		err = leave(t, owner, child.ID)
		require.True(t, IsErrCannotLeaveProject(err), "got %v", err)
		assert.Contains(t, err.(*ErrCannotLeaveProject).ParentProjects, "leave parent")
	})

	t.Run("no access", func(t *testing.T) {
		parent, _ := setup(t)
		err := leave(t, leaver, parent.ID)
		assert.True(t, IsErrGenericForbidden(err), "got %v", err)
	})

	t.Run("nonexistent project", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		err := leave(t, leaver, 9999999)
		assert.True(t, IsErrProjectDoesNotExist(err), "got %v", err)
	})

	t.Run("link share cannot leave", func(t *testing.T) {
		parent, _ := setup(t)
		s := db.NewSession()
		defer s.Close()
		err := LeaveProject(s, &LinkSharing{ID: 1, ProjectID: parent.ID}, parent.ID)
		assert.Error(t, err)
	})
}

// F4 (prime-tasks): self-removal must not open removing anyone else.
func TestProjectUser_CanDelete_Self(t *testing.T) {
	db.LoadAndAssertFixtures(t)
	s := db.NewSession()
	defer s.Close()

	// Fixture: user1 has a read-only direct share on project 9 (owned by user6), so does nobody else.
	self := &ProjectUser{ProjectID: 9, Username: "user1"}
	can, err := self.CanDelete(s, &user.User{ID: 1, Username: "user1"})
	require.NoError(t, err)
	assert.True(t, can, "a user may remove their own direct share")

	// user1 has a read share on project 3 and user2 too; user1 may not remove user2.
	other := &ProjectUser{ProjectID: 3, Username: "user2"}
	can, err = other.CanDelete(s, &user.User{ID: 1, Username: "user1"})
	require.NoError(t, err)
	assert.False(t, can)

	// Self-removal is about deleting, not about changing your own permission.
	can, err = (&ProjectUser{ProjectID: 9, Username: "user1", Permission: PermissionAdmin}).CanUpdate(s, &user.User{ID: 1, Username: "user1"})
	require.NoError(t, err)
	assert.False(t, can)

	// No row for this user on that project: nothing to remove, no admin rights either.
	can, err = (&ProjectUser{ProjectID: 9, Username: "user2"}).CanDelete(s, &user.User{ID: 2, Username: "user2"})
	require.NoError(t, err)
	assert.False(t, can)
}
