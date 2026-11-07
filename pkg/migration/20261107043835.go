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

package migration

import (
	"time"

	"src.techknowlogick.com/xormigrate"
	"xorm.io/xorm"
)

// Mirrors models.TaskClaim. The unique task_id is what keeps a task to one claim.
type taskClaims20261107043835 struct {
	ID        int64     `xorm:"bigint autoincr not null unique pk"`
	TaskID    int64     `xorm:"bigint not null unique"`
	BotID     int64     `xorm:"bigint not null INDEX"`
	LabelID   int64     `xorm:"bigint not null default 0"`
	ClaimedAt time.Time `xorm:"not null"`
	ExpiresAt time.Time `xorm:"not null INDEX"`
}

func (taskClaims20261107043835) TableName() string {
	return "task_claims"
}

func createTaskClaims20261107043835(tx *xorm.Engine) error {
	return tx.Sync(taskClaims20261107043835{}) //nolint:forbidigo // brand-new table, nothing to drop
}

func init() {
	migrations = append(migrations, &xormigrate.Migration{
		ID:          "20261107043835",
		Description: "Add task_claims table for bot task claims",
		Migrate:     createTaskClaims20261107043835,
		Rollback: func(tx *xorm.Engine) error {
			return tx.DropTables(taskClaims20261107043835{})
		},
	})
}
