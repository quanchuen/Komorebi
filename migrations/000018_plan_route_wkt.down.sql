ALTER TABLE plan.route_plan
    DROP COLUMN IF EXISTS route_wkt,
    DROP COLUMN IF EXISTS updated_at;
