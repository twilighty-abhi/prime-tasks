# Deploying prime-tasks on the office VM

Three containers: `prime-tasks` (API and frontend in one binary), `db` (Postgres 16) and `caddy` (TLS on 80/443).
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
git clone -b prime https://github.com/twilighty-abhi/prime-tasks.git /opt/prime-tasks
cd /opt/prime-tasks/deploy
cp .env.example .env
# fill in PRIME_DOMAIN, ACME_EMAIL, PRIME_TASKS_VERSION, and two `openssl rand -hex 32` values
chmod 600 .env
docker compose pull          # or `docker compose build` to build from this checkout
docker compose up -d
docker compose logs -f prime-tasks   # wait for "HTTP server listening"
```

Registration is off. Create the first account and make it an instance admin:

```bash
docker compose exec prime-tasks /app/vikunja/vikunja user create -u <username> -e <email>
docker compose exec prime-tasks /app/vikunja/vikunja user set-admin <username>
```

Then install the nightly backup (see [Backups](#backups)).

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
3. Set `PRIME_TASKS_VERSION` in `.env` to the new tag (without the `prime-v` prefix), then:

```bash
cd /opt/prime-tasks/deploy
git pull                       # picks up changes to config.yml, Caddyfile, compose
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
