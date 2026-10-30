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
	"strings"

	"code.vikunja.io/api/pkg/user"
	"code.vikunja.io/api/pkg/web"

	"xorm.io/builder"
	"xorm.io/xorm"
)

// Values of ProjectAccessEntry.Source.
const (
	ProjectAccessSourceOwner  = "owner"
	ProjectAccessSourceDirect = "direct"
	ProjectAccessSourceParent = "parent"
)

// ProjectAccessTeam is the part of a team an access list shows.
type ProjectAccessTeam struct {
	ID   int64  `json:"id" doc:"The team's id."`
	Name string `json:"name" doc:"The team's name."`
}

// ProjectAccessEntry is one user or team with access to a project, and why (prime-tasks F5).
type ProjectAccessEntry struct {
	Kind            string             `json:"kind" enum:"user,team" doc:"Whether this entry is a user or a team."`
	Source          string             `json:"source" enum:"owner,direct,parent" doc:"owner: owns this project. direct: shared on this project. parent: owns, or is shared on, a parent project, so it is managed there."`
	Permission      Permission         `json:"permission" doc:"0 = read, 1 = read & write, 2 = admin."`
	User            *user.User         `json:"user,omitempty" doc:"For kind user. Email addresses are never included."`
	Team            *ProjectAccessTeam `json:"team,omitempty" doc:"For kind team."`
	ViaProjectID    int64              `json:"via_project_id,omitempty" doc:"For source parent: the ancestor project the access comes from."`
	ViaProjectTitle string             `json:"via_project_title,omitempty" doc:"The title of that ancestor project."`

	depth int64
}

// ProjectAccessList lists everyone with access to a project: its owner, its user and team
// shares, and what it inherits from its parent projects.
type ProjectAccessList struct {
	ProjectID int64 `json:"-" param:"project"`

	web.CRUDable    `xorm:"-" json:"-"`
	web.Permissions `xorm:"-" json:"-"`
}

// ReadAll returns every entry, unpaginated: the owner first, then this project's shares, then
// inherited ones nearest first. Anyone who can read the project may see it; link shares may not.
func (pal *ProjectAccessList) ReadAll(s *xorm.Session, a web.Auth, _ string, _ int, _ int) (result interface{}, resultCount int, numberOfTotalItems int64, err error) {
	if _, is := a.(*LinkSharing); is {
		return nil, 0, 0, &user.ErrMustNotBeLinkShare{}
	}
	canRead, _, err := (&Project{ID: pal.ProjectID}).CanRead(s, a)
	if err != nil {
		return nil, 0, 0, err
	}
	if !canRead {
		return nil, 0, 0, ErrNeedToHaveProjectReadAccess{UserID: a.GetID(), ProjectID: pal.ProjectID}
	}

	entries, err := projectAccessEntries(s, pal.ProjectID)
	if err != nil {
		return nil, 0, 0, err
	}
	return entries, len(entries), int64(len(entries)), nil
}

func projectAccessEntries(s *xorm.Session, projectID int64) ([]*ProjectAccessEntry, error) {
	chain := []*ProjectAncestor{}
	if err := s.Where(builder.Eq{"project_id": projectID}).Find(&chain); err != nil {
		return nil, err
	}
	depths := make(map[int64]int64, len(chain))
	chainIDs := make([]int64, 0, len(chain))
	for _, c := range chain {
		depths[c.AncestorID] = c.Depth
		chainIDs = append(chainIDs, c.AncestorID)
	}
	if len(chainIDs) == 0 {
		return []*ProjectAccessEntry{}, nil
	}

	projects := map[int64]*Project{}
	if err := s.In("id", chainIDs).Find(&projects); err != nil {
		return nil, err
	}
	userShares := []*ProjectUser{}
	if err := s.In("project_id", chainIDs).Find(&userShares); err != nil {
		return nil, err
	}
	teamShares := []*TeamProject{}
	if err := s.In("project_id", chainIDs).Find(&teamShares); err != nil {
		return nil, err
	}

	userIDs := []int64{}
	for _, p := range projects {
		userIDs = append(userIDs, p.OwnerID)
	}
	for _, us := range userShares {
		userIDs = append(userIDs, us.UserID)
	}
	users, err := user.GetUsersByIDs(s, userIDs)
	if err != nil {
		return nil, err
	}
	teamIDs := make([]int64, 0, len(teamShares))
	for _, ts := range teamShares {
		teamIDs = append(teamIDs, ts.TeamID)
	}
	teams := map[int64]*Team{}
	if len(teamIDs) > 0 {
		if err := s.In("id", teamIDs).Find(&teams); err != nil {
			return nil, err
		}
	}

	entry := func(viaProjectID int64, source string) *ProjectAccessEntry {
		e := &ProjectAccessEntry{Source: source, depth: depths[viaProjectID]}
		if e.depth > 0 {
			e.Source = ProjectAccessSourceParent
			e.ViaProjectID = viaProjectID
			if p, has := projects[viaProjectID]; has {
				e.ViaProjectTitle = p.Title
			}
		}
		return e
	}
	withoutEmail := func(u *user.User) *user.User {
		c := *u
		c.Email = ""
		return &c
	}

	entries := []*ProjectAccessEntry{}
	for id, p := range projects {
		owner, has := users[p.OwnerID]
		if !has {
			continue
		}
		e := entry(id, ProjectAccessSourceOwner)
		e.Kind = "user"
		e.Permission = PermissionAdmin
		e.User = withoutEmail(owner)
		entries = append(entries, e)
	}
	for _, us := range userShares {
		u, has := users[us.UserID]
		if !has {
			continue
		}
		e := entry(us.ProjectID, ProjectAccessSourceDirect)
		e.Kind = "user"
		e.Permission = us.Permission
		e.User = withoutEmail(u)
		entries = append(entries, e)
	}
	for _, ts := range teamShares {
		t, has := teams[ts.TeamID]
		if !has {
			continue
		}
		e := entry(ts.ProjectID, ProjectAccessSourceDirect)
		e.Kind = "team"
		e.Permission = ts.Permission
		e.Team = &ProjectAccessTeam{ID: t.ID, Name: t.Name}
		entries = append(entries, e)
	}

	rank := map[string]int{ProjectAccessSourceOwner: 0, ProjectAccessSourceDirect: 1, ProjectAccessSourceParent: 2}
	name := func(e *ProjectAccessEntry) string {
		if e.User != nil {
			return strings.ToLower(e.User.GetName())
		}
		return strings.ToLower(e.Team.Name)
	}
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if a.depth != b.depth {
			return a.depth < b.depth
		}
		if rank[a.Source] != rank[b.Source] {
			return rank[a.Source] < rank[b.Source]
		}
		if a.Kind != b.Kind {
			return a.Kind == "user"
		}
		return name(a) < name(b)
	})
	return entries, nil
}
