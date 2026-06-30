package models

import (
	"time"

	"github.com/google/uuid"
)

// MessageType classifies the payload of a message.
type MessageType string

const (
	MessageTypeText     MessageType = "text"
	MessageTypeImage    MessageType = "image"
	MessageTypeVideo    MessageType = "video"
	MessageTypeAudio    MessageType = "audio"
	MessageTypeFile     MessageType = "file"
	MessageTypeSticker  MessageType = "sticker"
	MessageTypeGIF      MessageType = "gif"
	MessageTypeLocation MessageType = "location"
	MessageTypeSystem   MessageType = "system" // system-generated events
)

// MessageStatus tracks delivery/read state.
type MessageStatus string

const (
	MessageStatusSent      MessageStatus = "sent"
	MessageStatusDelivered MessageStatus = "delivered"
	MessageStatusRead      MessageStatus = "read"
	MessageStatusDeleted   MessageStatus = "deleted"
)

// ConversationType distinguishes direct (1:1) from group chats.
type ConversationType string

const (
	ConversationTypeDirect ConversationType = "direct"
	ConversationTypeGroup  ConversationType = "group"
)

// GroupRole defines a member's role inside a group conversation.
type GroupRole string

const (
	GroupRoleOwner  GroupRole = "owner"
	GroupRoleAdmin  GroupRole = "admin"
	GroupRoleMember GroupRole = "member"
)

// ---------------------------------------------------------------------------
// Core domain models
// ---------------------------------------------------------------------------

// Conversation is the top-level thread container (DM or group).
type Conversation struct {
	ID           uuid.UUID        `json:"id" db:"id"`
	Type         ConversationType `json:"type" db:"type"`
	CreatedByID  uuid.UUID        `json:"created_by_id" db:"created_by_id"`
	CreatedAt    time.Time        `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time        `json:"updated_at" db:"updated_at"`
	LastMessage  *Message         `json:"last_message,omitempty" db:"-"`
	Participants []Participant    `json:"participants,omitempty" db:"-"`
	// Populated only for group conversations
	GroupChat    *GroupChat       `json:"group_chat,omitempty" db:"-"`
	// UnreadCount for the requesting user
	UnreadCount  int              `json:"unread_count" db:"unread_count"`
}

// Participant links a user to a conversation.
type Participant struct {
	ConversationID uuid.UUID `json:"conversation_id" db:"conversation_id"`
	UserID         uuid.UUID `json:"user_id" db:"user_id"`
	JoinedAt       time.Time `json:"joined_at" db:"joined_at"`
	LastReadAt     time.Time `json:"last_read_at" db:"last_read_at"`
	// Role is only meaningful for group conversations.
	Role           GroupRole `json:"role" db:"role"`
	IsMuted        bool      `json:"is_muted" db:"is_muted"`
	// DisplayName / Avatar are denormalized from user-service for convenience.
	DisplayName    string    `json:"display_name,omitempty" db:"display_name"`
	AvatarURL      string    `json:"avatar_url,omitempty" db:"avatar_url"`
}

// GroupChat holds metadata specific to group conversations.
type GroupChat struct {
	ID             uuid.UUID  `json:"id" db:"id"`
	ConversationID uuid.UUID  `json:"conversation_id" db:"conversation_id"`
	Name           string     `json:"name" db:"name"`
	Description    string     `json:"description" db:"description"`
	AvatarURL      string     `json:"avatar_url" db:"avatar_url"`
	MaxMembers     int        `json:"max_members" db:"max_members"`
	CreatedByID    uuid.UUID  `json:"created_by_id" db:"created_by_id"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at" db:"updated_at"`
}

// Message is a single chat message within a conversation.
//
// The content field is stored AES-256-GCM encrypted (base64-encoded ciphertext).
// The nonce used during encryption is stored alongside as EncryptedNonce
// (also base64-encoded) so that decryption never requires an external lookup.
type Message struct {
	ID             uuid.UUID     `json:"id" db:"id"`
	ConversationID uuid.UUID     `json:"conversation_id" db:"conversation_id"`
	SenderID       uuid.UUID     `json:"sender_id" db:"sender_id"`
	// EncryptedContent is the AES-256-GCM ciphertext (base64-encoded).
	EncryptedContent string      `json:"-" db:"encrypted_content"`
	// Nonce is the 12-byte GCM nonce used when encrypting this message (base64-encoded).
	Nonce          string        `json:"-" db:"nonce"`
	// Content is the plaintext — populated after decryption, never persisted.
	Content        string        `json:"content,omitempty" db:"-"`
	Type           MessageType   `json:"type" db:"type"`
	Status         MessageStatus `json:"status" db:"status"`
	// ReplyToID optionally references a parent message for threaded replies.
	ReplyToID      *uuid.UUID    `json:"reply_to_id,omitempty" db:"reply_to_id"`
	ReplyToMessage *Message      `json:"reply_to,omitempty" db:"-"`
	// IsEdited is true if the message body has been changed since original send.
	IsEdited       bool          `json:"is_edited" db:"is_edited"`
	DeletedAt      *time.Time    `json:"deleted_at,omitempty" db:"deleted_at"`
	CreatedAt      time.Time     `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at" db:"updated_at"`
	Attachments    []Attachment  `json:"attachments,omitempty" db:"-"`
	Reactions      []MessageReaction `json:"reactions,omitempty" db:"-"`
	// ReadBy is the list of user IDs that have seen this message.
	ReadBy         []uuid.UUID   `json:"read_by,omitempty" db:"-"`
	// SenderInfo is denormalized for API responses.
	SenderDisplayName string     `json:"sender_display_name,omitempty" db:"-"`
	SenderAvatarURL   string     `json:"sender_avatar_url,omitempty" db:"-"`
}

// IsDeleted returns true when the message has been soft-deleted.
func (m *Message) IsDeleted() bool {
	return m.DeletedAt != nil
}

// Attachment represents a media file linked to a message.
type Attachment struct {
	ID         uuid.UUID  `json:"id" db:"id"`
	MessageID  uuid.UUID  `json:"message_id" db:"message_id"`
	FileURL    string     `json:"file_url" db:"file_url"`
	// S3Key is the raw object key; FileURL is the presigned / CDN URL.
	S3Key      string     `json:"-" db:"s3_key"`
	FileName   string     `json:"file_name" db:"file_name"`
	FileSize   int64      `json:"file_size" db:"file_size"`
	MimeType   string     `json:"mime_type" db:"mime_type"`
	Width      int        `json:"width,omitempty" db:"width"`
	Height     int        `json:"height,omitempty" db:"height"`
	// Duration is used for audio/video attachments (seconds).
	Duration   float64    `json:"duration,omitempty" db:"duration"`
	Thumbnail  string     `json:"thumbnail,omitempty" db:"thumbnail"`
	CreatedAt  time.Time  `json:"created_at" db:"created_at"`
}

// MessageReaction is an emoji reaction on a specific message.
type MessageReaction struct {
	ID        uuid.UUID `json:"id" db:"id"`
	MessageID uuid.UUID `json:"message_id" db:"message_id"`
	UserID    uuid.UUID `json:"user_id" db:"user_id"`
	Emoji     string    `json:"emoji" db:"emoji"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// MessageReadReceipt records when a specific user read a specific message.
type MessageReadReceipt struct {
	MessageID uuid.UUID `json:"message_id" db:"message_id"`
	UserID    uuid.UUID `json:"user_id" db:"user_id"`
	ReadAt    time.Time `json:"read_at" db:"read_at"`
}

// ---------------------------------------------------------------------------
// WebSocket event payloads
// ---------------------------------------------------------------------------

// WSEventType enumerates every event the WebSocket layer can emit or receive.
type WSEventType string

const (
	// Server → client events
	WSEventNewMessage     WSEventType = "new_message"
	WSEventMessageUpdated WSEventType = "message_updated"
	WSEventMessageDeleted WSEventType = "message_deleted"
	WSEventReadReceipt    WSEventType = "read_receipt"
	WSEventUserOnline     WSEventType = "user_online"
	WSEventUserOffline    WSEventType = "user_offline"
	WSEventReaction       WSEventType = "reaction"

	// Client → server events
	WSEventTypingStart WSEventType = "typing_start"
	WSEventTypingStop  WSEventType = "typing_stop"
	WSEventMarkRead    WSEventType = "mark_read"
	WSEventPing        WSEventType = "ping"

	// Server → client (in response to ping)
	WSEventPong WSEventType = "pong"
)

// WSEvent is the envelope for all WebSocket messages.
type WSEvent struct {
	Type           WSEventType `json:"type"`
	ConversationID string      `json:"conversation_id,omitempty"`
	SenderID       string      `json:"sender_id,omitempty"`
	Payload        interface{} `json:"payload,omitempty"`
	Timestamp      time.Time   `json:"timestamp"`
}

// TypingPayload is the payload for typing_start / typing_stop events.
type TypingPayload struct {
	UserID         string `json:"user_id"`
	ConversationID string `json:"conversation_id"`
}

// ReadReceiptPayload is the payload for mark_read / read_receipt events.
type ReadReceiptPayload struct {
	ConversationID string    `json:"conversation_id"`
	LastReadAt     time.Time `json:"last_read_at"`
	UserID         string    `json:"user_id,omitempty"`
}

// ---------------------------------------------------------------------------
// Request / Response DTOs
// ---------------------------------------------------------------------------

// CreateConversationRequest is used when starting a new DM.
type CreateConversationRequest struct {
	RecipientID uuid.UUID `json:"recipient_id" binding:"required"`
}

// CreateGroupRequest is used when creating a new group conversation.
type CreateGroupRequest struct {
	Name        string      `json:"name" binding:"required,min=1,max=100"`
	Description string      `json:"description" binding:"max=500"`
	MemberIDs   []uuid.UUID `json:"member_ids" binding:"required,min=1"`
	AvatarURL   string      `json:"avatar_url"`
}

// SendMessageRequest is the REST payload for sending a message.
type SendMessageRequest struct {
	ConversationID uuid.UUID   `json:"conversation_id" binding:"required"`
	Content        string      `json:"content" binding:"required_without=AttachmentIDs,max=4096"`
	Type           MessageType `json:"type" binding:"required"`
	ReplyToID      *uuid.UUID  `json:"reply_to_id"`
	AttachmentIDs  []uuid.UUID `json:"attachment_ids"`
}

// MessagesPage is the cursor-paginated response for GetMessages.
type MessagesPage struct {
	Messages   []Message `json:"messages"`
	NextCursor string    `json:"next_cursor,omitempty"`
	HasMore    bool      `json:"has_more"`
}

// ConversationsPage is the paginated response for listing conversations.
type ConversationsPage struct {
	Conversations []Conversation `json:"conversations"`
	NextCursor    string         `json:"next_cursor,omitempty"`
	HasMore       bool           `json:"has_more"`
}

// AddGroupMemberRequest adds one or more users to a group.
type AddGroupMemberRequest struct {
	UserIDs []uuid.UUID `json:"user_ids" binding:"required,min=1"`
}

// RemoveGroupMemberRequest removes a user from a group.
type RemoveGroupMemberRequest struct {
	UserID uuid.UUID `json:"user_id" binding:"required"`
}

// ShareMediaResponse is returned after a successful media upload.
type ShareMediaResponse struct {
	AttachmentID uuid.UUID `json:"attachment_id"`
	FileURL      string    `json:"file_url"`
	S3Key        string    `json:"s3_key"`
	MimeType     string    `json:"mime_type"`
	FileSize     int64     `json:"file_size"`
}
