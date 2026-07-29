package store

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// insertEventAt adds an event of an arbitrary type at an explicit time,
// bypassing CreateEvent (which forces timestamp = NOW()).
func insertEventAt(t *testing.T, pool *pgxpool.Pool, envID pgtype.UUID, eventType, service string, ts time.Time) pgtype.UUID {
	t.Helper()
	var id pgtype.UUID
	err := pool.QueryRow(context.Background(),
		`INSERT INTO events (type, service, environment_id, timestamp, status, metadata)
		 VALUES ($1, $2, $3, $4, 'success', '{}'::jsonb)
		 RETURNING id`,
		eventType, service, envID, ts).Scan(&id)
	if err != nil {
		t.Fatalf("insert %s event: %v", eventType, err)
	}
	return id
}

func TestListChangesBefore(t *testing.T) {
	q, pool := testStore(t)
	org := newTestOrg(t, pool)
	env := newTestEnv(t, pool, org)
	ctx := context.Background()
	now := time.Now()

	alertID := insertEventAt(t, pool, env, "alert", "api", now)

	// In-window changes, plus the noise that must not come back.
	insertEventAt(t, pool, env, "deploy", "api", now.Add(-10*time.Minute))
	insertEventAt(t, pool, env, "scale", "worker", now.Add(-30*time.Minute))
	insertEventAt(t, pool, env, "note", "api", now.Add(-15*time.Minute))  // not a change
	insertEventAt(t, pool, env, "alert", "api", now.Add(-20*time.Minute)) // not a change
	insertEventAt(t, pool, env, "deploy", "api", now.Add(-5*time.Hour))   // outside window
	insertEventAt(t, pool, env, "deploy", "api", now.Add(10*time.Minute)) // after the alert

	got, err := q.ListChangesBefore(ctx, ListChangesBeforeParams{
		OrgID:         org,
		EnvironmentID: env,
		Before:        now,
		Window:        2 * time.Hour,
		Limit:         50,
		ExcludeID:     alertID,
	})
	if err != nil {
		t.Fatalf("ListChangesBefore: %v", err)
	}

	if len(got) != 2 {
		for _, e := range got {
			t.Logf("got %s/%s at %s", e.Type, e.Service, e.Timestamp.Time)
		}
		t.Fatalf("want 2 changes, got %d", len(got))
	}

	// Newest first.
	if got[0].Type != "deploy" || got[1].Type != "scale" {
		t.Errorf("want deploy then scale (newest first), got %s then %s", got[0].Type, got[1].Type)
	}
	if got[0].Timestamp.Time.Before(got[1].Timestamp.Time) {
		t.Error("results are not ordered newest first")
	}
}

// events carries no org_id, so scoping depends entirely on the join through
// environments. A missing join here is a cross-tenant leak, not a cosmetic bug.
func TestListChangesBeforeScopesToOrg(t *testing.T) {
	q, pool := testStore(t)
	ctx := context.Background()
	now := time.Now()

	orgA := newTestOrg(t, pool)
	envA := newTestEnv(t, pool, orgA)
	orgB := newTestOrg(t, pool)
	envB := newTestEnv(t, pool, orgB)

	insertEventAt(t, pool, envA, "deploy", "api", now.Add(-5*time.Minute))
	insertEventAt(t, pool, envB, "deploy", "api", now.Add(-5*time.Minute))

	// Org A asking about org B's environment must get nothing back.
	got, err := q.ListChangesBefore(ctx, ListChangesBeforeParams{
		OrgID:         orgA,
		EnvironmentID: envB,
		Before:        now,
		Window:        2 * time.Hour,
		Limit:         50,
		ExcludeID:     pgtype.UUID{Valid: false},
	})
	if err != nil {
		t.Fatalf("ListChangesBefore: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("cross-tenant leak: org A got %d of org B's changes", len(got))
	}

	// And its own environment still works.
	got, err = q.ListChangesBefore(ctx, ListChangesBeforeParams{
		OrgID:         orgA,
		EnvironmentID: envA,
		Before:        now,
		Window:        2 * time.Hour,
		Limit:         50,
		ExcludeID:     pgtype.UUID{Valid: false},
	})
	if err != nil {
		t.Fatalf("ListChangesBefore own org: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("want 1 change in own org, got %d", len(got))
	}
}

func TestListChangesBeforeRespectsLimit(t *testing.T) {
	q, pool := testStore(t)
	org := newTestOrg(t, pool)
	env := newTestEnv(t, pool, org)
	ctx := context.Background()
	now := time.Now()

	for i := 1; i <= 5; i++ {
		insertEventAt(t, pool, env, "deploy", "api", now.Add(-time.Duration(i)*time.Minute))
	}

	got, err := q.ListChangesBefore(ctx, ListChangesBeforeParams{
		OrgID:         org,
		EnvironmentID: env,
		Before:        now,
		Window:        time.Hour,
		Limit:         3,
		ExcludeID:     pgtype.UUID{Valid: false},
	})
	if err != nil {
		t.Fatalf("ListChangesBefore: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 changes (limit), got %d", len(got))
	}
	// The limit must keep the newest, not an arbitrary three.
	if got[0].Timestamp.Time.Before(now.Add(-2 * time.Minute)) {
		t.Errorf("limit dropped the newest change; first result is at %s", got[0].Timestamp.Time)
	}
}
