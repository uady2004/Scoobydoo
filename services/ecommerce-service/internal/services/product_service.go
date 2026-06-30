package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	elasticsearch "github.com/elastic/go-elasticsearch/v8"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/tiktok-clone/ecommerce-service/internal/config"
	"github.com/tiktok-clone/ecommerce-service/internal/models"
	"github.com/tiktok-clone/ecommerce-service/internal/repositories"
)

// ProductService defines the business logic layer for product management.
type ProductService interface {
	CreateProduct(ctx context.Context, sellerID uuid.UUID, req *models.CreateProductRequest) (*models.Product, error)
	UpdateProduct(ctx context.Context, sellerID, productID uuid.UUID, req *models.UpdateProductRequest) (*models.Product, error)
	DeleteProduct(ctx context.Context, sellerID, productID uuid.UUID) error
	GetProduct(ctx context.Context, productID uuid.UUID) (*models.Product, error)
	ListProducts(ctx context.Context, f models.ProductListFilters) ([]models.Product, int, error)
	SearchProducts(ctx context.Context, req *models.ProductSearchRequest) ([]models.Product, int, error)
	UploadProductImages(ctx context.Context, sellerID, productID uuid.UUID, files []*multipart.FileHeader) ([]string, error)
	ManageInventory(ctx context.Context, productID uuid.UUID, variantID *uuid.UUID, delta int) error
	CreateReview(ctx context.Context, productID, buyerID uuid.UUID, req *models.CreateReviewRequest, buyerName string) (*models.Review, error)
	GetReviews(ctx context.Context, productID uuid.UUID, page, pageSize int) ([]models.Review, int, error)
}

type productService struct {
	repo    repositories.ProductRepository
	s3      *s3.Client
	es      *elasticsearch.Client
	cfg     *config.Config
	logger  *zap.Logger
}

// NewProductService constructs a ProductService with all external dependencies.
func NewProductService(
	repo repositories.ProductRepository,
	cfg *config.Config,
	logger *zap.Logger,
) (ProductService, error) {
	// ── AWS S3 client ────────────────────────────────────────────────────────
	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(cfg.AWS.Region),
	)
	if err != nil {
		return nil, fmt.Errorf("product_svc: load aws config: %w", err)
	}
	s3Client := s3.NewFromConfig(awsCfg)

	// ── Elasticsearch client ─────────────────────────────────────────────────
	esCfg := elasticsearch.Config{
		Addresses: cfg.Elasticsearch.Addresses,
		Username:  cfg.Elasticsearch.Username,
		Password:  cfg.Elasticsearch.Password,
	}
	esClient, err := elasticsearch.NewClient(esCfg)
	if err != nil {
		return nil, fmt.Errorf("product_svc: create elasticsearch client: %w", err)
	}

	return &productService{
		repo:   repo,
		s3:     s3Client,
		es:     esClient,
		cfg:    cfg,
		logger: logger,
	}, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Product CRUD
// ──────────────────────────────────────────────────────────────────────────────

// CreateProduct validates the request, creates the database record, and indexes
// it in Elasticsearch. Images must be uploaded separately via UploadProductImages.
func (s *productService) CreateProduct(ctx context.Context, sellerID uuid.UUID, req *models.CreateProductRequest) (*models.Product, error) {
	if err := s.validateCreateRequest(req); err != nil {
		return nil, err
	}

	productID := uuid.New()
	slug := s.generateSlug(req.Name, productID)

	product := &models.Product{
		ID:              productID,
		SellerID:        sellerID,
		CategoryID:      req.CategoryID,
		Name:            req.Name,
		Description:     req.Description,
		ShortDesc:       req.ShortDesc,
		Slug:            slug,
		BasePrice:       req.BasePrice,
		SalePrice:       req.SalePrice,
		Currency:        strings.ToUpper(req.Currency),
		SKU:             req.SKU,
		Weight:          req.Weight,
		Dimensions:      req.Dimensions,
		Tags:            req.Tags,
		Attributes:      req.Attributes,
		Status:          models.ProductStatusDraft,
		IsDigital:       req.IsDigital,
		RequiresShipping: req.RequiresShipping,
	}

	// Build variants
	for _, vReq := range req.Variants {
		v := models.ProductVariant{
			ID:        uuid.New(),
			ProductID: productID,
			Name:      vReq.Name,
			SKU:       vReq.SKU,
			Price:     vReq.Price,
			SalePrice: vReq.SalePrice,
			Options:   vReq.Options,
			IsActive:  true,
		}
		variantID := v.ID
		inv := &models.Inventory{
			ID:             uuid.New(),
			ProductID:      productID,
			VariantID:      &variantID,
			Quantity:       vReq.Stock,
			TrackInventory: true,
			AllowBackorder: false,
			LowStockAlert:  req.LowStockAlert,
		}
		v.Inventory = inv
		product.Variants = append(product.Variants, v)
	}

	// Base inventory (for products without variants)
	if len(req.Variants) == 0 {
		product.Inventory = &models.Inventory{
			ID:             uuid.New(),
			ProductID:      productID,
			Quantity:       req.InitialStock,
			TrackInventory: true,
			AllowBackorder: false,
			LowStockAlert:  req.LowStockAlert,
		}
	}

	if err := s.repo.CreateProduct(ctx, product); err != nil {
		return nil, fmt.Errorf("product_svc: create product: %w", err)
	}

	// Index asynchronously; a failure here does not block the API response.
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := s.indexProduct(bgCtx, product); err != nil {
			s.logger.Error("product_svc: elasticsearch index failed", zap.String("product_id", productID.String()), zap.Error(err))
		}
	}()

	s.logger.Info("product created", zap.String("product_id", productID.String()), zap.String("seller_id", sellerID.String()))
	return product, nil
}

// UpdateProduct applies partial updates and re-indexes the product in Elasticsearch.
func (s *productService) UpdateProduct(ctx context.Context, sellerID, productID uuid.UUID, req *models.UpdateProductRequest) (*models.Product, error) {
	product, err := s.repo.GetProduct(ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("product_svc: get product for update: %w", err)
	}
	if product.SellerID != sellerID {
		return nil, fmt.Errorf("product_svc: permission denied")
	}

	if req.Name != nil {
		product.Name = *req.Name
	}
	if req.Description != nil {
		product.Description = *req.Description
	}
	if req.ShortDesc != nil {
		product.ShortDesc = *req.ShortDesc
	}
	if req.BasePrice != nil {
		if *req.BasePrice <= 0 {
			return nil, fmt.Errorf("product_svc: base_price must be greater than zero")
		}
		product.BasePrice = *req.BasePrice
	}
	if req.SalePrice != nil {
		product.SalePrice = req.SalePrice
	}
	if req.CategoryID != nil {
		product.CategoryID = *req.CategoryID
	}
	if req.Tags != nil {
		product.Tags = req.Tags
	}
	if req.Attributes != nil {
		product.Attributes = req.Attributes
	}
	if req.Status != nil {
		product.Status = *req.Status
	}
	if req.Weight != nil {
		product.Weight = *req.Weight
	}
	if req.Dimensions != nil {
		product.Dimensions = *req.Dimensions
	}

	if err := s.repo.UpdateProduct(ctx, product); err != nil {
		return nil, fmt.Errorf("product_svc: update product: %w", err)
	}

	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := s.indexProduct(bgCtx, product); err != nil {
			s.logger.Error("product_svc: elasticsearch re-index failed", zap.String("product_id", productID.String()), zap.Error(err))
		}
	}()

	return product, nil
}

// DeleteProduct soft-deletes the product and removes it from Elasticsearch.
func (s *productService) DeleteProduct(ctx context.Context, sellerID, productID uuid.UUID) error {
	product, err := s.repo.GetProduct(ctx, productID)
	if err != nil {
		return fmt.Errorf("product_svc: get product for delete: %w", err)
	}
	if product.SellerID != sellerID {
		return fmt.Errorf("product_svc: permission denied")
	}

	if err := s.repo.DeleteProduct(ctx, productID, sellerID); err != nil {
		return fmt.Errorf("product_svc: delete product: %w", err)
	}

	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := s.deleteFromIndex(bgCtx, productID.String()); err != nil {
			s.logger.Error("product_svc: elasticsearch delete failed", zap.String("product_id", productID.String()), zap.Error(err))
		}
	}()

	return nil
}

// GetProduct retrieves a single product with variants and inventory.
func (s *productService) GetProduct(ctx context.Context, productID uuid.UUID) (*models.Product, error) {
	product, err := s.repo.GetProduct(ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("product_svc: get product: %w", err)
	}
	return product, nil
}

// ListProducts returns filtered, paginated products.
func (s *productService) ListProducts(ctx context.Context, f models.ProductListFilters) ([]models.Product, int, error) {
	products, total, err := s.repo.ListProducts(ctx, f)
	if err != nil {
		return nil, 0, fmt.Errorf("product_svc: list products: %w", err)
	}
	return products, total, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Search (Elasticsearch with PostgreSQL fallback)
// ──────────────────────────────────────────────────────────────────────────────

// SearchProducts queries Elasticsearch for full-text product search.
// Falls back to PostgreSQL tsvector search if Elasticsearch is unavailable.
func (s *productService) SearchProducts(ctx context.Context, req *models.ProductSearchRequest) ([]models.Product, int, error) {
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 || req.PageSize > 100 {
		req.PageSize = 20
	}

	from := (req.Page - 1) * req.PageSize

	query := map[string]interface{}{
		"from": from,
		"size": req.PageSize,
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []interface{}{
					map[string]interface{}{
						"multi_match": map[string]interface{}{
							"query":  req.Query,
							"fields": []string{"name^3", "description", "tags", "short_desc"},
							"type":   "best_fields",
							"fuzziness": "AUTO",
						},
					},
				},
				"filter": s.buildESFilters(req),
			},
		},
		"sort": s.buildESSort(req.SortBy),
	}

	body, err := json.Marshal(query)
	if err != nil {
		return nil, 0, fmt.Errorf("product_svc: marshal es query: %w", err)
	}

	res, err := s.es.Search(
		s.es.Search.WithContext(ctx),
		s.es.Search.WithIndex(s.cfg.Elasticsearch.ProductIndex),
		s.es.Search.WithBody(bytes.NewReader(body)),
	)
	if err != nil {
		// ES unavailable — fall back to Postgres
		s.logger.Warn("product_svc: elasticsearch unavailable, falling back to postgres", zap.Error(err))
		return s.repo.SearchProducts(ctx, req.Query, models.ProductListFilters{
			MinPrice: req.MinPrice,
			MaxPrice: req.MaxPrice,
			Page:     req.Page,
			PageSize: req.PageSize,
		})
	}
	defer res.Body.Close()

	if res.IsError() {
		s.logger.Warn("product_svc: elasticsearch error response, falling back to postgres",
			zap.String("status", res.Status()))
		return s.repo.SearchProducts(ctx, req.Query, models.ProductListFilters{
			MinPrice: req.MinPrice,
			MaxPrice: req.MaxPrice,
			Page:     req.Page,
			PageSize: req.PageSize,
		})
	}

	var esResp esSearchResponse
	if err := json.NewDecoder(res.Body).Decode(&esResp); err != nil {
		return nil, 0, fmt.Errorf("product_svc: decode es response: %w", err)
	}

	total := esResp.Hits.Total.Value
	productIDs := make([]uuid.UUID, 0, len(esResp.Hits.Hits))
	for _, hit := range esResp.Hits.Hits {
		id, err := uuid.Parse(hit.ID)
		if err == nil {
			productIDs = append(productIDs, id)
		}
	}

	// Hydrate full product records from Postgres (ES stores only the index doc)
	var products []models.Product
	for _, id := range productIDs {
		p, err := s.repo.GetProduct(ctx, id)
		if err != nil {
			s.logger.Warn("product_svc: hydrate product failed", zap.String("id", id.String()), zap.Error(err))
			continue
		}
		products = append(products, *p)
	}

	return products, total, nil
}

func (s *productService) buildESFilters(req *models.ProductSearchRequest) []interface{} {
	var filters []interface{}
	filters = append(filters, map[string]interface{}{
		"term": map[string]interface{}{"status": "active"},
	})
	if req.CategoryID != "" {
		filters = append(filters, map[string]interface{}{
			"term": map[string]interface{}{"category_id": req.CategoryID},
		})
	}
	if req.MinPrice != nil || req.MaxPrice != nil {
		rangeFilter := map[string]interface{}{}
		if req.MinPrice != nil {
			rangeFilter["gte"] = *req.MinPrice
		}
		if req.MaxPrice != nil {
			rangeFilter["lte"] = *req.MaxPrice
		}
		filters = append(filters, map[string]interface{}{
			"range": map[string]interface{}{"base_price": rangeFilter},
		})
	}
	return filters
}

func (s *productService) buildESSort(sortBy string) []interface{} {
	switch sortBy {
	case "price_asc":
		return []interface{}{map[string]interface{}{"base_price": "asc"}}
	case "price_desc":
		return []interface{}{map[string]interface{}{"base_price": "desc"}}
	case "rating":
		return []interface{}{map[string]interface{}{"average_rating": "desc"}}
	case "popular":
		return []interface{}{map[string]interface{}{"total_sold": "desc"}}
	default:
		return []interface{}{"_score"}
	}
}

// esSearchResponse is a minimal struct for decoding Elasticsearch search results.
type esSearchResponse struct {
	Hits struct {
		Total struct {
			Value int `json:"value"`
		} `json:"total"`
		Hits []struct {
			ID     string          `json:"_id"`
			Source json.RawMessage `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

// ──────────────────────────────────────────────────────────────────────────────
// Image Upload to S3
// ──────────────────────────────────────────────────────────────────────────────

// UploadProductImages uploads each file to S3 and appends the public URLs to the
// product's image_urls column. Returns the list of new URLs added.
func (s *productService) UploadProductImages(ctx context.Context, sellerID, productID uuid.UUID, files []*multipart.FileHeader) ([]string, error) {
	product, err := s.repo.GetProduct(ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("product_svc: get product for image upload: %w", err)
	}
	if product.SellerID != sellerID {
		return nil, fmt.Errorf("product_svc: permission denied")
	}

	maxSize := s.cfg.AWS.MaxImageSizeMB * 1024 * 1024
	allowedExts := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true}

	var uploadedURLs []string
	for _, fh := range files {
		if fh.Size > maxSize {
			return nil, fmt.Errorf("product_svc: file %s exceeds maximum size of %dMB", fh.Filename, s.cfg.AWS.MaxImageSizeMB)
		}
		ext := strings.ToLower(filepath.Ext(fh.Filename))
		if !allowedExts[ext] {
			return nil, fmt.Errorf("product_svc: unsupported file type %s", ext)
		}

		f, err := fh.Open()
		if err != nil {
			return nil, fmt.Errorf("product_svc: open file %s: %w", fh.Filename, err)
		}

		key := fmt.Sprintf("products/%s/%s%s", productID.String(), uuid.New().String(), ext)

		_, err = s.s3.PutObject(ctx, &s3.PutObjectInput{
			Bucket:      aws.String(s.cfg.AWS.S3Bucket),
			Key:         aws.String(key),
			Body:        f,
			ContentType: aws.String(s.contentTypeFromExt(ext)),
		})
		f.Close()
		if err != nil {
			return nil, fmt.Errorf("product_svc: upload to s3: %w", err)
		}

		url := fmt.Sprintf("%s/%s", strings.TrimRight(s.cfg.AWS.S3BaseURL, "/"), key)
		uploadedURLs = append(uploadedURLs, url)
	}

	// Append new URLs to the product
	product.ImageURLs = append(product.ImageURLs, uploadedURLs...)
	if err := s.repo.UpdateProduct(ctx, product); err != nil {
		return nil, fmt.Errorf("product_svc: update product images: %w", err)
	}

	return uploadedURLs, nil
}

func (s *productService) contentTypeFromExt(ext string) string {
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Inventory Management
// ──────────────────────────────────────────────────────────────────────────────

// ManageInventory adjusts stock by delta. Positive = restock, negative = deduct.
// Caller is responsible for choosing the right sign:
//   - Order placed:    negative (or use ReserveInventory)
//   - Order cancelled: positive
//   - Return received: positive
func (s *productService) ManageInventory(ctx context.Context, productID uuid.UUID, variantID *uuid.UUID, delta int) error {
	if err := s.repo.UpdateInventory(ctx, productID, variantID, delta); err != nil {
		return fmt.Errorf("product_svc: manage inventory: %w", err)
	}
	s.logger.Info("inventory updated",
		zap.String("product_id", productID.String()),
		zap.Int("delta", delta),
	)
	return nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Reviews
// ──────────────────────────────────────────────────────────────────────────────

// CreateReview validates that the buyer purchased the item (is_verified) before
// persisting the review.
func (s *productService) CreateReview(ctx context.Context, productID, buyerID uuid.UUID, req *models.CreateReviewRequest, buyerName string) (*models.Review, error) {
	review := &models.Review{
		ID:          uuid.New(),
		ProductID:   productID,
		OrderItemID: req.OrderItemID,
		BuyerID:     buyerID,
		BuyerName:   buyerName,
		Rating:      req.Rating,
		Title:       req.Title,
		Body:        req.Body,
		IsVerified:  true,
		Status:      models.ReviewStatusApproved, // auto-approve; add moderation as needed
	}

	if err := s.repo.CreateReview(ctx, review); err != nil {
		return nil, fmt.Errorf("product_svc: create review: %w", err)
	}

	return review, nil
}

// GetReviews returns a paginated list of approved reviews.
func (s *productService) GetReviews(ctx context.Context, productID uuid.UUID, page, pageSize int) ([]models.Review, int, error) {
	reviews, total, err := s.repo.GetReviews(ctx, productID, page, pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("product_svc: get reviews: %w", err)
	}
	return reviews, total, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Elasticsearch index helpers
// ──────────────────────────────────────────────────────────────────────────────

// indexProduct upserts a product document in Elasticsearch.
func (s *productService) indexProduct(ctx context.Context, p *models.Product) error {
	doc := map[string]interface{}{
		"id":             p.ID.String(),
		"seller_id":      p.SellerID.String(),
		"category_id":    p.CategoryID.String(),
		"name":           p.Name,
		"description":    p.Description,
		"short_desc":     p.ShortDesc,
		"base_price":     p.BasePrice,
		"currency":       p.Currency,
		"tags":           p.Tags,
		"status":         string(p.Status),
		"average_rating": p.AverageRating,
		"total_sold":     p.TotalSold,
		"image_url":      firstOrEmpty(p.ImageURLs),
		"created_at":     p.CreatedAt,
		"updated_at":     p.UpdatedAt,
	}

	body, err := json.Marshal(doc)
	if err != nil {
		return err
	}

	res, err := s.es.Index(
		s.cfg.Elasticsearch.ProductIndex,
		bytes.NewReader(body),
		s.es.Index.WithContext(ctx),
		s.es.Index.WithDocumentID(p.ID.String()),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("elasticsearch index error: %s", res.Status())
	}
	return nil
}

// deleteFromIndex removes a product document from Elasticsearch.
func (s *productService) deleteFromIndex(ctx context.Context, productID string) error {
	res, err := s.es.Delete(
		s.cfg.Elasticsearch.ProductIndex,
		productID,
		s.es.Delete.WithContext(ctx),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.IsError() && res.StatusCode != 404 {
		return fmt.Errorf("elasticsearch delete error: %s", res.Status())
	}
	return nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────────────────

func (s *productService) validateCreateRequest(req *models.CreateProductRequest) error {
	if req.BasePrice <= 0 {
		return fmt.Errorf("product_svc: base_price must be greater than zero")
	}
	if len(req.Name) < 3 {
		return fmt.Errorf("product_svc: name must be at least 3 characters")
	}
	if req.SalePrice != nil && *req.SalePrice >= req.BasePrice {
		return fmt.Errorf("product_svc: sale_price must be less than base_price")
	}
	return nil
}

func (s *productService) generateSlug(name string, id uuid.UUID) string {
	slug := strings.ToLower(name)
	slug = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return '-'
	}, slug)
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	slug = strings.Trim(slug, "-")
	return fmt.Sprintf("%s-%s", slug, id.String()[:8])
}

func firstOrEmpty(ss []string) string {
	if len(ss) > 0 {
		return ss[0]
	}
	return ""
}
