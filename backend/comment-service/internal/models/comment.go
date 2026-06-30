package models

import "time"

type Comment struct {
	ID          string    `json:"id"`
	VideoID     string    `json:"video_id"`
	UserID      string    `json:"user_id"`
	Username    string    `json:"username"`
	AvatarURL   string    `json:"avatar_url"`
	Content     string    `json:"content"`
	ParentID    string    `json:"parent_id,omitempty"`
	LikeCount   int64     `json:"like_count"`
	ReplyCount  int64     `json:"reply_count"`
	IsDeleted   bool      `json:"is_deleted"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CommentLike struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	CommentID string    `json:"comment_id"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateCommentReq struct {
	VideoID   string `json:"video_id"   binding:"required"`
	UserID    string `json:"-"`
	Username  string `json:"username"`
	AvatarURL string `json:"avatar_url"`
	Content   string `json:"content"    binding:"required,min=1,max=1000"`
	ParentID  string `json:"parent_id"`
}

type ListCommentsResp struct {
	Comments []*Comment `json:"comments"`
	Total    int        `json:"total"`
}
