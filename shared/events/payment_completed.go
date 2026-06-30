package events

import "time"

const TopicPaymentCompleted = "payment.completed"

type PaymentCompleted struct {
	EventID         string    `json:"event_id"`
	OccurredAt      time.Time `json:"occurred_at"`
	PaymentID       string    `json:"payment_id"`
	OrderID         string    `json:"order_id,omitempty"`
	UserID          string    `json:"user_id"`
	AmountMicroUSD  int64     `json:"amount_micro_usd"`
	Currency        string    `json:"currency"`
	PaymentMethod   string    `json:"payment_method"`
	StripePaymentID string    `json:"stripe_payment_id,omitempty"`
	Status          string    `json:"status"`
}
