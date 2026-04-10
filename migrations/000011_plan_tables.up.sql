CREATE TYPE plan.speed_model AS ENUM ('elevation', 'flat', 'custom');
CREATE TYPE plan.stop_type AS ENUM ('manual', 'venue_resolved', 'waypoint');
CREATE TYPE plan.task_status AS ENUM ('unresolved', 'matched', 'completed');

CREATE TABLE plan.route_plan (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES community.user (id) ON DELETE CASCADE,
    departure_at TIMESTAMPTZ,
    speed_model plan.speed_model NOT NULL DEFAULT 'flat',
    shade_weight DOUBLE PRECISION NOT NULL DEFAULT 0.33 CHECK (shade_weight >= 0 AND shade_weight <= 1),
    greenery_weight DOUBLE PRECISION NOT NULL DEFAULT 0.33 CHECK (greenery_weight >= 0 AND greenery_weight <= 1),
    wind_weight DOUBLE PRECISION NOT NULL DEFAULT 0.33 CHECK (wind_weight >= 0 AND wind_weight <= 1),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_route_plan_user_id ON plan.route_plan (user_id);

CREATE TABLE plan.stop_point (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    plan_id UUID NOT NULL REFERENCES plan.route_plan (id) ON DELETE CASCADE,
    geometry geometry(POINT, 4326) NOT NULL,
    type plan.stop_type NOT NULL DEFAULT 'manual',
    sort_order INT NOT NULL DEFAULT 0,
    venue_id UUID REFERENCES environment.venue (id) ON DELETE SET NULL,
    resolved_name TEXT
);
CREATE INDEX idx_stop_point_plan_id ON plan.stop_point (plan_id);
CREATE INDEX idx_stop_point_geometry ON plan.stop_point USING GIST (geometry);

CREATE TABLE plan.plan_task (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    plan_id UUID NOT NULL REFERENCES plan.route_plan (id) ON DELETE CASCADE,
    description TEXT,
    hashtag TEXT,
    status plan.task_status NOT NULL DEFAULT 'unresolved',
    resolved_venue_id UUID REFERENCES environment.venue (id) ON DELETE SET NULL
);
CREATE INDEX idx_plan_task_plan_id ON plan.plan_task (plan_id);
