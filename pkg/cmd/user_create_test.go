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

package cmd

import (
	"context"
	"testing"

	"code.vikunja.io/api/pkg/config"
	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/user"

	"github.com/stretchr/testify/require"
	"xorm.io/builder"
)

// prime-tasks F6: `vikunja user create` exits before the async event router gets to the
// auto-join listener, so the CLI has to apply the auto-join rules itself.
func TestCreateUserFromCLIAutoJoinsTeams(t *testing.T) {
	db.LoadAndAssertFixtures(t)
	config.ServiceAutoJoinTeams.Set([]interface{}{
		map[string]interface{}{"domain": "example.org", "team_id": 1},
		map[string]interface{}{"domain": "example.org", "team_id": 2},
		map[string]interface{}{"domain": "elsewhere.org", "team_id": 3},
	})
	t.Cleanup(func() { config.ServiceAutoJoinTeams.Set(nil) })

	s := db.NewSession()
	defer s.Close()
	u, err := createUserFromCLI(context.Background(), s, &user.User{
		Username: "cli-newbie",
		Email:    "cli-newbie@Example.org",
		Password: "a-long-password",
	})
	require.NoError(t, err)

	db.AssertCount(t, "team_members", builder.Eq{"team_id": 1, "user_id": u.ID}, 1)
	db.AssertCount(t, "team_members", builder.Eq{"team_id": 2, "user_id": u.ID}, 1)
	db.AssertCount(t, "team_members", builder.Eq{"team_id": 3, "user_id": u.ID}, 0)
}

func TestCreateUserFromCLIWithoutRulesJoinsNothing(t *testing.T) {
	db.LoadAndAssertFixtures(t)
	config.ServiceAutoJoinTeams.Set(nil)

	s := db.NewSession()
	defer s.Close()
	u, err := createUserFromCLI(context.Background(), s, &user.User{
		Username: "cli-plain",
		Email:    "cli-plain@example.org",
		Password: "a-long-password",
	})
	require.NoError(t, err)

	db.AssertCount(t, "team_members", builder.Eq{"user_id": u.ID}, 0)
}
