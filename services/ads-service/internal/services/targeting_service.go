package services

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/tiktok-clone/ads-service/internal/config"
	"github.com/tiktok-clone/ads-service/internal/models"
)

// UserProfile holds the targeting attributes we know about a given user.
// In production this is populated by the user-service and interest-engine.
type UserProfile struct {
	UserID      string   `json:"user_id"`
	Age         int      `json:"age"`
	Gender      string   `json:"gender"` // male | female | other
	Country     string   `json:"country"`
	City        string   `json:"city"`
	Region      string   `json:"region"`
	Language    string   `json:"language"`
	DeviceType  string   `json:"device_type"` // ios | android | web
	OSVersion   string   `json:"os_version"`
	Connection  string   `json:"connection_type"` // wifi | cellular
	InterestIDs []string `json:"interest_ids"`
	// Segment memberships: set of audience-segment IDs this user belongs to.
	SegmentIDs []string `json:"segment_ids"`
	// FeatureVector is a dense embedding used for lookalike matching (128-dim).
	FeatureVector []float64 `json:"feature_vector,omitempty"`
	// SeenAdIDs is the set of ad IDs shown to this user in the last SeenAdsWindowDays.
	SeenAdIDs []string `json:"seen_ad_ids,omitempty"`
}

// TargetingService matches ad sets to users based on demographic, interest,
// lookalike, and exclusion criteria.
type TargetingService struct {
	cfg    *config.Config
	redis  *redis.Client
	logger *zap.Logger
}

// NewTargetingService constructs the targeting service.
func NewTargetingService(cfg *config.Config, rdb *redis.Client, logger *zap.Logger) *TargetingService {
	return &TargetingService{cfg: cfg, redis: rdb, logger: logger}
}

// GetUserProfile fetches the user profile from Redis cache, falling back to a
// stub implementation. In production this calls the user-profile gRPC service.
func (s *TargetingService) GetUserProfile(ctx context.Context, userID string, req *models.AdRequest) (*UserProfile, error) {
	cacheKey := fmt.Sprintf("user:profile:targeting:%s", userID)

	// Try Redis cache first.
	cached, err := s.redis.Get(ctx, cacheKey).Bytes()
	if err == nil {
		var profile UserProfile
		if json.Unmarshal(cached, &profile) == nil {
			return &profile, nil
		}
	}

	// Build a profile from the ad request context (partial — enriched by user service in prod).
	profile := &UserProfile{
		UserID:     userID,
		Country:    req.Country,
		Language:   req.Language,
		DeviceType: req.DeviceType,
		OSVersion:  req.OSVersion,
		Connection: req.ConnectionType,
	}

	// Cache with TTL.
	if data, err := json.Marshal(profile); err == nil {
		ttl := time.Duration(s.cfg.Targeting.UserProfileCacheTTLSec) * time.Second
		_ = s.redis.Set(ctx, cacheKey, data, ttl).Err()
	}

	return profile, nil
}

// MatchesTargeting returns true when the given user profile satisfies the
// ad set's targeting specification. All specified criteria must match (AND logic).
// Within a multi-value criterion (e.g. countries list) OR logic applies.
func (s *TargetingService) MatchesTargeting(profile *UserProfile, spec models.TargetingSpec) bool {
	// --- Age targeting ---
	if spec.MinAge > 0 && profile.Age > 0 && profile.Age < spec.MinAge {
		return false
	}
	if spec.MaxAge > 0 && profile.Age > 0 && profile.Age > spec.MaxAge {
		return false
	}

	// --- Gender targeting ---
	if len(spec.Genders) > 0 {
		matched := false
		for _, g := range spec.Genders {
			if string(g) == string(models.GenderAll) || string(g) == profile.Gender {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// --- Geographic targeting ---
	if len(spec.Countries) > 0 && profile.Country != "" {
		if !containsString(spec.Countries, profile.Country) {
			return false
		}
	}
	if len(spec.Cities) > 0 && profile.City != "" {
		if !containsString(spec.Cities, profile.City) {
			return false
		}
	}
	if len(spec.Regions) > 0 && profile.Region != "" {
		if !containsString(spec.Regions, profile.Region) {
			return false
		}
	}

	// --- Language targeting ---
	if len(spec.Languages) > 0 && profile.Language != "" {
		if !containsString(spec.Languages, profile.Language) {
			return false
		}
	}

	// --- Device targeting ---
	if len(spec.DeviceTypes) > 0 && profile.DeviceType != "" {
		if !containsString(spec.DeviceTypes, profile.DeviceType) {
			return false
		}
	}
	if len(spec.ConnectionTypes) > 0 && profile.Connection != "" {
		if !containsString(spec.ConnectionTypes, profile.Connection) {
			return false
		}
	}

	// --- Interest targeting ---
	if len(spec.IncludeInterests) > 0 {
		if !hasAnyInterest(profile.InterestIDs, spec.IncludeInterests) {
			return false
		}
	}
	if len(spec.ExcludeInterests) > 0 {
		if hasAnyInterest(profile.InterestIDs, spec.ExcludeInterests) {
			return false
		}
	}

	// --- Custom audience targeting ---
	if len(spec.CustomAudienceIDs) > 0 {
		if !hasAnySegment(profile.SegmentIDs, spec.CustomAudienceIDs) {
			return false
		}
	}

	// --- Retargeting audience ---
	if spec.RetargetingAudienceID != "" {
		if !containsString(profile.SegmentIDs, spec.RetargetingAudienceID) {
			return false
		}
	}

	// --- Lookalike audience ---
	if spec.LookalikeAudienceID != "" {
		// Lookalike match is checked separately via LookalikeScore; here we
		// verify the user is tagged as a lookalike member.
		if !containsString(profile.SegmentIDs, "lookalike:"+spec.LookalikeAudienceID) {
			return false
		}
	}

	return true
}

// ExcludeAlreadySeen returns true when the ad has already been shown to this
// user within the configured seen-ads window, meaning the ad should be skipped.
func (s *TargetingService) ExcludeAlreadySeen(ctx context.Context, userID, adID string) (bool, error) {
	key := fmt.Sprintf("user:seen_ads:%s", userID)
	// SISMEMBER is O(1) for Redis sets.
	isMember, err := s.redis.SIsMember(ctx, key, adID).Result()
	if err != nil {
		// On Redis failure, default to showing the ad (don't penalise delivery).
		s.logger.Warn("seen-ads redis check failed", zap.String("user_id", userID), zap.Error(err))
		return false, nil
	}
	return isMember, nil
}

// MarkAdSeen records that a user has seen an ad, with a TTL of SeenAdsWindowDays.
func (s *TargetingService) MarkAdSeen(ctx context.Context, userID, adID string) error {
	key := fmt.Sprintf("user:seen_ads:%s", userID)
	pipe := s.redis.Pipeline()
	pipe.SAdd(ctx, key, adID)
	ttl := time.Duration(s.cfg.Targeting.SeenAdsWindowDays) * 24 * time.Hour
	pipe.Expire(ctx, key, ttl)
	_, err := pipe.Exec(ctx)
	return err
}

// LookalikeScore computes the cosine similarity between a user's feature vector
// and the seed audience's centroid vector. Returns a score in [0, 1].
// A higher score means the user is more similar to the seed audience.
func (s *TargetingService) LookalikeScore(userVector, seedCentroid []float64) float64 {
	if len(userVector) == 0 || len(seedCentroid) == 0 {
		return 0
	}
	if len(userVector) != len(seedCentroid) {
		return 0
	}

	var dot, normA, normB float64
	for i := range userVector {
		dot += userVector[i] * seedCentroid[i]
		normA += userVector[i] * userVector[i]
		normB += seedCentroid[i] * seedCentroid[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

// FilterEligibleAds applies targeting and frequency-cap checks to a candidate
// ad list and returns only the ads eligible for the given user in this request.
func (s *TargetingService) FilterEligibleAds(
	ctx context.Context,
	candidates []*models.Ad,
	adSets map[string]*models.AdSet,
	profile *UserProfile,
	campaignSvc *CampaignService,
) ([]*models.Ad, error) {
	eligible := make([]*models.Ad, 0, len(candidates))

	for _, ad := range candidates {
		adSet, ok := adSets[ad.AdSetID]
		if !ok {
			continue
		}

		// 1. Targeting match.
		if !s.MatchesTargeting(profile, adSet.Targeting) {
			continue
		}

		// 2. Exclude already-seen ads.
		seen, err := s.ExcludeAlreadySeen(ctx, profile.UserID, ad.ID)
		if err != nil {
			s.logger.Warn("seen-ads check error", zap.String("ad_id", ad.ID), zap.Error(err))
		}
		if seen {
			continue
		}

		// 3. Frequency cap check (max 3 impressions per user per ad set per day).
		maxPerDay := adSet.FrequencyCapPerDay
		if maxPerDay <= 0 {
			maxPerDay = 3
		}
		allowed, _, err := campaignSvc.FrequencyCapping(ctx, profile.UserID, adSet.ID, maxPerDay)
		if err != nil {
			s.logger.Warn("freq cap check error", zap.String("ad_set_id", adSet.ID), zap.Error(err))
		}
		if !allowed {
			continue
		}

		eligible = append(eligible, ad)
	}

	return eligible, nil
}

// BuildInterestIndex creates a fast-lookup set from a slice of interest IDs.
func BuildInterestIndex(ids []string) map[string]struct{} {
	m := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		m[strings.ToLower(id)] = struct{}{}
	}
	return m
}

// ---- helpers ---------------------------------------------------------------

func containsString(slice []string, val string) bool {
	lower := strings.ToLower(val)
	for _, s := range slice {
		if strings.ToLower(s) == lower {
			return true
		}
	}
	return false
}

func hasAnyInterest(userInterests, targetInterests []string) bool {
	idx := BuildInterestIndex(userInterests)
	for _, ti := range targetInterests {
		if _, ok := idx[strings.ToLower(ti)]; ok {
			return true
		}
	}
	return false
}

func hasAnySegment(userSegments, targetSegments []string) bool {
	idx := make(map[string]struct{}, len(userSegments))
	for _, s := range userSegments {
		idx[s] = struct{}{}
	}
	for _, ts := range targetSegments {
		if _, ok := idx[ts]; ok {
			return true
		}
	}
	return false
}
