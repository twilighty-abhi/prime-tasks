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
	"code.vikunja.io/api/pkg/events"
	"code.vikunja.io/api/pkg/user"

	"github.com/stretchr/testify/require"
)

// F6 (prime-tasks): auto-join teams by email domain.
func TestAutoJoinTeamsByEmailDomain(t *testing.T) {
	t.Cleanup(func() { config.ServiceAutoJoinTeams.Set(nil) })
	rules := func(entries ...map[string]interface{}) {
		list := make([]interface{}, len(entries))
		for i, e := range entries {
			list[i] = e
		}
		config.ServiceAutoJoinTeams.Set(list)
	}
	newUser := func(t *testing.T, username, email string, botOwner int64) *user.User {
		s := db.NewSession()
		defer s.Close()
		u := &user.User{Username: username, Email: email, Password: "a-long-password", BotOwnerID: botOwner}
		created, err := user.CreateUser(s, u)
		require.NoError(t, err)
		if botOwner != 0 {
			_, err = s.ID(created.ID).Cols("bot_owner_id").Update(&user.User{BotOwnerID: botOwner})
			require.NoError(t, err)
		}
		require.NoError(t, s.Commit())
		return created
	}
	fire := func(t *testing.T, u *user.User) {
		events.TestListener(t, &user.CreatedEvent{User: u}, &AutoJoinTeamsByEmailDomain{})
	}

	t.Run("joins every team for the domain, case-insensitively", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		rules(
			map[string]interface{}{"domain": "Example.org", "team_id": 1},
			map[string]interface{}{"domain": "example.org", "team_id": 2},
			map[string]interface{}{"domain": "elsewhere.org", "team_id": 3},
		)
		u := newUser(t, "newbie", "Newbie@EXAMPLE.org", 0)
		fire(t, u)
		db.AssertExists(t, "team_members", map[string]interface{}{"team_id": 1, "user_id": u.ID}, false)
		db.AssertExists(t, "team_members", map[string]interface{}{"team_id": 2, "user_id": u.ID}, false)
		db.AssertMissing(t, "team_members", map[string]interface{}{"team_id": 3, "user_id": u.ID})
	})

	t.Run("an auto-joined user gets the team's project access", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		// Fixture: team 1 has read access on project 3.
		rules(map[string]interface{}{"domain": "example.org", "team_id": 1})
		u := newUser(t, "joiner", "joiner@example.org", 0)

		s := db.NewSession()
		can, _, err := (&Project{ID: 3}).CanRead(s, u)
		s.Close()
		require.NoError(t, err)
		require.False(t, can, "no access before joining")

		fire(t, u)

		s2 := db.NewSession()
		defer s2.Close()
		can, _, err = (&Project{ID: 3}).CanRead(s2, u)
		require.NoError(t, err)
		require.True(t, can)
		can, err = (&Task{ProjectID: 3}).CanCreate(s2, u)
		require.NoError(t, err)
		require.False(t, can, "team 1 only has read access")
	})

	t.Run("a subdomain is not the domain", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		rules(map[string]interface{}{"domain": "example.org", "team_id": 1})
		u := newUser(t, "subby", "subby@mail.example.org", 0)
		fire(t, u)
		db.AssertMissing(t, "team_members", map[string]interface{}{"team_id": 1, "user_id": u.ID})
	})

	t.Run("bots are skipped", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		rules(map[string]interface{}{"domain": "example.org", "team_id": 1})
		u := newUser(t, "robot", "robot@example.org", 1)
		fire(t, u)
		db.AssertMissing(t, "team_members", map[string]interface{}{"team_id": 1, "user_id": u.ID})
	})

	t.Run("a missing team is skipped, the others still apply", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		rules(
			map[string]interface{}{"domain": "example.org", "team_id": 999999},
			map[string]interface{}{"domain": "example.org", "team_id": 2},
		)
		u := newUser(t, "survivor", "survivor@example.org", 0)
		fire(t, u)
		db.AssertExists(t, "team_members", map[string]interface{}{"team_id": 2, "user_id": u.ID}, false)
	})

	t.Run("already a member is fine", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		rules(map[string]interface{}{"domain": "example.org", "team_id": 1})
		u := newUser(t, "twice", "twice@example.org", 0)
		fire(t, u)
		fire(t, u)
		db.AssertExists(t, "team_members", map[string]interface{}{"team_id": 1, "user_id": u.ID}, false)
	})

	t.Run("empty config changes nothing", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		config.ServiceAutoJoinTeams.Set(nil)
		u := newUser(t, "plain", "plain@example.org", 0)
		fire(t, u)
		db.AssertMissing(t, "team_members", map[string]interface{}{"user_id": u.ID})
	})
}
