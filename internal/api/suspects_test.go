package api

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/urbangeeks/kollaber/internal/store"
)

func ev(eventType, service, status string, at time.Time) store.Event {
	return store.Event{
		ID:            pgtype.UUID{Bytes: uuid.New(), Valid: true},
		Type:          eventType,
		Service:       service,
		Status:        status,
		EnvironmentID: pgtype.UUID{Bytes: uuid.New(), Valid: true},
		Timestamp:     pgtype.Timestamptz{Time: at, Valid: true},
		Metadata:      []byte(`{}`),
	}
}

func TestRankSuspectsOrdersBySameServiceThenRecency(t *testing.T) {
	now := time.Now()
	window := 2 * time.Hour
	alert := ev("alert", "api", "failure", now)

	// A recent deploy to an unrelated service should still rank below an older
	// deploy to the alerting service: service match outweighs recency by design.
	other := ev("deploy", "worker", "success", now.Add(-1*time.Minute))
	same := ev("deploy", "api", "success", now.Add(-45*time.Minute))

	got := rankSuspects(alert, []store.Event{other, same}, window)

	if len(got) != 2 {
		t.Fatalf("want 2 suspects, got %d", len(got))
	}
	if got[0].Event.Service != "api" {
		t.Errorf("want same-service deploy first, got %q (scores: %d, %d)",
			got[0].Event.Service, got[0].Score, got[1].Score)
	}
}

func TestRankSuspectsScoreComponents(t *testing.T) {
	now := time.Now()
	window := 2 * time.Hour
	alert := ev("alert", "api", "failure", now)

	tests := []struct {
		name       string
		change     store.Event
		wantScore  int
		confidence string
	}{
		{
			// 45 service + 40 recency + 10 deploy + 5 failed = 100.
			name:       "same service failed deploy at zero lag",
			change:     ev("deploy", "api", "failure", now),
			wantScore:  100,
			confidence: "high",
		},
		{
			// 45 service + 20 recency (half the window) + 10 deploy = 75.
			name:       "same service deploy at half window",
			change:     ev("deploy", "api", "success", now.Add(-time.Hour)),
			wantScore:  75,
			confidence: "high",
		},
		{
			// 0 service + 40 recency + 4 scale = 44.
			name:       "other service scale at zero lag",
			change:     ev("scale", "worker", "success", now),
			wantScore:  44,
			confidence: "medium",
		},
		{
			// 0 service + ~0 recency + 4 scale = 4. At the window edge nothing
			// but the type weight survives.
			name:       "other service scale at window edge",
			change:     ev("scale", "worker", "success", now.Add(-2*time.Hour)),
			wantScore:  4,
			confidence: "low",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rankSuspects(alert, []store.Event{tt.change}, window)
			if len(got) != 1 {
				t.Fatalf("want 1 suspect, got %d", len(got))
			}
			if got[0].Score != tt.wantScore {
				t.Errorf("score = %d, want %d (reasons: %v)", got[0].Score, tt.wantScore, got[0].Reasons)
			}
			if got[0].Confidence != tt.confidence {
				t.Errorf("confidence = %q, want %q", got[0].Confidence, tt.confidence)
			}
		})
	}
}

// A change timestamped after the alert would earn a negative lag and an
// inflated recency score. ListChangesBefore filters these out, but the ranking
// clamps too so a future caller can't reintroduce the bug silently.
func TestRankSuspectsClampsNegativeLag(t *testing.T) {
	now := time.Now()
	alert := ev("alert", "api", "failure", now)
	future := ev("deploy", "api", "success", now.Add(30*time.Minute))

	got := rankSuspects(alert, []store.Event{future}, 2*time.Hour)

	if got[0].LagSeconds != 0 {
		t.Errorf("lag = %ds, want 0 for a change after the target", got[0].LagSeconds)
	}
	if got[0].Score > 100 {
		t.Errorf("score = %d, want <= 100", got[0].Score)
	}
}

func TestRankSuspectsTieBreaksTowardRecent(t *testing.T) {
	now := time.Now()
	alert := ev("alert", "api", "failure", now)

	// Identical in every scoring term except when they landed.
	older := ev("deploy", "worker", "success", now.Add(-90*time.Minute))
	newer := ev("deploy", "worker", "success", now.Add(-90*time.Minute))
	newer.Timestamp = pgtype.Timestamptz{Time: now.Add(-89 * time.Minute), Valid: true}

	got := rankSuspects(alert, []store.Event{older, newer}, 2*time.Hour)

	if got[0].LagSeconds > got[1].LagSeconds {
		t.Errorf("want the more recent change first on a tie, got lags %d then %d",
			got[0].LagSeconds, got[1].LagSeconds)
	}
}

func TestRankSuspectsEmpty(t *testing.T) {
	alert := ev("alert", "api", "failure", time.Now())
	got := rankSuspects(alert, nil, 2*time.Hour)
	if len(got) != 0 {
		t.Errorf("want no suspects, got %d", len(got))
	}
}
