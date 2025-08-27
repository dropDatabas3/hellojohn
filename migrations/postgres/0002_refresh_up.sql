-- Unicidad: una sola identidad por (user_id, provider)
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'ux_identity_user_provider'
  ) THEN
    ALTER TABLE identity
      ADD CONSTRAINT ux_identity_user_provider UNIQUE (user_id, provider);
  END IF;
END$$;

-- Tabla de refresh tokens (guardamos SOLO el hash del token)
CREATE TABLE IF NOT EXISTS refresh_token (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id      UUID NOT NULL REFERENCES app_user(id) ON DELETE CASCADE,
  client_id    UUID NOT NULL REFERENCES client(id) ON DELETE CASCADE,
  token_hash   TEXT NOT NULL,                -- p.ej. sha256(base64url) del token opaco
  issued_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at   TIMESTAMPTZ NOT NULL,
  rotated_from UUID NULL REFERENCES refresh_token(id) ON DELETE SET NULL,
  revoked_at   TIMESTAMPTZ NULL
);

-- Índices
CREATE UNIQUE INDEX IF NOT EXISTS ux_refresh_token_hash
  ON refresh_token(token_hash);

CREATE INDEX IF NOT EXISTS idx_refresh_user_client
  ON refresh_token(user_id, client_id);

-- Índice parcial “activos” (sin now() en el predicado -> permitido)
CREATE INDEX IF NOT EXISTS idx_refresh_active
  ON refresh_token(user_id)
  WHERE revoked_at IS NULL;

-- (Opcional pero útil para limpieza) ayuda a borrar expirados eficientemente
CREATE INDEX IF NOT EXISTS idx_refresh_expires_at
  ON refresh_token(expires_at)
  WHERE revoked_at IS NULL;
