package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	gorillaws "github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/tiktok-clone/messaging-service/internal/config"
	"github.com/tiktok-clone/messaging-service/internal/repositories"
	ws "github.com/tiktok-clone/messaging-service/internal/websocket"
)

// WSHandler upgrades HTTP connections to WebSocket and hands them off to the hub.
type WSHandler struct {
	hub      *ws.Hub
	presence *ws.PresenceService
	repo     repositories.MessageRepository
	cfg      *config.Config
	upgrader gorillaws.Upgrader
	log      *zap.Logger
}

// NewWSHandler creates a WSHandler. The cfg.WebSocket.AllowedOrigins field is a
// comma-separated list of permitted origins (use "*" to allow all).
func NewWSHandler(
	hub *ws.Hub,
	presence *ws.PresenceService,
	repo repositories.MessageRepository,
	cfg *config.Config,
	log *zap.Logger,
) *WSHandler {
	allowedOrigins := parseOrigins(cfg.WebSocket.AllowedOrigins)

	upgrader := gorillaws.Upgrader{
		HandshakeTimeout: 10 * time.Second,
		ReadBufferSize:   4096,
		WriteBufferSize:  4096,
		CheckOrigin: func(r *http.Request) bool {
			if len(allowedOrigins) == 0 {
				return true
			}
			origin := r.Header.Get("Origin")
			for _, allowed := range allowedOrigins {
				if allowed == "*" || allowed == origin {
					return true
				}
			}
			return false
		},
	}

	return &WSHandler{
		hub:      hub,
		presence: presence,
		repo:     repo,
		cfg:      cfg,
		upgrader: upgrader,
		log:      log,
	}
}

// ServeWS godoc
// GET /ws
//
// Upgrades the connection to WebSocket. The caller must be authenticated; the
// JWT middleware must have set "user_id" in the Gin context before this handler
// is reached.
//
// On connection the handler:
//  1. Resolves all conversation IDs the user participates in (for fan-out routing).
//  2. Marks the user as online in the Presence service.
//  3. Creates a Client and registers it with the Hub.
func (h *WSHandler) ServeWS(c *gin.Context) {
	userID, ok := callerID(c)
	if !ok {
		// callerID already wrote the error response.
		return
	}

	// Upgrade HTTP → WebSocket.
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.log.Warn("websocket upgrade failed",
			zap.String("user_id", userID.String()),
			zap.Error(err))
		// upgrader already wrote an HTTP error response.
		return
	}

	// Resolve conversation memberships so the hub can route fan-out correctly.
	ctx := c.Request.Context()
	convIDs, err := h.resolveConversationIDs(ctx, userID)
	if err != nil {
		h.log.Error("failed to resolve conversation IDs",
			zap.String("user_id", userID.String()),
			zap.Error(err))
		_ = conn.Close()
		return
	}

	// Mark the user as online in Redis.
	h.presence.SetOnline(userID)

	wsCfg := h.cfg.WebSocket
	ws.NewClient(
		h.hub,
		conn,
		userID,
		convIDs,
		h.presence,
		h.log,
		wsCfg.MaxMessageSize,
		wsCfg.PongWait,
		wsCfg.PingInterval,
		wsCfg.WriteWait,
		wsCfg.SendBufferSize,
	)

	h.log.Info("websocket client connected",
		zap.String("user_id", userID.String()),
		zap.Int("conversation_count", len(convIDs)),
	)
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// resolveConversationIDs fetches the first page of conversation IDs the user
// belongs to. This pre-populates the client's routing subscription at connect time.
func (h *WSHandler) resolveConversationIDs(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	page, err := h.repo.GetConversations(ctx, userID, "", 50)
	if err != nil {
		return nil, err
	}

	ids := make([]uuid.UUID, 0, len(page.Conversations))
	for _, conv := range page.Conversations {
		ids = append(ids, conv.ID)
	}
	return ids, nil
}

// parseOrigins splits a comma-separated origin allowlist and trims whitespace.
func parseOrigins(raw string) []string {
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
