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
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stringKeyMap accepts both shapes a nested config map arrives in (YAML and env/JSON).
func stringKeyMap(t *testing.T, raw any) map[string]any {
	t.Helper()
	switch m := raw.(type) {
	case map[string]any:
		return m
	case map[any]any:
		out := make(map[string]any, len(m))
		for k, v := range m {
			key, ok := k.(string)
			require.True(t, ok, "non-string key %v", k)
			out[key] = v
		}
		return out
	default:
		require.Failf(t, "not a map", "got %T", raw)
		return nil
	}
}

// prime-tasks G1: deploy/config.yml turns on Google sign-in, and the client id and secret
// come from the environment. docker-compose.yml maps PRIME_GOOGLE_CLIENT_ID and
// PRIME_GOOGLE_CLIENT_SECRET from .env onto the variables set here, so this is the exact
// path the VM uses.
func TestDeployConfigGoogleSignIn(t *testing.T) {
	deployConfig, err := filepath.Abs(filepath.Join("..", "..", "deploy", "config.yml"))
	require.NoError(t, err)

	t.Setenv("VIKUNJA_AUTH_OPENID_PROVIDERS_GOOGLE_CLIENTID", "id-from-env.apps.googleusercontent.com")
	t.Setenv("VIKUNJA_AUTH_OPENID_PROVIDERS_GOOGLE_CLIENTSECRET", "secret-from-env")
	// docker-compose.yml sets it from PRIME_DOMAIN; cors (on by default) needs it.
	t.Setenv("VIKUNJA_SERVICE_PUBLICURL", "https://tasks.example.org/")
	SetConfigFile(deployConfig)
	t.Cleanup(func() {
		SetConfigFile("")
		viper.Reset()
	})
	viper.Reset()
	InitConfig()

	t.Run("the google provider merges the file with the environment", func(t *testing.T) {
		require.True(t, AuthOpenIDEnabled.GetBool())
		providers := stringKeyMap(t, AuthOpenIDProviders.Get())
		require.Contains(t, providers, "google")
		google := stringKeyMap(t, providers["google"])

		assert.Equal(t, "Google", google["name"])
		assert.Equal(t, "https://accounts.google.com", google["authurl"])
		assert.Equal(t, "openid email profile", google["scope"])
		assert.Equal(t, true, google["emailfallback"])
		assert.Equal(t, true, google["usernamefallback"])
		assert.Equal(t, "id-from-env.apps.googleusercontent.com", google["clientid"])
		assert.Equal(t, "secret-from-env", google["clientsecret"])
	})
	t.Run("the public url comes from the environment", func(t *testing.T) {
		assert.Equal(t, "https://tasks.example.org/", ServicePublicURL.GetString())
	})
	t.Run("local login stays on, folded under the provider buttons", func(t *testing.T) {
		assert.True(t, AuthLocalEnabled.GetBool())
		assert.True(t, AuthLocalFoldOnLogin.GetBool())
	})
	t.Run("registration and the mailer stay off", func(t *testing.T) {
		assert.False(t, ServiceEnableRegistration.GetBool())
		assert.False(t, MailerEnabled.GetBool())
	})
}

// prime-tasks G1: the folded login form is opt-in; upstream shows the form as before.
func TestAuthLocalFoldOnLoginDefault(t *testing.T) {
	initConfigFromYAML(t, "")

	assert.False(t, AuthLocalFoldOnLogin.GetBool())
}
