package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/tiktok-clone/payment-service/internal/config"
	"github.com/tiktok-clone/payment-service/internal/models"
)

// ---------- error sentinels ----------

var (
	ErrDuplicatePayment    = errors.New("duplicate payment: idempotency key already used")
	ErrInvalidAmount       = errors.New("payment amount must be positive")
	ErrMissingStripeAcct   = errors.New("creator does not have a linked Stripe Connect account")
)

// ---------- repository interface (thin, defined inline for this package) ----------

// paymentRepository is the storage abstraction used by PaymentService.
// The concrete pgx implementation lives in repositories/payment_repository.go.
type paymentRepository interface {
	// Payments.
	CreatePayment(ctx context.Context, p *models.Payment, tx pgx.Tx) error
	GetPaymentByID(ctx context.Context, id uuid.UUID) (*models.Payment, error)
	GetPaymentByIdempotencyKey(ctx context.Context, key string) (*models.Payment, error)
	GetPaymentByStripeIntentID(ctx context.Context, intentID string) (*models.Payment, error)
	UpdatePaymentStatus(ctx context.Context, id uuid.UUID, status models.PaymentStatus, chargeID, failureCode, failureMessage string) error
	UpdatePaymentRefund(ctx context.Context, id uuid.UUID, refundedCents int64, refundID string) error
	GetPaymentsByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.Payment, error)

	// Customers.
	CreateStripeCustomer(ctx context.Context, sc *models.StripeCustomer) error
	GetStripeCustomerByUserID(ctx context.Context, userID uuid.UUID) (*models.StripeCustomer, error)
	GetStripeCustomerByStripeID(ctx context.Context, stripeCustomerID string) (*models.StripeCustomer, error)

	// Payment methods.
	CreatePaymentMethod(ctx context.Context, pm *models.PaymentMethod) error
	GetPaymentMethodsByUserID(ctx context.Context, userID uuid.UUID) ([]*models.PaymentMethod, error)
	SetDefaultPaymentMethod(ctx context.Context, userID uuid.UUID, paymentMethodID uuid.UUID) error
	DeletePaymentMethod(ctx context.Context, id uuid.UUID) error

	// Payouts.
	CreatePayout(ctx context.Context, po *models.Payout) error
	GetPayoutByID(ctx context.Context, id uuid.UUID) (*models.Payout, error)
	GetPayoutByStripeID(ctx context.Context, stripePayoutID string) (*models.Payout, error)
	UpdatePayoutStatus(ctx context.Context, id uuid.UUID, status models.PayoutStatus, failureCode, failureMessage string) error

	// Webhook events (idempotency).
	CreateWebhookEvent(ctx context.Context, e *models.WebhookEvent) error
	WebhookEventExists(ctx context.Context, stripeEventID string) (bool, error)

	// Creator Stripe Connect account lookup.
	GetStripeAccountIDForCreator(ctx context.Context, creatorUserID uuid.UUID) (string, error)

	// DB helper.
	BeginTx(ctx context.Context) (pgx.Tx, error)
}

// ---------- DTOs ----------

// ProcessCoinPurchaseRequest is sent by wallet-service to initiate a coin purchase charge.
type ProcessCoinPurchaseRequest struct {
	UserID          uuid.UUID `json:"user_id"`
	PackageID       string    `json:"package_id"`
	PriceCents      int64     `json:"price_cents"`
	PaymentMethodID string    `json:"payment_method_id"`
	IdempotencyKey  string    `json:"idempotency_key"`
}

// ProcessCoinPurchaseResponse is returned after a successful coin purchase charge.
type ProcessCoinPurchaseResponse struct {
	PaymentID       uuid.UUID `json:"payment_id"`
	PaymentIntentID string    `json:"payment_intent_id"`
	Status          string    `json:"status"` // "succeeded" | "requires_action"
}

// ProcessSubscriptionRequest is sent to charge a recurring subscription.
type ProcessSubscriptionRequest struct {
	UserID          uuid.UUID `json:"user_id"`
	CreatorID       uuid.UUID `json:"creator_id"`
	AmountCents     int64     `json:"amount_cents"`
	PaymentMethodID string    `json:"payment_method_id"`
	IdempotencyKey  string    `json:"idempotency_key"`
	Description     string    `json:"description"`
}

// ProcessWithdrawalRequest is sent by wallet-service when a creator cashes out.
type ProcessWithdrawalRequest struct {
	CreatorUserID  uuid.UUID `json:"creator_user_id"`
	AmountCents    int64     `json:"amount_cents"`
	IdempotencyKey string    `json:"idempotency_key"`
}

// ProcessWithdrawalResponse is returned after a payout is initiated.
type ProcessWithdrawalResponse struct {
	PayoutID       uuid.UUID `json:"payout_id"`
	StripePayoutID string    `json:"stripe_payout_id"`
	Status         string    `json:"status"`
}

// PaymentHistoryResponse wraps a paginated list of payments.
type PaymentHistoryResponse struct {
	Payments []*models.Payment `json:"payments"`
	Total    int               `json:"total"`
}

// AddPaymentMethodRequest is the input for linking a new card to a user.
type AddPaymentMethodRequest struct {
	UserID          uuid.UUID `json:"user_id"`
	Email           string    `json:"email"`
	Name            string    `json:"name"`
	PaymentMethodID string    `json:"payment_method_id"` // Stripe pm_... token
	SetAsDefault    bool      `json:"set_as_default"`
}

// RefundRequest asks for a full or partial refund of a payment.
type RefundRequest struct {
	PaymentID      uuid.UUID `json:"payment_id"`
	AmountCents    int64     `json:"amount_cents"` // 0 = full refund
	IdempotencyKey string    `json:"idempotency_key"`
}

// ---------- PaymentService ----------

// PaymentService handles all business logic for payments and payouts.
type PaymentService interface {
	// Core internal operations (called by wallet-service).
	ProcessCoinPurchase(ctx context.Context, req ProcessCoinPurchaseRequest) (*ProcessCoinPurchaseResponse, error)
	ProcessSubscription(ctx context.Context, req ProcessSubscriptionRequest) (*ProcessCoinPurchaseResponse, error)
	ProcessWithdrawal(ctx context.Context, req ProcessWithdrawalRequest) (*ProcessWithdrawalResponse, error)

	// User-facing operations.
	AddPaymentMethod(ctx context.Context, req AddPaymentMethodRequest) (*models.PaymentMethod, error)
	GetPaymentMethods(ctx context.Context, userID uuid.UUID) ([]*models.PaymentMethod, error)
	RemovePaymentMethod(ctx context.Context, userID uuid.UUID, paymentMethodID uuid.UUID) error
	GetPaymentHistory(ctx context.Context, userID uuid.UUID, limit, offset int) (*PaymentHistoryResponse, error)
	RefundPayment(ctx context.Context, req RefundRequest) (*models.Payment, error)

	// Webhook dispatcher.
	HandleStripeWebhook(ctx context.Context, result *WebhookResult, rawPayload []byte) error
}

type paymentService struct {
	repo   paymentRepository
	stripe StripeService
	cfg    *config.Config
	logger *zap.Logger
}

// NewPaymentService constructs a production PaymentService.
func NewPaymentService(repo paymentRepository, stripe StripeService, cfg *config.Config, logger *zap.Logger) PaymentService {
	return &paymentService{repo: repo, stripe: stripe, cfg: cfg, logger: logger}
}

// ---------- ProcessCoinPurchase ----------

// ProcessCoinPurchase creates and confirms a Stripe PaymentIntent for a coin package.
// It is idempotent: if a payment with the same idempotency key already succeeded it
// returns the cached result immediately.
func (s *paymentService) ProcessCoinPurchase(ctx context.Context, req ProcessCoinPurchaseRequest) (*ProcessCoinPurchaseResponse, error) {
	if req.PriceCents <= 0 {
		return nil, ErrInvalidAmount
	}

	// Idempotency check: return existing payment if already processed.
	existing, err := s.repo.GetPaymentByIdempotencyKey(ctx, req.IdempotencyKey)
	if err == nil && existing != nil {
		if existing.Status == models.PaymentStatusSucceeded {
			return &ProcessCoinPurchaseResponse{
				PaymentID:       existing.ID,
				PaymentIntentID: existing.StripePaymentIntentID,
				Status:          "succeeded",
			}, nil
		}
	}

	// Resolve or create Stripe customer.
	stripeCustomerID, err := s.ensureStripeCustomer(ctx, req.UserID, "", "")
	if err != nil {
		return nil, fmt.Errorf("ProcessCoinPurchase: ensure customer: %w", err)
	}

	// Create and confirm PaymentIntent in one call.
	piResult, err := s.stripe.CreatePaymentIntent(ctx, CreatePaymentIntentInput{
		UserID:           req.UserID,
		StripeCustomerID: stripeCustomerID,
		AmountCents:      req.PriceCents,
		PaymentMethodID:  req.PaymentMethodID,
		Description:      fmt.Sprintf("TikTok Clone - Coin Package %s", req.PackageID),
		IdempotencyKey:   req.IdempotencyKey,
		Metadata: map[string]string{
			"package_id": req.PackageID,
			"user_id":    req.UserID.String(),
		},
		Confirm: true,
	})
	if err != nil {
		// Record the failed attempt.
		s.recordPayment(ctx, req.UserID, stripeCustomerID, models.PaymentTypeCoinPurchase,
			req.PriceCents, "", req.IdempotencyKey, models.PaymentStatusFailed,
			fmt.Sprintf("Coin package %s", req.PackageID))
		return nil, fmt.Errorf("ProcessCoinPurchase: stripe: %w", err)
	}

	status := models.PaymentStatusProcessing
	if piResult.Status == "succeeded" {
		status = models.PaymentStatusSucceeded
	}

	payment := s.buildPayment(req.UserID, stripeCustomerID, models.PaymentTypeCoinPurchase,
		req.PriceCents, piResult.PaymentIntentID, req.IdempotencyKey, status,
		fmt.Sprintf("Coin package %s", req.PackageID),
		map[string]string{"package_id": req.PackageID},
	)
	if err = s.repo.CreatePayment(ctx, payment, nil); err != nil {
		s.logger.Error("ProcessCoinPurchase: failed to persist payment",
			zap.Error(err), zap.String("pi_id", piResult.PaymentIntentID))
		// Don't fail the request — the charge may have already gone through.
	}

	return &ProcessCoinPurchaseResponse{
		PaymentID:       payment.ID,
		PaymentIntentID: piResult.PaymentIntentID,
		Status:          piResult.Status,
	}, nil
}

// ---------- ProcessSubscription ----------

// ProcessSubscription charges a creator subscription via Stripe.
func (s *paymentService) ProcessSubscription(ctx context.Context, req ProcessSubscriptionRequest) (*ProcessCoinPurchaseResponse, error) {
	if req.AmountCents <= 0 {
		return nil, ErrInvalidAmount
	}

	existing, err := s.repo.GetPaymentByIdempotencyKey(ctx, req.IdempotencyKey)
	if err == nil && existing != nil && existing.Status == models.PaymentStatusSucceeded {
		return &ProcessCoinPurchaseResponse{
			PaymentID:       existing.ID,
			PaymentIntentID: existing.StripePaymentIntentID,
			Status:          "succeeded",
		}, nil
	}

	stripeCustomerID, err := s.ensureStripeCustomer(ctx, req.UserID, "", "")
	if err != nil {
		return nil, fmt.Errorf("ProcessSubscription: ensure customer: %w", err)
	}

	piResult, err := s.stripe.CreatePaymentIntent(ctx, CreatePaymentIntentInput{
		UserID:           req.UserID,
		StripeCustomerID: stripeCustomerID,
		AmountCents:      req.AmountCents,
		PaymentMethodID:  req.PaymentMethodID,
		Description:      req.Description,
		IdempotencyKey:   req.IdempotencyKey,
		Metadata: map[string]string{
			"creator_id": req.CreatorID.String(),
			"user_id":    req.UserID.String(),
			"type":       "subscription",
		},
		Confirm: true,
	})
	if err != nil {
		return nil, fmt.Errorf("ProcessSubscription: stripe: %w", err)
	}

	status := models.PaymentStatusProcessing
	if piResult.Status == "succeeded" {
		status = models.PaymentStatusSucceeded
	}

	payment := s.buildPayment(req.UserID, stripeCustomerID, models.PaymentTypeSubscription,
		req.AmountCents, piResult.PaymentIntentID, req.IdempotencyKey, status,
		req.Description,
		map[string]string{"creator_id": req.CreatorID.String()},
	)
	_ = s.repo.CreatePayment(ctx, payment, nil)

	return &ProcessCoinPurchaseResponse{
		PaymentID:       payment.ID,
		PaymentIntentID: piResult.PaymentIntentID,
		Status:          piResult.Status,
	}, nil
}

// ---------- ProcessWithdrawal ----------

// ProcessWithdrawal initiates a Stripe Connect payout to the creator's bank account.
func (s *paymentService) ProcessWithdrawal(ctx context.Context, req ProcessWithdrawalRequest) (*ProcessWithdrawalResponse, error) {
	if req.AmountCents <= 0 {
		return nil, ErrInvalidAmount
	}

	// Look up the creator's Stripe Connect account.
	stripeAccountID, err := s.repo.GetStripeAccountIDForCreator(ctx, req.CreatorUserID)
	if err != nil {
		return nil, fmt.Errorf("ProcessWithdrawal: %w: %s", ErrMissingStripeAcct, err.Error())
	}

	po, err := s.stripe.CreatePayout(ctx,
		stripeAccountID,
		req.AmountCents,
		s.cfg.Stripe.Currency,
		fmt.Sprintf("TikTok Clone creator payout %s", req.CreatorUserID.String()),
		req.IdempotencyKey,
	)
	if err != nil {
		return nil, fmt.Errorf("ProcessWithdrawal: stripe: %w", err)
	}

	now := time.Now()
	dbPayout := &models.Payout{
		ID:                    uuid.New(),
		CreatorUserID:         req.CreatorUserID,
		StripeAccountID:       stripeAccountID,
		StripePayoutID:        po.ID,
		AmountCents:           req.AmountCents,
		Currency:              s.cfg.Stripe.Currency,
		Status:                models.PayoutStatusInTransit,
		Description:           "Creator withdrawal",
		WalletPayoutRequestID: req.IdempotencyKey,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	if po.ArrivalDate != 0 {
		t := time.Unix(po.ArrivalDate, 0)
		dbPayout.ArrivalDate = &t
	}
	if err = s.repo.CreatePayout(ctx, dbPayout); err != nil {
		s.logger.Error("ProcessWithdrawal: failed to persist payout record",
			zap.Error(err), zap.String("stripe_payout_id", po.ID))
	}

	return &ProcessWithdrawalResponse{
		PayoutID:       dbPayout.ID,
		StripePayoutID: po.ID,
		Status:         string(po.Status),
	}, nil
}

// ---------- AddPaymentMethod ----------

// AddPaymentMethod links a Stripe PaymentMethod (pm_...) to the user and stores
// the masked card details.
func (s *paymentService) AddPaymentMethod(ctx context.Context, req AddPaymentMethodRequest) (*models.PaymentMethod, error) {
	// Ensure a Stripe Customer exists for the user.
	stripeCustomerID, err := s.ensureStripeCustomer(ctx, req.UserID, req.Email, req.Name)
	if err != nil {
		return nil, fmt.Errorf("AddPaymentMethod: ensure customer: %w", err)
	}

	// Attach in Stripe.
	pm, err := s.stripe.AddPaymentMethod(ctx, stripeCustomerID, req.PaymentMethodID)
	if err != nil {
		return nil, fmt.Errorf("AddPaymentMethod: stripe: %w", err)
	}

	now := time.Now()
	dbPM := &models.PaymentMethod{
		ID:                    uuid.New(),
		UserID:                req.UserID,
		StripeCustomerID:      stripeCustomerID,
		StripePaymentMethodID: pm.ID,
		Type:                  models.PaymentMethodTypeCard,
		IsDefault:             req.SetAsDefault,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	if pm.Card != nil {
		dbPM.CardBrand = models.CardBrand(pm.Card.Brand)
		dbPM.CardLast4 = pm.Card.Last4
		dbPM.CardExpMonth = int(pm.Card.ExpMonth)
		dbPM.CardExpYear = int(pm.Card.ExpYear)
		dbPM.CardCountry = pm.Card.Country
		dbPM.CardFingerprint = pm.Card.Fingerprint
	}

	if err = s.repo.CreatePaymentMethod(ctx, dbPM); err != nil {
		return nil, fmt.Errorf("AddPaymentMethod: persist: %w", err)
	}

	if req.SetAsDefault {
		_ = s.repo.SetDefaultPaymentMethod(ctx, req.UserID, dbPM.ID)
	}

	return dbPM, nil
}

// GetPaymentMethods returns all stored payment methods for a user.
func (s *paymentService) GetPaymentMethods(ctx context.Context, userID uuid.UUID) ([]*models.PaymentMethod, error) {
	return s.repo.GetPaymentMethodsByUserID(ctx, userID)
}

// RemovePaymentMethod detaches a payment method from Stripe and deletes the DB record.
func (s *paymentService) RemovePaymentMethod(ctx context.Context, userID uuid.UUID, paymentMethodID uuid.UUID) error {
	methods, err := s.repo.GetPaymentMethodsByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("RemovePaymentMethod: %w", err)
	}

	var target *models.PaymentMethod
	for _, m := range methods {
		if m.ID == paymentMethodID {
			target = m
			break
		}
	}
	if target == nil {
		return ErrPaymentNotFound
	}

	if err = s.stripe.DetachPaymentMethod(ctx, target.StripePaymentMethodID); err != nil {
		return fmt.Errorf("RemovePaymentMethod: stripe detach: %w", err)
	}
	return s.repo.DeletePaymentMethod(ctx, paymentMethodID)
}

// ---------- GetPaymentHistory ----------

// GetPaymentHistory returns paginated payments for a user.
func (s *paymentService) GetPaymentHistory(ctx context.Context, userID uuid.UUID, limit, offset int) (*PaymentHistoryResponse, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	payments, err := s.repo.GetPaymentsByUserID(ctx, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("GetPaymentHistory: %w", err)
	}
	if payments == nil {
		payments = []*models.Payment{}
	}
	return &PaymentHistoryResponse{
		Payments: payments,
		Total:    len(payments),
	}, nil
}

// ---------- RefundPayment ----------

// RefundPayment initiates a Stripe refund and updates the local payment record.
func (s *paymentService) RefundPayment(ctx context.Context, req RefundRequest) (*models.Payment, error) {
	payment, err := s.repo.GetPaymentByID(ctx, req.PaymentID)
	if err != nil {
		return nil, fmt.Errorf("RefundPayment: get payment: %w", err)
	}
	if payment.Status != models.PaymentStatusSucceeded {
		return nil, fmt.Errorf("RefundPayment: payment status is %s, must be succeeded", payment.Status)
	}
	if payment.RefundedAmountCents >= payment.AmountCents {
		return nil, ErrAlreadyRefunded
	}

	refundCents := req.AmountCents
	if refundCents == 0 {
		refundCents = payment.AmountCents - payment.RefundedAmountCents
	}

	rf, err := s.stripe.RefundPayment(ctx, payment.StripeChargeID, refundCents, req.IdempotencyKey)
	if err != nil {
		return nil, fmt.Errorf("RefundPayment: stripe: %w", err)
	}

	newRefundedTotal := payment.RefundedAmountCents + refundCents
	if err = s.repo.UpdatePaymentRefund(ctx, payment.ID, newRefundedTotal, rf.ID); err != nil {
		s.logger.Error("RefundPayment: failed to update refund record",
			zap.Error(err), zap.String("refund_id", rf.ID))
	}

	newStatus := models.PaymentStatusSucceeded
	if newRefundedTotal >= payment.AmountCents {
		newStatus = models.PaymentStatusRefunded
		_ = s.repo.UpdatePaymentStatus(ctx, payment.ID, newStatus, "", "", "")
	}
	payment.RefundedAmountCents = newRefundedTotal
	payment.StripeRefundID = rf.ID
	payment.Status = newStatus
	return payment, nil
}

// ---------- HandleStripeWebhook ----------

// HandleStripeWebhook processes the outcome of a verified Stripe webhook event.
// It is called by the HTTP handler after ConstructEvent has already succeeded.
func (s *paymentService) HandleStripeWebhook(ctx context.Context, result *WebhookResult, rawPayload []byte) error {
	// Deduplicate: skip if we've already processed this Stripe event.
	exists, err := s.repo.WebhookEventExists(ctx, result.EventID)
	if err != nil {
		s.logger.Warn("HandleStripeWebhook: could not check event existence", zap.Error(err))
	}
	if exists {
		s.logger.Debug("HandleStripeWebhook: duplicate event skipped",
			zap.String("event_id", result.EventID))
		return nil
	}

	// Persist the event for audit trail.
	_ = s.repo.CreateWebhookEvent(ctx, &models.WebhookEvent{
		ID:            uuid.New(),
		StripeEventID: result.EventID,
		EventType:     result.EventType,
		ProcessedAt:   time.Now(),
		RawPayload:    rawPayload,
	})

	if !result.Handled || result.PaymentID == "" {
		return nil
	}

	switch result.EventType {
	case "payment_intent.succeeded":
		payment, findErr := s.repo.GetPaymentByStripeIntentID(ctx, result.PaymentID)
		if findErr != nil {
			s.logger.Warn("HandleStripeWebhook: payment not found for intent",
				zap.String("intent_id", result.PaymentID))
			return nil
		}
		if payment.Status == models.PaymentStatusSucceeded {
			return nil // already marked
		}
		return s.repo.UpdatePaymentStatus(ctx, payment.ID, models.PaymentStatusSucceeded, "", "", "")

	case "payment_intent.payment_failed":
		payment, findErr := s.repo.GetPaymentByStripeIntentID(ctx, result.PaymentID)
		if findErr != nil {
			return nil
		}
		return s.repo.UpdatePaymentStatus(ctx, payment.ID, models.PaymentStatusFailed, "", "payment_failed", "Payment failed via webhook")

	case "payout.paid":
		po, findErr := s.repo.GetPayoutByStripeID(ctx, result.PaymentID)
		if findErr != nil {
			return nil
		}
		return s.repo.UpdatePayoutStatus(ctx, po.ID, models.PayoutStatusPaid, "", "")

	case "payout.failed":
		po, findErr := s.repo.GetPayoutByStripeID(ctx, result.PaymentID)
		if findErr != nil {
			return nil
		}
		return s.repo.UpdatePayoutStatus(ctx, po.ID, models.PayoutStatusFailed, "payout_failed", "Payout failed via webhook")
	}

	return nil
}

// ---------- internal helpers ----------

// ensureStripeCustomer retrieves or creates a Stripe Customer for the given user.
// email and name are only used when creating a new customer; they may be empty
// if the customer is expected to already exist.
func (s *paymentService) ensureStripeCustomer(ctx context.Context, userID uuid.UUID, email, name string) (string, error) {
	sc, err := s.repo.GetStripeCustomerByUserID(ctx, userID)
	if err == nil {
		return sc.StripeCustomerID, nil
	}

	// Create a new Stripe Customer.
	cust, err := s.stripe.CreateCustomer(ctx, userID, email, name)
	if err != nil {
		return "", fmt.Errorf("ensureStripeCustomer: %w", err)
	}

	dbCustomer := &models.StripeCustomer{
		ID:               uuid.New(),
		UserID:           userID,
		StripeCustomerID: cust.ID,
		Email:            email,
		Name:             name,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	if err = s.repo.CreateStripeCustomer(ctx, dbCustomer); err != nil {
		s.logger.Error("ensureStripeCustomer: persist failed",
			zap.Error(err), zap.String("stripe_customer_id", cust.ID))
		// Return the Stripe ID anyway — the charge can proceed.
	}
	return cust.ID, nil
}

// recordPayment persists a failed payment attempt for audit purposes.
func (s *paymentService) recordPayment(
	ctx context.Context,
	userID uuid.UUID,
	stripeCustomerID string,
	ptype models.PaymentType,
	amountCents int64,
	paymentIntentID string,
	idempotencyKey string,
	status models.PaymentStatus,
	description string,
) {
	p := s.buildPayment(userID, stripeCustomerID, ptype, amountCents, paymentIntentID,
		idempotencyKey, status, description, nil)
	if err := s.repo.CreatePayment(ctx, p, nil); err != nil {
		s.logger.Error("recordPayment: failed to persist",
			zap.Error(err), zap.String("idempotency_key", idempotencyKey))
	}
}

func (s *paymentService) buildPayment(
	userID uuid.UUID,
	stripeCustomerID string,
	ptype models.PaymentType,
	amountCents int64,
	paymentIntentID string,
	idempotencyKey string,
	status models.PaymentStatus,
	description string,
	metadata map[string]string,
) *models.Payment {
	now := time.Now()
	return &models.Payment{
		ID:                    uuid.New(),
		UserID:                userID,
		Type:                  ptype,
		Status:                status,
		AmountCents:           amountCents,
		Currency:              s.cfg.Stripe.Currency,
		StripePaymentIntentID: paymentIntentID,
		StripeCustomerID:      stripeCustomerID,
		IdempotencyKey:        idempotencyKey,
		Description:           description,
		Metadata:              metadata,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
}

// ---------- compile-time guard ----------
// The concrete repository is injected at wire-up time in main.go.
// This assertion is intentionally left as a comment rather than a
// static check because the concrete type lives in a sibling package:
//
//   var _ paymentRepository = (*repositories.PgPaymentRepository)(nil)
//
// See services/payment-service/internal/repositories/payment_repository.go.

// ensure pgxpool is used (pool is imported for BeginTx type).
var _ = (*pgxpool.Pool)(nil)
