package models

import (
	"time"
)

// StreamStatus represents the lifecycle state of a livestream.
type StreamStatus string

const (
	StreamStatusPending  StreamStatus = "pending"
	StreamStatusLive     StreamStatus = "live"
	StreamStatusEnded    StreamStatus = "ended"
	StreamStatusBanned   StreamStatus = "banned"
)

// ViewerStatus represents whether a viewer is present.
type ViewerStatus string

const (
	ViewerStatusJoined ViewerStatus = "joined"
	ViewerStatusLeft   ViewerStatus = "left"
	ViewerStatusBanned ViewerStatus = "banned"
)

// BattleStatus represents the state of a PK battle.
type BattleStatus string

const (
	BattleStatusPending  BattleStatus = "pending"
	BattleStatusActive   BattleStatus = "active"
	BattleStatusEnded    BattleStatus = "ended"
	BattleStatusDeclined BattleStatus = "declined"
)

// CoHostStatus represents co-host invite state.
type CoHostStatus string

const (
	CoHostStatusPending  CoHostStatus = "pending"
	CoHostStatusAccepted CoHostStatus = "accepted"
	CoHostStatusDeclined CoHostStatus = "declined"
	CoHostStatusRemoved  CoHostStatus = "removed"
)

// PollStatus represents the state of an in-stream poll.
type PollStatus string

const (
	PollStatusActive  PollStatus = "active"
	PollStatusClosed  PollStatus = "closed"
)

// AudioRoomStatus represents the state of an audio-only room.
type AudioRoomStatus string

const (
	AudioRoomStatusLive   AudioRoomStatus = "live"
	AudioRoomStatusClosed AudioRoomStatus = "closed"
)

// LiveStream is the core entity representing a streaming session.
type LiveStream struct {
	ID              string       `json:"id" db:"id"`
	UserID          string       `json:"user_id" db:"user_id"`
	Title           string       `json:"title" db:"title"`
	Description     string       `json:"description" db:"description"`
	RTMPKey         string       `json:"rtmp_key,omitempty" db:"rtmp_key"`
	RTMPIngestURL   string       `json:"rtmp_ingest_url,omitempty" db:"-"`
	HLSPlaylistURL  string       `json:"hls_playlist_url" db:"hls_playlist_url"`
	ThumbnailURL    string       `json:"thumbnail_url" db:"thumbnail_url"`
	Status          StreamStatus `json:"status" db:"status"`
	ViewerCount     int64        `json:"viewer_count" db:"viewer_count"`
	PeakViewerCount int64        `json:"peak_viewer_count" db:"peak_viewer_count"`
	TotalGiftCoins  int64        `json:"total_gift_coins" db:"total_gift_coins"`
	CategoryID      string       `json:"category_id" db:"category_id"`
	Tags            []string     `json:"tags" db:"tags"`
	IsRecorded      bool         `json:"is_recorded" db:"is_recorded"`
	RecordingURL    string       `json:"recording_url,omitempty" db:"recording_url"`
	// Language code e.g. "en", "zh"
	Language        string       `json:"language" db:"language"`
	// AgeRestricted marks 18+ streams
	AgeRestricted   bool         `json:"age_restricted" db:"age_restricted"`
	// AllowComments controls chat availability
	AllowComments   bool         `json:"allow_comments" db:"allow_comments"`
	// PKBattleID is set when a PK battle is active
	PKBattleID      string       `json:"pk_battle_id,omitempty" db:"pk_battle_id"`
	StartedAt       *time.Time   `json:"started_at,omitempty" db:"started_at"`
	EndedAt         *time.Time   `json:"ended_at,omitempty" db:"ended_at"`
	CreatedAt       time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time    `json:"updated_at" db:"updated_at"`
}

// LiveViewer represents a user who has joined a stream.
type LiveViewer struct {
	ID         string       `json:"id" db:"id"`
	StreamID   string       `json:"stream_id" db:"stream_id"`
	UserID     string       `json:"user_id" db:"user_id"`
	Username   string       `json:"username" db:"username"`
	AvatarURL  string       `json:"avatar_url" db:"avatar_url"`
	Status     ViewerStatus `json:"status" db:"status"`
	JoinedAt   time.Time    `json:"joined_at" db:"joined_at"`
	LeftAt     *time.Time   `json:"left_at,omitempty" db:"left_at"`
	// WatchDurationSecs is computed on leave.
	WatchDurationSecs int64 `json:"watch_duration_secs" db:"watch_duration_secs"`
	// IsFollower and IsModerator are denormalized for quick access.
	IsFollower    bool `json:"is_follower" db:"is_follower"`
	IsModerator   bool `json:"is_moderator" db:"is_moderator"`
}

// LiveMessage is a chat message sent during a stream.
type LiveMessage struct {
	ID         string    `json:"id" db:"id"`
	StreamID   string    `json:"stream_id" db:"stream_id"`
	UserID     string    `json:"user_id" db:"user_id"`
	Username   string    `json:"username" db:"username"`
	AvatarURL  string    `json:"avatar_url" db:"avatar_url"`
	Content    string    `json:"content" db:"content"`
	// Type: "text", "emoji", "sticker", "system"
	Type       string    `json:"type" db:"type"`
	// IsPinned marks a message as pinned by the host.
	IsPinned   bool      `json:"is_pinned" db:"is_pinned"`
	IsDeleted  bool      `json:"is_deleted" db:"is_deleted"`
	// Reactions is a map of emoji -> count.
	Reactions  map[string]int64 `json:"reactions" db:"-"`
	// ReplyToID is set for threaded replies.
	ReplyToID  string    `json:"reply_to_id,omitempty" db:"reply_to_id"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}

// Gift represents a virtual gift sent during a stream.
type Gift struct {
	ID            string    `json:"id" db:"id"`
	StreamID      string    `json:"stream_id" db:"stream_id"`
	SenderID      string    `json:"sender_id" db:"sender_id"`
	SenderName    string    `json:"sender_name" db:"sender_name"`
	ReceiverID    string    `json:"receiver_id" db:"receiver_id"`
	GiftTypeID    string    `json:"gift_type_id" db:"gift_type_id"`
	GiftName      string    `json:"gift_name" db:"gift_name"`
	// AnimationURL is the Lottie/SVGA file for gift animation.
	AnimationURL  string    `json:"animation_url" db:"animation_url"`
	IconURL       string    `json:"icon_url" db:"icon_url"`
	// CoinCost is what the sender paid.
	CoinCost      int64     `json:"coin_cost" db:"coin_cost"`
	// Quantity allows sending multiple copies at once.
	Quantity      int       `json:"quantity" db:"quantity"`
	// TotalCoins = CoinCost * Quantity
	TotalCoins    int64     `json:"total_coins" db:"total_coins"`
	// IsCombo marks this as part of a combo chain.
	IsCombo       bool      `json:"is_combo" db:"is_combo"`
	ComboCount    int       `json:"combo_count" db:"combo_count"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
}

// GiftType is the catalog entry for a virtual gift.
type GiftType struct {
	ID           string    `json:"id" db:"id"`
	Name         string    `json:"name" db:"name"`
	Description  string    `json:"description" db:"description"`
	IconURL      string    `json:"icon_url" db:"icon_url"`
	AnimationURL string    `json:"animation_url" db:"animation_url"`
	// CoinPrice is the cost in platform coins.
	CoinPrice    int64     `json:"coin_price" db:"coin_price"`
	// Category: "basic", "premium", "limited"
	Category     string    `json:"category" db:"category"`
	IsActive     bool      `json:"is_active" db:"is_active"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

// PKBattle represents a real-time battle between two streamers.
type PKBattle struct {
	ID            string       `json:"id" db:"id"`
	// InitiatorID is the user who sent the PK invite.
	InitiatorID   string       `json:"initiator_id" db:"initiator_id"`
	InitiatorName string       `json:"initiator_name" db:"initiator_name"`
	// TargetID is the invited streamer.
	TargetID      string       `json:"target_id" db:"target_id"`
	TargetName    string       `json:"target_name" db:"target_name"`
	// StreamID is the initiator's stream.
	StreamID      string       `json:"stream_id" db:"stream_id"`
	// TargetStreamID is the target's stream.
	TargetStreamID string      `json:"target_stream_id" db:"target_stream_id"`
	Status        BattleStatus `json:"status" db:"status"`
	// Scores are tracked in Redis during the battle.
	InitiatorScore int64       `json:"initiator_score" db:"initiator_score"`
	TargetScore    int64       `json:"target_score" db:"target_score"`
	// WinnerID is set when the battle ends.
	WinnerID       string      `json:"winner_id,omitempty" db:"winner_id"`
	// DurationSecs is the agreed battle duration.
	DurationSecs   int         `json:"duration_secs" db:"duration_secs"`
	StartedAt     *time.Time   `json:"started_at,omitempty" db:"started_at"`
	EndedAt       *time.Time   `json:"ended_at,omitempty" db:"ended_at"`
	CreatedAt      time.Time   `json:"created_at" db:"created_at"`
}

// CoHost represents a user invited to co-host a stream.
type CoHost struct {
	ID         string       `json:"id" db:"id"`
	StreamID   string       `json:"stream_id" db:"stream_id"`
	HostID     string       `json:"host_id" db:"host_id"`
	CoHostID   string       `json:"co_host_id" db:"co_host_id"`
	CoHostName string       `json:"co_host_name" db:"co_host_name"`
	Status     CoHostStatus `json:"status" db:"status"`
	// WebRTCSessionID links to the WebRTC session for the co-host.
	WebRTCSessionID string  `json:"webrtc_session_id,omitempty" db:"webrtc_session_id"`
	InvitedAt  time.Time    `json:"invited_at" db:"invited_at"`
	AcceptedAt *time.Time   `json:"accepted_at,omitempty" db:"accepted_at"`
	RemovedAt  *time.Time   `json:"removed_at,omitempty" db:"removed_at"`
}

// Poll represents an in-stream audience poll.
type Poll struct {
	ID         string     `json:"id" db:"id"`
	StreamID   string     `json:"stream_id" db:"stream_id"`
	CreatorID  string     `json:"creator_id" db:"creator_id"`
	Question   string     `json:"question" db:"question"`
	Options    []PollOption `json:"options" db:"-"`
	Status     PollStatus `json:"status" db:"status"`
	// DurationSecs is how long the poll stays open (0 = until manually closed).
	DurationSecs int      `json:"duration_secs" db:"duration_secs"`
	TotalVotes int64      `json:"total_votes" db:"total_votes"`
	CreatedAt  time.Time  `json:"created_at" db:"created_at"`
	ClosedAt   *time.Time `json:"closed_at,omitempty" db:"closed_at"`
}

// PollOption is a single answer choice in a poll.
type PollOption struct {
	ID         string `json:"id" db:"id"`
	PollID     string `json:"poll_id" db:"poll_id"`
	Text       string `json:"text" db:"text"`
	VoteCount  int64  `json:"vote_count" db:"vote_count"`
	// Percentage is computed on read.
	Percentage float64 `json:"percentage" db:"-"`
}

// PollVote records a single user's vote.
type PollVote struct {
	ID       string    `json:"id" db:"id"`
	PollID   string    `json:"poll_id" db:"poll_id"`
	OptionID string    `json:"option_id" db:"option_id"`
	UserID   string    `json:"user_id" db:"user_id"`
	VotedAt  time.Time `json:"voted_at" db:"voted_at"`
}

// AudioRoom represents a Clubhouse-style audio-only live room.
type AudioRoom struct {
	ID          string          `json:"id" db:"id"`
	HostID      string          `json:"host_id" db:"host_id"`
	Title       string          `json:"title" db:"title"`
	Description string          `json:"description" db:"description"`
	Status      AudioRoomStatus `json:"status" db:"status"`
	// MaxSpeakers limits the stage seats.
	MaxSpeakers  int            `json:"max_speakers" db:"max_speakers"`
	// SpeakerIDs is the list of active speakers (on stage).
	SpeakerIDs   []string       `json:"speaker_ids" db:"-"`
	// ListenerCount is tracked in Redis.
	ListenerCount int64         `json:"listener_count" db:"listener_count"`
	IsRecorded   bool           `json:"is_recorded" db:"is_recorded"`
	RecordingURL string         `json:"recording_url,omitempty" db:"recording_url"`
	StartedAt    *time.Time     `json:"started_at,omitempty" db:"started_at"`
	EndedAt      *time.Time     `json:"ended_at,omitempty" db:"ended_at"`
	CreatedAt    time.Time      `json:"created_at" db:"created_at"`
}

// AudioRoomSpeaker tracks who is on stage in an audio room.
type AudioRoomSpeaker struct {
	ID          string    `json:"id" db:"id"`
	RoomID      string    `json:"room_id" db:"room_id"`
	UserID      string    `json:"user_id" db:"user_id"`
	Username    string    `json:"username" db:"username"`
	AvatarURL   string    `json:"avatar_url" db:"avatar_url"`
	// IsMuted tracks whether the speaker has muted themselves.
	IsMuted     bool      `json:"is_muted" db:"is_muted"`
	// IsHandRaised indicates a listener requesting to speak.
	IsHandRaised bool     `json:"is_hand_raised" db:"is_hand_raised"`
	JoinedAt    time.Time `json:"joined_at" db:"joined_at"`
}

// -- WebSocket event payloads -------------------------------------------

// WSEventType identifies the event kind sent over the WebSocket.
type WSEventType string

const (
	WSEventViewerJoin     WSEventType = "viewer.join"
	WSEventViewerLeave    WSEventType = "viewer.leave"
	WSEventViewerCount    WSEventType = "viewer.count"
	WSEventChatMessage    WSEventType = "chat.message"
	WSEventChatDelete     WSEventType = "chat.delete"
	WSEventChatPin        WSEventType = "chat.pin"
	WSEventGiftSent       WSEventType = "gift.sent"
	WSEventGiftAnimation  WSEventType = "gift.animation"
	WSEventStreamStart    WSEventType = "stream.start"
	WSEventStreamEnd      WSEventType = "stream.end"
	WSEventPKBattleInvite WSEventType = "pk.invite"
	WSEventPKBattleStart  WSEventType = "pk.start"
	WSEventPKBattleScore  WSEventType = "pk.score"
	WSEventPKBattleEnd    WSEventType = "pk.end"
	WSEventPollCreate     WSEventType = "poll.create"
	WSEventPollVote       WSEventType = "poll.vote"
	WSEventPollClose      WSEventType = "poll.close"
	WSEventCoHostInvite   WSEventType = "cohost.invite"
	WSEventCoHostAccept   WSEventType = "cohost.accept"
	WSEventCoHostRemove   WSEventType = "cohost.remove"
	WSEventError          WSEventType = "error"
)

// WSEvent is the envelope for all WebSocket messages.
type WSEvent struct {
	Type      WSEventType `json:"type"`
	StreamID  string      `json:"stream_id"`
	Timestamp time.Time   `json:"timestamp"`
	Payload   interface{} `json:"payload"`
}

// ViewerCountPayload carries current and peak viewer counts.
type ViewerCountPayload struct {
	Current int64 `json:"current"`
	Peak    int64 `json:"peak"`
}

// GiftAnimationPayload triggers a client-side gift animation.
type GiftAnimationPayload struct {
	GiftName     string `json:"gift_name"`
	AnimationURL string `json:"animation_url"`
	SenderName   string `json:"sender_name"`
	Quantity     int    `json:"quantity"`
	ComboCount   int    `json:"combo_count"`
	TotalCoins   int64  `json:"total_coins"`
}

// PKScorePayload carries the live battle scores.
type PKScorePayload struct {
	BattleID       string `json:"battle_id"`
	InitiatorID    string `json:"initiator_id"`
	InitiatorScore int64  `json:"initiator_score"`
	TargetID       string `json:"target_id"`
	TargetScore    int64  `json:"target_score"`
	// SecondsRemaining counts down to battle end.
	SecondsRemaining int  `json:"seconds_remaining"`
}

// PollResultPayload carries live vote counts for all options.
type PollResultPayload struct {
	PollID     string       `json:"poll_id"`
	Question   string       `json:"question"`
	Options    []PollOption `json:"options"`
	TotalVotes int64        `json:"total_votes"`
	IsClosed   bool         `json:"is_closed"`
}

// -- Kafka event payloads -----------------------------------------------

// KafkaLivestreamStarted is published when a stream goes live.
type KafkaLivestreamStarted struct {
	StreamID  string    `json:"stream_id"`
	UserID    string    `json:"user_id"`
	Title     string    `json:"title"`
	StartedAt time.Time `json:"started_at"`
}

// KafkaLivestreamEnded is published when a stream ends.
type KafkaLivestreamEnded struct {
	StreamID          string    `json:"stream_id"`
	UserID            string    `json:"user_id"`
	EndedAt           time.Time `json:"ended_at"`
	PeakViewerCount   int64     `json:"peak_viewer_count"`
	TotalGiftCoins    int64     `json:"total_gift_coins"`
	DurationSecs      int64     `json:"duration_secs"`
}

// KafkaGiftSent is published when a gift is sent.
type KafkaGiftSent struct {
	GiftID     string    `json:"gift_id"`
	StreamID   string    `json:"stream_id"`
	SenderID   string    `json:"sender_id"`
	ReceiverID string    `json:"receiver_id"`
	GiftTypeID string    `json:"gift_type_id"`
	TotalCoins int64     `json:"total_coins"`
	SentAt     time.Time `json:"sent_at"`
}

// KafkaPKBattleResult is published when a PK battle concludes.
type KafkaPKBattleResult struct {
	BattleID       string    `json:"battle_id"`
	InitiatorID    string    `json:"initiator_id"`
	TargetID       string    `json:"target_id"`
	InitiatorScore int64     `json:"initiator_score"`
	TargetScore    int64     `json:"target_score"`
	WinnerID       string    `json:"winner_id"`
	EndedAt        time.Time `json:"ended_at"`
}
