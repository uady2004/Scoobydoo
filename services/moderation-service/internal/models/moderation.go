package models

import (
	"time"
)

// ContentType distinguishes between video, image, comment, and profile content.
type ContentType string

const (
	ContentTypeVideo   ContentType = "video"
	ContentTypeImage   ContentType = "image"
	ContentTypeComment ContentType = "comment"
	ContentTypeProfile ContentType = "profile"
)

// ModerationStatus represents the lifecycle state of a moderation request.
type ModerationStatus string

const (
	ModerationStatusPending      ModerationStatus = "pending"
	ModerationStatusProcessing   ModerationStatus = "processing"
	ModerationStatusApproved     ModerationStatus = "approved"
	ModerationStatusRejected     ModerationStatus = "rejected"
	ModerationStatusHumanReview  ModerationStatus = "human_review"
	ModerationStatusEscalated    ModerationStatus = "escalated"
)

// RejectionReason provides a machine-readable reason for rejection.
type RejectionReason string

const (
	RejectionReasonNSFW      RejectionReason = "nsfw"
	RejectionReasonViolence  RejectionReason = "violence"
	RejectionReasonSpam      RejectionReason = "spam"
	RejectionReasonHateSpeech RejectionReason = "hate_speech"
	RejectionReasonCopyright RejectionReason = "copyright"
	RejectionReasonOther     RejectionReason = "other"
)

// AppealStatus tracks where an appeal is in the review pipeline.
type AppealStatus string

const (
	AppealStatusPending   AppealStatus = "pending"
	AppealStatusReviewing AppealStatus = "reviewing"
	AppealStatusApproved  AppealStatus = "approved"
	AppealStatusDenied    AppealStatus = "denied"
)

// ModerationRequest is submitted when content needs review.
type ModerationRequest struct {
	ID          string      `json:"id" db:"id"`
	ContentID   string      `json:"content_id" db:"content_id"`
	ContentType ContentType `json:"content_type" db:"content_type"`
	UserID      string      `json:"user_id" db:"user_id"`

	// URL references — video URL, thumbnail URL, or text payload.
	ContentURL    string `json:"content_url" db:"content_url"`
	ThumbnailURL  string `json:"thumbnail_url,omitempty" db:"thumbnail_url"`
	TextContent   string `json:"text_content,omitempty" db:"text_content"`

	// Metadata passed in from the upstream service.
	Duration    float64           `json:"duration,omitempty" db:"duration"`
	FileSize    int64             `json:"file_size,omitempty" db:"file_size"`
	MIMEType    string            `json:"mime_type,omitempty" db:"mime_type"`
	Metadata    map[string]string `json:"metadata,omitempty" db:"metadata"`

	Priority  int              `json:"priority" db:"priority"` // higher = more urgent
	Status    ModerationStatus `json:"status" db:"status"`
	CreatedAt time.Time        `json:"created_at" db:"created_at"`
	UpdatedAt time.Time        `json:"updated_at" db:"updated_at"`
}

// DetectorScore records the raw output of a single detector.
type DetectorScore struct {
	DetectorName string    `json:"detector_name"`
	Score        float64   `json:"score"`        // [0.0, 1.0]
	Confidence   float64   `json:"confidence"`   // model confidence in the score
	Labels       []string  `json:"labels"`       // human-readable label tags
	ModelVersion string    `json:"model_version"`
	LatencyMs    int64     `json:"latency_ms"`
	RanAt        time.Time `json:"ran_at"`
}

// ModerationResult is written after all detectors have run.
type ModerationResult struct {
	ID                string           `json:"id" db:"id"`
	RequestID         string           `json:"request_id" db:"request_id"`
	ContentID         string           `json:"content_id" db:"content_id"`
	ContentType       ContentType      `json:"content_type" db:"content_type"`
	UserID            string           `json:"user_id" db:"user_id"`

	// Individual detector scores.
	NSFWScore     float64 `json:"nsfw_score" db:"nsfw_score"`
	ViolenceScore float64 `json:"violence_score" db:"violence_score"`
	SpamScore     float64 `json:"spam_score" db:"spam_score"`

	// Weighted composite score.
	CombinedScore float64 `json:"combined_score" db:"combined_score"`

	// Full detector outputs for audit trail.
	DetectorScores []DetectorScore `json:"detector_scores" db:"detector_scores"`

	// Final decision.
	Status          ModerationStatus `json:"status" db:"status"`
	RejectionReason RejectionReason  `json:"rejection_reason,omitempty" db:"rejection_reason"`
	RejectionDetail string           `json:"rejection_detail,omitempty" db:"rejection_detail"`

	// Human review fields (populated when status = human_review or escalated).
	ReviewedBy  string     `json:"reviewed_by,omitempty" db:"reviewed_by"`
	ReviewedAt  *time.Time `json:"reviewed_at,omitempty" db:"reviewed_at"`
	ReviewNotes string     `json:"review_notes,omitempty" db:"review_notes"`

	AutoProcessed bool      `json:"auto_processed" db:"auto_processed"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}

// ModeratorQueueItem is a row in the human-review work queue.
type ModeratorQueueItem struct {
	ID          string           `json:"id" db:"id"`
	ResultID    string           `json:"result_id" db:"result_id"`
	RequestID   string           `json:"request_id" db:"request_id"`
	ContentID   string           `json:"content_id" db:"content_id"`
	ContentType ContentType      `json:"content_type" db:"content_type"`
	UserID      string           `json:"user_id" db:"user_id"`
	ContentURL  string           `json:"content_url" db:"content_url"`

	NSFWScore     float64 `json:"nsfw_score" db:"nsfw_score"`
	ViolenceScore float64 `json:"violence_score" db:"violence_score"`
	SpamScore     float64 `json:"spam_score" db:"spam_score"`
	CombinedScore float64 `json:"combined_score" db:"combined_score"`

	Priority    int              `json:"priority" db:"priority"`
	Status      ModerationStatus `json:"status" db:"status"`
	AssignedTo  string           `json:"assigned_to,omitempty" db:"assigned_to"`
	AssignedAt  *time.Time       `json:"assigned_at,omitempty" db:"assigned_at"`

	// Escalation tracking.
	EscalatedAt    *time.Time `json:"escalated_at,omitempty" db:"escalated_at"`
	EscalationNote string     `json:"escalation_note,omitempty" db:"escalation_note"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// ReviewDecision is submitted by a human moderator.
type ReviewDecision struct {
	QueueItemID string           `json:"queue_item_id"`
	ModeratorID string           `json:"moderator_id"`
	Decision    ModerationStatus `json:"decision"` // approved or rejected
	Reason      RejectionReason  `json:"reason,omitempty"`
	Notes       string           `json:"notes,omitempty"`
}

// Appeal is filed by a user who disagrees with a moderation decision.
type Appeal struct {
	ID          string       `json:"id" db:"id"`
	ResultID    string       `json:"result_id" db:"result_id"`
	ContentID   string       `json:"content_id" db:"content_id"`
	UserID      string       `json:"user_id" db:"user_id"`

	// User-provided context.
	AppealText  string `json:"appeal_text" db:"appeal_text"`
	EvidenceURL string `json:"evidence_url,omitempty" db:"evidence_url"`

	Status      AppealStatus `json:"status" db:"status"`

	// Reviewer fields.
	ReviewedBy   string     `json:"reviewed_by,omitempty" db:"reviewed_by"`
	ReviewedAt   *time.Time `json:"reviewed_at,omitempty" db:"reviewed_at"`
	ReviewerNote string     `json:"reviewer_note,omitempty" db:"reviewer_note"`

	// Outcome: if approved the original decision is reversed.
	OutcomeStatus ModerationStatus `json:"outcome_status,omitempty" db:"outcome_status"`

	SubmittedAt time.Time `json:"submitted_at" db:"submitted_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// AppealRequest is the inbound payload for filing an appeal.
type AppealRequest struct {
	ResultID    string `json:"result_id" validate:"required"`
	ContentID   string `json:"content_id" validate:"required"`
	AppealText  string `json:"appeal_text" validate:"required,min=20,max=2000"`
	EvidenceURL string `json:"evidence_url,omitempty"`
}

// ModerationStats is returned by the dashboard stats endpoint.
type ModerationStats struct {
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`

	TotalReviewed   int64 `json:"total_reviewed"`
	AutoApproved    int64 `json:"auto_approved"`
	AutoRejected    int64 `json:"auto_rejected"`
	HumanReviewed   int64 `json:"human_reviewed"`
	PendingInQueue  int64 `json:"pending_in_queue"`
	EscalatedCount  int64 `json:"escalated_count"`

	NSFWRejections     int64 `json:"nsfw_rejections"`
	ViolenceRejections int64 `json:"violence_rejections"`
	SpamRejections     int64 `json:"spam_rejections"`

	ActiveAppeals   int64 `json:"active_appeals"`
	AppealsApproved int64 `json:"appeals_approved"`
	AppealsDenied   int64 `json:"appeals_denied"`

	// Average time from submission to decision (seconds).
	AvgReviewTimeSec float64 `json:"avg_review_time_sec"`
}
