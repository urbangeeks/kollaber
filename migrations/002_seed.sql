-- Seed user (password: "password")
INSERT INTO users (id, email, password_hash)
VALUES (
    '00000000-0000-0000-0000-000000000001',
    'admin@kollaber.dev',
    '$2a$10$ab6xHGgBrEkzJHdRmGFUqevLDuDvabXtuELJXhbRgFuQD4uAuEUXi'
) ON CONFLICT DO NOTHING;

-- Seed environment
INSERT INTO environments (id, name, cluster_name)
VALUES (
    '00000000-0000-0000-0000-000000000010',
    'prod',
    'prod-cluster'
) ON CONFLICT DO NOTHING;

-- Seed events
INSERT INTO events (id, type, service, environment_id, timestamp, metadata)
VALUES
(
    '00000000-0000-0000-0000-000000000100',
    'deploy',
    'api-service',
    '00000000-0000-0000-0000-000000000010',
    NOW() - INTERVAL '30 minutes',
    '{"version": "v1.2.3", "author": "jerome"}'
),
(
    '00000000-0000-0000-0000-000000000101',
    'alert',
    'api-service',
    '00000000-0000-0000-0000-000000000010',
    NOW() - INTERVAL '25 minutes',
    '{"severity": "high", "message": "5xx spike detected"}'
),
(
    '00000000-0000-0000-0000-000000000102',
    'note',
    'api-service',
    '00000000-0000-0000-0000-000000000010',
    NOW() - INTERVAL '20 minutes',
    '{"body": "Investigating latency spike, may be related to deploy"}'
) ON CONFLICT DO NOTHING;

-- Seed comments
INSERT INTO comments (event_id, user_id, body)
VALUES
(
    '00000000-0000-0000-0000-000000000100',
    '00000000-0000-0000-0000-000000000001',
    'Added new auth flow'
),
(
    '00000000-0000-0000-0000-000000000101',
    '00000000-0000-0000-0000-000000000001',
    'Investigating'
),
(
    '00000000-0000-0000-0000-000000000101',
    '00000000-0000-0000-0000-000000000001',
    'Rolling back'
) ON CONFLICT DO NOTHING;
