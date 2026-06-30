-- =============================================================================
-- revenue_metrics.sql
-- Platform-wide revenue ledger: every monetary transaction is recorded here.
-- Covers creator fund payouts, gift conversions, subscriptions, tipping,
-- ad revenue share, in-app purchases, and refunds.
-- =============================================================================

-- ---------------------------------------------------------------------------
-- Master transactions table
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS tiktok.revenue_transactions
(
    -- Identity
    transaction_id   UUID                    DEFAULT generateUUIDv4() COMMENT 'Global unique transaction ID',
    idempotency_key  String                  DEFAULT '' COMMENT 'Client-supplied dedup key',

    -- Parties
    creator_id       UInt64                  DEFAULT 0 COMMENT '0 = platform-level (no creator)',
    user_id          UInt64                  DEFAULT 0 COMMENT 'Payer / recipient user ID',

    -- Classification
    transaction_type LowCardinality(String)
        COMMENT 'gift_payout | creator_fund | subscription | tipping | coin_purchase | ad_revenue_share | refund | chargeback | withdrawal',
    status           LowCardinality(String)
        COMMENT 'pending | completed | failed | refunded | reversed',

    -- Amounts (all stored in minor units of the currency to avoid float errors,
    --          except where Decimal is used)
    amount           Decimal(18, 6)          COMMENT 'Gross transaction amount',
    currency         LowCardinality(FixedString(3)) COMMENT 'ISO-4217 currency code (USD, EUR, …)',
    amount_usd       Decimal(18, 6)          COMMENT 'Normalised USD equivalent',
    exchange_rate    Decimal(18, 8)          DEFAULT 1 COMMENT 'Rate used for USD conversion',

    -- Fee split
    platform_fee_pct Decimal(6, 4)           DEFAULT 0 COMMENT 'Platform cut percentage (e.g. 0.5000 = 50%)',
    platform_fee     Decimal(18, 6)          DEFAULT 0 COMMENT 'Absolute platform fee',
    net_amount       Decimal(18, 6)          COMMENT 'Amount paid to creator = amount - platform_fee',
    net_amount_usd   Decimal(18, 6)          DEFAULT 0,

    -- Tax
    tax_amount       Decimal(18, 6)          DEFAULT 0,
    tax_region       LowCardinality(String)  DEFAULT '' COMMENT 'Tax jurisdiction',

    -- Payment processor
    processor        LowCardinality(String)  DEFAULT '' COMMENT 'stripe | paypal | applepay | googlepay | internal',
    processor_txn_id String                  DEFAULT '' COMMENT 'External transaction reference',

    -- Source entity
    source_video_id  UInt64                  DEFAULT 0,
    source_room_id   UInt64                  DEFAULT 0,
    source_gift_id   UInt32                  DEFAULT 0,
    source_ad_id     UInt64                  DEFAULT 0,

    -- Geography
    country          LowCardinality(FixedString(2)) DEFAULT '',

    -- Time
    timestamp        DateTime                COMMENT 'UTC transaction time',
    settled_at       DateTime                DEFAULT toDateTime(0) COMMENT '0 = not yet settled',
    txn_date         Date                    MATERIALIZED toDate(timestamp)
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (creator_id, transaction_type, timestamp)
PRIMARY KEY (creator_id, transaction_type, timestamp)
TTL txn_date + INTERVAL 2555 DAY  -- 7 years (financial record retention)
SETTINGS index_granularity = 8192;

ALTER TABLE tiktok.revenue_transactions
    ADD INDEX idx_transaction_id transaction_id TYPE bloom_filter(0.001) GRANULARITY 4;
ALTER TABLE tiktok.revenue_transactions
    ADD INDEX idx_idempotency_key idempotency_key TYPE bloom_filter(0.001) GRANULARITY 4;
ALTER TABLE tiktok.revenue_transactions
    ADD INDEX idx_user_id user_id TYPE bloom_filter(0.01) GRANULARITY 4;

-- ---------------------------------------------------------------------------
-- Daily revenue rollup per creator
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS tiktok.revenue_by_creator_day
(
    creator_id           UInt64,
    transaction_type     LowCardinality(String),
    currency             LowCardinality(FixedString(3)),
    txn_date             Date,
    gross_amount_usd     Decimal(20, 6),
    platform_fee_usd     Decimal(20, 6),
    net_amount_usd       Decimal(20, 6),
    tax_amount_usd       Decimal(20, 6),
    transaction_count    UInt64,
    refund_amount_usd    Decimal(20, 6),
    refund_count         UInt64
)
ENGINE = SummingMergeTree(
    gross_amount_usd, platform_fee_usd,
    net_amount_usd, tax_amount_usd,
    transaction_count, refund_amount_usd, refund_count
)
PARTITION BY toYYYYMM(txn_date)
ORDER BY (creator_id, transaction_type, currency, txn_date)
TTL txn_date + INTERVAL 2555 DAY
SETTINGS index_granularity = 8192;

CREATE MATERIALIZED VIEW IF NOT EXISTS tiktok.mv_revenue_by_creator_day
TO tiktok.revenue_by_creator_day
AS
SELECT
    creator_id,
    transaction_type,
    currency,
    toDate(timestamp)                           AS txn_date,
    sumIf(amount_usd, status = 'completed')     AS gross_amount_usd,
    sumIf(platform_fee, status = 'completed') * assumeNotNull(any(exchange_rate)) AS platform_fee_usd,
    sumIf(net_amount_usd, status = 'completed') AS net_amount_usd,
    sumIf(tax_amount, status = 'completed') * assumeNotNull(any(exchange_rate)) AS tax_amount_usd,
    countIf(status = 'completed')               AS transaction_count,
    sumIf(amount_usd, status = 'refunded')      AS refund_amount_usd,
    countIf(status = 'refunded')                AS refund_count
FROM tiktok.revenue_transactions
WHERE creator_id > 0
GROUP BY creator_id, transaction_type, currency, txn_date;

-- ---------------------------------------------------------------------------
-- Platform-wide daily summary (all creators combined)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS tiktok.revenue_platform_day
(
    txn_date             Date,
    transaction_type     LowCardinality(String),
    gross_revenue_usd    Decimal(24, 6),
    platform_fees_usd    Decimal(24, 6),
    creator_payouts_usd  Decimal(24, 6),
    refunds_usd          Decimal(24, 6),
    transaction_count    UInt64,
    unique_creators      UInt64,
    unique_payers        UInt64
)
ENGINE = SummingMergeTree(
    gross_revenue_usd, platform_fees_usd,
    creator_payouts_usd, refunds_usd,
    transaction_count, unique_creators, unique_payers
)
PARTITION BY toYYYYMM(txn_date)
ORDER BY (txn_date, transaction_type)
TTL txn_date + INTERVAL 2555 DAY
SETTINGS index_granularity = 8192;

CREATE MATERIALIZED VIEW IF NOT EXISTS tiktok.mv_revenue_platform_day
TO tiktok.revenue_platform_day
AS
SELECT
    toDate(timestamp)                           AS txn_date,
    transaction_type,
    sumIf(amount_usd, status = 'completed')     AS gross_revenue_usd,
    sumIf(platform_fee, status = 'completed') * assumeNotNull(any(exchange_rate)) AS platform_fees_usd,
    sumIf(net_amount_usd, status = 'completed') AS creator_payouts_usd,
    sumIf(amount_usd, status = 'refunded')      AS refunds_usd,
    countIf(status = 'completed')               AS transaction_count,
    toUInt64(uniq(creator_id))                  AS unique_creators,
    toUInt64(uniq(user_id))                     AS unique_payers
FROM tiktok.revenue_transactions
GROUP BY txn_date, transaction_type;

-- ---------------------------------------------------------------------------
-- Coin economy ledger (TikTok virtual currency)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS tiktok.coin_ledger
(
    event_id         UUID        DEFAULT generateUUIDv4(),
    user_id          UInt64,
    event_type       LowCardinality(String)
        COMMENT 'purchase | gift_send | gift_receive | conversion | refund | bonus | expiry',
    coins_delta      Int64       COMMENT 'Positive = credit, negative = debit',
    usd_value        Decimal(12, 4) DEFAULT 0,
    balance_after    UInt64      DEFAULT 0 COMMENT 'Running balance snapshot',
    related_txn_id   UUID        DEFAULT toUUID('00000000-0000-0000-0000-000000000000'),
    timestamp        DateTime,
    event_date       Date        MATERIALIZED toDate(timestamp)
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (user_id, timestamp)
TTL event_date + INTERVAL 730 DAY
SETTINGS index_granularity = 8192;
