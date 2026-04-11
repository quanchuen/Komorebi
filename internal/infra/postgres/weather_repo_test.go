package postgres_test

import (
	"errors"
	"testing"
	"time"

	"github.com/cyclist-map/cyclist-map/internal/domain/environment"
	"github.com/cyclist-map/cyclist-map/internal/infra/postgres"
)

// seedWeatherCell inserts a single 5 km cell centred on (lat, lon) for the given time.
func seedWeatherCell(t *testing.T, repo *postgres.WeatherRepo, lat, lon float64, validAt time.Time) {
	t.Helper()
	half := 0.025
	cell := [][2]float64{
		{lon - half, lat - half},
		{lon + half, lat - half},
		{lon + half, lat + half},
		{lon - half, lat + half},
		{lon - half, lat - half},
	}
	err := repo.Upsert([]environment.WeatherGrid{{
		CellGeometry:       cell,
		ValidAt:            validAt,
		WindSpeedMS:        5.0,
		WindBearingDeg:     180.0,
		PrecipIntensityMMH: 0.0,
		TemperatureC:       18.0,
	}})
	if err != nil {
		t.Fatalf("seedWeatherCell: %v", err)
	}
}

func TestWeatherRepo_UpsertAndAtPoint(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewWeatherRepo(pool)

	// Use a fixed time well in the past to avoid index conflicts with real data.
	validAt := time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC)
	// Tokyo station ~35.6812°N, 139.7671°E
	lat, lon := 35.6812, 139.7671
	seedWeatherCell(t, repo, lat, lon, validAt)

	got, err := repo.AtPoint(lat, lon, validAt)
	if err != nil {
		t.Fatalf("AtPoint: %v", err)
	}
	if got.WindSpeedMS != 5.0 {
		t.Errorf("WindSpeedMS: want 5.0, got %v", got.WindSpeedMS)
	}
	if got.WindBearingDeg != 180.0 {
		t.Errorf("WindBearingDeg: want 180, got %v", got.WindBearingDeg)
	}

	// Cleanup
	_ = repo.DeleteBefore(validAt.Add(time.Second))
}

func TestWeatherRepo_AtPoint_NoData(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewWeatherRepo(pool)

	// Point in the middle of the ocean, definitely no data
	_, err := repo.AtPoint(0.0, 0.0, time.Now())
	if !errors.Is(err, environment.ErrNoWeather) {
		t.Fatalf("want ErrNoWeather, got %v", err)
	}
}

func TestWeatherRepo_Upsert_Idempotent(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewWeatherRepo(pool)

	validAt := time.Date(2020, 2, 1, 6, 0, 0, 0, time.UTC)
	lat, lon := 35.7000, 139.8000
	seedWeatherCell(t, repo, lat, lon, validAt)

	// Second upsert with updated wind speed — should not error or duplicate.
	half := 0.025
	cell := [][2]float64{
		{lon - half, lat - half}, {lon + half, lat - half},
		{lon + half, lat + half}, {lon - half, lat + half},
		{lon - half, lat - half},
	}
	err := repo.Upsert([]environment.WeatherGrid{{
		CellGeometry:   cell,
		ValidAt:        validAt,
		WindSpeedMS:    9.0,
		WindBearingDeg: 90.0,
	}})
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	got, err := repo.AtPoint(lat, lon, validAt)
	if err != nil {
		t.Fatalf("AtPoint after re-upsert: %v", err)
	}
	if got.WindSpeedMS != 9.0 {
		t.Errorf("WindSpeedMS after update: want 9.0, got %v", got.WindSpeedMS)
	}

	_ = repo.DeleteBefore(validAt.Add(time.Second))
}

func TestWeatherRepo_DeleteBefore(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewWeatherRepo(pool)

	validAt := time.Date(2020, 3, 1, 0, 0, 0, 0, time.UTC)
	lat, lon := 35.6500, 139.7500
	seedWeatherCell(t, repo, lat, lon, validAt)

	if err := repo.DeleteBefore(validAt.Add(time.Second)); err != nil {
		t.Fatalf("DeleteBefore: %v", err)
	}

	_, err := repo.AtPoint(lat, lon, validAt)
	if !errors.Is(err, environment.ErrNoWeather) {
		t.Fatalf("want ErrNoWeather after delete, got %v", err)
	}
}
