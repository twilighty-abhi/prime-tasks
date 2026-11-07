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
	"net/http"
	"slices"
)

// agentSafeCandidates is the prime-tasks A2 "Agent (safe)" preset: what an agent needs to find, claim, work on and
// release tasks. Deliberately absent: every delete, share and team management (including tasks.assignees_share),
// project create/update/duplicate, views and buckets writes, the bulk assignee and label replaces and the bulk task
// update (they can remove assignees or labels), webhooks, tokens, attachments, reactions and admin.
var agentSafeCandidates = APIPermissions{
	"mcp":                  {"access"},
	"other":                {"my_work", "users"},
	"projects":             {"access", "read_all", "read_one", "tasks_by_index", "users_search", "views_buckets", "views_buckets_tasks"},
	"projects_views":       {"read_all", "read_one"},
	"projects_views_tasks": {"read_all"},
	"tasks":                {"claim", "create", "create_bulk", "read_all", "read_one", "release", "update"},
	"tasks_comments":       {"create", "read_all", "read_one", "update"},
	"tasks_assignees":      {"create", "read_all"},
	"tasks_labels":         {"create", "read_all"},
	"labels":               {"create", "read_all", "read_one", "update"},
}

// AgentSafeAPIPermissions returns the "Agent (safe)" token preset (prime-tasks A2), resolved against the routes
// registered for API tokens: a permission without a route is left out, and so is any whose route is a DELETE.
// The MCP settings and `vikunja bot create` both issue exactly this set.
func AgentSafeAPIPermissions() APIPermissions {
	out := APIPermissions{}
	for group, permissions := range agentSafeCandidates {
		for _, permission := range permissions {
			if !agentSafeRouteExists(group, permission) {
				continue
			}
			out[group] = append(out[group], permission)
		}
	}
	for _, permissions := range out {
		slices.Sort(permissions)
	}
	return out
}

func agentSafeRouteExists(group, permission string) bool {
	found := false
	for _, table := range []map[string]APITokenRoute{apiTokenRoutes, apiTokenRoutesV2} {
		rd := table[group][permission]
		if rd == nil {
			continue
		}
		if rd.Method == http.MethodDelete {
			return false
		}
		found = true
	}
	return found
}
