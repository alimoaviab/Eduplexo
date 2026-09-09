-- ─── 000047_password_resets.up.sql ──────────────────────────────────────────
-- Password resets & secure 6-digit OTP verification table
CREATE TABLE IF NOT EXISTS password_resets (
    id                  TEXT        PRIMARY KEY,
    email               CITEXT      NOT NULL,
    user_id             TEXT        NOT NULL,
    otp_hash            TEXT        NOT NULL,
    reset_token         TEXT        UNIQUE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at          TIMESTAMPTZ NOT NULL,
    last_sent_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    attempts            INT         NOT NULL DEFAULT 0,
    max_attempts        INT         NOT NULL DEFAULT 5,
    send_count_hour     INT         NOT NULL DEFAULT 1,
    status              TEXT        NOT NULL DEFAULT 'pending',
    verified_at         TIMESTAMPTZ,
    used_at             TIMESTAMPTZ,
    ip_address          TEXT        NOT NULL DEFAULT '',
    CONSTRAINT password_resets_status_chk
        CHECK (status IN ('pending', 'verified', 'used', 'expired'))
);

CREATE INDEX IF NOT EXISTS idx_password_resets_email ON password_resets (email);
CREATE INDEX IF NOT EXISTS idx_password_resets_reset_token ON password_resets (reset_token);
CREATE INDEX IF NOT EXISTS idx_password_resets_status ON password_resets (status);
