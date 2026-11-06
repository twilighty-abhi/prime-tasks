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
	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/events"
	"code.vikunja.io/api/pkg/models"
	"code.vikunja.io/api/pkg/user"

	"github.com/stretchr/testify/require"
	"xorm.io/builder"
)

// prime-tasks F6: registration joins the auto-join teams exactly once. It dispatches one
// user.created event and doesn't join anything itself, so only the listener adds the user, and
// a repeated delivery of the same event adds nothing more.
func TestHumaRegisterAutoJoinsTeamsOnce(t *testing.T) {
	config.ServiceAutoJoinTeams.Set([]interface{}{map[string]interface{}{"domain": "example.org", "team_id": 1}})
	t.Cleanup(func() { config.ServiceAutoJoinTeams.Set(nil) })

	e, err := setupTestEnv()
	require.NoError(t, err)
	events.ClearDispatchedEvents()

	rec := humaRequest(t, e, http.MethodPost, "/api/v2/register",
		`{"username":"autojoiner","password":"12345678","email":"autojoiner@example.org"}`, "", "")
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	created := events.GetDispatchedEvents((&user.CreatedEvent{}).Name())
	require.Len(t, created, 1, "registration dispatches user.created once")
	u := created[0].(*user.CreatedEvent).User
	db.AssertCount(t, "team_members", builder.Eq{"user_id": u.ID}, 0)

	events.TestListener(t, created[0], &models.AutoJoinTeamsByEmailDomain{})
	db.AssertCount(t, "team_members", builder.Eq{"team_id": 1, "user_id": u.ID}, 1)

	events.TestListener(t, created[0], &models.AutoJoinTeamsByEmailDomain{})
	db.AssertCount(t, "team_members", builder.Eq{"team_id": 1, "user_id": u.ID}, 1)
}
