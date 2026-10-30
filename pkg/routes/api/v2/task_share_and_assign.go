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
package apiv2

import (
	"context"
	"net/http"

	"code.vikunja.io/api/pkg/models"
	"code.vikunja.io/api/pkg/web/handler"

	"github.com/danielgtaylor/huma/v2"
)

// RegisterTaskShareAndAssignRoutes registers POST /tasks/{task}/assignees/share (prime-tasks F2).
func RegisterTaskShareAndAssignRoutes(api huma.API) {
	Register(api, huma.Operation{
		OperationID: "task-assignees-share-and-assign",
		Summary:     "Share the project with a user and assign them",
		Description: "Assigns a user to the task. If they can't see the task's project yet, the project is first shared with them " +
			"with read & write permission; both happen in one transaction. Only admins of the task's project may call this; " +
			"anyone else gets 403 and nothing is shared. The response says whether the project was shared.",
		Method: http.MethodPost,
		Path:   "/tasks/{task}/assignees/share",
		Tags:   []string{"assignees"},
	}, taskShareAndAssign)
}

func init() { AddRouteRegistrar(RegisterTaskShareAndAssignRoutes) }

func taskShareAndAssign(ctx context.Context, in *struct {
	TaskID int64 `path:"task"`
	Body   models.TaskShareAndAssign
}) (*singleBody[models.TaskShareAndAssign], error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	in.Body.TaskID = in.TaskID // URL wins over body
	if err := handler.DoCreate(ctx, &in.Body, a); err != nil {
		return nil, translateDomainError(err)
	}
	return &singleBody[models.TaskShareAndAssign]{Body: &in.Body}, nil
}
