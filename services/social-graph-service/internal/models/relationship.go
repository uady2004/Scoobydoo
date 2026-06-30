package models

import "time"

// ---------------------------------------------------------------------------
// Relationship status
// ---------------------------------------------------------------------------

// RelationshipStatus captures the relationship state between two users.
type RelationshipStatus string

const (
	// RelationshipNone means neither party follows or blocks the other.
	RelationshipNone RelationshipStatus = "none"
	// RelationshipFollowing means the viewer follows the target.
	RelationshipFollowing RelationshipStatus = "following"
	// RelationshipFollowedBy means the target follows the viewer.
	RelationshipFollowedBy RelationshipStatus = "followed_by"
	// RelationshipMutual means both parties follow each other.
	RelationshipMutual RelationshipStatus = "mutual"
	// RelationshipBlocked means the viewer has blocked the target.
	RelationshipBlocked RelationshipStatus = "blocked"
	// RelationshipBlockedBy means the target has blocked the viewer.
	RelationshipBlockedBy RelationshipStatus = "blocked_by"
)

// ---------------------------------------------------------------------------
// Core graph edge types
// ---------------------------------------------------------------------------

// Follow represents a directed follow edge: FollowerID → FolloweeID.
// The edge is stored in the follows table with a unique constraint on
// (follower_id, followee_id).
type Follow struct {
	ID         int64     `json:"id"          db:"id"`
	FollowerID string    `json:"follower_id" db:"follower_id"`
	FolloweeID string    `json:"followee_id" db:"followee_id"`
	CreatedAt  time.Time `json:"created_at"  db:"created_at"`
}

// Block represents a user blocking another user. Blocking implicitly removes
// any follow edges between the two users in both directions.
type Block struct {
	ID        int64     `json:"id"         db:"id"`
	BlockerID string    `json:"blocker_id" db:"blocker_id"`
	BlockedID string    `json:"blocked_id" db:"blocked_id"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// ---------------------------------------------------------------------------
// Rich aggregate views
// ---------------------------------------------------------------------------

// Relationship is a rich view of the link between two users, returned by
// CheckRelationship. It aggregates follow and block state so callers never
// have to make multiple round-trips.
type Relationship struct {
	UserID   string             `json:"user_id"`
	TargetID string             `json:"target_id"`
	Status   RelationshipStatus `json:"status"`

	// IsFollowing is true when UserID follows TargetID.
	IsFollowing bool `json:"is_following"`
	// IsFollowedBy is true when TargetID follows UserID.
	IsFollowedBy bool `json:"is_followed_by"`
	// IsBlocking is true when UserID has blocked TargetID.
	IsBlocking bool `json:"is_blocking"`
	// IsBlockedBy is true when TargetID has blocked UserID.
	IsBlockedBy bool `json:"is_blocked_by"`
	// MutualFollowerCount is the number of users who follow both parties.
	MutualFollowerCount int `json:"mutual_follower_count"`
}

// DeriveStatus computes the RelationshipStatus from the boolean fields and
// ensures the Status field is always consistent with the booleans. Blocks
// take precedence over follows.
func (r *Relationship) DeriveStatus() {
	switch {
	case r.IsBlocking:
		r.Status = RelationshipBlocked
	case r.IsBlockedBy:
		r.Status = RelationshipBlockedBy
	case r.IsFollowing && r.IsFollowedBy:
		r.Status = RelationshipMutual
	case r.IsFollowing:
		r.Status = RelationshipFollowing
	case r.IsFollowedBy:
		r.Status = RelationshipFollowedBy
	default:
		r.Status = RelationshipNone
	}
}

// ---------------------------------------------------------------------------
// User projections
// ---------------------------------------------------------------------------

// UserSummary is the minimal user projection attached to follow/suggestion
// responses. The full user profile is owned by the user-service; this struct
// only carries the fields the social-graph-service needs in the hot path
// without cross-service calls.
type UserSummary struct {
	UserID      string `json:"user_id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url"`
	IsVerified  bool   `json:"is_verified"`
}

// FollowWithUser wraps a Follow with the enriched follower/followee profile
// and an indicator of whether the viewer already follows the listed user.
type FollowWithUser struct {
	Follow      Follow      `json:"follow"`
	User        UserSummary `json:"user"`
	// IsFollowing indicates whether the authenticated viewer follows this user.
	IsFollowing bool `json:"is_following"`
}

// ---------------------------------------------------------------------------
// Suggestion types
// ---------------------------------------------------------------------------

// FriendSuggestion represents a single suggested user plus the scoring
// metadata that drove the recommendation.
type FriendSuggestion struct {
	User UserSummary `json:"user"`

	// MutualFollowerCount is the number of users that both the viewer and the
	// candidate follow — the primary BFS signal.
	MutualFollowerCount int `json:"mutual_follower_count"`

	// MutualFollowers holds up to 3 mutual followers shown as social proof.
	MutualFollowers []UserSummary `json:"mutual_followers,omitempty"`

	// Score is the normalised recommendation score (0.0–1.0). Higher is
	// better. Combines mutual-follower count with an optional ML score.
	Score float64 `json:"score"`

	// MLScore is the raw score returned by the ML recommendation model, if
	// available. 0 means the ML layer was not consulted.
	MLScore float64 `json:"ml_score,omitempty"`

	// Reason is a human-readable label e.g. "5 mutual followers".
	Reason string `json:"reason"`
}

// ---------------------------------------------------------------------------
// Kafka event payload
// ---------------------------------------------------------------------------

// FollowEvent is the Kafka message payload emitted when a follow or unfollow
// occurs. It is shared between the producer (social_service) and the consumer
// (event_processor). The event_type field distinguishes the two cases.
type FollowEvent struct {
	EventType  string    `json:"event_type"`  // "followed" | "unfollowed"
	FollowerID string    `json:"follower_id"`
	FolloweeID string    `json:"followee_id"`
	OccurredAt time.Time `json:"occurred_at"`
}

// ---------------------------------------------------------------------------
// Pagination & response envelopes
// ---------------------------------------------------------------------------

// PaginationMeta carries offset-based pagination metadata in list responses.
type PaginationMeta struct {
	Total      int64  `json:"total"`
	Limit      int    `json:"limit"`
	Offset     int    `json:"offset"`
	NextCursor string `json:"next_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
}

// FollowRequest is the JSON body for POST /follow and DELETE /follow.
type FollowRequest struct {
	FolloweeID string `json:"followee_id" binding:"required"`
}

// FollowListResponse wraps a slice of FollowWithUser with pagination metadata.
type FollowListResponse struct {
	Users      []FollowWithUser `json:"users"`
	Pagination PaginationMeta   `json:"pagination"`
}

// SuggestionListResponse wraps a slice of FriendSuggestion with a generation
// timestamp so clients can implement staleness logic.
type SuggestionListResponse struct {
	Suggestions []FriendSuggestion `json:"suggestions"`
	GeneratedAt time.Time          `json:"generated_at"`
}

// ---------------------------------------------------------------------------
// Counter snapshot
// ---------------------------------------------------------------------------

// CounterSnapshot holds pre-aggregated follower/following counts for a user.
// These are maintained in Redis and used to avoid costly COUNT(*) queries.
type CounterSnapshot struct {
	UserID         string    `json:"user_id"`
	FollowerCount  int64     `json:"follower_count"`
	FollowingCount int64     `json:"following_count"`
	UpdatedAt      time.Time `json:"updated_at"`
}
