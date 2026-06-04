package store

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/urbangeeks/kollaber/internal/db"
)

// testStore connects to TEST_DATABASE_URL and applies migrations, skipping the
// test entirely when the env var is unset so `go test ./...` stays runnable
// without a database.
func testStore(t *testing.T) (*Queries, *pgxpool.Pool) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping DB integration test")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := db.Migrate(ctx, pool); err != nil {
		pool.Close()
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(pool.Close)
	return New(pool), pool
}

// newTestOrg inserts a throwaway org (cleaned up after the test; ai_usage rows
// cascade away with it) and returns its id.
func newTestOrg(t *testing.T, pool *pgxpool.Pool) pgtype.UUID {
	t.Helper()
	id := uuid.New()
	pg := pgtype.UUID{Bytes: id, Valid: true}
	slug := "test-" + id.String()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO orgs (id, name, slug) VALUES ($1, $2, $3)`, pg, slug, slug); err != nil {
		t.Fatalf("insert org: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM orgs WHERE id = $1`, pg)
	})
	return pg
}

func TestIncrementAIAgentUsage_CappedQuota(t *testing.T) {
	q, pool := testStore(t)
	org := newTestOrg(t, pool)
	ctx := context.Background()
	const period, quota = "2099-01", 3

	// The first `quota` calls are allowed and return an increasing count.
	for i := 1; i <= quota; i++ {
		count, allowed, err := q.IncrementAIAgentUsage(ctx, org, period, quota)
		if err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
		if !allowed || count != i {
			t.Fatalf("call %d: allowed=%v count=%d, want allowed=true count=%d", i, allowed, count, i)
		}
	}

	// Further calls are rejected and must not consume quota.
	for i := 0; i < 2; i++ {
		count, allowed, err := q.IncrementAIAgentUsage(ctx, org, period, quota)
		if err != nil {
			t.Fatalf("over-quota call: %v", err)
		}
		if allowed {
			t.Fatalf("over-quota call was allowed; count=%d", count)
		}
	}

	// The stored counter must be exactly the quota — rejected calls didn't bump it.
	var stored int
	if err := pool.QueryRow(ctx,
		`SELECT count FROM ai_usage WHERE org_id = $1 AND period = $2`, org, period).Scan(&stored); err != nil {
		t.Fatalf("read count: %v", err)
	}
	if stored != quota {
		t.Errorf("stored count = %d, want %d", stored, quota)
	}
}

func TestIncrementAIAgentUsage_Unlimited(t *testing.T) {
	q, pool := testStore(t)
	org := newTestOrg(t, pool)
	ctx := context.Background()

	for i := 1; i <= 5; i++ {
		count, allowed, err := q.IncrementAIAgentUsage(ctx, org, "2099-02", -1)
		if err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
		if !allowed || count != i {
			t.Errorf("call %d: allowed=%v count=%d, want allowed=true count=%d", i, allowed, count, i)
		}
	}
}

func TestIncrementAIAgentUsage_PeriodsAreIndependent(t *testing.T) {
	q, pool := testStore(t)
	org := newTestOrg(t, pool)
	ctx := context.Background()

	if _, _, err := q.IncrementAIAgentUsage(ctx, org, "2099-03", 5); err != nil {
		t.Fatal(err)
	}
	// A different period starts fresh at 1.
	count, allowed, err := q.IncrementAIAgentUsage(ctx, org, "2099-04", 5)
	if err != nil {
		t.Fatal(err)
	}
	if !allowed || count != 1 {
		t.Errorf("new period: allowed=%v count=%d, want allowed=true count=1", allowed, count)
	}
}
