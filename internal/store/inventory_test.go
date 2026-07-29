package store

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// insertVersionedEvent adds a deploy-family event carrying explicit metadata, so the
// version-extraction chain can be exercised per ingestion source.
func insertVersionedEvent(t *testing.T, pool *pgxpool.Pool, envID pgtype.UUID, eventType, service, status, metadata string, ts time.Time) pgtype.UUID {
	t.Helper()
	var id pgtype.UUID
	err := pool.QueryRow(context.Background(),
		`INSERT INTO events (type, service, environment_id, timestamp, status, metadata)
		 VALUES ($1, $2, $3, $4, $5, $6::jsonb)
		 RETURNING id`,
		eventType, service, envID, ts, status, metadata).Scan(&id)
	if err != nil {
		t.Fatalf("insert %s event: %v", eventType, err)
	}
	return id
}

func versionOf(t *testing.T, versions []ServiceVersion, service string) string {
	t.Helper()
	for _, v := range versions {
		if v.Service == service {
			if v.Version == nil {
				return "<nil>"
			}
			return *v.Version
		}
	}
	t.Fatalf("service %q missing from the inventory", service)
	return ""
}

func TestServiceVersionsAtReturnsTheLatestPerService(t *testing.T) {
	q, pool := testStore(t)
	org := newTestOrg(t, pool)
	env := newTestEnv(t, pool, org)
	ctx := context.Background()
	now := time.Now()

	insertVersionedEvent(t, pool, env, "deploy", "api", "success", `{"version":"v1.0.0"}`, now.Add(-3*time.Hour))
	insertVersionedEvent(t, pool, env, "deploy", "api", "success", `{"version":"v1.1.0"}`, now.Add(-1*time.Hour))
	insertVersionedEvent(t, pool, env, "deploy", "worker", "success", `{"version":"w-2.0"}`, now.Add(-2*time.Hour))

	got, err := q.ServiceVersionsAt(ctx, org, env, now)
	if err != nil {
		t.Fatalf("ServiceVersionsAt: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 services, got %d", len(got))
	}
	if v := versionOf(t, got, "api"); v != "v1.1.0" {
		t.Errorf("api = %q, want the newest deploy v1.1.0", v)
	}
	if v := versionOf(t, got, "worker"); v != "w-2.0" {
		t.Errorf("worker = %q", v)
	}
}

// The point of the feature: asking what was running at the moment something
// broke, not what is running now.
func TestServiceVersionsAtIsPointInTime(t *testing.T) {
	q, pool := testStore(t)
	org := newTestOrg(t, pool)
	env := newTestEnv(t, pool, org)
	ctx := context.Background()
	now := time.Now()

	insertVersionedEvent(t, pool, env, "deploy", "api", "success", `{"version":"v1.0.0"}`, now.Add(-3*time.Hour))
	insertVersionedEvent(t, pool, env, "deploy", "api", "success", `{"version":"v2.0.0"}`, now.Add(-1*time.Hour))

	got, err := q.ServiceVersionsAt(ctx, org, env, now.Add(-2*time.Hour))
	if err != nil {
		t.Fatalf("ServiceVersionsAt: %v", err)
	}
	if v := versionOf(t, got, "api"); v != "v1.0.0" {
		t.Errorf("api = %q, want v1.0.0 — the later deploy had not happened yet", v)
	}
}

// A failed deploy did not change what is running. Counting it is how an
// inventory reports a build in production that never got there.
func TestServiceVersionsAtIgnoresUnsuccessfulDeploys(t *testing.T) {
	q, pool := testStore(t)
	org := newTestOrg(t, pool)
	env := newTestEnv(t, pool, org)
	ctx := context.Background()
	now := time.Now()

	insertVersionedEvent(t, pool, env, "deploy", "api", "success", `{"version":"v1.0.0"}`, now.Add(-3*time.Hour))
	insertVersionedEvent(t, pool, env, "deploy", "api", "failure", `{"version":"v1.1.0-broken"}`, now.Add(-2*time.Hour))
	insertVersionedEvent(t, pool, env, "deploy", "api", "in_progress", `{"version":"v1.2.0-landing"}`, now.Add(-time.Minute))

	got, err := q.ServiceVersionsAt(ctx, org, env, now)
	if err != nil {
		t.Fatalf("ServiceVersionsAt: %v", err)
	}
	if v := versionOf(t, got, "api"); v != "v1.0.0" {
		t.Errorf("api = %q, want the last successful deploy v1.0.0", v)
	}
}

// A rollback changes what is running just as much as a deploy does.
func TestServiceVersionsAtCountsRollbacks(t *testing.T) {
	q, pool := testStore(t)
	org := newTestOrg(t, pool)
	env := newTestEnv(t, pool, org)
	ctx := context.Background()
	now := time.Now()

	insertVersionedEvent(t, pool, env, "deploy", "api", "success", `{"version":"v2.0.0"}`, now.Add(-2*time.Hour))
	insertVersionedEvent(t, pool, env, "rollback", "api", "success", `{"to":"v1.9.0"}`, now.Add(-time.Hour))

	got, err := q.ServiceVersionsAt(ctx, org, env, now)
	if err != nil {
		t.Fatalf("ServiceVersionsAt: %v", err)
	}
	if v := versionOf(t, got, "api"); v != "v1.9.0" {
		t.Errorf("api = %q, want the rolled-back version v1.9.0", v)
	}
	if got[0].EventType != "rollback" {
		t.Errorf("event type = %q, want rollback so the UI can flag it", got[0].EventType)
	}
}

// Every ingestion path names the version differently.
func TestServiceVersionsAtReadsEveryVersionKey(t *testing.T) {
	q, pool := testStore(t)
	org := newTestOrg(t, pool)
	env := newTestEnv(t, pool, org)
	ctx := context.Background()
	now := time.Now()

	tests := []struct {
		service  string
		metadata string
		want     string
	}{
		{"cli", `{"version":"v1.2.3"}`, "v1.2.3"},
		{"kube", `{"image_tag":"sha-abc123"}`, "sha-abc123"},
		{"argocd", `{"revision":"7fd1a60"}`, "7fd1a60"},
		{"atlantis", `{"head_commit":"deadbeef"}`, "deadbeef"},
		// version wins when a source sends more than one.
		{"both", `{"version":"v1","revision":"r1"}`, "v1"},
	}
	for _, tt := range tests {
		insertVersionedEvent(t, pool, env, "deploy", tt.service, "success", tt.metadata, now.Add(-time.Hour))
	}

	got, err := q.ServiceVersionsAt(ctx, org, env, now)
	if err != nil {
		t.Fatalf("ServiceVersionsAt: %v", err)
	}
	for _, tt := range tests {
		if v := versionOf(t, got, tt.service); v != tt.want {
			t.Errorf("%s = %q, want %q", tt.service, v, tt.want)
		}
	}
}

// Reporting the previous version would claim a build is running that is not.
// Unknown is the honest answer.
func TestServiceVersionsAtReportsUnknownRatherThanStale(t *testing.T) {
	q, pool := testStore(t)
	org := newTestOrg(t, pool)
	env := newTestEnv(t, pool, org)
	ctx := context.Background()
	now := time.Now()

	insertVersionedEvent(t, pool, env, "deploy", "api", "success", `{"version":"v1.0.0"}`, now.Add(-2*time.Hour))
	insertVersionedEvent(t, pool, env, "deploy", "api", "success", `{}`, now.Add(-time.Hour))

	got, err := q.ServiceVersionsAt(ctx, org, env, now)
	if err != nil {
		t.Fatalf("ServiceVersionsAt: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 service, got %d", len(got))
	}
	if got[0].Version != nil {
		t.Errorf("version = %q, want unknown", *got[0].Version)
	}
}

// A service that has not shipped in months is still running something.
func TestServiceVersionsAtKeepsLongIdleServices(t *testing.T) {
	q, pool := testStore(t)
	org := newTestOrg(t, pool)
	env := newTestEnv(t, pool, org)
	ctx := context.Background()
	now := time.Now()

	insertVersionedEvent(t, pool, env, "deploy", "legacy", "success", `{"version":"v0.9"}`, now.Add(-200*24*time.Hour))

	got, err := q.ServiceVersionsAt(ctx, org, env, now)
	if err != nil {
		t.Fatalf("ServiceVersionsAt: %v", err)
	}
	if v := versionOf(t, got, "legacy"); v != "v0.9" {
		t.Errorf("legacy = %q, want v0.9", v)
	}
}

// Alerts and notes say nothing about what is deployed.
func TestServiceVersionsAtIgnoresNonDeployTypes(t *testing.T) {
	q, pool := testStore(t)
	org := newTestOrg(t, pool)
	env := newTestEnv(t, pool, org)
	ctx := context.Background()
	now := time.Now()

	insertVersionedEvent(t, pool, env, "deploy", "api", "success", `{"version":"v1.0.0"}`, now.Add(-2*time.Hour))
	insertVersionedEvent(t, pool, env, "alert", "api", "success", `{"version":"not-a-deploy"}`, now.Add(-time.Hour))
	insertVersionedEvent(t, pool, env, "note", "api", "success", `{"version":"also-not"}`, now.Add(-30*time.Minute))

	got, err := q.ServiceVersionsAt(ctx, org, env, now)
	if err != nil {
		t.Fatalf("ServiceVersionsAt: %v", err)
	}
	if v := versionOf(t, got, "api"); v != "v1.0.0" {
		t.Errorf("api = %q, want v1.0.0", v)
	}
}

func TestServiceVersionsAtScopesToOrg(t *testing.T) {
	q, pool := testStore(t)
	ctx := context.Background()
	now := time.Now()

	orgA := newTestOrg(t, pool)
	orgB := newTestOrg(t, pool)
	envB := newTestEnv(t, pool, orgB)
	insertVersionedEvent(t, pool, envB, "deploy", "api", "success", `{"version":"v1.0.0"}`, now.Add(-time.Hour))

	got, err := q.ServiceVersionsAt(ctx, orgA, envB, now)
	if err != nil {
		t.Fatalf("ServiceVersionsAt: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("cross-tenant leak: org A saw %d of org B's services", len(got))
	}

	own, err := q.ServiceVersionsAt(ctx, orgB, envB, now)
	if err != nil {
		t.Fatalf("ServiceVersionsAt own org: %v", err)
	}
	if len(own) != 1 {
		t.Errorf("want org B's own service back, got %d", len(own))
	}
}

// Environments are separate inventories: staging running v2 says nothing about
// what prod is running.
func TestServiceVersionsAtIsPerEnvironment(t *testing.T) {
	q, pool := testStore(t)
	org := newTestOrg(t, pool)
	prod := newTestEnv(t, pool, org)
	staging := newTestEnv(t, pool, org)
	ctx := context.Background()
	now := time.Now()

	insertVersionedEvent(t, pool, prod, "deploy", "api", "success", `{"version":"v1.0.0"}`, now.Add(-time.Hour))
	insertVersionedEvent(t, pool, staging, "deploy", "api", "success", `{"version":"v2.0.0-rc1"}`, now.Add(-time.Hour))

	got, err := q.ServiceVersionsAt(ctx, org, prod, now)
	if err != nil {
		t.Fatalf("ServiceVersionsAt: %v", err)
	}
	if v := versionOf(t, got, "api"); v != "v1.0.0" {
		t.Errorf("prod api = %q, want v1.0.0 — staging's deploy leaked in", v)
	}
}
