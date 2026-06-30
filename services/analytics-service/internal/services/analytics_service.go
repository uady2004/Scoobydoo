package services

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/tiktok-clone/analytics-service/internal/config"
)

// ---------------------------------------------------------------------------
// Domain models
// ---------------------------------------------------------------------------

// VideoAnalytics holds per-video performance metrics over a date range.
type VideoAnalytics struct {
	VideoID        string    `json:"video_id"`
	PeriodStart    time.Time `json:"period_start"`
	PeriodEnd      time.Time `json:"period_end"`
	Views          int64     `json:"views"`
	UniqueViewers  int64     `json:"unique_viewers"`
	TotalWatchTime int64     `json:"total_watch_time_seconds"`
	AvgWatchTime   float64   `json:"avg_watch_time_seconds"`
	CompletionRate float64   `json:"completion_rate_percent"`
	Likes          int64     `json:"likes"`
	Comments       int64     `json:"comments"`
	Shares         int64     `json:"shares"`
	Bookmarks      int64     `json:"bookmarks"`
	Revenue        float64   `json:"revenue_usd"`
	// Traffic sources breakdown.
	SourceFYP      float64 `json:"source_fyp_percent"`
	SourceFollowing float64 `json:"source_following_percent"`
	SourceSearch   float64 `json:"source_search_percent"`
	SourceProfile  float64 `json:"source_profile_percent"`
	// Audience demographics.
	TopCountries []CountryBreakdown `json:"top_countries"`
}

// CountryBreakdown represents view share per country.
type CountryBreakdown struct {
	CountryCode string  `json:"country_code"`
	ViewShare   float64 `json:"view_share_percent"`
}

// CreatorAnalytics holds aggregate stats for a creator over a date range.
type CreatorAnalytics struct {
	CreatorID      string    `json:"creator_id"`
	PeriodStart    time.Time `json:"period_start"`
	PeriodEnd      time.Time `json:"period_end"`
	TotalViews     int64     `json:"total_views"`
	FollowersGained int64    `json:"followers_gained"`
	FollowersLost  int64     `json:"followers_lost"`
	NetFollowers   int64     `json:"net_followers"`
	EngagementRate float64   `json:"engagement_rate_percent"`
	TotalLikes     int64     `json:"total_likes"`
	TotalComments  int64     `json:"total_comments"`
	TotalShares    int64     `json:"total_shares"`
	TotalRevenue   float64   `json:"total_revenue_usd"`
	VideoCount     int64     `json:"video_count"`
	TopVideos      []TopVideo `json:"top_videos"`
}

// TopVideo is a summary entry in a creator's top-videos list.
type TopVideo struct {
	VideoID string  `json:"video_id"`
	Title   string  `json:"title"`
	Views   int64   `json:"views"`
	Likes   int64   `json:"likes"`
	Revenue float64 `json:"revenue_usd"`
}

// PlatformMetrics holds platform-wide health and growth metrics.
type PlatformMetrics struct {
	Date            time.Time `json:"date"`
	DAU             int64     `json:"dau"`
	MAU             int64     `json:"mau"`
	DAUToMAURatio   float64   `json:"dau_mau_ratio"`
	NewUsers        int64     `json:"new_users"`
	RetentionD1     float64   `json:"retention_d1_percent"`
	RetentionD7     float64   `json:"retention_d7_percent"`
	RetentionD30    float64   `json:"retention_d30_percent"`
	VideosUploaded  int64     `json:"videos_uploaded"`
	TotalVideoViews int64     `json:"total_video_views"`
	AvgSessionMin   float64   `json:"avg_session_minutes"`
	LiveSessions    int64     `json:"live_sessions"`
	TotalRevenue    float64   `json:"total_revenue_usd"`
}

// LiveAnalytics holds real-time metrics for an ongoing live stream.
type LiveAnalytics struct {
	LiveID           string    `json:"live_id"`
	CreatorID        string    `json:"creator_id"`
	StartedAt        time.Time `json:"started_at"`
	CurrentViewers   int64     `json:"current_viewers"`
	PeakViewers      int64     `json:"peak_viewers"`
	TotalViewers     int64     `json:"total_viewers"`
	AvgWatchTime     float64   `json:"avg_watch_time_seconds"`
	GiftsReceived    int64     `json:"gifts_received"`
	GiftRevenueUSD   float64   `json:"gift_revenue_usd"`
	NewFollowers     int64     `json:"new_followers"`
	CommentsCount    int64     `json:"comments_count"`
	SharesCount      int64     `json:"shares_count"`
	ViewersByMinute  []ViewersSnapshot `json:"viewers_by_minute"`
}

// ViewersSnapshot is a point-in-time viewer count.
type ViewersSnapshot struct {
	Timestamp time.Time `json:"timestamp"`
	Viewers   int64     `json:"viewers"`
}

// AdAnalytics holds performance metrics for an ad campaign.
type AdAnalytics struct {
	CampaignID    string    `json:"campaign_id"`
	AdID          string    `json:"ad_id"`
	PeriodStart   time.Time `json:"period_start"`
	PeriodEnd     time.Time `json:"period_end"`
	Impressions   int64     `json:"impressions"`
	Clicks        int64     `json:"clicks"`
	CTR           float64   `json:"ctr_percent"`
	Conversions   int64     `json:"conversions"`
	CVR           float64   `json:"cvr_percent"`
	SpendUSD      float64   `json:"spend_usd"`
	RevenueUSD    float64   `json:"revenue_usd"`
	ROAS          float64   `json:"roas"`
	CPM           float64   `json:"cpm_usd"`
	CPC           float64   `json:"cpc_usd"`
	VideoViews    int64     `json:"video_views"`
	Completions   int64     `json:"completions"`
	CompletionRate float64  `json:"completion_rate_percent"`
}

// ---------------------------------------------------------------------------
// Service
// ---------------------------------------------------------------------------

// AnalyticsService provides all analytics query methods.
type AnalyticsService struct {
	cfg    *config.Config
	ch     clickhouse.Conn
	redis  *redis.Client
	logger *zap.Logger
}

// NewAnalyticsService creates a new AnalyticsService.
func NewAnalyticsService(
	cfg *config.Config,
	ch clickhouse.Conn,
	redisClient *redis.Client,
	logger *zap.Logger,
) *AnalyticsService {
	return &AnalyticsService{
		cfg:    cfg,
		ch:     ch,
		redis:  redisClient,
		logger: logger,
	}
}

// ---------------------------------------------------------------------------
// GetVideoAnalytics
// ---------------------------------------------------------------------------

// GetVideoAnalytics returns performance metrics for a single video over [start, end).
func (s *AnalyticsService) GetVideoAnalytics(
	ctx context.Context,
	videoID string,
	start, end time.Time,
) (*VideoAnalytics, error) {
	result := &VideoAnalytics{
		VideoID:     videoID,
		PeriodStart: start,
		PeriodEnd:   end,
	}

	// Core view + watch-time metrics from video_views table.
	viewQuery := `
		SELECT
			countIf(1)                                          AS views,
			uniqExact(viewer_id)                               AS unique_viewers,
			sum(watch_duration_seconds)                        AS total_watch_time,
			avgIf(watch_duration_seconds, watch_duration_seconds > 0) AS avg_watch_time,
			avgIf(completion_pct, completion_pct > 0)          AS completion_rate
		FROM video_views
		WHERE video_id = ?
		  AND toDate(viewed_at) >= toDate(?)
		  AND toDate(viewed_at) <  toDate(?)
	`
	row := s.ch.QueryRow(ctx, viewQuery, videoID, start, end)
	if err := row.Scan(
		&result.Views,
		&result.UniqueViewers,
		&result.TotalWatchTime,
		&result.AvgWatchTime,
		&result.CompletionRate,
	); err != nil {
		return nil, fmt.Errorf("video view metrics: %w", err)
	}

	// Engagement metrics from engagement_events table.
	engQuery := `
		SELECT
			countIf(event_type = 'like')     AS likes,
			countIf(event_type = 'comment')  AS comments,
			countIf(event_type = 'share')    AS shares,
			countIf(event_type = 'bookmark') AS bookmarks
		FROM engagement_events
		WHERE video_id = ?
		  AND toDate(occurred_at) >= toDate(?)
		  AND toDate(occurred_at) <  toDate(?)
	`
	eRow := s.ch.QueryRow(ctx, engQuery, videoID, start, end)
	if err := eRow.Scan(
		&result.Likes,
		&result.Comments,
		&result.Shares,
		&result.Bookmarks,
	); err != nil {
		return nil, fmt.Errorf("video engagement metrics: %w", err)
	}

	// Revenue from ad_revenue table (ads shown on this video).
	revQuery := `
		SELECT ifNull(sum(revenue_usd), 0)
		FROM ad_revenue
		WHERE video_id = ?
		  AND toDate(earned_at) >= toDate(?)
		  AND toDate(earned_at) <  toDate(?)
	`
	revRow := s.ch.QueryRow(ctx, revQuery, videoID, start, end)
	if err := revRow.Scan(&result.Revenue); err != nil {
		return nil, fmt.Errorf("video revenue: %w", err)
	}

	// Traffic source breakdown.
	sourceQuery := `
		SELECT
			source,
			round(100.0 * count() / sum(count()) OVER (), 2) AS pct
		FROM video_views
		WHERE video_id = ?
		  AND toDate(viewed_at) >= toDate(?)
		  AND toDate(viewed_at) <  toDate(?)
		GROUP BY source
	`
	sourceRows, err := s.ch.Query(ctx, sourceQuery, videoID, start, end)
	if err != nil {
		return nil, fmt.Errorf("video source breakdown: %w", err)
	}
	defer sourceRows.Close()
	for sourceRows.Next() {
		var source string
		var pct float64
		if err := sourceRows.Scan(&source, &pct); err != nil {
			continue
		}
		switch source {
		case "fyp":
			result.SourceFYP = pct
		case "following":
			result.SourceFollowing = pct
		case "search":
			result.SourceSearch = pct
		case "profile":
			result.SourceProfile = pct
		}
	}

	// Top countries.
	countryQuery := `
		SELECT
			country_code,
			round(100.0 * count() / sum(count()) OVER (), 2) AS pct
		FROM video_views
		WHERE video_id = ?
		  AND toDate(viewed_at) >= toDate(?)
		  AND toDate(viewed_at) <  toDate(?)
		GROUP BY country_code
		ORDER BY pct DESC
		LIMIT 10
	`
	countryRows, err := s.ch.Query(ctx, countryQuery, videoID, start, end)
	if err != nil {
		return nil, fmt.Errorf("video country breakdown: %w", err)
	}
	defer countryRows.Close()
	for countryRows.Next() {
		var cb CountryBreakdown
		if err := countryRows.Scan(&cb.CountryCode, &cb.ViewShare); err != nil {
			continue
		}
		result.TopCountries = append(result.TopCountries, cb)
	}

	return result, nil
}

// ---------------------------------------------------------------------------
// GetCreatorAnalytics
// ---------------------------------------------------------------------------

// GetCreatorAnalytics returns aggregate metrics for a creator over [start, end).
func (s *AnalyticsService) GetCreatorAnalytics(
	ctx context.Context,
	creatorID string,
	start, end time.Time,
) (*CreatorAnalytics, error) {
	result := &CreatorAnalytics{
		CreatorID:   creatorID,
		PeriodStart: start,
		PeriodEnd:   end,
	}

	// Aggregate view metrics across all creator videos.
	viewQuery := `
		SELECT
			count()                       AS total_views,
			countDistinct(video_id)       AS video_count
		FROM video_views
		WHERE creator_id = ?
		  AND toDate(viewed_at) >= toDate(?)
		  AND toDate(viewed_at) <  toDate(?)
	`
	row := s.ch.QueryRow(ctx, viewQuery, creatorID, start, end)
	if err := row.Scan(&result.TotalViews, &result.VideoCount); err != nil {
		return nil, fmt.Errorf("creator view metrics: %w", err)
	}

	// Follower changes.
	followerQuery := `
		SELECT
			countIf(event_type = 'follow')   AS gained,
			countIf(event_type = 'unfollow') AS lost
		FROM social_events
		WHERE target_user_id = ?
		  AND toDate(occurred_at) >= toDate(?)
		  AND toDate(occurred_at) <  toDate(?)
	`
	fRow := s.ch.QueryRow(ctx, followerQuery, creatorID, start, end)
	if err := fRow.Scan(&result.FollowersGained, &result.FollowersLost); err != nil {
		s.logger.Warn("creator follower metrics unavailable", zap.Error(err))
	}
	result.NetFollowers = result.FollowersGained - result.FollowersLost

	// Engagement totals.
	engQuery := `
		SELECT
			countIf(event_type = 'like')    AS likes,
			countIf(event_type = 'comment') AS comments,
			countIf(event_type = 'share')   AS shares
		FROM engagement_events
		WHERE creator_id = ?
		  AND toDate(occurred_at) >= toDate(?)
		  AND toDate(occurred_at) <  toDate(?)
	`
	eRow := s.ch.QueryRow(ctx, engQuery, creatorID, start, end)
	if err := eRow.Scan(&result.TotalLikes, &result.TotalComments, &result.TotalShares); err != nil {
		return nil, fmt.Errorf("creator engagement metrics: %w", err)
	}
	if result.TotalViews > 0 {
		totalEngagements := result.TotalLikes + result.TotalComments + result.TotalShares
		result.EngagementRate = float64(totalEngagements) / float64(result.TotalViews) * 100
	}

	// Revenue.
	revQuery := `
		SELECT ifNull(sum(revenue_usd), 0)
		FROM ad_revenue
		WHERE creator_id = ?
		  AND toDate(earned_at) >= toDate(?)
		  AND toDate(earned_at) <  toDate(?)
	`
	revRow := s.ch.QueryRow(ctx, revQuery, creatorID, start, end)
	if err := revRow.Scan(&result.TotalRevenue); err != nil {
		s.logger.Warn("creator revenue unavailable", zap.Error(err))
	}

	// Top 10 videos by views.
	topQuery := `
		SELECT
			video_id,
			count()                  AS views,
			countIf(event_type = 'like') AS likes
		FROM video_views
		INNER JOIN engagement_events USING (video_id)
		WHERE video_views.creator_id = ?
		  AND toDate(video_views.viewed_at) >= toDate(?)
		  AND toDate(video_views.viewed_at) <  toDate(?)
		GROUP BY video_id
		ORDER BY views DESC
		LIMIT 10
	`
	topRows, err := s.ch.Query(ctx, topQuery, creatorID, start, end)
	if err == nil {
		defer topRows.Close()
		for topRows.Next() {
			var tv TopVideo
			if scanErr := topRows.Scan(&tv.VideoID, &tv.Views, &tv.Likes); scanErr == nil {
				result.TopVideos = append(result.TopVideos, tv)
			}
		}
	}

	return result, nil
}

// ---------------------------------------------------------------------------
// GetPlatformMetrics
// ---------------------------------------------------------------------------

// GetPlatformMetrics returns platform-wide metrics for a specific date.
func (s *AnalyticsService) GetPlatformMetrics(ctx context.Context, date time.Time) (*PlatformMetrics, error) {
	result := &PlatformMetrics{Date: date}

	day := date.Format("2006-01-02")

	// DAU and new users.
	dauQuery := `
		SELECT
			uniqExact(user_id)                          AS dau,
			countIf(is_new_user = 1)                    AS new_users
		FROM user_sessions
		WHERE toDate(session_start) = toDate(?)
	`
	dauRow := s.ch.QueryRow(ctx, dauQuery, day)
	if err := dauRow.Scan(&result.DAU, &result.NewUsers); err != nil {
		return nil, fmt.Errorf("DAU metrics: %w", err)
	}

	// MAU (rolling 30-day unique users up to date).
	mauQuery := `
		SELECT uniqExact(user_id)
		FROM user_sessions
		WHERE toDate(session_start) BETWEEN toDate(?) AND toDate(?)
	`
	mauStart := date.AddDate(0, 0, -29).Format("2006-01-02")
	mauRow := s.ch.QueryRow(ctx, mauQuery, mauStart, day)
	if err := mauRow.Scan(&result.MAU); err != nil {
		return nil, fmt.Errorf("MAU metrics: %w", err)
	}
	if result.MAU > 0 {
		result.DAUToMAURatio = float64(result.DAU) / float64(result.MAU) * 100
	}

	// Retention rates from pre-computed cohort table.
	retentionQuery := `
		SELECT
			ifNull(avgIf(retained, days_since_install = 1), 0)  AS r1,
			ifNull(avgIf(retained, days_since_install = 7), 0)  AS r7,
			ifNull(avgIf(retained, days_since_install = 30), 0) AS r30
		FROM cohort_retention
		WHERE toDate(cohort_date) = toDate(?)
	`
	retRow := s.ch.QueryRow(ctx, retentionQuery, day)
	if err := retRow.Scan(&result.RetentionD1, &result.RetentionD7, &result.RetentionD30); err != nil {
		s.logger.Warn("retention metrics unavailable", zap.Error(err))
	}

	// Content metrics.
	contentQuery := `
		SELECT
			countIf(event_type = 'upload')  AS uploaded,
			count()                          AS total_views
		FROM video_events
		WHERE toDate(occurred_at) = toDate(?)
	`
	cRow := s.ch.QueryRow(ctx, contentQuery, day)
	if err := cRow.Scan(&result.VideosUploaded, &result.TotalVideoViews); err != nil {
		return nil, fmt.Errorf("content metrics: %w", err)
	}

	// Average session duration.
	sessionQuery := `
		SELECT ifNull(avg(session_duration_seconds) / 60.0, 0)
		FROM user_sessions
		WHERE toDate(session_start) = toDate(?)
	`
	sessionRow := s.ch.QueryRow(ctx, sessionQuery, day)
	if err := sessionRow.Scan(&result.AvgSessionMin); err != nil {
		s.logger.Warn("session duration unavailable", zap.Error(err))
	}

	// Live session count.
	liveQuery := `
		SELECT count(DISTINCT live_id)
		FROM live_events
		WHERE toDate(started_at) = toDate(?)
	`
	liveRow := s.ch.QueryRow(ctx, liveQuery, day)
	if err := liveRow.Scan(&result.LiveSessions); err != nil {
		s.logger.Warn("live session count unavailable", zap.Error(err))
	}

	// Platform revenue.
	revQuery := `
		SELECT ifNull(sum(revenue_usd), 0)
		FROM ad_revenue
		WHERE toDate(earned_at) = toDate(?)
	`
	revRow := s.ch.QueryRow(ctx, revQuery, day)
	if err := revRow.Scan(&result.TotalRevenue); err != nil {
		s.logger.Warn("platform revenue unavailable", zap.Error(err))
	}

	return result, nil
}

// ---------------------------------------------------------------------------
// GetLiveAnalytics
// ---------------------------------------------------------------------------

// GetLiveAnalytics returns real-time and historical metrics for a live stream.
func (s *AnalyticsService) GetLiveAnalytics(ctx context.Context, liveID string) (*LiveAnalytics, error) {
	result := &LiveAnalytics{LiveID: liveID}

	coreQuery := `
		SELECT
			creator_id,
			min(occurred_at)              AS started_at,
			max(concurrent_viewers)       AS peak_viewers,
			uniqExact(viewer_id)          AS total_viewers,
			avgIf(watch_seconds, watch_seconds > 0) AS avg_watch,
			countIf(event_type = 'gift') AS gifts,
			ifNull(sum(gift_value_usd), 0) AS gift_revenue,
			countIf(event_type = 'follow') AS new_followers,
			countIf(event_type = 'comment') AS comments,
			countIf(event_type = 'share') AS shares
		FROM live_events
		WHERE live_id = ?
		GROUP BY creator_id
	`
	row := s.ch.QueryRow(ctx, coreQuery, liveID)
	if err := row.Scan(
		&result.CreatorID,
		&result.StartedAt,
		&result.PeakViewers,
		&result.TotalViewers,
		&result.AvgWatchTime,
		&result.GiftsReceived,
		&result.GiftRevenueUSD,
		&result.NewFollowers,
		&result.CommentsCount,
		&result.SharesCount,
	); err != nil {
		return nil, fmt.Errorf("live core metrics: %w", err)
	}

	// Current viewers from Redis (updated by the live-service).
	cacheKey := fmt.Sprintf("live:viewers:%s", liveID)
	if val, err := s.redis.Get(ctx, cacheKey).Int64(); err == nil {
		result.CurrentViewers = val
	}

	// Viewers-by-minute time series (last 60 snapshots).
	seriesQuery := `
		SELECT
			toStartOfMinute(occurred_at) AS minute,
			max(concurrent_viewers)      AS viewers
		FROM live_events
		WHERE live_id = ?
		GROUP BY minute
		ORDER BY minute ASC
		LIMIT 60
	`
	rows, err := s.ch.Query(ctx, seriesQuery, liveID)
	if err != nil {
		return nil, fmt.Errorf("live viewer series: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var snap ViewersSnapshot
		if scanErr := rows.Scan(&snap.Timestamp, &snap.Viewers); scanErr == nil {
			result.ViewersByMinute = append(result.ViewersByMinute, snap)
		}
	}

	return result, nil
}

// ---------------------------------------------------------------------------
// GetAdAnalytics
// ---------------------------------------------------------------------------

// GetAdAnalytics returns performance metrics for an ad/campaign over [start, end).
func (s *AnalyticsService) GetAdAnalytics(
	ctx context.Context,
	campaignID, adID string,
	start, end time.Time,
) (*AdAnalytics, error) {
	result := &AdAnalytics{
		CampaignID:  campaignID,
		AdID:        adID,
		PeriodStart: start,
		PeriodEnd:   end,
	}

	query := `
		SELECT
			countIf(event_type = 'impression')  AS impressions,
			countIf(event_type = 'click')       AS clicks,
			countIf(event_type = 'conversion')  AS conversions,
			countIf(event_type = 'view')        AS video_views,
			countIf(event_type = 'complete')    AS completions,
			ifNull(sum(spend_usd), 0)           AS spend,
			ifNull(sum(revenue_usd), 0)         AS revenue
		FROM ad_events
		WHERE campaign_id = ?
		  AND ad_id       = ?
		  AND toDate(occurred_at) >= toDate(?)
		  AND toDate(occurred_at) <  toDate(?)
	`
	row := s.ch.QueryRow(ctx, query, campaignID, adID, start, end)
	if err := row.Scan(
		&result.Impressions,
		&result.Clicks,
		&result.Conversions,
		&result.VideoViews,
		&result.Completions,
		&result.SpendUSD,
		&result.RevenueUSD,
	); err != nil {
		return nil, fmt.Errorf("ad metrics: %w", err)
	}

	// Derived ratios.
	if result.Impressions > 0 {
		result.CTR = float64(result.Clicks) / float64(result.Impressions) * 100
		result.CPM = result.SpendUSD / float64(result.Impressions) * 1000
	}
	if result.Clicks > 0 {
		result.CVR = float64(result.Conversions) / float64(result.Clicks) * 100
		result.CPC = result.SpendUSD / float64(result.Clicks)
	}
	if result.SpendUSD > 0 {
		result.ROAS = result.RevenueUSD / result.SpendUSD
	}
	if result.VideoViews > 0 {
		result.CompletionRate = float64(result.Completions) / float64(result.VideoViews) * 100
	}

	return result, nil
}
