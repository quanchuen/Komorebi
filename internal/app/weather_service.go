package app

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"komorebi/internal/domain/environment"
)

const (
	// weatherCellDeg snaps fetch coordinates to a ~5 km grid at Tokyo latitude
	// so nearby requests share one upstream forecast (matches the provider
	// cell size used by the grid pipeline).
	weatherCellDeg = 0.05
	// weatherMatchWindow mirrors the repository's +/-1 hour valid_at match.
	weatherMatchWindow   = time.Hour
	weatherCacheCapacity = 256
	weatherCacheTTL      = 30 * time.Minute
	// weatherNegativeTTL bounds how long a failed cell fetch is remembered so
	// a provider outage degrades to "no data" quickly instead of re-issuing
	// (and re-waiting on) the upstream call for every segment of every route.
	weatherNegativeTTL  = time.Minute
	weatherFetchTimeout = 5 * time.Second
)

// WeatherService is the application-layer facade over weather data.
//
// Reads are repository-first (the precomputed grid, when present). On a miss,
// the service fetches the point forecast live from the configured provider and
// serves it from an in-memory LRU so the grid pipeline is optional.
type WeatherService struct {
	repo    environment.WeatherRepository
	fetcher environment.WeatherFetcher
	cache   *pointForecastCache

	inflightMu sync.Mutex
	inflight   map[string]*forecastFetch
}

// forecastFetch coalesces concurrent live fetches for the same cell.
type forecastFetch struct {
	done chan struct{}
	rows []environment.WeatherGrid
	err  error
}

// NewWeatherService creates a WeatherService backed by the given repository.
// fetcher may be nil, in which case repository misses surface as ErrNoWeather.
func NewWeatherService(repo environment.WeatherRepository, fetcher environment.WeatherFetcher) *WeatherService {
	return &WeatherService{
		repo:     repo,
		fetcher:  fetcher,
		cache:    newPointForecastCache(weatherCacheCapacity),
		inflight: make(map[string]*forecastFetch),
	}
}

// AtPoint returns weather conditions at a geographic point and time.
// Returns ErrNoWeather if no data covers that point/time.
func (s *WeatherService) AtPoint(lat, lon float64, t time.Time) (*environment.WeatherGrid, error) {
	return s.AtPointContext(context.Background(), lat, lon, t)
}

// AtPointContext is AtPoint bound to a caller context: a live provider fetch
// stops waiting as soon as ctx is done. Every error after a repository miss
// wraps ErrNoWeather (there is no data to serve) and, for live fetches, the
// underlying provider error.
func (s *WeatherService) AtPointContext(ctx context.Context, lat, lon float64, t time.Time) (*environment.WeatherGrid, error) {
	wg, err := s.repo.AtPoint(lat, lon, t)
	if err == nil {
		return wg, nil
	}
	if s.fetcher == nil || !errors.Is(err, environment.ErrNoWeather) {
		return nil, err
	}
	wg, err = s.fetchedForecastAt(ctx, lat, lon, t)
	if err != nil && !errors.Is(err, environment.ErrNoWeather) {
		return nil, fmt.Errorf("%w: live fetch failed: %w", environment.ErrNoWeather, err)
	}
	return wg, err
}

// AlongRoute scores each segment by fetching the nearest weather cell and
// computing the wind benefit relative to each segment's bearing.
// segmentBearings[i] is the compass bearing (degrees) of segment i.
func (s *WeatherService) AlongRoute(
	segments []environment.WeatherSegmentQuery,
	segmentBearings []float64,
) ([]environment.SegmentWeather, error) {
	grids, err := s.repo.AlongRoute(segments)
	if err != nil {
		return nil, err
	}

	if s.fetcher != nil {
		for i := range grids {
			if !grids[i].ValidAt.IsZero() {
				continue
			}
			wg, err := s.fetchedForecastAt(context.Background(), segments[i].MidLat, segments[i].MidLon, segments[i].ArrivalAt)
			if err != nil {
				continue // leave zero value; caller treats as missing data
			}
			grids[i] = *wg
		}
	}

	results := make([]environment.SegmentWeather, len(grids))
	for i, g := range grids {
		bearing := 0.0
		if i < len(segmentBearings) {
			bearing = segmentBearings[i]
		}
		benefit := 0.0
		if g.WindSpeedMS > 0 {
			benefit = environment.WindBenefit(g.WindBearingDeg, bearing)
		}
		results[i] = environment.SegmentWeather{
			WindBenefit:        benefit,
			PrecipIntensityMMH: g.PrecipIntensityMMH,
			TemperatureC:       g.TemperatureC,
			WindSpeedMS:        g.WindSpeedMS,
		}
	}
	return results, nil
}

// fetchedForecastAt serves (lat, lon, t) from the LRU, fetching the cell's
// hourly forecast from the provider on a cache miss. A remembered failure
// (negative entry) is served as ErrNoWeather without touching the provider.
func (s *WeatherService) fetchedForecastAt(ctx context.Context, lat, lon float64, t time.Time) (*environment.WeatherGrid, error) {
	cellLat := math.Round(lat/weatherCellDeg) * weatherCellDeg
	cellLon := math.Round(lon/weatherCellDeg) * weatherCellDeg
	key := fmt.Sprintf("%.3f,%.3f", cellLat, cellLon)

	rows, ok := s.cache.get(key)
	if !ok {
		var err error
		rows, err = s.fetchPointShared(ctx, key, cellLat, cellLon)
		if err != nil {
			return nil, err
		}
	}
	return nearestForecast(rows, t)
}

// fetchPointShared fetches one cell's forecast, coalescing concurrent callers
// for the same key into a single provider request. The upstream call runs
// detached under its own timeout so it can still populate the cache for later
// requests, but each caller stops waiting once its ctx is done. Failures are
// negatively cached for weatherNegativeTTL.
func (s *WeatherService) fetchPointShared(ctx context.Context, key string, lat, lon float64) ([]environment.WeatherGrid, error) {
	s.inflightMu.Lock()
	f, ok := s.inflight[key]
	if !ok {
		f = &forecastFetch{done: make(chan struct{})}
		s.inflight[key] = f
		go s.runPointFetch(f, key, lat, lon)
	}
	s.inflightMu.Unlock()

	select {
	case <-f.done:
		return f.rows, f.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *WeatherService) runPointFetch(f *forecastFetch, key string, lat, lon float64) {
	ctx, cancel := context.WithTimeout(context.Background(), weatherFetchTimeout)
	f.rows, f.err = s.fetcher.FetchPoint(ctx, lat, lon)
	cancel()
	if f.err == nil {
		s.cache.put(key, f.rows, weatherCacheTTL)
	} else {
		s.cache.put(key, nil, weatherNegativeTTL)
	}

	s.inflightMu.Lock()
	delete(s.inflight, key)
	s.inflightMu.Unlock()
	close(f.done)
}

// nearestForecast picks the row whose ValidAt is closest to t, within the same
// +/-1 hour window the repository uses. Returns ErrNoWeather when t falls
// outside the fetched forecast horizon.
func nearestForecast(rows []environment.WeatherGrid, t time.Time) (*environment.WeatherGrid, error) {
	var best *environment.WeatherGrid
	bestDiff := weatherMatchWindow + 1
	for i := range rows {
		diff := rows[i].ValidAt.Sub(t)
		if diff < 0 {
			diff = -diff
		}
		if diff <= weatherMatchWindow && diff < bestDiff {
			best, bestDiff = &rows[i], diff
		}
	}
	if best == nil {
		return nil, environment.ErrNoWeather
	}
	return best, nil
}

// BearingDeg computes the compass bearing from point A to point B in degrees
// (0 = north, 90 = east). Used by the routing pipeline to compute segment bearings.
func BearingDeg(lat1, lon1, lat2, lon2 float64) float64 {
	phi1 := lat1 * math.Pi / 180
	phi2 := lat2 * math.Pi / 180
	deltaLon := (lon2 - lon1) * math.Pi / 180
	y := math.Sin(deltaLon) * math.Cos(phi2)
	x := math.Cos(phi1)*math.Sin(phi2) - math.Sin(phi1)*math.Cos(phi2)*math.Cos(deltaLon)
	return math.Mod(math.Atan2(y, x)*180/math.Pi+360, 360)
}
