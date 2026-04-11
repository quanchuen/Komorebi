-- migrations/000016_weather_grid_indexes.up.sql

-- Unique constraint on (centroid of cell, valid_at) for upsert semantics.
-- We represent centroid as the rounded lat/lon used as the grid key.
-- Because PostGIS doesn't support functional unique indexes on geometry expressions
-- without an immutable function, we store grid_lat and grid_lon as generated columns.
ALTER TABLE environment.weather_grid
    ADD COLUMN IF NOT EXISTS grid_lat DOUBLE PRECISION
        GENERATED ALWAYS AS (ST_Y(ST_Centroid(cell_geometry))) STORED,
    ADD COLUMN IF NOT EXISTS grid_lon DOUBLE PRECISION
        GENERATED ALWAYS AS (ST_X(ST_Centroid(cell_geometry))) STORED;

CREATE UNIQUE INDEX IF NOT EXISTS uidx_weather_grid_point_time
    ON environment.weather_grid (grid_lat, grid_lon, valid_at);

-- Index on valid_at for time-range queries in the point-lookup query.
CREATE INDEX IF NOT EXISTS idx_weather_grid_valid_at
    ON environment.weather_grid (valid_at);
