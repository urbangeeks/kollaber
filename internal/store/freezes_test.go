package store

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func newFreeze(t *testing.T, q *Queries, org pgtype.UUID, env *pgtype.UUID, reason string, from, to time.Time) FreezeWindow {
	t.Helper()
	f, err := q.CreateFreezeWindow(context.Background(), CreateFreezeWindowParams{
		OrgID:         org,
		EnvironmentID: env,
		Reason:        reason,
		StartsAt:      from,
		EndsAt:        to,
	})
	if err != nil {
		t.Fatalf("CreateFreezeWindow(%s): %v", reason, err)
	}
	return f
}

func TestActiveFreezeWindowCoversOnlyItsPeriod(t *testing.T) {
	q, pool := testStore(t)
	org := newTestOrg(t, pool)
	env := newTestEnv(t, pool, org)
	ctx := context.Background()
	now := time.Now()

	newFreeze(t, q, org, &env, "Black Friday", now.Add(-time.Hour), now.Add(time.Hour))

	tests := []struct {
		name string
		at   time.Time
		want bool
	}{
		{"inside", now, true},
		{"at the start is inside", now.Add(-time.Hour), true},
		{"just before", now.Add(-2 * time.Hour), false},
		{"just after", now.Add(2 * time.Hour), false},
		// Half-open: a freeze ending at 09:00 does not cover 09:00, so a window
		// that ends exactly when the next begins cannot report both.
		{"at the end is outside", now.Add(time.Hour), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, frozen, err := q.ActiveFreezeWindow(ctx, org, env, tt.at)
			if err != nil {
				t.Fatalf("ActiveFreezeWindow: %v", err)
			}
			if frozen != tt.want {
				t.Errorf("frozen = %v, want %v", frozen, tt.want)
			}
		})
	}
}

// A window naming no environment is what a company-wide freeze is.
func TestActiveFreezeWindowOrgWideCoversEveryEnvironment(t *testing.T) {
	q, pool := testStore(t)
	org := newTestOrg(t, pool)
	prod := newTestEnv(t, pool, org)
	staging := newTestEnv(t, pool, org)
	ctx := context.Background()
	now := time.Now()

	newFreeze(t, q, org, nil, "quarter end", now.Add(-time.Hour), now.Add(time.Hour))

	for _, env := range []pgtype.UUID{prod, staging} {
		got, frozen, err := q.ActiveFreezeWindow(ctx, org, env, now)
		if err != nil {
			t.Fatalf("ActiveFreezeWindow: %v", err)
		}
		if !frozen {
			t.Fatal("an org-wide freeze did not cover an environment")
		}
		if got.EnvironmentID != nil && got.EnvironmentID.Valid {
			t.Error("an org-wide window came back scoped to an environment")
		}
	}
}

// A freeze on prod must not stop staging from shipping.
func TestActiveFreezeWindowScopedToOneEnvironment(t *testing.T) {
	q, pool := testStore(t)
	org := newTestOrg(t, pool)
	prod := newTestEnv(t, pool, org)
	staging := newTestEnv(t, pool, org)
	ctx := context.Background()
	now := time.Now()

	newFreeze(t, q, org, &prod, "prod only", now.Add(-time.Hour), now.Add(time.Hour))

	if _, frozen, _ := q.ActiveFreezeWindow(ctx, org, prod, now); !frozen {
		t.Error("prod should be frozen")
	}
	if _, frozen, _ := q.ActiveFreezeWindow(ctx, org, staging, now); frozen {
		t.Error("a prod freeze leaked into staging")
	}
}

// When windows overlap, the one still in force after the others lapse is the
// honest one to name.
func TestActiveFreezeWindowPrefersTheLatestEnding(t *testing.T) {
	q, pool := testStore(t)
	org := newTestOrg(t, pool)
	env := newTestEnv(t, pool, org)
	ctx := context.Background()
	now := time.Now()

	newFreeze(t, q, org, &env, "short", now.Add(-time.Hour), now.Add(time.Hour))
	newFreeze(t, q, org, nil, "long org-wide", now.Add(-2*time.Hour), now.Add(48*time.Hour))

	got, frozen, err := q.ActiveFreezeWindow(ctx, org, env, now)
	if err != nil {
		t.Fatalf("ActiveFreezeWindow: %v", err)
	}
	if !frozen {
		t.Fatal("expected a freeze")
	}
	if got.Reason != "long org-wide" {
		t.Errorf("reason = %q, want the window that outlasts the others", got.Reason)
	}
}

func TestActiveFreezeWindowScopesToOrg(t *testing.T) {
	q, pool := testStore(t)
	ctx := context.Background()
	now := time.Now()

	orgA := newTestOrg(t, pool)
	orgB := newTestOrg(t, pool)
	envB := newTestEnv(t, pool, orgB)

	newFreeze(t, q, orgB, &envB, "org B freeze", now.Add(-time.Hour), now.Add(time.Hour))

	if _, frozen, _ := q.ActiveFreezeWindow(ctx, orgA, envB, now); frozen {
		t.Error("org A saw org B's freeze")
	}
	if _, frozen, _ := q.ActiveFreezeWindow(ctx, orgB, envB, now); !frozen {
		t.Error("org B could not see its own freeze")
	}
}

func TestCreateFreezeWindowRejectsAnInvertedPeriod(t *testing.T) {
	q, pool := testStore(t)
	org := newTestOrg(t, pool)
	env := newTestEnv(t, pool, org)
	now := time.Now()

	_, err := q.CreateFreezeWindow(context.Background(), CreateFreezeWindowParams{
		OrgID:         org,
		EnvironmentID: &env,
		Reason:        "backwards",
		StartsAt:      now.Add(time.Hour),
		EndsAt:        now,
	})
	if err == nil {
		t.Error("a window ending before it starts was accepted")
	}
}

func TestDeleteFreezeWindowScopesToOrg(t *testing.T) {
	q, pool := testStore(t)
	ctx := context.Background()
	now := time.Now()

	orgA := newTestOrg(t, pool)
	orgB := newTestOrg(t, pool)
	envB := newTestEnv(t, pool, orgB)
	window := newFreeze(t, q, orgB, &envB, "org B freeze", now.Add(-time.Hour), now.Add(time.Hour))

	found, err := q.DeleteFreezeWindow(ctx, window.ID, orgA)
	if err != nil {
		t.Fatalf("DeleteFreezeWindow: %v", err)
	}
	if found {
		t.Error("org A deleted org B's freeze window")
	}

	// Still there, and still in force.
	if _, frozen, _ := q.ActiveFreezeWindow(ctx, orgB, envB, now); !frozen {
		t.Error("org B's freeze was removed by another tenant")
	}

	found, err = q.DeleteFreezeWindow(ctx, window.ID, orgB)
	if err != nil {
		t.Fatalf("DeleteFreezeWindow own org: %v", err)
	}
	if !found {
		t.Error("the owning org could not delete its own window")
	}
}

func TestListFreezeWindowsScopesToOrg(t *testing.T) {
	q, pool := testStore(t)
	ctx := context.Background()
	now := time.Now()

	orgA := newTestOrg(t, pool)
	orgB := newTestOrg(t, pool)
	envA := newTestEnv(t, pool, orgA)
	newFreeze(t, q, orgA, &envA, "org A freeze", now, now.Add(time.Hour))
	newFreeze(t, q, orgB, nil, "org B freeze", now, now.Add(time.Hour))

	got, err := q.ListFreezeWindows(ctx, orgA)
	if err != nil {
		t.Fatalf("ListFreezeWindows: %v", err)
	}
	if len(got) != 1 || got[0].Reason != "org A freeze" {
		t.Errorf("want only org A's window, got %d rows", len(got))
	}
}

// A past window is still worth listing: it explains why a deploy from last
// month carries a freeze mark.
func TestListFreezeWindowsIncludesPastOnes(t *testing.T) {
	q, pool := testStore(t)
	org := newTestOrg(t, pool)
	env := newTestEnv(t, pool, org)
	ctx := context.Background()
	now := time.Now()

	newFreeze(t, q, org, &env, "last month", now.Add(-40*24*time.Hour), now.Add(-39*24*time.Hour))
	newFreeze(t, q, org, &env, "right now", now.Add(-time.Hour), now.Add(time.Hour))

	got, err := q.ListFreezeWindows(ctx, org)
	if err != nil {
		t.Fatalf("ListFreezeWindows: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 windows, got %d", len(got))
	}
	// Newest start first, so the current one leads.
	if got[0].Reason != "right now" {
		t.Errorf("first window = %q, want the current one", got[0].Reason)
	}
}
