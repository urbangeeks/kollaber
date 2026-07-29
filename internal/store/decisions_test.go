package store

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func markDecision(t *testing.T, q *Queries, commentID, orgID, userID pgtype.UUID) CommentDetail {
	t.Helper()
	got, found, err := q.SetCommentDecision(context.Background(), commentID, orgID, userID, true)
	if err != nil {
		t.Fatalf("SetCommentDecision: %v", err)
	}
	if !found {
		t.Fatal("comment not found when marking")
	}
	return got
}

func TestSetCommentDecisionMarksAndUnmarks(t *testing.T) {
	q, pool := testStore(t)
	org := newTestOrg(t, pool)
	env := newTestEnv(t, pool, org)
	user := newTestUser(t, pool)
	ctx := context.Background()

	event := insertEventAt(t, pool, env, "deploy", "api", time.Now())
	insertComment(t, pool, event, user, "we're rolling back")

	comments, err := q.ListCommentsForEvent(ctx, event, org)
	if err != nil {
		t.Fatalf("ListCommentsForEvent: %v", err)
	}
	if len(comments) != 1 {
		t.Fatalf("want 1 comment, got %d", len(comments))
	}
	if comments[0].IsDecision {
		t.Error("a new comment is already a decision")
	}

	marked := markDecision(t, q, comments[0].ID, org, user)
	if !marked.IsDecision {
		t.Error("comment was not marked")
	}
	if marked.DecidedBy == nil || !marked.DecidedBy.Valid {
		t.Error("decided_by was not recorded")
	}
	if !marked.DecidedAt.Valid {
		t.Error("decided_at was not recorded")
	}
	// The text of what was decided must survive curation untouched.
	if marked.Body != "we're rolling back" {
		t.Errorf("body changed to %q", marked.Body)
	}

	unmarked, found, err := q.SetCommentDecision(ctx, comments[0].ID, org, user, false)
	if err != nil || !found {
		t.Fatalf("unmark: err=%v found=%v", err, found)
	}
	if unmarked.IsDecision {
		t.Error("comment is still a decision")
	}
	// Cleared, so a re-marked comment cannot carry a stale attribution.
	if unmarked.DecidedBy != nil && unmarked.DecidedBy.Valid {
		t.Error("decided_by survived unmarking")
	}
	if unmarked.DecidedAt.Valid {
		t.Error("decided_at survived unmarking")
	}
}

// Comments carry no org_id of their own. A marker query that trusts the comment
// id alone lets one org edit another's discussion.
func TestSetCommentDecisionScopesToOrg(t *testing.T) {
	q, pool := testStore(t)
	ctx := context.Background()

	orgA := newTestOrg(t, pool)
	orgB := newTestOrg(t, pool)
	envB := newTestEnv(t, pool, orgB)
	userA := newTestUser(t, pool)
	userB := newTestUser(t, pool)

	eventB := insertEventAt(t, pool, envB, "deploy", "api", time.Now())
	insertComment(t, pool, eventB, userB, "org B decision")

	commentsB, err := q.ListCommentsForEvent(ctx, eventB, orgB)
	if err != nil {
		t.Fatalf("ListCommentsForEvent: %v", err)
	}
	if len(commentsB) != 1 {
		t.Fatalf("setup: want 1 comment, got %d", len(commentsB))
	}

	_, found, err := q.SetCommentDecision(ctx, commentsB[0].ID, orgA, userA, true)
	if err != nil {
		t.Fatalf("SetCommentDecision: %v", err)
	}
	if found {
		t.Error("org A marked a comment belonging to org B")
	}

	// And the row must actually be untouched, not merely reported as missing.
	after, err := q.ListCommentsForEvent(ctx, eventB, orgB)
	if err != nil {
		t.Fatalf("ListCommentsForEvent: %v", err)
	}
	if after[0].IsDecision {
		t.Error("org B's comment was modified by org A")
	}
}

// The pre-existing leak this replaced: listing by event id alone handed one
// org's discussion to anyone who could guess a uuid.
func TestListCommentsForEventScopesToOrg(t *testing.T) {
	q, pool := testStore(t)
	ctx := context.Background()

	orgA := newTestOrg(t, pool)
	orgB := newTestOrg(t, pool)
	envB := newTestEnv(t, pool, orgB)
	user := newTestUser(t, pool)

	eventB := insertEventAt(t, pool, envB, "alert", "api", time.Now())
	insertComment(t, pool, eventB, user, "org B private discussion")

	got, err := q.ListCommentsForEvent(ctx, eventB, orgA)
	if err != nil {
		t.Fatalf("ListCommentsForEvent: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("cross-tenant leak: org A read %d of org B's comments", len(got))
	}

	// The same call by the owning org must still work, so the assertion above
	// is proving scoping rather than a query that returns nothing either way.
	own, err := q.ListCommentsForEvent(ctx, eventB, orgB)
	if err != nil {
		t.Fatalf("ListCommentsForEvent own org: %v", err)
	}
	if len(own) != 1 || own[0].Body != "org B private discussion" {
		t.Errorf("want org B's own comment back, got %d rows", len(own))
	}
	if own[0].AuthorEmail == "" {
		t.Error("author email not populated")
	}
}

func TestListDecisionsReturnsOnlyMarkedComments(t *testing.T) {
	q, pool := testStore(t)
	org := newTestOrg(t, pool)
	env := newTestEnv(t, pool, org)
	user := newTestUser(t, pool)
	ctx := context.Background()

	event := insertEventAt(t, pool, env, "deploy", "api", time.Now())
	insertComment(t, pool, event, user, "looking into it")
	insertComment(t, pool, event, user, "accepting this risk until Q3")

	comments, err := q.ListCommentsForEvent(ctx, event, org)
	if err != nil {
		t.Fatalf("ListCommentsForEvent: %v", err)
	}
	var target pgtype.UUID
	for _, c := range comments {
		if c.Body == "accepting this risk until Q3" {
			target = c.ID
		}
	}
	markDecision(t, q, target, org, user)

	got, err := q.ListDecisions(ctx, ListDecisionsParams{OrgID: org, Limit: 50})
	if err != nil {
		t.Fatalf("ListDecisions: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 decision, got %d", len(got))
	}
	if got[0].Body != "accepting this risk until Q3" {
		t.Errorf("wrong decision returned: %q", got[0].Body)
	}
	// The event context is the point — a decision with no subject is useless.
	if got[0].EventType != "deploy" || got[0].EventService != "api" {
		t.Errorf("event context missing: %s/%s", got[0].EventType, got[0].EventService)
	}
	if got[0].EnvironmentName == "" {
		t.Error("environment name not populated")
	}
}

func TestListDecisionsScopesToOrg(t *testing.T) {
	q, pool := testStore(t)
	ctx := context.Background()

	orgA := newTestOrg(t, pool)
	orgB := newTestOrg(t, pool)
	envB := newTestEnv(t, pool, orgB)
	user := newTestUser(t, pool)

	eventB := insertEventAt(t, pool, envB, "deploy", "api", time.Now())
	insertComment(t, pool, eventB, user, "org B decision")
	commentsB, _ := q.ListCommentsForEvent(ctx, eventB, orgB)
	markDecision(t, q, commentsB[0].ID, orgB, user)

	got, err := q.ListDecisions(ctx, ListDecisionsParams{OrgID: orgA, Limit: 50})
	if err != nil {
		t.Fatalf("ListDecisions: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("cross-tenant leak: org A read %d of org B's decisions", len(got))
	}
}

func TestListDecisionsFiltersByEnvironment(t *testing.T) {
	q, pool := testStore(t)
	org := newTestOrg(t, pool)
	prod := newTestEnv(t, pool, org)
	staging := newTestEnv(t, pool, org)
	user := newTestUser(t, pool)
	ctx := context.Background()

	for _, env := range []pgtype.UUID{prod, staging} {
		event := insertEventAt(t, pool, env, "deploy", "api", time.Now())
		insertComment(t, pool, event, user, "a decision")
		comments, _ := q.ListCommentsForEvent(ctx, event, org)
		markDecision(t, q, comments[0].ID, org, user)
	}

	all, err := q.ListDecisions(ctx, ListDecisionsParams{OrgID: org, Limit: 50})
	if err != nil {
		t.Fatalf("ListDecisions: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("want 2 decisions across the org, got %d", len(all))
	}

	scoped, err := q.ListDecisions(ctx, ListDecisionsParams{OrgID: org, EnvironmentID: &prod, Limit: 50})
	if err != nil {
		t.Fatalf("ListDecisions scoped: %v", err)
	}
	if len(scoped) != 1 {
		t.Errorf("want 1 decision in prod, got %d", len(scoped))
	}
}

// Ordered by when the comment was written, not when it was marked, so someone
// tidying up old threads on a Friday does not reshuffle the history.
func TestListDecisionsOrderedByCommentTime(t *testing.T) {
	q, pool := testStore(t)
	org := newTestOrg(t, pool)
	env := newTestEnv(t, pool, org)
	user := newTestUser(t, pool)
	ctx := context.Background()

	event := insertEventAt(t, pool, env, "deploy", "api", time.Now())
	mustExec(t, pool,
		`INSERT INTO comments (event_id, user_id, body, created_at) VALUES ($1, $2, 'older', NOW() - INTERVAL '2 days')`,
		event, user)
	mustExec(t, pool,
		`INSERT INTO comments (event_id, user_id, body, created_at) VALUES ($1, $2, 'newer', NOW() - INTERVAL '1 hour')`,
		event, user)

	comments, _ := q.ListCommentsForEvent(ctx, event, org)
	// Mark the older one last, so marking order is the reverse of comment order.
	for _, want := range []string{"newer", "older"} {
		for _, c := range comments {
			if c.Body == want {
				markDecision(t, q, c.ID, org, user)
			}
		}
	}

	got, err := q.ListDecisions(ctx, ListDecisionsParams{OrgID: org, Limit: 50})
	if err != nil {
		t.Fatalf("ListDecisions: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 decisions, got %d", len(got))
	}
	if got[0].Body != "newer" || got[1].Body != "older" {
		t.Errorf("wrong order: %q then %q", got[0].Body, got[1].Body)
	}
}

func TestCountDecisions(t *testing.T) {
	q, pool := testStore(t)
	org := newTestOrg(t, pool)
	env := newTestEnv(t, pool, org)
	user := newTestUser(t, pool)
	ctx := context.Background()

	event := insertEventAt(t, pool, env, "deploy", "api", time.Now())
	insertComment(t, pool, event, user, "one")
	insertComment(t, pool, event, user, "two")

	n, err := q.CountDecisions(ctx, org, nil)
	if err != nil {
		t.Fatalf("CountDecisions: %v", err)
	}
	if n != 0 {
		t.Errorf("count = %d before anything was marked, want 0", n)
	}

	comments, _ := q.ListCommentsForEvent(ctx, event, org)
	markDecision(t, q, comments[0].ID, org, user)

	if n, err = q.CountDecisions(ctx, org, nil); err != nil {
		t.Fatalf("CountDecisions: %v", err)
	}
	if n != 1 {
		t.Errorf("count = %d, want 1", n)
	}
}
