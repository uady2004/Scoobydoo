package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/tiktok-clone/social-graph-service/internal/models"
)

// ---------------------------------------------------------------------------
// Sentinel errors
// ---------------------------------------------------------------------------

// ErrAlreadyFollowing is returned when a follow relationship already exists.
var ErrAlreadyFollowing = errors.New("already following")

// ErrNotFollowing is returned when an unfollow is attempted but no follow edge exists.
var ErrNotFollowing = errors.New("not following")

// ErrSelfFollow is returned when a user attempts to follow themselves.
var ErrSelfFollow = errors.New("cannot follow yourself")

// ErrUserBlocked is returned when the action is blocked due to a block relationship
// in either direction.
var ErrUserBlocked = errors.New("user is blocked")

// ---------------------------------------------------------------------------
// Repository interface
// ---------------------------------------------------------------------------

// GraphRepository defines the data-access contract for the social graph.
// All methods accept a context for cancellation and timeout propagation.
type GraphRepository interface {
	// Follow creates a directed follow edge followerID → followeeID.
	// Returns ErrSelfFollow, ErrAlreadyFollowing, or ErrUserBlocked as
	// appropriate.
	Follow(ctx context.Context, followerID, followeeID string) (*models.Follow, error)

	// Unfollow removes the follow edge followerID → followeeID.
	// Returns ErrNotFollowing when the edge does not exist.
	Unfollow(ctx context.Context, followerID, followeeID string) error

	// GetFollowers returns users who follow userID, newest first.
	// The second return value is the total count ignoring pagination.
	GetFollowers(ctx context.Context, userID string, limit, offset int) ([]models.Follow, int64, error)

	// GetFollowing returns users that userID follows, newest first.
	GetFollowing(ctx context.Context, userID string, limit, offset int) ([]models.Follow, int64, error)

	// GetMutualFollowers returns users who follow both userID and targetID.
	GetMutualFollowers(ctx context.Context, userID, targetID string, limit, offset int) ([]models.Follow, int64, error)

	// IsFollowing checks whether followerID currently follows followeeID.
	IsFollowing(ctx context.Context, followerID, followeeID string) (bool, error)

	// GetFollowerCount returns the total number of followers for userID.
	GetFollowerCount(ctx context.Context, userID string) (int64, error)

	// GetFollowingCount returns the total number of users that userID follows.
	GetFollowingCount(ctx context.Context, userID string) (int64, error)

	// BlockUser creates a block from blockerID to blockedID and removes any
	// follow edges between them in a single transaction.
	BlockUser(ctx context.Context, blockerID, blockedID string) error

	// UnblockUser removes a block relationship.
	UnblockUser(ctx context.Context, blockerID, blockedID string) error

	// GetBlockList returns all users that userID has blocked, newest first.
	GetBlockList(ctx context.Context, userID string, limit, offset int) ([]models.Block, int64, error)

	// IsBlocked returns true when blockerID has blocked blockedID.
	IsBlocked(ctx context.Context, blockerID, blockedID string) (bool, error)

	// GetFollowingIDs returns all user IDs that userID follows.
	// Used by the BFS suggestion algorithm; does not paginate.
	GetFollowingIDs(ctx context.Context, userID string) ([]string, error)

	// GetBlockedIDs returns all user IDs that userID has blocked or has been
	// blocked by (symmetric). Used to filter suggestion candidates.
	GetBlockedIDs(ctx context.Context, userID string) ([]string, error)
}

// ---------------------------------------------------------------------------
// PostgreSQL implementation
// ---------------------------------------------------------------------------

// pgGraphRepository is the PostgreSQL implementation of GraphRepository.
type pgGraphRepository struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

// NewGraphRepository creates a new PostgreSQL-backed GraphRepository.
func NewGraphRepository(pool *pgxpool.Pool, logger *zap.Logger) GraphRepository {
	return &pgGraphRepository{
		pool:   pool,
		logger: logger,
	}
}

// ---------------------------------------------------------------------------
// Follow / Unfollow
// ---------------------------------------------------------------------------

// Follow inserts a directed follow edge.
//
// Schema assumed:
//
//	CREATE TABLE follows (
//	    id          BIGSERIAL PRIMARY KEY,
//	    follower_id UUID        NOT NULL,
//	    followee_id UUID        NOT NULL,
//	    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
//	    UNIQUE (follower_id, followee_id)
//	);
func (r *pgGraphRepository) Follow(ctx context.Context, followerID, followeeID string) (*models.Follow, error) {
	if followerID == followeeID {
		return nil, ErrSelfFollow
	}

	// Check for a block in either direction before inserting.
	blocked, err := r.IsBlocked(ctx, followerID, followeeID)
	if err != nil {
		return nil, fmt.Errorf("follow: block check (forward): %w", err)
	}
	if blocked {
		return nil, ErrUserBlocked
	}

	blocked, err = r.IsBlocked(ctx, followeeID, followerID)
	if err != nil {
		return nil, fmt.Errorf("follow: block check (reverse): %w", err)
	}
	if blocked {
		return nil, ErrUserBlocked
	}

	const q = `
		INSERT INTO follows (follower_id, followee_id, created_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (follower_id, followee_id) DO NOTHING
		RETURNING id, follower_id, followee_id, created_at`

	var f models.Follow
	err = r.pool.QueryRow(ctx, q, followerID, followeeID, time.Now().UTC()).
		Scan(&f.ID, &f.FollowerID, &f.FolloweeID, &f.CreatedAt)
	if err != nil {
		// ON CONFLICT DO NOTHING returns no rows when the edge already exists.
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAlreadyFollowing
		}
		return nil, fmt.Errorf("follow: insert: %w", err)
	}

	r.logger.Info("follow edge created",
		zap.String("follower_id", followerID),
		zap.String("followee_id", followeeID),
		zap.Int64("id", f.ID),
	)
	return &f, nil
}

// Unfollow removes a follow edge. Returns ErrNotFollowing when absent.
func (r *pgGraphRepository) Unfollow(ctx context.Context, followerID, followeeID string) error {
	const q = `DELETE FROM follows WHERE follower_id = $1 AND followee_id = $2`

	tag, err := r.pool.Exec(ctx, q, followerID, followeeID)
	if err != nil {
		return fmt.Errorf("unfollow: delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFollowing
	}

	r.logger.Info("follow edge removed",
		zap.String("follower_id", followerID),
		zap.String("followee_id", followeeID),
	)
	return nil
}

// ---------------------------------------------------------------------------
// List operations
// ---------------------------------------------------------------------------

// GetFollowers returns the users who follow userID, newest first.
func (r *pgGraphRepository) GetFollowers(ctx context.Context, userID string, limit, offset int) ([]models.Follow, int64, error) {
	const countQ = `SELECT COUNT(*) FROM follows WHERE followee_id = $1`
	var total int64
	if err := r.pool.QueryRow(ctx, countQ, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("get_followers: count: %w", err)
	}

	const q = `
		SELECT id, follower_id, followee_id, created_at
		FROM follows
		WHERE followee_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.pool.Query(ctx, q, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("get_followers: query: %w", err)
	}
	defer rows.Close()

	follows, err := scanFollows(rows)
	if err != nil {
		return nil, 0, fmt.Errorf("get_followers: scan: %w", err)
	}
	return follows, total, nil
}

// GetFollowing returns the users that userID follows, newest first.
func (r *pgGraphRepository) GetFollowing(ctx context.Context, userID string, limit, offset int) ([]models.Follow, int64, error) {
	const countQ = `SELECT COUNT(*) FROM follows WHERE follower_id = $1`
	var total int64
	if err := r.pool.QueryRow(ctx, countQ, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("get_following: count: %w", err)
	}

	const q = `
		SELECT id, follower_id, followee_id, created_at
		FROM follows
		WHERE follower_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.pool.Query(ctx, q, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("get_following: query: %w", err)
	}
	defer rows.Close()

	follows, err := scanFollows(rows)
	if err != nil {
		return nil, 0, fmt.Errorf("get_following: scan: %w", err)
	}
	return follows, total, nil
}

// GetMutualFollowers returns users who follow both userID and targetID.
//
// A "mutual follower" of A and B is any user C such that:
//   - C follows A, AND
//   - C follows B, AND
//   - C is neither A nor B.
func (r *pgGraphRepository) GetMutualFollowers(ctx context.Context, userID, targetID string, limit, offset int) ([]models.Follow, int64, error) {
	const countQ = `
		SELECT COUNT(*)
		FROM   follows f1
		JOIN   follows f2 ON f1.follower_id = f2.follower_id
		WHERE  f1.followee_id = $1
		  AND  f2.followee_id = $2
		  AND  f1.follower_id != $1
		  AND  f1.follower_id != $2`

	var total int64
	if err := r.pool.QueryRow(ctx, countQ, userID, targetID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("get_mutual_followers: count: %w", err)
	}

	const q = `
		SELECT f1.id, f1.follower_id, f1.followee_id, f1.created_at
		FROM   follows f1
		JOIN   follows f2 ON f1.follower_id = f2.follower_id
		WHERE  f1.followee_id = $1
		  AND  f2.followee_id = $2
		  AND  f1.follower_id != $1
		  AND  f1.follower_id != $2
		ORDER BY f1.created_at DESC
		LIMIT  $3 OFFSET $4`

	rows, err := r.pool.Query(ctx, q, userID, targetID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("get_mutual_followers: query: %w", err)
	}
	defer rows.Close()

	follows, err := scanFollows(rows)
	if err != nil {
		return nil, 0, fmt.Errorf("get_mutual_followers: scan: %w", err)
	}
	return follows, total, nil
}

// ---------------------------------------------------------------------------
// Existence checks
// ---------------------------------------------------------------------------

// IsFollowing checks whether followerID currently follows followeeID.
func (r *pgGraphRepository) IsFollowing(ctx context.Context, followerID, followeeID string) (bool, error) {
	const q = `
		SELECT EXISTS (
			SELECT 1 FROM follows
			WHERE follower_id = $1 AND followee_id = $2
		)`
	var exists bool
	if err := r.pool.QueryRow(ctx, q, followerID, followeeID).Scan(&exists); err != nil {
		return false, fmt.Errorf("is_following: %w", err)
	}
	return exists, nil
}

// ---------------------------------------------------------------------------
// Counts
// ---------------------------------------------------------------------------

// GetFollowerCount returns the number of followers for userID.
func (r *pgGraphRepository) GetFollowerCount(ctx context.Context, userID string) (int64, error) {
	const q = `SELECT COUNT(*) FROM follows WHERE followee_id = $1`
	var count int64
	if err := r.pool.QueryRow(ctx, q, userID).Scan(&count); err != nil {
		return 0, fmt.Errorf("get_follower_count: %w", err)
	}
	return count, nil
}

// GetFollowingCount returns the number of users that userID follows.
func (r *pgGraphRepository) GetFollowingCount(ctx context.Context, userID string) (int64, error) {
	const q = `SELECT COUNT(*) FROM follows WHERE follower_id = $1`
	var count int64
	if err := r.pool.QueryRow(ctx, q, userID).Scan(&count); err != nil {
		return 0, fmt.Errorf("get_following_count: %w", err)
	}
	return count, nil
}

// ---------------------------------------------------------------------------
// Block operations
// ---------------------------------------------------------------------------

// BlockUser creates a block relationship and atomically removes any follow
// edges between the two users in both directions.
//
// Schema assumed:
//
//	CREATE TABLE blocks (
//	    id         BIGSERIAL PRIMARY KEY,
//	    blocker_id UUID        NOT NULL,
//	    blocked_id UUID        NOT NULL,
//	    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
//	    UNIQUE (blocker_id, blocked_id)
//	);
func (r *pgGraphRepository) BlockUser(ctx context.Context, blockerID, blockedID string) error {
	if blockerID == blockedID {
		return fmt.Errorf("cannot block yourself")
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("block_user: begin tx: %w", err)
	}
	// The named return variable allows the deferred rollback to inspect err.
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	// Insert block record — idempotent via ON CONFLICT DO NOTHING.
	const insertBlock = `
		INSERT INTO blocks (blocker_id, blocked_id, created_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (blocker_id, blocked_id) DO NOTHING`
	if _, err = tx.Exec(ctx, insertBlock, blockerID, blockedID, time.Now().UTC()); err != nil {
		return fmt.Errorf("block_user: insert block: %w", err)
	}

	// Remove follow edges in both directions so the blocked user disappears
	// from the blocker's feed and the blocker from the blocked user's feed.
	const deleteFollows = `
		DELETE FROM follows
		WHERE (follower_id = $1 AND followee_id = $2)
		   OR (follower_id = $2 AND followee_id = $1)`
	if _, err = tx.Exec(ctx, deleteFollows, blockerID, blockedID); err != nil {
		return fmt.Errorf("block_user: remove follow edges: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("block_user: commit: %w", err)
	}

	r.logger.Info("user blocked",
		zap.String("blocker_id", blockerID),
		zap.String("blocked_id", blockedID),
	)
	return nil
}

// UnblockUser removes a block relationship created by blockerID against blockedID.
func (r *pgGraphRepository) UnblockUser(ctx context.Context, blockerID, blockedID string) error {
	const q = `DELETE FROM blocks WHERE blocker_id = $1 AND blocked_id = $2`
	tag, err := r.pool.Exec(ctx, q, blockerID, blockedID)
	if err != nil {
		return fmt.Errorf("unblock_user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("not blocked")
	}
	r.logger.Info("user unblocked",
		zap.String("blocker_id", blockerID),
		zap.String("blocked_id", blockedID),
	)
	return nil
}

// GetBlockList returns all users that userID has blocked, newest first.
func (r *pgGraphRepository) GetBlockList(ctx context.Context, userID string, limit, offset int) ([]models.Block, int64, error) {
	const countQ = `SELECT COUNT(*) FROM blocks WHERE blocker_id = $1`
	var total int64
	if err := r.pool.QueryRow(ctx, countQ, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("get_block_list: count: %w", err)
	}

	const q = `
		SELECT id, blocker_id, blocked_id, created_at
		FROM   blocks
		WHERE  blocker_id = $1
		ORDER  BY created_at DESC
		LIMIT  $2 OFFSET $3`

	rows, err := r.pool.Query(ctx, q, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("get_block_list: query: %w", err)
	}
	defer rows.Close()

	var blocks []models.Block
	for rows.Next() {
		var b models.Block
		if err := rows.Scan(&b.ID, &b.BlockerID, &b.BlockedID, &b.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("get_block_list: scan: %w", err)
		}
		blocks = append(blocks, b)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("get_block_list: rows: %w", err)
	}
	return blocks, total, nil
}

// IsBlocked returns true when blockerID has blocked blockedID.
func (r *pgGraphRepository) IsBlocked(ctx context.Context, blockerID, blockedID string) (bool, error) {
	const q = `
		SELECT EXISTS (
			SELECT 1 FROM blocks
			WHERE blocker_id = $1 AND blocked_id = $2
		)`
	var exists bool
	if err := r.pool.QueryRow(ctx, q, blockerID, blockedID).Scan(&exists); err != nil {
		return false, fmt.Errorf("is_blocked: %w", err)
	}
	return exists, nil
}

// ---------------------------------------------------------------------------
// BFS helpers
// ---------------------------------------------------------------------------

// GetFollowingIDs returns all user IDs that userID follows. Unlike GetFollowing
// this does not paginate and is intended for the BFS suggestion algorithm.
// For high-cardinality users this may be slow; the recommendation is to cap
// the follow graph traversal in the caller.
func (r *pgGraphRepository) GetFollowingIDs(ctx context.Context, userID string) ([]string, error) {
	const q = `SELECT followee_id FROM follows WHERE follower_id = $1`
	rows, err := r.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("get_following_ids: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("get_following_ids: scan: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// GetBlockedIDs returns all user IDs that userID has blocked OR has been
// blocked by (symmetric union). Used to filter suggestion candidates.
func (r *pgGraphRepository) GetBlockedIDs(ctx context.Context, userID string) ([]string, error) {
	// UNION deduplicates automatically.
	const q = `
		SELECT blocked_id FROM blocks WHERE blocker_id = $1
		UNION
		SELECT blocker_id FROM blocks WHERE blocked_id = $1`

	rows, err := r.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("get_blocked_ids: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("get_blocked_ids: scan: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ---------------------------------------------------------------------------
// Internal scan helpers
// ---------------------------------------------------------------------------

// scanFollows collects Follow rows from a pgx.Rows cursor.
func scanFollows(rows pgx.Rows) ([]models.Follow, error) {
	var follows []models.Follow
	for rows.Next() {
		var f models.Follow
		if err := rows.Scan(&f.ID, &f.FollowerID, &f.FolloweeID, &f.CreatedAt); err != nil {
			return nil, err
		}
		follows = append(follows, f)
	}
	return follows, rows.Err()
}
