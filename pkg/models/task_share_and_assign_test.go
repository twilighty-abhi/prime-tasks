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
	"testing"

	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// F2 (prime-tasks): share the project and assign in one go.
func TestTaskShareAndAssign(t *testing.T) {
	owner := &user.User{ID: 1, Username: "user1"}
	outsider := &user.User{ID: 4, Username: "user4"}

	// setup: a project of user1 with one task; user2 gets the given share; user4 has no access.
	setup := func(t *testing.T, callerPermission Permission) (projectID, taskID int64) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		p := &Project{Title: "share and assign"}
		require.NoError(t, p.Create(s, owner))
		task := &Task{Title: "needs a hand", ProjectID: p.ID}
		require.NoError(t, task.Create(s, owner))
		require.NoError(t, (&ProjectUser{Username: "user2", ProjectID: p.ID, Permission: callerPermission}).Create(s, owner))
		require.NoError(t, s.Commit())
		return p.ID, task.ID
	}
	run := func(t *testing.T, caller *user.User, sa *TaskShareAndAssign) (bool, error) {
		s := db.NewSession()
		defer s.Close()
		can, err := sa.CanCreate(s, caller)
		require.NoError(t, err)
		if !can {
			return false, nil
		}
		if err := sa.Create(s, caller); err != nil {
			_ = s.Rollback()
			return true, err
		}
		require.NoError(t, s.Commit())
		return true, nil
	}

	t.Run("admin shares read & write and assigns, in one go", func(t *testing.T) {
		projectID, taskID := setup(t, PermissionAdmin)
		sa := &TaskShareAndAssign{TaskID: taskID, UserID: outsider.ID}
		allowed, err := run(t, &user.User{ID: 2, Username: "user2"}, sa)
		require.True(t, allowed)
		require.NoError(t, err)
		assert.True(t, sa.Shared)
		db.AssertExists(t, "users_projects", map[string]interface{}{"user_id": outsider.ID, "project_id": projectID, "permission": PermissionWrite}, false)
		db.AssertExists(t, "task_assignees", map[string]interface{}{"user_id": outsider.ID, "task_id": taskID}, false)
	})

	t.Run("someone who can already see the project is only assigned", func(t *testing.T) {
		projectID, taskID := setup(t, PermissionRead)
		sa := &TaskShareAndAssign{TaskID: taskID, UserID: 2}
		allowed, err := run(t, owner, sa)
		require.True(t, allowed)
		require.NoError(t, err)
		assert.False(t, sa.Shared)
		db.AssertExists(t, "users_projects", map[string]interface{}{"user_id": 2, "project_id": projectID, "permission": PermissionRead}, false)
		db.AssertExists(t, "task_assignees", map[string]interface{}{"user_id": 2, "task_id": taskID}, false)
	})

	for _, permission := range []Permission{PermissionRead, PermissionWrite} {
		t.Run("a "+matrixPermissionNames[permission]+" member can't share", func(t *testing.T) {
			projectID, taskID := setup(t, permission)
			allowed, _ := run(t, &user.User{ID: 2, Username: "user2"}, &TaskShareAndAssign{TaskID: taskID, UserID: outsider.ID})
			assert.False(t, allowed)
			db.AssertMissing(t, "users_projects", map[string]interface{}{"user_id": outsider.ID, "project_id": projectID})
			db.AssertMissing(t, "task_assignees", map[string]interface{}{"user_id": outsider.ID, "task_id": taskID})
		})
	}

	t.Run("a failed assign leaves no share behind", func(t *testing.T) {
		projectID, taskID := setup(t, PermissionAdmin)
		allowed, err := run(t, owner, &TaskShareAndAssign{TaskID: taskID, UserID: 9999999})
		require.True(t, allowed)
		require.Error(t, err)
		assert.True(t, user.IsErrUserDoesNotExist(err), "got %v", err)
		db.AssertMissing(t, "users_projects", map[string]interface{}{"project_id": projectID, "user_id": 9999999})
	})

	t.Run("link shares can't", func(t *testing.T) {
		_, taskID := setup(t, PermissionAdmin)
		s := db.NewSession()
		defer s.Close()
		can, err := (&TaskShareAndAssign{TaskID: taskID, UserID: outsider.ID}).CanCreate(s, &LinkSharing{ID: 1, ProjectID: 1, Permission: PermissionAdmin})
		require.NoError(t, err)
		assert.False(t, can)
	})
}
