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
| B2 | Test fixtures load on Windows (`path.Join` for `embed.FS` paths) | [#1](https://github.com/twilighty-abhi/prime-tasks/pull/1) | none (test tooling) | not sent yet; good upstream candidate |
| F9 | Permission test matrix for project access (`pkg/models/project_permission_matrix_test.go`) | [#1](https://github.com/twilighty-abhi/prime-tasks/pull/1) | none (tests only) | fork-only |

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
in the repo settings instead (Actions, then the workflow, then "Disable workflow"), or with:

```bash
for w in ci.yml crowdin.yml nixpkgs-update.yml auto-label.yml agentscan.yml automerge-label.yml \
         preview.yml stale-waiting-for-reply.yml issue-closed-comment.yml; do
  gh workflow disable "$w" -R twilighty-abhi/prime-tasks
done
```

`ci.yml` is disabled because its release job publishes to Docker Hub with upstream's secrets.
`prime-ci.yml` runs upstream's `test.yml` on PRs into and pushes to `prime`. If upstream adds a new workflow,
check whether it needs disabling after the merge.

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
MCP `POST http://abhi-t4gc:4173/api/v2/mcp` with an API token that has the `mcp` permission.

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
| F9 matrix, SQLite and Postgres 16.15 | 91 subtests passed |
| frontend e2e (Playwright) | not run on the laptop; CI runs it |

`test:feature` failures that only happen on Windows (upstream Linux CI is green on v2.7.1):

| Package | Tests | Cause |
|---|---|---|
| `pkg/config`, `pkg/db` | `TestResolvePath`, `TestPathsFollowRootpath`, `Test_resolveDatabasePath*` | expect Unix absolute paths like `/etc/vikunja/config.yml` |
| `pkg/log`, `pkg/models` | `TestConfigureStandardLogger*`, `TestMakeLogHandlerCreatesCorrectLogFile`, `TestWebhookErrorResponseBodyIsTruncated` | temp dir cleanup can't delete a log file that is still open (Windows locks open files) |
| `pkg/modules/keyvalue` | `TestPutWithTTL` | 1 ns TTL; Windows clock resolution doesn't advance between put and get |
| `pkg/models`, `pkg/metrics` | `TestAPITokenExpiring{Week,Day}Notification`, `TestActiveUsers` | elapsed-time assertions; most likely the same coarse clock (to confirm on Linux CI) |

`mage lint:fix` on Windows also reports `pkg/utils/umask_windows.go` (unused parameter). That file is only
linted on Windows; CI lints on Linux. Left as is.

