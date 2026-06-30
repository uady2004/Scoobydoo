package events

import "time"

const TopicUserRegistered = "user.registered"

type UserRegistered struct {
	EventID    string    `json:"event_id"`
	OccurredAt time.Time `json:"occurred_at"`
	UserID     string    `json:"user_id"`
	Username   string    `json:"username"`
	Email      string    `json:"email"`
	Provider   string    `json:"provider"`
}
