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

package openid

import (
	"context"
	"testing"

	"code.vikunja.io/api/pkg/config"
	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/events"
	"code.vikunja.io/api/pkg/models"
	"code.vikunja.io/api/pkg/user"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/stretchr/testify/require"
	"xorm.io/builder"
)

// prime-tasks F6: an OpenID sign-in that creates a user joins the auto-join teams exactly
// once, through the single user.created event; the sign-in itself joins nothing.
func TestGetOrCreateUserAutoJoinsTeamsOnce(t *testing.T) {
	db.LoadAndAssertFixtures(t)
	config.ServiceAutoJoinTeams.Set([]interface{}{map[string]interface{}{"domain": "example.org", "team_id": 1}})
	t.Cleanup(func() { config.ServiceAutoJoinTeams.Set(nil) })
	events.ClearDispatchedEvents()

	s := db.NewSession()
	defer s.Close()
	cl := &claims{Email: "oidc-joiner@example.org", PreferredUsername: "oidc-joiner"}
	u, _, err := getOrCreateUser(s, cl, &Provider{}, &oidc.IDToken{Issuer: "https://some.issuer", Subject: "autojoin-1"})
	require.NoError(t, err)
	require.NoError(t, s.Commit())
	events.DispatchPending(context.Background(), s)

	created := events.GetDispatchedEvents((&user.CreatedEvent{}).Name())
	require.Len(t, created, 1, "sign-in dispatches user.created once")
	db.AssertCount(t, "team_members", builder.Eq{"user_id": u.ID}, 0)

	events.TestListener(t, created[0], &models.AutoJoinTeamsByEmailDomain{})
	events.TestListener(t, created[0], &models.AutoJoinTeamsByEmailDomain{})
	db.AssertCount(t, "team_members", builder.Eq{"team_id": 1, "user_id": u.ID}, 1)
}
