BEGIN;

-- ─────────────────────────────────────────────────────────────
-- Limpieza previa: tablas de migras nuevas y/o legacy
-- (si no existen, no pasa nada)
-- ─────────────────────────────────────────────────────────────
DROP TABLE IF EXISTS user_consent        CASCADE;  -- 0003
DROP TABLE IF EXISTS oidc_consent        CASCADE;  -- LEGACY (viejita)
DROP TABLE IF EXISTS scope               CASCADE;  -- 0003
DROP TABLE IF EXISTS trusted_device      CASCADE;  -- 0002
DROP TABLE IF EXISTS mfa_recovery_code   CASCADE;  -- 0002
DROP TABLE IF EXISTS user_mfa_totp       CASCADE;  -- 0002

-- Quitar FK client.active_version_id si existe (idempotente)
DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM information_schema.table_constraints
    WHERE constraint_name = 'fk_client_active_version'
      AND table_name = 'client'
  ) THEN
    ALTER TABLE client DROP CONSTRAINT fk_client_active_version;
  END IF;
END$$;

-- ─────────────────────────────────────────────────────────────
-- Tablas del core
-- ─────────────────────────────────────────────────────────────
DROP TABLE IF EXISTS signing_keys            CASCADE;

DROP TABLE IF EXISTS password_reset_token    CASCADE;
DROP TABLE IF EXISTS email_verification_token CASCADE;

DROP TABLE IF EXISTS refresh_token           CASCADE;
DROP TABLE IF EXISTS identity                CASCADE;

-- Orden: primero versiones, luego user/client, y por último tenant
DROP TABLE IF EXISTS client_version          CASCADE;
DROP TABLE IF EXISTS app_user                CASCADE;
DROP TABLE IF EXISTS client                  CASCADE;
DROP TABLE IF EXISTS tenant                  CASCADE;

-- Si querés limpieza total de extensiones, descomentar:
-- DROP EXTENSION IF EXISTS "pgcrypto";

COMMIT;
