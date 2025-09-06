-- MFA TOTP por usuario
CREATE TABLE IF NOT EXISTS user_mfa_totp (
  user_id UUID PRIMARY KEY REFERENCES app_user(id) ON DELETE CASCADE,
  secret_encrypted TEXT NOT NULL,
  confirmed_at TIMESTAMPTZ,
  last_used_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE OR REPLACE FUNCTION trg_user_mfa_totp_updated_at()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = NOW();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER user_mfa_totp_updated_at
BEFORE UPDATE ON user_mfa_totp
FOR EACH ROW EXECUTE FUNCTION trg_user_mfa_totp_updated_at();
