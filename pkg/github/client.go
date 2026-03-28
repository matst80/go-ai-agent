package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/github/copilot-sdk/go"
	"github.com/matst80/go-ai-agent/pkg/ai"
)

// GitHubClient handles interaction with the GitHub Copilot CLI via copilot-sdk
type GitHubClient struct {
	client       *copilot.Client
	defaultModel string

	mu       sync.RWMutex
	sessions map[string]*copilot.Session
}

// NewGitHubClient creates a new GitHub client backed by Copilot SDK
func NewGitHubClient() *GitHubClient {
	c := copilot.NewClient(nil)
	if err := c.Start(context.Background()); err != nil {
		fmt.Printf("Warning: failed to start copilot client: %v\n", err)
	}
	return &GitHubClient{
		client:   c,
		sessions: make(map[string]*copilot.Session),
	}
}

// WithLogFile is retained for interface compatibility but ignored as we use copilot-sdk.
func (c *GitHubClient) WithLogFile(path string) *GitHubClient {
	return c
}

// WithDefaultModel sets the default model to use if no model is specified in a request.
func (c *GitHubClient) WithDefaultModel(model string) *GitHubClient {
	c.defaultModel = model
	return c
}

// getOrCreateSession retrieves an existing Copilot session or creates a new one.
func (c *GitHubClient) getOrCreateSession(ctx context.Context, sessionID string, model string, systemMessage string) (*copilot.Session, bool, error) {
	if model == "" {
		model = c.defaultModel
	}

	if sessionID != "" {
		c.mu.RLock()
		sess, ok := c.sessions[sessionID]
		c.mu.RUnlock()
		if ok {
			return sess, false, nil
		}

		c.mu.Lock()
		defer c.mu.Unlock()
		// Double-check locking
		if sess, ok := c.sessions[sessionID]; ok {
			return sess, false, nil
		}
	}

	config := &copilot.SessionConfig{
		Model:               model,
		Streaming:           true,
		OnPermissionRequest: copilot.PermissionHandler.ApproveAll,
	}

	if systemMessage != "" {
		config.SystemMessage = &copilot.SystemMessageConfig{
			Mode:    "replace",
			Content: systemMessage,
		}
	}

	sess, err := c.client.CreateSession(ctx, config)
	if err != nil {
		return nil, false, fmt.Errorf("failed to create copilot session: %w", err)
	}

	if sessionID != "" {
		c.sessions[sessionID] = sess
	}

	return sess, true, nil
}

// Chat handles a non-streaming request to GitHub Models
func (c *GitHubClient) Chat(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error) {
	var systemMessage string
	var messages []ai.Message
	if len(req.Messages) > 0 && req.Messages[0].Role == ai.MessageRoleSystem {
		systemMessage = req.Messages[0].Content
		messages = req.Messages[1:]
	} else {
		messages = req.Messages
	}

	sess, isNew, err := c.getOrCreateSession(ctx, req.SessionID, req.Model, systemMessage)
	if err != nil {
		return nil, err
	}

	if req.Model != "" {
		if err := sess.SetModel(ctx, req.Model); err != nil {
			return nil, fmt.Errorf("failed to set model: %w", err)
		}
	}

	prompt := c.formatPrompt(messages, isNew)

	event, err := sess.SendAndWait(ctx, copilot.MessageOptions{
		Prompt: prompt,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to send message: %w", err)
	}

	var content string
	var reasoning string
	var toolCalls []ai.ToolCall
	if event != nil {
		if event.Data.Content != nil {
			content = *event.Data.Content
		}
		if event.Data.ReasoningText != nil {
			reasoning = *event.Data.ReasoningText
		}
		if len(event.Data.ToolRequests) > 0 {
			toolCalls = c.parseToolCalls(event.Data.ToolRequests)
		}
	}

	return &ai.ChatResponse{
		BaseResponse: &ai.BaseResponse{
			Done: true,
		},
		Message: ai.Message{
			Role:             ai.MessageRoleAssistant,
			Content:          content,
			ReasoningContent: reasoning,
			ToolCalls:        toolCalls,
		},
	}, nil
}

// ChatStreamed handles the streaming request to GitHub Models
func (c *GitHubClient) ChatStreamed(ctx context.Context, req ai.ChatRequest, ch chan *ai.ChatResponse) error {
	defer close(ch)

	var systemMessage string
	var messages []ai.Message
	if len(req.Messages) > 0 && req.Messages[0].Role == ai.MessageRoleSystem {
		systemMessage = req.Messages[0].Content
		messages = req.Messages[1:]
	} else {
		messages = req.Messages
	}

	sess, isNew, err := c.getOrCreateSession(ctx, req.SessionID, req.Model, systemMessage)
	if err != nil {
		return err
	}

	if req.Model != "" {
		if err := sess.SetModel(ctx, req.Model); err != nil {
			return fmt.Errorf("failed to set model: %w", err)
		}
	}

	prompt := c.formatPrompt(messages, isNew)

	var currentMsgID string
	var msgIDMu sync.RWMutex
	var gotDeltas bool

	done := make(chan error, 1)
	chClosed := false
	var chMu sync.Mutex

	// Register event handler BEFORE sending so we don't miss early events
	unsubscribe := sess.On(func(event copilot.SessionEvent) {
		msgIDMu.RLock()
		mid := currentMsgID
		msgIDMu.RUnlock()

		// Filter events: if we have an interaction ID and we know our msgID, they must match.
		// If mid is empty, we allow it through (this handles events that arrive before Send returns).
		if event.Data.InteractionID != nil && mid != "" && *event.Data.InteractionID != mid {
			return
		}

		chMu.Lock()
		defer chMu.Unlock()
		if chClosed {
			return
		}

		switch event.Type {
		case copilot.SessionEventTypeAssistantMessageDelta:
			gotDeltas = true
			if event.Data.DeltaContent != nil {
				ch <- &ai.ChatResponse{
					BaseResponse: &ai.BaseResponse{Done: false},
					Message: ai.Message{
						Role:    ai.MessageRoleAssistant,
						Content: *event.Data.DeltaContent,
					},
				}
			}
		case copilot.SessionEventTypeAssistantReasoningDelta:
			if event.Data.DeltaContent != nil {
				ch <- &ai.ChatResponse{
					BaseResponse: &ai.BaseResponse{Done: false},
					Message: ai.Message{
						Role:             ai.MessageRoleAssistant,
						ReasoningContent: *event.Data.DeltaContent,
					},
				}
			}
		case copilot.SessionEventTypeAssistantIntent:
			if event.Data.Intent != nil {
				ch <- &ai.ChatResponse{
					BaseResponse: &ai.BaseResponse{Done: false},
					Message: ai.Message{
						Role:    ai.MessageRoleAssistant,
						Content: fmt.Sprintf("*%s...*\n", *event.Data.Intent),
					},
				}
			}
		case copilot.SessionEventTypeToolExecutionProgress:
			if event.Data.ProgressMessage != nil {
				ch <- &ai.ChatResponse{
					BaseResponse: &ai.BaseResponse{Done: false},
					Message: ai.Message{
						Role:    ai.MessageRoleAssistant,
						Content: fmt.Sprintf("  ↳ %s\n", *event.Data.ProgressMessage),
					},
				}
			}
		case copilot.SessionEventTypeToolExecutionStart:
			if name := safestr(event.Data.ToolName); name != "" {
				ch <- &ai.ChatResponse{
					BaseResponse: &ai.BaseResponse{Done: false},
					Message: ai.Message{
						Role:    ai.MessageRoleAssistant,
						Content: fmt.Sprintf("  🛠️ *Running tool*: %s\n", name),
					},
				}
			}
		case copilot.SessionEventTypeToolExecutionComplete:
			if name := safestr(event.Data.ToolName); name != "" {
				status := "success"
				if event.Data.Success != nil && !*event.Data.Success {
					status = "failed"
				}
				ch <- &ai.ChatResponse{
					BaseResponse: &ai.BaseResponse{Done: false},
					Message: ai.Message{
						Role:    ai.MessageRoleAssistant,
						Content: fmt.Sprintf("  ✅ *Tool %s finished*: %s\n", name, status),
					},
				}
			}
		case copilot.SessionEventTypeSessionWorkspaceFileChanged:
			if path := safestr(event.Data.Path); path != "" {
				op := "modified"
				if event.Data.Operation != nil {
					op = string(*event.Data.Operation)
				}
				ch <- &ai.ChatResponse{
					BaseResponse: &ai.BaseResponse{Done: false},
					Message: ai.Message{
						Role:    ai.MessageRoleAssistant,
						Content: fmt.Sprintf("  📄 *File %s*: %s\n", op, path),
					},
				}
			}
		case copilot.SessionEventTypeSubagentStarted:
			if name := safestr(event.Data.AgentDisplayName); name != "" {
				ch <- &ai.ChatResponse{
					BaseResponse: &ai.BaseResponse{Done: false},
					Message: ai.Message{
						Role:    ai.MessageRoleAssistant,
						Content: fmt.Sprintf("  🤖 *Sub-agent*: %s starting...\n", name),
					},
				}
			}
		case copilot.SessionEventTypeAssistantMessage:
			// Capture tool calls and potentially full message if we didn't get deltas
			resp := &ai.ChatResponse{
				BaseResponse: &ai.BaseResponse{Done: false},
				Message: ai.Message{
					Role: ai.MessageRoleAssistant,
				},
			}
			hasAnything := false
			if !gotDeltas && event.Data.Content != nil {
				resp.Message.Content = *event.Data.Content
				hasAnything = true
			}
			if len(event.Data.ToolRequests) > 0 {
				resp.Message.ToolCalls = c.parseToolCalls(event.Data.ToolRequests)
				hasAnything = true
			}
			if hasAnything {
				ch <- resp
			}
		case copilot.SessionEventTypeSessionError:
			var errMsg string
			if event.Data.ErrorReason != nil {
				errMsg = *event.Data.ErrorReason
			} else if event.Data.ErrorType != nil {
				errMsg = *event.Data.ErrorType
			} else {
				errMsg = "unknown assistant error"
			}
			select {
			case done <- fmt.Errorf("assistant error: %s", errMsg):
			default:
			}
		case copilot.SessionEventTypeSessionIdle:
			select {
			case done <- nil:
			default:
			}
		}
	})

	defer func() {
		unsubscribe()
		chMu.Lock()
		chClosed = true
		chMu.Unlock()
	}()

	msgID, err := sess.Send(ctx, copilot.MessageOptions{
		Prompt: prompt,
	})
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	msgIDMu.Lock()
	currentMsgID = msgID
	msgIDMu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		if err == nil {
			chMu.Lock()
			if !chClosed {
				ch <- &ai.ChatResponse{
					BaseResponse: &ai.BaseResponse{
						Done: true,
					},
				}
			}
			chMu.Unlock()
		}
		return err
	}
}

// ModelInfo represents information about a model from the GitHub catalog
type ModelInfo struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Publisher string   `json:"publisher"`
	Summary   string   `json:"summary,omitempty"`
	Tags      []string `json:"tags,omitempty"`
}

// GetModels returns the list of available models from the GitHub catalog
func (c *GitHubClient) GetModels(ctx context.Context) ([]ModelInfo, error) {
	sdkModels, err := c.client.ListModels(ctx)
	if err != nil {
		return nil, err
	}

	var models []ModelInfo
	for _, m := range sdkModels {
		models = append(models, ModelInfo{
			ID:      m.ID,
			Name:    m.Name,
			Summary: m.ID, // Fallback since Family does not exist
		})
	}

	return models, nil
}

// formatPrompt prepares the prompt for Sending.
// If the session is new, we prepend history to provide context.
// Otherwise, we only send the latest message.
func (c *GitHubClient) formatPrompt(messages []ai.Message, includeHistory bool) string {
	if len(messages) == 0 {
		return ""
	}

	if !includeHistory || len(messages) == 1 {
		return messages[len(messages)-1].Content
	}

	var sb strings.Builder
	sb.WriteString("Previous conversation history:\n\n")
	for i, m := range messages {
		if i == len(messages)-1 {
			sb.WriteString("\nCURRENT REQUEST:\n")
		}
		sb.WriteString(fmt.Sprintf("%s: %s\n\n", strings.ToUpper(string(m.Role)), m.Content))
	}
	return sb.String()
}

func (c *GitHubClient) parseToolCalls(requests []copilot.ToolRequest) []ai.ToolCall {
	var calls []ai.ToolCall
	for _, tr := range requests {
		call := ai.ToolCall{
			ID:   tr.ToolCallID,
			Type: "function",
			Function: ai.FunctionCall{
				Name:      tr.Name,
				Arguments: c.anyToRawJSON(tr.Arguments),
			},
		}
		calls = append(calls, call)
	}
	return calls
}

func (c *GitHubClient) anyToRawJSON(v any) json.RawMessage {
	if v == nil {
		return nil
	}
	if s, ok := v.(string); ok {
		return json.RawMessage(s)
	}
	b, _ := json.Marshal(v)
	return b
}

func safestr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// Verify interface compliance
var _ ai.ChatClientInterface = (*GitHubClient)(nil)
