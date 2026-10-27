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
package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAutoJoinTeamRules(t *testing.T) {
	t.Cleanup(func() { ServiceAutoJoinTeams.Set(nil) })

	t.Run("unset means no rules", func(t *testing.T) {
		ServiceAutoJoinTeams.Set(nil)
		rules, err := AutoJoinTeamRules()
		require.NoError(t, err)
		assert.Empty(t, rules)
	})
	t.Run("parses yaml and json shaped lists", func(t *testing.T) {
		ServiceAutoJoinTeams.Set([]interface{}{
			map[interface{}]interface{}{"domain": " @Example.ORG ", "team_id": 3},
			map[string]interface{}{"domain": "other.org", "team_id": float64(4)},
			map[string]interface{}{"domain": "third.org", "team_id": "5"},
		})
		rules, err := AutoJoinTeamRules()
		require.NoError(t, err)
		assert.Equal(t, []AutoJoinTeamRule{
			{Domain: "example.org", TeamID: 3},
			{Domain: "other.org", TeamID: 4},
			{Domain: "third.org", TeamID: 5},
		}, rules)
	})
	t.Run("rejects entries without a domain or team", func(t *testing.T) {
		for _, entry := range []interface{}{
			map[string]interface{}{"team_id": 3},
			map[string]interface{}{"domain": "example.org"},
			map[string]interface{}{"domain": "example.org", "team_id": 0},
			"example.org",
		} {
			ServiceAutoJoinTeams.Set([]interface{}{entry})
			_, err := AutoJoinTeamRules()
			assert.Error(t, err, "entry %v", entry)
		}
	})
	t.Run("rejects a map instead of a list", func(t *testing.T) {
		ServiceAutoJoinTeams.Set(map[string]interface{}{"domain": "example.org", "team_id": 3})
		_, err := AutoJoinTeamRules()
		assert.Error(t, err)
	})
}
