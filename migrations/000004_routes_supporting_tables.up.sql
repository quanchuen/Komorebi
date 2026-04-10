CREATE TABLE routes.waypoint (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    route_id UUID NOT NULL REFERENCES routes.route (id) ON DELETE CASCADE,
    geometry geometry(POINT, 4326) NOT NULL,
    name TEXT,
    type routes.waypoint_type NOT NULL DEFAULT 'other',
    sort_order INT NOT NULL DEFAULT 0
);
CREATE INDEX idx_waypoint_route_id ON routes.waypoint (route_id);
CREATE INDEX idx_waypoint_geometry ON routes.waypoint USING GIST (geometry);

CREATE TABLE routes.route_segment (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    route_id UUID NOT NULL REFERENCES routes.route (id) ON DELETE CASCADE,
    geometry geometry(LINESTRINGZ, 4326) NOT NULL,
    surface_type routes.surface_type NOT NULL DEFAULT 'paved',
    grade_percent DOUBLE PRECISION,
    segment_order INT NOT NULL DEFAULT 0
);
CREATE INDEX idx_route_segment_route_id ON routes.route_segment (route_id);
CREATE INDEX idx_route_segment_geometry ON routes.route_segment USING GIST (geometry);

CREATE TABLE routes.route_tag (
    route_id UUID NOT NULL REFERENCES routes.route (id) ON DELETE CASCADE,
    tag TEXT NOT NULL,
    PRIMARY KEY (route_id, tag)
);
CREATE INDEX idx_route_tag_tag ON routes.route_tag (tag);
