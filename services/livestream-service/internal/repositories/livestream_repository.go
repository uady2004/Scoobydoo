package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tiktok-clone/livestream-service/internal/models"
)

// LivestreamRepository defines all persistence operations for the livestream domain.
type LivestreamRepository interface {
	// Stream CRUD
	CreateStream(ctx context.Context, s *models.LiveStream) error
	GetStreamByID(ctx context.Context, id string) (*models.LiveStream, error)
	GetStreamByRTMPKey(ctx context.Context, rtmpKey string) (*models.LiveStream, error)
	UpdateStream(ctx context.Context, s *models.LiveStream) error
	UpdateStreamStatus(ctx context.Context, id string, status models.StreamStatus) error
	UpdateStreamViewerCount(ctx context.Context, id string, count int64) error
	IncrementGiftCoins(ctx context.Context, streamID string, coins int64) error
	GetActiveStreams(ctx context.Context, limit, offset int) ([]*models.LiveStream, error)
	GetStreamsByUserID(ctx context.Context, userID string, limit, offset int) ([]*models.LiveStream, error)
	DeleteStream(ctx context.Context, id string) error

	// Viewer CRUD
	UpsertViewer(ctx context.Context, v *models.LiveViewer) error
	GetViewer(ctx context.Context, streamID, userID string) (*models.LiveViewer, error)
	UpdateViewerStatus(ctx context.Context, streamID, userID string, status models.ViewerStatus, leftAt *time.Time, watchSecs int64) error
	GetActiveViewers(ctx context.Context, streamID string, limit int) ([]*models.LiveViewer, error)
	CountActiveViewers(ctx context.Context, streamID string) (int64, error)

	// Gift CRUD
	CreateGift(ctx context.Context, g *models.Gift) error
	GetGiftsByStream(ctx context.Context, streamID string, limit, offset int) ([]*models.Gift, error)
	GetGiftsByUser(ctx context.Context, senderID string, limit, offset int) ([]*models.Gift, error)
	SumGiftCoinsForStream(ctx context.Context, streamID string) (int64, error)

	// Poll CRUD
	CreatePoll(ctx context.Context, p *models.Poll) error
	GetPollByID(ctx context.Context, id string) (*models.Poll, error)
	GetActivePollForStream(ctx context.Context, streamID string) (*models.Poll, error)
	ClosePoll(ctx context.Context, id string) error
	CreatePollVote(ctx context.Context, v *models.PollVote) error
	HasUserVoted(ctx context.Context, pollID, userID string) (bool, error)
	IncrementPollOptionVote(ctx context.Context, pollID, optionID string) error
	GetPollOptions(ctx context.Context, pollID string) ([]models.PollOption, error)
	GetPollResults(ctx context.Context, pollID string) (*models.Poll, error)

	// PKBattle CRUD
	CreatePKBattle(ctx context.Context, b *models.PKBattle) error
	GetPKBattleByID(ctx context.Context, id string) (*models.PKBattle, error)
	GetActivePKBattleForStream(ctx context.Context, streamID string) (*models.PKBattle, error)
	UpdatePKBattleStatus(ctx context.Context, id string, status models.BattleStatus) error
	UpdatePKBattleScores(ctx context.Context, id string, initiatorScore, targetScore int64) error
	EndPKBattle(ctx context.Context, id string, winnerID string, initiatorScore, targetScore int64) error

	// CoHost CRUD
	CreateCoHost(ctx context.Context, ch *models.CoHost) error
	GetCoHostByID(ctx context.Context, id string) (*models.CoHost, error)
	GetCoHostsForStream(ctx context.Context, streamID string) ([]*models.CoHost, error)
	UpdateCoHostStatus(ctx context.Context, id string, status models.CoHostStatus) error
	RemoveCoHost(ctx context.Context, streamID, coHostID string) error
}

// pgLivestreamRepository is the PostgreSQL implementation.
type pgLivestreamRepository struct {
	pool *pgxpool.Pool
}

// NewLivestreamRepository creates a new PostgreSQL-backed repository.
func NewLivestreamRepository(pool *pgxpool.Pool) LivestreamRepository {
	return &pgLivestreamRepository{pool: pool}
}

// ─── Stream CRUD ────────────────────────────────────────────────────────────

func (r *pgLivestreamRepository) CreateStream(ctx context.Context, s *models.LiveStream) error {
	tags, _ := json.Marshal(s.Tags)
	_, err := r.pool.Exec(ctx, `
		INSERT INTO livestreams (
			id, user_id, title, description, rtmp_key, hls_playlist_url,
			thumbnail_url, status, viewer_count, peak_viewer_count,
			total_gift_coins, category_id, tags, is_recorded, language,
			age_restricted, allow_comments, created_at, updated_at
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19
		)`,
		s.ID, s.UserID, s.Title, s.Description, s.RTMPKey, s.HLSPlaylistURL,
		s.ThumbnailURL, s.Status, s.ViewerCount, s.PeakViewerCount,
		s.TotalGiftCoins, s.CategoryID, tags, s.IsRecorded, s.Language,
		s.AgeRestricted, s.AllowComments, s.CreatedAt, s.UpdatedAt,
	)
	return err
}

func (r *pgLivestreamRepository) GetStreamByID(ctx context.Context, id string) (*models.LiveStream, error) {
	return r.scanStream(ctx, `SELECT * FROM livestreams WHERE id = $1`, id)
}

func (r *pgLivestreamRepository) GetStreamByRTMPKey(ctx context.Context, rtmpKey string) (*models.LiveStream, error) {
	return r.scanStream(ctx, `SELECT * FROM livestreams WHERE rtmp_key = $1`, rtmpKey)
}

func (r *pgLivestreamRepository) UpdateStream(ctx context.Context, s *models.LiveStream) error {
	tags, _ := json.Marshal(s.Tags)
	s.UpdatedAt = time.Now()
	_, err := r.pool.Exec(ctx, `
		UPDATE livestreams SET
			title=$2, description=$3, thumbnail_url=$4, status=$5,
			hls_playlist_url=$6, category_id=$7, tags=$8, is_recorded=$9,
			language=$10, age_restricted=$11, allow_comments=$12,
			pk_battle_id=$13, started_at=$14, ended_at=$15, updated_at=$16
		WHERE id=$1`,
		s.ID, s.Title, s.Description, s.ThumbnailURL, s.Status,
		s.HLSPlaylistURL, s.CategoryID, tags, s.IsRecorded,
		s.Language, s.AgeRestricted, s.AllowComments,
		s.PKBattleID, s.StartedAt, s.EndedAt, s.UpdatedAt,
	)
	return err
}

func (r *pgLivestreamRepository) UpdateStreamStatus(ctx context.Context, id string, status models.StreamStatus) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE livestreams SET status=$2, updated_at=NOW() WHERE id=$1`,
		id, status,
	)
	return err
}

func (r *pgLivestreamRepository) UpdateStreamViewerCount(ctx context.Context, id string, count int64) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE livestreams
		SET viewer_count = $2,
		    peak_viewer_count = GREATEST(peak_viewer_count, $2),
		    updated_at = NOW()
		WHERE id = $1`, id, count,
	)
	return err
}

func (r *pgLivestreamRepository) IncrementGiftCoins(ctx context.Context, streamID string, coins int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE livestreams SET total_gift_coins = total_gift_coins + $2, updated_at = NOW() WHERE id = $1`,
		streamID, coins,
	)
	return err
}

func (r *pgLivestreamRepository) GetActiveStreams(ctx context.Context, limit, offset int) ([]*models.LiveStream, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT * FROM livestreams WHERE status = 'live' ORDER BY viewer_count DESC LIMIT $1 OFFSET $2`,
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	return r.collectStreams(rows)
}

func (r *pgLivestreamRepository) GetStreamsByUserID(ctx context.Context, userID string, limit, offset int) ([]*models.LiveStream, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT * FROM livestreams WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		userID, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	return r.collectStreams(rows)
}

func (r *pgLivestreamRepository) DeleteStream(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM livestreams WHERE id = $1`, id)
	return err
}

// ─── Viewer CRUD ────────────────────────────────────────────────────────────

func (r *pgLivestreamRepository) UpsertViewer(ctx context.Context, v *models.LiveViewer) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO live_viewers (
			id, stream_id, user_id, username, avatar_url, status,
			joined_at, is_follower, is_moderator
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (stream_id, user_id) DO UPDATE SET
			status = EXCLUDED.status,
			joined_at = EXCLUDED.joined_at,
			left_at = NULL,
			watch_duration_secs = 0`,
		v.ID, v.StreamID, v.UserID, v.Username, v.AvatarURL, v.Status,
		v.JoinedAt, v.IsFollower, v.IsModerator,
	)
	return err
}

func (r *pgLivestreamRepository) GetViewer(ctx context.Context, streamID, userID string) (*models.LiveViewer, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, stream_id, user_id, username, avatar_url, status,
		        joined_at, left_at, watch_duration_secs, is_follower, is_moderator
		 FROM live_viewers WHERE stream_id=$1 AND user_id=$2`,
		streamID, userID,
	)
	v := &models.LiveViewer{}
	err := row.Scan(
		&v.ID, &v.StreamID, &v.UserID, &v.Username, &v.AvatarURL, &v.Status,
		&v.JoinedAt, &v.LeftAt, &v.WatchDurationSecs, &v.IsFollower, &v.IsModerator,
	)
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (r *pgLivestreamRepository) UpdateViewerStatus(ctx context.Context, streamID, userID string, status models.ViewerStatus, leftAt *time.Time, watchSecs int64) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE live_viewers
		SET status=$3, left_at=$4, watch_duration_secs=$5
		WHERE stream_id=$1 AND user_id=$2`,
		streamID, userID, status, leftAt, watchSecs,
	)
	return err
}

func (r *pgLivestreamRepository) GetActiveViewers(ctx context.Context, streamID string, limit int) ([]*models.LiveViewer, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, stream_id, user_id, username, avatar_url, status,
		       joined_at, left_at, watch_duration_secs, is_follower, is_moderator
		FROM live_viewers
		WHERE stream_id=$1 AND status='joined'
		ORDER BY joined_at ASC
		LIMIT $2`,
		streamID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var viewers []*models.LiveViewer
	for rows.Next() {
		v := &models.LiveViewer{}
		if err := rows.Scan(
			&v.ID, &v.StreamID, &v.UserID, &v.Username, &v.AvatarURL, &v.Status,
			&v.JoinedAt, &v.LeftAt, &v.WatchDurationSecs, &v.IsFollower, &v.IsModerator,
		); err != nil {
			return nil, err
		}
		viewers = append(viewers, v)
	}
	return viewers, rows.Err()
}

func (r *pgLivestreamRepository) CountActiveViewers(ctx context.Context, streamID string) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM live_viewers WHERE stream_id=$1 AND status='joined'`,
		streamID,
	).Scan(&count)
	return count, err
}

// ─── Gift CRUD ──────────────────────────────────────────────────────────────

func (r *pgLivestreamRepository) CreateGift(ctx context.Context, g *models.Gift) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO gifts (
			id, stream_id, sender_id, sender_name, receiver_id,
			gift_type_id, gift_name, animation_url, icon_url,
			coin_cost, quantity, total_coins, is_combo, combo_count, created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
		g.ID, g.StreamID, g.SenderID, g.SenderName, g.ReceiverID,
		g.GiftTypeID, g.GiftName, g.AnimationURL, g.IconURL,
		g.CoinCost, g.Quantity, g.TotalCoins, g.IsCombo, g.ComboCount, g.CreatedAt,
	)
	return err
}

func (r *pgLivestreamRepository) GetGiftsByStream(ctx context.Context, streamID string, limit, offset int) ([]*models.Gift, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, stream_id, sender_id, sender_name, receiver_id,
		       gift_type_id, gift_name, animation_url, icon_url,
		       coin_cost, quantity, total_coins, is_combo, combo_count, created_at
		FROM gifts WHERE stream_id=$1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		streamID, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	return r.collectGifts(rows)
}

func (r *pgLivestreamRepository) GetGiftsByUser(ctx context.Context, senderID string, limit, offset int) ([]*models.Gift, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, stream_id, sender_id, sender_name, receiver_id,
		       gift_type_id, gift_name, animation_url, icon_url,
		       coin_cost, quantity, total_coins, is_combo, combo_count, created_at
		FROM gifts WHERE sender_id=$1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		senderID, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	return r.collectGifts(rows)
}

func (r *pgLivestreamRepository) SumGiftCoinsForStream(ctx context.Context, streamID string) (int64, error) {
	var total int64
	err := r.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(total_coins), 0) FROM gifts WHERE stream_id=$1`,
		streamID,
	).Scan(&total)
	return total, err
}

// ─── Poll CRUD ──────────────────────────────────────────────────────────────

func (r *pgLivestreamRepository) CreatePoll(ctx context.Context, p *models.Poll) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	_, err = tx.Exec(ctx, `
		INSERT INTO polls (id, stream_id, creator_id, question, status, duration_secs, total_votes, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,0,$7)`,
		p.ID, p.StreamID, p.CreatorID, p.Question, p.Status, p.DurationSecs, p.CreatedAt,
	)
	if err != nil {
		return err
	}
	for _, opt := range p.Options {
		_, err = tx.Exec(ctx,
			`INSERT INTO poll_options (id, poll_id, text, vote_count) VALUES ($1,$2,$3,0)`,
			opt.ID, p.ID, opt.Text,
		)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *pgLivestreamRepository) GetPollByID(ctx context.Context, id string) (*models.Poll, error) {
	p := &models.Poll{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, stream_id, creator_id, question, status, duration_secs, total_votes, created_at, closed_at
		FROM polls WHERE id=$1`, id,
	).Scan(&p.ID, &p.StreamID, &p.CreatorID, &p.Question, &p.Status, &p.DurationSecs, &p.TotalVotes, &p.CreatedAt, &p.ClosedAt)
	if err != nil {
		return nil, err
	}
	opts, err := r.GetPollOptions(ctx, id)
	if err != nil {
		return nil, err
	}
	p.Options = opts
	return p, nil
}

func (r *pgLivestreamRepository) GetActivePollForStream(ctx context.Context, streamID string) (*models.Poll, error) {
	p := &models.Poll{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, stream_id, creator_id, question, status, duration_secs, total_votes, created_at, closed_at
		FROM polls WHERE stream_id=$1 AND status='active' ORDER BY created_at DESC LIMIT 1`, streamID,
	).Scan(&p.ID, &p.StreamID, &p.CreatorID, &p.Question, &p.Status, &p.DurationSecs, &p.TotalVotes, &p.CreatedAt, &p.ClosedAt)
	if err != nil {
		return nil, err
	}
	opts, err := r.GetPollOptions(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	p.Options = opts
	return p, nil
}

func (r *pgLivestreamRepository) ClosePoll(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE polls SET status='closed', closed_at=NOW() WHERE id=$1`, id,
	)
	return err
}

func (r *pgLivestreamRepository) CreatePollVote(ctx context.Context, v *models.PollVote) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO poll_votes (id, poll_id, option_id, user_id, voted_at) VALUES ($1,$2,$3,$4,$5)`,
		v.ID, v.PollID, v.OptionID, v.UserID, v.VotedAt,
	)
	return err
}

func (r *pgLivestreamRepository) HasUserVoted(ctx context.Context, pollID, userID string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM poll_votes WHERE poll_id=$1 AND user_id=$2)`,
		pollID, userID,
	).Scan(&exists)
	return exists, err
}

func (r *pgLivestreamRepository) IncrementPollOptionVote(ctx context.Context, pollID, optionID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	_, err = tx.Exec(ctx,
		`UPDATE poll_options SET vote_count = vote_count + 1 WHERE id=$1 AND poll_id=$2`,
		optionID, pollID,
	)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx,
		`UPDATE polls SET total_votes = total_votes + 1 WHERE id=$1`,
		pollID,
	)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *pgLivestreamRepository) GetPollOptions(ctx context.Context, pollID string) ([]models.PollOption, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, poll_id, text, vote_count FROM poll_options WHERE poll_id=$1 ORDER BY id`,
		pollID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var opts []models.PollOption
	for rows.Next() {
		var o models.PollOption
		if err := rows.Scan(&o.ID, &o.PollID, &o.Text, &o.VoteCount); err != nil {
			return nil, err
		}
		opts = append(opts, o)
	}
	return opts, rows.Err()
}

func (r *pgLivestreamRepository) GetPollResults(ctx context.Context, pollID string) (*models.Poll, error) {
	return r.GetPollByID(ctx, pollID)
}

// ─── PKBattle CRUD ───────────────────────────────────────────────────────────

func (r *pgLivestreamRepository) CreatePKBattle(ctx context.Context, b *models.PKBattle) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO pk_battles (
			id, initiator_id, initiator_name, target_id, target_name,
			stream_id, target_stream_id, status, initiator_score, target_score,
			duration_secs, created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,0,0,$9,$10)`,
		b.ID, b.InitiatorID, b.InitiatorName, b.TargetID, b.TargetName,
		b.StreamID, b.TargetStreamID, b.Status, b.DurationSecs, b.CreatedAt,
	)
	return err
}

func (r *pgLivestreamRepository) GetPKBattleByID(ctx context.Context, id string) (*models.PKBattle, error) {
	b := &models.PKBattle{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, initiator_id, initiator_name, target_id, target_name,
		       stream_id, target_stream_id, status, initiator_score, target_score,
		       winner_id, duration_secs, started_at, ended_at, created_at
		FROM pk_battles WHERE id=$1`, id,
	).Scan(
		&b.ID, &b.InitiatorID, &b.InitiatorName, &b.TargetID, &b.TargetName,
		&b.StreamID, &b.TargetStreamID, &b.Status, &b.InitiatorScore, &b.TargetScore,
		&b.WinnerID, &b.DurationSecs, &b.StartedAt, &b.EndedAt, &b.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (r *pgLivestreamRepository) GetActivePKBattleForStream(ctx context.Context, streamID string) (*models.PKBattle, error) {
	b := &models.PKBattle{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, initiator_id, initiator_name, target_id, target_name,
		       stream_id, target_stream_id, status, initiator_score, target_score,
		       winner_id, duration_secs, started_at, ended_at, created_at
		FROM pk_battles
		WHERE (stream_id=$1 OR target_stream_id=$1) AND status='active'
		LIMIT 1`, streamID,
	).Scan(
		&b.ID, &b.InitiatorID, &b.InitiatorName, &b.TargetID, &b.TargetName,
		&b.StreamID, &b.TargetStreamID, &b.Status, &b.InitiatorScore, &b.TargetScore,
		&b.WinnerID, &b.DurationSecs, &b.StartedAt, &b.EndedAt, &b.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (r *pgLivestreamRepository) UpdatePKBattleStatus(ctx context.Context, id string, status models.BattleStatus) error {
	query := `UPDATE pk_battles SET status=$2 WHERE id=$1`
	if status == models.BattleStatusActive {
		query = `UPDATE pk_battles SET status=$2, started_at=NOW() WHERE id=$1`
	}
	_, err := r.pool.Exec(ctx, query, id, status)
	return err
}

func (r *pgLivestreamRepository) UpdatePKBattleScores(ctx context.Context, id string, initiatorScore, targetScore int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE pk_battles SET initiator_score=$2, target_score=$3 WHERE id=$1`,
		id, initiatorScore, targetScore,
	)
	return err
}

func (r *pgLivestreamRepository) EndPKBattle(ctx context.Context, id string, winnerID string, initiatorScore, targetScore int64) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE pk_battles
		SET status='ended', winner_id=$2, initiator_score=$3, target_score=$4, ended_at=NOW()
		WHERE id=$1`,
		id, winnerID, initiatorScore, targetScore,
	)
	return err
}

// ─── CoHost CRUD ─────────────────────────────────────────────────────────────

func (r *pgLivestreamRepository) CreateCoHost(ctx context.Context, ch *models.CoHost) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO co_hosts (id, stream_id, host_id, co_host_id, co_host_name, status, invited_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		ch.ID, ch.StreamID, ch.HostID, ch.CoHostID, ch.CoHostName, ch.Status, ch.InvitedAt,
	)
	return err
}

func (r *pgLivestreamRepository) GetCoHostByID(ctx context.Context, id string) (*models.CoHost, error) {
	ch := &models.CoHost{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, stream_id, host_id, co_host_id, co_host_name, status,
		       webrtc_session_id, invited_at, accepted_at, removed_at
		FROM co_hosts WHERE id=$1`, id,
	).Scan(&ch.ID, &ch.StreamID, &ch.HostID, &ch.CoHostID, &ch.CoHostName, &ch.Status,
		&ch.WebRTCSessionID, &ch.InvitedAt, &ch.AcceptedAt, &ch.RemovedAt)
	if err != nil {
		return nil, err
	}
	return ch, nil
}

func (r *pgLivestreamRepository) GetCoHostsForStream(ctx context.Context, streamID string) ([]*models.CoHost, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, stream_id, host_id, co_host_id, co_host_name, status,
		       webrtc_session_id, invited_at, accepted_at, removed_at
		FROM co_hosts WHERE stream_id=$1 AND status IN ('pending','accepted')
		ORDER BY invited_at`, streamID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var hosts []*models.CoHost
	for rows.Next() {
		ch := &models.CoHost{}
		if err := rows.Scan(&ch.ID, &ch.StreamID, &ch.HostID, &ch.CoHostID, &ch.CoHostName, &ch.Status,
			&ch.WebRTCSessionID, &ch.InvitedAt, &ch.AcceptedAt, &ch.RemovedAt); err != nil {
			return nil, err
		}
		hosts = append(hosts, ch)
	}
	return hosts, rows.Err()
}

func (r *pgLivestreamRepository) UpdateCoHostStatus(ctx context.Context, id string, status models.CoHostStatus) error {
	var query string
	switch status {
	case models.CoHostStatusAccepted:
		query = `UPDATE co_hosts SET status=$2, accepted_at=NOW() WHERE id=$1`
	case models.CoHostStatusRemoved:
		query = `UPDATE co_hosts SET status=$2, removed_at=NOW() WHERE id=$1`
	default:
		query = `UPDATE co_hosts SET status=$2 WHERE id=$1`
	}
	_, err := r.pool.Exec(ctx, query, id, status)
	return err
}

func (r *pgLivestreamRepository) RemoveCoHost(ctx context.Context, streamID, coHostID string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE co_hosts SET status='removed', removed_at=NOW()
		WHERE stream_id=$1 AND co_host_id=$2`,
		streamID, coHostID,
	)
	return err
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func (r *pgLivestreamRepository) scanStream(ctx context.Context, query string, args ...interface{}) (*models.LiveStream, error) {
	row := r.pool.QueryRow(ctx, query, args...)
	return r.scanStreamRow(row)
}

func (r *pgLivestreamRepository) scanStreamRow(row pgx.Row) (*models.LiveStream, error) {
	s := &models.LiveStream{}
	var tagsJSON []byte
	err := row.Scan(
		&s.ID, &s.UserID, &s.Title, &s.Description, &s.RTMPKey,
		&s.HLSPlaylistURL, &s.ThumbnailURL, &s.Status,
		&s.ViewerCount, &s.PeakViewerCount, &s.TotalGiftCoins,
		&s.CategoryID, &tagsJSON, &s.IsRecorded, &s.RecordingURL,
		&s.Language, &s.AgeRestricted, &s.AllowComments,
		&s.PKBattleID, &s.StartedAt, &s.EndedAt, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if len(tagsJSON) > 0 {
		_ = json.Unmarshal(tagsJSON, &s.Tags)
	}
	return s, nil
}

func (r *pgLivestreamRepository) collectStreams(rows pgx.Rows) ([]*models.LiveStream, error) {
	defer rows.Close()
	var streams []*models.LiveStream
	for rows.Next() {
		s := &models.LiveStream{}
		var tagsJSON []byte
		if err := rows.Scan(
			&s.ID, &s.UserID, &s.Title, &s.Description, &s.RTMPKey,
			&s.HLSPlaylistURL, &s.ThumbnailURL, &s.Status,
			&s.ViewerCount, &s.PeakViewerCount, &s.TotalGiftCoins,
			&s.CategoryID, &tagsJSON, &s.IsRecorded, &s.RecordingURL,
			&s.Language, &s.AgeRestricted, &s.AllowComments,
			&s.PKBattleID, &s.StartedAt, &s.EndedAt, &s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning stream row: %w", err)
		}
		if len(tagsJSON) > 0 {
			_ = json.Unmarshal(tagsJSON, &s.Tags)
		}
		streams = append(streams, s)
	}
	return streams, rows.Err()
}

func (r *pgLivestreamRepository) collectGifts(rows pgx.Rows) ([]*models.Gift, error) {
	defer rows.Close()
	var gifts []*models.Gift
	for rows.Next() {
		g := &models.Gift{}
		if err := rows.Scan(
			&g.ID, &g.StreamID, &g.SenderID, &g.SenderName, &g.ReceiverID,
			&g.GiftTypeID, &g.GiftName, &g.AnimationURL, &g.IconURL,
			&g.CoinCost, &g.Quantity, &g.TotalCoins, &g.IsCombo, &g.ComboCount, &g.CreatedAt,
		); err != nil {
			return nil, err
		}
		gifts = append(gifts, g)
	}
	return gifts, rows.Err()
}
