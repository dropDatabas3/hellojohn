-- 0003_scope_consent_up.sql
-- Requiere extensión pgcrypto (usada en 0001) para gen_random_uuid().

-- ───── Tabla: scope ─────
CREATE TABLE IF NOT EXISTS scope (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id  UUID NOT NULL REFERENCES tenant(id) ON DELETE CASCADE,
  name       TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, name)
);

CREATE INDEX IF NOT EXISTS idx_scope_tenant_name ON scope(tenant_id, name);

-- ───── Tabla: user_consent ─────
CREATE TABLE IF NOT EXISTS user_consent (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id        UUID NOT NULL REFERENCES app_user(id) ON DELETE CASCADE,
  client_id      UUID NOT NULL REFERENCES client(id)   ON DELETE CASCADE,
  granted_scopes TEXT[] NOT NULL,
  granted_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  revoked_at     TIMESTAMPTZ NULL,
  UNIQUE (user_id, client_id)
);

-- Índices operativos (rápidas búsquedas activas y por scopes)
CREATE INDEX IF NOT EXISTS idx_user_consent_user_active
  ON user_consent(user_id)
  WHERE revoked_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_user_consent_client_active
  ON user_consent(client_id)
  WHERE revoked_at IS NULL;

-- Búsqueda por scopes (intersección)
CREATE INDEX IF NOT EXISTS idx_user_consent_scopes_gin
  ON user_consent USING GIN (granted_scopes);
