ALTER TABLE environment.weather_grid DROP CONSTRAINT IF EXISTS weather_grid_wind_bearing_deg_check;
ALTER TABLE environment.weather_grid ADD CONSTRAINT weather_grid_wind_bearing_deg_check CHECK (wind_bearing_deg >= 0 AND wind_bearing_deg < 360);
