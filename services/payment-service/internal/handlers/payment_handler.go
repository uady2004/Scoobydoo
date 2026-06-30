package handlers

import (
	"errors"
	"io"
	"net/http"
	"strconv"

	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/tiktok-clone/payment-service/internal/config"
	"github.com/tiktok-clone/payment-service/internal/services"
)

// PaymentHandler wires together Echo routes and the PaymentService.
type PaymentHandler struct {
	svc    services.PaymentService
	stripe services.StripeService
	cfg    *config.Config
	logger *zap.Logger
}

// NewPaymentHandler constructs a PaymentHandler.
func NewPaymentHandler(
	svc services.PaymentService,
	stripe services.StripeService,
	cfg *config.Config,
	logger *zap.Logger,
) *PaymentHandler {
	return &PaymentHandler{svc: svc, stripe: stripe, cfg: cfg, logger: logger}
}

// RegisterRoutes attaches all payment endpoints to the Echo instance.
func (h *PaymentHandler) RegisterRoutes(e *echo.Echo) {
	// Public health probe.
	e.GET("/healthz", h.Health)

	// Stripe webhook (no JWT auth — signature is verified inside the handler).
	e.POST("/stripe/webhooks", h.StripeWebhook)

	// Internal API (called by wallet-service, not directly by clients).
	internal := e.Group("/internal/v1/payments")
	internal.POST("/coin-purchase", h.ProcessCoinPurchase)
	internal.POST("/subscription", h.ProcessSubscription)
	internal.POST("/withdrawal", h.ProcessWithdrawal)

	// User-facing API (JWT protected).
	api := e.Group("/api/v1/payments", h.authMiddleware())
	api.GET("/history", h.GetPaymentHistory)
	api.POST("/methods", h.AddPaymentMethod)
	api.GET("/methods", h.GetPaymentMethods)
	api.DELETE("/methods/:id", h.RemovePaymentMethod)
	api.POST("/refund", h.RefundPayment)
	api.GET("/publishable-key", h.GetPublishableKey)
}

// ---------- Health ----------

func (h *PaymentHandler) Health(c echo.Context) error {
	return c.JSON(http.StatusOK, echo.Map{"status": "ok", "service": "payment-service"})
}

// GetPublishableKey returns the Stripe publishable key for client-side Stripe.js.
func (h *PaymentHandler) GetPublishableKey(c echo.Context) error {
	return c.JSON(http.StatusOK, echo.Map{
		"publishable_key": h.cfg.Stripe.PublishableKey,
	})
}

// ---------- Stripe webhook ----------

// StripeWebhook godoc
// @Summary  Receive Stripe webhook events
// @Tags     webhooks
// @Accept   application/json
// @Produce  json
// @Header   200 {string} Stripe-Signature "Stripe webhook signature"
// @Success  200 {object} map[string]string
// @Failure  400 {object} errorResponse
// @Router   /stripe/webhooks [post]
//
// IMPORTANT: The request body must NOT be parsed before this handler is called.
// Echo must be configured to read the raw body. The Stripe SDK reads r.Body internally.
func (h *PaymentHandler) StripeWebhook(c echo.Context) error {
	// Read the raw body for signature verification.
	const maxBodyBytes = 65536
	r := c.Request()
	r.Body = http.MaxBytesReader(c.Response().Writer, r.Body, maxBodyBytes)
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Warn("StripeWebhook: failed to read body", zap.Error(err))
		return errBadRequest(c, "read_error", "failed to read request body")
	}

	// Verify webhook signature.
	sigHeader := r.Header.Get("Stripe-Signature")
	event, err := h.stripe.ConstructEvent(payload, sigHeader)
	if err != nil {
		h.logger.Warn("StripeWebhook: signature verification failed",
			zap.String("sig", sigHeader),
			zap.Error(err))
		return errUnauthorized(c, "invalid signature")
	}

	// Dispatch based on event type using the StripeService.
	result, err := h.stripe.HandleWebhook(c.Request().Context(), r)
	if err != nil {
		if errors.Is(err, services.ErrWebhookSignature) {
			return errUnauthorized(c, "invalid signature")
		}
		h.logger.Error("StripeWebhook: handle failed",
			zap.String("event_type", string(event.Type)),
			zap.Error(err))
		// Return 200 to Stripe to prevent retries for non-retriable errors.
		return c.JSON(http.StatusOK, echo.Map{"status": "error_logged"})
	}

	// Update the database based on the webhook outcome.
	if handleErr := h.svc.HandleStripeWebhook(c.Request().Context(), result, payload); handleErr != nil {
		h.logger.Error("StripeWebhook: business logic failed",
			zap.String("event_id", result.EventID),
			zap.Error(handleErr))
		// Return 200 regardless — retrying won't help a business-logic error.
	}

	return c.JSON(http.StatusOK, echo.Map{"received": true, "event_id": result.EventID})
}

// ---------- Internal endpoints (wallet-service → payment-service) ----------

// processCoinPurchaseBody is the request body for ProcessCoinPurchase.
type processCoinPurchaseBody struct {
	UserID          string `json:"user_id"           validate:"required,uuid"`
	PackageID       string `json:"package_id"        validate:"required"`
	PriceCents      int64  `json:"price_cents"       validate:"required,min=1"`
	PaymentMethodID string `json:"payment_method_id" validate:"required"`
	IdempotencyKey  string `json:"idempotency_key"   validate:"required"`
}

// ProcessCoinPurchase is the internal endpoint called by wallet-service.
func (h *PaymentHandler) ProcessCoinPurchase(c echo.Context) error {
	var body processCoinPurchaseBody
	if err := bindAndValidate(c, &body); err != nil {
		return err
	}

	userID, err := uuid.Parse(body.UserID)
	if err != nil {
		return errBadRequest(c, "invalid_user_id", "user_id must be a valid UUID")
	}

	resp, err := h.svc.ProcessCoinPurchase(c.Request().Context(), services.ProcessCoinPurchaseRequest{
		UserID:          userID,
		PackageID:       body.PackageID,
		PriceCents:      body.PriceCents,
		PaymentMethodID: body.PaymentMethodID,
		IdempotencyKey:  body.IdempotencyKey,
	})
	if err != nil {
		h.logger.Error("ProcessCoinPurchase failed", zap.Error(err))
		return errInternal(c, "coin purchase failed: "+err.Error())
	}
	return c.JSON(http.StatusOK, resp)
}

// processSubscriptionBody is the request body for ProcessSubscription.
type processSubscriptionBody struct {
	UserID          string `json:"user_id"           validate:"required,uuid"`
	CreatorID       string `json:"creator_id"        validate:"required,uuid"`
	AmountCents     int64  `json:"amount_cents"      validate:"required,min=1"`
	PaymentMethodID string `json:"payment_method_id" validate:"required"`
	IdempotencyKey  string `json:"idempotency_key"   validate:"required"`
	Description     string `json:"description"`
}

// ProcessSubscription is the internal endpoint for subscription payments.
func (h *PaymentHandler) ProcessSubscription(c echo.Context) error {
	var body processSubscriptionBody
	if err := bindAndValidate(c, &body); err != nil {
		return err
	}

	userID, err := uuid.Parse(body.UserID)
	if err != nil {
		return errBadRequest(c, "invalid_user_id", "user_id must be a valid UUID")
	}
	creatorID, err := uuid.Parse(body.CreatorID)
	if err != nil {
		return errBadRequest(c, "invalid_creator_id", "creator_id must be a valid UUID")
	}

	resp, err := h.svc.ProcessSubscription(c.Request().Context(), services.ProcessSubscriptionRequest{
		UserID:          userID,
		CreatorID:       creatorID,
		AmountCents:     body.AmountCents,
		PaymentMethodID: body.PaymentMethodID,
		IdempotencyKey:  body.IdempotencyKey,
		Description:     body.Description,
	})
	if err != nil {
		h.logger.Error("ProcessSubscription failed", zap.Error(err))
		return errInternal(c, "subscription payment failed: "+err.Error())
	}
	return c.JSON(http.StatusOK, resp)
}

// processWithdrawalBody is the request body for ProcessWithdrawal.
type processWithdrawalBody struct {
	CreatorUserID  string `json:"creator_user_id" validate:"required,uuid"`
	AmountCents    int64  `json:"amount_cents"    validate:"required,min=1"`
	IdempotencyKey string `json:"idempotency_key" validate:"required"`
}

// ProcessWithdrawal is the internal endpoint for creator withdrawals.
func (h *PaymentHandler) ProcessWithdrawal(c echo.Context) error {
	var body processWithdrawalBody
	if err := bindAndValidate(c, &body); err != nil {
		return err
	}

	creatorID, err := uuid.Parse(body.CreatorUserID)
	if err != nil {
		return errBadRequest(c, "invalid_creator_user_id", "creator_user_id must be a valid UUID")
	}

	resp, err := h.svc.ProcessWithdrawal(c.Request().Context(), services.ProcessWithdrawalRequest{
		CreatorUserID:  creatorID,
		AmountCents:    body.AmountCents,
		IdempotencyKey: body.IdempotencyKey,
	})
	if err != nil {
		if errors.Is(err, services.ErrMissingStripeAcct) {
			return errBadRequest(c, "missing_stripe_account", "creator has no linked Stripe account")
		}
		h.logger.Error("ProcessWithdrawal failed", zap.Error(err))
		return errInternal(c, "withdrawal failed: "+err.Error())
	}
	return c.JSON(http.StatusAccepted, resp)
}

// ---------- User-facing endpoints ----------

// GetPaymentHistory godoc
// @Summary  List payment history
// @Tags     payments
// @Security BearerAuth
// @Produce  json
// @Param    limit  query int false "Page size (default 20)"
// @Param    offset query int false "Page offset"
// @Success  200 {object} services.PaymentHistoryResponse
// @Router   /api/v1/payments/history [get]
func (h *PaymentHandler) GetPaymentHistory(c echo.Context) error {
	userID := mustUserID(c)
	limit := queryInt(c, "limit", 20)
	offset := queryInt(c, "offset", 0)

	resp, err := h.svc.GetPaymentHistory(c.Request().Context(), userID, limit, offset)
	if err != nil {
		h.logger.Error("GetPaymentHistory failed", zap.Error(err))
		return errInternal(c, "could not fetch payment history")
	}
	return c.JSON(http.StatusOK, resp)
}

// addPaymentMethodBody is the request body for AddPaymentMethod.
type addPaymentMethodBody struct {
	PaymentMethodID string `json:"payment_method_id" validate:"required"`
	Email           string `json:"email"`
	Name            string `json:"name"`
	SetAsDefault    bool   `json:"set_as_default"`
}

// AddPaymentMethod godoc
// @Summary  Add a payment method to the user's account
// @Tags     payments
// @Security BearerAuth
// @Accept   json
// @Produce  json
// @Param    body body addPaymentMethodBody true "Payment method"
// @Success  201 {object} models.PaymentMethod
// @Router   /api/v1/payments/methods [post]
func (h *PaymentHandler) AddPaymentMethod(c echo.Context) error {
	var body addPaymentMethodBody
	if err := bindAndValidate(c, &body); err != nil {
		return err
	}

	userID := mustUserID(c)
	pm, err := h.svc.AddPaymentMethod(c.Request().Context(), services.AddPaymentMethodRequest{
		UserID:          userID,
		Email:           body.Email,
		Name:            body.Name,
		PaymentMethodID: body.PaymentMethodID,
		SetAsDefault:    body.SetAsDefault,
	})
	if err != nil {
		h.logger.Error("AddPaymentMethod failed", zap.Error(err))
		return errInternal(c, "could not add payment method")
	}
	return c.JSON(http.StatusCreated, pm)
}

// GetPaymentMethods godoc
// @Summary  List saved payment methods
// @Tags     payments
// @Security BearerAuth
// @Produce  json
// @Success  200 {array} models.PaymentMethod
// @Router   /api/v1/payments/methods [get]
func (h *PaymentHandler) GetPaymentMethods(c echo.Context) error {
	userID := mustUserID(c)
	methods, err := h.svc.GetPaymentMethods(c.Request().Context(), userID)
	if err != nil {
		h.logger.Error("GetPaymentMethods failed", zap.Error(err))
		return errInternal(c, "could not fetch payment methods")
	}
	if methods == nil {
		methods = nil // keep slice nil for JSON []
	}
	return c.JSON(http.StatusOK, methods)
}

// RemovePaymentMethod godoc
// @Summary  Remove a saved payment method
// @Tags     payments
// @Security BearerAuth
// @Param    id path string true "Payment method UUID"
// @Success  204
// @Router   /api/v1/payments/methods/{id} [delete]
func (h *PaymentHandler) RemovePaymentMethod(c echo.Context) error {
	userID := mustUserID(c)
	pmIDStr := c.Param("id")
	pmID, err := uuid.Parse(pmIDStr)
	if err != nil {
		return errBadRequest(c, "invalid_id", "payment method id must be a valid UUID")
	}

	if err = h.svc.RemovePaymentMethod(c.Request().Context(), userID, pmID); err != nil {
		if errors.Is(err, services.ErrPaymentNotFound) {
			return errNotFound(c, "payment method not found")
		}
		h.logger.Error("RemovePaymentMethod failed", zap.Error(err))
		return errInternal(c, "could not remove payment method")
	}
	return c.NoContent(http.StatusNoContent)
}

// refundBody is the request body for RefundPayment.
type refundBody struct {
	PaymentID      string `json:"payment_id"      validate:"required,uuid"`
	AmountCents    int64  `json:"amount_cents"`    // 0 = full refund
	IdempotencyKey string `json:"idempotency_key" validate:"required"`
}

// RefundPayment godoc
// @Summary  Refund a payment (admin or support)
// @Tags     payments
// @Security BearerAuth
// @Accept   json
// @Produce  json
// @Param    body body refundBody true "Refund request"
// @Success  200 {object} models.Payment
// @Router   /api/v1/payments/refund [post]
func (h *PaymentHandler) RefundPayment(c echo.Context) error {
	var body refundBody
	if err := bindAndValidate(c, &body); err != nil {
		return err
	}

	paymentID, err := uuid.Parse(body.PaymentID)
	if err != nil {
		return errBadRequest(c, "invalid_payment_id", "payment_id must be a valid UUID")
	}

	payment, err := h.svc.RefundPayment(c.Request().Context(), services.RefundRequest{
		PaymentID:      paymentID,
		AmountCents:    body.AmountCents,
		IdempotencyKey: body.IdempotencyKey,
	})
	if err != nil {
		switch {
		case errors.Is(err, services.ErrAlreadyRefunded):
			return errConflict(c, "already_refunded", "payment has already been fully refunded")
		case errors.Is(err, services.ErrPaymentNotFound):
			return errNotFound(c, "payment not found")
		default:
			h.logger.Error("RefundPayment failed", zap.Error(err))
			return errInternal(c, "refund failed")
		}
	}
	return c.JSON(http.StatusOK, payment)
}

// ---------- Auth middleware ----------

func (h *PaymentHandler) authMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			auth := c.Request().Header.Get("Authorization")
			if len(auth) < 8 || auth[:7] != "Bearer " {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid or missing token")
			}
			claims := jwtv5.MapClaims{}
			_, err := jwtv5.ParseWithClaims(auth[7:], &claims, func(t *jwtv5.Token) (interface{}, error) {
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

// ---------- response / request helpers ----------

type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func errBadRequest(c echo.Context, code, msg string) error {
	return c.JSON(http.StatusBadRequest, errorResponse{Code: code, Message: msg})
}

func errUnauthorized(c echo.Context, msg string) error {
	return c.JSON(http.StatusUnauthorized, errorResponse{Code: "unauthorized", Message: msg})
}

func errNotFound(c echo.Context, msg string) error {
	return c.JSON(http.StatusNotFound, errorResponse{Code: "not_found", Message: msg})
}

func errConflict(c echo.Context, code, msg string) error {
	return c.JSON(http.StatusConflict, errorResponse{Code: code, Message: msg})
}

func errInternal(c echo.Context, msg string) error {
	return c.JSON(http.StatusInternalServerError, errorResponse{Code: "internal_error", Message: msg})
}

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

// JWT type stubs (see wallet_handler.go pattern).
type jwtToken interface {
	GetClaims() jwtMapClaims
}

type jwtMapClaims map[string]interface{}
