BEGIN;

-- =========================
-- MFA TOTP + Recovery + Trusted Devices
-- =========================
CREATE TABLE IF NOT EXISTS user_mfa_totp (
  user_id          UUID PRIMARY KEY REFERENCES app_user(id) ON DELETE CASCADE,
  secret_encrypted TEXT NOT NULL,
  confirmed_at     TIMESTAMPTZ,
  last_used_at     TIMESTAMPTZ,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE OR REPLACE FUNCTION trg_user_mfa_totp_updated_at()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'user_mfa_totp_updated_at') THEN
    CREATE TRIGGER user_mfa_totp_updated_at
    BEFORE UPDATE ON user_mfa_totp
    FOR EACH ROW EXECUTE FUNCTION trg_user_mfa_totp_updated_at();
  END IF;
END$$;

CREATE TABLE IF NOT EXISTS mfa_recovery_code (
  id         BIGSERIAL PRIMARY KEY,
  user_id    UUID NOT NULL REFERENCES app_user(id) ON DELETE CASCADE,
  code_hash  TEXT NOT NULL,
  used_at    TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_mfa_recovery_code_user_hash
  ON mfa_recovery_code(user_id, code_hash);

CREATE TABLE IF NOT EXISTS trusted_device (
  id          BIGSERIAL PRIMARY KEY,
  user_id     UUID NOT NULL REFERENCES app_user(id) ON DELETE CASCADE,
  device_hash TEXT NOT NULL,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at  TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS ix_trusted_device_user ON trusted_device(user_id);
CREATE UNIQUE INDEX IF NOT EXISTS ux_trusted_device_user_hash
  ON trusted_device(user_id, device_hash);


-- =========================
-- Scopes & Consents (requiere pgcrypto para gen_random_uuid())
-- =========================
CREATE TABLE IF NOT EXISTS scope (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id   UUID NOT NULL REFERENCES tenant(id) ON DELETE CASCADE,
  name        TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, name)
);

CREATE INDEX IF NOT EXISTS idx_scope_tenant_name ON scope(tenant_id, name);

CREATE TABLE IF NOT EXISTS user_consent (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id        UUID NOT NULL REFERENCES app_user(id) ON DELETE CASCADE,
  client_id      UUID NOT NULL REFERENCES client(id)   ON DELETE CASCADE,
  granted_scopes TEXT[] NOT NULL,
  granted_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  revoked_at     TIMESTAMPTZ NULL,
  UNIQUE (user_id, client_id)
);

CREATE INDEX IF NOT EXISTS idx_user_consent_user_active
  ON user_consent(user_id)
  WHERE revoked_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_user_consent_client_active
  ON user_consent(client_id)
  WHERE revoked_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_user_consent_scopes_gin
  ON user_consent USING GIN (granted_scopes);


-- =========================
-- Admin: disable/enable user (campos en app_user)
-- =========================
ALTER TABLE app_user ADD COLUMN IF NOT EXISTS disabled_at TIMESTAMPTZ NULL;
ALTER TABLE app_user ADD COLUMN IF NOT EXISTS disabled_reason TEXT NULL;

COMMIT;
