# MCP in prime-tasks

prime-tasks serves an MCP server at `POST /api/v2/mcp`. It speaks streamable HTTP (stateless, one server per
request) and accepts only API tokens; a JWT from a browser login is refused with 401. The token needs the `mcp` →
`access` permission, and the server lists only the tools whose routes the token may call. A token with nothing but
`mcp:access` gets a valid `tools/list` with 0 tools.

Connect Claude Code:

```bash
claude mcp add --transport http prime-tasks https://<host>/api/v2/mcp --header "Authorization: Bearer <token>"
```

For an agent, don't use a person's token. Create a bot per agent with `scripts/new-agent-bot.sh` (A1); it issues a
token with the **Agent (safe)** permissions and prints a ready `claude mcp add` line. See
[deploy/README.md, "Agent bots"](../deploy/README.md#agent-bots) for creating, sharing projects with and revoking a
bot. The Agent (safe) preset (A2) is `models.AgentSafeAPIPermissions` in `pkg/models/api_token_agent_preset.go`;
it is also offered in Settings → MCP. A token with it sees **23 tools** and 12 catalog actions, with no deletes and
no sharing. The tables below mark them **safe**.

## How tools are exposed

The tool list comes from the v2 API. `pkg/modules/mcp/exposure.go` (`exposedOperations`) names the operations that
are exposed and puts each in one of two tiers:

- **Typed tools** (27) are listed by `tools/list` and called directly by name.
- **Catalog actions** (67) are not listed. The agent finds them with `find_action` and calls them with `do_action`.

That is 94 operations in all. A token with every permission sees 30 tools: the 27 typed tools plus `find_action`,
`do_action` and `filter_syntax`. The last three appear only when the token can reach at least one catalog action
(`find_action`, `do_action`) or one tool that takes a filter (`filter_syntax`).

Tool names are the operation ids with `-` turned into `_`. Arguments are one flat JSON object: path parameters (for
example `task`, `project`, `id`), query parameters and the request body's fields side by side. Read-only body fields
and body fields the path already sets are left out.

Common arguments:

- `format`: `html` (default) or `markdown`, for rich-text fields (task and project descriptions, comments).
- `fields`: return only these response fields, e.g. `["id", "title", "labels.title"]`. Read and list tools only.
- `page`, `per_page`: pagination on list tools.
- `filter`: task filter query, e.g. `done = false && priority >= 3`. Use `&&` and `||`, not `and` / `or`. Call
  `filter_syntax` for the full syntax.
- Update tools are PATCH: only the fields you pass change, and `null` clears a field.

### Discovery tools

| Tool | Safe | What | Example |
|---|---|---|---|
| `find_action` | safe | Lists the catalog actions the token may use. No arguments: names and descriptions. `action` or `resource` (a name prefix such as `task_labels`): full input schemas too | `{"resource": "task_labels"}` |
| `do_action` | safe | Calls a catalog action. `action` is the action name; `arguments` is an object matching its input schema | `{"action": "task_labels_create", "arguments": {"task": 42, "label_id": 7}}` |
| `filter_syntax` | safe | Returns the filter query reference (fields, operators, date math, examples). Also served as resource `vikunja://docs/filter-syntax` | `{}` |

In the tables below, examples for catalog actions show the full `do_action` call.

## Tool reference

Tier: **T** = typed tool, **C** = catalog action (via `do_action`). Safe = in the Agent (safe) preset.

### Tasks

| Tool | Tier | Safe | What | Example |
|---|---|---|---|---|
| `tasks_list` | T | safe | List tasks across all projects you can see, with filter, search and sort | `{"filter": "done = false && due_date < now+7d", "sort_by": ["due_date"], "per_page": 20}` |
| `tasks_read` | T | safe | Get one task by id; `expand` adds comments, subtasks, buckets and more | `{"task": 42, "expand": ["comments"], "format": "markdown"}` |
| `tasks_create` | T | safe | Create a task in a project | `{"project": 5, "title": "Write release notes", "priority": 3, "format": "markdown", "description": "Draft in **Markdown**"}` |
| `tasks_update` | T | safe | Change a task's fields; `project_id` moves it to another project | `{"task": 42, "done": true}` |
| `tasks_delete` | T | | Move a task to the trash (restorable for 30 days) | `{"task": 42}` |
| `tasks_read_by_index` | C | safe | Get a task by its per-project index; `project` is an id or the project identifier | `{"action": "tasks_read_by_index", "arguments": {"project": "OPS", "index": 12}}` |
| `project_tasks_list` | C | safe | List the tasks of one project, with filter, search and sort | `{"action": "project_tasks_list", "arguments": {"project": 5, "filter": "done = false"}}` |
| `tasks_bulk_create` | C | safe | Create up to 100 tasks in a project in one atomic request | `{"action": "tasks_bulk_create", "arguments": {"project": 5, "tasks": [{"title": "Step 1"}, {"title": "Step 2"}]}}` |
| `tasks_bulk_update` | C | | Apply the named `fields` from `values` to every task in `task_ids` | `{"action": "tasks_bulk_update", "arguments": {"task_ids": [42, 43], "fields": ["done"], "values": {"done": true}}}` |
| `tasks_duplicate` | C | | Copy a task (labels, assignees, attachments, reminders) into the same project | `{"action": "tasks_duplicate", "arguments": {"task": 42}}` |
| `tasks_mark_read` | C | | Clear your unread status on a task | `{"action": "tasks_mark_read", "arguments": {"task": 42}}` |
| `tasks_position_update` | C | | Set a task's sort position within one view | `{"action": "tasks_position_update", "arguments": {"task": 42, "project_view_id": 9, "position": 1024}}` |
| `tasks_relations_create` | C | | Relate two tasks (`subtask`, `blocking`, `related`, ...); the inverse is added automatically | `{"action": "tasks_relations_create", "arguments": {"task": 42, "other_task_id": 43, "relation_kind": "blocking"}}` |
| `tasks_relations_delete` | C | | Remove a relation (and its inverse) | `{"action": "tasks_relations_delete", "arguments": {"task": 42, "relationKind": "blocking", "otherTask": 43}}` |
| `task_attachments_list` | C | | List a task's attachment metadata (no file bytes) | `{"action": "task_attachments_list", "arguments": {"task": 42}}` |
| `task_attachments_delete` | C | | Delete an attachment and its file | `{"action": "task_attachments_delete", "arguments": {"task": 42, "attachment": 3}}` |

### Comments

| Tool | Tier | Safe | What | Example |
|---|---|---|---|---|
| `task_comments_list` | T | safe | List a task's comments, oldest first (`order_by: "desc"` for newest first) | `{"task": 42, "order_by": "desc", "format": "markdown"}` |
| `task_comments_read` | T | safe | Get one comment | `{"task": 42, "commentid": 101}` |
| `task_comments_create` | T | safe | Comment on a task as the caller | `{"task": 42, "comment": "Found the cause, fixing now.", "format": "markdown"}` |
| `task_comments_update` | T | safe | Change a comment's text (author only) | `{"task": 42, "commentid": 101, "comment": "Fixed in #123."}` |
| `task_comments_delete` | T | | Delete a comment (author only) | `{"task": 42, "commentid": 101}` |

### Labels

| Tool | Tier | Safe | What | Example |
|---|---|---|---|---|
| `labels_list` | T | safe | List the labels you can see (your own plus those on tasks you can access) | `{"q": "bug"}` |
| `labels_read` | T | safe | Get one label | `{"id": 7}` |
| `labels_create` | T | safe | Create a label; you own it | `{"title": "needs-review", "hex_color": "e8a33d"}` |
| `labels_update` | T | safe | Change a label (owner only) | `{"id": 7, "title": "review"}` |
| `labels_delete` | T | | Delete a label (owner only) | `{"id": 7}` |
| `task_labels_list` | C | safe | List the labels on a task | `{"action": "task_labels_list", "arguments": {"task": 42}}` |
| `task_labels_create` | C | safe | Add an existing label to a task | `{"action": "task_labels_create", "arguments": {"task": 42, "label_id": 7}}` |
| `task_labels_delete` | C | | Remove a label from a task | `{"action": "task_labels_delete", "arguments": {"task": 42, "label": 7}}` |
| `task_labels_bulk_replace` | C | | Set a task's labels to exactly this list | `{"action": "task_labels_bulk_replace", "arguments": {"task": 42, "labels": [{"id": 7}, {"id": 8}]}}` |

### Assignees

| Tool | Tier | Safe | What | Example |
|---|---|---|---|---|
| `task_assignees_list` | T | safe | List a task's assignees | `{"task": 42}` |
| `task_assignees_create` | T | safe | Assign a user who already has access to the project | `{"task": 42, "user_id": 3}` |
| `task_assignees_delete` | T | | Unassign a user | `{"task": 42, "user": 3}` |
| `task_assignees_bulk` | C | | Replace a task's whole assignee set (empty list unassigns everyone) | `{"action": "task_assignees_bulk", "arguments": {"task": 42, "assignees": [{"id": 3}]}}` |

Share and assign (for a user without access yet) is under [Fork additions](#fork-additions).

### Projects

| Tool | Tier | Safe | What | Example |
|---|---|---|---|---|
| `projects_list` | T | safe | List your projects (owned and shared); `expand: "permissions"` adds your permission | `{"expand": "permissions", "fields": ["id", "title", "max_permission"]}` |
| `projects_read` | T | safe | Get one project with its views | `{"id": 5}` |
| `projects_create` | T | | Create a project; you own it | `{"title": "Q1 launch", "parent_project_id": 2}` |
| `projects_update` | T | | Change a project's fields | `{"id": 5, "is_archived": true}` |
| `projects_delete` | T | | Move a project and its sub-projects to the trash | `{"id": 5}` |
| `projects_duplicate` | C | | Deep-copy a project into a new one you own | `{"action": "projects_duplicate", "arguments": {"projectid": 5, "duplicate_shares": false}}` |
| `projects_users_search` | C | safe | Search the users who can access a project | `{"action": "projects_users_search", "arguments": {"project": 5, "q": "abhi"}}` |

The access list (who has access and why) is under [Fork additions](#fork-additions).

### Views and buckets

| Tool | Tier | Safe | What | Example |
|---|---|---|---|---|
| `project_views_list` | C | safe | List a project's views (list, gantt, table, kanban) | `{"action": "project_views_list", "arguments": {"project": 5}}` |
| `project_views_read` | C | safe | Get one view | `{"action": "project_views_read", "arguments": {"project": 5, "view": 9}}` |
| `project_views_create` | C | | Create a view (project admins) | `{"action": "project_views_create", "arguments": {"project": 5, "title": "Board", "view_kind": "kanban", "bucket_configuration_mode": "manual"}}` |
| `project_views_update` | C | | Change a view (project admins) | `{"action": "project_views_update", "arguments": {"project": 5, "view": 9, "title": "Sprint board"}}` |
| `project_views_delete` | C | | Delete a view with its buckets (project admins) | `{"action": "project_views_delete", "arguments": {"project": 5, "view": 9}}` |
| `project_view_tasks_list` | C | safe | List the tasks in a view, flat; the view's own filter applies too | `{"action": "project_view_tasks_list", "arguments": {"project": 5, "view": 9, "filter": "priority >= 3"}}` |
| `project_view_buckets_tasks_list` | C | safe | List a kanban view's buckets, each with its tasks | `{"action": "project_view_buckets_tasks_list", "arguments": {"project": 5, "view": 9}}` |
| `buckets_list` | C | safe | List a kanban view's buckets | `{"action": "buckets_list", "arguments": {"project": 5, "view": 9}}` |
| `buckets_create` | C | | Create a bucket | `{"action": "buckets_create", "arguments": {"project": 5, "view": 9, "title": "Review"}}` |
| `buckets_update` | C | | Change a bucket's title, limit or position | `{"action": "buckets_update", "arguments": {"project": 5, "view": 9, "bucket": 14, "title": "In review", "limit": 5}}` |
| `buckets_delete` | C | | Delete a bucket; its tasks move to the default bucket | `{"action": "buckets_delete", "arguments": {"project": 5, "view": 9, "bucket": 14}}` |
| `task_bucket_update` | C | | Move a task into a bucket (the done bucket marks it done) | `{"action": "task_bucket_update", "arguments": {"project": 5, "view": 9, "bucket": 14, "task_id": 42}}` |

An agent with the safe preset moves a task between buckets with `tasks_update` (`bucket_id`) or with
`release_task` (`bucket_id`).

### Sharing

Permissions: `0` read only, `1` read & write, `2` admin. Only project admins may change shares.

| Tool | Tier | Safe | What | Example |
|---|---|---|---|---|
| `project_users_list` | C | | List the users a project is shared with directly | `{"action": "project_users_list", "arguments": {"project": 5}}` |
| `project_users_create` | C | | Share a project with a user, by username | `{"action": "project_users_create", "arguments": {"project": 5, "username": "bot-claude-laptop", "permission": 1}}` |
| `project_users_update` | C | | Change a user's permission | `{"action": "project_users_update", "arguments": {"project": 5, "user": "abhiram", "permission": 2}}` |
| `project_users_delete` | C | | Remove a user's direct access | `{"action": "project_users_delete", "arguments": {"project": 5, "user": "abhiram"}}` |
| `project_teams_list` | C | | List the teams a project is shared with | `{"action": "project_teams_list", "arguments": {"project": 5}}` |
| `project_teams_create` | C | | Share a project with a team | `{"action": "project_teams_create", "arguments": {"project": 5, "team_id": 2, "permission": 1}}` |
| `project_teams_update` | C | | Change a team's permission | `{"action": "project_teams_update", "arguments": {"project": 5, "team": 2, "permission": 0}}` |
| `project_teams_delete` | C | | Remove a team's access | `{"action": "project_teams_delete", "arguments": {"project": 5, "team": 2}}` |

### Teams

| Tool | Tier | Safe | What | Example |
|---|---|---|---|---|
| `teams_list` | C | | List your teams (`include_public` adds public teams) | `{"action": "teams_list", "arguments": {"include_public": true}}` |
| `teams_read` | C | | Get one team | `{"action": "teams_read", "arguments": {"id": 2}}` |
| `teams_create` | C | | Create a team; you become its admin | `{"action": "teams_create", "arguments": {"name": "Ops"}}` |
| `teams_update` | C | | Change a team (team admins) | `{"action": "teams_update", "arguments": {"id": 2, "description": "On-call rota"}}` |
| `teams_delete` | C | | Delete a team (team admins) | `{"action": "teams_delete", "arguments": {"id": 2}}` |
| `teams_members_add` | C | | Add a user to a team by username | `{"action": "teams_members_add", "arguments": {"team": 2, "username": "abhiram", "admin": false}}` |
| `teams_members_remove` | C | | Remove a user from a team | `{"action": "teams_members_remove", "arguments": {"team": 2, "user": "abhiram"}}` |
| `teams_members_toggle_admin` | C | | Flip a member's admin flag | `{"action": "teams_members_toggle_admin", "arguments": {"team": 2, "user": "abhiram"}}` |

### Time entries

| Tool | Tier | Safe | What | Example |
|---|---|---|---|---|
| `time_entries_list` | C | | List the time entries you can see | `{"action": "time_entries_list", "arguments": {"per_page": 50}}` |
| `time_entries_read` | C | | Get one time entry | `{"action": "time_entries_read", "arguments": {"id": 11}}` |
| `time_entries_create` | C | | Log time; leave out `end_time` to start a timer | `{"action": "time_entries_create", "arguments": {"task_id": 42, "start_time": "2026-11-07T09:00:00Z", "end_time": "2026-11-07T10:30:00Z", "comment": "Debugging"}}` |
| `time_entries_update` | C | | Change a time entry (author only) | `{"action": "time_entries_update", "arguments": {"id": 11, "comment": "Debugging the importer"}}` |
| `time_entries_delete` | C | | Delete a time entry (author only) | `{"action": "time_entries_delete", "arguments": {"id": 11}}` |
| `time_entries_timer_stop` | C | | Stop your running timer | `{"action": "time_entries_timer_stop", "arguments": {}}` |
| `project_time_entries_list` | C | | List a project's time entries | `{"action": "project_time_entries_list", "arguments": {"project_id": 5}}` |
| `task_time_entries_list` | C | | List a task's time entries | `{"action": "task_time_entries_list", "arguments": {"task_id": 42}}` |

### Notifications

| Tool | Tier | Safe | What | Example |
|---|---|---|---|---|
| `notifications_list` | C | | List your notifications, newest first | `{"action": "notifications_list", "arguments": {"per_page": 20}}` |
| `notifications_mark_read` | C | | Mark one notification read (or unread) | `{"action": "notifications_mark_read", "arguments": {"notificationid": 31, "read": true}}` |
| `notifications_mark_all_read` | C | | Mark all your notifications read | `{"action": "notifications_mark_all_read", "arguments": {}}` |
| `notifications_delete_all` | C | | Delete all your notifications | `{"action": "notifications_delete_all", "arguments": {}}` |

### Saved filters

| Tool | Tier | Safe | What | Example |
|---|---|---|---|---|
| `filters_read` | C | | Get a saved filter (owner only) | `{"action": "filters_read", "arguments": {"filter": 4}}` |
| `filters_create` | C | | Save a filter; the query sits under `filters` | `{"action": "filters_create", "arguments": {"title": "Due this week", "filters": {"filter": "done = false && due_date < now+7d"}}}` |
| `filters_update` | C | | Change a saved filter (owner only) | `{"action": "filters_update", "arguments": {"filter": 4, "title": "Due soon"}}` |
| `filters_delete` | C | | Delete a saved filter (owner only) | `{"action": "filters_delete", "arguments": {"filter": 4}}` |

### Reactions

`entitykind` is `tasks` or `comments`.

| Tool | Tier | Safe | What | Example |
|---|---|---|---|---|
| `reactions_list` | C | | List the reactions on a task or comment | `{"action": "reactions_list", "arguments": {"entitykind": "tasks", "entityid": 42}}` |
| `reactions_create` | C | | Add your reaction | `{"action": "reactions_create", "arguments": {"entitykind": "comments", "entityid": 101, "value": "👍"}}` |
| `reactions_delete` | C | | Remove your reaction | `{"action": "reactions_delete", "arguments": {"entitykind": "comments", "entityid": 101, "value": "👍"}}` |

### Users

| Tool | Tier | Safe | What | Example |
|---|---|---|---|---|
| `users_search` | T | safe | Search users by username, name or email (in org directory mode (F1), part of any of them) | `{"q": "abhi"}` |

### Fork additions

| Tool | Tier | Safe | What | Example |
|---|---|---|---|---|
| `my_work` | T | safe | Your open tasks across all projects, by due date then priority (A4). For a bot: its owner's tasks, in projects the bot can read | `{"fields": ["id", "title", "project", "due_date", "url"]}` |
| `claim_task` | T | safe | Claim a task for the calling bot for 2 hours; adds the "agent: working" label and a pickup comment (A5) | `{"task": 42}` |
| `release_task` | T | safe | Release a claim, with an optional outcome comment and bucket move (A5) | `{"task": 42, "comment": "Done, PR #123 is ready for review.", "bucket_id": 14, "format": "markdown"}` |
| `task_assignees_share_and_assign` | C | | Assign a user, sharing the project with them (read & write) first if needed; project admins only (F2, A3) | `{"action": "task_assignees_share_and_assign", "arguments": {"task": 42, "user_id": 3}}` |
| `project_access_list` | C | safe | Who has access to a project and why: owner, users, teams, inherited from parents (F5, A3) | `{"action": "project_access_list", "arguments": {"project": 5}}` |

Leave project (F4) is not exposed: an agent should not drop its own access.

### Totals

| | Typed tools | Catalog actions | Helper tools | Operations |
|---|---|---|---|---|
| Every permission | 27 | 67 | 3 (`find_action`, `do_action`, `filter_syntax`) | 94 |
| Agent (safe) | 20 | 12 | 3 | 32 |

`tools/list` shows typed plus helper tools: 30 with every permission, 23 with Agent (safe). The safe lists are
pinned in `pkg/webtests/mcp_agent_safe_preset_test.go`.

## Agent workflow

The loop for a bot with the Agent (safe) token:

1. **Find work.** Call `my_work`. A bot gets the open tasks assigned to its owner, in projects shared with the bot.
   Tasks in trashed or archived projects are left out.
2. **Claim.** Call `claim_task` with the task id before starting. The claim is a 2-hour lease. It adds the
   "agent: working" label and a "Picked up by bot-…" comment, so people see who is on it. Only bots can claim;
   a person gets 403.
3. **Work and report.** Comment progress with `task_comments_create`; change fields with `tasks_update`.
4. **Renew.** Work longer than 2 hours: call `claim_task` again. A claim the bot already holds is renewed, without
   a second comment.
5. **Release.** Call `release_task` when done or stopping. Pass `comment` for the outcome and `bucket_id` to move
   the task, e.g. to Review (`buckets_list` gives the ids). Moving into the view's done bucket marks the task done.

Errors to expect:

- **409 on claim:** another bot holds a live claim. The error names the holder and when the claim expires. Pick
  another task or wait; an expired claim no longer blocks.
- **409 on release:** the task has no claim, or another bot holds it. The holder may release, and so may any person
  with write access to the task (to clear a stuck claim).
- **404:** the task's project is in the trash. Claim and release answer the same way upstream's routes do for a
  trashed project.

Expired claims are cleaned up every 10 minutes.

## End-to-end check (Phase 2 gate)

<!-- E2E-RESULT -->
