package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"komorebi/internal/infra/valhalla"
)

var demoRouteIDs = []string{
	"10000000-0000-0000-0000-000000000001", "10000000-0000-0000-0000-000000000002",
	"10000000-0000-0000-0000-000000000003", "10000000-0000-0000-0000-000000000004",
	"10000000-0000-0000-0000-000000000005",
}

func main() {
	ctx := context.Background()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	valhallaURL := os.Getenv("VALHALLA_URL")
	if valhallaURL == "" {
		valhallaURL = "http://localhost:8002"
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	router := valhalla.NewClient(valhallaURL)
	for _, routeID := range demoRouteIDs {
		if err := reroute(ctx, pool, router, routeID); err != nil {
			log.Fatalf("reroute %s: %v", routeID, err)
		}
	}
}

func reroute(ctx context.Context, pool *pgxpool.Pool, router *valhalla.Client, routeID string) error {
	rows, err := pool.Query(ctx, `SELECT ST_Y(geometry), ST_X(geometry) FROM routes.waypoint WHERE route_id = $1::uuid ORDER BY sort_order`, routeID)
	if err != nil {
		return err
	}
	var stops []valhalla.Location
	for rows.Next() {
		var stop valhalla.Location
		if err := rows.Scan(&stop.Lat, &stop.Lon); err != nil {
			rows.Close()
			return err
		}
		stops = append(stops, stop)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if len(stops) < 2 {
		return fmt.Errorf("needs at least two waypoints")
	}
	result, err := router.Route(stops, valhalla.ProfileSuggested)
	if err != nil {
		return err
	}
	var shape [][2]float64
	for index, leg := range result.Legs {
		if index == 0 {
			shape = append(shape, leg.Shape...)
		} else if len(leg.Shape) > 0 {
			shape = append(shape, leg.Shape[1:]...)
		}
	}
	if len(shape) < 2 {
		return fmt.Errorf("Valhalla returned empty geometry")
	}
	wkt := lineStringZ(shape, result.Elevation)
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	if _, err := tx.Exec(ctx, `UPDATE routes.route SET geometry = ST_GeomFromText($2, 4326), distance_m = $3, elevation_gain_m = $4, elevation_loss_m = $5, updated_at = now() WHERE id = $1::uuid`, routeID, wkt, result.TotalDistanceKm*1000, result.ElevationGainM, result.ElevationLossM); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM routes.route_segment WHERE route_id = $1::uuid`, routeID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO routes.route_segment (route_id, geometry, surface_type, grade_percent, segment_order) VALUES ($1::uuid, ST_GeomFromText($2, 4326), 'paved', $3, 0)`, routeID, wkt, averageGrade(result)); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM routes.route_tag WHERE route_id = $1::uuid AND tag = 'demo'`, routeID); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	log.Printf("routed %s: %.1f km, +%.0f m / -%.0f m", routeID, result.TotalDistanceKm, result.ElevationGainM, result.ElevationLossM)
	return nil
}

func lineStringZ(shape [][2]float64, profile []valhalla.ElevationPoint) string {
	parts := make([]string, len(shape))
	travelled := 0.0
	for index, point := range shape {
		if index > 0 {
			travelled += distanceM(shape[index-1], point)
		}
		parts[index] = strconv.FormatFloat(point[0], 'f', 6, 64) + " " + strconv.FormatFloat(point[1], 'f', 6, 64) + " " + strconv.FormatFloat(elevationAt(profile, travelled), 'f', 1, 64)
	}
	return "LINESTRING Z(" + strings.Join(parts, ",") + ")"
}

func elevationAt(profile []valhalla.ElevationPoint, distance float64) float64 {
	if len(profile) == 0 {
		return 0
	}
	for index := 1; index < len(profile); index++ {
		if profile[index].DistanceM >= distance {
			left, right := profile[index-1], profile[index]
			span := right.DistanceM - left.DistanceM
			if span <= 0 {
				return right.ElevationM
			}
			return left.ElevationM + (distance-left.DistanceM)/span*(right.ElevationM-left.ElevationM)
		}
	}
	return profile[len(profile)-1].ElevationM
}

func distanceM(a, b [2]float64) float64 {
	const radius = 6371000.0
	lat1, lat2 := a[1]*math.Pi/180, b[1]*math.Pi/180
	dLat, dLon := (b[1]-a[1])*math.Pi/180, (b[0]-a[0])*math.Pi/180
	h := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1)*math.Cos(lat2)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return radius * 2 * math.Atan2(math.Sqrt(h), math.Sqrt(1-h))
}

func averageGrade(result *valhalla.RouteResult) float64 {
	if result.TotalDistanceKm <= 0 {
		return 0
	}
	return (result.ElevationGainM - result.ElevationLossM) / (result.TotalDistanceKm * 1000) * 100
}
