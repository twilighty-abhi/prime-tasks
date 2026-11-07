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
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"code.vikunja.io/api/pkg/config"
	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/events"
	"code.vikunja.io/api/pkg/initialize"
	"code.vikunja.io/api/pkg/log"
	"code.vikunja.io/api/pkg/mail"
	"code.vikunja.io/api/pkg/models"
	"code.vikunja.io/api/pkg/modules/mcp"
	"code.vikunja.io/api/pkg/routes"
	"code.vikunja.io/api/pkg/user"

	"github.com/spf13/cobra"
	"xorm.io/xorm"
)

// prime-tasks A1: one bot per agent, created on the server so an owner who signs in with Google (no password)
// never has to log in from a script.

const (
	botUsernamePrefix  = "bot-"
	defaultBotLifetime = 90 * 24 * time.Hour
)

var (
	botFlagOwner   string
	botFlagName    string
	botFlagExpires string
	botFlagTitle   string
	botFlagReuse   bool
)

func init() {
	botCreateCmd.Flags().StringVar(&botFlagOwner, "owner", "", "Username or id of the human who owns the bot.")
	_ = botCreateCmd.MarkFlagRequired("owner")
	botCreateCmd.Flags().StringVar(&botFlagName, "name", "", "Name of the agent; the bot user is bot-<name>.")
	_ = botCreateCmd.MarkFlagRequired("name")
	botCreateCmd.Flags().StringVar(&botFlagExpires, "expires", "90d", "When the token expires: a lifetime (90d, 36h) or a date (2027-01-31, RFC 3339).")
	botCreateCmd.Flags().StringVar(&botFlagTitle, "title", "", "Title of the token. Defaults to \"bot-<name> agent\".")
	botCreateCmd.Flags().BoolVar(&botFlagReuse, "reuse", false, "If the bot already exists (and belongs to the owner), re-enable it and issue a new token instead of failing.")

	botRevokeCmd.Flags().StringVar(&botFlagOwner, "owner", "", "Username or id of the human who owns the bot.")
	_ = botRevokeCmd.MarkFlagRequired("owner")
	botRevokeCmd.Flags().StringVar(&botFlagName, "name", "", "Name of the agent (bot-<name>).")
	_ = botRevokeCmd.MarkFlagRequired("name")

	botCmd.AddCommand(botCreateCmd, botRevokeCmd)
	rootCmd.AddCommand(botCmd)
}

var botCmd = &cobra.Command{
	Use:   "bot",
	Short: "Manage agent bots: one bot user per agent, with an Agent (safe) MCP token.",
	PersistentPostRun: func(_ *cobra.Command, _ []string) {
		mail.StopMailDaemon()
	},
}

var botCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create bot-<name> for an owner and print a new Agent (safe) MCP token for it.",
	Long: `Creates the bot user bot-<name> owned by --owner and issues an API token with the Agent (safe)
permissions (the same set as the MCP settings preset). The token is printed once on stdout, with the MCP
endpoint and a ready "claude mcp add" command. It cannot be shown again.`,
	PreRun: func(_ *cobra.Command, _ []string) {
		initBotCLI()
	},
	Run: func(cmd *cobra.Command, _ []string) {
		expires, err := parseBotExpiry(botFlagExpires, time.Now())
		if err != nil {
			log.Fatalf("%s", err)
		}
		s := db.NewSession()
		defer s.Close()
		created, err := createAgentBot(cmd.Context(), s, agentBotOptions{
			Owner:   botFlagOwner,
			Name:    botFlagName,
			Expires: expires,
			Title:   botFlagTitle,
			Reuse:   botFlagReuse,
		})
		if err != nil {
			log.Fatalf("%s", err)
		}
		writeAgentBot(cmd.OutOrStdout(), created)
		fmt.Fprintln(os.Stderr, "Copy the token now: it is not stored in cleartext and cannot be shown again.")
	},
}

var botRevokeCmd = &cobra.Command{
	Use:   "revoke",
	Short: "Delete every API token of bot-<name> and disable the bot.",
	Long: `Deletes all API tokens of the bot and disables it, so it can no longer call the API or MCP.
Its project shares, comments and history stay. "bot create --reuse" re-enables it with a new token.`,
	PreRun: func(_ *cobra.Command, _ []string) {
		initBotCLI()
	},
	Run: func(cmd *cobra.Command, _ []string) {
		s := db.NewSession()
		defer s.Close()
		bot, deleted, err := revokeAgentBot(cmd.Context(), s, botFlagOwner, botFlagName)
		if err != nil {
			log.Fatalf("%s", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "bot: %s (id %d)\ndeleted_tokens: %d\nstatus: disabled\n", bot.Username, bot.ID, deleted)
	},
}

type agentBotOptions struct {
	Owner   string
	Name    string
	Expires time.Time
	Title   string
	Reuse   bool
}

type agentBot struct {
	// Name is the agent name without the bot- prefix.
	Name  string
	Bot   *user.User
	Owner *user.User
	Token *models.APIToken
}

// initBotCLI keeps stdout for the labelled result lines: the logs go to stderr.
func initBotCLI() {
	for _, key := range []string{"VIKUNJA_LOG_STANDARD", "VIKUNJA_LOG_HTTP"} {
		if err := os.Setenv(key, "stderr"); err != nil {
			fmt.Fprintf(os.Stderr, "could not set %s: %s\n", key, err)
			os.Exit(1)
		}
	}
	initialize.FullInit()
}

var tokenRoutesOnce sync.Once

// ensureTokenRoutes registers the API routes once so token permissions resolve: they derive from the routes, and
// the CLI never starts the web server.
func ensureTokenRoutes() {
	tokenRoutesOnce.Do(func() {
		routes.RegisterRoutes(routes.NewEcho())
	})
}

func agentBotName(name string) (short, username string) {
	short = strings.TrimPrefix(strings.TrimSpace(name), botUsernamePrefix)
	return short, botUsernamePrefix + short
}

// parseBotExpiry reads a lifetime (90d, 36h) or a date (2027-01-31, RFC 3339); empty means 90 days.
func parseBotExpiry(value string, now time.Time) (time.Time, error) {
	value = strings.TrimSpace(value)
	var expires time.Time
	switch {
	case value == "":
		expires = now.Add(defaultBotLifetime)
	case strings.HasSuffix(value, "d"):
		days, err := strconv.Atoi(strings.TrimSuffix(value, "d"))
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid --expires %q: use a lifetime like 90d or 36h, or a date like 2027-01-31", value)
		}
		expires = now.Add(time.Duration(days) * 24 * time.Hour)
	default:
		if d, err := time.ParseDuration(value); err == nil {
			expires = now.Add(d)
			break
		}
		if t, err := time.Parse(time.DateOnly, value); err == nil {
			expires = t
			break
		}
		t, err := time.Parse(time.RFC3339, value)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid --expires %q: use a lifetime like 90d or 36h, or a date like 2027-01-31", value)
		}
		expires = t
	}
	if !expires.After(now) {
		return time.Time{}, fmt.Errorf("--expires %q is not in the future", value)
	}
	return expires, nil
}

func findBotOwner(s *xorm.Session, identifier string) (*user.User, error) {
	filter := &user.User{}
	if id, err := strconv.ParseInt(identifier, 10, 64); err == nil {
		filter.ID = id
	} else {
		filter.Username = identifier
	}
	owner, err := user.GetUserWithEmail(s, filter)
	if err != nil {
		return nil, fmt.Errorf("owner %q: %w", identifier, err)
	}
	if owner.IsBot() {
		return nil, fmt.Errorf("owner %q is a bot; the owner must be a person", identifier)
	}
	return owner, nil
}

// findOwnedBot loads the bot in any status; a missing bot is (nil, nil).
func findOwnedBot(s *xorm.Session, username string, owner *user.User) (*user.User, error) {
	bot := &user.User{}
	has, err := s.Where("username = ?", username).Get(bot)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, nil
	}
	if !bot.IsBotOwnedBy(owner) {
		return nil, fmt.Errorf("%s exists and is not a bot owned by %s", username, owner.Username)
	}
	return bot, nil
}

func setBotStatus(s *xorm.Session, bot, owner *user.User, status user.Status) error {
	if bot.Status == status {
		return nil
	}
	if err := user.SetUserStatus(s, bot, status); err != nil {
		return err
	}
	events.DispatchOnCommit(s, &models.BotStatusChangedEvent{Bot: bot, Doer: owner, OldStatus: bot.Status, NewStatus: status})
	bot.Status = status
	return nil
}

// createAgentBot creates bot-<name> for the owner (or, with Reuse, takes the owner's existing one and re-enables it)
// and issues one token with exactly the Agent (safe) permissions. It commits the session.
func createAgentBot(ctx context.Context, s *xorm.Session, opts agentBotOptions) (created *agentBot, err error) {
	defer func() {
		if err != nil {
			_ = s.Rollback()
		}
	}()
	if !opts.Expires.After(time.Now()) {
		return nil, errors.New("the token expiry must be in the future")
	}
	ensureTokenRoutes()
	permissions := models.AgentSafeAPIPermissions()
	if len(permissions["tasks"]) == 0 {
		return nil, errors.New("the Agent (safe) permissions did not resolve; API routes are not registered")
	}

	short, username := agentBotName(opts.Name)
	if short == "" {
		return nil, errors.New("--name is required")
	}
	owner, err := findBotOwner(s, opts.Owner)
	if err != nil {
		return nil, err
	}
	bot, err := findOwnedBot(s, username, owner)
	if err != nil {
		return nil, err
	}
	switch {
	case bot != nil && !opts.Reuse:
		return nil, fmt.Errorf("%s already exists; pass --reuse to issue a new token for it", username)
	case bot != nil:
		if err = setBotStatus(s, bot, owner, user.StatusActive); err != nil {
			return nil, err
		}
	default:
		bot, err = user.CreateBotUser(s, &user.User{Username: username}, owner)
		if err != nil {
			return nil, fmt.Errorf("could not create %s: %w", username, err)
		}
	}

	title := opts.Title
	if title == "" {
		title = username + " agent"
	}
	token := &models.APIToken{
		Title:          title,
		APIPermissions: permissions,
		ExpiresAt:      opts.Expires,
		OwnerID:        bot.ID,
	}
	if err = token.Create(s, owner); err != nil {
		return nil, fmt.Errorf("could not create the token: %w", err)
	}
	if err = s.Commit(); err != nil {
		return nil, err
	}
	events.DispatchPending(ctx, s)
	return &agentBot{Name: short, Bot: bot, Owner: owner, Token: token}, nil
}

// revokeAgentBot deletes every token of the owner's bot-<name> and disables it. It commits the session.
func revokeAgentBot(ctx context.Context, s *xorm.Session, ownerIdentifier, name string) (bot *user.User, deleted int64, err error) {
	defer func() {
		if err != nil {
			_ = s.Rollback()
		}
	}()
	_, username := agentBotName(name)
	owner, err := findBotOwner(s, ownerIdentifier)
	if err != nil {
		return nil, 0, err
	}
	bot, err = findOwnedBot(s, username, owner)
	if err != nil {
		return nil, 0, err
	}
	if bot == nil {
		return nil, 0, fmt.Errorf("%s does not exist", username)
	}
	deleted, err = s.Where("owner_id = ?", bot.ID).Delete(&models.APIToken{})
	if err != nil {
		return nil, 0, err
	}
	if err = setBotStatus(s, bot, owner, user.StatusDisabled); err != nil {
		return nil, 0, err
	}
	if err = s.Commit(); err != nil {
		return nil, 0, err
	}
	events.DispatchPending(ctx, s)
	return bot, deleted, nil
}

func agentBotMCPEndpoint() string {
	return strings.TrimRight(config.ServicePublicURL.GetString(), "/") + mcp.RoutePrefix
}

// writeAgentBot prints labelled lines only; the token appears in the token and claude lines and nowhere else.
func writeAgentBot(w io.Writer, b *agentBot) {
	endpoint := agentBotMCPEndpoint()
	fmt.Fprintf(w, "bot: %s (id %d)\n", b.Bot.Username, b.Bot.ID)
	fmt.Fprintf(w, "owner: %s (id %d)\n", b.Owner.Username, b.Owner.ID)
	fmt.Fprintf(w, "token_expires: %s\n", b.Token.ExpiresAt.UTC().Format(time.RFC3339))
	fmt.Fprintf(w, "mcp_endpoint: %s\n", endpoint)
	fmt.Fprintf(w, "token: %s\n", b.Token.Token)
	fmt.Fprintf(w, "claude: claude mcp add --transport http %s %s --header \"Authorization: Bearer %s\"\n", b.Name, endpoint, b.Token.Token)
}
