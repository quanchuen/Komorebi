-- migrations/000016_weather_grid_indexes.down.sql
DROP INDEX IF EXISTS environment.idx_weather_grid_valid_at;
DROP INDEX IF EXISTS environment.uidx_weather_grid_point_time;
ALTER TABLE environment.weather_grid
    DROP COLUMN IF EXISTS grid_lat,
    DROP COLUMN IF EXISTS grid_lon;
