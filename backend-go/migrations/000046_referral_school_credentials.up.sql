-- 000046_referral_school_credentials.up.sql

ALTER TABLE schools
ADD COLUMN IF NOT EXISTS referral_admin_password TEXT;

ALTER TABLE pending_signups
ADD COLUMN IF NOT EXISTS referral_password TEXT;
