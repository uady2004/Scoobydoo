package main

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

var jwtKey = []byte("super-secret-key")

type User struct {
	ID          int    `json:"id"`
	Username    string `json:"username"`
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url"`
	Bio         string `json:"bio"`
	Website     string `json:"website"`
	IsPrivate   bool   `json:"is_private"`
	Followers   int    `json:"followers"`
	Following   int    `json:"following"`
	Likes       int    `json:"likes"`
	IsVerified  bool   `json:"is_verified"`
}

type Video struct {
	ID          int    `json:"id"`
	UserID      int    `json:"user_id"`
	Description string `json:"description"`
	URL         string `json:"url"`
	Likes       int    `json:"likes"`
	Views       int    `json:"views"`
}

var users []User
var videos []Video
var userIDCounter = 1
var videoIDCounter = 1

const usersFile = "users.json"

// ── Persistence ───────────────────────────────────────────────────────────────

func saveUsers() {
	data, err := json.MarshalIndent(users, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(usersFile, data, 0644)
}

func loadUsers() {
	data, err := os.ReadFile(usersFile)
	if err != nil {
		// File doesn't exist yet — seed with a default test user
		users = append(users, User{
			ID:          1,
			Username:    "testuser",
			Email:       "test@test.com",
			Password:    "test123",
			DisplayName: "Test User",
			AvatarURL:   "",
			Bio:         "Test account",
			IsVerified:  false,
		})
		userIDCounter = 2
		saveUsers()
		return
	}
	json.Unmarshal(data, &users)
	// Set counter to max ID + 1
	for _, u := range users {
		if u.ID >= userIDCounter {
			userIDCounter = u.ID + 1
		}
	}
}

// ── Auth helpers ──────────────────────────────────────────────────────────────

func generateToken(userID int) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
	})
	return token.SignedString(jwtKey)
}

func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
			c.Abort()
			return
		}
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return jwtKey, nil
		})
		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			c.Abort()
			return
		}
		claims := token.Claims.(jwt.MapClaims)
		c.Set("user_id", int(claims["user_id"].(float64)))
		c.Next()
	}
}

// ── Response helpers ──────────────────────────────────────────────────────────

func formatProfile(u User) gin.H {
	idStr := strconv.Itoa(u.ID)
	return gin.H{
		"id":              idStr,
		"user_id":         idStr,
		"username":        u.Username,
		"display_name":    u.DisplayName,
		"email":           u.Email,
		"avatar_url":      u.AvatarURL,
		"bio":             u.Bio,
		"website":         u.Website,
		"follower_count":  u.Followers,
		"following_count": u.Following,
		"like_count":      u.Likes,
		"video_count":     0,
		"is_verified":     u.IsVerified,
		"is_creator":      false,
		"is_private":      u.IsPrivate,
		"is_following":    false,
		"created_at":      time.Now().UTC().Format(time.RFC3339),
	}
}

func userResponse(u User, token string) gin.H {
	return gin.H{
		"access_token":  token,
		"refresh_token": token,
		"user": gin.H{
			"id":           strconv.Itoa(u.ID),
			"user_id":      strconv.Itoa(u.ID),
			"username":     u.Username,
			"display_name": u.DisplayName,
			"email":        u.Email,
			"avatar_url":   u.AvatarURL,
			"bio":          u.Bio,
			"is_verified":  u.IsVerified,
			"is_creator":   false,
			"created_at":   time.Now().UTC().Format(time.RFC3339),
		},
	}
}

// ── Auth handlers ─────────────────────────────────────────────────────────────

func register(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
		Phone    string `json:"phone"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	for _, u := range users {
		if u.Email == req.Email {
			c.JSON(409, gin.H{"error": "email already registered"})
			return
		}
	}
	user := User{
		ID:          userIDCounter,
		Username:    req.Username,
		Email:       req.Email,
		Password:    req.Password,
		DisplayName: req.Username,
		AvatarURL:   "",
		Bio:         "",
	}
	userIDCounter++
	users = append(users, user)
	saveUsers() // ← persist to disk
	token, _ := generateToken(user.ID)
	c.JSON(201, userResponse(user, token))
}

func login(c *gin.Context) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	c.ShouldBindJSON(&req)
	for _, user := range users {
		if user.Email == req.Email && user.Password == req.Password {
			token, _ := generateToken(user.ID)
			c.JSON(200, userResponse(user, token))
			return
		}
	}
	c.JSON(401, gin.H{"error": "invalid credentials"})
}

func logout(c *gin.Context) {
	c.JSON(200, gin.H{"message": "logged out"})
}

func refreshToken(c *gin.Context) {
	token, _ := generateToken(1)
	c.JSON(200, gin.H{
		"access_token":  token,
		"refresh_token": token,
	})
}

func forgotPassword(c *gin.Context) {
	c.JSON(200, gin.H{"message": "password reset email sent"})
}

// ── User handlers ─────────────────────────────────────────────────────────────

func getProfile(c *gin.Context) {
	id := c.Param("id")
	if numID, err := strconv.Atoi(id); err == nil {
		for _, user := range users {
			if user.ID == numID {
				c.JSON(200, formatProfile(user))
				return
			}
		}
	}
	for _, user := range users {
		if user.Username == id {
			c.JSON(200, formatProfile(user))
			return
		}
	}
	c.JSON(404, gin.H{"error": "user not found"})
}

func getMe(c *gin.Context) {
	userID, _ := c.Get("user_id")
	for _, user := range users {
		if user.ID == userID.(int) {
			c.JSON(200, formatProfile(user))
			return
		}
	}
	c.JSON(404, gin.H{"error": "user not found"})
}

func updateMe(c *gin.Context) {
	userID, _ := c.Get("user_id")
	uid := userID.(int)
	var req struct {
		DisplayName string `json:"display_name"`
		Username    string `json:"username"`
		Bio         string `json:"bio"`
		Website     string `json:"website"`
		IsPrivate   bool   `json:"is_private"`
	}
	c.ShouldBindJSON(&req)

	for i, user := range users {
		if user.ID == uid {
			if req.DisplayName != "" {
				users[i].DisplayName = req.DisplayName
			}
			if req.Username != "" {
				users[i].Username = req.Username
			}
			users[i].Bio = req.Bio
			users[i].Website = req.Website
			users[i].IsPrivate = req.IsPrivate
			saveUsers()
			c.JSON(200, formatProfile(users[i]))
			return
		}
	}

	// User not in memory (e.g. Render restarted and wiped users.json) — upsert.
	username := req.Username
	if username == "" {
		username = "user" + strconv.Itoa(uid)
	}
	name := req.DisplayName
	if name == "" {
		name = username
	}
	newUser := User{
		ID:          uid,
		Username:    username,
		DisplayName: name,
		Bio:         req.Bio,
		Website:     req.Website,
		IsPrivate:   req.IsPrivate,
	}
	users = append(users, newUser)
	if uid >= userIDCounter {
		userIDCounter = uid + 1
	}
	saveUsers()
	c.JSON(200, formatProfile(newUser))
}

func followUser(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	for i := range users {
		if users[i].ID == id {
			users[i].Followers++
			saveUsers()
			c.JSON(200, gin.H{"message": "followed"})
			return
		}
	}
	c.JSON(404, gin.H{"error": "user not found"})
}

func unfollowUser(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	for i := range users {
		if users[i].ID == id {
			if users[i].Followers > 0 {
				users[i].Followers--
			}
			saveUsers()
			c.JSON(200, gin.H{"message": "unfollowed"})
			return
		}
	}
	c.JSON(404, gin.H{"error": "user not found"})
}

// ── Video handlers ────────────────────────────────────────────────────────────

func uploadVideo(c *gin.Context) {
	var req Video
	c.ShouldBindJSON(&req)
	req.ID = videoIDCounter
	videoIDCounter++
	videos = append(videos, req)
	c.JSON(201, req)
}

func getFeed(c *gin.Context) {
	c.JSON(200, gin.H{"data": videos, "next_cursor": nil})
}

func likeVideo(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	for i := range videos {
		if videos[i].ID == id {
			videos[i].Likes++
			c.JSON(200, gin.H{"is_liked": true, "like_count": videos[i].Likes})
			return
		}
	}
	c.JSON(404, gin.H{"error": "video not found"})
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	loadUsers() // ← load saved users on startup

	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "Accept"},
		AllowCredentials: false,
	}))

	// Health
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Public auth routes
	r.POST("/register", register)
	r.POST("/login", login)
	r.POST("/auth/register", register)
	r.POST("/auth/login", login)
	r.POST("/api/v1/auth/register", register)
	r.POST("/api/v1/auth/login", login)
	r.POST("/api/v1/auth/logout", logout)
	r.POST("/api/v1/auth/refresh", refreshToken)
	r.POST("/auth/logout", logout)
	r.POST("/auth/refresh", refreshToken)
	r.POST("/auth/forgot-password", forgotPassword)
	r.POST("/auth/otp/send", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "OTP sent"})
	})
	r.POST("/auth/otp/verify", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "OTP verified"})
	})
	r.POST("/auth/verify-email", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "email verified"})
	})

	// Authenticated routes
	auth := r.Group("/")
	authV1 := r.Group("/api/v1")
	authV1.Use(authMiddleware())
	authV1.GET("/users/me", getMe)
	authV1.PUT("/users/me", updateMe)
	authV1.PATCH("/users/me", updateMe)
	authV1.POST("/users/me/avatar", func(c *gin.Context) {
		c.JSON(200, gin.H{"avatar_url": ""})
	})
	authV1.GET("/users/check-username", func(c *gin.Context) {
		username := c.Query("username")
		taken := false
		for _, u := range users {
			if strings.EqualFold(u.Username, username) {
				taken = true
				break
			}
		}
		c.JSON(200, gin.H{"available": !taken})
	})
	authV1.GET("/users/:id", getProfile)
	authV1.GET("/users/:id/profile", getProfile)
	authV1.POST("/users/:id/follow", followUser)
	authV1.DELETE("/users/:id/follow", unfollowUser)
	authV1.GET("/users/:id/followers", func(c *gin.Context) {
		c.JSON(200, gin.H{"data": []interface{}{}, "next_cursor": nil})
	})
	authV1.GET("/users/:id/following", func(c *gin.Context) {
		c.JSON(200, gin.H{"data": []interface{}{}, "next_cursor": nil})
	})
	authV1.GET("/feed", getFeed)
	authV1.GET("/feed/for-you", getFeed)
	authV1.POST("/feed/view", func(c *gin.Context) { c.JSON(200, gin.H{"message": "ok"}) })
	authV1.POST("/videos/:id/like", likeVideo)
	authV1.GET("/videos/:id/like-status", func(c *gin.Context) { c.JSON(200, gin.H{"is_liked": false}) })
	authV1.POST("/videos/:id/bookmark", func(c *gin.Context) { c.JSON(200, gin.H{"is_bookmarked": true}) })
	authV1.POST("/videos/:id/view", func(c *gin.Context) { c.JSON(200, gin.H{"message": "ok"}) })
	authV1.GET("/videos/:id/comments", func(c *gin.Context) { c.JSON(200, gin.H{"data": []interface{}{}, "next_cursor": nil}) })
	authV1.POST("/videos/:id/comments", func(c *gin.Context) {
		c.JSON(201, gin.H{"id": "c1", "content": "", "like_count": 0, "is_liked": false})
	})
	authV1.POST("/comments/:id/like", func(c *gin.Context) { c.JSON(200, gin.H{"is_liked": true}) })
	authV1.GET("/notifications", func(c *gin.Context) { c.JSON(200, gin.H{"data": []interface{}{}, "next_cursor": nil}) })
	authV1.GET("/notifications/unread-count", func(c *gin.Context) { c.JSON(200, gin.H{"count": 0}) })
	authV1.PUT("/notifications/:id/read", func(c *gin.Context) { c.JSON(200, gin.H{"message": "ok"}) })
	authV1.PUT("/notifications/read-all", func(c *gin.Context) { c.JSON(200, gin.H{"message": "ok"}) })
	authV1.GET("/search", func(c *gin.Context) { c.JSON(200, gin.H{"data": []interface{}{}}) })
	authV1.GET("/search/trending", func(c *gin.Context) { c.JSON(200, gin.H{"data": []string{"fyp", "trending"}}) })
	authV1.GET("/search/suggestions", func(c *gin.Context) { c.JSON(200, gin.H{"data": []interface{}{}}) })
	authV1.GET("/search/history", func(c *gin.Context) { c.JSON(200, gin.H{"data": []interface{}{}}) })
	authV1.GET("/wallet/balance", func(c *gin.Context) { c.JSON(200, gin.H{"balance": 0, "coins": 0}) })
	authV1.GET("/wallet/packages", func(c *gin.Context) { c.JSON(200, gin.H{"data": []interface{}{}}) })
	authV1.GET("/conversations", func(c *gin.Context) { c.JSON(200, gin.H{"data": []interface{}{}}) })
	authV1.GET("/sounds/trending", func(c *gin.Context) { c.JSON(200, gin.H{"data": []interface{}{}}) })
	authV1.GET("/hashtags/trending", func(c *gin.Context) { c.JSON(200, gin.H{"data": []interface{}{}}) })
	authV1.POST("/reports", func(c *gin.Context) { c.JSON(201, gin.H{"message": "reported"}) })
	authV1.POST("/devices", func(c *gin.Context) { c.JSON(200, gin.H{"message": "ok"}) })
	authV1.DELETE("/devices/:token", func(c *gin.Context) { c.JSON(200, gin.H{"message": "ok"}) })
	auth.Use(authMiddleware())

	// Users
	auth.GET("/users/me", getMe)
	auth.PUT("/users/me", updateMe)
	auth.PATCH("/users/me", updateMe)
	auth.POST("/users/me/avatar", func(c *gin.Context) {
		c.JSON(200, gin.H{"avatar_url": ""})
	})
	auth.GET("/users/check-username", func(c *gin.Context) {
		username := c.Query("username")
		taken := false
		for _, u := range users {
			if strings.EqualFold(u.Username, username) {
				taken = true
				break
			}
		}
		c.JSON(200, gin.H{"available": !taken})
	})
	auth.GET("/users/:id", getProfile)
	auth.GET("/users/:id/profile", getProfile)
	auth.POST("/users/:id/follow", followUser)
	auth.DELETE("/users/:id/follow", unfollowUser)
	auth.GET("/users/:id/followers", func(c *gin.Context) {
		c.JSON(200, gin.H{"data": []interface{}{}, "next_cursor": nil})
	})
	auth.GET("/users/:id/following", func(c *gin.Context) {
		c.JSON(200, gin.H{"data": []interface{}{}, "next_cursor": nil})
	})
	auth.GET("/users/:id/videos", func(c *gin.Context) {
		c.JSON(200, gin.H{"data": []interface{}{}, "next_cursor": nil})
	})

	// Feed
	auth.GET("/feed", getFeed)
	auth.GET("/feed/for-you", getFeed)
	auth.GET("/feed/following", func(c *gin.Context) {
		c.JSON(200, gin.H{"data": []interface{}{}, "next_cursor": nil})
	})
	auth.POST("/feed/view", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "ok"})
	})

	// Videos
	auth.POST("/videos", uploadVideo)
	auth.POST("/api/videos", uploadVideo)
	auth.GET("/videos/:id", func(c *gin.Context) {
		c.JSON(200, gin.H{"id": c.Param("id")})
	})
	auth.POST("/videos/:id/like", likeVideo)
	auth.GET("/videos/:id/like-status", func(c *gin.Context) {
		c.JSON(200, gin.H{"is_liked": false})
	})
	auth.POST("/videos/:id/bookmark", func(c *gin.Context) {
		c.JSON(200, gin.H{"is_bookmarked": true})
	})
	auth.POST("/videos/:id/view", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "ok"})
	})
	auth.GET("/videos/:id/comments", func(c *gin.Context) {
		c.JSON(200, gin.H{"data": []interface{}{}, "next_cursor": nil})
	})
	auth.POST("/videos/:id/comments", func(c *gin.Context) {
		var req struct {
			Content string `json:"content"`
		}
		c.ShouldBindJSON(&req)
		c.JSON(201, gin.H{
			"id":         "comment_1",
			"content":    req.Content,
			"like_count": 0,
			"is_liked":   false,
			"created_at": time.Now().UTC().Format(time.RFC3339),
		})
	})

	// Liked / bookmarks
	auth.GET("/me/liked-videos", func(c *gin.Context) {
		c.JSON(200, gin.H{"data": []interface{}{}, "next_cursor": nil})
	})
	auth.GET("/me/bookmarks", func(c *gin.Context) {
		c.JSON(200, gin.H{"data": []interface{}{}, "next_cursor": nil})
	})

	// Comments
	auth.POST("/comments/:id/like", func(c *gin.Context) {
		c.JSON(200, gin.H{"is_liked": true})
	})
	auth.POST("/comments/:id/pin", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "pinned"})
	})
	auth.DELETE("/comments/:id", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "deleted"})
	})

	// Notifications
	auth.GET("/notifications", func(c *gin.Context) {
		c.JSON(200, gin.H{"data": []interface{}{}, "next_cursor": nil})
	})
	auth.PUT("/notifications/:id/read", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "read"})
	})
	auth.PUT("/notifications/read-all", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "all read"})
	})
	auth.GET("/notifications/unread-count", func(c *gin.Context) {
		c.JSON(200, gin.H{"count": 0})
	})

	// Search
	auth.GET("/search", func(c *gin.Context) {
		c.JSON(200, gin.H{"data": []interface{}{}, "next_cursor": nil})
	})
	auth.GET("/search/trending", func(c *gin.Context) {
		c.JSON(200, gin.H{"data": []string{"fyp", "trending", "viral"}})
	})
	auth.GET("/search/suggestions", func(c *gin.Context) {
		c.JSON(200, gin.H{"data": []interface{}{}})
	})
	auth.GET("/search/history", func(c *gin.Context) {
		c.JSON(200, gin.H{"data": []interface{}{}})
	})
	auth.POST("/search/history", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "saved"})
	})
	auth.DELETE("/search/history", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "cleared"})
	})

	// Sounds
	auth.GET("/sounds/trending", func(c *gin.Context) {
		c.JSON(200, gin.H{"data": []interface{}{}})
	})
	auth.GET("/sounds/search", func(c *gin.Context) {
		c.JSON(200, gin.H{"data": []interface{}{}})
	})
	auth.GET("/sounds/:id", func(c *gin.Context) {
		c.JSON(200, gin.H{"id": c.Param("id")})
	})

	// Hashtags
	auth.GET("/hashtags/trending", func(c *gin.Context) {
		c.JSON(200, gin.H{"data": []interface{}{}})
	})
	auth.GET("/hashtags/:tag", func(c *gin.Context) {
		c.JSON(200, gin.H{"tag": c.Param("tag"), "video_count": 0})
	})

	// Wallet
	auth.GET("/wallet/balance", func(c *gin.Context) {
		c.JSON(200, gin.H{"balance": 0, "coins": 0})
	})
	auth.GET("/wallet/packages", func(c *gin.Context) {
		c.JSON(200, gin.H{"data": []gin.H{
			{"id": "1", "coins": 70, "price": 1.0, "label": "$1"},
			{"id": "2", "coins": 420, "price": 6.0, "label": "$6"},
			{"id": "3", "coins": 2100, "price": 30.0, "label": "$30"},
		}})
	})
	auth.GET("/wallet/transactions", func(c *gin.Context) {
		c.JSON(200, gin.H{"data": []interface{}{}, "next_cursor": nil})
	})
	auth.POST("/wallet/coins/buy", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "purchase initiated"})
	})
	auth.POST("/wallet/coins/confirm", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "purchase confirmed"})
	})
	auth.POST("/wallet/withdraw", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "withdrawal requested"})
	})

	// Messaging
	auth.GET("/conversations", func(c *gin.Context) {
		c.JSON(200, gin.H{"data": []interface{}{}, "next_cursor": nil})
	})
	auth.POST("/conversations", func(c *gin.Context) {
		c.JSON(201, gin.H{"id": "conv_1", "participants": []interface{}{}})
	})
	auth.GET("/conversations/:id/messages", func(c *gin.Context) {
		c.JSON(200, gin.H{"data": []interface{}{}, "next_cursor": nil})
	})
	auth.PUT("/conversations/:id/read", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "read"})
	})

	// Reports
	auth.POST("/reports", func(c *gin.Context) {
		c.JSON(201, gin.H{"message": "report submitted"})
	})

	// Gifts / Livestream
	auth.GET("/gifts", func(c *gin.Context) {
		c.JSON(200, gin.H{"data": []interface{}{}})
	})
	auth.POST("/gifts/send", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "gift sent"})
	})
	auth.GET("/live/:id", func(c *gin.Context) {
		c.JSON(200, gin.H{"id": c.Param("id"), "status": "live"})
	})

	// Analytics
	auth.GET("/analytics/profile/stats", func(c *gin.Context) {
		c.JSON(200, gin.H{"views": 0, "likes": 0, "followers": 0})
	})

	// Devices
	auth.POST("/devices", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "device registered"})
	})
	auth.DELETE("/devices/:token", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "device removed"})
	})

	// Old compat routes
	auth.GET("/api/profile/:id", getProfile)
	auth.GET("/api/feed", getFeed)
	auth.POST("/api/videos/:id/like", likeVideo)
	auth.POST("/api/users/:id/follow", followUser)

	r.Run(":8080")
}
