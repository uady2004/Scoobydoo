-- =============================================================================
-- platform_analytics.sql
-- Platform-level operational and growth analytics:
--   DAU / MAU, retention cohorts, funnel analysis, top content.
-- These queries target aggregation tables and raw events alike.
-- =============================================================================

-- ============================================================
-- 1. DAU (Daily Active Users)
-- A user is "active" on a day if they appear in video_views.
-- We use uniqExact over the aggregated HLL for accuracy on short ranges,
-- and uniqHLL12 for ranges > 90 days where approximation is acceptable.
-- ============================================================
SELECT
    event_date,
    toUInt64(uniqHLL12(user_id))  AS dau,
    count()                        AS total_view_events
FROM tiktok.video_views
WHERE
    event_date BETWEEN {start_date:Date} AND {end_date:Date}
    AND user_id > 0
GROUP BY event_date
ORDER BY event_date ASC;

-- ============================================================
-- 2. MAU (Monthly Active Users) — calendar-month grain
-- ============================================================
SELECT
    toStartOfMonth(event_date)     AS month,
    toUInt64(uniqHLL12(user_id))   AS mau
FROM tiktok.video_views
WHERE
    event_date BETWEEN {start_date:Date} AND {end_date:Date}
    AND user_id > 0
GROUP BY month
ORDER BY month ASC;

-- ============================================================
-- 3. DAU / MAU RATIO (stickiness) — trailing 28-day window
-- Stickiness = DAU / MAU expressed as a percentage.
-- 20%+ is considered healthy for social platforms.
-- ============================================================
WITH
    daily AS (
        SELECT
            event_date,
            toUInt64(uniqHLL12(user_id)) AS dau
        FROM tiktok.video_views
        WHERE event_date BETWEEN {start_date:Date} AND {end_date:Date}
          AND user_id > 0
        GROUP BY event_date
    ),
    rolling_mau AS (
        SELECT
            event_date,
            dau,
            -- Rolling 28-day unique users (approximation via sum of HLL — slight over-count)
            sum(dau) OVER (
                ORDER BY event_date
                ROWS BETWEEN 27 PRECEDING AND CURRENT ROW
            ) AS mau_approx
        FROM daily
    )
SELECT
    event_date,
    dau,
    mau_approx,
    if(mau_approx > 0, dau / mau_approx * 100, 0) AS stickiness_pct
FROM rolling_mau
ORDER BY event_date ASC;

-- ============================================================
-- 4. RETENTION COHORTS — Day-N retention by signup week
-- Cohort = week of first view (proxy for signup).
-- Requires a separate user_cohorts CTE built from video_views.
-- ============================================================
WITH
    first_seen AS (
        SELECT
            user_id,
            min(event_date) AS cohort_date
        FROM tiktok.video_views
        WHERE user_id > 0
        GROUP BY user_id
    ),
    cohorts AS (
        SELECT
            user_id,
            toMonday(cohort_date) AS cohort_week
        FROM first_seen
        WHERE cohort_date BETWEEN {start_date:Date} AND {end_date:Date}
    ),
    activity AS (
        SELECT
            v.user_id,
            v.event_date
        FROM tiktok.video_views AS v
        WHERE v.user_id > 0
          AND v.event_date BETWEEN {start_date:Date} AND {end_date:Date} + INTERVAL 60 DAY
    )
SELECT
    c.cohort_week,
    dateDiff('day', c.cohort_week, a.event_date) AS day_number,
    toUInt64(uniqHLL12(c.user_id))               AS cohort_size,
    toUInt64(uniqHLL12(a.user_id))               AS retained_users,
    if(toUInt64(uniqHLL12(c.user_id)) > 0,
        toUInt64(uniqHLL12(a.user_id)) / toUInt64(uniqHLL12(c.user_id)) * 100,
        0
    )                                            AS retention_pct
FROM cohorts AS c
INNER JOIN activity AS a ON c.user_id = a.user_id
WHERE day_number IN (0, 1, 3, 7, 14, 30, 60)
GROUP BY c.cohort_week, day_number
ORDER BY c.cohort_week ASC, day_number ASC;

-- ============================================================
-- 5. WEEKLY RETENTION TABLE (all cohort weeks in one pass)
-- More efficient version using arrayJoin for fixed intervals.
-- ============================================================
WITH
    user_cohort_week AS (
        SELECT
            user_id,
            toMonday(min(event_date)) AS cohort_week
        FROM tiktok.video_views
        WHERE user_id > 0
          AND event_date BETWEEN {start_date:Date} AND {end_date:Date}
        GROUP BY user_id
    ),
    user_active_weeks AS (
        SELECT
            user_id,
            toMonday(event_date) AS active_week
        FROM tiktok.video_views
        WHERE user_id > 0
          AND event_date BETWEEN {start_date:Date} AND {end_date:Date} + INTERVAL 90 DAY
        GROUP BY user_id, active_week
    )
SELECT
    ucw.cohort_week,
    dateDiff('week', ucw.cohort_week, uaw.active_week) AS weeks_since_signup,
    toUInt64(uniqHLL12(ucw.user_id))                   AS cohort_users,
    toUInt64(uniqHLL12(uaw.user_id))                   AS retained_users
FROM user_cohort_week AS ucw
JOIN user_active_weeks AS uaw ON ucw.user_id = uaw.user_id
WHERE weeks_since_signup >= 0 AND weeks_since_signup <= 12
GROUP BY ucw.cohort_week, weeks_since_signup
ORDER BY ucw.cohort_week ASC, weeks_since_signup ASC;

-- ============================================================
-- 6. USER ENGAGEMENT FUNNEL (daily snapshot)
-- Funnel: saw content → watched >3s → liked → commented → shared
-- ============================================================
SELECT
    event_date,
    -- Step 1: any view
    toUInt64(uniqHLL12(user_id))                                       AS step1_viewers,
    -- Step 2: watched more than 3 seconds
    toUInt64(uniqHLL12If(user_id, watch_duration_ms > 3000))           AS step2_watched_3s,
    -- Step 3: liked any video (proxy: user appears in engagement_events that day)
    toUInt64(uniqHLL12If(user_id, watch_percentage >= 0.5))            AS step3_watched_half,
    -- Step 4: watched to completion
    toUInt64(uniqHLL12If(user_id, watch_percentage >= 1.0))            AS step4_completions,
    -- Conversion rates between steps
    if(toUInt64(uniqHLL12(user_id)) > 0,
        toUInt64(uniqHLL12If(user_id, watch_duration_ms > 3000)) /
        toUInt64(uniqHLL12(user_id)) * 100, 0)                         AS step1_to_2_pct,
    if(toUInt64(uniqHLL12If(user_id, watch_duration_ms > 3000)) > 0,
        toUInt64(uniqHLL12If(user_id, watch_percentage >= 0.5)) /
        toUInt64(uniqHLL12If(user_id, watch_duration_ms > 3000)) * 100, 0) AS step2_to_3_pct,
    if(toUInt64(uniqHLL12If(user_id, watch_percentage >= 0.5)) > 0,
        toUInt64(uniqHLL12If(user_id, watch_percentage >= 1.0)) /
        toUInt64(uniqHLL12If(user_id, watch_percentage >= 0.5)) * 100, 0) AS step3_to_4_pct
FROM tiktok.video_views
WHERE
    user_id > 0
    AND event_date BETWEEN {start_date:Date} AND {end_date:Date}
GROUP BY event_date
ORDER BY event_date ASC;

-- ============================================================
-- 7. TOP CONTENT — highest-performing videos platform-wide
-- ============================================================
SELECT
    w.video_id,
    w.creator_id,
    sum(w.total_views)          AS total_views,
    sum(w.total_watch_ms)       AS total_watch_ms,
    sum(w.total_unique_users)   AS unique_viewers,
    sum(w.total_completions)    AS completions,
    if(sum(w.total_views) > 0,
        sum(w.total_completions) / sum(w.total_views) * 100, 0)   AS completion_pct,
    sum(e.net_likes)            AS likes,
    sum(e.comments)             AS comments,
    sum(e.shares)               AS shares,
    -- Viral score: combines reach, completion, and share velocity
    (
        sum(w.total_views) * 0.1 +
        sum(w.total_completions) * 0.5 +
        sum(e.shares) * 5.0 +
        sum(e.comments) * 2.0 +
        sum(e.net_likes) * 0.3
    )                           AS viral_score
FROM tiktok.v_watch_time_daily AS w
LEFT JOIN tiktok.v_engagement_daily AS e
    ON w.video_id = e.video_id AND w.event_date = e.event_date
WHERE
    w.event_date BETWEEN {start_date:Date} AND {end_date:Date}
GROUP BY w.video_id, w.creator_id
ORDER BY viral_score DESC
LIMIT 100;

-- ============================================================
-- 8. TOP CREATORS by total views
-- ============================================================
SELECT
    creator_id,
    sum(total_views)          AS total_views,
    sum(total_watch_ms) / 3600000 AS watch_hours,
    sum(total_unique_users)   AS unique_viewers,
    sum(total_completions)    AS completions,
    if(sum(total_views) > 0,
        sum(total_completions) / sum(total_views) * 100, 0) AS completion_rate_pct
FROM tiktok.watch_time_by_creator_day
WHERE event_date BETWEEN {start_date:Date} AND {end_date:Date}
GROUP BY creator_id
ORDER BY total_views DESC
LIMIT 50;

-- ============================================================
-- 9. PLATFORM REVENUE summary
-- ============================================================
SELECT
    txn_date,
    sum(gross_revenue_usd)    AS gross_revenue_usd,
    sum(platform_fees_usd)    AS platform_fees_usd,
    sum(creator_payouts_usd)  AS creator_payouts_usd,
    sum(refunds_usd)          AS refunds_usd,
    sum(transaction_count)    AS transactions,
    sum(unique_creators)      AS monetised_creators,
    sum(unique_payers)        AS paying_users,
    if(sum(gross_revenue_usd) > 0,
        sum(platform_fees_usd) / sum(gross_revenue_usd) * 100, 0) AS platform_take_pct
FROM tiktok.revenue_platform_day
WHERE txn_date BETWEEN {start_date:Date} AND {end_date:Date}
GROUP BY txn_date
ORDER BY txn_date ASC;

-- ============================================================
-- 10. CONTENT SOURCE ATTRIBUTION — FYP vs. Following mix
-- ============================================================
SELECT
    event_date,
    sum(views_fyp)             AS fyp_views,
    sum(views_following)       AS following_views,
    sum(views_search)          AS search_views,
    sum(views_other)           AS other_views,
    sum(total_views)           AS total_views,
    if(sum(total_views) > 0, sum(views_fyp) / sum(total_views) * 100, 0)       AS fyp_pct,
    if(sum(total_views) > 0, sum(views_following) / sum(total_views) * 100, 0) AS following_pct,
    if(sum(total_views) > 0, sum(views_search) / sum(total_views) * 100, 0)    AS search_pct
FROM tiktok.watch_time_by_video_day
WHERE event_date BETWEEN {start_date:Date} AND {end_date:Date}
GROUP BY event_date
ORDER BY event_date ASC;

-- ============================================================
-- 11. AD PLATFORM HEALTH — daily aggregates
-- ============================================================
SELECT
    event_date,
    sum(impressions)          AS total_impressions,
    sum(clicks)               AS total_clicks,
    sum(conversions)          AS total_conversions,
    sum(spend_usd)            AS total_spend_usd,
    sum(conversion_revenue_usd) AS total_conv_revenue_usd,
    count(DISTINCT advertiser_id) AS active_advertisers,
    if(sum(impressions) > 0, sum(clicks) / sum(impressions) * 100, 0)   AS platform_ctr_pct,
    if(sum(impressions) > 0, sum(spend_usd) / sum(impressions) * 1000, 0) AS platform_cpm_usd,
    if(sum(spend_usd) > 0, sum(conversion_revenue_usd) / sum(spend_usd), 0) AS platform_roas
FROM tiktok.ad_metrics_by_campaign_day
WHERE event_date BETWEEN {start_date:Date} AND {end_date:Date}
GROUP BY event_date
ORDER BY event_date ASC;

-- ============================================================
-- 12. HOURLY VIEW DISTRIBUTION (peak hours analysis)
-- ============================================================
SELECT
    toHour(timestamp)              AS hour_of_day,
    toDayOfWeek(timestamp)         AS day_of_week,  -- 1=Mon … 7=Sun
    count()                        AS view_events,
    toUInt64(uniqHLL12(user_id))   AS unique_users
FROM tiktok.video_views
WHERE
    user_id > 0
    AND timestamp >= {start_date:Date}
    AND timestamp <  {end_date:Date} + INTERVAL 1 DAY
GROUP BY hour_of_day, day_of_week
ORDER BY day_of_week ASC, hour_of_day ASC;

-- ============================================================
-- 13. NEW USER ACTIVATION FUNNEL
-- (Requires a users table; joined here on video_views as proxy)
-- ============================================================
WITH
    new_users AS (
        -- Users whose first ever view falls in the window
        SELECT user_id, min(event_date) AS first_seen
        FROM tiktok.video_views
        WHERE user_id > 0
        GROUP BY user_id
        HAVING first_seen BETWEEN {start_date:Date} AND {end_date:Date}
    )
SELECT
    toMonday(n.first_seen)                      AS cohort_week,
    count(DISTINCT n.user_id)                   AS new_users,
    -- D1 retention
    countIf(
        v2.user_id IS NOT NULL
    )                                           AS d1_retained,
    -- D7 retention
    countDistinctIf(
        n.user_id,
        dateDiff('day', n.first_seen, v7.event_date) BETWEEN 6 AND 8
    )                                           AS d7_retained
FROM new_users AS n
LEFT JOIN (
    SELECT DISTINCT user_id, event_date
    FROM tiktok.video_views
    WHERE user_id > 0
) AS v2 ON n.user_id = v2.user_id
       AND v2.event_date = n.first_seen + INTERVAL 1 DAY
LEFT JOIN (
    SELECT DISTINCT user_id, event_date
    FROM tiktok.video_views
    WHERE user_id > 0
) AS v7 ON n.user_id = v7.user_id
       AND v7.event_date BETWEEN n.first_seen + INTERVAL 6 DAY
                              AND n.first_seen + INTERVAL 8 DAY
GROUP BY cohort_week
ORDER BY cohort_week ASC;
