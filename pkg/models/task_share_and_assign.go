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
	"code.vikunja.io/api/pkg/user"
	"code.vikunja.io/api/pkg/web"

	"xorm.io/xorm"
)

// TaskShareAndAssign assigns a user to a task and, if they can't see the task's project yet,
// shares the project with them (read & write) first, both in the caller's transaction
// (prime-tasks F2). Only admins of the task's project may do this; everyone else gets a 403
// and nothing is shared.
type TaskShareAndAssign struct {
	TaskID int64 `xorm:"-" json:"-" param:"task"`
	UserID int64 `xorm:"-" json:"user_id" doc:"The user to assign."`
	Shared bool  `xorm:"-" json:"shared" readOnly:"true" doc:"True when this call shared the project with the user; false when they already had access."`

	web.CRUDable    `xorm:"-" json:"-"`
	web.Permissions `xorm:"-" json:"-"`
}

// CanCreate checks that the caller is an admin of the task's project.
func (sa *TaskShareAndAssign) CanCreate(s *xorm.Session, a web.Auth) (bool, error) {
	if _, isLinkShare := a.(*LinkSharing); isLinkShare {
		return false, nil
	}
	project, err := GetProjectSimpleByTaskID(s, sa.TaskID)
	if err != nil {
		return false, err
	}
	return project.IsAdmin(s, a)
}

// Create shares the project with the user when needed, then assigns them.
func (sa *TaskShareAndAssign) Create(s *xorm.Session, a web.Auth) error {
	project, err := GetProjectSimpleByTaskID(s, sa.TaskID)
	if err != nil {
		return err
	}
	assignee, err := user.GetUserByID(s, sa.UserID)
	if err != nil {
		return err
	}

	canRead, _, err := project.CanRead(s, assignee)
	if err != nil {
		return err
	}
	if !canRead {
		share := &ProjectUser{ProjectID: project.ID, Username: assignee.Username, Permission: PermissionWrite}
		if err := share.Create(s, a); err != nil {
			return err
		}
		sa.Shared = true
	}

	task := &Task{ID: sa.TaskID, ProjectID: project.ID}
	return task.addNewAssigneeByID(s, assignee.ID, project, a)
}
