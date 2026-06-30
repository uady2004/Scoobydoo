package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/tiktok-clone/ecommerce-service/internal/models"
	"github.com/tiktok-clone/ecommerce-service/internal/repositories"
)

// CartService defines the business logic layer for shopping cart management.
type CartService interface {
	// AddToCart adds a product (and optional variant) to the user's cart,
	// validating that sufficient stock is available.
	AddToCart(ctx context.Context, userID uuid.UUID, req *models.AddToCartRequest) (*models.Cart, error)

	// RemoveFromCart removes a specific item from the cart.
	RemoveFromCart(ctx context.Context, userID, itemID uuid.UUID) error

	// UpdateQuantity sets the exact quantity of a cart item. Passing qty=0
	// removes the item.
	UpdateQuantity(ctx context.Context, userID, itemID uuid.UUID, req *models.UpdateCartQuantityRequest) (*models.Cart, error)

	// GetCart returns the current cart for a user, creating an empty one if
	// none exists.
	GetCart(ctx context.Context, userID uuid.UUID) (*models.Cart, error)

	// CheckoutCart validates all items have sufficient stock, calculates totals,
	// and delegates to OrderService.PlaceOrder to convert the cart into orders.
	CheckoutCart(ctx context.Context, userID uuid.UUID, req *models.CheckoutCartRequest) (*models.CheckoutResponse, error)
}

type cartService struct {
	orderRepo   repositories.OrderRepository
	productRepo repositories.ProductRepository
	orderSvc    OrderService
	logger      *zap.Logger
}

// NewCartService constructs a CartService wired to the given dependencies.
// orderSvc is used by CheckoutCart to delegate order creation.
func NewCartService(
	orderRepo repositories.OrderRepository,
	productRepo repositories.ProductRepository,
	orderSvc OrderService,
	logger *zap.Logger,
) CartService {
	return &cartService{
		orderRepo:   orderRepo,
		productRepo: productRepo,
		orderSvc:    orderSvc,
		logger:      logger,
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// GetCart
// ──────────────────────────────────────────────────────────────────────────────

// GetCart returns the user's current cart. If no cart exists one is created.
func (s *cartService) GetCart(ctx context.Context, userID uuid.UUID) (*models.Cart, error) {
	cart, err := s.orderRepo.GetCart(ctx, userID)
	if err != nil {
		// "cart not found" — create one
		cart = &models.Cart{
			ID:     uuid.New(),
			UserID: userID,
		}
		if createErr := s.orderRepo.CreateCart(ctx, cart); createErr != nil {
			return nil, fmt.Errorf("cart_svc: create cart: %w", createErr)
		}
	}
	return cart, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// AddToCart
// ──────────────────────────────────────────────────────────────────────────────

// AddToCart validates product existence and available stock, then adds (or
// increments) the item in the user's cart. If no cart exists it is created.
func (s *cartService) AddToCart(ctx context.Context, userID uuid.UUID, req *models.AddToCartRequest) (*models.Cart, error) {
	// Validate product exists and is active
	product, err := s.productRepo.GetProduct(ctx, req.ProductID)
	if err != nil {
		return nil, fmt.Errorf("cart_svc: product not found: %w", err)
	}
	if product.Status != models.ProductStatusActive {
		return nil, fmt.Errorf("cart_svc: product is not available for purchase")
	}

	// Determine unit price and seller ID (variant takes precedence)
	unitPrice := product.BasePrice
	var variantName string
	var imagURL string
	if len(product.ImageURLs) > 0 {
		imagURL = product.ImageURLs[0]
	}

	if req.VariantID != nil {
		found := false
		for _, v := range product.Variants {
			if v.ID == *req.VariantID {
				unitPrice = v.Price
				if v.SalePrice != nil {
					unitPrice = *v.SalePrice
				}
				variantName = v.Name
				if v.ImageURL != "" {
					imagURL = v.ImageURL
				}
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("cart_svc: variant %s not found on product", *req.VariantID)
		}
	} else if product.SalePrice != nil {
		unitPrice = *product.SalePrice
	}

	// Validate available stock (non-locking read; the hard lock happens at PlaceOrder)
	inv, err := s.productRepo.GetInventory(ctx, req.ProductID, req.VariantID)
	if err != nil {
		return nil, fmt.Errorf("cart_svc: check inventory: %w", err)
	}
	if inv.TrackInventory && inv.AvailableQty < req.Quantity && !inv.AllowBackorder {
		return nil, fmt.Errorf("cart_svc: insufficient stock (available: %d, requested: %d)", inv.AvailableQty, req.Quantity)
	}

	// Retrieve or create the cart
	cart, err := s.GetCart(ctx, userID)
	if err != nil {
		return nil, err
	}

	item := &models.CartItem{
		ID:          uuid.New(),
		CartID:      cart.ID,
		ProductID:   req.ProductID,
		VariantID:   req.VariantID,
		SellerID:    product.SellerID,
		ProductName: product.Name,
		VariantName: variantName,
		ImageURL:    imagURL,
		SKU:         product.SKU,
		Quantity:    req.Quantity,
		UnitPrice:   unitPrice,
	}

	if err := s.orderRepo.AddToCart(ctx, item); err != nil {
		return nil, fmt.Errorf("cart_svc: add to cart: %w", err)
	}

	// Return fresh cart state
	return s.orderRepo.GetCartByID(ctx, cart.ID)
}

// ──────────────────────────────────────────────────────────────────────────────
// RemoveFromCart
// ──────────────────────────────────────────────────────────────────────────────

// RemoveFromCart removes a specific item from the user's cart, verifying
// ownership before deletion.
func (s *cartService) RemoveFromCart(ctx context.Context, userID, itemID uuid.UUID) error {
	cart, err := s.orderRepo.GetCart(ctx, userID)
	if err != nil {
		return fmt.Errorf("cart_svc: get cart for remove: %w", err)
	}

	if err := s.orderRepo.RemoveFromCart(ctx, cart.ID, itemID); err != nil {
		return fmt.Errorf("cart_svc: remove from cart: %w", err)
	}
	return nil
}

// ──────────────────────────────────────────────────────────────────────────────
// UpdateQuantity
// ──────────────────────────────────────────────────────────────────────────────

// UpdateQuantity sets the exact quantity of a cart item. qty=0 removes the item.
// Stock is re-validated if the quantity is being increased.
func (s *cartService) UpdateQuantity(ctx context.Context, userID, itemID uuid.UUID, req *models.UpdateCartQuantityRequest) (*models.Cart, error) {
	cart, err := s.orderRepo.GetCart(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("cart_svc: get cart for quantity update: %w", err)
	}

	// Find the item to validate stock if qty is being raised
	if req.Quantity > 0 {
		for _, ci := range cart.Items {
			if ci.ID == itemID {
				if ci.Quantity < req.Quantity {
					// Check stock for the additional units
					inv, err := s.productRepo.GetInventory(ctx, ci.ProductID, ci.VariantID)
					if err != nil {
						return nil, fmt.Errorf("cart_svc: check inventory: %w", err)
					}
					if inv.TrackInventory && inv.AvailableQty < req.Quantity && !inv.AllowBackorder {
						return nil, fmt.Errorf("cart_svc: insufficient stock (available: %d, requested: %d)", inv.AvailableQty, req.Quantity)
					}
				}
				break
			}
		}
	}

	if err := s.orderRepo.UpdateCartQuantity(ctx, cart.ID, itemID, req.Quantity); err != nil {
		return nil, fmt.Errorf("cart_svc: update quantity: %w", err)
	}

	return s.orderRepo.GetCartByID(ctx, cart.ID)
}

// ──────────────────────────────────────────────────────────────────────────────
// CheckoutCart
// ──────────────────────────────────────────────────────────────────────────────

// CheckoutCart validates all cart items have sufficient stock, builds a
// PlaceOrderRequest, and delegates to OrderService.PlaceOrder which handles
// inventory reservation and order creation transactionally.
func (s *cartService) CheckoutCart(ctx context.Context, userID uuid.UUID, req *models.CheckoutCartRequest) (*models.CheckoutResponse, error) {
	cart, err := s.orderRepo.GetCart(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("cart_svc: load cart for checkout: %w", err)
	}
	if len(cart.Items) == 0 {
		return nil, fmt.Errorf("cart_svc: cart is empty")
	}

	// Pre-validate stock levels (soft check; hard lock is inside PlaceOrder)
	for _, item := range cart.Items {
		inv, err := s.productRepo.GetInventory(ctx, item.ProductID, item.VariantID)
		if err != nil {
			return nil, fmt.Errorf("cart_svc: inventory check for product %s: %w", item.ProductID, err)
		}
		if inv.TrackInventory && inv.AvailableQty < item.Quantity && !inv.AllowBackorder {
			return nil, fmt.Errorf("cart_svc: product %s (%s) has insufficient stock — available %d, in cart %d",
				item.ProductName, item.ProductID, inv.AvailableQty, item.Quantity)
		}
	}

	placeReq := &models.PlaceOrderRequest{
		CartID:          cart.ID,
		ShippingAddress: req.ShippingAddress,
		ShippingMethod:  req.ShippingMethod,
		PaymentMethod:   req.PaymentMethod,
		Notes:           req.Notes,
		CouponCode:      req.CouponCode,
	}

	orders, err := s.orderSvc.PlaceOrder(ctx, userID, placeReq)
	if err != nil {
		return nil, fmt.Errorf("cart_svc: place order: %w", err)
	}

	var grandTotal float64
	var orderModels []models.Order
	for _, o := range orders {
		grandTotal += o.Total
		orderModels = append(orderModels, *o)
	}

	s.logger.Info("cart checked out",
		zap.String("user_id", userID.String()),
		zap.Int("orders_created", len(orders)),
		zap.Float64("grand_total", grandTotal),
	)

	return &models.CheckoutResponse{
		Orders:  orderModels,
		Total:   grandTotal,
		Message: fmt.Sprintf("%d order(s) created successfully", len(orders)),
	}, nil
}
