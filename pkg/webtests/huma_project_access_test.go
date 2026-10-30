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
	"testing"

	"code.vikunja.io/api/pkg/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// F5 (prime-tasks): GET /api/v2/projects/{project}/access.
func TestHumaProjectAccess(t *testing.T) {
	t.Run("owner gets the list without email addresses", func(t *testing.T) {
		e, err := setupTestEnv()
		require.NoError(t, err)
		// Fixture: user1 owns project 1.
		rec := humaRequest(t, e, http.MethodGet, "/api/v2/projects/1/access", "", humaTokenFor(t, &testuser1), "")
		require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
		var body struct {
			Items []struct {
				Kind   string `json:"kind"`
				Source string `json:"source"`
				User   *struct {
					Username string `json:"username"`
					Email    string `json:"email"`
				} `json:"user"`
			} `json:"items"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		require.NotEmpty(t, body.Items)
		assert.Equal(t, "user", body.Items[0].Kind)
		assert.Equal(t, "owner", body.Items[0].Source)
		require.NotNil(t, body.Items[0].User)
		assert.Equal(t, "user1", body.Items[0].User.Username)
		for _, item := range body.Items {
			if item.User != nil {
				assert.Empty(t, item.User.Email, "access lists never carry email addresses")
			}
		}
	})

	t.Run("no access gets 403", func(t *testing.T) {
		e, err := setupTestEnv()
		require.NoError(t, err)
		// Fixture: user3 has no access to project 1.
		rec := humaRequest(t, e, http.MethodGet, "/api/v2/projects/1/access", "", humaTokenFor(t, &user.User{ID: 3, Username: "user3"}), "")
		assert.Equal(t, http.StatusForbidden, rec.Code, "body: %s", rec.Body.String())
	})
}
