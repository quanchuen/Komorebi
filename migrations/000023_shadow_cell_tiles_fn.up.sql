-- Martin function source serving environment.shadow_grid as MVT, filtered by
-- the requested hour and month so the map's time slider can swap tile slices.
-- query_params: hour_slot (0-23, JST — matches how the pipeline stores slots)
-- and month (1-12). The month is snapped to the nearest month that actually
-- has data, because the shadow pipeline only computes representative months.
-- Tile queries filter one (hour_slot, month) slice by tile bbox. The existing
-- separate btree/gist indexes each leave ~78k-row slice scans per tile
-- (~550ms); the composite gist index makes them bbox-local.
CREATE EXTENSION IF NOT EXISTS btree_gist;
CREATE INDEX IF NOT EXISTS idx_shadow_grid_slice_geom
    ON environment.shadow_grid USING gist (hour_slot, month, cell_geometry);
-- Backs the loose index scan that enumerates which months have data.
CREATE INDEX IF NOT EXISTS idx_shadow_grid_month
    ON environment.shadow_grid (month);

CREATE OR REPLACE FUNCTION environment.shadow_cells(z integer, x integer, y integer, query_params json)
RETURNS bytea AS $$
DECLARE
    want_hour  integer := least(23, greatest(0, coalesce((query_params->>'hour_slot')::integer, 12)));
    want_month integer := least(12, greatest(1, coalesce((query_params->>'month')::integer, 7)));
    use_month  integer;
    tile_bounds geometry := ST_TileEnvelope(z, x, y);
    result      bytea;
BEGIN
    -- Loose index scan: a plain DISTINCT would walk all ~7.5M index entries.
    WITH RECURSIVE months(m) AS (
        SELECT min(month) FROM environment.shadow_grid
        UNION ALL
        SELECT (SELECT min(month) FROM environment.shadow_grid WHERE month > months.m)
        FROM months WHERE months.m IS NOT NULL
    )
    SELECT m INTO use_month
    FROM months
    WHERE m IS NOT NULL
    ORDER BY least(abs(m - want_month), 12 - abs(m - want_month)), m
    LIMIT 1;

    IF use_month IS NULL THEN
        RETURN NULL;
    END IF;

    SELECT INTO result
        ST_AsMVT(q, 'shadow_cells', 4096, 'mvt_geom')
    FROM (
        SELECT
            shade_coverage,
            ST_AsMVTGeom(
                ST_Transform(cell_geometry, 3857),
                tile_bounds,
                4096, 8, true
            ) AS mvt_geom
        FROM environment.shadow_grid
        WHERE
            hour_slot = want_hour
            AND month = use_month
            AND cell_geometry && ST_Transform(tile_bounds, 4326)
            -- Unshaded cells dominate most slices; dropping them keeps tiles small.
            AND shade_coverage >= 0.05
    ) q
    WHERE mvt_geom IS NOT NULL;

    RETURN result;
END;
$$ LANGUAGE plpgsql STABLE PARALLEL SAFE;
