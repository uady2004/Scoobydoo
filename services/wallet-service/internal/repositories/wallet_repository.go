package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/tiktok-clone/wallet-service/internal/models"
)

// ErrNotFound is returned when the requested resource does not exist.
var ErrNotFound = errors.New("record not found")

// ErrInsufficientBalance is returned when a debit would make the balance negative.
var ErrInsufficientBalance = errors.New("insufficient balance")

// ErrDuplicateIdempotency is returned when a transaction with the same idempotency key already exists.
var ErrDuplicateIdempotency = errors.New("duplicate idempotency key")

// WalletRepository defines all storage operations for the wallet domain.
type WalletRepository interface {
	// Wallet lifecycle.
	CreateWallet(ctx context.Context, userID uuid.UUID) (*models.Wallet, error)
	GetWalletByUserID(ctx context.Context, userID uuid.UUID) (*models.Wallet, error)

	// Balance.
	GetBalance(ctx context.Context, userID uuid.UUID) (*models.CoinBalance, error)
	CreditCoins(ctx context.Context, userID uuid.UUID, amount int64, tx pgx.Tx) error
	DebitCoins(ctx context.Context, userID uuid.UUID, amount int64, tx pgx.Tx) error
	CreditDiamonds(ctx context.Context, userID uuid.UUID, amount int64, tx pgx.Tx) error
	DebitDiamonds(ctx context.Context, userID uuid.UUID, amount int64, tx pgx.Tx) error

	// Transactions.
	CreateTransaction(ctx context.Context, t *models.Transaction, tx pgx.Tx) error
	GetTransactionByID(ctx context.Context, id uuid.UUID) (*models.Transaction, error)
	GetTransactionByIdempotencyKey(ctx context.Context, key string) (*models.Transaction, error)
	GetTransactions(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.Transaction, error)

	// Gifts.
	CreateGift(ctx context.Context, g *models.Gift, tx pgx.Tx) error
	GetGiftsByReceiver(ctx context.Context, receiverID uuid.UUID, limit, offset int) ([]*models.Gift, error)
	GetGiftsBySender(ctx context.Context, senderID uuid.UUID, limit, offset int) ([]*models.Gift, error)
	GetEarnings(ctx context.Context, creatorID uuid.UUID, from, to time.Time) (int64, error)

	// Subscriptions.
	CreateSubscription(ctx context.Context, s *models.Subscription, tx pgx.Tx) error
	GetActiveSubscription(ctx context.Context, subscriberID, creatorID uuid.UUID) (*models.Subscription, error)
	GetSubscriptionsByCreator(ctx context.Context, creatorID uuid.UUID, limit, offset int) ([]*models.Subscription, error)
	CancelSubscription(ctx context.Context, id uuid.UUID) error

	// Payout requests.
	CreatePayoutRequest(ctx context.Context, p *models.PayoutRequest, tx pgx.Tx) error
	GetPayoutRequest(ctx context.Context, id uuid.UUID) (*models.PayoutRequest, error)
	UpdatePayoutStatus(ctx context.Context, id uuid.UUID, status models.PayoutStatus, stripePayoutID, failureReason string) error

	// DB transaction helper.
	BeginTx(ctx context.Context) (pgx.Tx, error)
}

// pgWalletRepository implements WalletRepository using pgx.
type pgWalletRepository struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

// NewWalletRepository creates a production pgx-backed repository.
func NewWalletRepository(pool *pgxpool.Pool, logger *zap.Logger) WalletRepository {
	return &pgWalletRepository{pool: pool, logger: logger}
}

// BeginTx starts a serializable database transaction.
func (r *pgWalletRepository) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return r.pool.Begin(ctx)
}

// ---------- Wallet lifecycle ----------

func (r *pgWalletRepository) CreateWallet(ctx context.Context, userID uuid.UUID) (*models.Wallet, error) {
	w := &models.Wallet{}
	err := r.pool.QueryRow(ctx, `
		INSERT INTO wallets (user_id)
		VALUES ($1)
		ON CONFLICT (user_id) DO NOTHING
		RETURNING id, user_id, created_at, updated_at
	`, userID).Scan(&w.ID, &w.UserID, &w.CreatedAt, &w.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("CreateWallet: %w", err)
	}

	// Ensure coin_balances row exists.
	_, err = r.pool.Exec(ctx, `
		INSERT INTO coin_balances (wallet_id, user_id)
		VALUES ($1, $2)
		ON CONFLICT (wallet_id) DO NOTHING
	`, w.ID, userID)
	if err != nil {
		return nil, fmt.Errorf("CreateWallet: init balance: %w", err)
	}
	return w, nil
}

func (r *pgWalletRepository) GetWalletByUserID(ctx context.Context, userID uuid.UUID) (*models.Wallet, error) {
	w := &models.Wallet{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, created_at, updated_at FROM wallets WHERE user_id = $1
	`, userID).Scan(&w.ID, &w.UserID, &w.CreatedAt, &w.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("GetWalletByUserID: %w", err)
	}
	return w, nil
}

// ---------- Balance ----------

// GetBalance fetches the current coin/diamond balance for a user.
func (r *pgWalletRepository) GetBalance(ctx context.Context, userID uuid.UUID) (*models.CoinBalance, error) {
	b := &models.CoinBalance{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, wallet_id, user_id, coins, diamonds, lifetime_coins, lifetime_diamonds, updated_at
		FROM coin_balances
		WHERE user_id = $1
	`, userID).Scan(
		&b.ID, &b.WalletID, &b.UserID,
		&b.Coins, &b.Diamonds,
		&b.LifetimeCoins, &b.LifetimeDiamonds,
		&b.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("GetBalance: %w", err)
	}
	return b, nil
}

// CreditCoins adds coins to the user's balance. Must be called inside a transaction
// with a SELECT FOR UPDATE lock already held (pessimistic locking).
func (r *pgWalletRepository) CreditCoins(ctx context.Context, userID uuid.UUID, amount int64, tx pgx.Tx) error {
	if amount <= 0 {
		return fmt.Errorf("CreditCoins: amount must be positive, got %d", amount)
	}
	querier := querier(tx, r.pool)
	_, err := querier.Exec(ctx, `
		UPDATE coin_balances
		SET coins = coins + $2,
		    lifetime_coins = lifetime_coins + $2,
		    updated_at = NOW()
		WHERE user_id = $1
	`, userID, amount)
	if err != nil {
		return fmt.Errorf("CreditCoins: %w", err)
	}
	return nil
}

// DebitCoins removes coins from the user's balance using a pessimistic row-level
// lock (SELECT FOR UPDATE). Returns ErrInsufficientBalance if balance is too low.
func (r *pgWalletRepository) DebitCoins(ctx context.Context, userID uuid.UUID, amount int64, tx pgx.Tx) error {
	if amount <= 0 {
		return fmt.Errorf("DebitCoins: amount must be positive, got %d", amount)
	}
	if tx == nil {
		return fmt.Errorf("DebitCoins: must be called inside a transaction")
	}

	// Acquire pessimistic lock on the balance row.
	var current int64
	err := tx.QueryRow(ctx, `
		SELECT coins FROM coin_balances WHERE user_id = $1 FOR UPDATE
	`, userID).Scan(&current)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("DebitCoins: lock: %w", err)
	}

	if current < amount {
		return ErrInsufficientBalance
	}

	_, err = tx.Exec(ctx, `
		UPDATE coin_balances
		SET coins = coins - $2,
		    updated_at = NOW()
		WHERE user_id = $1
	`, userID, amount)
	if err != nil {
		return fmt.Errorf("DebitCoins: update: %w", err)
	}
	return nil
}

// CreditDiamonds adds diamonds to a creator's balance.
func (r *pgWalletRepository) CreditDiamonds(ctx context.Context, userID uuid.UUID, amount int64, tx pgx.Tx) error {
	if amount <= 0 {
		return fmt.Errorf("CreditDiamonds: amount must be positive, got %d", amount)
	}
	querier := querier(tx, r.pool)
	_, err := querier.Exec(ctx, `
		UPDATE coin_balances
		SET diamonds = diamonds + $2,
		    lifetime_diamonds = lifetime_diamonds + $2,
		    updated_at = NOW()
		WHERE user_id = $1
	`, userID, amount)
	if err != nil {
		return fmt.Errorf("CreditDiamonds: %w", err)
	}
	return nil
}

// DebitDiamonds removes diamonds from a creator's balance (for withdrawals).
func (r *pgWalletRepository) DebitDiamonds(ctx context.Context, userID uuid.UUID, amount int64, tx pgx.Tx) error {
	if amount <= 0 {
		return fmt.Errorf("DebitDiamonds: amount must be positive, got %d", amount)
	}
	if tx == nil {
		return fmt.Errorf("DebitDiamonds: must be called inside a transaction")
	}

	var current int64
	err := tx.QueryRow(ctx, `
		SELECT diamonds FROM coin_balances WHERE user_id = $1 FOR UPDATE
	`, userID).Scan(&current)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("DebitDiamonds: lock: %w", err)
	}
	if current < amount {
		return ErrInsufficientBalance
	}

	_, err = tx.Exec(ctx, `
		UPDATE coin_balances
		SET diamonds = diamonds - $2,
		    updated_at = NOW()
		WHERE user_id = $1
	`, userID, amount)
	if err != nil {
		return fmt.Errorf("DebitDiamonds: update: %w", err)
	}
	return nil
}

// ---------- Transactions ----------

func (r *pgWalletRepository) CreateTransaction(ctx context.Context, t *models.Transaction, tx pgx.Tx) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	t.CreatedAt = time.Now()
	t.UpdatedAt = t.CreatedAt

	querier := querier(tx, r.pool)
	_, err := querier.Exec(ctx, `
		INSERT INTO transactions
		    (id, wallet_id, user_id, type, status, coin_amount, diamond_amount,
		     usd_cents, related_user_id, reference_id, idempotency_key,
		     description, created_at, updated_at)
		VALUES
		    ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		ON CONFLICT (idempotency_key) DO NOTHING
	`,
		t.ID, t.WalletID, t.UserID, t.Type, t.Status,
		t.CoinAmount, t.DiamondAmount, t.USDCents,
		t.RelatedUserID, t.ReferenceID, t.IdempotencyKey,
		t.Description, t.CreatedAt, t.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("CreateTransaction: %w", err)
	}
	return nil
}

func (r *pgWalletRepository) GetTransactionByID(ctx context.Context, id uuid.UUID) (*models.Transaction, error) {
	t := &models.Transaction{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, wallet_id, user_id, type, status, coin_amount, diamond_amount,
		       usd_cents, related_user_id, reference_id, idempotency_key,
		       description, created_at, updated_at
		FROM transactions WHERE id = $1
	`, id).Scan(
		&t.ID, &t.WalletID, &t.UserID, &t.Type, &t.Status,
		&t.CoinAmount, &t.DiamondAmount, &t.USDCents,
		&t.RelatedUserID, &t.ReferenceID, &t.IdempotencyKey,
		&t.Description, &t.CreatedAt, &t.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("GetTransactionByID: %w", err)
	}
	return t, nil
}

func (r *pgWalletRepository) GetTransactionByIdempotencyKey(ctx context.Context, key string) (*models.Transaction, error) {
	t := &models.Transaction{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, wallet_id, user_id, type, status, coin_amount, diamond_amount,
		       usd_cents, related_user_id, reference_id, idempotency_key,
		       description, created_at, updated_at
		FROM transactions WHERE idempotency_key = $1
	`, key).Scan(
		&t.ID, &t.WalletID, &t.UserID, &t.Type, &t.Status,
		&t.CoinAmount, &t.DiamondAmount, &t.USDCents,
		&t.RelatedUserID, &t.ReferenceID, &t.IdempotencyKey,
		&t.Description, &t.CreatedAt, &t.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("GetTransactionByIdempotencyKey: %w", err)
	}
	return t, nil
}

func (r *pgWalletRepository) GetTransactions(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.Transaction, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, wallet_id, user_id, type, status, coin_amount, diamond_amount,
		       usd_cents, related_user_id, reference_id, idempotency_key,
		       description, created_at, updated_at
		FROM transactions
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("GetTransactions: query: %w", err)
	}
	defer rows.Close()

	var txns []*models.Transaction
	for rows.Next() {
		t := &models.Transaction{}
		if err := rows.Scan(
			&t.ID, &t.WalletID, &t.UserID, &t.Type, &t.Status,
			&t.CoinAmount, &t.DiamondAmount, &t.USDCents,
			&t.RelatedUserID, &t.ReferenceID, &t.IdempotencyKey,
			&t.Description, &t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("GetTransactions: scan: %w", err)
		}
		txns = append(txns, t)
	}
	return txns, rows.Err()
}

// ---------- Gifts ----------

func (r *pgWalletRepository) CreateGift(ctx context.Context, g *models.Gift, tx pgx.Tx) error {
	if g.ID == uuid.Nil {
		g.ID = uuid.New()
	}
	g.CreatedAt = time.Now()

	querier := querier(tx, r.pool)
	_, err := querier.Exec(ctx, `
		INSERT INTO gifts
		    (id, sender_user_id, receiver_user_id, gift_type, quantity,
		     coin_cost, diamond_earned, livestream_id, video_id,
		     transaction_id, message, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
	`,
		g.ID, g.SenderUserID, g.ReceiverUserID, g.GiftType, g.Quantity,
		g.CoinCost, g.DiamondEarned, g.LivestreamID, g.VideoID,
		g.TransactionID, g.Message, g.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("CreateGift: %w", err)
	}
	return nil
}

func (r *pgWalletRepository) GetGiftsByReceiver(ctx context.Context, receiverID uuid.UUID, limit, offset int) ([]*models.Gift, error) {
	return r.queryGifts(ctx, "receiver_user_id", receiverID, limit, offset)
}

func (r *pgWalletRepository) GetGiftsBySender(ctx context.Context, senderID uuid.UUID, limit, offset int) ([]*models.Gift, error) {
	return r.queryGifts(ctx, "sender_user_id", senderID, limit, offset)
}

func (r *pgWalletRepository) queryGifts(ctx context.Context, col string, id uuid.UUID, limit, offset int) ([]*models.Gift, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, sender_user_id, receiver_user_id, gift_type, quantity,
		       coin_cost, diamond_earned, livestream_id, video_id,
		       transaction_id, message, created_at
		FROM gifts WHERE %s = $1
		ORDER BY created_at DESC LIMIT $2 OFFSET $3
	`, col), id, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("queryGifts: %w", err)
	}
	defer rows.Close()

	var gifts []*models.Gift
	for rows.Next() {
		g := &models.Gift{}
		if err := rows.Scan(
			&g.ID, &g.SenderUserID, &g.ReceiverUserID, &g.GiftType, &g.Quantity,
			&g.CoinCost, &g.DiamondEarned, &g.LivestreamID, &g.VideoID,
			&g.TransactionID, &g.Message, &g.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("queryGifts: scan: %w", err)
		}
		gifts = append(gifts, g)
	}
	return gifts, rows.Err()
}

// GetEarnings returns the total diamonds earned by a creator in a date range.
func (r *pgWalletRepository) GetEarnings(ctx context.Context, creatorID uuid.UUID, from, to time.Time) (int64, error) {
	var total int64
	err := r.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(diamond_earned), 0)
		FROM gifts
		WHERE receiver_user_id = $1
		  AND created_at >= $2
		  AND created_at <= $3
	`, creatorID, from, to).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("GetEarnings: %w", err)
	}
	return total, nil
}

// ---------- Subscriptions ----------

func (r *pgWalletRepository) CreateSubscription(ctx context.Context, s *models.Subscription, tx pgx.Tx) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	now := time.Now()
	s.CreatedAt = now
	s.UpdatedAt = now

	querier := querier(tx, r.pool)
	_, err := querier.Exec(ctx, `
		INSERT INTO subscriptions
		    (id, subscriber_id, creator_id, tier, status, coin_cost, diamond_earned,
		     stripe_subscription_id, starts_at, expires_at, transaction_id, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
	`,
		s.ID, s.SubscriberID, s.CreatorID, s.Tier, s.Status,
		s.CoinCost, s.DiamondEarned, s.StripeSubscriptionID,
		s.StartsAt, s.ExpiresAt, s.TransactionID, s.CreatedAt, s.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("CreateSubscription: %w", err)
	}
	return nil
}

func (r *pgWalletRepository) GetActiveSubscription(ctx context.Context, subscriberID, creatorID uuid.UUID) (*models.Subscription, error) {
	s := &models.Subscription{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, subscriber_id, creator_id, tier, status, coin_cost, diamond_earned,
		       stripe_subscription_id, starts_at, expires_at, renewed_at, cancelled_at,
		       transaction_id, created_at, updated_at
		FROM subscriptions
		WHERE subscriber_id = $1
		  AND creator_id = $2
		  AND status = 'active'
		  AND expires_at > NOW()
		ORDER BY created_at DESC LIMIT 1
	`, subscriberID, creatorID).Scan(
		&s.ID, &s.SubscriberID, &s.CreatorID, &s.Tier, &s.Status,
		&s.CoinCost, &s.DiamondEarned, &s.StripeSubscriptionID,
		&s.StartsAt, &s.ExpiresAt, &s.RenewedAt, &s.CancelledAt,
		&s.TransactionID, &s.CreatedAt, &s.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("GetActiveSubscription: %w", err)
	}
	return s, nil
}

func (r *pgWalletRepository) GetSubscriptionsByCreator(ctx context.Context, creatorID uuid.UUID, limit, offset int) ([]*models.Subscription, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, subscriber_id, creator_id, tier, status, coin_cost, diamond_earned,
		       stripe_subscription_id, starts_at, expires_at, renewed_at, cancelled_at,
		       transaction_id, created_at, updated_at
		FROM subscriptions
		WHERE creator_id = $1
		ORDER BY created_at DESC LIMIT $2 OFFSET $3
	`, creatorID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("GetSubscriptionsByCreator: %w", err)
	}
	defer rows.Close()

	var subs []*models.Subscription
	for rows.Next() {
		s := &models.Subscription{}
		if err := rows.Scan(
			&s.ID, &s.SubscriberID, &s.CreatorID, &s.Tier, &s.Status,
			&s.CoinCost, &s.DiamondEarned, &s.StripeSubscriptionID,
			&s.StartsAt, &s.ExpiresAt, &s.RenewedAt, &s.CancelledAt,
			&s.TransactionID, &s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("GetSubscriptionsByCreator: scan: %w", err)
		}
		subs = append(subs, s)
	}
	return subs, rows.Err()
}

func (r *pgWalletRepository) CancelSubscription(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	tag, err := r.pool.Exec(ctx, `
		UPDATE subscriptions
		SET status = 'cancelled', cancelled_at = $2, updated_at = $2
		WHERE id = $1 AND status = 'active'
	`, id, now)
	if err != nil {
		return fmt.Errorf("CancelSubscription: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------- Payout requests ----------

func (r *pgWalletRepository) CreatePayoutRequest(ctx context.Context, p *models.PayoutRequest, tx pgx.Tx) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	p.RequestedAt = time.Now()

	querier := querier(tx, r.pool)
	_, err := querier.Exec(ctx, `
		INSERT INTO payout_requests
		    (id, creator_user_id, diamond_amount, usd_cents, status,
		     stripe_payout_id, transaction_id, requested_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
	`,
		p.ID, p.CreatorUserID, p.DiamondAmount, p.USDCents, p.Status,
		p.StripePayoutID, p.TransactionID, p.RequestedAt,
	)
	if err != nil {
		return fmt.Errorf("CreatePayoutRequest: %w", err)
	}
	return nil
}

func (r *pgWalletRepository) GetPayoutRequest(ctx context.Context, id uuid.UUID) (*models.PayoutRequest, error) {
	p := &models.PayoutRequest{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, creator_user_id, diamond_amount, usd_cents, status,
		       stripe_payout_id, failure_reason, transaction_id, requested_at, processed_at
		FROM payout_requests WHERE id = $1
	`, id).Scan(
		&p.ID, &p.CreatorUserID, &p.DiamondAmount, &p.USDCents, &p.Status,
		&p.StripePayoutID, &p.FailureReason, &p.TransactionID, &p.RequestedAt, &p.ProcessedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("GetPayoutRequest: %w", err)
	}
	return p, nil
}

func (r *pgWalletRepository) UpdatePayoutStatus(ctx context.Context, id uuid.UUID, status models.PayoutStatus, stripePayoutID, failureReason string) error {
	now := time.Now()
	_, err := r.pool.Exec(ctx, `
		UPDATE payout_requests
		SET status = $2, stripe_payout_id = $3, failure_reason = $4, processed_at = $5
		WHERE id = $1
	`, id, status, stripePayoutID, failureReason, now)
	if err != nil {
		return fmt.Errorf("UpdatePayoutStatus: %w", err)
	}
	return nil
}

// ---------- internal helpers ----------

// dbQuerier is satisfied by both *pgxpool.Pool and pgx.Tx.
type dbQuerier interface {
	Exec(ctx context.Context, sql string, arguments ...any) (interface{ RowsAffected() int64 }, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// querier returns tx if non-nil, otherwise falls back to pool.
// This allows the same query methods to work inside or outside a transaction.
func querier(tx pgx.Tx, pool *pgxpool.Pool) interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
} {
	if tx != nil {
		return tx
	}
	return pool
}
