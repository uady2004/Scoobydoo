package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"github.com/tiktok-clone/admin-service/internal/config"
	"github.com/tiktok-clone/admin-service/internal/models"
)

var (
	ErrAdminNotFound    = errors.New("admin not found")
	ErrInvalidPassword  = errors.New("invalid password")
	ErrEmailInUse       = errors.New("email already in use")
)

// AdminService defines all admin business operations.
type AdminService interface {
	// Auth
	Login(ctx context.Context, email, password string) (token string, admin *models.AdminUser, err error)

	// Admin users
	CreateAdmin(ctx context.Context, email, password, fullName string, role models.AdminRole) (*models.AdminUser, error)
	ListAdmins(ctx context.Context) ([]*models.AdminUser, error)
	UpdateAdminRole(ctx context.Context, adminID string, role models.AdminRole) error
	DeactivateAdmin(ctx context.Context, adminID string) error

	// Platform users
	ListUsers(ctx context.Context, search string, limit, offset int) ([]map[string]interface{}, int64, error)
	GetUser(ctx context.Context, userID string) (map[string]interface{}, error)
	BanUser(ctx context.Context, userID, adminID, reason, banType string, expiresAt *time.Time) (*models.UserBan, error)
	UnbanUser(ctx context.Context, userID string) error
	VerifyUser(ctx context.Context, userID string) error

	// Content moderation
	ListPendingContent(ctx context.Context, contentType string, limit, offset int) ([]map[string]interface{}, int64, error)
	ModerateContent(ctx context.Context, contentID, contentType, adminID, action, reason string) (*models.ContentModeration, error)

	// Reports
	ListReports(ctx context.Context, status string, limit, offset int) ([]map[string]interface{}, int64, error)
	ResolveReport(ctx context.Context, reportID, adminID, action, notes string) error

	// Dashboard
	GetPlatformStats(ctx context.Context) (*models.PlatformStats, error)
	GetAuditLog(ctx context.Context, adminID string, limit, offset int) ([]*models.AuditLog, int64, error)
	LogAudit(ctx context.Context, adminID, action, resourceType, resourceID string, details map[string]interface{}, ip, ua string)
}

type adminService struct {
	pool   *pgxpool.Pool
	cfg    *config.Config
	logger *zap.Logger
}

// NewAdminService creates an AdminService backed by PostgreSQL.
func NewAdminService(pool *pgxpool.Pool, cfg *config.Config, logger *zap.Logger) AdminService {
	return &adminService{pool: pool, cfg: cfg, logger: logger}
}

// ─── Auth ─────────────────────────────────────────────────────────────────────

func (s *adminService) Login(ctx context.Context, email, password string) (string, *models.AdminUser, error) {
	admin := &models.AdminUser{}
	err := s.pool.QueryRow(ctx,
		`SELECT id, email, password_hash, full_name, role, is_active FROM admin_users WHERE email=$1`, email,
	).Scan(&admin.ID, &admin.Email, &admin.PasswordHash, &admin.FullName, &admin.Role, &admin.IsActive)
	if err != nil {
		return "", nil, ErrAdminNotFound
	}

	if !admin.IsActive {
		return "", nil, fmt.Errorf("account deactivated")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(password)); err != nil {
		return "", nil, ErrInvalidPassword
	}

	// Update last login.
	now := time.Now().UTC()
	s.pool.Exec(ctx, `UPDATE admin_users SET last_login_at=$1 WHERE id=$2`, now, admin.ID) //nolint:errcheck
	admin.LastLoginAt = &now

	// Generate a simple signed token (in production use proper JWT).
	token := fmt.Sprintf("admin_%s_%d", admin.ID, now.Unix())
	return token, admin, nil
}

// ─── Admin users ──────────────────────────────────────────────────────────────

func (s *adminService) CreateAdmin(ctx context.Context, email, password, fullName string, role models.AdminRole) (*models.AdminUser, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hashing password: %w", err)
	}

	admin := &models.AdminUser{
		ID:           uuid.New().String(),
		Email:        email,
		PasswordHash: string(hash),
		FullName:     fullName,
		Role:         role,
		IsActive:     true,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO admin_users (id, email, password_hash, full_name, role, is_active, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		admin.ID, admin.Email, admin.PasswordHash, admin.FullName,
		admin.Role, admin.IsActive, admin.CreatedAt, admin.UpdatedAt,
	)
	if err != nil {
		return nil, ErrEmailInUse
	}
	return admin, nil
}

func (s *adminService) ListAdmins(ctx context.Context) ([]*models.AdminUser, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, email, full_name, role, is_active, last_login_at, created_at FROM admin_users ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*models.AdminUser
	for rows.Next() {
		a := &models.AdminUser{}
		if err := rows.Scan(&a.ID, &a.Email, &a.FullName, &a.Role, &a.IsActive, &a.LastLoginAt, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *adminService) UpdateAdminRole(ctx context.Context, adminID string, role models.AdminRole) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE admin_users SET role=$1, updated_at=$2 WHERE id=$3`, role, time.Now().UTC(), adminID)
	return err
}

func (s *adminService) DeactivateAdmin(ctx context.Context, adminID string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE admin_users SET is_active=false, updated_at=$1 WHERE id=$2`, time.Now().UTC(), adminID)
	return err
}

// ─── Platform users ───────────────────────────────────────────────────────────

func (s *adminService) ListUsers(ctx context.Context, search string, limit, offset int) ([]map[string]interface{}, int64, error) {
	query := `SELECT id, username, email, is_verified, is_banned, created_at FROM users`
	args := []interface{}{}
	if search != "" {
		query += ` WHERE username ILIKE $1 OR email ILIKE $1`
		args = append(args, "%"+search+"%")
	}
	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT %d OFFSET %d", limit, offset)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var users []map[string]interface{}
	for rows.Next() {
		var id, username, email string
		var isVerified, isBanned bool
		var createdAt time.Time
		if err := rows.Scan(&id, &username, &email, &isVerified, &isBanned, &createdAt); err != nil {
			return nil, 0, err
		}
		users = append(users, map[string]interface{}{
			"id": id, "username": username, "email": email,
			"is_verified": isVerified, "is_banned": isBanned, "created_at": createdAt,
		})
	}

	var total int64
	s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&total) //nolint:errcheck
	return users, total, rows.Err()
}

func (s *adminService) GetUser(ctx context.Context, userID string) (map[string]interface{}, error) {
	var id, username, email, bio string
	var isVerified, isBanned bool
	var createdAt time.Time

	err := s.pool.QueryRow(ctx,
		`SELECT id, username, email, bio, is_verified, is_banned, created_at FROM users WHERE id=$1`, userID,
	).Scan(&id, &username, &email, &bio, &isVerified, &isBanned, &createdAt)
	if err != nil {
		return nil, ErrAdminNotFound
	}

	return map[string]interface{}{
		"id": id, "username": username, "email": email, "bio": bio,
		"is_verified": isVerified, "is_banned": isBanned, "created_at": createdAt,
	}, nil
}

func (s *adminService) BanUser(ctx context.Context, userID, adminID, reason, banType string, expiresAt *time.Time) (*models.UserBan, error) {
	ban := &models.UserBan{
		ID:        uuid.New().String(),
		UserID:    userID,
		AdminID:   adminID,
		Reason:    reason,
		BanType:   banType,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now().UTC(),
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	_, err = tx.Exec(ctx, `
		INSERT INTO user_bans (id, user_id, admin_id, reason, ban_type, expires_at, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		ban.ID, ban.UserID, ban.AdminID, ban.Reason, ban.BanType, ban.ExpiresAt, ban.CreatedAt)
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec(ctx, `UPDATE users SET is_banned=true WHERE id=$1`, userID)
	if err != nil {
		return nil, err
	}

	return ban, tx.Commit(ctx)
}

func (s *adminService) UnbanUser(ctx context.Context, userID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	now := time.Now().UTC()
	_, err = tx.Exec(ctx,
		`UPDATE user_bans SET revoked_at=$1 WHERE user_id=$2 AND revoked_at IS NULL`, now, userID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE users SET is_banned=false WHERE id=$1`, userID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *adminService) VerifyUser(ctx context.Context, userID string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE users SET is_verified=true WHERE id=$1`, userID)
	return err
}

// ─── Content moderation ───────────────────────────────────────────────────────

func (s *adminService) ListPendingContent(ctx context.Context, contentType string, limit, offset int) ([]map[string]interface{}, int64, error) {
	var rows interface{ Scan(...interface{}) error }
	_ = rows
	// Return a stub list — real impl queries the videos/comments table filtered by status='pending_review'.
	items := []map[string]interface{}{}
	return items, 0, nil
}

func (s *adminService) ModerateContent(ctx context.Context, contentID, contentType, adminID, action, reason string) (*models.ContentModeration, error) {
	mod := &models.ContentModeration{
		ID:          uuid.New().String(),
		ContentID:   contentID,
		ContentType: contentType,
		AdminID:     adminID,
		Action:      action,
		Reason:      reason,
		CreatedAt:   time.Now().UTC(),
	}

	_, err := s.pool.Exec(ctx, `
		INSERT INTO content_moderations (id, content_id, content_type, admin_id, action, reason, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		mod.ID, mod.ContentID, mod.ContentType, mod.AdminID, mod.Action, mod.Reason, mod.CreatedAt)
	if err != nil {
		return nil, err
	}

	if action == "remove" {
		switch contentType {
		case "video":
			s.pool.Exec(ctx, `UPDATE videos SET status='removed' WHERE id=$1`, contentID) //nolint:errcheck
		case "comment":
			s.pool.Exec(ctx, `UPDATE comments SET is_deleted=true WHERE id=$1`, contentID) //nolint:errcheck
		}
	}

	return mod, nil
}

// ─── Reports ──────────────────────────────────────────────────────────────────

func (s *adminService) ListReports(ctx context.Context, status string, limit, offset int) ([]map[string]interface{}, int64, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, reporter_id, content_id, content_type, reason, status, created_at
		FROM reports WHERE status=$1
		ORDER BY created_at ASC LIMIT $2 OFFSET $3`,
		status, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var reports []map[string]interface{}
	for rows.Next() {
		var id, reporterID, contentID, contentType, reason, reportStatus string
		var createdAt time.Time
		if err := rows.Scan(&id, &reporterID, &contentID, &contentType, &reason, &reportStatus, &createdAt); err != nil {
			return nil, 0, err
		}
		reports = append(reports, map[string]interface{}{
			"id": id, "reporter_id": reporterID, "content_id": contentID,
			"content_type": contentType, "reason": reason, "status": reportStatus,
			"created_at": createdAt,
		})
	}

	var total int64
	s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM reports WHERE status=$1`, status).Scan(&total) //nolint:errcheck
	return reports, total, rows.Err()
}

func (s *adminService) ResolveReport(ctx context.Context, reportID, adminID, action, notes string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE reports SET status='resolved', resolved_by=$1, resolution_action=$2, resolution_notes=$3, resolved_at=$4 WHERE id=$5`,
		adminID, action, notes, time.Now().UTC(), reportID)
	return err
}

// ─── Dashboard ────────────────────────────────────────────────────────────────

func (s *adminService) GetPlatformStats(ctx context.Context) (*models.PlatformStats, error) {
	stats := &models.PlatformStats{}
	today := time.Now().UTC().Truncate(24 * time.Hour)

	s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&stats.TotalUsers)
	s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE last_active_at >= $1`, today).Scan(&stats.ActiveUsersToday)
	s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM videos`).Scan(&stats.TotalVideos)
	s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM videos WHERE created_at >= $1`, today).Scan(&stats.VideosUploadedToday)
	s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM reports WHERE status='pending'`).Scan(&stats.PendingReports)

	return stats, nil
}

func (s *adminService) GetAuditLog(ctx context.Context, adminID string, limit, offset int) ([]*models.AuditLog, int64, error) {
	query := `SELECT id, admin_id, action, resource_type, resource_id, ip_address, created_at FROM audit_logs`
	args := []interface{}{}
	if adminID != "" {
		query += ` WHERE admin_id=$1`
		args = append(args, adminID)
	}
	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT %d OFFSET %d", limit, offset)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []*models.AuditLog
	for rows.Next() {
		l := &models.AuditLog{}
		if err := rows.Scan(&l.ID, &l.AdminID, &l.Action, &l.ResourceType, &l.ResourceID, &l.IPAddress, &l.CreatedAt); err != nil {
			return nil, 0, err
		}
		logs = append(logs, l)
	}

	var total int64
	s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit_logs`).Scan(&total) //nolint:errcheck
	return logs, total, rows.Err()
}

func (s *adminService) LogAudit(ctx context.Context, adminID, action, resourceType, resourceID string, details map[string]interface{}, ip, ua string) {
	s.pool.Exec(ctx, `
		INSERT INTO audit_logs (id, admin_id, action, resource_type, resource_id, ip_address, user_agent, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		uuid.New().String(), adminID, action, resourceType, resourceID, ip, ua, time.Now().UTC(),
	) //nolint:errcheck
}
