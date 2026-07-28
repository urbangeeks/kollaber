package api

import (
	"net/http"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v4"
	"github.com/urbangeeks/kollaber/internal/middleware"
	"github.com/urbangeeks/kollaber/internal/store"
)

// Suspect scoring weights. They sum to 100, so a same-service deploy that failed
// and landed the instant before the alert scores 100 and the number reads as a
// percentage without further normalisation.
//
// These are heuristics, not statistics. We have no dependency graph and no
// causal model, so the score exists to *order* candidates and to show its
// working — never to assert a cause. That is why every component that fires
// also appends a human-readable reason: an SRE who disagrees with the ranking
// can see exactly which term they disagree with.
//
// Recency decays across the requested window rather than a fixed span, so the
// same change scores differently at different window sizes (a deploy 4m before
// an alert scores 93 over 2h but 78 over 10m). Scores are therefore only
// comparable within one response — which is all the UI ever does with them.
const (
	weightSameService  = 45
	weightRecency      = 40
	weightChangeType   = 10
	weightFailedChange = 5
)

// Confidence bands. A change scoring below mediumConfidence is still returned —
// "nothing changed in the last two hours" and "one unrelated scale event
// happened" are different answers, and the second is worth showing.
const (
	highConfidence   = 65
	mediumConfidence = 35
)

const (
	defaultSuspectWindow = 120  // minutes
	maxSuspectWindow     = 1440 // minutes; a day is the outer limit of plausible blame
	defaultSuspectLimit  = 5
	maxSuspectLimit      = 20
	// suspectCandidates caps what we pull from the database before ranking.
	// Ranking is O(n) and n is bounded by how much a team deploys in a day, so
	// this is a safety valve rather than a tuning knob.
	suspectCandidates = 200
)

// changeTypeWeight ranks how likely each kind of change is to have broken
// something, given no other information. A deploy ships new code and leads; a
// scale event changes capacity but not behaviour and trails.
func changeTypeWeight(eventType string) int {
	switch eventType {
	case "deploy":
		return 10
	case "rollback":
		return 8
	case "teardown":
		return 6
	case "scale":
		return 4
	default:
		return 0
	}
}

type suspectResponse struct {
	Event      eventResponse `json:"event"`
	Score      int           `json:"score"`
	Confidence string        `json:"confidence"`
	Reasons    []string      `json:"reasons"`
	// LagSeconds is how long before the triggering event this change landed.
	LagSeconds int64  `json:"lag_seconds"`
	LagDisplay string `json:"lag_display"`
}

type suspectsResponse struct {
	EventID       string            `json:"event_id"`
	WindowMinutes int               `json:"window_minutes"`
	Candidates    int               `json:"candidates"`
	Suspects      []suspectResponse `json:"suspects"`
}

// rankSuspects scores each change against the event it might explain and
// returns them best-first. It is pure so the weights can be tested without a
// database; the handler only supplies data.
//
// changes must already be filtered to events at or before target's timestamp —
// ListChangesBefore does this. A change with a later timestamp would produce a
// negative lag and an inflated recency score.
func rankSuspects(target store.Event, changes []store.Event, window time.Duration) []suspectResponse {
	targetAt := target.Timestamp.Time
	out := make([]suspectResponse, 0, len(changes))

	for _, ch := range changes {
		score := 0
		reasons := make([]string, 0, 4)

		if ch.Service == target.Service {
			score += weightSameService
			reasons = append(reasons, "same service as the alert")
		}

		lag := max(targetAt.Sub(ch.Timestamp.Time), 0)
		// Linear decay across the window: a change at the window's edge earns
		// nothing for recency, one at the same instant earns the full weight.
		if window > 0 {
			recency := float64(weightRecency) * (1 - lag.Seconds()/window.Seconds())
			if recency > 0 {
				score += int(recency)
			}
		}
		if lag <= 15*time.Minute {
			reasons = append(reasons, "landed shortly before the alert")
		}

		if w := changeTypeWeight(ch.Type); w > 0 {
			score += w
			reasons = append(reasons, "change type: "+ch.Type)
		}

		if ch.Status == "failure" {
			score += weightFailedChange
			reasons = append(reasons, "the change itself failed")
		}

		out = append(out, suspectResponse{
			Event:      toEventResponse(ch),
			Score:      score,
			Confidence: confidenceFor(score),
			Reasons:    reasons,
			LagSeconds: int64(lag.Seconds()),
			LagDisplay: humanDuration(lag.Seconds()) + " before",
		})
	}

	// Ties break toward the more recent change: with nothing else to separate
	// them, the last thing to touch the system is the better first guess.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].LagSeconds < out[j].LagSeconds
	})
	return out
}

func confidenceFor(score int) string {
	switch {
	case score >= highConfidence:
		return "high"
	case score >= mediumConfidence:
		return "medium"
	default:
		return "low"
	}
}

// Suspects handles GET /events/:id/suspects — the changes that preceded this
// event and could plausibly explain it.
//
// Any event type is accepted, not just alerts. The UI only offers this on
// alerts, but a failed deploy and the AI agent both benefit from asking "what
// else had just changed?", and refusing by type would block that for no gain.
func (h *EventsHandler) Suspects(c echo.Context) error {
	orgID := c.Get(middleware.OrgIDKey).(uuid.UUID)

	eventID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid event id"})
	}

	windowMinutes := parseInt32(c.QueryParam("window_minutes"), defaultSuspectWindow, maxSuspectWindow)
	limit := parseInt32(c.QueryParam("limit"), defaultSuspectLimit, maxSuspectLimit)
	window := time.Duration(windowMinutes) * time.Minute

	ctx := c.Request().Context()

	target, err := h.q.GetEventByIDForOrg(ctx,
		pgtype.UUID{Bytes: eventID, Valid: true},
		pgtype.UUID{Bytes: orgID, Valid: true},
	)
	if err != nil {
		// Joined on org, so a miss is either an unknown id or another org's
		// event. Both are "not found" — distinguishing them leaks existence.
		return c.JSON(http.StatusNotFound, echo.Map{"error": "event not found"})
	}

	changes, err := h.q.ListChangesBefore(ctx, store.ListChangesBeforeParams{
		OrgID:         pgtype.UUID{Bytes: orgID, Valid: true},
		EnvironmentID: target.EnvironmentID,
		Before:        target.Timestamp.Time,
		Window:        window,
		Limit:         suspectCandidates,
		ExcludeID:     target.ID,
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not load changes"})
	}

	ranked := rankSuspects(target, changes, window)
	if len(ranked) > int(limit) {
		ranked = ranked[:limit]
	}

	return c.JSON(http.StatusOK, suspectsResponse{
		EventID:       eventID.String(),
		WindowMinutes: int(windowMinutes),
		Candidates:    len(changes),
		Suspects:      ranked,
	})
}
