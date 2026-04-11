CREATE OR REPLACE FUNCTION public.osm_cycling_tiles(z integer, x integer, y integer)
RETURNS bytea AS $$
DECLARE
    tile_bounds geometry;
    result      bytea;
BEGIN
    tile_bounds := ST_TileEnvelope(z, x, y);

    SELECT INTO result
        ST_AsMVT(q, 'osm_cycling', 4096, 'mvt_geom')
    FROM (
        SELECT
            osm_id,
            name,
            name_en,
            highway,
            cycleway,
            cycleway_left,
            cycleway_right,
            bicycle,
            surface,
            smoothness,
            oneway,
            lit,
            ST_AsMVTGeom(
                ST_Transform(geom, 3857),
                ST_Transform(tile_bounds, 3857),
                4096, 64, true
            ) AS mvt_geom
        FROM osm.roads
        WHERE
            geom && tile_bounds
            AND (
                highway IN ('cycleway', 'path', 'track', 'footway', 'pedestrian')
                OR cycleway IS NOT NULL
                OR cycleway_left IS NOT NULL
                OR cycleway_right IS NOT NULL
                OR bicycle IN ('yes', 'designated', 'permissive')
            )
    ) q
    WHERE mvt_geom IS NOT NULL;

    RETURN result;
END;
$$ LANGUAGE plpgsql STABLE;
