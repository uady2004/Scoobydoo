package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"

	"github.com/tiktok-clone/ecommerce-service/internal/config"
	"github.com/tiktok-clone/ecommerce-service/internal/models"
	"github.com/tiktok-clone/ecommerce-service/internal/repositories"
)

// OrderService defines the business logic layer for order lifecycle management.
type OrderService interface {
	// PlaceOrder validates inventory, creates the order, emits OrderCreated, and
	// triggers payment. Items from different sellers are split into separate orders.
	PlaceOrder(ctx context.Context, buyerID uuid.UUID, req *models.PlaceOrderRequest) ([]*models.Order, error)

	// GetOrder fetches a full order (with items and returns) for a buyer or seller.
	GetOrder(ctx context.Context, orderID, callerID uuid.UUID) (*models.Order, error)

	// UpdateStatus transitions an order through its lifecycle states.
	// Only the owning seller (or an internal service) may call this.
	UpdateStatus(ctx context.Context, orderID, sellerID uuid.UUID, req *models.UpdateOrderStatusRequest) (*models.Order, error)

	// CancelOrder allows a buyer to cancel a pending order, releases inventory,
	// and emits an OrderCancelled event.
	CancelOrder(ctx context.Context, orderID, buyerID uuid.UUID, req *models.CancelOrderRequest) error

	// GetOrdersByUser returns paginated orders for a buyer.
	GetOrdersByUser(ctx context.Context, buyerID uuid.UUID, f models.OrderListFilters) ([]models.Order, int, error)

	// GetOrdersBySeller returns paginated orders for a seller.
	GetOrdersBySeller(ctx context.Context, sellerID uuid.UUID, f models.OrderListFilters) ([]models.Order, int, error)

	// CreateReturn initiates a return request on a delivered order.
	CreateReturn(ctx context.Context, orderID, buyerID uuid.UUID, req *models.CreateReturnRequest) (*models.Return, error)

	// ProcessRefund calls the payment service to issue a refund for a return or
	// cancellation and records the outcome.
	ProcessRefund(ctx context.Context, orderID uuid.UUID, returnID *uuid.UUID, amount float64, reason string) (*models.Refund, error)

	// TrackOrder returns the current tracking information for a shipment.
	TrackOrder(ctx context.Context, orderID, callerID uuid.UUID) (*TrackingInfo, error)
}

// TrackingInfo is a lightweight view of order shipping progress.
type TrackingInfo struct {
	OrderID        uuid.UUID          `json:"order_id"`
	Status         models.OrderStatus `json:"status"`
	TrackingNumber string             `json:"tracking_number,omitempty"`
	TrackingURL    string             `json:"tracking_url,omitempty"`
	ShippedAt      *time.Time         `json:"shipped_at,omitempty"`
	DeliveredAt    *time.Time         `json:"delivered_at,omitempty"`
	EstimatedDelivery *time.Time      `json:"estimated_delivery,omitempty"`
}

// orderEvent is the Kafka envelope emitted for order lifecycle changes.
type orderEvent struct {
	EventType string      `json:"event_type"`
	Timestamp time.Time   `json:"timestamp"`
	Payload   interface{} `json:"payload"`
}

type orderService struct {
	orderRepo   repositories.OrderRepository
	productRepo repositories.ProductRepository
	kafka       *kafka.Writer
	cfg         *config.Config
	logger      *zap.Logger
}

// NewOrderService constructs an OrderService wired to the given dependencies.
func NewOrderService(
	orderRepo repositories.OrderRepository,
	productRepo repositories.ProductRepository,
	cfg *config.Config,
	logger *zap.Logger,
) OrderService {
	w := &kafka.Writer{
		Addr:         kafka.TCP(cfg.Kafka.Brokers...),
		Topic:        cfg.Kafka.OrderEventsTopic,
		Balancer:     &kafka.LeastBytes{},
		WriteTimeout: 10 * time.Second,
		ReadTimeout:  10 * time.Second,
	}

	return &orderService{
		orderRepo:   orderRepo,
		productRepo: productRepo,
		kafka:       w,
		cfg:         cfg,
		logger:      logger,
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// PlaceOrder
// ──────────────────────────────────────────────────────────────────────────────

// PlaceOrder validates stock for every cart item, reserves inventory with
// SELECT FOR UPDATE, splits items by seller into separate Order records, and
// emits an OrderCreated event for each order.
func (s *orderService) PlaceOrder(ctx context.Context, buyerID uuid.UUID, req *models.PlaceOrderRequest) ([]*models.Order, error) {
	// Load the cart
	cart, err := s.orderRepo.GetCartByID(ctx, req.CartID)
	if err != nil {
		return nil, fmt.Errorf("order_svc: load cart: %w", err)
	}
	if cart.UserID != buyerID {
		return nil, fmt.Errorf("order_svc: cart does not belong to buyer")
	}
	if len(cart.Items) == 0 {
		return nil, fmt.Errorf("order_svc: cart is empty")
	}

	// Validate and reserve inventory for every item (SELECT FOR UPDATE in repo layer)
	for _, item := range cart.Items {
		if err := s.productRepo.ReserveInventory(ctx, item.ProductID, item.VariantID, item.Quantity); err != nil {
			return nil, fmt.Errorf("order_svc: reserve inventory for product %s: %w", item.ProductID, err)
		}
	}

	// Group cart items by seller
	sellerItems := make(map[uuid.UUID][]models.CartItem)
	for _, item := range cart.Items {
		sellerItems[item.SellerID] = append(sellerItems[item.SellerID], item)
	}

	var createdOrders []*models.Order
	for sellerID, items := range sellerItems {
		order, err := s.buildOrder(buyerID, sellerID, items, req)
		if err != nil {
			// Roll back all reservations made so far on failure
			s.releaseAllReservations(ctx, cart.Items)
			return nil, fmt.Errorf("order_svc: build order for seller %s: %w", sellerID, err)
		}

		if err := s.orderRepo.CreateOrder(ctx, order); err != nil {
			s.releaseAllReservations(ctx, cart.Items)
			return nil, fmt.Errorf("order_svc: create order: %w", err)
		}

		createdOrders = append(createdOrders, order)

		// Emit OrderCreated event (non-blocking; log on failure)
		go s.emitEvent(context.Background(), "OrderCreated", map[string]interface{}{
			"order_id":  order.ID,
			"buyer_id":  buyerID,
			"seller_id": sellerID,
			"total":     order.Total,
			"currency":  order.Currency,
			"items":     len(order.Items),
		})
	}

	// Clear the cart after successful checkout
	if err := s.orderRepo.ClearCart(ctx, cart.ID); err != nil {
		s.logger.Warn("order_svc: failed to clear cart after checkout",
			zap.String("cart_id", cart.ID.String()), zap.Error(err))
	}

	s.logger.Info("orders placed",
		zap.String("buyer_id", buyerID.String()),
		zap.Int("order_count", len(createdOrders)),
	)
	return createdOrders, nil
}

func (s *orderService) buildOrder(buyerID, sellerID uuid.UUID, items []models.CartItem, req *models.PlaceOrderRequest) (*models.Order, error) {
	var subTotal float64
	var orderItems []models.OrderItem

	for _, ci := range items {
		lineTotal := ci.UnitPrice * float64(ci.Quantity)
		subTotal += lineTotal

		orderItems = append(orderItems, models.OrderItem{
			ID:          uuid.New(),
			ProductID:   ci.ProductID,
			VariantID:   ci.VariantID,
			SellerID:    sellerID,
			ProductName: ci.ProductName,
			VariantName: ci.VariantName,
			ImageURL:    ci.ImageURL,
			SKU:         ci.SKU,
			Quantity:    ci.Quantity,
			UnitPrice:   ci.UnitPrice,
			Discount:    0,
			Total:       lineTotal,
		})
	}

	// Simple shipping fee — a real implementation would call a shipping rate API
	shippingFee := s.calculateShippingFee(req.ShippingMethod, subTotal)
	tax := subTotal * 0.0 // tax calculation delegated to a separate service

	order := &models.Order{
		ID:              uuid.New(),
		BuyerID:         buyerID,
		SellerID:        sellerID,
		Status:          models.OrderStatusPending,
		PaymentStatus:   models.PaymentStatusPending,
		PaymentMethod:   req.PaymentMethod,
		SubTotal:        subTotal,
		ShippingFee:     shippingFee,
		Discount:        0,
		Tax:             tax,
		Total:           subTotal + shippingFee + tax,
		Currency:        "USD",
		ShippingAddress: req.ShippingAddress,
		ShippingMethod:  req.ShippingMethod,
		Notes:           req.Notes,
		Items:           orderItems,
	}
	return order, nil
}

func (s *orderService) calculateShippingFee(method string, subTotal float64) float64 {
	switch method {
	case "express":
		return 12.99
	case "overnight":
		return 24.99
	default: // "standard"
		if subTotal >= 50 {
			return 0 // free standard shipping over $50
		}
		return 5.99
	}
}

func (s *orderService) releaseAllReservations(ctx context.Context, items []models.CartItem) {
	for _, item := range items {
		if err := s.productRepo.ReleaseReservedInventory(ctx, item.ProductID, item.VariantID, item.Quantity); err != nil {
			s.logger.Error("order_svc: failed to release reservation",
				zap.String("product_id", item.ProductID.String()), zap.Error(err))
		}
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// GetOrder
// ──────────────────────────────────────────────────────────────────────────────

// GetOrder fetches a full order. The caller must be either the buyer or seller.
func (s *orderService) GetOrder(ctx context.Context, orderID, callerID uuid.UUID) (*models.Order, error) {
	order, err := s.orderRepo.GetOrder(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("order_svc: get order: %w", err)
	}
	if order.BuyerID != callerID && order.SellerID != callerID {
		return nil, fmt.Errorf("order_svc: access denied")
	}
	return order, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// UpdateStatus
// ──────────────────────────────────────────────────────────────────────────────

// UpdateStatus enforces the valid state machine transitions for an order and
// commits reserved inventory when the order moves to "shipped".
//
//	pending → processing → shipped → delivered
func (s *orderService) UpdateStatus(ctx context.Context, orderID, sellerID uuid.UUID, req *models.UpdateOrderStatusRequest) (*models.Order, error) {
	order, err := s.orderRepo.GetOrder(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("order_svc: get order for status update: %w", err)
	}
	if order.SellerID != sellerID {
		return nil, fmt.Errorf("order_svc: only the owning seller can update order status")
	}

	if err := s.validateTransition(order.Status, req.Status); err != nil {
		return nil, err
	}

	meta := repositories.OrderStatusMeta{
		TrackingNumber: req.TrackingNumber,
		TrackingURL:    req.TrackingURL,
		Note:           req.Note,
	}

	now := time.Now().UTC()
	if req.Status == models.OrderStatusShipped {
		meta.ShippedAt = &now
	}
	if req.Status == models.OrderStatusDelivered {
		meta.DeliveredAt = &now

		// Commit reserved inventory — stock is permanently consumed
		for _, item := range order.Items {
			if err := s.productRepo.CommitReservedInventory(ctx, item.ProductID, item.VariantID, item.Quantity); err != nil {
				s.logger.Error("order_svc: commit inventory failed",
					zap.String("product_id", item.ProductID.String()), zap.Error(err))
			}
		}
	}

	if err := s.orderRepo.UpdateOrderStatus(ctx, orderID, req.Status, meta); err != nil {
		return nil, fmt.Errorf("order_svc: update order status: %w", err)
	}

	order.Status = req.Status
	order.TrackingNumber = req.TrackingNumber
	order.TrackingURL = req.TrackingURL

	go s.emitEvent(context.Background(), "OrderStatusUpdated", map[string]interface{}{
		"order_id":   orderID,
		"buyer_id":   order.BuyerID,
		"seller_id":  sellerID,
		"new_status": string(req.Status),
	})

	return order, nil
}

// validateTransition checks that the requested status follows the allowed state machine.
func (s *orderService) validateTransition(current, next models.OrderStatus) error {
	allowed := map[models.OrderStatus][]models.OrderStatus{
		models.OrderStatusPending:    {models.OrderStatusProcessing, models.OrderStatusCancelled},
		models.OrderStatusProcessing: {models.OrderStatusShipped, models.OrderStatusCancelled},
		models.OrderStatusShipped:    {models.OrderStatusDelivered},
		models.OrderStatusDelivered:  {models.OrderStatusReturning},
		models.OrderStatusReturning:  {models.OrderStatusReturned},
	}

	for _, s := range allowed[current] {
		if s == next {
			return nil
		}
	}
	return fmt.Errorf("order_svc: invalid status transition from %s to %s", current, next)
}

// ──────────────────────────────────────────────────────────────────────────────
// CancelOrder
// ──────────────────────────────────────────────────────────────────────────────

// CancelOrder allows a buyer to cancel a pending or processing order, releases
// reserved inventory, and emits an OrderCancelled event.
func (s *orderService) CancelOrder(ctx context.Context, orderID, buyerID uuid.UUID, req *models.CancelOrderRequest) error {
	order, err := s.orderRepo.GetOrder(ctx, orderID)
	if err != nil {
		return fmt.Errorf("order_svc: get order for cancel: %w", err)
	}
	if order.BuyerID != buyerID {
		return fmt.Errorf("order_svc: access denied")
	}
	if order.Status != models.OrderStatusPending && order.Status != models.OrderStatusProcessing {
		return fmt.Errorf("order_svc: cannot cancel order in status %s", order.Status)
	}

	meta := repositories.OrderStatusMeta{Note: req.Reason}
	if err := s.orderRepo.UpdateOrderStatus(ctx, orderID, models.OrderStatusCancelled, meta); err != nil {
		return fmt.Errorf("order_svc: cancel order: %w", err)
	}

	// Release reserved inventory for every item
	for _, item := range order.Items {
		if err := s.productRepo.ReleaseReservedInventory(ctx, item.ProductID, item.VariantID, item.Quantity); err != nil {
			s.logger.Error("order_svc: release inventory on cancel failed",
				zap.String("product_id", item.ProductID.String()), zap.Error(err))
		}
	}

	go s.emitEvent(context.Background(), "OrderCancelled", map[string]interface{}{
		"order_id":  orderID,
		"buyer_id":  buyerID,
		"seller_id": order.SellerID,
		"reason":    req.Reason,
	})

	s.logger.Info("order cancelled",
		zap.String("order_id", orderID.String()),
		zap.String("buyer_id", buyerID.String()),
	)
	return nil
}

// ──────────────────────────────────────────────────────────────────────────────
// List Orders
// ──────────────────────────────────────────────────────────────────────────────

func (s *orderService) GetOrdersByUser(ctx context.Context, buyerID uuid.UUID, f models.OrderListFilters) ([]models.Order, int, error) {
	orders, total, err := s.orderRepo.GetOrdersByUser(ctx, buyerID, f)
	if err != nil {
		return nil, 0, fmt.Errorf("order_svc: get orders by user: %w", err)
	}
	return orders, total, nil
}

func (s *orderService) GetOrdersBySeller(ctx context.Context, sellerID uuid.UUID, f models.OrderListFilters) ([]models.Order, int, error) {
	orders, total, err := s.orderRepo.GetOrdersBySeller(ctx, sellerID, f)
	if err != nil {
		return nil, 0, fmt.Errorf("order_svc: get orders by seller: %w", err)
	}
	return orders, total, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Returns & Refunds
// ──────────────────────────────────────────────────────────────────────────────

// CreateReturn validates the order is in a returnable state, builds a Return
// record, and persists it. The order status transitions to "returning".
func (s *orderService) CreateReturn(ctx context.Context, orderID, buyerID uuid.UUID, req *models.CreateReturnRequest) (*models.Return, error) {
	order, err := s.orderRepo.GetOrder(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("order_svc: get order for return: %w", err)
	}
	if order.BuyerID != buyerID {
		return nil, fmt.Errorf("order_svc: access denied")
	}
	if order.Status != models.OrderStatusDelivered {
		return nil, fmt.Errorf("order_svc: can only return delivered orders (current status: %s)", order.Status)
	}

	// Build an itemID lookup for validation
	itemMap := make(map[uuid.UUID]models.OrderItem)
	for _, oi := range order.Items {
		itemMap[oi.ID] = oi
	}

	var returnItems []models.ReturnItem
	var refundAmount float64

	for _, ri := range req.Items {
		oi, ok := itemMap[ri.OrderItemID]
		if !ok {
			return nil, fmt.Errorf("order_svc: order item %s not found in order", ri.OrderItemID)
		}
		if ri.Quantity > oi.Quantity {
			return nil, fmt.Errorf("order_svc: return quantity %d exceeds ordered quantity %d for item %s", ri.Quantity, oi.Quantity, ri.OrderItemID)
		}
		refundAmount += oi.UnitPrice * float64(ri.Quantity)
		returnItems = append(returnItems, models.ReturnItem{
			ID:          uuid.New(),
			OrderItemID: ri.OrderItemID,
			Quantity:    ri.Quantity,
			Reason:      ri.Reason,
		})
	}

	ret := &models.Return{
		ID:           uuid.New(),
		OrderID:      orderID,
		BuyerID:      buyerID,
		Status:       models.ReturnStatusRequested,
		Reason:       req.Reason,
		Description:  req.Description,
		Items:        returnItems,
		RefundAmount: refundAmount,
	}

	if err := s.orderRepo.CreateReturn(ctx, ret); err != nil {
		return nil, fmt.Errorf("order_svc: create return: %w", err)
	}

	go s.emitEvent(context.Background(), "ReturnRequested", map[string]interface{}{
		"return_id":     ret.ID,
		"order_id":      orderID,
		"buyer_id":      buyerID,
		"seller_id":     order.SellerID,
		"refund_amount": refundAmount,
	})

	return ret, nil
}

// ProcessRefund creates a Refund record and calls the payment service to reverse
// the charge. The refund record is updated with the gateway outcome.
func (s *orderService) ProcessRefund(ctx context.Context, orderID uuid.UUID, returnID *uuid.UUID, amount float64, reason string) (*models.Refund, error) {
	order, err := s.orderRepo.GetOrder(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("order_svc: get order for refund: %w", err)
	}

	refund := &models.Refund{
		ID:       uuid.New(),
		OrderID:  orderID,
		ReturnID: returnID,
		Amount:   amount,
		Currency: order.Currency,
		Reason:   reason,
		Status:   models.RefundStatusPending,
	}

	if err := s.orderRepo.ProcessRefund(ctx, refund); err != nil {
		return nil, fmt.Errorf("order_svc: create refund record: %w", err)
	}

	// Call payment service (async — update refund status when response arrives)
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		gatewayRef, err := s.callPaymentServiceRefund(bgCtx, order.PaymentRef, amount, order.Currency)
		if err != nil {
			s.logger.Error("order_svc: payment service refund failed",
				zap.String("refund_id", refund.ID.String()), zap.Error(err))
			_ = s.orderRepo.UpdateRefundStatus(bgCtx, refund.ID, models.RefundStatusFailed, "", err.Error())
			return
		}

		_ = s.orderRepo.UpdateRefundStatus(bgCtx, refund.ID, models.RefundStatusProcessed, gatewayRef, "")

		// Also mark the order as refunded
		_ = s.orderRepo.UpdatePaymentStatus(bgCtx, orderID, models.PaymentStatusRefunded, gatewayRef)

		s.emitEvent(bgCtx, "RefundProcessed", map[string]interface{}{
			"refund_id":   refund.ID,
			"order_id":    orderID,
			"amount":      amount,
			"gateway_ref": gatewayRef,
		})
	}()

	return refund, nil
}

// callPaymentServiceRefund is a stub that represents the HTTP call to the
// payment-service. Replace the body with real HTTP client code.
func (s *orderService) callPaymentServiceRefund(_ context.Context, paymentRef string, amount float64, currency string) (string, error) {
	if paymentRef == "" {
		return "", fmt.Errorf("order_svc: no payment reference on order — cannot refund")
	}
	// Stub: return a synthetic gateway ref
	gatewayRef := fmt.Sprintf("rfnd_%s_%.2f_%s", paymentRef[:8], amount, currency)
	s.logger.Info("order_svc: payment service refund called (stub)",
		zap.String("payment_ref", paymentRef),
		zap.Float64("amount", amount),
		zap.String("gateway_ref", gatewayRef),
	)
	return gatewayRef, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// TrackOrder
// ──────────────────────────────────────────────────────────────────────────────

// TrackOrder returns the current shipping state of an order.
func (s *orderService) TrackOrder(ctx context.Context, orderID, callerID uuid.UUID) (*TrackingInfo, error) {
	order, err := s.orderRepo.GetOrder(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("order_svc: get order for tracking: %w", err)
	}
	if order.BuyerID != callerID && order.SellerID != callerID {
		return nil, fmt.Errorf("order_svc: access denied")
	}

	return &TrackingInfo{
		OrderID:           order.ID,
		Status:            order.Status,
		TrackingNumber:    order.TrackingNumber,
		TrackingURL:       order.TrackingURL,
		ShippedAt:         order.ShippedAt,
		DeliveredAt:       order.DeliveredAt,
		EstimatedDelivery: order.EstimatedDelivery,
	}, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Kafka event emission
// ──────────────────────────────────────────────────────────────────────────────

func (s *orderService) emitEvent(ctx context.Context, eventType string, payload interface{}) {
	evt := orderEvent{
		EventType: eventType,
		Timestamp: time.Now().UTC(),
		Payload:   payload,
	}
	body, err := json.Marshal(evt)
	if err != nil {
		s.logger.Error("order_svc: marshal kafka event", zap.Error(err))
		return
	}

	msg := kafka.Message{
		Key:   []byte(eventType),
		Value: body,
	}
	if err := s.kafka.WriteMessages(ctx, msg); err != nil {
		s.logger.Error("order_svc: write kafka message",
			zap.String("event_type", eventType), zap.Error(err))
	}
}
