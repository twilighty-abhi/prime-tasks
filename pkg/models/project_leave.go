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

// LeaveProject removes the caller's own direct share on a project (prime-tasks F4).
// When the caller's access comes from somewhere else (ownership, a team, a parent
// project) there is nothing they can remove themselves, so it returns
// ErrCannotLeaveProject naming those sources.
func LeaveProject(s *xorm.Session, a web.Auth, projectID int64) error {
	u, err := user.GetFromAuth(a)
	if err != nil {
		return err
	}

	hasDirectShare, err := s.
		Where(builder.Eq{"user_id": u.ID, "project_id": projectID}).
		Exist(&ProjectUser{})
	if err != nil {
		return err
	}
	if hasDirectShare {
		share := &ProjectUser{ProjectID: projectID, Username: u.Username}
		can, err := share.CanDelete(s, a)
		if err != nil {
			return err
		}
		if !can {
			return ErrGenericForbidden{}
		}
		return share.Delete(s, a)
	}

	grants, err := getProjectGrants(s, u.ID, []int64{projectID})
	if err != nil {
		return err
	}
	if len(grants[projectID]) == 0 {
		exists, err := s.Where(builder.Eq{"id": projectID}).Exist(&Project{})
		if err != nil {
			return err
		}
		if !exists {
			return ErrProjectDoesNotExist{ID: projectID}
		}
		return ErrGenericForbidden{}
	}

	return newErrCannotLeaveProject(s, projectID, grants[projectID])
}

func newErrCannotLeaveProject(s *xorm.Session, projectID int64, grants []*projectGrant) error {
	errLeave := &ErrCannotLeaveProject{ProjectID: projectID}

	teamIDs := []int64{}
	projectIDs := []int64{}
	for _, g := range grants {
		switch {
		case g.Kind == projectGrantTeam:
			teamIDs = append(teamIDs, g.TeamID)
		case g.Depth > 0:
			projectIDs = append(projectIDs, g.ViaProjectID)
		}
	}

	teams := map[int64]*Team{}
	if len(teamIDs) > 0 {
		if err := s.In("id", teamIDs).Find(&teams); err != nil {
			return err
		}
	}
	projects := map[int64]*Project{}
	if len(projectIDs) > 0 {
		if err := s.In("id", projectIDs).Find(&projects); err != nil {
			return err
		}
	}

	seenTeams := map[int64]bool{}
	seenProjects := map[int64]bool{}
	for _, g := range grants {
		switch {
		case g.Kind == projectGrantOwner && g.Depth == 0:
			errLeave.Owner = true
		case g.Kind == projectGrantTeam:
			if t, has := teams[g.TeamID]; has && !seenTeams[g.TeamID] {
				seenTeams[g.TeamID] = true
				errLeave.Teams = append(errLeave.Teams, t.Name)
			}
		default: // owner of, or user share on, an ancestor
			if p, has := projects[g.ViaProjectID]; has && !seenProjects[g.ViaProjectID] {
				seenProjects[g.ViaProjectID] = true
				errLeave.ParentProjects = append(errLeave.ParentProjects, p.Title)
			}
		}
	}
	return errLeave
}
