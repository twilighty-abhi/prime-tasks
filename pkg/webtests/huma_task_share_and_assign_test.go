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
	"strconv"
	"testing"

	"code.vikunja.io/api/pkg/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// F2 (prime-tasks): POST /api/v2/tasks/{task}/assignees/share.
func TestHumaTaskShareAndAssign(t *testing.T) {
	user3 := user.User{ID: 3, Username: "user3"}

	t.Run("project admin shares and assigns in one call", func(t *testing.T) {
		e, err := setupTestEnv()
		require.NoError(t, err)
		token := humaTokenFor(t, &testuser1)
		// Fixture: user1 owns project 1 (task 1); user3 has no access to it.
		rec := humaRequest(t, e, http.MethodGet, "/api/v2/projects/1", "", humaTokenFor(t, &user3), "")
		require.Equal(t, http.StatusForbidden, rec.Code, "user3 must start without access; body: %s", rec.Body.String())

		rec = humaRequest(t, e, http.MethodPost, "/api/v2/tasks/1/assignees/share", `{"user_id":3}`, token, "")
		require.Equal(t, http.StatusCreated, rec.Code, "body: %s", rec.Body.String())
		var body struct {
			Shared bool `json:"shared"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.True(t, body.Shared)

		rec = humaRequest(t, e, http.MethodGet, "/api/v2/projects/1", "", humaTokenFor(t, &user3), "")
		assert.Equal(t, http.StatusOK, rec.Code, "user3 can now see the project; body: %s", rec.Body.String())
		rec = humaRequest(t, e, http.MethodGet, "/api/v2/tasks/1/assignees", "", token, "")
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), `"username":"user3"`)
	})

	t.Run("read & write member gets 403 and nothing is shared", func(t *testing.T) {
		e, err := setupTestEnv()
		require.NoError(t, err)
		token := humaTokenFor(t, &testuser1)
		// Fixture: user1 has read & write on project 10.
		rec := humaRequest(t, e, http.MethodPost, "/api/v2/projects/10/tasks", `{"title":"needs a hand"}`, token, "")
		require.Equal(t, http.StatusCreated, rec.Code, "body: %s", rec.Body.String())
		var task struct {
			ID int64 `json:"id"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &task))

		rec = humaRequest(t, e, http.MethodPost, "/api/v2/tasks/"+strconv.FormatInt(task.ID, 10)+"/assignees/share", `{"user_id":3}`, token, "")
		assert.Equal(t, http.StatusForbidden, rec.Code, "body: %s", rec.Body.String())
		rec = humaRequest(t, e, http.MethodGet, "/api/v2/projects/10", "", humaTokenFor(t, &user3), "")
		assert.Equal(t, http.StatusForbidden, rec.Code, "user3 must still have no access")
	})
}
