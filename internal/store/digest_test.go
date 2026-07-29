package store

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

func mustExec(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("exec: %v", err)
	}
}

func addMember(t *testing.T, pool *pgxpool.Pool, orgID, userID pgtype.UUID) {
	t.Helper()
	mustExec(t, pool,
		`INSERT INTO org_members (org_id, user_id, role) VALUES ($1, $2, 'member')`,
		orgID, userID)
}

// setNotifyOn writes a member's notification preferences. Pass an empty
// notificationEmail to leave the override unset, which is the common case.
func setNotifyOn(t *testing.T, pool *pgxpool.Pool, orgID, userID pgtype.UUID, notifyOn []string, notificationEmail string) {
	t.Helper()
	mustExec(t, pool,
		`INSERT INTO notification_prefs (org_id, user_id, notify_on, notification_email)
		 VALUES ($1, $2, $3, NULLIF($4, ''))`,
		orgID, userID, notifyOn, notificationEmail)
}

func mondayOf(t time.Time) time.Time {
	t = t.UTC()
	offset := (int(t.Weekday()) + 6) % 7
	d := t.AddDate(0, 0, -offset)
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
}

// The claim is the whole multi-replica story: every pod runs the same schedule,
// so if two of them could both claim a week, every org would be mailed twice.
func TestClaimWeeklyDigestIsExactlyOnce(t *testing.T) {
	q, pool := testStore(t)
	org := newTestOrg(t, pool)
	ctx := context.Background()
	week := mondayOf(time.Now())

	first, err := q.ClaimWeeklyDigest(ctx, org, week)
	if err != nil {
		t.Fatalf("first claim: %v", err)
	}
	if !first {
		t.Fatal("the first claim did not win")
	}

	// Stands in for a second replica on the same tick, and for the same replica
	// on the next tick an hour later.
	for i := range 3 {
		again, err := q.ClaimWeeklyDigest(ctx, org, week)
		if err != nil {
			t.Fatalf("claim %d: %v", i+2, err)
		}
		if again {
			t.Fatalf("claim %d won a week that was already claimed", i+2)
		}
	}
}

func TestClaimWeeklyDigestIsPerWeek(t *testing.T) {
	q, pool := testStore(t)
	org := newTestOrg(t, pool)
	ctx := context.Background()
	thisWeek := mondayOf(time.Now())

	if _, err := q.ClaimWeeklyDigest(ctx, org, thisWeek); err != nil {
		t.Fatalf("claim this week: %v", err)
	}

	// Next week is a different row, or the digest would only ever send once.
	next, err := q.ClaimWeeklyDigest(ctx, org, thisWeek.AddDate(0, 0, 7))
	if err != nil {
		t.Fatalf("claim next week: %v", err)
	}
	if !next {
		t.Error("a later week could not be claimed")
	}
}

func TestClaimWeeklyDigestIsPerOrg(t *testing.T) {
	q, pool := testStore(t)
	orgA := newTestOrg(t, pool)
	orgB := newTestOrg(t, pool)
	ctx := context.Background()
	week := mondayOf(time.Now())

	if _, err := q.ClaimWeeklyDigest(ctx, orgA, week); err != nil {
		t.Fatalf("claim org A: %v", err)
	}
	claimed, err := q.ClaimWeeklyDigest(ctx, orgB, week)
	if err != nil {
		t.Fatalf("claim org B: %v", err)
	}
	if !claimed {
		t.Error("one org's claim blocked another's")
	}
}

// A send that failed must be retryable, or a transient blip costs the org the
// whole week.
func TestReleaseWeeklyDigestAllowsARetry(t *testing.T) {
	q, pool := testStore(t)
	org := newTestOrg(t, pool)
	ctx := context.Background()
	week := mondayOf(time.Now())

	if _, err := q.ClaimWeeklyDigest(ctx, org, week); err != nil {
		t.Fatalf("claim: %v", err)
	}
	if err := q.ReleaseWeeklyDigest(ctx, org, week); err != nil {
		t.Fatalf("release: %v", err)
	}

	again, err := q.ClaimWeeklyDigest(ctx, org, week)
	if err != nil {
		t.Fatalf("reclaim: %v", err)
	}
	if !again {
		t.Error("a released week could not be reclaimed")
	}
}

func TestDigestRecipientsRespectOptIn(t *testing.T) {
	q, pool := testStore(t)
	org := newTestOrg(t, pool)
	ctx := context.Background()

	subscribed := newTestUser(t, pool)
	other := newTestUser(t, pool)
	addMember(t, pool, org, subscribed)
	addMember(t, pool, org, other)
	setNotifyOn(t, pool, org, subscribed, []string{"deploy", "digest"}, "")
	setNotifyOn(t, pool, org, other, []string{"deploy"}, "")

	got, err := q.ListDigestRecipients(ctx, org)
	if err != nil {
		t.Fatalf("ListDigestRecipients: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 recipient, got %d: %v", len(got), got)
	}
}

// Someone who signed up with one address and routes mail to another asked for
// it to go to the other one.
func TestDigestRecipientsPreferTheNotificationEmail(t *testing.T) {
	q, pool := testStore(t)
	org := newTestOrg(t, pool)
	ctx := context.Background()

	user := newTestUser(t, pool)
	addMember(t, pool, org, user)
	setNotifyOn(t, pool, org, user, []string{"digest"}, "ops-team@example.com")

	got, err := q.ListDigestRecipients(ctx, org)
	if err != nil {
		t.Fatalf("ListDigestRecipients: %v", err)
	}
	if len(got) != 1 || got[0] != "ops-team@example.com" {
		t.Errorf("recipients = %v, want the notification_email override", got)
	}
}

// A preference row outlives the membership it was written for. A former member
// must stop receiving the org's weekly summary.
func TestDigestRecipientsExcludeFormerMembers(t *testing.T) {
	q, pool := testStore(t)
	org := newTestOrg(t, pool)
	ctx := context.Background()

	user := newTestUser(t, pool)
	addMember(t, pool, org, user)
	setNotifyOn(t, pool, org, user, []string{"digest"}, "")

	if _, err := pool.Exec(ctx,
		`DELETE FROM org_members WHERE org_id = $1 AND user_id = $2`, org, user); err != nil {
		t.Fatalf("remove member: %v", err)
	}

	got, err := q.ListDigestRecipients(ctx, org)
	if err != nil {
		t.Fatalf("ListDigestRecipients: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("a former member is still receiving the digest: %v", got)
	}
}

func TestListDigestOrgsOnlyIncludesSubscribedOrgs(t *testing.T) {
	q, pool := testStore(t)
	ctx := context.Background()

	withSub := newTestOrg(t, pool)
	withoutSub := newTestOrg(t, pool)

	a := newTestUser(t, pool)
	addMember(t, pool, withSub, a)
	setNotifyOn(t, pool, withSub, a, []string{"digest"}, "")

	b := newTestUser(t, pool)
	addMember(t, pool, withoutSub, b)
	setNotifyOn(t, pool, withoutSub, b, []string{"deploy"}, "")

	orgs, err := q.ListDigestOrgs(ctx)
	if err != nil {
		t.Fatalf("ListDigestOrgs: %v", err)
	}

	var sawSubscribed, sawUnsubscribed bool
	for _, o := range orgs {
		if o.ID == withSub {
			sawSubscribed = true
		}
		if o.ID == withoutSub {
			sawUnsubscribed = true
		}
	}
	if !sawSubscribed {
		t.Error("an org with a subscriber was not listed")
	}
	if sawUnsubscribed {
		t.Error("an org with no subscriber was listed")
	}
}

func TestDigestEnvironmentsCountsByType(t *testing.T) {
	q, pool := testStore(t)
	org := newTestOrg(t, pool)
	env := newTestEnv(t, pool, org)
	ctx := context.Background()

	now := time.Now()
	from := now.Add(-24 * time.Hour)
	to := now.Add(time.Hour)

	insertEventAt(t, pool, env, "deploy", "api", now.Add(-2*time.Hour))
	insertEventAt(t, pool, env, "deploy", "api", now.Add(-3*time.Hour))
	insertEventAt(t, pool, env, "rollback", "api", now.Add(-time.Hour))
	insertEventAt(t, pool, env, "alert", "api", now.Add(-90*time.Minute))
	insertEventAt(t, pool, env, "note", "api", now.Add(-4*time.Hour))
	// Outside the window: must not be counted.
	insertEventAt(t, pool, env, "deploy", "api", now.Add(-72*time.Hour))

	got, err := q.DigestEnvironments(ctx, org, from, to)
	if err != nil {
		t.Fatalf("DigestEnvironments: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 environment, got %d", len(got))
	}

	e := got[0]
	if e.Deploys != 2 {
		t.Errorf("deploys = %d, want 2", e.Deploys)
	}
	if e.Rollbacks != 1 {
		t.Errorf("rollbacks = %d, want 1", e.Rollbacks)
	}
	if e.Alerts != 1 {
		t.Errorf("alerts = %d, want 1", e.Alerts)
	}
	// The note counts toward the total even though it is not broken out.
	if e.Total != 5 {
		t.Errorf("total = %d, want 5", e.Total)
	}
}

// An environment with no events in the window must report zero rather than
// being counted once for its own row by the LEFT JOIN.
func TestDigestEnvironmentsCountsQuietEnvironmentsAsZero(t *testing.T) {
	q, pool := testStore(t)
	org := newTestOrg(t, pool)
	newTestEnv(t, pool, org)
	ctx := context.Background()
	now := time.Now()

	got, err := q.DigestEnvironments(ctx, org, now.Add(-24*time.Hour), now)
	if err != nil {
		t.Fatalf("DigestEnvironments: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want the quiet environment listed, got %d rows", len(got))
	}
	if got[0].Total != 0 {
		t.Errorf("total = %d, want 0", got[0].Total)
	}
}

func TestDigestEnvironmentsScopesToOrg(t *testing.T) {
	q, pool := testStore(t)
	ctx := context.Background()
	now := time.Now()

	orgA := newTestOrg(t, pool)
	orgB := newTestOrg(t, pool)
	envB := newTestEnv(t, pool, orgB)
	insertEventAt(t, pool, envB, "deploy", "api", now.Add(-time.Hour))

	got, err := q.DigestEnvironments(ctx, orgA, now.Add(-24*time.Hour), now)
	if err != nil {
		t.Fatalf("DigestEnvironments: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("cross-tenant leak: org A saw %d of org B's environments", len(got))
	}
}

// Ranked by comments written this week, not by the event's own age, so an old
// event the team argued about on Tuesday still surfaces.
func TestDigestThreadsRankByCommentsInWindow(t *testing.T) {
	q, pool := testStore(t)
	org := newTestOrg(t, pool)
	env := newTestEnv(t, pool, org)
	user := newTestUser(t, pool)
	ctx := context.Background()
	now := time.Now()

	busy := insertEventAt(t, pool, env, "alert", "checkout", now.Add(-2*time.Hour))
	quiet := insertEventAt(t, pool, env, "deploy", "api", now.Add(-time.Hour))
	old := insertEventAt(t, pool, env, "alert", "legacy", now.Add(-90*24*time.Hour))

	insertComment(t, pool, busy, user, "one")
	insertComment(t, pool, busy, user, "two")
	insertComment(t, pool, busy, user, "three")
	insertComment(t, pool, quiet, user, "only one here")
	insertComment(t, pool, old, user, "still arguing")
	insertComment(t, pool, old, user, "about this")

	got, err := q.DigestThreads(ctx, org, now.Add(-24*time.Hour), now.Add(time.Hour), 5)
	if err != nil {
		t.Fatalf("DigestThreads: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 threads, got %d", len(got))
	}
	if got[0].Comments != 3 || got[0].Service != "checkout" {
		t.Errorf("busiest thread = %s with %d comments", got[0].Service, got[0].Comments)
	}
	// The months-old event is present because the discussion is recent.
	var sawOld bool
	for _, th := range got {
		if th.Service == "legacy" {
			sawOld = true
		}
	}
	if !sawOld {
		t.Error("an old event with recent discussion was dropped")
	}
}

func TestDigestIncidentsCountsOpenedAndResolved(t *testing.T) {
	q, pool := testStore(t)
	org := newTestOrg(t, pool)
	ctx := context.Background()
	now := time.Now()
	from, to := now.Add(-24*time.Hour), now.Add(time.Hour)

	mustExec(t, pool, `INSERT INTO incidents (org_id, title, opened_at) VALUES ($1, 'in window', $2)`,
		org, now.Add(-2*time.Hour))
	mustExec(t, pool, `INSERT INTO incidents (org_id, title, opened_at, resolved_at, status)
		VALUES ($1, 'resolved in window', $2, $3, 'resolved')`,
		org, now.Add(-5*time.Hour), now.Add(-time.Hour))
	mustExec(t, pool, `INSERT INTO incidents (org_id, title, opened_at) VALUES ($1, 'too old', $2)`,
		org, now.Add(-72*time.Hour))

	opened, resolved, err := q.DigestIncidents(ctx, org, from, to)
	if err != nil {
		t.Fatalf("DigestIncidents: %v", err)
	}
	if opened != 2 {
		t.Errorf("opened = %d, want 2", opened)
	}
	if resolved != 1 {
		t.Errorf("resolved = %d, want 1", resolved)
	}
}
