// Package constants defines shared identifiers used across all TikTok-clone
// microservices: Kafka topic names, service names, HTTP header names, context
// keys, default pagination limits, and canonical error codes.
package constants

import "time"

// ---- Service names ------------------------------------------------------------

const (
	ServiceAuth           = "auth-service"
	ServiceUser           = "user-service"
	ServiceVideo          = "video-service"
	ServiceFeed           = "feed-service"
	ServiceInteraction    = "interaction-service"
	ServiceComment        = "comment-service"
	ServiceLike           = "like-service"
	ServiceNotification   = "notification-service"
	ServiceMessaging      = "messaging-service"
	ServiceSearch         = "search-service"
	ServiceRecommendation = "recommendation-service"
	ServiceAnalytics      = "analytics-service"
	ServiceAds            = "ads-service"
	ServicePayment        = "payment-service"
	ServiceWallet         = "wallet-service"
	ServiceEcommerce      = "ecommerce-service"
	ServiceLivestream     = "livestream-service"
	ServiceSocialGraph    = "social-graph-service"
	ServiceModeration     = "moderation-service"
	ServiceAdmin          = "admin-service"
	ServiceAPIGateway     = "api-gateway"
)

// ---- Kafka topics -------------------------------------------------------------

const (
	TopicUserRegistered     = "user.registered"
	TopicUserDeleted        = "user.deleted"
	TopicUserFollowed       = "user.followed"
	TopicUserUnfollowed     = "user.unfollowed"
	TopicUserProfileUpdated = "user.profile.updated"

	TopicVideoUploaded  = "video.uploaded"
	TopicVideoPublished = "video.published"
	TopicVideoDeleted   = "video.deleted"
	TopicVideoViewed    = "video.viewed"
	TopicVideoLiked     = "video.liked"
	TopicVideoUnliked   = "video.unliked"

	TopicCommentCreated = "comment.created"
	TopicCommentDeleted = "comment.deleted"
	TopicCommentLiked   = "comment.liked"

	TopicGiftSent = "gift.sent"

	TopicLivestreamStarted = "livestream.started"
	TopicLivestreamEnded   = "livestream.ended"

	TopicOrderCreated    = "order.created"
	TopicOrderCancelled  = "order.cancelled"
	TopicOrderShipped    = "order.shipped"
	TopicOrderDelivered  = "order.delivered"
	TopicPaymentCompleted = "payment.completed"
	TopicPaymentFailed   = "payment.failed"
	TopicRefundIssued    = "refund.issued"

	TopicNotificationPush  = "notification.push"
	TopicNotificationEmail = "notification.email"
	TopicNotificationSMS   = "notification.sms"

	TopicModerationFlag    = "moderation.flag"
	TopicModerationReview  = "moderation.review"
	TopicModerationRemoved = "moderation.removed"

	TopicSearchIndex  = "search.index"
	TopicSearchDelete = "search.delete"

	TopicAdImpression = "ad.impression"
	TopicAdClick      = "ad.click"
)

// ---- HTTP headers -------------------------------------------------------------

const (
	HeaderAuthorization  = "Authorization"
	HeaderContentType    = "Content-Type"
	HeaderAccept         = "Accept"
	HeaderXUserID        = "X-User-ID"
	HeaderXUsername      = "X-Username"
	HeaderXRequestID     = "X-Request-ID"
	HeaderXCorrelationID = "X-Correlation-ID"
	HeaderXForwardedFor  = "X-Forwarded-For"
	HeaderXRealIP        = "X-Real-IP"
	HeaderXRateLimitLimit     = "X-RateLimit-Limit"
	HeaderXRateLimitRemaining = "X-RateLimit-Remaining"
	HeaderXRateLimitReset     = "X-RateLimit-Reset"
	HeaderCacheControl   = "Cache-Control"
	HeaderETag           = "ETag"
)

// Content-Type values.
const (
	ContentTypeJSON           = "application/json"
	ContentTypeFormURLEncoded = "application/x-www-form-urlencoded"
	ContentTypeMultipartForm  = "multipart/form-data"
	ContentTypeOctetStream    = "application/octet-stream"
)

// ---- Pagination ---------------------------------------------------------------

const (
	DefaultPageSize = 20
	MaxPageSize     = 100
	MinPageSize     = 1
)

// ---- Cache TTLs ---------------------------------------------------------------

const (
	TTLUserProfile    = 5 * time.Minute
	TTLVideoMeta      = 2 * time.Minute
	TTLFeedPage       = 30 * time.Second
	TTLSearchResults  = 1 * time.Minute
	TTLLikeCount      = 10 * time.Second
	TTLFollowerCount  = 1 * time.Minute
	TTLSession        = 24 * time.Hour
	TTLOTPCode        = 10 * time.Minute
	TTLRateLimit      = 1 * time.Minute
)

// ---- Role names ---------------------------------------------------------------

const (
	RoleUser      = "user"
	RoleCreator   = "creator"
	RoleModerator = "moderator"
	RoleAdmin     = "admin"
)

// ---- Error codes --------------------------------------------------------------

const (
	ErrCodeValidation   = "VALIDATION_ERROR"
	ErrCodeUnauthorized = "UNAUTHORIZED"
	ErrCodeForbidden    = "FORBIDDEN"
	ErrCodeNotFound     = "NOT_FOUND"
	ErrCodeConflict     = "CONFLICT"
	ErrCodeRateLimit    = "RATE_LIMIT_EXCEEDED"
	ErrCodeInternal     = "INTERNAL_ERROR"
	ErrCodeBadGateway   = "BAD_GATEWAY"
	ErrCodeUnavailable  = "SERVICE_UNAVAILABLE"
)

// ---- Content moderation -------------------------------------------------------

const (
	ModerationStatusPending  = "pending"
	ModerationStatusApproved = "approved"
	ModerationStatusRejected = "rejected"
	ModerationStatusFlagged  = "flagged"
)

// ---- Video status -------------------------------------------------------------

const (
	VideoStatusProcessing = "processing"
	VideoStatusPublished  = "published"
	VideoStatusPrivate    = "private"
	VideoStatusDeleted    = "deleted"
	VideoStatusFailed     = "failed"
)

// ---- Order status -------------------------------------------------------------

const (
	OrderStatusPending    = "pending"
	OrderStatusConfirmed  = "confirmed"
	OrderStatusShipped    = "shipped"
	OrderStatusDelivered  = "delivered"
	OrderStatusCancelled  = "cancelled"
	OrderStatusRefunded   = "refunded"
)
