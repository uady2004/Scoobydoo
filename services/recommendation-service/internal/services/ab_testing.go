package services

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/tiktok-clone/recommendation-service/internal/config"
)

// ExperimentStatus indicates whether an experiment is currently running.
type ExperimentStatus string

const (
	ExperimentStatusActive   ExperimentStatus = "active"
	ExperimentStatusPaused   ExperimentStatus = "paused"
	ExperimentStatusFinished ExperimentStatus = "finished"
)

// Variant defines a single arm of an A/B experiment.
type Variant struct {
	// ID is a stable identifier such as "control" or "treatment_v2".
	ID string `json:"id"`
	// TrafficPercent is the share of eligible users assigned to this variant [0, 100].
	TrafficPercent float64 `json:"traffic_percent"`
	// Config carries arbitrary variant-specific configuration overrides.
	Config map[string]interface{} `json:"config,omitempty"`
}

// Experiment describes a single A/B test.
type Experiment struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Status      ExperimentStatus `json:"status"`
	Variants    []Variant        `json:"variants"`
	// StartTime and EndTime bound when the experiment is eligible.
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	// EligibleCountries is an allow-list of ISO-3166-1 alpha-2 codes.
	// An empty slice means all countries are eligible.
	EligibleCountries []string `json:"eligible_countries,omitempty"`
}

// Assignment records which variant a user was assigned to for a given experiment.
type Assignment struct {
	ExperimentID string    `json:"experiment_id"`
	VariantID    string    `json:"variant_id"`
	AssignedAt   time.Time `json:"assigned_at"`
}

// ABTestingService manages experiment configuration, user assignment, and
// impression/conversion tracking.
//
// Assignment is performed via consistent hashing so that the same user always
// lands in the same variant bucket (for the same experiment salt), regardless
// of the number of active experiments.
type ABTestingService struct {
	cfg         *config.ABTestingConfig
	rdb         redis.UniversalClient
	logger      *zap.Logger
	experiments []*Experiment
	mu          sync.RWMutex
	stopCh      chan struct{}
}

// NewABTestingService constructs an ABTestingService and starts the background
// experiment-config refresh loop.
func NewABTestingService(
	cfg *config.ABTestingConfig,
	rdb redis.UniversalClient,
	logger *zap.Logger,
) *ABTestingService {
	svc := &ABTestingService{
		cfg:    cfg,
		rdb:    rdb,
		logger: logger,
		stopCh: make(chan struct{}),
	}
	go svc.refreshLoop()
	return svc
}

// Stop terminates the background refresh goroutine.
func (svc *ABTestingService) Stop() {
	close(svc.stopCh)
}

// -----------------------------------------------------------------
// Experiment assignment
// -----------------------------------------------------------------

// AssignVariant returns the variant assigned to a user for the first active
// experiment the user is eligible for.  If no active experiment applies,
// ("", nil) is returned.
//
// The assignment uses consistent hashing:
//
//	hash = SHA-256(experimentID + ":" + userID)
//	bucket = hash[0:8] as uint64 mod 10000   → value in [0, 9999]
//
// Variants are mapped to contiguous bucket ranges based on TrafficPercent.
func (svc *ABTestingService) AssignVariant(
	ctx context.Context,
	userID string,
	countryCode string,
) *Assignment {
	svc.mu.RLock()
	experiments := svc.experiments
	svc.mu.RUnlock()

	now := time.Now()

	for _, exp := range experiments {
		if exp.Status != ExperimentStatusActive {
			continue
		}
		if now.Before(exp.StartTime) || now.After(exp.EndTime) {
			continue
		}
		if !svc.isEligibleCountry(exp, countryCode) {
			continue
		}

		variantID := svc.consistentHash(exp.ID, userID, exp.Variants)
		if variantID == "" {
			continue
		}

		assignment := &Assignment{
			ExperimentID: exp.ID,
			VariantID:    variantID,
			AssignedAt:   now,
		}
		// Persist assignment to Redis so it can be replayed in analytics.
		if svc.cfg.TrackingEnabled {
			svc.persistAssignment(ctx, userID, assignment)
		}
		return assignment
	}
	return nil
}

// GetAssignment retrieves a previously stored assignment from Redis.
// Returns nil if no assignment is found.
func (svc *ABTestingService) GetAssignment(
	ctx context.Context,
	userID, experimentID string,
) *Assignment {
	key := abAssignmentKey(userID, experimentID)
	data, err := svc.rdb.Get(ctx, key).Result()
	if err != nil {
		return nil
	}
	var a Assignment
	if json.Unmarshal([]byte(data), &a) != nil {
		return nil
	}
	return &a
}

// TrackImpression increments the impression counter for a variant.
func (svc *ABTestingService) TrackImpression(
	ctx context.Context,
	experimentID, variantID string,
) {
	if !svc.cfg.TrackingEnabled {
		return
	}
	key := abMetricKey(experimentID, variantID, "impressions")
	if err := svc.rdb.Incr(ctx, key).Err(); err != nil {
		svc.logger.Warn("track impression failed",
			zap.String("experiment_id", experimentID),
			zap.String("variant_id", variantID),
			zap.Error(err))
	}
}

// TrackConversion records a conversion event (e.g., like, share) for a variant.
func (svc *ABTestingService) TrackConversion(
	ctx context.Context,
	experimentID, variantID, eventType string,
) {
	if !svc.cfg.TrackingEnabled {
		return
	}
	key := abMetricKey(experimentID, variantID, eventType)
	if err := svc.rdb.Incr(ctx, key).Err(); err != nil {
		svc.logger.Warn("track conversion failed",
			zap.String("experiment_id", experimentID),
			zap.String("variant_id", variantID),
			zap.String("event_type", eventType),
			zap.Error(err))
	}
}

// GetMetrics returns the current impression and conversion counts for every
// variant of an experiment.  Returns a map of variantID → metricName → count.
func (svc *ABTestingService) GetMetrics(
	ctx context.Context,
	experimentID string,
) (map[string]map[string]int64, error) {
	svc.mu.RLock()
	var exp *Experiment
	for _, e := range svc.experiments {
		if e.ID == experimentID {
			exp = e
			break
		}
	}
	svc.mu.RUnlock()

	if exp == nil {
		return nil, fmt.Errorf("experiment %s not found", experimentID)
	}

	result := make(map[string]map[string]int64, len(exp.Variants))
	for _, v := range exp.Variants {
		metrics := map[string]int64{}
		for _, metricName := range []string{"impressions", "likes", "shares", "follows", "comments"} {
			key := abMetricKey(experimentID, v.ID, metricName)
			val, err := svc.rdb.Get(ctx, key).Int64()
			if err != nil && err != redis.Nil {
				svc.logger.Warn("get metric failed", zap.Error(err))
			}
			metrics[metricName] = val
		}
		result[v.ID] = metrics
	}
	return result, nil
}

// -----------------------------------------------------------------
// Experiment config management
// -----------------------------------------------------------------

// UpsertExperiment writes an experiment definition to Redis and refreshes the
// in-memory cache.
func (svc *ABTestingService) UpsertExperiment(ctx context.Context, exp *Experiment) error {
	if err := validateExperiment(exp); err != nil {
		return err
	}
	data, err := json.Marshal(exp)
	if err != nil {
		return fmt.Errorf("marshal experiment: %w", err)
	}
	// Store individual experiment.
	expKey := abExperimentKey(exp.ID)
	if err := svc.rdb.Set(ctx, expKey, data, 0).Err(); err != nil {
		return fmt.Errorf("set experiment: %w", err)
	}
	// Add to the active experiments index.
	if err := svc.rdb.SAdd(ctx, svc.cfg.ExperimentsKey, exp.ID).Err(); err != nil {
		return fmt.Errorf("add experiment to index: %w", err)
	}
	// Invalidate in-memory cache.
	return svc.loadExperiments(ctx)
}

// loadExperiments fetches all experiments from Redis into memory.
func (svc *ABTestingService) loadExperiments(ctx context.Context) error {
	ids, err := svc.rdb.SMembers(ctx, svc.cfg.ExperimentsKey).Result()
	if err != nil {
		if err == redis.Nil {
			return nil
		}
		return fmt.Errorf("load experiment IDs: %w", err)
	}

	if len(ids) == 0 {
		svc.mu.Lock()
		svc.experiments = nil
		svc.mu.Unlock()
		return nil
	}

	pipe := svc.rdb.Pipeline()
	cmds := make(map[string]*redis.StringCmd, len(ids))
	for _, id := range ids {
		cmds[id] = pipe.Get(ctx, abExperimentKey(id))
	}
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return fmt.Errorf("load experiments pipeline: %w", err)
	}

	experiments := make([]*Experiment, 0, len(ids))
	for id, cmd := range cmds {
		data, err := cmd.Result()
		if err != nil {
			svc.logger.Warn("load experiment failed",
				zap.String("experiment_id", id), zap.Error(err))
			continue
		}
		var exp Experiment
		if err := json.Unmarshal([]byte(data), &exp); err != nil {
			svc.logger.Warn("unmarshal experiment failed",
				zap.String("experiment_id", id), zap.Error(err))
			continue
		}
		experiments = append(experiments, &exp)
	}

	svc.mu.Lock()
	svc.experiments = experiments
	svc.mu.Unlock()
	return nil
}

// refreshLoop periodically reloads experiment configuration from Redis.
func (svc *ABTestingService) refreshLoop() {
	ticker := time.NewTicker(svc.cfg.RefreshInterval)
	defer ticker.Stop()

	// Initial load.
	ctx := context.Background()
	if err := svc.loadExperiments(ctx); err != nil {
		svc.logger.Error("initial experiment load failed", zap.Error(err))
	}

	for {
		select {
		case <-svc.stopCh:
			return
		case <-ticker.C:
			if err := svc.loadExperiments(ctx); err != nil {
				svc.logger.Warn("experiment refresh failed", zap.Error(err))
			}
		}
	}
}

// -----------------------------------------------------------------
// Internal helpers
// -----------------------------------------------------------------

// consistentHash deterministically maps a userID to one of the provided
// variants using SHA-256 and bucket arithmetic.
func (svc *ABTestingService) consistentHash(
	experimentID, userID string,
	variants []Variant,
) string {
	if len(variants) == 0 {
		return ""
	}

	// Compute hash.
	h := sha256.Sum256([]byte(experimentID + ":" + userID))
	// Take the first 8 bytes as a uint64.
	bucketValue := binary.BigEndian.Uint64(h[:8]) % 10000 // [0, 9999]

	// Walk the variant list; assign bucket ranges sequentially.
	var cumulative float64
	for _, v := range variants {
		cumulative += v.TrafficPercent
		threshold := uint64(cumulative * 100) // [0, 10000]
		if bucketValue < threshold {
			return v.ID
		}
	}
	// If total traffic < 100%, the user is not in any variant (hold-out).
	return ""
}

func (svc *ABTestingService) isEligibleCountry(exp *Experiment, countryCode string) bool {
	if len(exp.EligibleCountries) == 0 {
		return true
	}
	for _, c := range exp.EligibleCountries {
		if c == countryCode {
			return true
		}
	}
	return false
}

func (svc *ABTestingService) persistAssignment(
	ctx context.Context,
	userID string,
	a *Assignment,
) {
	key := abAssignmentKey(userID, a.ExperimentID)
	data, err := json.Marshal(a)
	if err != nil {
		return
	}
	// Store for 90 days for analytics replay.
	if err := svc.rdb.Set(ctx, key, data, 90*24*time.Hour).Err(); err != nil {
		svc.logger.Warn("persist assignment failed",
			zap.String("user_id", userID),
			zap.String("experiment_id", a.ExperimentID),
			zap.Error(err))
	}
}

func validateExperiment(exp *Experiment) error {
	if exp.ID == "" {
		return fmt.Errorf("experiment ID is required")
	}
	if len(exp.Variants) == 0 {
		return fmt.Errorf("experiment must have at least one variant")
	}
	total := 0.0
	for _, v := range exp.Variants {
		if v.TrafficPercent < 0 || v.TrafficPercent > 100 {
			return fmt.Errorf("variant %s traffic_percent must be in [0, 100]", v.ID)
		}
		total += v.TrafficPercent
	}
	if total > 100.01 {
		return fmt.Errorf("variant traffic percentages sum to %.2f, must be ≤ 100", total)
	}
	if exp.EndTime.Before(exp.StartTime) {
		return fmt.Errorf("end_time must be after start_time")
	}
	return nil
}

// -----------------------------------------------------------------
// Redis key helpers
// -----------------------------------------------------------------

func abExperimentKey(experimentID string) string {
	return fmt.Sprintf("rec:ab:exp:%s", experimentID)
}

func abAssignmentKey(userID, experimentID string) string {
	return fmt.Sprintf("rec:ab:assign:%s:%s", userID, experimentID)
}

func abMetricKey(experimentID, variantID, metricName string) string {
	return fmt.Sprintf("rec:ab:metrics:%s:%s:%s", experimentID, variantID, metricName)
}
