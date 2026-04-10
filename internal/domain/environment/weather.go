package environment

import "time"

// WeatherGrid stores weather conditions for a map cell at a given time.
type WeatherGrid struct {
	ID                   string
	CellGeometry         [][2]float64
	ValidAt              time.Time
	WindSpeedMS          float64
	WindBearingDeg       float64
	PrecipIntensityMMH   float64
	TemperatureC         float64
}
