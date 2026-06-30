package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/tiktok-clone/payment-service/internal/models"
)

// PaymentRepository implements the storage layer for the payment service.
type PaymentRepository struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

// NewPaymentRepository constructs a PaymentRepository backed by a pgxpool.Pool.
func NewPaymentRepository(pool *pgxpool.Pool, logger *zap.Logger) *PaymentRepository {
	return &PaymentRepository{pool: pool, logger: logger}
}

// ── Payments ──────────────────────────────────────────────────────────────────

func (r *PaymentRepository) CreatePayment(ctx context.Context, p *models.Payment, tx pgx.Tx) error {
	const q = `INSERT INTO payments
		(id,user_id,type,status,amount_cents,currency,stripe_payment_intent_id,
		 stripe_customer_id,idempotency_key,metadata,description,created_at,updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`
	db := querier(tx, r.pool)
	_, err := db.Exec(ctx, q,
		p.ID, p.UserID, p.Type, p.Status, p.AmountCents, p.Currency,
		p.StripePaymentIntentID, p.StripeCustomerID, p.IdempotencyKey,
		p.Metadata, p.Description, p.CreatedAt, p.UpdatedAt,
	)
	return err
}

func (r *PaymentRepository) GetPaymentByID(ctx context.Context, id uuid.UUID) (*models.Payment, error) {
	p := &models.Payment{}
	err := r.pool.QueryRow(ctx,
		`SELECT id,user_id,type,status,amount_cents,currency,
		        stripe_payment_intent_id,stripe_charge_id,stripe_customer_id,
		        idempotency_key,metadata,description,failure_code,failure_message,
		        refunded_amount_cents,stripe_refund_id,created_at,updated_at
		 FROM payments WHERE id=$1`, id).
		Scan(&p.ID, &p.UserID, &p.Type, &p.Status, &p.AmountCents, &p.Currency,
			&p.StripePaymentIntentID, &p.StripeChargeID, &p.StripeCustomerID,
			&p.IdempotencyKey, &p.Metadata, &p.Description, &p.FailureCode, &p.FailureMessage,
			&p.RefundedAmountCents, &p.StripeRefundID, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("payment not found")
	}
	return p, err
}

func (r *PaymentRepository) GetPaymentByIdempotencyKey(ctx context.Context, key string) (*models.Payment, error) {
	p := &models.Payment{}
	err := r.pool.QueryRow(ctx,
		`SELECT id,user_id,type,status,amount_cents,currency,
		        stripe_payment_intent_id,stripe_charge_id,stripe_customer_id,
		        idempotency_key,metadata,description,failure_code,failure_message,
		        refunded_amount_cents,stripe_refund_id,created_at,updated_at
		 FROM payments WHERE idempotency_key=$1`, key).
		Scan(&p.ID, &p.UserID, &p.Type, &p.Status, &p.AmountCents, &p.Currency,
			&p.StripePaymentIntentID, &p.StripeChargeID, &p.StripeCustomerID,
			&p.IdempotencyKey, &p.Metadata, &p.Description, &p.FailureCode, &p.FailureMessage,
			&p.RefundedAmountCents, &p.StripeRefundID, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return p, err
}

func (r *PaymentRepository) GetPaymentByStripeIntentID(ctx context.Context, intentID string) (*models.Payment, error) {
	p := &models.Payment{}
	err := r.pool.QueryRow(ctx,
		`SELECT id,user_id,type,status,amount_cents,currency,
		        stripe_payment_intent_id,stripe_charge_id,stripe_customer_id,
		        idempotency_key,metadata,description,failure_code,failure_message,
		        refunded_amount_cents,stripe_refund_id,created_at,updated_at
		 FROM payments WHERE stripe_payment_intent_id=$1`, intentID).
		Scan(&p.ID, &p.UserID, &p.Type, &p.Status, &p.AmountCents, &p.Currency,
			&p.StripePaymentIntentID, &p.StripeChargeID, &p.StripeCustomerID,
			&p.IdempotencyKey, &p.Metadata, &p.Description, &p.FailureCode, &p.FailureMessage,
			&p.RefundedAmountCents, &p.StripeRefundID, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("payment not found")
	}
	return p, err
}

func (r *PaymentRepository) UpdatePaymentStatus(ctx context.Context, id uuid.UUID, status models.PaymentStatus, chargeID, failureCode, failureMessage string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE payments SET status=$2,stripe_charge_id=$3,failure_code=$4,failure_message=$5,updated_at=$6 WHERE id=$1`,
		id, status, chargeID, failureCode, failureMessage, time.Now().UTC())
	return err
}

func (r *PaymentRepository) UpdatePaymentRefund(ctx context.Context, id uuid.UUID, refundedCents int64, refundID string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE payments SET refunded_amount_cents=$2,stripe_refund_id=$3,updated_at=$4 WHERE id=$1`,
		id, refundedCents, refundID, time.Now().UTC())
	return err
}

func (r *PaymentRepository) GetPaymentsByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.Payment, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id,user_id,type,status,amount_cents,currency,
		        stripe_payment_intent_id,stripe_charge_id,stripe_customer_id,
		        idempotency_key,metadata,description,failure_code,failure_message,
		        refunded_amount_cents,stripe_refund_id,created_at,updated_at
		 FROM payments WHERE user_id=$1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var payments []*models.Payment
	for rows.Next() {
		p := &models.Payment{}
		if err := rows.Scan(&p.ID, &p.UserID, &p.Type, &p.Status, &p.AmountCents, &p.Currency,
			&p.StripePaymentIntentID, &p.StripeChargeID, &p.StripeCustomerID,
			&p.IdempotencyKey, &p.Metadata, &p.Description, &p.FailureCode, &p.FailureMessage,
			&p.RefundedAmountCents, &p.StripeRefundID, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		payments = append(payments, p)
	}
	return payments, rows.Err()
}

// ── Stripe Customers ──────────────────────────────────────────────────────────

func (r *PaymentRepository) CreateStripeCustomer(ctx context.Context, sc *models.StripeCustomer) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO stripe_customers (id,user_id,stripe_customer_id,email,name,created_at,updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		sc.ID, sc.UserID, sc.StripeCustomerID, sc.Email, sc.Name, sc.CreatedAt, sc.UpdatedAt)
	return err
}

func (r *PaymentRepository) GetStripeCustomerByUserID(ctx context.Context, userID uuid.UUID) (*models.StripeCustomer, error) {
	sc := &models.StripeCustomer{}
	err := r.pool.QueryRow(ctx,
		`SELECT id,user_id,stripe_customer_id,email,name,created_at,updated_at
		 FROM stripe_customers WHERE user_id=$1`, userID).
		Scan(&sc.ID, &sc.UserID, &sc.StripeCustomerID, &sc.Email, &sc.Name, &sc.CreatedAt, &sc.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return sc, err
}

func (r *PaymentRepository) GetStripeCustomerByStripeID(ctx context.Context, stripeCustomerID string) (*models.StripeCustomer, error) {
	sc := &models.StripeCustomer{}
	err := r.pool.QueryRow(ctx,
		`SELECT id,user_id,stripe_customer_id,email,name,created_at,updated_at
		 FROM stripe_customers WHERE stripe_customer_id=$1`, stripeCustomerID).
		Scan(&sc.ID, &sc.UserID, &sc.StripeCustomerID, &sc.Email, &sc.Name, &sc.CreatedAt, &sc.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return sc, err
}

// ── Payment Methods ───────────────────────────────────────────────────────────

func (r *PaymentRepository) CreatePaymentMethod(ctx context.Context, pm *models.PaymentMethod) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO payment_methods
		 (id,user_id,stripe_customer_id,stripe_payment_method_id,type,
		  card_brand,card_last4,card_exp_month,card_exp_year,card_country,card_fingerprint,
		  is_default,created_at,updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		pm.ID, pm.UserID, pm.StripeCustomerID, pm.StripePaymentMethodID, pm.Type,
		pm.CardBrand, pm.CardLast4, pm.CardExpMonth, pm.CardExpYear,
		pm.CardCountry, pm.CardFingerprint, pm.IsDefault, pm.CreatedAt, pm.UpdatedAt)
	return err
}

func (r *PaymentRepository) GetPaymentMethodsByUserID(ctx context.Context, userID uuid.UUID) ([]*models.PaymentMethod, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id,user_id,stripe_customer_id,stripe_payment_method_id,type,
		        card_brand,card_last4,card_exp_month,card_exp_year,card_country,card_fingerprint,
		        is_default,created_at,updated_at
		 FROM payment_methods WHERE user_id=$1 ORDER BY is_default DESC,created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var pms []*models.PaymentMethod
	for rows.Next() {
		pm := &models.PaymentMethod{}
		if err := rows.Scan(&pm.ID, &pm.UserID, &pm.StripeCustomerID, &pm.StripePaymentMethodID, &pm.Type,
			&pm.CardBrand, &pm.CardLast4, &pm.CardExpMonth, &pm.CardExpYear,
			&pm.CardCountry, &pm.CardFingerprint, &pm.IsDefault, &pm.CreatedAt, &pm.UpdatedAt); err != nil {
			return nil, err
		}
		pms = append(pms, pm)
	}
	return pms, rows.Err()
}

func (r *PaymentRepository) SetDefaultPaymentMethod(ctx context.Context, userID, paymentMethodID uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	if _, err := tx.Exec(ctx, `UPDATE payment_methods SET is_default=false WHERE user_id=$1`, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE payment_methods SET is_default=true WHERE id=$1 AND user_id=$2`, paymentMethodID, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PaymentRepository) DeletePaymentMethod(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM payment_methods WHERE id=$1`, id)
	return err
}

// ── Payouts ───────────────────────────────────────────────────────────────────

func (r *PaymentRepository) CreatePayout(ctx context.Context, po *models.Payout) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO payouts
		 (id,creator_user_id,stripe_account_id,stripe_payout_id,amount_cents,currency,
		  status,description,failure_code,failure_message,wallet_payout_request_id,
		  arrival_date,created_at,updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		po.ID, po.CreatorUserID, po.StripeAccountID, po.StripePayoutID, po.AmountCents, po.Currency,
		po.Status, po.Description, po.FailureCode, po.FailureMessage, po.WalletPayoutRequestID,
		po.ArrivalDate, po.CreatedAt, po.UpdatedAt)
	return err
}

func (r *PaymentRepository) GetPayoutByID(ctx context.Context, id uuid.UUID) (*models.Payout, error) {
	po := &models.Payout{}
	err := r.pool.QueryRow(ctx,
		`SELECT id,creator_user_id,stripe_account_id,stripe_payout_id,amount_cents,currency,
		        status,description,failure_code,failure_message,wallet_payout_request_id,
		        arrival_date,created_at,updated_at
		 FROM payouts WHERE id=$1`, id).
		Scan(&po.ID, &po.CreatorUserID, &po.StripeAccountID, &po.StripePayoutID, &po.AmountCents, &po.Currency,
			&po.Status, &po.Description, &po.FailureCode, &po.FailureMessage, &po.WalletPayoutRequestID,
			&po.ArrivalDate, &po.CreatedAt, &po.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("payout not found")
	}
	return po, err
}

func (r *PaymentRepository) GetPayoutByStripeID(ctx context.Context, stripePayoutID string) (*models.Payout, error) {
	po := &models.Payout{}
	err := r.pool.QueryRow(ctx,
		`SELECT id,creator_user_id,stripe_account_id,stripe_payout_id,amount_cents,currency,
		        status,description,failure_code,failure_message,wallet_payout_request_id,
		        arrival_date,created_at,updated_at
		 FROM payouts WHERE stripe_payout_id=$1`, stripePayoutID).
		Scan(&po.ID, &po.CreatorUserID, &po.StripeAccountID, &po.StripePayoutID, &po.AmountCents, &po.Currency,
			&po.Status, &po.Description, &po.FailureCode, &po.FailureMessage, &po.WalletPayoutRequestID,
			&po.ArrivalDate, &po.CreatedAt, &po.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("payout not found")
	}
	return po, err
}

func (r *PaymentRepository) UpdatePayoutStatus(ctx context.Context, id uuid.UUID, status models.PayoutStatus, failureCode, failureMessage string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE payouts SET status=$2,failure_code=$3,failure_message=$4,updated_at=$5 WHERE id=$1`,
		id, status, failureCode, failureMessage, time.Now().UTC())
	return err
}

// ── Webhook Events ────────────────────────────────────────────────────────────

func (r *PaymentRepository) CreateWebhookEvent(ctx context.Context, e *models.WebhookEvent) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO webhook_events (id,stripe_event_id,event_type,raw_payload,processed_at)
		 VALUES ($1,$2,$3,$4,$5)`,
		e.ID, e.StripeEventID, e.EventType, e.RawPayload, e.ProcessedAt)
	return err
}

func (r *PaymentRepository) WebhookEventExists(ctx context.Context, stripeEventID string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM webhook_events WHERE stripe_event_id=$1)`, stripeEventID,
	).Scan(&exists)
	return exists, err
}

// ── Creator Connect ───────────────────────────────────────────────────────────

func (r *PaymentRepository) GetStripeAccountIDForCreator(ctx context.Context, creatorUserID uuid.UUID) (string, error) {
	var accountID string
	err := r.pool.QueryRow(ctx,
		`SELECT stripe_account_id FROM creator_stripe_accounts WHERE creator_user_id=$1`, creatorUserID,
	).Scan(&accountID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("no stripe account found for creator")
	}
	return accountID, err
}

// ── Transaction ──────────────────────────────────────────────────────────────

func (r *PaymentRepository) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return r.pool.Begin(ctx)
}

// ── helpers ───────────────────────────────────────────────────────────────────

type dbExecer interface {
	Exec(ctx context.Context, sql string, arguments ...any) (interface{ RowsAffected() int64 }, error)
}

func querier(tx pgx.Tx, pool *pgxpool.Pool) dbExecer {
	if tx != nil {
		return &txExecer{tx}
	}
	return &poolExecer{pool}
}

type txExecer struct{ tx pgx.Tx }

func (e *txExecer) Exec(ctx context.Context, sql string, args ...any) (interface{ RowsAffected() int64 }, error) {
	tag, err := e.tx.Exec(ctx, sql, args...)
	return tag, err
}

type poolExecer struct{ pool *pgxpool.Pool }

func (e *poolExecer) Exec(ctx context.Context, sql string, args ...any) (interface{ RowsAffected() int64 }, error) {
	tag, err := e.pool.Exec(ctx, sql, args...)
	return tag, err
}
