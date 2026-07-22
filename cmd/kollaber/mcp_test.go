package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// fakeAPI stands in for a Kollaber instance. It records the paths it was asked
// for so tests can assert how a tool translated its arguments into requests —
// the query string is the part most likely to regress.
type fakeAPI struct {
	*httptest.Server
	mu    []string
	posts map[string]json.RawMessage
}

func newFakeAPI(t *testing.T) *fakeAPI {
	t.Helper()
	f := &fakeAPI{posts: map[string]json.RawMessage{}}

	mux := http.NewServeMux()
	record := func(r *http.Request) {
		p := r.URL.Path
		if r.URL.RawQuery != "" {
			p += "?" + r.URL.RawQuery
		}
		f.mu = append(f.mu, p)
	}

	mux.HandleFunc("/environments", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		writeJSON(w, []environment{
			{ID: "11111111-1111-1111-1111-111111111111", Name: "production", ClusterName: "prod-eks"},
			{ID: "22222222-2222-2222-2222-222222222222", Name: "staging", ClusterName: "stg-eks"},
		})
	})

	mux.HandleFunc("/events/", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		switch {
		case strings.HasSuffix(r.URL.Path, "/comments") && r.Method == http.MethodPost:
			body, _ := readBody(r)
			f.posts["comment"] = body
			writeJSON(w, mcpComment{ID: "c1", Body: "root cause: bad migration", UserEmail: "you@example.com"})
		case strings.HasSuffix(r.URL.Path, "/comments"):
			writeJSON(w, []mcpComment{{ID: "c1", Body: "rolling back", UserEmail: "sre@example.com"}})
		default:
			id := strings.TrimPrefix(r.URL.Path, "/events/")
			if id == "99999999-9999-9999-9999-999999999999" {
				w.WriteHeader(http.StatusNotFound)
				writeJSON(w, map[string]string{"error": "event not found"})
				return
			}
			writeJSON(w, mcpEvent{
				ID: id, Type: "alert", Service: "checkout",
				EnvID:     "11111111-1111-1111-1111-111111111111",
				Timestamp: "2026-07-22T10:30:00Z", Status: "failure",
				Metadata: map[string]any{"severity": "critical"},
			})
		}
	})

	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		if r.Method == http.MethodPost {
			body, _ := readBody(r)
			f.posts["event"] = body
			writeJSON(w, mcpEvent{ID: "new-event", Type: "note", Service: "manual"})
			return
		}
		// Two deploys before the 10:30 anchor, one alert after it, and one
		// sharing the anchor's second.
		writeJSON(w, []mcpEvent{
			{ID: "e-after", Type: "alert", Service: "checkout", Timestamp: "2026-07-22T10:41:00Z"},
			{ID: "e-tie", Type: "alert", Service: "payments", Timestamp: "2026-07-22T10:30:00Z"},
			{ID: "e-anchor", Type: "alert", Service: "checkout", Timestamp: "2026-07-22T10:30:00Z"},
			{ID: "e-before-1", Type: "deploy", Service: "checkout", Timestamp: "2026-07-22T10:22:00Z"},
			{ID: "e-before-2", Type: "deploy", Service: "api", Timestamp: "2026-07-22T10:05:00Z"},
		})
	})

	mux.HandleFunc("/incidents", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		writeJSON(w, []incidentResp{{ID: "i1", Title: "5xx spike", Severity: "sev2", Status: "open", EventCount: 3}})
	})

	mux.HandleFunc("/metrics/dora", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		writeJSON(w, doraResp{
			WindowDays:      30,
			DeployFrequency: doraMetricResp{Value: 3.2, Display: "3.2/day", Rating: "elite", Samples: 96},
		})
	})

	f.Server = httptest.NewServer(mux)
	t.Cleanup(f.Close)
	return f
}

func readBody(r *http.Request) (json.RawMessage, error) {
	var raw json.RawMessage
	err := json.NewDecoder(r.Body).Decode(&raw)
	return raw, err
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// requestedPath reports whether any recorded request path contains substr.
func (f *fakeAPI) requestedPath(substr string) bool {
	for _, p := range f.mu {
		if strings.Contains(p, substr) {
			return true
		}
	}
	return false
}

// newTestSession points the CLI's config at a fake API and connects an MCP
// client to the real server over an in-memory transport, exercising the actual
// protocol rather than calling handlers directly.
func newTestSession(t *testing.T, api *fakeAPI) *mcp.ClientSession {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".kollaber"), 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cfg, _ := json.Marshal(config{Token: "test-token", APIURL: api.URL})
	if err := os.WriteFile(filepath.Join(home, ".kollaber", "config.json"), cfg, 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("KOLLABER_API", api.URL)

	ctx := context.Background()
	clientT, serverT := mcp.NewInMemoryTransports()

	go func() {
		_ = newMCPServer().Run(ctx, serverT)
	}()

	session, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).
		Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

// callTool invokes a tool and returns its text content, failing the test if the
// tool reported an error.
func callTool(t *testing.T, s *mcp.ClientSession, name string, args map[string]any) string {
	t.Helper()
	res, err := s.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("CallTool(%s): %v", name, err)
	}
	text := toolText(res)
	if res.IsError {
		t.Fatalf("CallTool(%s) reported an error: %s", name, text)
	}
	return text
}

func toolText(res *mcp.CallToolResult) string {
	var sb strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			sb.WriteString(tc.Text)
		}
	}
	return sb.String()
}

func TestMCPListsAllTools(t *testing.T) {
	session := newTestSession(t, newFakeAPI(t))

	res, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	got := map[string]bool{}
	for _, tool := range res.Tools {
		got[tool.Name] = true
		if tool.Description == "" {
			t.Errorf("tool %q has no description; the model relies on it to choose", tool.Name)
		}
		if tool.InputSchema == nil {
			t.Errorf("tool %q has no input schema", tool.Name)
		}
	}

	for _, want := range []string{
		"list_environments", "get_timeline", "get_event", "find_related_events",
		"list_incidents", "get_dora_metrics", "add_note", "add_comment",
	} {
		if !got[want] {
			t.Errorf("tool %q not registered", want)
		}
	}
	if len(res.Tools) != 8 {
		t.Errorf("registered %d tools, want 8", len(res.Tools))
	}
}

func TestMCPReadToolsAreMarkedReadOnly(t *testing.T) {
	session := newTestSession(t, newFakeAPI(t))

	res, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	writers := map[string]bool{"add_note": true, "add_comment": true}
	for _, tool := range res.Tools {
		readOnly := tool.Annotations != nil && tool.Annotations.ReadOnlyHint
		if writers[tool.Name] && readOnly {
			t.Errorf("%q writes to the timeline but is annotated read-only", tool.Name)
		}
		if !writers[tool.Name] && !readOnly {
			t.Errorf("%q is read-only but not annotated as such; clients use this to skip confirmation", tool.Name)
		}
	}
}

func TestMCPListEnvironments(t *testing.T) {
	session := newTestSession(t, newFakeAPI(t))
	out := callTool(t, session, "list_environments", nil)

	if !strings.Contains(out, "production") || !strings.Contains(out, "prod-eks") {
		t.Errorf("output missing environment details:\n%s", out)
	}
}

func TestMCPGetTimelineResolvesEnvironmentName(t *testing.T) {
	api := newFakeAPI(t)
	session := newTestSession(t, api)

	callTool(t, session, "get_timeline", map[string]any{
		"environment": "production",
		"type":        "deploy",
		"limit":       10,
	})

	// The name must be resolved to its UUID before hitting /events.
	if !api.requestedPath("environment_id=11111111-1111-1111-1111-111111111111") {
		t.Errorf("environment name was not resolved to an id; requests: %v", api.mu)
	}
	if !api.requestedPath("type=deploy") {
		t.Errorf("type filter not forwarded; requests: %v", api.mu)
	}
	if !api.requestedPath("limit=10") {
		t.Errorf("limit not forwarded; requests: %v", api.mu)
	}
}

func TestMCPGetTimelineDefaultsLimit(t *testing.T) {
	api := newFakeAPI(t)
	session := newTestSession(t, api)

	callTool(t, session, "get_timeline", nil)

	if !api.requestedPath("limit=50") {
		t.Errorf("expected default limit=50; requests: %v", api.mu)
	}
	// With no environment given the filter must be omitted, not sent empty.
	if api.requestedPath("environment_id=") {
		t.Errorf("empty environment_id was sent; requests: %v", api.mu)
	}
}

func TestMCPGetTimelineRejectsUnknownEnvironment(t *testing.T) {
	session := newTestSession(t, newFakeAPI(t))

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "get_timeline",
		Arguments: map[string]any{"environment": "does-not-exist"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected a tool error for an unknown environment")
	}
	if !strings.Contains(toolText(res), "not found") {
		t.Errorf("error should name the problem, got: %s", toolText(res))
	}
}

func TestMCPGetEventIncludesComments(t *testing.T) {
	session := newTestSession(t, newFakeAPI(t))
	out := callTool(t, session, "get_event", map[string]any{
		"event_id": "33333333-3333-3333-3333-333333333333",
	})

	if !strings.Contains(out, "critical") {
		t.Errorf("event metadata missing:\n%s", out)
	}
	if !strings.Contains(out, "rolling back") {
		t.Errorf("comment thread was not folded into the result:\n%s", out)
	}
}

func TestMCPGetEventReportsNotFound(t *testing.T) {
	session := newTestSession(t, newFakeAPI(t))

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "get_event",
		Arguments: map[string]any{"event_id": "99999999-9999-9999-9999-999999999999"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected a tool error for a missing event")
	}
}

func TestMCPFindRelatedEventsSplitsAroundAnchor(t *testing.T) {
	api := newFakeAPI(t)
	session := newTestSession(t, api)

	out := callTool(t, session, "find_related_events", map[string]any{
		"event_id":       "e-anchor",
		"window_minutes": 20,
	})

	var got struct {
		WindowMinutes int        `json:"window_minutes"`
		Before        []mcpEvent `json:"before"`
		After         []mcpEvent `json:"after"`
		Concurrent    []mcpEvent `json:"concurrent"`
		Note          string     `json:"note"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal result: %v\n%s", err, out)
	}

	if got.WindowMinutes != 20 {
		t.Errorf("window_minutes = %d, want 20", got.WindowMinutes)
	}
	if len(got.Before) != 2 {
		t.Errorf("len(before) = %d, want 2 (the two deploys):\n%s", len(got.Before), out)
	}
	if len(got.After) != 1 {
		t.Errorf("len(after) = %d, want 1:\n%s", len(got.After), out)
	}

	// An event sharing the anchor's second is unorderable against it, so it
	// must not be reported as preceding or following the anchor.
	if len(got.Concurrent) != 1 || got.Concurrent[0].ID != "e-tie" {
		t.Errorf("same-second event was not isolated into concurrent:\n%s", out)
	}
	if got.Note == "" {
		t.Error("concurrent events present but no note warning against inferring causality")
	}

	// The anchor itself must not appear in any bucket.
	all := append(append(append([]mcpEvent{}, got.Before...), got.After...), got.Concurrent...)
	for _, e := range all {
		if e.ID == "e-anchor" {
			t.Error("anchor event leaked into its own related list")
		}
	}
	// The window must be translated into before/after bounds on the query.
	if !api.requestedPath("after=2026-07-22T10%3A10%3A00Z") {
		t.Errorf("window lower bound not applied; requests: %v", api.mu)
	}
	if !api.requestedPath("before=2026-07-22T10%3A50%3A00Z") {
		t.Errorf("window upper bound not applied; requests: %v", api.mu)
	}
}

func TestMCPFindRelatedEventsDefaultsWindow(t *testing.T) {
	api := newFakeAPI(t)
	session := newTestSession(t, api)

	callTool(t, session, "find_related_events", map[string]any{"event_id": "e-anchor"})

	// 30 minutes either side of the 10:30 anchor.
	if !api.requestedPath("after=2026-07-22T10%3A00%3A00Z") {
		t.Errorf("default window not applied; requests: %v", api.mu)
	}
}

func TestMCPGetDORAMetrics(t *testing.T) {
	api := newFakeAPI(t)
	session := newTestSession(t, api)

	out := callTool(t, session, "get_dora_metrics", map[string]any{
		"environment": "staging",
		"days":        7,
	})

	if !strings.Contains(out, "elite") {
		t.Errorf("metrics missing from output:\n%s", out)
	}
	if !api.requestedPath("days=7") {
		t.Errorf("days not forwarded; requests: %v", api.mu)
	}
	if !api.requestedPath("environment_id=22222222-2222-2222-2222-222222222222") {
		t.Errorf("environment not resolved; requests: %v", api.mu)
	}
}

func TestMCPAddNote(t *testing.T) {
	api := newFakeAPI(t)
	session := newTestSession(t, api)

	callTool(t, session, "add_note", map[string]any{
		"environment": "production",
		"body":        "investigating latency spike",
	})

	var sent struct {
		Type     string            `json:"type"`
		Service  string            `json:"service"`
		EnvID    string            `json:"environment_id"`
		Metadata map[string]string `json:"metadata"`
	}
	if err := json.Unmarshal(api.posts["event"], &sent); err != nil {
		t.Fatalf("no event was posted: %v", err)
	}
	if sent.Type != "note" {
		t.Errorf("type = %q, want note", sent.Type)
	}
	if sent.Service != "manual" {
		t.Errorf("service = %q, want the default \"manual\"", sent.Service)
	}
	if sent.EnvID != "11111111-1111-1111-1111-111111111111" {
		t.Errorf("environment_id = %q, not resolved from the name", sent.EnvID)
	}
	if sent.Metadata["body"] != "investigating latency spike" {
		t.Errorf("body = %q", sent.Metadata["body"])
	}
}

func TestMCPAddNoteRequiresBody(t *testing.T) {
	session := newTestSession(t, newFakeAPI(t))

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "add_note",
		Arguments: map[string]any{"environment": "production", "body": "   "},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected a tool error for a blank body")
	}
}

func TestMCPAddComment(t *testing.T) {
	api := newFakeAPI(t)
	session := newTestSession(t, api)

	callTool(t, session, "add_comment", map[string]any{
		"event_id": "33333333-3333-3333-3333-333333333333",
		"body":     "root cause: bad migration",
	})

	var sent struct {
		Body string `json:"body"`
	}
	if err := json.Unmarshal(api.posts["comment"], &sent); err != nil {
		t.Fatalf("no comment was posted: %v", err)
	}
	if sent.Body != "root cause: bad migration" {
		t.Errorf("body = %q", sent.Body)
	}
}
