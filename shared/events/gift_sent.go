package events

import "time"

const TopicGiftSent = "gift.sent"

type GiftSent struct {
	EventID        string    `json:"event_id"`
	OccurredAt     time.Time `json:"occurred_at"`
	GiftID         string    `json:"gift_id"`
	GiftName       string    `json:"gift_name"`
	GiftValue      float64   `json:"gift_value"`
	Currency       string    `json:"currency"`
	SenderID       string    `json:"sender_id"`
	SenderName     string    `json:"sender_name"`
	SenderAvatar   string    `json:"sender_avatar"`
	RecipientID    string    `json:"recipient_id"`
	RecipientName  string    `json:"recipient_name"`
	RecipientEmail string    `json:"recipient_email"`
	VideoID        string    `json:"video_id"`
	VideoTitle     string    `json:"video_title"`
	DashboardURL   string    `json:"dashboard_url"`
}
