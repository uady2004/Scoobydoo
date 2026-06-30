package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/tiktok-clone/moderation-service/internal/config"
	"github.com/tiktok-clone/moderation-service/internal/models"
)

// Sentinel errors exposed to callers.
var (
	ErrRequestNotFound    = errors.New("moderation request not found")
	ErrResultNotFound     = errors.New("moderation result not found")
	ErrQueueItemNotFound  = errors.New("queue item not found")
	ErrAppealNotFound     = errors.New("appeal not found")
	ErrAlreadyReviewed    = errors.New("content has already been reviewed")
	ErrTooManyAppeals     = errors.New("user has exceeded the maximum number of active appeals")
	ErrInvalidDecision    = errors.New("decision must be approved or rejected")
	ErrAppealNotReviewable = errors.New("appeal is not in a reviewable state")
)

// Repository defines all persistence operations required by the service.
// The concrete implementation lives in the repository package (not shown here).
type Repository interface {
	// Requests
	CreateRequest(ctx context.Context, req *models.ModerationRequest) error
	GetRequest(ctx context.Context, id string) (*models.ModerationRequest, error)
	UpdateRequestStatus(ctx context.Context, id string, status models.ModerationStatus) error

	// Results
	CreateResult(ctx context.Context, result *models.ModerationResult) error
	GetResult(ctx context.Context, id string) (*models.ModerationResult, error)
	GetResultByContentID(ctx context.Context, contentID string) (*models.ModerationResult, error)
	UpdateResult(ctx context.Context, result *models.ModerationResult) error

	// Queue
	CreateQueueItem(ctx context.Context, item *models.ModeratorQueueItem) error
	GetQueueItem(ctx context.Context, id string) (*models.ModeratorQueueItem, error)
	UpdateQueueItem(ctx context.Context, item *models.ModeratorQueueItem) error
	ListQueueItems(ctx context.Context, filter QueueFilter) ([]*models.ModeratorQueueItem, int64, error)
	EscalateStaleItems(ctx context.Context, maxAge time.Duration) (int64, error)

	// Appeals
	CreateAppeal(ctx context.Context, appeal *models.Appeal) error
	GetAppeal(ctx context.Context, id string) (*models.Appeal, error)
	GetAppealByResultID(ctx context.Context, resultID string) (*models.Appeal, error)
	UpdateAppeal(ctx context.Context, appeal *models.Appeal) error
	CountActiveAppeals(ctx context.Context, userID string) (int64, error)

	// Stats
	GetStats(ctx context.Context, start, end time.Time) (*models.ModerationStats, error)
}

// QueueFilter controls which queue items are listed.
type QueueFilter struct {
	Status      models.ModerationStatus
	ContentType models.ContentType
	AssignedTo  string
	MinScore    float64
	Limit       int
	Offset      int
}

// EventPublisher publishes moderation outcome events to Kafka.
type EventPublisher interface {
	PublishModerationResult(ctx context.Context, result *models.ModerationResult) error
}

// ModerationService orchestrates all detectors and manages the review lifecycle.
type ModerationService struct {
	cfg       *config.Config
	repo      Repository
	publisher EventPublisher
	redis     *redis.Client
	logger    *zap.Logger

	nsfwDetector     *NSFWDetector
	violenceDetector *ViolenceDetector
	spamDetector     *SpamDetector
}

// NewModerationService wires up the service with all its dependencies.
func NewModerationService(
	cfg *config.Config,
	repo Repository,
	publisher EventPublisher,
	rdb *redis.Client,
	logger *zap.Logger,
) *ModerationService {
	return &ModerationService{
		cfg:              cfg,
		repo:             repo,
		publisher:        publisher,
		redis:            rdb,
		logger:           logger,
		nsfwDetector:     NewNSFWDetector(cfg.AI, cfg.Thresholds),
		violenceDetector: NewViolenceDetector(cfg.AI, cfg.Thresholds),
		spamDetector:     NewSpamDetector(cfg.AI, cfg.Thresholds),
	}
}

// ModerateContent is the main entry point. It runs all detectors concurrently,
// computes a combined score, then either auto-decides or queues for human review.
func (s *ModerationService) ModerateContent(ctx context.Context, req *models.ModerationRequest) (*models.ModerationResult, error) {
	if req.ID == "" {
		req.ID = uuid.NewString()
	}
	req.Status = models.ModerationStatusProcessing
	req.CreatedAt = time.Now().UTC()
	req.UpdatedAt = time.Now().UTC()

	if err := s.repo.CreateRequest(ctx, req); err != nil {
		return nil, fmt.Errorf("moderation_service: persist request: %w", err)
	}

	// Run all detectors concurrently.
	type detectorResult struct {
		score *models.DetectorScore
		err   error
		name  string
	}

	ch := make(chan detectorResult, 3)

	go func() {
		score, err := s.nsfwDetector.Detect(ctx, req)
		ch <- detectorResult{score: score, err: err, name: "nsfw"}
	}()
	go func() {
		score, err := s.violenceDetector.Detect(ctx, req)
		ch <- detectorResult{score: score, err: err, name: "violence"}
	}()
	go func() {
		score, err := s.spamDetector.Detect(ctx, req)
		ch <- detectorResult{score: score, err: err, name: "spam"}
	}()

	var (
		nsfwScore     float64
		violenceScore float64
		spamScore     float64
		detectorScores []models.DetectorScore
		detectorErrors []string
	)

	for i := 0; i < 3; i++ {
		dr := <-ch
		if dr.err != nil {
			s.logger.Warn("detector failed",
				zap.String("detector", dr.name),
				zap.Error(dr.err),
				zap.String("content_id", req.ContentID),
			)
			detectorErrors = append(detectorErrors, dr.name+": "+dr.err.Error())
			continue
		}
		detectorScores = append(detectorScores, *dr.score)
		switch dr.name {
		case "nsfw":
			nsfwScore = dr.score.Score
		case "violence":
			violenceScore = dr.score.Score
		case "spam":
			spamScore = dr.score.Score
		}
	}

	// Weighted combined score.
	t := s.cfg.Thresholds
	combinedScore := nsfwScore*t.NSFWWeight +
		violenceScore*t.ViolenceWeight +
		spamScore*t.SpamWeight

	// Determine decision.
	status, rejectionReason, rejectionDetail := s.decide(
		nsfwScore, violenceScore, spamScore, combinedScore,
	)

	result := &models.ModerationResult{
		ID:              uuid.NewString(),
		RequestID:       req.ID,
		ContentID:       req.ContentID,
		ContentType:     req.ContentType,
		UserID:          req.UserID,
		NSFWScore:       nsfwScore,
		ViolenceScore:   violenceScore,
		SpamScore:       spamScore,
		CombinedScore:   combinedScore,
		DetectorScores:  detectorScores,
		Status:          status,
		RejectionReason: rejectionReason,
		RejectionDetail: rejectionDetail,
		AutoProcessed:   status != models.ModerationStatusHumanReview,
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}

	if err := s.repo.CreateResult(ctx, result); err != nil {
		return nil, fmt.Errorf("moderation_service: persist result: %w", err)
	}
	if err := s.repo.UpdateRequestStatus(ctx, req.ID, status); err != nil {
		s.logger.Error("failed to update request status", zap.Error(err))
	}

	// If the decision is uncertain, create a queue item for human review.
	if status == models.ModerationStatusHumanReview {
		item := &models.ModeratorQueueItem{
			ID:            uuid.NewString(),
			ResultID:      result.ID,
			RequestID:     req.ID,
			ContentID:     req.ContentID,
			ContentType:   req.ContentType,
			UserID:        req.UserID,
			ContentURL:    req.ContentURL,
			NSFWScore:     nsfwScore,
			ViolenceScore: violenceScore,
			SpamScore:     spamScore,
			CombinedScore: combinedScore,
			Priority:      s.computePriority(nsfwScore, violenceScore, spamScore),
			Status:        models.ModerationStatusHumanReview,
			CreatedAt:     time.Now().UTC(),
			UpdatedAt:     time.Now().UTC(),
		}
		if err := s.repo.CreateQueueItem(ctx, item); err != nil {
			s.logger.Error("failed to create queue item", zap.Error(err))
		}
	}

	// Publish outcome event (best-effort).
	if err := s.publisher.PublishModerationResult(ctx, result); err != nil {
		s.logger.Warn("failed to publish moderation result event",
			zap.Error(err),
			zap.String("result_id", result.ID),
		)
	}

	s.logger.Info("moderation complete",
		zap.String("content_id", req.ContentID),
		zap.String("status", string(status)),
		zap.Float64("nsfw", nsfwScore),
		zap.Float64("violence", violenceScore),
		zap.Float64("spam", spamScore),
		zap.Float64("combined", combinedScore),
	)

	return result, nil
}

// decide translates scores into a ModerationStatus and optional rejection info.
func (s *ModerationService) decide(nsfw, violence, spam, combined float64) (
	models.ModerationStatus, models.RejectionReason, string,
) {
	t := s.cfg.Thresholds

	// Auto-reject: any single detector is above its threshold OR combined is above threshold.
	if nsfw >= t.NSFWAutoReject {
		return models.ModerationStatusRejected, models.RejectionReasonNSFW,
			fmt.Sprintf("NSFW score %.3f exceeds threshold %.3f", nsfw, t.NSFWAutoReject)
	}
	if violence >= t.ViolenceAutoReject {
		return models.ModerationStatusRejected, models.RejectionReasonViolence,
			fmt.Sprintf("violence score %.3f exceeds threshold %.3f", violence, t.ViolenceAutoReject)
	}
	if spam >= t.SpamAutoReject {
		return models.ModerationStatusRejected, models.RejectionReasonSpam,
			fmt.Sprintf("spam score %.3f exceeds threshold %.3f", spam, t.SpamAutoReject)
	}
	if combined >= t.CombinedAutoReject {
		return models.ModerationStatusRejected, models.RejectionReasonOther,
			fmt.Sprintf("combined score %.3f exceeds threshold %.3f", combined, t.CombinedAutoReject)
	}

	// Auto-approve: all individual scores and combined are safely low.
	if nsfw < t.NSFWAutoApprove &&
		violence < t.ViolenceAutoApprove &&
		spam < t.SpamAutoApprove &&
		combined < t.CombinedAutoApprove {
		return models.ModerationStatusApproved, "", ""
	}

	// Borderline: route to human review.
	return models.ModerationStatusHumanReview, "", ""
}

// computePriority assigns a queue priority score (higher = more urgent).
// Priority 10 = highest concern, 1 = lowest.
func (s *ModerationService) computePriority(nsfw, violence, spam float64) int {
	maxScore := nsfw
	if violence > maxScore {
		maxScore = violence
	}
	if spam > maxScore {
		maxScore = spam
	}
	switch {
	case maxScore >= 0.75:
		return 10
	case maxScore >= 0.60:
		return 7
	case maxScore >= 0.45:
		return 5
	default:
		return 3
	}
}

// GetModeratorQueue returns the current human-review queue, optionally filtered.
func (s *ModerationService) GetModeratorQueue(ctx context.Context, filter QueueFilter) ([]*models.ModeratorQueueItem, int64, error) {
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Limit > 200 {
		filter.Limit = 200
	}

	// Escalate stale items before returning the queue so moderators see urgency correctly.
	maxAge := time.Duration(s.cfg.Thresholds.ReviewQueueMaxAgeHours) * time.Hour
	if _, err := s.repo.EscalateStaleItems(ctx, maxAge); err != nil {
		s.logger.Warn("failed to escalate stale items", zap.Error(err))
	}

	return s.repo.ListQueueItems(ctx, filter)
}

// ReviewContent processes a human moderator's decision on a queue item.
func (s *ModerationService) ReviewContent(ctx context.Context, decision *models.ReviewDecision) (*models.ModerationResult, error) {
	if decision.Decision != models.ModerationStatusApproved &&
		decision.Decision != models.ModerationStatusRejected {
		return nil, ErrInvalidDecision
	}

	item, err := s.repo.GetQueueItem(ctx, decision.QueueItemID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrQueueItemNotFound
		}
		return nil, fmt.Errorf("moderation_service: get queue item: %w", err)
	}

	if item.Status != models.ModerationStatusHumanReview &&
		item.Status != models.ModerationStatusEscalated {
		return nil, ErrAlreadyReviewed
	}

	now := time.Now().UTC()

	// Update the queue item.
	item.Status = decision.Decision
	item.AssignedTo = decision.ModeratorID
	item.UpdatedAt = now
	if err := s.repo.UpdateQueueItem(ctx, item); err != nil {
		return nil, fmt.Errorf("moderation_service: update queue item: %w", err)
	}

	// Update the associated result.
	result, err := s.repo.GetResult(ctx, item.ResultID)
	if err != nil {
		return nil, fmt.Errorf("moderation_service: get result: %w", err)
	}

	result.Status = decision.Decision
	result.ReviewedBy = decision.ModeratorID
	result.ReviewedAt = &now
	result.ReviewNotes = decision.Notes
	if decision.Decision == models.ModerationStatusRejected {
		result.RejectionReason = decision.Reason
		result.RejectionDetail = decision.Notes
	}
	result.UpdatedAt = now

	if err := s.repo.UpdateResult(ctx, result); err != nil {
		return nil, fmt.Errorf("moderation_service: update result: %w", err)
	}

	// Publish outcome event.
	if err := s.publisher.PublishModerationResult(ctx, result); err != nil {
		s.logger.Warn("failed to publish review result", zap.Error(err), zap.String("result_id", result.ID))
	}

	s.logger.Info("human review complete",
		zap.String("queue_item_id", decision.QueueItemID),
		zap.String("moderator_id", decision.ModeratorID),
		zap.String("decision", string(decision.Decision)),
		zap.String("content_id", item.ContentID),
	)

	return result, nil
}

// AppealDecision allows a user to challenge a rejection.
func (s *ModerationService) AppealDecision(ctx context.Context, userID string, req *models.AppealRequest) (*models.Appeal, error) {
	// Verify the result exists and was rejected.
	result, err := s.repo.GetResultByContentID(ctx, req.ContentID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrResultNotFound
		}
		return nil, fmt.Errorf("moderation_service: get result: %w", err)
	}

	if result.Status != models.ModerationStatusRejected {
		return nil, errors.New("only rejected content can be appealed")
	}
	if result.UserID != userID {
		return nil, errors.New("only the content owner can appeal a decision")
	}

	// Check for duplicate appeal.
	if existing, err := s.repo.GetAppealByResultID(ctx, result.ID); err == nil && existing != nil {
		return nil, errors.New("an appeal for this content already exists")
	}

	// Check rate limit: users can have at most MaxAppealsPerUser active appeals.
	count, err := s.repo.CountActiveAppeals(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("moderation_service: count appeals: %w", err)
	}
	if count >= int64(s.cfg.Thresholds.MaxAppealsPerUser) {
		return nil, ErrTooManyAppeals
	}

	appeal := &models.Appeal{
		ID:          uuid.NewString(),
		ResultID:    result.ID,
		ContentID:   req.ContentID,
		UserID:      userID,
		AppealText:  req.AppealText,
		EvidenceURL: req.EvidenceURL,
		Status:      models.AppealStatusPending,
		SubmittedAt: time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	if err := s.repo.CreateAppeal(ctx, appeal); err != nil {
		return nil, fmt.Errorf("moderation_service: create appeal: %w", err)
	}

	// Cache the appeal status in Redis for fast reads.
	s.cacheAppealStatus(ctx, appeal)

	s.logger.Info("appeal submitted",
		zap.String("appeal_id", appeal.ID),
		zap.String("user_id", userID),
		zap.String("content_id", req.ContentID),
	)

	return appeal, nil
}

// ReviewAppeal processes a moderator's decision on an appeal.
func (s *ModerationService) ReviewAppeal(ctx context.Context, appealID, moderatorID string, approve bool, note string) (*models.Appeal, error) {
	appeal, err := s.repo.GetAppeal(ctx, appealID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAppealNotFound
		}
		return nil, fmt.Errorf("moderation_service: get appeal: %w", err)
	}

	if appeal.Status != models.AppealStatusPending && appeal.Status != models.AppealStatusReviewing {
		return nil, ErrAppealNotReviewable
	}

	now := time.Now().UTC()
	appeal.ReviewedBy = moderatorID
	appeal.ReviewedAt = &now
	appeal.ReviewerNote = note
	appeal.UpdatedAt = now

	if approve {
		appeal.Status = models.AppealStatusApproved
		appeal.OutcomeStatus = models.ModerationStatusApproved

		// Reverse the original moderation decision.
		result, err := s.repo.GetResult(ctx, appeal.ResultID)
		if err == nil {
			result.Status = models.ModerationStatusApproved
			result.ReviewedBy = moderatorID
			result.ReviewedAt = &now
			result.ReviewNotes = "Appeal approved: " + note
			result.UpdatedAt = now
			_ = s.repo.UpdateResult(ctx, result)
			_ = s.publisher.PublishModerationResult(ctx, result)
		}
	} else {
		appeal.Status = models.AppealStatusDenied
		appeal.OutcomeStatus = models.ModerationStatusRejected
	}

	if err := s.repo.UpdateAppeal(ctx, appeal); err != nil {
		return nil, fmt.Errorf("moderation_service: update appeal: %w", err)
	}

	s.invalidateAppealCache(ctx, appealID)

	return appeal, nil
}

// GetAppealStatus returns the current status of an appeal, using Redis cache.
func (s *ModerationService) GetAppealStatus(ctx context.Context, appealID, userID string) (*models.Appeal, error) {
	// Try the cache first.
	cacheKey := fmt.Sprintf("appeal:status:%s", appealID)
	cached, err := s.redis.Get(ctx, cacheKey).Bytes()
	if err == nil {
		var appeal models.Appeal
		if json.Unmarshal(cached, &appeal) == nil {
			if appeal.UserID != userID {
				return nil, errors.New("access denied")
			}
			return &appeal, nil
		}
	}

	appeal, err := s.repo.GetAppeal(ctx, appealID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAppealNotFound
		}
		return nil, fmt.Errorf("moderation_service: get appeal: %w", err)
	}
	if appeal.UserID != userID {
		return nil, errors.New("access denied")
	}

	s.cacheAppealStatus(ctx, appeal)
	return appeal, nil
}

// GetStats returns aggregated moderation statistics for a time window.
func (s *ModerationService) GetStats(ctx context.Context, start, end time.Time) (*models.ModerationStats, error) {
	return s.repo.GetStats(ctx, start, end)
}

// cacheAppealStatus writes an appeal to Redis with a short TTL.
func (s *ModerationService) cacheAppealStatus(ctx context.Context, appeal *models.Appeal) {
	data, err := json.Marshal(appeal)
	if err != nil {
		return
	}
	key := fmt.Sprintf("appeal:status:%s", appeal.ID)
	_ = s.redis.Set(ctx, key, data, 5*time.Minute).Err()
}

// invalidateAppealCache removes a cached appeal status.
func (s *ModerationService) invalidateAppealCache(ctx context.Context, appealID string) {
	key := fmt.Sprintf("appeal:status:%s", appealID)
	_ = s.redis.Del(ctx, key).Err()
}
