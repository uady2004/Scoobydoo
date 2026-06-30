package events

import "time"

const TopicCommentCreated = "comment.created"

type CommentCreated struct {
	EventID        string    `json:"event_id"`
	OccurredAt     time.Time `json:"occurred_at"`
	CommentID      string    `json:"comment_id"`
	VideoID        string    `json:"video_id"`
	VideoOwnerID   string    `json:"video_owner_id"`
	AuthorID       string    `json:"author_id"`
	AuthorName     string    `json:"author_name"`
	AuthorAvatar   string    `json:"author_avatar"`
	ParentID       string    `json:"parent_id,omitempty"`
	Text           string    `json:"text"`
	CommentPreview string    `json:"comment_preview"`
}
