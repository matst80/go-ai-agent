package api

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Client handles communication with OpenAI-compatible APIs.
type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

// New creates a new API client.
func New(baseURL, apiKey string) *Client {
	return &Client{
		BaseURL: baseURL,
		APIKey:  apiKey,
		HTTPClient: &http.Client{
			Timeout: 0, // no timeout, streaming can be long
		},
	}
}

// ChatMessage represents a message in the conversation.
type ChatMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

// FunctionCall represents a tool call from the model.
type FunctionCall struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// ToolCall represents a complete tool call.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// Tool represents a tool definition.
type Tool struct {
	Type     string   `json:"type"`
	Function ToolFunc `json:"function"`
}

// ToolFunc is the function definition for a tool.
type ToolFunc struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// ChatRequest is the request for chat completions.
type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Tools       []Tool        `json:"tools,omitempty"`
	Stream      bool          `json:"stream"`
	Temperature *float64      `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
}

// ChatResponseChunk is a streaming chunk from the API.
type ChatResponseChunk struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []Choice `json:"choices"`
	Done    bool   `json:"-"`
}

// Choice is a choice in the response.
type Choice struct {
	Index        int     `json:"index"`
	Delta        Delta   `json:"delta"`
	FinishReason string  `json:"finish_reason"`
	LogProbs     []float64 `json:"logprobs,omitempty"`
}

// Delta is the delta content in a streaming chunk.
type Delta struct {
	Role             string     `json:"role,omitempty"`
	Content          string     `json:"content,omitempty"`
	ReasoningContent string     `json:"reasoning,omitempty"`
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
}

// ChatResponse is the full non-streaming response.
type ChatResponse struct {
	ID      string   `json:"id"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   *Usage   `json:"usage,omitempty"`
}

// Usage contains token usage information.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Chat sends a non-streaming chat request.
func (c *Client) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	req.Stream = false
	url := fmt.Sprintf("%s/chat/completions", strings.TrimRight(c.BaseURL, "/"))

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.APIKey))

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &chatResp, nil
}

// ChatStream sends a streaming chat request and returns a channel to receive chunks.
func (c *Client) ChatStream(ctx context.Context, req ChatRequest) (<-chan *ChatResponseChunk, error) {
	req.Stream = true
	url := fmt.Sprintf("%s/chat/completions", strings.TrimRight(c.BaseURL, "/"))

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.APIKey))
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	ch := make(chan *ChatResponseChunk, 100)
	go func() {
		defer resp.Body.Close()
		defer close(ch)

		scanner := bufio.NewScanner(resp.Body)
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, 1024*1024)

		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				break
			}

			var chunk ChatResponseChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}

			// Mark as done if all choices have finish_reason
			if len(chunk.Choices) > 0 && chunk.Choices[0].FinishReason != "" {
				chunk.Done = true
			}

			ch <- &chunk
		}
	}()

	return ch, nil
}

// Accumulator holds the accumulated state during streaming.
type Accumulator struct {
	Role           string
	Content        string
	Reasoning      string
	ToolCalls      map[int]ToolCall
	ToolCallDeltas map[int]ToolCallDelta
}

// ToolCallDelta holds partial data for a tool call being streamed.
type ToolCallDelta struct {
	Name      string
	Arguments strings.Builder
}

// NewAccumulator creates a new Accumulator.
func NewAccumulator() *Accumulator {
	return &Accumulator{
		ToolCalls:      make(map[int]ToolCall),
		ToolCallDeltas: make(map[int]ToolCallDelta),
	}
}

// ProcessChunk processes a streaming chunk and updates the accumulator.
func (a *Accumulator) ProcessChunk(chunk *ChatResponseChunk) bool {
	if len(chunk.Choices) == 0 {
		return chunk.Done
	}

	for _, choice := range chunk.Choices {
		delta := choice.Delta

		// Accumulate role
		if delta.Role != "" {
			a.Role = delta.Role
		}

		// Accumulate content
		if delta.Content != "" {
			a.Content += delta.Content
		}

		// Accumulate reasoning
		if delta.ReasoningContent != "" {
			a.Reasoning += delta.ReasoningContent
		}

		// Accumulate tool calls
		for _, tc := range delta.ToolCalls {
			idx := 0
			if tc.ID != "" {
				a.ToolCalls[idx] = ToolCall{
					ID:   tc.ID,
					Type: tc.Type,
					Function: FunctionCall{
						Name: tc.Function.Name,
					},
				}
				a.ToolCallDeltas[idx] = ToolCallDelta{}
			}

			if d, ok := a.ToolCallDeltas[idx]; ok {
				if tc.Function.Name != "" {
					d.Name = tc.Function.Name
				}
				if len(tc.Function.Arguments) > 0 {
					d.Arguments.Write(tc.Function.Arguments)
				}
				a.ToolCallDeltas[idx] = d

				// Update the tool call
				if tc.ID != "" {
					a.ToolCalls[idx] = ToolCall{
						ID:   tc.ID,
						Type: "function",
						Function: FunctionCall{
							Name:      d.Name,
							Arguments: json.RawMessage(d.Arguments.String()),
						},
					}
				}
			}
		}
	}

	return len(chunk.Choices) > 0 && chunk.Choices[0].FinishReason != ""
}

// Finalize finalizes the accumulated tool calls.
func (a *Accumulator) Finalize() []ToolCall {
	var calls []ToolCall
	for idx, tc := range a.ToolCalls {
		if delta, ok := a.ToolCallDeltas[idx]; ok {
			tc.Function.Name = delta.Name
			tc.Function.Arguments = json.RawMessage(delta.Arguments.String())
		}
		calls = append(calls, tc)
	}
	return calls
}
