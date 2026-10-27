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

package user

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"code.vikunja.io/api/pkg/config"

	"code.vikunja.io/api/pkg/db"

	"xorm.io/builder"
	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

// OrgDirectorySearchLimit caps the users one org directory search returns (service.orgdirectory).
const OrgDirectorySearchLimit = 20

// orgDirectoryMinQueryLength keeps a one-letter query from listing most of the organisation.
const orgDirectoryMinQueryLength = 2

type ProjectUserOpts struct {
	AdditionalCond              builder.Cond
	ReturnAllIfNoSearchProvided bool
	MatchFuzzily                bool
}

// SearchUsers performs the global user search shared by both API versions:
// it lists users matching the search string and obfuscates their email
// addresses before returning.
func SearchUsers(s *xorm.Session, search string, currentUser *User) (users []*User, err error) {
	users, err = ListUsers(s, search, currentUser, nil)
	if err != nil {
		return nil, err
	}
	for i := range users {
		users[i].Email = ""
	}
	return users, nil
}

// ListUsers returns a list with all users, filtered by an optional search string
func ListUsers(s *xorm.Session, search string, currentUser *User, opts *ProjectUserOpts) (users []*User, err error) {
	if opts == nil {
		opts = &ProjectUserOpts{}
	}

	// Prevent searching for placeholders
	search = strings.ReplaceAll(search, "%", "")

	if (search == "" || strings.ReplaceAll(search, " ", "") == "") && !opts.ReturnAllIfNoSearchProvided {
		return
	}

	orgDirectory, err := orgDirectoryApplies(s, currentUser, opts)
	if err != nil {
		return nil, err
	}

	conds := []builder.Cond{}

	// Subquery: find user IDs that share an external team with the current user
	externalTeamMemberIDs := builder.Select("tm2.user_id").
		From("team_members tm1").
		Join("INNER", "team_members tm2", "tm1.team_id = tm2.team_id").
		Join("INNER", "teams t", "t.id = tm1.team_id").
		Where(builder.And(
			builder.Eq{"tm1.user_id": currentUser.ID},
			builder.Neq{"t.external_id": ""},
			builder.Neq{"tm2.user_id": currentUser.ID},
		))

	queryParts := strings.Split(search, ",")

	if search != "" {
		for _, queryPart := range queryParts {

			if opts.MatchFuzzily {
				conds = append(conds,
					db.ILIKE("name", queryPart),
					db.ILIKE("username", queryPart),
					db.ILIKE("email", queryPart),
				)
				continue
			}

			if orgDirectory && utf8.RuneCountInString(strings.TrimSpace(queryPart)) >= orgDirectoryMinQueryLength {
				conds = append(conds,
					db.ILIKE("name", queryPart),
					db.ILIKE("username", queryPart),
					db.ILIKE("email", queryPart),
				)
			}

			var usernameCond builder.Cond = builder.Eq{"username": queryPart}
			if db.Type() == schemas.POSTGRES {
				usernameCond = builder.Expr("username ILIKE ?", queryPart)
			}
			if db.Type() == schemas.SQLITE {
				usernameCond = builder.Expr("username = ? COLLATE NOCASE", queryPart)
			}

			conds = append(conds,
				usernameCond,
				builder.And(
					db.ILIKE("name", queryPart),
					builder.Eq{"discoverable_by_name": true},
				),
				// External team bypass: match by name or email without discoverability check
				builder.And(
					builder.In("id", externalTeamMemberIDs),
					builder.Or(
						db.ILIKE("name", queryPart),
						builder.Eq{"email": queryPart},
					),
				),
			)
		}
	}

	if !opts.MatchFuzzily {
		conds = append(conds,
			builder.And(
				builder.In("email", queryParts),
				builder.Eq{"discoverable_by_email": true},
			),
		)
	}

	cond := builder.Or(conds...)

	if opts.AdditionalCond != nil {
		cond = builder.And(
			cond,
			opts.AdditionalCond,
		)
	}

	if config.ServiceEnableOpenIDTeamUserOnlySearch.GetBool() && !orgDirectory {
		teamMemberCond := builder.In("id", builder.Select("user_id").
			From("team_members").
			Where(builder.In("team_id",
				builder.Select("team_id").
					From("team_members").
					Where(builder.Eq{"team_members.user_id": currentUser.ID}),
			)),
		)

		if !opts.MatchFuzzily {
			cond = builder.And(
				cond,
				builder.Or(
					teamMemberCond,
					builder.And(
						builder.In("email", queryParts),
						builder.Eq{"discoverable_by_email": true},
					),
				),
			)
		} else {
			cond = builder.And(
				cond,
				teamMemberCond,
			)
		}
	}

	notSomeoneElsesBot := builder.Or(
		builder.IsNull{"bot_owner_id"},
		builder.Eq{"bot_owner_id": 0},
		builder.Eq{"bot_owner_id": currentUser.ID},
	)

	// Own bots: filtered by the search string when one is provided (matched
	// against username and name), but bypass the discoverable_by_name flag
	// regular users have. Without a search, ListUsers only returns rows when
	// ReturnAllIfNoSearchProvided is set, so we surface own bots there too.
	var ownBotCond builder.Cond = builder.Eq{"bot_owner_id": currentUser.ID}
	if search != "" {
		botSearchConds := []builder.Cond{}
		for _, queryPart := range queryParts {
			if opts.MatchFuzzily {
				botSearchConds = append(botSearchConds,
					db.ILIKE("name", queryPart),
					db.ILIKE("username", queryPart),
				)
				continue
			}

			botSearchConds = append(botSearchConds,
				db.ILIKE("username", queryPart),
				db.ILIKE("name", queryPart),
			)
		}
		ownBotCond = builder.And(
			builder.Eq{"bot_owner_id": currentUser.ID},
			builder.Or(botSearchConds...),
		)
	}

	query := s.
		Where(builder.Or(
			builder.And(cond, notSomeoneElsesBot),
			ownBotCond,
		))
	if orgDirectory {
		// Rank exact and prefix matches first so the cap cuts the weakest matches.
		order, args := db.SearchRelevanceOrder(strings.TrimSpace(queryParts[0]), "username", "name", "email")
		query = query.OrderBy(order, args...).Limit(OrgDirectorySearchLimit)
	}
	query = query.OrderBy("id")
	err = query.Find(&users)

outer:
	for _, u := range users {
		for _, part := range strings.Split(search, ",") {
			if u.Email == part {
				continue outer
			}
		}
		u.Email = ""
	}
	return
}

// orgDirectoryApplies reports whether service.orgdirectory widens this search:
// only the global search (not the fuzzy per-project listing), and only for real users.
// The bot check reads the database because the auth user may not carry bot_owner_id.
func orgDirectoryApplies(s *xorm.Session, currentUser *User, opts *ProjectUserOpts) (bool, error) {
	if !config.ServiceOrgDirectory.GetBool() || opts.MatchFuzzily {
		return false, nil
	}
	searcher := &User{}
	exists, err := s.Where(builder.Eq{"id": currentUser.ID}).Cols("bot_owner_id").Get(searcher)
	if err != nil {
		return false, fmt.Errorf("org directory: load searching user: %w", err)
	}
	return exists && !searcher.IsBot(), nil
}

// ListAllUsers returns all users
func ListAllUsers(s *xorm.Session) (users []*User, err error) {
	err = s.Find(&users)
	return
}
