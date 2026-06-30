package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	kafka "github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// giftItem is a single entry in the gift catalog.
type giftItem struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Emoji    string `json:"emoji"`
	Price    int    `json:"price"`
	Category string `json:"category"`
}

// giftCatalog is the static gift catalog served to clients.
var giftCatalog = []giftItem{
	{ID: "rose", Name: "Rose", Emoji: "🌹", Price: 1, Category: "basic"},
	{ID: "heart", Name: "Heart", Emoji: "❤️", Price: 5, Category: "basic"},
	{ID: "thumbsup", Name: "Thumbs Up", Emoji: "👍", Price: 1, Category: "basic"},
	{ID: "star", Name: "Star", Emoji: "⭐", Price: 10, Category: "premium"},
	{ID: "crown", Name: "Crown", Emoji: "👑", Price: 50, Category: "premium"},
	{ID: "diamond", Name: "Diamond", Emoji: "💎", Price: 100, Category: "luxury"},
	{ID: "rocket", Name: "Rocket", Emoji: "🚀", Price: 200, Category: "luxury"},
	{ID: "unicorn", Name: "Unicorn", Emoji: "🦄", Price: 500, Category: "luxury"},
}

// GiftHandler handles gift catalog and send endpoints.
type GiftHandler struct {
	kafkaWriter *kafka.Writer
	logger      *zap.Logger
}

// NewGiftHandler creates a GiftHandler.
func NewGiftHandler(kafkaWriter *kafka.Writer, logger *zap.Logger) *GiftHandler {
	return &GiftHandler{kafkaWriter: kafkaWriter, logger: logger}
}

// RegisterRoutes mounts gift routes on the provided RouterGroup.
func (h *GiftHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("", h.GetGifts)
	rg.POST("/send", h.SendGift)
}

// GetGifts returns the full gift catalog.
func (h *GiftHandler) GetGifts(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"gifts": giftCatalog})
}

// SendGift publishes a gift-sent event and confirms the transaction.
func (h *GiftHandler) SendGift(c *gin.Context) {
	senderID := c.GetString("user_id")

	var req struct {
		GiftID       string `json:"gift_id" binding:"required"`
		TargetUserID string `json:"target_user_id" binding:"required"`
		Count        int    `json:"count" binding:"required,min=1,max=100"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate gift ID.
	var gift *giftItem
	for i := range giftCatalog {
		if giftCatalog[i].ID == req.GiftID {
			gift = &giftCatalog[i]
			break
		}
	}
	if gift == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown gift_id"})
		return
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"sender_id":     senderID,
		"target_user_id": req.TargetUserID,
		"gift_id":       req.GiftID,
		"gift_name":     gift.Name,
		"count":         req.Count,
		"total_cost":    gift.Price * req.Count,
		"sent_at":       time.Now().UTC(),
	})

	if err := h.kafkaWriter.WriteMessages(c.Request.Context(), kafka.Message{
		Topic: "gift.sent",
		Key:   []byte(senderID),
		Value: payload,
		Time:  time.Now(),
	}); err != nil {
		h.logger.Warn("kafka gift.sent publish failed", zap.Error(err))
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"gift_id":    req.GiftID,
		"count":      req.Count,
		"total_cost": gift.Price * req.Count,
	})
}
