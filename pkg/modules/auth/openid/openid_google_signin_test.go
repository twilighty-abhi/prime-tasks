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
	"testing"

	"code.vikunja.io/api/pkg/config"
	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/events"
	"code.vikunja.io/api/pkg/models"
	"code.vikunja.io/api/pkg/modules/keyvalue"
	"code.vikunja.io/api/pkg/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/builder"
)

const googleProviderKey = "google"

// configureGoogleLike points a provider configured like deploy/config.yml's google entry at the mock IdP.
func (idp *mockIDP) configureGoogleLike(t *testing.T) {
	t.Helper()
	config.AuthOpenIDEnabled.Set(true)
	config.AuthOpenIDProviders.Set(map[string]interface{}{
		googleProviderKey: map[string]interface{}{
			"name":             "Google",
			"authurl":          idp.URL,
			"clientid":         "client1",
			"clientsecret":     "secret1",
			"scope":            "openid email profile",
			"emailfallback":    true,
			"usernamefallback": true,
		},
	})
	_ = keyvalue.Del("openid_providers")
	_ = keyvalue.Del("openid_provider_" + googleProviderKey)
	t.Cleanup(CleanupSavedOpenIDProviders)
}

// runCreatedListeners runs the F6 listener for every user.created event the sign-in queued,
// the way the async event router does in production.
func runCreatedListeners(t *testing.T) int {
	t.Helper()
	created := events.GetDispatchedEvents((&user.CreatedEvent{}).Name())
	for _, e := range created {
		events.TestListener(t, e, &models.AutoJoinTeamsByEmailDomain{})
	}
	events.ClearDispatchedEvents()
	return len(created)
}

// prime-tasks G1: Google sign-in on the office deploy. The first sign-in creates the account
// and F6 puts it in the team for its email domain; later sign-ins reuse the same account.
func TestGoogleSignIn(t *testing.T) {
	t.Run("first sign-in creates the account and joins the domain's team, the next reuses it", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		config.ServiceAutoJoinTeams.Set([]interface{}{map[string]interface{}{"domain": "example.org", "team_id": 1}})
		t.Cleanup(func() { config.ServiceAutoJoinTeams.Set(nil) })
		events.ClearDispatchedEvents()

		idp := newMockIDP(t)
		idp.configureGoogleLike(t)
		idp.setClaims(map[string]any{
			"sub":                "108234567890123456789",
			"email":              "asha@example.org",
			"email_verified":     true,
			"name":               "Asha Rao",
			"preferred_username": "asha",
		})

		first, _, err := AuthenticateCallback(t.Context(), &Callback{Code: "code"}, googleProviderKey)
		require.NoError(t, err)
		assert.Equal(t, idp.URL, first.Issuer)
		assert.Equal(t, "108234567890123456789", first.Subject)
		db.AssertCount(t, "users", builder.Eq{"email": "asha@example.org"}, 1)
		require.Equal(t, 1, runCreatedListeners(t), "the first sign-in creates one user")
		db.AssertCount(t, "team_members", builder.Eq{"team_id": 1, "user_id": first.ID}, 1)

		second, _, err := AuthenticateCallback(t.Context(), &Callback{Code: "code"}, googleProviderKey)
		require.NoError(t, err)
		assert.Equal(t, first.ID, second.ID)
		db.AssertCount(t, "users", builder.Eq{"email": "asha@example.org"}, 1)
		assert.Equal(t, 0, runCreatedListeners(t), "a second sign-in creates no user")
		db.AssertCount(t, "team_members", builder.Eq{"user_id": first.ID}, 1)
	})
	t.Run("an account from another domain joins no team", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		config.ServiceAutoJoinTeams.Set([]interface{}{map[string]interface{}{"domain": "example.org", "team_id": 1}})
		t.Cleanup(func() { config.ServiceAutoJoinTeams.Set(nil) })
		events.ClearDispatchedEvents()

		idp := newMockIDP(t)
		idp.configureGoogleLike(t)
		idp.setClaims(map[string]any{
			"sub":                "208234567890123456789",
			"email":              "guest@gmail.example",
			"email_verified":     true,
			"name":               "Guest",
			"preferred_username": "guest",
		})

		u, _, err := AuthenticateCallback(t.Context(), &Callback{Code: "code"}, googleProviderKey)
		require.NoError(t, err)
		require.Equal(t, 1, runCreatedListeners(t))
		db.AssertCount(t, "team_members", builder.Eq{"user_id": u.ID}, 0)
	})
	t.Run("a verified email signs in to the account an admin created for it", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		events.ClearDispatchedEvents()

		idp := newMockIDP(t)
		idp.configureGoogleLike(t)
		// user1 is a local account, as `vikunja user create` makes them.
		idp.setClaims(map[string]any{
			"sub":                "308234567890123456789",
			"email":              "user1@example.com",
			"email_verified":     true,
			"name":               "User One",
			"preferred_username": "someone-else",
		})

		u, _, err := AuthenticateCallback(t.Context(), &Callback{Code: "code"}, googleProviderKey)
		require.NoError(t, err)
		assert.Equal(t, int64(1), u.ID)
		db.AssertCount(t, "users", builder.Eq{"email": "user1@example.com"}, 1)
		assert.Equal(t, 0, runCreatedListeners(t))
	})
}
