-- =============================================================================
-- ad_metrics.sql
-- Advertising analytics: impression, click, conversion events plus daily
-- rollup tables for CPM / CPC / ROAS reporting by campaign / ad / day.
-- =============================================================================

-- ---------------------------------------------------------------------------
-- Raw ad impression events
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS tiktok.ad_impressions
(
    -- Identity
    impression_id    UUID                    DEFAULT generateUUIDv4(),
    ad_id            UInt64                  COMMENT 'Individual creative / ad unit ID',
    ad_set_id        UInt64                  COMMENT 'Ad set / ad group',
    campaign_id      UInt64                  COMMENT 'Parent campaign',
    advertiser_id    UInt64                  COMMENT 'Advertiser account',

    -- Viewer
    user_id          UInt64                  DEFAULT 0 COMMENT '0 = anonymous',
    device_id        String                  DEFAULT '' COMMENT 'Device fingerprint (hashed)',

    -- Ad type
    ad_format        LowCardinality(String)
        COMMENT 'in_feed | topview | branded_hashtag | branded_effect | spark | search',
    placement        LowCardinality(String)
        COMMENT 'fyp | search | profile | live | story',

    -- Auction / pricing
    bid_type         LowCardinality(String)  COMMENT 'CPM | CPC | CPV | CPA | OCPM',
    bid_amount_usd   Decimal(12, 6)          COMMENT 'Advertiser bid',
    clearing_price_usd Decimal(12, 6)        DEFAULT 0 COMMENT 'Actual CPM cleared in auction',
    is_won           UInt8                   DEFAULT 1 COMMENT '1 = impression delivered',

    -- Viewability
    view_duration_ms UInt32                  DEFAULT 0,
    is_viewable      UInt8                   DEFAULT 0 COMMENT 'MRC viewability standard met',
    is_skipped       UInt8                   DEFAULT 0,
    skipped_at_ms    UInt32                  DEFAULT 0,

    -- Geo / device
    country          LowCardinality(FixedString(2)) DEFAULT '',
    region           LowCardinality(String)  DEFAULT '',
    device_type      LowCardinality(String)  DEFAULT '',
    os               LowCardinality(String)  DEFAULT '',

    -- Targeting match info
    audience_segment LowCardinality(String)  DEFAULT '' COMMENT 'Matched audience segment label',
    is_retargeting   UInt8                   DEFAULT 0,

    -- Time
    timestamp        DateTime,
    event_date       Date                    MATERIALIZED toDate(timestamp)
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (campaign_id, ad_id, timestamp)
PRIMARY KEY (campaign_id, ad_id, timestamp)
TTL event_date + INTERVAL 365 DAY
SETTINGS index_granularity = 8192;

ALTER TABLE tiktok.ad_impressions
    ADD INDEX idx_advertiser advertiser_id TYPE set(10000) GRANULARITY 4;

-- ---------------------------------------------------------------------------
-- Ad click events
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS tiktok.ad_clicks
(
    click_id         UUID            DEFAULT generateUUIDv4(),
    impression_id    UUID            COMMENT 'Links back to ad_impressions',
    ad_id            UInt64,
    ad_set_id        UInt64,
    campaign_id      UInt64,
    advertiser_id    UInt64,
    user_id          UInt64          DEFAULT 0,
    click_type       LowCardinality(String) COMMENT 'cta | logo | title | hashtag | profile',
    destination_url  String          DEFAULT '',
    country          LowCardinality(FixedString(2)) DEFAULT '',
    device_type      LowCardinality(String)  DEFAULT '',
    timestamp        DateTime,
    event_date       Date            MATERIALIZED toDate(timestamp)
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (campaign_id, ad_id, timestamp)
TTL event_date + INTERVAL 365 DAY
SETTINGS index_granularity = 8192;

-- ---------------------------------------------------------------------------
-- Ad conversion events (post-click / post-view attribution)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS tiktok.ad_conversions
(
    conversion_id    UUID            DEFAULT generateUUIDv4(),
    click_id         UUID            DEFAULT toUUID('00000000-0000-0000-0000-000000000000'),
    impression_id    UUID            DEFAULT toUUID('00000000-0000-0000-0000-000000000000'),
    ad_id            UInt64,
    ad_set_id        UInt64,
    campaign_id      UInt64,
    advertiser_id    UInt64,
    user_id          UInt64          DEFAULT 0,
    conversion_type  LowCardinality(String)
        COMMENT 'install | purchase | signup | lead | add_to_cart | checkout | view_content | subscribe',
    attribution_window LowCardinality(String) COMMENT '1d_click | 7d_click | 1d_view | 7d_view',
    revenue_usd      Decimal(14, 4)  DEFAULT 0 COMMENT 'Reported conversion value',
    quantity         UInt16          DEFAULT 1,
    country          LowCardinality(FixedString(2)) DEFAULT '',
    device_type      LowCardinality(String) DEFAULT '',
    timestamp        DateTime,
    event_date       Date            MATERIALIZED toDate(timestamp)
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (campaign_id, ad_id, timestamp)
TTL event_date + INTERVAL 365 DAY
SETTINGS index_granularity = 8192;

-- ---------------------------------------------------------------------------
-- Daily performance rollup per (campaign, ad, day) — the core reporting table
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS tiktok.ad_metrics_by_campaign_day
(
    campaign_id          UInt64,
    ad_set_id            UInt64,
    ad_id                UInt64,
    advertiser_id        UInt64,
    event_date           Date,

    -- Volume
    impressions          UInt64,
    clicks               UInt64,
    conversions          UInt64,
    unique_users_reached UInt64,

    -- Spend
    spend_usd            Decimal(20, 6) COMMENT 'Total clearing price sum',

    -- Conversion value
    conversion_revenue_usd Decimal(20, 6),

    -- Viewability
    viewable_impressions UInt64,
    video_views_25pct    UInt64,
    video_views_50pct    UInt64,
    video_views_75pct    UInt64,
    video_completions    UInt64,
    skips                UInt64
)
ENGINE = SummingMergeTree(
    impressions, clicks, conversions, unique_users_reached,
    viewable_impressions, video_views_25pct, video_views_50pct,
    video_views_75pct, video_completions, skips
)
PARTITION BY toYYYYMM(event_date)
ORDER BY (advertiser_id, campaign_id, ad_set_id, ad_id, event_date)
TTL event_date + INTERVAL 730 DAY
SETTINGS index_granularity = 8192;

CREATE MATERIALIZED VIEW IF NOT EXISTS tiktok.mv_ad_impressions_day
TO tiktok.ad_metrics_by_campaign_day
AS
SELECT
    campaign_id,
    ad_set_id,
    ad_id,
    advertiser_id,
    toDate(timestamp)                           AS event_date,
    count()                                     AS impressions,
    0                                           AS clicks,
    0                                           AS conversions,
    toUInt64(uniqHLL12(user_id))                AS unique_users_reached,
    sum(clearing_price_usd)                     AS spend_usd,
    toDecimal64(0, 6)                           AS conversion_revenue_usd,
    countIf(is_viewable = 1)                    AS viewable_impressions,
    countIf(view_duration_ms > 0 AND view_duration_ms / nullIfZero(watch_percentage_proxy) >= 0.25) AS video_views_25pct,
    0 AS video_views_50pct,
    0 AS video_views_75pct,
    countIf(is_skipped = 0 AND view_duration_ms > 0) AS video_completions,
    countIf(is_skipped = 1)                     AS skips
FROM tiktok.ad_impressions
GROUP BY campaign_id, ad_set_id, ad_id, advertiser_id, event_date;

-- ---------------------------------------------------------------------------
-- Computed metrics view: CPM, CPC, CTR, CVR, ROAS
-- (Never store derived rates in base tables — calculate them here)
-- ---------------------------------------------------------------------------
CREATE VIEW IF NOT EXISTS tiktok.v_ad_performance AS
SELECT
    advertiser_id,
    campaign_id,
    ad_set_id,
    ad_id,
    event_date,
    sum(impressions)          AS impressions,
    sum(clicks)               AS clicks,
    sum(conversions)          AS conversions,
    sum(spend_usd)            AS spend_usd,
    sum(conversion_revenue_usd) AS revenue_usd,
    sum(unique_users_reached) AS unique_reach,
    sum(viewable_impressions) AS viewable_impressions,
    sum(video_completions)    AS video_completions,
    sum(skips)                AS skips,

    -- Click-through rate
    if(sum(impressions) > 0,
        sum(clicks) / sum(impressions) * 100,
        0
    ) AS ctr_pct,

    -- Conversion rate (from clicks)
    if(sum(clicks) > 0,
        sum(conversions) / sum(clicks) * 100,
        0
    ) AS cvr_pct,

    -- Cost per mille (CPM)
    if(sum(impressions) > 0,
        sum(spend_usd) / sum(impressions) * 1000,
        0
    ) AS cpm_usd,

    -- Cost per click (CPC)
    if(sum(clicks) > 0,
        sum(spend_usd) / sum(clicks),
        0
    ) AS cpc_usd,

    -- Cost per acquisition (CPA)
    if(sum(conversions) > 0,
        sum(spend_usd) / sum(conversions),
        0
    ) AS cpa_usd,

    -- Return on ad spend (ROAS)
    if(sum(spend_usd) > 0,
        sum(conversion_revenue_usd) / sum(spend_usd),
        0
    ) AS roas,

    -- Viewability rate
    if(sum(impressions) > 0,
        sum(viewable_impressions) / sum(impressions) * 100,
        0
    ) AS viewability_pct,

    -- Video completion rate
    if(sum(impressions) > 0,
        sum(video_completions) / sum(impressions) * 100,
        0
    ) AS vcr_pct

FROM tiktok.ad_metrics_by_campaign_day
GROUP BY
    advertiser_id, campaign_id, ad_set_id, ad_id, event_date;
