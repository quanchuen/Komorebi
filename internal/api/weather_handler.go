package api

import (
	"encoding/json"
	"errors"
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

	wg, err := h.svc.AtPoint(lat, lon, t)
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
