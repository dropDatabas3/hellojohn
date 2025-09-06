-- Códigos de recuperación (hash persistido)
CREATE TABLE IF NOT EXISTS mfa_recovery_code (
  id BIGSERIAL PRIMARY KEY,
  user_id UUID NOT NULL REFERENCES app_user(id) ON DELETE CASCADE,
  code_hash TEXT NOT NULL,
  used_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_mfa_recovery_code_user_hash
ON mfa_recovery_code(user_id, code_hash);