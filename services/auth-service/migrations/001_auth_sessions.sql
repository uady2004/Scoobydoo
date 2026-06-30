-- Migration: 001_auth_sessions
-- Creates the core auth tables: users, sessions, otp_codes, device_sessions.
-- Run with: psql $DATABASE_URL -f migrations/001_auth_sessions.sql
-- Or wire into golang-migrate / goose in your CI pipeline.

-- ── Extensions ────────────────────────────────────────────────────────────────

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ── Enum types ─────────────────────────────────────────────────────────────────

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'auth_provider') THEN
        CREATE TYPE auth_provider AS ENUM ('local', 'google', 'apple');
    END IF;
END$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'user_status') THEN
        CREATE TYPE user_status AS ENUM ('active', 'inactive', 'suspended', 'deleted');
    END IF;
END$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'otp_type') THEN
        CREATE TYPE otp_type AS ENUM (
            'phone_verification',
            'email_verification',
            'password_reset',
            'login'
        );
    END IF;
END$$;

-- ── users ──────────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS users (
    id               UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),

    -- Identifiers (at least one must be non-null for local accounts)
    email            TEXT         UNIQUE,
    phone            TEXT         UNIQUE,
    username         TEXT         NOT NULL UNIQUE,

    -- Authentication
    password_hash    TEXT,                            -- NULL for pure-OAuth accounts
    provider         auth_provider NOT NULL DEFAULT 'local',
    provider_user_id TEXT,                            -- OAuth subject claim

    -- Verification
    email_verified   BOOLEAN      NOT NULL DEFAULT FALSE,
    phone_verified   BOOLEAN      NOT NULL DEFAULT FALSE,

    -- MFA (TOTP)
    mfa_enabled      BOOLEAN      NOT NULL DEFAULT FALSE,
    mfa_secret       TEXT,                            -- base32 TOTP shared secret

    -- Account lifecycle
    status           user_status  NOT NULL DEFAULT 'active',

    -- Profile hints (full profile lives in user-service)
    display_name     TEXT,
    avatar_url       TEXT,

    -- Timestamps
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at       TIMESTAMPTZ                      -- soft delete
);

-- Provider-identity lookup (used by OAuth flows)
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_provider_id
    ON users (provider, provider_user_id)
    WHERE provider_user_id IS NOT NULL;

-- Fast look-ups
CREATE INDEX IF NOT EXISTS idx_users_email       ON users (email)   WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_users_phone       ON users (phone)   WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_users_username    ON users (username) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_users_status      ON users (status)  WHERE deleted_at IS NULL;

-- ── sessions ───────────────────────────────────────────────────────────────────
-- Each row represents a refresh-token / device session.
-- refresh_token stores SHA-256(raw_token) — never the raw token itself.

CREATE TABLE IF NOT EXISTS sessions (
    id            UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id       UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- SHA-256 hex digest of the raw refresh token
    refresh_token TEXT        NOT NULL UNIQUE,

    -- Client context
    user_agent    TEXT        NOT NULL DEFAULT '',
    ip_address    TEXT        NOT NULL DEFAULT '',
    device_id     TEXT,

    -- Lifecycle
    is_revoked    BOOLEAN     NOT NULL DEFAULT FALSE,
    expires_at    TIMESTAMPTZ NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at    TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_sessions_user_id       ON sessions (user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_refresh_token ON sessions (refresh_token);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at    ON sessions (expires_at) WHERE NOT is_revoked;

-- ── otp_codes ──────────────────────────────────────────────────────────────────
-- Short-lived verification codes sent via SMS or email.
-- The `code` column holds SHA-256(plaintext_code).

CREATE TABLE IF NOT EXISTS otp_codes (
    id         UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- SHA-256 hex digest of the plaintext OTP
    code       TEXT        NOT NULL,

    type       otp_type    NOT NULL,
    target     TEXT        NOT NULL,   -- phone number or email address

    attempts   INT         NOT NULL DEFAULT 0,
    max_trials INT         NOT NULL DEFAULT 5,

    is_used    BOOLEAN     NOT NULL DEFAULT FALSE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_otp_codes_user_type ON otp_codes (user_id, type)
    WHERE NOT is_used;
CREATE INDEX IF NOT EXISTS idx_otp_codes_expires_at ON otp_codes (expires_at)
    WHERE NOT is_used;

-- ── device_sessions ────────────────────────────────────────────────────────────
-- Associates a persistent device fingerprint to a user for trust scoring.

CREATE TABLE IF NOT EXISTS device_sessions (
    id             UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id        UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_id      TEXT        NOT NULL,
    device_name    TEXT        NOT NULL DEFAULT '',
    platform       TEXT        NOT NULL DEFAULT '',

    -- Last-seen network context
    ip_address     TEXT        NOT NULL DEFAULT '',
    user_agent     TEXT        NOT NULL DEFAULT '',

    is_trusted     BOOLEAN     NOT NULL DEFAULT FALSE,
    last_active_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at     TIMESTAMPTZ,

    CONSTRAINT uq_device_sessions_user_device UNIQUE (user_id, device_id)
);

CREATE INDEX IF NOT EXISTS idx_device_sessions_user_id
    ON device_sessions (user_id)
    WHERE revoked_at IS NULL;

-- ── updated_at trigger ────────────────────────────────────────────────────────
-- Automatically bumps updated_at on the users table.

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_users_updated_at ON users;
CREATE TRIGGER trg_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ── expired-record cleanup (optional scheduled job) ───────────────────────────
-- The application layer handles cleanup, but this view makes it easy to spot
-- stale rows during development.

CREATE OR REPLACE VIEW v_active_sessions AS
SELECT *
FROM sessions
WHERE NOT is_revoked
  AND expires_at > NOW();

CREATE OR REPLACE VIEW v_active_otp_codes AS
SELECT *
FROM otp_codes
WHERE NOT is_used
  AND expires_at > NOW();
