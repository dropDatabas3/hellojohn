-- 005_client_active_version_fk_down.sql
-- Elimina la FK fk_client_active_version si existe

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.table_constraints
        WHERE constraint_name = 'fk_client_active_version'
          AND table_name = 'client'
    ) THEN
        ALTER TABLE client
          DROP CONSTRAINT fk_client_active_version;
    END IF;
END$$;
