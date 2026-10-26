#!/usr/bin/env bash
# Nightly prime-tasks backup: pg_dump of the database plus a tarball of the files volume.
# Keeps KEEP_DAYS days (default 14). Run from cron on the office VM, e.g.
#   30 2 * * * /opt/prime-tasks/deploy/backup.sh >> /var/log/prime-tasks-backup.log 2>&1
set -euo pipefail

cd "$(dirname "$0")"

BACKUP_DIR="${BACKUP_DIR:-/var/backups/prime-tasks}"
KEEP_DAYS="${KEEP_DAYS:-14}"
FILES_VOLUME="${FILES_VOLUME:-prime-tasks-files}"
STAMP="$(date +%Y%m%d-%H%M%S)"

mkdir -p "$BACKUP_DIR"

# Write to .partial first so a failed run never looks like a good backup.
docker compose exec -T db sh -c 'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc' \
	> "$BACKUP_DIR/db-$STAMP.dump.partial"
mv "$BACKUP_DIR/db-$STAMP.dump.partial" "$BACKUP_DIR/db-$STAMP.dump"

docker run --rm \
	-v "$FILES_VOLUME":/files:ro \
	-v "$BACKUP_DIR":/backup \
	busybox:1.37 \
	tar -czf "/backup/files-$STAMP.tar.gz.partial" -C /files .
mv "$BACKUP_DIR/files-$STAMP.tar.gz.partial" "$BACKUP_DIR/files-$STAMP.tar.gz"

find "$BACKUP_DIR" -maxdepth 1 -type f \( -name 'db-*.dump' -o -name 'files-*.tar.gz' -o -name '*.partial' \) \
	-mtime +"$((KEEP_DAYS - 1))" -delete

echo "$(date -Iseconds) backup ok: db-$STAMP.dump files-$STAMP.tar.gz"
