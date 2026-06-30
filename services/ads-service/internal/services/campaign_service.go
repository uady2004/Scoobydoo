package services

import (
	"context"
	"database/sql"
	encodingJSON "encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/tiktok-clone/ads-service/internal/config"
	"github.com/tiktok-clone/ads-service/internal/models"
)

// Sentinel errors.
var (
	ErrCampaignNotFound  = errors.New("campaign not found")
	ErrAdSetNotFound     = errors.New("ad set not found")
	ErrAdNotFound        = errors.New("ad not found")
	ErrBudgetNotFound    = errors.New("budget not found")
	ErrBudgetExhausted   = errors.New("campaign daily budget exhausted")
	ErrInvalidDateRange  = errors.New("end_time must be after start_time")
	ErrInvalidBidAmount  = errors.New("bid amount below minimum")
	ErrCampaignNotPaused = errors.New("campaign must be paused before editing")
)

// CampaignRepository defines persistence operations for campaigns.
type CampaignRepository interface {
	// Campaigns
	CreateCampaign(ctx context.Context, c *models.Campaign) error
	GetCampaign(ctx context.Context, id string) (*models.Campaign, error)
	UpdateCampaign(ctx context.Context, c *models.Campaign) error
	ListCampaigns(ctx context.Context, advertiserID string, status models.CampaignStatus, limit, offset int) ([]*models.Campaign, int64, error)

	// Ad Sets
	CreateAdSet(ctx context.Context, s *models.AdSet) error
	GetAdSet(ctx context.Context, id string) (*models.AdSet, error)
	UpdateAdSet(ctx context.Context, s *models.AdSet) error
	ListAdSetsByCampaign(ctx context.Context, campaignID string) ([]*models.AdSet, error)

	// Ads
	CreateAd(ctx context.Context, a *models.Ad) error
	GetAd(ctx context.Context, id string) (*models.Ad, error)
	UpdateAd(ctx context.Context, a *models.Ad) error
	ListAdsByAdSet(ctx context.Context, adSetID string) ([]*models.Ad, error)

	// Budget
	CreateBudget(ctx context.Context, b *models.Budget) error
	GetBudget(ctx context.Context, id string) (*models.Budget, error)
	GetBudgetByCampaign(ctx context.Context, campaignID string) (*models.Budget, error)
	UpdateBudget(ctx context.Context, b *models.Budget) error

	// Stats
	GetCampaignStats(ctx context.Context, campaignID string, start, end time.Time) (*models.CampaignStats, error)
	IncrementImpressions(ctx context.Context, adID, adSetID, campaignID string, spendMicroUSD int64) error
	IncrementClicks(ctx context.Context, adID, adSetID, campaignID string) error

	// Active ad retrieval for auction.
	GetActiveAdsByCriteria(ctx context.Context, placement string, limit int) ([]*models.Ad, error)
}

// CampaignService manages the full lifecycle of campaigns, ad sets, ads, and budgets.
type CampaignService struct {
	cfg    *config.Config
	repo   CampaignRepository
	redis  *redis.Client
	logger *zap.Logger
}

// NewCampaignService constructs the service.
func NewCampaignService(cfg *config.Config, repo CampaignRepository, rdb *redis.Client, logger *zap.Logger) *CampaignService {
	return &CampaignService{cfg: cfg, repo: repo, redis: rdb, logger: logger}
}

// CreateCampaign validates and persists a new campaign with its initial budget.
func (s *CampaignService) CreateCampaign(ctx context.Context, req *CreateCampaignRequest) (*models.Campaign, error) {
	if req.EndTime != nil && !req.EndTime.After(req.StartTime) {
		return nil, ErrInvalidDateRange
	}

	budgetID := uuid.NewString()
	budget := &models.Budget{
		ID:                     budgetID,
		CampaignID:             "", // filled after campaign is created
		AdvertiserID:           req.AdvertiserID,
		LifetimeBudgetMicroUSD: req.LifetimeBudgetMicroUSD,
		DailyBudgetMicroUSD:    req.DailyBudgetMicroUSD,
		CycleStartDate:         time.Now().UTC().Truncate(24 * time.Hour),
		CycleEndDate:           time.Now().UTC().Truncate(24 * time.Hour).Add(time.Duration(s.cfg.Budget.BillingCycleDays) * 24 * time.Hour),
		CreatedAt:              time.Now().UTC(),
		UpdatedAt:              time.Now().UTC(),
	}
	// Compute initial hourly budget.
	budget.HourlyBudgetMicroUSD = s.computeHourlyBudget(budget.DailyBudgetMicroUSD)

	campaign := &models.Campaign{
		ID:           uuid.NewString(),
		AdvertiserID: req.AdvertiserID,
		Name:         req.Name,
		Objective:    req.Objective,
		Status:       models.CampaignStatusReview,
		StartTime:    req.StartTime,
		EndTime:      req.EndTime,
		BudgetID:     budgetID,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	if err := s.repo.CreateCampaign(ctx, campaign); err != nil {
		return nil, fmt.Errorf("campaign_service: create campaign: %w", err)
	}

	budget.CampaignID = campaign.ID
	if err := s.repo.CreateBudget(ctx, budget); err != nil {
		return nil, fmt.Errorf("campaign_service: create budget: %w", err)
	}

	s.logger.Info("campaign created",
		zap.String("campaign_id", campaign.ID),
		zap.String("advertiser_id", req.AdvertiserID),
	)
	return campaign, nil
}

// UpdateCampaign updates mutable fields on an existing campaign.
func (s *CampaignService) UpdateCampaign(ctx context.Context, campaignID string, req *UpdateCampaignRequest) (*models.Campaign, error) {
	campaign, err := s.repo.GetCampaign(ctx, campaignID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrCampaignNotFound
		}
		return nil, fmt.Errorf("campaign_service: get campaign: %w", err)
	}

	// Only paused or draft campaigns can have their core settings changed.
	if req.Name != "" {
		campaign.Name = req.Name
	}
	if req.EndTime != nil {
		if !req.EndTime.After(campaign.StartTime) {
			return nil, ErrInvalidDateRange
		}
		campaign.EndTime = req.EndTime
	}
	campaign.UpdatedAt = time.Now().UTC()

	if err := s.repo.UpdateCampaign(ctx, campaign); err != nil {
		return nil, fmt.Errorf("campaign_service: update campaign: %w", err)
	}

	// Update budget if provided.
	if req.DailyBudgetMicroUSD > 0 || req.LifetimeBudgetMicroUSD > 0 {
		budget, err := s.repo.GetBudgetByCampaign(ctx, campaignID)
		if err == nil {
			if req.DailyBudgetMicroUSD > 0 {
				budget.DailyBudgetMicroUSD = req.DailyBudgetMicroUSD
				budget.HourlyBudgetMicroUSD = s.computeHourlyBudget(req.DailyBudgetMicroUSD)
			}
			if req.LifetimeBudgetMicroUSD > 0 {
				budget.LifetimeBudgetMicroUSD = req.LifetimeBudgetMicroUSD
			}
			budget.UpdatedAt = time.Now().UTC()
			_ = s.repo.UpdateBudget(ctx, budget)
		}
	}

	return campaign, nil
}

// PauseCampaign transitions an active campaign to paused.
func (s *CampaignService) PauseCampaign(ctx context.Context, campaignID, requesterID string) (*models.Campaign, error) {
	campaign, err := s.repo.GetCampaign(ctx, campaignID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrCampaignNotFound
		}
		return nil, fmt.Errorf("campaign_service: get campaign: %w", err)
	}

	if campaign.Status != models.CampaignStatusActive {
		return nil, fmt.Errorf("campaign_service: cannot pause a campaign in %s state", campaign.Status)
	}

	campaign.Status = models.CampaignStatusPaused
	campaign.UpdatedAt = time.Now().UTC()

	if err := s.repo.UpdateCampaign(ctx, campaign); err != nil {
		return nil, fmt.Errorf("campaign_service: update campaign: %w", err)
	}

	// Invalidate any budget cache so the pacer stops spending.
	s.invalidateBudgetCache(ctx, campaignID)

	s.logger.Info("campaign paused",
		zap.String("campaign_id", campaignID),
		zap.String("requester_id", requesterID),
	)
	return campaign, nil
}

// ResumeCampaign transitions a paused campaign back to active.
func (s *CampaignService) ResumeCampaign(ctx context.Context, campaignID string) (*models.Campaign, error) {
	campaign, err := s.repo.GetCampaign(ctx, campaignID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrCampaignNotFound
		}
		return nil, err
	}
	if campaign.Status != models.CampaignStatusPaused {
		return nil, fmt.Errorf("campaign is in %s state, not paused", campaign.Status)
	}

	// Verify budget is not exhausted.
	budget, err := s.repo.GetBudgetByCampaign(ctx, campaignID)
	if err == nil && budget.BudgetExhausted {
		return nil, ErrBudgetExhausted
	}

	campaign.Status = models.CampaignStatusActive
	campaign.UpdatedAt = time.Now().UTC()

	if err := s.repo.UpdateCampaign(ctx, campaign); err != nil {
		return nil, err
	}

	return campaign, nil
}

// GetCampaignStats returns aggregated performance metrics for a campaign.
func (s *CampaignService) GetCampaignStats(ctx context.Context, campaignID string, start, end time.Time) (*models.CampaignStats, error) {
	cacheKey := fmt.Sprintf("campaign:stats:%s:%d:%d", campaignID, start.Unix(), end.Unix())
	// Short cache for stats to reduce DB load.
	cached, err := s.redis.Get(ctx, cacheKey).Bytes()
	if err == nil {
		var stats models.CampaignStats
		if jsonUnmarshal(cached, &stats) == nil {
			return &stats, nil
		}
	}

	stats, err := s.repo.GetCampaignStats(ctx, campaignID, start, end)
	if err != nil {
		return nil, fmt.Errorf("campaign_service: get stats: %w", err)
	}

	// Compute derived rates.
	if stats.Impressions > 0 {
		stats.CTR = float64(stats.Clicks) / float64(stats.Impressions)
		stats.CPMMicroUSD = stats.TotalSpendMicroUSD * 1000 / stats.Impressions
	}
	if stats.Clicks > 0 {
		stats.CVR = float64(stats.Conversions) / float64(stats.Clicks)
		if stats.TotalSpendMicroUSD > 0 {
			stats.CPCMicroUSD = stats.TotalSpendMicroUSD / stats.Clicks
		}
	}
	if stats.Conversions > 0 && stats.TotalSpendMicroUSD > 0 {
		stats.CPAMicroUSD = stats.TotalSpendMicroUSD / stats.Conversions
	}

	// Budget utilisation.
	budget, err := s.repo.GetBudgetByCampaign(ctx, campaignID)
	if err == nil && budget.DailyBudgetMicroUSD > 0 {
		stats.BudgetUtilisation = math.Min(
			float64(budget.TodaySpendMicroUSD)/float64(budget.DailyBudgetMicroUSD),
			1.0,
		)
	}

	// Cache for 60 seconds.
	if data, err := jsonMarshal(stats); err == nil {
		_ = s.redis.Set(ctx, cacheKey, data, 60*time.Second).Err()
	}

	return stats, nil
}

// BudgetPacing computes the ideal spend rate for the remaining day hours.
// Returns the amount (microUSD) that can be spent in the next minute.
// Formula: remainingBudget / remainingMinutes.
func (s *CampaignService) BudgetPacing(ctx context.Context, campaignID string) (int64, error) {
	budget, err := s.repo.GetBudgetByCampaign(ctx, campaignID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrBudgetNotFound
		}
		return 0, fmt.Errorf("campaign_service: get budget: %w", err)
	}

	// How much is left for today?
	remainingDailyBudget := budget.DailyBudgetMicroUSD - budget.TodaySpendMicroUSD
	if remainingDailyBudget <= 0 {
		// Mark exhausted and pause the campaign.
		if !budget.BudgetExhausted {
			budget.BudgetExhausted = true
			budget.UpdatedAt = time.Now().UTC()
			_ = s.repo.UpdateBudget(ctx, budget)
			_, _ = s.PauseCampaign(ctx, campaignID, "budget_pacer")
			s.logger.Warn("campaign paused: daily budget exhausted",
				zap.String("campaign_id", campaignID),
				zap.Int64("daily_budget_micro_usd", budget.DailyBudgetMicroUSD),
			)
		}
		return 0, ErrBudgetExhausted
	}

	// How many minutes are left today?
	now := time.Now().UTC()
	endOfDay := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, time.UTC)
	remainingMinutes := int64(endOfDay.Sub(now).Minutes())
	if remainingMinutes <= 0 {
		remainingMinutes = 1
	}

	// Spend evenly over the remaining day.
	perMinuteBudget := remainingDailyBudget / remainingMinutes

	// Update the cached hourly budget for use by the auction service.
	budget.HourlyBudgetMicroUSD = perMinuteBudget * 60
	budget.UpdatedAt = time.Now().UTC()
	_ = s.repo.UpdateBudget(ctx, budget)

	s.logger.Debug("budget pacing calculated",
		zap.String("campaign_id", campaignID),
		zap.Int64("remaining_budget_micro_usd", remainingDailyBudget),
		zap.Int64("remaining_minutes", remainingMinutes),
		zap.Int64("per_minute_budget_micro_usd", perMinuteBudget),
	)

	return perMinuteBudget, nil
}

// FrequencyCapping checks and increments the ad view count for a user/ad-set pair.
// Returns (allowed, currentCount, error). Uses Redis with a 24-hour TTL.
func (s *CampaignService) FrequencyCapping(ctx context.Context, userID, adSetID string, maxPerDay int) (bool, int64, error) {
	// Key resets daily via the TTL.
	key := fmt.Sprintf("freq:user:%s:adset:%s:day:%s", userID, adSetID, today())

	// INCR is atomic; the key will be created if it doesn't exist.
	count, err := s.redis.Incr(ctx, key).Result()
	if err != nil {
		return true, 0, fmt.Errorf("campaign_service: freq cap incr: %w", err)
	}

	// Set TTL on first increment.
	if count == 1 {
		_ = s.redis.Expire(ctx, key, 25*time.Hour).Err() // 25h to be safe across midnight
	}

	if maxPerDay <= 0 {
		maxPerDay = 3 // default cap: 3 impressions per user per day per ad set
	}

	if count > int64(maxPerDay) {
		// Undo the increment so the count stays accurate.
		_ = s.redis.Decr(ctx, key).Err()
		return false, count - 1, nil
	}

	return true, count, nil
}

// RecordImpression persists an impression and updates spend counters.
func (s *CampaignService) RecordImpression(ctx context.Context, imp *models.Impression) error {
	// Update DB spend counters.
	if err := s.repo.IncrementImpressions(ctx, imp.AdID, imp.AdSetID, imp.CampaignID, imp.ChargeMicroUSD); err != nil {
		s.logger.Error("failed to increment impression counters", zap.Error(err))
	}

	// Update today's spend in the budget row.
	budget, err := s.repo.GetBudgetByCampaign(ctx, imp.CampaignID)
	if err == nil {
		budget.TodaySpendMicroUSD += imp.ChargeMicroUSD
		budget.TotalSpendMicroUSD += imp.ChargeMicroUSD
		budget.UpdatedAt = time.Now().UTC()

		// Allow a small overspend buffer (e.g. 5%) before hard pausing.
		maxSpend := int64(float64(budget.DailyBudgetMicroUSD) * (1 + s.cfg.Budget.OverspendBuffer))
		if budget.TodaySpendMicroUSD >= maxSpend {
			budget.BudgetExhausted = true
			_, _ = s.PauseCampaign(ctx, imp.CampaignID, "budget_pacer")
		}
		_ = s.repo.UpdateBudget(ctx, budget)
		s.invalidateBudgetCache(ctx, imp.CampaignID)
	}

	return nil
}

// RecordClick persists a click event.
func (s *CampaignService) RecordClick(ctx context.Context, adID, adSetID, campaignID string) error {
	return s.repo.IncrementClicks(ctx, adID, adSetID, campaignID)
}

// computeHourlyBudget divides the daily budget by 24 for even pacing.
func (s *CampaignService) computeHourlyBudget(dailyBudgetMicroUSD int64) int64 {
	return dailyBudgetMicroUSD / 24
}

// invalidateBudgetCache removes cached budget data.
func (s *CampaignService) invalidateBudgetCache(ctx context.Context, campaignID string) {
	key := fmt.Sprintf("budget:campaign:%s", campaignID)
	_ = s.redis.Del(ctx, key).Err()
}

// today returns YYYY-MM-DD in UTC for use as a cache key suffix.
func today() string {
	now := time.Now().UTC()
	return fmt.Sprintf("%04d-%02d-%02d", now.Year(), int(now.Month()), now.Day())
}

// ---- Request DTOs ----------------------------------------------------------

// CreateCampaignRequest is the inbound payload for creating a campaign.
type CreateCampaignRequest struct {
	AdvertiserID           string                  `json:"advertiser_id"`
	Name                   string                  `json:"name"`
	Objective              models.CampaignObjective `json:"objective"`
	StartTime              time.Time               `json:"start_time"`
	EndTime                *time.Time              `json:"end_time,omitempty"`
	DailyBudgetMicroUSD    int64                   `json:"daily_budget_micro_usd"`
	LifetimeBudgetMicroUSD int64                   `json:"lifetime_budget_micro_usd"`
}

// UpdateCampaignRequest carries mutable fields for campaign updates.
type UpdateCampaignRequest struct {
	Name                   string     `json:"name,omitempty"`
	EndTime                *time.Time `json:"end_time,omitempty"`
	DailyBudgetMicroUSD    int64      `json:"daily_budget_micro_usd,omitempty"`
	LifetimeBudgetMicroUSD int64      `json:"lifetime_budget_micro_usd,omitempty"`
}

// ---- JSON helpers ----------------------------------------------------------

func jsonMarshal(v interface{}) ([]byte, error) {
	return encodingJSON.Marshal(v)
}

func jsonUnmarshal(data []byte, v interface{}) error {
	return encodingJSON.Unmarshal(data, v)
}
