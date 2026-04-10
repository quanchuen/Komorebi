CREATE TABLE routes.route (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT NOT NULL,
    description TEXT,
    geometry geometry(LINESTRINGZ, 4326) NOT NULL,
    distance_m DOUBLE PRECISION NOT NULL,
    elevation_gain_m DOUBLE PRECISION NOT NULL DEFAULT 0,
    elevation_loss_m DOUBLE PRECISION NOT NULL DEFAULT 0,
    difficulty routes.difficulty NOT NULL DEFAULT 'moderate',
    status routes.route_status NOT NULL DEFAULT 'draft',
    creator_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_route_geometry ON routes.route USING GIST (geometry);
CREATE INDEX idx_route_status ON routes.route (status);
CREATE INDEX idx_route_difficulty ON routes.route (difficulty);
CREATE INDEX idx_route_creator_id ON routes.route (creator_id);
