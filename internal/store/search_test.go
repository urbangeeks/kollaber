package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

func newTestUser(t *testing.T, pool *pgxpool.Pool) pgtype.UUID {
	t.Helper()
	id := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO users (id, email, password_hash) VALUES ($1, $2, 'x')`,
		id, "search-"+uuid.NewString()+"@example.com"); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	return id
}

// insertEventWithMetadata adds an event carrying explicit metadata and returns
// its id. Timestamps are left at NOW(); search ordering by recency is not what
// these tests exercise.
func insertEventWithMetadata(t *testing.T, pool *pgxpool.Pool, envID pgtype.UUID, eventType, service, metadata string) pgtype.UUID {
	t.Helper()
	var id pgtype.UUID
	err := pool.QueryRow(context.Background(),
		`INSERT INTO events (type, service, environment_id, status, metadata)
		 VALUES ($1, $2, $3, 'success', $4::jsonb)
		 RETURNING id`,
		eventType, service, envID, metadata).Scan(&id)
	if err != nil {
		t.Fatalf("insert %s event: %v", eventType, err)
	}
	return id
}

func insertComment(t *testing.T, pool *pgxpool.Pool, eventID, userID pgtype.UUID, body string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO comments (event_id, user_id, body) VALUES ($1, $2, $3)`,
		eventID, userID, body); err != nil {
		t.Fatalf("insert comment: %v", err)
	}
}

func kinds(hits []SearchHit) []string {
	out := make([]string, len(hits))
	for i, h := range hits {
		out[i] = h.Kind
	}
	return out
}

func TestSearchMatchesEventsAndComments(t *testing.T) {
	q, pool := testStore(t)
	org := newTestOrg(t, pool)
	env := newTestEnv(t, pool, org)
	user := newTestUser(t, pool)
	ctx := context.Background()

	deploy := insertEventWithMetadata(t, pool, env, "deploy", "checkout-api",
		`{"version":"v4.1.0","author":"priya"}`)
	alert := insertEventWithMetadata(t, pool, env, "alert", "checkout-api",
		`{"summary":"latency spike on payments"}`)
	insertEventWithMetadata(t, pool, env, "deploy", "billing", `{"version":"v2.0.0"}`)

	insertComment(t, pool, alert, user, "Rolled back the token refresh change")
	insertComment(t, pool, deploy, user, "Unrelated chatter about lunch")

	tests := []struct {
		name      string
		query     string
		wantCount int
		wantKind  string
	}{
		{"metadata value", "priya", 1, "event"},
		{"multi word with stemming", "latency spike", 1, "event"},
		{"comment body", "token refresh", 1, "comment"},
		{"service name", "billing", 1, "event"},
		{"no matches", "kubernetes", 0, ""},
		// "version" and "summary" are metadata *keys*. Indexing keys would make
		// this match every deploy, which is the noise the '["string","numeric"]'
		// filter exists to prevent.
		{"metadata key is not indexed", "version", 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hits, err := q.Search(ctx, SearchParams{OrgID: org, Query: tt.query, Limit: 25})
			if err != nil {
				t.Fatalf("Search: %v", err)
			}
			if len(hits) != tt.wantCount {
				t.Fatalf("want %d hits, got %d (%v)", tt.wantCount, len(hits), kinds(hits))
			}
			if tt.wantCount > 0 && hits[0].Kind != tt.wantKind {
				t.Errorf("kind = %q, want %q", hits[0].Kind, tt.wantKind)
			}
		})
	}
}

// A comment hit must carry the event it was posted on, so the UI can show the
// match in context instead of as a floating quote.
func TestSearchCommentHitCarriesItsEvent(t *testing.T) {
	q, pool := testStore(t)
	org := newTestOrg(t, pool)
	env := newTestEnv(t, pool, org)
	user := newTestUser(t, pool)
	ctx := context.Background()

	alert := insertEventWithMetadata(t, pool, env, "alert", "payments-api", `{}`)
	insertComment(t, pool, alert, user, "Root cause was a stale connection pool")

	hits, err := q.Search(ctx, SearchParams{OrgID: org, Query: "stale connection pool", Limit: 25})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("want 1 hit, got %d", len(hits))
	}

	h := hits[0]
	if h.Kind != "comment" {
		t.Fatalf("kind = %q, want comment", h.Kind)
	}
	if h.CommentBody != "Root cause was a stale connection pool" {
		t.Errorf("body = %q", h.CommentBody)
	}
	if h.Event.ID != alert {
		t.Error("comment hit does not carry its parent event")
	}
	if h.Event.Service != "payments-api" {
		t.Errorf("parent event service = %q, want payments-api", h.Event.Service)
	}
	if !h.CommentUserID.Valid || h.CommentUserID != user {
		t.Error("comment hit does not carry its author")
	}
}

// Neither events nor comments carry org_id, so both halves of the UNION depend
// on the join through environments. A miss on either is a cross-tenant leak.
func TestSearchScopesToOrg(t *testing.T) {
	q, pool := testStore(t)
	ctx := context.Background()

	orgA := newTestOrg(t, pool)
	envA := newTestEnv(t, pool, orgA)
	orgB := newTestOrg(t, pool)
	envB := newTestEnv(t, pool, orgB)
	userB := newTestUser(t, pool)

	insertEventWithMetadata(t, pool, envA, "deploy", "shared-name", `{"author":"alice"}`)
	evB := insertEventWithMetadata(t, pool, envB, "deploy", "shared-name", `{"author":"alice"}`)
	insertComment(t, pool, evB, userB, "alice investigated this")

	hits, err := q.Search(ctx, SearchParams{OrgID: orgA, Query: "alice", Limit: 25})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("want only org A's event, got %d hits (%v)", len(hits), kinds(hits))
	}
	if hits[0].Event.EnvironmentID != envA {
		t.Error("cross-tenant leak: org A received org B's row")
	}
}

func TestSearchFiltersByEnvironment(t *testing.T) {
	q, pool := testStore(t)
	org := newTestOrg(t, pool)
	prod := newTestEnv(t, pool, org)
	staging := newTestEnv(t, pool, org)
	ctx := context.Background()

	insertEventWithMetadata(t, pool, prod, "deploy", "api", `{"author":"dana"}`)
	insertEventWithMetadata(t, pool, staging, "deploy", "api", `{"author":"dana"}`)

	all, err := q.Search(ctx, SearchParams{OrgID: org, Query: "dana", Limit: 25})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("want 2 hits across both environments, got %d", len(all))
	}

	scoped, err := q.Search(ctx, SearchParams{OrgID: org, Query: "dana", EnvironmentID: &prod, Limit: 25})
	if err != nil {
		t.Fatalf("Search scoped: %v", err)
	}
	if len(scoped) != 1 {
		t.Fatalf("want 1 hit in prod, got %d", len(scoped))
	}
	if scoped[0].Event.EnvironmentID != prod {
		t.Error("environment filter returned another environment's event")
	}
}

// websearch_to_tsquery must absorb whatever a user types. to_tsquery would
// raise a syntax error on several of these and surface as a 500.
func TestSearchToleratesJunkInput(t *testing.T) {
	q, pool := testStore(t)
	org := newTestOrg(t, pool)
	env := newTestEnv(t, pool, org)
	ctx := context.Background()

	insertEventWithMetadata(t, pool, env, "deploy", "api", `{"author":"sam"}`)

	for _, query := range []string{"", "   ", "the and of", "&&&", `"unclosed`, "a | b & !c", "<>?!"} {
		t.Run("query="+query, func(t *testing.T) {
			if _, err := q.Search(ctx, SearchParams{OrgID: org, Query: query, Limit: 25}); err != nil {
				t.Errorf("Search(%q) errored: %v", query, err)
			}
		})
	}
}
