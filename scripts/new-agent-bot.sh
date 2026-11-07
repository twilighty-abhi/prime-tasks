#!/usr/bin/env bash
# prime-tasks A1: create one bot per agent and print its Agent (safe) MCP token.
#
# A thin wrapper around `vikunja bot create` / `vikunja bot revoke`, which run on the server and need no owner
# login (owners sign in with Google and have no password). The token is printed once on stdout by the CLI; this
# script never writes it to a file or a log. Don't pipe the output into a file you keep.
set -euo pipefail

usage() {
	cat <<'EOF'
Usage:
  scripts/new-agent-bot.sh --owner <username-or-id> --name <name> [options]
  scripts/new-agent-bot.sh --revoke --owner <username-or-id> --name <name> [options]

Creates the bot user bot-<name> owned by --owner, issues an MCP token with the Agent (safe) permissions and
prints it once, with the MCP endpoint and a ready `claude mcp add` command. --revoke deletes every token of
bot-<name> and disables the bot instead.

Options:
  --owner <user>      Username or id of the person who owns the bot (required).
  --name <name>       Agent name; the bot user is bot-<name> (required).
  --expires <when>    Token expiry: a lifetime (90d, 36h) or a date (2027-01-31). Default 90d.
  --title <title>     Token title. Default "bot-<name> agent".
  --reuse             The bot already exists: re-enable it and issue a new token.
  --revoke            Delete the bot's tokens and disable it.

Where the CLI runs (default: inside the deploy stack):
  --docker            docker compose -f deploy/docker-compose.yml exec -T prime-tasks /app/vikunja/vikunja ...
  --compose-file <f>  Compose file for --docker. Default deploy/docker-compose.yml (next to this script).
  --local             Run a local vikunja binary instead.
  --binary <path>     Binary for --local. Default ./vikunja, else run/vikunja.
  --config <path>     Config file for --local (passed as --config).
  -h, --help          Show this help.

Examples:
  scripts/new-agent-bot.sh --owner abhiram --name claude-laptop
  scripts/new-agent-bot.sh --owner abhiram --name claude-laptop --reuse --expires 30d
  scripts/new-agent-bot.sh --revoke --owner abhiram --name claude-laptop
  scripts/new-agent-bot.sh --local --binary run/vikunja --config run/config.yml --owner alice --name test
EOF
}

die() {
	echo "new-agent-bot: $*" >&2
	echo "Run with --help for usage." >&2
	exit 2
}

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
mode=docker
compose_file="$script_dir/../deploy/docker-compose.yml"
binary=""
config=""
owner=""
name=""
expires=""
title=""
reuse=false
revoke=false

while [[ $# -gt 0 ]]; do
	case "$1" in
		--owner) [[ $# -ge 2 ]] || die "--owner needs a value"; owner="$2"; shift 2 ;;
		--name) [[ $# -ge 2 ]] || die "--name needs a value"; name="$2"; shift 2 ;;
		--expires) [[ $# -ge 2 ]] || die "--expires needs a value"; expires="$2"; shift 2 ;;
		--title) [[ $# -ge 2 ]] || die "--title needs a value"; title="$2"; shift 2 ;;
		--reuse) reuse=true; shift ;;
		--revoke) revoke=true; shift ;;
		--docker) mode=docker; shift ;;
		--compose-file) [[ $# -ge 2 ]] || die "--compose-file needs a value"; compose_file="$2"; shift 2 ;;
		--local) mode=local; shift ;;
		--binary) [[ $# -ge 2 ]] || die "--binary needs a value"; binary="$2"; mode=local; shift 2 ;;
		--config) [[ $# -ge 2 ]] || die "--config needs a value"; config="$2"; shift 2 ;;
		-h | --help) usage; exit 0 ;;
		*) die "unknown argument: $1" ;;
	esac
done

[[ -n "$owner" ]] || die "--owner is required"
[[ -n "$name" ]] || die "--name is required"

if $revoke; then
	if [[ -n "$expires" || -n "$title" ]] || $reuse; then
		die "--revoke takes only --owner and --name"
	fi
	args=(bot revoke --owner "$owner" --name "$name")
else
	args=(bot create --owner "$owner" --name "$name")
	[[ -n "$expires" ]] && args+=(--expires "$expires")
	[[ -n "$title" ]] && args+=(--title "$title")
	$reuse && args+=(--reuse)
fi

case "$mode" in
	docker)
		[[ -z "$config" ]] || die "--config only applies to --local; the container reads /etc/vikunja/config.yml"
		[[ -f "$compose_file" ]] || die "compose file not found: $compose_file"
		exec docker compose -f "$compose_file" exec -T prime-tasks /app/vikunja/vikunja "${args[@]}"
		;;
	local)
		if [[ -z "$binary" ]]; then
			if [[ -x ./vikunja ]]; then
				binary=./vikunja
			elif [[ -x run/vikunja ]]; then
				binary=run/vikunja
			else
				die "no vikunja binary at ./vikunja or run/vikunja; pass --binary"
			fi
		fi
		[[ -x "$binary" ]] || die "not executable: $binary"
		if [[ -n "$config" ]]; then
			exec "$binary" --config "$config" "${args[@]}"
		fi
		exec "$binary" "${args[@]}"
		;;
esac
