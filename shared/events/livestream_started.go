package events

import "time"

const TopicLivestreamStarted = "livestream.started"

type LivestreamStarted struct {
	EventID      string    `json:"event_id"`
	OccurredAt   time.Time `json:"occurred_at"`
	StreamID     string    `json:"stream_id"`
	StreamerID   string    `json:"streamer_id"`
	StreamerName string    `json:"streamer_name"`
	StreamerAvatar string  `json:"streamer_avatar"`
	Title        string    `json:"title"`
	FollowerIDs  []string  `json:"follower_ids"`
}
