-- Drop disabled fields if present
ALTER TABLE app_user DROP COLUMN IF EXISTS disabled_at;
ALTER TABLE app_user DROP COLUMN IF EXISTS disabled_reason;
