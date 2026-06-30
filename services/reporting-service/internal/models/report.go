package models

import "time"

// ReportType classifies what's being reported.
type ReportType string

const (
	ReportTypeVideo     ReportType = "video"
	ReportTypeComment   ReportType = "comment"
	ReportTypeUser      ReportType = "user"
	ReportTypeLivestream ReportType = "livestream"
)

// ReportReason is the stated reason for the report.
type ReportReason string

const (
	ReportReasonSpam          ReportReason = "spam"
	ReportReasonHarassment    ReportReason = "harassment"
	ReportReasonHateSpeech    ReportReason = "hate_speech"
	ReportReasonMisinformation ReportReason = "misinformation"
	ReportReasonNudity        ReportReason = "nudity"
	ReportReasonViolence      ReportReason = "violence"
	ReportReasonCopyright     ReportReason = "copyright"
	ReportReasonOther         ReportReason = "other"
)

// ReportStatus tracks the lifecycle of a report.
type ReportStatus string

const (
	ReportStatusPending   ReportStatus = "pending"
	ReportStatusReviewing ReportStatus = "reviewing"
	ReportStatusResolved  ReportStatus = "resolved"
	ReportStatusDismissed ReportStatus = "dismissed"
)

// Report represents a user-submitted content report.
type Report struct {
	ID           string       `json:"id" db:"id"`
	ReporterID   string       `json:"reporter_id" db:"reporter_id"`
	ContentID    string       `json:"content_id" db:"content_id"`
	ContentType  ReportType   `json:"content_type" db:"content_type"`
	Reason       ReportReason `json:"reason" db:"reason"`
	Description  string       `json:"description,omitempty" db:"description"`
	Status       ReportStatus `json:"status" db:"status"`
	// ResolvedBy is the admin who handled the report.
	ResolvedBy   string       `json:"resolved_by,omitempty" db:"resolved_by"`
	// ResolutionAction: "dismiss" | "remove" | "warn" | "ban"
	ResolutionAction string    `json:"resolution_action,omitempty" db:"resolution_action"`
	ResolutionNotes  string    `json:"resolution_notes,omitempty" db:"resolution_notes"`
	ResolvedAt   *time.Time   `json:"resolved_at,omitempty" db:"resolved_at"`
	CreatedAt    time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at" db:"updated_at"`
}
