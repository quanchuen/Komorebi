// internal/infra/postgres/plan_repo.go
package postgres

import (
	"context"
	"errors"
	"fmt"

	"komorebi/internal/domain/plan"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PlanRepo implements plan.Repository using PostgreSQL.
type PlanRepo struct {
	pool *pgxpool.Pool
}

// NewPlanRepo creates a new PlanRepo.
func NewPlanRepo(pool *pgxpool.Pool) *PlanRepo {
	return &PlanRepo{pool: pool}
}

// Create inserts a RoutePlan with its stops and tasks in a single transaction.
func (r *PlanRepo) Create(p *plan.RoutePlan) error {
	ctx := context.Background()
	return r.withTx(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO plan.route_plan
				(id, user_id, departure_at, speed_model,
				 shade_weight, greenery_weight, wind_weight,
				 route_wkt, created_at, updated_at)
			VALUES ($1::uuid, $2::uuid, $3, $4::plan.speed_model,
			        $5, $6, $7, $8, $9, $10)
		`,
			p.ID, nullableUUID(p.UserID), p.DepartureAt,
			string(p.SpeedModel),
			p.Preferences.ShadeWeight, p.Preferences.GreeneryWeight, p.Preferences.WindWeight,
			nullableStr(p.RouteWKT),
			p.CreatedAt, p.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("insert route_plan: %w", err)
		}
		if err := insertStops(ctx, tx, p.ID, p.Stops); err != nil {
			return err
		}
		return insertTasks(ctx, tx, p.ID, p.Tasks)
	})
}

// GetByID fetches a RoutePlan by UUID, including stops and tasks.
// Returns ErrNotFound if no row exists.
func (r *PlanRepo) GetByID(id string) (*plan.RoutePlan, error) {
	ctx := context.Background()
	row := r.pool.QueryRow(ctx, `
		SELECT id, COALESCE(user_id::text, ''),
		       departure_at, speed_model,
		       shade_weight, greenery_weight, wind_weight,
		       COALESCE(route_wkt, ''),
		       created_at, updated_at
		FROM plan.route_plan
		WHERE id = $1::uuid
	`, id)

	var p plan.RoutePlan
	var speedModel string
	err := row.Scan(
		&p.ID, &p.UserID,
		&p.DepartureAt, &speedModel,
		&p.Preferences.ShadeWeight, &p.Preferences.GreeneryWeight, &p.Preferences.WindWeight,
		&p.RouteWKT,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("GetByID plan: %w", err)
	}
	p.SpeedModel = plan.SpeedModel(speedModel)

	stops, err := r.loadStops(ctx, id)
	if err != nil {
		return nil, err
	}
	p.Stops = stops

	tasks, err := r.loadTasks(ctx, id)
	if err != nil {
		return nil, err
	}
	p.Tasks = tasks

	return &p, nil
}

// Update replaces all fields, stops, and tasks for an existing plan in a transaction.
func (r *PlanRepo) Update(p *plan.RoutePlan) error {
	ctx := context.Background()
	return r.withTx(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE plan.route_plan SET
				departure_at    = $2,
				speed_model     = $3::plan.speed_model,
				shade_weight    = $4,
				greenery_weight = $5,
				wind_weight     = $6,
				route_wkt       = $7,
				updated_at      = $8
			WHERE id = $1::uuid
		`,
			p.ID, p.DepartureAt, string(p.SpeedModel),
			p.Preferences.ShadeWeight, p.Preferences.GreeneryWeight, p.Preferences.WindWeight,
			nullableStr(p.RouteWKT),
			p.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("update route_plan: %w", err)
		}
		for _, tbl := range []string{"plan.stop_point", "plan.plan_task"} {
			if _, err := tx.Exec(ctx, fmt.Sprintf("DELETE FROM %s WHERE plan_id = $1::uuid", tbl), p.ID); err != nil {
				return fmt.Errorf("delete from %s: %w", tbl, err)
			}
		}
		if err := insertStops(ctx, tx, p.ID, p.Stops); err != nil {
			return err
		}
		return insertTasks(ctx, tx, p.ID, p.Tasks)
	})
}

// Delete removes a plan by ID. Child rows cascade via FK.
func (r *PlanRepo) Delete(id string) error {
	ctx := context.Background()
	_, err := r.pool.Exec(ctx, "DELETE FROM plan.route_plan WHERE id = $1::uuid", id)
	return err
}

// --- helpers ---

func (r *PlanRepo) withTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}

func (r *PlanRepo) loadStops(ctx context.Context, planID string) ([]plan.StopPoint, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id,
		       ST_Y(geometry) AS lat,
		       ST_X(geometry) AS lon,
		       type,
		       sort_order,
		       COALESCE(venue_id::text, ''),
		       COALESCE(resolved_name, '')
		FROM plan.stop_point
		WHERE plan_id = $1::uuid
		ORDER BY sort_order
	`, planID)
	if err != nil {
		return nil, fmt.Errorf("loadStops: %w", err)
	}
	defer rows.Close()
	var stops []plan.StopPoint
	for rows.Next() {
		var sp plan.StopPoint
		var stopType string
		if err := rows.Scan(
			&sp.ID, &sp.Lat, &sp.Lon,
			&stopType, &sp.SortOrder,
			&sp.VenueID, &sp.ResolvedName,
		); err != nil {
			return nil, fmt.Errorf("scan stop_point row: %w", err)
		}
		sp.Type = plan.StopType(stopType)
		stops = append(stops, sp)
	}
	if stops == nil {
		stops = []plan.StopPoint{}
	}
	return stops, rows.Err()
}

func (r *PlanRepo) loadTasks(ctx context.Context, planID string) ([]plan.PlanTask, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id,
		       description,
		       COALESCE(hashtag, ''),
		       status,
		       COALESCE(resolved_venue_id::text, '')
		FROM plan.plan_task
		WHERE plan_id = $1::uuid
		ORDER BY id
	`, planID)
	if err != nil {
		return nil, fmt.Errorf("loadTasks: %w", err)
	}
	defer rows.Close()
	var tasks []plan.PlanTask
	for rows.Next() {
		var t plan.PlanTask
		var status string
		if err := rows.Scan(
			&t.ID, &t.Description, &t.Hashtag,
			&status, &t.ResolvedVenueID,
		); err != nil {
			return nil, fmt.Errorf("scan plan_task row: %w", err)
		}
		t.Status = plan.TaskStatus(status)
		tasks = append(tasks, t)
	}
	if tasks == nil {
		tasks = []plan.PlanTask{}
	}
	return tasks, rows.Err()
}

func insertStops(ctx context.Context, tx pgx.Tx, planID string, stops []plan.StopPoint) error {
	for _, sp := range stops {
		pt := fmt.Sprintf("POINT(%f %f)", sp.Lon, sp.Lat)
		if sp.ID == "" {
			sp.ID = genUUID()
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO plan.stop_point
				(id, plan_id, geometry, type, sort_order, venue_id, resolved_name)
			VALUES ($1::uuid, $2::uuid, ST_GeomFromText($3, 4326),
			        $4::plan.stop_type, $5, $6, $7)
		`,
			sp.ID, planID, pt,
			string(sp.Type), sp.SortOrder,
			nullableUUID(sp.VenueID), sp.ResolvedName,
		)
		if err != nil {
			return fmt.Errorf("insert stop_point: %w", err)
		}
	}
	return nil
}

func insertTasks(ctx context.Context, tx pgx.Tx, planID string, tasks []plan.PlanTask) error {
	for _, t := range tasks {
		if t.ID == "" {
			t.ID = genUUID()
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO plan.plan_task
				(id, plan_id, description, hashtag, status, resolved_venue_id)
			VALUES ($1::uuid, $2::uuid, $3, $4, $5::plan.task_status, $6)
		`,
			t.ID, planID, t.Description,
			nullableStr(t.Hashtag),
			string(t.Status),
			nullableUUID(t.ResolvedVenueID),
		)
		if err != nil {
			return fmt.Errorf("insert plan_task: %w", err)
		}
	}
	return nil
}
