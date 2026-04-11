-- Seed user
INSERT INTO community.user (id, display_name, email)
VALUES ('00000000-0000-0000-0000-000000000001', 'Cyclist Map Team', 'team@cyclist-map.dev')
ON CONFLICT (id) DO NOTHING;

-- ============================================================
-- Route 1: Tama River Cycling Path (多摩川サイクリングロード)
-- ============================================================
INSERT INTO routes.route (id, name, description, geometry, distance_m, elevation_gain_m, elevation_loss_m, difficulty, status, creator_id)
VALUES (
    '10000000-0000-0000-0000-000000000001',
    'Tama River Cycling Path',
    '多摩川サイクリングロード — A flat 40km riverside ride along the Tama River from Haneda to Hamura.',
    ST_SetSRID(ST_GeomFromText('LINESTRING Z(139.738 35.553 5, 139.650 35.590 5, 139.540 35.640 8, 139.430 35.695 10, 139.310 35.740 12)'), 4326),
    40200,
    30,
    30,
    'easy',
    'published',
    '00000000-0000-0000-0000-000000000001'
);

INSERT INTO routes.waypoint (id, route_id, geometry, name, type, sort_order) VALUES
    ('20000000-0000-0000-0000-000000000001', '10000000-0000-0000-0000-000000000001', ST_SetSRID(ST_GeomFromText('POINT(139.738 35.553)'), 4326), 'Haneda River Mouth (Start)', 'other', 0),
    ('20000000-0000-0000-0000-000000000002', '10000000-0000-0000-0000-000000000001', ST_SetSRID(ST_GeomFromText('POINT(139.540 35.640)'), 4326), 'Fuchu Rest Area', 'rest_stop', 1),
    ('20000000-0000-0000-0000-000000000003', '10000000-0000-0000-0000-000000000001', ST_SetSRID(ST_GeomFromText('POINT(139.430 35.695)'), 4326), 'Tama River Water Point', 'water', 2),
    ('20000000-0000-0000-0000-000000000004', '10000000-0000-0000-0000-000000000001', ST_SetSRID(ST_GeomFromText('POINT(139.310 35.740)'), 4326), 'Hamura Weir (End)', 'other', 3);

INSERT INTO routes.route_segment (id, route_id, geometry, surface_type, grade_percent, segment_order) VALUES
    ('30000000-0000-0000-0000-000000000001', '10000000-0000-0000-0000-000000000001',
     ST_SetSRID(ST_GeomFromText('LINESTRING Z(139.738 35.553 5, 139.650 35.590 5, 139.540 35.640 8)'), 4326),
     'paved', 0.1, 0),
    ('30000000-0000-0000-0000-000000000002', '10000000-0000-0000-0000-000000000001',
     ST_SetSRID(ST_GeomFromText('LINESTRING Z(139.540 35.640 8, 139.430 35.695 10)'), 4326),
     'paved', 0.1, 1),
    ('30000000-0000-0000-0000-000000000003', '10000000-0000-0000-0000-000000000001',
     ST_SetSRID(ST_GeomFromText('LINESTRING Z(139.430 35.695 10, 139.310 35.740 12)'), 4326),
     'paved', 0.1, 2);

INSERT INTO routes.route_tag (route_id, tag) VALUES
    ('10000000-0000-0000-0000-000000000001', 'river'),
    ('10000000-0000-0000-0000-000000000001', 'flat'),
    ('10000000-0000-0000-0000-000000000001', 'beginner-friendly'),
    ('10000000-0000-0000-0000-000000000001', 'long-ride');

-- ============================================================
-- Route 2: Imperial Palace Loop (皇居一周)
-- ============================================================
INSERT INTO routes.route (id, name, description, geometry, distance_m, elevation_gain_m, elevation_loss_m, difficulty, status, creator_id)
VALUES (
    '10000000-0000-0000-0000-000000000002',
    'Imperial Palace Loop',
    '皇居一周 — The classic 5km loop around the Imperial Palace, Tokyo''s most iconic urban cycling route.',
    ST_SetSRID(ST_GeomFromText('LINESTRING Z(139.756 35.6825 20, 139.749 35.686 20, 139.745 35.681 20, 139.750 35.677 20, 139.757 35.678 20, 139.756 35.6825 20)'), 4326),
    5000,
    10,
    10,
    'easy',
    'published',
    '00000000-0000-0000-0000-000000000001'
);

INSERT INTO routes.waypoint (id, route_id, geometry, name, type, sort_order) VALUES
    ('20000000-0000-0000-0000-000000000005', '10000000-0000-0000-0000-000000000002', ST_SetSRID(ST_GeomFromText('POINT(139.756 35.6825)'), 4326), 'Babasaki Gate (Start/End)', 'other', 0),
    ('20000000-0000-0000-0000-000000000006', '10000000-0000-0000-0000-000000000002', ST_SetSRID(ST_GeomFromText('POINT(139.745 35.681)'), 4326), 'Hanzomon Station Rest', 'rest_stop', 1),
    ('20000000-0000-0000-0000-000000000007', '10000000-0000-0000-0000-000000000002', ST_SetSRID(ST_GeomFromText('POINT(139.750 35.677)'), 4326), 'Sakuradamon Gate Viewpoint', 'viewpoint', 2);

INSERT INTO routes.route_segment (id, route_id, geometry, surface_type, grade_percent, segment_order) VALUES
    ('30000000-0000-0000-0000-000000000004', '10000000-0000-0000-0000-000000000002',
     ST_SetSRID(ST_GeomFromText('LINESTRING Z(139.756 35.6825 20, 139.749 35.686 20, 139.745 35.681 20)'), 4326),
     'paved', 0.0, 0),
    ('30000000-0000-0000-0000-000000000005', '10000000-0000-0000-0000-000000000002',
     ST_SetSRID(ST_GeomFromText('LINESTRING Z(139.745 35.681 20, 139.750 35.677 20, 139.757 35.678 20, 139.756 35.6825 20)'), 4326),
     'paved', 0.0, 1);

INSERT INTO routes.route_tag (route_id, tag) VALUES
    ('10000000-0000-0000-0000-000000000002', 'loop'),
    ('10000000-0000-0000-0000-000000000002', 'urban'),
    ('10000000-0000-0000-0000-000000000002', 'beginner-friendly'),
    ('10000000-0000-0000-0000-000000000002', 'iconic');

-- ============================================================
-- Route 3: Arakawa River to Tokyo Bay (荒川下流)
-- ============================================================
INSERT INTO routes.route (id, name, description, geometry, distance_m, elevation_gain_m, elevation_loss_m, difficulty, status, creator_id)
VALUES (
    '10000000-0000-0000-0000-000000000003',
    'Arakawa River to Tokyo Bay',
    '荒川下流 — A 25km flat riverside ride downstream along the Arakawa to Kasai Rinkai Park and Tokyo Bay.',
    ST_SetSRID(ST_GeomFromText('LINESTRING Z(139.784 35.780 5, 139.800 35.740 4, 139.820 35.700 3, 139.847 35.650 2)'), 4326),
    25300,
    5,
    15,
    'easy',
    'published',
    '00000000-0000-0000-0000-000000000001'
);

INSERT INTO routes.waypoint (id, route_id, geometry, name, type, sort_order) VALUES
    ('20000000-0000-0000-0000-000000000008', '10000000-0000-0000-0000-000000000003', ST_SetSRID(ST_GeomFromText('POINT(139.784 35.780)'), 4326), 'Kita-Senju Bridge (Start)', 'other', 0),
    ('20000000-0000-0000-0000-000000000009', '10000000-0000-0000-0000-000000000003', ST_SetSRID(ST_GeomFromText('POINT(139.800 35.740)'), 4326), 'Arakawa Rest House', 'rest_stop', 1),
    ('20000000-0000-0000-0000-000000000010', '10000000-0000-0000-0000-000000000003', ST_SetSRID(ST_GeomFromText('POINT(139.820 35.700)'), 4326), 'Riverside Water Fountain', 'water', 2),
    ('20000000-0000-0000-0000-000000000011', '10000000-0000-0000-0000-000000000003', ST_SetSRID(ST_GeomFromText('POINT(139.847 35.650)'), 4326), 'Kasai Rinkai Park (End)', 'viewpoint', 3);

INSERT INTO routes.route_segment (id, route_id, geometry, surface_type, grade_percent, segment_order) VALUES
    ('30000000-0000-0000-0000-000000000006', '10000000-0000-0000-0000-000000000003',
     ST_SetSRID(ST_GeomFromText('LINESTRING Z(139.784 35.780 5, 139.800 35.740 4, 139.820 35.700 3)'), 4326),
     'paved', -0.1, 0),
    ('30000000-0000-0000-0000-000000000007', '10000000-0000-0000-0000-000000000003',
     ST_SetSRID(ST_GeomFromText('LINESTRING Z(139.820 35.700 3, 139.847 35.650 2)'), 4326),
     'paved', -0.1, 1);

INSERT INTO routes.route_tag (route_id, tag) VALUES
    ('10000000-0000-0000-0000-000000000003', 'river'),
    ('10000000-0000-0000-0000-000000000003', 'flat'),
    ('10000000-0000-0000-0000-000000000003', 'seaside'),
    ('10000000-0000-0000-0000-000000000003', 'beginner-friendly');

-- ============================================================
-- Route 4: Ome to Okutama Hill Climb (青梅・奥多摩ヒルクライム)
-- ============================================================
INSERT INTO routes.route (id, name, description, geometry, distance_m, elevation_gain_m, elevation_loss_m, difficulty, status, creator_id)
VALUES (
    '10000000-0000-0000-0000-000000000004',
    'Ome to Okutama Hill Climb',
    '青梅・奥多摩ヒルクライム — A challenging 35km mountain climb from Ome (185m) up to Lake Okutama (530m) through scenic valleys and forests.',
    ST_SetSRID(ST_GeomFromText('LINESTRING Z(139.275 35.788 185, 139.200 35.800 280, 139.120 35.810 380, 139.055 35.825 530)'), 4326),
    35400,
    370,
    25,
    'hard',
    'published',
    '00000000-0000-0000-0000-000000000001'
);

INSERT INTO routes.waypoint (id, route_id, geometry, name, type, sort_order) VALUES
    ('20000000-0000-0000-0000-000000000012', '10000000-0000-0000-0000-000000000004', ST_SetSRID(ST_GeomFromText('POINT(139.275 35.788)'), 4326), 'Ome Station (Start)', 'other', 0),
    ('20000000-0000-0000-0000-000000000013', '10000000-0000-0000-0000-000000000004', ST_SetSRID(ST_GeomFromText('POINT(139.200 35.800)'), 4326), 'Mitake Valley Shrine', 'shrine', 1),
    ('20000000-0000-0000-0000-000000000014', '10000000-0000-0000-0000-000000000004', ST_SetSRID(ST_GeomFromText('POINT(139.120 35.810)'), 4326), 'Mountain Pass Konbini', 'konbini', 2),
    ('20000000-0000-0000-0000-000000000015', '10000000-0000-0000-0000-000000000004', ST_SetSRID(ST_GeomFromText('POINT(139.055 35.825)'), 4326), 'Lake Okutama Dam (End)', 'viewpoint', 3);

INSERT INTO routes.route_segment (id, route_id, geometry, surface_type, grade_percent, segment_order) VALUES
    ('30000000-0000-0000-0000-000000000008', '10000000-0000-0000-0000-000000000004',
     ST_SetSRID(ST_GeomFromText('LINESTRING Z(139.275 35.788 185, 139.200 35.800 280)'), 4326),
     'paved', 2.5, 0),
    ('30000000-0000-0000-0000-000000000009', '10000000-0000-0000-0000-000000000004',
     ST_SetSRID(ST_GeomFromText('LINESTRING Z(139.200 35.800 280, 139.120 35.810 380)'), 4326),
     'paved', 3.1, 1),
    ('30000000-0000-0000-0000-000000000010', '10000000-0000-0000-0000-000000000004',
     ST_SetSRID(ST_GeomFromText('LINESTRING Z(139.120 35.810 380, 139.055 35.825 530)'), 4326),
     'paved', 4.2, 2);

INSERT INTO routes.route_tag (route_id, tag) VALUES
    ('10000000-0000-0000-0000-000000000004', 'hill-climb'),
    ('10000000-0000-0000-0000-000000000004', 'mountain'),
    ('10000000-0000-0000-0000-000000000004', 'scenic'),
    ('10000000-0000-0000-0000-000000000004', 'challenging');

-- ============================================================
-- Route 5: Edogawa River Path (江戸川サイクリングロード)
-- ============================================================
INSERT INTO routes.route (id, name, description, geometry, distance_m, elevation_gain_m, elevation_loss_m, difficulty, status, creator_id)
VALUES (
    '10000000-0000-0000-0000-000000000005',
    'Edogawa River Path',
    '江戸川サイクリングロード — A peaceful 20km flat ride along the Edogawa River, perfect for a sunset ride toward Tokyo Bay.',
    ST_SetSRID(ST_GeomFromText('LINESTRING Z(139.895 35.780 5, 139.900 35.730 4, 139.905 35.680 3)'), 4326),
    20100,
    5,
    10,
    'easy',
    'published',
    '00000000-0000-0000-0000-000000000001'
);

INSERT INTO routes.waypoint (id, route_id, geometry, name, type, sort_order) VALUES
    ('20000000-0000-0000-0000-000000000016', '10000000-0000-0000-0000-000000000005', ST_SetSRID(ST_GeomFromText('POINT(139.895 35.780)'), 4326), 'Matsudo Bridge (Start)', 'other', 0),
    ('20000000-0000-0000-0000-000000000017', '10000000-0000-0000-0000-000000000005', ST_SetSRID(ST_GeomFromText('POINT(139.900 35.730)'), 4326), 'Edogawa Riverside Rest Stop', 'rest_stop', 1),
    ('20000000-0000-0000-0000-000000000018', '10000000-0000-0000-0000-000000000005', ST_SetSRID(ST_GeomFromText('POINT(139.905 35.680)'), 4326), 'Tokyo Bay Sunset Viewpoint (End)', 'viewpoint', 2);

INSERT INTO routes.route_segment (id, route_id, geometry, surface_type, grade_percent, segment_order) VALUES
    ('30000000-0000-0000-0000-000000000011', '10000000-0000-0000-0000-000000000005',
     ST_SetSRID(ST_GeomFromText('LINESTRING Z(139.895 35.780 5, 139.900 35.730 4)'), 4326),
     'paved', -0.1, 0),
    ('30000000-0000-0000-0000-000000000012', '10000000-0000-0000-0000-000000000005',
     ST_SetSRID(ST_GeomFromText('LINESTRING Z(139.900 35.730 4, 139.905 35.680 3)'), 4326),
     'paved', -0.1, 1);

INSERT INTO routes.route_tag (route_id, tag) VALUES
    ('10000000-0000-0000-0000-000000000005', 'river'),
    ('10000000-0000-0000-0000-000000000005', 'flat'),
    ('10000000-0000-0000-0000-000000000005', 'beginner-friendly'),
    ('10000000-0000-0000-0000-000000000005', 'sunset');
