ALTER TABLE app_user DROP COLUMN IF EXISTS email_verified;

DROP TABLE IF EXISTS password_reset_token;
DROP TABLE IF EXISTS email_verification_token;
