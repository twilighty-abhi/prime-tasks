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
	"errors"
	"fmt"
	"net/http"
	"strings"

	"code.vikunja.io/api/pkg/web"
)

// prime-tasks errors. Their codes start at x900 within upstream's ranges, so a
// new upstream error never takes the same code.

// ErrCannotLeaveProject is returned when a user tries to leave a project whose access
// doesn't come from a direct share of their own.
type ErrCannotLeaveProject struct {
	ProjectID      int64
	Owner          bool
	Teams          []string
	ParentProjects []string
}

// IsErrCannotLeaveProject checks if an error is ErrCannotLeaveProject.
func IsErrCannotLeaveProject(err error) bool {
	errCannotLeaveProject := &ErrCannotLeaveProject{}
	ok := errors.As(err, &errCannotLeaveProject)
	return ok
}

func (err *ErrCannotLeaveProject) Error() string {
	return fmt.Sprintf("Cannot leave project [ProjectID: %d, Owner: %t, Teams: %v, ParentProjects: %v]",
		err.ProjectID, err.Owner, err.Teams, err.ParentProjects)
}

// ErrCodeCannotLeaveProject holds the unique world-error code of this error
const ErrCodeCannotLeaveProject = 3901

// HTTPError holds the http error description
func (err *ErrCannotLeaveProject) HTTPError() web.HTTPError {
	return web.HTTPError{
		HTTPCode: http.StatusPreconditionFailed,
		Code:     ErrCodeCannotLeaveProject,
		Message:  err.message(),
	}
}

func (err *ErrCannotLeaveProject) message() string {
	if err.Owner {
		return "You own this project, so you can't leave it. Transfer or delete it instead."
	}

	sources := []string{}
	for _, t := range err.Teams {
		sources = append(sources, fmt.Sprintf("the team %q", t))
	}
	for _, p := range err.ParentProjects {
		sources = append(sources, fmt.Sprintf("the parent project %q", p))
	}
	return fmt.Sprintf("You can't leave this project: your access comes from %s. Ask an admin to remove you there.",
		joinWithAnd(sources))
}

func joinWithAnd(parts []string) string {
	switch len(parts) {
	case 0:
		return "somewhere else"
	case 1:
		return parts[0]
	default:
		return strings.Join(parts[:len(parts)-1], ", ") + " and " + parts[len(parts)-1]
	}
}
