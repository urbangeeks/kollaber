package store

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func newAlertEvent(t *testing.T, q *Queries, envID pgtype.UUID, fingerprint, status string, ts time.Time) Event {
	t.Helper()
	e, err := q.CreateEventAt(context.Background(), CreateEventAtParams{
		Type:          "alert",
		Service:       "api",
		EnvironmentID: envID,
		Metadata:      []byte(`{"fingerprint":"` + fingerprint + `"}`),
		Status:        status,
		Timestamp:     pgtype.Timestamptz{Time: ts, Valid: true},
	})
	if err != nil {
		t.Fatalf("create alert event: %v", err)
	}
	return e
}

func TestCreateEventAtUsesSuppliedTimestamp(t *testing.T) {
	q, pool := testStore(t)
	org := newTestOrg(t, pool)
	env := newTestEnv(t, pool, org)

	want := time.Now().Add(-90 * time.Minute).UTC().Truncate(time.Second)
	got := newAlertEvent(t, q, env, "fp-ts", "failure", want)

	if !got.Timestamp.Time.UTC().Truncate(time.Second).Equal(want) {
		t.Errorf("timestamp = %v, want %v", got.Timestamp.Time.UTC(), want)
	}
	// created_at should still track insertion, not the event time.
	if got.CreatedAt.Time.Before(want.Add(time.Hour)) {
		t.Errorf("created_at = %v, expected it to track insertion time, not the event time", got.CreatedAt.Time)
	}
}

func TestLatestAlertStatusByFingerprint(t *testing.T) {
	q, pool := testStore(t)
	org := newTestOrg(t, pool)
	env := newTestEnv(t, pool, org)
	ctx := context.Background()

	// Unknown fingerprint reports not-found rather than an error.
	if _, found, err := q.LatestAlertStatusByFingerprint(ctx, env, "nope"); err != nil {
		t.Fatalf("lookup unknown: %v", err)
	} else if found {
		t.Error("found = true for a fingerprint that was never ingested")
	}

	base := time.Now().Add(-time.Hour)
	newAlertEvent(t, q, env, "fp-1", "failure", base)

	status, found, err := q.LatestAlertStatusByFingerprint(ctx, env, "fp-1")
	if err != nil {
		t.Fatalf("lookup firing: %v", err)
	}
	if !found || status != "failure" {
		t.Errorf("got (%q, %v), want (failure, true)", status, found)
	}

	// A later resolution must win over the earlier firing row.
	newAlertEvent(t, q, env, "fp-1", "success", base.Add(10*time.Minute))

	status, found, err = q.LatestAlertStatusByFingerprint(ctx, env, "fp-1")
	if err != nil {
		t.Fatalf("lookup resolved: %v", err)
	}
	if !found || status != "success" {
		t.Errorf("got (%q, %v), want (success, true)", status, found)
	}
}

func TestLatestAlertStatusByFingerprintIsScopedToEnvironment(t *testing.T) {
	q, pool := testStore(t)
	org := newTestOrg(t, pool)
	envA := newTestEnv(t, pool, org)
	envB := newTestEnv(t, pool, org)

	newAlertEvent(t, q, envA, "shared-fp", "failure", time.Now())

	_, found, err := q.LatestAlertStatusByFingerprint(context.Background(), envB, "shared-fp")
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if found {
		t.Error("fingerprint from another environment leaked into this one")
	}
}
