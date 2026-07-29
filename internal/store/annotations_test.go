package store

import (
	"context"
	"testing"
	"time"
)

// changeTypes is the default set the annotations endpoint asks for: everything
// that marks a change or a firing, which is every valid type except "note".
var changeTypes = []string{"deploy", "alert", "rollback", "scale", "teardown"}

func TestListAnnotations(t *testing.T) {
	q, pool := testStore(t)
	org := newTestOrg(t, pool)
	env := newTestEnv(t, pool, org)
	ctx := context.Background()
	now := time.Now()

	insertEventAt(t, pool, env, "deploy", "api", now.Add(-30*time.Minute))
	insertEventAt(t, pool, env, "alert", "api", now.Add(-20*time.Minute))
	insertEventAt(t, pool, env, "note", "api", now.Add(-25*time.Minute))  // excluded type
	insertEventAt(t, pool, env, "deploy", "api", now.Add(-9*time.Hour))   // before the window
	insertEventAt(t, pool, env, "deploy", "api", now.Add(30*time.Minute)) // after the window

	got, err := q.ListAnnotations(ctx, ListAnnotationsParams{
		OrgID: org,
		Types: changeTypes,
		From:  now.Add(-time.Hour),
		To:    now,
		Limit: 100,
	})
	if err != nil {
		t.Fatalf("ListAnnotations: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("want 2 annotations, got %d", len(got))
	}
	// Oldest first, so the markers read along the time axis.
	if got[0].Type != "deploy" || got[1].Type != "alert" {
		t.Errorf("not ordered oldest first: %s then %s", got[0].Type, got[1].Type)
	}
	if got[0].EnvironmentName == "" {
		t.Error("environment name not populated; the marker would have no env tag")
	}
}

func TestListAnnotationsFiltersByType(t *testing.T) {
	q, pool := testStore(t)
	org := newTestOrg(t, pool)
	env := newTestEnv(t, pool, org)
	ctx := context.Background()
	now := time.Now()

	insertEventAt(t, pool, env, "deploy", "api", now.Add(-10*time.Minute))
	insertEventAt(t, pool, env, "alert", "api", now.Add(-11*time.Minute))
	insertEventAt(t, pool, env, "rollback", "api", now.Add(-12*time.Minute))

	got, err := q.ListAnnotations(ctx, ListAnnotationsParams{
		OrgID: org,
		Types: []string{"deploy"},
		From:  now.Add(-time.Hour),
		To:    now,
		Limit: 100,
	})
	if err != nil {
		t.Fatalf("ListAnnotations: %v", err)
	}
	if len(got) != 1 || got[0].Type != "deploy" {
		t.Fatalf("want only the deploy back, got %d rows", len(got))
	}
}

func TestListAnnotationsFiltersByService(t *testing.T) {
	q, pool := testStore(t)
	org := newTestOrg(t, pool)
	env := newTestEnv(t, pool, org)
	ctx := context.Background()
	now := time.Now()

	insertEventAt(t, pool, env, "deploy", "api", now.Add(-10*time.Minute))
	insertEventAt(t, pool, env, "deploy", "worker", now.Add(-11*time.Minute))

	got, err := q.ListAnnotations(ctx, ListAnnotationsParams{
		OrgID:   org,
		Types:   changeTypes,
		Service: "worker",
		From:    now.Add(-time.Hour),
		To:      now,
		Limit:   100,
	})
	if err != nil {
		t.Fatalf("ListAnnotations: %v", err)
	}
	if len(got) != 1 || got[0].Service != "worker" {
		t.Fatalf("want only the worker deploy, got %d rows", len(got))
	}
}

// The cap has to drop the oldest, not the newest: a dashboard is read at its
// recent end, and truncating the other way would leave the right-hand side of
// the panel bare while the far left stayed dense.
func TestListAnnotationsCapKeepsTheNewest(t *testing.T) {
	q, pool := testStore(t)
	org := newTestOrg(t, pool)
	env := newTestEnv(t, pool, org)
	ctx := context.Background()
	now := time.Now()

	insertEventAt(t, pool, env, "deploy", "oldest", now.Add(-40*time.Minute))
	insertEventAt(t, pool, env, "deploy", "middle", now.Add(-30*time.Minute))
	insertEventAt(t, pool, env, "deploy", "newer", now.Add(-20*time.Minute))
	insertEventAt(t, pool, env, "deploy", "newest", now.Add(-10*time.Minute))

	got, err := q.ListAnnotations(ctx, ListAnnotationsParams{
		OrgID: org,
		Types: changeTypes,
		From:  now.Add(-time.Hour),
		To:    now,
		Limit: 2,
	})
	if err != nil {
		t.Fatalf("ListAnnotations: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 annotations, got %d", len(got))
	}
	// Still ascending after the reverse, and holding the recent end.
	if got[0].Service != "newer" || got[1].Service != "newest" {
		t.Errorf("cap kept the wrong end: %s then %s", got[0].Service, got[1].Service)
	}
}

func TestListAnnotationsScopesToOrg(t *testing.T) {
	q, pool := testStore(t)
	ctx := context.Background()
	now := time.Now()

	orgA := newTestOrg(t, pool)
	envA := newTestEnv(t, pool, orgA)
	orgB := newTestOrg(t, pool)
	envB := newTestEnv(t, pool, orgB)

	insertEventAt(t, pool, envA, "deploy", "org-a-api", now.Add(-10*time.Minute))
	insertEventAt(t, pool, envB, "deploy", "org-b-api", now.Add(-10*time.Minute))

	// Org A naming org B's environment must come back empty, not with org B's
	// deploys — the environment id is the only thing tying an event to an org.
	envBRef := envB
	got, err := q.ListAnnotations(ctx, ListAnnotationsParams{
		OrgID:         orgA,
		EnvironmentID: &envBRef,
		Types:         changeTypes,
		From:          now.Add(-time.Hour),
		To:            now,
		Limit:         100,
	})
	if err != nil {
		t.Fatalf("ListAnnotations: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("cross-tenant leak: org A read %d of org B's events", len(got))
	}

	// The same call against org A's own environment must still return rows, so
	// the assertion above is proving scoping rather than a query that is broken
	// and returns nothing either way.
	envARef := envA
	got, err = q.ListAnnotations(ctx, ListAnnotationsParams{
		OrgID:         orgA,
		EnvironmentID: &envARef,
		Types:         changeTypes,
		From:          now.Add(-time.Hour),
		To:            now,
		Limit:         100,
	})
	if err != nil {
		t.Fatalf("ListAnnotations own org: %v", err)
	}
	if len(got) != 1 || got[0].Service != "org-a-api" {
		t.Errorf("want org A's own deploy back, got %d rows", len(got))
	}
}
