CREATE TABLE environment.weather_grid (
    id BIGSERIAL PRIMARY KEY,
    cell_geometry geometry(POLYGON, 4326) NOT NULL,
    valid_at TIMESTAMPTZ NOT NULL,
    wind_speed_ms DOUBLE PRECISION,
    wind_bearing_deg DOUBLE PRECISION CHECK (wind_bearing_deg >= 0 AND wind_bearing_deg < 360),
    precip_intensity_mmh DOUBLE PRECISION,
    temperature_c DOUBLE PRECISION
);
CREATE INDEX idx_weather_grid_cell_geometry ON environment.weather_grid USING GIST (cell_geometry);
CREATE INDEX idx_weather_grid_valid_at ON environment.weather_grid (valid_at);

CREATE TABLE environment.traffic_signal (
    osm_node_id BIGINT PRIMARY KEY,
    geometry geometry(POINT, 4326) NOT NULL
);
CREATE INDEX idx_traffic_signal_geometry ON environment.traffic_signal USING GIST (geometry);
