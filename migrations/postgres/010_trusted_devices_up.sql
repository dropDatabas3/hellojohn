-- "Remember device" (hash del token del dispositivo)
CREATE TABLE IF NOT EXISTS trusted_device (
  id BIGSERIAL PRIMARY KEY,
  user_id UUID NOT NULL REFERENCES app_user(id) ON DELETE CASCADE,
  device_hash TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS ix_trusted_device_user ON trusted_device(user_id);
CREATE UNIQUE INDEX IF NOT EXISTS ux_trusted_device_user_hash
ON trusted_device(user_id, device_hash);
