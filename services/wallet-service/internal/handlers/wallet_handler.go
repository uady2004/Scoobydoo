package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/tiktok-clone/wallet-service/internal/config"
	"github.com/tiktok-clone/wallet-service/internal/models"
	"github.com/tiktok-clone/wallet-service/internal/repositories"
	"github.com/tiktok-clone/wallet-service/internal/services"
)

// WalletHandler wires together Echo routes and the WalletService.
type WalletHandler struct {
	svc    services.WalletService
	cfg    *config.Config
	logger *zap.Logger
}

// NewWalletHandler constructs a WalletHandler.
func NewWalletHandler(svc services.WalletService, cfg *config.Config, logger *zap.Logger) *WalletHandler {
	return &WalletHandler{svc: svc, cfg: cfg, logger: logger}
}

// RegisterRoutes attaches all wallet endpoints to the given Echo instance.
func (h *WalletHandler) RegisterRoutes(e *echo.Echo) {
	// Public health probe.
	e.GET("/healthz", h.Health)

	// JWT-protected wallet API.
	api := e.Group("/api/v1/wallet", h.authMiddleware())
	api.GET("/balance", h.GetBalance)
	api.GET("/packages", h.GetCoinPackages)
	api.POST("/buy-coins", h.BuyCoins)
	api.POST("/gift", h.SendGift)
	api.POST("/tip", h.TipCreator)
	api.POST("/subscribe", h.SubscribeToCreator)
	api.POST("/withdraw", h.WithdrawEarnings)
	api.GET("/transactions", h.GetTransactionHistory)
	api.GET("/convert-diamonds", h.ConvertDiamonds)
}

// ---------- Route handlers ----------

// Health godoc
// @Summary  Health check
// @Tags     system
// @Produce  json
// @Success  200 {object} map[string]string
// @Router   /healthz [get]
func (h *WalletHandler) Health(c echo.Context) error {
	return c.JSON(http.StatusOK, echo.Map{"status": "ok", "service": "wallet-service"})
}

// GetBalance godoc
// @Summary  Get coin and diamond balance
// @Tags     wallet
// @Security BearerAuth
// @Produce  json
// @Success  200 {object} models.CoinBalance
// @Failure  401 {object} errorResponse
// @Failure  500 {object} errorResponse
// @Router   /api/v1/wallet/balance [get]
func (h *WalletHandler) GetBalance(c echo.Context) error {
	userID := mustUserID(c)
	bal, err := h.svc.GetBalance(c.Request().Context(), userID)
	if err != nil {
		h.logger.Error("GetBalance failed", zap.String("user_id", userID.String()), zap.Error(err))
		return errInternal(c, "could not fetch balance")
	}
	return c.JSON(http.StatusOK, bal)
}

// GetCoinPackages godoc
// @Summary  List purchasable coin packages
// @Tags     wallet
// @Security BearerAuth
// @Produce  json
// @Success  200 {array} models.CoinPackage
// @Router   /api/v1/wallet/packages [get]
func (h *WalletHandler) GetCoinPackages(c echo.Context) error {
	pkgs := h.svc.GetCoinPackages(c.Request().Context())
	return c.JSON(http.StatusOK, pkgs)
}

// buyCoinsRequest is the request body for BuyCoins.
type buyCoinsRequest struct {
	PackageID       string `json:"package_id"        validate:"required"`
	PaymentMethodID string `json:"payment_method_id" validate:"required"`
	IdempotencyKey  string `json:"idempotency_key"   validate:"required"`
}

// BuyCoins godoc
// @Summary  Purchase a coin package
// @Tags     wallet
// @Security BearerAuth
// @Accept   json
// @Produce  json
// @Param    body body buyCoinsRequest true "Purchase request"
// @Success  201 {object} services.BuyCoinsResponse
// @Failure  400 {object} errorResponse
// @Failure  402 {object} errorResponse
// @Failure  500 {object} errorResponse
// @Router   /api/v1/wallet/buy-coins [post]
func (h *WalletHandler) BuyCoins(c echo.Context) error {
	var req buyCoinsRequest
	if err := bindAndValidate(c, &req); err != nil {
		return err
	}

	userID := mustUserID(c)
	resp, err := h.svc.BuyCoins(c.Request().Context(), services.BuyCoinsRequest{
		UserID:          userID,
		PackageID:       req.PackageID,
		PaymentMethodID: req.PaymentMethodID,
		IdempotencyKey:  req.IdempotencyKey,
	})
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidPackage):
			return errBadRequest(c, "invalid_package", err.Error())
		case errors.Is(err, services.ErrPaymentFailed):
			return errPaymentRequired(c, err.Error())
		default:
			h.logger.Error("BuyCoins failed", zap.String("user_id", userID.String()), zap.Error(err))
			return errInternal(c, "coin purchase failed")
		}
	}
	return c.JSON(http.StatusCreated, resp)
}

// sendGiftRequest is the request body for SendGift.
type sendGiftRequest struct {
	ReceiverUserID string          `json:"receiver_user_id" validate:"required,uuid"`
	GiftType       models.GiftType `json:"gift_type"        validate:"required"`
	Quantity       int32           `json:"quantity"         validate:"min=1,max=99"`
	LivestreamID   *string         `json:"livestream_id,omitempty"`
	VideoID        *string         `json:"video_id,omitempty"`
	Message        string          `json:"message"`
	IdempotencyKey string          `json:"idempotency_key" validate:"required"`
}

// SendGift godoc
// @Summary  Send a virtual gift to a creator
// @Tags     wallet
// @Security BearerAuth
// @Accept   json
// @Produce  json
// @Param    body body sendGiftRequest true "Gift request"
// @Success  201 {object} services.SendGiftResponse
// @Failure  400 {object} errorResponse
// @Failure  402 {object} errorResponse
// @Router   /api/v1/wallet/gift [post]
func (h *WalletHandler) SendGift(c echo.Context) error {
	var req sendGiftRequest
	if err := bindAndValidate(c, &req); err != nil {
		return err
	}

	receiverID, err := uuid.Parse(req.ReceiverUserID)
	if err != nil {
		return errBadRequest(c, "invalid_receiver_id", "receiver_user_id must be a valid UUID")
	}
	senderID := mustUserID(c)

	svcReq := services.SendGiftRequest{
		SenderUserID:   senderID,
		ReceiverUserID: receiverID,
		GiftType:       req.GiftType,
		Quantity:       req.Quantity,
		Message:        req.Message,
		IdempotencyKey: req.IdempotencyKey,
	}
	if req.LivestreamID != nil {
		if id, err2 := uuid.Parse(*req.LivestreamID); err2 == nil {
			svcReq.LivestreamID = &id
		}
	}
	if req.VideoID != nil {
		if id, err2 := uuid.Parse(*req.VideoID); err2 == nil {
			svcReq.VideoID = &id
		}
	}

	resp, err := h.svc.SendGift(c.Request().Context(), svcReq)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrSelfGift):
			return errBadRequest(c, "self_gift", "you cannot gift yourself")
		case errors.Is(err, services.ErrInsufficientCoins):
			return errPaymentRequired(c, "insufficient coins")
		default:
			h.logger.Error("SendGift failed", zap.String("sender", senderID.String()), zap.Error(err))
			return errInternal(c, "gift failed")
		}
	}
	return c.JSON(http.StatusCreated, resp)
}

// tipRequest is the request body for TipCreator.
type tipRequest struct {
	ReceiverUserID string `json:"receiver_user_id" validate:"required,uuid"`
	CoinAmount     int64  `json:"coin_amount"      validate:"required,min=1"`
	Message        string `json:"message"`
	IdempotencyKey string `json:"idempotency_key"  validate:"required"`
}

// TipCreator godoc
// @Summary  Send a coin tip to a creator
// @Tags     wallet
// @Security BearerAuth
// @Accept   json
// @Produce  json
// @Param    body body tipRequest true "Tip request"
// @Success  201 {object} models.Transaction
// @Router   /api/v1/wallet/tip [post]
func (h *WalletHandler) TipCreator(c echo.Context) error {
	var req tipRequest
	if err := bindAndValidate(c, &req); err != nil {
		return err
	}

	receiverID, err := uuid.Parse(req.ReceiverUserID)
	if err != nil {
		return errBadRequest(c, "invalid_receiver_id", "receiver_user_id must be a valid UUID")
	}
	senderID := mustUserID(c)

	txn, err := h.svc.TipCreator(c.Request().Context(), services.TipRequest{
		SenderUserID:   senderID,
		ReceiverUserID: receiverID,
		CoinAmount:     req.CoinAmount,
		Message:        req.Message,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInsufficientCoins):
			return errPaymentRequired(c, "insufficient coins")
		case errors.Is(err, services.ErrSelfGift):
			return errBadRequest(c, "self_tip", "cannot tip yourself")
		default:
			h.logger.Error("TipCreator failed", zap.Error(err))
			return errInternal(c, "tip failed")
		}
	}
	return c.JSON(http.StatusCreated, txn)
}

// subscribeRequest is the request body for SubscribeToCreator.
type subscribeRequest struct {
	CreatorID      string                  `json:"creator_id"      validate:"required,uuid"`
	Tier           models.SubscriptionTier `json:"tier"            validate:"required"`
	IdempotencyKey string                  `json:"idempotency_key" validate:"required"`
}

// SubscribeToCreator godoc
// @Summary  Subscribe to a creator
// @Tags     wallet
// @Security BearerAuth
// @Accept   json
// @Produce  json
// @Param    body body subscribeRequest true "Subscription request"
// @Success  201 {object} models.Subscription
// @Router   /api/v1/wallet/subscribe [post]
func (h *WalletHandler) SubscribeToCreator(c echo.Context) error {
	var req subscribeRequest
	if err := bindAndValidate(c, &req); err != nil {
		return err
	}

	creatorID, err := uuid.Parse(req.CreatorID)
	if err != nil {
		return errBadRequest(c, "invalid_creator_id", "creator_id must be a valid UUID")
	}
	subscriberID := mustUserID(c)

	sub, err := h.svc.SubscribeToCreator(c.Request().Context(), services.SubscribeRequest{
		SubscriberID:   subscriberID,
		CreatorID:      creatorID,
		Tier:           req.Tier,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		switch {
		case errors.Is(err, services.ErrAlreadySubscribed):
			return errConflict(c, "already_subscribed", "you already have an active subscription")
		case errors.Is(err, services.ErrInsufficientCoins):
			return errPaymentRequired(c, "insufficient coins")
		default:
			h.logger.Error("SubscribeToCreator failed", zap.Error(err))
			return errInternal(c, "subscription failed")
		}
	}
	return c.JSON(http.StatusCreated, sub)
}

// withdrawRequest is the request body for WithdrawEarnings.
type withdrawRequest struct {
	DiamondAmount  int64  `json:"diamond_amount"  validate:"required,min=1000"`
	IdempotencyKey string `json:"idempotency_key" validate:"required"`
}

// WithdrawEarnings godoc
// @Summary  Withdraw diamond earnings to USD (creator only)
// @Tags     wallet
// @Security BearerAuth
// @Accept   json
// @Produce  json
// @Param    body body withdrawRequest true "Withdrawal request"
// @Success  202 {object} services.WithdrawResponse
// @Router   /api/v1/wallet/withdraw [post]
func (h *WalletHandler) WithdrawEarnings(c echo.Context) error {
	var req withdrawRequest
	if err := bindAndValidate(c, &req); err != nil {
		return err
	}

	creatorID := mustUserID(c)
	resp, err := h.svc.WithdrawEarnings(c.Request().Context(), services.WithdrawRequest{
		CreatorUserID:  creatorID,
		DiamondAmount:  req.DiamondAmount,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInsufficientDiamonds):
			return errPaymentRequired(c, "insufficient diamonds")
		default:
			h.logger.Error("WithdrawEarnings failed", zap.Error(err))
			return errInternal(c, "withdrawal failed")
		}
	}
	return c.JSON(http.StatusAccepted, resp)
}

// GetTransactionHistory godoc
// @Summary  List transaction history
// @Tags     wallet
// @Security BearerAuth
// @Produce  json
// @Param    limit  query int false "Page size (default 20)"
// @Param    offset query int false "Page offset (default 0)"
// @Success  200 {array} models.Transaction
// @Router   /api/v1/wallet/transactions [get]
func (h *WalletHandler) GetTransactionHistory(c echo.Context) error {
	userID := mustUserID(c)
	limit := queryInt(c, "limit", 20)
	offset := queryInt(c, "offset", 0)

	txns, err := h.svc.GetTransactionHistory(c.Request().Context(), userID, limit, offset)
	if err != nil {
		h.logger.Error("GetTransactionHistory failed", zap.Error(err))
		return errInternal(c, "could not fetch transactions")
	}
	if txns == nil {
		txns = []*models.Transaction{}
	}
	return c.JSON(http.StatusOK, txns)
}

// ConvertDiamonds godoc
// @Summary  Get diamond-to-USD conversion quote
// @Tags     wallet
// @Security BearerAuth
// @Produce  json
// @Param    amount query int true "Number of diamonds to convert"
// @Success  200 {object} services.ConversionQuote
// @Router   /api/v1/wallet/convert-diamonds [get]
func (h *WalletHandler) ConvertDiamonds(c echo.Context) error {
	userID := mustUserID(c)
	amount := int64(queryInt(c, "amount", 0))
	if amount <= 0 {
		return errBadRequest(c, "invalid_amount", "amount must be a positive integer")
	}

	quote, err := h.svc.ConvertDiamondsToMoney(c.Request().Context(), userID, amount)
	if err != nil {
		return errBadRequest(c, "conversion_error", err.Error())
	}
	return c.JSON(http.StatusOK, quote)
}

// ---------- Auth middleware (JWT validation) ----------

// authMiddleware validates the Bearer JWT and stores the user_id UUID in context.
func (h *WalletHandler) authMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			authHeader := c.Request().Header.Get("Authorization")
			if len(authHeader) < 8 || authHeader[:7] != "Bearer " {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid or missing token")
			}
			tokenStr := authHeader[7:]

			claims := jwtv5.MapClaims{}
			_, err := jwtv5.ParseWithClaims(tokenStr, &claims, func(t *jwtv5.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwtv5.SigningMethodHMAC); !ok {
					return nil, errors.New("unexpected signing method")
				}
				return []byte(h.cfg.JWT.Secret), nil
			})
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid or missing token")
			}

			if uid, ok := claims["user_id"].(string); ok {
				if parsed, err := uuid.Parse(uid); err == nil {
					c.Set("user_id", parsed)
				}
			}
			return next(c)
		}
	}
}

// ---------- response helpers ----------

type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	TraceID string `json:"trace_id,omitempty"`
}

func errBadRequest(c echo.Context, code, msg string) error {
	return c.JSON(http.StatusBadRequest, errorResponse{Code: code, Message: msg})
}

func errPaymentRequired(c echo.Context, msg string) error {
	return c.JSON(http.StatusPaymentRequired, errorResponse{Code: "payment_required", Message: msg})
}

func errInternal(c echo.Context, msg string) error {
	return c.JSON(http.StatusInternalServerError, errorResponse{Code: "internal_error", Message: msg})
}

func errConflict(c echo.Context, code, msg string) error {
	return c.JSON(http.StatusConflict, errorResponse{Code: code, Message: msg})
}

// ---------- internal helpers ----------

// mustUserID extracts the authenticated user UUID from Echo context.
func mustUserID(c echo.Context) uuid.UUID {
	id, ok := c.Get("user_id").(uuid.UUID)
	if !ok {
		panic("auth middleware not applied: user_id not set")
	}
	return id
}

func queryInt(c echo.Context, key string, def int) int {
	v := c.QueryParam(key)
	if v == "" {
		return def
	}
	i, err := strconv.Atoi(v)
	if err != nil || i < 0 {
		return def
	}
	return i
}

func bindAndValidate(c echo.Context, dst interface{}) error {
	if err := c.Bind(dst); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "malformed request body: "+err.Error())
	}
	if err := c.Validate(dst); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, errorResponse{
			Code:    "validation_error",
			Message: err.Error(),
		})
	}
	return nil
}

// ---------- compile-time guards ----------

var _ = time.Now
var _ = repositories.ErrNotFound
