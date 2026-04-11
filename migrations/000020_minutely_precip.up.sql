CREATE TABLE environment.minutely_precip (
    id                BIGSERIAL PRIMARY KEY,
    lat               DOUBLE PRECISION NOT NULL,
    lon               DOUBLE PRECISION NOT NULL,
    at                TIMESTAMPTZ NOT NULL,
    intensity_mmh     DOUBLE PRECISION NOT NULL DEFAULT 0,
    fetched_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_minutely_precip_location ON environment.minutely_precip (lat, lon);
CREATE INDEX idx_minutely_precip_at ON environment.minutely_precip (at);
CREATE UNIQUE INDEX idx_minutely_precip_unique ON environment.minutely_precip (lat, lon, at);
