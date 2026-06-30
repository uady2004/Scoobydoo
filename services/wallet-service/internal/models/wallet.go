package models

import (
	"time"

	"github.com/google/uuid"
)

// ---------- Wallet ----------

// Wallet is the root financial account for a user. Each user has exactly one wallet.
type Wallet struct {
	ID        uuid.UUID `db:"id"         json:"id"`
	UserID    uuid.UUID `db:"user_id"    json:"user_id"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// ---------- CoinBalance ----------

// CoinBalance tracks a user's spendable coin balance and a creator's diamond
// (earnings) balance separately. Only one row per wallet.
type CoinBalance struct {
	ID              uuid.UUID `db:"id"               json:"id"`
	WalletID        uuid.UUID `db:"wallet_id"        json:"wallet_id"`
	UserID          uuid.UUID `db:"user_id"          json:"user_id"`
	Coins           int64     `db:"coins"            json:"coins"`            // purchased coins
	Diamonds        int64     `db:"diamonds"         json:"diamonds"`         // earned as creator
	LifetimeCoins   int64     `db:"lifetime_coins"   json:"lifetime_coins"`   // cumulative coins ever bought
	LifetimeDiamonds int64    `db:"lifetime_diamonds" json:"lifetime_diamonds"` // cumulative diamonds ever earned
	UpdatedAt       time.Time `db:"updated_at"       json:"updated_at"`
}

// ---------- Transaction ----------

// TransactionType enumerates all movement types for the ledger.
type TransactionType string

const (
	TxTypeCoinPurchase     TransactionType = "coin_purchase"     // user bought coins via Stripe
	TxTypeGiftSent         TransactionType = "gift_sent"         // user spent coins gifting a creator
	TxTypeGiftReceived     TransactionType = "gift_received"     // creator received diamonds from gift
	TxTypeTip              TransactionType = "tip"               // user sent a tip
	TxTypeTipReceived      TransactionType = "tip_received"      // creator received tip diamonds
	TxTypeSubscription     TransactionType = "subscription"      // user subscribed to creator
	TxTypeSubReceived      TransactionType = "subscription_received" // creator received sub diamonds
	TxTypeWithdrawal       TransactionType = "withdrawal"        // creator cashed out diamonds → USD
	TxTypeRefund           TransactionType = "refund"            // refund back to coin balance
)

// TransactionStatus tracks the lifecycle state of a transaction.
type TransactionStatus string

const (
	TxStatusPending   TransactionStatus = "pending"
	TxStatusCompleted TransactionStatus = "completed"
	TxStatusFailed    TransactionStatus = "failed"
	TxStatusRefunded  TransactionStatus = "refunded"
)

// Transaction is an immutable ledger entry recording every balance movement.
type Transaction struct {
	ID              uuid.UUID         `db:"id"               json:"id"`
	WalletID        uuid.UUID         `db:"wallet_id"        json:"wallet_id"`
	UserID          uuid.UUID         `db:"user_id"          json:"user_id"`
	Type            TransactionType   `db:"type"             json:"type"`
	Status          TransactionStatus `db:"status"           json:"status"`
	// CoinAmount is positive for credits, negative for debits (in coins).
	CoinAmount      int64             `db:"coin_amount"      json:"coin_amount"`
	// DiamondAmount is positive for credits, negative for debits (in diamonds).
	DiamondAmount   int64             `db:"diamond_amount"   json:"diamond_amount"`
	// USDCents is set for coin purchases and withdrawals (in USD cents).
	USDCents        int64             `db:"usd_cents"        json:"usd_cents"`
	// RelatedUserID is the counterparty (e.g. creator receiving gift).
	RelatedUserID   *uuid.UUID        `db:"related_user_id"  json:"related_user_id,omitempty"`
	// ReferenceID is an external ID (Stripe payment intent, gift ID, etc.).
	ReferenceID     string            `db:"reference_id"     json:"reference_id"`
	// IdempotencyKey prevents duplicate processing.
	IdempotencyKey  string            `db:"idempotency_key"  json:"idempotency_key"`
	Description     string            `db:"description"      json:"description"`
	Metadata        map[string]string `db:"metadata"         json:"metadata,omitempty"`
	CreatedAt       time.Time         `db:"created_at"       json:"created_at"`
	UpdatedAt       time.Time         `db:"updated_at"       json:"updated_at"`
}

// ---------- Gift ----------

// GiftType represents the virtual gift item sent during a livestream or video.
type GiftType string

const (
	GiftTypeRose      GiftType = "rose"
	GiftTypeDiamond   GiftType = "diamond"
	GiftTypeRocket    GiftType = "rocket"
	GiftTypeLion      GiftType = "lion"
	GiftTypeUniverse  GiftType = "universe"
	GiftTypeCustom    GiftType = "custom"
)

// CoinCosts maps each gift type to its coin cost.
var GiftCoinCosts = map[GiftType]int64{
	GiftTypeRose:     1,
	GiftTypeDiamond:  5,
	GiftTypeRocket:   20,
	GiftTypeLion:     199,
	GiftTypeUniverse: 34999,
	GiftTypeCustom:   1,
}

// Gift records a single gift transaction from a sender to a creator.
type Gift struct {
	ID              uuid.UUID `db:"id"               json:"id"`
	SenderUserID    uuid.UUID `db:"sender_user_id"   json:"sender_user_id"`
	ReceiverUserID  uuid.UUID `db:"receiver_user_id" json:"receiver_user_id"`
	GiftType        GiftType  `db:"gift_type"        json:"gift_type"`
	Quantity        int32     `db:"quantity"         json:"quantity"`
	CoinCost        int64     `db:"coin_cost"        json:"coin_cost"`        // total coins deducted from sender
	DiamondEarned   int64     `db:"diamond_earned"   json:"diamond_earned"`   // total diamonds credited to receiver
	// LivestreamID is optional — set when gift was sent during a live broadcast.
	LivestreamID    *uuid.UUID `db:"livestream_id"   json:"livestream_id,omitempty"`
	VideoID         *uuid.UUID `db:"video_id"        json:"video_id,omitempty"`
	// TransactionID links to the sender's ledger entry.
	TransactionID   uuid.UUID `db:"transaction_id"   json:"transaction_id"`
	Message         string    `db:"message"          json:"message"`
	CreatedAt       time.Time `db:"created_at"       json:"created_at"`
}

// ---------- Subscription ----------

// SubscriptionTier defines the level of creator subscription.
type SubscriptionTier string

const (
	SubTierBasic   SubscriptionTier = "basic"
	SubTierPremium SubscriptionTier = "premium"
	SubTierVIP     SubscriptionTier = "vip"
)

// SubTierCoinCosts maps tier → monthly coin cost.
var SubTierCoinCosts = map[SubscriptionTier]int64{
	SubTierBasic:   100,
	SubTierPremium: 500,
	SubTierVIP:     1500,
}

// SubscriptionStatus tracks whether the subscription is active.
type SubscriptionStatus string

const (
	SubStatusActive    SubscriptionStatus = "active"
	SubStatusExpired   SubscriptionStatus = "expired"
	SubStatusCancelled SubscriptionStatus = "cancelled"
)

// Subscription records an active or historical subscription between a fan and creator.
type Subscription struct {
	ID             uuid.UUID          `db:"id"              json:"id"`
	SubscriberID   uuid.UUID          `db:"subscriber_id"   json:"subscriber_id"`
	CreatorID      uuid.UUID          `db:"creator_id"      json:"creator_id"`
	Tier           SubscriptionTier   `db:"tier"            json:"tier"`
	Status         SubscriptionStatus `db:"status"          json:"status"`
	CoinCost       int64              `db:"coin_cost"       json:"coin_cost"`
	DiamondEarned  int64              `db:"diamond_earned"  json:"diamond_earned"`
	// StripeSubscriptionID is set when the subscription is backed by Stripe recurring billing.
	StripeSubscriptionID string         `db:"stripe_subscription_id" json:"stripe_subscription_id,omitempty"`
	StartsAt       time.Time          `db:"starts_at"       json:"starts_at"`
	ExpiresAt      time.Time          `db:"expires_at"      json:"expires_at"`
	RenewedAt      *time.Time         `db:"renewed_at"      json:"renewed_at,omitempty"`
	CancelledAt    *time.Time         `db:"cancelled_at"    json:"cancelled_at,omitempty"`
	TransactionID  uuid.UUID          `db:"transaction_id"  json:"transaction_id"`
	CreatedAt      time.Time          `db:"created_at"      json:"created_at"`
	UpdatedAt      time.Time          `db:"updated_at"      json:"updated_at"`
}

// ---------- CoinPackage ----------

// CoinPackage is a purchasable bundle of coins.
type CoinPackage struct {
	ID         string `json:"id"`
	Coins      int64  `json:"coins"`
	PriceCents int64  `json:"price_cents"` // in USD cents
	BonusCoins int64  `json:"bonus_coins"`
	Label      string `json:"label"`
}

// DefaultCoinPackages is the catalogue shown to users in the app.
var DefaultCoinPackages = []CoinPackage{
	{ID: "pkg_100",   Coins: 100,   PriceCents: 99,    BonusCoins: 0,    Label: "100 coins"},
	{ID: "pkg_500",   Coins: 500,   PriceCents: 499,   BonusCoins: 25,   Label: "500 coins + 25 bonus"},
	{ID: "pkg_1000",  Coins: 1000,  PriceCents: 999,   BonusCoins: 75,   Label: "1000 coins + 75 bonus"},
	{ID: "pkg_5000",  Coins: 5000,  PriceCents: 4999,  BonusCoins: 500,  Label: "5000 coins + 500 bonus"},
	{ID: "pkg_10000", Coins: 10000, PriceCents: 9999,  BonusCoins: 1500, Label: "10000 coins + 1500 bonus"},
}

// ---------- PayoutRequest ----------

// PayoutStatus tracks the lifecycle of a creator withdrawal.
type PayoutStatus string

const (
	PayoutStatusPending    PayoutStatus = "pending"
	PayoutStatusProcessing PayoutStatus = "processing"
	PayoutStatusCompleted  PayoutStatus = "completed"
	PayoutStatusFailed     PayoutStatus = "failed"
)

// PayoutRequest is created when a creator requests to withdraw their diamond earnings.
type PayoutRequest struct {
	ID              uuid.UUID    `db:"id"               json:"id"`
	CreatorUserID   uuid.UUID    `db:"creator_user_id"  json:"creator_user_id"`
	DiamondAmount   int64        `db:"diamond_amount"   json:"diamond_amount"`
	USDCents        int64        `db:"usd_cents"        json:"usd_cents"`
	Status          PayoutStatus `db:"status"           json:"status"`
	StripePayoutID  string       `db:"stripe_payout_id" json:"stripe_payout_id,omitempty"`
	FailureReason   string       `db:"failure_reason"   json:"failure_reason,omitempty"`
	TransactionID   uuid.UUID    `db:"transaction_id"   json:"transaction_id"`
	RequestedAt     time.Time    `db:"requested_at"     json:"requested_at"`
	ProcessedAt     *time.Time   `db:"processed_at"     json:"processed_at,omitempty"`
}
