-- extract_venues.sql
-- Populates environment.venue from osm.pois after an osm2pgsql import.
-- Safe to re-run: uses INSERT ... ON CONFLICT DO UPDATE.

INSERT INTO environment.venue (osm_id, geometry, name, category, brand, osm_tags)
SELECT
    p.osm_id,
    p.geom                                          AS geometry,

    COALESCE(p.name, p.brand, p.operator, 'unnamed') AS name,

    -- Derive a single category string from OSM tags
    CASE
        WHEN p.shop      = 'convenience'        THEN 'konbini'
        WHEN p.shop      = 'supermarket'        THEN 'supermarket'
        WHEN p.shop      = 'bicycle'            THEN 'bike-shop'
        WHEN p.amenity   = 'cafe'               THEN 'cafe'
        WHEN p.amenity   = 'restaurant'         THEN 'restaurant'
        WHEN p.amenity   = 'fast_food'          THEN 'fast-food'
        WHEN p.amenity   = 'toilets'            THEN 'toilet'
        WHEN p.amenity   = 'drinking_water'     THEN 'water'
        WHEN p.amenity   = 'vending_machine'    THEN 'vending'
        WHEN p.amenity   = 'atm'                THEN 'atm'
        WHEN p.amenity   = 'bicycle_parking'    THEN 'bicycle-parking'
        WHEN p.amenity   = 'bicycle_repair_station' THEN 'bike-repair'
        WHEN p.amenity   = 'place_of_worship'   THEN 'worship'
        WHEN p.amenity   = 'rest_area'          THEN 'rest-area'
        WHEN p.leisure   = 'park'               THEN 'park'
        WHEN p.leisure   = 'hot_spring'         THEN 'onsen'
        WHEN p.highway   = 'traffic_signals'    THEN 'traffic-signal'
        ELSE 'other'
    END                                             AS category,

    p.brand                                         AS brand,
    p.tags                                          AS osm_tags

FROM osm.pois p
WHERE
    -- Exclude traffic signals from venue table (handled by environment.traffic_signal)
    p.highway IS DISTINCT FROM 'traffic_signals'

ON CONFLICT (osm_id) DO UPDATE
    SET
        geometry  = EXCLUDED.geometry,
        name      = EXCLUDED.name,
        category  = EXCLUDED.category,
        brand     = EXCLUDED.brand,
        osm_tags  = EXCLUDED.osm_tags;

-- Also populate environment.traffic_signal from osm.pois
INSERT INTO environment.traffic_signal (osm_node_id, geometry)
SELECT
    p.osm_id,
    p.geom
FROM osm.pois p
WHERE p.highway = 'traffic_signals'
ON CONFLICT (osm_node_id) DO UPDATE
    SET geometry = EXCLUDED.geometry;

-- Report counts
SELECT
    (SELECT COUNT(*) FROM environment.venue)          AS venue_count,
    (SELECT COUNT(*) FROM environment.traffic_signal) AS signal_count;
