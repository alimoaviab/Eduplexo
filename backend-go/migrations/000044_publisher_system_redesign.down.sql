-- 000044_publisher_system_redesign.down.sql

DROP INDEX IF EXISTS idx_schools_referred_by_publisher_id;
ALTER TABLE schools DROP COLUMN IF EXISTS referred_by_publisher_id;

DROP INDEX IF EXISTS idx_publishers_email;
DROP INDEX IF EXISTS idx_publishers_referral_token;
ALTER TABLE publishers 
DROP COLUMN IF EXISTS deleted_at,
DROP COLUMN IF EXISTS referral_token,
DROP COLUMN IF EXISTS password_hash,
DROP COLUMN IF EXISTS email;
