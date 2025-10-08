-- Reversión (deja las tablas como estaban)
DROP INDEX IF EXISTS idx_refresh_tokens_tenant_client;
DROP INDEX IF EXISTS idx_auth_codes_tenant_client;
DROP INDEX IF EXISTS idx_sessions_tenant_client;
DROP INDEX IF EXISTS idx_user_consents_tenant_client_user;

ALTER TABLE IF EXISTS refresh_tokens  DROP COLUMN IF EXISTS tenant_id, DROP COLUMN IF EXISTS client_id;
ALTER TABLE IF EXISTS auth_codes      DROP COLUMN IF EXISTS tenant_id, DROP COLUMN IF EXISTS client_id;
ALTER TABLE IF EXISTS sessions        DROP COLUMN IF EXISTS tenant_id, DROP COLUMN IF EXISTS client_id;
ALTER TABLE IF EXISTS user_consents   DROP COLUMN IF EXISTS tenant_id, DROP COLUMN IF EXISTS client_id;
ALTER TABLE IF EXISTS trusted_devices DROP COLUMN IF EXISTS tenant_id, DROP COLUMN IF EXISTS client_id;
