package websocket

import (
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/tiktok-clone/messaging-service/internal/models"
	"github.com/tiktok-clone/messaging-service/internal/repositories"
)

// Hub manages all active WebSocket client connections.
//
// Architecture overview:
//   - Each authenticated user maps to a set of *Client connections (they may
//     have multiple browser tabs / devices open simultaneously).
//   - Messages are dispatched via channels to avoid mutex contention in hot paths.
//   - Register / Unregister are processed serially by the run() goroutine so
//     map mutations are always single-threaded.
type Hub struct {
	// clients maps userID → set of connected clients for that user.
	clients map[uuid.UUID]map[*Client]struct{}
	// conversationClients maps conversationID → set of user IDs currently subscribed.
	conversationClients map[uuid.UUID]map[uuid.UUID]struct{}

	// Inbound channels
	register   chan *Client
	unregister chan *Client
	// broadcast carries a targeted delivery to a specific user.
	broadcast chan userMessage
	// broadcastConv carries a delivery to everyone in a conversation.
	broadcastConv chan convMessage

	mu   sync.RWMutex // protects reads of clients outside the run loop
	repo repositories.MessageRepository
	log  *zap.Logger
}

type userMessage struct {
	userID uuid.UUID
	event  *models.WSEvent
}

type convMessage struct {
	conversationID uuid.UUID
	event          *models.WSEvent
	excludeUserID  *uuid.UUID
}

// NewHub creates and returns a Hub; call Run() to start the event loop.
func NewHub(repo repositories.MessageRepository, log *zap.Logger) *Hub {
	return &Hub{
		clients:             make(map[uuid.UUID]map[*Client]struct{}),
		conversationClients: make(map[uuid.UUID]map[uuid.UUID]struct{}),
		register:            make(chan *Client, 64),
		unregister:          make(chan *Client, 64),
		broadcast:           make(chan userMessage, 512),
		broadcastConv:       make(chan convMessage, 512),
		repo:                repo,
		log:                 log,
	}
}

// Run starts the hub's event loop. It blocks until the provided done channel
// is closed. Call in a separate goroutine.
func (h *Hub) Run(done <-chan struct{}) {
	for {
		select {
		case <-done:
			h.log.Info("websocket hub shutting down")
			h.closeAll()
			return

		case client := <-h.register:
			h.handleRegister(client)

		case client := <-h.unregister:
			h.handleUnregister(client)

		case msg := <-h.broadcast:
			h.deliverToUser(msg.userID, msg.event)

		case msg := <-h.broadcastConv:
			h.deliverToConversation(msg.conversationID, msg.event, msg.excludeUserID)
		}
	}
}

// Register adds a client to the hub. Safe to call from any goroutine.
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// Unregister removes a client from the hub. Safe to call from any goroutine.
func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

// BroadcastToUser queues an event for delivery to all connections of the given user.
// Implements WSBroadcaster.
func (h *Hub) BroadcastToUser(userID uuid.UUID, event *models.WSEvent) {
	h.broadcast <- userMessage{userID: userID, event: event}
}

// BroadcastToConversation queues an event for delivery to all participants of the
// given conversation. If excludeUserID is non-nil that user is skipped.
// Implements WSBroadcaster.
func (h *Hub) BroadcastToConversation(conversationID uuid.UUID, event *models.WSEvent, excludeUserID *uuid.UUID) {
	h.broadcastConv <- convMessage{
		conversationID: conversationID,
		event:          event,
		excludeUserID:  excludeUserID,
	}
}

// OnlineUsers returns the set of user IDs currently connected to the hub.
func (h *Hub) OnlineUsers() []uuid.UUID {
	h.mu.RLock()
	defer h.mu.RUnlock()
	ids := make([]uuid.UUID, 0, len(h.clients))
	for id := range h.clients {
		ids = append(ids, id)
	}
	return ids
}

// IsOnline returns true if the given user has at least one active connection.
func (h *Hub) IsOnline(userID uuid.UUID) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	conns, ok := h.clients[userID]
	return ok && len(conns) > 0
}

// ---------------------------------------------------------------------------
// Internal event-loop handlers (called only from Run())
// ---------------------------------------------------------------------------

func (h *Hub) handleRegister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.clients[client.userID]; !ok {
		h.clients[client.userID] = make(map[*Client]struct{})
	}
	h.clients[client.userID][client] = struct{}{}

	// Subscribe user to all their conversations
	for _, convID := range client.conversationIDs {
		if _, ok := h.conversationClients[convID]; !ok {
			h.conversationClients[convID] = make(map[uuid.UUID]struct{})
		}
		h.conversationClients[convID][client.userID] = struct{}{}
	}

	h.log.Info("client registered",
		zap.String("user_id", client.userID.String()),
		zap.Int("total_user_connections", len(h.clients[client.userID])),
	)

	// Broadcast presence to contacts (best-effort, non-blocking)
	go h.broadcastPresenceChange(client.userID, true)
}

func (h *Hub) handleUnregister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	conns, ok := h.clients[client.userID]
	if !ok {
		return
	}
	delete(conns, client)
	close(client.send)

	if len(conns) == 0 {
		delete(h.clients, client.userID)
		// Remove user from all conversation subscriber maps
		for convID := range h.conversationClients {
			delete(h.conversationClients[convID], client.userID)
		}
		h.log.Info("user fully disconnected", zap.String("user_id", client.userID.String()))
		go h.broadcastPresenceChange(client.userID, false)
	} else {
		h.log.Info("client connection closed (user still has other connections)",
			zap.String("user_id", client.userID.String()),
			zap.Int("remaining", len(conns)),
		)
	}
}

func (h *Hub) deliverToUser(userID uuid.UUID, event *models.WSEvent) {
	h.mu.RLock()
	conns, ok := h.clients[userID]
	h.mu.RUnlock()
	if !ok {
		return
	}
	for client := range conns {
		select {
		case client.send <- event:
		default:
			// Client send buffer full — drop the message and disconnect.
			h.log.Warn("client send buffer full, dropping and unregistering",
				zap.String("user_id", userID.String()),
			)
			h.unregister <- client
		}
	}
}

func (h *Hub) deliverToConversation(conversationID uuid.UUID, event *models.WSEvent, excludeUserID *uuid.UUID) {
	h.mu.RLock()
	userIDs, ok := h.conversationClients[conversationID]
	if !ok {
		h.mu.RUnlock()
		return
	}
	// Copy slice to avoid holding the lock while sending
	targets := make([]uuid.UUID, 0, len(userIDs))
	for uid := range userIDs {
		if excludeUserID != nil && uid == *excludeUserID {
			continue
		}
		targets = append(targets, uid)
	}
	h.mu.RUnlock()

	for _, uid := range targets {
		h.deliverToUser(uid, event)
	}
}

func (h *Hub) broadcastPresenceChange(userID uuid.UUID, online bool) {
	eventType := models.WSEventUserOnline
	if !online {
		eventType = models.WSEventUserOffline
	}
	event := &models.WSEvent{
		Type:      eventType,
		SenderID:  userID.String(),
		Timestamp: time.Now().UTC(),
		Payload:   map[string]interface{}{"user_id": userID.String(), "online": online},
	}

	// Send to all conversations the user participates in (best-effort).
	h.mu.RLock()
	convIDs := make([]uuid.UUID, 0)
	for convID, users := range h.conversationClients {
		if _, exists := users[userID]; exists {
			convIDs = append(convIDs, convID)
		}
	}
	h.mu.RUnlock()

	for _, convID := range convIDs {
		h.deliverToConversation(convID, event, &userID)
	}
}

func (h *Hub) closeAll() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, conns := range h.clients {
		for client := range conns {
			close(client.send)
		}
	}
	h.clients = make(map[uuid.UUID]map[*Client]struct{})
	h.conversationClients = make(map[uuid.UUID]map[uuid.UUID]struct{})
}
