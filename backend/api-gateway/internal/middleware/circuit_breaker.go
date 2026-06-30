package middleware

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sony/gobreaker"
	"github.com/tiktok-clone/api-gateway/internal/config"
)

// ServiceThreshold customizes circuit breaker settings for a specific service.
type ServiceThreshold struct {
	MaxRequests  uint32        // max requests in half-open state
	Interval     time.Duration // cyclic period to reset counts in closed state
	Timeout      time.Duration // time to stay open before half-open
	FailureRatio float64       // failure ratio threshold to open the circuit
	MinRequests  uint32        // min requests in interval before ratio is evaluated
}

// defaultThresholds defines per-service circuit breaker settings.
// Services with stricter SLAs (e.g., video upload) use lower thresholds.
var defaultThresholds = map[string]ServiceThreshold{
	"user":         {MaxRequests: 5, Interval: 60 * time.Second, Timeout: 30 * time.Second, FailureRatio: 0.6, MinRequests: 10},
	"video":        {MaxRequests: 3, Interval: 60 * time.Second, Timeout: 45 * time.Second, FailureRatio: 0.5, MinRequests: 5},
	"feed":         {MaxRequests: 5, Interval: 30 * time.Second, Timeout: 20 * time.Second, FailureRatio: 0.6, MinRequests: 10},
	"comment":      {MaxRequests: 5, Interval: 60 * time.Second, Timeout: 30 * time.Second, FailureRatio: 0.6, MinRequests: 10},
	"like":         {MaxRequests: 5, Interval: 60 * time.Second, Timeout: 30 * time.Second, FailureRatio: 0.6, MinRequests: 10},
	"follow":       {MaxRequests: 5, Interval: 60 * time.Second, Timeout: 30 * time.Second, FailureRatio: 0.6, MinRequests: 10},
	"search":       {MaxRequests: 5, Interval: 30 * time.Second, Timeout: 20 * time.Second, FailureRatio: 0.7, MinRequests: 10},
	"notification": {MaxRequests: 5, Interval: 60 * time.Second, Timeout: 30 * time.Second, FailureRatio: 0.6, MinRequests: 5},
	"analytics":    {MaxRequests: 3, Interval: 60 * time.Second, Timeout: 60 * time.Second, FailureRatio: 0.5, MinRequests: 5},
	"live":         {MaxRequests: 3, Interval: 30 * time.Second, Timeout: 45 * time.Second, FailureRatio: 0.5, MinRequests: 5},
	"ecommerce":    {MaxRequests: 5, Interval: 60 * time.Second, Timeout: 30 * time.Second, FailureRatio: 0.6, MinRequests: 10},
}

// CircuitBreakerManager manages gobreaker instances per downstream service.
type CircuitBreakerManager struct {
	mu       sync.RWMutex
	breakers map[string]*gobreaker.CircuitBreaker
	cfg      *config.CircuitBreakerConfig
}

// NewCircuitBreakerManager creates a manager and pre-wires breakers for all
// known services.
func NewCircuitBreakerManager(cfg *config.CircuitBreakerConfig) *CircuitBreakerManager {
	m := &CircuitBreakerManager{
		breakers: make(map[string]*gobreaker.CircuitBreaker),
		cfg:      cfg,
	}

	for service := range defaultThresholds {
		m.getOrCreate(service)
	}

	return m
}

// getOrCreate returns the circuit breaker for a service, creating it if needed.
func (m *CircuitBreakerManager) getOrCreate(service string) *gobreaker.CircuitBreaker {
	m.mu.RLock()
	cb, ok := m.breakers[service]
	m.mu.RUnlock()
	if ok {
		return cb
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock.
	if cb, ok = m.breakers[service]; ok {
		return cb
	}

	threshold, exists := defaultThresholds[service]
	if !exists {
		threshold = ServiceThreshold{
			MaxRequests:  m.cfg.MaxRequests,
			Interval:     m.cfg.Interval,
			Timeout:      m.cfg.Timeout,
			FailureRatio: m.cfg.FailureRatio,
			MinRequests:  m.cfg.MinRequests,
		}
	}

	settings := gobreaker.Settings{
		Name:        service,
		MaxRequests: threshold.MaxRequests,
		Interval:    threshold.Interval,
		Timeout:     threshold.Timeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			if counts.Requests < threshold.MinRequests {
				return false
			}
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return failureRatio >= threshold.FailureRatio
		},
		OnStateChange: func(name string, from, to gobreaker.State) {
			log.Printf("[circuit_breaker] service=%s state_change from=%s to=%s",
				name, from.String(), to.String())
		},
		IsSuccessful: func(err error) bool {
			if err == nil {
				return true
			}
			// Treat client errors (4xx) as successes from the circuit breaker's
			// perspective — the service is responding, just rejecting the request.
			var httpErr *HTTPError
			if errors.As(err, &httpErr) {
				return httpErr.StatusCode < 500
			}
			return false
		},
	}

	cb = gobreaker.NewCircuitBreaker(settings)
	m.breakers[service] = cb
	return cb
}

// Execute runs fn inside the circuit breaker for the named service.
// Returns gobreaker.ErrOpenState if the circuit is open.
func (m *CircuitBreakerManager) Execute(service string, fn func() (interface{}, error)) (interface{}, error) {
	cb := m.getOrCreate(service)
	return cb.Execute(fn)
}

// Middleware returns a gin.HandlerFunc that wraps downstream calls with a
// circuit breaker for the service named in the "X-Target-Service" context value.
func (m *CircuitBreakerManager) Middleware(service string) gin.HandlerFunc {
	return func(c *gin.Context) {
		cb := m.getOrCreate(service)

		// We use the circuit breaker to guard calling c.Next().
		// The response writer wrapper lets us detect 5xx responses.
		rw := newResponseWriter(c.Writer)
		c.Writer = rw
		c.Set("target_service", service)

		_, err := cb.Execute(func() (interface{}, error) {
			c.Next()

			// Treat 5xx status codes as failures.
			if rw.statusCode >= 500 {
				return nil, &HTTPError{
					StatusCode: rw.statusCode,
					Message:    fmt.Sprintf("upstream returned %d", rw.statusCode),
				}
			}
			return nil, nil
		})

		if err != nil {
			if errors.Is(err, gobreaker.ErrOpenState) {
				c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
					"error":   "service_unavailable",
					"message": fmt.Sprintf("service %q is temporarily unavailable; circuit is open", service),
					"service": service,
				})
				return
			}
			if errors.Is(err, gobreaker.ErrTooManyRequests) {
				c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
					"error":   "service_overloaded",
					"message": fmt.Sprintf("service %q is in recovery mode; try again shortly", service),
					"service": service,
				})
				return
			}
		}
	}
}

// State returns the current state of the named service's circuit breaker.
func (m *CircuitBreakerManager) State(service string) gobreaker.State {
	m.mu.RLock()
	cb, ok := m.breakers[service]
	m.mu.RUnlock()
	if !ok {
		return gobreaker.StateClosed
	}
	return cb.State()
}

// Counts returns current request counts for the named service's breaker.
func (m *CircuitBreakerManager) Counts(service string) gobreaker.Counts {
	m.mu.RLock()
	cb, ok := m.breakers[service]
	m.mu.RUnlock()
	if !ok {
		return gobreaker.Counts{}
	}
	return cb.Counts()
}

// AllStates returns a snapshot of every service's circuit breaker state.
func (m *CircuitBreakerManager) AllStates() map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]string, len(m.breakers))
	for name, cb := range m.breakers {
		result[name] = cb.State().String()
	}
	return result
}

// Reset forcefully resets the circuit breaker for a service (admin action).
// gobreaker does not expose a direct Reset; we recreate the breaker.
func (m *CircuitBreakerManager) Reset(service string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.breakers, service)
	log.Printf("[circuit_breaker] reset service=%s", service)
}

// HTTPError is used to communicate HTTP-layer errors through the circuit breaker.
type HTTPError struct {
	StatusCode int
	Message    string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Message)
}
