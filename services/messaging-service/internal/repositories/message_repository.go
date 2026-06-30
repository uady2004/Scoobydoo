package repositories

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/tiktok-clone/messaging-service/internal/models"
)

// ErrNotFound is returned when a requested resource does not exist.
var ErrNotFound = errors.New("not found")

// ErrForbidden is returned when the caller lacks permission for an operation.
var ErrForbidden = errors.New("forbidden")

// MessageRepository defines the data-access contract for the messaging domain.
type MessageRepository interface {
	// Conversation management
	CreateConversation(ctx context.Context, creatorID, recipientID uuid.UUID) (*models.Conversation, error)
	GetConversation(ctx context.Context, id uuid.UUID) (*models.Conversation, error)
	GetConversationByParticipants(ctx context.Context, userA, userB uuid.UUID) (*models.Conversation, error)
	GetConversations(ctx context.Context, userID uuid.UUID, cursor string, limit int) (*models.ConversationsPage, error)

	// Group management
	CreateGroup(ctx context.Context, req *models.CreateGroupRequest, creatorID uuid.UUID) (*models.Conversation, error)
	AddGroupMember(ctx context.Context, conversationID uuid.UUID, adderID uuid.UUID, memberIDs []uuid.UUID) error
	RemoveGroupMember(ctx context.Context, conversationID, removerID, memberID uuid.UUID) error
	GetGroupChat(ctx context.Context, conversationID uuid.UUID) (*models.GroupChat, error)

	// Message CRUD
	SendMessage(ctx context.Context, msg *models.Message) (*models.Message, error)
	GetMessages(ctx context.Context, conversationID uuid.UUID, cursor string, limit int) (*models.MessagesPage, error)
	GetMessage(ctx context.Context, id uuid.UUID) (*models.Message, error)
	DeleteMessage(ctx context.Context, messageID, requestorID uuid.UUID) error

	// Attachments
	CreateAttachment(ctx context.Context, a *models.Attachment) (*models.Attachment, error)
	GetAttachments(ctx context.Context, messageID uuid.UUID) ([]models.Attachment, error)

	// Read receipts
	MarkAsRead(ctx context.Context, conversationID, userID uuid.UUID) error
	GetUnreadCount(ctx context.Context, conversationID, userID uuid.UUID) (int, error)
	GetTotalUnreadCount(ctx context.Context, userID uuid.UUID) (int, error)

	// Reactions
	AddReaction(ctx context.Context, r *models.MessageReaction) (*models.MessageReaction, error)
	RemoveReaction(ctx context.Context, messageID, userID uuid.UUID, emoji string) error
	GetReactions(ctx context.Context, messageID uuid.UUID) ([]models.MessageReaction, error)

	// Participant helpers
	IsParticipant(ctx context.Context, conversationID, userID uuid.UUID) (bool, error)
	GetParticipants(ctx context.Context, conversationID uuid.UUID) ([]models.Participant, error)
}

// pgMessageRepository is the PostgreSQL implementation backed by pgxpool.
type pgMessageRepository struct {
	db  *pgxpool.Pool
	log *zap.Logger
}

// NewMessageRepository constructs a production PostgreSQL repository.
func NewMessageRepository(db *pgxpool.Pool, log *zap.Logger) MessageRepository {
	return &pgMessageRepository{db: db, log: log}
}

// ---------------------------------------------------------------------------
// Conversation management
// ---------------------------------------------------------------------------

func (r *pgMessageRepository) CreateConversation(ctx context.Context, creatorID, recipientID uuid.UUID) (*models.Conversation, error) {
	// Check if a DM already exists between these two users.
	existing, err := r.GetConversationByParticipants(ctx, creatorID, recipientID)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	convID := uuid.New()
	conv := &models.Conversation{}

	err = tx.QueryRow(ctx, `
		INSERT INTO conversations (id, type, created_by_id, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		RETURNING id, type, created_by_id, created_at, updated_at`,
		convID, models.ConversationTypeDirect, creatorID,
	).Scan(&conv.ID, &conv.Type, &conv.CreatedByID, &conv.CreatedAt, &conv.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert conversation: %w", err)
	}

	// Add both participants
	for _, uid := range []uuid.UUID{creatorID, recipientID} {
		role := models.GroupRoleMember
		_, err = tx.Exec(ctx, `
			INSERT INTO conversation_participants (conversation_id, user_id, joined_at, last_read_at, role)
			VALUES ($1, $2, NOW(), NOW(), $3)`,
			conv.ID, uid, role,
		)
		if err != nil {
			return nil, fmt.Errorf("insert participant %s: %w", uid, err)
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	conv.UnreadCount = 0
	return conv, nil
}

func (r *pgMessageRepository) GetConversation(ctx context.Context, id uuid.UUID) (*models.Conversation, error) {
	conv := &models.Conversation{}
	err := r.db.QueryRow(ctx, `
		SELECT id, type, created_by_id, created_at, updated_at
		FROM conversations WHERE id = $1`, id,
	).Scan(&conv.ID, &conv.Type, &conv.CreatedByID, &conv.CreatedAt, &conv.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get conversation: %w", err)
	}
	return conv, nil
}

func (r *pgMessageRepository) GetConversationByParticipants(ctx context.Context, userA, userB uuid.UUID) (*models.Conversation, error) {
	conv := &models.Conversation{}
	err := r.db.QueryRow(ctx, `
		SELECT c.id, c.type, c.created_by_id, c.created_at, c.updated_at
		FROM conversations c
		JOIN conversation_participants pa ON pa.conversation_id = c.id AND pa.user_id = $1
		JOIN conversation_participants pb ON pb.conversation_id = c.id AND pb.user_id = $2
		WHERE c.type = 'direct'
		LIMIT 1`,
		userA, userB,
	).Scan(&conv.ID, &conv.Type, &conv.CreatedByID, &conv.CreatedAt, &conv.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get conversation by participants: %w", err)
	}
	return conv, nil
}

// GetConversations returns all conversations for a user, cursor-paginated by
// the conversation's updated_at timestamp (ISO-8601 base64-encoded).
func (r *pgMessageRepository) GetConversations(ctx context.Context, userID uuid.UUID, cursor string, limit int) (*models.ConversationsPage, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	var cursorTime time.Time
	if cursor != "" {
		decoded, err := base64.StdEncoding.DecodeString(cursor)
		if err != nil {
			return nil, fmt.Errorf("invalid cursor: %w", err)
		}
		if err = cursorTime.UnmarshalText(decoded); err != nil {
			return nil, fmt.Errorf("invalid cursor time: %w", err)
		}
	}

	query := `
		SELECT c.id, c.type, c.created_by_id, c.created_at, c.updated_at,
		       COALESCE(
		           (SELECT COUNT(*) FROM messages m
		            JOIN conversation_participants cp2 ON cp2.conversation_id = m.conversation_id
		                AND cp2.user_id = $1
		            WHERE m.conversation_id = c.id
		              AND m.created_at > cp2.last_read_at
		              AND m.sender_id != $1
		              AND m.deleted_at IS NULL), 0
		       ) AS unread_count
		FROM conversations c
		JOIN conversation_participants cp ON cp.conversation_id = c.id AND cp.user_id = $1
		WHERE ($2::timestamptz IS NULL OR c.updated_at < $2)
		ORDER BY c.updated_at DESC
		LIMIT $3`

	var cursorArg interface{}
	if cursorTime.IsZero() {
		cursorArg = nil
	} else {
		cursorArg = cursorTime
	}

	rows, err := r.db.Query(ctx, query, userID, cursorArg, limit+1)
	if err != nil {
		return nil, fmt.Errorf("query conversations: %w", err)
	}
	defer rows.Close()

	convs := make([]models.Conversation, 0, limit)
	for rows.Next() {
		var c models.Conversation
		if err = rows.Scan(&c.ID, &c.Type, &c.CreatedByID, &c.CreatedAt, &c.UpdatedAt, &c.UnreadCount); err != nil {
			return nil, fmt.Errorf("scan conversation: %w", err)
		}
		convs = append(convs, c)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	page := &models.ConversationsPage{}
	if len(convs) > limit {
		page.HasMore = true
		convs = convs[:limit]
		lastUpdated := convs[len(convs)-1].UpdatedAt
		t, _ := lastUpdated.MarshalText()
		page.NextCursor = base64.StdEncoding.EncodeToString(t)
	}
	page.Conversations = convs
	return page, nil
}

// ---------------------------------------------------------------------------
// Group management
// ---------------------------------------------------------------------------

func (r *pgMessageRepository) CreateGroup(ctx context.Context, req *models.CreateGroupRequest, creatorID uuid.UUID) (*models.Conversation, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	convID := uuid.New()
	conv := &models.Conversation{}

	err = tx.QueryRow(ctx, `
		INSERT INTO conversations (id, type, created_by_id, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		RETURNING id, type, created_by_id, created_at, updated_at`,
		convID, models.ConversationTypeGroup, creatorID,
	).Scan(&conv.ID, &conv.Type, &conv.CreatedByID, &conv.CreatedAt, &conv.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert group conversation: %w", err)
	}

	groupID := uuid.New()
	gc := &models.GroupChat{}
	err = tx.QueryRow(ctx, `
		INSERT INTO group_chats (id, conversation_id, name, description, avatar_url, max_members, created_by_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, 256, $6, NOW(), NOW())
		RETURNING id, conversation_id, name, description, avatar_url, max_members, created_by_id, created_at, updated_at`,
		groupID, conv.ID, req.Name, req.Description, req.AvatarURL, creatorID,
	).Scan(&gc.ID, &gc.ConversationID, &gc.Name, &gc.Description, &gc.AvatarURL, &gc.MaxMembers, &gc.CreatedByID, &gc.CreatedAt, &gc.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert group_chat: %w", err)
	}

	// Add creator as owner
	_, err = tx.Exec(ctx, `
		INSERT INTO conversation_participants (conversation_id, user_id, joined_at, last_read_at, role)
		VALUES ($1, $2, NOW(), NOW(), $3)`,
		conv.ID, creatorID, models.GroupRoleOwner,
	)
	if err != nil {
		return nil, fmt.Errorf("insert owner participant: %w", err)
	}

	// Add other members
	for _, uid := range req.MemberIDs {
		if uid == creatorID {
			continue
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO conversation_participants (conversation_id, user_id, joined_at, last_read_at, role)
			VALUES ($1, $2, NOW(), NOW(), $3)`,
			conv.ID, uid, models.GroupRoleMember,
		)
		if err != nil {
			return nil, fmt.Errorf("insert member %s: %w", uid, err)
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	conv.GroupChat = gc
	return conv, nil
}

func (r *pgMessageRepository) AddGroupMember(ctx context.Context, conversationID uuid.UUID, adderID uuid.UUID, memberIDs []uuid.UUID) error {
	// Verify the adder is an admin or owner
	isAdmin, err := r.isMemberWithRole(ctx, conversationID, adderID, models.GroupRoleAdmin, models.GroupRoleOwner)
	if err != nil {
		return err
	}
	if !isAdmin {
		return ErrForbidden
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, uid := range memberIDs {
		_, err = tx.Exec(ctx, `
			INSERT INTO conversation_participants (conversation_id, user_id, joined_at, last_read_at, role)
			VALUES ($1, $2, NOW(), NOW(), $3)
			ON CONFLICT (conversation_id, user_id) DO NOTHING`,
			conversationID, uid, models.GroupRoleMember,
		)
		if err != nil {
			return fmt.Errorf("add member %s: %w", uid, err)
		}
	}

	// Bump conversation updated_at so it resurfaces in feeds
	_, err = tx.Exec(ctx, `UPDATE conversations SET updated_at = NOW() WHERE id = $1`, conversationID)
	if err != nil {
		return fmt.Errorf("bump updated_at: %w", err)
	}

	return tx.Commit(ctx)
}

func (r *pgMessageRepository) RemoveGroupMember(ctx context.Context, conversationID, removerID, memberID uuid.UUID) error {
	// The member can remove themselves; otherwise adder must be admin/owner.
	if removerID != memberID {
		isAdmin, err := r.isMemberWithRole(ctx, conversationID, removerID, models.GroupRoleAdmin, models.GroupRoleOwner)
		if err != nil {
			return err
		}
		if !isAdmin {
			return ErrForbidden
		}
	}

	ct, err := r.db.Exec(ctx, `
		DELETE FROM conversation_participants
		WHERE conversation_id = $1 AND user_id = $2`,
		conversationID, memberID,
	)
	if err != nil {
		return fmt.Errorf("remove member: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *pgMessageRepository) GetGroupChat(ctx context.Context, conversationID uuid.UUID) (*models.GroupChat, error) {
	gc := &models.GroupChat{}
	err := r.db.QueryRow(ctx, `
		SELECT id, conversation_id, name, description, avatar_url, max_members, created_by_id, created_at, updated_at
		FROM group_chats WHERE conversation_id = $1`, conversationID,
	).Scan(&gc.ID, &gc.ConversationID, &gc.Name, &gc.Description, &gc.AvatarURL, &gc.MaxMembers, &gc.CreatedByID, &gc.CreatedAt, &gc.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get group chat: %w", err)
	}
	return gc, nil
}

// ---------------------------------------------------------------------------
// Message CRUD
// ---------------------------------------------------------------------------

func (r *pgMessageRepository) SendMessage(ctx context.Context, msg *models.Message) (*models.Message, error) {
	msg.ID = uuid.New()
	msg.Status = models.MessageStatusSent

	err := r.db.QueryRow(ctx, `
		INSERT INTO messages (id, conversation_id, sender_id, encrypted_content, nonce, type, status, reply_to_id, is_edited, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, FALSE, NOW(), NOW())
		RETURNING id, conversation_id, sender_id, encrypted_content, nonce, type, status, reply_to_id, is_edited, created_at, updated_at`,
		msg.ID, msg.ConversationID, msg.SenderID,
		msg.EncryptedContent, msg.Nonce,
		msg.Type, msg.Status, msg.ReplyToID,
	).Scan(
		&msg.ID, &msg.ConversationID, &msg.SenderID,
		&msg.EncryptedContent, &msg.Nonce,
		&msg.Type, &msg.Status, &msg.ReplyToID,
		&msg.IsEdited, &msg.CreatedAt, &msg.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert message: %w", err)
	}

	// Bump conversation updated_at for feed ordering
	_, err = r.db.Exec(ctx, `UPDATE conversations SET updated_at = NOW() WHERE id = $1`, msg.ConversationID)
	if err != nil {
		r.log.Warn("failed to bump conversation updated_at", zap.Error(err), zap.String("conversation_id", msg.ConversationID.String()))
	}

	return msg, nil
}

// GetMessages returns messages for a conversation in reverse-chronological order
// using an opaque base64-encoded cursor (message created_at timestamp).
func (r *pgMessageRepository) GetMessages(ctx context.Context, conversationID uuid.UUID, cursor string, limit int) (*models.MessagesPage, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}

	var cursorTime time.Time
	if cursor != "" {
		decoded, err := base64.StdEncoding.DecodeString(cursor)
		if err != nil {
			return nil, fmt.Errorf("invalid cursor: %w", err)
		}
		if err = cursorTime.UnmarshalText(decoded); err != nil {
			return nil, fmt.Errorf("invalid cursor time: %w", err)
		}
	}

	var cursorArg interface{}
	if !cursorTime.IsZero() {
		cursorArg = cursorTime
	}

	rows, err := r.db.Query(ctx, `
		SELECT id, conversation_id, sender_id, encrypted_content, nonce,
		       type, status, reply_to_id, is_edited, deleted_at, created_at, updated_at
		FROM messages
		WHERE conversation_id = $1
		  AND ($2::timestamptz IS NULL OR created_at < $2)
		ORDER BY created_at DESC
		LIMIT $3`,
		conversationID, cursorArg, limit+1,
	)
	if err != nil {
		return nil, fmt.Errorf("query messages: %w", err)
	}
	defer rows.Close()

	msgs := make([]models.Message, 0, limit)
	for rows.Next() {
		var m models.Message
		if err = rows.Scan(
			&m.ID, &m.ConversationID, &m.SenderID,
			&m.EncryptedContent, &m.Nonce,
			&m.Type, &m.Status, &m.ReplyToID,
			&m.IsEdited, &m.DeletedAt, &m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		msgs = append(msgs, m)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	page := &models.MessagesPage{}
	if len(msgs) > limit {
		page.HasMore = true
		msgs = msgs[:limit]
		oldest := msgs[len(msgs)-1].CreatedAt
		t, _ := oldest.MarshalText()
		page.NextCursor = base64.StdEncoding.EncodeToString(t)
	}
	page.Messages = msgs
	return page, nil
}

func (r *pgMessageRepository) GetMessage(ctx context.Context, id uuid.UUID) (*models.Message, error) {
	m := &models.Message{}
	err := r.db.QueryRow(ctx, `
		SELECT id, conversation_id, sender_id, encrypted_content, nonce,
		       type, status, reply_to_id, is_edited, deleted_at, created_at, updated_at
		FROM messages WHERE id = $1`, id,
	).Scan(
		&m.ID, &m.ConversationID, &m.SenderID,
		&m.EncryptedContent, &m.Nonce,
		&m.Type, &m.Status, &m.ReplyToID,
		&m.IsEdited, &m.DeletedAt, &m.CreatedAt, &m.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get message: %w", err)
	}
	return m, nil
}

// DeleteMessage performs a soft delete; only the original sender may delete.
func (r *pgMessageRepository) DeleteMessage(ctx context.Context, messageID, requestorID uuid.UUID) error {
	now := time.Now().UTC()
	ct, err := r.db.Exec(ctx, `
		UPDATE messages
		SET deleted_at = $1, status = $2, updated_at = $1
		WHERE id = $3 AND sender_id = $4 AND deleted_at IS NULL`,
		now, models.MessageStatusDeleted, messageID, requestorID,
	)
	if err != nil {
		return fmt.Errorf("soft delete message: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrForbidden
	}
	return nil
}

// ---------------------------------------------------------------------------
// Attachments
// ---------------------------------------------------------------------------

func (r *pgMessageRepository) CreateAttachment(ctx context.Context, a *models.Attachment) (*models.Attachment, error) {
	a.ID = uuid.New()
	err := r.db.QueryRow(ctx, `
		INSERT INTO message_attachments (id, message_id, file_url, s3_key, file_name, file_size, mime_type, width, height, duration, thumbnail, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW())
		RETURNING id, message_id, file_url, s3_key, file_name, file_size, mime_type, width, height, duration, thumbnail, created_at`,
		a.ID, a.MessageID, a.FileURL, a.S3Key, a.FileName, a.FileSize, a.MimeType, a.Width, a.Height, a.Duration, a.Thumbnail,
	).Scan(
		&a.ID, &a.MessageID, &a.FileURL, &a.S3Key, &a.FileName, &a.FileSize, &a.MimeType, &a.Width, &a.Height, &a.Duration, &a.Thumbnail, &a.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create attachment: %w", err)
	}
	return a, nil
}

func (r *pgMessageRepository) GetAttachments(ctx context.Context, messageID uuid.UUID) ([]models.Attachment, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, message_id, file_url, s3_key, file_name, file_size, mime_type, width, height, duration, thumbnail, created_at
		FROM message_attachments WHERE message_id = $1 ORDER BY created_at ASC`, messageID,
	)
	if err != nil {
		return nil, fmt.Errorf("get attachments: %w", err)
	}
	defer rows.Close()

	var attachments []models.Attachment
	for rows.Next() {
		var a models.Attachment
		if err = rows.Scan(&a.ID, &a.MessageID, &a.FileURL, &a.S3Key, &a.FileName, &a.FileSize, &a.MimeType, &a.Width, &a.Height, &a.Duration, &a.Thumbnail, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan attachment: %w", err)
		}
		attachments = append(attachments, a)
	}
	return attachments, rows.Err()
}

// ---------------------------------------------------------------------------
// Read receipts
// ---------------------------------------------------------------------------

func (r *pgMessageRepository) MarkAsRead(ctx context.Context, conversationID, userID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `
		UPDATE conversation_participants
		SET last_read_at = NOW()
		WHERE conversation_id = $1 AND user_id = $2`,
		conversationID, userID,
	)
	if err != nil {
		return fmt.Errorf("mark as read: %w", err)
	}

	// Record per-message receipts for messages that arrived after last_read_at
	_, err = r.db.Exec(ctx, `
		INSERT INTO message_read_receipts (message_id, user_id, read_at)
		SELECT m.id, $2, NOW()
		FROM messages m
		WHERE m.conversation_id = $1
		  AND m.sender_id != $2
		  AND m.deleted_at IS NULL
		ON CONFLICT (message_id, user_id) DO NOTHING`,
		conversationID, userID,
	)
	if err != nil {
		return fmt.Errorf("insert read receipts: %w", err)
	}
	return nil
}

func (r *pgMessageRepository) GetUnreadCount(ctx context.Context, conversationID, userID uuid.UUID) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM messages m
		JOIN conversation_participants cp ON cp.conversation_id = m.conversation_id AND cp.user_id = $2
		WHERE m.conversation_id = $1
		  AND m.sender_id != $2
		  AND m.created_at > cp.last_read_at
		  AND m.deleted_at IS NULL`,
		conversationID, userID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("get unread count: %w", err)
	}
	return count, nil
}

func (r *pgMessageRepository) GetTotalUnreadCount(ctx context.Context, userID uuid.UUID) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM messages m
		JOIN conversation_participants cp ON cp.conversation_id = m.conversation_id AND cp.user_id = $1
		WHERE m.sender_id != $1
		  AND m.created_at > cp.last_read_at
		  AND m.deleted_at IS NULL`,
		userID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("get total unread count: %w", err)
	}
	return count, nil
}

// ---------------------------------------------------------------------------
// Reactions
// ---------------------------------------------------------------------------

func (r *pgMessageRepository) AddReaction(ctx context.Context, react *models.MessageReaction) (*models.MessageReaction, error) {
	react.ID = uuid.New()
	err := r.db.QueryRow(ctx, `
		INSERT INTO message_reactions (id, message_id, user_id, emoji, created_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (message_id, user_id, emoji) DO UPDATE SET created_at = NOW()
		RETURNING id, message_id, user_id, emoji, created_at`,
		react.ID, react.MessageID, react.UserID, react.Emoji,
	).Scan(&react.ID, &react.MessageID, &react.UserID, &react.Emoji, &react.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("add reaction: %w", err)
	}
	return react, nil
}

func (r *pgMessageRepository) RemoveReaction(ctx context.Context, messageID, userID uuid.UUID, emoji string) error {
	ct, err := r.db.Exec(ctx, `
		DELETE FROM message_reactions WHERE message_id = $1 AND user_id = $2 AND emoji = $3`,
		messageID, userID, emoji,
	)
	if err != nil {
		return fmt.Errorf("remove reaction: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *pgMessageRepository) GetReactions(ctx context.Context, messageID uuid.UUID) ([]models.MessageReaction, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, message_id, user_id, emoji, created_at
		FROM message_reactions WHERE message_id = $1 ORDER BY created_at ASC`, messageID,
	)
	if err != nil {
		return nil, fmt.Errorf("get reactions: %w", err)
	}
	defer rows.Close()

	var reactions []models.MessageReaction
	for rows.Next() {
		var re models.MessageReaction
		if err = rows.Scan(&re.ID, &re.MessageID, &re.UserID, &re.Emoji, &re.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan reaction: %w", err)
		}
		reactions = append(reactions, re)
	}
	return reactions, rows.Err()
}

// ---------------------------------------------------------------------------
// Participant helpers
// ---------------------------------------------------------------------------

func (r *pgMessageRepository) IsParticipant(ctx context.Context, conversationID, userID uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM conversation_participants
			WHERE conversation_id = $1 AND user_id = $2
		)`, conversationID, userID,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check participant: %w", err)
	}
	return exists, nil
}

func (r *pgMessageRepository) GetParticipants(ctx context.Context, conversationID uuid.UUID) ([]models.Participant, error) {
	rows, err := r.db.Query(ctx, `
		SELECT conversation_id, user_id, joined_at, last_read_at, role, is_muted
		FROM conversation_participants WHERE conversation_id = $1`, conversationID,
	)
	if err != nil {
		return nil, fmt.Errorf("get participants: %w", err)
	}
	defer rows.Close()

	var participants []models.Participant
	for rows.Next() {
		var p models.Participant
		if err = rows.Scan(&p.ConversationID, &p.UserID, &p.JoinedAt, &p.LastReadAt, &p.Role, &p.IsMuted); err != nil {
			return nil, fmt.Errorf("scan participant: %w", err)
		}
		participants = append(participants, p)
	}
	return participants, rows.Err()
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// isMemberWithRole returns true if userID has one of the specified roles in the conversation.
func (r *pgMessageRepository) isMemberWithRole(ctx context.Context, conversationID, userID uuid.UUID, roles ...models.GroupRole) (bool, error) {
	roleStrings := make([]string, len(roles))
	for i, role := range roles {
		roleStrings[i] = string(role)
	}

	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM conversation_participants
			WHERE conversation_id = $1 AND user_id = $2 AND role = ANY($3)
		)`, conversationID, userID, roleStrings,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check member role: %w", err)
	}
	return exists, nil
}
