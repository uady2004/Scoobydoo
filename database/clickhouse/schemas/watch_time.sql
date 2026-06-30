-- =============================================================================
-- watch_time.sql
-- Materialized aggregation of total watch time per video per day.
-- The MaterializedView incrementally populates watch_time_by_video_day from
-- every INSERT into tiktok.video_views, so dashboards never touch raw data.
-- =============================================================================

-- ---------------------------------------------------------------------------
-- Target (aggregate) table
-- SummingMergeTree keeps per-(video_id, event_date) sums consistent across
-- background merges; reads require a GROUP BY + sum() in queries.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS tiktok.watch_time_by_video_day
(
    video_id            UInt64   COMMENT 'Video identifier',
    creator_id          UInt64   COMMENT 'Creator who owns the video',
    event_date          Date     COMMENT 'UTC calendar date of views',

    -- Metrics accumulated by SummingMergeTree
    total_watch_ms      UInt64   COMMENT 'Sum of watch_duration_ms across all views',
    total_views         UInt64   COMMENT 'Count of view events',
    total_unique_users  UInt64   COMMENT 'Approximate unique viewers (HLL sketch)',
    total_completions   UInt64   COMMENT 'Views where watch_percentage >= 1.0',
    total_replays       UInt64   COMMENT 'Sum of replay counts',

    -- Source breakdown (stored as separate columns for quick slice)
    views_fyp           UInt64   DEFAULT 0,
    views_following     UInt64   DEFAULT 0,
    views_search        UInt64   DEFAULT 0,
    views_other         UInt64   DEFAULT 0,

    -- Device breakdown
    views_ios           UInt64   DEFAULT 0,
    views_android       UInt64   DEFAULT 0,
    views_web           UInt64   DEFAULT 0
)
ENGINE = SummingMergeTree(
    total_watch_ms,
    total_views,
    total_unique_users,
    total_completions,
    total_replays,
    views_fyp,
    views_following,
    views_search,
    views_other,
    views_ios,
    views_android,
    views_web
)
PARTITION BY toYYYYMM(event_date)
ORDER BY (video_id, event_date)
PRIMARY KEY (video_id, event_date)
TTL event_date + INTERVAL 730 DAY   -- keep aggregates 2 years
SETTINGS index_granularity = 8192;

-- ---------------------------------------------------------------------------
-- MaterializedView — incremental population from raw video_views
-- ---------------------------------------------------------------------------
CREATE MATERIALIZED VIEW IF NOT EXISTS tiktok.mv_watch_time_by_video_day
TO tiktok.watch_time_by_video_day
AS
SELECT
    video_id,
    creator_id,
    toDate(timestamp)                                   AS event_date,
    sum(watch_duration_ms)                              AS total_watch_ms,
    count()                                             AS total_views,
    -- Use uniqHLL12 for cardinality; cast to UInt64 for SummingMergeTree
    toUInt64(uniqHLL12(user_id))                        AS total_unique_users,
    countIf(watch_percentage >= 1.0)                    AS total_completions,
    sum(replays)                                        AS total_replays,
    countIf(source = 'fyp')                             AS views_fyp,
    countIf(source = 'following')                       AS views_following,
    countIf(source = 'search')                          AS views_search,
    countIf(source NOT IN ('fyp', 'following', 'search')) AS views_other,
    countIf(device_type = 'ios')                        AS views_ios,
    countIf(device_type = 'android')                    AS views_android,
    countIf(device_type = 'web')                        AS views_web
FROM tiktok.video_views
GROUP BY
    video_id,
    creator_id,
    event_date;

-- ---------------------------------------------------------------------------
-- Creator-level daily rollup (secondary aggregation)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS tiktok.watch_time_by_creator_day
(
    creator_id          UInt64,
    event_date          Date,
    total_watch_ms      UInt64,
    total_views         UInt64,
    total_unique_users  UInt64,
    total_completions   UInt64,
    distinct_videos     UInt64   COMMENT 'Number of distinct videos that received views'
)
ENGINE = SummingMergeTree(
    total_watch_ms,
    total_views,
    total_unique_users,
    total_completions,
    distinct_videos
)
PARTITION BY toYYYYMM(event_date)
ORDER BY (creator_id, event_date)
TTL event_date + INTERVAL 730 DAY
SETTINGS index_granularity = 8192;

CREATE MATERIALIZED VIEW IF NOT EXISTS tiktok.mv_watch_time_by_creator_day
TO tiktok.watch_time_by_creator_day
AS
SELECT
    creator_id,
    toDate(timestamp)              AS event_date,
    sum(watch_duration_ms)         AS total_watch_ms,
    count()                        AS total_views,
    toUInt64(uniqHLL12(user_id))   AS total_unique_users,
    countIf(watch_percentage >= 1) AS total_completions,
    toUInt64(uniq(video_id))       AS distinct_videos
FROM tiktok.video_views
GROUP BY creator_id, event_date;

-- ---------------------------------------------------------------------------
-- Helper view: readable daily stats (collapses SummingMergeTree duplicates)
-- Use this in application queries rather than reading the base table directly.
-- ---------------------------------------------------------------------------
CREATE VIEW IF NOT EXISTS tiktok.v_watch_time_daily AS
SELECT
    video_id,
    creator_id,
    event_date,
    sum(total_watch_ms)      AS total_watch_ms,
    sum(total_views)         AS total_views,
    sum(total_unique_users)  AS total_unique_users,
    sum(total_completions)   AS total_completions,
    sum(total_replays)       AS total_replays,
    sum(views_fyp)           AS views_fyp,
    sum(views_following)     AS views_following,
    sum(views_search)        AS views_search,
    sum(views_other)         AS views_other,
    sum(views_ios)           AS views_ios,
    sum(views_android)       AS views_android,
    sum(views_web)           AS views_web,
    if(sum(total_views) > 0,
        sum(total_completions) / sum(total_views),
        0
    ) AS completion_rate,
    if(sum(total_views) > 0,
        sum(total_watch_ms) / sum(total_views),
        0
    ) AS avg_watch_ms
FROM tiktok.watch_time_by_video_day
GROUP BY video_id, creator_id, event_date;
