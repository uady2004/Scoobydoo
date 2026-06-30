package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/tiktok-clone/livestream-service/internal/models"
	"github.com/tiktok-clone/livestream-service/internal/services"
)

// ─────────────────────────────────────────────────────────────────────────────
// WebSocket tuning constants
// ─────────────────────────────────────────────────────────────────────────────

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 8192
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// ─────────────────────────────────────────────────────────────────────────────
// Hub — manages all active stream rooms
// ─────────────────────────────────────────────────────────────────────────────

// Hub is the global WebSocket hub. One Hub per service instance.
type Hub struct {
	mu        sync.RWMutex
	rooms     map[string]*Room // streamID → Room
	logger    *zap.Logger
	streamSvc services.StreamService
}

// NewHub creates a Hub and starts its internal goroutines.
func NewHub(streamSvc services.StreamService, logger *zap.Logger) *Hub {
	return &Hub{
		rooms:     make(map[string]*Room),
		logger:    logger,
		streamSvc: streamSvc,
	}
}

// Room holds all connected clients for a single stream.
type Room struct {
	mu      sync.RWMutex
	clients map[*client]bool
}

func newRoom() *Room {
	return &Room{clients: make(map[*client]bool)}
}

// ─────────────────────────────────────────────────────────────────────────────
// Client
// ─────────────────────────────────────────────────────────────────────────────

type client struct {
	hub        *Hub
	conn       *websocket.Conn
	streamID   string
	userID     string
	username   string
	avatarURL  string
	isHost     bool
	send       chan []byte
}

// ─────────────────────────────────────────────────────────────────────────────
// Hub public API
// ─────────────────────────────────────────────────────────────────────────────

// ServeWS is the Gin handler that upgrades HTTP → WebSocket.
// Route: GET /ws/:streamId
func (h *Hub) ServeWS(c *gin.Context) {
	streamID := c.Param("streamId")
	if streamID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "stream_id required"})
		return
	}

	// Extract viewer identity from query params (token validated upstream by middleware).
	userID := c.Query("user_id")
	username := c.Query("username")
	avatarURL := c.Query("avatar_url")
	isHost := c.Query("role") == "host"

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Warn("websocket upgrade failed",
			zap.String("stream_id", streamID),
			zap.Error(err),
		)
		return
	}

	cl := &client{
		hub:       h,
		conn:      conn,
		streamID:  streamID,
		userID:    userID,
		username:  username,
		avatarURL: avatarURL,
		isHost:    isHost,
		send:      make(chan []byte, 512),
	}

	h.register(cl)

	go cl.writePump()
	go cl.readPump()
}

// BroadcastToRoom marshals event and sends it to every client in the room.
func (h *Hub) BroadcastToRoom(streamID string, event models.WSEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		h.logger.Error("marshal ws event failed", zap.Error(err))
		return
	}
	h.broadcastRaw(streamID, data)
}

// ViewerCount returns the live WebSocket-connected viewer count for a stream.
func (h *Hub) ViewerCount(streamID string) int {
	h.mu.RLock()
	room, ok := h.rooms[streamID]
	h.mu.RUnlock()
	if !ok {
		return 0
	}
	room.mu.RLock()
	defer room.mu.RUnlock()
	return len(room.clients)
}

// EndStream broadcasts stream.end to all clients in the room then removes the room.
func (h *Hub) EndStream(streamID string) {
	h.BroadcastToRoom(streamID, models.WSEvent{
		Type:      models.WSEventStreamEnd,
		StreamID:  streamID,
		Timestamp: time.Now().UTC(),
		Payload:   map[string]string{"message": "Stream has ended"},
	})

	h.mu.Lock()
	delete(h.rooms, streamID)
	h.mu.Unlock()
}

// ─────────────────────────────────────────────────────────────────────────────
// private helpers
// ─────────────────────────────────────────────────────────────────────────────

func (h *Hub) register(cl *client) {
	h.mu.Lock()
	room, ok := h.rooms[cl.streamID]
	if !ok {
		room = newRoom()
		h.rooms[cl.streamID] = room
	}
	h.mu.Unlock()

	room.mu.Lock()
	room.clients[cl] = true
	count := len(room.clients)
	room.mu.Unlock()

	h.logger.Info("client joined stream",
		zap.String("stream_id", cl.streamID),
		zap.String("user_id", cl.userID),
		zap.Int("viewers", count),
	)

	// Notify all others that a viewer joined.
	h.BroadcastToRoom(cl.streamID, models.WSEvent{
		Type:      models.WSEventViewerJoin,
		StreamID:  cl.streamID,
		Timestamp: time.Now().UTC(),
		Payload: map[string]interface{}{
			"user_id":    cl.userID,
			"username":   cl.username,
			"avatar_url": cl.avatarURL,
		},
	})

	// Broadcast updated viewer count.
	h.broadcastViewerCount(cl.streamID)

	// Record join in the stream service.
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = h.streamSvc.JoinStream(ctx, services.JoinStreamRequest{
			StreamID:  cl.streamID,
			UserID:    cl.userID,
			Username:  cl.username,
			AvatarURL: cl.avatarURL,
		})
	}()
}

func (h *Hub) unregister(cl *client) {
	h.mu.RLock()
	room, ok := h.rooms[cl.streamID]
	h.mu.RUnlock()
	if !ok {
		return
	}

	room.mu.Lock()
	if _, exists := room.clients[cl]; exists {
		delete(room.clients, cl)
		close(cl.send)
	}
	count := len(room.clients)
	isEmpty := count == 0
	room.mu.Unlock()

	if isEmpty {
		h.mu.Lock()
		// Double-check; another goroutine might have re-added a client.
		if r2, ok2 := h.rooms[cl.streamID]; ok2 {
			r2.mu.RLock()
			stillEmpty := len(r2.clients) == 0
			r2.mu.RUnlock()
			if stillEmpty {
				delete(h.rooms, cl.streamID)
			}
		}
		h.mu.Unlock()
	}

	h.logger.Info("client left stream",
		zap.String("stream_id", cl.streamID),
		zap.String("user_id", cl.userID),
		zap.Int("viewers", count),
	)

	h.BroadcastToRoom(cl.streamID, models.WSEvent{
		Type:      models.WSEventViewerLeave,
		StreamID:  cl.streamID,
		Timestamp: time.Now().UTC(),
		Payload: map[string]interface{}{
			"user_id":  cl.userID,
			"username": cl.username,
		},
	})
	h.broadcastViewerCount(cl.streamID)

	// Record leave in the stream service.
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = h.streamSvc.LeaveStream(ctx, cl.streamID, cl.userID)
	}()
}

func (h *Hub) broadcastViewerCount(streamID string) {
	count := int64(h.ViewerCount(streamID))
	h.BroadcastToRoom(streamID, models.WSEvent{
		Type:      models.WSEventViewerCount,
		StreamID:  streamID,
		Timestamp: time.Now().UTC(),
		Payload: models.ViewerCountPayload{
			Current: count,
			Peak:    count,
		},
	})
}

func (h *Hub) broadcastRaw(streamID string, data []byte) {
	h.mu.RLock()
	room, ok := h.rooms[streamID]
	h.mu.RUnlock()
	if !ok {
		return
	}

	room.mu.RLock()
	defer room.mu.RUnlock()
	for cl := range room.clients {
		select {
		case cl.send <- data:
		default:
			// Slow consumer — drop and close.
			close(cl.send)
			delete(room.clients, cl)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Client pumps
// ─────────────────────────────────────────────────────────────────────────────

func (cl *client) readPump() {
	defer func() {
		cl.hub.unregister(cl)
		cl.conn.Close()
	}()

	cl.conn.SetReadLimit(maxMessageSize)
	_ = cl.conn.SetReadDeadline(time.Now().Add(pongWait))
	cl.conn.SetPongHandler(func(string) error {
		return cl.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, raw, err := cl.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseGoingAway,
				websocket.CloseAbnormalClosure,
			) {
				cl.hub.logger.Warn("ws read error",
					zap.String("user_id", cl.userID),
					zap.Error(err),
				)
			}
			break
		}

		cl.handleClientMessage(raw)
	}
}

func (cl *client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		cl.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-cl.send:
			_ = cl.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = cl.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			w, err := cl.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			_, _ = w.Write(msg)

			// Drain buffered messages into the same frame.
			n := len(cl.send)
			for i := 0; i < n; i++ {
				_, _ = w.Write([]byte{'\n'})
				_, _ = w.Write(<-cl.send)
			}
			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			_ = cl.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := cl.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Incoming message dispatcher
// ─────────────────────────────────────────────────────────────────────────────

// incomingMsg is the generic envelope for messages sent by a client.
type incomingMsg struct {
	Type    string                 `json:"type"`
	Payload map[string]interface{} `json:"payload"`
}

func (cl *client) handleClientMessage(raw []byte) {
	var msg incomingMsg
	if err := json.Unmarshal(raw, &msg); err != nil {
		return
	}

	switch msg.Type {
	case "chat.send":
		cl.handleChatSend(msg.Payload)
	case "gift.send":
		cl.handleGiftSend(msg.Payload)
	case "poll.vote":
		cl.handlePollVote(msg.Payload)
	case "pk.invite":
		if cl.isHost {
			cl.hub.BroadcastToRoom(cl.streamID, models.WSEvent{
				Type:      models.WSEventPKBattleInvite,
				StreamID:  cl.streamID,
				Timestamp: time.Now().UTC(),
				Payload:   msg.Payload,
			})
		}
	case "cohost.invite":
		if cl.isHost {
			cl.hub.BroadcastToRoom(cl.streamID, models.WSEvent{
				Type:      models.WSEventCoHostInvite,
				StreamID:  cl.streamID,
				Timestamp: time.Now().UTC(),
				Payload:   msg.Payload,
			})
		}
	}
}

func (cl *client) handleChatSend(payload map[string]interface{}) {
	content, _ := payload["content"].(string)
	content = strings.TrimSpace(content)
	if content == "" || len(content) > 500 {
		return
	}

	cl.hub.BroadcastToRoom(cl.streamID, models.WSEvent{
		Type:      models.WSEventChatMessage,
		StreamID:  cl.streamID,
		Timestamp: time.Now().UTC(),
		Payload: map[string]interface{}{
			"user_id":    cl.userID,
			"username":   cl.username,
			"avatar_url": cl.avatarURL,
			"content":    content,
			"type":       "text",
			"created_at": time.Now().UTC(),
		},
	})
}

func (cl *client) handleGiftSend(payload map[string]interface{}) {
	cl.hub.BroadcastToRoom(cl.streamID, models.WSEvent{
		Type:      models.WSEventGiftAnimation,
		StreamID:  cl.streamID,
		Timestamp: time.Now().UTC(),
		Payload:   payload,
	})
}

func (cl *client) handlePollVote(payload map[string]interface{}) {
	cl.hub.BroadcastToRoom(cl.streamID, models.WSEvent{
		Type:      models.WSEventPollVote,
		StreamID:  cl.streamID,
		Timestamp: time.Now().UTC(),
		Payload:   payload,
	})
}
