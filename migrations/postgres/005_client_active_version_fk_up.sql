-- 005_client_active_version_fk_up.sql
-- Añade FK opcional de client.active_version_id -> client_version(id)

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints
        WHERE constraint_name = 'fk_client_active_version'
          AND table_name = 'client'
    ) THEN
        ALTER TABLE client
          ADD CONSTRAINT fk_client_active_version
          FOREIGN KEY (active_version_id)
          REFERENCES client_version(id)
          ON DELETE SET NULL;
    END IF;
END$$;
