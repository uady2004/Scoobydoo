package models

import "time"

type Like struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	VideoID   string    `json:"video_id"`
	CreatedAt time.Time `json:"created_at"`
}

type VideoLikeCount struct {
	VideoID   string `json:"video_id"`
	LikeCount int64  `json:"like_count"`
}

type BatchLikeStatusReq struct {
	VideoIDs []string `json:"video_ids" binding:"required,min=1,max=100"`
}

type BatchLikeStatusResp struct {
	Statuses map[string]bool `json:"statuses"`
}
