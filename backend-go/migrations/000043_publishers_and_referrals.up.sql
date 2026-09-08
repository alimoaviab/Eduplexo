-- 000043_publishers_and_referrals.up.sql

CREATE TABLE IF NOT EXISTS publishers (
    id VARCHAR(32) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'active',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS referral_tokens (
    id VARCHAR(32) PRIMARY KEY,
    publisher_id VARCHAR(32) NOT NULL REFERENCES publishers(id),
    token_hash VARCHAR(64) NOT NULL UNIQUE,
    status VARCHAR(50) NOT NULL DEFAULT 'UNUSED',
    plan_id VARCHAR(100) NOT NULL,
    plan_name_snapshot VARCHAR(255) NOT NULL,
    monthly_price_snapshot NUMERIC(10, 2) NOT NULL,
    currency VARCHAR(10) NOT NULL DEFAULT 'PKR',
    billing_period VARCHAR(50) NOT NULL DEFAULT 'monthly',
    expires_at TIMESTAMP WITH TIME ZONE,
    used_at TIMESTAMP WITH TIME ZONE,
    used_by_school_id VARCHAR(32),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    revoked_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_referral_tokens_publisher_id ON referral_tokens(publisher_id);
CREATE INDEX idx_referral_tokens_status ON referral_tokens(status);

CREATE TABLE IF NOT EXISTS referrals (
    id VARCHAR(32) PRIMARY KEY,
    publisher_id VARCHAR(32) NOT NULL REFERENCES publishers(id),
    referral_token_id VARCHAR(32) NOT NULL REFERENCES referral_tokens(id),
    school_id VARCHAR(32) NOT NULL,
    plan_id VARCHAR(100) NOT NULL,
    monthly_price_snapshot NUMERIC(10, 2) NOT NULL,
    commission_status VARCHAR(50) NOT NULL DEFAULT 'pending',
    commission_amount NUMERIC(10, 2),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_referrals_publisher_id ON referrals(publisher_id);
CREATE INDEX idx_referrals_school_id ON referrals(school_id);
