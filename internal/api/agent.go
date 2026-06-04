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
	"golang.org/x/time/rate"
)

const (
	agentMaxSteps      = 6
	agentMaxHistory    = 20
	agentDefaultLimit  = 20
	agentMaxEventLimit = 50
	agentDefaultWindow = 120 // minutes, for get_events_around_time

	// agentBurstRate/agentBurst bound how fast a single org may call the agent,
	// smoothing spikes. ~10/min with a small burst comfortably covers
	// interactive use while blocking scripted hammering.
	agentBurstRate = rate.Limit(1.0 / 6.0) // tokens per second (10/min)
	agentBurst     = 3
)

type AgentHandler struct {
	q       *store.Queries
	limiter *orgRateLimiter
}

func NewAgentHandler(q *store.Queries) *AgentHandler {
	return &AgentHandler{q: q, limiter: newOrgRateLimiter(agentBurstRate, agentBurst)}
}

type agentChatRequest struct {
	EnvironmentID string `json:"environment_id"`
	Messages      []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
}

// Chat runs the read-only timeline agent. The agent answers questions about an
// org's events/comments by calling tools that are scoped to the caller's org —
// every tool handler re-derives the org from the request, so the model can
// never read across tenants regardless of what it asks for.
func (h *AgentHandler) Chat(c echo.Context) error {
	orgID := c.Get(middleware.OrgIDKey).(uuid.UUID)
	pgOrgID := pgtype.UUID{Bytes: orgID, Valid: true}
	ctx := context.Background()

	b, err := h.q.GetOrgBilling(ctx, pgOrgID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not load billing"})
	}
	ent := billing.EntitlementsFor(b.Plan)
	if !ent.AIAgent {
		return c.JSON(http.StatusPaymentRequired, echo.Map{
			"error":            "The AI timeline assistant requires the Team plan or higher",
			"upgrade_required": true,
		})
	}

	// Burst protection: smooth out spikes from a single org before they reach
	// the (slow, expensive) model loop.
	if !h.limiter.Allow(orgID) {
		return c.JSON(http.StatusTooManyRequests, echo.Map{
			"error": "You're sending messages too quickly. Give the assistant a moment and try again.",
		})
	}

	// Monthly cost cap: atomically reserve a unit of this month's quota. A
	// rejected call does not consume quota.
	period := time.Now().UTC().Format("2006-01")
	_, allowed, err := h.q.IncrementAIAgentUsage(ctx, pgOrgID, period, ent.AIAgentMonthlyQuota)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not check usage"})
	}
	if !allowed {
		return c.JSON(http.StatusTooManyRequests, echo.Map{
			"error":            "You've reached this month's AI assistant limit. Upgrade your plan for a higher limit.",
			"upgrade_required": true,
		})
	}

	var req agentChatRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request"})
	}
	if len(req.Messages) == 0 {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "messages required"})
	}
	if len(req.Messages) > agentMaxHistory {
		req.Messages = req.Messages[len(req.Messages)-agentMaxHistory:]
	}

	history := make([]kollabai.Turn, 0, len(req.Messages))
	for _, m := range req.Messages {
		if strings.TrimSpace(m.Content) == "" {
			continue
		}
		history = append(history, kollabai.Turn{Role: m.Role, Text: m.Content})
	}
	if len(history) == 0 {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "messages required"})
	}

	// Optional default environment so the user doesn't have to name it.
	var defaultEnv *pgtype.UUID
	var defaultEnvName string
	if req.EnvironmentID != "" {
		if id, err := uuid.Parse(req.EnvironmentID); err == nil {
			pg := pgtype.UUID{Bytes: id, Valid: true}
			if env, err := h.q.GetEnvironmentByID(ctx, pg); err == nil && env.OrgID == pgOrgID {
				defaultEnv = &pg
				defaultEnvName = env.Name
			}
		}
	}

	tools, handlers := h.buildTools(pgOrgID, defaultEnv)
	system := buildAgentSystemPrompt(defaultEnvName)

	// All gating has passed; from here we stream Server-Sent Events. Each event
	// is a JSON object with a "type" of "token" (answer text), "step" (a tool
	// ran), "error", or "done".
	res := c.Response()
	res.Header().Set("Content-Type", "text/event-stream")
	res.Header().Set("Cache-Control", "no-cache")
	res.Header().Set("Connection", "keep-alive")
	res.Header().Set("X-Accel-Buffering", "no") // disable proxy buffering
	res.WriteHeader(http.StatusOK)

	send := func(v echo.Map) {
		if data, err := json.Marshal(v); err == nil {
			fmt.Fprintf(res, "data: %s\n\n", data)
			res.Flush()
		}
	}
	emit := func(ev kollabai.StreamEvent) {
		switch ev.Type {
		case "token":
			send(echo.Map{"type": "token", "text": ev.Token})
		case "step":
			send(echo.Map{"type": "step", "tool": ev.Step.Tool})
		}
	}

	// Tie the agent run to the request so a client disconnect cancels it.
	if _, err := kollabai.RunAgent(c.Request().Context(), system, tools, handlers, history, agentMaxSteps, emit); err != nil {
		send(echo.Map{"type": "error", "error": "the assistant is unavailable right now"})
		return nil
	}
	send(echo.Map{"type": "done"})
	return nil
}

func buildAgentSystemPrompt(envName string) string {
	var sb strings.Builder
	sb.WriteString(`You are Kollaber's timeline assistant, helping an infrastructure team understand activity in their shared event timeline. The timeline contains events of type "deploy", "alert", and "note", each belonging to an environment (e.g. prod, staging) and a service.

Answer questions by calling the provided read-only tools to look up real data. Never invent events, services, timestamps, or metadata — if the tools don't return it, say you don't know. Resolve relative times (e.g. "this afternoon", "yesterday") against the current time given below. Be concise and factual. When you reference an event, mention its service and time so the user can find it.

`)
	fmt.Fprintf(&sb, "Current time: %s UTC.\n", time.Now().UTC().Format(time.RFC3339))
	if envName != "" {
		fmt.Fprintf(&sb, "The user is currently viewing the %q environment. Unless they say otherwise, scope queries to it.\n", envName)
	}
	return sb.String()
}

// buildTools returns the tool schemas and their handlers, all bound to orgID.
// defaultEnv, when set, is used by list_events when the model omits an environment.
func (h *AgentHandler) buildTools(orgID pgtype.UUID, defaultEnv *pgtype.UUID) ([]kollabai.Tool, map[string]kollabai.ToolHandler) {
	tools := []kollabai.Tool{
		{
			Name:        "list_environments",
			Description: "List the environments in this organization with their IDs and names. Use this to map an environment name to its ID.",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			Name:        "list_events",
			Description: "List recent events in the timeline, newest first. All filters are optional. Use this to find deploys, alerts, or notes matching some criteria.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"environment_id": map[string]any{"type": "string", "description": "Restrict to one environment (UUID). Omit to use the current environment or search all."},
					"type":           map[string]any{"type": "string", "enum": []string{"deploy", "alert", "note"}},
					"service":        map[string]any{"type": "string", "description": "Exact service name."},
					"status":         map[string]any{"type": "string", "enum": []string{"success", "failure", "in_progress"}},
					"after":          map[string]any{"type": "string", "description": "Only events after this RFC3339 timestamp."},
					"before":         map[string]any{"type": "string", "description": "Only events before this RFC3339 timestamp."},
					"limit":          map[string]any{"type": "integer", "description": "Max events to return (default 20, max 50)."},
				},
			},
		},
		{
			Name:        "get_event",
			Description: "Get full detail (including metadata) for a single event by its ID.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{"event_id": map[string]any{"type": "string"}},
				"required":   []string{"event_id"},
			},
		},
		{
			Name:        "list_comments",
			Description: "List the discussion comments people have left on a specific event.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{"event_id": map[string]any{"type": "string"}},
				"required":   []string{"event_id"},
			},
		},
		{
			Name:        "get_events_around_time",
			Description: "List events in the same environment that occurred near a given event (within a time window). Useful for correlating an alert with nearby deploys.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"event_id":       map[string]any{"type": "string"},
					"window_minutes": map[string]any{"type": "integer", "description": "Half-width of the window in minutes (default 120)."},
				},
				"required": []string{"event_id"},
			},
		},
	}

	handlers := map[string]kollabai.ToolHandler{
		"list_environments":      h.toolListEnvironments(orgID),
		"list_events":            h.toolListEvents(orgID, defaultEnv),
		"get_event":              h.toolGetEvent(orgID),
		"list_comments":          h.toolListComments(orgID),
		"get_events_around_time": h.toolEventsAroundTime(orgID),
	}
	return tools, handlers
}

func (h *AgentHandler) toolListEnvironments(orgID pgtype.UUID) kollabai.ToolHandler {
	return func(ctx context.Context, _ json.RawMessage) (string, error) {
		envs, err := h.q.ListEnvironments(ctx, orgID)
		if err != nil {
			return "", fmt.Errorf("could not list environments")
		}
		if len(envs) == 0 {
			return "No environments.", nil
		}
		var sb strings.Builder
		for _, e := range envs {
			fmt.Fprintf(&sb, "- %s (id=%s, cluster=%s)\n", e.Name, uuidString(e.ID), e.ClusterName)
		}
		return sb.String(), nil
	}
}

func (h *AgentHandler) toolListEvents(orgID pgtype.UUID, defaultEnv *pgtype.UUID) kollabai.ToolHandler {
	return func(ctx context.Context, input json.RawMessage) (string, error) {
		var in struct {
			EnvironmentID string `json:"environment_id"`
			Type          string `json:"type"`
			Service       string `json:"service"`
			Status        string `json:"status"`
			After         string `json:"after"`
			Before        string `json:"before"`
			Limit         int    `json:"limit"`
		}
		_ = json.Unmarshal(input, &in)

		params := store.ListEventsFilterParams{
			OrgID:   orgID,
			Type:    in.Type,
			Service: in.Service,
			Status:  in.Status,
			Limit:   agentDefaultLimit,
		}
		if in.Limit > 0 {
			if in.Limit > agentMaxEventLimit {
				in.Limit = agentMaxEventLimit
			}
			params.Limit = int32(in.Limit)
		}
		switch {
		case in.EnvironmentID != "":
			id, err := uuid.Parse(in.EnvironmentID)
			if err != nil {
				return "", fmt.Errorf("invalid environment_id")
			}
			pg := pgtype.UUID{Bytes: id, Valid: true}
			params.EnvironmentID = &pg
		case defaultEnv != nil:
			params.EnvironmentID = defaultEnv
		}
		if in.After != "" {
			if t, err := time.Parse(time.RFC3339, in.After); err == nil {
				params.After = &t
			}
		}
		if in.Before != "" {
			if t, err := time.Parse(time.RFC3339, in.Before); err == nil {
				params.Before = &t
			}
		}

		events, err := h.q.ListEventsFiltered(ctx, params)
		if err != nil {
			return "", fmt.Errorf("could not list events")
		}
		if len(events) == 0 {
			return "No matching events.", nil
		}
		var sb strings.Builder
		for _, e := range events {
			writeEventLine(&sb, e)
		}
		return sb.String(), nil
	}
}

func (h *AgentHandler) toolGetEvent(orgID pgtype.UUID) kollabai.ToolHandler {
	return func(ctx context.Context, input json.RawMessage) (string, error) {
		event, err := h.eventForOrg(ctx, input, orgID)
		if err != nil {
			return "", err
		}
		var sb strings.Builder
		writeEventLine(&sb, event)
		var meta map[string]any
		if len(event.Metadata) > 0 {
			_ = json.Unmarshal(event.Metadata, &meta)
		}
		if len(meta) > 0 {
			sb.WriteString("metadata:\n")
			for k, v := range meta {
				fmt.Fprintf(&sb, "  %s = %v\n", k, v)
			}
		}
		if event.AISummary != nil {
			fmt.Fprintf(&sb, "existing AI summary: %s\n", *event.AISummary)
		}
		return sb.String(), nil
	}
}

func (h *AgentHandler) toolListComments(orgID pgtype.UUID) kollabai.ToolHandler {
	return func(ctx context.Context, input json.RawMessage) (string, error) {
		event, err := h.eventForOrg(ctx, input, orgID)
		if err != nil {
			return "", err
		}
		comments, err := h.q.ListCommentsByEvent(ctx, event.ID)
		if err != nil {
			return "", fmt.Errorf("could not list comments")
		}
		if len(comments) == 0 {
			return "No comments on this event.", nil
		}
		var sb strings.Builder
		for _, cm := range comments {
			fmt.Fprintf(&sb, "- [%s] %s\n", cm.CreatedAt.Time.UTC().Format(time.RFC3339), cm.Body)
		}
		return sb.String(), nil
	}
}

func (h *AgentHandler) toolEventsAroundTime(orgID pgtype.UUID) kollabai.ToolHandler {
	return func(ctx context.Context, input json.RawMessage) (string, error) {
		event, err := h.eventForOrg(ctx, input, orgID)
		if err != nil {
			return "", err
		}
		var in struct {
			WindowMinutes int `json:"window_minutes"`
		}
		_ = json.Unmarshal(input, &in)
		if in.WindowMinutes <= 0 {
			in.WindowMinutes = agentDefaultWindow
		}
		window := time.Duration(in.WindowMinutes) * time.Minute
		nearby, err := h.q.GetEventsAroundTime(ctx, event.EnvironmentID, event.Timestamp.Time, window, event.ID)
		if err != nil {
			return "", fmt.Errorf("could not list nearby events")
		}
		if len(nearby) == 0 {
			return "No nearby events.", nil
		}
		var sb strings.Builder
		for _, e := range nearby {
			writeEventLine(&sb, e)
		}
		return sb.String(), nil
	}
}

// eventForOrg parses {event_id} from input and loads it, enforcing org scoping.
func (h *AgentHandler) eventForOrg(ctx context.Context, input json.RawMessage, orgID pgtype.UUID) (store.Event, error) {
	var in struct {
		EventID string `json:"event_id"`
	}
	_ = json.Unmarshal(input, &in)
	id, err := uuid.Parse(in.EventID)
	if err != nil {
		return store.Event{}, fmt.Errorf("invalid event_id")
	}
	event, err := h.q.GetEventByIDForOrg(ctx, pgtype.UUID{Bytes: id, Valid: true}, orgID)
	if err != nil {
		return store.Event{}, fmt.Errorf("event not found")
	}
	return event, nil
}

func writeEventLine(sb *strings.Builder, e store.Event) {
	fmt.Fprintf(sb, "[%s] id=%s type=%s service=%s status=%s\n",
		e.Timestamp.Time.UTC().Format(time.RFC3339),
		uuidString(e.ID), e.Type, e.Service, e.Status,
	)
}

func uuidString(p pgtype.UUID) string {
	return uuid.UUID(p.Bytes).String()
}
