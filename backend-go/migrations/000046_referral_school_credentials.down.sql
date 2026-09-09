-- 000046_referral_school_credentials.down.sql

ALTER TABLE schools
DROP COLUMN IF EXISTS referral_admin_password;

ALTER TABLE pending_signups
DROP COLUMN IF EXISTS referral_password;
