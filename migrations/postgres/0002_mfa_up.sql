BEGIN;

-- =========================
-- MFA TOTP
-- =========================
CREATE TABLE IF NOT EXISTS user_mfa_totp (
  user_id          UUID PRIMARY KEY REFERENCES app_user(id) ON DELETE CASCADE,
  secret_encrypted TEXT NOT NULL,
  confirmed_at     TIMESTAMPTZ,
  last_used_at     TIMESTAMPTZ,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Función de trigger (idempotente via OR REPLACE)
CREATE OR REPLACE FUNCTION trg_user_mfa_totp_updated_at()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger: sólo si no existe
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'user_mfa_totp_updated_at') THEN
    CREATE TRIGGER user_mfa_totp_updated_at
    BEFORE UPDATE ON user_mfa_totp
    FOR EACH ROW EXECUTE FUNCTION trg_user_mfa_totp_updated_at();
  END IF;
END$$;

-- =========================
-- Recovery Codes
-- =========================
CREATE TABLE IF NOT EXISTS mfa_recovery_code (
  id         BIGSERIAL PRIMARY KEY,
  user_id    UUID NOT NULL REFERENCES app_user(id) ON DELETE CASCADE,
  code_hash  TEXT NOT NULL,
  used_at    TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_mfa_recovery_code_user_hash
  ON mfa_recovery_code(user_id, code_hash);

-- =========================
-- Trusted Devices
-- =========================
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

COMMIT;
