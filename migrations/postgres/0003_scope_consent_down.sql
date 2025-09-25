-- 0003_scope_consent_down.sql

-- Borrar índices antes de tablas
DROP INDEX IF EXISTS idx_user_consent_scopes_gin;
DROP INDEX IF EXISTS idx_user_consent_client_active;
DROP INDEX IF EXISTS idx_user_consent_user_active;

DROP TABLE IF EXISTS user_consent;

DROP INDEX IF EXISTS idx_scope_tenant_name;

DROP TABLE IF EXISTS scope;
