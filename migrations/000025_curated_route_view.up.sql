-- Martin publishes this view instead of routes.route, so the map tiles hide
-- exactly what discovery hides: unpublished routes and the demo sketches.
CREATE VIEW routes.curated_route AS
SELECT r.id, r.name, r.difficulty, r.status, r.distance_m, r.geometry
FROM routes.route r
WHERE r.status = 'published'
  AND NOT EXISTS (
      SELECT 1 FROM routes.route_tag rt WHERE rt.route_id = r.id AND rt.tag = 'demo'
  );
