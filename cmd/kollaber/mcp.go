package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
)

// The MCP server exposes the timeline to editor-side agents (Claude Code,
// Cursor, ...) over stdio. It reuses the CLI's saved token and API URL, so
// `kollaber login` is the only setup step — and inference runs on the user's
// own client subscription rather than the server's Anthropic key, which is why
// this is available on every plan while /ai/chat is gated to Team.

const mcpInstructions = `Kollaber is an infrastructure event timeline: deploys, alerts,
teardowns, and human notes for each environment, plus incidents grouping related events.

Use it to answer "what changed?" during debugging. A typical flow is
list_environments to resolve a name, get_timeline to see recent activity, then
find_related_events on a suspicious alert to see which deploys preceded it.

Timestamps are RFC3339. Environment arguments accept either a name ("production")
or a UUID.`

// --- shared response shapes ---

type mcpEvent struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	Service   string         `json:"service"`
	EnvID     string         `json:"environment_id"`
	Timestamp string         `json:"timestamp"`
	Status    string         `json:"status"`
	Metadata  map[string]any `json:"metadata"`
}

type mcpComment struct {
	ID        string `json:"id"`
	Body      string `json:"body"`
	UserEmail string `json:"user_email"`
	CreatedAt string `json:"created_at"`
}

// resolveEnv maps a name or UUID onto an environment id. Empty means "all
// environments", which the timeline endpoint expresses by omitting the filter.
func resolveEnv(nameOrID string) (string, error) {
	if strings.TrimSpace(nameOrID) == "" {
		return "", nil
	}
	env, err := findEnv(nameOrID)
	if err != nil {
		return "", err
	}
	return env.ID, nil
}

// getJSON issues an authenticated GET and decodes the result.
func getJSON(path string, dst any) error {
	res, err := do("GET", path, nil)
	if err != nil {
		return err
	}
	return decodeOK(res, dst)
}

// jsonResult renders a value as the tool's text content. Agents consume these
// results as text, so returning compact JSON keeps them parseable without
// committing to an output schema per tool.
func jsonResult(v any) (*mcp.CallToolResult, any, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, nil, err
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
	}, nil, nil
}

// toolError reports a failure to the model rather than to the transport. A
// transport-level error aborts the call; this lets the agent read what went
// wrong (a bad environment name, an expired token) and correct itself.
func toolError(format string, args ...any) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf(format, args...)}},
	}, nil, nil
}

// --- tool inputs ---

type listEnvironmentsInput struct{}

type getTimelineInput struct {
	Environment string `json:"environment,omitempty" jsonschema:"Environment name or UUID. Omit to search every environment."`
	Type        string `json:"type,omitempty" jsonschema:"Filter by event type: deploy, alert, note, teardown, rollback, or scale."`
	Service     string `json:"service,omitempty" jsonschema:"Filter by service name."`
	Status      string `json:"status,omitempty" jsonschema:"Filter by status: success, failure, or in_progress."`
	Since       string `json:"since,omitempty" jsonschema:"Only events after this RFC3339 timestamp."`
	Until       string `json:"until,omitempty" jsonschema:"Only events before this RFC3339 timestamp."`
	Limit       int    `json:"limit,omitempty" jsonschema:"Maximum events to return (default 50, max 200)."`
}

type getEventInput struct {
	EventID string `json:"event_id" jsonschema:"UUID of the event."`
}

type findRelatedEventsInput struct {
	EventID       string `json:"event_id" jsonschema:"UUID of the event to search around."`
	WindowMinutes int    `json:"window_minutes,omitempty" jsonschema:"How many minutes either side of the event to search (default 30)."`
}

type listIncidentsInput struct {
	Status string `json:"status,omitempty" jsonschema:"Filter by status: open, mitigated, or resolved."`
}

type getDORAMetricsInput struct {
	Environment string `json:"environment,omitempty" jsonschema:"Environment name or UUID. Omit for all environments."`
	Days        int    `json:"days,omitempty" jsonschema:"Window size in days (default 30)."`
}

type addNoteInput struct {
	Environment string `json:"environment" jsonschema:"Environment name or UUID to attach the note to."`
	Body        string `json:"body" jsonschema:"The note text."`
	Service     string `json:"service,omitempty" jsonschema:"Service the note concerns (default \"manual\")."`
}

type addCommentInput struct {
	EventID string `json:"event_id" jsonschema:"UUID of the event to comment on."`
	Body    string `json:"body" jsonschema:"The comment text."`
}

// --- tool handlers ---

func mcpListEnvironments(context.Context, *mcp.CallToolRequest, listEnvironmentsInput) (*mcp.CallToolResult, any, error) {
	var envs []environment
	if err := getJSON("/environments", &envs); err != nil {
		return toolError("could not list environments: %v", err)
	}
	return jsonResult(envs)
}

func mcpGetTimeline(_ context.Context, _ *mcp.CallToolRequest, in getTimelineInput) (*mcp.CallToolResult, any, error) {
	envID, err := resolveEnv(in.Environment)
	if err != nil {
		return toolError("%v", err)
	}

	q := url.Values{}
	if envID != "" {
		q.Set("environment_id", envID)
	}
	for key, val := range map[string]string{
		"type": in.Type, "service": in.Service, "status": in.Status,
		"after": in.Since, "before": in.Until,
	} {
		if val != "" {
			q.Set(key, val)
		}
	}
	if in.Limit <= 0 {
		in.Limit = 50
	}
	q.Set("limit", fmt.Sprint(in.Limit))

	var events []mcpEvent
	if err := getJSON("/events?"+q.Encode(), &events); err != nil {
		return toolError("could not fetch timeline: %v", err)
	}
	return jsonResult(events)
}

func mcpGetEvent(_ context.Context, _ *mcp.CallToolRequest, in getEventInput) (*mcp.CallToolResult, any, error) {
	if in.EventID == "" {
		return toolError("event_id is required")
	}

	var event mcpEvent
	if err := getJSON("/events/"+url.PathEscape(in.EventID), &event); err != nil {
		return toolError("could not fetch event: %v", err)
	}

	// Comments are where the team's reasoning lives, so an agent asking about
	// one event almost always wants them too — folding them in saves a round
	// trip and stops the discussion being missed entirely.
	var comments []mcpComment
	if err := getJSON("/events/"+url.PathEscape(in.EventID)+"/comments", &comments); err != nil {
		comments = nil
	}

	return jsonResult(struct {
		mcpEvent
		Comments []mcpComment `json:"comments"`
	}{event, comments})
}

func mcpFindRelatedEvents(_ context.Context, _ *mcp.CallToolRequest, in findRelatedEventsInput) (*mcp.CallToolResult, any, error) {
	if in.EventID == "" {
		return toolError("event_id is required")
	}
	if in.WindowMinutes <= 0 {
		in.WindowMinutes = 30
	}

	var anchor mcpEvent
	if err := getJSON("/events/"+url.PathEscape(in.EventID), &anchor); err != nil {
		return toolError("could not fetch event: %v", err)
	}

	ts, err := time.Parse(time.RFC3339, anchor.Timestamp)
	if err != nil {
		return toolError("event %s has an unparseable timestamp %q", in.EventID, anchor.Timestamp)
	}
	window := time.Duration(in.WindowMinutes) * time.Minute

	q := url.Values{}
	q.Set("environment_id", anchor.EnvID)
	q.Set("after", ts.Add(-window).Format(time.RFC3339))
	q.Set("before", ts.Add(window).Format(time.RFC3339))
	q.Set("limit", "100")

	var events []mcpEvent
	if err := getJSON("/events?"+q.Encode(), &events); err != nil {
		return toolError("could not fetch related events: %v", err)
	}

	// Split around the anchor: "what happened before this" is the question
	// being asked almost every time, and making the agent compare timestamps
	// itself is a step it can get wrong.
	//
	// The API serializes timestamps at second precision, so events sharing the
	// anchor's second cannot be ordered against it. They get their own bucket
	// rather than being folded into "before" or "after" — a deploy reported as
	// preceding an alert is read as its cause, and that is not a claim this
	// data can support.
	before := []mcpEvent{}
	after := []mcpEvent{}
	concurrent := []mcpEvent{}
	for _, e := range events {
		if e.ID == anchor.ID {
			continue
		}
		t, err := time.Parse(time.RFC3339, e.Timestamp)
		if err != nil {
			continue
		}
		switch {
		case t.Equal(ts):
			concurrent = append(concurrent, e)
		case t.Before(ts):
			before = append(before, e)
		default:
			after = append(after, e)
		}
	}

	out := map[string]any{
		"anchor":         anchor,
		"window_minutes": in.WindowMinutes,
		"before":         before,
		"after":          after,
	}
	if len(concurrent) > 0 {
		out["concurrent"] = concurrent
		out["note"] = "Events in \"concurrent\" share the anchor's timestamp to the second " +
			"and cannot be ordered against it; do not infer that one caused the other."
	}
	return jsonResult(out)
}

func mcpListIncidents(_ context.Context, _ *mcp.CallToolRequest, in listIncidentsInput) (*mcp.CallToolResult, any, error) {
	path := "/incidents"
	if in.Status != "" {
		path += "?status=" + url.QueryEscape(in.Status)
	}
	var incidents []incidentResp
	if err := getJSON(path, &incidents); err != nil {
		return toolError("could not list incidents: %v", err)
	}
	return jsonResult(incidents)
}

func mcpGetDORAMetrics(_ context.Context, _ *mcp.CallToolRequest, in getDORAMetricsInput) (*mcp.CallToolResult, any, error) {
	envID, err := resolveEnv(in.Environment)
	if err != nil {
		return toolError("%v", err)
	}
	if in.Days <= 0 {
		in.Days = 30
	}

	q := url.Values{}
	q.Set("days", fmt.Sprint(in.Days))
	if envID != "" {
		q.Set("environment_id", envID)
	}

	var metrics doraResp
	if err := getJSON("/metrics/dora?"+q.Encode(), &metrics); err != nil {
		return toolError("could not fetch DORA metrics: %v", err)
	}
	return jsonResult(metrics)
}

func mcpAddNote(_ context.Context, _ *mcp.CallToolRequest, in addNoteInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.Body) == "" {
		return toolError("body is required")
	}
	if strings.TrimSpace(in.Environment) == "" {
		return toolError("environment is required")
	}
	envID, err := resolveEnv(in.Environment)
	if err != nil {
		return toolError("%v", err)
	}
	service := in.Service
	if service == "" {
		service = "manual"
	}

	res, err := do("POST", "/events", map[string]any{
		"type":           "note",
		"service":        service,
		"environment_id": envID,
		"metadata":       map[string]string{"body": in.Body},
	})
	if err != nil {
		return toolError("could not create note: %v", err)
	}
	var created mcpEvent
	if err := decodeOK(res, &created); err != nil {
		return toolError("could not create note: %v", err)
	}
	return jsonResult(created)
}

func mcpAddComment(_ context.Context, _ *mcp.CallToolRequest, in addCommentInput) (*mcp.CallToolResult, any, error) {
	if in.EventID == "" {
		return toolError("event_id is required")
	}
	if strings.TrimSpace(in.Body) == "" {
		return toolError("body is required")
	}

	res, err := do("POST", "/events/"+url.PathEscape(in.EventID)+"/comments",
		map[string]string{"body": in.Body})
	if err != nil {
		return toolError("could not add comment: %v", err)
	}
	var created mcpComment
	if err := decodeOK(res, &created); err != nil {
		return toolError("could not add comment: %v", err)
	}
	return jsonResult(created)
}

// newMCPServer builds the server and registers every tool. Split out from the
// command so tests can drive it over an in-memory transport.
func newMCPServer() *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{
		Name:    "kollaber",
		Version: version,
	}, &mcp.ServerOptions{Instructions: mcpInstructions})

	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_environments",
		Description: "List the environments in your organization, with their ids and cluster names.",
		Annotations: readOnly,
	}, mcpListEnvironments)

	mcp.AddTool(s, &mcp.Tool{
		Name: "get_timeline",
		Description: "Fetch infrastructure events (deploys, alerts, notes, teardowns, rollbacks, scales) " +
			"for an environment, newest first. Use this to answer what changed and when.",
		Annotations: readOnly,
	}, mcpGetTimeline)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_event",
		Description: "Fetch one event by id, including its full metadata and the team's comment thread.",
		Annotations: readOnly,
	}, mcpGetEvent)

	mcp.AddTool(s, &mcp.Tool{
		Name: "find_related_events",
		Description: "Find events surrounding a given event in the same environment, split into those " +
			"before and after it. Use this on an alert to see which deploys preceded it.",
		Annotations: readOnly,
	}, mcpFindRelatedEvents)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_incidents",
		Description: "List incidents, optionally filtered by status (open, mitigated, resolved).",
		Annotations: readOnly,
	}, mcpListIncidents)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_dora_metrics",
		Description: "Fetch DORA metrics: deploy frequency, lead time, change failure rate, and time to restore.",
		Annotations: readOnly,
	}, mcpGetDORAMetrics)

	mcp.AddTool(s, &mcp.Tool{
		Name: "add_note",
		Description: "Add a note to an environment's timeline — what you are investigating, why you " +
			"rolled back, what to watch. Visible to the whole team.",
		Annotations: &mcp.ToolAnnotations{Title: "Add timeline note"},
	}, mcpAddNote)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "add_comment",
		Description: "Comment on an existing event, e.g. to record a root cause. Visible to the whole team.",
		Annotations: &mcp.ToolAnnotations{Title: "Comment on event"},
	}, mcpAddComment)

	return s
}

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Run an MCP server exposing the timeline to editor agents",
	Long: `Run a Model Context Protocol server over stdio.

Point an MCP client (Claude Code, Cursor, ...) at "kollaber mcp" and it can query
your timeline, correlate alerts with deploys, and leave notes — using the token
saved by "kollaber login".

Claude Code:
    claude mcp add kollaber -- kollaber mcp

Or add to your client's MCP config:
    {"mcpServers": {"kollaber": {"command": "kollaber", "args": ["mcp"]}}}`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if loadConfig().Token == "" {
			return fmt.Errorf("not logged in — run 'kollaber login' first")
		}
		// stdout is the JSON-RPC channel; anything written to it that is not a
		// protocol message corrupts the stream, so log to stderr.
		fmt.Fprintf(os.Stderr, "kollaber mcp: serving %s over stdio\n", apiURL())
		return newMCPServer().Run(cmd.Context(), &mcp.StdioTransport{})
	},
}
