package models

import (
	"time"

	"github.com/google/uuid"
)

// OrderStatus tracks the fulfillment lifecycle of an order.
type OrderStatus string

const (
	OrderStatusPending    OrderStatus = "pending"    // awaiting payment confirmation
	OrderStatusProcessing OrderStatus = "processing" // payment confirmed, preparing
	OrderStatusShipped    OrderStatus = "shipped"    // handed to carrier
	OrderStatusDelivered  OrderStatus = "delivered"  // received by buyer
	OrderStatusCancelled  OrderStatus = "cancelled"
	OrderStatusRefunded   OrderStatus = "refunded"
	OrderStatusReturning  OrderStatus = "returning" // return in transit
	OrderStatusReturned   OrderStatus = "returned"
)

// PaymentStatus tracks payment collection state.
type PaymentStatus string

const (
	PaymentStatusPending   PaymentStatus = "pending"
	PaymentStatusPaid      PaymentStatus = "paid"
	PaymentStatusFailed    PaymentStatus = "failed"
	PaymentStatusRefunded  PaymentStatus = "refunded"
	PaymentStatusPartial   PaymentStatus = "partial_refund"
)

// ReturnStatus tracks the state of a return request.
type ReturnStatus string

const (
	ReturnStatusRequested ReturnStatus = "requested"
	ReturnStatusApproved  ReturnStatus = "approved"
	ReturnStatusRejected  ReturnStatus = "rejected"
	ReturnStatusReceived  ReturnStatus = "received"
	ReturnStatusCompleted ReturnStatus = "completed"
)

// RefundStatus tracks the state of a refund.
type RefundStatus string

const (
	RefundStatusPending   RefundStatus = "pending"
	RefundStatusProcessed RefundStatus = "processed"
	RefundStatusFailed    RefundStatus = "failed"
)

// ShippingAddress is an embedded value object stored as JSONB on the order.
type ShippingAddress struct {
	FullName    string `json:"full_name"`
	Phone       string `json:"phone"`
	AddressLine1 string `json:"address_line1"`
	AddressLine2 string `json:"address_line2,omitempty"`
	City        string `json:"city"`
	State       string `json:"state"`
	PostalCode  string `json:"postal_code"`
	Country     string `json:"country"` // ISO 3166-1 alpha-2
}

// Order is the root entity representing a buyer's purchase from a single seller.
// Cross-seller carts are split into multiple orders at checkout.
type Order struct {
	ID              uuid.UUID       `json:"id" db:"id"`
	BuyerID         uuid.UUID       `json:"buyer_id" db:"buyer_id"`
	SellerID        uuid.UUID       `json:"seller_id" db:"seller_id"`
	Status          OrderStatus     `json:"status" db:"status"`
	PaymentStatus   PaymentStatus   `json:"payment_status" db:"payment_status"`
	PaymentMethod   string          `json:"payment_method" db:"payment_method"`
	PaymentRef      string          `json:"payment_ref,omitempty" db:"payment_ref"` // gateway transaction ID
	SubTotal        float64         `json:"sub_total" db:"sub_total"`
	ShippingFee     float64         `json:"shipping_fee" db:"shipping_fee"`
	Discount        float64         `json:"discount" db:"discount"`
	Tax             float64         `json:"tax" db:"tax"`
	Total           float64         `json:"total" db:"total"`
	Currency        string          `json:"currency" db:"currency"`
	ShippingAddress ShippingAddress `json:"shipping_address" db:"shipping_address"` // JSONB
	ShippingMethod  string          `json:"shipping_method" db:"shipping_method"`
	TrackingNumber  string          `json:"tracking_number,omitempty" db:"tracking_number"`
	TrackingURL     string          `json:"tracking_url,omitempty" db:"tracking_url"`
	Notes           string          `json:"notes,omitempty" db:"notes"`
	CancelReason    string          `json:"cancel_reason,omitempty" db:"cancel_reason"`
	EstimatedDelivery *time.Time    `json:"estimated_delivery,omitempty" db:"estimated_delivery"`
	PaidAt          *time.Time      `json:"paid_at,omitempty" db:"paid_at"`
	ShippedAt       *time.Time      `json:"shipped_at,omitempty" db:"shipped_at"`
	DeliveredAt     *time.Time      `json:"delivered_at,omitempty" db:"delivered_at"`
	CreatedAt       time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at" db:"updated_at"`

	// Populated joins
	Items   []OrderItem `json:"items,omitempty"`
	Returns []Return    `json:"returns,omitempty"`
}

// OrderItem is a single line within an order.
type OrderItem struct {
	ID          uuid.UUID  `json:"id" db:"id"`
	OrderID     uuid.UUID  `json:"order_id" db:"order_id"`
	ProductID   uuid.UUID  `json:"product_id" db:"product_id"`
	VariantID   *uuid.UUID `json:"variant_id,omitempty" db:"variant_id"`
	SellerID    uuid.UUID  `json:"seller_id" db:"seller_id"`
	ProductName string     `json:"product_name" db:"product_name"` // snapshot at order time
	VariantName string     `json:"variant_name,omitempty" db:"variant_name"`
	ImageURL    string     `json:"image_url,omitempty" db:"image_url"`
	SKU         string     `json:"sku" db:"sku"`
	Quantity    int        `json:"quantity" db:"quantity"`
	UnitPrice   float64    `json:"unit_price" db:"unit_price"`
	Discount    float64    `json:"discount" db:"discount"`
	Total       float64    `json:"total" db:"total"` // (unit_price - discount) * quantity
	IsReviewed  bool       `json:"is_reviewed" db:"is_reviewed"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
}

// Cart is the persistent shopping cart stored in PostgreSQL (also cached in Redis).
type Cart struct {
	ID        uuid.UUID  `json:"id" db:"id"`
	UserID    uuid.UUID  `json:"user_id" db:"user_id"`
	Items     []CartItem `json:"items"`
	Total     float64    `json:"total"`
	ItemCount int        `json:"item_count"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt time.Time  `json:"updated_at" db:"updated_at"`
}

// CartItem is a line in the cart.
type CartItem struct {
	ID          uuid.UUID  `json:"id" db:"id"`
	CartID      uuid.UUID  `json:"cart_id" db:"cart_id"`
	ProductID   uuid.UUID  `json:"product_id" db:"product_id"`
	VariantID   *uuid.UUID `json:"variant_id,omitempty" db:"variant_id"`
	SellerID    uuid.UUID  `json:"seller_id" db:"seller_id"`
	ProductName string     `json:"product_name" db:"product_name"`
	VariantName string     `json:"variant_name,omitempty" db:"variant_name"`
	ImageURL    string     `json:"image_url,omitempty" db:"image_url"`
	SKU         string     `json:"sku" db:"sku"`
	Quantity    int        `json:"quantity" db:"quantity"`
	UnitPrice   float64    `json:"unit_price" db:"unit_price"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at" db:"updated_at"`
}

// Return represents a buyer's return request for one or more items.
type Return struct {
	ID          uuid.UUID    `json:"id" db:"id"`
	OrderID     uuid.UUID    `json:"order_id" db:"order_id"`
	BuyerID     uuid.UUID    `json:"buyer_id" db:"buyer_id"`
	Status      ReturnStatus `json:"status" db:"status"`
	Reason      string       `json:"reason" db:"reason"`
	Description string       `json:"description,omitempty" db:"description"`
	ImageURLs   []string     `json:"image_urls,omitempty" db:"image_urls"`
	Items       []ReturnItem `json:"items"`
	RefundAmount float64     `json:"refund_amount" db:"refund_amount"`
	TrackingNumber string    `json:"tracking_number,omitempty" db:"tracking_number"`
	ApprovedAt  *time.Time   `json:"approved_at,omitempty" db:"approved_at"`
	ReceivedAt  *time.Time   `json:"received_at,omitempty" db:"received_at"`
	CreatedAt   time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at" db:"updated_at"`
}

// ReturnItem links a return to a specific order item and quantity.
type ReturnItem struct {
	ID          uuid.UUID `json:"id" db:"id"`
	ReturnID    uuid.UUID `json:"return_id" db:"return_id"`
	OrderItemID uuid.UUID `json:"order_item_id" db:"order_item_id"`
	Quantity    int       `json:"quantity" db:"quantity"`
	Reason      string    `json:"reason,omitempty" db:"reason"`
}

// Refund tracks the payment reversal triggered by a return or cancellation.
type Refund struct {
	ID            uuid.UUID    `json:"id" db:"id"`
	OrderID       uuid.UUID    `json:"order_id" db:"order_id"`
	ReturnID      *uuid.UUID   `json:"return_id,omitempty" db:"return_id"`
	Amount        float64      `json:"amount" db:"amount"`
	Currency      string       `json:"currency" db:"currency"`
	Reason        string       `json:"reason" db:"reason"`
	Status        RefundStatus `json:"status" db:"status"`
	GatewayRef    string       `json:"gateway_ref,omitempty" db:"gateway_ref"` // payment gateway refund ID
	FailureReason string       `json:"failure_reason,omitempty" db:"failure_reason"`
	ProcessedAt   *time.Time   `json:"processed_at,omitempty" db:"processed_at"`
	CreatedAt     time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at" db:"updated_at"`
}

// ──────────────────────────────────────────────────────────────────────────────
// Request / Response DTOs
// ──────────────────────────────────────────────────────────────────────────────

// PlaceOrderRequest is sent by the checkout flow after cart validation.
type PlaceOrderRequest struct {
	CartID          uuid.UUID       `json:"cart_id" binding:"required"`
	ShippingAddress ShippingAddress `json:"shipping_address" binding:"required"`
	ShippingMethod  string          `json:"shipping_method" binding:"required"`
	PaymentMethod   string          `json:"payment_method" binding:"required"`
	Notes           string          `json:"notes,omitempty"`
	CouponCode      string          `json:"coupon_code,omitempty"`
}

// UpdateOrderStatusRequest is sent by a seller or internal service.
type UpdateOrderStatusRequest struct {
	Status         OrderStatus `json:"status" binding:"required"`
	TrackingNumber string      `json:"tracking_number,omitempty"`
	TrackingURL    string      `json:"tracking_url,omitempty"`
	Note           string      `json:"note,omitempty"`
}

// CancelOrderRequest allows a buyer to cancel a pending order.
type CancelOrderRequest struct {
	Reason string `json:"reason" binding:"required,min=5,max=500"`
}

// CreateReturnRequest is submitted by a buyer after delivery.
type CreateReturnRequest struct {
	Reason      string       `json:"reason" binding:"required"`
	Description string       `json:"description,omitempty"`
	Items       []ReturnItemRequest `json:"items" binding:"required,min=1"`
}

// ReturnItemRequest identifies an order item and the quantity being returned.
type ReturnItemRequest struct {
	OrderItemID uuid.UUID `json:"order_item_id" binding:"required"`
	Quantity    int       `json:"quantity" binding:"required,min=1"`
	Reason      string    `json:"reason,omitempty"`
}

// AddToCartRequest is sent when a user adds a product to their cart.
type AddToCartRequest struct {
	ProductID uuid.UUID  `json:"product_id" binding:"required"`
	VariantID *uuid.UUID `json:"variant_id,omitempty"`
	Quantity  int        `json:"quantity" binding:"required,min=1,max=999"`
}

// UpdateCartQuantityRequest adjusts the quantity of an existing cart item.
type UpdateCartQuantityRequest struct {
	Quantity int `json:"quantity" binding:"required,min=0,max=999"`
}

// CheckoutCartRequest finalises a cart into one or more orders.
type CheckoutCartRequest struct {
	ShippingAddress ShippingAddress `json:"shipping_address" binding:"required"`
	ShippingMethod  string          `json:"shipping_method" binding:"required"`
	PaymentMethod   string          `json:"payment_method" binding:"required"`
	Notes           string          `json:"notes,omitempty"`
	CouponCode      string          `json:"coupon_code,omitempty"`
}

// CheckoutResponse is returned after successful checkout; multiple orders may be
// created when a cart contains products from different sellers.
type CheckoutResponse struct {
	Orders  []Order `json:"orders"`
	Total   float64 `json:"total"`
	Message string  `json:"message"`
}

// OrderListFilters carries query-string parameters for order list endpoints.
type OrderListFilters struct {
	Status   *OrderStatus `form:"status"`
	FromDate *time.Time   `form:"from_date" time_format:"2006-01-02"`
	ToDate   *time.Time   `form:"to_date" time_format:"2006-01-02"`
	Page     int          `form:"page"`
	PageSize int          `form:"page_size"`
}
