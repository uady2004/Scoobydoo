package tests

import (
	"context"
	"encoding/hex"
	"errors"
	"mime/multipart"
	"net/textproto"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/tiktok-clone/messaging-service/internal/config"
	"github.com/tiktok-clone/messaging-service/internal/models"
	"github.com/tiktok-clone/messaging-service/internal/repositories"
	"github.com/tiktok-clone/messaging-service/internal/services"
)

// ---------------------------------------------------------------------------
// Mock: MessageRepository
// ---------------------------------------------------------------------------

type mockRepo struct{ mock.Mock }

func (m *mockRepo) CreateConversation(ctx context.Context, creatorID, recipientID uuid.UUID) (*models.Conversation, error) {
	args := m.Called(ctx, creatorID, recipientID)
	if v := args.Get(0); v != nil {
		return v.(*models.Conversation), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockRepo) GetConversation(ctx context.Context, id uuid.UUID) (*models.Conversation, error) {
	args := m.Called(ctx, id)
	if v := args.Get(0); v != nil {
		return v.(*models.Conversation), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockRepo) GetConversationByParticipants(ctx context.Context, userA, userB uuid.UUID) (*models.Conversation, error) {
	args := m.Called(ctx, userA, userB)
	if v := args.Get(0); v != nil {
		return v.(*models.Conversation), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockRepo) GetConversations(ctx context.Context, userID uuid.UUID, cursor string, limit int) (*models.ConversationsPage, error) {
	args := m.Called(ctx, userID, cursor, limit)
	if v := args.Get(0); v != nil {
		return v.(*models.ConversationsPage), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockRepo) CreateGroup(ctx context.Context, req *models.CreateGroupRequest, creatorID uuid.UUID) (*models.Conversation, error) {
	args := m.Called(ctx, req, creatorID)
	if v := args.Get(0); v != nil {
		return v.(*models.Conversation), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockRepo) AddGroupMember(ctx context.Context, conversationID uuid.UUID, adderID uuid.UUID, memberIDs []uuid.UUID) error {
	return m.Called(ctx, conversationID, adderID, memberIDs).Error(0)
}

func (m *mockRepo) RemoveGroupMember(ctx context.Context, conversationID, removerID, memberID uuid.UUID) error {
	return m.Called(ctx, conversationID, removerID, memberID).Error(0)
}

func (m *mockRepo) GetGroupChat(ctx context.Context, conversationID uuid.UUID) (*models.GroupChat, error) {
	args := m.Called(ctx, conversationID)
	if v := args.Get(0); v != nil {
		return v.(*models.GroupChat), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockRepo) SendMessage(ctx context.Context, msg *models.Message) (*models.Message, error) {
	args := m.Called(ctx, msg)
	if v := args.Get(0); v != nil {
		return v.(*models.Message), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockRepo) GetMessages(ctx context.Context, conversationID uuid.UUID, cursor string, limit int) (*models.MessagesPage, error) {
	args := m.Called(ctx, conversationID, cursor, limit)
	if v := args.Get(0); v != nil {
		return v.(*models.MessagesPage), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockRepo) GetMessage(ctx context.Context, id uuid.UUID) (*models.Message, error) {
	args := m.Called(ctx, id)
	if v := args.Get(0); v != nil {
		return v.(*models.Message), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockRepo) DeleteMessage(ctx context.Context, messageID, requestorID uuid.UUID) error {
	return m.Called(ctx, messageID, requestorID).Error(0)
}

func (m *mockRepo) CreateAttachment(ctx context.Context, a *models.Attachment) (*models.Attachment, error) {
	args := m.Called(ctx, a)
	if v := args.Get(0); v != nil {
		return v.(*models.Attachment), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockRepo) GetAttachments(ctx context.Context, messageID uuid.UUID) ([]models.Attachment, error) {
	args := m.Called(ctx, messageID)
	if v := args.Get(0); v != nil {
		return v.([]models.Attachment), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockRepo) MarkAsRead(ctx context.Context, conversationID, userID uuid.UUID) error {
	return m.Called(ctx, conversationID, userID).Error(0)
}

func (m *mockRepo) GetUnreadCount(ctx context.Context, conversationID, userID uuid.UUID) (int, error) {
	args := m.Called(ctx, conversationID, userID)
	return args.Int(0), args.Error(1)
}

func (m *mockRepo) GetTotalUnreadCount(ctx context.Context, userID uuid.UUID) (int, error) {
	args := m.Called(ctx, userID)
	return args.Int(0), args.Error(1)
}

func (m *mockRepo) AddReaction(ctx context.Context, r *models.MessageReaction) (*models.MessageReaction, error) {
	args := m.Called(ctx, r)
	if v := args.Get(0); v != nil {
		return v.(*models.MessageReaction), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockRepo) RemoveReaction(ctx context.Context, messageID, userID uuid.UUID, emoji string) error {
	return m.Called(ctx, messageID, userID, emoji).Error(0)
}

func (m *mockRepo) GetReactions(ctx context.Context, messageID uuid.UUID) ([]models.MessageReaction, error) {
	args := m.Called(ctx, messageID)
	if v := args.Get(0); v != nil {
		return v.([]models.MessageReaction), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockRepo) IsParticipant(ctx context.Context, conversationID, userID uuid.UUID) (bool, error) {
	args := m.Called(ctx, conversationID, userID)
	return args.Bool(0), args.Error(1)
}

func (m *mockRepo) GetParticipants(ctx context.Context, conversationID uuid.UUID) ([]models.Participant, error) {
	args := m.Called(ctx, conversationID)
	if v := args.Get(0); v != nil {
		return v.([]models.Participant), args.Error(1)
	}
	return nil, args.Error(1)
}

// ---------------------------------------------------------------------------
// Mock: WSBroadcaster
// ---------------------------------------------------------------------------

type mockBroadcaster struct{ mock.Mock }

func (b *mockBroadcaster) BroadcastToUser(userID uuid.UUID, event *models.WSEvent) {
	b.Called(userID, event)
}

func (b *mockBroadcaster) BroadcastToConversation(conversationID uuid.UUID, event *models.WSEvent, excludeUserID *uuid.UUID) {
	b.Called(conversationID, event, excludeUserID)
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

const testKeyHex = "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"

func testConfig() *config.Config {
	return &config.Config{
		Crypto: config.CryptoConfig{EncryptionKeyHex: testKeyHex},
		JWT:    config.JWTConfig{Secret: "test-secret"},
		S3: config.S3Config{
			Endpoint:        "http://localhost:9000",
			Region:          "us-east-1",
			Bucket:          "test-bucket",
			AccessKeyID:     "test",
			SecretAccessKey: "test",
			UsePathStyle:    true,
		},
	}
}

func buildService(t *testing.T, repo repositories.MessageRepository, broadcaster services.WSBroadcaster) services.MessageService {
	t.Helper()
	logger := zap.NewNop()
	svc, err := services.NewMessageService(repo, broadcaster, testConfig(), logger)
	require.NoError(t, err)
	return svc
}

// ---------------------------------------------------------------------------
// Tests: CreateConversation
// ---------------------------------------------------------------------------

func TestCreateConversation_Success(t *testing.T) {
	repo := &mockRepo{}
	bcast := &mockBroadcaster{}
	svc := buildService(t, repo, bcast)

	creatorID := uuid.New()
	recipientID := uuid.New()
	convID := uuid.New()

	expected := &models.Conversation{
		ID:          convID,
		Type:        models.ConversationTypeDirect,
		CreatedByID: creatorID,
		CreatedAt:   time.Now(),
	}
	repo.On("CreateConversation", mock.Anything, creatorID, recipientID).Return(expected, nil)

	got, err := svc.CreateConversation(context.Background(), creatorID, recipientID)
	require.NoError(t, err)
	assert.Equal(t, convID, got.ID)
	repo.AssertExpectations(t)
}

func TestCreateConversation_SelfNotAllowed(t *testing.T) {
	repo := &mockRepo{}
	bcast := &mockBroadcaster{}
	svc := buildService(t, repo, bcast)

	id := uuid.New()
	_, err := svc.CreateConversation(context.Background(), id, id)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "yourself")
}

// ---------------------------------------------------------------------------
// Tests: SendMessage with encryption
// ---------------------------------------------------------------------------

func TestSendMessage_EncryptsContent(t *testing.T) {
	repo := &mockRepo{}
	bcast := &mockBroadcaster{}
	svc := buildService(t, repo, bcast)

	senderID := uuid.New()
	convID := uuid.New()
	msgID := uuid.New()
	plaintext := "Hello, encrypted world!"

	repo.On("IsParticipant", mock.Anything, convID, senderID).Return(true, nil)

	// The service encrypts content before calling SendMessage on the repo.
	// Capture what was passed so we can verify it is NOT the plaintext.
	var capturedMsg *models.Message
	repo.On("SendMessage", mock.Anything, mock.MatchedBy(func(m *models.Message) bool {
		capturedMsg = m
		return m.ConversationID == convID && m.SenderID == senderID
	})).Return(&models.Message{
		ID:               msgID,
		ConversationID:   convID,
		SenderID:         senderID,
		EncryptedContent: "will-be-replaced-by-capture",
		Nonce:            "nonce",
		Type:             models.MessageTypeText,
		Status:           models.MessageStatusSent,
		CreatedAt:        time.Now(),
	}, nil)

	bcast.On("BroadcastToConversation", convID, mock.Anything, mock.Anything).Return()

	req := &models.SendMessageRequest{
		ConversationID: convID,
		Content:        plaintext,
		Type:           models.MessageTypeText,
	}
	got, err := svc.SendMessage(context.Background(), senderID, req)
	require.NoError(t, err)
	assert.Equal(t, msgID, got.ID)

	// The plaintext is returned in Content for the caller.
	assert.Equal(t, plaintext, got.Content)

	// The repo received ciphertext, not plaintext.
	require.NotNil(t, capturedMsg)
	assert.NotEqual(t, plaintext, capturedMsg.EncryptedContent, "ciphertext must not equal plaintext")
	assert.NotEmpty(t, capturedMsg.Nonce, "nonce must be set")

	repo.AssertExpectations(t)
	bcast.AssertExpectations(t)
}

func TestSendMessage_ForbiddenWhenNotParticipant(t *testing.T) {
	repo := &mockRepo{}
	bcast := &mockBroadcaster{}
	svc := buildService(t, repo, bcast)

	senderID := uuid.New()
	convID := uuid.New()

	repo.On("IsParticipant", mock.Anything, convID, senderID).Return(false, nil)

	_, err := svc.SendMessage(context.Background(), senderID, &models.SendMessageRequest{
		ConversationID: convID,
		Content:        "test",
		Type:           models.MessageTypeText,
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, repositories.ErrForbidden))
}

// ---------------------------------------------------------------------------
// Tests: GetMessages with decryption
// ---------------------------------------------------------------------------

func TestGetMessages_DecryptsContent(t *testing.T) {
	repo := &mockRepo{}
	bcast := &mockBroadcaster{}
	svc := buildService(t, repo, bcast)

	userID := uuid.New()
	convID := uuid.New()
	msgID := uuid.New()
	plaintext := "decryption test message"

	// Encrypt a real ciphertext using the service's crypto so the test is end-to-end.
	// We reach into the service via a SendMessage mock to capture real ciphertext.
	var ciphertext, nonce string
	{
		key, _ := hex.DecodeString(testKeyHex)
		_ = key
		// Use the service's own helper by calling the exported DecryptContent interface.
		// To get a real ciphertext we call SendMessage and capture it.
		senderID := userID
		repo.On("IsParticipant", mock.Anything, convID, senderID).Return(true, nil).Once()
		repo.On("SendMessage", mock.Anything, mock.MatchedBy(func(m *models.Message) bool {
			ciphertext = m.EncryptedContent
			nonce = m.Nonce
			return true
		})).Return(&models.Message{
			ID:               msgID,
			ConversationID:   convID,
			SenderID:         senderID,
			EncryptedContent: "placeholder",
			Nonce:            "placeholder",
			Type:             models.MessageTypeText,
			Status:           models.MessageStatusSent,
			CreatedAt:        time.Now(),
		}, nil).Once()
		bcast.On("BroadcastToConversation", mock.Anything, mock.Anything, mock.Anything).Return().Once()
		_, _ = svc.SendMessage(context.Background(), senderID, &models.SendMessageRequest{
			ConversationID: convID, Content: plaintext, Type: models.MessageTypeText,
		})
	}

	// Now set up GetMessages to return the stored ciphertext.
	repo.On("IsParticipant", mock.Anything, convID, userID).Return(true, nil).Once()
	repo.On("GetMessages", mock.Anything, convID, "", 30).Return(&models.MessagesPage{
		Messages: []models.Message{
			{
				ID:               msgID,
				ConversationID:   convID,
				SenderID:         userID,
				EncryptedContent: ciphertext,
				Nonce:            nonce,
				Type:             models.MessageTypeText,
				Status:           models.MessageStatusSent,
				CreatedAt:        time.Now(),
			},
		},
		HasMore: false,
	}, nil).Once()
	repo.On("GetAttachments", mock.Anything, msgID).Return([]models.Attachment{}, nil).Once()
	repo.On("GetReactions", mock.Anything, msgID).Return([]models.MessageReaction{}, nil).Once()

	page, err := svc.GetMessages(context.Background(), userID, convID, "", 30)
	require.NoError(t, err)
	require.Len(t, page.Messages, 1)
	assert.Equal(t, plaintext, page.Messages[0].Content)

	repo.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// Tests: MarkRead
// ---------------------------------------------------------------------------

func TestMarkRead_BroadcastsReadReceipt(t *testing.T) {
	repo := &mockRepo{}
	bcast := &mockBroadcaster{}
	svc := buildService(t, repo, bcast)

	userID := uuid.New()
	convID := uuid.New()

	repo.On("IsParticipant", mock.Anything, convID, userID).Return(true, nil)
	repo.On("MarkAsRead", mock.Anything, convID, userID).Return(nil)
	bcast.On("BroadcastToConversation", convID, mock.MatchedBy(func(e *models.WSEvent) bool {
		return e.Type == models.WSEventReadReceipt
	}), mock.Anything).Return()

	err := svc.MarkRead(context.Background(), convID, userID)
	require.NoError(t, err)

	bcast.AssertCalled(t, "BroadcastToConversation", convID, mock.Anything, mock.Anything)
}

func TestMarkRead_ForbiddenWhenNotParticipant(t *testing.T) {
	repo := &mockRepo{}
	bcast := &mockBroadcaster{}
	svc := buildService(t, repo, bcast)

	userID := uuid.New()
	convID := uuid.New()

	repo.On("IsParticipant", mock.Anything, convID, userID).Return(false, nil)

	err := svc.MarkRead(context.Background(), convID, userID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, repositories.ErrForbidden))
}

// ---------------------------------------------------------------------------
// Tests: DeleteMessage
// ---------------------------------------------------------------------------

func TestDeleteMessage_BroadcastsDeletion(t *testing.T) {
	repo := &mockRepo{}
	bcast := &mockBroadcaster{}
	svc := buildService(t, repo, bcast)

	requestorID := uuid.New()
	msgID := uuid.New()
	convID := uuid.New()

	repo.On("DeleteMessage", mock.Anything, msgID, requestorID).Return(nil)
	now := time.Now()
	repo.On("GetMessage", mock.Anything, msgID).Return(&models.Message{
		ID:             msgID,
		ConversationID: convID,
		SenderID:       requestorID,
		DeletedAt:      &now,
	}, nil)
	bcast.On("BroadcastToConversation", convID, mock.MatchedBy(func(e *models.WSEvent) bool {
		return e.Type == models.WSEventMessageDeleted
	}), (*uuid.UUID)(nil)).Return()

	err := svc.DeleteMessage(context.Background(), msgID, requestorID)
	require.NoError(t, err)
	bcast.AssertExpectations(t)
}

func TestDeleteMessage_Forbidden(t *testing.T) {
	repo := &mockRepo{}
	bcast := &mockBroadcaster{}
	svc := buildService(t, repo, bcast)

	repo.On("DeleteMessage", mock.Anything, mock.Anything, mock.Anything).Return(repositories.ErrForbidden)

	err := svc.DeleteMessage(context.Background(), uuid.New(), uuid.New())
	require.Error(t, err)
	assert.True(t, errors.Is(err, repositories.ErrForbidden))
}

// ---------------------------------------------------------------------------
// Tests: CreateGroup
// ---------------------------------------------------------------------------

func TestCreateGroup_AddsSelfToMembers(t *testing.T) {
	repo := &mockRepo{}
	bcast := &mockBroadcaster{}
	svc := buildService(t, repo, bcast)

	creatorID := uuid.New()
	member1 := uuid.New()
	member2 := uuid.New()

	// creator is NOT in MemberIDs initially — service should add it automatically.
	req := &models.CreateGroupRequest{
		Name:      "Test Group",
		MemberIDs: []uuid.UUID{member1, member2},
	}

	convID := uuid.New()
	repo.On("CreateGroup", mock.Anything, mock.MatchedBy(func(r *models.CreateGroupRequest) bool {
		for _, id := range r.MemberIDs {
			if id == creatorID {
				return true
			}
		}
		return false
	}), creatorID).Return(&models.Conversation{
		ID:          convID,
		Type:        models.ConversationTypeGroup,
		CreatedByID: creatorID,
	}, nil)

	conv, err := svc.CreateGroup(context.Background(), creatorID, req)
	require.NoError(t, err)
	assert.Equal(t, convID, conv.ID)
	repo.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// Tests: AddReaction broadcasts event
// ---------------------------------------------------------------------------

func TestAddReaction_BroadcastsEvent(t *testing.T) {
	repo := &mockRepo{}
	bcast := &mockBroadcaster{}
	svc := buildService(t, repo, bcast)

	userID := uuid.New()
	msgID := uuid.New()
	convID := uuid.New()
	emoji := "❤️"

	reaction := &models.MessageReaction{
		ID:        uuid.New(),
		MessageID: msgID,
		UserID:    userID,
		Emoji:     emoji,
		CreatedAt: time.Now(),
	}
	repo.On("AddReaction", mock.Anything, mock.MatchedBy(func(r *models.MessageReaction) bool {
		return r.MessageID == msgID && r.UserID == userID && r.Emoji == emoji
	})).Return(reaction, nil)
	repo.On("GetMessage", mock.Anything, msgID).Return(&models.Message{
		ID:             msgID,
		ConversationID: convID,
		SenderID:       uuid.New(),
	}, nil)
	bcast.On("BroadcastToConversation", convID, mock.MatchedBy(func(e *models.WSEvent) bool {
		return e.Type == models.WSEventReaction
	}), (*uuid.UUID)(nil)).Return()

	r, err := svc.AddReaction(context.Background(), msgID, userID, emoji)
	require.NoError(t, err)
	assert.Equal(t, emoji, r.Emoji)
	bcast.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// Tests: GetUnreadCount / GetTotalUnreadCount
// ---------------------------------------------------------------------------

func TestGetUnreadCount(t *testing.T) {
	repo := &mockRepo{}
	bcast := &mockBroadcaster{}
	svc := buildService(t, repo, bcast)

	userID := uuid.New()
	convID := uuid.New()

	repo.On("GetUnreadCount", mock.Anything, convID, userID).Return(7, nil)

	count, err := svc.GetUnreadCount(context.Background(), convID, userID)
	require.NoError(t, err)
	assert.Equal(t, 7, count)
}

func TestGetTotalUnreadCount(t *testing.T) {
	repo := &mockRepo{}
	bcast := &mockBroadcaster{}
	svc := buildService(t, repo, bcast)

	userID := uuid.New()
	repo.On("GetTotalUnreadCount", mock.Anything, userID).Return(42, nil)

	count, err := svc.GetTotalUnreadCount(context.Background(), userID)
	require.NoError(t, err)
	assert.Equal(t, 42, count)
}

// ---------------------------------------------------------------------------
// Tests: ShareMedia - placeholder (S3 requires real/mock client)
// ---------------------------------------------------------------------------

func TestShareMedia_ForbiddenWhenNotParticipant(t *testing.T) {
	repo := &mockRepo{}
	bcast := &mockBroadcaster{}
	svc := buildService(t, repo, bcast)

	senderID := uuid.New()
	convID := uuid.New()

	repo.On("IsParticipant", mock.Anything, convID, senderID).Return(false, nil)

	fh := &multipart.FileHeader{
		Filename: "test.jpg",
		Header:   make(textproto.MIMEHeader),
		Size:     1024,
	}
	_, err := svc.ShareMedia(context.Background(), senderID, convID, fh)
	require.Error(t, err)
	assert.True(t, errors.Is(err, repositories.ErrForbidden))
}

// ---------------------------------------------------------------------------
// Tests: GetConversations delegation
// ---------------------------------------------------------------------------

func TestGetConversations_Delegates(t *testing.T) {
	repo := &mockRepo{}
	bcast := &mockBroadcaster{}
	svc := buildService(t, repo, bcast)

	userID := uuid.New()
	expected := &models.ConversationsPage{
		Conversations: []models.Conversation{
			{ID: uuid.New(), Type: models.ConversationTypeDirect},
		},
		HasMore: false,
	}
	repo.On("GetConversations", mock.Anything, userID, "", 20).Return(expected, nil)

	page, err := svc.GetConversations(context.Background(), userID, "", 20)
	require.NoError(t, err)
	assert.Len(t, page.Conversations, 1)
	repo.AssertExpectations(t)
}
