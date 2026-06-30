package models

import "time"

// SourceStrategy identifies how a candidate was generated.
type SourceStrategy string

const (
	SourceCollaborativeFiltering SourceStrategy = "collaborative_filtering"
	SourceContentBased           SourceStrategy = "content_based"
	SourceTrending               SourceStrategy = "trending"
	SourceFollowingNetwork       SourceStrategy = "following_network"
	SourceRecentInteractionGraph SourceStrategy = "recent_interaction_graph"
)

// DeviceType classifies the viewer's device for feature engineering.
type DeviceType string

const (
	DeviceMobile  DeviceType = "mobile"
	DeviceTablet  DeviceType = "tablet"
	DeviceDesktop DeviceType = "desktop"
	DeviceTV      DeviceType = "tv"
	DeviceUnknown DeviceType = "unknown"
)

// InteractionType represents user engagement with a video.
type InteractionType string

const (
	InteractionView     InteractionType = "view"
	InteractionLike     InteractionType = "like"
	InteractionComment  InteractionType = "comment"
	InteractionShare    InteractionType = "share"
	InteractionBookmark InteractionType = "bookmark"
	InteractionFollow   InteractionType = "follow" // follow triggered from video
	InteractionSkip     InteractionType = "skip"
)

// EngagementWeights defines the weight of each interaction type when computing
// an engagement score. Higher-intent signals receive larger weights.
var EngagementWeights = map[InteractionType]float64{
	InteractionView:     0.1,
	InteractionLike:     0.6,
	InteractionComment:  0.8,
	InteractionShare:    1.0,
	InteractionBookmark: 0.7,
	InteractionFollow:   1.0,
	InteractionSkip:     -0.2,
}

// -----------------------------------------------------------------
// UserFeatures
// -----------------------------------------------------------------

// UserFeatures represents the feature vector for a single user, populated
// from the feature store at request time.
type UserFeatures struct {
	UserID string `json:"user_id"`

	// WatchHistory holds the last N video IDs watched by the user (most recent first).
	WatchHistory []string `json:"watch_history"`

	// LikedCategories maps category name → normalised affinity score [0, 1].
	LikedCategories map[string]float64 `json:"liked_categories"`

	// FollowedCreators is the set of creator IDs the user follows.
	FollowedCreators []string `json:"followed_creators"`

	// Embedding is the user's latent representation derived from interaction history.
	Embedding []float64 `json:"embedding,omitempty"`

	// Device / context features.
	DeviceType DeviceType `json:"device_type"`
	CountryCode string    `json:"country_code"`
	LanguageCode string   `json:"language_code"`
	Timezone    string    `json:"timezone"`

	// DaypartIndex encodes the time of day (0 = midnight, 23 = 11 pm).
	DaypartIndex int `json:"daypart_index"`

	// IsNewUser is true when the user has fewer than MinInteractionsForItem total events.
	IsNewUser bool `json:"is_new_user"`

	// RetrievedAt is the timestamp when this feature set was fetched.
	RetrievedAt time.Time `json:"retrieved_at"`
}

// -----------------------------------------------------------------
// VideoFeatures
// -----------------------------------------------------------------

// VideoFeatures represents the feature vector for a single video.
type VideoFeatures struct {
	VideoID   string `json:"video_id"`
	CreatorID string `json:"creator_id"`

	// Category is the primary content category (e.g., "comedy", "education").
	Category string `json:"category"`

	// Tags are secondary descriptors.
	Tags []string `json:"tags"`

	// Embedding is the video's latent content representation.
	Embedding []float64 `json:"embedding,omitempty"`

	// EngagementRate = (likes + comments*2 + shares*3) / views, clamped to [0, 1].
	EngagementRate float64 `json:"engagement_rate"`

	// ViewCount is the total number of plays.
	ViewCount int64 `json:"view_count"`

	// LikeCount is the total number of likes.
	LikeCount int64 `json:"like_count"`

	// CommentCount is the total number of comments.
	CommentCount int64 `json:"comment_count"`

	// ShareCount is the total number of shares.
	ShareCount int64 `json:"share_count"`

	// Duration is the video length in seconds.
	Duration float64 `json:"duration"`

	// Language of the video transcript / description.
	LanguageCode string `json:"language_code"`

	// PublishedAt is when the video was made publicly available.
	PublishedAt time.Time `json:"published_at"`

	// TrendingScore is a pre-computed signal refreshed periodically by the
	// analytics pipeline.
	TrendingScore float64 `json:"trending_score"`

	// IsActive indicates the video is not deleted, not moderated-off, and not
	// age-restricted relative to the requesting user.
	IsActive bool `json:"is_active"`
}

// -----------------------------------------------------------------
// CandidateVideo
// -----------------------------------------------------------------

// CandidateVideo is a video that has been retrieved from at least one
// candidate-generation strategy and is awaiting ranking.
type CandidateVideo struct {
	VideoID   string `json:"video_id"`
	CreatorID string `json:"creator_id"`

	// Source records which strategy produced this candidate.
	Source SourceStrategy `json:"source"`

	// RetrievalScore is the raw score assigned during candidate retrieval
	// (e.g., CF similarity, KNN distance, trending score).
	RetrievalScore float64 `json:"retrieval_score"`

	// Features are populated lazily before ranking.
	Features *VideoFeatures `json:"features,omitempty"`
}

// -----------------------------------------------------------------
// RankedResult
// -----------------------------------------------------------------

// RankedResult is a fully scored and ordered feed item ready to be served.
type RankedResult struct {
	VideoID   string `json:"video_id"`
	CreatorID string `json:"creator_id"`

	// FinalScore is the output of the fine-ranker; higher is better.
	FinalScore float64 `json:"final_score"`

	// CoarseScore is the intermediate score from the coarse ranker.
	CoarseScore float64 `json:"coarse_score"`

	// EngagementScore is the component derived from the video's engagement rate.
	EngagementScore float64 `json:"engagement_score"`

	// FreshnessScore decays exponentially from publish time.
	FreshnessScore float64 `json:"freshness_score"`

	// RelevanceScore is the cosine similarity between the user and video embeddings.
	RelevanceScore float64 `json:"relevance_score"`

	// Source records how this video was originally generated as a candidate.
	Source SourceStrategy `json:"source"`

	// Position is the 0-based index in the final ranked list.
	Position int `json:"position"`

	// ExperimentID is non-empty when this result was produced under an A/B experiment.
	ExperimentID string `json:"experiment_id,omitempty"`

	// VideoFeatures are the full features used during ranking (useful for logging).
	VideoFeatures *VideoFeatures `json:"video_features,omitempty"`
}

// -----------------------------------------------------------------
// RecommendationRequest / Response
// -----------------------------------------------------------------

// RecommendationRequest carries all inputs for a single feed generation call.
type RecommendationRequest struct {
	UserID string `json:"user_id" validate:"required"`

	// PageSize is how many items to return; defaults to the configured FinalFeedSize.
	PageSize int `json:"page_size" validate:"min=1,max=100"`

	// Cursor is an opaque token for paginating through previously seen results.
	Cursor string `json:"cursor,omitempty"`

	// Context contains situational signals injected at request time.
	Context RequestContext `json:"context"`
}

// RequestContext captures per-request situational signals.
type RequestContext struct {
	DeviceType  DeviceType `json:"device_type"`
	CountryCode string     `json:"country_code"`
	LanguageCode string    `json:"language_code"`
	Timezone    string     `json:"timezone"`
	// ClientTime is the local time on the user's device.
	ClientTime  time.Time  `json:"client_time"`
	// AppVersion can be used for gradual rollout gating.
	AppVersion  string     `json:"app_version"`
}

// RecommendationResponse is the payload returned to the caller.
type RecommendationResponse struct {
	UserID   string          `json:"user_id"`
	Items    []*RankedResult `json:"items"`
	// NextCursor is set when more results are available.
	NextCursor   string `json:"next_cursor,omitempty"`
	ExperimentID string `json:"experiment_id,omitempty"`
	GeneratedAt  time.Time `json:"generated_at"`
}

// -----------------------------------------------------------------
// Impression / Feedback events
// -----------------------------------------------------------------

// ImpressionEvent records that a video was shown to a user.
type ImpressionEvent struct {
	UserID      string    `json:"user_id"`
	VideoID     string    `json:"video_id"`
	Position    int       `json:"position"`
	Source      SourceStrategy `json:"source"`
	ExperimentID string   `json:"experiment_id,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

// FeedbackEvent records explicit or implicit user engagement after impression.
type FeedbackEvent struct {
	UserID          string          `json:"user_id"`
	VideoID         string          `json:"video_id"`
	Interaction     InteractionType `json:"interaction"`
	WatchPercentage float64         `json:"watch_percentage"` // [0, 1]
	ExperimentID    string          `json:"experiment_id,omitempty"`
	Timestamp       time.Time       `json:"timestamp"`
}

// EngagementEvent is the normalised Kafka message produced for every user action.
// Workers downstream consume this to update the collaborative-filtering matrix.
type EngagementEvent struct {
	UserID    string          `json:"user_id"`
	VideoID   string          `json:"video_id"`
	CreatorID string          `json:"creator_id"`
	Type      InteractionType `json:"type"`
	Score     float64         `json:"score"` // pre-weighted by EngagementWeights
	Timestamp time.Time       `json:"timestamp"`
}

// -----------------------------------------------------------------
// Collaborative filtering internals
// -----------------------------------------------------------------

// ItemSimilarityEntry represents a single neighbour in the item-item CF matrix.
type ItemSimilarityEntry struct {
	VideoID    string  `json:"video_id"`
	Similarity float64 `json:"similarity"` // Jaccard or cosine of co-occurrence vectors
}

// CFMatrix maps each video ID to its top-K most similar videos.
type CFMatrix map[string][]ItemSimilarityEntry
