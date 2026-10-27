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

	"code.vikunja.io/api/pkg/config"
	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// F1 (prime-tasks): service.orgdirectory lets real users find every other
// non-bot user by partial name, username or email.
func TestListUsersOrgDirectory(t *testing.T) {
	user1 := &user.User{ID: 1}

	search := func(t *testing.T, query string, currentUser *user.User) []string {
		t.Helper()
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		found, err := user.ListUsers(s, query, currentUser, nil)
		require.NoError(t, err)
		usernames := make([]string, 0, len(found))
		for _, u := range found {
			usernames = append(usernames, u.Username)
		}
		return usernames
	}
	setOrgDirectory := func(t *testing.T, enabled bool) {
		config.ServiceOrgDirectory.Set(enabled)
		t.Cleanup(func() { config.ServiceOrgDirectory.Set(false) })
	}

	t.Run("flag off", func(t *testing.T) {
		t.Run("partial username of a non-discoverable user finds nobody", func(t *testing.T) {
			setOrgDirectory(t, false)
			assert.Empty(t, search(t, "ser6", user1))
		})
		t.Run("partial name of a non-discoverable user finds nobody", func(t *testing.T) {
			setOrgDirectory(t, false)
			assert.Empty(t, search(t, "one els", user1))
		})
		t.Run("partial email finds nobody", func(t *testing.T) {
			setOrgDirectory(t, false)
			assert.Empty(t, search(t, "user7@exa", user1))
		})
	})

	t.Run("flag on", func(t *testing.T) {
		t.Run("partial username ignores discoverability", func(t *testing.T) {
			setOrgDirectory(t, true)
			assert.Equal(t, []string{"user6"}, search(t, "ser6", user1))
		})
		t.Run("partial name ignores discoverable_by_name", func(t *testing.T) {
			setOrgDirectory(t, true)
			assert.Equal(t, []string{"user11"}, search(t, "one els", user1))
		})
		t.Run("partial email ignores discoverable_by_email", func(t *testing.T) {
			setOrgDirectory(t, true)
			assert.Equal(t, []string{"user14"}, search(t, "@some.serv", user1))
		})
		t.Run("case insensitive", func(t *testing.T) {
			setOrgDirectory(t, true)
			assert.Equal(t, []string{"user6"}, search(t, "SER6", user1))
		})
		t.Run("partial email match does not reveal the email", func(t *testing.T) {
			setOrgDirectory(t, true)
			db.LoadAndAssertFixtures(t)
			s := db.NewSession()
			defer s.Close()

			found, err := user.ListUsers(s, "user7@exa", user1, nil)
			require.NoError(t, err)
			require.Len(t, found, 1)
			assert.Equal(t, "user7", found[0].Username)
			assert.Empty(t, found[0].Email)
		})
		t.Run("other users' bots stay hidden", func(t *testing.T) {
			setOrgDirectory(t, true)
			assert.Empty(t, search(t, "Owner B", user1))
		})
		t.Run("own bots keep their current rules", func(t *testing.T) {
			setOrgDirectory(t, true)
			assert.ElementsMatch(t,
				[]string{
					"bot-owner-a-assistant",
					"bot-owner-a-scheduler",
				},
				search(t, "Owner A", &user.User{ID: 21}),
			)
		})
		t.Run("a bot searching gets no directory access", func(t *testing.T) {
			setOrgDirectory(t, true)
			// The auth user may not carry bot_owner_id, so this must come from the database.
			assert.Empty(t, search(t, "ser6", &user.User{ID: 23}))
		})
		t.Run("one-character query does not match partially", func(t *testing.T) {
			setOrgDirectory(t, true)
			assert.Empty(t, search(t, "u", user1))
		})
		t.Run("exact matches rank before partial ones, so the cap never hides them", func(t *testing.T) {
			setOrgDirectory(t, true)
			// user14's email is user15@some.service.com: a partial match with a lower id
			assert.Equal(t,
				[]string{
					"user15",
					"user14",
				},
				search(t, "user15", user1),
			)
		})
		t.Run("results are capped", func(t *testing.T) {
			setOrgDirectory(t, true)
			// 21 human fixture users contain "user"
			assert.Len(t, search(t, "user", user1), user.OrgDirectorySearchLimit)
		})
		t.Run("openid team-only search does not restrict the directory", func(t *testing.T) {
			setOrgDirectory(t, true)
			config.ServiceEnableOpenIDTeamUserOnlySearch.Set(true)
			t.Cleanup(func() { config.ServiceEnableOpenIDTeamUserOnlySearch.Set(false) })
			assert.Equal(t, []string{"user6"}, search(t, "ser6", user1))
		})
	})
}
