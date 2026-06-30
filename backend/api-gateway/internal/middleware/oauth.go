package middleware

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tiktok-clone/api-gateway/internal/config"
)

// Ensure rand is used (needed for rsa.VerifyPKCS1v15 indirect dep).
var _ = rand.Reader

const (
	googleCertsURL = "https://www.googleapis.com/oauth2/v3/certs"
	appleKeysURL   = "https://appleid.apple.com/auth/keys"

	oauthProviderGoogle = "google"
	oauthProviderApple  = "apple"
)

// OAuthClaims contains the normalized identity extracted from any OAuth provider.
type OAuthClaims struct {
	Provider      string
	ProviderUID   string
	Email         string
	EmailVerified bool
	Name          string
	Picture       string
	RawIDToken    string
}

// jwksCache caches provider public keys with a configurable TTL.
type jwksCache struct {
	mu        sync.RWMutex
	keys      map[string]*rsa.PublicKey
	fetchedAt time.Time
	ttl       time.Duration
}

func newJWKSCache(ttl time.Duration) *jwksCache {
	return &jwksCache{
		keys: make(map[string]*rsa.PublicKey),
		ttl:  ttl,
	}
}

func (c *jwksCache) get(kid string) (*rsa.PublicKey, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if time.Since(c.fetchedAt) > c.ttl {
		return nil, false
	}
	k, ok := c.keys[kid]
	return k, ok
}

func (c *jwksCache) set(keys map[string]*rsa.PublicKey) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.keys = keys
	c.fetchedAt = time.Now()
}

// OAuthValidator validates Google and Apple OAuth2 ID tokens.
type OAuthValidator struct {
	cfg         *config.OAuthConfig
	httpClient  *http.Client
	googleCache *jwksCache
	appleCache  *jwksCache
}

// NewOAuthValidator creates a new OAuthValidator.
func NewOAuthValidator(cfg *config.OAuthConfig) *OAuthValidator {
	return &OAuthValidator{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		googleCache: newJWKSCache(5 * time.Minute),
		appleCache:  newJWKSCache(10 * time.Minute),
	}
}

// GoogleIDToken is a Gin middleware that validates a Google ID token passed as
// the Bearer token.
func (v *OAuthValidator) GoogleIDToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr, err := extractBearerToken(c.Request)
		if err != nil {
			abortWithError(c, http.StatusUnauthorized, "missing_token", err.Error())
			return
		}

		claims, err := v.validateGoogleIDToken(c.Request.Context(), tokenStr)
		if err != nil {
			abortWithError(c, http.StatusUnauthorized, "invalid_google_token", err.Error())
			return
		}

		v.populateContext(c, claims, oauthProviderGoogle)
		c.Next()
	}
}

// AppleIDToken is a Gin middleware that validates an Apple Sign-In ID token.
func (v *OAuthValidator) AppleIDToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr, err := extractBearerToken(c.Request)
		if err != nil {
			abortWithError(c, http.StatusUnauthorized, "missing_token", err.Error())
			return
		}

		claims, err := v.validateAppleIDToken(c.Request.Context(), tokenStr)
		if err != nil {
			abortWithError(c, http.StatusUnauthorized, "invalid_apple_token", err.Error())
			return
		}

		v.populateContext(c, claims, oauthProviderApple)
		c.Next()
	}
}

// MultiProviderOAuth validates tokens from either Google or Apple, chosen by
// the X-OAuth-Provider header.
func (v *OAuthValidator) MultiProviderOAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		provider := strings.ToLower(c.GetHeader("X-OAuth-Provider"))

		tokenStr, err := extractBearerToken(c.Request)
		if err != nil {
			abortWithError(c, http.StatusUnauthorized, "missing_token", err.Error())
			return
		}

		var claims *OAuthClaims
		switch provider {
		case oauthProviderGoogle:
			claims, err = v.validateGoogleIDToken(c.Request.Context(), tokenStr)
		case oauthProviderApple:
			claims, err = v.validateAppleIDToken(c.Request.Context(), tokenStr)
		default:
			abortWithError(c, http.StatusBadRequest, "unknown_provider",
				"X-OAuth-Provider must be 'google' or 'apple'")
			return
		}

		if err != nil {
			abortWithError(c, http.StatusUnauthorized, "invalid_oauth_token", err.Error())
			return
		}

		v.populateContext(c, claims, provider)
		c.Next()
	}
}

func (v *OAuthValidator) populateContext(c *gin.Context, claims *OAuthClaims, provider string) {
	c.Set("oauth_claims", claims)
	c.Set("oauth_provider", provider)
	c.Request.Header.Set("X-OAuth-Provider", provider)
	c.Request.Header.Set("X-OAuth-UID", claims.ProviderUID)
	c.Request.Header.Set("X-OAuth-Email", claims.Email)
}

// --- Google validation ---

type googleIDTokenPayload struct {
	Iss           string `json:"iss"`
	Sub           string `json:"sub"`
	Aud           string `json:"aud"`
	Iat           int64  `json:"iat"`
	Exp           int64  `json:"exp"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

func (v *OAuthValidator) validateGoogleIDToken(ctx context.Context, tokenStr string) (*OAuthClaims, error) {
	header, payloadBytes, err := splitAndDecodeJWT(tokenStr)
	if err != nil {
		return nil, fmt.Errorf("malformed token: %w", err)
	}

	if alg, _ := header["alg"].(string); alg != "RS256" {
		return nil, fmt.Errorf("unsupported algorithm: %s", alg)
	}

	kid, _ := header["kid"].(string)
	pubKey, err := v.getGooglePublicKey(ctx, kid)
	if err != nil {
		return nil, fmt.Errorf("fetching google public key: %w", err)
	}

	if err := verifyJWTSignatureRS256(tokenStr, pubKey); err != nil {
		return nil, fmt.Errorf("signature verification: %w", err)
	}

	var payload googleIDTokenPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return nil, fmt.Errorf("decoding claims: %w", err)
	}

	now := time.Now().Unix()
	if payload.Exp < now {
		return nil, errors.New("token is expired")
	}
	if payload.Iat > now+60 {
		return nil, errors.New("token issued in the future")
	}
	if payload.Aud != v.cfg.Google.ClientID {
		return nil, fmt.Errorf("audience mismatch: got %q", payload.Aud)
	}
	if payload.Iss != "accounts.google.com" && payload.Iss != "https://accounts.google.com" {
		return nil, fmt.Errorf("unexpected issuer: %s", payload.Iss)
	}
	if payload.Sub == "" {
		return nil, errors.New("missing subject claim")
	}

	return &OAuthClaims{
		Provider:      oauthProviderGoogle,
		ProviderUID:   payload.Sub,
		Email:         payload.Email,
		EmailVerified: payload.EmailVerified,
		Name:          payload.Name,
		Picture:       payload.Picture,
		RawIDToken:    tokenStr,
	}, nil
}

func (v *OAuthValidator) getGooglePublicKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	if key, ok := v.googleCache.get(kid); ok {
		return key, nil
	}
	keys, err := v.fetchJWKS(ctx, googleCertsURL)
	if err != nil {
		return nil, err
	}
	v.googleCache.set(keys)
	key, ok := keys[kid]
	if !ok {
		return nil, fmt.Errorf("no key with kid %q in Google JWKS", kid)
	}
	return key, nil
}

// --- Apple validation ---

type appleIDTokenPayload struct {
	Iss           string `json:"iss"`
	Sub           string `json:"sub"`
	Aud           string `json:"aud"`
	Iat           int64  `json:"iat"`
	Exp           int64  `json:"exp"`
	Email         string `json:"email"`
	EmailVerified string `json:"email_verified"` // Apple sends "true"/"false"
}

func (v *OAuthValidator) validateAppleIDToken(ctx context.Context, tokenStr string) (*OAuthClaims, error) {
	header, payloadBytes, err := splitAndDecodeJWT(tokenStr)
	if err != nil {
		return nil, fmt.Errorf("malformed token: %w", err)
	}

	if alg, _ := header["alg"].(string); alg != "RS256" {
		return nil, fmt.Errorf("unsupported algorithm: %s", alg)
	}

	kid, _ := header["kid"].(string)
	pubKey, err := v.getApplePublicKey(ctx, kid)
	if err != nil {
		return nil, fmt.Errorf("fetching apple public key: %w", err)
	}

	if err := verifyJWTSignatureRS256(tokenStr, pubKey); err != nil {
		return nil, fmt.Errorf("signature verification: %w", err)
	}

	var payload appleIDTokenPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return nil, fmt.Errorf("decoding claims: %w", err)
	}

	now := time.Now().Unix()
	if payload.Exp < now {
		return nil, errors.New("token is expired")
	}
	if payload.Iat > now+60 {
		return nil, errors.New("token issued in the future")
	}
	if payload.Aud != v.cfg.Apple.ClientID {
		return nil, fmt.Errorf("audience mismatch: got %q", payload.Aud)
	}
	if payload.Iss != "https://appleid.apple.com" {
		return nil, fmt.Errorf("unexpected issuer: %s", payload.Iss)
	}
	if payload.Sub == "" {
		return nil, errors.New("missing subject claim")
	}

	return &OAuthClaims{
		Provider:      oauthProviderApple,
		ProviderUID:   payload.Sub,
		Email:         payload.Email,
		EmailVerified: strings.EqualFold(payload.EmailVerified, "true"),
		RawIDToken:    tokenStr,
	}, nil
}

func (v *OAuthValidator) getApplePublicKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	if key, ok := v.appleCache.get(kid); ok {
		return key, nil
	}
	keys, err := v.fetchJWKS(ctx, appleKeysURL)
	if err != nil {
		return nil, err
	}
	v.appleCache.set(keys)
	key, ok := keys[kid]
	if !ok {
		return nil, fmt.Errorf("no key with kid %q in Apple JWKS", kid)
	}
	return key, nil
}

// --- JWKS helpers ---

type jwksDocument struct {
	Keys []jwkEntry `json:"keys"`
}

type jwkEntry struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
}

func (v *OAuthValidator) fetchJWKS(ctx context.Context, url string) (map[string]*rsa.PublicKey, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JWKS endpoint %s returned HTTP %d", url, resp.StatusCode)
	}

	var doc jwksDocument
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return nil, fmt.Errorf("decoding JWKS: %w", err)
	}

	result := make(map[string]*rsa.PublicKey, len(doc.Keys))
	for _, entry := range doc.Keys {
		if entry.Kty != "RSA" || entry.Kid == "" {
			continue
		}
		pub, err := rsaPublicKeyFromJWK(entry)
		if err != nil {
			continue
		}
		result[entry.Kid] = pub
	}

	return result, nil
}

func rsaPublicKeyFromJWK(k jwkEntry) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, fmt.Errorf("decoding modulus: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, fmt.Errorf("decoding exponent: %w", err)
	}
	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: int(new(big.Int).SetBytes(eBytes).Int64()),
	}, nil
}

// splitAndDecodeJWT parses a JWT string into its decoded header and payload.
func splitAndDecodeJWT(tokenStr string) (header map[string]interface{}, payload []byte, err error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return nil, nil, errors.New("token must have exactly three parts")
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, nil, fmt.Errorf("decoding header: %w", err)
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, nil, fmt.Errorf("parsing header: %w", err)
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, nil, fmt.Errorf("decoding payload: %w", err)
	}

	return header, payloadBytes, nil
}

// verifyJWTSignatureRS256 verifies the RS256 signature of a JWT using the given public key.
func verifyJWTSignatureRS256(tokenStr string, pubKey *rsa.PublicKey) error {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return errors.New("invalid JWT structure")
	}

	signingInput := parts[0] + "." + parts[1]
	digest := sha256.Sum256([]byte(signingInput))

	sigBytes, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return fmt.Errorf("decoding signature: %w", err)
	}

	return rsa.VerifyPKCS1v15(pubKey, crypto.SHA256, digest[:], sigBytes)
}
