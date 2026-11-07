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
	"fmt"
	"net/http"

	"code.vikunja.io/api/pkg/models"
	"code.vikunja.io/api/pkg/web/handler"

	"github.com/danielgtaylor/huma/v2"
)

type myWorkListBody struct {
	Body Paginated[*models.MyWorkTask]
}

// RegisterMyWorkRoutes registers GET /my-work (prime-tasks A4).
func RegisterMyWorkRoutes(api huma.API) {
	Register(api, huma.Operation{
		OperationID: "my-work",
		Summary:     "List my open tasks",
		Description: "Returns the open (not done) tasks assigned to the caller, across every project the caller can read. " +
			"For a bot user it returns the tasks assigned to the bot's owner instead, but only in projects the bot itself can read. " +
			"Tasks in trashed or archived projects are left out. Sorted by due date (tasks without one last), then priority " +
			"(highest first), then id. Each task carries its project (id and title), its labels and a url to open it in the " +
			"web interface. Any user or bot may call this for their own work; link shares get 403.",
		Method: http.MethodGet,
		Path:   "/my-work",
		Tags:   []string{"tasks"},
	}, myWorkList)
}

func init() { AddRouteRegistrar(RegisterMyWorkRoutes) }

func myWorkList(ctx context.Context, in *struct {
	PageParams
	Format string `query:"format" enum:"html,markdown" doc:"How rich-text fields are exchanged. See the API description."`
}) (*myWorkListBody, error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	result, _, total, err := handler.DoReadAll(ctx, &models.MyWork{}, a, "", in.Page, in.PerPage)
	if err != nil {
		return nil, translateDomainError(err)
	}
	items, ok := result.([]*models.MyWorkTask)
	if !ok {
		return nil, fmt.Errorf("myWork.ReadAll returned unexpected type %T (expected []*models.MyWorkTask)", result)
	}
	tasks := make([]*models.Task, 0, len(items))
	for _, item := range items {
		tasks = append(tasks, &item.Task)
	}
	convertTasksToMarkdown(ctx, tasks...)
	return &myWorkListBody{Body: NewPaginated(items, total, in.Page, in.PerPage)}, nil
}
