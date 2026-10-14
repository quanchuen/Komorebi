package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"komorebi/internal/domain/environment"
	"komorebi/internal/infra/postgres"
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

func TestWeatherRepo_GridInBBox(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewWeatherRepo(pool)
	ctx := context.Background()

	// Fixed times well in the past to avoid clashing with live forecast rows.
	snapA := time.Date(2021, 5, 1, 12, 0, 0, 0, time.UTC)
	snapB := snapA.Add(time.Hour)
	// Two cells inside the bbox at snapA, one outside, one inside at snapB.
	seedWeatherCell(t, repo, 35.625, 139.625, snapA)
	seedWeatherCell(t, repo, 35.675, 139.675, snapA)
	seedWeatherCell(t, repo, 36.500, 140.500, snapA) // outside bbox
	seedWeatherCell(t, repo, 35.625, 139.625, snapB)

	bbox := [4]float64{139.60, 35.60, 139.85, 35.75}

	// 10 min past snapA: nearest distinct snapshot is snapA.
	cells, err := repo.GridInBBox(ctx, bbox, snapA.Add(10*time.Minute))
	if err != nil {
		t.Fatalf("GridInBBox: %v", err)
	}
	if len(cells) != 2 {
		t.Fatalf("cells: want 2, got %d", len(cells))
	}
	for _, c := range cells {
		if !c.ValidAt.Equal(snapA) {
			t.Errorf("ValidAt: want %v, got %v", snapA, c.ValidAt)
		}
		if len(c.CellGeometry) != 5 {
			t.Errorf("CellGeometry: want closed 5-point ring, got %d points", len(c.CellGeometry))
		}
	}
	// seedWeatherCell centres a 0.05 degree cell on (lat, lon).
	first := cells[0]
	if first.CellGeometry[0][0] != 139.60 || first.CellGeometry[0][1] != 35.60 {
		t.Errorf("first cell SW corner: want (139.60, 35.60), got %v", first.CellGeometry[0])
	}

	// Outside the +/-1h window: empty result, no error.
	cells, err = repo.GridInBBox(ctx, bbox, snapA.Add(-3*time.Hour))
	if err != nil {
		t.Fatalf("GridInBBox out of window: %v", err)
	}
	if len(cells) != 0 {
		t.Errorf("cells out of window: want 0, got %d", len(cells))
	}

	_ = repo.DeleteBefore(snapB.Add(time.Second))
}

func TestWeatherRepo_MinutelySnapshot(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewWeatherRepo(pool)
	ctx := context.Background()

	minuteA := time.Date(2021, 5, 1, 12, 0, 0, 0, time.UTC)
	minuteB := minuteA.Add(time.Minute)
	fetched := minuteA
	err := repo.UpsertMinutely([]environment.MinutelyPrecip{
		{Lat: 35.60, Lon: 139.60, At: minuteA, IntensityMMH: 1.5, FetchedAt: fetched},
		{Lat: 35.70, Lon: 139.70, At: minuteA, IntensityMMH: 0.5, FetchedAt: fetched},
		{Lat: 35.60, Lon: 139.60, At: minuteB, IntensityMMH: 2.0, FetchedAt: fetched},
	})
	if err != nil {
		t.Fatalf("UpsertMinutely: %v", err)
	}

	// 20s past minuteA: nearest distinct minute is minuteA with both points.
	rows, err := repo.MinutelySnapshot(ctx, minuteA.Add(20*time.Second))
	if err != nil {
		t.Fatalf("MinutelySnapshot: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows: want 2, got %d", len(rows))
	}
	for _, m := range rows {
		if !m.At.Equal(minuteA) {
			t.Errorf("At: want %v, got %v", minuteA, m.At)
		}
	}

	// Beyond the +/-5 min window: empty, no error.
	rows, err = repo.MinutelySnapshot(ctx, minuteA.Add(20*time.Minute))
	if err != nil {
		t.Fatalf("MinutelySnapshot out of window: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("rows out of window: want 0, got %d", len(rows))
	}

	_ = repo.DeleteMinutelyBefore(minuteB.Add(time.Second))
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
