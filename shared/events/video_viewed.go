package events

import "time"

const TopicVideoViewed = "video.viewed"

type VideoViewed struct {
	EventID     string    `json:"event_id"`
	OccurredAt  time.Time `json:"occurred_at"`
	VideoID     string    `json:"video_id"`
	OwnerID     string    `json:"owner_id"`
	ViewerID    string    `json:"viewer_id,omitempty"`
	WatchedSec  float64   `json:"watched_seconds"`
	Completed   bool      `json:"completed"`
	Source      string    `json:"source"`
	DeviceType  string    `json:"device_type"`
	Country     string    `json:"country"`
}
