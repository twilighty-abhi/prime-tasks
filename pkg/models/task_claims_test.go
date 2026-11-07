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

package models

import (
	"net/http"
	"testing"
	"time"

	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/user"
	"code.vikunja.io/api/pkg/web"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/builder"
)

// Bots 23 and 25 belong to user 21, bot 24 to user 22. Project 1 (tasks 1-3, kanban view 4 with done bucket 3)
// is owned by user 1 and shared with nobody, so each test shares it as needed.
var (
	claimBotA     = &user.User{ID: 23}
	claimBotAName = "bot-owner-a-assistant"
	claimBotB     = &user.User{ID: 25}
	claimBotRead  = &user.User{ID: 24}
	claimOwner    = &user.User{ID: 1}
	claimStranger = &user.User{ID: 2}
)

func shareProjectForClaimTest(t *testing.T, projectID, userID int64, permission Permission) {
	t.Helper()
	s := db.NewSession()
	defer s.Close()
	_, err := s.Insert(&ProjectUser{
		UserID:     userID,
		ProjectID:  projectID,
		Permission: permission,
	})
	require.NoError(t, err)
	require.NoError(t, s.Commit())
}

func setupClaimTest(t *testing.T) {
	t.Helper()
	db.LoadAndAssertFixtures(t)
	shareProjectForClaimTest(t, 1, claimBotA.ID, PermissionWrite)
	shareProjectForClaimTest(t, 1, claimBotB.ID, PermissionWrite)
	shareProjectForClaimTest(t, 1, claimBotRead.ID, PermissionRead)
}

func claimTaskForTest(t *testing.T, taskID int64, a web.Auth) (*TaskClaim, error) {
	t.Helper()
	s := db.NewSession()
	defer s.Close()
	claim := &TaskClaim{TaskID: taskID}
	can, err := claim.CanCreate(s, a)
	if err != nil {
		return nil, err
	}
	if !can {
		return nil, ErrGenericForbidden{}
	}
	if err := claim.Create(s, a); err != nil {
		return nil, err
	}
	require.NoError(t, s.Commit())
	return claim, nil
}

func releaseTaskForTest(t *testing.T, release *TaskClaim, a web.Auth) error {
	t.Helper()
	s := db.NewSession()
	defer s.Close()
	can, err := release.CanDelete(s, a)
	if err != nil {
		return err
	}
	if !can {
		return ErrGenericForbidden{}
	}
	if err := release.Delete(s, a); err != nil {
		return err
	}
	require.NoError(t, s.Commit())
	return nil
}

func setTask1ClaimExpiryForTest(t *testing.T, expiresAt time.Time) {
	t.Helper()
	s := db.NewAutocommitSession()
	defer s.Close()
	_, err := s.Table("task_claims").Where("task_id = ?", 1).Update(map[string]any{"expires_at": expiresAt})
	require.NoError(t, err)
}

func agentLabelIDsOnTask(t *testing.T, taskID int64) []int64 {
	t.Helper()
	s := db.NewSession()
	defer s.Close()
	ids := []int64{}
	err := s.Table("label_tasks").
		Join("INNER", "labels", "labels.id = label_tasks.label_id").
		Where(builder.Eq{
			"label_tasks.task_id": taskID,
			"labels.title":        TaskClaimLabelTitle,
		}).
		Cols("label_tasks.label_id").
		Find(&ids)
	require.NoError(t, err)
	return ids
}

func countPickupComments(t *testing.T, taskID int64) int64 {
	t.Helper()
	s := db.NewSession()
	defer s.Close()
	count, err := s.Where(builder.And(
		builder.Eq{"task_id": taskID},
		builder.Like{"comment", "Picked up by"},
	)).Count(&TaskComment{})
	require.NoError(t, err)
	return count
}

func TestTaskClaim_Claim(t *testing.T) {
	t.Run("a bot with write access claims the task", func(t *testing.T) {
		setupClaimTest(t)
		before := time.Now()

		claim, err := claimTaskForTest(t, 1, claimBotA)
		require.NoError(t, err)

		assert.Equal(t, int64(23), claim.BotID)
		require.NotNil(t, claim.Bot)
		assert.Equal(t, claimBotAName, claim.Bot.Username)
		assert.WithinDuration(t, before.Add(TaskClaimLease), claim.ExpiresAt, time.Minute)
		assert.Equal(t, 2*time.Hour, TaskClaimLease)
		db.AssertExists(t, "task_claims", map[string]any{
			"task_id": 1,
			"bot_id":  23,
		}, false)
		assert.Len(t, agentLabelIDsOnTask(t, 1), 1)
		db.AssertExists(t, "task_comments", map[string]any{
			"task_id":   1,
			"author_id": 23,
			"comment":   "<p>Picked up by " + claimBotAName + "</p>",
		}, false)
	})

	t.Run("the holder claiming again renews the lease without a second comment", func(t *testing.T) {
		setupClaimTest(t)
		first, err := claimTaskForTest(t, 1, claimBotA)
		require.NoError(t, err)
		setTask1ClaimExpiryForTest(t, time.Now().Add(10*time.Minute))

		renewed, err := claimTaskForTest(t, 1, claimBotA)
		require.NoError(t, err)

		assert.WithinDuration(t, time.Now().Add(TaskClaimLease), renewed.ExpiresAt, time.Minute)
		assert.WithinDuration(t, first.ClaimedAt, renewed.ClaimedAt, time.Second)
		assert.Equal(t, int64(1), countPickupComments(t, 1))
		assert.Len(t, agentLabelIDsOnTask(t, 1), 1)
		db.AssertCount(t, "task_claims", builder.Eq{"task_id": 1}, 1)
	})

	t.Run("refused while another bot holds a live claim, naming the holder", func(t *testing.T) {
		setupClaimTest(t)
		_, err := claimTaskForTest(t, 1, claimBotA)
		require.NoError(t, err)

		_, err = claimTaskForTest(t, 1, claimBotB)
		var claimed ErrTaskClaimedByOtherBot
		require.ErrorAs(t, err, &claimed)
		httpErr := claimed.HTTPError()
		assert.Equal(t, http.StatusConflict, httpErr.HTTPCode)
		assert.Equal(t, ErrCodeTaskClaimedByOtherBot, httpErr.Code)
		assert.Contains(t, httpErr.Message, "task is claimed by "+claimBotAName+" until ")
		db.AssertExists(t, "task_claims", map[string]any{
			"task_id": 1,
			"bot_id":  23,
		}, false)
		assert.Equal(t, int64(1), countPickupComments(t, 1))
	})

	t.Run("an expired claim is ignored and another bot can claim", func(t *testing.T) {
		setupClaimTest(t)
		_, err := claimTaskForTest(t, 1, claimBotA)
		require.NoError(t, err)
		setTask1ClaimExpiryForTest(t, time.Now().Add(-time.Minute))

		claim, err := claimTaskForTest(t, 1, claimBotB)
		require.NoError(t, err)

		assert.Equal(t, int64(25), claim.BotID)
		db.AssertCount(t, "task_claims", builder.Eq{"task_id": 1}, 1)
		db.AssertExists(t, "task_claims", map[string]any{
			"task_id": 1,
			"bot_id":  25,
		}, false)
		assert.Len(t, agentLabelIDsOnTask(t, 1), 1)
	})

	t.Run("humans are refused with a clear 403", func(t *testing.T) {
		setupClaimTest(t)
		_, err := claimTaskForTest(t, 1, claimOwner)
		var notBot ErrOnlyBotsCanClaimTasks
		require.ErrorAs(t, err, &notBot)
		assert.Equal(t, http.StatusForbidden, notBot.HTTPError().HTTPCode)
		db.AssertMissing(t, "task_claims", map[string]any{"task_id": 1})
	})

	t.Run("a bot with read-only access is refused", func(t *testing.T) {
		setupClaimTest(t)
		_, err := claimTaskForTest(t, 1, claimBotRead)
		require.Error(t, err)
		assert.True(t, IsErrGenericForbidden(err), "got %v", err)
		db.AssertMissing(t, "task_claims", map[string]any{"task_id": 1})
	})

	t.Run("a bot without access is refused", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		_, err := claimTaskForTest(t, 1, claimBotA)
		require.Error(t, err)
		db.AssertMissing(t, "task_claims", map[string]any{"task_id": 1})
	})

	t.Run("reuses a label titled agent: working the bot can see", func(t *testing.T) {
		setupClaimTest(t)
		s := db.NewSession()
		existing := &Label{Title: TaskClaimLabelTitle}
		require.NoError(t, existing.Create(s, claimBotA))
		require.NoError(t, s.Commit())
		s.Close()

		_, err := claimTaskForTest(t, 1, claimBotA)
		require.NoError(t, err)

		assert.Equal(t, []int64{existing.ID}, agentLabelIDsOnTask(t, 1))
		db.AssertCount(t, "labels", builder.Eq{"title": TaskClaimLabelTitle}, 1)
	})

	t.Run("creates its own label when the only one is invisible to the bot", func(t *testing.T) {
		setupClaimTest(t)
		s := db.NewSession()
		hidden := &Label{Title: TaskClaimLabelTitle}
		require.NoError(t, hidden.Create(s, claimStranger))
		require.NoError(t, s.Commit())
		s.Close()

		_, err := claimTaskForTest(t, 1, claimBotA)
		require.NoError(t, err)

		ids := agentLabelIDsOnTask(t, 1)
		require.Len(t, ids, 1)
		assert.NotEqual(t, hidden.ID, ids[0])
		db.AssertExists(t, "labels", map[string]any{
			"id":            ids[0],
			"created_by_id": 23,
		}, false)
	})

	t.Run("refused with project-in-trash for a task in a trashed project", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		// Project 45 is in user 1's trash; task 53 lives in it.
		shareProjectForClaimTest(t, 45, claimBotA.ID, PermissionWrite)

		_, err := claimTaskForTest(t, 53, claimBotA)
		var inTrash ErrProjectIsInTrash
		require.ErrorAs(t, err, &inTrash)
		assert.Equal(t, int64(45), inTrash.ID)
		assert.Equal(t, ErrCodeProjectIsInTrash, inTrash.HTTPError().Code)
		assert.Equal(t, http.StatusNotFound, inTrash.HTTPError().HTTPCode)
		db.AssertMissing(t, "task_claims", map[string]any{"task_id": 53})
	})
}

func TestTaskClaim_Release(t *testing.T) {
	t.Run("the holder releases its claim", func(t *testing.T) {
		setupClaimTest(t)
		_, err := claimTaskForTest(t, 1, claimBotA)
		require.NoError(t, err)

		require.NoError(t, releaseTaskForTest(t, &TaskClaim{TaskID: 1}, claimBotA))

		db.AssertMissing(t, "task_claims", map[string]any{"task_id": 1})
		assert.Empty(t, agentLabelIDsOnTask(t, 1))
		db.AssertCount(t, "task_comments", builder.Eq{
			"task_id":   1,
			"author_id": 23,
		}, 1)
	})

	t.Run("posts the outcome comment and moves the task into the done bucket", func(t *testing.T) {
		setupClaimTest(t)
		_, err := claimTaskForTest(t, 1, claimBotA)
		require.NoError(t, err)

		require.NoError(t, releaseTaskForTest(t, &TaskClaim{
			TaskID:   1,
			Comment:  "<p>Opened PR #42</p>",
			BucketID: 3,
		}, claimBotA))

		db.AssertMissing(t, "task_claims", map[string]any{"task_id": 1})
		db.AssertExists(t, "task_comments", map[string]any{
			"task_id":   1,
			"author_id": 23,
			"comment":   "<p>Opened PR #42</p>",
		}, false)
		db.AssertExists(t, "task_buckets", map[string]any{
			"task_id":         1,
			"project_view_id": 4,
			"bucket_id":       3,
		}, false)
		// Bucket 3 is the view's done bucket, so the move marks the task done.
		db.AssertExists(t, "tasks", map[string]any{
			"id":   1,
			"done": true,
		}, false)
	})

	t.Run("a bucket of another project is refused and nothing changes", func(t *testing.T) {
		setupClaimTest(t)
		_, err := claimTaskForTest(t, 1, claimBotA)
		require.NoError(t, err)

		err = releaseTaskForTest(t, &TaskClaim{
			TaskID:   1,
			Comment:  "<p>moving</p>",
			BucketID: 4,
		}, claimBotA)
		require.Error(t, err)

		db.AssertExists(t, "task_claims", map[string]any{"task_id": 1}, false)
		db.AssertMissing(t, "task_comments", map[string]any{"comment": "<p>moving</p>"})
	})

	t.Run("a human with write access clears a stuck claim", func(t *testing.T) {
		setupClaimTest(t)
		_, err := claimTaskForTest(t, 1, claimBotA)
		require.NoError(t, err)

		require.NoError(t, releaseTaskForTest(t, &TaskClaim{TaskID: 1}, claimOwner))

		db.AssertMissing(t, "task_claims", map[string]any{"task_id": 1})
		assert.Empty(t, agentLabelIDsOnTask(t, 1))
	})

	t.Run("a human can clear an expired claim", func(t *testing.T) {
		setupClaimTest(t)
		_, err := claimTaskForTest(t, 1, claimBotA)
		require.NoError(t, err)
		setTask1ClaimExpiryForTest(t, time.Now().Add(-time.Minute))

		require.NoError(t, releaseTaskForTest(t, &TaskClaim{TaskID: 1}, claimOwner))
		db.AssertMissing(t, "task_claims", map[string]any{"task_id": 1})
	})

	t.Run("another bot is refused, naming the holder", func(t *testing.T) {
		setupClaimTest(t)
		_, err := claimTaskForTest(t, 1, claimBotA)
		require.NoError(t, err)

		err = releaseTaskForTest(t, &TaskClaim{TaskID: 1}, claimBotB)
		var claimed ErrTaskClaimedByOtherBot
		require.ErrorAs(t, err, &claimed)
		assert.Contains(t, claimed.HTTPError().Message, claimBotAName)
		db.AssertExists(t, "task_claims", map[string]any{"task_id": 1}, false)
	})

	t.Run("a human without write access is refused", func(t *testing.T) {
		setupClaimTest(t)
		_, err := claimTaskForTest(t, 1, claimBotA)
		require.NoError(t, err)

		err = releaseTaskForTest(t, &TaskClaim{TaskID: 1}, claimStranger)
		require.Error(t, err)
		db.AssertExists(t, "task_claims", map[string]any{"task_id": 1}, false)
	})

	t.Run("an unclaimed task cannot be released", func(t *testing.T) {
		setupClaimTest(t)
		err := releaseTaskForTest(t, &TaskClaim{TaskID: 1}, claimBotA)
		var notClaimed ErrTaskIsNotClaimed
		require.ErrorAs(t, err, &notClaimed)
		assert.Equal(t, http.StatusConflict, notClaimed.HTTPError().HTTPCode)
	})
}

func TestTaskClaim_ShownOnTaskReads(t *testing.T) {
	t.Run("a live claim is on the read and list", func(t *testing.T) {
		setupClaimTest(t)
		_, err := claimTaskForTest(t, 1, claimBotA)
		require.NoError(t, err)

		s := db.NewSession()
		defer s.Close()
		task := &Task{ID: 1}
		require.NoError(t, task.ReadOne(s, claimOwner))
		require.NotNil(t, task.Claim)
		assert.Equal(t, int64(23), task.Claim.BotID)
		require.NotNil(t, task.Claim.Bot)
		assert.Equal(t, claimBotAName, task.Claim.Bot.Username)
		assert.Empty(t, task.Claim.Bot.Email)
		assert.False(t, task.Claim.ExpiresAt.IsZero())

		tasks, _, _, err := (&TaskCollection{ProjectID: 1}).ReadAll(s, claimOwner, "", 1, 50)
		require.NoError(t, err)
		claims := map[int64]*TaskClaim{}
		for _, listed := range tasks.([]*Task) {
			claims[listed.ID] = listed.Claim
		}
		require.NotNil(t, claims[1])
		assert.Equal(t, int64(23), claims[1].BotID)
		assert.Nil(t, claims[2])
	})

	t.Run("an expired claim is not shown", func(t *testing.T) {
		setupClaimTest(t)
		_, err := claimTaskForTest(t, 1, claimBotA)
		require.NoError(t, err)
		setTask1ClaimExpiryForTest(t, time.Now().Add(-time.Minute))

		s := db.NewSession()
		defer s.Close()
		task := &Task{ID: 1}
		require.NoError(t, task.ReadOne(s, claimOwner))
		assert.Nil(t, task.Claim)
	})
}

func TestTaskClaim_CleanupExpired(t *testing.T) {
	setupClaimTest(t)
	_, err := claimTaskForTest(t, 1, claimBotA)
	require.NoError(t, err)
	_, err = claimTaskForTest(t, 3, claimBotA)
	require.NoError(t, err)
	setTask1ClaimExpiryForTest(t, time.Now().Add(-time.Minute))

	require.NoError(t, cleanupExpiredTaskClaims(time.Now()))

	db.AssertMissing(t, "task_claims", map[string]any{"task_id": 1})
	assert.Empty(t, agentLabelIDsOnTask(t, 1))
	db.AssertExists(t, "task_claims", map[string]any{"task_id": 3}, false)
	assert.Len(t, agentLabelIDsOnTask(t, 3), 1)
}
