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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// F3 (prime-tasks): v2 project responses carry shared_via and owner_display_name; v1 doesn't change.
func TestHumaProjectSharedVia(t *testing.T) {
	e, err := setupTestEnv()
	require.NoError(t, err)
	token := humaTokenFor(t, &testuser1)

	type sharedVia struct {
		Type     string `json:"type"`
		TeamName string `json:"team_name"`
	}
	type project struct {
		ID               int64      `json:"id"`
		SharedVia        *sharedVia `json:"shared_via"`
		OwnerDisplayName string     `json:"owner_display_name"`
	}

	t.Run("list", func(t *testing.T) {
		rec := humaRequest(t, e, http.MethodGet, "/api/v2/projects?per_page=100", "", token, "")
		require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
		var body struct {
			Items []project `json:"items"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		byID := map[int64]project{}
		for _, p := range body.Items {
			byID[p.ID] = p
		}
		require.Contains(t, byID, int64(1))
		assert.Nil(t, byID[1].SharedVia, "own project")
		assert.Equal(t, "user1", byID[1].OwnerDisplayName)
		require.Contains(t, byID, int64(6))
		require.NotNil(t, byID[6].SharedVia)
		assert.Equal(t, sharedVia{Type: "team", TeamName: "testteam2_read_only_on_project6"}, *byID[6].SharedVia)
		require.Contains(t, byID, int64(9))
		require.NotNil(t, byID[9].SharedVia)
		assert.Equal(t, "direct", byID[9].SharedVia.Type)
	})

	t.Run("read one", func(t *testing.T) {
		rec := humaRequest(t, e, http.MethodGet, "/api/v2/projects/6", "", token, "")
		require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
		var p project
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &p))
		require.NotNil(t, p.SharedVia)
		assert.Equal(t, "team", p.SharedVia.Type)
	})

	t.Run("v1 is unchanged", func(t *testing.T) {
		rec := humaRequest(t, e, http.MethodGet, "/api/v1/projects/6", "", token, "")
		require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
		assert.NotContains(t, rec.Body.String(), "shared_via")
		assert.NotContains(t, rec.Body.String(), "owner_display_name")
	})
}
