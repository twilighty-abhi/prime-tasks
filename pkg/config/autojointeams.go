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
	"fmt"
	"strconv"
	"strings"
)

// AutoJoinTeamRule adds every new user whose email address is on Domain to the team TeamID
// (prime-tasks F6, service.autojointeams).
type AutoJoinTeamRule struct {
	Domain string
	TeamID int64
}

// AutoJoinTeamRules parses service.autojointeams. An unset or empty option yields no rules.
func AutoJoinTeamRules() ([]AutoJoinTeamRule, error) {
	raw := ServiceAutoJoinTeams.Get()
	if raw == nil || raw == "" {
		return nil, nil
	}
	list, ok := raw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("service.autojointeams must be a list of {domain, team_id}, got %T", raw)
	}

	rules := make([]AutoJoinTeamRule, 0, len(list))
	for i, item := range list {
		entry, ok := stringKeyedMap(item)
		if !ok {
			return nil, fmt.Errorf("service.autojointeams[%d] must be a map with domain and team_id, got %T", i, item)
		}
		domain := NormalizeEmailDomain(fmt.Sprint(entry["domain"]))
		if domain == "" || entry["domain"] == nil {
			return nil, fmt.Errorf("service.autojointeams[%d] has no domain", i)
		}
		teamID, err := toInt64(entry["team_id"])
		if err != nil || teamID < 1 {
			return nil, fmt.Errorf("service.autojointeams[%d] has no valid team_id: %v", i, entry["team_id"])
		}
		rules = append(rules, AutoJoinTeamRule{Domain: domain, TeamID: teamID})
	}
	return rules, nil
}

// NormalizeEmailDomain lowercases a domain and drops surrounding spaces and a leading "@".
func NormalizeEmailDomain(domain string) string {
	return strings.ToLower(strings.TrimPrefix(strings.TrimSpace(domain), "@"))
}

func stringKeyedMap(item interface{}) (map[string]interface{}, bool) {
	switch m := item.(type) {
	case map[string]interface{}:
		return m, true
	case map[interface{}]interface{}:
		out := make(map[string]interface{}, len(m))
		for k, v := range m {
			key, ok := k.(string)
			if !ok {
				return nil, false
			}
			out[key] = v
		}
		return out, true
	default:
		return nil, false
	}
}

func toInt64(v interface{}) (int64, error) {
	switch n := v.(type) {
	case int:
		return int64(n), nil
	case int64:
		return n, nil
	case float64:
		if n != float64(int64(n)) {
			return 0, fmt.Errorf("not a whole number: %v", n)
		}
		return int64(n), nil
	case string:
		return strconv.ParseInt(strings.TrimSpace(n), 10, 64)
	default:
		return 0, fmt.Errorf("not a number: %T", v)
	}
}
