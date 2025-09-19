BEGIN;

-- Al caer tablas, sus índices caen solos

-- Quitar FK client.active_version_id si existe
DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.table_constraints
    WHERE constraint_name = 'fk_client_active_version'
      AND table_name = 'client'
  ) THEN
    ALTER TABLE client DROP CONSTRAINT fk_client_active_version;
  END IF;
END$$;

DROP TABLE IF EXISTS signing_keys;

DROP TABLE IF EXISTS password_reset_token;
DROP TABLE IF EXISTS email_verification_token;

DROP TABLE IF EXISTS refresh_token;

-- Identity depende de app_user
DROP TABLE IF EXISTS identity;
DROP TABLE IF EXISTS app_user;

-- client_version depende de client
DROP TABLE IF EXISTS client_version;
DROP TABLE IF EXISTS client;

DROP TABLE IF EXISTS tenant;

-- La extensión podés dejarla; si querés limpieza total:
-- DROP EXTENSION IF EXISTS "pgcrypto";

COMMIT;
