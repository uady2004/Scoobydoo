package handlers

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sony/gobreaker"
	"github.com/tiktok-clone/api-gateway/internal/config"
	"github.com/tiktok-clone/api-gateway/internal/middleware"
)

// roundRobinBalancer selects upstream targets in round-robin order.
type roundRobinBalancer struct {
	targets []*url.URL
	counter uint64
}

func newRoundRobinBalancer(addrs []string) (*roundRobinBalancer, error) {
	targets := make([]*url.URL, 0, len(addrs))
	for _, addr := range addrs {
		u, err := url.Parse(addr)
		if err != nil {
			return nil, fmt.Errorf("parsing address %q: %w", addr, err)
		}
		targets = append(targets, u)
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("no targets provided")
	}
	return &roundRobinBalancer{targets: targets}, nil
}

func (b *roundRobinBalancer) next() *url.URL {
	idx := atomic.AddUint64(&b.counter, 1) % uint64(len(b.targets))
	return b.targets[idx]
}

// timedTransport wraps an http.RoundTripper and enforces a per-request timeout
// via context cancellation.
type timedTransport struct {
	transport http.RoundTripper
	timeout   time.Duration
}

func (t *timedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.timeout <= 0 {
		return t.transport.RoundTrip(req)
	}
	ctx, cancel := context.WithTimeout(req.Context(), t.timeout)
	defer cancel()
	return t.transport.RoundTrip(req.WithContext(ctx))
}

// reverseProxyPool holds a load balancer and shared transport for one service.
type reverseProxyPool struct {
	balancer  *roundRobinBalancer
	transport http.RoundTripper
	timeout   time.Duration
}

func newReverseProxyPool(cfg config.ServiceEndpoint) (*reverseProxyPool, error) {
	balancer, err := newRoundRobinBalancer(cfg.Addresses)
	if err != nil {
		return nil, err
	}

	maxIdle := cfg.MaxIdleConn
	if maxIdle == 0 {
		maxIdle = 100
	}
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          maxIdle,
		MaxIdleConnsPerHost:   maxIdle,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: timeout,
	}

	return &reverseProxyPool{
		balancer:  balancer,
		transport: transport,
		timeout:   timeout,
	}, nil
}

// ProxyRouter routes incoming requests to downstream microservices.
type ProxyRouter struct {
	pools     map[string]*reverseProxyPool
	cbManager *middleware.CircuitBreakerManager
	cfg       *config.ServicesConfig
	mu        sync.RWMutex
}

// NewProxyRouter creates a ProxyRouter wired to all downstream services.
func NewProxyRouter(cfg *config.ServicesConfig, cbm *middleware.CircuitBreakerManager) (*ProxyRouter, error) {
	pools := make(map[string]*reverseProxyPool)

	serviceMap := map[string]config.ServiceEndpoint{
		"auth":         cfg.Auth,
		"user":         cfg.User,
		"video":        cfg.Video,
		"feed":         cfg.Feed,
		"comment":      cfg.Comment,
		"like":         cfg.Like,
		"follow":       cfg.Follow,
		"search":       cfg.Search,
		"notification": cfg.Notification,
		"analytics":    cfg.Analytics,
		"live":         cfg.Live,
		"interaction":  cfg.Interaction,
		"reporting":    cfg.Reporting,
		"wallet":       cfg.Wallet,
		"messaging":    cfg.Messaging,
		"ecommerce":    cfg.Ecommerce,
	}

	for name, svcCfg := range serviceMap {
		pool, err := newReverseProxyPool(svcCfg)
		if err != nil {
			return nil, fmt.Errorf("initializing pool for service %q: %w", name, err)
		}
		pools[name] = pool
	}

	return &ProxyRouter{
		pools:     pools,
		cbManager: cbm,
		cfg:       cfg,
	}, nil
}

// RegisterRoutes attaches all API routes to the given Gin engine.
func (pr *ProxyRouter) RegisterRoutes(
	r *gin.Engine,
	jwtValidator *middleware.JWTValidator,
	rbac *middleware.RBACMiddleware,
	rateLimiter *middleware.RateLimiter,
	waf *middleware.WAFMiddleware,
	auditLogger *middleware.AuditLogger,
) {
	// Global middleware applied to every route.
	r.Use(
		waf.Protect(),
		auditLogger.Middleware(),
	)

	// ── Public endpoints ──────────────────────────────────────────────────────
	// IP rate-limited; no JWT required.
	public := r.Group("/api/v1")
	public.Use(rateLimiter.IPRateLimit())
	{
		public.POST("/auth/login", pr.proxyTo("auth", false))
		public.POST("/auth/register", pr.proxyTo("auth", false))
		public.POST("/auth/refresh", pr.proxyTo("auth", false))
		public.POST("/auth/oauth/google", pr.proxyTo("auth", false))
		public.POST("/auth/oauth/apple", pr.proxyTo("auth", false))
		public.POST("/auth/otp/send",
			rateLimiter.EndpointRateLimit("otp-send", 5),
			pr.proxyTo("auth", false))
		public.POST("/auth/otp/verify", pr.proxyTo("auth", false))
		public.POST("/auth/verify-email", pr.proxyTo("auth", false))
		public.POST("/auth/forgot-password",
			rateLimiter.EndpointRateLimit("forgot-pwd", 5),
			pr.proxyTo("auth", false))
		public.POST("/auth/reset-password", pr.proxyTo("auth", false))

		// Optional-auth video browsing.
		public.GET("/videos/trending", jwtValidator.OptionalJWT(), pr.proxyTo("feed", true))
		public.GET("/videos/:video_id", jwtValidator.OptionalJWT(), pr.proxyTo("video", true))

		// Search — public with optional auth for personalisation
		public.GET("/search", pr.proxyTo("search", true))
		public.GET("/search/trending", pr.proxyTo("search", false))
		public.GET("/search/suggestions", pr.proxyTo("search", false))

		// Public content browsing
		public.GET("/sounds/trending", pr.proxyTo("video", false))
		public.GET("/sounds/search", pr.proxyTo("video", false))
		public.GET("/sounds/:sound_id", pr.proxyTo("video", false))
		public.GET("/hashtags/trending", pr.proxyTo("search", false))
		public.GET("/hashtags/:tag", pr.proxyTo("search", false))
	}

	// ── Authenticated endpoints ───────────────────────────────────────────────
	authed := r.Group("/api/v1")
	authed.Use(
		jwtValidator.ValidateJWT(),
		rateLimiter.UserRateLimit(),
	)
	{
		// User profile
		authed.GET("/users/me", pr.proxyTo("user", true))
		authed.PUT("/users/me", pr.proxyTo("user", true))
		authed.DELETE("/users/me", pr.proxyTo("user", true))
		authed.POST("/users/me/avatar", pr.proxyTo("user", true))
		authed.GET("/users/:user_id", pr.proxyTo("user", true))
		authed.GET("/users/:user_id/videos", pr.proxyTo("video", true))

		// Video CRUD
		videos := authed.Group("/videos")
		{
			videos.POST("",
				rbac.RequirePermission(middleware.PermVideoUpload),
				pr.proxyTo("video", true))
			videos.PUT("/:video_id", pr.proxyTo("video", true))
			videos.DELETE("/:video_id", pr.proxyTo("video", true))
			videos.GET("/:video_id/comments", pr.proxyTo("interaction", true))
		}

		// Feed
		authed.GET("/feed", pr.proxyTo("feed", true))
		authed.GET("/feed/following", pr.proxyTo("feed", true))
		authed.GET("/feed/for-you", pr.proxyTo("feed", true))
		authed.POST("/feed/view", pr.proxyTo("feed", true))

		// Comments
		authed.POST("/videos/:video_id/comments",
			rbac.RequirePermission(middleware.PermCommentCreate),
			pr.proxyTo("interaction", true))
		authed.DELETE("/videos/:video_id/comments/:comment_id", pr.proxyTo("interaction", true))
		authed.DELETE("/comments/:comment_id", pr.proxyTo("interaction", true))
		authed.POST("/comments/:comment_id/like", pr.proxyTo("interaction", true))
		authed.DELETE("/comments/:comment_id/like", pr.proxyTo("interaction", true))
		authed.POST("/comments/:comment_id/pin",
			rbac.RequirePermission(middleware.PermCommentCreate),
			pr.proxyTo("interaction", true))
		authed.POST("/comments/:comment_id/report", pr.proxyTo("interaction", true))

		// Likes
		authed.POST("/videos/:video_id/like", pr.proxyTo("interaction", true))
		authed.DELETE("/videos/:video_id/like", pr.proxyTo("interaction", true))
		authed.GET("/videos/:video_id/likes", pr.proxyTo("interaction", true))
		authed.GET("/videos/:video_id/like-status", pr.proxyTo("interaction", true))
		authed.GET("/me/liked-videos", pr.proxyTo("interaction", true))

		// Bookmarks
		authed.POST("/videos/:video_id/bookmark", pr.proxyTo("interaction", true))
		authed.DELETE("/videos/:video_id/bookmark", pr.proxyTo("interaction", true))
		authed.GET("/me/bookmarks", pr.proxyTo("interaction", true))
		authed.GET("/bookmarks", pr.proxyTo("interaction", true))
		authed.POST("/bookmarks/collections", pr.proxyTo("interaction", true))
		authed.GET("/bookmarks/collections", pr.proxyTo("interaction", true))

		// Following
		authed.POST("/users/:user_id/follow", pr.proxyTo("follow", true))
		authed.DELETE("/users/:user_id/follow", pr.proxyTo("follow", true))
		authed.GET("/users/:user_id/followers", pr.proxyTo("follow", true))
		authed.GET("/users/:user_id/following", pr.proxyTo("follow", true))

		// Notifications
		authed.GET("/notifications", pr.proxyTo("notification", true))
		authed.PUT("/notifications/:notification_id/read", pr.proxyTo("notification", true))
		authed.PUT("/notifications/read-all", pr.proxyTo("notification", true))
		authed.GET("/notifications/unread-count", pr.proxyTo("notification", true))
		authed.DELETE("/notifications/:notification_id", pr.proxyTo("notification", true))

		// Creator analytics (creator+ only)
		analytics := authed.Group("/analytics")
		analytics.Use(rbac.CreatorOrAbove())
		{
			analytics.GET("/videos/:video_id/stats", pr.proxyTo("analytics", true))
			analytics.GET("/profile/stats", pr.proxyTo("analytics", true))
			analytics.GET("/revenue",
				rbac.RequirePermission(middleware.PermAnalyticsWrite),
				pr.proxyTo("analytics", true))
		}

		// Live streaming
		live := authed.Group("/live")
		{
			live.POST("/start",
				rbac.RequirePermission(middleware.PermLiveStart),
				pr.proxyTo("live", true))
			live.DELETE("/:stream_id/stop", pr.proxyTo("live", true))
			live.GET("/:stream_id", pr.proxyTo("live", true))
			live.GET("/:stream_id/viewers", pr.proxyTo("live", true))
			live.POST("/:stream_id/comments",
				rbac.RequirePermission(middleware.PermCommentCreate),
				pr.proxyTo("live", true))
		}

		// Session management
		authed.POST("/auth/logout", pr.proxyTo("auth", true))
		authed.GET("/auth/sessions", pr.proxyTo("auth", true))
		authed.DELETE("/auth/sessions/:session_id", pr.proxyTo("auth", true))

		// Reports (user-submitted content reports)
		authed.POST("/reports", pr.proxyTo("reporting", true))

		// Gifts
		authed.GET("/gifts", pr.proxyTo("live", true))
		authed.POST("/gifts/send", pr.proxyTo("live", true))

		// Search (authenticated for history)
		authed.POST("/search/history", pr.proxyTo("search", true))
		authed.GET("/search/history", pr.proxyTo("search", true))
		authed.DELETE("/search/history", pr.proxyTo("search", true))
		authed.GET("/search/videos", pr.proxyTo("search", true))
		authed.GET("/search/users", pr.proxyTo("search", true))
		authed.GET("/search/hashtags", pr.proxyTo("search", true))
		authed.GET("/search/sounds", pr.proxyTo("search", true))

		// Video view tracking
		authed.POST("/videos/:video_id/view", pr.proxyTo("video", true))

		// Video uploads (chunked)
		authed.POST("/uploads/initiate",
			rbac.RequirePermission(middleware.PermVideoUpload),
			pr.proxyTo("video", true))
		authed.POST("/uploads/chunk", pr.proxyTo("video", true))
		authed.POST("/uploads/complete", pr.proxyTo("video", true))
		authed.GET("/uploads/resume", pr.proxyTo("video", true))
		authed.GET("/uploads/progress", pr.proxyTo("video", true))

		// Wallet
		authed.GET("/wallet/balance", pr.proxyTo("wallet", true))
		authed.GET("/wallet/packages", pr.proxyTo("wallet", true))
		authed.GET("/wallet/transactions", pr.proxyTo("wallet", true))
		authed.POST("/wallet/coins/buy", pr.proxyTo("wallet", true))
		authed.POST("/wallet/coins/confirm", pr.proxyTo("wallet", true))
		authed.POST("/wallet/buy-coins", pr.proxyTo("wallet", true))
		authed.POST("/wallet/gift", pr.proxyTo("wallet", true))
		authed.POST("/wallet/tip", pr.proxyTo("wallet", true))
		authed.POST("/wallet/subscribe", pr.proxyTo("wallet", true))
		authed.POST("/wallet/withdraw", pr.proxyTo("wallet", true))
		authed.GET("/wallet/convert-diamonds", pr.proxyTo("wallet", true))

		// Messaging
		authed.GET("/conversations", pr.proxyTo("messaging", true))
		authed.POST("/conversations", pr.proxyTo("messaging", true))
		authed.GET("/conversations/unread/total", pr.proxyTo("messaging", true))
		authed.GET("/conversations/:conversation_id/unread", pr.proxyTo("messaging", true))
		authed.GET("/messages", pr.proxyTo("messaging", true))
		authed.POST("/messages", pr.proxyTo("messaging", true))
		authed.POST("/messages/read", pr.proxyTo("messaging", true))
		authed.DELETE("/messages/:message_id", pr.proxyTo("messaging", true))
		authed.POST("/messages/:message_id/reactions", pr.proxyTo("messaging", true))
		authed.DELETE("/messages/:message_id/reactions/:emoji", pr.proxyTo("messaging", true))
		authed.GET("/ws", pr.proxyTo("messaging", true))
		authed.POST("/groups", pr.proxyTo("messaging", true))
		authed.POST("/groups/:group_id/members", pr.proxyTo("messaging", true))
		authed.DELETE("/groups/:group_id/members", pr.proxyTo("messaging", true))
		authed.POST("/media/upload", pr.proxyTo("messaging", true))
	}

	// ── Ecommerce endpoints ───────────────────────────────────────────────────
	{
		// Public product browsing
		public.GET("/products", pr.proxyTo("ecommerce", true))
		public.GET("/products/search", pr.proxyTo("ecommerce", true))
		public.GET("/products/:id", pr.proxyTo("ecommerce", true))
		public.GET("/products/:id/reviews", pr.proxyTo("ecommerce", true))

		// Authenticated buyer actions
		authed.POST("/products/:id/reviews", pr.proxyTo("ecommerce", true))
		authed.GET("/cart", pr.proxyTo("ecommerce", true))
		authed.POST("/cart/items", pr.proxyTo("ecommerce", true))
		authed.PUT("/cart/items/:item_id", pr.proxyTo("ecommerce", true))
		authed.DELETE("/cart/items/:item_id", pr.proxyTo("ecommerce", true))
		authed.POST("/cart/checkout", pr.proxyTo("ecommerce", true))
		authed.POST("/orders", pr.proxyTo("ecommerce", true))
		authed.GET("/orders", pr.proxyTo("ecommerce", true))
		authed.GET("/orders/:id", pr.proxyTo("ecommerce", true))
		authed.POST("/orders/:id/cancel", pr.proxyTo("ecommerce", true))
		authed.GET("/orders/:id/track", pr.proxyTo("ecommerce", true))
		authed.POST("/orders/:id/returns", pr.proxyTo("ecommerce", true))
		authed.POST("/orders/:id/refund", pr.proxyTo("ecommerce", true))

		// Seller management
		authed.GET("/seller/products", pr.proxyTo("ecommerce", true))
		authed.POST("/seller/products", pr.proxyTo("ecommerce", true))
		authed.PUT("/seller/products/:id", pr.proxyTo("ecommerce", true))
		authed.DELETE("/seller/products/:id", pr.proxyTo("ecommerce", true))
		authed.POST("/seller/products/:id/images", pr.proxyTo("ecommerce", true))
		authed.GET("/seller/orders", pr.proxyTo("ecommerce", true))
		authed.GET("/seller/orders/:id", pr.proxyTo("ecommerce", true))
		authed.PUT("/seller/orders/:id/status", pr.proxyTo("ecommerce", true))
	}

	// ── Moderator endpoints ───────────────────────────────────────────────────
	mod := r.Group("/api/v1/moderation")
	mod.Use(
		jwtValidator.ValidateJWT(),
		rateLimiter.UserRateLimit(),
		rbac.ModeratorOrAbove(),
	)
	{
		mod.GET("/reports", pr.proxyTo("video", true))
		mod.PUT("/videos/:video_id/hide", pr.proxyTo("video", true))
		mod.PUT("/videos/:video_id/restore", pr.proxyTo("video", true))
		mod.PUT("/comments/:comment_id/hide", pr.proxyTo("comment", true))
		mod.PUT("/users/:user_id/ban",
			rbac.RequirePermission(middleware.PermUserBan),
			pr.proxyTo("user", true))
		mod.PUT("/users/:user_id/unban",
			rbac.RequirePermission(middleware.PermUserBan),
			pr.proxyTo("user", true))
		mod.GET("/live/:stream_id/moderate", pr.proxyTo("live", true))
	}

	// ── Admin endpoints ───────────────────────────────────────────────────────
	admin := r.Group("/api/v1/admin")
	admin.Use(
		jwtValidator.ValidateJWT(),
		rateLimiter.UserRateLimit(),
		rbac.AdminOnly(),
	)
	{
		admin.GET("/users", pr.proxyTo("user", true))
		admin.DELETE("/users/:user_id",
			rbac.RequirePermission(middleware.PermUserDelete),
			pr.proxyTo("user", true))
		admin.PUT("/users/:user_id/role",
			rbac.RequirePermission(middleware.PermRoleAssign),
			pr.proxyTo("user", true))
		admin.GET("/audit-logs",
			rbac.RequirePermission(middleware.PermAuditRead),
			pr.proxyTo("analytics", true))
		admin.GET("/system/config",
			rbac.RequirePermission(middleware.PermSystemConfig),
			pr.proxyTo("user", true))

		// Circuit breaker management
		admin.GET("/circuit-breakers", pr.circuitBreakerStatus())
		admin.POST("/circuit-breakers/:service/reset", pr.circuitBreakerReset())
	}

	// ── Internal probes (no auth) ─────────────────────────────────────────────
	r.GET("/health", pr.healthCheck())
	r.GET("/ready", pr.readinessCheck())
	r.GET("/metrics", pr.metricsHandler())
}

// proxyTo returns a gin.HandlerFunc that reverse-proxies the request to the
// named service, optionally guarding with the circuit breaker.
func (pr *ProxyRouter) proxyTo(service string, useCircuitBreaker bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		pr.mu.RLock()
		pool, ok := pr.pools[service]
		pr.mu.RUnlock()

		if !ok {
			c.AbortWithStatusJSON(http.StatusBadGateway, gin.H{
				"error":   "unknown_service",
				"message": fmt.Sprintf("no pool registered for service %q", service),
			})
			return
		}

		// Circuit breaker short-circuit before allocating the proxy.
		if useCircuitBreaker && pr.cbManager != nil {
			if pr.cbManager.State(service) == gobreaker.StateOpen {
				c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
					"error":   "circuit_open",
					"message": fmt.Sprintf("service %q is temporarily unavailable", service),
					"service": service,
				})
				return
			}
		}

		target := pool.balancer.next()
		c.Set("target_service", service)

		proxy := &httputil.ReverseProxy{
			Director: func(req *http.Request) {
				req.URL.Scheme = target.Scheme
				req.URL.Host = target.Host
				// Preserve the full path (including /api/v1) so each downstream
				// service — which registers its own /api/v1 route group — receives
				// exactly what it expects.
				if target.Path != "" {
					req.URL.Path = singleJoiningSlash(target.Path, req.URL.Path)
				}

				removeHopByHopHeaders(req.Header)

				if clientIP := c.ClientIP(); clientIP != "" {
					req.Header.Set("X-Forwarded-For", clientIP)
					req.Header.Set("X-Real-IP", clientIP)
				}
				if reqID := c.GetHeader("X-Request-ID"); reqID != "" {
					req.Header.Set("X-Request-ID", reqID)
				}
				req.Header.Set("X-Forwarded-Host", c.Request.Host)
				req.Header.Set("X-Forwarded-Proto", requestScheme(c.Request))
				req.Host = target.Host
			},
			Transport: &timedTransport{
				transport: pool.transport,
				timeout:   pool.timeout,
			},
			ModifyResponse: func(resp *http.Response) error {
				resp.Header.Del("X-Powered-By")
				resp.Header.Del("Server")
				return nil
			},
			ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
				log.Printf("[proxy] service=%s target=%s error=%v", service, target.Host, err)
				c.AbortWithStatusJSON(http.StatusBadGateway, gin.H{
					"error":   "upstream_error",
					"message": "upstream service is unavailable",
					"service": service,
				})
			},
		}

		proxy.ServeHTTP(c.Writer, c.Request)
	}
}

// --- Internal endpoint handlers ---

func (pr *ProxyRouter) healthCheck() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "ok",
			"timestamp": time.Now().UTC(),
			"service":   "api-gateway",
		})
	}
}

func (pr *ProxyRouter) readinessCheck() gin.HandlerFunc {
	return func(c *gin.Context) {
		states := pr.cbManager.AllStates()
		health := "ready"
		for _, state := range states {
			if state == gobreaker.StateOpen.String() {
				health = "degraded"
				break
			}
		}
		c.JSON(http.StatusOK, gin.H{
			"status":    health,
			"timestamp": time.Now().UTC(),
			"services":  states,
		})
	}
}

func (pr *ProxyRouter) metricsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		states := pr.cbManager.AllStates()
		metrics := make(map[string]interface{}, len(states))
		for svc, state := range states {
			counts := pr.cbManager.Counts(svc)
			metrics[svc] = map[string]interface{}{
				"state":                 state,
				"requests":              counts.Requests,
				"total_successes":       counts.TotalSuccesses,
				"total_failures":        counts.TotalFailures,
				"consecutive_failures":  counts.ConsecutiveFailures,
				"consecutive_successes": counts.ConsecutiveSuccesses,
			}
		}
		c.JSON(http.StatusOK, gin.H{
			"circuit_breakers": metrics,
			"timestamp":        time.Now().UTC(),
		})
	}
}

func (pr *ProxyRouter) circuitBreakerStatus() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"services":  pr.cbManager.AllStates(),
			"timestamp": time.Now().UTC(),
		})
	}
}

func (pr *ProxyRouter) circuitBreakerReset() gin.HandlerFunc {
	return func(c *gin.Context) {
		service := c.Param("service")
		pr.cbManager.Reset(service)
		c.JSON(http.StatusOK, gin.H{
			"message":   fmt.Sprintf("circuit breaker for %q has been reset", service),
			"service":   service,
			"timestamp": time.Now().UTC(),
		})
	}
}

// --- URL / header helpers ---

// stripAPIPrefix removes /api/v1 from the path so downstream services receive
// a clean path without the gateway prefix.
func stripAPIPrefix(path string) string {
	const prefix = "/api/v1"
	if strings.HasPrefix(path, prefix) {
		path = path[len(prefix):]
	}
	if path == "" {
		path = "/"
	}
	return path
}

func singleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		return a + "/" + b
	}
	return a + b
}

func requestScheme(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		return proto
	}
	return "http"
}

// removeHopByHopHeaders strips headers that must not be forwarded by a proxy.
func removeHopByHopHeaders(h http.Header) {
	hopHeaders := []string{
		"Connection", "Proxy-Connection", "Keep-Alive",
		"Proxy-Authenticate", "Proxy-Authorization",
		"Te", "Trailers", "Transfer-Encoding", "Upgrade",
	}
	for _, name := range hopHeaders {
		h.Del(name)
	}
	// Also remove headers listed in the Connection header value itself.
	if conn := h.Get("Connection"); conn != "" {
		for _, f := range strings.Split(conn, ",") {
			h.Del(strings.TrimSpace(f))
		}
	}
}
