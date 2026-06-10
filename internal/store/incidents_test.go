package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// newTestEnv inserts a throwaway environment in the given org and returns its id.
// It cascades away when the org is cleaned up.
func newTestEnv(t *testing.T, pool *pgxpool.Pool, orgID pgtype.UUID) pgtype.UUID {
	t.Helper()
	id := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO environments (id, org_id, name) VALUES ($1, $2, 'test-env')`, id, orgID); err != nil {
		t.Fatalf("insert env: %v", err)
	}
	return id
}

func newTestEvent(t *testing.T, q *Queries, envID pgtype.UUID) Event {
	t.Helper()
	e, err := q.CreateEvent(context.Background(), CreateEventParams{
		Type:          "alert",
		Service:       "api",
		EnvironmentID: envID,
		Metadata:      []byte(`{}`),
		Status:        "failure",
	})
	if err != nil {
		t.Fatalf("create event: %v", err)
	}
	return e
}

func TestCreateAndGetIncident(t *testing.T) {
	q, pool := testStore(t)
	org := newTestOrg(t, pool)
	ctx := context.Background()

	created, err := q.CreateIncident(ctx, CreateIncidentParams{
		OrgID:    org,
		Title:    "Elevated 5xx",
		Severity: "sev2",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Status != "open" {
		t.Errorf("default status = %q, want open", created.Status)
	}
	if created.ResolvedAt.Valid {
		t.Errorf("resolved_at should be null on a new incident")
	}

	got, err := q.GetIncidentByID(ctx, created.ID, org)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Title != "Elevated 5xx" || got.Severity != "sev2" {
		t.Errorf("got %+v, want title/severity to round-trip", got)
	}

	// An incident in another org must not be visible.
	other := newTestOrg(t, pool)
	if _, err := q.GetIncidentByID(ctx, created.ID, other); err == nil {
		t.Error("expected error fetching incident across orgs, got nil")
	}
}

func TestUpdateIncident_ResolvedAtLifecycle(t *testing.T) {
	q, pool := testStore(t)
	org := newTestOrg(t, pool)
	ctx := context.Background()

	inc, err := q.CreateIncident(ctx, CreateIncidentParams{OrgID: org, Title: "DB latency", Severity: "sev3"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	base := UpdateIncidentParams{ID: inc.ID, OrgID: org, Title: inc.Title, Severity: inc.Severity, OwnerID: inc.OwnerID}

	// open -> resolved stamps resolved_at.
	base.Status = "resolved"
	resolved, err := q.UpdateIncident(ctx, base)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !resolved.ResolvedAt.Valid {
		t.Fatal("resolved_at should be set after resolving")
	}
	stamp := resolved.ResolvedAt.Time

	// resolved -> open clears resolved_at.
	base.Status = "open"
	reopened, err := q.UpdateIncident(ctx, base)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if reopened.ResolvedAt.Valid {
		t.Error("resolved_at should be cleared after reopening")
	}

	// open -> resolved again stamps a fresh time (not the stale one).
	base.Status = "resolved"
	again, err := q.UpdateIncident(ctx, base)
	if err != nil {
		t.Fatalf("re-resolve: %v", err)
	}
	if !again.ResolvedAt.Valid || again.ResolvedAt.Time.Before(stamp) {
		t.Error("re-resolving should stamp resolved_at again")
	}
}

func TestListIncidents_StatusFilter(t *testing.T) {
	q, pool := testStore(t)
	org := newTestOrg(t, pool)
	ctx := context.Background()

	mk := func(status string) {
		inc, err := q.CreateIncident(ctx, CreateIncidentParams{OrgID: org, Title: "x", Severity: "sev3"})
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if status != "open" {
			_, err = q.UpdateIncident(ctx, UpdateIncidentParams{ID: inc.ID, OrgID: org, Title: inc.Title, Severity: inc.Severity, Status: status, OwnerID: inc.OwnerID})
			if err != nil {
				t.Fatalf("set status: %v", err)
			}
		}
	}
	mk("open")
	mk("open")
	mk("resolved")

	all, err := q.ListIncidents(ctx, org, "")
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("list all = %d, want 3", len(all))
	}

	open, err := q.ListIncidents(ctx, org, "open")
	if err != nil {
		t.Fatalf("list open: %v", err)
	}
	if len(open) != 2 {
		t.Errorf("list open = %d, want 2", len(open))
	}
}

func TestAttachEventsToIncident_OrgScoped(t *testing.T) {
	q, pool := testStore(t)
	org := newTestOrg(t, pool)
	ctx := context.Background()

	env := newTestEnv(t, pool, org)
	ev := newTestEvent(t, q, env)

	inc, err := q.CreateIncident(ctx, CreateIncidentParams{OrgID: org, Title: "outage", Severity: "sev1"})
	if err != nil {
		t.Fatalf("create incident: %v", err)
	}

	// An event from a different org must not attach.
	otherOrg := newTestOrg(t, pool)
	otherEnv := newTestEnv(t, pool, otherOrg)
	otherEv := newTestEvent(t, q, otherEnv)

	attached, err := q.AttachEventsToIncident(ctx, inc.ID, org, []uuid.UUID{ev.ID.Bytes, otherEv.ID.Bytes})
	if err != nil {
		t.Fatalf("attach: %v", err)
	}
	if attached != 1 {
		t.Errorf("attached = %d, want 1 (cross-org event must be rejected)", attached)
	}

	events, err := q.ListIncidentEvents(ctx, inc.ID, org)
	if err != nil {
		t.Fatalf("list incident events: %v", err)
	}
	if len(events) != 1 || events[0].ID != ev.ID {
		t.Fatalf("ListIncidentEvents = %+v, want only the in-org event", events)
	}

	counts, err := q.CountEventsByIncidentForOrg(ctx, org)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if counts[inc.ID.Bytes] != 1 {
		t.Errorf("event count = %d, want 1", counts[inc.ID.Bytes])
	}
}
