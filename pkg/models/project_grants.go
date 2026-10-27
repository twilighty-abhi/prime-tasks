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

	"xorm.io/builder"
	"xorm.io/xorm"
)

// projectGrantKind says what gives a user access to a project (prime-tasks).
type projectGrantKind string

const (
	projectGrantOwner projectGrantKind = "owner"
	projectGrantUser  projectGrantKind = "user"
	projectGrantTeam  projectGrantKind = "team"
)

// projectGrant is one grant that reaches a project, either on the project itself
// (Depth 0) or on one of its ancestors.
type projectGrant struct {
	ProjectID    int64            `xorm:"project_id"`
	Kind         projectGrantKind `xorm:"kind"`
	ViaProjectID int64            `xorm:"via_project_id"`
	TeamID       int64            `xorm:"team_id"`
	Permission   int64            `xorm:"permission"`
	Depth        int64            `xorm:"depth"`
}

// Same three grant sources as projectAccessCTE, but kept apart so callers can tell
// where access comes from. Binds the user id three times.
const projectGrantsQuery = `
SELECT pa.project_id AS project_id, g.kind, g.via_project_id, g.team_id, g.permission, pa.depth
FROM (
    SELECT 'owner' AS kind, id AS via_project_id, 0 AS team_id, 2 AS permission FROM projects WHERE owner_id = ?
    UNION ALL
    SELECT 'user' AS kind, project_id AS via_project_id, 0 AS team_id, permission FROM users_projects WHERE user_id = ?
    UNION ALL
    SELECT 'team' AS kind, tp.project_id AS via_project_id, tp.team_id, tp.permission
    FROM team_projects tp
    INNER JOIN team_members tm ON tm.team_id = tp.team_id
    WHERE tm.user_id = ?
) g
INNER JOIN project_ancestors pa ON pa.ancestor_id = g.via_project_id
WHERE `

// getProjectGrants returns every grant through which userID reaches each of projectIDs,
// nearest first. A project the user can't reach has no entry.
func getProjectGrants(s *xorm.Session, userID int64, projectIDs []int64) (map[int64][]*projectGrant, error) {
	out := make(map[int64][]*projectGrant, len(projectIDs))
	if len(projectIDs) == 0 {
		return out, nil
	}

	inSQL, inArgs, err := builder.ToSQL(builder.In("pa.project_id", projectIDs))
	if err != nil {
		return nil, err
	}
	args := append([]any{userID, userID, userID}, inArgs...)

	rows := []*projectGrant{}
	if err := s.SQL(projectGrantsQuery+inSQL, args...).Find(&rows); err != nil {
		return nil, err
	}

	for _, r := range rows {
		r.Kind = projectGrantKind(strings.TrimSpace(string(r.Kind))) // CHAR padding on some databases
		out[r.ProjectID] = append(out[r.ProjectID], r)
	}
	for _, grants := range out {
		sort.SliceStable(grants, func(i, j int) bool {
			if grants[i].Depth != grants[j].Depth {
				return grants[i].Depth < grants[j].Depth
			}
			return grants[i].Permission > grants[j].Permission
		})
	}
	return out, nil
}
