package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"komorebi/internal/domain/community"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ============================================================
// ContributionRepo
// ============================================================

// ContributionRepo implements community.ContributionRepository.
type ContributionRepo struct {
	pool *pgxpool.Pool
}

// NewContributionRepo creates a new ContributionRepo.
func NewContributionRepo(pool *pgxpool.Pool) *ContributionRepo {
	return &ContributionRepo{pool: pool}
}

// Create persists a new Contribution.
// Maps domain.RouteGeometry ([][3]float64) → PostGIS LINESTRING Z.
// ModeratorNotes are stored inside the metadata JSONB field.
func (r *ContributionRepo) Create(c *community.Contribution) error {
	ctx := context.Background()
	meta, err := json.Marshal(c.Metadata)
	if err != nil {
		return fmt.Errorf("ContributionRepo.Create marshal metadata: %w", err)
	}
	now := time.Now().UTC()
	if len(c.RouteGeometry) > 0 {
		wkt := coordsToWKT(c.RouteGeometry)
		_, err = r.pool.Exec(ctx, `
			INSERT INTO community.contribution
				(id, user_id, route_geometry, metadata, status, created_at, updated_at)
			VALUES ($1::uuid, $2::uuid, ST_GeomFromText($3, 4326), $4, $5::community.contribution_status, $6, $7)
		`, c.ID, c.UserID, wkt, meta, string(c.Status), now, now)
	} else {
		_, err = r.pool.Exec(ctx, `
			INSERT INTO community.contribution
				(id, user_id, metadata, status, created_at, updated_at)
			VALUES ($1::uuid, $2::uuid, $3, $4::community.contribution_status, $5, $6)
		`, c.ID, c.UserID, meta, string(c.Status), now, now)
	}
	if err != nil {
		return fmt.Errorf("ContributionRepo.Create: %w", err)
	}
	return nil
}

// ============================================================
// ReviewRepo
// ============================================================

// ReviewRepo implements community.ReviewRepository.
type ReviewRepo struct {
	pool *pgxpool.Pool
}

// NewReviewRepo creates a new ReviewRepo.
func NewReviewRepo(pool *pgxpool.Pool) *ReviewRepo {
	return &ReviewRepo{pool: pool}
}

// Create inserts a new Review.
func (r *ReviewRepo) Create(rev *community.Review) error {
	ctx := context.Background()
	now := time.Now().UTC()
	_, err := r.pool.Exec(ctx, `
		INSERT INTO community.review (id, user_id, route_id, rating, body, created_at, updated_at)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7)
	`, rev.ID, rev.UserID, rev.RouteID, rev.Rating, rev.Body, now, now)
	if err != nil {
		return fmt.Errorf("ReviewRepo.Create: %w", err)
	}
	return nil
}

// ListByRoute returns all reviews for the given route, newest first.
func (r *ReviewRepo) ListByRoute(routeID string) ([]*community.Review, error) {
	ctx := context.Background()
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, route_id, rating, COALESCE(body,''), created_at
		FROM community.review
		WHERE route_id = $1::uuid
		ORDER BY created_at DESC
	`, routeID)
	if err != nil {
		return nil, fmt.Errorf("ReviewRepo.ListByRoute: %w", err)
	}
	defer rows.Close()
	var reviews []*community.Review
	for rows.Next() {
		var rev community.Review
		if err := rows.Scan(&rev.ID, &rev.UserID, &rev.RouteID, &rev.Rating, &rev.Body, &rev.CreatedAt); err != nil {
			return nil, err
		}
		reviews = append(reviews, &rev)
	}
	if reviews == nil {
		reviews = []*community.Review{}
	}
	if err := rows.Err(); err != nil && !isNotFound(err) { // 22P02 surfaces here, not from Query
		return nil, err
	}
	return reviews, nil
}

// ============================================================
// RideLogRepo
// ============================================================

// RideLogRepo implements community.RideLogRepository.
// Schema mapping:
//
//	domain.RiddenAt  (unix int)  ↔ DB started_at (TIMESTAMPTZ)
//	domain.DurationS (int secs)  ↔ derived: finished_at = started_at + duration_s * interval
type RideLogRepo struct {
	pool *pgxpool.Pool
}

// NewRideLogRepo creates a new RideLogRepo.
func NewRideLogRepo(pool *pgxpool.Pool) *RideLogRepo {
	return &RideLogRepo{pool: pool}
}

// Create inserts a RideLog, converting unix timestamps to TIMESTAMPTZ.
func (r *RideLogRepo) Create(rl *community.RideLog) error {
	ctx := context.Background()
	startedAt := time.Unix(int64(rl.RiddenAt), 0).UTC()
	finishedAt := startedAt.Add(time.Duration(rl.DurationS) * time.Second)
	now := time.Now().UTC()

	var err error
	if len(rl.GPXTrack) > 0 {
		wkt := coordsToWKT(rl.GPXTrack)
		_, err = r.pool.Exec(ctx, `
			INSERT INTO community.ride_log
				(id, user_id, route_id, gpx_track, started_at, finished_at, created_at)
			VALUES ($1::uuid, $2::uuid, $3::uuid, ST_GeomFromText($4, 4326), $5, $6, $7)
		`, rl.ID, rl.UserID, nullableUUID(rl.RouteID), wkt, startedAt, finishedAt, now)
	} else {
		_, err = r.pool.Exec(ctx, `
			INSERT INTO community.ride_log
				(id, user_id, route_id, started_at, finished_at, created_at)
			VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6)
		`, rl.ID, rl.UserID, nullableUUID(rl.RouteID), startedAt, finishedAt, now)
	}
	if err != nil {
		return fmt.Errorf("RideLogRepo.Create: %w", err)
	}
	return nil
}

// ListByUser returns all ride logs for the given user, newest first.
func (r *RideLogRepo) ListByUser(userID string) ([]*community.RideLog, error) {
	ctx := context.Background()
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, COALESCE(route_id::text, ''),
		       CASE WHEN gpx_track IS NOT NULL THEN ST_AsText(gpx_track) ELSE '' END,
		       started_at, finished_at, created_at
		FROM community.ride_log
		WHERE user_id = $1::uuid
		ORDER BY started_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("RideLogRepo.ListByUser: %w", err)
	}
	defer rows.Close()
	return collectRideLogs(rows)
}

// --- helpers ---

func scanRideLog(row pgx.Row) (*community.RideLog, error) {
	var rl community.RideLog
	var gpxWKT string
	var startedAt, finishedAt, createdAt time.Time
	if err := row.Scan(&rl.ID, &rl.UserID, &rl.RouteID, &gpxWKT, &startedAt, &finishedAt, &createdAt); err != nil {
		return nil, err
	}
	rl.RiddenAt = int(startedAt.Unix())
	rl.DurationS = int(finishedAt.Sub(startedAt).Seconds())
	rl.CreatedAt = createdAt
	if gpxWKT != "" {
		coords, err := wktToCoords(gpxWKT)
		if err == nil {
			rl.GPXTrack = coords
		}
	}
	// Ensure nil slices are empty slices for consistent JSON encoding.
	if rl.GPXTrack == nil {
		rl.GPXTrack = [][3]float64{}
	}
	return &rl, nil
}

func collectRideLogs(rows pgx.Rows) ([]*community.RideLog, error) {
	var logs []*community.RideLog
	for rows.Next() {
		rl, err := scanRideLog(rows)
		if err != nil {
			return nil, err
		}
		logs = append(logs, rl)
	}
	if logs == nil {
		logs = []*community.RideLog{}
	}
	if err := rows.Err(); err != nil && !isNotFound(err) {
		return nil, err
	}
	return logs, nil
}
