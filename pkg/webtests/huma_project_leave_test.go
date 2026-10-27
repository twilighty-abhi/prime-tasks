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

	"code.vikunja.io/api/pkg/models"
	"code.vikunja.io/api/pkg/modules/auth"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// F4 (prime-tasks): DELETE /api/v2/projects/{project}/shares/me.
func TestHumaProjectLeave(t *testing.T) {
	t.Run("direct share: leaves and loses access", func(t *testing.T) {
		e, err := setupTestEnv()
		require.NoError(t, err)
		token := humaTokenFor(t, &testuser1)
		// Fixture: user1 has a direct read share on project 9 and no other way in.
		rec := humaRequest(t, e, http.MethodDelete, "/api/v2/projects/9/shares/me", "", token, "")
		require.Equal(t, http.StatusNoContent, rec.Code, "body: %s", rec.Body.String())

		rec = humaRequest(t, e, http.MethodGet, "/api/v2/projects/9", "", token, "")
		assert.Equal(t, http.StatusForbidden, rec.Code, "body: %s", rec.Body.String())
	})
	t.Run("team access: 412 naming the team", func(t *testing.T) {
		e, err := setupTestEnv()
		require.NoError(t, err)
		token := humaTokenFor(t, &testuser1)
		// Fixture: user1 reaches project 6 only through testteam2_read_only_on_project6.
		rec := humaRequest(t, e, http.MethodDelete, "/api/v2/projects/6/shares/me", "", token, "")
		require.Equal(t, http.StatusPreconditionFailed, rec.Code, "body: %s", rec.Body.String())
		assert.Contains(t, rec.Body.String(), "testteam2_read_only_on_project6")
		assert.Contains(t, rec.Body.String(), "3901")
	})
	t.Run("owner: 412", func(t *testing.T) {
		e, err := setupTestEnv()
		require.NoError(t, err)
		token := humaTokenFor(t, &testuser1)
		rec := humaRequest(t, e, http.MethodDelete, "/api/v2/projects/1/shares/me", "", token, "")
		assert.Equal(t, http.StatusPreconditionFailed, rec.Code, "body: %s", rec.Body.String())
	})
	t.Run("no access: 403", func(t *testing.T) {
		e, err := setupTestEnv()
		require.NoError(t, err)
		token := humaTokenFor(t, &testuser1)
		rec := humaRequest(t, e, http.MethodDelete, "/api/v2/projects/2/shares/me", "", token, "")
		assert.Equal(t, http.StatusForbidden, rec.Code, "body: %s", rec.Body.String())
	})
	t.Run("link share: 403", func(t *testing.T) {
		e, err := setupTestEnv()
		require.NoError(t, err)
		share := &models.LinkSharing{ID: 1, Hash: "test", ProjectID: 1, Permission: models.PermissionRead}
		linkToken, err := auth.NewLinkShareJWTAuthtoken(share)
		require.NoError(t, err)
		rec := humaRequest(t, e, http.MethodDelete, "/api/v2/projects/1/shares/me", "", linkToken, "")
		assert.Equal(t, http.StatusForbidden, rec.Code, "body: %s", rec.Body.String())
	})
}

// F4: the existing DELETE by username now also lets a user remove themselves.
func TestHumaProjectUserDeleteSelf(t *testing.T) {
	e, err := setupTestEnv()
	require.NoError(t, err)
	token := humaTokenFor(t, &testuser1)

	rec := humaRequest(t, e, http.MethodDelete, "/api/v2/projects/3/users/user2", "", token, "")
	assert.Equal(t, http.StatusForbidden, rec.Code, "removing someone else stays admin-only; body: %s", rec.Body.String())

	rec = humaRequest(t, e, http.MethodDelete, "/api/v2/projects/9/users/user1", "", token, "")
	assert.Equal(t, http.StatusNoContent, rec.Code, "body: %s", rec.Body.String())
}
