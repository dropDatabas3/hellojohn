-- Add disabled columns for admin disable/enable feature
ALTER TABLE app_user ADD COLUMN IF NOT EXISTS disabled_at TIMESTAMPTZ NULL;
ALTER TABLE app_user ADD COLUMN IF NOT EXISTS disabled_reason TEXT NULL;
