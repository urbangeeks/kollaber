package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
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
func RunAgent(ctx context.Context, system string, tools []Tool, handlers map[string]ToolHandler, history []Turn, maxSteps int) (AgentResult, error) {
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
		resp, err := callMessages(ctx, apiKey, agentRequest{
			Model:     agentModel,
			MaxTokens: 1024,
			System: []systemBlock{{
				Type:         "text",
				Text:         system,
				CacheControl: &cacheControl{Type: "ephemeral"},
			}},
			Tools:    tools,
			Messages: messages,
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
			handler, ok := handlers[block.Name]
			var (
				out     string
				isError bool
			)
			if !ok {
				out, isError = fmt.Sprintf("unknown tool %q", block.Name), true
			} else if res, herr := handler(ctx, block.Input); herr != nil {
				out, isError = herr.Error(), true
			} else {
				out = res
			}
			steps = append(steps, AgentStep{Tool: block.Name, Input: block.Input, Result: out})
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

func callMessages(ctx context.Context, apiKey string, reqBody agentRequest) (agentResponse, error) {
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

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return agentResponse{}, fmt.Errorf("claude request: %w", err)
	}
	defer resp.Body.Close()

	var result agentResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return agentResponse{}, fmt.Errorf("claude decode: %w", err)
	}
	if result.Error != nil {
		return agentResponse{}, fmt.Errorf("claude error: %s", result.Error.Message)
	}
	return result, nil
}
