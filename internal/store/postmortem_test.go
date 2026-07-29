package store

import (
	"context"
	"testing"
	"time"
)

func TestListCommentsInRange(t *testing.T) {
	q, pool := testStore(t)
	org := newTestOrg(t, pool)
	env := newTestEnv(t, pool, org)
	user := newTestUser(t, pool)
	ctx := context.Background()
	now := time.Now()

	inWindow := insertEventAt(t, pool, env, "alert", "api", now.Add(-30*time.Minute))
	alsoInWindow := insertEventAt(t, pool, env, "deploy", "api", now.Add(-45*time.Minute))
	outOfWindow := insertEventAt(t, pool, env, "deploy", "api", now.Add(-10*time.Hour))

	insertComment(t, pool, inWindow, user, "second in the thread")
	insertComment(t, pool, alsoInWindow, user, "first by event time")
	insertComment(t, pool, outOfWindow, user, "should not appear")

	got, err := q.ListCommentsInRange(ctx, ListCommentsInRangeParams{
		OrgID:         org,
		EnvironmentID: env,
		From:          now.Add(-2 * time.Hour),
		To:            now,
		Limit:         100,
	})
	if err != nil {
		t.Fatalf("ListCommentsInRange: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("want 2 comments, got %d", len(got))
	}
	// Ordered by the event's time, so the document reads forwards.
	if got[0].Body != "first by event time" {
		t.Errorf("comments not ordered by event time: first is %q", got[0].Body)
	}
	if got[0].AuthorEmail == "" {
		t.Error("author email not populated")
	}
}

func TestListCommentsInRangeScopesToOrg(t *testing.T) {
	q, pool := testStore(t)
	ctx := context.Background()
	now := time.Now()

	orgA := newTestOrg(t, pool)
	envA := newTestEnv(t, pool, orgA)
	orgB := newTestOrg(t, pool)
	envB := newTestEnv(t, pool, orgB)
	userA := newTestUser(t, pool)
	userB := newTestUser(t, pool)

	evA := insertEventAt(t, pool, envA, "alert", "api", now.Add(-10*time.Minute))
	insertComment(t, pool, evA, userA, "org A discussion")
	evB := insertEventAt(t, pool, envB, "alert", "api", now.Add(-10*time.Minute))
	insertComment(t, pool, evB, userB, "org B private discussion")

	got, err := q.ListCommentsInRange(ctx, ListCommentsInRangeParams{
		OrgID:         orgA,
		EnvironmentID: envB,
		From:          now.Add(-time.Hour),
		To:            now,
		Limit:         100,
	})
	if err != nil {
		t.Fatalf("ListCommentsInRange: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("cross-tenant leak: org A read %d of org B's comments", len(got))
	}

	// The same call against org A's own environment must still work, so the
	// test above is proving scoping rather than a query that returns nothing.
	got, err = q.ListCommentsInRange(ctx, ListCommentsInRangeParams{
		OrgID:         orgA,
		EnvironmentID: envA,
		From:          now.Add(-time.Hour),
		To:            now,
		Limit:         100,
	})
	if err != nil {
		t.Fatalf("ListCommentsInRange own org: %v", err)
	}
	if len(got) != 1 || got[0].Body != "org A discussion" {
		t.Errorf("want org A's own comment back, got %d rows", len(got))
	}
}
