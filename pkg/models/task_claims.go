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
	"fmt"
	"html"
	"strings"
	"time"

	"code.vikunja.io/api/pkg/cron"
	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/log"
	"code.vikunja.io/api/pkg/user"
	"code.vikunja.io/api/pkg/web"

	"xorm.io/builder"
	"xorm.io/xorm"
)

// TaskClaimLease is how long a claim lasts before another bot may take the task; claiming again renews it.
const TaskClaimLease = 2 * time.Hour

// TaskClaimLabelTitle is the label put on a task while a bot holds a claim on it.
const TaskClaimLabelTitle = "agent: working"

// TaskClaim is a bot's lease on a task: while it is live, no other bot may claim the task.
type TaskClaim struct {
	ID        int64      `xorm:"bigint autoincr not null unique pk" json:"-"`
	TaskID    int64      `xorm:"bigint not null unique" json:"task_id" readOnly:"true" doc:"The claimed task."`
	BotID     int64      `xorm:"bigint not null INDEX" json:"bot_id" readOnly:"true" doc:"The id of the bot user holding the claim."`
	LabelID   int64      `xorm:"bigint not null default 0" json:"-"`
	ClaimedAt time.Time  `xorm:"not null" json:"claimed_at" readOnly:"true" doc:"When the bot claimed the task. Renewing keeps this value."`
	ExpiresAt time.Time  `xorm:"not null INDEX" json:"expires_at" readOnly:"true" doc:"When the claim lapses unless the bot claims the task again. After this, another bot may claim it."`
	Bot       *user.User `xorm:"-" json:"bot,omitempty" readOnly:"true" doc:"The bot user holding the claim."`

	// Release inputs, set by the route from the request.
	Comment  string `xorm:"-" json:"-"`
	BucketID int64  `xorm:"-" json:"-"`

	web.CRUDable    `xorm:"-" json:"-"`
	web.Permissions `xorm:"-" json:"-"`
}

// TableName returns the table name for task claims.
func (*TaskClaim) TableName() string {
	return "task_claims"
}

func (c *TaskClaim) isLive(now time.Time) bool {
	return c.ExpiresAt.After(now)
}

func getTaskClaim(s *xorm.Session, taskID int64) (*TaskClaim, error) {
	claim := &TaskClaim{}
	has, err := s.Where("task_id = ?", taskID).Get(claim)
	if err != nil || !has {
		return nil, err
	}
	return claim, nil
}

// Write access to the task, with upstream's project-in-trash answer for tasks in a trashed project.
func canWriteClaimedTask(s *xorm.Session, a web.Auth, taskID int64) (bool, error) {
	task, err := getTaskByIDForAuth(s, a, taskID)
	if err != nil {
		return false, err
	}
	can, err := (&Project{ID: task.ProjectID}).CanWrite(s, a)
	if IsErrProjectDoesNotExist(err) {
		return false, trashedProjectReadError(s, a, task.ProjectID, err)
	}
	return can, err
}

func claimCaller(s *xorm.Session, a web.Auth) (*user.User, error) {
	if _, isShare := a.(*LinkSharing); isShare {
		return nil, nil
	}
	return user.GetUserByID(s, a.GetID())
}

// CanCreate checks that the caller is a bot with write access to the task.
func (c *TaskClaim) CanCreate(s *xorm.Session, a web.Auth) (bool, error) {
	can, err := canWriteClaimedTask(s, a, c.TaskID)
	if err != nil || !can {
		return false, err
	}
	caller, err := claimCaller(s, a)
	if err != nil {
		return false, err
	}
	if caller == nil || !caller.IsBot() {
		return false, ErrOnlyBotsCanClaimTasks{TaskID: c.TaskID}
	}
	return true, nil
}

// Create claims the task for the calling bot, or renews the bot's own live claim.
// A new claim adds the "agent: working" label and a pickup comment; a renewal only extends the lease.
func (c *TaskClaim) Create(s *xorm.Session, a web.Auth) error {
	if _, err := lockTaskForWrite(s, a, c.TaskID); err != nil {
		return err
	}
	now := time.Now()
	existing, err := getTaskClaim(s, c.TaskID)
	if err != nil {
		return err
	}

	if existing != nil && existing.isLive(now) && existing.BotID != a.GetID() {
		return claimedByOtherBotError(s, existing)
	}

	if existing != nil && existing.isLive(now) {
		existing.ExpiresAt = now.Add(TaskClaimLease)
		if _, err := s.ID(existing.ID).Cols("expires_at").Update(existing); err != nil {
			return err
		}
		*c = *existing
		if err := updateTaskLastUpdated(s, &Task{ID: c.TaskID}); err != nil {
			return err
		}
		return c.loadBot(s)
	}

	if existing != nil {
		if err := removeTaskClaim(s, a, existing); err != nil {
			return err
		}
	}

	labelID, err := ensureClaimLabelOnTask(s, a, c.TaskID)
	if err != nil {
		return err
	}

	c.ID = 0
	c.BotID = a.GetID()
	c.LabelID = labelID
	c.ClaimedAt = now
	c.ExpiresAt = now.Add(TaskClaimLease)
	if _, err := s.Insert(c); err != nil {
		return err
	}
	if err := c.loadBot(s); err != nil {
		return err
	}

	comment := &TaskComment{
		TaskID:  c.TaskID,
		Comment: "<p>Picked up by " + html.EscapeString(botDisplayUsername(c.Bot)) + "</p>",
	}
	return comment.Create(s, a)
}

// Bot usernames start with "bot-" already; older ones may not.
func botDisplayUsername(bot *user.User) string {
	if strings.HasPrefix(bot.Username, "bot-") {
		return bot.Username
	}
	return "bot-" + bot.Username
}

func (c *TaskClaim) loadBot(s *xorm.Session) error {
	users, err := getUsersOrLinkSharesFromIDs(s, []int64{c.BotID})
	if err != nil {
		return err
	}
	c.Bot = users[c.BotID]
	if c.Bot == nil {
		return user.ErrUserDoesNotExist{UserID: c.BotID}
	}
	return nil
}

func claimedByOtherBotError(s *xorm.Session, claim *TaskClaim) error {
	err := ErrTaskClaimedByOtherBot{
		TaskID:    claim.TaskID,
		BotID:     claim.BotID,
		ExpiresAt: claim.ExpiresAt,
	}
	if loadErr := claim.loadBot(s); loadErr != nil && !user.IsErrUserDoesNotExist(loadErr) {
		return loadErr
	}
	err.BotUsername = fmt.Sprintf("bot #%d", claim.BotID)
	if claim.Bot != nil {
		err.BotUsername = botDisplayUsername(claim.Bot)
	}
	return err
}

// Reuses a label with the exact title the caller can see, else creates one as the caller.
func ensureClaimLabelOnTask(s *xorm.Session, a web.Auth, taskID int64) (int64, error) {
	visible, err := labelVisibleCond(s, a)
	if err != nil {
		return 0, err
	}
	label := &Label{}
	has, err := s.Table("labels").
		Select("labels.*").
		Where(builder.And(visible, builder.Eq{"labels.title": TaskClaimLabelTitle})).
		OrderBy("labels.id ASC").
		Get(label)
	if err != nil {
		return 0, err
	}
	if !has {
		label = &Label{Title: TaskClaimLabelTitle}
		if err := label.Create(s, a); err != nil {
			return 0, err
		}
	}

	onTask, err := s.Exist(&LabelTask{
		LabelID: label.ID,
		TaskID:  taskID,
	})
	if err != nil {
		return 0, err
	}
	if onTask {
		return label.ID, nil
	}
	lt := &LabelTask{
		LabelID: label.ID,
		TaskID:  taskID,
	}
	return label.ID, lt.Create(s, a)
}

func removeTaskClaim(s *xorm.Session, a web.Auth, claim *TaskClaim) error {
	if _, err := s.ID(claim.ID).Delete(&TaskClaim{}); err != nil {
		return err
	}
	if claim.LabelID == 0 {
		return nil
	}
	onTask, err := s.Exist(&LabelTask{
		LabelID: claim.LabelID,
		TaskID:  claim.TaskID,
	})
	if err != nil || !onTask {
		return err
	}
	return (&LabelTask{
		LabelID: claim.LabelID,
		TaskID:  claim.TaskID,
	}).Delete(s, a)
}

// CanDelete lets the holding bot, or a person with write access, release the claim.
func (c *TaskClaim) CanDelete(s *xorm.Session, a web.Auth) (bool, error) {
	can, err := canWriteClaimedTask(s, a, c.TaskID)
	if err != nil || !can {
		return false, err
	}
	caller, err := claimCaller(s, a)
	if err != nil {
		return false, err
	}
	if caller == nil {
		return false, nil
	}
	existing, err := getTaskClaim(s, c.TaskID)
	if err != nil {
		return false, err
	}
	if existing == nil {
		return false, ErrTaskIsNotClaimed{TaskID: c.TaskID}
	}
	if caller.IsBot() && existing.BotID != caller.ID {
		if !existing.isLive(time.Now()) {
			return false, ErrTaskIsNotClaimed{TaskID: c.TaskID}
		}
		return false, claimedByOtherBotError(s, existing)
	}

	comment, bucketID := c.Comment, c.BucketID
	*c = *existing
	c.Comment, c.BucketID = comment, bucketID
	if err := c.loadBot(s); err != nil && !user.IsErrUserDoesNotExist(err) {
		return false, err
	}
	return true, nil
}

// Delete releases the claim: it removes the claim and its label, posts the optional outcome comment
// and moves the task into the optional bucket through the regular kanban move.
func (c *TaskClaim) Delete(s *xorm.Session, a web.Auth) error {
	if err := removeTaskClaim(s, a, c); err != nil {
		return err
	}
	if c.BucketID != 0 {
		if err := moveReleasedTaskToBucket(s, a, c.TaskID, c.BucketID); err != nil {
			return err
		}
	}
	if strings.TrimSpace(c.Comment) == "" {
		return nil
	}
	return (&TaskComment{
		TaskID:  c.TaskID,
		Comment: c.Comment,
	}).Create(s, a)
}

func moveReleasedTaskToBucket(s *xorm.Session, a web.Auth, taskID, bucketID int64) error {
	bucket, err := getBucketByID(s, bucketID)
	if err != nil {
		return err
	}
	view, err := GetProjectViewByID(s, bucket.ProjectViewID)
	if err != nil {
		return err
	}
	tb := &TaskBucket{
		BucketID:      bucket.ID,
		TaskID:        taskID,
		ProjectViewID: view.ID,
		ProjectID:     view.ProjectID,
	}
	can, err := tb.CanUpdate(s, a)
	if err != nil {
		return err
	}
	if !can {
		return ErrGenericForbidden{}
	}
	return tb.Update(s, a)
}

// Only live claims are shown; expired ones linger until the cleanup cron removes them.
func addClaimsToTasks(s *xorm.Session, taskIDs []int64, taskMap map[int64]*Task) error {
	claims := []*TaskClaim{}
	err := s.
		Where(builder.And(
			builder.In("task_id", taskIDs),
			builder.Gt{"expires_at": time.Now()},
		)).
		Find(&claims)
	if err != nil || len(claims) == 0 {
		return err
	}
	botIDs := make([]int64, 0, len(claims))
	for _, claim := range claims {
		botIDs = append(botIDs, claim.BotID)
	}
	bots, err := getUsersOrLinkSharesFromIDs(s, botIDs)
	if err != nil {
		return err
	}
	for _, claim := range claims {
		claim.Bot = bots[claim.BotID]
		if task, has := taskMap[claim.TaskID]; has {
			task.Claim = claim
		}
	}
	return nil
}

// RegisterTaskClaimCleanupCron removes expired claims and their "agent: working" label.
func RegisterTaskClaimCleanupCron() {
	err := cron.Schedule("*/10 * * * *", func() {
		if err := cleanupExpiredTaskClaims(time.Now()); err != nil {
			log.Errorf("[Task Claim Cleanup Cron] Could not clean up expired claims: %s", err)
		}
	})
	if err != nil {
		log.Fatalf("Could not register task claim cleanup cron: %s", err)
	}
}

func cleanupExpiredTaskClaims(now time.Time) error {
	s := db.NewSession()
	defer s.Close()

	expired := []*TaskClaim{}
	if err := s.Where(builder.Lte{"expires_at": now}).Find(&expired); err != nil {
		_ = s.Rollback()
		return err
	}
	for _, claim := range expired {
		// No doer here, so the label goes without the per-user permission checks of LabelTask.Delete.
		if _, err := s.Where(builder.Eq{
			"task_id":  claim.TaskID,
			"label_id": claim.LabelID,
		}).Delete(&LabelTask{}); err != nil {
			_ = s.Rollback()
			return err
		}
		if _, err := s.ID(claim.ID).Delete(&TaskClaim{}); err != nil {
			_ = s.Rollback()
			return err
		}
		if err := updateTaskLastUpdated(s, &Task{ID: claim.TaskID}); err != nil {
			_ = s.Rollback()
			return err
		}
	}
	if len(expired) > 0 {
		log.Debugf("[Task Claim Cleanup Cron] Removed %d expired claims", len(expired))
	}
	return s.Commit()
}
