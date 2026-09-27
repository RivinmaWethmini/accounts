ALTER TABLE Users ADD COLUMN IF NOT EXISTS is_verified BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE Users ADD COLUMN IF NOT EXISTS verification_token TEXT;
ALTER TABLE Users ADD COLUMN IF NOT EXISTS verification_token_expires_at TIMESTAMPTZ;

-- Backfill existing users as verified so existing accounts remain functional
UPDATE Users SET is_verified = TRUE WHERE is_verified = FALSE;
