CREATE TYPE community.contribution_status AS ENUM ('pending', 'approved', 'rejected');

CREATE TABLE community.user (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    display_name TEXT NOT NULL,
    email TEXT NOT NULL UNIQUE,
    avatar_url TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE routes.route
    ADD CONSTRAINT fk_route_creator_id
    FOREIGN KEY (creator_id) REFERENCES community.user (id) ON DELETE SET NULL;
