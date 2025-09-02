-- Claves firmantes persistidas para JWT/JWKS (Ed25519)
-- Notas:
-- - En dev se almacena private_key en claro (BYTEA) para simplicidad.
-- - En prod se recomienda cifrar private_key (KMS o envelope encryption) o dejarla NULL si se usa HSM/KMS externo.
-- - Se publica en JWKS toda clave con status IN ('active','retiring').
-- - Sólo puede existir UNA clave con status = 'active' (índice único parcial).

BEGIN;

CREATE TABLE IF NOT EXISTS signing_keys (
    kid           TEXT PRIMARY KEY,                              -- Key ID (header 'kid')
    alg           TEXT NOT NULL CHECK (alg = 'EdDSA'),           -- Algoritmo (EdDSA para Ed25519)
    public_key    BYTEA NOT NULL,                                -- Ed25519 public (32 bytes)
    private_key   BYTEA,                                         -- Ed25519 private (64 bytes); NULL si se gestiona fuera
    status        TEXT NOT NULL CHECK (status IN ('active','retiring','retired')),
    not_before    TIMESTAMPTZ NOT NULL DEFAULT now(),            -- desde cuándo firmar/validar con esta clave
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    rotated_at    TIMESTAMPTZ
);

-- Sólo una 'active' a la vez
CREATE UNIQUE INDEX IF NOT EXISTS ux_signing_keys_active
    ON signing_keys (status)
    WHERE (status = 'active');

-- Acelera JWKS (publicables) y lookups por estado
CREATE INDEX IF NOT EXISTS ix_signing_keys_status
    ON signing_keys (status);

CREATE INDEX IF NOT EXISTS ix_signing_keys_not_before
    ON signing_keys (not_before);

COMMIT;
