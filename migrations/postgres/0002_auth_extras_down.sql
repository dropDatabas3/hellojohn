BEGIN;

-- Admin disable fields
ALTER TABLE app_user DROP COLUMN IF EXISTS disabled_reason;
ALTER TABLE app_user DROP COLUMN IF EXISTS disabled_at;

-- Consents
DROP INDEX IF EXISTS idx_user_consent_scopes_gin;
DROP INDEX IF EXISTS idx_user_consent_client_active;
DROP INDEX IF EXISTS idx_user_consent_user_active;
DROP TABLE IF EXISTS user_consent;

-- Scopes
DROP INDEX IF EXISTS idx_scope_tenant_name;
DROP TABLE IF EXISTS scope;

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
