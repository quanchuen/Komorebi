// pipelines/weather_fetch/main.go
//
// Weather Fetch Pipeline
//
// Fetches hourly forecasts for the Greater Tokyo grid and stores them in
// environment.weather_grid. Designed to run hourly via cron:
//
//	0 * * * * DATABASE_URL=... /path/to/weather_fetch
//
// Provider selection via WEATHER_PROVIDER env var:
//
//	open-meteo       (default, free, no API key)
//	tomorrow-io      (requires WEATHER_API_KEY)
//	openweathermap   (requires WEATHER_API_KEY)
package main

import (
	"context"
	"log"
	"os"
	"time"

	"komorebi/internal/domain/environment"
	"komorebi/internal/infra/openmeteo"
	"komorebi/internal/infra/openweathermap"
	"komorebi/internal/infra/postgres"
	"komorebi/internal/infra/tomorrowio"
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
	fetcher := newFetcher()

	log.Printf("fetching weather grid via %s...", fetcher.Name())

	cells, err := fetcher.FetchGrid(ctx, gridMinLat, gridMaxLat, gridMinLon, gridMaxLon, gridStepDeg)
	if err != nil {
		log.Fatalf("FetchGrid: %v", err)
	}
	log.Printf("fetched %d forecast rows", len(cells))

	log.Println("upserting into weather_grid...")
	if err := weatherRepo.Upsert(cells); err != nil {
		log.Fatalf("Upsert: %v", err)
	}

	// Fetch minutely precipitation for key grid points (sparser grid, center Tokyo)
	log.Printf("fetching minutely precipitation via %s...", fetcher.Name())
	minutelyCount := 0
	for lat := 35.60; lat <= 35.80+1e-9; lat += 0.10 {
		for lon := 139.60; lon <= 139.90+1e-9; lon += 0.10 {
			rows, err := fetcher.FetchMinutely(ctx, lat, lon)
			if err != nil {
				log.Printf("WARN: minutely %f,%f: %v (skipping)", lat, lon, err)
				continue
			}
			if len(rows) > 0 {
				if err := weatherRepo.UpsertMinutely(rows); err != nil {
					log.Printf("WARN: minutely upsert: %v (skipping)", err)
					continue
				}
				minutelyCount += len(rows)
			}
		}
	}
	log.Printf("upserted %d minutely rows", minutelyCount)

	cutoff := time.Now().UTC().Add(-retainHours * time.Hour)
	log.Printf("pruning rows older than %v...", cutoff.Format(time.RFC3339))
	if err := weatherRepo.DeleteBefore(cutoff); err != nil {
		log.Printf("WARN: DeleteBefore: %v (non-fatal)", err)
	}
	if err := weatherRepo.DeleteMinutelyBefore(cutoff); err != nil {
		log.Printf("WARN: DeleteMinutelyBefore: %v (non-fatal)", err)
	}

	log.Println("weather_fetch: done")
}

func newFetcher() environment.WeatherFetcher {
	provider := os.Getenv("WEATHER_PROVIDER")
	apiKey := os.Getenv("WEATHER_API_KEY")
	baseURL := os.Getenv("WEATHER_BASE_URL") // override for testing

	switch provider {
	case "tomorrow-io":
		if apiKey == "" {
			log.Fatal("WEATHER_API_KEY is required for tomorrow-io")
		}
		return tomorrowio.NewClient(apiKey, baseURL)

	case "openweathermap":
		if apiKey == "" {
			log.Fatal("WEATHER_API_KEY is required for openweathermap")
		}
		return openweathermap.NewClient(apiKey, baseURL)

	case "open-meteo", "":
		return openmeteo.NewClient(baseURL)

	default:
		log.Fatalf("unknown WEATHER_PROVIDER: %q (supported: open-meteo, tomorrow-io, openweathermap)", provider)
		return nil
	}
}
