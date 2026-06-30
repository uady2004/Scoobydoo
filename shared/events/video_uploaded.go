package events

import "time"

const TopicVideoUploaded = "video.uploaded"

type VideoUploaded struct {
	EventID     string    `json:"event_id"`
	OccurredAt  time.Time `json:"occurred_at"`
	VideoID     string    `json:"video_id"`
	UploaderID  string    `json:"uploader_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Duration    float64   `json:"duration_seconds"`
	StorageKey  string    `json:"storage_key"`
	ThumbnailKey string   `json:"thumbnail_key"`
	Tags        []string  `json:"tags"`
}
