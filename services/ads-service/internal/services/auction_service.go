package services

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/tiktok-clone/ads-service/internal/config"
	"github.com/tiktok-clone/ads-service/internal/models"
)

// auctionCandidate holds an ad enriched with its auction metrics.
type auctionCandidate struct {
	Ad           *models.Ad
	AdSet        *models.AdSet
	PredictedCTR float64
	// eCPM = bid * predictedCTR * 1000  (all in micro-USD)
	eCPMMicroUSD int64
}

// AuctionWinner is the result of a successful auction.
type AuctionWinner struct {
	Ad             *models.Ad
	AdSet          *models.AdSet
	ImpressionID   string
	// ChargeMicroUSD is the second-price charge: second-highest eCPM + premium.
	ChargeMicroUSD int64
	// ECPMMicroUSD is the winner's effective CPM used for ranking.
	ECPMMicroUSD   int64
	PredictedCTR   float64
}

// AuctionService implements second-price (Vickrey) auction mechanics.
//
// Ranking: each eligible ad is scored by eCPM = bid * predictedCTR.
// The ad with the highest eCPM wins and is charged:
//   charge = second_highest_eCPM + PricePremiumMicroUSD
//
// If only one ad enters the auction it pays its own eCPM (floor price).
type AuctionService struct {
	cfg            *config.Config
	redis          *redis.Client
	campaignRepo   CampaignRepository
	targetingSvc   *TargetingService
	campaignSvc    *CampaignService
	logger         *zap.Logger
}

// NewAuctionService constructs the auction service.
func NewAuctionService(
	cfg *config.Config,
	repo CampaignRepository,
	redis *redis.Client,
	targetingSvc *TargetingService,
	campaignSvc *CampaignService,
	logger *zap.Logger,
) *AuctionService {
	return &AuctionService{
		cfg:          cfg,
		redis:        redis,
		campaignRepo: repo,
		targetingSvc: targetingSvc,
		campaignSvc:  campaignSvc,
		logger:       logger,
	}
}

// RunAuction selects the winning ad for a single ad request.
//
// Pipeline:
//  1. Fetch candidate ads from the repository (pre-filtered by placement).
//  2. Load ad sets for targeting metadata.
//  3. Build the user profile from the request context.
//  4. Apply targeting + frequency-cap filters via TargetingService.
//  5. Score each eligible ad: eCPM = bid * predictedCTR.
//  6. Sort descending by eCPM; winner = index 0.
//  7. Charge = eCPM[1] + premium (second-price); fall back to eCPM[0] if only one.
//  8. Persist the impression and mark the ad as seen for this user.
func (s *AuctionService) RunAuction(ctx context.Context, req *models.AdRequest) (*AuctionWinner, error) {
	if req.UserID == "" {
		return nil, fmt.Errorf("auction: user_id is required")
	}
	if req.Placement == "" {
		req.Placement = "in_feed"
	}

	// 1. Fetch candidate active ads for this placement.
	candidates, err := s.campaignRepo.GetActiveAdsByCriteria(ctx, req.Placement, s.cfg.Auction.MaxCandidates)
	if err != nil {
		return nil, fmt.Errorf("auction: fetch candidates: %w", err)
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("auction: no active ads for placement %q", req.Placement)
	}

	// 2. Load ad sets for all candidates.
	adSets, err := s.loadAdSets(ctx, candidates)
	if err != nil {
		return nil, fmt.Errorf("auction: load ad sets: %w", err)
	}

	// 3. Build user profile.
	profile, err := s.targetingSvc.GetUserProfile(ctx, req.UserID, req)
	if err != nil {
		return nil, fmt.Errorf("auction: get user profile: %w", err)
	}

	// 4. Filter by targeting + frequency cap.
	eligible, err := s.targetingSvc.FilterEligibleAds(ctx, candidates, adSets, profile, s.campaignSvc)
	if err != nil {
		return nil, fmt.Errorf("auction: filter eligible ads: %w", err)
	}
	if len(eligible) == 0 {
		return nil, fmt.Errorf("auction: no eligible ads after targeting for user %s", req.UserID)
	}

	// 5. Score eligible ads.
	scored := make([]auctionCandidate, 0, len(eligible))
	for _, ad := range eligible {
		adSet, ok := adSets[ad.AdSetID]
		if !ok {
			continue
		}
		if adSet.BidAmountMicroUSD < s.cfg.Auction.MinBidMicroUSD {
			continue // below floor price
		}
		ctr := s.predictCTR(ctx, ad)
		eCPM := s.computeECPM(adSet.BidAmountMicroUSD, ctr)
		scored = append(scored, auctionCandidate{
			Ad:           ad,
			AdSet:        adSet,
			PredictedCTR: ctr,
			eCPMMicroUSD: eCPM,
		})
	}
	if len(scored) == 0 {
		return nil, fmt.Errorf("auction: all eligible ads below floor price")
	}

	// 6. Sort by eCPM descending.
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].eCPMMicroUSD > scored[j].eCPMMicroUSD
	})

	winner := scored[0]

	// 7. Second-price charge.
	var chargeMicroUSD int64
	if len(scored) > 1 {
		// Second price = runner-up eCPM + premium.
		chargeMicroUSD = scored[1].eCPMMicroUSD + s.cfg.Auction.PricePremiumMicroUSD
	} else {
		// Only one bidder — charge their own eCPM (floor).
		chargeMicroUSD = winner.eCPMMicroUSD + s.cfg.Auction.PricePremiumMicroUSD
	}
	// Never charge more than the winner bid.
	if chargeMicroUSD > winner.eCPMMicroUSD {
		chargeMicroUSD = winner.eCPMMicroUSD
	}
	// Enforce floor.
	if chargeMicroUSD < s.cfg.Auction.MinBidMicroUSD {
		chargeMicroUSD = s.cfg.Auction.MinBidMicroUSD
	}

	impressionID := uuid.NewString()

	// 8. Persist impression and update spend (best-effort).
	imp := &models.Impression{
		ID:             impressionID,
		AdID:           winner.Ad.ID,
		AdSetID:        winner.AdSet.ID,
		CampaignID:     winner.AdSet.CampaignID,
		UserID:         req.UserID,
		BidMicroUSD:    winner.AdSet.BidAmountMicroUSD,
		ChargeMicroUSD: chargeMicroUSD,
		PredictedCTR:   winner.PredictedCTR,
		ECPMMicroUSD:   winner.eCPMMicroUSD,
		Placement:      req.Placement,
		DeviceType:     req.DeviceType,
		Country:        req.Country,
		Platform:       req.Platform,
		ImpressionedAt: time.Now().UTC(),
	}
	if err := s.campaignSvc.RecordImpression(ctx, imp); err != nil {
		s.logger.Warn("auction: record impression failed", zap.Error(err), zap.String("ad_id", winner.Ad.ID))
	}

	// Mark ad as seen so it's excluded from future auctions for this user
	// within the seen-ads window.
	if err := s.targetingSvc.MarkAdSeen(ctx, req.UserID, winner.Ad.ID); err != nil {
		s.logger.Warn("auction: mark ad seen failed", zap.Error(err))
	}

	s.logger.Info("auction complete",
		zap.String("impression_id", impressionID),
		zap.String("winning_ad_id", winner.Ad.ID),
		zap.String("user_id", req.UserID),
		zap.Int64("ecpm_micro_usd", winner.eCPMMicroUSD),
		zap.Int64("charge_micro_usd", chargeMicroUSD),
		zap.Int("candidates", len(candidates)),
		zap.Int("eligible", len(eligible)),
		zap.Int("scored", len(scored)),
	)

	return &AuctionWinner{
		Ad:             winner.Ad,
		AdSet:          winner.AdSet,
		ImpressionID:   impressionID,
		ChargeMicroUSD: chargeMicroUSD,
		ECPMMicroUSD:   winner.eCPMMicroUSD,
		PredictedCTR:   winner.PredictedCTR,
	}, nil
}

// computeECPM calculates effective CPM: eCPM = bid_microUSD * predictedCTR * 1000.
// The *1000 converts from per-impression to per-mille.
// All values stay in micro-USD to avoid floating-point precision loss.
func (s *AuctionService) computeECPM(bidMicroUSD int64, predictedCTR float64) int64 {
	return int64(float64(bidMicroUSD) * predictedCTR * 1000)
}

// predictCTR returns the predicted click-through rate for an ad.
// It checks a short-lived Redis cache; on miss it falls back to the ad's
// historical CTR or the configured default.
func (s *AuctionService) predictCTR(ctx context.Context, ad *models.Ad) float64 {
	cacheKey := fmt.Sprintf("ctr:ad:%s", ad.ID)
	if cached, err := s.redis.Get(ctx, cacheKey).Float64(); err == nil {
		return cached
	}

	// Use the ad's historical CTR when it has enough data (>= 1000 impressions).
	if ad.Impressions >= 1000 && ad.CTR > 0 {
		ttl := time.Duration(s.cfg.Auction.CTRCacheTTLSec) * time.Second
		_ = s.redis.Set(ctx, cacheKey, ad.CTR, ttl).Err()
		return ad.CTR
	}

	// Fall back to the configured default CTR.
	return s.cfg.Auction.DefaultCTR
}

// loadAdSets fetches the ad sets for a slice of candidate ads, using a single
// batch-style approach (one call per unique ad-set ID found).
func (s *AuctionService) loadAdSets(ctx context.Context, ads []*models.Ad) (map[string]*models.AdSet, error) {
	// Collect unique ad-set IDs.
	seen := make(map[string]struct{}, len(ads))
	for _, ad := range ads {
		seen[ad.AdSetID] = struct{}{}
	}

	result := make(map[string]*models.AdSet, len(seen))
	for adSetID := range seen {
		// Try Redis cache first.
		cacheKey := fmt.Sprintf("adset:%s", adSetID)
		if cached, err := s.redis.Get(ctx, cacheKey).Bytes(); err == nil {
			var adSet models.AdSet
			if json.Unmarshal(cached, &adSet) == nil {
				result[adSetID] = &adSet
				continue
			}
		}

		adSet, err := s.campaignRepo.GetAdSet(ctx, adSetID)
		if err != nil {
			s.logger.Warn("auction: could not load ad set",
				zap.String("ad_set_id", adSetID),
				zap.Error(err),
			)
			continue
		}
		result[adSetID] = adSet

		// Cache for a short period.
		if data, err := json.Marshal(adSet); err == nil {
			_ = s.redis.Set(ctx, cacheKey, data, 5*time.Minute).Err()
		}
	}

	return result, nil
}
