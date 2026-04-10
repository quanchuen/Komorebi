CREATE TABLE environment.green_wave (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    osm_way_ids BIGINT[] NOT NULL,
    direction_bearing DOUBLE PRECISION NOT NULL CHECK (direction_bearing >= 0 AND direction_bearing < 360),
    target_speed_kmh DOUBLE PRECISION NOT NULL,
    confidence DOUBLE PRECISION NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
    source environment.green_wave_source NOT NULL,
    detected_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_green_wave_osm_way_ids ON environment.green_wave USING GIN (osm_way_ids);

CREATE TABLE environment.venue (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    osm_id BIGINT UNIQUE,
    geometry geometry(POINT, 4326) NOT NULL,
    name TEXT NOT NULL,
    category TEXT NOT NULL,
    brand TEXT,
    osm_tags JSONB
);
CREATE INDEX idx_venue_geometry ON environment.venue USING GIST (geometry);
CREATE INDEX idx_venue_osm_tags ON environment.venue USING GIN (osm_tags);

CREATE TABLE environment.venue_tag_mapping (
    hashtag TEXT PRIMARY KEY,
    osm_filter JSONB NOT NULL,
    description TEXT,
    is_brand BOOLEAN NOT NULL DEFAULT false
);
