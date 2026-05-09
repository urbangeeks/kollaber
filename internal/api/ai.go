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

type AIHandler struct{ q *store.Queries }

func NewAIHandler(q *store.Queries) *AIHandler { return &AIHandler{q} }

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

	comments, _ := h.q.ListCommentsByEvent(ctx, event.ID)

	prompt := buildSummaryPrompt(event, comments)
	summary, err := kollabai.SummarizeEvent(ctx, prompt)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not generate summary"})
	}

	return c.JSON(http.StatusOK, echo.Map{"summary": summary})
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
