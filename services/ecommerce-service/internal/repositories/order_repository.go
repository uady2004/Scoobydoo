package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/tiktok-clone/ecommerce-service/internal/models"
)

// OrderRepository defines all persistence operations for orders, carts and returns.
type OrderRepository interface {
	// Orders
	CreateOrder(ctx context.Context, order *models.Order) error
	GetOrder(ctx context.Context, orderID uuid.UUID) (*models.Order, error)
	UpdateOrderStatus(ctx context.Context, orderID uuid.UUID, status models.OrderStatus, meta OrderStatusMeta) error
	UpdatePaymentStatus(ctx context.Context, orderID uuid.UUID, status models.PaymentStatus, paymentRef string) error
	GetOrdersByUser(ctx context.Context, buyerID uuid.UUID, f models.OrderListFilters) ([]models.Order, int, error)
	GetOrdersBySeller(ctx context.Context, sellerID uuid.UUID, f models.OrderListFilters) ([]models.Order, int, error)

	// Returns & Refunds
	CreateReturn(ctx context.Context, ret *models.Return) error
	GetReturn(ctx context.Context, returnID uuid.UUID) (*models.Return, error)
	UpdateReturnStatus(ctx context.Context, returnID uuid.UUID, status models.ReturnStatus) error
	ProcessRefund(ctx context.Context, refund *models.Refund) error
	GetRefund(ctx context.Context, refundID uuid.UUID) (*models.Refund, error)
	UpdateRefundStatus(ctx context.Context, refundID uuid.UUID, status models.RefundStatus, gatewayRef, failureReason string) error

	// Cart
	GetCart(ctx context.Context, userID uuid.UUID) (*models.Cart, error)
	GetCartByID(ctx context.Context, cartID uuid.UUID) (*models.Cart, error)
	CreateCart(ctx context.Context, cart *models.Cart) error
	AddToCart(ctx context.Context, item *models.CartItem) error
	RemoveFromCart(ctx context.Context, cartID, itemID uuid.UUID) error
	UpdateCartQuantity(ctx context.Context, cartID, itemID uuid.UUID, qty int) error
	ClearCart(ctx context.Context, cartID uuid.UUID) error
}

// OrderStatusMeta holds optional metadata when transitioning order status.
type OrderStatusMeta struct {
	TrackingNumber string
	TrackingURL    string
	Note           string
	ShippedAt      *time.Time
	DeliveredAt    *time.Time
}

type pgOrderRepository struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

// NewOrderRepository returns a PostgreSQL-backed OrderRepository.
func NewOrderRepository(pool *pgxpool.Pool, logger *zap.Logger) OrderRepository {
	return &pgOrderRepository{pool: pool, logger: logger}
}

// ──────────────────────────────────────────────────────────────────────────────
// Orders
// ──────────────────────────────────────────────────────────────────────────────

// CreateOrder inserts the order header and all line items in a single transaction.
func (r *pgOrderRepository) CreateOrder(ctx context.Context, order *models.Order) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("order_repo: begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	addrJSON, err := json.Marshal(order.ShippingAddress)
	if err != nil {
		return fmt.Errorf("order_repo: marshal shipping address: %w", err)
	}

	now := time.Now().UTC()
	order.CreatedAt = now
	order.UpdatedAt = now

	const q = `
		INSERT INTO orders (
			id, buyer_id, seller_id, status, payment_status, payment_method,
			sub_total, shipping_fee, discount, tax, total, currency,
			shipping_address, shipping_method, notes, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`

	_, err = tx.Exec(ctx, q,
		order.ID, order.BuyerID, order.SellerID,
		string(order.Status), string(order.PaymentStatus), order.PaymentMethod,
		order.SubTotal, order.ShippingFee, order.Discount, order.Tax, order.Total, order.Currency,
		addrJSON, order.ShippingMethod, order.Notes, order.CreatedAt, order.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("order_repo: insert order: %w", err)
	}

	for i := range order.Items {
		item := &order.Items[i]
		item.OrderID = order.ID
		item.CreatedAt = now

		const itemQ = `
			INSERT INTO order_items (
				id, order_id, product_id, variant_id, seller_id,
				product_name, variant_name, image_url, sku,
				quantity, unit_price, discount, total, is_reviewed, created_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`

		_, err = tx.Exec(ctx, itemQ,
			item.ID, item.OrderID, item.ProductID, item.VariantID, item.SellerID,
			item.ProductName, item.VariantName, item.ImageURL, item.SKU,
			item.Quantity, item.UnitPrice, item.Discount, item.Total, false, item.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("order_repo: insert order item: %w", err)
		}
	}

	return tx.Commit(ctx)
}

// GetOrder fetches a full order including its items and returns.
func (r *pgOrderRepository) GetOrder(ctx context.Context, orderID uuid.UUID) (*models.Order, error) {
	const q = `
		SELECT id, buyer_id, seller_id, status, payment_status, payment_method, payment_ref,
		       sub_total, shipping_fee, discount, tax, total, currency,
		       shipping_address, shipping_method, tracking_number, tracking_url,
		       notes, cancel_reason, estimated_delivery,
		       paid_at, shipped_at, delivered_at, created_at, updated_at
		FROM orders WHERE id=$1`

	row := r.pool.QueryRow(ctx, q, orderID)
	order, err := r.scanOrder(row)
	if err != nil {
		return nil, fmt.Errorf("order_repo: get order: %w", err)
	}

	items, err := r.getOrderItems(ctx, orderID)
	if err != nil {
		return nil, err
	}
	order.Items = items

	returns, err := r.getOrderReturns(ctx, orderID)
	if err != nil {
		return nil, err
	}
	order.Returns = returns

	return order, nil
}

func (r *pgOrderRepository) scanOrder(row pgx.Row) (*models.Order, error) {
	var o models.Order
	var addrRaw []byte
	var status, payStatus string

	err := row.Scan(
		&o.ID, &o.BuyerID, &o.SellerID, &status, &payStatus, &o.PaymentMethod, &o.PaymentRef,
		&o.SubTotal, &o.ShippingFee, &o.Discount, &o.Tax, &o.Total, &o.Currency,
		&addrRaw, &o.ShippingMethod, &o.TrackingNumber, &o.TrackingURL,
		&o.Notes, &o.CancelReason, &o.EstimatedDelivery,
		&o.PaidAt, &o.ShippedAt, &o.DeliveredAt, &o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("order not found")
		}
		return nil, err
	}

	o.Status = models.OrderStatus(status)
	o.PaymentStatus = models.PaymentStatus(payStatus)
	if len(addrRaw) > 0 {
		_ = json.Unmarshal(addrRaw, &o.ShippingAddress)
	}
	return &o, nil
}

func (r *pgOrderRepository) getOrderItems(ctx context.Context, orderID uuid.UUID) ([]models.OrderItem, error) {
	const q = `
		SELECT id, order_id, product_id, variant_id, seller_id,
		       product_name, variant_name, image_url, sku,
		       quantity, unit_price, discount, total, is_reviewed, created_at
		FROM order_items WHERE order_id=$1 ORDER BY created_at ASC`

	rows, err := r.pool.Query(ctx, q, orderID)
	if err != nil {
		return nil, fmt.Errorf("order_repo: get items: %w", err)
	}
	defer rows.Close()

	var items []models.OrderItem
	for rows.Next() {
		var item models.OrderItem
		err := rows.Scan(
			&item.ID, &item.OrderID, &item.ProductID, &item.VariantID, &item.SellerID,
			&item.ProductName, &item.VariantName, &item.ImageURL, &item.SKU,
			&item.Quantity, &item.UnitPrice, &item.Discount, &item.Total, &item.IsReviewed, &item.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *pgOrderRepository) getOrderReturns(ctx context.Context, orderID uuid.UUID) ([]models.Return, error) {
	const q = `
		SELECT id, order_id, buyer_id, status, reason, description,
		       image_urls, refund_amount, tracking_number,
		       approved_at, received_at, created_at, updated_at
		FROM returns WHERE order_id=$1 ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, q, orderID)
	if err != nil {
		return nil, fmt.Errorf("order_repo: get returns: %w", err)
	}
	defer rows.Close()

	var returns []models.Return
	for rows.Next() {
		var ret models.Return
		var status string
		err := rows.Scan(
			&ret.ID, &ret.OrderID, &ret.BuyerID, &status, &ret.Reason, &ret.Description,
			&ret.ImageURLs, &ret.RefundAmount, &ret.TrackingNumber,
			&ret.ApprovedAt, &ret.ReceivedAt, &ret.CreatedAt, &ret.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		ret.Status = models.ReturnStatus(status)
		returns = append(returns, ret)
	}
	return returns, rows.Err()
}

// UpdateOrderStatus transitions an order to the given status and records metadata.
func (r *pgOrderRepository) UpdateOrderStatus(ctx context.Context, orderID uuid.UUID, status models.OrderStatus, meta OrderStatusMeta) error {
	const q = `
		UPDATE orders SET
			status=$1, tracking_number=COALESCE(NULLIF($2,''), tracking_number),
			tracking_url=COALESCE(NULLIF($3,''), tracking_url),
			shipped_at=COALESCE($4, shipped_at),
			delivered_at=COALESCE($5, delivered_at),
			updated_at=$6
		WHERE id=$7`

	_, err := r.pool.Exec(ctx, q,
		string(status), meta.TrackingNumber, meta.TrackingURL,
		meta.ShippedAt, meta.DeliveredAt,
		time.Now().UTC(), orderID,
	)
	if err != nil {
		return fmt.Errorf("order_repo: update status: %w", err)
	}
	return nil
}

// UpdatePaymentStatus records payment confirmation on an order.
func (r *pgOrderRepository) UpdatePaymentStatus(ctx context.Context, orderID uuid.UUID, status models.PaymentStatus, paymentRef string) error {
	var paidAt *time.Time
	if status == models.PaymentStatusPaid {
		now := time.Now().UTC()
		paidAt = &now
	}
	const q = `
		UPDATE orders SET
			payment_status=$1, payment_ref=COALESCE(NULLIF($2,''), payment_ref),
			paid_at=COALESCE($3, paid_at), updated_at=$4
		WHERE id=$5`

	_, err := r.pool.Exec(ctx, q, string(status), paymentRef, paidAt, time.Now().UTC(), orderID)
	if err != nil {
		return fmt.Errorf("order_repo: update payment status: %w", err)
	}
	return nil
}

// GetOrdersByUser returns paginated orders for a buyer.
func (r *pgOrderRepository) GetOrdersByUser(ctx context.Context, buyerID uuid.UUID, f models.OrderListFilters) ([]models.Order, int, error) {
	return r.listOrders(ctx, "buyer_id", buyerID, f)
}

// GetOrdersBySeller returns paginated orders for a seller.
func (r *pgOrderRepository) GetOrdersBySeller(ctx context.Context, sellerID uuid.UUID, f models.OrderListFilters) ([]models.Order, int, error) {
	return r.listOrders(ctx, "seller_id", sellerID, f)
}

func (r *pgOrderRepository) listOrders(ctx context.Context, field string, id uuid.UUID, f models.OrderListFilters) ([]models.Order, int, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 || f.PageSize > 50 {
		f.PageSize = 20
	}

	args := []interface{}{id}
	argIdx := 2
	cond := fmt.Sprintf("%s=$1", field)

	if f.Status != nil {
		cond += fmt.Sprintf(" AND status=$%d", argIdx)
		args = append(args, string(*f.Status))
		argIdx++
	}
	if f.FromDate != nil {
		cond += fmt.Sprintf(" AND created_at>=$%d", argIdx)
		args = append(args, *f.FromDate)
		argIdx++
	}
	if f.ToDate != nil {
		cond += fmt.Sprintf(" AND created_at<=$%d", argIdx)
		args = append(args, *f.ToDate)
		argIdx++
	}

	var total int
	countQ := fmt.Sprintf("SELECT COUNT(*) FROM orders WHERE %s", cond)
	if err := r.pool.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("order_repo: list count: %w", err)
	}

	offset := (f.Page - 1) * f.PageSize
	listQ := fmt.Sprintf(`
		SELECT id, buyer_id, seller_id, status, payment_status, payment_method, payment_ref,
		       sub_total, shipping_fee, discount, tax, total, currency,
		       shipping_address, shipping_method, tracking_number, tracking_url,
		       notes, cancel_reason, estimated_delivery,
		       paid_at, shipped_at, delivered_at, created_at, updated_at
		FROM orders WHERE %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d`, cond, argIdx, argIdx+1)

	args = append(args, f.PageSize, offset)
	rows, err := r.pool.Query(ctx, listQ, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("order_repo: list orders: %w", err)
	}
	defer rows.Close()

	var orders []models.Order
	for rows.Next() {
		o, err := r.scanOrder(rows)
		if err != nil {
			return nil, 0, err
		}
		orders = append(orders, *o)
	}
	return orders, total, rows.Err()
}

// ──────────────────────────────────────────────────────────────────────────────
// Returns & Refunds
// ──────────────────────────────────────────────────────────────────────────────

// CreateReturn inserts a return request and its line items.
func (r *pgOrderRepository) CreateReturn(ctx context.Context, ret *models.Return) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("order_repo: begin return tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	now := time.Now().UTC()
	ret.CreatedAt = now
	ret.UpdatedAt = now

	const q = `
		INSERT INTO returns (
			id, order_id, buyer_id, status, reason, description,
			image_urls, refund_amount, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`

	_, err = tx.Exec(ctx, q,
		ret.ID, ret.OrderID, ret.BuyerID, string(ret.Status), ret.Reason, ret.Description,
		ret.ImageURLs, ret.RefundAmount, ret.CreatedAt, ret.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("order_repo: insert return: %w", err)
	}

	for _, item := range ret.Items {
		const itemQ = `
			INSERT INTO return_items (id, return_id, order_item_id, quantity, reason)
			VALUES ($1,$2,$3,$4,$5)`
		_, err = tx.Exec(ctx, itemQ, item.ID, ret.ID, item.OrderItemID, item.Quantity, item.Reason)
		if err != nil {
			return fmt.Errorf("order_repo: insert return item: %w", err)
		}
	}

	// Transition order to returning status
	const statusQ = `UPDATE orders SET status='returning', updated_at=$1 WHERE id=$2`
	_, _ = tx.Exec(ctx, statusQ, now, ret.OrderID)

	return tx.Commit(ctx)
}

// GetReturn fetches a return by ID including its items.
func (r *pgOrderRepository) GetReturn(ctx context.Context, returnID uuid.UUID) (*models.Return, error) {
	const q = `
		SELECT id, order_id, buyer_id, status, reason, description,
		       image_urls, refund_amount, tracking_number,
		       approved_at, received_at, created_at, updated_at
		FROM returns WHERE id=$1`

	var ret models.Return
	var status string
	err := r.pool.QueryRow(ctx, q, returnID).Scan(
		&ret.ID, &ret.OrderID, &ret.BuyerID, &status, &ret.Reason, &ret.Description,
		&ret.ImageURLs, &ret.RefundAmount, &ret.TrackingNumber,
		&ret.ApprovedAt, &ret.ReceivedAt, &ret.CreatedAt, &ret.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("return not found")
		}
		return nil, fmt.Errorf("order_repo: get return: %w", err)
	}
	ret.Status = models.ReturnStatus(status)

	// Load items
	const itemsQ = `
		SELECT id, return_id, order_item_id, quantity, reason
		FROM return_items WHERE return_id=$1`
	rows, err := r.pool.Query(ctx, itemsQ, returnID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var item models.ReturnItem
		if err := rows.Scan(&item.ID, &item.ReturnID, &item.OrderItemID, &item.Quantity, &item.Reason); err != nil {
			return nil, err
		}
		ret.Items = append(ret.Items, item)
	}

	return &ret, rows.Err()
}

// UpdateReturnStatus transitions a return to a new status.
func (r *pgOrderRepository) UpdateReturnStatus(ctx context.Context, returnID uuid.UUID, status models.ReturnStatus) error {
	now := time.Now().UTC()
	var approvedAt, receivedAt *time.Time
	switch status {
	case models.ReturnStatusApproved:
		approvedAt = &now
	case models.ReturnStatusReceived:
		receivedAt = &now
	}

	const q = `
		UPDATE returns SET
			status=$1,
			approved_at=COALESCE($2, approved_at),
			received_at=COALESCE($3, received_at),
			updated_at=$4
		WHERE id=$5`

	_, err := r.pool.Exec(ctx, q, string(status), approvedAt, receivedAt, now, returnID)
	if err != nil {
		return fmt.Errorf("order_repo: update return status: %w", err)
	}
	return nil
}

// ProcessRefund inserts a new refund record in pending state.
func (r *pgOrderRepository) ProcessRefund(ctx context.Context, refund *models.Refund) error {
	now := time.Now().UTC()
	refund.CreatedAt = now
	refund.UpdatedAt = now

	const q = `
		INSERT INTO refunds (
			id, order_id, return_id, amount, currency, reason,
			status, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`

	_, err := r.pool.Exec(ctx, q,
		refund.ID, refund.OrderID, refund.ReturnID, refund.Amount, refund.Currency, refund.Reason,
		string(refund.Status), refund.CreatedAt, refund.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("order_repo: process refund: %w", err)
	}
	return nil
}

// GetRefund fetches a refund record by ID.
func (r *pgOrderRepository) GetRefund(ctx context.Context, refundID uuid.UUID) (*models.Refund, error) {
	const q = `
		SELECT id, order_id, return_id, amount, currency, reason,
		       status, gateway_ref, failure_reason, processed_at, created_at, updated_at
		FROM refunds WHERE id=$1`

	var ref models.Refund
	var status string
	err := r.pool.QueryRow(ctx, q, refundID).Scan(
		&ref.ID, &ref.OrderID, &ref.ReturnID, &ref.Amount, &ref.Currency, &ref.Reason,
		&status, &ref.GatewayRef, &ref.FailureReason, &ref.ProcessedAt, &ref.CreatedAt, &ref.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("refund not found")
		}
		return nil, fmt.Errorf("order_repo: get refund: %w", err)
	}
	ref.Status = models.RefundStatus(status)
	return &ref, nil
}

// UpdateRefundStatus records the outcome of a payment-gateway refund call.
func (r *pgOrderRepository) UpdateRefundStatus(ctx context.Context, refundID uuid.UUID, status models.RefundStatus, gatewayRef, failureReason string) error {
	now := time.Now().UTC()
	var processedAt *time.Time
	if status == models.RefundStatusProcessed {
		processedAt = &now
	}

	const q = `
		UPDATE refunds SET
			status=$1,
			gateway_ref=COALESCE(NULLIF($2,''), gateway_ref),
			failure_reason=COALESCE(NULLIF($3,''), failure_reason),
			processed_at=COALESCE($4, processed_at),
			updated_at=$5
		WHERE id=$6`

	_, err := r.pool.Exec(ctx, q, string(status), gatewayRef, failureReason, processedAt, now, refundID)
	if err != nil {
		return fmt.Errorf("order_repo: update refund status: %w", err)
	}
	return nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Cart
// ──────────────────────────────────────────────────────────────────────────────

// GetCart fetches a user's cart and all its items.
func (r *pgOrderRepository) GetCart(ctx context.Context, userID uuid.UUID) (*models.Cart, error) {
	const q = `SELECT id, user_id, created_at, updated_at FROM carts WHERE user_id=$1`
	var cart models.Cart
	err := r.pool.QueryRow(ctx, q, userID).Scan(&cart.ID, &cart.UserID, &cart.CreatedAt, &cart.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("cart not found")
		}
		return nil, fmt.Errorf("order_repo: get cart: %w", err)
	}

	items, err := r.getCartItems(ctx, cart.ID)
	if err != nil {
		return nil, err
	}
	cart.Items = items
	for _, item := range items {
		cart.Total += item.UnitPrice * float64(item.Quantity)
		cart.ItemCount += item.Quantity
	}
	return &cart, nil
}

// GetCartByID fetches a cart by its own ID (used at checkout).
func (r *pgOrderRepository) GetCartByID(ctx context.Context, cartID uuid.UUID) (*models.Cart, error) {
	const q = `SELECT id, user_id, created_at, updated_at FROM carts WHERE id=$1`
	var cart models.Cart
	err := r.pool.QueryRow(ctx, q, cartID).Scan(&cart.ID, &cart.UserID, &cart.CreatedAt, &cart.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("cart not found")
		}
		return nil, fmt.Errorf("order_repo: get cart by id: %w", err)
	}

	items, err := r.getCartItems(ctx, cart.ID)
	if err != nil {
		return nil, err
	}
	cart.Items = items
	for _, item := range items {
		cart.Total += item.UnitPrice * float64(item.Quantity)
		cart.ItemCount += item.Quantity
	}
	return &cart, nil
}

func (r *pgOrderRepository) getCartItems(ctx context.Context, cartID uuid.UUID) ([]models.CartItem, error) {
	const q = `
		SELECT id, cart_id, product_id, variant_id, seller_id,
		       product_name, variant_name, image_url, sku,
		       quantity, unit_price, created_at, updated_at
		FROM cart_items WHERE cart_id=$1 ORDER BY created_at ASC`

	rows, err := r.pool.Query(ctx, q, cartID)
	if err != nil {
		return nil, fmt.Errorf("order_repo: get cart items: %w", err)
	}
	defer rows.Close()

	var items []models.CartItem
	for rows.Next() {
		var item models.CartItem
		err := rows.Scan(
			&item.ID, &item.CartID, &item.ProductID, &item.VariantID, &item.SellerID,
			&item.ProductName, &item.VariantName, &item.ImageURL, &item.SKU,
			&item.Quantity, &item.UnitPrice, &item.CreatedAt, &item.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// CreateCart inserts an empty cart for a user.
func (r *pgOrderRepository) CreateCart(ctx context.Context, cart *models.Cart) error {
	now := time.Now().UTC()
	cart.CreatedAt = now
	cart.UpdatedAt = now

	const q = `INSERT INTO carts (id, user_id, created_at, updated_at) VALUES ($1,$2,$3,$4)`
	_, err := r.pool.Exec(ctx, q, cart.ID, cart.UserID, cart.CreatedAt, cart.UpdatedAt)
	if err != nil {
		return fmt.Errorf("order_repo: create cart: %w", err)
	}
	return nil
}

// AddToCart inserts a new cart item or increments quantity if the same
// product/variant already exists in the cart.
func (r *pgOrderRepository) AddToCart(ctx context.Context, item *models.CartItem) error {
	now := time.Now().UTC()
	item.CreatedAt = now
	item.UpdatedAt = now

	// Use an upsert: if (cart_id, product_id, variant_id) already exists, add qty.
	const q = `
		INSERT INTO cart_items (
			id, cart_id, product_id, variant_id, seller_id,
			product_name, variant_name, image_url, sku,
			quantity, unit_price, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		ON CONFLICT (cart_id, product_id, COALESCE(variant_id, '00000000-0000-0000-0000-000000000000'))
		DO UPDATE SET
			quantity = cart_items.quantity + EXCLUDED.quantity,
			unit_price = EXCLUDED.unit_price,
			updated_at = EXCLUDED.updated_at`

	_, err := r.pool.Exec(ctx, q,
		item.ID, item.CartID, item.ProductID, item.VariantID, item.SellerID,
		item.ProductName, item.VariantName, item.ImageURL, item.SKU,
		item.Quantity, item.UnitPrice, item.CreatedAt, item.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("order_repo: add to cart: %w", err)
	}

	// Touch cart updated_at
	_, _ = r.pool.Exec(ctx, `UPDATE carts SET updated_at=$1 WHERE id=$2`, now, item.CartID)
	return nil
}

// RemoveFromCart deletes a specific item from the cart.
func (r *pgOrderRepository) RemoveFromCart(ctx context.Context, cartID, itemID uuid.UUID) error {
	const q = `DELETE FROM cart_items WHERE id=$1 AND cart_id=$2`
	tag, err := r.pool.Exec(ctx, q, itemID, cartID)
	if err != nil {
		return fmt.Errorf("order_repo: remove from cart: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("cart item not found")
	}
	_, _ = r.pool.Exec(ctx, `UPDATE carts SET updated_at=$1 WHERE id=$2`, time.Now().UTC(), cartID)
	return nil
}

// UpdateCartQuantity sets an exact quantity on a cart item; qty=0 removes the item.
func (r *pgOrderRepository) UpdateCartQuantity(ctx context.Context, cartID, itemID uuid.UUID, qty int) error {
	if qty == 0 {
		return r.RemoveFromCart(ctx, cartID, itemID)
	}

	const q = `UPDATE cart_items SET quantity=$1, updated_at=$2 WHERE id=$3 AND cart_id=$4`
	tag, err := r.pool.Exec(ctx, q, qty, time.Now().UTC(), itemID, cartID)
	if err != nil {
		return fmt.Errorf("order_repo: update cart quantity: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("cart item not found")
	}
	_, _ = r.pool.Exec(ctx, `UPDATE carts SET updated_at=$1 WHERE id=$2`, time.Now().UTC(), cartID)
	return nil
}

// ClearCart removes all items from a cart (called after successful checkout).
func (r *pgOrderRepository) ClearCart(ctx context.Context, cartID uuid.UUID) error {
	const q = `DELETE FROM cart_items WHERE cart_id=$1`
	_, err := r.pool.Exec(ctx, q, cartID)
	if err != nil {
		return fmt.Errorf("order_repo: clear cart: %w", err)
	}
	_, _ = r.pool.Exec(ctx, `UPDATE carts SET updated_at=$1 WHERE id=$2`, time.Now().UTC(), cartID)
	return nil
}
