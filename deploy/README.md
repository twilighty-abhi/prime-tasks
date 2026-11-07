# Deploying prime-tasks on the office VM

Four services (one runs once to set file permissions): `prime-tasks` (API and frontend in one binary), `db`
(Postgres 16), `caddy` (TLS on 80/443) and `files-permissions` (chowns the files volume for the app, then exits).
Volumes: `prime-tasks-pgdata` (database), `prime-tasks-files` (attachments, avatars, backgrounds),
`prime-tasks-caddy-data` (certificates).

Sizing for 20-30 people plus a few agent bots: 2 vCPU, 4 GB RAM, 40 GB disk. Most of the disk is attachments
(capped at 50 MB per file in `config.yml`), so watch `docker system df -v` and grow the disk when it gets past 70%.

## First install

Prerequisites: a Linux VM with Docker Engine and the compose plugin, a DNS name pointing at it, and ports 80
and 443 reachable (Caddy needs them to get a Let's Encrypt certificate). For a tailnet-only or LAN-only name,
replace the site block in `Caddyfile` with `{$PRIME_DOMAIN} { tls internal ... }`.

```bash
sudo mkdir -p /opt/prime-tasks && sudo chown "$USER" /opt/prime-tasks
git clone https://github.com/twilighty-abhi/prime-tasks.git /opt/prime-tasks
cd /opt/prime-tasks
git checkout prime-v2.8.0      # the release tag to run (see PRIME.md, "Releases")
cd deploy
cp .env.example .env
# fill in PRIME_DOMAIN, ACME_EMAIL, PRIME_GOOGLE_CLIENT_ID, PRIME_GOOGLE_CLIENT_SECRET (all required; see
# "Google sign-in" below), PRIME_TASKS_VERSION=2.8.0, and two `openssl rand -hex 32` values
chmod 600 .env
docker compose build --build-arg RELEASE_VERSION=prime-v2.8.0
docker compose up -d
docker compose logs -f prime-tasks   # wait for "HTTP server listening"
```

**Image: build from a tag (for now).** GitHub Actions is unavailable on the fork, so `prime-release.yml` doesn't
publish images to GHCR yet. Until it does, the VM builds the image itself from a release tag:
`git checkout prime-v2.8.0 && docker compose build`. Building takes a while (it compiles the frontend and the API);
`--build-arg RELEASE_VERSION=<tag>` makes the app report the right version, as the release workflow does.

**Image: pull (once releases publish to GHCR).** When `prime-release.yml` has pushed the tag's image to
`ghcr.io/twilighty-abhi/prime-tasks` (see PRIME.md, "When Actions is back"), set `PRIME_TASKS_VERSION` to the
release (for example `2.8.0` for tag `prime-v2.8.0`) and use `docker compose pull` instead of building.

### Accounts, teams and sharing (no license needed)

prime-tasks runs without a Vikunja license, so the instance admin panel and `vikunja user set-admin` are not
available. Nothing below needs them.

- **Accounts.** Registration is off. People sign in with Google (see [Google sign-in](#google-sign-in)); the
  first sign-in creates their account. For an account without Google (an agent bot, a contractor), create it on
  the VM:

  ```bash
  docker compose exec prime-tasks /app/vikunja/vikunja user create -u <username> -e <email>
  ```

  It asks for a password (or pass `-p`). That person logs in under **Other ways to sign in** on the login page.
  If an admin-created account's email is later used with Google, Google sign-in logs in to that same account.
- **Teams.** New users join their teams automatically by email domain (F6): list the domains and team ids under
  `service.autojointeams` in `config.yml` and restart the app. This applies to `user create`, Google sign-in and
  registration alike. Whoever creates a team is its admin and can add or remove members on the Teams page.
- **Sharing.** Each project's owner (and anyone they give admin access) manages who can see it, from **Share** in
  the project header. With `service.orgdirectory` on, everyone can find everyone by name.
- **Passwords.** The mailer is off, so there are no reset emails. To reset a password, find the user's id and set
  a new one:

  ```bash
  docker compose exec prime-tasks /app/vikunja/vikunja user list
  docker compose exec prime-tasks /app/vikunja/vikunja user reset-password <user id> --direct
  ```

  It asks for the new password (or pass `-p`).

Then install the nightly backup (see [Backups](#backups)).

## Google sign-in

The login page leads with **Sign in with Google**; username and password sit under **Other ways to sign in**
(`auth.local.foldonlogin` in `config.yml`). The first Google sign-in creates the account, `service.autojointeams`
adds it to the teams for its email domain, and later sign-ins reuse it.

Redirect URI to register at Google (the frontend sends `<address in the browser>/auth/openid/google`, so it must be
the address people actually open, the same one as `PRIME_DOMAIN`):

| Where | Authorized redirect URI | Authorized JavaScript origin |
|---|---|---|
| Office VM (`PRIME_DOMAIN=tasks.example.org`) | `https://tasks.example.org/auth/openid/google` | `https://tasks.example.org` |
| Local test on the Mac | `http://localhost:3456/auth/openid/google` | `http://localhost:3456` |

In [Google Cloud Console](https://console.cloud.google.com/):

1. Pick or create a project. **APIs & Services → OAuth consent screen**: user type **Internal** if everyone has an
   account in the office's Google Workspace (only they can sign in), otherwise **External**; add the `openid`,
   `email` and `profile` scopes.
2. **APIs & Services → Credentials → Create credentials → OAuth client ID**, type **Web application**. Add the
   redirect URI and origin from the table above (both rows if you also test locally).
3. Copy the client ID and secret into `.env` as `PRIME_GOOGLE_CLIENT_ID` and `PRIME_GOOGLE_CLIENT_SECRET`, then
   `docker compose up -d`. `docker-compose.yml` hands them to the app as
   `VIKUNJA_AUTH_OPENID_PROVIDERS_GOOGLE_CLIENTID` / `_CLIENTSECRET`; `config.yml` holds the rest of the provider.

The stack refuses to start while either variable is empty. Google answers `redirect_uri_mismatch` when the address in
the browser differs from the registered URI (`http` vs `https`, `www.`, a port).

## Migrating from an existing Vikunja

prime-tasks runs Vikunja's normal migrations on startup. The source instance must be on the same or an
older Vikunja version than the one prime-tasks is based on (see PRIME.md). Downgrades are not supported.
Stop the old instance before you start, so nothing writes to it during the copy.

### From Postgres (keep the data, move it into this stack)

```bash
# on the old host
pg_dump -h <old-host> -U <old-user> -d <old-db> -Fc > vikunja-old.dump
tar -czf files-old.tar.gz -C <old files dir> .

# on the VM, with the stack created but the app stopped
cd /opt/prime-tasks/deploy
docker compose up -d db files-permissions
docker compose stop prime-tasks
docker compose exec -T db pg_restore -U vikunja -d vikunja --no-owner --clean --if-exists < vikunja-old.dump
docker run --rm -i -v prime-tasks-files:/files busybox:1.37 tar -xzf - -C /files < files-old.tar.gz
docker compose run --rm files-permissions
docker compose up -d
```

To keep using an existing Postgres server in place instead of the bundled one, remove the `db` service and its
`depends_on` entry, then set `database.host`, `database.user` and `database.database` in `config.yml` (password via
`POSTGRES_PASSWORD` in `.env`). Take a `pg_dump` first: the first start runs migrations against that database.

### From SQLite or MySQL (`vikunja dump` / `vikunja restore`)

```bash
# on the old host (adjust for how it runs; this writes a zip with db, files and config)
vikunja dump --path /tmp
# or in Docker: docker exec <old-container> /app/vikunja/vikunja dump --path /tmp && docker cp <old-container>:/tmp/<zip> .

# on the VM: start a fresh, empty stack, then restore into it
cd /opt/prime-tasks/deploy
docker compose up -d
docker compose cp vikunja-dump_<date>.zip prime-tasks:/tmp/dump.zip
docker compose exec -it prime-tasks /app/vikunja/vikunja restore --preserve-config /tmp/dump.zip
docker compose restart prime-tasks
```

`--preserve-config` keeps this folder's `config.yml` instead of the old instance's. The restore asks for
confirmation and needs an empty database.

After any migration: check that people can log in, open a few projects with attachments, and then decide about
`sql/flip-discoverable.sql` (run it only after a `pg_dump`, and only after telling the team).

## Upgrades

1. Read PRIME.md for the new `prime-v*` tag and any config flag changes.
2. Back up: `./backup.sh`.
3. Set `PRIME_TASKS_VERSION` in `.env` to the new tag (without the `prime-v` prefix), then, until releases publish
   to GHCR, build from the tag:

```bash
cd /opt/prime-tasks
git fetch --tags
git checkout prime-vX.Y.Z      # the new release tag; also picks up config.yml, Caddyfile and compose changes
cd deploy
docker compose build --build-arg RELEASE_VERSION=prime-vX.Y.Z
docker compose up -d
docker compose logs -f prime-tasks   # migrations run here
```

   Once releases publish to GHCR, pull instead of building:

```bash
cd /opt/prime-tasks && git fetch --tags && git checkout prime-vX.Y.Z && cd deploy
docker compose pull
docker compose up -d
docker compose logs -f prime-tasks   # migrations run here
```

Rolling back means restoring the pre-upgrade backup, because migrations aren't reversible.

## Backups

`backup.sh` writes `db-<stamp>.dump` (pg_dump custom format) and `files-<stamp>.tar.gz` to
`/var/backups/prime-tasks` and deletes anything older than 14 days (`BACKUP_DIR` and `KEEP_DAYS` override).

```bash
sudo mkdir -p /var/backups/prime-tasks && sudo chown "$USER" /var/backups/prime-tasks
crontab -e
# 30 2 * * * /opt/prime-tasks/deploy/backup.sh >> /var/log/prime-tasks-backup.log 2>&1
```

Copy the backup folder off the VM too (rsync, restic, or whatever the office already uses). A backup that only
lives on the same disk doesn't survive losing the VM.

### Restore

```bash
cd /opt/prime-tasks/deploy
docker compose stop prime-tasks
docker compose exec -T db pg_restore -U vikunja -d vikunja --clean --if-exists --no-owner < /var/backups/prime-tasks/db-<stamp>.dump
docker run --rm -v prime-tasks-files:/files busybox:1.37 sh -c 'rm -rf /files/* /files/.[!.]*' 
docker run --rm -i -v prime-tasks-files:/files busybox:1.37 tar -xzf - -C /files < /var/backups/prime-tasks/files-<stamp>.tar.gz
docker compose run --rm files-permissions
docker compose up -d
```

Test a restore on a scratch VM once after setup, and again every few months.
