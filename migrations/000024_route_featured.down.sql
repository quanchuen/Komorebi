DROP INDEX IF EXISTS routes.route_featured_idx;
ALTER TABLE routes.route DROP COLUMN IF EXISTS featured;
