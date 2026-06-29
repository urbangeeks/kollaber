package store

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// insertDeploy adds a deploy event at an explicit time with the given status and
// raw metadata, bypassing CreateEvent (which forces timestamp = NOW()).
func insertDeploy(t *testing.T, pool *pgxpool.Pool, envID pgtype.UUID, ts time.Time, status, metadata string) {
	t.Helper()
	if metadata == "" {
		metadata = "{}"
	}
	_, err := pool.Exec(context.Background(),
		`INSERT INTO events (type, service, environment_id, timestamp, status, metadata)
		 VALUES ('deploy', 'api', $1, $2, $3, $4::jsonb)`,
		envID, ts, status, metadata)
	if err != nil {
		t.Fatalf("insert deploy: %v", err)
	}
}

func TestDORA(t *testing.T) {
	q, pool := testStore(t)
	org := newTestOrg(t, pool)
	env := newTestEnv(t, pool, org)
	ctx := context.Background()
	now := time.Now()

	// 4 deploys in-window: 3 succeed, 1 fails. One success carries a commit
	// timestamp 2h before the deploy, so lead time = 2h over 1 sample.
	insertDeploy(t, pool, env, now.Add(-1*time.Hour), "success", "")
	insertDeploy(t, pool, env, now.Add(-2*time.Hour), "success", "")
	insertDeploy(t, pool, env, now.Add(-3*time.Hour), "failure", "")
	committed := now.Add(-26 * time.Hour).UTC().Format(time.RFC3339)
	insertDeploy(t, pool, env, now.Add(-24*time.Hour), "success",
		`{"committed_at":"`+committed+`"}`)
	// Out-of-window deploy must be excluded.
	insertDeploy(t, pool, env, now.Add(-40*24*time.Hour), "success", "")

	// One resolved incident: 30m to restore. Plus an unresolved one (ignored).
	opened := now.Add(-5 * time.Hour)
	resolved := opened.Add(30 * time.Minute)
	if _, err := pool.Exec(ctx,
		`INSERT INTO incidents (org_id, title, severity, status, opened_at, resolved_at)
		 VALUES ($1, 'db down', 'sev1', 'resolved', $2, $3)`,
		org, opened, resolved); err != nil {
		t.Fatalf("insert incident: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO incidents (org_id, title, severity, status, opened_at)
		 VALUES ($1, 'still open', 'sev2', 'open', $2)`,
		org, now.Add(-1*time.Hour)); err != nil {
		t.Fatalf("insert incident: %v", err)
	}

	params := DORAParams{
		OrgID: org,
		Since: pgtype.Timestamptz{Time: now.AddDate(0, 0, -30), Valid: true},
	}

	m, err := q.DORA(ctx, params)
	if err != nil {
		t.Fatalf("DORA: %v", err)
	}
	if m.TotalDeploys != 4 {
		t.Errorf("TotalDeploys = %d, want 4", m.TotalDeploys)
	}
	if m.FailedDeploys != 1 {
		t.Errorf("FailedDeploys = %d, want 1", m.FailedDeploys)
	}
	if m.LeadTimeSamples != 1 {
		t.Errorf("LeadTimeSamples = %d, want 1", m.LeadTimeSamples)
	}
	if got := m.LeadTimeSeconds; got < 7100 || got > 7300 {
		t.Errorf("LeadTimeSeconds = %.0f, want ~7200 (2h)", got)
	}
	if m.ResolvedIncidents != 1 {
		t.Errorf("ResolvedIncidents = %d, want 1", m.ResolvedIncidents)
	}
	if got := m.RestoreSeconds; got < 1790 || got > 1810 {
		t.Errorf("RestoreSeconds = %.0f, want ~1800 (30m)", got)
	}

	trend, err := q.DeployTrend(ctx, params)
	if err != nil {
		t.Fatalf("DeployTrend: %v", err)
	}
	var trendTotal int64
	for _, p := range trend {
		trendTotal += p.Deploys
	}
	if trendTotal != 4 {
		t.Errorf("trend total = %d, want 4 (in-window deploys)", trendTotal)
	}
}

func TestDORA_EnvironmentFilter(t *testing.T) {
	q, pool := testStore(t)
	org := newTestOrg(t, pool)
	envA := newTestEnv(t, pool, org)
	envB := newTestEnv(t, pool, org)
	ctx := context.Background()
	now := time.Now()

	insertDeploy(t, pool, envA, now.Add(-1*time.Hour), "success", "")
	insertDeploy(t, pool, envA, now.Add(-2*time.Hour), "failure", "")
	insertDeploy(t, pool, envB, now.Add(-1*time.Hour), "success", "")

	params := DORAParams{
		OrgID:         org,
		EnvironmentID: &envA,
		Since:         pgtype.Timestamptz{Time: now.AddDate(0, 0, -30), Valid: true},
	}
	m, err := q.DORA(ctx, params)
	if err != nil {
		t.Fatalf("DORA: %v", err)
	}
	if m.TotalDeploys != 2 {
		t.Errorf("TotalDeploys for envA = %d, want 2", m.TotalDeploys)
	}
	if m.FailedDeploys != 1 {
		t.Errorf("FailedDeploys for envA = %d, want 1", m.FailedDeploys)
	}
}
