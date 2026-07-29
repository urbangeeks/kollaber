package api

import (
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v4"
	"github.com/urbangeeks/kollaber/internal/middleware"
	"github.com/urbangeeks/kollaber/internal/store"
)

const (
	defaultDecisionLimit = 50
	maxDecisionLimit     = 200
)

type DecisionsHandler struct{ q *store.Queries }

func NewDecisionsHandler(q *store.Queries) *DecisionsHandler { return &DecisionsHandler{q} }

// decisionResponse carries the event the decision was made about. A decision
// without its context is a sentence with no subject: "we're rolling back" is
// only useful next to the deploy it was said about.
type decisionResponse struct {
	ID              string  `json:"id"`
	EventID         string  `json:"event_id"`
	Body            string  `json:"body"`
	Author          string  `json:"author"`
	CreatedAt       string  `json:"created_at"`
	DecidedBy       *string `json:"decided_by"`
	DecidedAt       *string `json:"decided_at"`
	EventType       string  `json:"event_type"`
	EventService    string  `json:"event_service"`
	EventTimestamp  string  `json:"event_timestamp"`
	EnvironmentID   string  `json:"environment_id"`
	EnvironmentName string  `json:"environment_name"`
}

type decisionsResponse struct {
	Decisions []decisionResponse `json:"decisions"`
	Total     int64              `json:"total"`
}

func toDecisionResponse(d store.Decision) decisionResponse {
	out := decisionResponse{
		ID:              uuid.UUID(d.ID.Bytes).String(),
		EventID:         uuid.UUID(d.EventID.Bytes).String(),
		Body:            d.Body,
		Author:          d.AuthorEmail,
		CreatedAt:       rfc3339(d.CreatedAt),
		EventType:       d.EventType,
		EventService:    d.EventService,
		EventTimestamp:  rfc3339(d.EventTimestamp),
		EnvironmentID:   uuid.UUID(d.EnvironmentID.Bytes).String(),
		EnvironmentName: d.EnvironmentName,
	}
	if d.DecidedBy != nil && d.DecidedBy.Valid {
		s := uuid.UUID(d.DecidedBy.Bytes).String()
		out.DecidedBy = &s
	}
	if d.DecidedAt.Valid {
		s := rfc3339(d.DecidedAt)
		out.DecidedAt = &s
	}
	return out
}

func rfc3339(ts pgtype.Timestamptz) string {
	if !ts.Valid {
		return ""
	}
	return ts.Time.Format("2006-01-02T15:04:05Z07:00")
}

// List handles GET /decisions — the org's decision log, newest first,
// optionally narrowed to one environment.
func (h *DecisionsHandler) List(c echo.Context) error {
	orgID := c.Get(middleware.OrgIDKey).(uuid.UUID)
	pgOrgID := pgtype.UUID{Bytes: orgID, Valid: true}
	ctx := c.Request().Context()

	var envID *pgtype.UUID
	if raw := c.QueryParam("environment_id"); raw != "" {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid environment_id"})
		}
		pg := pgtype.UUID{Bytes: parsed, Valid: true}
		envID = &pg
	}

	limit := defaultDecisionLimit
	if raw := c.QueryParam("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "limit must be a positive integer"})
		}
		limit = min(parsed, maxDecisionLimit)
	}

	offset := 0
	if raw := c.QueryParam("offset"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "offset must be zero or greater"})
		}
		offset = parsed
	}

	decisions, err := h.q.ListDecisions(ctx, store.ListDecisionsParams{
		OrgID:         pgOrgID,
		EnvironmentID: envID,
		Limit:         int32(limit),
		Offset:        int32(offset),
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not load decisions"})
	}

	total, err := h.q.CountDecisions(ctx, pgOrgID, envID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not count decisions"})
	}

	out := make([]decisionResponse, 0, len(decisions))
	for _, d := range decisions {
		out = append(out, toDecisionResponse(d))
	}
	return c.JSON(http.StatusOK, decisionsResponse{Decisions: out, Total: total})
}
