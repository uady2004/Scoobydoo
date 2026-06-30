package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/tiktok-clone/ecommerce-service/internal/models"
	"github.com/tiktok-clone/ecommerce-service/internal/services"
)

// ProductHandler exposes seller product management and public product browsing endpoints.
type ProductHandler struct {
	svc    services.ProductService
	logger *zap.Logger
}

// NewProductHandler creates a new ProductHandler.
func NewProductHandler(svc services.ProductService, logger *zap.Logger) *ProductHandler {
	return &ProductHandler{svc: svc, logger: logger}
}

// RegisterRoutes registers all product routes on the given router group.
// Authenticated routes (requiring a valid JWT) are on the "auth" group;
// public routes use "public".
func (h *ProductHandler) RegisterRoutes(public, auth gin.IRouter) {
	// Public product browsing
	public.GET("/products", h.ListProducts)
	public.GET("/products/search", h.SearchProducts)
	public.GET("/products/:id", h.GetProduct)
	public.GET("/products/:id/reviews", h.GetReviews)

	// Authenticated: buyer posts a review
	auth.POST("/products/:id/reviews", h.CreateReview)

	// Authenticated: seller product management
	seller := auth.Group("/seller/products")
	seller.POST("", h.CreateProduct)
	seller.PUT("/:id", h.UpdateProduct)
	seller.DELETE("/:id", h.DeleteProduct)
	seller.POST("/:id/images", h.UploadImages)
	seller.GET("", h.ListSellerProducts)
}

// ──────────────────────────────────────────────────────────────────────────────
// Public endpoints
// ──────────────────────────────────────────────────────────────────────────────

// ListProducts godoc
//
//	@Summary     List products
//	@Description Returns a paginated, filtered list of active products.
//	@Tags        products
//	@Produce     json
//	@Param       category_id query string  false "Filter by category UUID"
//	@Param       min_price   query number  false "Minimum price"
//	@Param       max_price   query number  false "Maximum price"
//	@Param       sort_by     query string  false "price_asc|price_desc|rating|popular|newest"
//	@Param       page        query int     false "Page number (default 1)"
//	@Param       page_size   query int     false "Page size (default 20, max 100)"
//	@Success     200 {object} map[string]interface{}
//	@Router      /products [get]
func (h *ProductHandler) ListProducts(c *gin.Context) {
	var f models.ProductListFilters
	if err := c.ShouldBindQuery(&f); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 {
		f.PageSize = 20
	}

	products, total, err := h.svc.ListProducts(c.Request.Context(), f)
	if err != nil {
		h.logger.Error("list products failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list products"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":      products,
		"total":     total,
		"page":      f.Page,
		"page_size": f.PageSize,
	})
}

// GetProduct godoc
//
//	@Summary     Get product
//	@Description Returns a single product with variants and inventory.
//	@Tags        products
//	@Produce     json
//	@Param       id path string true "Product UUID"
//	@Success     200 {object} models.Product
//	@Router      /products/{id} [get]
func (h *ProductHandler) GetProduct(c *gin.Context) {
	productID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product id"})
		return
	}

	product, err := h.svc.GetProduct(c.Request.Context(), productID)
	if err != nil {
		h.logger.Error("get product failed", zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
		return
	}

	c.JSON(http.StatusOK, product)
}

// SearchProducts godoc
//
//	@Summary     Search products
//	@Description Full-text search via Elasticsearch with PostgreSQL fallback.
//	@Tags        products
//	@Produce     json
//	@Param       q           query string  true  "Search query"
//	@Param       category_id query string  false "Filter by category UUID"
//	@Param       min_price   query number  false "Minimum price"
//	@Param       max_price   query number  false "Maximum price"
//	@Param       sort_by     query string  false "price_asc|price_desc|rating|popular"
//	@Param       page        query int     false "Page number"
//	@Param       page_size   query int     false "Page size"
//	@Success     200 {object} map[string]interface{}
//	@Router      /products/search [get]
func (h *ProductHandler) SearchProducts(c *gin.Context) {
	var req models.ProductSearchRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 {
		req.PageSize = 20
	}

	products, total, err := h.svc.SearchProducts(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error("search products failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "search failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":      products,
		"total":     total,
		"page":      req.Page,
		"page_size": req.PageSize,
		"query":     req.Query,
	})
}

// GetReviews godoc
//
//	@Summary     Get product reviews
//	@Tags        products
//	@Produce     json
//	@Param       id        path  string true  "Product UUID"
//	@Param       page      query int    false "Page number"
//	@Param       page_size query int    false "Page size"
//	@Success     200 {object} map[string]interface{}
//	@Router      /products/{id}/reviews [get]
func (h *ProductHandler) GetReviews(c *gin.Context) {
	productID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product id"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	reviews, total, err := h.svc.GetReviews(c.Request.Context(), productID, page, pageSize)
	if err != nil {
		h.logger.Error("get reviews failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load reviews"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":      reviews,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// ──────────────────────────────────────────────────────────────────────────────
// Authenticated — Buyer
// ──────────────────────────────────────────────────────────────────────────────

// CreateReview godoc
//
//	@Summary     Submit a product review
//	@Description Buyer must have purchased the product (verified via order_item_id).
//	@Tags        products
//	@Accept      json
//	@Produce     json
//	@Param       id   path     string                      true "Product UUID"
//	@Param       body body     models.CreateReviewRequest  true "Review payload"
//	@Success     201  {object} models.Review
//	@Router      /products/{id}/reviews [post]
func (h *ProductHandler) CreateReview(c *gin.Context) {
	productID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product id"})
		return
	}

	buyerID, buyerName, ok := callerIdentity(c)
	if !ok {
		return
	}

	var req models.CreateReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	review, err := h.svc.CreateReview(c.Request.Context(), productID, buyerID, &req, buyerName)
	if err != nil {
		h.logger.Error("create review failed", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, review)
}

// ──────────────────────────────────────────────────────────────────────────────
// Authenticated — Seller
// ──────────────────────────────────────────────────────────────────────────────

// ListSellerProducts godoc
//
//	@Summary     List seller's own products
//	@Tags        seller
//	@Produce     json
//	@Success     200 {object} map[string]interface{}
//	@Router      /seller/products [get]
func (h *ProductHandler) ListSellerProducts(c *gin.Context) {
	sellerID, _, ok := callerIdentity(c)
	if !ok {
		return
	}

	var f models.ProductListFilters
	if err := c.ShouldBindQuery(&f); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	f.SellerID = &sellerID
	// Sellers can see all statuses including draft/inactive
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 {
		f.PageSize = 20
	}

	products, total, err := h.svc.ListProducts(c.Request.Context(), f)
	if err != nil {
		h.logger.Error("list seller products failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list products"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":      products,
		"total":     total,
		"page":      f.Page,
		"page_size": f.PageSize,
	})
}

// CreateProduct godoc
//
//	@Summary     Create a product listing
//	@Tags        seller
//	@Accept      json
//	@Produce     json
//	@Param       body body     models.CreateProductRequest true "Product payload"
//	@Success     201  {object} models.Product
//	@Router      /seller/products [post]
func (h *ProductHandler) CreateProduct(c *gin.Context) {
	sellerID, _, ok := callerIdentity(c)
	if !ok {
		return
	}

	var req models.CreateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	product, err := h.svc.CreateProduct(c.Request.Context(), sellerID, &req)
	if err != nil {
		h.logger.Error("create product failed", zap.String("seller_id", sellerID.String()), zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, product)
}

// UpdateProduct godoc
//
//	@Summary     Update a product listing
//	@Tags        seller
//	@Accept      json
//	@Produce     json
//	@Param       id   path     string                      true "Product UUID"
//	@Param       body body     models.UpdateProductRequest true "Update payload"
//	@Success     200  {object} models.Product
//	@Router      /seller/products/{id} [put]
func (h *ProductHandler) UpdateProduct(c *gin.Context) {
	sellerID, _, ok := callerIdentity(c)
	if !ok {
		return
	}

	productID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product id"})
		return
	}

	var req models.UpdateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	product, err := h.svc.UpdateProduct(c.Request.Context(), sellerID, productID, &req)
	if err != nil {
		h.logger.Error("update product failed", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, product)
}

// DeleteProduct godoc
//
//	@Summary     Delete (soft-delete) a product listing
//	@Tags        seller
//	@Produce     json
//	@Param       id path string true "Product UUID"
//	@Success     204
//	@Router      /seller/products/{id} [delete]
func (h *ProductHandler) DeleteProduct(c *gin.Context) {
	sellerID, _, ok := callerIdentity(c)
	if !ok {
		return
	}

	productID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product id"})
		return
	}

	if err := h.svc.DeleteProduct(c.Request.Context(), sellerID, productID); err != nil {
		h.logger.Error("delete product failed", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// UploadImages godoc
//
//	@Summary     Upload product images
//	@Description Uploads up to 10 images to S3 and appends their URLs to the product.
//	@Tags        seller
//	@Accept      multipart/form-data
//	@Produce     json
//	@Param       id     path      string true "Product UUID"
//	@Param       images formData  file   true "Image files (jpg/png/webp)"
//	@Success     200 {object} map[string]interface{}
//	@Router      /seller/products/{id}/images [post]
func (h *ProductHandler) UploadImages(c *gin.Context) {
	sellerID, _, ok := callerIdentity(c)
	if !ok {
		return
	}

	productID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product id"})
		return
	}

	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid multipart form"})
		return
	}

	files := form.File["images"]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no images provided"})
		return
	}
	if len(files) > 10 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "maximum 10 images per upload"})
		return
	}

	urls, err := h.svc.UploadProductImages(c.Request.Context(), sellerID, productID, files)
	if err != nil {
		h.logger.Error("upload images failed", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"uploaded": len(urls),
		"urls":     urls,
	})
}

// ──────────────────────────────────────────────────────────────────────────────
// Shared helpers
// ──────────────────────────────────────────────────────────────────────────────

// callerIdentity extracts the authenticated caller's UUID and display name from
// Gin's context (populated by the JWT middleware). Returns false and writes a
// 401 if the values are missing.
func callerIdentity(c *gin.Context) (uuid.UUID, string, bool) {
	rawID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return uuid.Nil, "", false
	}

	var callerID uuid.UUID
	switch v := rawID.(type) {
	case uuid.UUID:
		callerID = v
	case string:
		var err error
		callerID, err = uuid.Parse(v)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user id in token"})
			return uuid.Nil, "", false
		}
	default:
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return uuid.Nil, "", false
	}

	displayName, _ := c.Get("display_name")
	name, _ := displayName.(string)
	if name == "" {
		name = "User"
	}

	return callerID, name, true
}
