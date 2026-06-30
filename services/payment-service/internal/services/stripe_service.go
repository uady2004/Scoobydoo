package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/charge"
	"github.com/stripe/stripe-go/v76/customer"
	"github.com/stripe/stripe-go/v76/paymentintent"
	"github.com/stripe/stripe-go/v76/paymentmethod"
	"github.com/stripe/stripe-go/v76/payout"
	"github.com/stripe/stripe-go/v76/refund"
	"github.com/stripe/stripe-go/v76/webhook"
	"go.uber.org/zap"

	"github.com/tiktok-clone/payment-service/internal/config"
	"github.com/tiktok-clone/payment-service/internal/models"
)

// ---------- error sentinels ----------

var (
	ErrWebhookSignature   = errors.New("invalid webhook signature")
	ErrUnknownEvent       = errors.New("unhandled webhook event type")
	ErrPaymentNotFound    = errors.New("payment not found")
	ErrCustomerNotFound   = errors.New("stripe customer not found")
	ErrAlreadyRefunded    = errors.New("payment already fully refunded")
)

// ---------- DTOs ----------

// CreatePaymentIntentInput is the input for CreatePaymentIntent.
type CreatePaymentIntentInput struct {
	UserID           uuid.UUID
	StripeCustomerID string
	AmountCents      int64
	Currency         string
	PaymentMethodID  string
	Description      string
	// IdempotencyKey must be unique per logical payment attempt.
	IdempotencyKey   string
	// Metadata is stored on the PaymentIntent in Stripe for reconciliation.
	Metadata         map[string]string
	// Confirm=true will attempt to confirm the intent immediately.
	Confirm          bool
}

// CreatePaymentIntentResult contains the result of CreatePaymentIntent.
type CreatePaymentIntentResult struct {
	PaymentIntentID string
	ClientSecret    string
	Status          string
}

// WebhookResult is the structured outcome of handling a Stripe webhook event.
type WebhookResult struct {
	EventID   string
	EventType string
	// PaymentID is set when the event relates to a PaymentIntent.
	PaymentID string
	Status    string
	Handled   bool
}

// ---------- StripeService ----------

// StripeService wraps all direct Stripe API operations.
type StripeService interface {
	// PaymentIntent lifecycle.
	CreatePaymentIntent(ctx context.Context, in CreatePaymentIntentInput) (*CreatePaymentIntentResult, error)
	ConfirmPayment(ctx context.Context, paymentIntentID, paymentMethodID, idempotencyKey string) (*stripe.PaymentIntent, error)
	CancelPaymentIntent(ctx context.Context, paymentIntentID string) (*stripe.PaymentIntent, error)

	// Customers.
	CreateCustomer(ctx context.Context, userID uuid.UUID, email, name string) (*stripe.Customer, error)
	GetCustomer(ctx context.Context, stripeCustomerID string) (*stripe.Customer, error)

	// Payment methods.
	AddPaymentMethod(ctx context.Context, stripeCustomerID, paymentMethodID string) (*stripe.PaymentMethod, error)
	ListPaymentMethods(ctx context.Context, stripeCustomerID string) ([]*stripe.PaymentMethod, error)
	DetachPaymentMethod(ctx context.Context, paymentMethodID string) error

	// Payouts (Stripe Connect).
	CreatePayout(ctx context.Context, stripeAccountID string, amountCents int64, currency, description, idempotencyKey string) (*stripe.Payout, error)

	// Refunds.
	RefundPayment(ctx context.Context, chargeID string, amountCents int64, idempotencyKey string) (*stripe.Refund, error)

	// Webhooks.
	HandleWebhook(ctx context.Context, r *http.Request) (*WebhookResult, error)
	ConstructEvent(payload []byte, sigHeader string) (stripe.Event, error)
}

type stripeService struct {
	cfg    *config.Config
	logger *zap.Logger
}

// NewStripeService creates a production StripeService with the configured API key.
func NewStripeService(cfg *config.Config, logger *zap.Logger) StripeService {
	// Set the global Stripe key used by all stripe-go API calls.
	stripe.Key = cfg.Stripe.SecretKey
	stripe.GetBackend(stripe.APIBackend).(interface{ SetMaxNetworkRetries(int64) }).SetMaxNetworkRetries(int64(cfg.Stripe.MaxNetworkRetries))

	return &stripeService{cfg: cfg, logger: logger}
}

// ---------- PaymentIntent ----------

// CreatePaymentIntent creates a Stripe PaymentIntent with idempotency.
// If Confirm=true the intent is confirmed in the same call (one-step payment).
func (s *stripeService) CreatePaymentIntent(ctx context.Context, in CreatePaymentIntentInput) (*CreatePaymentIntentResult, error) {
	currency := in.Currency
	if currency == "" {
		currency = s.cfg.Stripe.Currency
	}

	// Build Stripe metadata.
	meta := map[string]string{
		"user_id":         in.UserID.String(),
		"idempotency_key": in.IdempotencyKey,
	}
	for k, v := range in.Metadata {
		meta[k] = v
	}

	params := &stripe.PaymentIntentParams{
		Amount:      stripe.Int64(in.AmountCents),
		Currency:    stripe.String(currency),
		Description: stripe.String(in.Description),
		Metadata:    convertMeta(meta),
	}
	params.SetIdempotencyKey(in.IdempotencyKey)
	params.Context = ctx

	if in.StripeCustomerID != "" {
		params.Customer = stripe.String(in.StripeCustomerID)
	}
	if in.PaymentMethodID != "" {
		params.PaymentMethod = stripe.String(in.PaymentMethodID)
	}
	if in.Confirm {
		params.Confirm = stripe.Bool(true)
		params.ReturnURL = stripe.String("https://tiktokclone.app/payment/return")
	}
	if s.cfg.Stripe.StatementDescriptor != "" {
		params.StatementDescriptor = stripe.String(s.cfg.Stripe.StatementDescriptor)
	}

	pi, err := paymentintent.New(params)
	if err != nil {
		return nil, fmt.Errorf("CreatePaymentIntent: stripe: %w", wrapStripeError(err))
	}

	s.logger.Info("stripe: payment intent created",
		zap.String("payment_intent_id", pi.ID),
		zap.String("status", string(pi.Status)),
		zap.String("user_id", in.UserID.String()),
	)

	return &CreatePaymentIntentResult{
		PaymentIntentID: pi.ID,
		ClientSecret:    pi.ClientSecret,
		Status:          string(pi.Status),
	}, nil
}

// ConfirmPayment explicitly confirms a PaymentIntent that requires confirmation.
func (s *stripeService) ConfirmPayment(ctx context.Context, paymentIntentID, paymentMethodID, idempotencyKey string) (*stripe.PaymentIntent, error) {
	params := &stripe.PaymentIntentConfirmParams{
		PaymentMethod: stripe.String(paymentMethodID),
		ReturnURL:     stripe.String("https://tiktokclone.app/payment/return"),
	}
	params.SetIdempotencyKey(idempotencyKey + "_confirm")
	params.Context = ctx

	pi, err := paymentintent.Confirm(paymentIntentID, params)
	if err != nil {
		return nil, fmt.Errorf("ConfirmPayment: stripe: %w", wrapStripeError(err))
	}

	s.logger.Info("stripe: payment intent confirmed",
		zap.String("payment_intent_id", pi.ID),
		zap.String("status", string(pi.Status)),
	)
	return pi, nil
}

// CancelPaymentIntent cancels an uncaptured PaymentIntent.
func (s *stripeService) CancelPaymentIntent(ctx context.Context, paymentIntentID string) (*stripe.PaymentIntent, error) {
	params := &stripe.PaymentIntentCancelParams{}
	params.Context = ctx
	pi, err := paymentintent.Cancel(paymentIntentID, params)
	if err != nil {
		return nil, fmt.Errorf("CancelPaymentIntent: stripe: %w", wrapStripeError(err))
	}
	return pi, nil
}

// ---------- Customers ----------

// CreateCustomer creates a Stripe Customer object for a new user.
func (s *stripeService) CreateCustomer(ctx context.Context, userID uuid.UUID, email, name string) (*stripe.Customer, error) {
	params := &stripe.CustomerParams{
		Email: stripe.String(email),
		Name:  stripe.String(name),
		Metadata: map[string]string{
			"user_id": userID.String(),
		},
	}
	// Idempotency key: customer creation is idempotent per user_id.
	params.SetIdempotencyKey("create_customer_" + userID.String())
	params.Context = ctx

	cust, err := customer.New(params)
	if err != nil {
		return nil, fmt.Errorf("CreateCustomer: stripe: %w", wrapStripeError(err))
	}
	s.logger.Info("stripe: customer created",
		zap.String("stripe_customer_id", cust.ID),
		zap.String("user_id", userID.String()),
	)
	return cust, nil
}

// GetCustomer retrieves a Stripe Customer by its ID.
func (s *stripeService) GetCustomer(ctx context.Context, stripeCustomerID string) (*stripe.Customer, error) {
	params := &stripe.CustomerParams{}
	params.Context = ctx
	cust, err := customer.Get(stripeCustomerID, params)
	if err != nil {
		return nil, fmt.Errorf("GetCustomer: stripe: %w", wrapStripeError(err))
	}
	return cust, nil
}

// ---------- Payment Methods ----------

// AddPaymentMethod attaches a Stripe PaymentMethod to a Customer.
func (s *stripeService) AddPaymentMethod(ctx context.Context, stripeCustomerID, paymentMethodID string) (*stripe.PaymentMethod, error) {
	params := &stripe.PaymentMethodAttachParams{
		Customer: stripe.String(stripeCustomerID),
	}
	params.SetIdempotencyKey("attach_pm_" + stripeCustomerID + "_" + paymentMethodID)
	params.Context = ctx

	pm, err := paymentmethod.Attach(paymentMethodID, params)
	if err != nil {
		return nil, fmt.Errorf("AddPaymentMethod: stripe: %w", wrapStripeError(err))
	}
	s.logger.Info("stripe: payment method attached",
		zap.String("stripe_customer_id", stripeCustomerID),
		zap.String("payment_method_id", paymentMethodID),
	)
	return pm, nil
}

// ListPaymentMethods returns all card payment methods for a Customer.
func (s *stripeService) ListPaymentMethods(ctx context.Context, stripeCustomerID string) ([]*stripe.PaymentMethod, error) {
	params := &stripe.PaymentMethodListParams{
		Customer: stripe.String(stripeCustomerID),
		Type:     stripe.String(string(stripe.PaymentMethodTypeCard)),
	}
	params.Context = ctx

	var methods []*stripe.PaymentMethod
	iter := paymentmethod.List(params)
	for iter.Next() {
		methods = append(methods, iter.PaymentMethod())
	}
	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("ListPaymentMethods: stripe: %w", wrapStripeError(err))
	}
	return methods, nil
}

// DetachPaymentMethod removes a payment method from its customer.
func (s *stripeService) DetachPaymentMethod(ctx context.Context, paymentMethodID string) error {
	params := &stripe.PaymentMethodDetachParams{}
	params.Context = ctx
	if _, err := paymentmethod.Detach(paymentMethodID, params); err != nil {
		return fmt.Errorf("DetachPaymentMethod: stripe: %w", wrapStripeError(err))
	}
	return nil
}

// ---------- Payouts (Stripe Connect) ----------

// CreatePayout creates a Stripe Payout to a connected account's bank account.
// The stripeAccountID must be a Stripe Connect account ID (acct_...).
func (s *stripeService) CreatePayout(ctx context.Context, stripeAccountID string, amountCents int64, currency, description, idempotencyKey string) (*stripe.Payout, error) {
	if currency == "" {
		currency = s.cfg.Stripe.Currency
	}

	params := &stripe.PayoutParams{
		Amount:      stripe.Int64(amountCents),
		Currency:    stripe.String(currency),
		Description: stripe.String(description),
	}
	// For Connect payouts, set the Stripe-Account header via params.
	params.SetStripeAccount(stripeAccountID)
	params.SetIdempotencyKey(idempotencyKey)
	params.Context = ctx

	po, err := payout.New(params)
	if err != nil {
		return nil, fmt.Errorf("CreatePayout: stripe: %w", wrapStripeError(err))
	}

	s.logger.Info("stripe: payout created",
		zap.String("payout_id", po.ID),
		zap.String("status", string(po.Status)),
		zap.String("account_id", stripeAccountID),
		zap.Int64("amount_cents", amountCents),
	)
	return po, nil
}

// ---------- Refunds ----------

// RefundPayment creates a Stripe refund for the given charge.
// Pass amountCents=0 to refund the full charge amount.
func (s *stripeService) RefundPayment(ctx context.Context, chargeID string, amountCents int64, idempotencyKey string) (*stripe.Refund, error) {
	params := &stripe.RefundParams{
		Charge: stripe.String(chargeID),
	}
	if amountCents > 0 {
		params.Amount = stripe.Int64(amountCents)
	}
	params.SetIdempotencyKey(idempotencyKey)
	params.Context = ctx

	rf, err := refund.New(params)
	if err != nil {
		return nil, fmt.Errorf("RefundPayment: stripe: %w", wrapStripeError(err))
	}

	s.logger.Info("stripe: refund created",
		zap.String("refund_id", rf.ID),
		zap.String("charge_id", chargeID),
		zap.String("status", string(rf.Status)),
	)
	return rf, nil
}

// ---------- Webhooks ----------

// ConstructEvent verifies the Stripe webhook signature and unmarshals the event.
// It MUST be called with the raw request body bytes (before any JSON parsing).
func (s *stripeService) ConstructEvent(payload []byte, sigHeader string) (stripe.Event, error) {
	event, err := webhook.ConstructEvent(payload, sigHeader, s.cfg.Stripe.WebhookSecret)
	if err != nil {
		return stripe.Event{}, fmt.Errorf("%w: %s", ErrWebhookSignature, err.Error())
	}
	return event, nil
}

// HandleWebhook reads the raw body from the HTTP request, verifies the
// Stripe-Signature header, and dispatches based on event type.
//
// Supported events:
//   - payment_intent.succeeded
//   - payment_intent.payment_failed
//   - charge.refunded
//   - payout.paid
//   - payout.failed
func (s *stripeService) HandleWebhook(ctx context.Context, r *http.Request) (*WebhookResult, error) {
	const maxBodyBytes = 65536
	r.Body = http.MaxBytesReader(nil, r.Body, maxBodyBytes)
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("HandleWebhook: read body: %w", err)
	}

	sigHeader := r.Header.Get("Stripe-Signature")
	event, err := s.ConstructEvent(payload, sigHeader)
	if err != nil {
		return nil, err
	}

	result := &WebhookResult{
		EventID:   event.ID,
		EventType: string(event.Type),
	}

	switch event.Type {
	case "payment_intent.succeeded":
		pi, parseErr := parsePaymentIntent(event.Data.Raw)
		if parseErr != nil {
			return nil, fmt.Errorf("HandleWebhook: parse payment_intent.succeeded: %w", parseErr)
		}
		s.logger.Info("stripe webhook: payment_intent.succeeded",
			zap.String("payment_intent_id", pi.ID),
			zap.Int64("amount", pi.Amount),
		)
		result.PaymentID = pi.ID
		result.Status = string(pi.Status)
		result.Handled = true

	case "payment_intent.payment_failed":
		pi, parseErr := parsePaymentIntent(event.Data.Raw)
		if parseErr != nil {
			return nil, fmt.Errorf("HandleWebhook: parse payment_intent.payment_failed: %w", parseErr)
		}
		failMsg := ""
		if pi.LastPaymentError != nil {
			failMsg = pi.LastPaymentError.Msg
		}
		s.logger.Warn("stripe webhook: payment_intent.payment_failed",
			zap.String("payment_intent_id", pi.ID),
			zap.String("failure", failMsg),
		)
		result.PaymentID = pi.ID
		result.Status = string(pi.Status)
		result.Handled = true

	case "charge.refunded":
		var ch stripe.Charge
		if parseErr := json.Unmarshal(event.Data.Raw, &ch); parseErr != nil {
			return nil, fmt.Errorf("HandleWebhook: parse charge.refunded: %w", parseErr)
		}
		s.logger.Info("stripe webhook: charge.refunded",
			zap.String("charge_id", ch.ID),
			zap.Int64("amount_refunded", ch.AmountRefunded),
		)
		result.PaymentID = ch.ID
		result.Status = "refunded"
		result.Handled = true

	case "payout.paid":
		var po stripe.Payout
		if parseErr := json.Unmarshal(event.Data.Raw, &po); parseErr != nil {
			return nil, fmt.Errorf("HandleWebhook: parse payout.paid: %w", parseErr)
		}
		s.logger.Info("stripe webhook: payout.paid",
			zap.String("payout_id", po.ID),
			zap.Int64("amount", po.Amount),
		)
		result.PaymentID = po.ID
		result.Status = "paid"
		result.Handled = true

	case "payout.failed":
		var po stripe.Payout
		if parseErr := json.Unmarshal(event.Data.Raw, &po); parseErr != nil {
			return nil, fmt.Errorf("HandleWebhook: parse payout.failed: %w", parseErr)
		}
		s.logger.Warn("stripe webhook: payout.failed",
			zap.String("payout_id", po.ID),
			zap.String("failure_code", string(po.FailureCode)),
		)
		result.PaymentID = po.ID
		result.Status = "failed"
		result.Handled = true

	default:
		s.logger.Debug("stripe webhook: unhandled event type",
			zap.String("event_type", string(event.Type)),
		)
		result.Handled = false
	}

	return result, nil
}

// ---------- internal helpers ----------

func parsePaymentIntent(raw json.RawMessage) (*stripe.PaymentIntent, error) {
	var pi stripe.PaymentIntent
	if err := json.Unmarshal(raw, &pi); err != nil {
		return nil, err
	}
	return &pi, nil
}

// wrapStripeError converts a Stripe SDK error to a readable Go error,
// preserving the Stripe error code and decline code for logging.
func wrapStripeError(err error) error {
	var stripeErr *stripe.Error
	if errors.As(err, &stripeErr) {
		return fmt.Errorf("stripe [%s/%s]: %s", stripeErr.Code, stripeErr.DeclineCode, stripeErr.Msg)
	}
	return err
}

// convertMeta converts a map[string]string to the stripe Metadata type.
func convertMeta(m map[string]string) map[string]string {
	return m
}

// ---------- unused reference guard ----------
// These variables ensure the charge and time packages are used.
var _ = charge.Get
var _ = time.Now
var _ = models.PaymentStatusSucceeded
