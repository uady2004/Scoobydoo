package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/tiktok-clone/ecommerce-service/internal/models"
	"github.com/tiktok-clone/ecommerce-service/internal/services"
)

// CartHandler exposes shopping cart management endpoints.
// All routes require an authenticated user (JWT middleware applied at the
// router group level by the caller).
type CartHandler struct {
	svc    services.CartService
	logger *zap.Logger
}

// NewCartHandler creates a new CartHandler.
func NewCartHandler(svc services.CartService, logger *zap.Logger) *CartHandler {
	return &CartHandler{svc: svc, logger: logger}
}

// RegisterRoutes registers all cart routes under the given authenticated group.
func (h *CartHandler) RegisterRoutes(auth gin.IRouter) {
	cart := auth.Group("/cart")
	cart.GET("", h.GetCart)
	cart.POST("/items", h.AddItem)
	cart.DELETE("/items/:item_id", h.RemoveItem)
	cart.PUT("/items/:item_id", h.UpdateItemQuantity)
	cart.POST("/checkout", h.Checkout)
}

// ──────────────────────────────────────────────────────────────────────────────
// Handlers
// ──────────────────────────────────────────────────────────────────────────────

// GetCart godoc
//
//	@Summary     Get the current user's cart
//	@Description Returns the cart with all items and computed totals.
//	             An empty cart is created on first access.
//	@Tags        cart
//	@Produce     json
//	@Success     200 {object} models.Cart
//	@Router      /cart [get]
func (h *CartHandler) GetCart(c *gin.Context) {
	userID, _, ok := callerIdentity(c)
	if !ok {
		return
	}

	cart, err := h.svc.GetCart(c.Request.Context(), userID)
	if err != nil {
		h.logger.Error("get cart failed", zap.String("user_id", userID.String()), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load cart"})
		return
	}

	c.JSON(http.StatusOK, cart)
}

// AddItem godoc
//
//	@Summary     Add an item to the cart
//	@Description Validates product availability and stock, then adds (or
//	             increments) the item. Returns the updated cart.
//	@Tags        cart
//	@Accept      json
//	@Produce     json
//	@Param       body body     models.AddToCartRequest true "Add to cart payload"
//	@Success     200  {object} models.Cart
//	@Router      /cart/items [post]
func (h *CartHandler) AddItem(c *gin.Context) {
	userID, _, ok := callerIdentity(c)
	if !ok {
		return
	}

	var req models.AddToCartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cart, err := h.svc.AddToCart(c.Request.Context(), userID, &req)
	if err != nil {
		h.logger.Error("add to cart failed",
			zap.String("user_id", userID.String()),
			zap.String("product_id", req.ProductID.String()),
			zap.Error(err),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, cart)
}

// RemoveItem godoc
//
//	@Summary     Remove an item from the cart
//	@Tags        cart
//	@Produce     json
//	@Param       item_id path string true "Cart item UUID"
//	@Success     204
//	@Router      /cart/items/{item_id} [delete]
func (h *CartHandler) RemoveItem(c *gin.Context) {
	userID, _, ok := callerIdentity(c)
	if !ok {
		return
	}

	itemID, err := uuid.Parse(c.Param("item_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid item id"})
		return
	}

	if err := h.svc.RemoveFromCart(c.Request.Context(), userID, itemID); err != nil {
		h.logger.Error("remove from cart failed", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// UpdateItemQuantity godoc
//
//	@Summary     Update item quantity
//	@Description Sets the exact quantity. Sending quantity=0 removes the item.
//	@Tags        cart
//	@Accept      json
//	@Produce     json
//	@Param       item_id path     string                          true "Cart item UUID"
//	@Param       body    body     models.UpdateCartQuantityRequest true "Quantity"
//	@Success     200     {object} models.Cart
//	@Router      /cart/items/{item_id} [put]
func (h *CartHandler) UpdateItemQuantity(c *gin.Context) {
	userID, _, ok := callerIdentity(c)
	if !ok {
		return
	}

	itemID, err := uuid.Parse(c.Param("item_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid item id"})
		return
	}

	var req models.UpdateCartQuantityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cart, err := h.svc.UpdateQuantity(c.Request.Context(), userID, itemID, &req)
	if err != nil {
		h.logger.Error("update cart quantity failed", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, cart)
}

// Checkout godoc
//
//	@Summary     Checkout the cart
//	@Description Validates stock, splits items by seller into individual orders,
//	             reserves inventory, and clears the cart. Returns all created orders.
//	@Tags        cart
//	@Accept      json
//	@Produce     json
//	@Param       body body     models.CheckoutCartRequest true "Checkout payload"
//	@Success     201  {object} models.CheckoutResponse
//	@Router      /cart/checkout [post]
func (h *CartHandler) Checkout(c *gin.Context) {
	userID, _, ok := callerIdentity(c)
	if !ok {
		return
	}

	var req models.CheckoutCartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.svc.CheckoutCart(c.Request.Context(), userID, &req)
	if err != nil {
		h.logger.Error("checkout failed",
			zap.String("user_id", userID.String()), zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, resp)
}
