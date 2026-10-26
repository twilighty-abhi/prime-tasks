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
	"net/http"
	"testing"

	"code.vikunja.io/api/pkg/config"
	"code.vikunja.io/api/pkg/models"
	"code.vikunja.io/api/pkg/modules/auth"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// F1 (prime-tasks): the global user search in org directory mode.
func TestHumaUserSearchOrgDirectory(t *testing.T) {
	e, err := setupTestEnv()
	require.NoError(t, err)
	token := humaTokenFor(t, &testuser1)

	t.Run("flag off: partial username finds nobody", func(t *testing.T) {
		config.ServiceOrgDirectory.Set(false)
		rec := humaRequest(t, e, http.MethodGet, "/api/v2/users?q=ser6", "", token, "")
		require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
		usernames, _ := usersFromSearch(t, rec.Body.Bytes())
		assert.Empty(t, usernames)
	})
	t.Run("flag on: partial username finds the user, without email", func(t *testing.T) {
		config.ServiceOrgDirectory.Set(true)
		t.Cleanup(func() { config.ServiceOrgDirectory.Set(false) })
		rec := humaRequest(t, e, http.MethodGet, "/api/v2/users?q=ser6", "", token, "")
		require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
		usernames, emails := usersFromSearch(t, rec.Body.Bytes())
		assert.Equal(t, []string{"user6"}, usernames)
		for _, em := range emails {
			assert.Empty(t, em, "user search must never return email addresses")
		}
	})
	t.Run("flag on: link shares are still rejected", func(t *testing.T) {
		config.ServiceOrgDirectory.Set(true)
		t.Cleanup(func() { config.ServiceOrgDirectory.Set(false) })
		share := &models.LinkSharing{
			ID:         1,
			Hash:       "test",
			ProjectID:  1,
			Permission: models.PermissionRead,
		}
		linkToken, err := auth.NewLinkShareJWTAuthtoken(share)
		require.NoError(t, err)
		rec := humaRequest(t, e, http.MethodGet, "/api/v2/users?q=ser6", "", linkToken, "")
		assert.Equal(t, http.StatusForbidden, rec.Code, "body: %s", rec.Body.String())
	})
}
