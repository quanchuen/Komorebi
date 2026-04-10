CREATE TABLE environment.shadow_grid (
    id BIGSERIAL PRIMARY KEY,
    cell_geometry geometry(POLYGON, 4326) NOT NULL,
    hour_slot INT NOT NULL CHECK (hour_slot >= 0 AND hour_slot <= 23),
    month INT NOT NULL CHECK (month >= 1 AND month <= 12),
    shade_coverage DOUBLE PRECISION NOT NULL CHECK (shade_coverage >= 0 AND shade_coverage <= 1)
);
CREATE INDEX idx_shadow_grid_cell_geometry ON environment.shadow_grid USING GIST (cell_geometry);
CREATE INDEX idx_shadow_grid_time ON environment.shadow_grid (hour_slot, month);

CREATE TABLE environment.greenery_edge (
    osm_way_id BIGINT PRIMARY KEY,
    greenery_score DOUBLE PRECISION NOT NULL CHECK (greenery_score >= 0 AND greenery_score <= 1),
    tree_lined BOOLEAN NOT NULL DEFAULT false,
    park_adjacent BOOLEAN NOT NULL DEFAULT false
);
