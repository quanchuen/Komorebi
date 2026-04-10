ALTER TABLE routes.route DROP CONSTRAINT IF EXISTS fk_route_creator_id;
DROP TABLE IF EXISTS community.user;
DROP TYPE IF EXISTS community.contribution_status;
