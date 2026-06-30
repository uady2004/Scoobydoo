package websocket

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	gorillaws "github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/tiktok-clone/messaging-service/internal/models"
)

// Client represents a single WebSocket connection from one browser tab / device.
//
// Each Client has:
//   - A goroutine running readPump that reads frames from the network.
//   - A goroutine running writePump that serialises outbound events.
//   - A buffered send channel bridging the hub and the write pump.
type Client struct {
	hub             *Hub
	conn            *gorillaws.Conn
	send            chan *models.WSEvent
	userID          uuid.UUID
	conversationIDs []uuid.UUID // conversations the user participates in at connect time
	presence        *PresenceService
	log             *zap.Logger

	// ws config (copied from hub / config at creation time)
	maxMessageSize int64
	pongWait       time.Duration
	pingInterval   time.Duration
	writeWait      time.Duration
}

// NewClient creates a Client and starts its read/write pump goroutines.
func NewClient(
	hub *Hub,
	conn *gorillaws.Conn,
	userID uuid.UUID,
	conversationIDs []uuid.UUID,
	presence *PresenceService,
	log *zap.Logger,
	maxMessageSize int64,
	pongWait, pingInterval, writeWait time.Duration,
	sendBufferSize int,
) *Client {
	c := &Client{
		hub:             hub,
		conn:            conn,
		send:            make(chan *models.WSEvent, sendBufferSize),
		userID:          userID,
		conversationIDs: conversationIDs,
		presence:        presence,
		log:             log,
		maxMessageSize:  maxMessageSize,
		pongWait:        pongWait,
		pingInterval:    pingInterval,
		writeWait:       writeWait,
	}

	hub.Register(c)

	go c.writePump()
	go c.readPump()

	return c
}

// ---------------------------------------------------------------------------
// Read pump — network → application
// ---------------------------------------------------------------------------

// readPump reads WebSocket frames from the network connection. It handles:
//   - typing_start / typing_stop events (forwarded to PresenceService + hub)
//   - mark_read events (forwarded back to the message service via hub)
//   - ping events (responded to with pong)
//
// readPump owns the connection read side and always runs in its own goroutine.
// When the connection closes it unregisters the client from the hub.
func (c *Client) readPump() {
	defer func() {
		c.hub.Unregister(c)
		_ = c.conn.Close()
		if c.presence != nil {
			c.presence.SetOffline(c.userID)
		}
	}()

	c.conn.SetReadLimit(c.maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(c.pongWait))

	// The pong handler resets the read deadline whenever a pong arrives.
	c.conn.SetPongHandler(func(appData string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(c.pongWait))
		return nil
	})

	for {
		_, rawMsg, err := c.conn.ReadMessage()
		if err != nil {
			if gorillaws.IsUnexpectedCloseError(err,
				gorillaws.CloseGoingAway,
				gorillaws.CloseNormalClosure,
				gorillaws.CloseNoStatusReceived,
			) {
				c.log.Warn("unexpected websocket close", zap.String("user_id", c.userID.String()), zap.Error(err))
			}
			return
		}

		var event models.WSEvent
		if err = json.Unmarshal(rawMsg, &event); err != nil {
			c.log.Warn("malformed ws event", zap.Error(err), zap.ByteString("raw", rawMsg))
			continue
		}

		c.handleInboundEvent(&event)
	}
}

// handleInboundEvent dispatches a parsed client-sent event.
func (c *Client) handleInboundEvent(event *models.WSEvent) {
	switch event.Type {
	case models.WSEventTypingStart:
		c.handleTyping(event, true)

	case models.WSEventTypingStop:
		c.handleTyping(event, false)

	case models.WSEventMarkRead:
		c.handleMarkRead(event)

	case models.WSEventPing:
		pong := &models.WSEvent{
			Type:      models.WSEventPong,
			Timestamp: time.Now().UTC(),
		}
		select {
		case c.send <- pong:
		default:
		}

	default:
		c.log.Debug("unknown inbound ws event type", zap.String("type", string(event.Type)))
	}
}

// handleTyping processes typing_start and typing_stop client events.
// It sets/clears the typing indicator in Redis (via PresenceService) and
// broadcasts the event to other participants of the conversation.
func (c *Client) handleTyping(event *models.WSEvent, isTyping bool) {
	if event.ConversationID == "" {
		return
	}
	convID, err := uuid.Parse(event.ConversationID)
	if err != nil {
		return
	}

	if c.presence != nil {
		if isTyping {
			c.presence.SetTyping(c.userID, convID)
		} else {
			c.presence.ClearTyping(c.userID, convID)
		}
	}

	// Fan-out the typing indicator to other participants (excluding self)
	outEvent := &models.WSEvent{
		Type:           event.Type,
		ConversationID: event.ConversationID,
		SenderID:       c.userID.String(),
		Payload: models.TypingPayload{
			UserID:         c.userID.String(),
			ConversationID: event.ConversationID,
		},
		Timestamp: time.Now().UTC(),
	}
	excludeID := c.userID
	c.hub.BroadcastToConversation(convID, outEvent, &excludeID)
}

// handleMarkRead processes a mark_read event coming from the client.
// The actual database update is handled server-side via the REST endpoint;
// here we only broadcast the receipt to other participants.
func (c *Client) handleMarkRead(event *models.WSEvent) {
	if event.ConversationID == "" {
		return
	}
	convID, err := uuid.Parse(event.ConversationID)
	if err != nil {
		return
	}

	outEvent := &models.WSEvent{
		Type:           models.WSEventReadReceipt,
		ConversationID: event.ConversationID,
		SenderID:       c.userID.String(),
		Payload: models.ReadReceiptPayload{
			ConversationID: event.ConversationID,
			LastReadAt:     time.Now().UTC(),
			UserID:         c.userID.String(),
		},
		Timestamp: time.Now().UTC(),
	}
	excludeID := c.userID
	c.hub.BroadcastToConversation(convID, outEvent, &excludeID)
}

// ---------------------------------------------------------------------------
// Write pump — application → network
// ---------------------------------------------------------------------------

// writePump serialises outbound events onto the WebSocket connection and sends
// periodic ping frames to keep the connection alive.
//
// writePump owns the connection write side and always runs in its own goroutine.
// It exits when the send channel is closed (by the hub during unregister).
func (c *Client) writePump() {
	ticker := time.NewTicker(c.pingInterval)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case event, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(c.writeWait))
			if !ok {
				// Hub closed the channel — send a close frame.
				_ = c.conn.WriteMessage(gorillaws.CloseMessage, gorillaws.FormatCloseMessage(gorillaws.CloseNormalClosure, ""))
				return
			}

			data, err := json.Marshal(event)
			if err != nil {
				c.log.Error("failed to marshal ws event", zap.Error(err))
				continue
			}
			if err = c.conn.WriteMessage(gorillaws.TextMessage, data); err != nil {
				c.log.Warn("write message error", zap.String("user_id", c.userID.String()), zap.Error(err))
				return
			}

			// Drain any additional queued events into the same write cycle
			// using NextWriter for efficiency (batching under one frame lock).
			n := len(c.send)
			for i := 0; i < n; i++ {
				next, ok := <-c.send
				if !ok {
					break
				}
				nextData, err := json.Marshal(next)
				if err != nil {
					c.log.Error("failed to marshal queued ws event", zap.Error(err))
					continue
				}
				// Send each as a separate text frame
				_ = c.conn.SetWriteDeadline(time.Now().Add(c.writeWait))
				if err = c.conn.WriteMessage(gorillaws.TextMessage, nextData); err != nil {
					c.log.Warn("write queued message error", zap.Error(err))
					return
				}
			}

		case <-ticker.C:
			// Send a ping frame; pong handler in readPump resets the read deadline.
			_ = c.conn.SetWriteDeadline(time.Now().Add(c.writeWait))
			if err := c.conn.WriteMessage(gorillaws.PingMessage, nil); err != nil {
				c.log.Warn("ping error, closing connection", zap.String("user_id", c.userID.String()), zap.Error(err))
				return
			}
		}
	}
}
