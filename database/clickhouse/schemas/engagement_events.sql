-- =============================================================================
-- engagement_events.sql
-- Captures every discrete user-action event: likes, comments, shares,
-- follows, bookmarks, and more. Stored in a single wide table partitioned
-- by month; event_type distinguishes rows. Secondary aggregation MVs roll
-- up counts per (content, day) and per (user, day).
-- =============================================================================

-- ---------------------------------------------------------------------------
-- Raw engagement events
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS tiktok.engagement_events
(
    -- Core identity
    event_id         UUID                    DEFAULT generateUUIDv4() COMMENT 'Deduplification key',
    user_id          UInt64                  COMMENT 'Actor (who performed the action)',

    -- Event classification
    event_type       LowCardinality(String)  COMMENT 'like | comment | share | follow | bookmark | duet | stitch | report | download',
    is_undo          UInt8                   DEFAULT 0 COMMENT '1 = un-like / un-follow / un-bookmark',

    -- Target content (only one of video_id / sound_id / user_id_target is set)
    video_id         UInt64                  DEFAULT 0 COMMENT 'Target video (0 = N/A)',
    comment_id       UInt64                  DEFAULT 0 COMMENT 'Target comment (for comment-likes)',
    sound_id         UInt64                  DEFAULT 0 COMMENT 'Target sound',
    user_id_target   UInt64                  DEFAULT 0 COMMENT 'Target user (for follows)',
    hashtag_id       UInt64                  DEFAULT 0 COMMENT 'Target hashtag (for hashtag follows)',

    -- Creator of the target content
    creator_id       UInt64                  DEFAULT 0 COMMENT 'Creator of the target video/sound',

    -- Share metadata
    share_platform   LowCardinality(String)  DEFAULT '' COMMENT 'tiktok | instagram | twitter | whatsapp | link | other',

    -- Comment payload (stored inline to avoid JOIN; truncated)
    comment_text     String                  DEFAULT '' COMMENT 'First 500 chars of comment text',
    comment_lang     LowCardinality(FixedString(5)) DEFAULT '' COMMENT 'BCP-47 language tag',

    -- Device / geo context
    device_type      LowCardinality(String)  COMMENT 'ios | android | web | tablet',
    country          LowCardinality(FixedString(2)) COMMENT 'ISO-3166-1 alpha-2',
    region           LowCardinality(String)  DEFAULT '',

    -- Session
    session_id       UUID                    DEFAULT generateUUIDv4(),

    -- Time
    timestamp        DateTime                COMMENT 'UTC event time',
    event_date       Date                    MATERIALIZED toDate(timestamp)
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (event_type, video_id, timestamp)
PRIMARY KEY (event_type, video_id, timestamp)
TTL event_date + INTERVAL 365 DAY
SETTINGS index_granularity = 8192;

-- Bloom filter on user_id so "all actions by user" queries skip granules fast
ALTER TABLE tiktok.engagement_events
    ADD INDEX idx_user_id user_id TYPE bloom_filter(0.01) GRANULARITY 4;

-- Set index on creator_id for "all engagement on creator's content"
ALTER TABLE tiktok.engagement_events
    ADD INDEX idx_creator_id creator_id TYPE set(1000) GRANULARITY 4;

-- ---------------------------------------------------------------------------
-- Aggregation: engagement counts per video per day
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS tiktok.engagement_by_video_day
(
    video_id        UInt64,
    creator_id      UInt64,
    event_date      Date,
    likes           UInt64,
    unlikes         UInt64,
    comments        UInt64,
    shares          UInt64,
    bookmarks       UInt64,
    duets           UInt64,
    stitches        UInt64,
    downloads       UInt64,
    reports         UInt64
)
ENGINE = SummingMergeTree(
    likes, unlikes, comments, shares,
    bookmarks, duets, stitches, downloads, reports
)
PARTITION BY toYYYYMM(event_date)
ORDER BY (video_id, event_date)
TTL event_date + INTERVAL 730 DAY
SETTINGS index_granularity = 8192;

CREATE MATERIALIZED VIEW IF NOT EXISTS tiktok.mv_engagement_by_video_day
TO tiktok.engagement_by_video_day
AS
SELECT
    video_id,
    creator_id,
    toDate(timestamp)                       AS event_date,
    countIf(event_type = 'like'      AND is_undo = 0) AS likes,
    countIf(event_type = 'like'      AND is_undo = 1) AS unlikes,
    countIf(event_type = 'comment'   AND is_undo = 0) AS comments,
    countIf(event_type = 'share'     AND is_undo = 0) AS shares,
    countIf(event_type = 'bookmark'  AND is_undo = 0) AS bookmarks,
    countIf(event_type = 'duet'      AND is_undo = 0) AS duets,
    countIf(event_type = 'stitch'    AND is_undo = 0) AS stitches,
    countIf(event_type = 'download'  AND is_undo = 0) AS downloads,
    countIf(event_type = 'report'    AND is_undo = 0) AS reports
FROM tiktok.engagement_events
WHERE video_id > 0
GROUP BY video_id, creator_id, event_date;

-- ---------------------------------------------------------------------------
-- Aggregation: follow/unfollow counts per creator per day
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS tiktok.follow_events_by_creator_day
(
    creator_id    UInt64,
    event_date    Date,
    follows       UInt64,
    unfollows     UInt64,
    net_follows   Int64   -- computed at query time from sum(follows) - sum(unfollows)
)
ENGINE = SummingMergeTree(follows, unfollows, net_follows)
PARTITION BY toYYYYMM(event_date)
ORDER BY (creator_id, event_date)
TTL event_date + INTERVAL 730 DAY
SETTINGS index_granularity = 8192;

CREATE MATERIALIZED VIEW IF NOT EXISTS tiktok.mv_follow_events_by_creator_day
TO tiktok.follow_events_by_creator_day
AS
SELECT
    user_id_target                              AS creator_id,
    toDate(timestamp)                           AS event_date,
    countIf(is_undo = 0)                        AS follows,
    countIf(is_undo = 1)                        AS unfollows,
    toInt64(countIf(is_undo = 0)) - toInt64(countIf(is_undo = 1)) AS net_follows
FROM tiktok.engagement_events
WHERE event_type = 'follow' AND user_id_target > 0
GROUP BY creator_id, event_date;

-- ---------------------------------------------------------------------------
-- Helper view: net engagement per video per day (deduplicated)
-- ---------------------------------------------------------------------------
CREATE VIEW IF NOT EXISTS tiktok.v_engagement_daily AS
SELECT
    video_id,
    creator_id,
    event_date,
    sum(likes)      - sum(unlikes)   AS net_likes,
    sum(likes)                        AS gross_likes,
    sum(comments)                     AS comments,
    sum(shares)                       AS shares,
    sum(bookmarks)                    AS bookmarks,
    sum(duets)                        AS duets,
    sum(stitches)                     AS stitches,
    sum(downloads)                    AS downloads,
    sum(reports)                      AS reports
FROM tiktok.engagement_by_video_day
GROUP BY video_id, creator_id, event_date;
