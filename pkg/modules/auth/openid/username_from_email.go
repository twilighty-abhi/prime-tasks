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

package openid

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/asaskevich/govalidator"
	"xorm.io/xorm"
)

// maxUsernameBaseLength leaves room for a numeric suffix within the 250 byte
// username column.
const maxUsernameBaseLength = 200

// maxUsernameSuffix bounds the clash search; past it we fall back to a random name.
const maxUsernameSuffix = 1000

var reservedLinkSharePattern = regexp.MustCompile(`^link-share-\d+$`)

// sanitizeUsernameCandidate turns a free-form value into something
// user.CreateUser and the registration validator accept: letters and digits
// (any script) plus . _ -, everything else becomes a dash; no leading or
// trailing separators; no URL lookalikes; no reserved bot-/link-share- names.
// Returns "" when nothing usable is left.
func sanitizeUsernameCandidate(raw string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range raw {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '_':
			b.WriteRune(r)
			lastDash = false
		case !lastDash:
			b.WriteRune('-')
			lastDash = true
		}
	}
	candidate := strings.Trim(b.String(), "-._")

	if len(candidate) > maxUsernameBaseLength {
		cut := maxUsernameBaseLength
		for cut > 0 && !utf8.RuneStart(candidate[cut]) {
			cut--
		}
		candidate = strings.Trim(candidate[:cut], "-._")
	}

	if candidate == "" {
		return ""
	}

	// The registration validator rejects anything that parses as a URL, which
	// includes dotted names like first.last.
	if govalidator.IsURL(candidate) {
		candidate = strings.ReplaceAll(candidate, ".", "-")
	}

	lower := strings.ToLower(candidate)
	if strings.HasPrefix(lower, "bot-") || reservedLinkSharePattern.MatchString(lower) {
		candidate = "user-" + candidate
	}

	return candidate
}

// usernameCandidateFromEmail returns the sanitised, lowercased local part of
// an email address, without any +subaddress.
func usernameCandidateFromEmail(email string) string {
	at := strings.LastIndex(email, "@")
	if at <= 0 {
		return ""
	}
	local := strings.ToLower(email[:at])
	if plus := strings.Index(local, "+"); plus >= 0 {
		local = local[:plus]
	}
	return sanitizeUsernameCandidate(local)
}

func usernameTaken(s *xorm.Session, username string) (bool, error) {
	return s.Table("users").
		Where("LOWER(username) = LOWER(?) OR username = ?", username, username).
		Exist()
}

// newUserUsername returns the username to try for a new OpenID user. Without
// usernamefromemail it is the upstream preferred_username with spaces as
// dashes; empty or taken names then become random ones.
func newUserUsername(s *xorm.Session, cl *claims, provider *Provider) (string, error) {
	if !provider.UsernameFromEmail {
		return strings.ReplaceAll(cl.PreferredUsername, " ", "-"), nil
	}
	return usernameFromClaims(s, cl)
}

// usernameFromClaims picks a username for a new OpenID user: the
// preferred_username, else the email local part, with a numeric suffix on
// clashes. Returns "" when nothing usable exists so the caller falls back to
// a random name.
func usernameFromClaims(s *xorm.Session, cl *claims) (string, error) {
	base := sanitizeUsernameCandidate(cl.PreferredUsername)
	if base == "" {
		base = usernameCandidateFromEmail(cl.Email)
	}
	if base == "" {
		return "", nil
	}

	separator := ""
	if last, _ := utf8.DecodeLastRuneInString(base); unicode.IsDigit(last) {
		separator = "-"
	}

	candidate := base
	for i := 2; i <= maxUsernameSuffix; i++ {
		taken, err := usernameTaken(s, candidate)
		if err != nil {
			return "", err
		}
		if !taken {
			return candidate, nil
		}
		candidate = base + separator + strconv.Itoa(i)
	}

	return "", nil
}
