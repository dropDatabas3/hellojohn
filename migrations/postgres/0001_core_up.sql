BEGIN;

-- Extensiones
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- =========================
-- Tenants / Clients / Versions
-- =========================
CREATE TABLE IF NOT EXISTS tenant (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name        TEXT NOT NULL,
  slug        TEXT UNIQUE NOT NULL,
  settings    JSONB DEFAULT '{}'::jsonb,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS client (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id        UUID NOT NULL REFERENCES tenant(id) ON DELETE CASCADE,
  name             TEXT NOT NULL,
  client_id        TEXT UNIQUE NOT NULL,
  client_type      TEXT NOT NULL CHECK (client_type IN ('public','confidential')),
  redirect_uris    TEXT[] NOT NULL DEFAULT '{}',
  allowed_origins  TEXT[] NOT NULL DEFAULT '{}',
  providers        TEXT[] NOT NULL DEFAULT '{}',
  scopes           TEXT[] NOT NULL DEFAULT '{}',
  active_version_id UUID, -- FK se agrega abajo (necesita client_version creada)
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS client_version (
  id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  client_id          UUID NOT NULL REFERENCES client(id) ON DELETE CASCADE,
  version            TEXT NOT NULL,
  claim_schema_json  JSONB NOT NULL,
  claim_mapping_json JSONB NOT NULL,
  crypto_config_json JSONB NOT NULL,
  status             TEXT NOT NULL CHECK (status IN ('draft','active','retired')),
  created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
  promoted_at        TIMESTAMPTZ
);

-- FK client.active_version_id -> client_version(id) si no existe
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM information_schema.table_constraints
    WHERE constraint_name = 'fk_client_active_version'
      AND table_name = 'client'
  ) THEN
    ALTER TABLE client
      ADD CONSTRAINT fk_client_active_version
      FOREIGN KEY (active_version_id)
      REFERENCES client_version(id)
      ON DELETE SET NULL;
  END IF;
END$$;

-- =========================
-- Users / Identities
-- =========================
CREATE TABLE IF NOT EXISTS app_user (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id      UUID NOT NULL REFERENCES tenant(id) ON DELETE CASCADE,
  email          TEXT NOT NULL,
  email_verified BOOLEAN NOT NULL DEFAULT false,
  status         TEXT NOT NULL DEFAULT 'active',
  metadata       JSONB NOT NULL DEFAULT '{}',
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, email)
);

CREATE TABLE IF NOT EXISTS identity (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id          UUID NOT NULL REFERENCES app_user(id) ON DELETE CASCADE,
  provider         TEXT NOT NULL CHECK (provider IN ('password','google','facebook')),
  provider_user_id TEXT,
  email            TEXT,
  email_verified   BOOLEAN,
  password_hash    TEXT,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_identity_user ON identity(user_id);
CREATE INDEX IF NOT EXISTS idx_identity_provider_uid ON identity(provider, provider_user_id);

-- Unicidad (user_id, provider)
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'ux_identity_user_provider'
  ) THEN
    ALTER TABLE identity
      ADD CONSTRAINT ux_identity_user_provider UNIQUE (user_id, provider);
  END IF;
END$$;

-- =========================
-- Refresh Tokens
-- =========================
CREATE TABLE IF NOT EXISTS refresh_token (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id      UUID NOT NULL REFERENCES app_user(id) ON DELETE CASCADE,
  client_id    UUID NOT NULL REFERENCES client(id) ON DELETE CASCADE,
  token_hash   TEXT NOT NULL,
  issued_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at   TIMESTAMPTZ NOT NULL,
  rotated_from UUID NULL REFERENCES refresh_token(id) ON DELETE SET NULL,
  revoked_at   TIMESTAMPTZ NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_refresh_token_hash
  ON refresh_token(token_hash);

CREATE INDEX IF NOT EXISTS idx_refresh_user_client
  ON refresh_token(user_id, client_id);

-- Índice parcial de “activos”
CREATE INDEX IF NOT EXISTS idx_refresh_active
  ON refresh_token(user_id)
  WHERE revoked_at IS NULL;

-- Para limpieza de expirados
CREATE INDEX IF NOT EXISTS idx_refresh_expires_at
  ON refresh_token(expires_at)
  WHERE revoked_at IS NULL;

-- =========================
-- Email flows (verify / reset)
-- =========================
CREATE TABLE IF NOT EXISTS email_verification_token (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id   UUID NOT NULL,
  user_id     UUID NOT NULL,
  token_hash  BYTEA NOT NULL UNIQUE,  -- sha256 crudo
  sent_to     TEXT  NOT NULL,
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

CREATE TABLE IF NOT EXISTS password_reset_token (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id   UUID NOT NULL,
  user_id     UUID NOT NULL,
  token_hash  BYTEA NOT NULL UNIQUE,  -- sha256 crudo
  sent_to     TEXT  NOT NULL,
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

-- =========================
-- Signing Keys (JWKS)
-- =========================
CREATE TABLE IF NOT EXISTS signing_keys (
  kid         TEXT PRIMARY KEY,
  alg         TEXT NOT NULL CHECK (alg = 'EdDSA'),
  public_key  BYTEA NOT NULL,
  private_key BYTEA,
  status      TEXT NOT NULL CHECK (status IN ('active','retiring','retired')),
  not_before  TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  rotated_at  TIMESTAMPTZ
);

-- Una sola 'active'
CREATE UNIQUE INDEX IF NOT EXISTS ux_signing_keys_active
  ON signing_keys (status)
  WHERE (status = 'active');

CREATE INDEX IF NOT EXISTS ix_signing_keys_status
  ON signing_keys (status);

CREATE INDEX IF NOT EXISTS ix_signing_keys_not_before
  ON signing_keys (not_before);

COMMIT;
