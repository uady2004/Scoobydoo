-- =============================================================================
-- creator_analytics.sql
-- Pre-built analytical queries for the Creator Studio dashboard.
-- All queries are parameterised with {creator_id:UInt64}, {start_date:Date},
-- {end_date:Date} placeholders — substitute before execution or use
-- ClickHouse parameterised queries (--param_xxx).
-- =============================================================================

-- ============================================================
-- 1. TOP VIDEOS by watch time in a date range
-- ============================================================
-- Returns the creator's top 20 videos ranked by total watch time.
-- Uses the pre-aggregated view to avoid scanning raw events.
SELECT
    w.video_id,
    sum(w.total_watch_ms)       AS total_watch_ms,
    sum(w.total_views)          AS total_views,
    sum(w.total_unique_users)   AS unique_viewers,
    sum(w.total_completions)    AS completions,
    if(sum(w.total_views) > 0,
        sum(w.total_completions) / sum(w.total_views),
        0
    )                           AS completion_rate,
    if(sum(w.total_views) > 0,
        sum(w.total_watch_ms) / sum(w.total_views) / 1000.0,
        0
    )                           AS avg_watch_seconds,
    -- Engagement signals from the engagement rollup
    sum(e.net_likes)            AS net_likes,
    sum(e.comments)             AS comments,
    sum(e.shares)               AS shares,
    sum(e.bookmarks)            AS bookmarks,
    -- Engagement rate = (likes + comments + shares) / views
    if(sum(w.total_views) > 0,
        (sum(e.net_likes) + sum(e.comments) + sum(e.shares)) / sum(w.total_views) * 100,
        0
    )                           AS engagement_rate_pct
FROM tiktok.v_watch_time_daily AS w
LEFT JOIN tiktok.v_engagement_daily AS e
    ON w.video_id = e.video_id AND w.event_date = e.event_date
WHERE
    w.creator_id  = {creator_id:UInt64}
    AND w.event_date BETWEEN {start_date:Date} AND {end_date:Date}
GROUP BY w.video_id
ORDER BY total_watch_ms DESC
LIMIT 20;

-- ============================================================
-- 2. ENGAGEMENT RATE breakdown per video (last 30 days)
-- ============================================================
SELECT
    video_id,
    sum(total_views)        AS views,
    sum(net_likes)          AS likes,
    sum(comments)           AS comments,
    sum(shares)             AS shares,
    sum(bookmarks)          AS bookmarks,
    sum(duets)              AS duets,
    sum(stitches)           AS stitches,
    -- Weighted engagement rate includes all interaction types
    if(sum(total_views) > 0,
        (
            sum(net_likes)   * 1.0 +
            sum(comments)    * 2.0 +   -- comments weighted higher
            sum(shares)      * 3.0 +   -- shares weighted highest
            sum(bookmarks)   * 1.5 +
            sum(duets)       * 2.5 +
            sum(stitches)    * 2.5
        ) / sum(total_views) * 100,
        0
    )                       AS weighted_engagement_rate_pct,
    -- Simple rate
    if(sum(total_views) > 0,
        (sum(net_likes) + sum(comments) + sum(shares)) / sum(total_views) * 100,
        0
    )                       AS simple_engagement_rate_pct
FROM (
    SELECT
        w.video_id,
        w.event_date,
        w.total_views,
        coalesce(e.net_likes, 0)  AS net_likes,
        coalesce(e.comments, 0)   AS comments,
        coalesce(e.shares, 0)     AS shares,
        coalesce(e.bookmarks, 0)  AS bookmarks,
        coalesce(e.duets, 0)      AS duets,
        coalesce(e.stitches, 0)   AS stitches
    FROM tiktok.v_watch_time_daily AS w
    LEFT JOIN tiktok.v_engagement_daily AS e
        ON w.video_id = e.video_id AND w.event_date = e.event_date
    WHERE
        w.creator_id = {creator_id:UInt64}
        AND w.event_date >= today() - 30
)
GROUP BY video_id
ORDER BY weighted_engagement_rate_pct DESC
LIMIT 50;

-- ============================================================
-- 3. FOLLOWER GROWTH over time (daily net follows)
-- ============================================================
SELECT
    event_date,
    sum(follows)                AS daily_follows,
    sum(unfollows)              AS daily_unfollows,
    sum(net_follows)            AS daily_net,
    -- Running total (requires window function — ClickHouse 22.6+)
    sum(sum(net_follows)) OVER (
        ORDER BY event_date
        ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW
    )                           AS running_total_followers
FROM tiktok.follow_events_by_creator_day
WHERE
    creator_id = {creator_id:UInt64}
    AND event_date BETWEEN {start_date:Date} AND {end_date:Date}
GROUP BY event_date
ORDER BY event_date ASC;

-- ============================================================
-- 4. FOLLOWER GROWTH — week-over-week comparison
-- ============================================================
WITH
    current_week  AS (
        SELECT sum(net_follows) AS net_follows
        FROM tiktok.follow_events_by_creator_day
        WHERE creator_id = {creator_id:UInt64}
          AND event_date >= today() - 7
    ),
    previous_week AS (
        SELECT sum(net_follows) AS net_follows
        FROM tiktok.follow_events_by_creator_day
        WHERE creator_id = {creator_id:UInt64}
          AND event_date BETWEEN today() - 14 AND today() - 8
    )
SELECT
    current_week.net_follows  AS this_week,
    previous_week.net_follows AS last_week,
    if(previous_week.net_follows != 0,
        (current_week.net_follows - previous_week.net_follows) / abs(previous_week.net_follows) * 100,
        NULL
    )                         AS wow_change_pct
FROM current_week, previous_week;

-- ============================================================
-- 5. REVENUE BREAKDOWN by type over a date range
-- ============================================================
SELECT
    transaction_type,
    currency,
    sum(gross_amount_usd)   AS gross_usd,
    sum(platform_fee_usd)   AS fee_usd,
    sum(net_amount_usd)     AS net_usd,
    sum(tax_amount_usd)     AS tax_usd,
    sum(refund_amount_usd)  AS refunds_usd,
    sum(transaction_count)  AS transactions,
    sum(refund_count)       AS refunds,
    if(sum(gross_amount_usd) > 0,
        sum(net_amount_usd) / sum(gross_amount_usd) * 100,
        0
    )                       AS creator_take_rate_pct
FROM tiktok.revenue_by_creator_day
WHERE
    creator_id = {creator_id:UInt64}
    AND txn_date BETWEEN {start_date:Date} AND {end_date:Date}
GROUP BY transaction_type, currency
ORDER BY gross_usd DESC;

-- ============================================================
-- 6. REVENUE TREND — daily net earnings
-- ============================================================
SELECT
    txn_date,
    sum(net_amount_usd)                 AS daily_net_usd,
    sumIf(net_amount_usd,
        transaction_type = 'gift_payout')   AS gift_usd,
    sumIf(net_amount_usd,
        transaction_type = 'creator_fund')  AS fund_usd,
    sumIf(net_amount_usd,
        transaction_type = 'subscription')  AS subscription_usd,
    sumIf(net_amount_usd,
        transaction_type = 'tipping')       AS tips_usd,
    sumIf(net_amount_usd,
        transaction_type = 'ad_revenue_share') AS ads_usd
FROM tiktok.revenue_by_creator_day
WHERE
    creator_id = {creator_id:UInt64}
    AND txn_date BETWEEN {start_date:Date} AND {end_date:Date}
GROUP BY txn_date
ORDER BY txn_date ASC;

-- ============================================================
-- 7. WATCH TIME TRENDS — daily average watch duration
-- ============================================================
SELECT
    event_date,
    sum(total_views)                                AS daily_views,
    sum(total_watch_ms)                             AS daily_watch_ms,
    sum(total_unique_users)                         AS daily_unique_viewers,
    sum(total_completions)                          AS daily_completions,
    if(sum(total_views) > 0,
        sum(total_watch_ms) / sum(total_views) / 1000,
        0
    )                                               AS avg_watch_seconds,
    if(sum(total_views) > 0,
        sum(total_completions) / sum(total_views) * 100,
        0
    )                                               AS completion_rate_pct,
    -- 7-day moving average of watch time (window function)
    avg(sum(total_watch_ms) / greatest(sum(total_views), 1)) OVER (
        ORDER BY event_date
        ROWS BETWEEN 6 PRECEDING AND CURRENT ROW
    ) / 1000                                        AS avg_watch_s_7d_ma
FROM tiktok.watch_time_by_creator_day
WHERE
    creator_id = {creator_id:UInt64}
    AND event_date BETWEEN {start_date:Date} AND {end_date:Date}
GROUP BY event_date
ORDER BY event_date ASC;

-- ============================================================
-- 8. WATCH TIME by traffic source (FYP vs. following vs. search)
-- ============================================================
SELECT
    event_date,
    sum(views_fyp)           AS fyp_views,
    sum(views_following)     AS following_views,
    sum(views_search)        AS search_views,
    sum(views_other)         AS other_views,
    sum(total_views)         AS total_views,
    if(sum(total_views) > 0, sum(views_fyp) / sum(total_views) * 100, 0)       AS fyp_share_pct,
    if(sum(total_views) > 0, sum(views_following) / sum(total_views) * 100, 0) AS following_share_pct,
    if(sum(total_views) > 0, sum(views_search) / sum(total_views) * 100, 0)    AS search_share_pct
FROM tiktok.watch_time_by_video_day
WHERE
    creator_id = {creator_id:UInt64}
    AND event_date BETWEEN {start_date:Date} AND {end_date:Date}
GROUP BY event_date
ORDER BY event_date ASC;

-- ============================================================
-- 9. AUDIENCE GEOGRAPHY — top countries by views
-- ============================================================
SELECT
    country,
    count()                                       AS views,
    toUInt64(uniqHLL12(user_id))                  AS unique_viewers,
    sum(watch_duration_ms)                        AS total_watch_ms,
    if(count() > 0, sum(watch_duration_ms) / count() / 1000, 0) AS avg_watch_seconds
FROM tiktok.video_views
WHERE
    creator_id = {creator_id:UInt64}
    AND timestamp >= {start_date:Date}
    AND timestamp < {end_date:Date} + INTERVAL 1 DAY
GROUP BY country
ORDER BY views DESC
LIMIT 30;

-- ============================================================
-- 10. DEVICE MIX breakdown
-- ============================================================
SELECT
    device_type,
    count()                                       AS views,
    toUInt64(uniqHLL12(user_id))                  AS unique_viewers,
    if(count() > 0, sum(watch_duration_ms) / count() / 1000, 0) AS avg_watch_seconds,
    if(count() > 0, countIf(watch_percentage >= 1) / count() * 100, 0) AS completion_rate_pct
FROM tiktok.video_views
WHERE
    creator_id = {creator_id:UInt64}
    AND event_date BETWEEN {start_date:Date} AND {end_date:Date}
GROUP BY device_type
ORDER BY views DESC;

-- ============================================================
-- 11. LIVE STREAM performance summary
-- ============================================================
SELECT
    toDate(started_at)        AS session_date,
    count()                   AS sessions,
    sum(duration_seconds)     AS total_live_seconds,
    max(peak_viewers)         AS peak_viewers,
    sum(total_unique_viewers) AS unique_viewers,
    sum(gifts_received)       AS gifts,
    sum(coins_earned)         AS coins,
    sum(usd_earned)           AS usd_earned
FROM tiktok.live_sessions
WHERE
    creator_id = {creator_id:UInt64}
    AND session_date BETWEEN {start_date:Date} AND {end_date:Date}
GROUP BY session_date
ORDER BY session_date ASC;

-- ============================================================
-- 12. CREATOR SUMMARY CARD — single-row KPI snapshot
-- ============================================================
SELECT
    -- Watch time
    sum(w.total_views)              AS total_views,
    sum(w.total_watch_ms) / 3600000 AS total_watch_hours,
    sum(w.total_unique_users)       AS unique_viewers,
    if(sum(w.total_views) > 0,
        sum(w.total_completions) / sum(w.total_views) * 100, 0) AS avg_completion_pct,

    -- Followers
    (SELECT sum(net_follows) FROM tiktok.follow_events_by_creator_day
     WHERE creator_id = {creator_id:UInt64}
       AND txn_date <= {end_date:Date}) AS total_followers,
    (SELECT sum(net_follows) FROM tiktok.follow_events_by_creator_day
     WHERE creator_id = {creator_id:UInt64}
       AND event_date BETWEEN {start_date:Date} AND {end_date:Date}) AS period_follower_growth,

    -- Revenue
    (SELECT sum(net_amount_usd) FROM tiktok.revenue_by_creator_day
     WHERE creator_id = {creator_id:UInt64}
       AND txn_date BETWEEN {start_date:Date} AND {end_date:Date}) AS net_revenue_usd
FROM tiktok.watch_time_by_creator_day AS w
WHERE
    w.creator_id = {creator_id:UInt64}
    AND w.event_date BETWEEN {start_date:Date} AND {end_date:Date};
