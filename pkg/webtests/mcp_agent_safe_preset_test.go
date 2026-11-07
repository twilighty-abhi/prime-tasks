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

package webtests

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"testing"

	"code.vikunja.io/api/pkg/models"
	"code.vikunja.io/api/pkg/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// prime-tasks A2: the "Agent (safe)" MCP token preset. Read, create and update on the task workflow, claim and
// release; no delete of any kind, no share management, no project create/update.
var agentSafeExpected = models.APIPermissions{
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

// Typed tools, then the catalog and docs tools a token with any catalog action or filter-taking tool gets.
var agentSafeTools = []string{
	"claim_task",
	"do_action",
	"filter_syntax",
	"find_action",
	"labels_create",
	"labels_list",
	"labels_read",
	"labels_update",
	"my_work",
	"projects_list",
	"projects_read",
	"release_task",
	"task_assignees_create",
	"task_assignees_list",
	"task_comments_create",
	"task_comments_list",
	"task_comments_read",
	"task_comments_update",
	"tasks_create",
	"tasks_list",
	"tasks_read",
	"tasks_update",
	"users_search",
}

var agentSafeActions = []string{
	"buckets_list",
	"project_access_list",
	"project_tasks_list",
	"project_view_buckets_tasks_list",
	"project_view_tasks_list",
	"project_views_list",
	"project_views_read",
	"projects_users_search",
	"task_labels_create",
	"task_labels_list",
	"tasks_bulk_create",
	"tasks_read_by_index",
}

func TestAgentSafePreset_ExactPermissions(t *testing.T) {
	_, err := setupTestEnv()
	require.NoError(t, err)
	preset := models.AgentSafeAPIPermissions()
	require.Len(t, preset, len(agentSafeExpected), "groups: %v", preset)
	for group, permissions := range agentSafeExpected {
		assert.Equal(t, permissions, preset[group], group)
	}
	require.NoError(t, models.PermissionsAreValid(preset))
}

func TestAgentSafePreset_NoDeleteOrShareManagement(t *testing.T) {
	e, err := setupTestEnv()
	require.NoError(t, err)
	preset := models.AgentSafeAPIPermissions()
	require.NotEmpty(t, preset)

	for group, permissions := range preset {
		for _, permission := range permissions {
			name := group + "." + permission
			for _, bad := range []string{"delete", "share", "permanent", "restore", "admin", "webhook", "team", "token", "duplicate", "background", "migration"} {
				assert.NotContains(t, name, bad)
			}
		}
	}
	for _, group := range []string{"projects_users", "projects_teams", "projects_shares", "projects_webhooks", "teams", "teams_members", "tokens", "webhooks", "filters", "notifications", "reactions", "tasks_relations", "tasks_attachments", "migration"} {
		assert.NotContains(t, preset, group)
	}
	assert.NotContains(t, preset["projects"], "create")
	assert.NotContains(t, preset["projects"], "update")
	assert.NotContains(t, preset["tasks"], "assignees_share")

	// Every route the preset unlocks, on v1 and v2.
	token := &models.APIToken{APIPermissions: preset}
	var unlocked []string
	for _, route := range e.Router().Routes() {
		if !strings.HasPrefix(route.Path, "/api/") || !token.CanUseRoute(route.Path, route.Method) {
			continue
		}
		unlocked = append(unlocked, route.Method+" "+route.Path)
		assert.NotEqual(t, http.MethodDelete, route.Method, route.Path)
		for _, bad := range []string{"/shares", "/users/:user", "/teams", "/webhooks", "/tokens", "/admin", "/assignees/share", "/assignees/bulk", "/labels/bulk", "/duplicate", "/permanent", "/restore", "/reactions/delete", "/background"} {
			assert.NotContains(t, route.Path, bad, route.Method)
		}
		if route.Path == "/api/v1/projects" || route.Path == "/api/v2/projects" ||
			strings.HasPrefix(route.Path, "/api/v1/projects/:project") && !strings.Contains(route.Path, "/tasks") && !strings.Contains(route.Path, "/views") ||
			strings.HasPrefix(route.Path, "/api/v2/projects/:id") {
			assert.Equal(t, http.MethodGet, route.Method, "projects can only be read: %s", route.Path)
		}
	}
	sort.Strings(unlocked)
	assert.NotEmpty(t, unlocked)
}

// /mcp/info serves the preset, and only names permissions the MCP tools use, so the settings form shows them all.
func TestAgentSafePreset_InMCPInfo(t *testing.T) {
	e, err := setupTestEnv()
	require.NoError(t, err)
	rec := humaRequest(t, e, http.MethodGet, "/api/v2/mcp/info", "", humaTokenFor(t, &user.User{ID: 1}), "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var info struct {
		Routes  map[string]map[string]any `json:"routes"`
		Presets struct {
			AgentSafe models.APIPermissions `json:"agent_safe"`
		} `json:"presets"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &info))
	assert.Equal(t, agentSafeExpected, info.Presets.AgentSafe)
	for group, permissions := range info.Presets.AgentSafe {
		for _, permission := range permissions {
			assert.Contains(t, info.Routes[group], permission, group)
		}
	}
}

func TestAgentSafePreset_MCPTools(t *testing.T) {
	e, err := setupTestEnv()
	require.NoError(t, err)
	shareProjectWithClaimBots(t, 1, 23)
	token := botAPIToken(t, 23, models.AgentSafeAPIPermissions())
	c := newMCPClientOn(t, e, token)

	var names []string
	for name := range c.toolNames() {
		names = append(names, name)
	}
	sort.Strings(names)
	assert.Equal(t, agentSafeTools, names)
	assert.Len(t, names, 23)

	var actions []string
	for _, a := range findActions(t, c, map[string]any{}) {
		actions = append(actions, a["name"].(string))
	}
	sort.Strings(actions)
	assert.Equal(t, agentSafeActions, actions)

	// The safe workflow works: claim, comment, release.
	claimed := c.callTool("claim_task", map[string]any{"task": 1})
	require.NotContains(t, claimed, "isError", toolResultText(t, claimed))
	commented := c.callTool("task_comments_create", map[string]any{"task": 1, "comment": "working on it"})
	require.NotContains(t, commented, "isError", toolResultText(t, commented))
	released := c.callTool("release_task", map[string]any{"task": 1})
	require.NotContains(t, released, "isError", toolResultText(t, released))

	// tasks_delete is not listed and cannot be called.
	resp := c.rpc("tools/call", map[string]any{"name": "tasks_delete", "arguments": map[string]any{"id": 1}})
	if result, ok := resp["result"].(map[string]any); ok {
		assert.Equal(t, true, result["isError"], "%v", resp)
	} else {
		assert.NotNil(t, resp["error"], "%v", resp)
	}
	refused := c.callTool("do_action", map[string]any{
		"action":    "task_labels_delete",
		"arguments": map[string]any{"task": 1, "label": 1},
	})
	assert.Equal(t, true, refused["isError"])

	// The REST routes refuse the token too.
	for _, path := range []string{"/api/v2/tasks/1", "/api/v1/tasks/1", "/api/v2/projects/1", "/api/v2/tasks/1/comments/1", "/api/v2/labels/1"} {
		rec := humaRequest(t, e, http.MethodDelete, path, "", token, "")
		assert.Contains(t, []int{http.StatusUnauthorized, http.StatusForbidden}, rec.Code, "%s: %s", path, rec.Body.String())
	}
	rec := humaRequest(t, e, http.MethodGet, "/api/v2/tasks/1", "", token, "")
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}
