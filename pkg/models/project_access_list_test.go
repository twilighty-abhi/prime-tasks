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
	"code.vikunja.io/api/pkg/web"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// F5 (prime-tasks): who has access to a project, and why.
func TestProjectAccessList(t *testing.T) {
	owner := &user.User{ID: 1, Username: "user1"}

	type row struct {
		Kind, Source, Name, Via string
		Permission              Permission
	}
	read := func(t *testing.T, a web.Auth, projectID int64) ([]row, error) {
		s := db.NewSession()
		defer s.Close()
		result, _, _, err := (&ProjectAccessList{ProjectID: projectID}).ReadAll(s, a, "", 1, 0)
		if err != nil {
			return nil, err
		}
		rows := []row{}
		for _, e := range result.([]*ProjectAccessEntry) {
			r := row{Kind: e.Kind, Source: e.Source, Via: e.ViaProjectTitle, Permission: e.Permission}
			if e.User != nil {
				r.Name = e.User.Username
				assert.Empty(t, e.User.Email, "access lists never carry email addresses")
			} else {
				r.Name = e.Team.Name
			}
			rows = append(rows, r)
		}
		return rows, nil
	}

	db.LoadAndAssertFixtures(t)
	s := db.NewSession()
	parent := &Project{Title: "access parent"}
	require.NoError(t, parent.Create(s, owner))
	child := &Project{Title: "access child", ParentProjectID: &parent.ID}
	require.NoError(t, child.Create(s, owner))
	require.NoError(t, (&ProjectUser{Username: "user2", ProjectID: child.ID, Permission: PermissionWrite}).Create(s, owner))
	require.NoError(t, (&ProjectUser{Username: "user3", ProjectID: parent.ID, Permission: PermissionRead}).Create(s, owner))
	team := &Team{Name: "access team"}
	require.NoError(t, team.CreateNewTeam(s, owner, true))
	require.NoError(t, (&TeamProject{TeamID: team.ID, ProjectID: child.ID, Permission: PermissionAdmin}).Create(s, owner))
	parentTeam := &Team{Name: "parent team"}
	require.NoError(t, parentTeam.CreateNewTeam(s, owner, true))
	require.NoError(t, (&TeamProject{TeamID: parentTeam.ID, ProjectID: parent.ID, Permission: PermissionRead}).Create(s, owner))
	require.NoError(t, s.Commit())
	s.Close()

	want := []row{
		{Kind: "user", Source: ProjectAccessSourceOwner, Name: "user1", Permission: PermissionAdmin},
		{Kind: "user", Source: ProjectAccessSourceDirect, Name: "user2", Permission: PermissionWrite},
		{Kind: "team", Source: ProjectAccessSourceDirect, Name: "access team", Permission: PermissionAdmin},
		{Kind: "user", Source: ProjectAccessSourceParent, Name: "user1", Via: "access parent", Permission: PermissionAdmin},
		{Kind: "user", Source: ProjectAccessSourceParent, Name: "user3", Via: "access parent", Permission: PermissionRead},
		{Kind: "team", Source: ProjectAccessSourceParent, Name: "parent team", Via: "access parent", Permission: PermissionRead},
	}

	t.Run("owner sees every source, nearest first", func(t *testing.T) {
		rows, err := read(t, owner, child.ID)
		require.NoError(t, err)
		assert.Equal(t, want, rows)
	})

	t.Run("anyone who can read the project sees the same list", func(t *testing.T) {
		rows, err := read(t, &user.User{ID: 3, Username: "user3"}, child.ID)
		require.NoError(t, err)
		assert.Equal(t, want, rows)
	})

	t.Run("no access", func(t *testing.T) {
		_, err := read(t, &user.User{ID: 4, Username: "user4"}, child.ID)
		assert.True(t, IsErrNeedToHaveProjectReadAccess(err), "got %v", err)
	})

	t.Run("link shares can't list people", func(t *testing.T) {
		_, err := read(t, &LinkSharing{ID: 1, ProjectID: child.ID, Permission: PermissionAdmin}, child.ID)
		assert.Error(t, err)
	})
}
