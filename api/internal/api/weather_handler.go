package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"komorebi/internal/app"
	"komorebi/internal/domain/environment"
)

// WeatherHandler serves weather condition endpoints.
type WeatherHandler struct {
	svc *app.WeatherService
}

// NewWeatherHandler creates a WeatherHandler.
func NewWeatherHandler(svc *app.WeatherService) *WeatherHandler {
	return &WeatherHandler{svc: svc}
}

// AtPoint handles GET /api/v1/weather/point?lat=&lon=&at=
// at is optional ISO-8601 (RFC3339); defaults to current time.
func (h *WeatherHandler) AtPoint(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	lat, err := strconv.ParseFloat(q.Get("lat"), 64)
	if err != nil {
		http.Error(w, "invalid lat", http.StatusBadRequest)
		return
	}
	lon, err := strconv.ParseFloat(q.Get("lon"), 64)
	if err != nil {
		http.Error(w, "invalid lon", http.StatusBadRequest)
		return
	}
	t := time.Now().UTC()
	if atStr := q.Get("at"); atStr != "" {
		t, err = time.Parse(time.RFC3339, atStr)
		if err != nil {
			http.Error(w, "invalid at (use RFC3339)", http.StatusBadRequest)
			return
		}
		t = t.UTC()
	}

	wg, err := h.svc.AtPointContext(r.Context(), lat, lon, t)
	if err != nil {
		if errors.Is(err, environment.ErrNoWeather) {
			http.Error(w, "no weather data for point/time", http.StatusNotFound)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	type response struct {
		ValidAt            time.Time `json:"valid_at"`
		WindSpeedMS        float64   `json:"wind_speed_ms"`
		WindBearingDeg     float64   `json:"wind_bearing_deg"`
		PrecipIntensityMMH float64   `json:"precip_intensity_mmh"`
		TemperatureC       float64   `json:"temperature_c"`
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response{
		ValidAt:            wg.ValidAt,
		WindSpeedMS:        wg.WindSpeedMS,
		WindBearingDeg:     wg.WindBearingDeg,
		PrecipIntensityMMH: wg.PrecipIntensityMMH,
		TemperatureC:       wg.TemperatureC,
	})
}

type weatherGridCellJSON struct {
	MinLon             float64 `json:"min_lon"`
	MinLat             float64 `json:"min_lat"`
	MaxLon             float64 `json:"max_lon"`
	MaxLat             float64 `json:"max_lat"`
	PrecipIntensityMMH float64 `json:"precip_intensity_mmh"`
	WindSpeedMS        float64 `json:"wind_speed_ms"`
	WindBearingDeg     float64 `json:"wind_bearing_deg"`
	TemperatureC       float64 `json:"temperature_c"`
	UVIndex            float64 `json:"uv_index"`
	PrecipSource       string  `json:"precip_source"`
}

type weatherGridResponse struct {
	ValidAt time.Time             `json:"valid_at"`
	Cells   []weatherGridCellJSON `json:"cells"`
}

// Grid handles GET /api/v1/weather/grid?bbox=minLon,minLat,maxLon,maxLat&at=
// at is optional RFC3339; defaults to current time. An empty grid is 200 with
// no cells and valid_at echoing the requested time.
func (h *WeatherHandler) Grid(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	bboxStr := q.Get("bbox")
	if bboxStr == "" {
		writeError(w, http.StatusBadRequest, "bbox parameter required (minLon,minLat,maxLon,maxLat)")
		return
	}
	var bbox [4]float64
	if _, err := fmt.Sscanf(bboxStr, "%f,%f,%f,%f", &bbox[0], &bbox[1], &bbox[2], &bbox[3]); err != nil {
		writeError(w, http.StatusBadRequest, "bbox must be minLon,minLat,maxLon,maxLat")
		return
	}

	at := time.Now().UTC()
	if atStr := q.Get("at"); atStr != "" {
		parsed, err := time.Parse(time.RFC3339, atStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "at must be RFC3339 format")
			return
		}
		at = parsed.UTC()
	}

	snap, err := h.svc.GridSnapshot(r.Context(), bbox, at)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load weather grid")
		return
	}

	cells := make([]weatherGridCellJSON, len(snap.Cells))
	for i, c := range snap.Cells {
		cells[i] = weatherGridCellJSON{
			MinLon:             c.MinLon,
			MinLat:             c.MinLat,
			MaxLon:             c.MaxLon,
			MaxLat:             c.MaxLat,
			PrecipIntensityMMH: c.PrecipIntensityMMH,
			WindSpeedMS:        c.WindSpeedMS,
			WindBearingDeg:     c.WindBearingDeg,
			TemperatureC:       c.TemperatureC,
			UVIndex:            c.UVIndex,
			PrecipSource:       c.PrecipSource,
		}
	}
	writeJSON(w, http.StatusOK, weatherGridResponse{ValidAt: snap.ValidAt, Cells: cells})
}
