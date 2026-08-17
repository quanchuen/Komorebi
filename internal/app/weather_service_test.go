package app

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"komorebi/internal/domain/environment"
)

// stubWeatherRepo implements environment.WeatherRepository. AtPoint returns
// grid when set, otherwise ErrNoWeather.
type stubWeatherRepo struct {
	grid *environment.WeatherGrid
	err  error
}

func (s *stubWeatherRepo) Upsert(_ []environment.WeatherGrid) error { return nil }
func (s *stubWeatherRepo) AtPoint(_, _ float64, _ time.Time) (*environment.WeatherGrid, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.grid == nil {
		return nil, environment.ErrNoWeather
	}
	return s.grid, nil
}
func (s *stubWeatherRepo) AlongRoute(segments []environment.WeatherSegmentQuery) ([]environment.WeatherGrid, error) {
	results := make([]environment.WeatherGrid, len(segments))
	for i := range segments {
		if s.grid != nil {
			results[i] = *s.grid
		}
	}
	return results, nil
}
func (s *stubWeatherRepo) DeleteBefore(_ time.Time) error                    { return nil }
func (s *stubWeatherRepo) UpsertMinutely(_ []environment.MinutelyPrecip) error { return nil }
func (s *stubWeatherRepo) MinutelyAt(_, _ float64, _, _ time.Time) ([]environment.MinutelyPrecip, error) {
	return nil, nil
}
func (s *stubWeatherRepo) DeleteMinutelyBefore(_ time.Time) error { return nil }

// stubFetcher implements environment.WeatherFetcher, counting FetchPoint calls.
type stubFetcher struct {
	rows  []environment.WeatherGrid
	err   error
	calls atomic.Int64
	block chan struct{} // when set, FetchPoint waits until closed
}

func (f *stubFetcher) FetchPoint(_ context.Context, _, _ float64) ([]environment.WeatherGrid, error) {
	f.calls.Add(1)
	if f.block != nil {
		<-f.block
	}
	return f.rows, f.err
}
func (f *stubFetcher) FetchGrid(_ context.Context, _, _, _, _, _ float64) ([]environment.WeatherGrid, error) {
	return nil, nil
}
func (f *stubFetcher) FetchMinutely(_ context.Context, _, _ float64) ([]environment.MinutelyPrecip, error) {
	return nil, nil
}
func (f *stubFetcher) Name() string { return "stub" }

func hourlyRows(start time.Time, hours int, temp float64) []environment.WeatherGrid {
	rows := make([]environment.WeatherGrid, hours)
	for i := range rows {
		rows[i] = environment.WeatherGrid{
			ValidAt:      start.Add(time.Duration(i) * time.Hour),
			TemperatureC: temp,
			WindSpeedMS:  3,
		}
	}
	return rows
}

func TestAtPointPrefersRepository(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Hour)
	repo := &stubWeatherRepo{grid: &environment.WeatherGrid{ValidAt: now, TemperatureC: 20}}
	fetcher := &stubFetcher{rows: hourlyRows(now, 3, 99)}
	svc := NewWeatherService(repo, fetcher)

	wg, err := svc.AtPoint(35.68, 139.76, now)
	if err != nil {
		t.Fatalf("AtPoint: %v", err)
	}
	if wg.TemperatureC != 20 {
		t.Errorf("expected repo value 20, got %v", wg.TemperatureC)
	}
	if fetcher.calls.Load() != 0 {
		t.Errorf("fetcher should not be called on repo hit, got %d calls", fetcher.calls.Load())
	}
}

func TestAtPointFetchesOnRepoMissAndCaches(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Hour)
	repo := &stubWeatherRepo{}
	fetcher := &stubFetcher{rows: hourlyRows(now, 6, 25)}
	svc := NewWeatherService(repo, fetcher)

	for i := 0; i < 3; i++ {
		wg, err := svc.AtPoint(35.681, 139.762, now.Add(30*time.Minute))
		if err != nil {
			t.Fatalf("AtPoint[%d]: %v", i, err)
		}
		if wg.TemperatureC != 25 {
			t.Errorf("expected fetched value 25, got %v", wg.TemperatureC)
		}
	}
	// Third call in the same ~5 km cell: still one upstream fetch.
	if _, err := svc.AtPoint(35.685, 139.758, now); err != nil {
		t.Fatalf("AtPoint same cell: %v", err)
	}
	if got := fetcher.calls.Load(); got != 1 {
		t.Errorf("expected 1 upstream fetch for same cell, got %d", got)
	}

	// A different cell fetches again.
	if _, err := svc.AtPoint(36.0, 140.0, now); err != nil {
		t.Fatalf("AtPoint other cell: %v", err)
	}
	if got := fetcher.calls.Load(); got != 2 {
		t.Errorf("expected 2 upstream fetches after new cell, got %d", got)
	}
}

func TestAtPointNoFetcherPassesThroughMiss(t *testing.T) {
	svc := NewWeatherService(&stubWeatherRepo{}, nil)
	_, err := svc.AtPoint(35.68, 139.76, time.Now())
	if !errors.Is(err, environment.ErrNoWeather) {
		t.Fatalf("expected ErrNoWeather, got %v", err)
	}
}

func TestAtPointOutsideForecastHorizon(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Hour)
	fetcher := &stubFetcher{rows: hourlyRows(now, 3, 25)}
	svc := NewWeatherService(&stubWeatherRepo{}, fetcher)

	_, err := svc.AtPoint(35.68, 139.76, now.Add(12*time.Hour))
	if !errors.Is(err, environment.ErrNoWeather) {
		t.Fatalf("expected ErrNoWeather beyond horizon, got %v", err)
	}
}

func TestAtPointFetchErrorSurfaces(t *testing.T) {
	fetchErr := errors.New("upstream down")
	svc := NewWeatherService(&stubWeatherRepo{}, &stubFetcher{err: fetchErr})
	_, err := svc.AtPoint(35.68, 139.76, time.Now())
	if !errors.Is(err, fetchErr) {
		t.Fatalf("expected fetch error, got %v", err)
	}
}

func TestCacheTTLExpiryRefetches(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Hour)
	fetcher := &stubFetcher{rows: hourlyRows(now, 6, 25)}
	svc := NewWeatherService(&stubWeatherRepo{}, fetcher)

	clock := now
	svc.cache.now = func() time.Time { return clock }

	if _, err := svc.AtPoint(35.68, 139.76, now); err != nil {
		t.Fatalf("AtPoint: %v", err)
	}
	clock = clock.Add(weatherCacheTTL + time.Minute)
	if _, err := svc.AtPoint(35.68, 139.76, now); err != nil {
		t.Fatalf("AtPoint after expiry: %v", err)
	}
	if got := fetcher.calls.Load(); got != 2 {
		t.Errorf("expected refetch after TTL expiry, got %d fetches", got)
	}
}

func TestCacheEvictsLeastRecentlyUsed(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Hour)
	fetcher := &stubFetcher{rows: hourlyRows(now, 6, 25)}
	svc := NewWeatherService(&stubWeatherRepo{}, fetcher)
	svc.cache.capacity = 2

	pts := [][2]float64{{35.0, 139.0}, {35.2, 139.2}, {35.4, 139.4}}
	for _, p := range pts {
		if _, err := svc.AtPoint(p[0], p[1], now); err != nil {
			t.Fatalf("AtPoint(%v): %v", p, err)
		}
	}
	// First point was evicted (capacity 2), so it refetches.
	if _, err := svc.AtPoint(pts[0][0], pts[0][1], now); err != nil {
		t.Fatalf("AtPoint evicted: %v", err)
	}
	if got := fetcher.calls.Load(); got != 4 {
		t.Errorf("expected 4 fetches (3 fills + 1 after eviction), got %d", got)
	}
	// Second-to-last point is still cached.
	if _, err := svc.AtPoint(pts[2][0], pts[2][1], now); err != nil {
		t.Fatalf("AtPoint cached: %v", err)
	}
	if got := fetcher.calls.Load(); got != 4 {
		t.Errorf("expected cached hit, got %d fetches", got)
	}
}

func TestConcurrentMissesCoalesceToOneFetch(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Hour)
	fetcher := &stubFetcher{rows: hourlyRows(now, 6, 25), block: make(chan struct{})}
	svc := NewWeatherService(&stubWeatherRepo{}, fetcher)

	var wg sync.WaitGroup
	errs := make([]error, 5)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = svc.AtPoint(35.68, 139.76, now)
		}(i)
	}
	// Let goroutines pile up on the in-flight fetch, then release it.
	time.Sleep(50 * time.Millisecond)
	close(fetcher.block)
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d: %v", i, err)
		}
	}
	if got := fetcher.calls.Load(); got != 1 {
		t.Errorf("expected concurrent misses to coalesce into 1 fetch, got %d", got)
	}
}

func TestAlongRouteFillsMissingCellsFromFetcher(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Hour)
	fetcher := &stubFetcher{rows: hourlyRows(now, 6, 25)}
	svc := NewWeatherService(&stubWeatherRepo{}, fetcher)

	segments := []environment.WeatherSegmentQuery{
		{MidLat: 35.68, MidLon: 139.76, ArrivalAt: now},
		{MidLat: 35.69, MidLon: 139.77, ArrivalAt: now.Add(20 * time.Minute)},
	}
	results, err := svc.AlongRoute(segments, []float64{0, 90})
	if err != nil {
		t.Fatalf("AlongRoute: %v", err)
	}
	for i, r := range results {
		if r.TemperatureC != 25 {
			t.Errorf("segment %d: expected fetched temperature 25, got %v", i, r.TemperatureC)
		}
		if r.WindSpeedMS != 3 {
			t.Errorf("segment %d: expected wind 3, got %v", i, r.WindSpeedMS)
		}
	}
	if got := fetcher.calls.Load(); got != 1 {
		t.Errorf("expected nearby segments to share 1 fetch, got %d", got)
	}
}
