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
	"sort"

	"code.vikunja.io/api/pkg/user"
	"code.vikunja.io/api/pkg/web"

	"xorm.io/xorm"
)

// Values of ProjectSharedVia.Type.
const (
	ProjectSharedViaDirect = "direct"
	ProjectSharedViaTeam   = "team"
	ProjectSharedViaParent = "parent"
)

// ProjectSharedVia says how a project someone else owns reaches the caller (prime-tasks F3).
type ProjectSharedVia struct {
	Type               string     `json:"type" enum:"direct,team,parent" doc:"How the project is shared with you: direct (shared with you as a user), team (shared with one of your teams) or parent (you have access to a parent project)."`
	Permission         Permission `json:"permission" doc:"The permission this share gives you: 0 = read, 1 = read & write, 2 = admin."`
	TeamID             int64      `json:"team_id,omitempty" doc:"For team, and for parent when the parent is shared with a team: the team."`
	TeamName           string     `json:"team_name,omitempty" doc:"The name of that team."`
	ParentProjectID    int64      `json:"parent_project_id,omitempty" doc:"For parent: the ancestor project that is shared with you."`
	ParentProjectTitle string     `json:"parent_project_title,omitempty" doc:"The title of that ancestor project."`
}

// AddProjectSharedVia fills SharedVia and OwnerDisplayName for the caller. Projects in the caller's own
// tree (owned, or under a project they own) get no SharedVia. Pseudo projects and link shares are skipped.
func AddProjectSharedVia(s *xorm.Session, a web.Auth, projects []*Project) error {
	for _, p := range projects {
		if p.Owner != nil {
			p.OwnerDisplayName = p.Owner.GetName()
		}
	}

	if _, isLinkShare := a.(*LinkSharing); isLinkShare {
		return nil
	}
	u, err := user.GetFromAuth(a)
	if err != nil {
		return err
	}

	ids := make([]int64, 0, len(projects))
	for _, p := range projects {
		if p.ID > 0 {
			ids = append(ids, p.ID)
		}
	}
	grantsByProject, err := getProjectGrants(s, u.ID, ids)
	if err != nil {
		return err
	}

	chosen := make(map[int64]*projectGrant, len(ids))
	teamIDs := []int64{}
	parentIDs := []int64{}
	for id, grants := range grantsByProject {
		g := nearestShare(grants)
		if g == nil {
			continue
		}
		chosen[id] = g
		if g.Kind == projectGrantTeam {
			teamIDs = append(teamIDs, g.TeamID)
		}
		if g.Depth > 0 {
			parentIDs = append(parentIDs, g.ViaProjectID)
		}
	}

	teams := map[int64]*Team{}
	if len(teamIDs) > 0 {
		if err := s.In("id", teamIDs).Find(&teams); err != nil {
			return err
		}
	}
	parents := map[int64]*Project{}
	if len(parentIDs) > 0 {
		if err := s.In("id", parentIDs).Find(&parents); err != nil {
			return err
		}
	}

	for _, p := range projects {
		g, has := chosen[p.ID]
		if !has {
			p.SharedVia = nil
			continue
		}
		via := &ProjectSharedVia{Type: ProjectSharedViaDirect, Permission: Permission(g.Permission)}
		if g.Kind == projectGrantTeam {
			via.Type = ProjectSharedViaTeam
			via.TeamID = g.TeamID
			if t, ok := teams[g.TeamID]; ok {
				via.TeamName = t.Name
			}
		}
		if g.Depth > 0 {
			via.Type = ProjectSharedViaParent
			via.ParentProjectID = g.ViaProjectID
			if parent, ok := parents[g.ViaProjectID]; ok {
				via.ParentProjectTitle = parent.Title
			}
		}
		p.SharedVia = via
	}
	return nil
}

// nearestShare picks the share that explains the caller's access: the closest one in the tree,
// then the strongest, then a direct share over a team share. Nil when the caller owns the
// project or one of its ancestors: that's their own tree, not something shared with them.
func nearestShare(grants []*projectGrant) *projectGrant {
	shares := make([]*projectGrant, 0, len(grants))
	for _, g := range grants {
		if g.Kind == projectGrantOwner {
			return nil
		}
		shares = append(shares, g)
	}
	if len(shares) == 0 {
		return nil
	}
	sort.SliceStable(shares, func(i, j int) bool {
		a, b := shares[i], shares[j]
		if a.Depth != b.Depth {
			return a.Depth < b.Depth
		}
		if a.Permission != b.Permission {
			return a.Permission > b.Permission
		}
		return a.Kind == projectGrantUser && b.Kind != projectGrantUser
	})
	return shares[0]
}
