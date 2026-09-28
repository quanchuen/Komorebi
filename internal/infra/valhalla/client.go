package valhalla

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"time"
)

// ErrTooFewLocations is returned when fewer than 2 stops are provided.
var ErrTooFewLocations = errors.New("valhalla: at least 2 locations required")

// Location is a lat/lon coordinate to route through.
type Location struct {
	Lat float64
	Lon float64
}

// Leg is one segment of a multi-stop route (between consecutive stops).
type Leg struct {
	DistanceKm float64
	DurationS  float64
	// Shape is the decoded polyline6 geometry as [lon, lat] pairs.
	Shape [][2]float64
}

// RouteResult holds the parsed Valhalla response.
type RouteResult struct {
	Profile         RouteProfile
	TotalDistanceKm float64
	TotalDurationS  float64
	ElevationGainM  float64
	ElevationLossM  float64
	Elevation       []ElevationPoint
	Legs            []Leg
}

type ElevationPoint struct {
	DistanceM  float64
	ElevationM float64
}

// Client is an HTTP client for the Valhalla routing engine.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a Client targeting the given Valhalla base URL.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// RouteProfile controls Valhalla's costing parameters for different route styles.
type RouteProfile string

const (
	// ProfileSuggested balances greenery, speed, and bike route usage.
	ProfileSuggested RouteProfile = "suggested"
	// ProfileFast optimizes for shortest travel time, uses main roads.
	ProfileFast RouteProfile = "fast"
	// ProfileAvoidMainRoads avoids large/fast roads, prefers cycling paths and residential streets.
	ProfileAvoidMainRoads RouteProfile = "avoid_main_roads"
)

var profileCosting = map[RouteProfile]map[string]any{
	ProfileSuggested: {
		"bicycle_type":  "Road",
		"cycling_speed": 15,
		"use_roads":     0.5,
		"use_hills":     0.3,
	},
	ProfileFast: {
		"bicycle_type":  "Road",
		"cycling_speed": 20,
		"use_roads":     0.9,
		"use_hills":     0.5,
	},
	ProfileAvoidMainRoads: {
		"bicycle_type":  "Hybrid",
		"cycling_speed": 13,
		"use_roads":     0.05,
		"use_hills":     0.2,
	},
}

// maxPenaltySeconds is Valhalla's upper bound for costing penalties (kMaxPenalty, 12 h).
const maxPenaltySeconds = 12 * 60 * 60

// accessCosting keeps every profile off private land. Valhalla marks
// access=private/destination/customers/delivery/permit/residents ways as
// destination-only but only adds a 600 s penalty for entering one, so a large
// enough saving still cuts through private property. Maxing the penalties makes
// that practically never worthwhile while a ride may still start or end on a
// private way (Valhalla exempts the destination). Penalties are soft, not a
// guarantee (ADR 0002).
var accessCosting = map[string]any{
	"destination_only_penalty": maxPenaltySeconds, // entering a private or destination-only way
	"private_access_penalty":   maxPenaltySeconds, // passing a private gate or barrier
}

// Route requests a bicycle route with the given profile.
func (c *Client) Route(stops []Location, profile RouteProfile) (*RouteResult, error) {
	if len(stops) < 2 {
		return nil, ErrTooFewLocations
	}
	if profile == "" {
		profile = ProfileSuggested
	}

	locations := make([]map[string]any, len(stops))
	for i, s := range stops {
		loc := map[string]any{"lat": s.Lat, "lon": s.Lon}
		if i > 0 && i < len(stops)-1 {
			loc["type"] = "through"
		} else {
			loc["type"] = "break"
		}
		locations[i] = loc
	}

	profileOpts, ok := profileCosting[profile]
	if !ok {
		profileOpts = profileCosting[ProfileSuggested]
	}
	costing := make(map[string]any, len(profileOpts)+len(accessCosting))
	for k, v := range profileOpts {
		costing[k] = v
	}
	for k, v := range accessCosting {
		costing[k] = v
	}

	body := map[string]any{
		"locations": locations,
		"costing":   "bicycle",
		"costing_options": map[string]any{
			"bicycle": costing,
		},
		"directions_options": map[string]any{
			"units": "km",
		},
	}

	buf, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("valhalla: marshal request: %w", err)
	}

	resp, err := c.httpClient.Post(c.baseURL+"/route", "application/json", bytes.NewReader(buf))
	if err != nil {
		return nil, fmt.Errorf("valhalla: http post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errBody struct {
			Error     string `json:"error"`
			ErrorCode int    `json:"error_code"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&errBody)
		return nil, fmt.Errorf("valhalla: http %d: %s (code %d)", resp.StatusCode, errBody.Error, errBody.ErrorCode)
	}

	var raw struct {
		Trip struct {
			Summary struct {
				Length float64 `json:"length"`
				Time   float64 `json:"time"`
			} `json:"summary"`
			Legs []struct {
				Summary struct {
					Length float64 `json:"length"`
					Time   float64 `json:"time"`
				} `json:"summary"`
				Shape string `json:"shape"`
			} `json:"legs"`
		} `json:"trip"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("valhalla: decode response: %w", err)
	}

	result := &RouteResult{
		Profile:         profile,
		TotalDistanceKm: raw.Trip.Summary.Length,
		TotalDurationS:  raw.Trip.Summary.Time,
		Legs:            make([]Leg, len(raw.Trip.Legs)),
	}
	for i, l := range raw.Trip.Legs {
		result.Legs[i] = Leg{
			DistanceKm: l.Summary.Length,
			DurationS:  l.Summary.Time,
			Shape:      decodePolyline6(l.Shape),
		}
	}

	var shape [][2]float64
	for i, leg := range result.Legs {
		if i == 0 {
			shape = append(shape, leg.Shape...)
		} else if len(leg.Shape) > 0 {
			shape = append(shape, leg.Shape[1:]...)
		}
	}
	if elevation, err := c.ElevationProfile(shape); err != nil {
		// Elevation is enrichment, not a hard dependency — keep the route but
		// make the missing profile visible in logs instead of silently empty.
		log.Printf("valhalla: elevation profile unavailable: %v", err)
	} else {
		result.Elevation = elevation
		for i := 1; i < len(elevation); i++ {
			delta := elevation[i].ElevationM - elevation[i-1].ElevationM
			if delta > 0 {
				result.ElevationGainM += delta
			} else {
				result.ElevationLossM -= delta
			}
		}
	}
	return result, nil
}

// ElevationProfile returns a distance/elevation profile for the shape,
// resampled every 100 m by Valhalla's /height endpoint.
func (c *Client) ElevationProfile(shape [][2]float64) ([]ElevationPoint, error) {
	if len(shape) < 2 {
		return nil, nil
	}
	locations := make([]map[string]float64, len(shape))
	for i, coordinate := range shape {
		locations[i] = map[string]float64{"lon": coordinate[0], "lat": coordinate[1]}
	}
	body, err := json.Marshal(map[string]any{
		"shape": locations, "range": true, "resample_distance": 100, "height_precision": 1,
	})
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Post(c.baseURL+"/height", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("valhalla height: http %d", resp.StatusCode)
	}
	var raw struct {
		RangeHeight [][]*float64 `json:"range_height"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	points := make([]ElevationPoint, 0, len(raw.RangeHeight))
	for _, pair := range raw.RangeHeight {
		if len(pair) != 2 || pair[0] == nil || pair[1] == nil {
			continue
		}
		points = append(points, ElevationPoint{DistanceM: *pair[0], ElevationM: *pair[1]})
	}
	return points, nil
}

// Heights returns one elevation per input point (no resampling), aligned with
// shape. Points Valhalla has no data for come back as 0.
func (c *Client) Heights(shape [][2]float64) ([]float64, error) {
	if len(shape) == 0 {
		return nil, nil
	}
	locations := make([]map[string]float64, len(shape))
	for i, coordinate := range shape {
		locations[i] = map[string]float64{"lon": coordinate[0], "lat": coordinate[1]}
	}
	body, err := json.Marshal(map[string]any{
		"shape": locations, "height_precision": 1,
	})
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Post(c.baseURL+"/height", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("valhalla height: http %d", resp.StatusCode)
	}
	var raw struct {
		Height []*float64 `json:"height"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	if len(raw.Height) != len(shape) {
		return nil, fmt.Errorf("valhalla height: got %d heights for %d points", len(raw.Height), len(shape))
	}
	heights := make([]float64, len(raw.Height))
	for i, h := range raw.Height {
		if h != nil {
			heights[i] = *h
		}
	}
	return heights, nil
}

// decodePolyline6 decodes a Valhalla polyline6-encoded string into [lon, lat] pairs.
// Valhalla uses precision 6 (factor 1e6), encoding [lat, lon] pairs.
func decodePolyline6(encoded string) [][2]float64 {
	const factor = 1e6
	var coords [][2]float64
	var lat, lon int64
	i := 0
	for i < len(encoded) {
		lat += decodeChunk(encoded, &i)
		lon += decodeChunk(encoded, &i)
		coords = append(coords, [2]float64{
			math.Round(float64(lon)/factor*factor) / factor, // lon first (GeoJSON order)
			math.Round(float64(lat)/factor*factor) / factor,
		})
	}
	return coords
}

func decodeChunk(encoded string, i *int) int64 {
	var result int64
	var shift uint
	for *i < len(encoded) {
		b := int64(encoded[*i]) - 63
		*i++
		result |= (b & 0x1f) << shift
		shift += 5
		if b < 0x20 {
			break
		}
	}
	if result&1 != 0 {
		return ^(result >> 1)
	}
	return result >> 1
}
