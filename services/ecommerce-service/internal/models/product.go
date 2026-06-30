package models

import (
	"time"

	"github.com/google/uuid"
)

// ProductStatus represents the lifecycle state of a product listing.
type ProductStatus string

const (
	ProductStatusDraft     ProductStatus = "draft"
	ProductStatusActive    ProductStatus = "active"
	ProductStatusInactive  ProductStatus = "inactive"
	ProductStatusDeleted   ProductStatus = "deleted"
	ProductStatusSoldOut   ProductStatus = "sold_out"
)

// Category represents a hierarchical product category.
type Category struct {
	ID          uuid.UUID  `json:"id" db:"id"`
	ParentID    *uuid.UUID `json:"parent_id,omitempty" db:"parent_id"`
	Name        string     `json:"name" db:"name"`
	Slug        string     `json:"slug" db:"slug"`
	Description string     `json:"description" db:"description"`
	ImageURL    string     `json:"image_url" db:"image_url"`
	SortOrder   int        `json:"sort_order" db:"sort_order"`
	IsActive    bool       `json:"is_active" db:"is_active"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at" db:"updated_at"`
}

// Product is the core listing entity owned by a seller.
type Product struct {
	ID              uuid.UUID     `json:"id" db:"id"`
	SellerID        uuid.UUID     `json:"seller_id" db:"seller_id"`
	CategoryID      uuid.UUID     `json:"category_id" db:"category_id"`
	Name            string        `json:"name" db:"name"`
	Description     string        `json:"description" db:"description"`
	ShortDesc       string        `json:"short_desc" db:"short_desc"`
	Slug            string        `json:"slug" db:"slug"`
	BasePrice       float64       `json:"base_price" db:"base_price"`
	SalePrice       *float64      `json:"sale_price,omitempty" db:"sale_price"`
	Currency        string        `json:"currency" db:"currency"`
	SKU             string        `json:"sku" db:"sku"`
	Barcode         string        `json:"barcode,omitempty" db:"barcode"`
	Weight          float64       `json:"weight" db:"weight"`         // kg
	Dimensions      Dimensions    `json:"dimensions" db:"dimensions"` // stored as JSONB
	ImageURLs       []string      `json:"image_urls" db:"image_urls"` // stored as TEXT[]
	VideoURL        string        `json:"video_url,omitempty" db:"video_url"`
	Tags            []string      `json:"tags" db:"tags"`             // stored as TEXT[]
	Attributes      ProductAttrs  `json:"attributes" db:"attributes"` // stored as JSONB
	Status          ProductStatus `json:"status" db:"status"`
	IsDigital       bool          `json:"is_digital" db:"is_digital"`
	RequiresShipping bool         `json:"requires_shipping" db:"requires_shipping"`
	TotalSold       int64         `json:"total_sold" db:"total_sold"`
	ViewCount       int64         `json:"view_count" db:"view_count"`
	AverageRating   float64       `json:"average_rating" db:"average_rating"`
	ReviewCount     int           `json:"review_count" db:"review_count"`
	CreatedAt       time.Time     `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at" db:"updated_at"`
	DeletedAt       *time.Time    `json:"deleted_at,omitempty" db:"deleted_at"`

	// Populated joins (not stored in products table)
	Variants  []ProductVariant `json:"variants,omitempty"`
	Inventory *Inventory       `json:"inventory,omitempty"`
	Category  *Category        `json:"category,omitempty"`
}

// Dimensions stores physical package dimensions.
type Dimensions struct {
	Length float64 `json:"length"` // cm
	Width  float64 `json:"width"`  // cm
	Height float64 `json:"height"` // cm
}

// ProductAttrs holds arbitrary key-value attributes like "color", "material", etc.
type ProductAttrs map[string]interface{}

// ProductVariant represents a specific SKU variant of a product (e.g., size S / color Red).
type ProductVariant struct {
	ID         uuid.UUID    `json:"id" db:"id"`
	ProductID  uuid.UUID    `json:"product_id" db:"product_id"`
	Name       string       `json:"name" db:"name"` // e.g. "S / Red"
	SKU        string       `json:"sku" db:"sku"`
	Barcode    string       `json:"barcode,omitempty" db:"barcode"`
	Price      float64      `json:"price" db:"price"`
	SalePrice  *float64     `json:"sale_price,omitempty" db:"sale_price"`
	ImageURL   string       `json:"image_url,omitempty" db:"image_url"`
	Options    VariantOpts  `json:"options" db:"options"` // e.g. {"size":"S","color":"Red"}
	SortOrder  int          `json:"sort_order" db:"sort_order"`
	IsActive   bool         `json:"is_active" db:"is_active"`
	CreatedAt  time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time    `json:"updated_at" db:"updated_at"`

	// Populated join
	Inventory *Inventory `json:"inventory,omitempty"`
}

// VariantOpts maps option name to selected value.
type VariantOpts map[string]string

// Inventory tracks stock levels for a product or variant.
// Variants take priority; if VariantID is nil the record belongs to the base product.
type Inventory struct {
	ID              uuid.UUID  `json:"id" db:"id"`
	ProductID       uuid.UUID  `json:"product_id" db:"product_id"`
	VariantID       *uuid.UUID `json:"variant_id,omitempty" db:"variant_id"`
	Quantity        int        `json:"quantity" db:"quantity"`
	ReservedQty     int        `json:"reserved_qty" db:"reserved_qty"` // held by pending orders
	AvailableQty    int        `json:"available_qty" db:"available_qty"` // computed: quantity - reserved_qty
	LowStockAlert   int        `json:"low_stock_alert" db:"low_stock_alert"`
	TrackInventory  bool       `json:"track_inventory" db:"track_inventory"`
	AllowBackorder  bool       `json:"allow_backorder" db:"allow_backorder"`
	WarehouseID     string     `json:"warehouse_id,omitempty" db:"warehouse_id"`
	UpdatedAt       time.Time  `json:"updated_at" db:"updated_at"`
}

// ReviewStatus tracks whether a review has been moderated.
type ReviewStatus string

const (
	ReviewStatusPending  ReviewStatus = "pending"
	ReviewStatusApproved ReviewStatus = "approved"
	ReviewStatusRejected ReviewStatus = "rejected"
)

// Review is a buyer rating and text review for a product.
type Review struct {
	ID          uuid.UUID    `json:"id" db:"id"`
	ProductID   uuid.UUID    `json:"product_id" db:"product_id"`
	OrderItemID uuid.UUID    `json:"order_item_id" db:"order_item_id"`
	BuyerID     uuid.UUID    `json:"buyer_id" db:"buyer_id"`
	BuyerName   string       `json:"buyer_name" db:"buyer_name"`
	Rating      int          `json:"rating" db:"rating"` // 1-5
	Title       string       `json:"title,omitempty" db:"title"`
	Body        string       `json:"body" db:"body"`
	ImageURLs   []string     `json:"image_urls,omitempty" db:"image_urls"`
	IsVerified  bool         `json:"is_verified" db:"is_verified"` // buyer actually purchased
	HelpfulCount int         `json:"helpful_count" db:"helpful_count"`
	Status      ReviewStatus `json:"status" db:"status"`
	SellerReply string       `json:"seller_reply,omitempty" db:"seller_reply"`
	CreatedAt   time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at" db:"updated_at"`
}

// ──────────────────────────────────────────────────────────────────────────────
// Request / Response DTOs
// ──────────────────────────────────────────────────────────────────────────────

// CreateProductRequest is the payload sent by a seller to list a new product.
type CreateProductRequest struct {
	CategoryID       uuid.UUID    `json:"category_id" binding:"required"`
	Name             string       `json:"name" binding:"required,min=3,max=200"`
	Description      string       `json:"description" binding:"required,min=10"`
	ShortDesc        string       `json:"short_desc" binding:"max=500"`
	BasePrice        float64      `json:"base_price" binding:"required,gt=0"`
	SalePrice        *float64     `json:"sale_price,omitempty"`
	Currency         string       `json:"currency" binding:"required,len=3"`
	SKU              string       `json:"sku" binding:"required"`
	Weight           float64      `json:"weight"`
	Dimensions       Dimensions   `json:"dimensions"`
	Tags             []string     `json:"tags"`
	Attributes       ProductAttrs `json:"attributes"`
	IsDigital        bool         `json:"is_digital"`
	RequiresShipping bool         `json:"requires_shipping"`
	InitialStock     int          `json:"initial_stock" binding:"min=0"`
	LowStockAlert    int          `json:"low_stock_alert" binding:"min=0"`
	Variants         []CreateVariantRequest `json:"variants"`
}

// CreateVariantRequest is embedded inside CreateProductRequest for each variant.
type CreateVariantRequest struct {
	Name      string      `json:"name" binding:"required"`
	SKU       string      `json:"sku" binding:"required"`
	Price     float64     `json:"price" binding:"required,gt=0"`
	SalePrice *float64    `json:"sale_price,omitempty"`
	Options   VariantOpts `json:"options"`
	Stock     int         `json:"stock" binding:"min=0"`
}

// UpdateProductRequest allows partial updates to a product listing.
type UpdateProductRequest struct {
	Name             *string      `json:"name,omitempty"`
	Description      *string      `json:"description,omitempty"`
	ShortDesc        *string      `json:"short_desc,omitempty"`
	BasePrice        *float64     `json:"base_price,omitempty"`
	SalePrice        *float64     `json:"sale_price,omitempty"`
	CategoryID       *uuid.UUID   `json:"category_id,omitempty"`
	Tags             []string     `json:"tags,omitempty"`
	Attributes       ProductAttrs `json:"attributes,omitempty"`
	Status           *ProductStatus `json:"status,omitempty"`
	Weight           *float64     `json:"weight,omitempty"`
	Dimensions       *Dimensions  `json:"dimensions,omitempty"`
}

// ProductListFilters carries query-string filter parameters for listing products.
type ProductListFilters struct {
	CategoryID *uuid.UUID    `form:"category_id"`
	SellerID   *uuid.UUID    `form:"seller_id"`
	MinPrice   *float64      `form:"min_price"`
	MaxPrice   *float64      `form:"max_price"`
	Tags       []string      `form:"tags"`
	Status     *ProductStatus `form:"status"`
	InStock    *bool         `form:"in_stock"`
	SortBy     string        `form:"sort_by"`   // "price_asc","price_desc","rating","newest","popular"
	Page       int           `form:"page"`
	PageSize   int           `form:"page_size"`
}

// CreateReviewRequest is submitted by a verified buyer.
type CreateReviewRequest struct {
	OrderItemID uuid.UUID `json:"order_item_id" binding:"required"`
	Rating      int       `json:"rating" binding:"required,min=1,max=5"`
	Title       string    `json:"title" binding:"max=100"`
	Body        string    `json:"body" binding:"required,min=10,max=2000"`
}

// ProductSearchRequest carries the query parameters for full-text search.
type ProductSearchRequest struct {
	Query      string   `form:"q" binding:"required,min=1"`
	CategoryID string   `form:"category_id"`
	MinPrice   *float64 `form:"min_price"`
	MaxPrice   *float64 `form:"max_price"`
	SortBy     string   `form:"sort_by"`
	Page       int      `form:"page"`
	PageSize   int      `form:"page_size"`
}
