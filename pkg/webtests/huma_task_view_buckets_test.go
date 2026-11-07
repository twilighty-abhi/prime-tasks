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

	"code.vikunja.io/api/pkg/db"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type viewBucketsItem struct {
	ID          int64 `json:"id"`
	ViewBuckets []struct {
		ProjectViewID int64  `json:"project_view_id"`
		BucketID      int64  `json:"bucket_id"`
		BucketTitle   string `json:"bucket_title"`
	} `json:"view_buckets"`
}

// Agents need to know which Kanban column a task is in; fixtures: task 1 is in bucket 1 ("testbucket1") of view 4.
func TestHumaTask_ViewBuckets(t *testing.T) {
	t.Run("single read lists the bucket per kanban view", func(t *testing.T) {
		e, err := setupTestEnv()
		require.NoError(t, err)
		rec := humaRequest(t, e, http.MethodGet, "/api/v2/tasks/1", "", humaTokenFor(t, &testuser1), "")
		require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
		var item viewBucketsItem
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &item))
		require.Len(t, item.ViewBuckets, 1)
		assert.Equal(t, int64(4), item.ViewBuckets[0].ProjectViewID)
		assert.Equal(t, int64(1), item.ViewBuckets[0].BucketID)
		assert.Equal(t, "testbucket1", item.ViewBuckets[0].BucketTitle)
	})

	t.Run("a view the caller cannot see is left out", func(t *testing.T) {
		e, err := setupTestEnv()
		require.NoError(t, err)
		// Task 1 is in project 1, which user 13 can read; add a placement in a view of a project they cannot.
		s := db.NewSession()
		mustExec(t, s, "INSERT INTO task_buckets (task_id, bucket_id, project_view_id) VALUES (1, 4, 8)")
		require.NoError(t, s.Commit())
		s.Close()

		rec := humaRequest(t, e, http.MethodGet, "/api/v2/tasks/1", "", humaTokenFor(t, &testuser1), "")
		require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
		var item viewBucketsItem
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &item))
		viewIDs := []int64{}
		for _, vb := range item.ViewBuckets {
			viewIDs = append(viewIDs, vb.ProjectViewID)
		}
		assert.Contains(t, viewIDs, int64(4))
		// Project 2 is owned by user 3; user 1 has no access to it.
		assert.NotContains(t, viewIDs, int64(8))
	})

	t.Run("my-work items carry their bucket", func(t *testing.T) {
		e, err := setupTestEnv()
		require.NoError(t, err)
		setupMyWork(t)
		rec := humaRequest(t, e, http.MethodGet, "/api/v2/my-work", "", myWorkToken(t, myWorkOwnerID, map[string][]string{"other": {"my_work"}}), "")
		require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
		var page struct {
			Items []viewBucketsItem `json:"items"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &page))
		require.NotEmpty(t, page.Items)
		for _, item := range page.Items {
			require.Len(t, item.ViewBuckets, 1, "task %d", item.ID)
			assert.NotZero(t, item.ViewBuckets[0].BucketID)
			assert.NotEmpty(t, item.ViewBuckets[0].BucketTitle)
		}
	})
}
