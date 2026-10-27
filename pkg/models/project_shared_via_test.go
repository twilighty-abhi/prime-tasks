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

// F3 (prime-tasks): shared_via for every grant source of the F9 matrix.
func TestProjectPermissionMatrixSharedVia(t *testing.T) {
	grantee := &user.User{ID: 2, Username: "user2"}
	owner := &user.User{ID: 1, Username: "user1"}

	sharedVia := func(t *testing.T, a *user.User, projectID int64) *ProjectSharedVia {
		s := db.NewSession()
		defer s.Close()
		p := &Project{ID: projectID}
		require.NoError(t, AddProjectSharedVia(s, a, []*Project{p}))
		return p.SharedVia
	}

	rows := []struct {
		source matrixGrantSource
		want   *ProjectSharedVia
	}{
		{source: matrixGrantNone, want: nil},
		{source: matrixGrantUser, want: &ProjectSharedVia{Type: ProjectSharedViaDirect, Permission: PermissionWrite}},
		{source: matrixGrantTeam, want: &ProjectSharedVia{Type: ProjectSharedViaTeam, Permission: PermissionWrite, TeamName: "matrix team"}},
		{source: matrixGrantParentUser, want: &ProjectSharedVia{Type: ProjectSharedViaParent, Permission: PermissionWrite, ParentProjectTitle: "matrix parent"}},
		{source: matrixGrantParentTeam, want: &ProjectSharedVia{Type: ProjectSharedViaParent, Permission: PermissionWrite, TeamName: "matrix team", ParentProjectTitle: "matrix parent"}},
	}
	for _, row := range rows {
		t.Run(string(row.source), func(t *testing.T) {
			f := setupPermissionMatrix(t, row.source, PermissionWrite)
			got := sharedVia(t, grantee, f.projectID)
			if row.want == nil {
				assert.Nil(t, got)
			} else {
				require.NotNil(t, got)
				// ids are created per run; compare them for presence, the rest by value
				if row.want.TeamName != "" {
					assert.NotZero(t, got.TeamID)
				}
				if row.want.Type == ProjectSharedViaParent {
					assert.NotZero(t, got.ParentProjectID)
				}
				got.TeamID, got.ParentProjectID = 0, 0
				assert.Equal(t, row.want, got)
			}
			assert.Nil(t, sharedVia(t, owner, f.projectID), "the owner's own project is not shared with them")
		})
	}
}

func TestAddProjectSharedVia(t *testing.T) {
	user1 := &user.User{ID: 1, Username: "user1"}

	t.Run("direct share beats a team share on the same project", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		// Fixture: project 3 is shared with user1 directly (read) and with testteam1 (read).
		s := db.NewSession()
		defer s.Close()
		p := &Project{ID: 3}
		require.NoError(t, AddProjectSharedVia(s, user1, []*Project{p}))
		require.NotNil(t, p.SharedVia)
		assert.Equal(t, ProjectSharedViaDirect, p.SharedVia.Type)
	})

	t.Run("the stronger share wins at the same level", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		// Fixture: project 8 reaches user1 only through testteam4 (admin).
		s := db.NewSession()
		defer s.Close()
		p := &Project{ID: 8}
		require.NoError(t, AddProjectSharedVia(s, user1, []*Project{p}))
		require.NotNil(t, p.SharedVia)
		assert.Equal(t, ProjectSharedVia{Type: ProjectSharedViaTeam, Permission: PermissionAdmin, TeamID: 4, TeamName: "testteam4_admin_on_project8"}, *p.SharedVia)
	})

	t.Run("owner display name, pseudo projects and link shares", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		own := &Project{ID: 1, Owner: &user.User{ID: 1, Username: "user1", Name: ""}}
		named := &Project{ID: 6, Owner: &user.User{ID: 6, Username: "user6", Name: "Six"}}
		favorites := &Project{ID: FavoritesPseudoProjectID}
		require.NoError(t, AddProjectSharedVia(s, user1, []*Project{own, named, favorites}))
		assert.Nil(t, own.SharedVia)
		assert.Equal(t, "user1", own.OwnerDisplayName)
		assert.Equal(t, "Six", named.OwnerDisplayName)
		assert.Nil(t, favorites.SharedVia)

		linked := &Project{ID: 1}
		require.NoError(t, AddProjectSharedVia(s, &LinkSharing{ID: 1, ProjectID: 1}, []*Project{linked}))
		assert.Nil(t, linked.SharedVia)
	})
}
