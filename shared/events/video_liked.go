package events

import "time"

const TopicVideoLiked = "video.liked"

type VideoLiked struct {
	EventID      string    `json:"event_id"`
	OccurredAt   time.Time `json:"occurred_at"`
	VideoID      string    `json:"video_id"`
	VideoOwnerID string    `json:"video_owner_id"`
	VideoTitle   string    `json:"video_title"`
	ActorID      string    `json:"actor_id"`
	ActorName    string    `json:"actor_name"`
	ActorAvatar  string    `json:"actor_avatar"`
}
