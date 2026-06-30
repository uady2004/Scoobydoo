package events

import "time"

const TopicUserFollowed = "user.followed"

type UserFollowed struct {
	EventID        string    `json:"event_id"`
	OccurredAt     time.Time `json:"occurred_at"`
	FollowerID     string    `json:"follower_id"`
	FollowerName   string    `json:"follower_name"`
	FollowerAvatar string    `json:"follower_avatar"`
	FollowedUserID string    `json:"followed_user_id"`
}
