package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v4"
	kollabai "github.com/urbangeeks/kollaber/internal/ai"
	"github.com/urbangeeks/kollaber/internal/billing"
	"github.com/urbangeeks/kollaber/internal/middleware"
	"github.com/urbangeeks/kollaber/internal/store"
)

const (
	postmortemWindow = 2 * time.Hour
	// Token budgets for the Haiku completions. Summaries are 1-3 sentences;
	// postmortems are a multi-section document and need much more room.
	summaryMaxTokens    = 256
	postmortemMaxTokens = 1024
)

type AIHandler struct{ q *store.Queries }

func NewAIHandler(q *store.Queries) *AIHandler { return &AIHandler{q} }

// wantsRefresh reports whether the caller asked to bypass the cached result and
// regenerate (e.g. ?refresh=true). The freshly generated value overwrites the
// cache.
func wantsRefresh(c echo.Context) bool {
	return c.QueryParam("refresh") == "true"
}

func (h *AIHandler) SummarizeEvent(c echo.Context) error {
	orgID := c.Get(middleware.OrgIDKey).(uuid.UUID)
	pgOrgID := pgtype.UUID{Bytes: orgID, Valid: true}
	ctx := context.Background()

	// entitlement check
	b, err := h.q.GetOrgBilling(ctx, pgOrgID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not load billing"})
	}
	if !billing.EntitlementsFor(b.Plan).AISummaries {
		return c.JSON(http.StatusPaymentRequired, echo.Map{
			"error":            "AI summaries require the Team plan or higher",
			"upgrade_required": true,
		})
	}

	eventID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid event id"})
	}

	event, err := h.q.GetEventByIDForOrg(ctx, pgtype.UUID{Bytes: eventID, Valid: true}, pgOrgID)
	if err != nil {
		return c.JSON(http.StatusNotFound, echo.Map{"error": "event not found"})
	}

	if event.AISummary != nil && !wantsRefresh(c) {
		return c.JSON(http.StatusOK, echo.Map{"summary": *event.AISummary})
	}

	comments, _ := h.q.ListCommentsByEvent(ctx, event.ID)

	prompt := buildSummaryPrompt(event, comments)
	summary, err := kollabai.Generate(ctx, prompt, summaryMaxTokens)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not generate summary"})
	}

	_ = h.q.SetEventAISummary(ctx, event.ID, summary)

	return c.JSON(http.StatusOK, echo.Map{"summary": summary})
}

func (h *AIHandler) PostmortemEvent(c echo.Context) error {
	orgID := c.Get(middleware.OrgIDKey).(uuid.UUID)
	pgOrgID := pgtype.UUID{Bytes: orgID, Valid: true}
	ctx := context.Background()

	b, err := h.q.GetOrgBilling(ctx, pgOrgID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not load billing"})
	}
	if !billing.EntitlementsFor(b.Plan).AIPostmortems {
		return c.JSON(http.StatusPaymentRequired, echo.Map{
			"error":            "AI postmortems require the Pro plan",
			"upgrade_required": true,
		})
	}

	eventID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid event id"})
	}

	event, err := h.q.GetEventByIDForOrg(ctx, pgtype.UUID{Bytes: eventID, Valid: true}, pgOrgID)
	if err != nil {
		return c.JSON(http.StatusNotFound, echo.Map{"error": "event not found"})
	}

	if event.AIPostmortem != nil && !wantsRefresh(c) {
		return c.JSON(http.StatusOK, echo.Map{"postmortem": *event.AIPostmortem})
	}

	comments, _ := h.q.ListCommentsByEvent(ctx, event.ID)
	nearby, _ := h.q.GetEventsAroundTime(ctx, event.EnvironmentID, event.Timestamp.Time, postmortemWindow, event.ID)

	prompt := buildPostmortemPrompt(event, comments, nearby)
	postmortem, err := kollabai.Generate(ctx, prompt, postmortemMaxTokens)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not generate postmortem"})
	}

	_ = h.q.SetEventAIPostmortem(ctx, event.ID, postmortem)

	return c.JSON(http.StatusOK, echo.Map{"postmortem": postmortem})
}

func (h *AIHandler) PostmortemIncident(c echo.Context) error {
	orgID := c.Get(middleware.OrgIDKey).(uuid.UUID)
	pgOrgID := pgtype.UUID{Bytes: orgID, Valid: true}
	ctx := context.Background()

	b, err := h.q.GetOrgBilling(ctx, pgOrgID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not load billing"})
	}
	if !billing.EntitlementsFor(b.Plan).AIPostmortems {
		return c.JSON(http.StatusPaymentRequired, echo.Map{
			"error":            "AI postmortems require the Pro plan",
			"upgrade_required": true,
		})
	}

	incidentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid incident id"})
	}
	pgID := pgtype.UUID{Bytes: incidentID, Valid: true}

	incident, err := h.q.GetIncidentByID(ctx, pgID, pgOrgID)
	if err != nil {
		return c.JSON(http.StatusNotFound, echo.Map{"error": "incident not found"})
	}

	if incident.AIPostmortem != nil && !wantsRefresh(c) {
		return c.JSON(http.StatusOK, echo.Map{"postmortem": *incident.AIPostmortem})
	}

	events, err := h.q.ListIncidentEvents(ctx, pgID, pgOrgID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not load incident events"})
	}

	// Gather comments across all linked events so the model sees the discussion.
	var comments []store.Comment
	for _, e := range events {
		ec, _ := h.q.ListCommentsByEvent(ctx, e.ID)
		comments = append(comments, ec...)
	}

	prompt := buildIncidentPostmortemPrompt(incident, events, comments)
	postmortem, err := kollabai.Generate(ctx, prompt, postmortemMaxTokens)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not generate postmortem"})
	}

	_ = h.q.SetIncidentAIPostmortem(ctx, incident.ID, postmortem)

	return c.JSON(http.StatusOK, echo.Map{"postmortem": postmortem})
}

func buildIncidentPostmortemPrompt(incident store.Incident, events []store.Event, comments []store.Comment) string {
	var sb strings.Builder
	sb.WriteString(`You are an SRE assistant. Write a concise incident postmortem based on the information below.
Use this exact structure with these section headers:

## What Happened
## Timeline
## Impact
## Root Cause
## Action Items

Be factual and concise. Use only information provided — do not invent details.

---
INCIDENT
`)
	fmt.Fprintf(&sb, "Title: %s\n", incident.Title)
	fmt.Fprintf(&sb, "Severity: %s\n", incident.Severity)
	fmt.Fprintf(&sb, "Status: %s\n", incident.Status)
	fmt.Fprintf(&sb, "Opened: %s UTC\n", incident.OpenedAt.Time.UTC().Format(time.RFC3339))
	if incident.ResolvedAt.Valid {
		fmt.Fprintf(&sb, "Resolved: %s UTC\n", incident.ResolvedAt.Time.UTC().Format(time.RFC3339))
	}

	if len(events) > 0 {
		sb.WriteString("\nLINKED EVENTS\n")
		for _, e := range events {
			writeEventBlock(&sb, e)
		}
	}

	if len(comments) > 0 {
		sb.WriteString("\nCOMMENTS\n")
		for _, cm := range comments {
			fmt.Fprintf(&sb, "- %s\n", cm.Body)
		}
	}

	return sb.String()
}

func buildPostmortemPrompt(event store.Event, comments []store.Comment, nearby []store.Event) string {
	var sb strings.Builder
	sb.WriteString(`You are an SRE assistant. Write a concise incident postmortem based on the information below.
Use this exact structure with these section headers:

## What Happened
## Timeline
## Impact
## Root Cause
## Action Items

Be factual and concise. Use only information provided — do not invent details.

---
INCIDENT EVENT
`)
	writeEventBlock(&sb, event)

	if len(comments) > 0 {
		sb.WriteString("\nCOMMENTS ON INCIDENT\n")
		for _, c := range comments {
			fmt.Fprintf(&sb, "- %s\n", c.Body)
		}
	}

	if len(nearby) > 0 {
		sb.WriteString("\nRELATED EVENTS (±2 hours, same environment)\n")
		for _, e := range nearby {
			writeEventBlock(&sb, e)
		}
	}

	return sb.String()
}

func writeEventBlock(sb *strings.Builder, e store.Event) {
	fmt.Fprintf(sb, "[%s] %s | service=%s status=%s\n",
		e.Timestamp.Time.UTC().Format(time.RFC3339),
		e.Type, e.Service, e.Status,
	)
	var meta map[string]any
	if len(e.Metadata) > 0 {
		_ = json.Unmarshal(e.Metadata, &meta)
	}
	for k, v := range meta {
		fmt.Fprintf(sb, "  %s=%v\n", k, v)
	}
}

func buildSummaryPrompt(event store.Event, comments []store.Comment) string {
	var sb strings.Builder
	sb.WriteString("You are an assistant for an infrastructure team. Summarize the following event in 1-3 concise sentences in plain English. Be factual. Output only the summary, no preamble.\n\n")
	fmt.Fprintf(&sb, "Type: %s\n", event.Type)
	fmt.Fprintf(&sb, "Service: %s\n", event.Service)
	fmt.Fprintf(&sb, "Status: %s\n", event.Status)
	fmt.Fprintf(&sb, "Time: %s UTC\n", event.Timestamp.Time.UTC().Format(time.RFC3339))

	var meta map[string]any
	if len(event.Metadata) > 0 {
		_ = json.Unmarshal(event.Metadata, &meta)
	}
	if len(meta) > 0 {
		sb.WriteString("Metadata:")
		for k, v := range meta {
			fmt.Fprintf(&sb, " %s=%v", k, v)
		}
		sb.WriteString("\n")
	}

	if len(comments) > 0 {
		sb.WriteString("Comments:\n")
		for _, cm := range comments {
			fmt.Fprintf(&sb, "- %s\n", cm.Body)
		}
	}

	return sb.String()
}
