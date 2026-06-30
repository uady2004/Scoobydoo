package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/tiktok-clone/ecommerce-service/internal/models"
)

// ProductRepository defines all persistence operations for products, variants,
// inventory and reviews.
type ProductRepository interface {
	CreateProduct(ctx context.Context, p *models.Product) error
	UpdateProduct(ctx context.Context, p *models.Product) error
	DeleteProduct(ctx context.Context, productID, sellerID uuid.UUID) error
	GetProduct(ctx context.Context, productID uuid.UUID) (*models.Product, error)
	GetProductBySKU(ctx context.Context, sku string) (*models.Product, error)
	ListProducts(ctx context.Context, f models.ProductListFilters) ([]models.Product, int, error)
	UpdateInventory(ctx context.Context, productID uuid.UUID, variantID *uuid.UUID, delta int) error
	ReserveInventory(ctx context.Context, productID uuid.UUID, variantID *uuid.UUID, qty int) error
	ReleaseReservedInventory(ctx context.Context, productID uuid.UUID, variantID *uuid.UUID, qty int) error
	CommitReservedInventory(ctx context.Context, productID uuid.UUID, variantID *uuid.UUID, qty int) error
	GetInventory(ctx context.Context, productID uuid.UUID, variantID *uuid.UUID) (*models.Inventory, error)
	CreateReview(ctx context.Context, r *models.Review) error
	GetReviews(ctx context.Context, productID uuid.UUID, page, pageSize int) ([]models.Review, int, error)
	SearchProducts(ctx context.Context, query string, f models.ProductListFilters) ([]models.Product, int, error)
	CreateVariant(ctx context.Context, v *models.ProductVariant) error
	UpdateVariant(ctx context.Context, v *models.ProductVariant) error
	GetVariants(ctx context.Context, productID uuid.UUID) ([]models.ProductVariant, error)
}

type pgProductRepository struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

// NewProductRepository returns a PostgreSQL-backed ProductRepository.
func NewProductRepository(pool *pgxpool.Pool, logger *zap.Logger) ProductRepository {
	return &pgProductRepository{pool: pool, logger: logger}
}

// ──────────────────────────────────────────────────────────────────────────────
// Product CRUD
// ──────────────────────────────────────────────────────────────────────────────

// CreateProduct inserts a new product row and, inside the same transaction,
// inserts variants and an initial inventory record.
func (r *pgProductRepository) CreateProduct(ctx context.Context, p *models.Product) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("product_repo: begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	dimsJSON, err := json.Marshal(p.Dimensions)
	if err != nil {
		return fmt.Errorf("product_repo: marshal dimensions: %w", err)
	}
	attrsJSON, err := json.Marshal(p.Attributes)
	if err != nil {
		return fmt.Errorf("product_repo: marshal attributes: %w", err)
	}

	const q = `
		INSERT INTO products (
			id, seller_id, category_id, name, description, short_desc, slug,
			base_price, sale_price, currency, sku, barcode, weight, dimensions,
			image_urls, video_url, tags, attributes, status,
			is_digital, requires_shipping, created_at, updated_at
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,
			$8,$9,$10,$11,$12,$13,$14,
			$15,$16,$17,$18,$19,
			$20,$21,$22,$23
		)`

	now := time.Now().UTC()
	p.CreatedAt = now
	p.UpdatedAt = now

	_, err = tx.Exec(ctx, q,
		p.ID, p.SellerID, p.CategoryID, p.Name, p.Description, p.ShortDesc, p.Slug,
		p.BasePrice, p.SalePrice, p.Currency, p.SKU, p.Barcode, p.Weight, dimsJSON,
		p.ImageURLs, p.VideoURL, p.Tags, attrsJSON, string(p.Status),
		p.IsDigital, p.RequiresShipping, p.CreatedAt, p.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("product_repo: insert product: %w", err)
	}

	// Insert variants
	for i := range p.Variants {
		if err := r.insertVariantTx(ctx, tx, &p.Variants[i]); err != nil {
			return err
		}
	}

	// Insert base inventory record
	if p.Inventory != nil {
		if err := r.insertInventoryTx(ctx, tx, p.Inventory); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *pgProductRepository) insertVariantTx(ctx context.Context, tx pgx.Tx, v *models.ProductVariant) error {
	optsJSON, err := json.Marshal(v.Options)
	if err != nil {
		return fmt.Errorf("product_repo: marshal variant options: %w", err)
	}
	now := time.Now().UTC()
	v.CreatedAt = now
	v.UpdatedAt = now

	const q = `
		INSERT INTO product_variants (
			id, product_id, name, sku, barcode, price, sale_price,
			image_url, options, sort_order, is_active, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`

	_, err = tx.Exec(ctx, q,
		v.ID, v.ProductID, v.Name, v.SKU, v.Barcode, v.Price, v.SalePrice,
		v.ImageURL, optsJSON, v.SortOrder, v.IsActive, v.CreatedAt, v.UpdatedAt,
	)
	return err
}

func (r *pgProductRepository) insertInventoryTx(ctx context.Context, tx pgx.Tx, inv *models.Inventory) error {
	const q = `
		INSERT INTO inventory (
			id, product_id, variant_id, quantity, reserved_qty,
			low_stock_alert, track_inventory, allow_backorder,
			warehouse_id, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`

	now := time.Now().UTC()
	inv.UpdatedAt = now
	_, err := tx.Exec(ctx, q,
		inv.ID, inv.ProductID, inv.VariantID, inv.Quantity, 0,
		inv.LowStockAlert, inv.TrackInventory, inv.AllowBackorder,
		inv.WarehouseID, inv.UpdatedAt,
	)
	return err
}

// UpdateProduct applies patch-style updates to a product row.
func (r *pgProductRepository) UpdateProduct(ctx context.Context, p *models.Product) error {
	dimsJSON, err := json.Marshal(p.Dimensions)
	if err != nil {
		return fmt.Errorf("product_repo: marshal dimensions: %w", err)
	}
	attrsJSON, err := json.Marshal(p.Attributes)
	if err != nil {
		return fmt.Errorf("product_repo: marshal attributes: %w", err)
	}

	const q = `
		UPDATE products SET
			category_id=$1, name=$2, description=$3, short_desc=$4,
			base_price=$5, sale_price=$6, currency=$7, weight=$8,
			dimensions=$9, image_urls=$10, video_url=$11,
			tags=$12, attributes=$13, status=$14,
			requires_shipping=$15, updated_at=$16
		WHERE id=$17 AND seller_id=$18 AND deleted_at IS NULL`

	p.UpdatedAt = time.Now().UTC()
	tag, err := r.pool.Exec(ctx, q,
		p.CategoryID, p.Name, p.Description, p.ShortDesc,
		p.BasePrice, p.SalePrice, p.Currency, p.Weight,
		dimsJSON, p.ImageURLs, p.VideoURL,
		p.Tags, attrsJSON, string(p.Status),
		p.RequiresShipping, p.UpdatedAt,
		p.ID, p.SellerID,
	)
	if err != nil {
		return fmt.Errorf("product_repo: update product: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("product_repo: product not found or permission denied")
	}
	return nil
}

// DeleteProduct performs a soft-delete by setting deleted_at.
func (r *pgProductRepository) DeleteProduct(ctx context.Context, productID, sellerID uuid.UUID) error {
	const q = `
		UPDATE products SET status='deleted', deleted_at=$1, updated_at=$1
		WHERE id=$2 AND seller_id=$3 AND deleted_at IS NULL`

	now := time.Now().UTC()
	tag, err := r.pool.Exec(ctx, q, now, productID, sellerID)
	if err != nil {
		return fmt.Errorf("product_repo: delete product: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("product_repo: product not found or permission denied")
	}
	return nil
}

// GetProduct fetches a single product by ID including its variants and inventory.
func (r *pgProductRepository) GetProduct(ctx context.Context, productID uuid.UUID) (*models.Product, error) {
	const q = `
		SELECT
			id, seller_id, category_id, name, description, short_desc, slug,
			base_price, sale_price, currency, sku, barcode, weight, dimensions,
			image_urls, video_url, tags, attributes, status,
			is_digital, requires_shipping, total_sold, view_count,
			average_rating, review_count, created_at, updated_at
		FROM products
		WHERE id=$1 AND deleted_at IS NULL`

	row := r.pool.QueryRow(ctx, q, productID)
	p, err := r.scanProduct(row)
	if err != nil {
		return nil, fmt.Errorf("product_repo: get product: %w", err)
	}

	// Load variants and inventory in parallel queries
	variants, err := r.GetVariants(ctx, productID)
	if err != nil {
		return nil, err
	}
	p.Variants = variants

	inv, err := r.GetInventory(ctx, productID, nil)
	if err == nil {
		p.Inventory = inv
	}

	return p, nil
}

// GetProductBySKU fetches a product by its seller SKU.
func (r *pgProductRepository) GetProductBySKU(ctx context.Context, sku string) (*models.Product, error) {
	const q = `
		SELECT
			id, seller_id, category_id, name, description, short_desc, slug,
			base_price, sale_price, currency, sku, barcode, weight, dimensions,
			image_urls, video_url, tags, attributes, status,
			is_digital, requires_shipping, total_sold, view_count,
			average_rating, review_count, created_at, updated_at
		FROM products WHERE sku=$1 AND deleted_at IS NULL`

	row := r.pool.QueryRow(ctx, q, sku)
	p, err := r.scanProduct(row)
	if err != nil {
		return nil, fmt.Errorf("product_repo: get product by sku: %w", err)
	}
	return p, nil
}

// scanProduct reads a single product row from a pgx.Row.
func (r *pgProductRepository) scanProduct(row pgx.Row) (*models.Product, error) {
	var p models.Product
	var dimsRaw, attrsRaw []byte
	var status string

	err := row.Scan(
		&p.ID, &p.SellerID, &p.CategoryID, &p.Name, &p.Description, &p.ShortDesc, &p.Slug,
		&p.BasePrice, &p.SalePrice, &p.Currency, &p.SKU, &p.Barcode, &p.Weight, &dimsRaw,
		&p.ImageURLs, &p.VideoURL, &p.Tags, &attrsRaw, &status,
		&p.IsDigital, &p.RequiresShipping, &p.TotalSold, &p.ViewCount,
		&p.AverageRating, &p.ReviewCount, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("product not found")
		}
		return nil, err
	}

	p.Status = models.ProductStatus(status)
	if len(dimsRaw) > 0 {
		_ = json.Unmarshal(dimsRaw, &p.Dimensions)
	}
	if len(attrsRaw) > 0 {
		_ = json.Unmarshal(attrsRaw, &p.Attributes)
	}
	return &p, nil
}

// ListProducts returns a paginated, filtered list of products.
func (r *pgProductRepository) ListProducts(ctx context.Context, f models.ProductListFilters) ([]models.Product, int, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 || f.PageSize > 100 {
		f.PageSize = 20
	}

	var conditions []string
	var args []interface{}
	argIdx := 1

	conditions = append(conditions, "deleted_at IS NULL")

	if f.SellerID != nil {
		conditions = append(conditions, fmt.Sprintf("seller_id=$%d", argIdx))
		args = append(args, *f.SellerID)
		argIdx++
	}
	if f.CategoryID != nil {
		conditions = append(conditions, fmt.Sprintf("category_id=$%d", argIdx))
		args = append(args, *f.CategoryID)
		argIdx++
	}
	if f.MinPrice != nil {
		conditions = append(conditions, fmt.Sprintf("base_price>=$%d", argIdx))
		args = append(args, *f.MinPrice)
		argIdx++
	}
	if f.MaxPrice != nil {
		conditions = append(conditions, fmt.Sprintf("base_price<=$%d", argIdx))
		args = append(args, *f.MaxPrice)
		argIdx++
	}
	if f.Status != nil {
		conditions = append(conditions, fmt.Sprintf("status=$%d", argIdx))
		args = append(args, string(*f.Status))
		argIdx++
	} else {
		conditions = append(conditions, "status='active'")
	}
	if f.InStock != nil && *f.InStock {
		conditions = append(conditions, `EXISTS (
			SELECT 1 FROM inventory inv
			WHERE inv.product_id=products.id
			  AND inv.variant_id IS NULL
			  AND (inv.quantity - inv.reserved_qty) > 0
		)`)
	}

	where := "WHERE " + strings.Join(conditions, " AND ")

	orderBy := "ORDER BY created_at DESC"
	switch f.SortBy {
	case "price_asc":
		orderBy = "ORDER BY base_price ASC"
	case "price_desc":
		orderBy = "ORDER BY base_price DESC"
	case "rating":
		orderBy = "ORDER BY average_rating DESC"
	case "popular":
		orderBy = "ORDER BY total_sold DESC"
	case "newest":
		orderBy = "ORDER BY created_at DESC"
	}

	countQ := fmt.Sprintf("SELECT COUNT(*) FROM products %s", where)
	var total int
	if err := r.pool.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("product_repo: list count: %w", err)
	}

	offset := (f.Page - 1) * f.PageSize
	listQ := fmt.Sprintf(`
		SELECT
			id, seller_id, category_id, name, description, short_desc, slug,
			base_price, sale_price, currency, sku, barcode, weight, dimensions,
			image_urls, video_url, tags, attributes, status,
			is_digital, requires_shipping, total_sold, view_count,
			average_rating, review_count, created_at, updated_at
		FROM products %s %s
		LIMIT $%d OFFSET $%d`, where, orderBy, argIdx, argIdx+1)

	args = append(args, f.PageSize, offset)
	rows, err := r.pool.Query(ctx, listQ, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("product_repo: list products: %w", err)
	}
	defer rows.Close()

	var products []models.Product
	for rows.Next() {
		p, err := r.scanProduct(rows)
		if err != nil {
			return nil, 0, err
		}
		products = append(products, *p)
	}
	return products, total, rows.Err()
}

// ──────────────────────────────────────────────────────────────────────────────
// Inventory Management — uses SELECT FOR UPDATE to prevent overselling
// ──────────────────────────────────────────────────────────────────────────────

// UpdateInventory adjusts the absolute quantity of a product/variant inventory
// record. delta may be positive (restock) or negative (deduct).
func (r *pgProductRepository) UpdateInventory(ctx context.Context, productID uuid.UUID, variantID *uuid.UUID, delta int) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("inventory_repo: begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	inv, err := r.lockInventory(ctx, tx, productID, variantID)
	if err != nil {
		return err
	}

	newQty := inv.Quantity + delta
	if newQty < 0 {
		return fmt.Errorf("inventory_repo: insufficient stock (have %d, delta %d)", inv.Quantity, delta)
	}

	const q = `
		UPDATE inventory SET quantity=$1, updated_at=$2
		WHERE product_id=$3 AND (variant_id=$4 OR (variant_id IS NULL AND $4::uuid IS NULL))`

	_, err = tx.Exec(ctx, q, newQty, time.Now().UTC(), productID, variantID)
	if err != nil {
		return fmt.Errorf("inventory_repo: update quantity: %w", err)
	}
	return tx.Commit(ctx)
}

// ReserveInventory moves qty from available into reserved_qty (called on order placement).
// Uses SELECT FOR UPDATE to serialise concurrent reservations and prevent overselling.
func (r *pgProductRepository) ReserveInventory(ctx context.Context, productID uuid.UUID, variantID *uuid.UUID, qty int) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("inventory_repo: begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	inv, err := r.lockInventory(ctx, tx, productID, variantID)
	if err != nil {
		return err
	}

	available := inv.Quantity - inv.ReservedQty
	if available < qty && !inv.AllowBackorder {
		return fmt.Errorf("inventory_repo: insufficient available stock (available %d, requested %d)", available, qty)
	}

	const q = `
		UPDATE inventory SET reserved_qty=reserved_qty+$1, updated_at=$2
		WHERE product_id=$3 AND (variant_id=$4 OR (variant_id IS NULL AND $4::uuid IS NULL))`

	_, err = tx.Exec(ctx, q, qty, time.Now().UTC(), productID, variantID)
	if err != nil {
		return fmt.Errorf("inventory_repo: reserve inventory: %w", err)
	}
	return tx.Commit(ctx)
}

// ReleaseReservedInventory returns qty from reserved back to available (on cancellation/return).
func (r *pgProductRepository) ReleaseReservedInventory(ctx context.Context, productID uuid.UUID, variantID *uuid.UUID, qty int) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("inventory_repo: begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	inv, err := r.lockInventory(ctx, tx, productID, variantID)
	if err != nil {
		return err
	}

	newReserved := inv.ReservedQty - qty
	if newReserved < 0 {
		newReserved = 0
	}

	const q = `
		UPDATE inventory SET reserved_qty=$1, updated_at=$2
		WHERE product_id=$3 AND (variant_id=$4 OR (variant_id IS NULL AND $4::uuid IS NULL))`

	_, err = tx.Exec(ctx, q, newReserved, time.Now().UTC(), productID, variantID)
	if err != nil {
		return fmt.Errorf("inventory_repo: release reserved inventory: %w", err)
	}
	return tx.Commit(ctx)
}

// CommitReservedInventory decrements both quantity and reserved_qty (on delivery/fulfilment).
func (r *pgProductRepository) CommitReservedInventory(ctx context.Context, productID uuid.UUID, variantID *uuid.UUID, qty int) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("inventory_repo: begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	inv, err := r.lockInventory(ctx, tx, productID, variantID)
	if err != nil {
		return err
	}

	newQty := inv.Quantity - qty
	newReserved := inv.ReservedQty - qty
	if newQty < 0 {
		newQty = 0
	}
	if newReserved < 0 {
		newReserved = 0
	}

	const q = `
		UPDATE inventory SET quantity=$1, reserved_qty=$2, updated_at=$3
		WHERE product_id=$4 AND (variant_id=$5 OR (variant_id IS NULL AND $5::uuid IS NULL))`

	_, err = tx.Exec(ctx, q, newQty, newReserved, time.Now().UTC(), productID, variantID)
	if err != nil {
		return fmt.Errorf("inventory_repo: commit reserved inventory: %w", err)
	}
	return tx.Commit(ctx)
}

// lockInventory runs SELECT ... FOR UPDATE inside an open transaction to obtain
// a row-level lock, preventing concurrent inventory mutations.
func (r *pgProductRepository) lockInventory(ctx context.Context, tx pgx.Tx, productID uuid.UUID, variantID *uuid.UUID) (*models.Inventory, error) {
	const q = `
		SELECT id, product_id, variant_id, quantity, reserved_qty,
		       low_stock_alert, track_inventory, allow_backorder, warehouse_id, updated_at
		FROM inventory
		WHERE product_id=$1 AND (variant_id=$2 OR (variant_id IS NULL AND $2::uuid IS NULL))
		FOR UPDATE`

	row := tx.QueryRow(ctx, q, productID, variantID)
	var inv models.Inventory
	err := row.Scan(
		&inv.ID, &inv.ProductID, &inv.VariantID, &inv.Quantity, &inv.ReservedQty,
		&inv.LowStockAlert, &inv.TrackInventory, &inv.AllowBackorder, &inv.WarehouseID, &inv.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("inventory_repo: inventory record not found for product %s", productID)
		}
		return nil, fmt.Errorf("inventory_repo: lock inventory: %w", err)
	}
	inv.AvailableQty = inv.Quantity - inv.ReservedQty
	return &inv, nil
}

// GetInventory reads the inventory record without locking.
func (r *pgProductRepository) GetInventory(ctx context.Context, productID uuid.UUID, variantID *uuid.UUID) (*models.Inventory, error) {
	const q = `
		SELECT id, product_id, variant_id, quantity, reserved_qty,
		       low_stock_alert, track_inventory, allow_backorder, warehouse_id, updated_at
		FROM inventory
		WHERE product_id=$1 AND (variant_id=$2 OR (variant_id IS NULL AND $2::uuid IS NULL))`

	row := r.pool.QueryRow(ctx, q, productID, variantID)
	var inv models.Inventory
	err := row.Scan(
		&inv.ID, &inv.ProductID, &inv.VariantID, &inv.Quantity, &inv.ReservedQty,
		&inv.LowStockAlert, &inv.TrackInventory, &inv.AllowBackorder, &inv.WarehouseID, &inv.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("inventory not found")
		}
		return nil, fmt.Errorf("inventory_repo: get inventory: %w", err)
	}
	inv.AvailableQty = inv.Quantity - inv.ReservedQty
	return &inv, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Variants
// ──────────────────────────────────────────────────────────────────────────────

func (r *pgProductRepository) CreateVariant(ctx context.Context, v *models.ProductVariant) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("product_repo: begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	if err := r.insertVariantTx(ctx, tx, v); err != nil {
		return fmt.Errorf("product_repo: create variant: %w", err)
	}
	return tx.Commit(ctx)
}

func (r *pgProductRepository) UpdateVariant(ctx context.Context, v *models.ProductVariant) error {
	optsJSON, err := json.Marshal(v.Options)
	if err != nil {
		return fmt.Errorf("product_repo: marshal variant options: %w", err)
	}
	const q = `
		UPDATE product_variants SET
			name=$1, sku=$2, price=$3, sale_price=$4,
			image_url=$5, options=$6, sort_order=$7, is_active=$8, updated_at=$9
		WHERE id=$10 AND product_id=$11`

	v.UpdatedAt = time.Now().UTC()
	_, err = r.pool.Exec(ctx, q,
		v.Name, v.SKU, v.Price, v.SalePrice,
		v.ImageURL, optsJSON, v.SortOrder, v.IsActive, v.UpdatedAt,
		v.ID, v.ProductID,
	)
	return err
}

func (r *pgProductRepository) GetVariants(ctx context.Context, productID uuid.UUID) ([]models.ProductVariant, error) {
	const q = `
		SELECT id, product_id, name, sku, barcode, price, sale_price,
		       image_url, options, sort_order, is_active, created_at, updated_at
		FROM product_variants
		WHERE product_id=$1 AND is_active=true
		ORDER BY sort_order ASC`

	rows, err := r.pool.Query(ctx, q, productID)
	if err != nil {
		return nil, fmt.Errorf("product_repo: get variants: %w", err)
	}
	defer rows.Close()

	var variants []models.ProductVariant
	for rows.Next() {
		var v models.ProductVariant
		var optsRaw []byte
		err := rows.Scan(
			&v.ID, &v.ProductID, &v.Name, &v.SKU, &v.Barcode, &v.Price, &v.SalePrice,
			&v.ImageURL, &optsRaw, &v.SortOrder, &v.IsActive, &v.CreatedAt, &v.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		if len(optsRaw) > 0 {
			_ = json.Unmarshal(optsRaw, &v.Options)
		}
		variants = append(variants, v)
	}
	return variants, rows.Err()
}

// ──────────────────────────────────────────────────────────────────────────────
// Reviews
// ──────────────────────────────────────────────────────────────────────────────

// CreateReview inserts a new review and updates the product's aggregate rating.
func (r *pgProductRepository) CreateReview(ctx context.Context, rev *models.Review) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("product_repo: begin review tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	now := time.Now().UTC()
	rev.CreatedAt = now
	rev.UpdatedAt = now

	const insertQ = `
		INSERT INTO reviews (
			id, product_id, order_item_id, buyer_id, buyer_name,
			rating, title, body, image_urls, is_verified,
			helpful_count, status, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`

	_, err = tx.Exec(ctx, insertQ,
		rev.ID, rev.ProductID, rev.OrderItemID, rev.BuyerID, rev.BuyerName,
		rev.Rating, rev.Title, rev.Body, rev.ImageURLs, rev.IsVerified,
		0, string(rev.Status), rev.CreatedAt, rev.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("product_repo: insert review: %w", err)
	}

	// Atomically update the product's aggregate rating
	const aggQ = `
		UPDATE products SET
			average_rating = (
				SELECT ROUND(AVG(rating)::numeric, 2)
				FROM reviews WHERE product_id=$1 AND status='approved'
			),
			review_count = (
				SELECT COUNT(*) FROM reviews WHERE product_id=$1 AND status='approved'
			),
			updated_at=$2
		WHERE id=$1`

	_, err = tx.Exec(ctx, aggQ, rev.ProductID, now)
	if err != nil {
		return fmt.Errorf("product_repo: update aggregate rating: %w", err)
	}

	// Mark the order item as reviewed
	const markQ = `UPDATE order_items SET is_reviewed=true WHERE id=$1`
	_, _ = tx.Exec(ctx, markQ, rev.OrderItemID)

	return tx.Commit(ctx)
}

// GetReviews returns approved reviews for a product with pagination.
func (r *pgProductRepository) GetReviews(ctx context.Context, productID uuid.UUID, page, pageSize int) ([]models.Review, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 50 {
		pageSize = 20
	}

	var total int
	const countQ = `SELECT COUNT(*) FROM reviews WHERE product_id=$1 AND status='approved'`
	if err := r.pool.QueryRow(ctx, countQ, productID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("product_repo: review count: %w", err)
	}

	const q = `
		SELECT id, product_id, order_item_id, buyer_id, buyer_name,
		       rating, title, body, image_urls, is_verified,
		       helpful_count, status, seller_reply, created_at, updated_at
		FROM reviews
		WHERE product_id=$1 AND status='approved'
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	offset := (page - 1) * pageSize
	rows, err := r.pool.Query(ctx, q, productID, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("product_repo: get reviews: %w", err)
	}
	defer rows.Close()

	var reviews []models.Review
	for rows.Next() {
		var rev models.Review
		var status string
		err := rows.Scan(
			&rev.ID, &rev.ProductID, &rev.OrderItemID, &rev.BuyerID, &rev.BuyerName,
			&rev.Rating, &rev.Title, &rev.Body, &rev.ImageURLs, &rev.IsVerified,
			&rev.HelpfulCount, &status, &rev.SellerReply, &rev.CreatedAt, &rev.UpdatedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		rev.Status = models.ReviewStatus(status)
		reviews = append(reviews, rev)
	}
	return reviews, total, rows.Err()
}

// ──────────────────────────────────────────────────────────────────────────────
// Full-text search (PostgreSQL tsvector fallback; primary search via Elasticsearch)
// ──────────────────────────────────────────────────────────────────────────────

// SearchProducts performs a PostgreSQL full-text search. When Elasticsearch is
// unavailable this acts as the fallback implementation.
func (r *pgProductRepository) SearchProducts(ctx context.Context, query string, f models.ProductListFilters) ([]models.Product, int, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 || f.PageSize > 100 {
		f.PageSize = 20
	}

	tsQuery := strings.Join(strings.Fields(query), " & ")

	var conditions []string
	var args []interface{}
	argIdx := 1

	conditions = append(conditions, fmt.Sprintf("to_tsvector('english', name||' '||description) @@ to_tsquery('english', $%d)", argIdx))
	args = append(args, tsQuery)
	argIdx++

	conditions = append(conditions, "deleted_at IS NULL", "status='active'")

	if f.MinPrice != nil {
		conditions = append(conditions, fmt.Sprintf("base_price>=$%d", argIdx))
		args = append(args, *f.MinPrice)
		argIdx++
	}
	if f.MaxPrice != nil {
		conditions = append(conditions, fmt.Sprintf("base_price<=$%d", argIdx))
		args = append(args, *f.MaxPrice)
		argIdx++
	}
	if f.CategoryID != nil {
		conditions = append(conditions, fmt.Sprintf("category_id=$%d", argIdx))
		args = append(args, *f.CategoryID)
		argIdx++
	}

	where := "WHERE " + strings.Join(conditions, " AND ")

	var total int
	countQ := fmt.Sprintf("SELECT COUNT(*) FROM products %s", where)
	if err := r.pool.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("product_repo: search count: %w", err)
	}

	offset := (f.Page - 1) * f.PageSize
	listQ := fmt.Sprintf(`
		SELECT
			id, seller_id, category_id, name, description, short_desc, slug,
			base_price, sale_price, currency, sku, barcode, weight, dimensions,
			image_urls, video_url, tags, attributes, status,
			is_digital, requires_shipping, total_sold, view_count,
			average_rating, review_count, created_at, updated_at
		FROM products %s
		ORDER BY ts_rank(to_tsvector('english', name||' '||description), to_tsquery('english', $1)) DESC
		LIMIT $%d OFFSET $%d`, where, argIdx, argIdx+1)

	args = append(args, f.PageSize, offset)
	rows, err := r.pool.Query(ctx, listQ, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("product_repo: search products: %w", err)
	}
	defer rows.Close()

	var products []models.Product
	for rows.Next() {
		p, err := r.scanProduct(rows)
		if err != nil {
			return nil, 0, err
		}
		products = append(products, *p)
	}
	return products, total, rows.Err()
}
