package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// sse joins event lines into a single Anthropic-style SSE body. Each element is
// one "data:" payload; events are separated by a blank line.
func sse(payloads ...string) string {
	var b strings.Builder
	for _, p := range payloads {
		b.WriteString("data: ")
		b.WriteString(p)
		b.WriteString("\n\n")
	}
	return b.String()
}

func TestParseMessageStream_TextAndToolUse(t *testing.T) {
	stream := sse(
		`{"type":"message_start"}`,
		`{"type":"content_block_start","index":0,"content_block":{"type":"text"}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" world"}}`,
		`{"type":"content_block_stop","index":0}`,
		`{"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"tu_1","name":"list_events"}}`,
		`{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"limit\":"}}`,
		`{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"5}"}}`,
		`{"type":"content_block_stop","index":1}`,
		`{"type":"message_delta","delta":{"stop_reason":"tool_use"}}`,
		`{"type":"message_stop"}`,
	)

	var streamed string
	resp, err := parseMessageStream(strings.NewReader(stream), func(s string) { streamed += s })
	if err != nil {
		t.Fatalf("parseMessageStream: %v", err)
	}
	if streamed != "Hello world" {
		t.Errorf("onText got %q, want %q", streamed, "Hello world")
	}
	if resp.StopReason != "tool_use" {
		t.Errorf("stop reason = %q, want tool_use", resp.StopReason)
	}
	if len(resp.Content) != 2 {
		t.Fatalf("got %d content blocks, want 2", len(resp.Content))
	}
	if resp.Content[0].Type != "text" || resp.Content[0].Text != "Hello world" {
		t.Errorf("block 0 = %+v, want assembled text", resp.Content[0])
	}
	tu := resp.Content[1]
	if tu.Type != "tool_use" || tu.Name != "list_events" || tu.ID != "tu_1" {
		t.Errorf("block 1 = %+v, want tool_use list_events", tu)
	}
	if string(tu.Input) != `{"limit":5}` {
		t.Errorf("tool input = %q, want %q", tu.Input, `{"limit":5}`)
	}
}

func TestParseMessageStream_EmptyInputAndEmptyText(t *testing.T) {
	// A tool_use with no input deltas must default to "{}", and an empty text
	// block must be dropped — both required for the turn to round-trip to the API.
	stream := sse(
		`{"type":"content_block_start","index":0,"content_block":{"type":"text"}}`,
		`{"type":"content_block_stop","index":0}`,
		`{"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"tu_9","name":"list_environments"}}`,
		`{"type":"content_block_stop","index":1}`,
		`{"type":"message_delta","delta":{"stop_reason":"tool_use"}}`,
	)
	resp, err := parseMessageStream(strings.NewReader(stream), nil)
	if err != nil {
		t.Fatalf("parseMessageStream: %v", err)
	}
	if len(resp.Content) != 1 {
		t.Fatalf("got %d content blocks, want 1 (empty text dropped)", len(resp.Content))
	}
	if got := resp.Content[0]; got.Type != "tool_use" || string(got.Input) != "{}" {
		t.Errorf("block = %+v, want tool_use with input {}", got)
	}
}

func TestParseMessageStream_ErrorEvent(t *testing.T) {
	stream := sse(`{"type":"error","error":{"message":"overloaded"}}`)
	_, err := parseMessageStream(strings.NewReader(stream), nil)
	if err == nil || !strings.Contains(err.Error(), "overloaded") {
		t.Fatalf("err = %v, want one mentioning overloaded", err)
	}
}

// TestRunAgent_ToolLoop drives the full loop against a fake Messages API: the
// first turn calls a tool, the second returns the final answer. It asserts the
// tool ran, the answer is returned, and the right stream events were emitted.
func TestRunAgent_ToolLoop(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-key")

	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Content-Type", "text/event-stream")
		if calls == 1 {
			_, _ = w.Write([]byte(sse(
				`{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"tu_1","name":"list_events"}}`,
				`{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{}"}}`,
				`{"type":"content_block_stop","index":0}`,
				`{"type":"message_delta","delta":{"stop_reason":"tool_use"}}`,
			)))
			return
		}
		_, _ = w.Write([]byte(sse(
			`{"type":"content_block_start","index":0,"content_block":{"type":"text"}}`,
			`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"All good."}}`,
			`{"type":"content_block_stop","index":0}`,
			`{"type":"message_delta","delta":{"stop_reason":"end_turn"}}`,
		)))
	}))
	defer srv.Close()

	prev := messagesEndpoint
	messagesEndpoint = srv.URL
	defer func() { messagesEndpoint = prev }()

	toolRan := false
	tools := []Tool{{Name: "list_events", Description: "list", InputSchema: map[string]any{"type": "object"}}}
	handlers := map[string]ToolHandler{
		"list_events": func(_ context.Context, _ json.RawMessage) (string, error) {
			toolRan = true
			return "2 events", nil
		},
	}

	var steps, tokens int
	var answer string
	res, err := RunAgent(context.Background(), "system", tools, handlers,
		[]Turn{{Role: "user", Text: "status?"}}, 5, func(ev StreamEvent) {
			switch ev.Type {
			case "step":
				steps++
			case "token":
				tokens++
				answer += ev.Token
			}
		})
	if err != nil {
		t.Fatalf("RunAgent: %v", err)
	}
	if !toolRan {
		t.Error("tool handler was not invoked")
	}
	if res.Answer != "All good." {
		t.Errorf("answer = %q, want %q", res.Answer, "All good.")
	}
	if len(res.Steps) != 1 || res.Steps[0].Tool != "list_events" || res.Steps[0].Result != "2 events" {
		t.Errorf("steps = %+v, want one list_events step", res.Steps)
	}
	if steps != 1 {
		t.Errorf("emitted %d step events, want 1", steps)
	}
	if tokens == 0 || answer != "All good." {
		t.Errorf("token events: count=%d answer=%q", tokens, answer)
	}
	if calls != 2 {
		t.Errorf("made %d API calls, want 2", calls)
	}
}

func TestRunAgent_RequiresAPIKey(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	if _, err := RunAgent(context.Background(), "s", nil, nil, []Turn{{Role: "user", Text: "hi"}}, 3, nil); err == nil {
		t.Fatal("expected error when ANTHROPIC_API_KEY is unset")
	}
}
