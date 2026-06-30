#!/bin/bash
# Creates all service databases and their schemas.
# Runs once on first postgres container start (fresh volume).
set -e

PGUSER="${POSTGRES_USER:-postgres}"

echo "==> Creating service databases..."

psql -v ON_ERROR_STOP=1 --username "$PGUSER" <<-EOSQL
    CREATE DATABASE tiktok_auth;
    CREATE DATABASE tiktok_users;
    CREATE DATABASE tiktok_videos;
    CREATE DATABASE tiktok_interaction;
    CREATE DATABASE tiktok_feed;
    CREATE DATABASE tiktok_notifications;
    CREATE DATABASE tiktok_social;
    CREATE DATABASE tiktok_wallet;
    CREATE DATABASE tiktok_messaging;
    CREATE DATABASE tiktok_ecommerce;
    CREATE DATABASE tiktok_livestream;
    CREATE DATABASE tiktok_recommendations;
    CREATE DATABASE tiktok_reports;
    CREATE DATABASE tiktok_admin;
    CREATE DATABASE tiktok_comments;
    CREATE DATABASE tiktok_likes;
    CREATE DATABASE tiktok_analytics;
EOSQL

echo "==> Creating auth-service schema (tiktok_auth)..."

psql -v ON_ERROR_STOP=1 --username "$PGUSER" -d tiktok_auth <<-EOSQL
    CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
    CREATE EXTENSION IF NOT EXISTS "pgcrypto";

    DO \$\$ BEGIN
        IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'auth_provider') THEN
            CREATE TYPE auth_provider AS ENUM ('local', 'google', 'apple');
        END IF;
    END \$\$;

    DO \$\$ BEGIN
        IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'user_status') THEN
            CREATE TYPE user_status AS ENUM ('active', 'inactive', 'suspended', 'deleted');
        END IF;
    END \$\$;

    DO \$\$ BEGIN
        IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'otp_type') THEN
            CREATE TYPE otp_type AS ENUM ('phone_verification', 'email_verification', 'password_reset', 'login');
        END IF;
    END \$\$;

    CREATE TABLE IF NOT EXISTS users (
        id               UUID          PRIMARY KEY DEFAULT uuid_generate_v4(),
        email            TEXT          UNIQUE,
        phone            TEXT          UNIQUE,
        username         TEXT          NOT NULL UNIQUE,
        password_hash    TEXT,
        provider         auth_provider NOT NULL DEFAULT 'local',
        provider_user_id TEXT,
        email_verified   BOOLEAN       NOT NULL DEFAULT FALSE,
        phone_verified   BOOLEAN       NOT NULL DEFAULT FALSE,
        mfa_enabled      BOOLEAN       NOT NULL DEFAULT FALSE,
        mfa_secret       TEXT,
        status           user_status   NOT NULL DEFAULT 'active',
        display_name     TEXT,
        avatar_url       TEXT,
        created_at       TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
        updated_at       TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
        deleted_at       TIMESTAMPTZ
    );

    CREATE UNIQUE INDEX IF NOT EXISTS idx_users_provider_id
        ON users (provider, provider_user_id) WHERE provider_user_id IS NOT NULL;
    CREATE INDEX IF NOT EXISTS idx_users_email    ON users (email)    WHERE deleted_at IS NULL;
    CREATE INDEX IF NOT EXISTS idx_users_phone    ON users (phone)    WHERE deleted_at IS NULL;
    CREATE INDEX IF NOT EXISTS idx_users_username ON users (username) WHERE deleted_at IS NULL;

    CREATE TABLE IF NOT EXISTS sessions (
        id            UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
        user_id       UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
        refresh_token TEXT        NOT NULL UNIQUE,
        user_agent    TEXT        NOT NULL DEFAULT '',
        ip_address    TEXT        NOT NULL DEFAULT '',
        device_id     TEXT,
        is_revoked    BOOLEAN     NOT NULL DEFAULT FALSE,
        expires_at    TIMESTAMPTZ NOT NULL,
        created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
        last_seen_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
        revoked_at    TIMESTAMPTZ
    );

    CREATE INDEX IF NOT EXISTS idx_sessions_user_id       ON sessions (user_id);
    CREATE INDEX IF NOT EXISTS idx_sessions_refresh_token ON sessions (refresh_token);
    CREATE INDEX IF NOT EXISTS idx_sessions_expires_at    ON sessions (expires_at) WHERE NOT is_revoked;

    CREATE TABLE IF NOT EXISTS otp_codes (
        id         UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
        user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
        code       TEXT        NOT NULL,
        type       otp_type    NOT NULL,
        target     TEXT        NOT NULL,
        attempts   INT         NOT NULL DEFAULT 0,
        max_trials INT         NOT NULL DEFAULT 5,
        is_used    BOOLEAN     NOT NULL DEFAULT FALSE,
        expires_at TIMESTAMPTZ NOT NULL,
        created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
    );

    CREATE INDEX IF NOT EXISTS idx_otp_user_type ON otp_codes (user_id, type) WHERE NOT is_used;

    CREATE TABLE IF NOT EXISTS device_sessions (
        id             UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
        user_id        UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
        device_id      TEXT        NOT NULL,
        device_name    TEXT        NOT NULL DEFAULT '',
        platform       TEXT        NOT NULL DEFAULT '',
        ip_address     TEXT        NOT NULL DEFAULT '',
        user_agent     TEXT        NOT NULL DEFAULT '',
        is_trusted     BOOLEAN     NOT NULL DEFAULT FALSE,
        last_active_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
        created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
        revoked_at     TIMESTAMPTZ,
        CONSTRAINT uq_device_user_device UNIQUE (user_id, device_id)
    );

    CREATE OR REPLACE FUNCTION set_updated_at()
    RETURNS TRIGGER LANGUAGE plpgsql AS \$\$
    BEGIN
        NEW.updated_at = NOW();
        RETURN NEW;
    END;
    \$\$;

    DROP TRIGGER IF EXISTS trg_users_updated_at ON users;
    CREATE TRIGGER trg_users_updated_at
        BEFORE UPDATE ON users
        FOR EACH ROW EXECUTE FUNCTION set_updated_at();
EOSQL

echo "==> Creating user-service schema (tiktok_users)..."

psql -v ON_ERROR_STOP=1 --username "$PGUSER" -d tiktok_users <<-EOSQL
    CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
    CREATE EXTENSION IF NOT EXISTS "citext";

    CREATE TABLE IF NOT EXISTS users (
        id              UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
        username        CITEXT      NOT NULL UNIQUE,
        display_name    TEXT        NOT NULL DEFAULT '',
        email           TEXT        UNIQUE,
        phone           TEXT,
        avatar_url      TEXT        NOT NULL DEFAULT '',
        bio             TEXT        NOT NULL DEFAULT '',
        website_url     TEXT,
        location        TEXT,
        is_verified     BOOLEAN     NOT NULL DEFAULT FALSE,
        is_private      BOOLEAN     NOT NULL DEFAULT FALSE,
        followers_count INT         NOT NULL DEFAULT 0,
        following_count INT         NOT NULL DEFAULT 0,
        videos_count    INT         NOT NULL DEFAULT 0,
        likes_received  BIGINT      NOT NULL DEFAULT 0,
        created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
        updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
        deleted_at      TIMESTAMPTZ
    );

    CREATE INDEX IF NOT EXISTS idx_users_username ON users (username) WHERE deleted_at IS NULL;
    CREATE INDEX IF NOT EXISTS idx_users_email    ON users (email)    WHERE deleted_at IS NULL;

    CREATE TABLE IF NOT EXISTS user_settings (
        user_id            UUID    PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
        language           TEXT    NOT NULL DEFAULT 'en',
        notification_push  BOOLEAN NOT NULL DEFAULT TRUE,
        notification_email BOOLEAN NOT NULL DEFAULT TRUE,
        allow_duet         BOOLEAN NOT NULL DEFAULT TRUE,
        allow_stitch       BOOLEAN NOT NULL DEFAULT TRUE,
        allow_download     BOOLEAN NOT NULL DEFAULT TRUE,
        updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
    );

    CREATE TABLE IF NOT EXISTS user_blocks (
        blocker_id UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
        blocked_id UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
        created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
        PRIMARY KEY (blocker_id, blocked_id)
    );
EOSQL

echo "==> Creating video-service schema (tiktok_videos)..."

psql -v ON_ERROR_STOP=1 --username "$PGUSER" -d tiktok_videos <<-EOSQL
    CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

    DO \$\$ BEGIN
        IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'video_status') THEN
            CREATE TYPE video_status AS ENUM ('uploading', 'processing', 'published', 'failed', 'deleted', 'private');
        END IF;
    END \$\$;

    CREATE TABLE IF NOT EXISTS videos (
        id             UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
        user_id        UUID         NOT NULL,
        title          TEXT         NOT NULL DEFAULT '',
        description    TEXT         NOT NULL DEFAULT '',
        video_url      TEXT         NOT NULL DEFAULT '',
        hls_url        TEXT,
        thumbnail_url  TEXT,
        duration_ms    BIGINT       NOT NULL DEFAULT 0,
        file_size      BIGINT       NOT NULL DEFAULT 0,
        mime_type      TEXT         NOT NULL DEFAULT 'video/mp4',
        width          INT,
        height         INT,
        status         video_status NOT NULL DEFAULT 'uploading',
        view_count     BIGINT       NOT NULL DEFAULT 0,
        like_count     BIGINT       NOT NULL DEFAULT 0,
        comment_count  BIGINT       NOT NULL DEFAULT 0,
        share_count    BIGINT       NOT NULL DEFAULT 0,
        bookmark_count BIGINT       NOT NULL DEFAULT 0,
        is_private     BOOLEAN      NOT NULL DEFAULT FALSE,
        allow_comments BOOLEAN      NOT NULL DEFAULT TRUE,
        allow_duet     BOOLEAN      NOT NULL DEFAULT TRUE,
        allow_stitch   BOOLEAN      NOT NULL DEFAULT TRUE,
        location       TEXT,
        language       TEXT,
        created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
        updated_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
        published_at   TIMESTAMPTZ,
        deleted_at     TIMESTAMPTZ
    );

    CREATE INDEX IF NOT EXISTS idx_videos_user_id    ON videos (user_id)            WHERE deleted_at IS NULL;
    CREATE INDEX IF NOT EXISTS idx_videos_status     ON videos (status)             WHERE deleted_at IS NULL;
    CREATE INDEX IF NOT EXISTS idx_videos_created_at ON videos (created_at DESC)    WHERE deleted_at IS NULL;

    CREATE TABLE IF NOT EXISTS video_hashtags (
        video_id UUID NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
        hashtag  TEXT NOT NULL,
        PRIMARY KEY (video_id, hashtag)
    );

    CREATE INDEX IF NOT EXISTS idx_video_hashtags_tag ON video_hashtags (hashtag);

    CREATE TABLE IF NOT EXISTS upload_sessions (
        id              UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
        user_id         UUID        NOT NULL,
        file_name       TEXT        NOT NULL,
        file_size       BIGINT      NOT NULL,
        mime_type       TEXT        NOT NULL,
        total_chunks    INT         NOT NULL,
        chunk_size      BIGINT      NOT NULL,
        uploaded_chunks INT[]       NOT NULL DEFAULT '{}',
        status          TEXT        NOT NULL DEFAULT 'pending',
        storage_key     TEXT,
        expires_at      TIMESTAMPTZ NOT NULL,
        created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
        updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
    );

    CREATE INDEX IF NOT EXISTS idx_upload_sessions_user_id   ON upload_sessions (user_id);
    CREATE INDEX IF NOT EXISTS idx_upload_sessions_expires_at ON upload_sessions (expires_at);
EOSQL

echo "==> Creating interaction-service schema (tiktok_interaction)..."

psql -v ON_ERROR_STOP=1 --username "$PGUSER" -d tiktok_interaction <<-EOSQL
    CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

    CREATE TABLE IF NOT EXISTS comments (
        id                UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
        video_id          UUID        NOT NULL,
        user_id           UUID        NOT NULL,
        parent_comment_id UUID        REFERENCES comments(id) ON DELETE SET NULL,
        content           TEXT        NOT NULL,
        like_count        INT         NOT NULL DEFAULT 0,
        reply_count       INT         NOT NULL DEFAULT 0,
        is_pinned         BOOLEAN     NOT NULL DEFAULT FALSE,
        is_deleted        BOOLEAN     NOT NULL DEFAULT FALSE,
        created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
        updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
    );

    CREATE INDEX IF NOT EXISTS idx_comments_video_id  ON comments (video_id, created_at DESC) WHERE NOT is_deleted;
    CREATE INDEX IF NOT EXISTS idx_comments_user_id   ON comments (user_id)                   WHERE NOT is_deleted;
    CREATE INDEX IF NOT EXISTS idx_comments_parent_id ON comments (parent_comment_id)         WHERE parent_comment_id IS NOT NULL;

    CREATE TABLE IF NOT EXISTS likes (
        id          UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
        user_id     UUID        NOT NULL,
        target_id   UUID        NOT NULL,
        target_type TEXT        NOT NULL,
        created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
        UNIQUE (user_id, target_id, target_type)
    );

    CREATE INDEX IF NOT EXISTS idx_likes_target ON likes (target_id, target_type);
    CREATE INDEX IF NOT EXISTS idx_likes_user   ON likes (user_id);

    CREATE TABLE IF NOT EXISTS bookmarks (
        id         UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
        user_id    UUID        NOT NULL,
        video_id   UUID        NOT NULL,
        created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
        UNIQUE (user_id, video_id)
    );

    CREATE INDEX IF NOT EXISTS idx_bookmarks_user_id ON bookmarks (user_id, created_at DESC);

    CREATE TABLE IF NOT EXISTS video_views (
        id          UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
        video_id    UUID        NOT NULL,
        user_id     UUID,
        watch_ms    BIGINT      NOT NULL DEFAULT 0,
        source      TEXT        NOT NULL DEFAULT 'fyp',
        device_type TEXT,
        created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
    );

    CREATE INDEX IF NOT EXISTS idx_video_views_video_id ON video_views (video_id, created_at DESC);
EOSQL

echo "==> Creating feed-service schema (tiktok_feed)..."

psql -v ON_ERROR_STOP=1 --username "$PGUSER" -d tiktok_feed <<-EOSQL
    CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

    CREATE TABLE IF NOT EXISTS feed_items (
        id         UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
        user_id    UUID        NOT NULL,
        video_id   UUID        NOT NULL,
        score      FLOAT8      NOT NULL DEFAULT 0,
        feed_type  TEXT        NOT NULL DEFAULT 'fyp',
        expires_at TIMESTAMPTZ,
        created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
        UNIQUE (user_id, video_id, feed_type)
    );

    CREATE INDEX IF NOT EXISTS idx_feed_items_user_id   ON feed_items (user_id, feed_type, score DESC);
    CREATE INDEX IF NOT EXISTS idx_feed_items_expires_at ON feed_items (expires_at) WHERE expires_at IS NOT NULL;

    CREATE TABLE IF NOT EXISTS trending_videos (
        video_id    UUID    PRIMARY KEY,
        trend_score FLOAT8  NOT NULL DEFAULT 0,
        region      TEXT    NOT NULL DEFAULT 'global',
        category    TEXT,
        updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
    );

    CREATE INDEX IF NOT EXISTS idx_trending_region ON trending_videos (region, trend_score DESC);
EOSQL

echo "==> Creating notification-service schema (tiktok_notifications)..."

psql -v ON_ERROR_STOP=1 --username "$PGUSER" -d tiktok_notifications <<-EOSQL
    CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

    DO \$\$ BEGIN
        IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'notification_type') THEN
            CREATE TYPE notification_type AS ENUM ('like', 'comment', 'follow', 'mention', 'gift', 'live_start', 'order_update', 'system');
        END IF;
    END \$\$;

    CREATE TABLE IF NOT EXISTS notifications (
        id          UUID              PRIMARY KEY DEFAULT uuid_generate_v4(),
        user_id     UUID              NOT NULL,
        type        notification_type NOT NULL,
        title       TEXT              NOT NULL DEFAULT '',
        body        TEXT              NOT NULL DEFAULT '',
        data        JSONB,
        actor_id    UUID,
        entity_id   UUID,
        entity_type TEXT,
        is_read     BOOLEAN           NOT NULL DEFAULT FALSE,
        read_at     TIMESTAMPTZ,
        created_at  TIMESTAMPTZ       NOT NULL DEFAULT NOW()
    );

    CREATE INDEX IF NOT EXISTS idx_notifications_user_id ON notifications (user_id, created_at DESC);
    CREATE INDEX IF NOT EXISTS idx_notifications_unread  ON notifications (user_id, is_read) WHERE NOT is_read;

    CREATE TABLE IF NOT EXISTS device_tokens (
        id          UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
        user_id     UUID        NOT NULL,
        token       TEXT        NOT NULL UNIQUE,
        platform    TEXT        NOT NULL,
        app_version TEXT,
        is_active   BOOLEAN     NOT NULL DEFAULT TRUE,
        created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
        updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
    );

    CREATE INDEX IF NOT EXISTS idx_device_tokens_user_id ON device_tokens (user_id) WHERE is_active;

    CREATE TABLE IF NOT EXISTS notification_preferences (
        user_id          UUID    PRIMARY KEY,
        push_enabled     BOOLEAN NOT NULL DEFAULT TRUE,
        email_enabled    BOOLEAN NOT NULL DEFAULT TRUE,
        likes_enabled    BOOLEAN NOT NULL DEFAULT TRUE,
        comments_enabled BOOLEAN NOT NULL DEFAULT TRUE,
        follows_enabled  BOOLEAN NOT NULL DEFAULT TRUE,
        mentions_enabled BOOLEAN NOT NULL DEFAULT TRUE,
        gifts_enabled    BOOLEAN NOT NULL DEFAULT TRUE,
        live_enabled     BOOLEAN NOT NULL DEFAULT TRUE,
        updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
    );
EOSQL

echo "==> Creating social-graph-service schema (tiktok_social)..."

psql -v ON_ERROR_STOP=1 --username "$PGUSER" -d tiktok_social <<-EOSQL
    CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

    CREATE TABLE IF NOT EXISTS follows (
        follower_id  UUID        NOT NULL,
        following_id UUID        NOT NULL,
        is_mutual    BOOLEAN     NOT NULL DEFAULT FALSE,
        created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
        PRIMARY KEY (follower_id, following_id)
    );

    CREATE INDEX IF NOT EXISTS idx_follows_following_id ON follows (following_id, created_at DESC);
    CREATE INDEX IF NOT EXISTS idx_follows_follower_id  ON follows (follower_id,  created_at DESC);

    CREATE TABLE IF NOT EXISTS blocked_users (
        blocker_id UUID        NOT NULL,
        blocked_id UUID        NOT NULL,
        created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
        PRIMARY KEY (blocker_id, blocked_id)
    );

    CREATE INDEX IF NOT EXISTS idx_blocked_users_blocker ON blocked_users (blocker_id);
EOSQL

echo "==> Creating wallet-service schema (tiktok_wallet)..."

psql -v ON_ERROR_STOP=1 --username "$PGUSER" -d tiktok_wallet <<-EOSQL
    CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

    DO \$\$ BEGIN
        IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'transaction_type') THEN
            CREATE TYPE transaction_type AS ENUM (
                'deposit', 'withdrawal', 'transfer_in', 'transfer_out',
                'gift_sent', 'gift_received', 'coin_purchase', 'coin_convert', 'refund'
            );
        END IF;
    END \$\$;

    DO \$\$ BEGIN
        IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'transaction_status') THEN
            CREATE TYPE transaction_status AS ENUM ('pending', 'completed', 'failed', 'cancelled', 'refunded');
        END IF;
    END \$\$;

    CREATE TABLE IF NOT EXISTS wallets (
        id           UUID               PRIMARY KEY DEFAULT uuid_generate_v4(),
        user_id      UUID               NOT NULL UNIQUE,
        balance      DECIMAL(18,4)      NOT NULL DEFAULT 0 CHECK (balance >= 0),
        coin_balance BIGINT             NOT NULL DEFAULT 0 CHECK (coin_balance >= 0),
        currency     TEXT               NOT NULL DEFAULT 'USD',
        is_frozen    BOOLEAN            NOT NULL DEFAULT FALSE,
        created_at   TIMESTAMPTZ        NOT NULL DEFAULT NOW(),
        updated_at   TIMESTAMPTZ        NOT NULL DEFAULT NOW()
    );

    CREATE TABLE IF NOT EXISTS transactions (
        id             UUID               PRIMARY KEY DEFAULT uuid_generate_v4(),
        wallet_id      UUID               NOT NULL REFERENCES wallets(id),
        type           transaction_type   NOT NULL,
        amount         DECIMAL(18,4)      NOT NULL,
        currency       TEXT               NOT NULL DEFAULT 'USD',
        status         transaction_status NOT NULL DEFAULT 'pending',
        reference_id   TEXT,
        reference_type TEXT,
        description    TEXT,
        metadata       JSONB,
        created_at     TIMESTAMPTZ        NOT NULL DEFAULT NOW(),
        updated_at     TIMESTAMPTZ        NOT NULL DEFAULT NOW()
    );

    CREATE INDEX IF NOT EXISTS idx_transactions_wallet_id ON transactions (wallet_id, created_at DESC);
    CREATE INDEX IF NOT EXISTS idx_transactions_reference ON transactions (reference_id, reference_type);

    CREATE TABLE IF NOT EXISTS coin_packages (
        id          UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
        name        TEXT         NOT NULL,
        coins       BIGINT       NOT NULL,
        price_usd   DECIMAL(8,2) NOT NULL,
        bonus_coins BIGINT       NOT NULL DEFAULT 0,
        is_active   BOOLEAN      NOT NULL DEFAULT TRUE,
        created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
    );

    INSERT INTO coin_packages (name, coins, price_usd) VALUES
        ('Starter',    100,   0.99),
        ('Basic',      500,   4.99),
        ('Popular',   1000,   9.99),
        ('Value',     2500,  19.99),
        ('Premium',   5000,  34.99),
        ('Ultimate', 10000,  64.99)
    ON CONFLICT DO NOTHING;
EOSQL

echo "==> Creating messaging-service schema (tiktok_messaging)..."

psql -v ON_ERROR_STOP=1 --username "$PGUSER" -d tiktok_messaging <<-EOSQL
    CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

    DO \$\$ BEGIN
        IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'conversation_type') THEN
            CREATE TYPE conversation_type AS ENUM ('direct', 'group');
        END IF;
    END \$\$;

    DO \$\$ BEGIN
        IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'message_type') THEN
            CREATE TYPE message_type AS ENUM ('text', 'image', 'video', 'audio', 'file', 'video_share', 'system');
        END IF;
    END \$\$;

    CREATE TABLE IF NOT EXISTS conversations (
        id          UUID              PRIMARY KEY DEFAULT uuid_generate_v4(),
        type        conversation_type NOT NULL DEFAULT 'direct',
        name        TEXT,
        avatar_url  TEXT,
        last_msg_at TIMESTAMPTZ,
        created_at  TIMESTAMPTZ       NOT NULL DEFAULT NOW(),
        updated_at  TIMESTAMPTZ       NOT NULL DEFAULT NOW()
    );

    CREATE TABLE IF NOT EXISTS conversation_members (
        conversation_id UUID        NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
        user_id         UUID        NOT NULL,
        role            TEXT        NOT NULL DEFAULT 'member',
        joined_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
        last_read_at    TIMESTAMPTZ,
        is_muted        BOOLEAN     NOT NULL DEFAULT FALSE,
        PRIMARY KEY (conversation_id, user_id)
    );

    CREATE INDEX IF NOT EXISTS idx_conv_members_user_id ON conversation_members (user_id, joined_at DESC);

    CREATE TABLE IF NOT EXISTS messages (
        id              UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
        conversation_id UUID         NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
        sender_id       UUID         NOT NULL,
        type            message_type NOT NULL DEFAULT 'text',
        content         TEXT         NOT NULL DEFAULT '',
        media_url       TEXT,
        is_deleted      BOOLEAN      NOT NULL DEFAULT FALSE,
        reply_to_id     UUID         REFERENCES messages(id) ON DELETE SET NULL,
        created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
        updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
    );

    CREATE INDEX IF NOT EXISTS idx_messages_conv_id   ON messages (conversation_id, created_at DESC) WHERE NOT is_deleted;
    CREATE INDEX IF NOT EXISTS idx_messages_sender_id ON messages (sender_id);
EOSQL

echo "==> Creating ecommerce-service schema (tiktok_ecommerce)..."

psql -v ON_ERROR_STOP=1 --username "$PGUSER" -d tiktok_ecommerce <<-EOSQL
    CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

    DO \$\$ BEGIN
        IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'product_status') THEN
            CREATE TYPE product_status AS ENUM ('draft', 'active', 'inactive', 'deleted');
        END IF;
    END \$\$;

    DO \$\$ BEGIN
        IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'order_status') THEN
            CREATE TYPE order_status AS ENUM ('pending', 'confirmed', 'processing', 'shipped', 'delivered', 'cancelled', 'refunded');
        END IF;
    END \$\$;

    CREATE TABLE IF NOT EXISTS products (
        id             UUID           PRIMARY KEY DEFAULT uuid_generate_v4(),
        seller_id      UUID           NOT NULL,
        name           TEXT           NOT NULL,
        description    TEXT           NOT NULL DEFAULT '',
        price          DECIMAL(12,2)  NOT NULL CHECK (price >= 0),
        stock_quantity INT            NOT NULL DEFAULT 0 CHECK (stock_quantity >= 0),
        category       TEXT           NOT NULL DEFAULT 'other',
        images         TEXT[]         NOT NULL DEFAULT '{}',
        status         product_status NOT NULL DEFAULT 'draft',
        sku            TEXT           UNIQUE,
        weight_kg      DECIMAL(8,3),
        created_at     TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
        updated_at     TIMESTAMPTZ    NOT NULL DEFAULT NOW()
    );

    CREATE INDEX IF NOT EXISTS idx_products_seller_id ON products (seller_id)   WHERE status != 'deleted';
    CREATE INDEX IF NOT EXISTS idx_products_category  ON products (category)    WHERE status = 'active';
    CREATE INDEX IF NOT EXISTS idx_products_price     ON products (price)       WHERE status = 'active';

    CREATE TABLE IF NOT EXISTS product_reviews (
        id          UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
        product_id  UUID        NOT NULL REFERENCES products(id) ON DELETE CASCADE,
        user_id     UUID        NOT NULL,
        rating      SMALLINT    NOT NULL CHECK (rating BETWEEN 1 AND 5),
        content     TEXT        NOT NULL DEFAULT '',
        images      TEXT[]      NOT NULL DEFAULT '{}',
        is_verified BOOLEAN     NOT NULL DEFAULT FALSE,
        created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
        UNIQUE (product_id, user_id)
    );

    CREATE INDEX IF NOT EXISTS idx_product_reviews_product ON product_reviews (product_id, created_at DESC);

    CREATE TABLE IF NOT EXISTS cart_items (
        id         UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
        user_id    UUID        NOT NULL,
        product_id UUID        NOT NULL REFERENCES products(id),
        quantity   INT         NOT NULL DEFAULT 1 CHECK (quantity > 0),
        created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
        updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
        UNIQUE (user_id, product_id)
    );

    CREATE INDEX IF NOT EXISTS idx_cart_items_user_id ON cart_items (user_id);

    CREATE TABLE IF NOT EXISTS orders (
        id              UUID          PRIMARY KEY DEFAULT uuid_generate_v4(),
        buyer_id        UUID          NOT NULL,
        status          order_status  NOT NULL DEFAULT 'pending',
        total_amount    DECIMAL(12,2) NOT NULL,
        currency        TEXT          NOT NULL DEFAULT 'USD',
        shipping_addr   JSONB,
        tracking_number TEXT,
        notes           TEXT,
        created_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
        updated_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW()
    );

    CREATE INDEX IF NOT EXISTS idx_orders_buyer_id ON orders (buyer_id, created_at DESC);
    CREATE INDEX IF NOT EXISTS idx_orders_status   ON orders (status);

    CREATE TABLE IF NOT EXISTS order_items (
        id          UUID          PRIMARY KEY DEFAULT uuid_generate_v4(),
        order_id    UUID          NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
        product_id  UUID          NOT NULL REFERENCES products(id),
        seller_id   UUID          NOT NULL,
        quantity    INT           NOT NULL CHECK (quantity > 0),
        unit_price  DECIMAL(12,2) NOT NULL,
        total_price DECIMAL(12,2) NOT NULL,
        created_at  TIMESTAMPTZ   NOT NULL DEFAULT NOW()
    );

    CREATE INDEX IF NOT EXISTS idx_order_items_order_id  ON order_items (order_id);
    CREATE INDEX IF NOT EXISTS idx_order_items_seller_id ON order_items (seller_id);
EOSQL

echo "==> Creating livestream-service schema (tiktok_livestream)..."

psql -v ON_ERROR_STOP=1 --username "$PGUSER" -d tiktok_livestream <<-EOSQL
    CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

    DO \$\$ BEGIN
        IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'stream_status') THEN
            CREATE TYPE stream_status AS ENUM ('scheduled', 'live', 'ended', 'cancelled');
        END IF;
    END \$\$;

    CREATE TABLE IF NOT EXISTS streams (
        id            UUID          PRIMARY KEY DEFAULT uuid_generate_v4(),
        user_id       UUID          NOT NULL,
        title         TEXT          NOT NULL DEFAULT '',
        description   TEXT          NOT NULL DEFAULT '',
        thumbnail_url TEXT,
        stream_key    TEXT          NOT NULL UNIQUE DEFAULT md5(random()::text),
        rtmp_url      TEXT,
        hls_url       TEXT,
        status        stream_status NOT NULL DEFAULT 'scheduled',
        viewer_count  INT           NOT NULL DEFAULT 0,
        peak_viewers  INT           NOT NULL DEFAULT 0,
        total_viewers INT           NOT NULL DEFAULT 0,
        gift_count    INT           NOT NULL DEFAULT 0,
        coins_earned  BIGINT        NOT NULL DEFAULT 0,
        category      TEXT,
        language      TEXT          NOT NULL DEFAULT 'en',
        is_recorded   BOOLEAN       NOT NULL DEFAULT FALSE,
        recording_url TEXT,
        scheduled_at  TIMESTAMPTZ,
        started_at    TIMESTAMPTZ,
        ended_at      TIMESTAMPTZ,
        created_at    TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
        updated_at    TIMESTAMPTZ   NOT NULL DEFAULT NOW()
    );

    CREATE INDEX IF NOT EXISTS idx_streams_user_id    ON streams (user_id);
    CREATE INDEX IF NOT EXISTS idx_streams_status     ON streams (status)       WHERE status = 'live';
    CREATE INDEX IF NOT EXISTS idx_streams_started_at ON streams (started_at DESC);

    CREATE TABLE IF NOT EXISTS stream_gifts (
        id         UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
        stream_id  UUID        NOT NULL REFERENCES streams(id) ON DELETE CASCADE,
        sender_id  UUID        NOT NULL,
        gift_type  TEXT        NOT NULL,
        gift_name  TEXT        NOT NULL,
        coin_value INT         NOT NULL,
        quantity   INT         NOT NULL DEFAULT 1,
        created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
    );

    CREATE INDEX IF NOT EXISTS idx_stream_gifts_stream_id ON stream_gifts (stream_id, created_at DESC);

    CREATE TABLE IF NOT EXISTS stream_chat_messages (
        id         UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
        stream_id  UUID        NOT NULL REFERENCES streams(id) ON DELETE CASCADE,
        user_id    UUID        NOT NULL,
        content    TEXT        NOT NULL,
        type       TEXT        NOT NULL DEFAULT 'message',
        created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
    );

    CREATE INDEX IF NOT EXISTS idx_stream_chat_stream_id ON stream_chat_messages (stream_id, created_at DESC);
EOSQL

echo "==> Creating recommendation-service schema (tiktok_recommendations)..."

psql -v ON_ERROR_STOP=1 --username "$PGUSER" -d tiktok_recommendations <<-EOSQL
    CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

    CREATE TABLE IF NOT EXISTS user_preferences (
        user_id              UUID        PRIMARY KEY,
        preferred_categories TEXT[]      NOT NULL DEFAULT '{}',
        preferred_sounds     TEXT[]      NOT NULL DEFAULT '{}',
        language             TEXT        NOT NULL DEFAULT 'en',
        algorithm_version    TEXT        NOT NULL DEFAULT 'v1',
        updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
    );

    CREATE TABLE IF NOT EXISTS content_scores (
        user_id    UUID        NOT NULL,
        video_id   UUID        NOT NULL,
        score      FLOAT8      NOT NULL DEFAULT 0,
        reason     TEXT,
        expires_at TIMESTAMPTZ,
        created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
        PRIMARY KEY (user_id, video_id)
    );

    CREATE INDEX IF NOT EXISTS idx_content_scores_user_score
        ON content_scores (user_id, score DESC);
EOSQL

echo "==> Creating reporting-service schema (tiktok_reports)..."

psql -v ON_ERROR_STOP=1 --username "$PGUSER" -d tiktok_reports <<-EOSQL
    CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

    DO \$\$ BEGIN
        IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'report_status') THEN
            CREATE TYPE report_status AS ENUM ('pending', 'reviewing', 'resolved', 'dismissed');
        END IF;
    END \$\$;

    DO \$\$ BEGIN
        IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'report_entity_type') THEN
            CREATE TYPE report_entity_type AS ENUM ('video', 'user', 'comment', 'live_stream', 'message');
        END IF;
    END \$\$;

    CREATE TABLE IF NOT EXISTS reports (
        id           UUID               PRIMARY KEY DEFAULT uuid_generate_v4(),
        reporter_id  UUID               NOT NULL,
        entity_id    UUID               NOT NULL,
        entity_type  report_entity_type NOT NULL,
        reason       TEXT               NOT NULL,
        description  TEXT               NOT NULL DEFAULT '',
        status       report_status      NOT NULL DEFAULT 'pending',
        reviewed_by  UUID,
        review_notes TEXT,
        resolved_at  TIMESTAMPTZ,
        created_at   TIMESTAMPTZ        NOT NULL DEFAULT NOW(),
        updated_at   TIMESTAMPTZ        NOT NULL DEFAULT NOW()
    );

    CREATE INDEX IF NOT EXISTS idx_reports_entity     ON reports (entity_id, entity_type);
    CREATE INDEX IF NOT EXISTS idx_reports_reporter   ON reports (reporter_id);
    CREATE INDEX IF NOT EXISTS idx_reports_status     ON reports (status, created_at DESC);
EOSQL

echo "==> Creating admin-service schema (tiktok_admin)..."

psql -v ON_ERROR_STOP=1 --username "$PGUSER" -d tiktok_admin <<-EOSQL
    CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

    DO \$\$ BEGIN
        IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'admin_role') THEN
            CREATE TYPE admin_role AS ENUM ('super_admin', 'admin', 'moderator', 'analyst', 'support');
        END IF;
    END \$\$;

    CREATE TABLE IF NOT EXISTS admin_users (
        id            UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
        username      TEXT        NOT NULL UNIQUE,
        email         TEXT        NOT NULL UNIQUE,
        password_hash TEXT        NOT NULL,
        role          admin_role  NOT NULL DEFAULT 'support',
        is_active     BOOLEAN     NOT NULL DEFAULT TRUE,
        last_login_at TIMESTAMPTZ,
        created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
        updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
    );

    CREATE TABLE IF NOT EXISTS audit_logs (
        id          UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
        admin_id    UUID        NOT NULL REFERENCES admin_users(id),
        action      TEXT        NOT NULL,
        entity_type TEXT        NOT NULL,
        entity_id   TEXT,
        old_values  JSONB,
        new_values  JSONB,
        ip_address  TEXT,
        user_agent  TEXT,
        created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
    );

    CREATE INDEX IF NOT EXISTS idx_audit_logs_admin_id  ON audit_logs (admin_id, created_at DESC);
    CREATE INDEX IF NOT EXISTS idx_audit_logs_entity    ON audit_logs (entity_type, entity_id);
    CREATE INDEX IF NOT EXISTS idx_audit_logs_created   ON audit_logs (created_at DESC);

    CREATE TABLE IF NOT EXISTS moderation_actions (
        id          UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
        admin_id    UUID        NOT NULL REFERENCES admin_users(id),
        target_id   UUID        NOT NULL,
        target_type TEXT        NOT NULL,
        action      TEXT        NOT NULL,
        reason      TEXT        NOT NULL DEFAULT '',
        expires_at  TIMESTAMPTZ,
        created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
    );

    CREATE INDEX IF NOT EXISTS idx_moderation_target ON moderation_actions (target_id, target_type);
EOSQL

echo "==> Creating comment-service schema (tiktok_comments)..."

psql -v ON_ERROR_STOP=1 --username "$PGUSER" -d tiktok_comments <<-EOSQL
    CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

    CREATE TABLE IF NOT EXISTS comments (
        id                UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
        video_id          UUID        NOT NULL,
        user_id           UUID        NOT NULL,
        parent_comment_id UUID        REFERENCES comments(id) ON DELETE CASCADE,
        content           TEXT        NOT NULL,
        like_count        INT         NOT NULL DEFAULT 0,
        reply_count       INT         NOT NULL DEFAULT 0,
        is_deleted        BOOLEAN     NOT NULL DEFAULT FALSE,
        created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
        updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
    );

    CREATE INDEX IF NOT EXISTS idx_comments_video_id ON comments (video_id, created_at DESC) WHERE NOT is_deleted;
    CREATE INDEX IF NOT EXISTS idx_comments_user_id  ON comments (user_id)                   WHERE NOT is_deleted;
EOSQL

echo "==> Creating like-service schema (tiktok_likes)..."

psql -v ON_ERROR_STOP=1 --username "$PGUSER" -d tiktok_likes <<-EOSQL
    CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

    CREATE TABLE IF NOT EXISTS likes (
        id          UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
        user_id     UUID        NOT NULL,
        target_id   UUID        NOT NULL,
        target_type TEXT        NOT NULL DEFAULT 'video',
        created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
        UNIQUE (user_id, target_id, target_type)
    );

    CREATE INDEX IF NOT EXISTS idx_likes_target  ON likes (target_id, target_type);
    CREATE INDEX IF NOT EXISTS idx_likes_user_id ON likes (user_id, created_at DESC);
EOSQL

echo "==> Database initialization complete!"
