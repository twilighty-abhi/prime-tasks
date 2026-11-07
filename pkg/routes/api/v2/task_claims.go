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

package apiv2

import (
	"context"
	"net/http"

	"code.vikunja.io/api/pkg/models"
	"code.vikunja.io/api/pkg/web/handler"

	"github.com/danielgtaylor/huma/v2"
)

// RegisterTaskClaimRoutes wires the bot claim and release actions onto the Huma API.
// Both answer 200: a claim may be a renewal, and a release creates nothing.
func RegisterTaskClaimRoutes(api huma.API) {
	tags := []string{"tasks"}

	Register(api, huma.Operation{
		OperationID: "claim-task",
		Summary:     "Claim a task for a bot",
		Description: "Marks the task as being worked on by the calling bot for a 2-hour lease: adds the \"agent: working\" label and a \"Picked up by bot-…\" comment. " +
			"Only bot users with write access to the task may call this; people get 403. " +
			"Claiming a task the bot already holds renews the lease without a second comment. " +
			"If another bot holds a live claim, the call fails with 409 and the error names the holder and when its claim expires. Expired claims are ignored.",
		Method:        http.MethodPost,
		Path:          "/tasks/{task}/claim",
		Tags:          tags,
		DefaultStatus: http.StatusOK,
	}, claimTask)

	Register(api, huma.Operation{
		OperationID: "release-task",
		Summary:     "Release a bot's claim on a task",
		Description: "Removes the claim and the \"agent: working\" label. Optionally posts comment as a task comment (the outcome) and moves the task into bucket_id, with the same rules as a kanban move (moving into the done bucket marks the task done). " +
			"The bot holding the claim may call this, and so may any person with write access to the task, to clear a stuck claim; other bots get 409 naming the holder. Fails with 409 if the task has no claim.",
		Method:        http.MethodPost,
		Path:          "/tasks/{task}/release",
		Tags:          tags,
		DefaultStatus: http.StatusOK,
	}, releaseTask)
}

func init() { AddRouteRegistrar(RegisterTaskClaimRoutes) }

func claimTask(ctx context.Context, in *struct {
	TaskID int64 `path:"task" doc:"The numeric id of the task to claim."`
}) (*singleBody[models.TaskClaim], error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	claim := &models.TaskClaim{TaskID: in.TaskID}
	if err := handler.DoCreate(ctx, claim, a); err != nil {
		return nil, translateDomainError(err)
	}
	return &singleBody[models.TaskClaim]{Body: claim}, nil
}

type taskReleaseBody struct {
	Comment  string `json:"comment,omitempty" doc:"Optional outcome to post as a task comment, as the caller."`
	BucketID int64  `json:"bucket_id,omitempty" doc:"Optional kanban bucket to move the task into, e.g. Review. Must belong to a view of the task's project."`
}

func releaseTask(ctx context.Context, in *struct {
	TaskID int64  `path:"task" doc:"The numeric id of the task to release."`
	Format string `query:"format" enum:"html,markdown" doc:"How rich-text fields are exchanged. See the API description."`
	Body   *taskReleaseBody
}) (*singleBody[models.TaskClaim], error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	release := &models.TaskClaim{TaskID: in.TaskID}
	if in.Body != nil {
		if err := convertToHTML(ctx, &in.Body.Comment); err != nil {
			return nil, translateDomainError(err)
		}
		release.Comment = in.Body.Comment
		release.BucketID = in.Body.BucketID
	}
	if err := handler.DoDelete(ctx, release, a); err != nil {
		return nil, translateDomainError(err)
	}
	return &singleBody[models.TaskClaim]{Body: release}, nil
}
