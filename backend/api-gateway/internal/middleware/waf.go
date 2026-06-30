package middleware

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tiktok-clone/api-gateway/internal/config"
)

// wafRule describes a single WAF detection rule.
type wafRule struct {
	Name    string
	Pattern *regexp.Regexp
	Score   int // contribution to threat score; request blocked when total >= blockThreshold
}

const blockThreshold = 10

// --- SQLi detection patterns ---
var sqliRules = []wafRule{
	{
		Name:    "sqli_union_select",
		Pattern: regexp.MustCompile(`(?i)(union\s+(all\s+)?select|intersect\s+select|except\s+select)`),
		Score:   10,
	},
	{
		Name:    "sqli_or_and_tautology",
		Pattern: regexp.MustCompile(`(?i)\b(or|and)\s+[\w'"]+\s*=\s*[\w'"]+`),
		Score:   5,
	},
	{
		Name:    "sqli_comment",
		Pattern: regexp.MustCompile(`(--|#|/\*[\s\S]*?\*/|;--)`),
		Score:   5,
	},
	{
		Name:    "sqli_sleep_benchmark",
		Pattern: regexp.MustCompile(`(?i)(sleep\s*\(|benchmark\s*\(|waitfor\s+delay)`),
		Score:   10,
	},
	{
		Name:    "sqli_drop_table",
		Pattern: regexp.MustCompile(`(?i)(drop\s+table|truncate\s+table|delete\s+from|insert\s+into|update\s+\w+\s+set)`),
		Score:   10,
	},
	{
		Name:    "sqli_hex_encoding",
		Pattern: regexp.MustCompile(`(?i)(0x[0-9a-f]{4,}|char\s*\(|concat\s*\(|convert\s*\()`),
		Score:   5,
	},
	{
		Name:    "sqli_information_schema",
		Pattern: regexp.MustCompile(`(?i)(information_schema|sysobjects|syscolumns|pg_catalog)`),
		Score:   10,
	},
}

// --- XSS detection patterns ---
var xssRules = []wafRule{
	{
		Name:    "xss_script_tag",
		Pattern: regexp.MustCompile(`(?i)<\s*script[^>]*>`),
		Score:   10,
	},
	{
		Name:    "xss_event_handler",
		Pattern: regexp.MustCompile(`(?i)\bon\w+\s*=`),
		Score:   8,
	},
	{
		Name:    "xss_javascript_proto",
		Pattern: regexp.MustCompile(`(?i)(javascript\s*:|vbscript\s*:|data\s*:text/html)`),
		Score:   10,
	},
	{
		Name:    "xss_iframe",
		Pattern: regexp.MustCompile(`(?i)<\s*(iframe|frame|object|embed|applet)[^>]*>`),
		Score:   8,
	},
	{
		Name:    "xss_eval",
		Pattern: regexp.MustCompile(`(?i)(eval\s*\(|expression\s*\(|document\.write\s*\()`),
		Score:   8,
	},
	{
		Name:    "xss_html_entities",
		Pattern: regexp.MustCompile(`(?i)&#x?[0-9a-f]+;`),
		Score:   4,
	},
	{
		Name:    "xss_svg_onload",
		Pattern: regexp.MustCompile(`(?i)<\s*svg[^>]*onload\s*=`),
		Score:   10,
	},
}

// --- Path traversal detection patterns ---
var pathTraversalRules = []wafRule{
	{
		Name:    "path_traversal_dotdot",
		Pattern: regexp.MustCompile(`(\.\./|\.\.\\|%2e%2e%2f|%2e%2e/|\.\.%2f|%2e\.%2f)`),
		Score:   10,
	},
	{
		Name:    "path_traversal_encoded",
		Pattern: regexp.MustCompile(`(?i)(%c0%ae|%c0af|%c1%9c|%25c0%25ae)`),
		Score:   10,
	},
	{
		Name:    "path_traversal_etc_passwd",
		Pattern: regexp.MustCompile(`(?i)(etc/passwd|etc/shadow|etc/hosts|windows/system32|win\.ini|boot\.ini)`),
		Score:   10,
	},
	{
		Name:    "path_traversal_null_byte",
		Pattern: regexp.MustCompile(`(%00|\x00)`),
		Score:   10,
	},
}

// WAFMiddleware implements a Web Application Firewall.
type WAFMiddleware struct {
	cfg             *config.WAFConfig
	sqliRules       []wafRule
	xssRules        []wafRule
	pathRules       []wafRule
	blockedIPs      map[string]struct{}
	allowedHosts    map[string]struct{}
}

// NewWAFMiddleware creates a configured WAF middleware instance.
func NewWAFMiddleware(cfg *config.WAFConfig) *WAFMiddleware {
	blockedIPs := make(map[string]struct{}, len(cfg.BlockedIPs))
	for _, ip := range cfg.BlockedIPs {
		blockedIPs[strings.TrimSpace(ip)] = struct{}{}
	}

	allowedHosts := make(map[string]struct{}, len(cfg.AllowedHosts))
	for _, h := range cfg.AllowedHosts {
		allowedHosts[strings.TrimSpace(strings.ToLower(h))] = struct{}{}
	}

	return &WAFMiddleware{
		cfg:          cfg,
		sqliRules:    sqliRules,
		xssRules:     xssRules,
		pathRules:    pathTraversalRules,
		blockedIPs:   blockedIPs,
		allowedHosts: allowedHosts,
	}
}

// Protect returns the WAF gin.HandlerFunc.
func (w *WAFMiddleware) Protect() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !w.cfg.Enabled {
			c.Next()
			return
		}

		// 1. IP blocklist check.
		if _, blocked := w.blockedIPs[c.ClientIP()]; blocked {
			w.block(c, "blocked_ip", "your IP address has been blocked")
			return
		}

		// 2. Host allowlist check (when configured).
		if len(w.allowedHosts) > 0 {
			host := strings.ToLower(c.Request.Host)
			if _, ok := w.allowedHosts[host]; !ok {
				w.block(c, "invalid_host", fmt.Sprintf("host %q is not allowed", host))
				return
			}
		}

		// 3. Body size limit.
		if c.Request.ContentLength > w.cfg.MaxBodySize {
			w.block(c, "body_too_large",
				fmt.Sprintf("request body exceeds maximum size of %d bytes", w.cfg.MaxBodySize))
			return
		}

		// 4. Read and inspect body (with size cap as a safety net).
		var bodyBytes []byte
		if c.Request.Body != nil {
			limited := io.LimitReader(c.Request.Body, w.cfg.MaxBodySize+1)
			data, err := io.ReadAll(limited)
			if err == nil {
				if int64(len(data)) > w.cfg.MaxBodySize {
					w.block(c, "body_too_large", "request body exceeds maximum allowed size")
					return
				}
				bodyBytes = data
				// Restore body for downstream handlers.
				c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			}
		}

		// 5. Collect all inspectable surfaces.
		surfaces := w.collectSurfaces(c.Request, bodyBytes)

		// 6. Evaluate each enabled rule category.
		if w.cfg.BlockSQLi {
			if violation := w.evaluate(surfaces, w.sqliRules); violation != "" {
				w.block(c, "sqli_detected", "SQL injection attempt detected: "+violation)
				return
			}
		}

		if w.cfg.BlockXSS {
			if violation := w.evaluate(surfaces, w.xssRules); violation != "" {
				w.block(c, "xss_detected", "cross-site scripting attempt detected: "+violation)
				return
			}
		}

		if w.cfg.BlockPathTraversal {
			if violation := w.evaluate(surfaces, w.pathRules); violation != "" {
				w.block(c, "path_traversal_detected", "path traversal attempt detected: "+violation)
				return
			}
		}

		c.Next()
	}
}

// collectSurfaces gathers all user-controlled input into inspectable strings.
func (w *WAFMiddleware) collectSurfaces(r *http.Request, body []byte) []string {
	surfaces := make([]string, 0, 10)

	// Raw URL and path.
	surfaces = append(surfaces, r.URL.Path)
	surfaces = append(surfaces, r.URL.RawQuery)

	// Decoded query parameters.
	for _, values := range r.URL.Query() {
		for _, v := range values {
			decoded, err := url.QueryUnescape(v)
			if err == nil {
				surfaces = append(surfaces, decoded)
			}
			surfaces = append(surfaces, v)
		}
	}

	// Headers that carry user data.
	userHeaders := []string{
		"User-Agent",
		"Referer",
		"X-Forwarded-For",
		"X-Real-IP",
		"Cookie",
		"Content-Disposition",
		"X-Custom-Header",
	}
	for _, h := range userHeaders {
		if v := r.Header.Get(h); v != "" {
			surfaces = append(surfaces, v)
		}
	}

	// Request body.
	if len(body) > 0 {
		surfaces = append(surfaces, string(body))
		// Also URL-decode the body in case of form encoding.
		decoded, err := url.QueryUnescape(string(body))
		if err == nil && decoded != string(body) {
			surfaces = append(surfaces, decoded)
		}
	}

	return surfaces
}

// evaluate runs all rules against all surfaces, accumulating a threat score.
// Returns the name of the first rule that causes the score to exceed the threshold.
func (w *WAFMiddleware) evaluate(surfaces []string, rules []wafRule) string {
	score := 0
	for _, surface := range surfaces {
		for _, rule := range rules {
			if rule.Pattern.MatchString(surface) {
				score += rule.Score
				if score >= blockThreshold {
					return rule.Name
				}
			}
		}
	}
	return ""
}

func (w *WAFMiddleware) block(c *gin.Context, code, message string) {
	c.Header("X-WAF-Block-Reason", code)
	c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
		"error":   code,
		"message": message,
	})
}

// AddBlockedIP dynamically adds an IP to the blocklist at runtime.
func (w *WAFMiddleware) AddBlockedIP(ip string) {
	w.blockedIPs[strings.TrimSpace(ip)] = struct{}{}
}

// RemoveBlockedIP removes an IP from the runtime blocklist.
func (w *WAFMiddleware) RemoveBlockedIP(ip string) {
	delete(w.blockedIPs, strings.TrimSpace(ip))
}
