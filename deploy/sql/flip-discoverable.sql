-- prime-tasks Phase 0: make every existing human account discoverable by name and email.
--
-- RUN ONLY AFTER A FRESH pg_dump, AND ONLY AFTER TELLING THE TEAM.
-- It overrides a privacy choice each person may have made in their settings.
--
--   docker compose exec -T db pg_dump -U vikunja -d vikunja -Fc > before-flip.dump
--   docker compose exec -T db psql -U vikunja -d vikunja -v ON_ERROR_STOP=1 < sql/flip-discoverable.sql
--
-- Bot accounts (bot_owner_id set) are left alone.

BEGIN;

UPDATE users SET discoverable_by_name = true, discoverable_by_email = true
WHERE bot_owner_id IS NULL OR bot_owner_id = 0;

COMMIT;
