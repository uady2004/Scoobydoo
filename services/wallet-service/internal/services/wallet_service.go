package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/tiktok-clone/wallet-service/internal/config"
	"github.com/tiktok-clone/wallet-service/internal/models"
	"github.com/tiktok-clone/wallet-service/internal/repositories"
)

// ---------- error sentinels ----------

var (
	ErrInsufficientCoins    = errors.New("insufficient coins")
	ErrInsufficientDiamonds = errors.New("insufficient diamonds")
	ErrAlreadySubscribed    = errors.New("already subscribed to this creator")
	ErrInvalidPackage       = errors.New("invalid coin package")
	ErrSelfGift             = errors.New("cannot gift yourself")
	ErrPaymentFailed        = errors.New("payment processing failed")
)

// ---------- request / response DTOs ----------

// BuyCoinsRequest is the input for purchasing a coin package.
type BuyCoinsRequest struct {
	UserID          uuid.UUID `json:"user_id"`
	PackageID       string    `json:"package_id"`
	PaymentMethodID string    `json:"payment_method_id"` // Stripe payment method ID
	IdempotencyKey  string    `json:"idempotency_key"`
}

// BuyCoinsResponse is returned after a successful purchase.
type BuyCoinsResponse struct {
	TransactionID   uuid.UUID          `json:"transaction_id"`
	CoinsAdded      int64              `json:"coins_added"`
	NewBalance      int64              `json:"new_balance"`
	PaymentIntentID string             `json:"payment_intent_id"`
	Package         models.CoinPackage `json:"package"`
}

// SendGiftRequest is the input for sending a virtual gift.
type SendGiftRequest struct {
	SenderUserID   uuid.UUID       `json:"sender_user_id"`
	ReceiverUserID uuid.UUID       `json:"receiver_user_id"`
	GiftType       models.GiftType `json:"gift_type"`
	Quantity       int32           `json:"quantity"`
	LivestreamID   *uuid.UUID      `json:"livestream_id,omitempty"`
	VideoID        *uuid.UUID      `json:"video_id,omitempty"`
	Message        string          `json:"message"`
	IdempotencyKey string          `json:"idempotency_key"`
}

// SendGiftResponse is returned after a gift is sent.
type SendGiftResponse struct {
	GiftID        uuid.UUID `json:"gift_id"`
	CoinsDeducted int64     `json:"coins_deducted"`
	DiamondsSent  int64     `json:"diamonds_sent"`
	SenderBalance int64     `json:"sender_balance"`
}

// TipRequest is the input for sending a coin tip to a creator.
type TipRequest struct {
	SenderUserID   uuid.UUID  `json:"sender_user_id"`
	ReceiverUserID uuid.UUID  `json:"receiver_user_id"`
	CoinAmount     int64      `json:"coin_amount"`
	Message        string     `json:"message"`
	IdempotencyKey string     `json:"idempotency_key"`
}

// SubscribeRequest is the input for subscribing to a creator.
type SubscribeRequest struct {
	SubscriberID   uuid.UUID              `json:"subscriber_id"`
	CreatorID      uuid.UUID              `json:"creator_id"`
	Tier           models.SubscriptionTier `json:"tier"`
	IdempotencyKey string                 `json:"idempotency_key"`
}

// WithdrawRequest is the input for a creator withdrawal.
type WithdrawRequest struct {
	CreatorUserID  uuid.UUID `json:"creator_user_id"`
	DiamondAmount  int64     `json:"diamond_amount"`
	IdempotencyKey string    `json:"idempotency_key"`
}

// WithdrawResponse is returned when a withdrawal is initiated.
type WithdrawResponse struct {
	PayoutRequestID uuid.UUID `json:"payout_request_id"`
	DiamondAmount   int64     `json:"diamonds_converted"`
	USDCents        int64     `json:"usd_cents"`
}

// ConversionQuote explains the diamond-to-money conversion.
type ConversionQuote struct {
	Diamonds       int64   `json:"diamonds"`
	USDCents       int64   `json:"usd_cents"`
	USDDollars     float64 `json:"usd_dollars"`
	Rate           float64 `json:"rate_per_diamond_usd_cents"`
}

// ---------- payment service client DTOs ----------

type processCoinPurchaseRequest struct {
	UserID          string `json:"user_id"`
	PackageID       string `json:"package_id"`
	PriceCents      int64  `json:"price_cents"`
	PaymentMethodID string `json:"payment_method_id"`
	IdempotencyKey  string `json:"idempotency_key"`
}

type processCoinPurchaseResponse struct {
	PaymentIntentID string `json:"payment_intent_id"`
	Status          string `json:"status"`
}

type processWithdrawalRequest struct {
	CreatorUserID  string `json:"creator_user_id"`
	AmountCents    int64  `json:"amount_cents"`
	IdempotencyKey string `json:"idempotency_key"`
}

type processWithdrawalResponse struct {
	StripePayoutID string `json:"stripe_payout_id"`
	Status         string `json:"status"`
}

// ---------- WalletService ----------

// WalletService defines all business-logic operations on user wallets.
type WalletService interface {
	GetBalance(ctx context.Context, userID uuid.UUID) (*models.CoinBalance, error)
	BuyCoins(ctx context.Context, req BuyCoinsRequest) (*BuyCoinsResponse, error)
	SendGift(ctx context.Context, req SendGiftRequest) (*SendGiftResponse, error)
	TipCreator(ctx context.Context, req TipRequest) (*models.Transaction, error)
	SubscribeToCreator(ctx context.Context, req SubscribeRequest) (*models.Subscription, error)
	WithdrawEarnings(ctx context.Context, req WithdrawRequest) (*WithdrawResponse, error)
	GetTransactionHistory(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.Transaction, error)
	ConvertDiamondsToMoney(ctx context.Context, userID uuid.UUID, diamonds int64) (*ConversionQuote, error)
	GetCoinPackages(ctx context.Context) []models.CoinPackage
	EnsureWallet(ctx context.Context, userID uuid.UUID) (*models.Wallet, error)
}

type walletService struct {
	repo   repositories.WalletRepository
	cfg    *config.Config
	logger *zap.Logger
	http   *http.Client
}

// NewWalletService constructs a production WalletService.
func NewWalletService(repo repositories.WalletRepository, cfg *config.Config, logger *zap.Logger) WalletService {
	return &walletService{
		repo:   repo,
		cfg:    cfg,
		logger: logger,
		http: &http.Client{
			Timeout: time.Duration(cfg.Payment.TimeoutSeconds) * time.Second,
		},
	}
}

// EnsureWallet creates a wallet for the user if one does not already exist.
func (s *walletService) EnsureWallet(ctx context.Context, userID uuid.UUID) (*models.Wallet, error) {
	w, err := s.repo.GetWalletByUserID(ctx, userID)
	if err == nil {
		return w, nil
	}
	if !errors.Is(err, repositories.ErrNotFound) {
		return nil, fmt.Errorf("EnsureWallet: %w", err)
	}
	return s.repo.CreateWallet(ctx, userID)
}

// GetBalance returns the current coin and diamond balance for a user.
func (s *walletService) GetBalance(ctx context.Context, userID uuid.UUID) (*models.CoinBalance, error) {
	bal, err := s.repo.GetBalance(ctx, userID)
	if errors.Is(err, repositories.ErrNotFound) {
		// Lazily create wallet if missing.
		if _, err2 := s.EnsureWallet(ctx, userID); err2 != nil {
			return nil, err2
		}
		return s.repo.GetBalance(ctx, userID)
	}
	return bal, err
}

// GetCoinPackages returns the catalogue of purchasable coin packages.
func (s *walletService) GetCoinPackages(_ context.Context) []models.CoinPackage {
	return models.DefaultCoinPackages
}

// BuyCoins processes a coin package purchase. It calls the payment-service to
// create and confirm a Stripe PaymentIntent, then credits coins on success.
func (s *walletService) BuyCoins(ctx context.Context, req BuyCoinsRequest) (*BuyCoinsResponse, error) {
	// Locate the requested package.
	var pkg *models.CoinPackage
	for i, p := range models.DefaultCoinPackages {
		if p.ID == req.PackageID {
			pkg = &models.DefaultCoinPackages[i]
			break
		}
	}
	if pkg == nil {
		return nil, ErrInvalidPackage
	}

	// Ensure the wallet exists.
	wallet, err := s.EnsureWallet(ctx, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("BuyCoins: ensure wallet: %w", err)
	}

	// Call payment-service to charge the card.
	payResp, err := s.callProcessCoinPurchase(ctx, processCoinPurchaseRequest{
		UserID:          req.UserID.String(),
		PackageID:       pkg.ID,
		PriceCents:      pkg.PriceCents,
		PaymentMethodID: req.PaymentMethodID,
		IdempotencyKey:  req.IdempotencyKey,
	})
	if err != nil {
		return nil, fmt.Errorf("BuyCoins: payment: %w", err)
	}
	if payResp.Status != "succeeded" {
		return nil, fmt.Errorf("BuyCoins: %w: status=%s", ErrPaymentFailed, payResp.Status)
	}

	totalCoins := pkg.Coins + pkg.BonusCoins

	// Credit coins inside a transaction.
	dbTx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("BuyCoins: begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = dbTx.Rollback(ctx)
		}
	}()

	if err = s.repo.CreditCoins(ctx, req.UserID, totalCoins, dbTx); err != nil {
		return nil, fmt.Errorf("BuyCoins: credit: %w", err)
	}

	txn := &models.Transaction{
		WalletID:       wallet.ID,
		UserID:         req.UserID,
		Type:           models.TxTypeCoinPurchase,
		Status:         models.TxStatusCompleted,
		CoinAmount:     totalCoins,
		USDCents:       pkg.PriceCents,
		ReferenceID:    payResp.PaymentIntentID,
		IdempotencyKey: req.IdempotencyKey,
		Description:    fmt.Sprintf("Purchased %s: %d coins", pkg.Label, totalCoins),
	}
	if err = s.repo.CreateTransaction(ctx, txn, dbTx); err != nil {
		return nil, fmt.Errorf("BuyCoins: record transaction: %w", err)
	}

	if err = dbTx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("BuyCoins: commit: %w", err)
	}

	bal, _ := s.repo.GetBalance(ctx, req.UserID)
	newBalance := int64(0)
	if bal != nil {
		newBalance = bal.Coins
	}

	return &BuyCoinsResponse{
		TransactionID:   txn.ID,
		CoinsAdded:      totalCoins,
		NewBalance:      newBalance,
		PaymentIntentID: payResp.PaymentIntentID,
		Package:         *pkg,
	}, nil
}

// SendGift deducts coins from the sender and credits diamonds to the receiver.
// The diamond conversion uses a 1:1 coin-to-diamond ratio with the platform
// retaining the configured revenue share percentage.
func (s *walletService) SendGift(ctx context.Context, req SendGiftRequest) (*SendGiftResponse, error) {
	if req.SenderUserID == req.ReceiverUserID {
		return nil, ErrSelfGift
	}

	perCoinCost, ok := models.GiftCoinCosts[req.GiftType]
	if !ok {
		return nil, fmt.Errorf("SendGift: unknown gift type %q", req.GiftType)
	}
	if req.Quantity <= 0 {
		req.Quantity = 1
	}
	totalCoinCost := perCoinCost * int64(req.Quantity)

	// Diamonds = coins × creator revenue share (e.g. 50 %).
	diamondEarned := int64(math.Floor(float64(totalCoinCost) * s.cfg.App.CreatorRevenueSharePct))
	if diamondEarned < 1 {
		diamondEarned = 1
	}

	senderWallet, err := s.EnsureWallet(ctx, req.SenderUserID)
	if err != nil {
		return nil, fmt.Errorf("SendGift: sender wallet: %w", err)
	}
	_, err = s.EnsureWallet(ctx, req.ReceiverUserID)
	if err != nil {
		return nil, fmt.Errorf("SendGift: receiver wallet: %w", err)
	}

	dbTx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("SendGift: begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = dbTx.Rollback(ctx)
		}
	}()

	// Debit sender (pessimistic lock inside DebitCoins).
	if err = s.repo.DebitCoins(ctx, req.SenderUserID, totalCoinCost, dbTx); err != nil {
		if errors.Is(err, repositories.ErrInsufficientBalance) {
			return nil, ErrInsufficientCoins
		}
		return nil, fmt.Errorf("SendGift: debit: %w", err)
	}

	// Credit receiver.
	if err = s.repo.CreditDiamonds(ctx, req.ReceiverUserID, diamondEarned, dbTx); err != nil {
		return nil, fmt.Errorf("SendGift: credit receiver: %w", err)
	}

	// Record sender transaction.
	senderTx := &models.Transaction{
		WalletID:       senderWallet.ID,
		UserID:         req.SenderUserID,
		Type:           models.TxTypeGiftSent,
		Status:         models.TxStatusCompleted,
		CoinAmount:     -totalCoinCost,
		RelatedUserID:  &req.ReceiverUserID,
		IdempotencyKey: req.IdempotencyKey,
		Description:    fmt.Sprintf("Sent %d x %s gift", req.Quantity, req.GiftType),
	}
	if err = s.repo.CreateTransaction(ctx, senderTx, dbTx); err != nil {
		return nil, fmt.Errorf("SendGift: sender tx: %w", err)
	}

	receiverWallet, _ := s.repo.GetWalletByUserID(ctx, req.ReceiverUserID)
	receiverWalletID := uuid.UUID{}
	if receiverWallet != nil {
		receiverWalletID = receiverWallet.ID
	}

	// Record receiver (creator) transaction.
	receiverTx := &models.Transaction{
		WalletID:       receiverWalletID,
		UserID:         req.ReceiverUserID,
		Type:           models.TxTypeGiftReceived,
		Status:         models.TxStatusCompleted,
		DiamondAmount:  diamondEarned,
		RelatedUserID:  &req.SenderUserID,
		IdempotencyKey: req.IdempotencyKey + "_recv",
		Description:    fmt.Sprintf("Received %d x %s gift from fan", req.Quantity, req.GiftType),
	}
	if err = s.repo.CreateTransaction(ctx, receiverTx, dbTx); err != nil {
		return nil, fmt.Errorf("SendGift: receiver tx: %w", err)
	}

	// Record the gift itself.
	gift := &models.Gift{
		SenderUserID:   req.SenderUserID,
		ReceiverUserID: req.ReceiverUserID,
		GiftType:       req.GiftType,
		Quantity:       req.Quantity,
		CoinCost:       totalCoinCost,
		DiamondEarned:  diamondEarned,
		LivestreamID:   req.LivestreamID,
		VideoID:        req.VideoID,
		TransactionID:  senderTx.ID,
		Message:        req.Message,
	}
	if err = s.repo.CreateGift(ctx, gift, dbTx); err != nil {
		return nil, fmt.Errorf("SendGift: create gift: %w", err)
	}

	if err = dbTx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("SendGift: commit: %w", err)
	}

	bal, _ := s.repo.GetBalance(ctx, req.SenderUserID)
	newBal := int64(0)
	if bal != nil {
		newBal = bal.Coins
	}

	return &SendGiftResponse{
		GiftID:        gift.ID,
		CoinsDeducted: totalCoinCost,
		DiamondsSent:  diamondEarned,
		SenderBalance: newBal,
	}, nil
}

// TipCreator sends a direct coin tip from a viewer to a creator.
// Diamonds are credited at the configured revenue share rate.
func (s *walletService) TipCreator(ctx context.Context, req TipRequest) (*models.Transaction, error) {
	if req.CoinAmount <= 0 {
		return nil, fmt.Errorf("TipCreator: coin_amount must be positive")
	}
	if req.SenderUserID == req.ReceiverUserID {
		return nil, ErrSelfGift
	}

	diamondEarned := int64(math.Floor(float64(req.CoinAmount) * s.cfg.App.CreatorRevenueSharePct))
	if diamondEarned < 1 {
		diamondEarned = 1
	}

	senderWallet, err := s.EnsureWallet(ctx, req.SenderUserID)
	if err != nil {
		return nil, fmt.Errorf("TipCreator: sender wallet: %w", err)
	}
	_, err = s.EnsureWallet(ctx, req.ReceiverUserID)
	if err != nil {
		return nil, fmt.Errorf("TipCreator: receiver wallet: %w", err)
	}

	dbTx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("TipCreator: begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = dbTx.Rollback(ctx)
		}
	}()

	if err = s.repo.DebitCoins(ctx, req.SenderUserID, req.CoinAmount, dbTx); err != nil {
		if errors.Is(err, repositories.ErrInsufficientBalance) {
			return nil, ErrInsufficientCoins
		}
		return nil, fmt.Errorf("TipCreator: debit: %w", err)
	}

	if err = s.repo.CreditDiamonds(ctx, req.ReceiverUserID, diamondEarned, dbTx); err != nil {
		return nil, fmt.Errorf("TipCreator: credit: %w", err)
	}

	txn := &models.Transaction{
		WalletID:       senderWallet.ID,
		UserID:         req.SenderUserID,
		Type:           models.TxTypeTip,
		Status:         models.TxStatusCompleted,
		CoinAmount:     -req.CoinAmount,
		RelatedUserID:  &req.ReceiverUserID,
		IdempotencyKey: req.IdempotencyKey,
		Description:    fmt.Sprintf("Tip of %d coins to creator", req.CoinAmount),
	}
	if err = s.repo.CreateTransaction(ctx, txn, dbTx); err != nil {
		return nil, fmt.Errorf("TipCreator: record tx: %w", err)
	}

	if err = dbTx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("TipCreator: commit: %w", err)
	}
	return txn, nil
}

// SubscribeToCreator deducts coins for a monthly creator subscription.
func (s *walletService) SubscribeToCreator(ctx context.Context, req SubscribeRequest) (*models.Subscription, error) {
	cost, ok := models.SubTierCoinCosts[req.Tier]
	if !ok {
		return nil, fmt.Errorf("SubscribeToCreator: unknown tier %q", req.Tier)
	}

	// Check for existing active subscription.
	existing, err := s.repo.GetActiveSubscription(ctx, req.SubscriberID, req.CreatorID)
	if err == nil && existing != nil {
		return nil, ErrAlreadySubscribed
	}

	diamondEarned := int64(math.Floor(float64(cost) * s.cfg.App.CreatorRevenueSharePct))
	if diamondEarned < 1 {
		diamondEarned = 1
	}

	subWallet, err := s.EnsureWallet(ctx, req.SubscriberID)
	if err != nil {
		return nil, fmt.Errorf("SubscribeToCreator: subscriber wallet: %w", err)
	}
	_, err = s.EnsureWallet(ctx, req.CreatorID)
	if err != nil {
		return nil, fmt.Errorf("SubscribeToCreator: creator wallet: %w", err)
	}

	dbTx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("SubscribeToCreator: begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = dbTx.Rollback(ctx)
		}
	}()

	if err = s.repo.DebitCoins(ctx, req.SubscriberID, cost, dbTx); err != nil {
		if errors.Is(err, repositories.ErrInsufficientBalance) {
			return nil, ErrInsufficientCoins
		}
		return nil, fmt.Errorf("SubscribeToCreator: debit: %w", err)
	}

	if err = s.repo.CreditDiamonds(ctx, req.CreatorID, diamondEarned, dbTx); err != nil {
		return nil, fmt.Errorf("SubscribeToCreator: credit creator: %w", err)
	}

	now := time.Now()
	txn := &models.Transaction{
		WalletID:       subWallet.ID,
		UserID:         req.SubscriberID,
		Type:           models.TxTypeSubscription,
		Status:         models.TxStatusCompleted,
		CoinAmount:     -cost,
		RelatedUserID:  &req.CreatorID,
		IdempotencyKey: req.IdempotencyKey,
		Description:    fmt.Sprintf("Subscribed to creator (%s tier)", req.Tier),
	}
	if err = s.repo.CreateTransaction(ctx, txn, dbTx); err != nil {
		return nil, fmt.Errorf("SubscribeToCreator: record tx: %w", err)
	}

	sub := &models.Subscription{
		SubscriberID:  req.SubscriberID,
		CreatorID:     req.CreatorID,
		Tier:          req.Tier,
		Status:        models.SubStatusActive,
		CoinCost:      cost,
		DiamondEarned: diamondEarned,
		StartsAt:      now,
		ExpiresAt:     now.AddDate(0, 1, 0), // 1 month
		TransactionID: txn.ID,
	}
	if err = s.repo.CreateSubscription(ctx, sub, dbTx); err != nil {
		return nil, fmt.Errorf("SubscribeToCreator: create sub: %w", err)
	}

	if err = dbTx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("SubscribeToCreator: commit: %w", err)
	}
	return sub, nil
}

// WithdrawEarnings converts a creator's diamonds to USD and creates a payout
// request via the payment-service (Stripe Connect payout).
func (s *walletService) WithdrawEarnings(ctx context.Context, req WithdrawRequest) (*WithdrawResponse, error) {
	if req.DiamondAmount <= 0 {
		return nil, fmt.Errorf("WithdrawEarnings: diamond_amount must be positive")
	}

	// Compute USD value.
	quote, err := s.ConvertDiamondsToMoney(ctx, req.CreatorUserID, req.DiamondAmount)
	if err != nil {
		return nil, fmt.Errorf("WithdrawEarnings: convert: %w", err)
	}

	creatorWallet, err := s.EnsureWallet(ctx, req.CreatorUserID)
	if err != nil {
		return nil, fmt.Errorf("WithdrawEarnings: wallet: %w", err)
	}

	dbTx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("WithdrawEarnings: begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = dbTx.Rollback(ctx)
		}
	}()

	// Debit diamonds (pessimistic lock).
	if err = s.repo.DebitDiamonds(ctx, req.CreatorUserID, req.DiamondAmount, dbTx); err != nil {
		if errors.Is(err, repositories.ErrInsufficientBalance) {
			return nil, ErrInsufficientDiamonds
		}
		return nil, fmt.Errorf("WithdrawEarnings: debit diamonds: %w", err)
	}

	txn := &models.Transaction{
		WalletID:       creatorWallet.ID,
		UserID:         req.CreatorUserID,
		Type:           models.TxTypeWithdrawal,
		Status:         models.TxStatusPending,
		DiamondAmount:  -req.DiamondAmount,
		USDCents:       quote.USDCents,
		IdempotencyKey: req.IdempotencyKey,
		Description:    fmt.Sprintf("Withdrawal of %d diamonds ($%.2f)", req.DiamondAmount, float64(quote.USDCents)/100),
	}
	if err = s.repo.CreateTransaction(ctx, txn, dbTx); err != nil {
		return nil, fmt.Errorf("WithdrawEarnings: record tx: %w", err)
	}

	payout := &models.PayoutRequest{
		CreatorUserID: req.CreatorUserID,
		DiamondAmount: req.DiamondAmount,
		USDCents:      quote.USDCents,
		Status:        models.PayoutStatusPending,
		TransactionID: txn.ID,
	}
	if err = s.repo.CreatePayoutRequest(ctx, payout, dbTx); err != nil {
		return nil, fmt.Errorf("WithdrawEarnings: create payout: %w", err)
	}

	if err = dbTx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("WithdrawEarnings: commit: %w", err)
	}

	// Asynchronously call payment-service to initiate the Stripe payout.
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		payResp, payErr := s.callProcessWithdrawal(bgCtx, processWithdrawalRequest{
			CreatorUserID:  req.CreatorUserID.String(),
			AmountCents:    quote.USDCents,
			IdempotencyKey: req.IdempotencyKey,
		})
		if payErr != nil {
			s.logger.Error("WithdrawEarnings: payment service call failed",
				zap.Error(payErr),
				zap.String("payout_request_id", payout.ID.String()),
			)
			_ = s.repo.UpdatePayoutStatus(bgCtx, payout.ID,
				models.PayoutStatusFailed, "", payErr.Error())
			return
		}

		if payResp.Status == "paid" || payResp.Status == "in_transit" {
			_ = s.repo.UpdatePayoutStatus(bgCtx, payout.ID,
				models.PayoutStatusProcessing, payResp.StripePayoutID, "")
		} else {
			_ = s.repo.UpdatePayoutStatus(bgCtx, payout.ID,
				models.PayoutStatusFailed, payResp.StripePayoutID,
				fmt.Sprintf("unexpected status: %s", payResp.Status))
		}
	}()

	return &WithdrawResponse{
		PayoutRequestID: payout.ID,
		DiamondAmount:   req.DiamondAmount,
		USDCents:        quote.USDCents,
	}, nil
}

// GetTransactionHistory returns paginated transactions for a user.
func (s *walletService) GetTransactionHistory(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.Transaction, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return s.repo.GetTransactions(ctx, userID, limit, offset)
}

// ConvertDiamondsToMoney returns the USD value for a given diamond amount.
// Rate: 1 diamond = DiamondsToUSDCents USD cents.
func (s *walletService) ConvertDiamondsToMoney(_ context.Context, _ uuid.UUID, diamonds int64) (*ConversionQuote, error) {
	if diamonds <= 0 {
		return nil, fmt.Errorf("ConvertDiamondsToMoney: diamonds must be positive")
	}
	usdCents := int64(math.Floor(float64(diamonds) * s.cfg.App.DiamondsToUSDCents))
	return &ConversionQuote{
		Diamonds:   diamonds,
		USDCents:   usdCents,
		USDDollars: float64(usdCents) / 100.0,
		Rate:       s.cfg.App.DiamondsToUSDCents,
	}, nil
}

// ---------- payment-service HTTP helpers ----------

func (s *walletService) callProcessCoinPurchase(ctx context.Context, req processCoinPurchaseRequest) (*processCoinPurchaseResponse, error) {
	url := s.cfg.Payment.BaseURL + "/internal/v1/payments/coin-purchase"
	return httpPost[processCoinPurchaseRequest, processCoinPurchaseResponse](ctx, s.http, url, req)
}

func (s *walletService) callProcessWithdrawal(ctx context.Context, req processWithdrawalRequest) (*processWithdrawalResponse, error) {
	url := s.cfg.Payment.BaseURL + "/internal/v1/payments/withdrawal"
	return httpPost[processWithdrawalRequest, processWithdrawalResponse](ctx, s.http, url, req)
}

// httpPost is a generic JSON POST helper.
func httpPost[Req any, Resp any](ctx context.Context, client *http.Client, url string, body Req) (*Resp, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("httpPost: marshal: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("httpPost: new request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("httpPost: do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("httpPost: server returned %d: %s", resp.StatusCode, body)
	}

	var result Resp
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("httpPost: decode: %w", err)
	}
	return &result, nil
}
