// pipelines/weather_fetch/main.go
//
// Weather Fetch Pipeline
//
// Fetches hourly Open-Meteo forecasts for the Greater Tokyo grid and stores
// them in environment.weather_grid. Designed to run hourly via cron:
//
//	0 * * * * DATABASE_URL=... /path/to/weather_fetch
//
// Grid coverage: 35.5–35.85°N, 139.4–140.0°E at 0.05° spacing (~5 km cells).
// Each run fetches ~48 forecast hours × ~91 grid points = ~4 400 rows.
// Rows older than 48 hours are pruned after each successful fetch.
package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/cyclist-map/cyclist-map/internal/infra/openmeteo"
	"github.com/cyclist-map/cyclist-map/internal/infra/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	gridMinLat  = 35.50
	gridMaxLat  = 35.85
	gridMinLon  = 139.40
	gridMaxLon  = 140.00
	gridStepDeg = 0.05 // ~5 km at Tokyo latitude
	retainHours = 48
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("db ping: %v", err)
	}

	weatherRepo := postgres.NewWeatherRepo(pool)

	omBaseURL := os.Getenv("OPENMETEO_BASE_URL") // override in tests
	client := openmeteo.NewClient(omBaseURL)

	log.Println("fetching Open-Meteo grid...")

	cells, err := client.FetchGrid(ctx, gridMinLat, gridMaxLat, gridMinLon, gridMaxLon, gridStepDeg)
	if err != nil {
		log.Fatalf("FetchGrid: %v", err)
	}
	log.Printf("fetched %d forecast rows", len(cells))

	log.Println("upserting into weather_grid...")
	if err := weatherRepo.Upsert(cells); err != nil {
		log.Fatalf("Upsert: %v", err)
	}

	cutoff := time.Now().UTC().Add(-retainHours * time.Hour)
	log.Printf("pruning rows older than %v...", cutoff.Format(time.RFC3339))
	if err := weatherRepo.DeleteBefore(cutoff); err != nil {
		log.Printf("WARN: DeleteBefore: %v (non-fatal)", err)
	}

	log.Println("weather_fetch: done")
}
