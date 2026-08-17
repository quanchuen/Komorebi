-- The original development seeds use sparse straight-line sketches between
-- landmarks. Keep them for integration fixtures, but never advertise them as
-- rideable routes through discovery.
INSERT INTO routes.route_tag (route_id, tag)
SELECT id, 'demo'
FROM routes.route
WHERE id IN (
    '10000000-0000-0000-0000-000000000001',
    '10000000-0000-0000-0000-000000000002',
    '10000000-0000-0000-0000-000000000003',
    '10000000-0000-0000-0000-000000000004',
    '10000000-0000-0000-0000-000000000005'
)
ON CONFLICT (route_id, tag) DO NOTHING;
