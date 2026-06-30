-- =============================================================================
-- live_metrics.sql
-- Live stream analytics: per-session summary rows AND high-frequency
-- minute-level viewer-count time-series for real-time charts.
-- =============================================================================

-- ---------------------------------------------------------------------------
-- Live session summary (one row per completed or ongoing live stream)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS tiktok.live_sessions
(
    -- Identity
    room_id          UInt64                  COMMENT 'Unique live room identifier',
    creator_id       UInt64                  COMMENT 'Host creator user ID',
    session_id       UUID                    DEFAULT generateUUIDv4() COMMENT 'Internal dedup key',

    -- Status
    status           LowCardinality(String)  COMMENT 'active | ended | banned | error',

    -- Audience
    viewer_count     UInt64                  COMMENT 'Current / last-known viewer count',
    peak_viewers     UInt64                  COMMENT 'Max concurrent viewers during session',
    total_unique_viewers UInt64              COMMENT 'Distinct users who joined',
    avg_concurrent_viewers Float32           DEFAULT 0 COMMENT 'Mean concurrent over session lifetime',

    -- Gifting / monetisation
    gifts_received   UInt64                  DEFAULT 0 COMMENT 'Count of gift events received',
    coins_earned     UInt64                  DEFAULT 0 COMMENT 'Total TikTok coins received from gifts',
    usd_earned       Decimal(12, 4)          DEFAULT 0 COMMENT 'Estimated USD value of coins',
    top_gifter_user_id UInt64               DEFAULT 0,
    top_gifter_coins   UInt64               DEFAULT 0,

    -- Content
    title            String                  DEFAULT '',
    category         LowCardinality(String)  DEFAULT '' COMMENT 'gaming | music | talk | beauty | sports | other',
    tags             Array(LowCardinality(String)) DEFAULT [] COMMENT 'User-defined tags',

    -- Duration
    started_at       DateTime                COMMENT 'UTC stream start time',
    ended_at         DateTime                DEFAULT toDateTime(0) COMMENT '0 = still active',
    duration_seconds UInt32                  MATERIALIZED
                        if(ended_at > started_at, dateDiff('second', started_at, ended_at), 0),

    -- Device / geo
    country          LowCardinality(FixedString(2)) DEFAULT '',
    device_type      LowCardinality(String)  DEFAULT '',

    -- Moderation
    warning_count    UInt8                   DEFAULT 0,
    is_age_restricted UInt8                  DEFAULT 0,

    -- Derived date
    session_date     Date                    MATERIALIZED toDate(started_at)
)
ENGINE = ReplacingMergeTree(ended_at)  -- last write per room_id wins (use for upserts)
PARTITION BY toYYYYMM(started_at)
ORDER BY (room_id, started_at)
PRIMARY KEY (room_id, started_at)
TTL session_date + INTERVAL 730 DAY
SETTINGS index_granularity = 8192;

ALTER TABLE tiktok.live_sessions
    ADD INDEX idx_creator_id creator_id TYPE bloom_filter(0.01) GRANULARITY 4;

-- ---------------------------------------------------------------------------
-- Minute-level viewer-count time-series (high write frequency)
-- Each row = one metric sample for one live room at one minute boundary.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS tiktok.live_viewer_timeseries
(
    room_id          UInt64,
    creator_id       UInt64,
    sampled_at       DateTime       COMMENT 'Rounded to nearest minute',
    viewer_count     UInt32         COMMENT 'Concurrent viewer count at sample time',
    new_joins        UInt32         DEFAULT 0 COMMENT 'Users who joined in this interval',
    new_leaves       UInt32         DEFAULT 0 COMMENT 'Users who left in this interval',
    gifts_in_period  UInt32         DEFAULT 0 COMMENT 'Gifts received in this 1-min window',
    coins_in_period  UInt64         DEFAULT 0,
    comments_in_period UInt32       DEFAULT 0,
    shares_in_period   UInt32       DEFAULT 0,
    sample_date      Date           MATERIALIZED toDate(sampled_at)
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(sampled_at)
ORDER BY (room_id, sampled_at)
TTL sample_date + INTERVAL 90 DAY   -- high-frequency data kept 3 months only
SETTINGS index_granularity = 8192;

-- ---------------------------------------------------------------------------
-- Gift events log
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS tiktok.live_gift_events
(
    event_id         UUID            DEFAULT generateUUIDv4(),
    room_id          UInt64,
    creator_id       UInt64,
    sender_user_id   UInt64,
    gift_id          UInt32          COMMENT 'Gift catalogue item ID',
    gift_name        LowCardinality(String),
    quantity         UInt16          DEFAULT 1,
    coins_spent      UInt32          COMMENT 'Coins per unit * quantity',
    usd_value        Decimal(10, 4)  COMMENT 'Approximate USD value',
    is_combo         UInt8           DEFAULT 0 COMMENT '1 = part of a combo streak',
    combo_id         UUID            DEFAULT toUUID('00000000-0000-0000-0000-000000000000'),
    timestamp        DateTime,
    event_date       Date            MATERIALIZED toDate(timestamp)
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (room_id, timestamp)
TTL event_date + INTERVAL 365 DAY
SETTINGS index_granularity = 8192;

-- ---------------------------------------------------------------------------
-- Daily rollup per creator
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS tiktok.live_metrics_by_creator_day
(
    creator_id           UInt64,
    event_date           Date,
    sessions             UInt64,
    total_duration_s     UInt64,
    total_unique_viewers UInt64,
    peak_viewers         UInt64,
    total_gifts          UInt64,
    total_coins          UInt64,
    total_usd            Decimal(16, 4)
)
ENGINE = SummingMergeTree(
    sessions, total_duration_s, total_unique_viewers,
    peak_viewers, total_gifts, total_coins
)
PARTITION BY toYYYYMM(event_date)
ORDER BY (creator_id, event_date)
TTL event_date + INTERVAL 730 DAY
SETTINGS index_granularity = 8192;

CREATE MATERIALIZED VIEW IF NOT EXISTS tiktok.mv_live_metrics_by_creator_day
TO tiktok.live_metrics_by_creator_day
AS
SELECT
    creator_id,
    toDate(started_at)                    AS event_date,
    count()                               AS sessions,
    sum(duration_seconds)                 AS total_duration_s,
    sum(total_unique_viewers)             AS total_unique_viewers,
    max(peak_viewers)                     AS peak_viewers,
    sum(gifts_received)                   AS total_gifts,
    sum(coins_earned)                     AS total_coins,
    sum(usd_earned)                       AS total_usd
FROM tiktok.live_sessions
GROUP BY creator_id, event_date;
