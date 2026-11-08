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
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"code.vikunja.io/api/pkg/config"
	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/models"
	"code.vikunja.io/api/pkg/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/builder"
)

// prime-tasks A1: `vikunja bot create` / `vikunja bot revoke`.

func setupBotCLI(t *testing.T) {
	t.Helper()
	db.LoadAndAssertFixtures(t)
	ensureTokenRoutes()
}

func botTokens(t *testing.T, botID int64) []*models.APIToken {
	t.Helper()
	s := db.NewSession()
	defer s.Close()
	var tokens []*models.APIToken
	require.NoError(t, s.Where("owner_id = ?", botID).OrderBy("id").Find(&tokens))
	return tokens
}

func TestBotCLICreate(t *testing.T) {
	t.Run("creates bot-<name> owned by the owner with an agent-safe token", func(t *testing.T) {
		setupBotCLI(t)
		expires := time.Now().Add(90 * 24 * time.Hour).Truncate(time.Second)
		s := db.NewSession()
		defer s.Close()
		created, err := createAgentBot(context.Background(), s, agentBotOptions{
			Owner:   "user1",
			Name:    "claude",
			Expires: expires,
		})
		require.NoError(t, err)

		assert.Equal(t, "bot-claude", created.Bot.Username)
		assert.Equal(t, int64(1), created.Bot.BotOwnerID)
		assert.Equal(t, int64(1), created.Owner.ID)
		db.AssertExists(t, "users", map[string]any{"username": "bot-claude", "bot_owner_id": 1}, false)

		require.True(t, strings.HasPrefix(created.Token.Token, models.APITokenPrefix))
		tokens := botTokens(t, created.Bot.ID)
		require.Len(t, tokens, 1)
		assert.Equal(t, models.AgentSafeAPIPermissions(), tokens[0].APIPermissions)
		assert.Contains(t, tokens[0].APIPermissions["tasks"], "claim")
		assert.Contains(t, tokens[0].APIPermissions["mcp"], "access")
		assert.WithinDuration(t, expires, tokens[0].ExpiresAt, time.Second)
		assert.Equal(t, "bot-claude agent", tokens[0].Title)
		assert.Equal(t, models.HashAPIToken(created.Token.Token), tokens[0].TokenSha256)
	})

	t.Run("accepts the owner by id and a name that already has the prefix", func(t *testing.T) {
		setupBotCLI(t)
		s := db.NewSession()
		defer s.Close()
		created, err := createAgentBot(context.Background(), s, agentBotOptions{
			Owner:   "21",
			Name:    "bot-reviewer",
			Expires: time.Now().Add(time.Hour),
			Title:   "review agent",
		})
		require.NoError(t, err)
		assert.Equal(t, "bot-reviewer", created.Bot.Username)
		assert.Equal(t, int64(21), created.Bot.BotOwnerID)
		assert.Equal(t, "review agent", botTokens(t, created.Bot.ID)[0].Title)
	})

	t.Run("refuses an owner that does not exist or is a bot", func(t *testing.T) {
		setupBotCLI(t)
		for _, owner := range []string{"nobody-here", "9999", "bot-owner-a-assistant", "23"} {
			s := db.NewSession()
			_, err := createAgentBot(context.Background(), s, agentBotOptions{
				Owner:   owner,
				Name:    "x-" + owner,
				Expires: time.Now().Add(time.Hour),
			})
			s.Close()
			require.Error(t, err, owner)
		}
		db.AssertCount(t, "users", builder.Like{"username", "bot-x-%"}, 0)
	})

	t.Run("refuses an existing bot unless reuse is set, and only for its owner", func(t *testing.T) {
		setupBotCLI(t)
		opts := agentBotOptions{Owner: "user_bot_owner_a", Name: "owner-a-assistant", Expires: time.Now().Add(time.Hour)}
		s := db.NewSession()
		_, err := createAgentBot(context.Background(), s, opts)
		s.Close()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--reuse")
		assert.Empty(t, botTokens(t, 23))

		other := opts
		other.Owner = "user_bot_owner_b"
		other.Reuse = true
		s = db.NewSession()
		_, err = createAgentBot(context.Background(), s, other)
		s.Close()
		require.Error(t, err)
		assert.Empty(t, botTokens(t, 23))

		opts.Reuse = true
		s = db.NewSession()
		created, err := createAgentBot(context.Background(), s, opts)
		s.Close()
		require.NoError(t, err)
		assert.Equal(t, int64(23), created.Bot.ID)
		require.Len(t, botTokens(t, 23), 1)
		assert.Equal(t, models.AgentSafeAPIPermissions(), botTokens(t, 23)[0].APIPermissions)
	})

	t.Run("refuses a past expiry", func(t *testing.T) {
		setupBotCLI(t)
		s := db.NewSession()
		defer s.Close()
		_, err := createAgentBot(context.Background(), s, agentBotOptions{Owner: "user1", Name: "late", Expires: time.Now().Add(-time.Hour)})
		require.Error(t, err)
		db.AssertMissing(t, "users", map[string]any{"username": "bot-late"})
	})
}

func TestBotCLIParseExpiry(t *testing.T) {
	now := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	for in, want := range map[string]time.Time{
		"":                     now.Add(90 * 24 * time.Hour),
		"90d":                  now.Add(90 * 24 * time.Hour),
		"7d":                   now.Add(7 * 24 * time.Hour),
		"36h":                  now.Add(36 * time.Hour),
		"2027-01-31":           time.Date(2027, 1, 31, 0, 0, 0, 0, time.UTC),
		"2027-01-31T12:00:00Z": time.Date(2027, 1, 31, 12, 0, 0, 0, time.UTC),
	} {
		got, err := parseBotExpiry(in, now)
		require.NoError(t, err, in)
		assert.Equal(t, want, got, in)
	}
	for _, in := range []string{"soon", "-5d", "0d", "2020-01-01"} {
		_, err := parseBotExpiry(in, now)
		assert.Error(t, err, in)
	}
}

func TestBotCLIOutput(t *testing.T) {
	setupBotCLI(t)
	config.ServicePublicURL.Set("https://tasks.example.org/")
	t.Cleanup(func() { config.ServicePublicURL.Set("") })
	s := db.NewSession()
	defer s.Close()
	created, err := createAgentBot(context.Background(), s, agentBotOptions{Owner: "user1", Name: "codex", Expires: time.Now().Add(time.Hour)})
	require.NoError(t, err)

	var out bytes.Buffer
	writeAgentBot(&out, created)
	text := out.String()
	assert.Contains(t, text, "bot: bot-codex (id ")
	assert.Contains(t, text, "mcp_endpoint: https://tasks.example.org/api/v2/mcp\n")
	assert.Contains(t, text, "token: "+created.Token.Token+"\n")
	assert.Contains(t, text, `claude mcp add --transport http codex https://tasks.example.org/api/v2/mcp --header "Authorization: Bearer `+created.Token.Token+`"`)
	for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
		label, _, ok := strings.Cut(line, ": ")
		require.True(t, ok, "every line is labelled: %q", line)
		assert.NotContains(t, label, " ", line)
	}
}

func TestBotCLIRevoke(t *testing.T) {
	setupBotCLI(t)
	s := db.NewSession()
	created, err := createAgentBot(context.Background(), s, agentBotOptions{Owner: "user1", Name: "temp", Expires: time.Now().Add(time.Hour)})
	s.Close()
	require.NoError(t, err)
	s = db.NewSession()
	_, err = createAgentBot(context.Background(), s, agentBotOptions{Owner: "user1", Name: "temp", Expires: time.Now().Add(time.Hour), Reuse: true})
	s.Close()
	require.NoError(t, err)
	require.Len(t, botTokens(t, created.Bot.ID), 2)

	// Only the bot's owner can revoke it.
	s = db.NewSession()
	_, _, err = revokeAgentBot(context.Background(), s, "user2", "temp")
	s.Close()
	require.Error(t, err)
	require.Len(t, botTokens(t, created.Bot.ID), 2)

	s = db.NewSession()
	bot, deleted, err := revokeAgentBot(context.Background(), s, "user1", "temp")
	s.Close()
	require.NoError(t, err)
	assert.Equal(t, created.Bot.ID, bot.ID)
	assert.Equal(t, int64(2), deleted)
	assert.Empty(t, botTokens(t, created.Bot.ID))
	db.AssertExists(t, "users", map[string]any{"id": created.Bot.ID, "status": user.StatusDisabled}, false)

	// A new token for a revoked bot needs --reuse, which re-enables it.
	s = db.NewSession()
	again, err := createAgentBot(context.Background(), s, agentBotOptions{Owner: "user1", Name: "temp", Expires: time.Now().Add(time.Hour), Reuse: true})
	s.Close()
	require.NoError(t, err)
	assert.Equal(t, created.Bot.ID, again.Bot.ID)
	db.AssertExists(t, "users", map[string]any{"id": created.Bot.ID, "status": user.StatusActive}, false)
	require.Len(t, botTokens(t, created.Bot.ID), 1)

	// A missing bot cannot be revoked.
	s = db.NewSession()
	_, _, err = revokeAgentBot(context.Background(), s, "user1", "does-not-exist")
	s.Close()
	require.Error(t, err)
}

func TestBotCLIRevokeReleasesClaims(t *testing.T) {
	setupBotCLI(t)
	s := db.NewSession()
	created, err := createAgentBot(context.Background(), s, agentBotOptions{Owner: "user1", Name: "claimer", Expires: time.Now().Add(time.Hour)})
	s.Close()
	require.NoError(t, err)
	other, err := func() (*agentBot, error) {
		s := db.NewSession()
		defer s.Close()
		return createAgentBot(context.Background(), s, agentBotOptions{Owner: "user1", Name: "bystander", Expires: time.Now().Add(time.Hour)})
	}()
	require.NoError(t, err)

	// Both bots get write access to project 1 (owned by user 1) and claim one task each.
	s = db.NewSession()
	for _, b := range []*agentBot{created, other} {
		_, err = s.Insert(&models.ProjectUser{UserID: b.Bot.ID, ProjectID: 1, Permission: models.PermissionWrite})
		require.NoError(t, err)
	}
	require.NoError(t, s.Commit())
	s.Close()
	for taskID, b := range map[int64]*agentBot{1: created, 2: other} {
		s = db.NewSession()
		require.NoError(t, (&models.TaskClaim{TaskID: taskID}).Create(s, b.Bot))
		require.NoError(t, s.Commit())
		s.Close()
	}
	db.AssertExists(t, "task_claims", map[string]any{"task_id": 1, "bot_id": created.Bot.ID}, false)

	s = db.NewSession()
	_, _, err = revokeAgentBot(context.Background(), s, "user1", "claimer")
	s.Close()
	require.NoError(t, err)

	db.AssertMissing(t, "task_claims", map[string]any{"task_id": 1})
	db.AssertExists(t, "task_comments", map[string]any{
		"task_id":   1,
		"author_id": created.Bot.ID,
		"comment":   "<p>Released: bot-claimer was revoked</p>",
	}, false)
	db.AssertMissing(t, "label_tasks", builder.Eq{"task_id": 1, "label_id": claimLabelID(t)})
	// The other bot's claim is untouched.
	db.AssertExists(t, "task_claims", map[string]any{"task_id": 2, "bot_id": other.Bot.ID}, false)
}

func claimLabelID(t *testing.T) int64 {
	t.Helper()
	s := db.NewSession()
	defer s.Close()
	var id int64
	has, err := s.Table("labels").Where("title = ?", models.TaskClaimLabelTitle).OrderBy("id").Cols("id").Get(&id)
	require.NoError(t, err)
	require.True(t, has)
	return id
}
