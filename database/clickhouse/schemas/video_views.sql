-- =============================================================================
-- video_views.sql
-- Tracks every video view event: who watched what, for how long, and from where.
-- Partitioned by month; ordered by (video_id, timestamp) for efficient per-video
-- time-series scans. TTL removes raw rows after 365 days.
-- =============================================================================

CREATE DATABASE IF NOT EXISTS tiktok;

-- ---------------------------------------------------------------------------
-- Raw event table
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS tiktok.video_views
(
    -- Identity
    user_id          UInt64                  COMMENT 'Viewer user ID (0 = anonymous)',
    video_id         UInt64                  COMMENT 'Video being watched',
    creator_id       UInt64                  COMMENT 'Owner / uploader of the video',

    -- Engagement signals
    watch_duration_ms UInt32                 COMMENT 'Milliseconds the user actually watched',
    watch_percentage  Float32                COMMENT 'Fraction of video watched (0.0–1.0+)',

    -- Traffic source
    source           LowCardinality(String)  COMMENT 'fyp | following | search | profile | share | hashtag | sound',

    -- Device / geo
    device_type      LowCardinality(String)  COMMENT 'ios | android | web | tablet',
    country          LowCardinality(FixedString(2)) COMMENT 'ISO-3166-1 alpha-2 country code',
    region           LowCardinality(String)  DEFAULT '' COMMENT 'State / province (optional)',

    -- Network
    network_type     LowCardinality(String)  DEFAULT '' COMMENT 'wifi | 4g | 5g | 3g | unknown',

    -- Session
    session_id       UUID                    DEFAULT generateUUIDv4() COMMENT 'Client session identifier',

    -- Playback metadata
    is_autoplay      UInt8                   DEFAULT 0 COMMENT '1 if started by autoplay, 0 if manual',
    is_muted         UInt8                   DEFAULT 0 COMMENT '1 if audio muted during watch',
    replays          UInt8                   DEFAULT 0 COMMENT 'Number of times user replayed',

    -- Time
    timestamp        DateTime                COMMENT 'UTC event timestamp',
    event_date       Date                    MATERIALIZED toDate(timestamp) COMMENT 'Derived date for TTL / partition pruning'
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (video_id, timestamp)
PRIMARY KEY (video_id, timestamp)
SAMPLE BY intHash32(user_id)
TTL event_date + INTERVAL 365 DAY
SETTINGS
    index_granularity = 8192,
    min_bytes_for_wide_part = 10485760; -- 10 MB

-- ---------------------------------------------------------------------------
-- Bloom-filter skipping index on user_id for "all views by user" queries
-- ---------------------------------------------------------------------------
ALTER TABLE tiktok.video_views
    ADD INDEX idx_user_id user_id TYPE bloom_filter(0.01) GRANULARITY 4;

-- ---------------------------------------------------------------------------
-- Skipping index on source for low-cardinality filter queries
-- ---------------------------------------------------------------------------
ALTER TABLE tiktok.video_views
    ADD INDEX idx_source source TYPE set(10) GRANULARITY 4;

-- ---------------------------------------------------------------------------
-- Comment
-- ---------------------------------------------------------------------------
-- Usage patterns this schema is optimised for:
--   1. "All views of video X in the last 7 days"      → ORDER BY (video_id, timestamp)
--   2. "All videos watched by user U"                  → idx_user_id bloom filter
--   3. "Views by source / country"                     → LowCardinality columns, idx_source
--   4. Monthly partition pruning keeps scans fast.
--   5. TTL drops rows older than 1 year automatically.
