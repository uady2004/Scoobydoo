package models

import "time"

// Like represents a user liking a video.
type Like struct {
	ID        string    `json:"id" db:"id"`
	UserID    string    `json:"user_id" db:"user_id"`
	VideoID   string    `json:"video_id" db:"video_id"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// Comment represents a comment on a video.
type Comment struct {
	ID          string    `json:"id" db:"id"`
	VideoID     string    `json:"video_id" db:"video_id"`
	UserID      string    `json:"user_id" db:"user_id"`
	Username    string    `json:"username" db:"username"`
	AvatarURL   string    `json:"avatar_url" db:"avatar_url"`
	Content     string    `json:"content" db:"content"`
	ParentID    string    `json:"parent_id,omitempty" db:"parent_id"`
	LikeCount   int64     `json:"like_count" db:"like_count"`
	ReplyCount  int64     `json:"reply_count" db:"reply_count"`
	IsPinned    bool      `json:"is_pinned" db:"is_pinned"`
	IsDeleted   bool      `json:"is_deleted" db:"is_deleted"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// ContentReport represents a user-submitted report for a piece of content.
type ContentReport struct {
	ID          string    `json:"id" db:"id"`
	ContentType string    `json:"content_type" db:"content_type"`
	ContentID   string    `json:"content_id" db:"content_id"`
	ReporterID  string    `json:"reporter_id" db:"reporter_id"`
	Reason      string    `json:"reason" db:"reason"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// CommentLike represents a user liking a comment.
type CommentLike struct {
	ID        string    `json:"id" db:"id"`
	UserID    string    `json:"user_id" db:"user_id"`
	CommentID string    `json:"comment_id" db:"comment_id"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// Bookmark represents a user saving a video.
type Bookmark struct {
	ID          string    `json:"id" db:"id"`
	UserID      string    `json:"user_id" db:"user_id"`
	VideoID     string    `json:"video_id" db:"video_id"`
	CollectionID string   `json:"collection_id,omitempty" db:"collection_id"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// BookmarkCollection is a named folder for saved videos.
type BookmarkCollection struct {
	ID          string    `json:"id" db:"id"`
	UserID      string    `json:"user_id" db:"user_id"`
	Name        string    `json:"name" db:"name"`
	VideoCount  int64     `json:"video_count" db:"video_count"`
	IsPrivate   bool      `json:"is_private" db:"is_private"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}
