CREATE TABLE community.contribution (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES community.user (id) ON DELETE CASCADE,
    route_id UUID REFERENCES routes.route (id) ON DELETE SET NULL,
    route_geometry geometry(LINESTRINGZ, 4326),
    metadata JSONB,
    status community.contribution_status NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_contribution_user_id ON community.contribution (user_id);
CREATE INDEX idx_contribution_route_id ON community.contribution (route_id);
CREATE INDEX idx_contribution_route_geometry ON community.contribution USING GIST (route_geometry);

CREATE TABLE community.review (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES community.user (id) ON DELETE CASCADE,
    route_id UUID NOT NULL REFERENCES routes.route (id) ON DELETE CASCADE,
    rating INT NOT NULL CHECK (rating >= 1 AND rating <= 5),
    body TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_review_user_id ON community.review (user_id);
CREATE INDEX idx_review_route_id ON community.review (route_id);

CREATE TABLE community.ride_log (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES community.user (id) ON DELETE CASCADE,
    route_id UUID REFERENCES routes.route (id) ON DELETE SET NULL,
    gpx_track geometry(LINESTRINGZ, 4326),
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_ride_log_user_id ON community.ride_log (user_id);
CREATE INDEX idx_ride_log_route_id ON community.ride_log (route_id);
CREATE INDEX idx_ride_log_gpx_track ON community.ride_log USING GIST (gpx_track);
