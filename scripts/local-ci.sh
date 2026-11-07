#!/usr/bin/env bash
# prime-tasks: local equivalent of prime-ci (.github/workflows/test.yml), for while GitHub Actions
# is unavailable. Runs on the Mac with the tools in .local-tools/ (see PRIME.md, "Local CI").
#
#   scripts/local-ci.sh            run everything, write run/local-ci/<sha>/summary.md
#   LOCAL_CI_ONLY="lint,unit" ...  run a subset while iterating (the summary says so; not valid for a merge)
#
# Exits non-zero if any step fails. Every step runs even after a failure, so the summary is complete.
# Needs: Homebrew postgresql@16, Docker Desktop running (Mailpit and Dex for three e2e tests),
# and a clean working tree (the summary names the commit it ran on).

set -uo pipefail

ROOT=$(git rev-parse --show-toplevel)
cd "$ROOT"
SHA=$(git rev-parse HEAD)
SHORT=${SHA:0:9}
OUT=${LOCAL_CI_OUT:-$ROOT/run/local-ci/$SHORT}
ONLY=${LOCAL_CI_ONLY:-}
PG_BIN=/opt/homebrew/opt/postgresql@16/bin
PG_PORT=${LOCAL_CI_PG_PORT:-55433}
PG_DIR=$ROOT/run/local-ci-pg
PG_SOCK=$ROOT/run/lcpg
DEX_IMAGE=ghcr.io/go-vikunja/dex-testing:main@sha256:d401c06a9f8fd36ece446a07499b827232af7f21eb36872a76c9eac4d0c77bab
MAILPIT_IMAGE=axllent/mailpit:v1.31.6@sha256:23fe39e81bde596a918000b3135c0f85dc8bb4c9ea6be274b051143f3e300387
TEST_SECRET=averyLongSecretToSe33dtheDB
# e2e tests that fail only because the host is a Mac (see e2e_run); reported, but don't fail the step.
KNOWN_MAC_E2E_FAILURES=("tests/e2e/misc/menu.spec.ts:19:2")
# Mailpit on its own ports: other local projects often hold 1025/8025.
MAILPIT_SMTP=${LOCAL_CI_MAILPIT_SMTP:-11025}
MAILPIT_HTTP=${LOCAL_CI_MAILPIT_HTTP:-18025}

# swag (check:got-swag) scans the whole folder: a git worktree inside the repo duplicates pkg/ and breaks it.
NESTED=$(git worktree list --porcelain | awk '/^worktree /{print $2}' | grep -F "$ROOT/" || true)
if [ -n "$NESTED" ]; then
	echo "local-ci: move these git worktrees out of the repo folder first:" >&2
	echo "$NESTED" >&2
	exit 2
fi

if [ -n "$(git status --porcelain --untracked-files=no)" ] && [ "${LOCAL_CI_ALLOW_DIRTY:-}" != 1 ]; then
	echo "local-ci: the working tree has uncommitted changes; commit them so the summary names the code it ran on" >&2
	exit 2
fi

# Toolchain: pnpm 12 from v2.8.0 on, pnpm 11 before (frontend/package.json#packageManager).
if grep -q '"packageManager": "pnpm@12' frontend/package.json; then
	source "$ROOT/.local-tools/env-pnpm12.sh"
else
	source "$ROOT/.local-tools/env.sh"
fi
export GOTOOLCHAIN=local
# CI runs in UTC. Some e2e factories write local time as a naive datetime that the API reads as UTC,
# so on a UTC+5:30 Mac those time entries land in "tomorrow" after 18:30 and the day views miss them.
export TZ=UTC
# Never reuse cached test results: every run tests this commit for real.
export GOFLAGS=-count=1
export PLAYWRIGHT_BROWSERS_PATH="$ROOT/.local-tools/ms-playwright"

mkdir -p "$OUT"
ROWS="$OUT/rows.d"
rm -rf "$ROWS"; mkdir -p "$ROWS"
ORDER=()
START_ALL=$(date +%s)

want() { [ -z "$ONLY" ] || [[ ",$ONLY," == *",$1,"* ]]; }

# step <name> <detail-function or -> <command...>; writes its summary row to $ROWS/<name> (safe in parallel lanes).
step() {
	local name=$1 detail=$2; shift 2
	want "$name" || return 0
	local log="$OUT/$name.log" t0 rc d
	t0=$(date +%s)
	"$@" >"$log" 2>&1
	rc=$?
	d=$(( $(date +%s) - t0 ))
	local info=""
	[ "$detail" != - ] && info=$($detail "$log")
	if [ $rc -eq 0 ]; then
		printf '%-22s pass  %ss  %s\n' "$name" "$d" "$info"
		echo "| $name | pass | ${d}s | $info |" >"$ROWS/$name"
	else
		printf '%-22s FAIL  %ss  %s  (log: %s)\n' "$name" "$d" "$info" "$log"
		echo "| $name | **FAIL** (exit $rc) | ${d}s | $info |" >"$ROWS/$name"
		touch "$ROWS/$name.failed"
	fi
}

go_counts() { echo "$(grep -c '^ok' "$1") ok, $(grep -c '^FAIL[[:space:]]' "$1") failed packages"; }
f9_counts() {
	local m s f
	m=$(grep -E -- '--- PASS: TestProjectPermissionMatrix/' "$1" | awk '{n=gsub(/\//,"/",$3); if(n==3)c++} END{print c+0}')
	s=$(grep -cE -- '--- PASS: TestProjectPermissionMatrixSharedVia/' "$1")
	f=$(grep -c -- '--- FAIL' "$1")
	echo "$m matrix + $s shared-via passed, $f failed"
}
vitest_counts() { grep -E '^ +Tests ' "$1" | tail -1 | sed -E 's/^ +Tests +//'; }
eslint_counts() { grep -E '✖|problems' "$1" | tail -1 || echo "no problems"; }
pw_counts() {
	local c; c=$(grep -E '^ +[0-9]+ (passed|failed|flaky|skipped|did not run)' "$1" | sed -E 's/^ +//; s/ \(.*//' | paste -sd ',' -)
	[ -f "$OUT/e2e-known-mac-failures.txt" ] && c="$c; known macOS-host failure: $(paste -sd ' ' "$OUT/e2e-known-mac-failures.txt") (Cmd vs Ctrl, see PRIME.md)"
	echo "$c"
}
lint_counts() { local n; n=$(grep -cE '^[^ ].*\.go:[0-9]+:[0-9]+: ' "$1"); echo "$n issues"; }

# --- Postgres 16, throwaway cluster ---------------------------------------------------------
pg_start() {
	rm -rf "$PG_DIR" "$PG_SOCK"; mkdir -p "$PG_SOCK"
	"$PG_BIN/initdb" -D "$PG_DIR" -U postgres --auth=trust >/dev/null &&
		"$PG_BIN/pg_ctl" -D "$PG_DIR" -o "-p $PG_PORT -k $PG_SOCK -c listen_addresses=127.0.0.1" -l "$OUT/postgres.log" -w start >/dev/null
}
pg_stop() { [ -d "$PG_DIR" ] && "$PG_BIN/pg_ctl" -D "$PG_DIR" -m fast stop >/dev/null 2>&1; rm -rf "$PG_DIR" "$PG_SOCK"; }
pg_fresh_db() { "$PG_BIN/psql" -h 127.0.0.1 -p "$PG_PORT" -U postgres -qc "drop database if exists vikunjatest" -c "create database vikunjatest" >/dev/null; }
on_pg() {
	pg_fresh_db || return 1
	VIKUNJA_TESTS_USE_CONFIG=1 VIKUNJA_DATABASE_TYPE=postgres VIKUNJA_DATABASE_HOST=127.0.0.1:$PG_PORT \
		VIKUNJA_DATABASE_USER=postgres VIKUNJA_DATABASE_PASSWORD=vikunjatest VIKUNJA_DATABASE_DATABASE=vikunjatest \
		VIKUNJA_DATABASE_SSLMODE=disable VIKUNJA_SERVICE_PUBLICURL=http://127.0.0.1:3456 "$@"
}

# --- e2e services: Mailpit, Dex and a mail-enabled API, as in CI's Playwright job -------------
E2E_PIDS=()
e2e_services_start() {
	docker info >/dev/null 2>&1 || { echo "Docker isn't running"; return 1; }
	docker rm -f lci-mailpit lci-dex >/dev/null 2>&1
	docker run -d --name lci-mailpit -p 127.0.0.1:$MAILPIT_SMTP:1025 -p 127.0.0.1:$MAILPIT_HTTP:8025 "$MAILPIT_IMAGE" >/dev/null || return 1
	# CI reaches Dex as http://dex:5556; the Mac can't resolve "dex", so the same config with a 127.0.0.1 issuer.
	docker run --rm --platform linux/amd64 --entrypoint cat "$DEX_IMAGE" /etc/dex/config.docker.yaml |
		sed 's#^issuer: http://dex:5556#issuer: http://127.0.0.1:5556#' >"$OUT/dex-config.yaml" || return 1
	docker run -d --name lci-dex --platform linux/amd64 -p 127.0.0.1:5556:5556 \
		-v "$OUT/dex-config.yaml:/etc/dex/local.yaml:ro" "$DEX_IMAGE" dex serve /etc/dex/local.yaml >/dev/null || return 1
	local mroot; mroot=$(mktemp -d); mkdir -p "$mroot/files"
	VIKUNJA_SERVICE_INTERFACE=127.0.0.1:3457 VIKUNJA_SERVICE_PUBLICURL=http://127.0.0.1:4173/ \
		VIKUNJA_SERVICE_ROOTPATH="$mroot" VIKUNJA_FILES_BASEPATH="$mroot/files" \
		VIKUNJA_DATABASE_TYPE=sqlite VIKUNJA_DATABASE_PATH=memory VIKUNJA_AUTH_OPENID_ENABLED=0 VIKUNJA_REDIS_ENABLED=0 \
		VIKUNJA_MAILER_ENABLED=1 VIKUNJA_MAILER_HOST=127.0.0.1 VIKUNJA_MAILER_PORT=$MAILPIT_SMTP VIKUNJA_MAILER_FORCESSL=0 \
		VIKUNJA_SERVICE_TESTINGTOKEN=$TEST_SECRET VIKUNJA_RATELIMIT_NOAUTHLIMIT=1000 VIKUNJA_CORS_ENABLE=1 \
		./vikunja web >"$OUT/e2e-mailer-api.log" 2>&1 &
	E2E_PIDS+=($!)
	for _ in $(seq 1 60); do
		curl -fs http://127.0.0.1:3457/api/v1/info >/dev/null && curl -fs http://127.0.0.1:$MAILPIT_HTTP/readyz >/dev/null &&
			curl -fs http://127.0.0.1:5556/.well-known/openid-configuration >/dev/null && return 0
		sleep 2
	done
	echo "e2e services didn't come up"; return 1
}
e2e_services_stop() {
	for p in "${E2E_PIDS[@]:-}"; do [ -n "$p" ] && kill "$p" 2>/dev/null; done
	docker rm -f lci-mailpit lci-dex >/dev/null 2>&1 || true
}
e2e_run() {
	e2e_services_start || { e2e_services_stop; return 1; }
	# CI=1 gives Playwright the same retries (2) and settings as the CI job.
	CI=1 VIKUNJA_E2E_FRONTEND_PORT=4173 VIKUNJA_E2E_SKIP_BUILD=true VIKUNJA_E2E_TESTING_TOKEN=$TEST_SECRET TEST_SECRET=$TEST_SECRET \
		MAILER_API_URL=http://127.0.0.1:3457/api/v1 MAILPIT_URL=http://127.0.0.1:$MAILPIT_HTTP \
		VIKUNJA_AUTH_OPENID_ENABLED=1 VIKUNJA_AUTH_OPENID_PROVIDERS_DEX_NAME=Dex \
		VIKUNJA_AUTH_OPENID_PROVIDERS_DEX_AUTHURL=http://127.0.0.1:5556 VIKUNJA_AUTH_OPENID_PROVIDERS_DEX_CLIENTID=vikunja \
		VIKUNJA_AUTH_OPENID_PROVIDERS_DEX_CLIENTSECRET=secret \
		go tool mage test:e2e ""
	local rc=$?
	e2e_services_stop
	# Playwright's "Desktop Chrome" device reports a Windows user agent, so the app (isAppleDevice reads the
	# UA) expects Ctrl; on a Mac host ControlOrMeta sends Cmd. CI runs on Linux, where it sends Ctrl.
	# Only these exact tests may fail; any other failure fails the step.
	if [ $rc -ne 0 ]; then
		local failed unexpected
		failed=$(awk '/^ +[0-9]+ failed$/{f=1;next} f&&/^ +[0-9]+ (flaky|passed|skipped|did not run|interrupted)/{exit} f{print}' "$OUT/e2e-playwright.log" | sed -E 's/^ +\[chromium\] › //; s/ › .*//')
		unexpected=$(echo "$failed" | grep -v -x -F -f <(printf '%s\n' "${KNOWN_MAC_E2E_FAILURES[@]}") | grep -v '^$')
		if [ -n "$failed" ] && [ -z "$unexpected" ]; then
			echo "$failed" > "$OUT/e2e-known-mac-failures.txt"
			return 0
		fi
	fi
	return $rc
}

cleanup() { e2e_services_stop; pg_stop; }
trap cleanup EXIT

echo "local-ci on $SHA ($(git branch --show-current)), logs in $OUT"
echo "pnpm $(pnpm -v), go $(go env GOVERSION), golangci-lint $(golangci-lint version --short 2>/dev/null || golangci-lint --version | awk '{print $4}')"

# --- build ----------------------------------------------------------------------------------
# CI starts from a fresh checkout. A stale vitest/vite transform cache here once served old
# locale messages and failed new i18n strings, so start the frontend without one.
rm -rf frontend/node_modules/.vitest-cache frontend/node_modules/.vite
step frontend-install -               bash -c 'cd frontend && pnpm install --frozen-lockfile'
step frontend-build -                 bash -c 'cd frontend && pnpm build'
step api-build -                      go tool mage build

# --- generators and Go lint -----------------------------------------------------------------
step check-translations -             go tool mage check:translations
step check-frontend-client -          go tool mage check:frontend-client
step check-yaegi-symbols -            go tool mage check:yaegi-symbols
step check-swagger -                  go tool mage check:got-swag
step golangci-lint lint_counts        golangci-lint run
step golangci-lint-veans lint_counts  bash -c 'cd veans && golangci-lint run'

# --- frontend checks, then two lanes in parallel: Go suites on SQLite and on Postgres 16 --------------
# (They share nothing but the build cache. e2e runs alone afterwards: it's timing-sensitive under load.)
lane_sqlite() {
	step sqlite-feature go_counts     go tool mage test:feature
	step sqlite-web go_counts         go tool mage test:web
	step sqlite-caldav go_counts      go tool mage test:caldav
	step sqlite-e2e-api go_counts     go tool mage test:e2e-api
	step sqlite-f9 f9_counts          go tool mage -v test:filter TestProjectPermissionMatrix
}
lane_pg() {
	if want pg-feature || want pg-web || want pg-caldav || want pg-e2e-api || want pg-f9; then
		if pg_start; then
			step pg-feature go_counts     on_pg go tool mage test:feature
			step pg-web go_counts         on_pg go tool mage test:web
			step pg-caldav go_counts      on_pg go tool mage test:caldav
			step pg-e2e-api go_counts     on_pg go tool mage test:e2e-api
			step pg-f9 f9_counts          on_pg go tool mage -v test:filter TestProjectPermissionMatrix
			pg_stop
		else
			echo "| postgres | **FAIL** | | couldn't start Postgres 16 (see $OUT/postgres.log) |" >"$ROWS/postgres"; touch "$ROWS/postgres.failed"
		fi
	fi
}
lane_frontend() {
	step frontend-lint eslint_counts      bash -c 'cd frontend && pnpm lint'
	step frontend-stylelint -             bash -c 'cd frontend && pnpm lint:styles'
	step frontend-typecheck -             bash -c 'cd frontend && pnpm typecheck'
	step frontend-unit vitest_counts      bash -c 'cd frontend && pnpm exec vitest run --dir ./src'
}
# The frontend checks take about a minute and vitest's module loading flaked under the Go lanes' load,
# so they run first, then the two Go lanes in parallel.
lane_frontend
lane_sqlite & LANE1=$!
lane_pg & LANE2=$!
wait $LANE1 $LANE2

# --- Playwright e2e (needs the api-build binary) --------------------------------------------
step e2e-playwright pw_counts         e2e_run

TOTAL=$(( $(date +%s) - START_ALL ))
{
	echo "### Local CI on \`$SHA\`"
	echo
	echo "Branch \`$(git branch --show-current)\`, $(date '+%Y-%m-%d %H:%M %Z'), $((TOTAL / 60)) min. Mac (arm64), go $(go env GOVERSION | sed 's/go//'), pnpm $(pnpm -v), Postgres 16 ($("$PG_BIN/postgres" --version | awk '{print $3}')), golangci-lint $(golangci-lint version --short 2>/dev/null), Playwright $(cd frontend && pnpm exec playwright --version 2>/dev/null | awk '/^Version/{print $2}')."
	[ -n "$ONLY" ] && echo && echo "**Partial run** (\`LOCAL_CI_ONLY=$ONLY\`): not valid for a merge."
	echo
	echo "| Step | Result | Time | Detail |"
	echo "|---|---|---|---|"
	for n in frontend-install frontend-build api-build check-translations check-frontend-client check-yaegi-symbols \
		check-swagger golangci-lint golangci-lint-veans sqlite-feature sqlite-web sqlite-caldav sqlite-e2e-api sqlite-f9 \
		postgres pg-feature pg-web pg-caldav pg-e2e-api pg-f9 frontend-lint frontend-stylelint frontend-typecheck \
		frontend-unit e2e-playwright; do
		[ -f "$ROWS/$n" ] && cat "$ROWS/$n"
	done
	echo
	if [ -z "$(ls "$ROWS"/*.failed 2>/dev/null)" ]; then echo "**All steps passed.**"; else echo "**Some steps failed.** Logs: \`$OUT\`"; fi
	echo
	echo "Not run locally (CI-only infrastructure): MySQL/MariaDB/ParadeDB suites, test-migration-smoke, test-s3-integration, veans-test, test-veans-e2e."
} >"$OUT/summary.md"
echo
cat "$OUT/summary.md"
[ -z "$(ls "$ROWS"/*.failed 2>/dev/null)" ]
