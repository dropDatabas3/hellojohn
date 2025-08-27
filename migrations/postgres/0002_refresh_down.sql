-- Bajar índices en orden seguro
DROP INDEX IF EXISTS idx_refresh_expires_at;
DROP INDEX IF EXISTS idx_refresh_active;
DROP INDEX IF EXISTS idx_refresh_user_client;
DROP INDEX IF EXISTS ux_refresh_token_hash;

-- Bajar tabla
DROP TABLE IF EXISTS refresh_token;

-- Quitar la constraint de unicidad si existiera
ALTER TABLE identity
  DROP CONSTRAINT IF EXISTS ux_identity_user_provider;
