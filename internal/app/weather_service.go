package app

import (
	"math"
	"time"

	"komorebi/internal/domain/environment"
)

// WeatherService is the application-layer facade over weather data.
type WeatherService struct {
	repo environment.WeatherRepository
}

// NewWeatherService creates a WeatherService backed by the given repository.
func NewWeatherService(repo environment.WeatherRepository) *WeatherService {
	return &WeatherService{repo: repo}
}

// AtPoint returns weather conditions at a geographic point and time.
// Returns ErrNoWeather if no data covers that point/time.
func (s *WeatherService) AtPoint(lat, lon float64, t time.Time) (*environment.WeatherGrid, error) {
	return s.repo.AtPoint(lat, lon, t)
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
