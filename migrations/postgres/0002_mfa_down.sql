BEGIN;

-- Borrar en orden seguro
-- Trusted devices
DROP INDEX IF EXISTS ux_trusted_device_user_hash;
DROP INDEX IF EXISTS ix_trusted_device_user;
DROP TABLE IF EXISTS trusted_device;

-- Recovery codes
DROP INDEX IF EXISTS ux_mfa_recovery_code_user_hash;
DROP TABLE IF EXISTS mfa_recovery_code;

-- TOTP
DROP TRIGGER IF EXISTS user_mfa_totp_updated_at ON user_mfa_totp;
DROP FUNCTION IF EXISTS trg_user_mfa_totp_updated_at;
DROP TABLE IF EXISTS user_mfa_totp;

COMMIT;
