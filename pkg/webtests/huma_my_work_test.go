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
	"time"

	"code.vikunja.io/api/pkg/config"
	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/models"
	"code.vikunja.io/api/pkg/modules/auth"
	"code.vikunja.io/api/pkg/user"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/xorm"
)

// Fixture: user 21 owns bot 23.
const (
	myWorkOwnerID = int64(21)
	myWorkBotID   = int64(23)
	myWorkBotName = "bot-owner-a-assistant"
)

type myWorkData struct {
	shared, private int64
	// Open tasks assigned to user 21, in the expected my-work order.
	inOrder []int64
	// The same, without the one in the project the bot cannot read.
	botOrder  []int64
	labelled  int64
	labelName string
}

func mustExec(t *testing.T, s *xorm.Session, query string, args ...any) {
	t.Helper()
	_, err := s.Exec(append([]any{query}, args...)...)
	require.NoError(t, err)
}

// setupMyWork builds user 21's work: projects shared with their bot or not, trashed, archived, and another user's
// project user 21 cannot read but still has an assignment in.
func setupMyWork(t *testing.T) myWorkData {
	t.Helper()
	s := db.NewSession()
	defer s.Close()
	owner := &user.User{ID: myWorkOwnerID}

	project := func(title string, a *user.User, shareWithBot bool) int64 {
		p := &models.Project{Title: title}
		require.NoError(t, models.CreateProject(s, p, a, false, false))
		if shareWithBot {
			require.NoError(t, (&models.ProjectUser{ProjectID: p.ID, Username: myWorkBotName, Permission: models.PermissionRead}).Create(s, a))
		}
		return p.ID
	}
	due := func(day int) time.Time { return time.Date(2030, 1, day, 12, 0, 0, 0, time.UTC) }
	task := func(projectID int64, a *user.User, title string, dueDate time.Time, priority int64, assignTo ...int64) int64 {
		tk := &models.Task{ProjectID: projectID, Title: title, DueDate: dueDate, Priority: priority}
		require.NoError(t, tk.Create(s, a))
		for _, uid := range assignTo {
			_, err := s.Insert(&models.TaskAssginee{TaskID: tk.ID, UserID: uid, Created: time.Now()})
			require.NoError(t, err)
		}
		return tk.ID
	}

	d := myWorkData{labelName: "my work label"}
	d.shared = project("my work: shared with the bot", owner, true)
	d.private = project("my work: not shared with the bot", owner, false)
	trashed := project("my work: trashed", owner, true)
	archived := project("my work: archived", owner, true)
	foreign := project("my work: someone else's", &user.User{ID: 22}, false)

	t1 := task(d.shared, owner, "due 10th, priority 1", due(10), 1, myWorkOwnerID)
	t2 := task(d.shared, owner, "due 5th, priority 1", due(5), 1, myWorkOwnerID)
	t3 := task(d.shared, owner, "no due date, priority 5", time.Time{}, 5, myWorkOwnerID)
	t4 := task(d.shared, owner, "due 10th, priority 3", due(10), 3, myWorkOwnerID)
	t5 := task(d.shared, owner, "no due date, priority 5 too", time.Time{}, 5, myWorkOwnerID)
	t6 := task(d.shared, owner, "no due date, priority 1", time.Time{}, 1, myWorkOwnerID)
	done := task(d.shared, owner, "done", due(1), 5, myWorkOwnerID)
	mustExec(t, s, "UPDATE tasks SET done = ? WHERE id = ?", true, done)
	task(d.shared, owner, "unassigned", due(1), 5)
	task(d.shared, owner, "assigned to the bot only", due(1), 5, myWorkBotID)
	private := task(d.private, owner, "due 7th, not shared with the bot", due(7), 1, myWorkOwnerID)
	task(trashed, owner, "in the trash", due(1), 5, myWorkOwnerID)
	task(archived, owner, "archived", due(1), 5, myWorkOwnerID)
	task(foreign, &user.User{ID: 22}, "no read access", due(1), 5, myWorkOwnerID)
	mustExec(t, s, "UPDATE projects SET deleted_at = ? WHERE id = ?", time.Now(), trashed)
	mustExec(t, s, "UPDATE projects SET is_archived = ? WHERE id = ?", true, archived)

	label := &models.Label{Title: d.labelName}
	require.NoError(t, label.Create(s, owner))
	require.NoError(t, (&models.LabelTask{TaskID: t2, LabelID: label.ID}).Create(s, owner))
	d.labelled = t2

	require.NoError(t, s.Commit())
	d.inOrder = []int64{t2, private, t4, t1, t3, t5, t6}
	d.botOrder = []int64{t2, t4, t1, t3, t5, t6}
	return d
}

func myWorkToken(t *testing.T, ownerID int64, permissions models.APIPermissions) string {
	t.Helper()
	s := db.NewSession()
	defer s.Close()
	token := &models.APIToken{
		Title:          "my work",
		APIPermissions: permissions,
		ExpiresAt:      time.Now().Add(time.Hour),
		OwnerID:        ownerID,
	}
	require.NoError(t, token.Create(s, &user.User{ID: myWorkOwnerID}))
	require.NoError(t, s.Commit())
	return token.Token
}

type myWorkItem struct {
	ID        int64  `json:"id"`
	Done      bool   `json:"done"`
	ProjectID int64  `json:"project_id"`
	URL       string `json:"url"`
	Project   *struct {
		ID    int64  `json:"id"`
		Title string `json:"title"`
	} `json:"project"`
	Labels []struct {
		Title string `json:"title"`
	} `json:"labels"`
}

type myWorkPage struct {
	Items      []myWorkItem `json:"items"`
	Total      int64        `json:"total"`
	Page       int          `json:"page"`
	PerPage    int          `json:"per_page"`
	TotalPages int64        `json:"total_pages"`
}

func getMyWork(t *testing.T, e *echo.Echo, token, query string) myWorkPage {
	t.Helper()
	rec := humaRequest(t, e, http.MethodGet, "/api/v2/my-work"+query, "", token, "")
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
	var page myWorkPage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &page))
	return page
}

func myWorkIDs(page myWorkPage) []int64 {
	ids := make([]int64, 0, len(page.Items))
	for _, item := range page.Items {
		ids = append(ids, item.ID)
	}
	return ids
}

// prime-tasks A4: GET /api/v2/my-work.
func TestHumaMyWork(t *testing.T) {
	myWorkPermissions := models.APIPermissions{"other": {"my_work"}}

	t.Run("user gets their open assigned tasks across readable projects, sorted", func(t *testing.T) {
		e, err := setupTestEnv()
		require.NoError(t, err)
		d := setupMyWork(t)
		page := getMyWork(t, e, myWorkToken(t, myWorkOwnerID, myWorkPermissions), "")
		assert.Equal(t, d.inOrder, myWorkIDs(page), "due date ascending with no due date last, then priority descending, then id")
		assert.Equal(t, int64(len(d.inOrder)), page.Total)
		for _, item := range page.Items {
			assert.False(t, item.Done)
			require.NotNil(t, item.Project, "task %d", item.ID)
			assert.Equal(t, item.ProjectID, item.Project.ID)
			assert.NotEmpty(t, item.Project.Title)
			assert.Equal(t, config.ServicePublicURL.GetString()+"tasks/"+strconv.FormatInt(item.ID, 10), item.URL)
			assert.NotNil(t, item.Labels, "labels are always present, empty or not")
		}
		require.Equal(t, d.labelled, page.Items[0].ID)
		require.Len(t, page.Items[0].Labels, 1)
		assert.Equal(t, d.labelName, page.Items[0].Labels[0].Title)
	})

	t.Run("a session works too", func(t *testing.T) {
		e, err := setupTestEnv()
		require.NoError(t, err)
		d := setupMyWork(t)
		page := getMyWork(t, e, humaTokenFor(t, &user.User{ID: myWorkOwnerID, Username: "user_bot_owner_a"}), "")
		assert.Equal(t, d.inOrder, myWorkIDs(page))
	})

	t.Run("bot gets its owner's tasks, only in projects the bot can read", func(t *testing.T) {
		e, err := setupTestEnv()
		require.NoError(t, err)
		d := setupMyWork(t)
		page := getMyWork(t, e, myWorkToken(t, myWorkBotID, myWorkPermissions), "")
		assert.Equal(t, d.botOrder, myWorkIDs(page))
		assert.Equal(t, int64(len(d.botOrder)), page.Total)
		for _, item := range page.Items {
			assert.Equal(t, d.shared, item.ProjectID)
		}
	})

	t.Run("pagination", func(t *testing.T) {
		e, err := setupTestEnv()
		require.NoError(t, err)
		d := setupMyWork(t)
		token := myWorkToken(t, myWorkOwnerID, myWorkPermissions)
		page := getMyWork(t, e, token, "?per_page=2&page=2")
		assert.Equal(t, d.inOrder[2:4], myWorkIDs(page))
		assert.Equal(t, int64(len(d.inOrder)), page.Total)
		assert.Equal(t, 2, page.Page)
		assert.Equal(t, 2, page.PerPage)
		assert.Equal(t, int64(4), page.TotalPages)
		last := getMyWork(t, e, token, "?per_page=2&page=4")
		assert.Equal(t, d.inOrder[6:], myWorkIDs(last))
	})

	t.Run("nothing assigned gives an empty list", func(t *testing.T) {
		e, err := setupTestEnv()
		require.NoError(t, err)
		// Fixture: user 22 has no assignments.
		page := getMyWork(t, e, humaTokenFor(t, &user.User{ID: 22, Username: "user_bot_owner_b"}), "")
		assert.Empty(t, page.Items)
		assert.Zero(t, page.Total)
	})

	t.Run("token without the my_work permission is refused", func(t *testing.T) {
		e, err := setupTestEnv()
		require.NoError(t, err)
		token := myWorkToken(t, myWorkOwnerID, models.APIPermissions{"tasks": {"read_all"}})
		rec := humaRequest(t, e, http.MethodGet, "/api/v2/my-work", "", token, "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code, "body: %s", rec.Body.String())
	})

	t.Run("link share is refused", func(t *testing.T) {
		e, err := setupTestEnv()
		require.NoError(t, err)
		share, err := auth.NewLinkShareJWTAuthtoken(&models.LinkSharing{ID: 1, Hash: "test", ProjectID: 1, Permission: models.PermissionRead, SharingType: models.SharingTypeWithoutPassword, SharedByID: 1})
		require.NoError(t, err)
		rec := humaRequest(t, e, http.MethodGet, "/api/v2/my-work", "", share, "")
		assert.Equal(t, http.StatusForbidden, rec.Code, "body: %s", rec.Body.String())
	})
}

// prime-tasks A4: my_work is a typed MCP tool a bot's token can call.
func TestMCP_Tools_MyWork(t *testing.T) {
	e, err := setupTestEnv()
	require.NoError(t, err)
	d := setupMyWork(t)
	token := myWorkToken(t, myWorkBotID, models.APIPermissions{"mcp": {"access"}, "other": {"my_work"}})
	c := newMCPClientOn(t, e, token)
	assert.Equal(t, map[string]bool{"my_work": true}, c.toolNames())
	assert.Contains(t, c.toolDescription("my_work"), "bot")

	var items []myWorkItem
	readAllItems(t, c.callTool("my_work", map[string]any{}), &items)
	ids := make([]int64, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
		assert.NotEmpty(t, item.URL)
		require.NotNil(t, item.Project)
	}
	assert.Equal(t, d.botOrder, ids)

	readAllItems(t, c.callTool("my_work", map[string]any{"page": 2, "per_page": 4}), &items)
	require.Len(t, items, 2)
	assert.Equal(t, []int64{items[0].ID, items[1].ID}, d.botOrder[4:])
}
