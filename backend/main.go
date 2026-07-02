package main

import (
	"database/sql"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

// ── Globals ───────────────────────────────────────────────────────────────────

var db *sql.DB
var jwtKey = []byte(getEnv("JWT_SECRET", "super-secret-key"))

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// ── Structs ───────────────────────────────────────────────────────────────────

type User struct {
	ID          int
	Username    string
	Email       string
	Password    string
	DisplayName string
	AvatarURL   string
	Bio         string
	Website     string
	IsPrivate   bool
	Followers   int
	Following   int
	Likes       int
	IsVerified  bool
	CreatedAt   time.Time
}

type Video struct {
	ID            int
	UserID        int
	Description   string
	URL           string
	ThumbnailURL  string
	Likes         int
	Views         int
	Comments      int
	IsPublic      bool
	AllowComments bool
	AllowDuet     bool
	AllowStitch   bool
	CreatedAt     time.Time
}

type Comment struct {
	ID        int
	VideoID   int
	UserID    int
	ParentID  sql.NullInt64
	Content   string
	LikeCount int
	IsPinned  bool
	IsDeleted bool
	CreatedAt time.Time
}

// ── Database setup ────────────────────────────────────────────────────────────

func initDB() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}
	var err error
	db, err = sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	if err = db.Ping(); err != nil {
		log.Fatalf("Database ping failed: %v", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	createTables()
	log.Println("PostgreSQL connected — tables ready")
}

func createTables() {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id           SERIAL PRIMARY KEY,
			username     VARCHAR(50)  UNIQUE NOT NULL,
			email        VARCHAR(255) UNIQUE NOT NULL,
			password     VARCHAR(255) NOT NULL,
			display_name VARCHAR(100) DEFAULT '',
			avatar_url   TEXT         DEFAULT '',
			bio          TEXT         DEFAULT '',
			website      VARCHAR(255) DEFAULT '',
			is_private   BOOLEAN      DEFAULT FALSE,
			followers    INTEGER      DEFAULT 0,
			following    INTEGER      DEFAULT 0,
			likes        INTEGER      DEFAULT 0,
			is_verified  BOOLEAN      DEFAULT FALSE,
			created_at   TIMESTAMPTZ  DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS videos (
			id             SERIAL PRIMARY KEY,
			user_id        INTEGER REFERENCES users(id) ON DELETE CASCADE,
			description    TEXT    DEFAULT '',
			url            TEXT    DEFAULT '',
			thumbnail_url  TEXT    DEFAULT '',
			likes          INTEGER DEFAULT 0,
			views          INTEGER DEFAULT 0,
			comments       INTEGER DEFAULT 0,
			is_public      BOOLEAN DEFAULT TRUE,
			allow_comments BOOLEAN DEFAULT TRUE,
			allow_duet     BOOLEAN DEFAULT TRUE,
			allow_stitch   BOOLEAN DEFAULT TRUE,
			created_at     TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS follows (
			follower_id  INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			following_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			created_at   TIMESTAMPTZ DEFAULT NOW(),
			PRIMARY KEY (follower_id, following_id)
		)`,
		`CREATE TABLE IF NOT EXISTS likes (
			user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			video_id   INTEGER NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			PRIMARY KEY (user_id, video_id)
		)`,
		`CREATE TABLE IF NOT EXISTS comments (
			id         SERIAL PRIMARY KEY,
			video_id   INTEGER NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
			user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			parent_id  INTEGER REFERENCES comments(id) ON DELETE CASCADE,
			content    TEXT NOT NULL,
			like_count INTEGER DEFAULT 0,
			is_pinned  BOOLEAN DEFAULT FALSE,
			is_deleted BOOLEAN DEFAULT FALSE,
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS comment_likes (
			user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			comment_id INTEGER NOT NULL REFERENCES comments(id) ON DELETE CASCADE,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			PRIMARY KEY (user_id, comment_id)
		)`,
		`CREATE TABLE IF NOT EXISTS bookmarks (
			user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			video_id   INTEGER NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			PRIMARY KEY (user_id, video_id)
		)`,
		`CREATE TABLE IF NOT EXISTS notifications (
			id          SERIAL PRIMARY KEY,
			user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			type        TEXT NOT NULL,
			title       TEXT DEFAULT '',
			body        TEXT DEFAULT '',
			actor_id    INTEGER REFERENCES users(id) ON DELETE SET NULL,
			entity_id   INTEGER,
			entity_type TEXT,
			is_read     BOOLEAN DEFAULT FALSE,
			read_at     TIMESTAMPTZ,
			created_at  TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS conversations (
			id         SERIAL PRIMARY KEY,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS conversation_members (
			conversation_id INTEGER NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
			user_id         INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			last_read_at    TIMESTAMPTZ,
			PRIMARY KEY (conversation_id, user_id)
		)`,
		`CREATE TABLE IF NOT EXISTS messages (
			id              SERIAL PRIMARY KEY,
			conversation_id INTEGER NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
			sender_id       INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			content         TEXT NOT NULL DEFAULT '',
			type            TEXT NOT NULL DEFAULT 'text',
			is_deleted      BOOLEAN DEFAULT FALSE,
			created_at      TIMESTAMPTZ DEFAULT NOW()
		)`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			log.Fatalf("createTables: %v", err)
		}
	}
}

// ── DB helpers ────────────────────────────────────────────────────────────────

const userSelectCols = `id, username, email, password, display_name, avatar_url,
	bio, website, is_private, followers, following, likes, is_verified, created_at`

func scanUser(row *sql.Row) (User, error) {
	var u User
	err := row.Scan(
		&u.ID, &u.Username, &u.Email, &u.Password,
		&u.DisplayName, &u.AvatarURL, &u.Bio, &u.Website,
		&u.IsPrivate, &u.Followers, &u.Following, &u.Likes,
		&u.IsVerified, &u.CreatedAt,
	)
	return u, err
}

func getUserByID(id int) (User, error) {
	return scanUser(db.QueryRow(
		`SELECT `+userSelectCols+` FROM users WHERE id = $1`, id,
	))
}

const videoSelectCols = `v.id, v.user_id, v.description, v.url, v.thumbnail_url,
	v.likes, v.views, v.comments, v.is_public, v.allow_comments,
	v.allow_duet, v.allow_stitch, v.created_at, u.username, u.avatar_url`

func scanVideoRow(rows *sql.Rows) (Video, string, string, error) {
	var v Video
	var username, avatarURL string
	err := rows.Scan(
		&v.ID, &v.UserID, &v.Description, &v.URL, &v.ThumbnailURL,
		&v.Likes, &v.Views, &v.Comments, &v.IsPublic,
		&v.AllowComments, &v.AllowDuet, &v.AllowStitch, &v.CreatedAt,
		&username, &avatarURL,
	)
	return v, username, avatarURL, err
}

func collectVideoRows(rows *sql.Rows, viewerID int) []gin.H {
	var result []gin.H
	for rows.Next() {
		v, username, avatarURL, err := scanVideoRow(rows)
		if err == nil {
			result = append(result, formatVideo(v, username, avatarURL, viewerID))
		}
	}
	if result == nil {
		result = []gin.H{}
	}
	return result
}

func scanUserList(rows *sql.Rows) []gin.H {
	var result []gin.H
	for rows.Next() {
		var id, followers int
		var username, displayName, avatarURL string
		var isVerified bool
		if err := rows.Scan(&id, &username, &displayName, &avatarURL, &isVerified, &followers); err != nil {
			continue
		}
		result = append(result, gin.H{
			"id":             strconv.Itoa(id),
			"username":       username,
			"display_name":   displayName,
			"avatar_url":     avatarURL,
			"is_verified":    isVerified,
			"follower_count": followers,
		})
	}
	if result == nil {
		result = []gin.H{}
	}
	return result
}

// ── Auth helpers ──────────────────────────────────────────────────────────────

func hashPassword(plain string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	return string(h), err
}

func checkPassword(stored, plain string) bool {
	if strings.HasPrefix(stored, "$2") {
		return bcrypt.CompareHashAndPassword([]byte(stored), []byte(plain)) == nil
	}
	// Legacy plaintext fallback — automatically migrated on next login
	return stored == plain
}

func generateToken(userID int, expiry time.Duration) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(expiry).Unix(),
	})
	return token.SignedString(jwtKey)
}

func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(401, gin.H{"error": "missing token"})
			c.Abort()
			return
		}
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
			return jwtKey, nil
		})
		if err != nil || !token.Valid {
			c.JSON(401, gin.H{"error": "invalid token"})
			c.Abort()
			return
		}
		claims := token.Claims.(jwt.MapClaims)
		c.Set("user_id", int(claims["user_id"].(float64)))
		c.Next()
	}
}

// ── Notification helper ───────────────────────────────────────────────────────

func createNotification(userID int, nType, title, body string, actorID, entityID int, entityType string) {
	if userID <= 0 {
		return
	}
	var aID, eID any
	if actorID > 0 {
		aID = actorID
	}
	if entityID > 0 {
		eID = entityID
	}
	db.Exec(
		`INSERT INTO notifications (user_id, type, title, body, actor_id, entity_id, entity_type)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		userID, nType, title, body, aID, eID, entityType,
	)
}

// ── Response formatters ───────────────────────────────────────────────────────

func formatProfile(u User, viewerID int) gin.H {
	idStr := strconv.Itoa(u.ID)
	var videoCount int
	db.QueryRow(`SELECT COUNT(*) FROM videos WHERE user_id = $1`, u.ID).Scan(&videoCount)

	isFollowing := false
	if viewerID > 0 && viewerID != u.ID {
		db.QueryRow(
			`SELECT EXISTS(SELECT 1 FROM follows WHERE follower_id = $1 AND following_id = $2)`,
			viewerID, u.ID,
		).Scan(&isFollowing)
	}

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
		"video_count":     videoCount,
		"is_verified":     u.IsVerified,
		"is_creator":      false,
		"is_private":      u.IsPrivate,
		"is_following":    isFollowing,
		"created_at":      u.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func formatVideo(v Video, username, avatarURL string, viewerID int) gin.H {
	isLiked, isBookmarked := false, false
	if viewerID > 0 {
		db.QueryRow(
			`SELECT EXISTS(SELECT 1 FROM likes WHERE user_id = $1 AND video_id = $2)`,
			viewerID, v.ID,
		).Scan(&isLiked)
		db.QueryRow(
			`SELECT EXISTS(SELECT 1 FROM bookmarks WHERE user_id = $1 AND video_id = $2)`,
			viewerID, v.ID,
		).Scan(&isBookmarked)
	}
	return gin.H{
		"video_id":            strconv.Itoa(v.ID),
		"id":                  strconv.Itoa(v.ID),
		"video_url":           v.URL,
		"thumbnail_url":       v.ThumbnailURL,
		"hls_url":             "",
		"description":         v.Description,
		"title":               "",
		"hashtags":            []string{},
		"sound_title":         "Original Sound",
		"sound_artist":        username,
		"creator_id":          strconv.Itoa(v.UserID),
		"creator_username":    username,
		"creator_avatar_url":  avatarURL,
		"is_creator_verified": false,
		"is_following":        false,
		"like_count":          v.Likes,
		"comment_count":       v.Comments,
		"share_count":         0,
		"bookmark_count":      0,
		"view_count":          v.Views,
		"is_liked":            isLiked,
		"is_bookmarked":       isBookmarked,
		"duration":            0,
		"is_public":           v.IsPublic,
		"allow_comments":      v.AllowComments,
		"allow_duet":          v.AllowDuet,
		"allow_stitch":        v.AllowStitch,
		"created_at":          v.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func userResponse(u User, accessToken, refreshToken string) gin.H {
	return gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
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
			"created_at":   u.CreatedAt.UTC().Format(time.RFC3339),
		},
	}
}

// ── Auth handlers ─────────────────────────────────────────────────────────────

func register(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil ||
		req.Username == "" || req.Email == "" || req.Password == "" {
		c.JSON(400, gin.H{"error": "username, email and password are required"})
		return
	}

	hash, err := hashPassword(req.Password)
	if err != nil {
		c.JSON(500, gin.H{"error": "server error"})
		return
	}

	var id int
	err = db.QueryRow(
		`INSERT INTO users (username, email, password, display_name)
		 VALUES ($1, $2, $3, $4) RETURNING id`,
		req.Username, req.Email, hash, req.Username,
	).Scan(&id)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			c.JSON(409, gin.H{"error": "username or email already taken"})
		} else {
			log.Printf("register error: %v", err)
			c.JSON(500, gin.H{"error": "failed to create user"})
		}
		return
	}

	u, _ := getUserByID(id)
	access, _ := generateToken(id, 24*time.Hour)
	refresh, _ := generateToken(id, 7*24*time.Hour)
	c.JSON(201, userResponse(u, access, refresh))
}

func login(c *gin.Context) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	c.ShouldBindJSON(&req)

	u, err := scanUser(db.QueryRow(
		`SELECT `+userSelectCols+` FROM users WHERE email = $1`, req.Email,
	))
	if err == sql.ErrNoRows {
		c.JSON(401, gin.H{"error": "invalid credentials"})
		return
	}
	if err != nil {
		c.JSON(500, gin.H{"error": "server error"})
		return
	}
	if !checkPassword(u.Password, req.Password) {
		c.JSON(401, gin.H{"error": "invalid credentials"})
		return
	}

	// Migrate legacy plaintext password to bcrypt on first login
	if !strings.HasPrefix(u.Password, "$2") {
		if h, e := hashPassword(req.Password); e == nil {
			db.Exec(`UPDATE users SET password = $1 WHERE id = $2`, h, u.ID)
		}
	}

	access, _ := generateToken(u.ID, 24*time.Hour)
	refresh, _ := generateToken(u.ID, 7*24*time.Hour)
	c.JSON(200, userResponse(u, access, refresh))
}

func logout(c *gin.Context) {
	c.JSON(200, gin.H{"message": "logged out"})
}

func refreshToken(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	c.ShouldBindJSON(&req)

	tokenStr := strings.TrimPrefix(req.RefreshToken, "Bearer ")
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		return jwtKey, nil
	})
	if err != nil || !token.Valid {
		c.JSON(401, gin.H{"error": "invalid refresh token"})
		return
	}
	claims := token.Claims.(jwt.MapClaims)
	uid := int(claims["user_id"].(float64))
	access, _ := generateToken(uid, 24*time.Hour)
	refresh, _ := generateToken(uid, 7*24*time.Hour)
	c.JSON(200, gin.H{"access_token": access, "refresh_token": refresh})
}

func forgotPassword(c *gin.Context) {
	c.JSON(200, gin.H{"message": "password reset email sent"})
}

// ── User handlers ─────────────────────────────────────────────────────────────

func getProfile(c *gin.Context) {
	uid := c.MustGet("user_id").(int)
	id := c.Param("id")
	var u User
	var err error

	if numID, convErr := strconv.Atoi(id); convErr == nil {
		u, err = getUserByID(numID)
	} else {
		u, err = scanUser(db.QueryRow(
			`SELECT `+userSelectCols+` FROM users WHERE username = $1`, id,
		))
	}

	if err == sql.ErrNoRows {
		c.JSON(404, gin.H{"error": "user not found"})
		return
	}
	if err != nil {
		c.JSON(500, gin.H{"error": "server error"})
		return
	}
	c.JSON(200, formatProfile(u, uid))
}

func getMe(c *gin.Context) {
	uid := c.MustGet("user_id").(int)
	u, err := getUserByID(uid)
	if err == sql.ErrNoRows {
		c.JSON(404, gin.H{"error": "user not found"})
		return
	}
	if err != nil {
		c.JSON(500, gin.H{"error": "server error"})
		return
	}
	c.JSON(200, formatProfile(u, uid))
}

func updateMe(c *gin.Context) {
	uid := c.MustGet("user_id").(int)
	var req struct {
		DisplayName string `json:"display_name"`
		Username    string `json:"username"`
		Bio         string `json:"bio"`
		Website     string `json:"website"`
		IsPrivate   bool   `json:"is_private"`
	}
	c.ShouldBindJSON(&req)

	_, err := db.Exec(
		`UPDATE users
		 SET display_name = CASE WHEN $1 != '' THEN $1 ELSE display_name END,
		     username     = CASE WHEN $2 != '' THEN $2 ELSE username END,
		     bio          = $3,
		     website      = $4,
		     is_private   = $5
		 WHERE id = $6`,
		req.DisplayName, req.Username, req.Bio, req.Website, req.IsPrivate, uid,
	)
	if err != nil {
		log.Printf("updateMe error: %v", err)
		c.JSON(500, gin.H{"error": "update failed"})
		return
	}
	u, _ := getUserByID(uid)
	c.JSON(200, formatProfile(u, uid))
}

func followUser(c *gin.Context) {
	uid := c.MustGet("user_id").(int)
	targetID, _ := strconv.Atoi(c.Param("id"))
	if uid == targetID {
		c.JSON(400, gin.H{"error": "cannot follow yourself"})
		return
	}

	result, err := db.Exec(
		`INSERT INTO follows (follower_id, following_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`,
		uid, targetID,
	)
	if err != nil {
		c.JSON(500, gin.H{"error": "server error"})
		return
	}

	if rows, _ := result.RowsAffected(); rows > 0 {
		db.Exec(`UPDATE users SET followers = followers + 1 WHERE id = $1`, targetID)
		db.Exec(`UPDATE users SET following = following + 1 WHERE id = $1`, uid)
		createNotification(targetID, "follow", "started following you", "", uid, uid, "user")
	}
	c.JSON(200, gin.H{"message": "followed", "is_following": true})
}

func unfollowUser(c *gin.Context) {
	uid := c.MustGet("user_id").(int)
	targetID, _ := strconv.Atoi(c.Param("id"))

	result, err := db.Exec(
		`DELETE FROM follows WHERE follower_id = $1 AND following_id = $2`, uid, targetID,
	)
	if err != nil {
		c.JSON(500, gin.H{"error": "server error"})
		return
	}

	if rows, _ := result.RowsAffected(); rows > 0 {
		db.Exec(`UPDATE users SET followers = GREATEST(followers - 1, 0) WHERE id = $1`, targetID)
		db.Exec(`UPDATE users SET following = GREATEST(following - 1, 0) WHERE id = $1`, uid)
	}
	c.JSON(200, gin.H{"message": "unfollowed", "is_following": false})
}

func getFollowers(c *gin.Context) {
	targetID, _ := strconv.Atoi(c.Param("id"))
	rows, err := db.Query(`
		SELECT u.id, u.username, u.display_name, u.avatar_url, u.is_verified, u.followers
		FROM follows f JOIN users u ON u.id = f.follower_id
		WHERE f.following_id = $1 ORDER BY f.created_at DESC LIMIT 50`, targetID,
	)
	if err != nil {
		c.JSON(200, gin.H{"data": []any{}, "next_cursor": nil})
		return
	}
	defer rows.Close()
	c.JSON(200, gin.H{"data": scanUserList(rows), "next_cursor": nil})
}

func getFollowing(c *gin.Context) {
	targetID, _ := strconv.Atoi(c.Param("id"))
	rows, err := db.Query(`
		SELECT u.id, u.username, u.display_name, u.avatar_url, u.is_verified, u.followers
		FROM follows f JOIN users u ON u.id = f.following_id
		WHERE f.follower_id = $1 ORDER BY f.created_at DESC LIMIT 50`, targetID,
	)
	if err != nil {
		c.JSON(200, gin.H{"data": []any{}, "next_cursor": nil})
		return
	}
	defer rows.Close()
	c.JSON(200, gin.H{"data": scanUserList(rows), "next_cursor": nil})
}

// ── Video handlers ────────────────────────────────────────────────────────────

func uploadVideo(c *gin.Context) {
	uid := c.MustGet("user_id").(int)
	var req struct {
		Description   string `json:"description"`
		URL           string `json:"url"`
		ThumbnailURL  string `json:"thumbnail_url"`
		IsPublic      bool   `json:"is_public"`
		AllowComments bool   `json:"allow_comments"`
		AllowDuet     bool   `json:"allow_duet"`
		AllowStitch   bool   `json:"allow_stitch"`
	}
	c.ShouldBindJSON(&req)

	var v Video
	err := db.QueryRow(
		`INSERT INTO videos (user_id, description, url, thumbnail_url,
		                     is_public, allow_comments, allow_duet, allow_stitch)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		 RETURNING id, user_id, description, url, thumbnail_url, likes, views,
		           comments, is_public, allow_comments, allow_duet, allow_stitch, created_at`,
		uid, req.Description, req.URL, req.ThumbnailURL,
		req.IsPublic, req.AllowComments, req.AllowDuet, req.AllowStitch,
	).Scan(
		&v.ID, &v.UserID, &v.Description, &v.URL, &v.ThumbnailURL,
		&v.Likes, &v.Views, &v.Comments, &v.IsPublic,
		&v.AllowComments, &v.AllowDuet, &v.AllowStitch, &v.CreatedAt,
	)
	if err != nil {
		log.Printf("uploadVideo error: %v", err)
		c.JSON(500, gin.H{"error": "failed to save video"})
		return
	}
	u, _ := getUserByID(uid)
	c.JSON(201, formatVideo(v, u.Username, u.AvatarURL, uid))
}

func updateVideo(c *gin.Context) {
	uid := c.MustGet("user_id").(int)
	videoID, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		Description   string `json:"description"`
		IsPublic      bool   `json:"is_public"`
		AllowComments bool   `json:"allow_comments"`
		AllowDuet     bool   `json:"allow_duet"`
		AllowStitch   bool   `json:"allow_stitch"`
	}
	c.ShouldBindJSON(&req)

	var ownerID int
	if err := db.QueryRow(`SELECT user_id FROM videos WHERE id = $1`, videoID).Scan(&ownerID); err == sql.ErrNoRows {
		c.JSON(404, gin.H{"error": "video not found"})
		return
	}
	if ownerID != uid {
		c.JSON(403, gin.H{"error": "not your video"})
		return
	}

	var v Video
	err := db.QueryRow(
		`UPDATE videos
		 SET description = $1, is_public = $2, allow_comments = $3,
		     allow_duet = $4, allow_stitch = $5
		 WHERE id = $6
		 RETURNING id, user_id, description, url, thumbnail_url, likes, views,
		           comments, is_public, allow_comments, allow_duet, allow_stitch, created_at`,
		req.Description, req.IsPublic, req.AllowComments, req.AllowDuet, req.AllowStitch, videoID,
	).Scan(
		&v.ID, &v.UserID, &v.Description, &v.URL, &v.ThumbnailURL,
		&v.Likes, &v.Views, &v.Comments, &v.IsPublic,
		&v.AllowComments, &v.AllowDuet, &v.AllowStitch, &v.CreatedAt,
	)
	if err != nil {
		c.JSON(500, gin.H{"error": "update failed"})
		return
	}
	u, _ := getUserByID(uid)
	c.JSON(200, formatVideo(v, u.Username, u.AvatarURL, uid))
}

func getFeed(c *gin.Context) {
	uid := c.MustGet("user_id").(int)
	rows, err := db.Query(
		`SELECT `+videoSelectCols+`
		 FROM videos v JOIN users u ON u.id = v.user_id
		 WHERE v.is_public = TRUE ORDER BY v.created_at DESC LIMIT 50`,
	)
	if err != nil {
		c.JSON(200, gin.H{"data": []any{}, "next_cursor": nil})
		return
	}
	defer rows.Close()
	c.JSON(200, gin.H{"data": collectVideoRows(rows, uid), "next_cursor": nil})
}

func getFeedFollowing(c *gin.Context) {
	uid := c.MustGet("user_id").(int)
	rows, err := db.Query(`
		SELECT `+videoSelectCols+`
		FROM videos v
		JOIN users u ON u.id = v.user_id
		JOIN follows f ON f.following_id = v.user_id AND f.follower_id = $1
		WHERE v.is_public = TRUE ORDER BY v.created_at DESC LIMIT 50`, uid,
	)
	if err != nil {
		c.JSON(200, gin.H{"data": []any{}, "next_cursor": nil})
		return
	}
	defer rows.Close()
	c.JSON(200, gin.H{"data": collectVideoRows(rows, uid), "next_cursor": nil})
}

func getUserVideos(c *gin.Context) {
	uid := c.MustGet("user_id").(int)
	id, _ := strconv.Atoi(c.Param("id"))
	rows, err := db.Query(
		`SELECT `+videoSelectCols+`
		 FROM videos v JOIN users u ON u.id = v.user_id
		 WHERE v.user_id = $1 ORDER BY v.created_at DESC`, id,
	)
	if err != nil {
		c.JSON(200, gin.H{"data": []any{}, "next_cursor": nil})
		return
	}
	defer rows.Close()
	c.JSON(200, gin.H{"data": collectVideoRows(rows, uid), "next_cursor": nil})
}

func likeVideo(c *gin.Context) {
	uid := c.MustGet("user_id").(int)
	videoID, _ := strconv.Atoi(c.Param("id"))

	result, err := db.Exec(
		`INSERT INTO likes (user_id, video_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, uid, videoID,
	)
	if err != nil {
		c.JSON(500, gin.H{"error": "server error"})
		return
	}

	var likeCount int
	var isLiked bool
	if inserted, _ := result.RowsAffected(); inserted > 0 {
		db.QueryRow(`UPDATE videos SET likes = likes + 1 WHERE id = $1 RETURNING likes`, videoID).Scan(&likeCount)
		isLiked = true
		var ownerID int
		if db.QueryRow(`SELECT user_id FROM videos WHERE id = $1`, videoID).Scan(&ownerID) == nil && ownerID != uid {
			createNotification(ownerID, "like", "liked your video", "", uid, videoID, "video")
		}
	} else {
		db.Exec(`DELETE FROM likes WHERE user_id = $1 AND video_id = $2`, uid, videoID)
		db.QueryRow(`UPDATE videos SET likes = GREATEST(likes - 1, 0) WHERE id = $1 RETURNING likes`, videoID).Scan(&likeCount)
		isLiked = false
	}
	c.JSON(200, gin.H{"is_liked": isLiked, "like_count": likeCount})
}

func getLikedVideos(c *gin.Context) {
	uid := c.MustGet("user_id").(int)
	rows, err := db.Query(`
		SELECT `+videoSelectCols+`
		FROM likes l
		JOIN videos v ON v.id = l.video_id
		JOIN users u ON u.id = v.user_id
		WHERE l.user_id = $1 ORDER BY l.created_at DESC`, uid,
	)
	if err != nil {
		c.JSON(200, gin.H{"data": []any{}, "next_cursor": nil})
		return
	}
	defer rows.Close()
	c.JSON(200, gin.H{"data": collectVideoRows(rows, uid), "next_cursor": nil})
}

// ── Comment handlers ──────────────────────────────────────────────────────────

func getComments(c *gin.Context) {
	uid := c.MustGet("user_id").(int)
	videoID, _ := strconv.Atoi(c.Param("id"))

	rows, err := db.Query(`
		SELECT c.id, c.video_id, c.user_id, c.parent_id, c.content,
		       c.like_count, c.is_pinned, c.is_deleted, c.created_at,
		       u.username, u.avatar_url, u.display_name
		FROM comments c JOIN users u ON u.id = c.user_id
		WHERE c.video_id = $1 AND c.is_deleted = FALSE AND c.parent_id IS NULL
		ORDER BY c.is_pinned DESC, c.created_at DESC LIMIT 50`, videoID,
	)
	if err != nil {
		c.JSON(200, gin.H{"data": []any{}, "next_cursor": nil})
		return
	}
	defer rows.Close()

	var result []gin.H
	for rows.Next() {
		var cm Comment
		var username, avatarURL, displayName string
		if err := rows.Scan(
			&cm.ID, &cm.VideoID, &cm.UserID, &cm.ParentID, &cm.Content,
			&cm.LikeCount, &cm.IsPinned, &cm.IsDeleted, &cm.CreatedAt,
			&username, &avatarURL, &displayName,
		); err != nil {
			continue
		}
		isLiked := false
		db.QueryRow(
			`SELECT EXISTS(SELECT 1 FROM comment_likes WHERE user_id = $1 AND comment_id = $2)`,
			uid, cm.ID,
		).Scan(&isLiked)

		result = append(result, gin.H{
			"id":         strconv.Itoa(cm.ID),
			"video_id":   strconv.Itoa(cm.VideoID),
			"content":    cm.Content,
			"like_count": cm.LikeCount,
			"is_pinned":  cm.IsPinned,
			"is_liked":   isLiked,
			"created_at": cm.CreatedAt.UTC().Format(time.RFC3339),
			"user": gin.H{
				"id":           strconv.Itoa(cm.UserID),
				"username":     username,
				"avatar_url":   avatarURL,
				"display_name": displayName,
			},
		})
	}
	if result == nil {
		result = []gin.H{}
	}
	c.JSON(200, gin.H{"data": result, "next_cursor": nil})
}

func postComment(c *gin.Context) {
	uid := c.MustGet("user_id").(int)
	videoID, _ := strconv.Atoi(c.Param("id"))

	var req struct {
		Content  string `json:"content"`
		ParentID *int   `json:"parent_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Content) == "" {
		c.JSON(400, gin.H{"error": "content is required"})
		return
	}

	var cm Comment
	var err error
	if req.ParentID != nil {
		err = db.QueryRow(
			`INSERT INTO comments (video_id, user_id, parent_id, content)
			 VALUES ($1,$2,$3,$4)
			 RETURNING id, video_id, user_id, parent_id, content, like_count, is_pinned, is_deleted, created_at`,
			videoID, uid, *req.ParentID, req.Content,
		).Scan(&cm.ID, &cm.VideoID, &cm.UserID, &cm.ParentID, &cm.Content,
			&cm.LikeCount, &cm.IsPinned, &cm.IsDeleted, &cm.CreatedAt)
	} else {
		err = db.QueryRow(
			`INSERT INTO comments (video_id, user_id, content)
			 VALUES ($1,$2,$3)
			 RETURNING id, video_id, user_id, parent_id, content, like_count, is_pinned, is_deleted, created_at`,
			videoID, uid, req.Content,
		).Scan(&cm.ID, &cm.VideoID, &cm.UserID, &cm.ParentID, &cm.Content,
			&cm.LikeCount, &cm.IsPinned, &cm.IsDeleted, &cm.CreatedAt)
	}
	if err != nil {
		log.Printf("postComment error: %v", err)
		c.JSON(500, gin.H{"error": "failed to post comment"})
		return
	}

	db.Exec(`UPDATE videos SET comments = comments + 1 WHERE id = $1`, videoID)

	var ownerID int
	if db.QueryRow(`SELECT user_id FROM videos WHERE id = $1`, videoID).Scan(&ownerID) == nil && ownerID != uid {
		createNotification(ownerID, "comment", "commented on your video", req.Content, uid, videoID, "video")
	}

	u, _ := getUserByID(uid)
	c.JSON(201, gin.H{
		"id":         strconv.Itoa(cm.ID),
		"video_id":   strconv.Itoa(cm.VideoID),
		"content":    cm.Content,
		"like_count": cm.LikeCount,
		"is_pinned":  cm.IsPinned,
		"is_liked":   false,
		"created_at": cm.CreatedAt.UTC().Format(time.RFC3339),
		"user": gin.H{
			"id":           strconv.Itoa(uid),
			"username":     u.Username,
			"avatar_url":   u.AvatarURL,
			"display_name": u.DisplayName,
		},
	})
}

func likeComment(c *gin.Context) {
	uid := c.MustGet("user_id").(int)
	commentID, _ := strconv.Atoi(c.Param("id"))

	result, err := db.Exec(
		`INSERT INTO comment_likes (user_id, comment_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, uid, commentID,
	)
	if err != nil {
		c.JSON(500, gin.H{"error": "server error"})
		return
	}

	var likeCount int
	var isLiked bool
	if inserted, _ := result.RowsAffected(); inserted > 0 {
		db.QueryRow(`UPDATE comments SET like_count = like_count + 1 WHERE id = $1 RETURNING like_count`, commentID).Scan(&likeCount)
		isLiked = true
	} else {
		db.Exec(`DELETE FROM comment_likes WHERE user_id = $1 AND comment_id = $2`, uid, commentID)
		db.QueryRow(`UPDATE comments SET like_count = GREATEST(like_count - 1, 0) WHERE id = $1 RETURNING like_count`, commentID).Scan(&likeCount)
		isLiked = false
	}
	c.JSON(200, gin.H{"is_liked": isLiked, "like_count": likeCount})
}

func deleteComment(c *gin.Context) {
	uid := c.MustGet("user_id").(int)
	commentID, _ := strconv.Atoi(c.Param("id"))

	var ownerID, videoID int
	if err := db.QueryRow(`SELECT user_id, video_id FROM comments WHERE id = $1`, commentID).Scan(&ownerID, &videoID); err == sql.ErrNoRows {
		c.JSON(404, gin.H{"error": "comment not found"})
		return
	}
	if ownerID != uid {
		c.JSON(403, gin.H{"error": "not your comment"})
		return
	}

	db.Exec(`UPDATE comments SET is_deleted = TRUE WHERE id = $1`, commentID)
	db.Exec(`UPDATE videos SET comments = GREATEST(comments - 1, 0) WHERE id = $1`, videoID)
	c.JSON(200, gin.H{"message": "deleted"})
}

func pinComment(c *gin.Context) {
	uid := c.MustGet("user_id").(int)
	commentID, _ := strconv.Atoi(c.Param("id"))

	var videoOwnerID int
	err := db.QueryRow(`
		SELECT v.user_id FROM videos v
		JOIN comments cm ON cm.video_id = v.id
		WHERE cm.id = $1`, commentID,
	).Scan(&videoOwnerID)
	if err == sql.ErrNoRows {
		c.JSON(404, gin.H{"error": "comment not found"})
		return
	}
	if videoOwnerID != uid {
		c.JSON(403, gin.H{"error": "only the video creator can pin comments"})
		return
	}

	var isPinned bool
	db.QueryRow(`SELECT is_pinned FROM comments WHERE id = $1`, commentID).Scan(&isPinned)
	db.Exec(`UPDATE comments SET is_pinned = $1 WHERE id = $2`, !isPinned, commentID)
	c.JSON(200, gin.H{"is_pinned": !isPinned})
}

// ── Bookmark handlers ─────────────────────────────────────────────────────────

func toggleBookmark(c *gin.Context) {
	uid := c.MustGet("user_id").(int)
	videoID, _ := strconv.Atoi(c.Param("id"))

	result, err := db.Exec(
		`INSERT INTO bookmarks (user_id, video_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, uid, videoID,
	)
	if err != nil {
		c.JSON(500, gin.H{"error": "server error"})
		return
	}
	if inserted, _ := result.RowsAffected(); inserted > 0 {
		c.JSON(200, gin.H{"is_bookmarked": true})
	} else {
		db.Exec(`DELETE FROM bookmarks WHERE user_id = $1 AND video_id = $2`, uid, videoID)
		c.JSON(200, gin.H{"is_bookmarked": false})
	}
}

func getBookmarks(c *gin.Context) {
	uid := c.MustGet("user_id").(int)
	rows, err := db.Query(`
		SELECT `+videoSelectCols+`
		FROM bookmarks b
		JOIN videos v ON v.id = b.video_id
		JOIN users u ON u.id = v.user_id
		WHERE b.user_id = $1 ORDER BY b.created_at DESC`, uid,
	)
	if err != nil {
		c.JSON(200, gin.H{"data": []any{}, "next_cursor": nil})
		return
	}
	defer rows.Close()
	c.JSON(200, gin.H{"data": collectVideoRows(rows, uid), "next_cursor": nil})
}

// ── Notification handlers ─────────────────────────────────────────────────────

func getNotifications(c *gin.Context) {
	uid := c.MustGet("user_id").(int)
	rows, err := db.Query(`
		SELECT n.id, n.type, n.title, n.body, n.actor_id, n.entity_id, n.entity_type,
		       n.is_read, n.created_at, u.username, u.avatar_url
		FROM notifications n
		LEFT JOIN users u ON u.id = n.actor_id
		WHERE n.user_id = $1 ORDER BY n.created_at DESC LIMIT 50`, uid,
	)
	if err != nil {
		c.JSON(200, gin.H{"data": []any{}, "next_cursor": nil})
		return
	}
	defer rows.Close()

	var result []gin.H
	for rows.Next() {
		var id int
		var nType, title, body string
		var actorID, entityID sql.NullInt64
		var entityType sql.NullString
		var isRead bool
		var createdAt time.Time
		var actorUsername, actorAvatar sql.NullString

		if err := rows.Scan(&id, &nType, &title, &body, &actorID, &entityID, &entityType,
			&isRead, &createdAt, &actorUsername, &actorAvatar); err != nil {
			continue
		}
		item := gin.H{
			"id":         strconv.Itoa(id),
			"type":       nType,
			"title":      title,
			"body":       body,
			"is_read":    isRead,
			"created_at": createdAt.UTC().Format(time.RFC3339),
		}
		if actorID.Valid {
			item["actor"] = gin.H{
				"id":         strconv.FormatInt(actorID.Int64, 10),
				"username":   actorUsername.String,
				"avatar_url": actorAvatar.String,
			}
		}
		if entityID.Valid {
			item["entity_id"] = strconv.FormatInt(entityID.Int64, 10)
		}
		if entityType.Valid {
			item["entity_type"] = entityType.String
		}
		result = append(result, item)
	}
	if result == nil {
		result = []gin.H{}
	}
	c.JSON(200, gin.H{"data": result, "next_cursor": nil})
}

func getUnreadCount(c *gin.Context) {
	uid := c.MustGet("user_id").(int)
	var count int
	db.QueryRow(`SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND is_read = FALSE`, uid).Scan(&count)
	c.JSON(200, gin.H{"count": count})
}

func markNotificationRead(c *gin.Context) {
	uid := c.MustGet("user_id").(int)
	nID, _ := strconv.Atoi(c.Param("id"))
	db.Exec(`UPDATE notifications SET is_read = TRUE, read_at = NOW() WHERE id = $1 AND user_id = $2`, nID, uid)
	c.JSON(200, gin.H{"message": "ok"})
}

func markAllNotificationsRead(c *gin.Context) {
	uid := c.MustGet("user_id").(int)
	db.Exec(`UPDATE notifications SET is_read = TRUE, read_at = NOW() WHERE user_id = $1 AND is_read = FALSE`, uid)
	c.JSON(200, gin.H{"message": "ok"})
}

// ── Messaging handlers ────────────────────────────────────────────────────────

func getConversations(c *gin.Context) {
	uid := c.MustGet("user_id").(int)
	rows, err := db.Query(`
		SELECT c.id, c.updated_at,
		       u.id, u.username, u.avatar_url, u.display_name,
		       (SELECT content FROM messages WHERE conversation_id = c.id
		        AND is_deleted = FALSE ORDER BY created_at DESC LIMIT 1)
		FROM conversations c
		JOIN conversation_members cm  ON cm.conversation_id  = c.id AND cm.user_id  = $1
		JOIN conversation_members cm2 ON cm2.conversation_id = c.id AND cm2.user_id != $1
		JOIN users u ON u.id = cm2.user_id
		ORDER BY c.updated_at DESC`, uid,
	)
	if err != nil {
		c.JSON(200, gin.H{"data": []any{}, "next_cursor": nil})
		return
	}
	defer rows.Close()

	var result []gin.H
	for rows.Next() {
		var convID, partnerID int
		var updatedAt time.Time
		var partnerUsername, partnerAvatar, partnerDisplay string
		var lastMsg sql.NullString
		if err := rows.Scan(&convID, &updatedAt, &partnerID, &partnerUsername, &partnerAvatar, &partnerDisplay, &lastMsg); err != nil {
			continue
		}
		item := gin.H{
			"id":         strconv.Itoa(convID),
			"updated_at": updatedAt.UTC().Format(time.RFC3339),
			"partner": gin.H{
				"id":           strconv.Itoa(partnerID),
				"username":     partnerUsername,
				"avatar_url":   partnerAvatar,
				"display_name": partnerDisplay,
			},
		}
		if lastMsg.Valid {
			item["last_message"] = lastMsg.String
		}
		result = append(result, item)
	}
	if result == nil {
		result = []gin.H{}
	}
	c.JSON(200, gin.H{"data": result, "next_cursor": nil})
}

func createOrGetConversation(c *gin.Context) {
	uid := c.MustGet("user_id").(int)
	var req struct {
		UserID int `json:"user_id"`
	}
	c.ShouldBindJSON(&req)
	if req.UserID == 0 || req.UserID == uid {
		c.JSON(400, gin.H{"error": "invalid user_id"})
		return
	}

	// Find existing DM between the two users
	var convID int
	err := db.QueryRow(`
		SELECT c.id FROM conversations c
		JOIN conversation_members cm1 ON cm1.conversation_id = c.id AND cm1.user_id = $1
		JOIN conversation_members cm2 ON cm2.conversation_id = c.id AND cm2.user_id = $2
		LIMIT 1`, uid, req.UserID,
	).Scan(&convID)

	if err == sql.ErrNoRows {
		if err = db.QueryRow(`INSERT INTO conversations DEFAULT VALUES RETURNING id`).Scan(&convID); err != nil {
			c.JSON(500, gin.H{"error": "failed to create conversation"})
			return
		}
		db.Exec(
			`INSERT INTO conversation_members (conversation_id, user_id) VALUES ($1,$2),($1,$3)`,
			convID, uid, req.UserID,
		)
	}
	c.JSON(200, gin.H{"id": strconv.Itoa(convID)})
}

func getMessages(c *gin.Context) {
	uid := c.MustGet("user_id").(int)
	convID, _ := strconv.Atoi(c.Param("id"))

	var isMember bool
	db.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM conversation_members WHERE conversation_id = $1 AND user_id = $2)`,
		convID, uid,
	).Scan(&isMember)
	if !isMember {
		c.JSON(403, gin.H{"error": "not a member of this conversation"})
		return
	}

	rows, err := db.Query(`
		SELECT m.id, m.sender_id, m.content, m.type, m.created_at,
		       u.username, u.avatar_url
		FROM messages m JOIN users u ON u.id = m.sender_id
		WHERE m.conversation_id = $1 AND m.is_deleted = FALSE
		ORDER BY m.created_at ASC LIMIT 100`, convID,
	)
	if err != nil {
		c.JSON(200, gin.H{"data": []any{}, "next_cursor": nil})
		return
	}
	defer rows.Close()

	var result []gin.H
	for rows.Next() {
		var msgID, senderID int
		var content, msgType, senderUsername, senderAvatar string
		var createdAt time.Time
		if err := rows.Scan(&msgID, &senderID, &content, &msgType, &createdAt, &senderUsername, &senderAvatar); err != nil {
			continue
		}
		result = append(result, gin.H{
			"id":         strconv.Itoa(msgID),
			"sender_id":  strconv.Itoa(senderID),
			"content":    content,
			"type":       msgType,
			"is_mine":    senderID == uid,
			"created_at": createdAt.UTC().Format(time.RFC3339),
			"sender": gin.H{
				"id":         strconv.Itoa(senderID),
				"username":   senderUsername,
				"avatar_url": senderAvatar,
			},
		})
	}
	if result == nil {
		result = []gin.H{}
	}
	c.JSON(200, gin.H{"data": result, "next_cursor": nil})
}

func sendMessage(c *gin.Context) {
	uid := c.MustGet("user_id").(int)
	convID, _ := strconv.Atoi(c.Param("id"))

	var isMember bool
	db.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM conversation_members WHERE conversation_id = $1 AND user_id = $2)`,
		convID, uid,
	).Scan(&isMember)
	if !isMember {
		c.JSON(403, gin.H{"error": "not a member of this conversation"})
		return
	}

	var req struct {
		Content string `json:"content"`
		Type    string `json:"type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Content) == "" {
		c.JSON(400, gin.H{"error": "content is required"})
		return
	}
	if req.Type == "" {
		req.Type = "text"
	}

	var msgID int
	var createdAt time.Time
	if err := db.QueryRow(
		`INSERT INTO messages (conversation_id, sender_id, content, type)
		 VALUES ($1,$2,$3,$4) RETURNING id, created_at`,
		convID, uid, req.Content, req.Type,
	).Scan(&msgID, &createdAt); err != nil {
		c.JSON(500, gin.H{"error": "failed to send message"})
		return
	}
	db.Exec(`UPDATE conversations SET updated_at = NOW() WHERE id = $1`, convID)

	u, _ := getUserByID(uid)
	c.JSON(201, gin.H{
		"id":         strconv.Itoa(msgID),
		"sender_id":  strconv.Itoa(uid),
		"content":    req.Content,
		"type":       req.Type,
		"is_mine":    true,
		"created_at": createdAt.UTC().Format(time.RFC3339),
		"sender": gin.H{
			"id":         strconv.Itoa(uid),
			"username":   u.Username,
			"avatar_url": u.AvatarURL,
		},
	})
}

// ── Route registration ────────────────────────────────────────────────────────

func registerRoutes(g *gin.RouterGroup) {
	// Profile
	g.GET("/users/me", getMe)
	g.PUT("/users/me", updateMe)
	g.PATCH("/users/me", updateMe)
	g.POST("/users/me/avatar", func(c *gin.Context) { c.JSON(200, gin.H{"avatar_url": ""}) })
	g.GET("/users/check-username", func(c *gin.Context) {
		username := c.Query("username")
		var exists bool
		db.QueryRow(
			`SELECT EXISTS(SELECT 1 FROM users WHERE LOWER(username) = LOWER($1))`, username,
		).Scan(&exists)
		c.JSON(200, gin.H{"available": !exists})
	})
	g.GET("/users/:id", getProfile)
	g.GET("/users/:id/profile", getProfile)
	g.POST("/users/:id/follow", followUser)
	g.DELETE("/users/:id/follow", unfollowUser)
	g.GET("/users/:id/followers", getFollowers)
	g.GET("/users/:id/following", getFollowing)
	g.GET("/users/:id/videos", getUserVideos)

	// Feed
	g.GET("/feed", getFeed)
	g.GET("/feed/for-you", getFeed)
	g.GET("/feed/following", getFeedFollowing)
	g.POST("/feed/view", func(c *gin.Context) { c.JSON(200, gin.H{"message": "ok"}) })

	// Videos
	g.POST("/videos", uploadVideo)
	g.PUT("/videos/:id", updateVideo)
	g.GET("/videos/:id", func(c *gin.Context) {
		uid := c.MustGet("user_id").(int)
		id, _ := strconv.Atoi(c.Param("id"))
		rows, err := db.Query(
			`SELECT `+videoSelectCols+`
			 FROM videos v JOIN users u ON u.id = v.user_id
			 WHERE v.id = $1`, id,
		)
		if err != nil {
			c.JSON(500, gin.H{"error": "query failed"})
			return
		}
		defer rows.Close()
		if !rows.Next() {
			c.JSON(404, gin.H{"error": "video not found"})
			return
		}
		v, username, avatarURL, err := scanVideoRow(rows)
		if err != nil {
			c.JSON(500, gin.H{"error": "scan failed"})
			return
		}
		c.JSON(200, formatVideo(v, username, avatarURL, uid))
	})
	g.POST("/videos/:id/like", likeVideo)
	g.GET("/videos/:id/like-status", func(c *gin.Context) {
		uid := c.MustGet("user_id").(int)
		videoID, _ := strconv.Atoi(c.Param("id"))
		var isLiked bool
		db.QueryRow(
			`SELECT EXISTS(SELECT 1 FROM likes WHERE user_id = $1 AND video_id = $2)`, uid, videoID,
		).Scan(&isLiked)
		c.JSON(200, gin.H{"is_liked": isLiked})
	})
	g.POST("/videos/:id/bookmark", toggleBookmark)
	g.POST("/videos/:id/view", func(c *gin.Context) {
		id, _ := strconv.Atoi(c.Param("id"))
		db.Exec(`UPDATE videos SET views = views + 1 WHERE id = $1`, id)
		c.JSON(200, gin.H{"message": "ok"})
	})
	g.GET("/videos/:id/comments", getComments)
	g.POST("/videos/:id/comments", postComment)

	// Social
	g.POST("/comments/:id/like", likeComment)
	g.GET("/me/liked-videos", getLikedVideos)
	g.GET("/me/bookmarks", getBookmarks)
	g.GET("/users/me/bookmarks", getBookmarks)

	// Notifications
	g.GET("/notifications", getNotifications)
	g.GET("/notifications/unread-count", getUnreadCount)
	g.PUT("/notifications/:id/read", markNotificationRead)
	g.PUT("/notifications/read-all", markAllNotificationsRead)

	// Search — real DB queries
	g.GET("/search", func(c *gin.Context) {
		uid := c.MustGet("user_id").(int)
		q := strings.TrimSpace(c.Query("q"))
		searchType := c.Query("type")
		if q == "" {
			c.JSON(200, gin.H{"data": []gin.H{}, "next_cursor": nil})
			return
		}
		pattern := "%" + strings.ToLower(q) + "%"
		var result []gin.H

		// User search
		if searchType == "" || searchType == "user" {
			rows, err := db.Query(`
				SELECT id, username, display_name, avatar_url, is_verified, followers, following
				FROM users
				WHERE LOWER(username) LIKE $1 OR LOWER(display_name) LIKE $1
				ORDER BY followers DESC LIMIT 20`, pattern)
			if err == nil {
				defer rows.Close()
				for rows.Next() {
					var id, followers, following int
					var username, displayName, avatarURL string
					var isVerified bool
					if err := rows.Scan(&id, &username, &displayName, &avatarURL, &isVerified, &followers, &following); err == nil {
						result = append(result, gin.H{
							"result_type":     "user",
							"id":              strconv.Itoa(id),
							"user_id":         strconv.Itoa(id),
							"username":        username,
							"display_name":    displayName,
							"avatar_url":      avatarURL,
							"is_verified":     isVerified,
							"follower_count":  followers,
							"following_count": following,
						})
					}
				}
			}
		}

		// Video search
		if searchType == "" || searchType == "video" {
			videoRows, err := db.Query(`
				SELECT `+videoSelectCols+`
				FROM videos v JOIN users u ON u.id = v.user_id
				WHERE v.is_public = TRUE AND LOWER(v.description) LIKE $1
				ORDER BY v.likes DESC, v.created_at DESC LIMIT 20`, pattern)
			if err == nil {
				defer videoRows.Close()
				for videoRows.Next() {
					v, username, avatarURL, err := scanVideoRow(videoRows)
					if err == nil {
						vf := formatVideo(v, username, avatarURL, uid)
						vf["result_type"] = "video"
						result = append(result, vf)
					}
				}
			}
		}

		if result == nil {
			result = []gin.H{}
		}
		c.JSON(200, gin.H{"data": result, "next_cursor": nil})
	})
	g.GET("/search/trending", func(c *gin.Context) {
		rows, _ := db.Query(`SELECT username FROM users ORDER BY followers DESC LIMIT 10`)
		var tags []string
		if rows != nil {
			defer rows.Close()
			for rows.Next() {
				var u string
				if rows.Scan(&u) == nil {
					tags = append(tags, u)
				}
			}
		}
		if len(tags) == 0 {
			tags = []string{"fyp", "trending", "viral"}
		}
		c.JSON(200, gin.H{"data": tags})
	})
	g.GET("/search/suggestions", func(c *gin.Context) {
		q := strings.TrimSpace(c.Query("q"))
		if q == "" {
			c.JSON(200, gin.H{"suggestions": []string{}})
			return
		}
		pattern := strings.ToLower(q) + "%"
		rows, _ := db.Query(`SELECT username FROM users WHERE LOWER(username) LIKE $1 LIMIT 8`, pattern)
		var suggestions []string
		if rows != nil {
			defer rows.Close()
			for rows.Next() {
				var u string
				if rows.Scan(&u) == nil {
					suggestions = append(suggestions, u)
				}
			}
		}
		if suggestions == nil {
			suggestions = []string{}
		}
		c.JSON(200, gin.H{"suggestions": suggestions})
	})
	g.GET("/search/history", func(c *gin.Context) { c.JSON(200, gin.H{"data": []string{}, "history": []string{}}) })
	g.POST("/search/history", func(c *gin.Context) { c.JSON(200, gin.H{"message": "saved"}) })
	g.DELETE("/search/history", func(c *gin.Context) { c.JSON(200, gin.H{"message": "cleared"}) })

	// Wallet (stub — no payment integration yet)
	g.GET("/wallet/balance", func(c *gin.Context) { c.JSON(200, gin.H{"balance": 0, "coins": 0}) })
	g.GET("/wallet/packages", func(c *gin.Context) {
		c.JSON(200, gin.H{"data": []gin.H{
			{"id": "1", "coins": 70, "price": 1.0, "label": "$1"},
			{"id": "2", "coins": 420, "price": 6.0, "label": "$6"},
			{"id": "3", "coins": 2100, "price": 30.0, "label": "$30"},
		}})
	})

	// Messaging
	g.GET("/conversations", getConversations)
	g.POST("/conversations", createOrGetConversation)
	g.GET("/conversations/:id/messages", getMessages)
	g.POST("/conversations/:id/messages", sendMessage)

	// Misc stubs
	g.GET("/sounds/trending", func(c *gin.Context) { c.JSON(200, gin.H{"data": []any{}}) })
	g.GET("/hashtags/trending", func(c *gin.Context) { c.JSON(200, gin.H{"data": []any{}}) })
	g.POST("/reports", func(c *gin.Context) { c.JSON(201, gin.H{"message": "reported"}) })
	g.POST("/devices", func(c *gin.Context) { c.JSON(200, gin.H{"message": "ok"}) })
	g.DELETE("/devices/:token", func(c *gin.Context) { c.JSON(200, gin.H{"message": "ok"}) })
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	initDB()

	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "Accept"},
		AllowCredentials: false,
	}))

	r.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })
	r.GET("/healthz", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })

	for _, prefix := range []string{"", "/auth", "/api/v1/auth"} {
		r.POST(prefix+"/register", register)
		r.POST(prefix+"/login", login)
	}
	r.POST("/auth/logout", logout)
	r.POST("/api/v1/auth/logout", logout)
	r.POST("/auth/refresh", refreshToken)
	r.POST("/api/v1/auth/refresh", refreshToken)
	r.POST("/auth/forgot-password", forgotPassword)
	r.POST("/auth/otp/send", func(c *gin.Context) { c.JSON(200, gin.H{"message": "OTP sent"}) })
	r.POST("/auth/otp/verify", func(c *gin.Context) { c.JSON(200, gin.H{"message": "OTP verified"}) })
	r.POST("/auth/verify-email", func(c *gin.Context) { c.JSON(200, gin.H{"message": "email verified"}) })

	auth := r.Group("/")
	auth.Use(authMiddleware())
	authV1 := r.Group("/api/v1")
	authV1.Use(authMiddleware())

	registerRoutes(auth)
	registerRoutes(authV1)

	// Legacy compat aliases
	auth.POST("/api/videos", uploadVideo)
	auth.GET("/api/profile/:id", getProfile)
	auth.GET("/api/feed", getFeed)
	auth.POST("/api/videos/:id/like", likeVideo)
	auth.POST("/api/users/:id/follow", followUser)

	// Extra routes on auth group only
	auth.POST("/comments/:id/pin", pinComment)
	auth.DELETE("/comments/:id", deleteComment)
	auth.GET("/wallet/transactions", func(c *gin.Context) {
		c.JSON(200, gin.H{"data": []any{}, "next_cursor": nil})
	})
	auth.POST("/wallet/coins/buy", func(c *gin.Context) { c.JSON(200, gin.H{"message": "purchase initiated"}) })
	auth.POST("/wallet/coins/confirm", func(c *gin.Context) { c.JSON(200, gin.H{"message": "purchase confirmed"}) })
	auth.POST("/wallet/withdraw", func(c *gin.Context) { c.JSON(200, gin.H{"message": "withdrawal requested"}) })
	auth.PUT("/conversations/:id/read", func(c *gin.Context) {
		uid := c.MustGet("user_id").(int)
		convID, _ := strconv.Atoi(c.Param("id"))
		db.Exec(`UPDATE conversation_members SET last_read_at = NOW() WHERE conversation_id = $1 AND user_id = $2`, convID, uid)
		c.JSON(200, gin.H{"message": "read"})
	})
	auth.GET("/analytics/profile/stats", func(c *gin.Context) {
		uid := c.MustGet("user_id").(int)
		var views, likes, followers int
		db.QueryRow(`SELECT COALESCE(SUM(views),0), COALESCE(SUM(likes),0) FROM videos WHERE user_id = $1`, uid).Scan(&views, &likes)
		db.QueryRow(`SELECT followers FROM users WHERE id = $1`, uid).Scan(&followers)
		c.JSON(200, gin.H{"views": views, "likes": likes, "followers": followers})
	})
	auth.GET("/hashtags/:tag", func(c *gin.Context) {
		c.JSON(200, gin.H{"tag": c.Param("tag"), "video_count": 0})
	})
	auth.GET("/sounds/:id", func(c *gin.Context) { c.JSON(200, gin.H{"id": c.Param("id")}) })
	auth.GET("/sounds/search", func(c *gin.Context) { c.JSON(200, gin.H{"data": []any{}}) })
	auth.GET("/gifts", func(c *gin.Context) { c.JSON(200, gin.H{"data": []any{}}) })
	auth.POST("/gifts/send", func(c *gin.Context) { c.JSON(200, gin.H{"message": "gift sent"}) })
	auth.GET("/live/:id", func(c *gin.Context) {
		c.JSON(200, gin.H{"id": c.Param("id"), "status": "live"})
	})

	port := getEnv("PORT", "8080")
	log.Printf("Server starting on :%s", port)
	r.Run(":" + port)
}
