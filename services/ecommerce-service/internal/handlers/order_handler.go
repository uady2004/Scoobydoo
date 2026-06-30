package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/tiktok-clone/ecommerce-service/internal/models"
	"github.com/tiktok-clone/ecommerce-service/internal/services"
)

// OrderHandler exposes buyer and seller order management endpoints.
type OrderHandler struct {
	svc    services.OrderService
	logger *zap.Logger
}

// NewOrderHandler creates a new OrderHandler.
func NewOrderHandler(svc services.OrderService, logger *zap.Logger) *OrderHandler {
	return &OrderHandler{svc: svc, logger: logger}
}

// RegisterRoutes registers all order routes on the authenticated router group.
func (h *OrderHandler) RegisterRoutes(auth gin.IRouter) {
	// Buyer order routes
	orders := auth.Group("/orders")
	orders.POST("", h.PlaceOrder)
	orders.GET("", h.ListBuyerOrders)
	orders.GET("/:id", h.GetOrder)
	orders.POST("/:id/cancel", h.CancelOrder)
	orders.GET("/:id/track", h.TrackOrder)
	orders.POST("/:id/returns", h.CreateReturn)
	orders.POST("/:id/refund", h.ProcessRefund)

	// Seller order management routes
	seller := auth.Group("/seller/orders")
	seller.GET("", h.ListSellerOrders)
	seller.GET("/:id", h.GetSellerOrder)
	seller.PUT("/:id/status", h.UpdateOrderStatus)
}

// ──────────────────────────────────────────────────────────────────────────────
// Buyer endpoints
// ──────────────────────────────────────────────────────────────────────────────

// PlaceOrder godoc
//
//	@Summary     Place an order
//	@Description Validates inventory, splits by seller, creates orders, and emits events.
//	@Tags        orders
//	@Accept      json
//	@Produce     json
//	@Param       body body     models.PlaceOrderRequest true "Order payload"
//	@Success     201  {object} map[string]interface{}
//	@Router      /orders [post]
func (h *OrderHandler) PlaceOrder(c *gin.Context) {
	buyerID, _, ok := callerIdentity(c)
	if !ok {
		return
	}

	var req models.PlaceOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	orders, err := h.svc.PlaceOrder(c.Request.Context(), buyerID, &req)
	if err != nil {
		h.logger.Error("place order failed",
			zap.String("buyer_id", buyerID.String()), zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"orders": orders,
		"count":  len(orders),
	})
}

// ListBuyerOrders godoc
//
//	@Summary     List buyer's orders
//	@Tags        orders
//	@Produce     json
//	@Param       status    query string false "Filter by status"
//	@Param       from_date query string false "From date (YYYY-MM-DD)"
//	@Param       to_date   query string false "To date (YYYY-MM-DD)"
//	@Param       page      query int    false "Page number"
//	@Param       page_size query int    false "Page size"
//	@Success     200 {object} map[string]interface{}
//	@Router      /orders [get]
func (h *OrderHandler) ListBuyerOrders(c *gin.Context) {
	buyerID, _, ok := callerIdentity(c)
	if !ok {
		return
	}

	f := parseOrderFilters(c)
	orders, total, err := h.svc.GetOrdersByUser(c.Request.Context(), buyerID, f)
	if err != nil {
		h.logger.Error("list buyer orders failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list orders"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":      orders,
		"total":     total,
		"page":      f.Page,
		"page_size": f.PageSize,
	})
}

// GetOrder godoc
//
//	@Summary     Get a single order
//	@Tags        orders
//	@Produce     json
//	@Param       id path string true "Order UUID"
//	@Success     200 {object} models.Order
//	@Router      /orders/{id} [get]
func (h *OrderHandler) GetOrder(c *gin.Context) {
	orderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order id"})
		return
	}

	callerID, _, ok := callerIdentity(c)
	if !ok {
		return
	}

	order, err := h.svc.GetOrder(c.Request.Context(), orderID, callerID)
	if err != nil {
		h.logger.Error("get order failed", zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found or access denied"})
		return
	}

	c.JSON(http.StatusOK, order)
}

// CancelOrder godoc
//
//	@Summary     Cancel an order
//	@Description Buyer may cancel pending or processing orders. Inventory is released.
//	@Tags        orders
//	@Accept      json
//	@Produce     json
//	@Param       id   path     string                     true "Order UUID"
//	@Param       body body     models.CancelOrderRequest  true "Cancellation reason"
//	@Success     204
//	@Router      /orders/{id}/cancel [post]
func (h *OrderHandler) CancelOrder(c *gin.Context) {
	orderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order id"})
		return
	}

	buyerID, _, ok := callerIdentity(c)
	if !ok {
		return
	}

	var req models.CancelOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.svc.CancelOrder(c.Request.Context(), orderID, buyerID, &req); err != nil {
		h.logger.Error("cancel order failed", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// TrackOrder godoc
//
//	@Summary     Track order shipment
//	@Tags        orders
//	@Produce     json
//	@Param       id path string true "Order UUID"
//	@Success     200 {object} services.TrackingInfo
//	@Router      /orders/{id}/track [get]
func (h *OrderHandler) TrackOrder(c *gin.Context) {
	orderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order id"})
		return
	}

	callerID, _, ok := callerIdentity(c)
	if !ok {
		return
	}

	info, err := h.svc.TrackOrder(c.Request.Context(), orderID, callerID)
	if err != nil {
		h.logger.Error("track order failed", zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found or access denied"})
		return
	}

	c.JSON(http.StatusOK, info)
}

// CreateReturn godoc
//
//	@Summary     Request a return
//	@Description Buyer may request a return on a delivered order.
//	@Tags        orders
//	@Accept      json
//	@Produce     json
//	@Param       id   path     string                      true "Order UUID"
//	@Param       body body     models.CreateReturnRequest  true "Return payload"
//	@Success     201  {object} models.Return
//	@Router      /orders/{id}/returns [post]
func (h *OrderHandler) CreateReturn(c *gin.Context) {
	orderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order id"})
		return
	}

	buyerID, _, ok := callerIdentity(c)
	if !ok {
		return
	}

	var req models.CreateReturnRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ret, err := h.svc.CreateReturn(c.Request.Context(), orderID, buyerID, &req)
	if err != nil {
		h.logger.Error("create return failed", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, ret)
}

// ProcessRefund godoc
//
//	@Summary     Process a refund
//	@Description Initiates a payment reversal for a return or cancellation.
//	@Tags        orders
//	@Accept      json
//	@Produce     json
//	@Param       id   path     string true "Order UUID"
//	@Success     202  {object} models.Refund
//	@Router      /orders/{id}/refund [post]
func (h *OrderHandler) ProcessRefund(c *gin.Context) {
	orderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order id"})
		return
	}

	callerID, _, ok := callerIdentity(c)
	if !ok {
		return
	}

	// Verify the caller is the buyer of this order
	order, err := h.svc.GetOrder(c.Request.Context(), orderID, callerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
		return
	}
	if order.BuyerID != callerID {
		c.JSON(http.StatusForbidden, gin.H{"error": "only the buyer may request a refund"})
		return
	}

	// Parse optional return_id and reason from body
	var body struct {
		ReturnID *uuid.UUID `json:"return_id,omitempty"`
		Reason   string     `json:"reason" binding:"required"`
		Amount   float64    `json:"amount" binding:"required,gt=0"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	refund, err := h.svc.ProcessRefund(c.Request.Context(), orderID, body.ReturnID, body.Amount, body.Reason)
	if err != nil {
		h.logger.Error("process refund failed", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 202 Accepted — the actual reversal is async
	c.JSON(http.StatusAccepted, refund)
}

// ──────────────────────────────────────────────────────────────────────────────
// Seller endpoints
// ──────────────────────────────────────────────────────────────────────────────

// ListSellerOrders godoc
//
//	@Summary     List orders for a seller's shop
//	@Tags        seller
//	@Produce     json
//	@Success     200 {object} map[string]interface{}
//	@Router      /seller/orders [get]
func (h *OrderHandler) ListSellerOrders(c *gin.Context) {
	sellerID, _, ok := callerIdentity(c)
	if !ok {
		return
	}

	f := parseOrderFilters(c)
	orders, total, err := h.svc.GetOrdersBySeller(c.Request.Context(), sellerID, f)
	if err != nil {
		h.logger.Error("list seller orders failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list orders"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":      orders,
		"total":     total,
		"page":      f.Page,
		"page_size": f.PageSize,
	})
}

// GetSellerOrder godoc
//
//	@Summary     Get a single order as the seller
//	@Tags        seller
//	@Produce     json
//	@Param       id path string true "Order UUID"
//	@Success     200 {object} models.Order
//	@Router      /seller/orders/{id} [get]
func (h *OrderHandler) GetSellerOrder(c *gin.Context) {
	orderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order id"})
		return
	}

	sellerID, _, ok := callerIdentity(c)
	if !ok {
		return
	}

	order, err := h.svc.GetOrder(c.Request.Context(), orderID, sellerID)
	if err != nil {
		h.logger.Error("get seller order failed", zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found or access denied"})
		return
	}

	c.JSON(http.StatusOK, order)
}

// UpdateOrderStatus godoc
//
//	@Summary     Update order status (seller)
//	@Description Advances the order through pending→processing→shipped→delivered.
//	@Tags        seller
//	@Accept      json
//	@Produce     json
//	@Param       id   path     string                          true "Order UUID"
//	@Param       body body     models.UpdateOrderStatusRequest true "Status update"
//	@Success     200  {object} models.Order
//	@Router      /seller/orders/{id}/status [put]
func (h *OrderHandler) UpdateOrderStatus(c *gin.Context) {
	orderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order id"})
		return
	}

	sellerID, _, ok := callerIdentity(c)
	if !ok {
		return
	}

	var req models.UpdateOrderStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	order, err := h.svc.UpdateStatus(c.Request.Context(), orderID, sellerID, &req)
	if err != nil {
		h.logger.Error("update order status failed", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, order)
}

// ──────────────────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────────────────

func parseOrderFilters(c *gin.Context) models.OrderListFilters {
	var f models.OrderListFilters
	_ = c.ShouldBindQuery(&f)
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 {
		f.PageSize = 20
	}
	return f
}
