-- The queries returning a full event row are hand-written in
-- internal/store/events_core.go: the table carries a stored tsvector, and sqlc
-- reuses a table's struct only when a query selects every column, so generating
-- them would put the search vector on the wire for every row of the timeline.
-- This one returns a single text column and has no such problem.

-- name: ListServicesByEnvironment :many
SELECT DISTINCT e.service FROM events e
JOIN environments env ON env.id = e.environment_id
WHERE e.environment_id = $1 AND env.org_id = $2
ORDER BY e.service;
