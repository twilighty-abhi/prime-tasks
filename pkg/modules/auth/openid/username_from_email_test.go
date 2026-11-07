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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/user"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/xorm"
)

func createOpenIDUserForTest(t *testing.T, s *xorm.Session, provider *Provider, cl *claims, subject string) *user.User {
	t.Helper()
	idToken := &oidc.IDToken{Issuer: "https://accounts.example.com", Subject: subject}
	u, _, err := getOrCreateUser(s, cl, provider, idToken)
	require.NoError(t, err)
	return u
}

func TestGetOrCreateUserUsernameFromEmail(t *testing.T) {
	provider := &Provider{UsernameFromEmail: true}

	t.Run("no preferred_username uses the email local part", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		cl := &claims{Email: "abhiram@t4gc.org", Name: "Abhiram N J"}
		u := createOpenIDUserForTest(t, s, provider, cl, "sub-1")
		require.NoError(t, s.Commit())

		assert.Equal(t, "abhiram", u.Username)
		assert.Equal(t, "Abhiram N J", u.Name)
		db.AssertExists(t, "users", map[string]interface{}{
			"id":       u.ID,
			"username": "abhiram",
			"name":     "Abhiram N J",
			"email":    "abhiram@t4gc.org",
		}, false)
	})

	t.Run("clashes get a numeric suffix starting at 2", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		u1 := createOpenIDUserForTest(t, s, provider, &claims{Email: "abhiram@t4gc.org"}, "sub-1")
		u2 := createOpenIDUserForTest(t, s, provider, &claims{Email: "abhiram@other.org"}, "sub-2")
		u3 := createOpenIDUserForTest(t, s, provider, &claims{Email: "abhiram@third.org"}, "sub-3")
		require.NoError(t, s.Commit())

		assert.Equal(t, "abhiram", u1.Username)
		assert.Equal(t, "abhiram2", u2.Username)
		assert.Equal(t, "abhiram3", u3.Username)
	})

	t.Run("clash check is case-insensitive", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		u1 := createOpenIDUserForTest(t, s, provider, &claims{Email: "x@t4gc.org", PreferredUsername: "Abhiram"}, "sub-1")
		u2 := createOpenIDUserForTest(t, s, provider, &claims{Email: "ABHIRAM@t4gc.org"}, "sub-2")
		require.NoError(t, s.Commit())

		assert.Equal(t, "Abhiram", u1.Username)
		assert.Equal(t, "abhiram2", u2.Username)
	})

	t.Run("a clash with a name ending in a digit uses a dash before the suffix", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		// user1 exists in the fixtures; user12 does too, so plain appending would be ambiguous.
		u := createOpenIDUserForTest(t, s, provider, &claims{Email: "user1@corp.example"}, "sub-1")
		require.NoError(t, s.Commit())

		assert.Equal(t, "user1-2", u.Username)
	})

	t.Run("a clash with a bot user counts", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		// Bot usernames start with bot-, which is reserved, so the base is
		// prefixed; the existing bot still must not be reused.
		u := createOpenIDUserForTest(t, s, provider, &claims{Email: "bot-owner-a-assistant@corp.example"}, "sub-1")
		require.NoError(t, s.Commit())

		assert.Equal(t, "user-bot-owner-a-assistant", u.Username)
	})

	t.Run("preferred_username is used when present", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		u := createOpenIDUserForTest(t, s, provider, &claims{Email: "abhiram@t4gc.org", PreferredUsername: "abhi nj"}, "sub-1")
		require.NoError(t, s.Commit())

		assert.Equal(t, "abhi-nj", u.Username)
	})

	t.Run("preferred_username clash gets a numeric suffix", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		u := createOpenIDUserForTest(t, s, provider, &claims{Email: "abhiram@t4gc.org", PreferredUsername: "user_openid_avatar"}, "sub-1")
		require.NoError(t, s.Commit())

		assert.Equal(t, "user_openid_avatar2", u.Username)
	})

	t.Run("display name with spaces and unicode is kept exactly", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		name := "Ábhíram ñ 日本"
		u := createOpenIDUserForTest(t, s, provider, &claims{Email: "abhiram@t4gc.org", Name: name}, "sub-1")
		require.NoError(t, s.Commit())

		assert.Equal(t, "abhiram", u.Username)
		assert.Equal(t, name, u.Name)
		db.AssertExists(t, "users", map[string]interface{}{
			"id":       u.ID,
			"username": "abhiram",
			"name":     name,
		}, false)
	})

	t.Run("no usable local part falls back to a random username", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		u := createOpenIDUserForTest(t, s, provider, &claims{Email: "+tag@t4gc.org"}, "sub-1")
		require.NoError(t, s.Commit())

		assert.NotEmpty(t, u.Username)
		assert.Len(t, strings.Split(u.Username, "-"), 3, "expected a petname, got %q", u.Username)
	})

	t.Run("existing user from the issuer is not renamed", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		cl := &claims{Email: "abhiram@t4gc.org", Name: "Abhiram N J"}
		idToken := &oidc.IDToken{Issuer: "https://some.service.com", Subject: "12345"}
		u, _, err := getOrCreateUser(s, cl, provider, idToken)
		require.NoError(t, err)
		require.NoError(t, s.Commit())

		assert.Equal(t, int64(14), u.ID)
		assert.Equal(t, "user14", u.Username)
	})

	t.Run("flag off keeps the upstream random username", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		u := createOpenIDUserForTest(t, s, &Provider{}, &claims{Email: "abhiram@t4gc.org"}, "sub-1")
		require.NoError(t, s.Commit())

		assert.NotEqual(t, "abhiram", u.Username)
		assert.Len(t, strings.Split(u.Username, "-"), 3, "expected a petname, got %q", u.Username)
	})
}

func TestUsernameCandidateFromEmail(t *testing.T) {
	cases := map[string]string{
		"abhiram@t4gc.org":          "abhiram",
		"Abhiram@T4GC.org":          "abhiram",
		"abhiram+tasks@t4gc.org":    "abhiram",
		"first.last@t4gc.org":       "first-last", // a dotted name looks like a URL to the registration validator
		"first_last@t4gc.org":       "first_last",
		"o'brien@t4gc.org":          "o-brien",
		"a,b@t4gc.org":              "a-b",
		"ñandú@t4gc.org":            "ñandú",
		"bot-alice@t4gc.org":        "user-bot-alice",
		"link-share-5@t4gc.org":     "user-link-share-5",
		"--x--@t4gc.org":            "x",
		"...@t4gc.org":              "",
		"+tag@t4gc.org":             "",
		"no-at-sign":                "",
		"":                          "",
		"\"quoted local\"@t4gc.org": "quoted-local",
	}
	for email, want := range cases {
		t.Run(email, func(t *testing.T) {
			assert.Equal(t, want, usernameCandidateFromEmail(email))
		})
	}
}

func TestSanitizeUsernameCandidate(t *testing.T) {
	cases := map[string]string{
		"Abhiram N J":            "Abhiram-N-J",
		"  spaced  ":             "spaced",
		"tab\there":              "tab-here",
		"comma,name":             "comma-name",
		"日本":                     "日本",
		"bot-x":                  "user-bot-x",
		"":                       "",
		strings.Repeat("a", 300): strings.Repeat("a", maxUsernameBaseLength),
	}
	for in, want := range cases {
		t.Run(in, func(t *testing.T) {
			assert.Equal(t, want, sanitizeUsernameCandidate(in))
		})
	}
}

func TestGetProviderFromMapUsernameFromEmail(t *testing.T) {
	defer CleanupSavedOpenIDProviders()

	server := newMockOIDCServer()
	defer server.Close()

	base := func() map[string]interface{} {
		return map[string]interface{}{
			"name":         "Test Provider",
			"authurl":      server.URL,
			"clientid":     "client1",
			"clientsecret": "secret1",
		}
	}

	t.Run("defaults to false", func(t *testing.T) {
		provider, err := getProviderFromMap(base(), "test")
		require.NoError(t, err)
		assert.False(t, provider.UsernameFromEmail)
	})
	t.Run("native bool", func(t *testing.T) {
		pi := base()
		pi["usernamefromemail"] = true
		provider, err := getProviderFromMap(pi, "test")
		require.NoError(t, err)
		assert.True(t, provider.UsernameFromEmail)
	})
	t.Run("string", func(t *testing.T) {
		pi := base()
		pi["usernamefromemail"] = "true"
		provider, err := getProviderFromMap(pi, "test")
		require.NoError(t, err)
		assert.True(t, provider.UsernameFromEmail)
	})
	t.Run("from file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "usernamefromemail")
		require.NoError(t, os.WriteFile(path, []byte("true\n"), 0o600))
		key := "auth.openid.providers.filetest.usernamefromemail.file"
		viper.Set(key, path)
		defer viper.Set(key, "")

		provider, err := getProviderFromMap(base(), "filetest")
		require.NoError(t, err)
		assert.True(t, provider.UsernameFromEmail)
	})
}
