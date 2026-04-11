package valhalla

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
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
	Legs            []Leg
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

	costing, ok := profileCosting[profile]
	if !ok {
		costing = profileCosting[ProfileSuggested]
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
	return result, nil
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
