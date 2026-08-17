// pipelines/weather_fetch/main.go
//
// # Weather Fetch Pipeline
//
// Fetches hourly forecasts for the Greater Tokyo grid and stores them in
// environment.weather_grid. Designed to run hourly via cron:
//
//	0 * * * * DATABASE_URL=... /path/to/weather_fetch
//
// Provider selection:
//
//	open-meteo       (default, free, no API key)
//	tomorrow-io      (requires WEATHER_API_KEY)
//	openweathermap   (requires WEATHER_API_KEY)
//
// The bulk grid (~100 calls/run) always uses open-meteo unless
// WEATHER_GRID_PROVIDER overrides it — rate-limited paid tiers cannot absorb
// it hourly. The minutely nowcast (~12 calls/run) uses WEATHER_PROVIDER, so a
// premium nowcast provider is applied where it fits its quota.
package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"komorebi/internal/domain/environment"
	"komorebi/internal/infra/postgres"
	"komorebi/internal/infra/weatherprovider"
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
	gridFetcher := newGridFetcher()
	minutelyFetcher := newMinutelyFetcher()

	log.Printf("fetching weather grid via %s...", gridFetcher.Name())

	cells, err := gridFetcher.FetchGrid(ctx, gridMinLat, gridMaxLat, gridMinLon, gridMaxLon, gridStepDeg)
	if err != nil {
		log.Fatalf("FetchGrid: %v", err)
	}
	log.Printf("fetched %d forecast rows", len(cells))

	log.Println("upserting into weather_grid...")
	if err := weatherRepo.Upsert(cells); err != nil {
		log.Fatalf("Upsert: %v", err)
	}

	// Fetch minutely precipitation for key grid points (sparser grid, center
	// Tokyo). A rate-limited premium provider falls back to open-meteo per
	// point so a 429 hour still produces a nowcast.
	log.Printf("fetching minutely precipitation via %s...", minutelyFetcher.Name())
	fallback := newMinutelyFallback(minutelyFetcher)
	minutelyCount := 0
	for lat := 35.60; lat <= 35.80+1e-9; lat += 0.10 {
		for lon := 139.60; lon <= 139.90+1e-9; lon += 0.10 {
			rows, err := minutelyFetcher.FetchMinutely(ctx, lat, lon)
			if err != nil && fallback != nil {
				log.Printf("WARN: minutely %f,%f via %s: %v (falling back to %s)",
					lat, lon, minutelyFetcher.Name(), err, fallback.Name())
				rows, err = fallback.FetchMinutely(ctx, lat, lon)
			}
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

// newGridFetcher returns the provider for the bulk grid. It defaults to
// open-meteo regardless of WEATHER_PROVIDER: the grid burns ~100 calls per
// run, which free open-meteo absorbs and rate-limited paid tiers do not.
func newGridFetcher() environment.WeatherFetcher {
	provider := os.Getenv("WEATHER_GRID_PROVIDER")
	if provider == "" {
		provider = "open-meteo"
	}
	fetcher, err := weatherprovider.Named(provider)
	if err != nil {
		log.Fatalf("grid weather provider: %v", err)
	}
	return fetcher
}

// newMinutelyFetcher returns the provider for the minutely nowcast (~12 calls
// per run), which follows WEATHER_PROVIDER so a premium nowcast source is
// used when configured.
func newMinutelyFetcher() environment.WeatherFetcher {
	fetcher, err := weatherprovider.FromEnv()
	if err != nil {
		log.Fatalf("minutely weather provider: %v", err)
	}
	return fetcher
}

// newMinutelyFallback returns open-meteo as a per-point fallback when the
// primary minutely provider is a different (rate-limitable) one, else nil.
func newMinutelyFallback(primary environment.WeatherFetcher) environment.WeatherFetcher {
	fallback, err := weatherprovider.Named("open-meteo")
	if err != nil || fallback.Name() == primary.Name() {
		return nil
	}
	return fallback
}
