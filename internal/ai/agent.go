package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// agentModel is used for the multi-step tool loop where reasoning matters.
// Summaries/postmortems keep using the cheaper Haiku model in summarize.go.
const agentModel = "claude-sonnet-4-6"

// Tool describes a function the agent is allowed to call. InputSchema is a
// JSON Schema object describing the tool's parameters.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

// ToolHandler executes a tool call. It receives the raw JSON input the model
// produced and returns a string result that is fed back to the model. Returning
// an error surfaces it to the model as a tool error (so it can recover) rather
// than aborting the whole run.
type ToolHandler func(ctx context.Context, input json.RawMessage) (string, error)

// Turn is a single conversational turn supplied by the caller (the chat history).
type Turn struct {
	Role string // "user" or "assistant"
	Text string
}

// AgentStep records one tool invocation, returned to the client for transparency.
type AgentStep struct {
	Tool   string          `json:"tool"`
	Input  json.RawMessage `json:"input"`
	Result string          `json:"result"`
}

// AgentResult is the outcome of a RunAgent call.
type AgentResult struct {
	Answer string      `json:"answer"`
	Steps  []AgentStep `json:"steps"`
}

// StreamEvent is emitted incrementally during a RunAgent call so callers can
// stream progress to a client. Type is one of "token" (a chunk of the answer)
// or "step" (a tool was just executed).
type StreamEvent struct {
	Type  string
	Token string     // set when Type == "token"
	Step  *AgentStep // set when Type == "step"
}

// Emitter receives StreamEvents as the agent makes progress. It is called
// synchronously from RunAgent; pass nil to disable streaming.
type Emitter func(StreamEvent)

type cacheControl struct {
	Type string `json:"type"` // "ephemeral"
}

type systemBlock struct {
	Type         string        `json:"type"` // "text"
	Text         string        `json:"text"`
	CacheControl *cacheControl `json:"cache_control,omitempty"`
}

// contentBlock is a single block in a message. The same struct is used both for
// serialising requests and for decoding responses; omitempty keeps unused
// fields out of the wire format so blocks round-trip cleanly.
type contentBlock struct {
	Type string `json:"type"`

	// text block
	Text string `json:"text,omitempty"`

	// tool_use block (from the model)
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`

	// tool_result block (back to the model)
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   string `json:"content,omitempty"`
	IsError   bool   `json:"is_error,omitempty"`
}

type agentMessage struct {
	Role    string         `json:"role"`
	Content []contentBlock `json:"content"`
}

type agentRequest struct {
	Model     string         `json:"model"`
	MaxTokens int            `json:"max_tokens"`
	Stream    bool           `json:"stream,omitempty"`
	System    []systemBlock  `json:"system,omitempty"`
	Tools     []Tool         `json:"tools,omitempty"`
	Messages  []agentMessage `json:"messages"`
}

type agentResponse struct {
	Content    []contentBlock `json:"content"`
	StopReason string         `json:"stop_reason"`
	Error      *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// RunAgent runs an agentic tool-use loop against the Anthropic Messages API.
// It seeds the conversation with history, then repeatedly lets the model call
// tools (resolved via handlers) until it produces a final text answer or
// maxSteps is reached. The static system prompt + tool definitions are marked
// for prompt caching so repeated calls in a session are cheaper.
// As the model produces the answer and calls tools, it emits StreamEvents via
// emit (which may be nil). It still returns the full AgentResult at the end.
func RunAgent(ctx context.Context, system string, tools []Tool, handlers map[string]ToolHandler, history []Turn, maxSteps int, emit Emitter) (AgentResult, error) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return AgentResult{}, fmt.Errorf("ANTHROPIC_API_KEY not set")
	}

	messages := make([]agentMessage, 0, len(history)+2*maxSteps)
	for _, t := range history {
		role := t.Role
		if role != "assistant" {
			role = "user"
		}
		messages = append(messages, agentMessage{
			Role:    role,
			Content: []contentBlock{{Type: "text", Text: t.Text}},
		})
	}

	var steps []AgentStep

	for i := 0; i < maxSteps; i++ {
		resp, err := callMessagesStream(ctx, apiKey, agentRequest{
			Model:     agentModel,
			MaxTokens: 1024,
			System: []systemBlock{{
				Type:         "text",
				Text:         system,
				CacheControl: &cacheControl{Type: "ephemeral"},
			}},
			Tools:    tools,
			Messages: messages,
		}, func(text string) {
			if emit != nil {
				emit(StreamEvent{Type: "token", Token: text})
			}
		})
		if err != nil {
			return AgentResult{}, err
		}

		// Record the assistant turn so the model sees its own tool calls.
		messages = append(messages, agentMessage{Role: "assistant", Content: resp.Content})

		if resp.StopReason != "tool_use" {
			return AgentResult{Answer: collectText(resp.Content), Steps: steps}, nil
		}

		// Resolve every tool_use block in this turn.
		var results []contentBlock
		for _, block := range resp.Content {
			if block.Type != "tool_use" {
				continue
			}
			input := block.Input
			if len(input) == 0 {
				input = json.RawMessage("{}")
			}
			handler, ok := handlers[block.Name]
			var (
				out     string
				isError bool
			)
			if !ok {
				out, isError = fmt.Sprintf("unknown tool %q", block.Name), true
			} else if res, herr := handler(ctx, input); herr != nil {
				out, isError = herr.Error(), true
			} else {
				out = res
			}
			step := AgentStep{Tool: block.Name, Input: input, Result: out}
			steps = append(steps, step)
			if emit != nil {
				emit(StreamEvent{Type: "step", Step: &step})
			}
			results = append(results, contentBlock{
				Type:      "tool_result",
				ToolUseID: block.ID,
				Content:   out,
				IsError:   isError,
			})
		}
		messages = append(messages, agentMessage{Role: "user", Content: results})
	}

	return AgentResult{Answer: "I wasn't able to finish answering within the allowed number of steps. Try narrowing your question.", Steps: steps}, nil
}

func collectText(blocks []contentBlock) string {
	var buf bytes.Buffer
	for _, b := range blocks {
		if b.Type == "text" {
			buf.WriteString(b.Text)
		}
	}
	return buf.String()
}

// streamChunk is one server-sent event from the Anthropic streaming Messages
// API. Only the fields the agent loop needs are decoded.
type streamChunk struct {
	Type         string `json:"type"`
	Index        int    `json:"index"`
	ContentBlock struct {
		Type string `json:"type"`
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"content_block"`
	Delta struct {
		Type        string `json:"type"`
		Text        string `json:"text"`
		PartialJSON string `json:"partial_json"`
		StopReason  string `json:"stop_reason"`
	} `json:"delta"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// callMessagesStream calls the Anthropic Messages API with streaming enabled,
// invoking onText for each chunk of answer text as it arrives, and reassembles
// the full response (content blocks + stop reason) to return once the stream
// completes. tool_use input arrives as incremental JSON and is buffered per
// block, then attached when the block closes.
func callMessagesStream(ctx context.Context, apiKey string, reqBody agentRequest, onText func(string)) (agentResponse, error) {
	reqBody.Stream = true
	body, err := json.Marshal(reqBody)
	if err != nil {
		return agentResponse{}, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return agentResponse{}, err
	}
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")
	req.Header.Set("accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return agentResponse{}, fmt.Errorf("claude request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Errors come back as a normal JSON body, not a stream.
		var er agentResponse
		_ = json.NewDecoder(resp.Body).Decode(&er)
		if er.Error != nil {
			return agentResponse{}, fmt.Errorf("claude error: %s", er.Error.Message)
		}
		return agentResponse{}, fmt.Errorf("claude stream status %d", resp.StatusCode)
	}

	blocks := map[int]*contentBlock{}
	jsonBufs := map[int]*bytes.Buffer{}
	var stopReason string
	maxIdx := -1

	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(line[len("data:"):])
		if data == "" {
			continue
		}
		var ev streamChunk
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			continue
		}
		switch ev.Type {
		case "content_block_start":
			blocks[ev.Index] = &contentBlock{
				Type: ev.ContentBlock.Type,
				ID:   ev.ContentBlock.ID,
				Name: ev.ContentBlock.Name,
			}
			if ev.Index > maxIdx {
				maxIdx = ev.Index
			}
		case "content_block_delta":
			switch ev.Delta.Type {
			case "text_delta":
				if b := blocks[ev.Index]; b != nil {
					b.Text += ev.Delta.Text
				}
				if onText != nil && ev.Delta.Text != "" {
					onText(ev.Delta.Text)
				}
			case "input_json_delta":
				buf := jsonBufs[ev.Index]
				if buf == nil {
					buf = &bytes.Buffer{}
					jsonBufs[ev.Index] = buf
				}
				buf.WriteString(ev.Delta.PartialJSON)
			}
		case "content_block_stop":
			if buf := jsonBufs[ev.Index]; buf != nil && buf.Len() > 0 {
				if b := blocks[ev.Index]; b != nil {
					b.Input = json.RawMessage(buf.Bytes())
				}
			}
		case "message_delta":
			if ev.Delta.StopReason != "" {
				stopReason = ev.Delta.StopReason
			}
		case "error":
			if ev.Error != nil {
				return agentResponse{}, fmt.Errorf("claude stream error: %s", ev.Error.Message)
			}
			return agentResponse{}, fmt.Errorf("claude stream error")
		}
	}
	if err := sc.Err(); err != nil {
		return agentResponse{}, fmt.Errorf("claude stream read: %w", err)
	}

	content := make([]contentBlock, 0, maxIdx+1)
	for i := 0; i <= maxIdx; i++ {
		b := blocks[i]
		if b == nil {
			continue
		}
		switch b.Type {
		case "tool_use":
			// The API rejects a tool_use block with no input field; tools that
			// take no arguments produce an empty buffer, so default to {}.
			if len(b.Input) == 0 {
				b.Input = json.RawMessage("{}")
			}
		case "text":
			// Skip empty text blocks — the API rejects whitespace-only text and
			// they carry nothing to round-trip.
			if strings.TrimSpace(b.Text) == "" {
				continue
			}
		}
		content = append(content, *b)
	}
	return agentResponse{Content: content, StopReason: stopReason}, nil
}
