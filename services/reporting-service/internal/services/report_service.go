package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/tiktok-clone/reporting-service/internal/models"
)

var ErrReportNotFound = errors.New("report not found")

// CreateReportReq carries the parameters for submitting a report.
type CreateReportReq struct {
	ReporterID  string
	ContentID   string
	ContentType models.ReportType
	Reason      models.ReportReason
	Description string
}

// ReportService defines all report business operations.
type ReportService interface {
	CreateReport(ctx context.Context, req CreateReportReq) (*models.Report, error)
	GetReport(ctx context.Context, id string) (*models.Report, error)
	ListReports(ctx context.Context, status models.ReportStatus, contentType models.ReportType, limit, offset int) ([]*models.Report, int64, error)
	ResolveReport(ctx context.Context, id, adminID string, status models.ReportStatus, action, notes string) error
}

type reportService struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

// NewReportService creates a ReportService backed by PostgreSQL.
func NewReportService(pool *pgxpool.Pool, logger *zap.Logger) ReportService {
	return &reportService{pool: pool, logger: logger}
}

func (s *reportService) CreateReport(ctx context.Context, req CreateReportReq) (*models.Report, error) {
	report := &models.Report{
		ID:          uuid.New().String(),
		ReporterID:  req.ReporterID,
		ContentID:   req.ContentID,
		ContentType: req.ContentType,
		Reason:      req.Reason,
		Description: req.Description,
		Status:      models.ReportStatusPending,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	_, err := s.pool.Exec(ctx, `
		INSERT INTO reports (id, reporter_id, content_id, content_type, reason, description, status, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		report.ID, report.ReporterID, report.ContentID, report.ContentType,
		report.Reason, report.Description, report.Status,
		report.CreatedAt, report.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting report: %w", err)
	}

	s.logger.Info("report created",
		zap.String("id", report.ID),
		zap.String("content_type", string(report.ContentType)),
		zap.String("reason", string(report.Reason)),
	)
	return report, nil
}

func (s *reportService) GetReport(ctx context.Context, id string) (*models.Report, error) {
	r := &models.Report{}
	err := s.pool.QueryRow(ctx, `
		SELECT id, reporter_id, content_id, content_type, reason,
		       COALESCE(description,''), status,
		       COALESCE(resolved_by,''), COALESCE(resolution_action,''), COALESCE(resolution_notes,''),
		       resolved_at, created_at, updated_at
		FROM reports WHERE id=$1`, id,
	).Scan(
		&r.ID, &r.ReporterID, &r.ContentID, &r.ContentType, &r.Reason,
		&r.Description, &r.Status, &r.ResolvedBy, &r.ResolutionAction, &r.ResolutionNotes,
		&r.ResolvedAt, &r.CreatedAt, &r.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrReportNotFound
	}
	return r, err
}

func (s *reportService) ListReports(ctx context.Context, status models.ReportStatus, contentType models.ReportType, limit, offset int) ([]*models.Report, int64, error) {
	if limit > 100 {
		limit = 100
	}

	query := `SELECT id, reporter_id, content_id, content_type, reason,
	                 COALESCE(description,''), status, created_at, updated_at
	          FROM reports WHERE status=$1`
	args := []interface{}{status}

	if contentType != "" {
		query += ` AND content_type=$2`
		args = append(args, contentType)
		query += fmt.Sprintf(" ORDER BY created_at ASC LIMIT %d OFFSET %d", limit, offset)
	} else {
		query += fmt.Sprintf(" ORDER BY created_at ASC LIMIT %d OFFSET %d", limit, offset)
	}

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var reports []*models.Report
	for rows.Next() {
		r := &models.Report{}
		if err := rows.Scan(&r.ID, &r.ReporterID, &r.ContentID, &r.ContentType,
			&r.Reason, &r.Description, &r.Status, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, 0, err
		}
		reports = append(reports, r)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	var total int64
	s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM reports WHERE status=$1`, status).Scan(&total) //nolint:errcheck
	return reports, total, nil
}

func (s *reportService) ResolveReport(ctx context.Context, id, adminID string, status models.ReportStatus, action, notes string) error {
	now := time.Now().UTC()
	tag, err := s.pool.Exec(ctx, `
		UPDATE reports
		SET status=$1, resolved_by=$2, resolution_action=$3, resolution_notes=$4, resolved_at=$5, updated_at=$6
		WHERE id=$7`,
		status, adminID, action, notes, now, now, id,
	)
	if err != nil {
		return fmt.Errorf("resolving report: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrReportNotFound
	}
	return nil
}
