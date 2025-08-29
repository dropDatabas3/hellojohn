-- 003_email_flows_up.sql
-- Tablas para verificación de email y reset de contraseña

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Email verification tokens
CREATE TABLE IF NOT EXISTS email_verification_token (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID        NOT NULL,
    user_id     UUID        NOT NULL,
    token_hash  BYTEA       NOT NULL UNIQUE,   -- sha256 crudo
    sent_to     TEXT        NOT NULL,
    ip          INET,
    user_agent  TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at  TIMESTAMPTZ NOT NULL,
    used_at     TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_email_verif_token_expires_at
  ON email_verification_token (expires_at);

CREATE INDEX IF NOT EXISTS idx_email_verif_token_tenant_user
  ON email_verification_token (tenant_id, user_id);

-- Password reset tokens
CREATE TABLE IF NOT EXISTS password_reset_token (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID        NOT NULL,
    user_id     UUID        NOT NULL,
    token_hash  BYTEA       NOT NULL UNIQUE,   -- sha256 crudo
    sent_to     TEXT        NOT NULL,
    ip          INET,
    user_agent  TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at  TIMESTAMPTZ NOT NULL,
    used_at     TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_pwd_reset_token_expires_at
  ON password_reset_token (expires_at);

CREATE INDEX IF NOT EXISTS idx_pwd_reset_token_tenant_user
  ON password_reset_token (tenant_id, user_id);

-- Flag de verificación en app_user (idempotente)
ALTER TABLE app_user
  ADD COLUMN IF NOT EXISTS email_verified BOOLEAN NOT NULL DEFAULT FALSE;
