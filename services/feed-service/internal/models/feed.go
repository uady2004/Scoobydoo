// Package models defines the domain types used throughout the feed-service.
// All types are serialisation-agnostic; JSON tags exist only for convenience
// when marshalling to/from Redis and HTTP responses.
package models

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"time"
)

// ---- Feed types -------------------------------------------------------------

// FeedType enumerates the feed variants the service can produce.
type FeedType string

const (
	// FeedTypeForYou is the personalised recommendation feed.
	FeedTypeForYou FeedType = "for_you"
	// FeedTypeFollowing shows content from accounts the user follows.
	FeedTypeFollowing FeedType = "following"
	// FeedTypeTrending shows globally trending videos.
	FeedTypeTrending FeedType = "trending"
	// FeedTypeNearby shows videos created near the user's geolocation.
	FeedTypeNearby FeedType = "nearby"
	// FeedTypeExplore shows category-based discovery content.
	FeedTypeExplore FeedType = "explore"
	// FeedTypeCategory shows content for a specific category.
	FeedTypeCategory FeedType = "category"
)

// ---- Author / User stub -----------------------------------------------------

// Author is a lightweight author representation embedded in FeedItem.
// The full user profile is owned by the user-service.
type Author struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url,omitempty"`
	IsVerified  bool   `json:"is_verified"`
	IsFollowing bool   `json:"is_following,omitempty"`
}

// ---- Video statistics -------------------------------------------------------

// VideoStats holds the engagement counters for a video.
type VideoStats struct {
	Views    int64 `json:"views"`
	Likes    int64 `json:"likes"`
	Comments int64 `json:"comments"`
	Shares   int64 `json:"shares"`
	// TrendingScore is the computed sliding-window trending score.
	// views*0.4 + likes*0.3 + shares*0.2 + comments*0.1, with time decay.
	TrendingScore float64 `json:"trending_score,omitempty"`
}

// ---- Geolocation ------------------------------------------------------------

// GeoPoint is a WGS-84 coordinate pair used for nearby feeds.
type GeoPoint struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// ---- FeedItem ---------------------------------------------------------------

// FeedItem represents a single entry in any feed type. It carries enough
// information for the client to render the video card without requiring a
// separate video-detail call for the common case.
type FeedItem struct {
	// VideoID is the globally unique video identifier.
	VideoID string `json:"video_id"`
	// Author is a lightweight representation of the video creator.
	Author Author `json:"author"`
	// Title is the video title (may be empty for untitled clips).
	Title string `json:"title,omitempty"`
	// Description is the video caption / description.
	Description string `json:"description,omitempty"`
	// ThumbnailURL is the CDN URL for the video thumbnail image.
	ThumbnailURL string `json:"thumbnail_url"`
	// VideoURL is the CDN URL for the HLS/DASH manifest or direct MP4.
	VideoURL string `json:"video_url"`
	// Duration is the video duration in seconds.
	Duration float64 `json:"duration"`
	// Stats holds engagement counters.
	Stats VideoStats `json:"stats"`
	// Tags is the list of hashtags associated with the video.
	Tags []string `json:"tags,omitempty"`
	// Category is the primary content category (e.g. "comedy", "sports").
	Category string `json:"category,omitempty"`
	// Location is the geolocation of the video creator (optional).
	Location *GeoPoint `json:"location,omitempty"`
	// DistanceKm is the distance to the user in km; only set for nearby feeds.
	DistanceKm *float64 `json:"distance_km,omitempty"`
	// FeedScore is the internal ranking score used to order this item in the
	// feed. Not exposed to end-users but useful for debugging.
	FeedScore float64 `json:"feed_score,omitempty"`
	// FeedType records which feed variant this item came from (used for
	// de-duplication auditing).
	FeedType FeedType `json:"feed_type"`
	// IsLiked indicates whether the requesting user has liked this video.
	IsLiked bool `json:"is_liked"`
	// IsSaved indicates whether the requesting user has saved this video.
	IsSaved bool `json:"is_saved"`
	// CreatedAt is when the video was published.
	CreatedAt time.Time `json:"created_at"`
	// FeaturedAt is when this item was added to this particular feed.
	FeaturedAt time.Time `json:"featured_at"`
}

// ---- FeedCursor -------------------------------------------------------------

// FeedCursor is the opaque pagination token for cursor-based feed traversal.
// It encodes the score (for Redis ZRANGEBYSCORE) and the last video ID seen so
// that the next page can be fetched without offset drift.
//
// Clients receive it as a base64-encoded JSON blob in the "next_cursor" field
// of FeedPage. They must pass it verbatim as the "cursor" query parameter.
type FeedCursor struct {
	// Score is the Redis sorted-set score (typically a Unix timestamp or a
	// trending score multiplied by 1e6 for precision).
	Score float64 `json:"score"`
	// VideoID is the last video ID on the current page, used to break ties
	// when multiple entries share the same score.
	VideoID string `json:"video_id"`
	// FeedType records which feed type this cursor belongs to so the handler
	// can reject mismatched cursors quickly.
	FeedType FeedType `json:"feed_type"`
	// Timestamp is when the cursor was issued (used for TTL sanity checks).
	Timestamp time.Time `json:"ts"`
}

// Encode serialises the cursor to an opaque base64 URL-safe string.
func (c *FeedCursor) Encode() (string, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("feed cursor encode: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// DecodeFeedCursor decodes an opaque cursor string produced by Encode.
// Returns nil cursor and nil error when token is empty (first page).
func DecodeFeedCursor(token string) (*FeedCursor, error) {
	if token == "" {
		return nil, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return nil, fmt.Errorf("feed cursor decode base64: %w", err)
	}
	var c FeedCursor
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("feed cursor decode json: %w", err)
	}
	return &c, nil
}

// ---- FeedPage ---------------------------------------------------------------

// FeedPage is the top-level HTTP response for all feed endpoints.
type FeedPage struct {
	// Items contains the ordered list of feed entries for this page.
	Items []*FeedItem `json:"items"`
	// NextCursor is the opaque token the client must pass to fetch the next
	// page. Empty string means there are no more results.
	NextCursor string `json:"next_cursor,omitempty"`
	// HasMore indicates whether additional pages exist.
	HasMore bool `json:"has_more"`
	// Count is the number of items in this page.
	Count int `json:"count"`
	// FeedType is the feed variant that produced this page.
	FeedType FeedType `json:"feed_type"`
	// GeneratedAt is when this page was assembled.
	GeneratedAt time.Time `json:"generated_at"`
}

// ---- FeedRequest ------------------------------------------------------------

// FeedRequest carries all parameters needed to fetch a feed page.
// It is populated by the handler from the HTTP query string and path context.
type FeedRequest struct {
	// UserID is the authenticated user requesting the feed.
	UserID string
	// FeedType is which feed variant to return.
	FeedType FeedType
	// Cursor is the opaque token from the previous page (empty = first page).
	Cursor string
	// Limit is the number of items to return; bounded by MaxPageSize.
	Limit int
	// Category filters the explore / category feeds.
	Category string
	// Latitude / Longitude are used for nearby feeds.
	Latitude  float64
	Longitude float64
	// RadiusKm is the search radius for nearby feeds.
	RadiusKm float64
	// SessionID is used as a key component for per-session deduplication.
	SessionID string
	// Language preference (e.g. "en", "zh") for content filtering.
	Language string
}

// ---- TrendingEntry ----------------------------------------------------------

// TrendingEntry is stored in the Redis sorted set that backs the trending feed.
// The member is VideoID; the Score is the computed trending score.
type TrendingEntry struct {
	VideoID       string    `json:"video_id"`
	Category      string    `json:"category,omitempty"`
	Score         float64   `json:"score"`
	Views         int64     `json:"views"`
	Likes         int64     `json:"likes"`
	Comments      int64     `json:"comments"`
	Shares        int64     `json:"shares"`
	CreatedAt     time.Time `json:"created_at"`
	LastUpdatedAt time.Time `json:"last_updated_at"`
}

// TrendingScore computes the sliding-window engagement score using the formula:
//
//	raw   = views*0.4 + likes*0.3 + shares*0.2 + comments*0.1
//	score = raw / (age_hours + 2)^1.5
//
// The "+2" bias prevents division by zero for brand-new content and also
// gives new videos a small initial boost. Decay is gravity-based (Hacker News).
func TrendingScore(views, likes, shares, comments int64, ageHours float64) float64 {
	raw := float64(views)*0.4 + float64(likes)*0.3 + float64(shares)*0.2 + float64(comments)*0.1
	if ageHours < 0 {
		ageHours = 0
	}
	base := ageHours + 2.0
	// pow(base, 1.5) = base * sqrt(base)
	decay := base * math.Sqrt(base)
	if decay <= 0 {
		return raw
	}
	return raw / decay
}

// ---- VideoEvent (internal) --------------------------------------------------

// VideoEvent is the internal representation of a Kafka event consumed by the
// trending updater worker.
type VideoEvent struct {
	EventType  string    `json:"event_type"` // "view" | "like" | "share" | "comment"
	VideoID    string    `json:"video_id"`
	UserID     string    `json:"user_id"`
	OccurredAt time.Time `json:"occurred_at"`
}

// ---- CategoryFeed -----------------------------------------------------------

// CategoryInfo describes a content category for explore feeds.
type CategoryInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	IconURL     string `json:"icon_url,omitempty"`
	VideoCount  int64  `json:"video_count"`
}

// ---- Precompute metadata ----------------------------------------------------

// PrecomputeMeta is stored alongside a pre-computed feed in Redis so the
// precompute worker can determine whether the cached feed is still fresh.
type PrecomputeMeta struct {
	UserID     string    `json:"user_id"`
	FeedType   FeedType  `json:"feed_type"`
	ComputedAt time.Time `json:"computed_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	VideoCount int       `json:"video_count"`
}
