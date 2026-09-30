-- Featured routes are the house picks shown as "Suggested routes" in the
-- resting map view, independent of the viewport. Discovery's suggested query
-- reads this flag instead of a proximity radius.
ALTER TABLE routes.route
    ADD COLUMN featured boolean NOT NULL DEFAULT false;

CREATE INDEX route_featured_idx ON routes.route (featured) WHERE featured;

-- The current curated Tokyo routes become the initial featured set. Demo
-- sketches stay out, as they do from every discovery query.
UPDATE routes.route r
SET featured = true
WHERE r.status = 'published'
  AND NOT EXISTS (
      SELECT 1 FROM routes.route_tag rt WHERE rt.route_id = r.id AND rt.tag = 'demo'
  );
