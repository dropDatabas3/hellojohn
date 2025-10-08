-- Fase B: agregar tenant_id (uuid) + client_id (text) a entidades del data-plane.
-- NOTA: ajusta nombres si tus tablas difieren (usa \d para verificar).

-- 1) Refresh tokens
ALTER TABLE IF EXISTS refresh_token
  ADD COLUMN IF NOT EXISTS tenant_id uuid,
  ADD COLUMN IF NOT EXISTS client_id text;
CREATE INDEX IF NOT EXISTS idx_refresh_token_tenant_client
  ON refresh_token (tenant_id, client_id);

-- 2) Authorization codes (si aplica)
-- ALTER TABLE IF EXISTS auth_code
--   ADD COLUMN IF NOT EXISTS tenant_id uuid,
--   ADD COLUMN IF NOT EXISTS client_id text;
-- CREATE INDEX IF NOT EXISTS idx_auth_code_tenant_client
--   ON auth_code (tenant_id, client_id);

-- 3) Sessions (si guardás sesión de login en BD)
-- ALTER TABLE IF EXISTS session
--   ADD COLUMN IF NOT EXISTS tenant_id uuid,
--   ADD COLUMN IF NOT EXISTS client_id text;
-- CREATE INDEX IF NOT EXISTS idx_session_tenant_client
--   ON session (tenant_id, client_id);

-- 4) User consents
ALTER TABLE IF EXISTS user_consent
  ADD COLUMN IF NOT EXISTS tenant_id uuid,
  ADD COLUMN IF NOT EXISTS client_id text;
-- Mantén tu PK/UK existente y agrega índice de consulta típica:
CREATE INDEX IF NOT EXISTS idx_user_consent_tenant_client_user
  ON user_consent (tenant_id, client_id, user_id);

-- 5) (Opcional) Trusted devices / MFA binds, si referencian client
ALTER TABLE IF EXISTS trusted_device
  ADD COLUMN IF NOT EXISTS tenant_id uuid,
  ADD COLUMN IF NOT EXISTS client_id text;

-- 6) Backfill mínimo (monotenant):
--    Si tu entorno actual es mono-tenant (p.ej. 'local'), puedes setear un valor estable.
--    Reemplaza el UUID por el de tu tenant 'local' actual (está en ./data/.../tenant.yaml).
--    Si prefieres, salta este bloque y haz el backfill con un comando/cron aparte.
UPDATE refresh_token   SET tenant_id = 'b7268f99-606d-469f-9f77-aaaaaaaaaaaa' WHERE tenant_id IS NULL;
-- UPDATE auth_code       SET tenant_id = 'b7268f99-606d-469f-9f77-aaaaaaaaaaaa' WHERE tenant_id IS NULL;
-- UPDATE session         SET tenant_id = 'b7268f99-606d-469f-9f77-aaaaaaaaaaaa' WHERE tenant_id IS NULL;
UPDATE user_consent    SET tenant_id = 'b7268f99-606d-469f-9f77-aaaaaaaaaaaa' WHERE tenant_id IS NULL;
UPDATE trusted_device  SET tenant_id = 'b7268f99-606d-469f-9f77-aaaaaaaaaaaa' WHERE tenant_id IS NULL;

-- client_id: si no lo tenían, puedes setear el del flujo principal (p.ej. web-frontend)
-- o dejarlo NULL y que el código lo empiece a grabar hacia adelante.
