// Package services - TrendingService maintains real-time trending video scores
// using a sliding-window engagement model with exponential time decay.
//
// Scoring formula (per video, within the observation window):
//
//	raw_score = views*0.4 + likes*0.3 + shares*0.2 + comments*0.1
//	score     = raw_score / (age_hours + 2)^1.5
//
// The decay denominator ((age+2)^1.5) mirrors the "gravity" function used by
// Hacker News. Fresh content with a high engagement rate rises quickly; older
// content decays even if its absolute counts keep growing.
//
// Scores are stored in two Redis sorted sets:
//   - feed:trending:global         — global top-N across all categories
//   - feed:trending:cat:{category} — per-category top-N
//
// Both sets are trimmed to the top 10 000 entries after every update cycle to
// prevent unbounded growth.
package services

import (
	"context"
	"fmt"
	"math"
	"time"

	"go.uber.org/zap"

	"github.com/tiktok-clone/feed-service/internal/models"
	"github.com/tiktok-clone/feed-service/internal/repositories"
)

const (
	// trendingMaxEntries is the maximum number of entries kept in any trending
	// sorted set. Entries beyond this cap (lowest scores) are evicted.
	trendingMaxEntries = 10_000

	// defaultTrendingWindowHours is the look-back window for engagement data
	// when no explicit window is configured.
	defaultTrendingWindowHours = 24

	// defaultTrendingFetchBatch is the number of video IDs fetched from Postgres
	// per recalculation batch.
	defaultTrendingFetchBatch = 5_000
)

// ---- TrendingService --------------------------------------------------------

// TrendingService handles all logic related to computing and maintaining
// trending video scores.
type TrendingService struct {
	repo        *repositories.FeedRepository
	logger      *zap.Logger
	windowHours int
	fetchBatch  int
}

// NewTrendingService constructs a TrendingService.
func NewTrendingService(
	repo *repositories.FeedRepository,
	windowHours int,
	logger *zap.Logger,
) *TrendingService {
	if windowHours <= 0 {
		windowHours = defaultTrendingWindowHours
	}
	return &TrendingService{
		repo:        repo,
		logger:      logger,
		windowHours: windowHours,
		fetchBatch:  defaultTrendingFetchBatch,
	}
}

// ---- RecalculateAll ---------------------------------------------------------

// RecalculateAll is the main entry point called by the TrendingUpdaterWorker
// every hour. It:
//  1. Fetches all video IDs published within the trending window from Postgres.
//  2. Loads their raw engagement counters.
//  3. Computes the time-decayed score for each.
//  4. Upserts scores into the Redis sorted sets (global + per-category).
//  5. Trims both sets to keep only the top trendingMaxEntries.
func (s *TrendingService) RecalculateAll(ctx context.Context) error {
	start := time.Now()
	s.logger.Info("trending recalculation started",
		zap.Int("window_hours", s.windowHours),
	)

	videoIDs, err := s.repo.GetRecentVideoIDs(ctx, s.windowHours, s.fetchBatch)
	if err != nil {
		return fmt.Errorf("trending recalc: fetch video IDs: %w", err)
	}
	if len(videoIDs) == 0 {
		s.logger.Info("trending recalculation: no recent videos found")
		return nil
	}

	s.logger.Info("trending recalculation: processing videos",
		zap.Int("count", len(videoIDs)),
	)

	entries, err := s.repo.GetVideoEngagementStats(ctx, videoIDs)
	if err != nil {
		return fmt.Errorf("trending recalc: fetch engagement stats: %w", err)
	}

	now := time.Now()
	var updated, skipped int

	for _, entry := range entries {
		score := s.computeScore(entry, now)
		entry.Score = score
		entry.LastUpdatedAt = now

		// Persist to Redis — both global set and per-category set.
		if err := s.repo.UpsertTrendingScore(ctx, entry.VideoID, entry.Category, score); err != nil {
			s.logger.Warn("failed to upsert trending score",
				zap.String("video_id", entry.VideoID),
				zap.Error(err),
			)
			skipped++
			continue
		}
		updated++
	}

	// Trim global set to cap after bulk update.
	if err := s.repo.TrimTrendingSet(ctx, trendingMaxEntries); err != nil {
		s.logger.Warn("failed to trim trending set", zap.Error(err))
	}

	s.logger.Info("trending recalculation complete",
		zap.Int("updated", updated),
		zap.Int("skipped", skipped),
		zap.Duration("elapsed", time.Since(start)),
	)
	return nil
}

// RecalculateVideo recomputes the trending score for a single video and updates
// Redis immediately. Called on hot-path events (e.g. a video goes viral).
func (s *TrendingService) RecalculateVideo(ctx context.Context, videoID, category string) error {
	entries, err := s.repo.GetVideoEngagementStats(ctx, []string{videoID})
	if err != nil {
		return fmt.Errorf("recalc video %s: %w", videoID, err)
	}
	if len(entries) == 0 {
		return fmt.Errorf("recalc video %s: not found", videoID)
	}
	entry := entries[0]
	// Use the category passed by the caller if the DB returned empty.
	if entry.Category == "" {
		entry.Category = category
	}
	score := s.computeScore(entry, time.Now())
	return s.repo.UpsertTrendingScore(ctx, videoID, entry.Category, score)
}

// RemoveVideo removes a video from all trending sorted sets (e.g. on deletion
// or moderation action).
func (s *TrendingService) RemoveVideo(ctx context.Context, videoID, category string) error {
	return s.repo.RemoveTrendingEntry(ctx, videoID, category)
}

// ---- computeScore -----------------------------------------------------------

// computeScore applies the sliding-window trending formula to a TrendingEntry.
//
// Formula:
//
//	raw   = views*0.4 + likes*0.3 + shares*0.2 + comments*0.1
//	score = raw / (age_hours + 2)^1.5
//
// The "+2" bias prevents division by zero for brand-new content and also
// gives new videos a small initial boost relative to older high-count videos.
func (s *TrendingService) computeScore(entry *models.TrendingEntry, now time.Time) float64 {
	if entry == nil {
		return 0
	}

	raw := float64(entry.Views)*0.4 +
		float64(entry.Likes)*0.3 +
		float64(entry.Shares)*0.2 +
		float64(entry.Comments)*0.1

	ageHours := now.Sub(entry.CreatedAt).Hours()
	if ageHours < 0 {
		ageHours = 0
	}

	// Decay: divide by (age+2)^1.5
	// pow(base, 1.5) = base * sqrt(base)
	base := ageHours + 2.0
	decay := base * math.Sqrt(base)
	if decay <= 0 {
		return raw
	}

	return raw / decay
}

// ---- WindowScoreBreakdown ---------------------------------------------------

// WindowScoreBreakdown is a debug/analytics struct that exposes the individual
// components that went into a trending score calculation.
type WindowScoreBreakdown struct {
	VideoID    string    `json:"video_id"`
	Views      int64     `json:"views"`
	Likes      int64     `json:"likes"`
	Shares     int64     `json:"shares"`
	Comments   int64     `json:"comments"`
	RawScore   float64   `json:"raw_score"`
	AgeHours   float64   `json:"age_hours"`
	Decay      float64   `json:"decay"`
	FinalScore float64   `json:"final_score"`
	ComputedAt time.Time `json:"computed_at"`
}

// ExplainScore returns a detailed breakdown of how a video's trending score was
// computed. Useful for monitoring dashboards and debugging feed ranking.
func (s *TrendingService) ExplainScore(ctx context.Context, videoID string) (*WindowScoreBreakdown, error) {
	entries, err := s.repo.GetVideoEngagementStats(ctx, []string{videoID})
	if err != nil {
		return nil, fmt.Errorf("explain score: %w", err)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("explain score: video %s not found", videoID)
	}
	entry := entries[0]
	now := time.Now()

	ageHours := now.Sub(entry.CreatedAt).Hours()
	if ageHours < 0 {
		ageHours = 0
	}

	raw := float64(entry.Views)*0.4 +
		float64(entry.Likes)*0.3 +
		float64(entry.Shares)*0.2 +
		float64(entry.Comments)*0.1

	base := ageHours + 2.0
	decay := base * math.Sqrt(base)
	final := 0.0
	if decay > 0 {
		final = raw / decay
	}

	return &WindowScoreBreakdown{
		VideoID:    videoID,
		Views:      entry.Views,
		Likes:      entry.Likes,
		Shares:     entry.Shares,
		Comments:   entry.Comments,
		RawScore:   raw,
		AgeHours:   ageHours,
		Decay:      decay,
		FinalScore: final,
		ComputedAt: now,
	}, nil
}
