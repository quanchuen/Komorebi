CREATE TYPE routes.difficulty AS ENUM ('easy', 'moderate', 'hard', 'expert');
CREATE TYPE routes.route_status AS ENUM ('draft', 'published', 'archived');
CREATE TYPE routes.waypoint_type AS ENUM ('viewpoint', 'rest_stop', 'water', 'shrine', 'konbini', 'other');
CREATE TYPE routes.surface_type AS ENUM ('paved', 'gravel', 'dirt', 'cobblestone');
