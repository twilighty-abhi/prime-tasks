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

	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/models"

	"github.com/danielgtaylor/huma/v2"
)

// RegisterProjectLeaveRoutes registers DELETE /projects/{project}/shares/me (prime-tasks F4).
func RegisterProjectLeaveRoutes(api huma.API) {
	Register(api, huma.Operation{
		OperationID: "project-shares-leave",
		Summary:     "Leave a project",
		Description: "Removes the caller's own direct user share on the project. " +
			"If the caller's access comes from owning the project, a team or a parent project, " +
			"nothing is removed and a 412 names where the access comes from. Link shares can't leave.",
		Method: http.MethodDelete,
		Path:   "/projects/{project}/shares/me",
		Tags:   []string{"sharing"},
	}, projectSharesLeave)
}

func init() { AddRouteRegistrar(RegisterProjectLeaveRoutes) }

func projectSharesLeave(ctx context.Context, in *struct {
	ProjectID int64 `path:"project"`
}) (*emptyBody, error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	s := db.NewSession()
	defer s.Close()

	// LeaveProject only ever removes the caller's own share, after ProjectUser.CanDelete.
	if err := models.LeaveProject(s, a, in.ProjectID); err != nil {
		_ = s.Rollback()
		return nil, translateDomainError(err)
	}
	if err := s.Commit(); err != nil {
		return nil, translateDomainError(err)
	}
	return &emptyBody{}, nil
}
