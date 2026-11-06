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
	"encoding/json"
	"strings"

	"code.vikunja.io/api/pkg/config"
	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/log"
	"code.vikunja.io/api/pkg/user"

	"github.com/ThreeDotsLabs/watermill/message"
	"xorm.io/xorm"
)

// AutoJoinTeamsByEmailDomain adds a newly created user to the teams service.autojointeams
// lists for their email domain (prime-tasks F6). Every server-side way a user gets created
// (registration, OpenID, LDAP, admin API) dispatches user.CreatedEvent once, so this
// listener runs once for them. `vikunja user create` exits before the async router gets to
// the event, so the CLI calls AutoJoinTeams itself before it commits.
type AutoJoinTeamsByEmailDomain struct{}

// Name defines the name for the AutoJoinTeamsByEmailDomain listener
func (l *AutoJoinTeamsByEmailDomain) Name() string {
	return "prime.user.autojointeams"
}

// Handle is executed when the event AutoJoinTeamsByEmailDomain listens on is fired
func (l *AutoJoinTeamsByEmailDomain) Handle(msg *message.Message) (err error) {
	event := &user.CreatedEvent{}
	if err := json.Unmarshal(msg.Payload, event); err != nil {
		return err
	}
	if event.User == nil || event.User.ID == 0 {
		return nil
	}

	s := db.NewSession()
	defer s.Close()

	if err := AutoJoinTeams(s, event.User.ID); err != nil {
		_ = s.Rollback()
		return err
	}
	return s.Commit()
}

// AutoJoinTeams adds the user to every team service.autojointeams lists for their email
// domain (prime-tasks F6). It works inside the caller's session and doesn't commit. Teams the
// user is already in are skipped, so running it again for the same user changes nothing.
// A config mistake is logged and ignored, so it never fails user creation or retries an event.
func AutoJoinTeams(s *xorm.Session, userID int64) error {
	rules, err := config.AutoJoinTeamRules()
	if err != nil {
		log.Errorf("Auto-join teams: %s", err)
		return nil
	}
	if len(rules) == 0 {
		return nil
	}

	// The event's copy of the user has no email address; read the stored row with it.
	u, err := user.GetUserWithEmail(s, &user.User{ID: userID})
	if err != nil && !user.IsErrUserStatusError(err) {
		if user.IsErrUserDoesNotExist(err) {
			return nil
		}
		return err
	}
	if u.IsBot() {
		return nil
	}
	domain := emailDomain(u.Email)
	if domain == "" {
		return nil
	}

	for _, rule := range rules {
		if rule.Domain != domain {
			continue
		}
		if _, err := GetTeamByID(s, rule.TeamID); err != nil {
			if IsErrTeamDoesNotExist(err) {
				log.Warningf("Auto-join teams: team %d for domain %s does not exist, skipping it for user %d", rule.TeamID, rule.Domain, u.ID)
				continue
			}
			return err
		}
		member := &TeamMember{TeamID: rule.TeamID, Username: u.Username}
		if err := member.Create(s, u); err != nil {
			if IsErrUserIsMemberOfTeam(err) {
				continue
			}
			return err
		}
		log.Infof("Auto-join teams: added user %d to team %d (domain %s)", u.ID, rule.TeamID, domain)
	}

	return nil
}

func emailDomain(email string) string {
	at := strings.LastIndex(email, "@")
	if at < 0 || at == len(email)-1 {
		return ""
	}
	return config.NormalizeEmailDomain(email[at+1:])
}
