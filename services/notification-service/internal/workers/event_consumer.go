package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/IBM/sarama"
	"go.uber.org/zap"

	"github.com/tiktok-clone/notification-service/internal/config"
	"github.com/tiktok-clone/notification-service/internal/models"
	"github.com/tiktok-clone/notification-service/internal/services"
)

// EventConsumer consumes Kafka events and dispatches notifications.
type EventConsumer struct {
	group               sarama.ConsumerGroup
	notificationService services.NotificationService
	emailService        services.EmailService
	topics              []string
	topicCfg            config.KafkaTopics
	logger              *zap.Logger
}

// NewEventConsumer creates and returns a configured Kafka consumer group.
func NewEventConsumer(
	cfg config.KafkaConfig,
	notificationService services.NotificationService,
	emailService services.EmailService,
	logger *zap.Logger,
) (*EventConsumer, error) {
	scfg := sarama.NewConfig()
	scfg.Consumer.Return.Errors = true
	if cfg.AutoOffsetReset == "latest" {
		scfg.Consumer.Offsets.Initial = sarama.OffsetNewest
	} else {
		scfg.Consumer.Offsets.Initial = sarama.OffsetOldest
	}
	scfg.Consumer.Offsets.AutoCommit.Enable = false

	group, err := sarama.NewConsumerGroup(cfg.Brokers, cfg.GroupID, scfg)
	if err != nil {
		return nil, fmt.Errorf("create kafka consumer group: %w", err)
	}

	topics := cfg.Topics.AllTopics()
	logger.Info("kafka consumer group created",
		zap.Strings("topics", topics),
		zap.String("group_id", cfg.GroupID),
	)

	return &EventConsumer{
		group:               group,
		notificationService: notificationService,
		emailService:        emailService,
		topics:              topics,
		topicCfg:            cfg.Topics,
		logger:              logger,
	}, nil
}

// Start begins consuming messages. It blocks until ctx is cancelled.
// wg.Done is called once the consume loop exits.
func (ec *EventConsumer) Start(ctx context.Context, wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		handler := &consumerGroupHandler{ec: ec}
		for {
			if err := ec.group.Consume(ctx, ec.topics, handler); err != nil {
				ec.logger.Error("kafka consume error", zap.Error(err))
			}
			if ctx.Err() != nil {
				ec.logger.Info("kafka consumer shutting down")
				return
			}
		}
	}()

	// Log errors from the consumer group in background.
	go func() {
		for err := range ec.group.Errors() {
			ec.logger.Error("kafka consumer group error", zap.Error(err))
		}
	}()
}

// Close tears down the consumer group.
func (ec *EventConsumer) Close() error {
	return ec.group.Close()
}

// ---------------------------------------------------------------------------
// sarama.ConsumerGroupHandler implementation
// ---------------------------------------------------------------------------

type consumerGroupHandler struct {
	ec *EventConsumer
}

func (h *consumerGroupHandler) Setup(sarama.ConsumerGroupSession) error   { return nil }
func (h *consumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error { return nil }

func (h *consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case msg, ok := <-claim.Messages():
			if !ok {
				return nil
			}
			h.ec.handleMessage(session.Context(), msg)
			session.MarkMessage(msg, "")
			session.Commit()
		case <-session.Context().Done():
			return nil
		}
	}
}

// ---------------------------------------------------------------------------
// Message dispatch
// ---------------------------------------------------------------------------

func (ec *EventConsumer) handleMessage(ctx context.Context, msg *sarama.ConsumerMessage) {
	topic := msg.Topic

	var event models.KafkaEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		ec.logger.Error("unmarshal kafka event",
			zap.String("topic", topic),
			zap.Error(err),
		)
		return
	}

	ec.logger.Debug("received kafka event",
		zap.String("topic", topic),
		zap.String("event_type", event.EventType),
		zap.String("event_id", event.EventID),
	)

	h := &messageHandler{ec: ec, logger: ec.logger}
	var err error

	topics := ec.topicCfg
	switch topic {
	case topics.VideoLiked:
		err = h.handleVideoLiked(ctx, event)
	case topics.UserFollowed:
		err = h.handleUserFollowed(ctx, event)
	case topics.CommentCreated:
		err = h.handleCommentCreated(ctx, event)
	case topics.GiftSent:
		err = h.handleGiftSent(ctx, event)
	case topics.OrderCreated:
		err = h.handleOrderCreated(ctx, event)
	case topics.VideoMentioned:
		err = h.handleVideoMentioned(ctx, event)
	case topics.LiveStreamStart:
		err = h.handleLiveStreamStart(ctx, event)
	default:
		ec.logger.Warn("unhandled topic", zap.String("topic", topic))
	}

	if err != nil {
		ec.logger.Error("handle kafka event",
			zap.String("topic", topic),
			zap.String("event_id", event.EventID),
			zap.Error(err),
		)
	}
}

// ---------------------------------------------------------------------------
// messageHandler holds per-event processing logic.
// ---------------------------------------------------------------------------

type messageHandler struct {
	ec     *EventConsumer
	logger *zap.Logger
}

// ---------------------------------------------------------------------------
// Per-event handlers
// ---------------------------------------------------------------------------

func (h *messageHandler) handleVideoLiked(ctx context.Context, event models.KafkaEvent) error {
	p := event.Payload
	videoOwnerID := strFromMap(p, "video_owner_id")
	actorID := strFromMap(p, "actor_id")
	actorName := strFromMap(p, "actor_name")
	actorAvatar := strFromMap(p, "actor_avatar")
	videoID := strFromMap(p, "video_id")
	videoTitle := strFromMap(p, "video_title")

	if videoOwnerID == actorID {
		return nil
	}

	groupKey := fmt.Sprintf("like:%s", videoID)
	req := &models.CreateNotificationRequest{
		UserID:      videoOwnerID,
		ActorID:     actorID,
		ActorName:   actorName,
		ActorAvatar: actorAvatar,
		Type:        models.NotificationTypeLike,
		Title:       "New Like",
		Body:        fmt.Sprintf("%s liked your video", actorName),
		DeepLink:    fmt.Sprintf("/video/%s", videoID),
		GroupKey:    groupKey,
		Metadata: map[string]interface{}{
			"video_id":    videoID,
			"video_title": videoTitle,
			"actor_id":    actorID,
		},
	}
	_, err := h.ec.notificationService.CreateNotification(ctx, req)
	return err
}

func (h *messageHandler) handleUserFollowed(ctx context.Context, event models.KafkaEvent) error {
	p := event.Payload
	followedUserID := strFromMap(p, "followed_user_id")
	followerID := strFromMap(p, "follower_id")
	followerName := strFromMap(p, "follower_name")
	followerAvatar := strFromMap(p, "follower_avatar")

	if followedUserID == followerID {
		return nil
	}

	req := &models.CreateNotificationRequest{
		UserID:      followedUserID,
		ActorID:     followerID,
		ActorName:   followerName,
		ActorAvatar: followerAvatar,
		Type:        models.NotificationTypeFollow,
		Title:       "New Follower",
		Body:        fmt.Sprintf("%s started following you", followerName),
		DeepLink:    fmt.Sprintf("/profile/%s", followerID),
		GroupKey:    fmt.Sprintf("follow:%s", followedUserID),
		Metadata: map[string]interface{}{
			"follower_id":   followerID,
			"follower_name": followerName,
		},
	}
	_, err := h.ec.notificationService.CreateNotification(ctx, req)
	return err
}

func (h *messageHandler) handleCommentCreated(ctx context.Context, event models.KafkaEvent) error {
	p := event.Payload
	videoOwnerID := strFromMap(p, "video_owner_id")
	actorID := strFromMap(p, "actor_id")
	actorName := strFromMap(p, "actor_name")
	actorAvatar := strFromMap(p, "actor_avatar")
	videoID := strFromMap(p, "video_id")
	commentID := strFromMap(p, "comment_id")
	commentPreview := strFromMap(p, "comment_preview")

	if videoOwnerID == actorID {
		return nil
	}

	body := fmt.Sprintf("%s commented: %s", actorName, commentPreview)
	if len(body) > 100 {
		body = body[:97] + "..."
	}

	req := &models.CreateNotificationRequest{
		UserID:      videoOwnerID,
		ActorID:     actorID,
		ActorName:   actorName,
		ActorAvatar: actorAvatar,
		Type:        models.NotificationTypeComment,
		Title:       "New Comment",
		Body:        body,
		DeepLink:    fmt.Sprintf("/video/%s?comment=%s", videoID, commentID),
		GroupKey:    fmt.Sprintf("comment:%s", videoID),
		Metadata: map[string]interface{}{
			"video_id":   videoID,
			"comment_id": commentID,
		},
	}
	_, err := h.ec.notificationService.CreateNotification(ctx, req)
	return err
}

func (h *messageHandler) handleGiftSent(ctx context.Context, event models.KafkaEvent) error {
	p := event.Payload
	recipientID := strFromMap(p, "recipient_id")
	senderID := strFromMap(p, "sender_id")
	senderName := strFromMap(p, "sender_name")
	senderAvatar := strFromMap(p, "sender_avatar")
	giftName := strFromMap(p, "gift_name")
	videoID := strFromMap(p, "video_id")

	pushReq := &models.CreateNotificationRequest{
		UserID:      recipientID,
		ActorID:     senderID,
		ActorName:   senderName,
		ActorAvatar: senderAvatar,
		Type:        models.NotificationTypeGift,
		Title:       "You received a gift!",
		Body:        fmt.Sprintf("%s sent you a %s", senderName, giftName),
		DeepLink:    fmt.Sprintf("/video/%s", videoID),
		Channels:    []models.Channel{models.ChannelPush, models.ChannelInApp},
		Metadata: map[string]interface{}{
			"gift_name": giftName,
			"sender_id": senderID,
			"video_id":  videoID,
		},
	}
	if _, err := h.ec.notificationService.CreateNotification(ctx, pushReq); err != nil {
		return err
	}

	recipientEmail := strFromMap(p, "recipient_email")
	recipientName := strFromMap(p, "recipient_name")
	if recipientEmail != "" {
		giftValue, _ := p["gift_value"].(float64)
		emailData := services.GiftEmailData{
			SenderName:   senderName,
			GiftName:     giftName,
			GiftValue:    giftValue,
			Currency:     strFromMap(p, "currency"),
			VideoTitle:   strFromMap(p, "video_title"),
			DashboardURL: strFromMap(p, "dashboard_url"),
		}
		if err := h.ec.emailService.SendGiftReceived(ctx, recipientEmail, recipientName, emailData); err != nil {
			h.logger.Error("gift email delivery failed",
				zap.String("recipient_id", recipientID),
				zap.Error(err),
			)
		}
	}

	return nil
}

func (h *messageHandler) handleOrderCreated(ctx context.Context, event models.KafkaEvent) error {
	p := event.Payload
	buyerID := strFromMap(p, "buyer_id")
	buyerEmail := strFromMap(p, "buyer_email")
	buyerName := strFromMap(p, "buyer_name")
	orderID := strFromMap(p, "order_id")
	productName := strFromMap(p, "product_name")

	inAppReq := &models.CreateNotificationRequest{
		UserID:   buyerID,
		Type:     models.NotificationTypeOrderCreated,
		Title:    "Order Confirmed",
		Body:     fmt.Sprintf("Your order for %s has been confirmed.", productName),
		DeepLink: fmt.Sprintf("/orders/%s", orderID),
		Channels: []models.Channel{models.ChannelInApp},
		Metadata: map[string]interface{}{
			"order_id":     orderID,
			"product_name": productName,
		},
	}
	if _, err := h.ec.notificationService.CreateNotification(ctx, inAppReq); err != nil {
		return err
	}

	if buyerEmail != "" {
		totalAmount, _ := p["total_amount"].(float64)
		quantity, _ := p["quantity"].(float64)
		emailData := services.OrderEmailData{
			OrderID:      orderID,
			ProductName:  productName,
			Quantity:     int(quantity),
			TotalAmount:  totalAmount,
			Currency:     strFromMap(p, "currency"),
			SupportEmail: strFromMap(p, "support_email"),
		}
		if err := h.ec.emailService.SendOrderConfirmation(ctx, buyerEmail, buyerName, emailData); err != nil {
			h.logger.Error("order confirmation email failed",
				zap.String("order_id", orderID),
				zap.Error(err),
			)
		}
	}

	return nil
}

func (h *messageHandler) handleVideoMentioned(ctx context.Context, event models.KafkaEvent) error {
	p := event.Payload
	mentionedUserID := strFromMap(p, "mentioned_user_id")
	actorID := strFromMap(p, "actor_id")
	actorName := strFromMap(p, "actor_name")
	actorAvatar := strFromMap(p, "actor_avatar")
	videoID := strFromMap(p, "video_id")

	if mentionedUserID == actorID {
		return nil
	}

	req := &models.CreateNotificationRequest{
		UserID:      mentionedUserID,
		ActorID:     actorID,
		ActorName:   actorName,
		ActorAvatar: actorAvatar,
		Type:        models.NotificationTypeMention,
		Title:       "You were mentioned",
		Body:        fmt.Sprintf("%s mentioned you in a video", actorName),
		DeepLink:    fmt.Sprintf("/video/%s", videoID),
		Metadata: map[string]interface{}{
			"video_id": videoID,
			"actor_id": actorID,
		},
	}
	_, err := h.ec.notificationService.CreateNotification(ctx, req)
	return err
}

func (h *messageHandler) handleLiveStreamStart(ctx context.Context, event models.KafkaEvent) error {
	p := event.Payload
	streamerID := strFromMap(p, "streamer_id")
	streamerName := strFromMap(p, "streamer_name")
	streamerAvatar := strFromMap(p, "streamer_avatar")
	streamID := strFromMap(p, "stream_id")

	followerIDsRaw, _ := p["follower_ids"].([]interface{})
	for _, raw := range followerIDsRaw {
		followerID, ok := raw.(string)
		if !ok || followerID == streamerID {
			continue
		}
		req := &models.CreateNotificationRequest{
			UserID:      followerID,
			ActorID:     streamerID,
			ActorName:   streamerName,
			ActorAvatar: streamerAvatar,
			Type:        models.NotificationTypeLiveStream,
			Title:       "Live Now!",
			Body:        fmt.Sprintf("%s is live now — tap to watch!", streamerName),
			DeepLink:    fmt.Sprintf("/live/%s", streamID),
			Metadata: map[string]interface{}{
				"stream_id":   streamID,
				"streamer_id": streamerID,
			},
		}
		if _, err := h.ec.notificationService.CreateNotification(ctx, req); err != nil {
			h.logger.Error("livestream notification failed",
				zap.String("follower_id", followerID),
				zap.Error(err),
			)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Utility
// ---------------------------------------------------------------------------

func strFromMap(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
