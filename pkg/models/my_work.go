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

	"xorm.io/builder"
	"xorm.io/xorm"
)

// MyWorkProject is the part of a task's project a my-work entry shows.
type MyWorkProject struct {
	ID    int64  `json:"id" doc:"The project's id."`
	Title string `json:"title" doc:"The project's title."`
}

// MyWorkTask is one open task in a my-work list (prime-tasks A4): the task with its project and a link to it.
type MyWorkTask struct {
	Task
	Project *MyWorkProject `json:"project" readOnly:"true" doc:"The project the task is in."`
	URL     string         `json:"url" readOnly:"true" doc:"Link to the task in the web interface (service.publicurl + tasks/<id>)."`
}

// MyWork lists the open tasks assigned to the caller, or for a bot to its owner (prime-tasks A4).
type MyWork struct {
	web.CRUDable    `xorm:"-" json:"-"`
	web.Permissions `xorm:"-" json:"-"`
}

// myWorkOrder sorts by due date with no due date last, then priority (highest first), then id.
const myWorkOrder = "CASE WHEN tasks.due_date IS NULL THEN 1 ELSE 0 END ASC, tasks.due_date ASC, tasks.priority DESC, tasks.id ASC"

// ReadAll returns the open tasks assigned to the caller in projects the caller can read; for a bot, the tasks
// assigned to its owner, still limited to the projects the bot itself can read. Tasks in trashed or archived
// projects are left out. Link shares are refused.
func (mw *MyWork) ReadAll(s *xorm.Session, a web.Auth, _ string, page int, perPage int) (result interface{}, resultCount int, numberOfTotalItems int64, err error) {
	if _, is := a.(*LinkSharing); is {
		return nil, 0, 0, &user.ErrMustNotBeLinkShare{}
	}
	caller, err := user.GetUserByID(s, a.GetID())
	if err != nil {
		return nil, 0, 0, err
	}
	assigneeID := caller.ID
	if caller.IsBot() {
		assigneeID = caller.BotOwnerID
	}

	// Trashed projects are already left out of the readable set.
	readable, err := accessibleProjectIDsCond(s, caller, "id")
	if err != nil {
		return nil, 0, 0, err
	}
	// is_archived is materialized down the project tree, so a child of an archived project is archived too.
	cond := builder.And(
		builder.Eq{"tasks.done": false},
		builder.In("tasks.id", builder.Select("task_id").From("task_assignees").Where(builder.Eq{"user_id": assigneeID})),
		builder.In("tasks.project_id", builder.Select("id").From("projects").Where(builder.And(
			readable,
			builder.Eq{"is_archived": false},
			builder.IsNull{"deleted_at"},
		))),
	)

	numberOfTotalItems, err = s.Where(cond).Count(&Task{})
	if err != nil {
		return nil, 0, 0, err
	}

	tasks := []*Task{}
	query := s.Where(cond).OrderBy(myWorkOrder)
	if limit, start := getLimitFromPageIndex(page, perPage); limit > 0 {
		query = query.Limit(limit, start)
	}
	if err = query.Find(&tasks); err != nil {
		return nil, 0, 0, err
	}

	items := make([]*MyWorkTask, 0, len(tasks))
	if len(tasks) == 0 {
		return items, 0, numberOfTotalItems, nil
	}

	taskMap := make(map[int64]*Task, len(tasks))
	projectIDs := make([]int64, 0, len(tasks))
	for _, t := range tasks {
		taskMap[t.ID] = t
		projectIDs = append(projectIDs, t.ProjectID)
	}
	if err = addMoreInfoToTasks(s, taskMap, a, nil, nil); err != nil {
		return nil, 0, 0, err
	}
	if err = addViewBucketsToTasks(s, a, taskMap); err != nil {
		return nil, 0, 0, err
	}
	projects, err := GetProjectsMapByIDs(s, projectIDs)
	if err != nil {
		return nil, 0, 0, err
	}

	for _, t := range tasks {
		item := &MyWorkTask{Task: *t, URL: t.GetFrontendURL()}
		if p, has := projects[t.ProjectID]; has {
			item.Project = &MyWorkProject{ID: p.ID, Title: p.Title}
		}
		if item.Labels == nil {
			item.Labels = []*Label{}
		}
		items = append(items, item)
	}
	return items, len(items), numberOfTotalItems, nil
}
