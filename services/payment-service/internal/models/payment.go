package models

import (
	"time"

	"github.com/google/uuid"
)

// ---------- Payment ----------

// PaymentStatus mirrors Stripe's PaymentIntent status lifecycle.
type PaymentStatus string

const (
	PaymentStatusRequiresPaymentMethod PaymentStatus = "requires_payment_method"
	PaymentStatusRequiresConfirmation  PaymentStatus = "requires_confirmation"
	PaymentStatusRequiresAction        PaymentStatus = "requires_action"
	PaymentStatusProcessing            PaymentStatus = "processing"
	PaymentStatusSucceeded             PaymentStatus = "succeeded"
	PaymentStatusCanceled              PaymentStatus = "canceled"
	PaymentStatusFailed                PaymentStatus = "failed"
	PaymentStatusRefunded              PaymentStatus = "refunded"
)

// PaymentType classifies the purpose of a payment.
type PaymentType string

const (
	PaymentTypeCoinPurchase  PaymentType = "coin_purchase"
	PaymentTypeSubscription  PaymentType = "subscription"
	PaymentTypeWithdrawal    PaymentType = "withdrawal" // internal Stripe Connect payout
)

// Payment records every charge or payout attempted via Stripe.
type Payment struct {
	ID              uuid.UUID     `db:"id"               json:"id"`
	UserID          uuid.UUID     `db:"user_id"          json:"user_id"`
	Type            PaymentType   `db:"type"             json:"type"`
	Status          PaymentStatus `db:"status"           json:"status"`
	// AmountCents is the charge/payout amount in the smallest currency unit.
	AmountCents     int64         `db:"amount_cents"     json:"amount_cents"`
	Currency        string        `db:"currency"         json:"currency"`
	// StripePaymentIntentID is the pi_... identifier returned by Stripe.
	StripePaymentIntentID string   `db:"stripe_payment_intent_id" json:"stripe_payment_intent_id,omitempty"`
	// StripeChargeID is the ch_... identifier once a charge succeeds.
	StripeChargeID  string        `db:"stripe_charge_id"         json:"stripe_charge_id,omitempty"`
	// StripeCustomerID is the cus_... identifier for the paying user.
	StripeCustomerID string       `db:"stripe_customer_id"       json:"stripe_customer_id,omitempty"`
	// IdempotencyKey is used to deduplicate Stripe API calls and DB inserts.
	IdempotencyKey  string        `db:"idempotency_key"  json:"idempotency_key"`
	// Metadata stores additional context (package_id, creator_id, etc.).
	Metadata        map[string]string `db:"metadata"     json:"metadata,omitempty"`
	Description     string        `db:"description"      json:"description"`
	FailureCode     string        `db:"failure_code"     json:"failure_code,omitempty"`
	FailureMessage  string        `db:"failure_message"  json:"failure_message,omitempty"`
	// RefundedAmountCents is set on partial or full refunds.
	RefundedAmountCents int64     `db:"refunded_amount_cents" json:"refunded_amount_cents"`
	StripeRefundID  string        `db:"stripe_refund_id"      json:"stripe_refund_id,omitempty"`
	CreatedAt       time.Time     `db:"created_at"       json:"created_at"`
	UpdatedAt       time.Time     `db:"updated_at"       json:"updated_at"`
}

// ---------- PaymentMethod ----------

// PaymentMethodType enumerates supported Stripe payment method types.
type PaymentMethodType string

const (
	PaymentMethodTypeCard   PaymentMethodType = "card"
	PaymentMethodTypeWallet PaymentMethodType = "wallet" // Apple Pay / Google Pay
)

// CardBrand enumerates common card brands returned by Stripe.
type CardBrand string

const (
	CardBrandVisa       CardBrand = "visa"
	CardBrandMastercard CardBrand = "mastercard"
	CardBrandAmex       CardBrand = "amex"
	CardBrandDiscover   CardBrand = "discover"
	CardBrandUnknown    CardBrand = "unknown"
)

// PaymentMethod stores a tokenised Stripe payment method attached to a user.
type PaymentMethod struct {
	ID                    uuid.UUID         `db:"id"                      json:"id"`
	UserID                uuid.UUID         `db:"user_id"                 json:"user_id"`
	StripeCustomerID      string            `db:"stripe_customer_id"      json:"stripe_customer_id"`
	StripePaymentMethodID string            `db:"stripe_payment_method_id" json:"stripe_payment_method_id"`
	Type                  PaymentMethodType `db:"type"                    json:"type"`
	// Card-specific fields (populated when Type == "card").
	CardBrand         CardBrand `db:"card_brand"         json:"card_brand,omitempty"`
	CardLast4         string    `db:"card_last4"         json:"card_last4,omitempty"`
	CardExpMonth      int       `db:"card_exp_month"     json:"card_exp_month,omitempty"`
	CardExpYear       int       `db:"card_exp_year"      json:"card_exp_year,omitempty"`
	CardCountry       string    `db:"card_country"       json:"card_country,omitempty"`
	CardFingerprint   string    `db:"card_fingerprint"   json:"card_fingerprint,omitempty"`
	IsDefault         bool      `db:"is_default"         json:"is_default"`
	CreatedAt         time.Time `db:"created_at"         json:"created_at"`
	UpdatedAt         time.Time `db:"updated_at"         json:"updated_at"`
}

// ---------- Payout ----------

// PayoutStatus mirrors Stripe's Payout status.
type PayoutStatus string

const (
	PayoutStatusPending    PayoutStatus = "pending"
	PayoutStatusInTransit  PayoutStatus = "in_transit"
	PayoutStatusPaid       PayoutStatus = "paid"
	PayoutStatusFailed     PayoutStatus = "failed"
	PayoutStatusCanceled   PayoutStatus = "canceled"
)

// Payout records a Stripe Connect payout to a creator's bank account.
type Payout struct {
	ID                  uuid.UUID    `db:"id"                    json:"id"`
	CreatorUserID       uuid.UUID    `db:"creator_user_id"       json:"creator_user_id"`
	// StripeAccountID is the creator's Stripe Connect account (acct_...).
	StripeAccountID     string       `db:"stripe_account_id"     json:"stripe_account_id"`
	StripePayoutID      string       `db:"stripe_payout_id"      json:"stripe_payout_id,omitempty"`
	AmountCents         int64        `db:"amount_cents"          json:"amount_cents"`
	Currency            string       `db:"currency"              json:"currency"`
	Status              PayoutStatus `db:"status"                json:"status"`
	Description         string       `db:"description"           json:"description"`
	FailureCode         string       `db:"failure_code"          json:"failure_code,omitempty"`
	FailureMessage      string       `db:"failure_message"       json:"failure_message,omitempty"`
	// WalletPayoutRequestID links back to the wallet-service's payout_requests table.
	WalletPayoutRequestID string     `db:"wallet_payout_request_id" json:"wallet_payout_request_id,omitempty"`
	ArrivalDate         *time.Time   `db:"arrival_date"          json:"arrival_date,omitempty"`
	CreatedAt           time.Time    `db:"created_at"            json:"created_at"`
	UpdatedAt           time.Time    `db:"updated_at"            json:"updated_at"`
}

// ---------- StripeCustomer ----------

// StripeCustomer maps an internal user ID to a Stripe Customer ID.
// One-to-one relationship stored in the payments database.
type StripeCustomer struct {
	ID               uuid.UUID `db:"id"                json:"id"`
	UserID           uuid.UUID `db:"user_id"           json:"user_id"`
	StripeCustomerID string    `db:"stripe_customer_id" json:"stripe_customer_id"`
	Email            string    `db:"email"             json:"email"`
	Name             string    `db:"name"              json:"name"`
	CreatedAt        time.Time `db:"created_at"        json:"created_at"`
	UpdatedAt        time.Time `db:"updated_at"        json:"updated_at"`
}

// ---------- WebhookEvent ----------

// WebhookEvent stores raw Stripe webhook payloads for idempotency and auditing.
type WebhookEvent struct {
	ID             uuid.UUID `db:"id"              json:"id"`
	StripeEventID  string    `db:"stripe_event_id" json:"stripe_event_id"` // evt_...
	EventType      string    `db:"event_type"      json:"event_type"`       // e.g. payment_intent.succeeded
	ProcessedAt    time.Time `db:"processed_at"    json:"processed_at"`
	RawPayload     []byte    `db:"raw_payload"     json:"-"`
}
