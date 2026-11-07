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

package migration

import (
	"testing"
	"time"

	"code.vikunja.io/api/pkg/db"

	"github.com/stretchr/testify/require"
)

func TestMigrationTaskClaims20261107043835(t *testing.T) {
	engine, err := db.CreateTestEngine()
	require.NoError(t, err)
	table := taskClaims20261107043835{}
	require.NoError(t, engine.DropTables(table))
	require.NoError(t, createTaskClaims20261107043835(engine))
	t.Cleanup(func() {
		require.NoError(t, engine.DropTables(table))
	})

	insert := func(taskID, botID int64) error {
		now := time.Now()
		_, err := engine.Insert(&taskClaims20261107043835{
			TaskID:    taskID,
			BotID:     botID,
			ClaimedAt: now,
			ExpiresAt: now.Add(2 * time.Hour),
		})
		return err
	}

	require.NoError(t, insert(1, 23))
	require.NoError(t, insert(2, 23))
	require.Error(t, insert(1, 25), "a task has at most one claim")
}
