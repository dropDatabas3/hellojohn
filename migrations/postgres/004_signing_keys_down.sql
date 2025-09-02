BEGIN;

DROP INDEX IF EXISTS ix_signing_keys_not_before;
DROP INDEX IF EXISTS ix_signing_keys_status;
DROP INDEX IF EXISTS ux_signing_keys_active;

DROP TABLE IF EXISTS signing_keys;

COMMIT;
