-- 000044_publisher_system_redesign.up.sql

-- 1. Extend publishers table with credentials, unique referral token, and soft-delete support
ALTER TABLE publishers 
ADD COLUMN IF NOT EXISTS email CITEXT,
ADD COLUMN IF NOT EXISTS password_hash VARCHAR(255),
ADD COLUMN IF NOT EXISTS referral_token VARCHAR(64),
ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP WITH TIME ZONE;

CREATE UNIQUE INDEX IF NOT EXISTS idx_publishers_referral_token ON publishers(referral_token) WHERE referral_token IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_publishers_email ON publishers(email) WHERE email IS NOT NULL;

-- 2. Add referred_by_publisher_id directly to schools table for permanent, clean attribution
ALTER TABLE schools
ADD COLUMN IF NOT EXISTS referred_by_publisher_id VARCHAR(32) REFERENCES publishers(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_schools_referred_by_publisher_id ON schools(referred_by_publisher_id);
