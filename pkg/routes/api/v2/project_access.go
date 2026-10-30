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

type projectAccessListBody struct {
	Body Paginated[*models.ProjectAccessEntry]
}

// RegisterProjectAccessRoutes registers GET /projects/{project}/access (prime-tasks F5).
func RegisterProjectAccessRoutes(api huma.API) {
	Register(api, huma.Operation{
		OperationID: "project-access-list",
		Summary:     "List who has access to a project",
		Description: "Returns everyone with access to the project and why: the owner, the users and teams it is shared with, " +
			"and what it inherits from its parent projects (source parent, managed on that parent). Requires read access; " +
			"link shares get 403. Not paginated: every entry comes on the first page. Email addresses are never included.",
		Method: http.MethodGet,
		Path:   "/projects/{project}/access",
		Tags:   []string{"sharing"},
	}, projectAccessList)
}

func init() { AddRouteRegistrar(RegisterProjectAccessRoutes) }

func projectAccessList(ctx context.Context, in *struct {
	ProjectID int64 `path:"project"`
}) (*projectAccessListBody, error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	result, _, total, err := handler.DoReadAll(ctx, &models.ProjectAccessList{ProjectID: in.ProjectID}, a, "", 1, 0)
	if err != nil {
		return nil, translateDomainError(err)
	}
	items, ok := result.([]*models.ProjectAccessEntry)
	if !ok {
		return nil, fmt.Errorf("projectAccess.ReadAll returned unexpected type %T (expected []*models.ProjectAccessEntry)", result)
	}
	return &projectAccessListBody{Body: NewPaginated(items, total, 1, len(items))}, nil
}
