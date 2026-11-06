# prime-tasks

prime-tasks is a thin fork of [Vikunja](https://github.com/go-vikunja/vikunja) (AGPL-3.0) for a 20-30 person
team. On top of upstream it adds fixed sharing and teams, a custom UI theme, and agent-friendly MCP additions.

Rules for the fork:

- Every behaviour change sits behind a config flag that defaults to upstream behaviour.
- The Go module path and package names stay upstream's. Branding lives in config, the theme layer and one constant.
- `pkg/license/` is not touched.
- Upstream's `AGENTS.md` and `.agents/docs/` apply here too (mage, `/api/v2` for new routes, generated client,
  no hand edits to `pkg/swagger/` or `config.yml.sample`).

## Branches and remotes

| Branch | What it is |
|---|---|
| `main` | Exact mirror of `upstream/main`. Never commit to it. |
| `prime` | Our work: the latest upstream release tag plus the patches below. PRs target `prime`. |

`upstream` = `https://github.com/go-vikunja/vikunja.git` (push disabled locally). Releases are tagged
`prime-vX.Y.Z` (upstream version) or `prime-vX.Y.Z-pN` for our own releases between upstream releases.

## Patch list

| # | What | PR | Config flag | Upstream status |
|---|---|---|---|---|
| B1 | Magefile compiles on Windows (process-group helpers split by OS) | [#1](https://github.com/twilighty-abhi/prime-tasks/pull/1) | none (build tooling) | not sent yet; good upstream candidate |
| D1 | Office VM deploy: `deploy/` compose stack (app, Postgres 16, Caddy, one-shot file permissions), `config.yml`, `.env.example`, `backup.sh`, `sql/flip-discoverable.sql`, README with a license-free account, team, sharing and password-reset setup; `ACME_EMAIL` is required | [#1](https://github.com/twilighty-abhi/prime-tasks/pull/1), license-free setup in [#10](https://github.com/twilighty-abhi/prime-tasks/pull/10) | none (deploy files only) | fork-only |
| B2 | Test fixtures load on Windows (`path.Join` for `embed.FS` paths) | [#1](https://github.com/twilighty-abhi/prime-tasks/pull/1) | none (test tooling) | not sent yet; good upstream candidate |
| W1 | Fork CI and release: `prime-ci.yml` runs upstream's `test.yml` on PRs into and pushes to `prime`; `prime-release.yml` pushes `ghcr.io/twilighty-abhi/prime-tasks` on `prime-v*` tags; upstream-only workflows disabled (see below) | [#1](https://github.com/twilighty-abhi/prime-tasks/pull/1), workflow list in [#12](https://github.com/twilighty-abhi/prime-tasks/pull/12) | none (CI only) | fork-only |
| Y1 | Yuyu theme layer: tokens, Vikunja variables mapped onto them, self-hosted fonts, component rules, `PRODUCT_NAME` branding | [#5](https://github.com/twilighty-abhi/prime-tasks/pull/5) | none (branding) | fork-only |
| F9 | Permission test matrix for project access (`pkg/models/project_permission_matrix_test.go`) | [#1](https://github.com/twilighty-abhi/prime-tasks/pull/1) | none (tests only) | fork-only |
| F4 | Leave project: `DELETE /api/v2/projects/{project}/shares/me`, self-removal in `ProjectUser.CanDelete`, "Leave project" in the project menu for everyone but the owner; without a direct share, the dialog names the team (with a link) or parent project the access comes from | [#3](https://github.com/twilighty-abhi/prime-tasks/pull/3), team-only dialog in [#11](https://github.com/twilighty-abhi/prime-tasks/pull/11) | none (new endpoint and menu item; removing anyone else stays admin-only) | fork-only for now |
| F3 | Shared indicator: `shared_via` and `owner_display_name` on v2 projects; sidebar Shared with me section, Shared badge, All / Personal / Shared filter | [#6](https://github.com/twilighty-abhi/prime-tasks/pull/6) | none (new fields and UI) | fork-only for now |
| F1 | Org directory mode: user search matches part of the name, username or email of any non-bot user | [#2](https://github.com/twilighty-abhi/prime-tasks/pull/2) | `service.orgdirectory` (default `false`; on in `deploy/config.yml`) | fork-only for now |
| F6 | Auto-join teams: new users join teams by email domain. `models.AutoJoinTeams` runs from the `user.created` listener (registration, OpenID, LDAP, admin API) and synchronously in `vikunja user create`, which exits before async listeners run | [#4](https://github.com/twilighty-abhi/prime-tasks/pull/4), CLI fix in [#9](https://github.com/twilighty-abhi/prime-tasks/pull/9) | `service.autojointeams` (default empty; commented example in `deploy/config.yml`) | fork-only for now |

## Merging a new upstream release

```bash
git fetch upstream --tags
git switch main && git merge --ff-only upstream/main && git push origin main
git switch prime
git merge vX.Y.Z               # the new upstream release tag
# resolve conflicts, then:
mage lint:fix
mage test:feature 2>&1 | tee /tmp/feature.log
mage test:web     2>&1 | tee /tmp/web.log
mage test:filter TestProjectPermissionMatrix 2>&1 | tee /tmp/matrix.log   # F9 must stay green
git push origin prime
git tag -a prime-vX.Y.Z -m "prime-tasks on Vikunja vX.Y.Z" && git push origin prime-vX.Y.Z
```

The `prime-v*` tag triggers `.github/workflows/prime-release.yml`, which pushes
`ghcr.io/twilighty-abhi/prime-tasks:X.Y.Z` and `:latest`.

Upstream workflow files stay in the tree unchanged, so merges don't conflict. The upstream-only ones are disabled
in the repo settings instead (Actions, then the workflow, then "Disable workflow"), or with
`gh workflow disable <file> -R twilighty-abhi/prime-tasks`.

GitHub only lists a workflow (and only lets you disable it) after something has triggered it once. A workflow marked
"never registered" below has never run on the fork: disable it the first time it shows up under Actions.

| Workflow | Trigger | State on the fork (2026-11-06) | Why |
|---|---|---|---|
| `prime-ci.yml` | PRs into and pushes to `prime` | **active** | ours: runs `test.yml` |
| `prime-release.yml` | push of a `prime-v*` tag | never registered (no tag yet) | ours: GHCR image |
| `test.yml` | `workflow_call` | reusable, called by `prime-ci.yml` | upstream test suite |
| `release.yml` | `workflow_call` | reusable, only called by `ci.yml` (disabled) | upstream release jobs |
| `ci.yml` | pushes and PRs on upstream branches | disabled | its release job publishes to Docker Hub with upstream's secrets |
| `agentscan.yml` | PRs | disabled | upstream bot |
| `auto-label.yml` | issues and PRs | disabled | upstream bot |
| `automerge-label.yml` | PRs | disabled | upstream bot |
| `preview.yml` | PRs | disabled | upstream preview deploys |
| `stale-waiting-for-reply.yml` | schedule, issue comments | disabled | upstream issue triage |
| `dependency-diff.yml` | PRs that change `pnpm-lock.yaml` | never registered; disable on first run | upstream dependency report (read-only, comments on the PR) |
| `crowdin.yml` | schedule | never registered | upstream translations |
| `nixpkgs-update.yml` | published GitHub release, manual | never registered | upstream Nix packaging; we only push tags |
| `issue-closed-comment.yml` | issue closed | never registered | upstream issue bot |

If an upstream merge adds a workflow, add it to this table and decide whether it needs disabling.

## Building and running on abhi-t4gc (dev laptop, Windows 11)

Everything lives in `C:\dev\prime-tasks` on the laptop:

| Path | What |
|---|---|
| `repo\` | this repository |
| `tools\` | portable Go 1.27.3, PostgreSQL 16.15, MinGW-w64 GCC 16.2 (cgo for go-sqlite3), pnpm 11.27 |
| `gopath\`, `cache\` | Go modules, build cache, pnpm store |
| `pgdata\` | dev Postgres cluster (localhost:5432, db and user `prime_dev`) |
| `run\` | `vikunja.exe`, dev `config.yml`, `files\` |
| `logs\` | one log per background process and test run |
| `scripts\` | `env.cmd`, `start-all.ps1`, `stop-all.ps1`, `status.ps1`, `bg.ps1` |
| `env.sh` | `source` it in Git Bash to get the same environment |

Nothing is installed system-wide. Node 24 is the existing system install.

```bash
ssh abhi-t4gc
# Git Bash
. /c/dev/prime-tasks/env.sh && cd /c/dev/prime-tasks/repo
(cd frontend && pnpm install && pnpm run build)   # the API binary embeds frontend/dist
mage build && cp vikunja.exe ../run/
```

Background processes (started through WMI so they survive an SSH disconnect; output in `logs\<name>.log`):

```powershell
C:\dev\prime-tasks\scripts\start-all.ps1   # postgres, api (:3456), frontend dev server (:4173)
C:\dev\prime-tasks\scripts\status.ps1
C:\dev\prime-tasks\scripts\stop-all.ps1
```

From the tailnet: frontend <http://abhi-t4gc:4173/> (proxies `/api` to the API), API <http://abhi-t4gc:3456/>,
MCP `POST http://abhi-t4gc:4173/api/v2/mcp` with an API token (see [MCP tokens](#mcp-tokens)).

## Building and running on the Mac

Since 2026-11-06 development happens on the Mac in `~/Documents/Projects/prime-tasks`. Nothing is installed
system-wide: Node 24.21.0 and pnpm 11.27.0 live in `.local-tools/` (`source .local-tools/env.sh`), Go is Homebrew's
(`GOTOOLCHAIN=local`), and `run/` holds the dev binary, its `config.yml` (SQLite) and the demo seed
(`run/seed-users.sh`, then `python3 -I run/seed_demo.py`). `.local-tools/` and `run/` are in `.git/info/exclude`.

```bash
source .local-tools/env.sh
(cd frontend && pnpm install && pnpm build)
GOTOOLCHAIN=local go build -o run/vikunja . && (cd run && ./vikunja web)   # http://localhost:3456/
```

For Postgres, use Homebrew's `postgresql@16` in a throwaway cluster (`initdb` into `run/`, any free port) and
point the tests at it with `VIKUNJA_TESTS_USE_CONFIG=1 VIKUNJA_DATABASE_TYPE=postgres VIKUNJA_DATABASE_HOST=…`.
`golangci-lint` and Playwright's own Chromium aren't installed on the Mac, so CI is the gate for `mage lint` and
`mage test:e2e`.

## MCP tokens

Upstream's MCP endpoint is `POST /api/v2/mcp`. The token needs the `mcp` → `access` permission **and** the route
permissions of every tool it should see: the server lists only tools whose routes the token may call. A token with
only `mcp:access` gets a valid `tools/list` with **0 tools**; one with every permission sees 27 typed tools plus
`find_action` / `do_action` (89 operations in all, `pkg/modules/mcp/exposure.go`). The fork's own endpoints (leave
project, access list, share and assign) are not exposed over MCP yet.

## Login rate limit

Upstream limits logins per client IP (`/api/v1/login`, `/api/v2/login`): after about ten in a minute it answers
`429 Too Many Requests` for a while. Scripts and browser tests that log in as several users should log in once per
user and reuse the token (put it in `localStorage.token` for the frontend) instead of submitting the login form
every time.

## Local CI (while GitHub Actions is unavailable)

`scripts/local-ci.sh` runs the local equivalent of `prime-ci` on the Mac and exits non-zero if any step fails.
A PR may merge only after a full run passes on its head commit, with the summary
(`run/local-ci/<sha>/summary.md`) posted as a PR comment.

| Step | What |
|---|---|
| build | `pnpm install --frozen-lockfile`, `pnpm build`, `mage build` |
| generators | `mage check:translations`, `check:frontend-client`, `check:yaegi-symbols`, `check:got-swag` (swagger) |
| Go lint | `golangci-lint run` in the repo and in `veans/` |
| Go suites | `mage test:feature`, `test:web`, `test:caldav`, `test:e2e-api`, each on SQLite (in memory) and on Postgres 16, with `GOFLAGS=-count=1` so no cached result is reused |
| F9 | `mage -v test:filter TestProjectPermissionMatrix` on SQLite and Postgres 16, with counts |
| frontend | `pnpm lint`, `lint:styles`, `typecheck`, `vitest run --dir ./src` |
| e2e | `mage test:e2e ""` with CI's env: Mailpit (on 127.0.0.1:11025/18025, since other projects often hold 1025/8025) and Dex in Docker (CI's pinned images), a mail-enabled API on :3457, `CI=1` for Playwright's retries |

Needs: a clean working tree, Homebrew `postgresql@16` (the script makes a throwaway cluster in `run/` and deletes
it), and Docker Desktop running for the e2e step. It doesn't run the CI-only MySQL/MariaDB/ParadeDB suites,
`test-migration-smoke`, `test-s3-integration`, `veans-test` or `test-veans-e2e`.

Known macOS-host e2e failure: `tests/e2e/misc/menu.spec.ts:19` ("Can be toggled with keyboard shortcut on desktop")
presses `ControlOrMeta+e`, which on a Mac host sends Cmd. Playwright's "Desktop Chrome" device reports a Windows user
agent, and the app picks Ctrl or Cmd from the user agent (`helpers/isAppleDevice.ts`), so it waits for Ctrl. On CI's
Linux runner the same key is Ctrl and the test passes. The script still runs it and names it in the summary, but
doesn't fail the e2e step for it; any other failure does.

The script runs everything with `TZ=UTC`, as CI does: the e2e time-entry factory writes local time as a naive
datetime that the API reads as UTC, so in IST the time-tracking tests fail after 18:30.

Differences from CI: Dex's issuer is `http://127.0.0.1:5556` instead of `http://dex:5556` (same config otherwise;
the Mac can't resolve `dex`), and the Dex image runs under amd64 emulation. Playwright runs serially
(`workers: 1`, as in upstream's config) instead of in 6 shards, so the e2e step takes the longest.

Tools in `.local-tools/` (versions pinned to CI; nothing system-wide):

| Tool | Version | Where | Source |
|---|---|---|---|
| golangci-lint | v2.14.0 (`.github/actions/golangci-lint`) | `.local-tools/bin/` | GitHub release tarball, checksum verified |
| Playwright Chromium + headless shell | for `@playwright/test` 1.63.0 (CI: `mcr.microsoft.com/playwright:v1.63.0-jammy`) | `.local-tools/ms-playwright/` (`PLAYWRIGHT_BROWSERS_PATH`) | `pnpm exec playwright install chromium chromium-headless-shell` |
| swag | as pinned in `go.mod` | `.local-tools/bin/` | `go build -o .local-tools/bin/swag github.com/swaggo/swag/cmd/swag` (otherwise `mage check:got-swag` installs it into `~/go/bin`) |

`source .local-tools/env.sh` (pnpm 11) or `.local-tools/env-pnpm12.sh` (pnpm 12, from v2.8.0 on) puts them on
`PATH`; the script picks the right one from `frontend/package.json`.

## Test baseline

Measured on abhi-t4gc (Windows 11, Go 1.27.3, SQLite in memory) on 2026-10-27. Logs are in `C:\dev\prime-tasks\logs\`.

**Pure upstream v2.7.1:** on Windows, every DB-backed suite died in `TestMain` because fixture paths were built with
`filepath.Join` (backslashes) against an `embed.FS` (fix B2 below). Before the fix: `test:feature` 34 packages ok
and 20 failed, and `test:web`, `test:caldav` and `test:e2EApi` failed before running a single test. `mage` itself
didn't compile on Windows (fix B1).

**With B1 and B2 (the foundation branch):**

| Suite | Result |
|---|---|
| `mage test:feature` | 48 packages ok, 6 fail, all Windows-only (below) |
| `mage test:web` | ok |
| `mage test:caldav` | ok |
| `mage test:e2EApi` | ok |
| frontend unit (`vitest run --dir ./src`) | 202 files, 2536 tests passed |
| F9 matrix, SQLite and Postgres 16.15 | 91 subtests passed (F9 alone, before F3) |
| frontend e2e (Playwright) | not run on the laptop; CI runs it |

**macOS baseline** (the Mac, arm64, Go 1.27.1, `prime` at `853898a59`, 2026-11-06). Logs are in `run/logs-prime/`.

| Suite | Result |
|---|---|
| `mage test:feature` | 54 packages ok, 0 fail (the 6 Windows-only failures pass here) |
| `mage test:web`, `test:caldav`, `test:e2EApi` | ok |
| frontend unit (`vitest run --dir ./src`) | 205 files, 2554 tests passed |
| F9 matrix (`mage -v test:filter TestProjectPermissionMatrix`), SQLite and Postgres 16.15 | 84 matrix checks + 5 shared-via checks = 89, all passed; F2 (#7) adds a 12-check column |
| frontend e2e (Playwright) | not run on the Mac (Playwright's Chromium isn't installed); CI runs it |

`test:feature` failures that only happen on Windows (upstream Linux CI is green on v2.7.1):

| Package | Tests | Cause |
|---|---|---|
| `pkg/config`, `pkg/db` | `TestResolvePath`, `TestPathsFollowRootpath`, `Test_resolveDatabasePath*` | expect Unix absolute paths like `/etc/vikunja/config.yml` |
| `pkg/log`, `pkg/models` | `TestConfigureStandardLogger*`, `TestMakeLogHandlerCreatesCorrectLogFile`, `TestWebhookErrorResponseBodyIsTruncated` | temp dir cleanup can't delete a log file that is still open (Windows locks open files) |
| `pkg/modules/keyvalue` | `TestPutWithTTL` | 1 ns TTL; Windows clock resolution doesn't advance between put and get |
| `pkg/models`, `pkg/metrics` | `TestAPITokenExpiring{Week,Day}Notification`, `TestActiveUsers` | elapsed-time assertions; most likely the same coarse clock (to confirm on Linux CI) |

`mage lint:fix` on Windows also reports `pkg/utils/umask_windows.go` (unused parameter). That file is only
linted on Windows; CI lints on Linux. Left as is.

