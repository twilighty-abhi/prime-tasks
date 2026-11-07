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
	"fmt"
	"net/http"
	"testing"
	"time"

	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/models"
	"code.vikunja.io/api/pkg/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Bots 23 and 25 are owned by user 21; project 1 (owned by user 1) is shared with them per test.
const claimBotAName = "bot-owner-a-assistant"

var claimBotPermissions = models.APIPermissions{
	"mcp":   {"access"},
	"tasks": {"claim", "release", "read_one"},
}

func shareProjectWithClaimBots(t *testing.T, projectID int64, botIDs ...int64) {
	t.Helper()
	s := db.NewSession()
	defer s.Close()
	for _, id := range botIDs {
		_, err := s.Insert(&models.ProjectUser{
			UserID:     id,
			ProjectID:  projectID,
			Permission: models.PermissionWrite,
		})
		require.NoError(t, err)
	}
	require.NoError(t, s.Commit())
}

// Bots have no password, so they authenticate with a token their owner issues for them.
func botAPIToken(t *testing.T, botID int64, permissions models.APIPermissions) string {
	t.Helper()
	s := db.NewSession()
	defer s.Close()
	owner, err := user.GetUserByID(s, 21)
	require.NoError(t, err)
	token := &models.APIToken{
		Title:          fmt.Sprintf("claim bot %d", botID),
		OwnerID:        botID,
		APIPermissions: permissions,
		ExpiresAt:      time.Now().Add(time.Hour),
	}
	require.NoError(t, token.Create(s, owner))
	require.NoError(t, s.Commit())
	return token.Token
}

func TestTaskClaimV2(t *testing.T) {
	t.Run("claim, refuse a second bot, show on the task, release into the done bucket", func(t *testing.T) {
		e, err := setupTestEnv()
		require.NoError(t, err)
		shareProjectWithClaimBots(t, 1, 23, 25)
		botA := botAPIToken(t, 23, claimBotPermissions)
		botB := botAPIToken(t, 25, claimBotPermissions)

		rec := humaRequest(t, e, http.MethodPost, "/api/v2/tasks/1/claim", ``, botA, "")
		require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
		var claim struct {
			TaskID    int64     `json:"task_id"`
			BotID     int64     `json:"bot_id"`
			ExpiresAt time.Time `json:"expires_at"`
			Bot       struct {
				Username string `json:"username"`
			} `json:"bot"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &claim))
		assert.Equal(t, int64(1), claim.TaskID)
		assert.Equal(t, int64(23), claim.BotID)
		assert.Equal(t, claimBotAName, claim.Bot.Username)
		assert.WithinDuration(t, time.Now().Add(2*time.Hour), claim.ExpiresAt, time.Minute)

		rec = humaRequest(t, e, http.MethodPost, "/api/v2/tasks/1/claim", ``, botB, "")
		require.Equal(t, http.StatusConflict, rec.Code, "body: %s", rec.Body.String())
		assert.Contains(t, rec.Body.String(), "task is claimed by "+claimBotAName+" until ")
		assert.Contains(t, rec.Body.String(), fmt.Sprintf(`"code":%d`, models.ErrCodeTaskClaimedByOtherBot))

		rec = humaRequest(t, e, http.MethodGet, "/api/v2/tasks/1", ``, humaTokenFor(t, &testuser1), "")
		require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
		var task struct {
			Done  bool `json:"done"`
			Claim *struct {
				BotID int64 `json:"bot_id"`
				Bot   struct {
					Username string `json:"username"`
				} `json:"bot"`
			} `json:"claim"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &task))
		require.NotNil(t, task.Claim)
		assert.Equal(t, int64(23), task.Claim.BotID)
		assert.Equal(t, claimBotAName, task.Claim.Bot.Username)

		rec = humaRequest(t, e, http.MethodPost, "/api/v2/tasks/1/release?format=markdown",
			`{"comment":"Opened **PR 42**","bucket_id":3}`, botA, "")
		require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

		rec = humaRequest(t, e, http.MethodGet, "/api/v2/tasks/1", ``, humaTokenFor(t, &testuser1), "")
		require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
		task.Claim = nil
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &task))
		assert.Nil(t, task.Claim)
		assert.True(t, task.Done, "bucket 3 is the done bucket")
		db.AssertExists(t, "task_comments", map[string]any{
			"task_id":   1,
			"author_id": 23,
			"comment":   "<p>Opened <strong>PR 42</strong></p>",
		}, false)
	})

	t.Run("a human is refused with 403", func(t *testing.T) {
		e, err := setupTestEnv()
		require.NoError(t, err)

		rec := humaRequest(t, e, http.MethodPost, "/api/v2/tasks/1/claim", ``, humaTokenFor(t, &testuser1), "")
		require.Equal(t, http.StatusForbidden, rec.Code, "body: %s", rec.Body.String())
		assert.Contains(t, rec.Body.String(), fmt.Sprintf(`"code":%d`, models.ErrCodeOnlyBotsCanClaimTasks))
	})

	t.Run("a human with write access releases a bot's claim", func(t *testing.T) {
		e, err := setupTestEnv()
		require.NoError(t, err)
		shareProjectWithClaimBots(t, 1, 23)
		botA := botAPIToken(t, 23, claimBotPermissions)
		rec := humaRequest(t, e, http.MethodPost, "/api/v2/tasks/1/claim", ``, botA, "")
		require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

		rec = humaRequest(t, e, http.MethodPost, "/api/v2/tasks/1/release", ``, humaTokenFor(t, &testuser1), "")
		require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
		db.AssertMissing(t, "task_claims", map[string]any{"task_id": 1})
	})

	t.Run("a stranger cannot release", func(t *testing.T) {
		e, err := setupTestEnv()
		require.NoError(t, err)
		shareProjectWithClaimBots(t, 1, 23)
		botA := botAPIToken(t, 23, claimBotPermissions)
		rec := humaRequest(t, e, http.MethodPost, "/api/v2/tasks/1/claim", ``, botA, "")
		require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

		rec = humaRequest(t, e, http.MethodPost, "/api/v2/tasks/1/release", ``, humaTokenFor(t, &testuser15), "")
		assert.Equal(t, http.StatusForbidden, rec.Code, "body: %s", rec.Body.String())
		db.AssertExists(t, "task_claims", map[string]any{"task_id": 1}, false)
	})

	t.Run("a task in a trashed project answers project-in-trash", func(t *testing.T) {
		e, err := setupTestEnv()
		require.NoError(t, err)
		shareProjectWithClaimBots(t, 45, 23)
		botA := botAPIToken(t, 23, claimBotPermissions)

		rec := humaRequest(t, e, http.MethodPost, "/api/v2/tasks/53/claim", ``, botA, "")
		require.Equal(t, http.StatusNotFound, rec.Code, "body: %s", rec.Body.String())
		assert.Contains(t, rec.Body.String(), fmt.Sprintf(`"code":%d`, models.ErrCodeProjectIsInTrash))
	})
}

func TestTaskClaimV2_APITokenPermissions(t *testing.T) {
	_, err := setupTestEnv()
	require.NoError(t, err)
	routes := models.GetAPITokenRoutes()
	require.Contains(t, routes, "tasks")
	claim := routes["tasks"]["claim"]
	require.NotNil(t, claim)
	assert.Equal(t, http.MethodPost, claim.Method)
	assert.Equal(t, "/api/v2/tasks/:task/claim", claim.Path)
	release := routes["tasks"]["release"]
	require.NotNil(t, release)
	assert.Equal(t, http.MethodPost, release.Method)
	assert.Equal(t, "/api/v2/tasks/:task/release", release.Path)

	token := &models.APIToken{APIPermissions: models.APIPermissions{"tasks": {"claim", "release"}}}
	assert.True(t, token.CanUseRoute("/api/v2/tasks/:task/claim", http.MethodPost))
	assert.True(t, token.CanUseRoute("/api/v2/tasks/:task/release", http.MethodPost))
}

func TestMCP_Tools_ClaimAndRelease(t *testing.T) {
	e, err := setupTestEnv()
	require.NoError(t, err)
	shareProjectWithClaimBots(t, 1, 23)
	c := newMCPClientOn(t, e, botAPIToken(t, 23, claimBotPermissions))

	names := c.toolNames()
	assert.True(t, names["claim_task"], "tools: %v", names)
	assert.True(t, names["release_task"], "tools: %v", names)

	var claim map[string]any
	toolResultJSON(t, c.callTool("claim_task", map[string]any{"task": 1}), &claim)
	assert.InDelta(t, 23, claim["bot_id"], 0.001)

	released := c.callTool("release_task", map[string]any{
		"task":    1,
		"comment": "done via mcp",
		"format":  "markdown",
	})
	require.NotContains(t, released, "isError", toolResultText(t, released))
	db.AssertMissing(t, "task_claims", map[string]any{"task_id": 1})
	db.AssertExists(t, "task_comments", map[string]any{
		"task_id":   1,
		"author_id": 23,
		"comment":   "<p>done via mcp</p>",
	}, false)
}
