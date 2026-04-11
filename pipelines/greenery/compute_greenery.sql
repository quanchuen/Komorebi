-- pipelines/greenery/compute_greenery.sql
--
-- Populates environment.greenery_edge for every row in osm.roads.
-- Score formula:
--   park_adjacent (0.4) + tree_lined (0.3) + near_forest (0.2) + near_water (0.1)
--
-- Proximity radii use planar degrees (SRID 4326) to keep GiST indexes hot:
--   100 m  ≈ 0.00090°  (park_adjacent)
--   200 m  ≈ 0.00180°  (near_forest, near_water)
--
-- Casting osm.roads.geom to ::geography causes a parallel seq scan (2.3M rows,
-- no geography index). Keep all ST_DWithin calls in planar degrees.

BEGIN;

INSERT INTO environment.greenery_edge (
    osm_way_id,
    greenery_score,
    tree_lined,
    park_adjacent
)
SELECT
    r.way_id,
    LEAST(1.0,
        (CASE WHEN park.area_id IS NOT NULL THEN 0.4 ELSE 0.0 END) +
        (CASE WHEN r.tags->>'natural' = 'tree_row'    THEN 0.3 ELSE 0.0 END) +
        (CASE WHEN forest.area_id IS NOT NULL THEN 0.2 ELSE 0.0 END) +
        (CASE WHEN water.area_id  IS NOT NULL THEN 0.1 ELSE 0.0 END)
    ) AS greenery_score,
    COALESCE(r.tags->>'natural' = 'tree_row', false)  AS tree_lined,
    (park.area_id IS NOT NULL)                        AS park_adjacent
FROM osm.roads r

-- park_adjacent: road centroid within 100 m of a park polygon
LEFT JOIN LATERAL (
    SELECT l.area_id
    FROM osm.landuse l
    WHERE l.leisure = 'park'
      AND ST_DWithin(
            ST_Centroid(r.geom),
            l.geom,
            0.00090           -- ~100 m in degrees at Tokyo latitude
          )
    LIMIT 1
) park ON true

-- near_forest: road centroid within 200 m of forest or wood polygon
LEFT JOIN LATERAL (
    SELECT l.area_id
    FROM osm.landuse l
    WHERE (l.landuse IN ('forest', 'wood') OR l.natural_ = 'wood')
      AND ST_DWithin(
            ST_Centroid(r.geom),
            l.geom,
            0.00180           -- ~200 m in degrees
          )
    LIMIT 1
) forest ON true

-- near_water: road centroid within 200 m of water polygon
LEFT JOIN LATERAL (
    SELECT l.area_id
    FROM osm.landuse l
    WHERE (l.natural_ = 'water' OR l.landuse IN ('basin', 'reservoir'))
      AND ST_DWithin(
            ST_Centroid(r.geom),
            l.geom,
            0.00180
          )
    LIMIT 1
) water ON true

ON CONFLICT (osm_way_id) DO UPDATE SET
    greenery_score = EXCLUDED.greenery_score,
    tree_lined     = EXCLUDED.tree_lined,
    park_adjacent  = EXCLUDED.park_adjacent;

COMMIT;
