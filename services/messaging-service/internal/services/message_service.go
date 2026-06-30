package services

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/tiktok-clone/messaging-service/internal/config"
	"github.com/tiktok-clone/messaging-service/internal/models"
	"github.com/tiktok-clone/messaging-service/internal/repositories"
)

// MessageService is the application-layer facade for all messaging operations.
type MessageService interface {
	// Conversation lifecycle
	CreateConversation(ctx context.Context, requesterID, recipientID uuid.UUID) (*models.Conversation, error)
	GetConversations(ctx context.Context, userID uuid.UUID, cursor string, limit int) (*models.ConversationsPage, error)

	// Messaging
	SendMessage(ctx context.Context, senderID uuid.UUID, req *models.SendMessageRequest) (*models.Message, error)
	GetMessages(ctx context.Context, requesterID uuid.UUID, conversationID uuid.UUID, cursor string, limit int) (*models.MessagesPage, error)
	MarkRead(ctx context.Context, conversationID, userID uuid.UUID) error
	DeleteMessage(ctx context.Context, messageID, requestorID uuid.UUID) error
	GetUnreadCount(ctx context.Context, conversationID, userID uuid.UUID) (int, error)
	GetTotalUnreadCount(ctx context.Context, userID uuid.UUID) (int, error)

	// Group management
	CreateGroup(ctx context.Context, creatorID uuid.UUID, req *models.CreateGroupRequest) (*models.Conversation, error)
	AddGroupMembers(ctx context.Context, conversationID, adderID uuid.UUID, req *models.AddGroupMemberRequest) error
	RemoveGroupMember(ctx context.Context, conversationID, removerID uuid.UUID, req *models.RemoveGroupMemberRequest) error

	// Media
	ShareMedia(ctx context.Context, senderID uuid.UUID, conversationID uuid.UUID, fileHeader *multipart.FileHeader) (*models.ShareMediaResponse, error)

	// Reactions
	AddReaction(ctx context.Context, messageID, userID uuid.UUID, emoji string) (*models.MessageReaction, error)
	RemoveReaction(ctx context.Context, messageID, userID uuid.UUID, emoji string) error

	// Crypto helpers exposed for the WebSocket layer
	DecryptContent(ciphertext, nonce string) (string, error)
}

// WSBroadcaster is a narrow interface the service uses to push real-time events
// without depending on the full WebSocket hub implementation.
type WSBroadcaster interface {
	BroadcastToUser(userID uuid.UUID, event *models.WSEvent)
	BroadcastToConversation(conversationID uuid.UUID, event *models.WSEvent, excludeUserID *uuid.UUID)
}

type messageService struct {
	repo        repositories.MessageRepository
	broadcaster WSBroadcaster
	s3Client    *s3.Client
	cfg         *config.Config
	encKey      []byte // 32-byte AES-256 key
	log         *zap.Logger
}

// NewMessageService constructs and validates a MessageService.
func NewMessageService(
	repo repositories.MessageRepository,
	broadcaster WSBroadcaster,
	cfg *config.Config,
	log *zap.Logger,
) (MessageService, error) {
	key, err := hex.DecodeString(cfg.Crypto.EncryptionKeyHex)
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("invalid encryption key: must be 64 hex chars (32 bytes)")
	}

	s3Client, err := buildS3Client(cfg)
	if err != nil {
		return nil, fmt.Errorf("build s3 client: %w", err)
	}

	return &messageService{
		repo:        repo,
		broadcaster: broadcaster,
		s3Client:    s3Client,
		cfg:         cfg,
		encKey:      key,
		log:         log,
	}, nil
}

// ---------------------------------------------------------------------------
// Conversation lifecycle
// ---------------------------------------------------------------------------

func (s *messageService) CreateConversation(ctx context.Context, requesterID, recipientID uuid.UUID) (*models.Conversation, error) {
	if requesterID == recipientID {
		return nil, errors.New("cannot create conversation with yourself")
	}
	return s.repo.CreateConversation(ctx, requesterID, recipientID)
}

func (s *messageService) GetConversations(ctx context.Context, userID uuid.UUID, cursor string, limit int) (*models.ConversationsPage, error) {
	return s.repo.GetConversations(ctx, userID, cursor, limit)
}

// ---------------------------------------------------------------------------
// Messaging
// ---------------------------------------------------------------------------

// SendMessage encrypts the content with AES-256-GCM, persists the message, then
// broadcasts a new_message WebSocket event to all participants of the conversation.
func (s *messageService) SendMessage(ctx context.Context, senderID uuid.UUID, req *models.SendMessageRequest) (*models.Message, error) {
	// Authorization: sender must be a participant
	ok, err := s.repo.IsParticipant(ctx, req.ConversationID, senderID)
	if err != nil {
		return nil, fmt.Errorf("participant check: %w", err)
	}
	if !ok {
		return nil, repositories.ErrForbidden
	}

	// Encrypt content
	ciphertext, nonce, err := s.encryptContent(req.Content)
	if err != nil {
		return nil, fmt.Errorf("encrypt content: %w", err)
	}

	msg := &models.Message{
		ConversationID:   req.ConversationID,
		SenderID:         senderID,
		EncryptedContent: ciphertext,
		Nonce:            nonce,
		Type:             req.Type,
		ReplyToID:        req.ReplyToID,
	}

	saved, err := s.repo.SendMessage(ctx, msg)
	if err != nil {
		return nil, fmt.Errorf("persist message: %w", err)
	}

	// Decrypt for the response (plaintext goes to caller only, not stored)
	saved.Content = req.Content

	// Hydrate reply-to if present
	if saved.ReplyToID != nil {
		if parent, err := s.repo.GetMessage(ctx, *saved.ReplyToID); err == nil {
			if decrypted, err := s.DecryptContent(parent.EncryptedContent, parent.Nonce); err == nil {
				parent.Content = decrypted
			}
			saved.ReplyToMessage = parent
		}
	}

	// Broadcast to all participants except sender (they already have the message)
	excludeID := senderID
	s.broadcastNewMessage(saved, &excludeID)

	return saved, nil
}

func (s *messageService) GetMessages(ctx context.Context, requesterID uuid.UUID, conversationID uuid.UUID, cursor string, limit int) (*models.MessagesPage, error) {
	ok, err := s.repo.IsParticipant(ctx, conversationID, requesterID)
	if err != nil {
		return nil, fmt.Errorf("participant check: %w", err)
	}
	if !ok {
		return nil, repositories.ErrForbidden
	}

	page, err := s.repo.GetMessages(ctx, conversationID, cursor, limit)
	if err != nil {
		return nil, err
	}

	// Decrypt every message for the response
	for i := range page.Messages {
		m := &page.Messages[i]
		if m.IsDeleted() {
			m.Content = ""
			continue
		}
		plain, err := s.DecryptContent(m.EncryptedContent, m.Nonce)
		if err != nil {
			s.log.Warn("failed to decrypt message", zap.String("message_id", m.ID.String()), zap.Error(err))
			m.Content = "[decryption error]"
		} else {
			m.Content = plain
		}

		// Load attachments
		if atts, err := s.repo.GetAttachments(ctx, m.ID); err == nil {
			m.Attachments = atts
		}

		// Load reactions
		if reactions, err := s.repo.GetReactions(ctx, m.ID); err == nil {
			m.Reactions = reactions
		}
	}

	return page, nil
}

func (s *messageService) MarkRead(ctx context.Context, conversationID, userID uuid.UUID) error {
	ok, err := s.repo.IsParticipant(ctx, conversationID, userID)
	if err != nil {
		return err
	}
	if !ok {
		return repositories.ErrForbidden
	}

	if err = s.repo.MarkAsRead(ctx, conversationID, userID); err != nil {
		return err
	}

	// Notify other participants that this user has read up to now
	event := &models.WSEvent{
		Type:           models.WSEventReadReceipt,
		ConversationID: conversationID.String(),
		SenderID:       userID.String(),
		Payload: models.ReadReceiptPayload{
			ConversationID: conversationID.String(),
			LastReadAt:     time.Now().UTC(),
			UserID:         userID.String(),
		},
		Timestamp: time.Now().UTC(),
	}
	excludeID := userID
	s.broadcaster.BroadcastToConversation(conversationID, event, &excludeID)
	return nil
}

func (s *messageService) DeleteMessage(ctx context.Context, messageID, requestorID uuid.UUID) error {
	if err := s.repo.DeleteMessage(ctx, messageID, requestorID); err != nil {
		return err
	}

	// Retrieve conversation ID to broadcast deletion
	msg, err := s.repo.GetMessage(ctx, messageID)
	if err != nil {
		return nil // message deleted, broadcast best-effort
	}

	event := &models.WSEvent{
		Type:           models.WSEventMessageDeleted,
		ConversationID: msg.ConversationID.String(),
		SenderID:       requestorID.String(),
		Payload:        map[string]string{"message_id": messageID.String()},
		Timestamp:      time.Now().UTC(),
	}
	s.broadcaster.BroadcastToConversation(msg.ConversationID, event, nil)
	return nil
}

func (s *messageService) GetUnreadCount(ctx context.Context, conversationID, userID uuid.UUID) (int, error) {
	return s.repo.GetUnreadCount(ctx, conversationID, userID)
}

func (s *messageService) GetTotalUnreadCount(ctx context.Context, userID uuid.UUID) (int, error) {
	return s.repo.GetTotalUnreadCount(ctx, userID)
}

// ---------------------------------------------------------------------------
// Group management
// ---------------------------------------------------------------------------

func (s *messageService) CreateGroup(ctx context.Context, creatorID uuid.UUID, req *models.CreateGroupRequest) (*models.Conversation, error) {
	// Ensure the creator is in the member list
	hasSelf := false
	for _, id := range req.MemberIDs {
		if id == creatorID {
			hasSelf = true
			break
		}
	}
	if !hasSelf {
		req.MemberIDs = append(req.MemberIDs, creatorID)
	}

	return s.repo.CreateGroup(ctx, req, creatorID)
}

func (s *messageService) AddGroupMembers(ctx context.Context, conversationID, adderID uuid.UUID, req *models.AddGroupMemberRequest) error {
	return s.repo.AddGroupMember(ctx, conversationID, adderID, req.UserIDs)
}

func (s *messageService) RemoveGroupMember(ctx context.Context, conversationID, removerID uuid.UUID, req *models.RemoveGroupMemberRequest) error {
	return s.repo.RemoveGroupMember(ctx, conversationID, removerID, req.UserID)
}

// ---------------------------------------------------------------------------
// Media upload (S3/MinIO)
// ---------------------------------------------------------------------------

// ShareMedia uploads the file to S3 and creates an Attachment record.
// The attachment is linked to a sentinel "media" message; the caller is responsible
// for associating it with an actual message via AttachmentIDs in SendMessage.
func (s *messageService) ShareMedia(ctx context.Context, senderID uuid.UUID, conversationID uuid.UUID, fileHeader *multipart.FileHeader) (*models.ShareMediaResponse, error) {
	ok, err := s.repo.IsParticipant(ctx, conversationID, senderID)
	if err != nil {
		return nil, fmt.Errorf("participant check: %w", err)
	}
	if !ok {
		return nil, repositories.ErrForbidden
	}

	f, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	objectKey := fmt.Sprintf("messaging/%s/%s%s", conversationID, uuid.New(), ext)
	mimeType := fileHeader.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	_, err = s.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.cfg.S3.Bucket),
		Key:         aws.String(objectKey),
		Body:        f,
		ContentType: aws.String(mimeType),
		// Objects are private; presigned URLs are generated per-request
	})
	if err != nil {
		return nil, fmt.Errorf("s3 put object: %w", err)
	}

	// Generate a 1-hour presigned URL
	presignClient := s3.NewPresignClient(s.s3Client)
	presigned, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.cfg.S3.Bucket),
		Key:    aws.String(objectKey),
	}, s3.WithPresignExpires(time.Hour))
	if err != nil {
		return nil, fmt.Errorf("presign url: %w", err)
	}

	// Persist a placeholder attachment (message_id = zero UUID; linked later via SendMessage)
	att := &models.Attachment{
		MessageID: uuid.Nil, // will be updated when linked to a message
		FileURL:   presigned.URL,
		S3Key:     objectKey,
		FileName:  fileHeader.Filename,
		FileSize:  fileHeader.Size,
		MimeType:  mimeType,
	}
	saved, err := s.repo.CreateAttachment(ctx, att)
	if err != nil {
		return nil, fmt.Errorf("create attachment record: %w", err)
	}

	return &models.ShareMediaResponse{
		AttachmentID: saved.ID,
		FileURL:      saved.FileURL,
		S3Key:        saved.S3Key,
		MimeType:     saved.MimeType,
		FileSize:     saved.FileSize,
	}, nil
}

// ---------------------------------------------------------------------------
// Reactions
// ---------------------------------------------------------------------------

func (s *messageService) AddReaction(ctx context.Context, messageID, userID uuid.UUID, emoji string) (*models.MessageReaction, error) {
	react := &models.MessageReaction{
		MessageID: messageID,
		UserID:    userID,
		Emoji:     emoji,
	}
	saved, err := s.repo.AddReaction(ctx, react)
	if err != nil {
		return nil, err
	}

	// Broadcast to conversation
	msg, err := s.repo.GetMessage(ctx, messageID)
	if err == nil {
		event := &models.WSEvent{
			Type:           models.WSEventReaction,
			ConversationID: msg.ConversationID.String(),
			SenderID:       userID.String(),
			Payload:        saved,
			Timestamp:      time.Now().UTC(),
		}
		s.broadcaster.BroadcastToConversation(msg.ConversationID, event, nil)
	}
	return saved, nil
}

func (s *messageService) RemoveReaction(ctx context.Context, messageID, userID uuid.UUID, emoji string) error {
	return s.repo.RemoveReaction(ctx, messageID, userID, emoji)
}

// ---------------------------------------------------------------------------
// Encryption / Decryption (AES-256-GCM)
// ---------------------------------------------------------------------------

// encryptContent encrypts plaintext using AES-256-GCM and returns
// (base64-ciphertext, base64-nonce, error).
func (s *messageService) encryptContent(plaintext string) (ciphertext, nonce string, err error) {
	block, err := aes.NewCipher(s.encKey)
	if err != nil {
		return "", "", fmt.Errorf("new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", fmt.Errorf("new gcm: %w", err)
	}

	nonceBytes := make([]byte, gcm.NonceSize()) // 12 bytes
	if _, err = io.ReadFull(rand.Reader, nonceBytes); err != nil {
		return "", "", fmt.Errorf("generate nonce: %w", err)
	}

	encrypted := gcm.Seal(nil, nonceBytes, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(encrypted),
		base64.StdEncoding.EncodeToString(nonceBytes),
		nil
}

// DecryptContent is the inverse of encryptContent. It is exported so the
// WebSocket layer can decrypt inbound messages for server-side processing.
func (s *messageService) DecryptContent(ciphertextB64, nonceB64 string) (string, error) {
	ciphertextBytes, err := base64.StdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}
	nonceBytes, err := base64.StdEncoding.DecodeString(nonceB64)
	if err != nil {
		return "", fmt.Errorf("decode nonce: %w", err)
	}

	block, err := aes.NewCipher(s.encKey)
	if err != nil {
		return "", fmt.Errorf("new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("new gcm: %w", err)
	}

	plaintext, err := gcm.Open(nil, nonceBytes, ciphertextBytes, nil)
	if err != nil {
		return "", fmt.Errorf("gcm open: %w", err)
	}
	return string(plaintext), nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func (s *messageService) broadcastNewMessage(msg *models.Message, excludeUserID *uuid.UUID) {
	event := &models.WSEvent{
		Type:           models.WSEventNewMessage,
		ConversationID: msg.ConversationID.String(),
		SenderID:       msg.SenderID.String(),
		Payload:        msg,
		Timestamp:      time.Now().UTC(),
	}
	s.broadcaster.BroadcastToConversation(msg.ConversationID, event, excludeUserID)
}

func buildS3Client(cfg *config.Config) (*s3.Client, error) {
	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.S3.Region),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.S3.AccessKeyID, cfg.S3.SecretAccessKey, ""),
		),
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.S3.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.S3.Endpoint)
		}
		o.UsePathStyle = cfg.S3.UsePathStyle
	})
	return client, nil
}
