package github

import (
	"context"
	"fmt"
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
func (c *GitHubClient) getOrCreateSession(ctx context.Context, sessionID string, model string) (*copilot.Session, error) {
	if model == "" {
		model = c.defaultModel
	}

	if sessionID != "" {
		c.mu.RLock()
		sess, ok := c.sessions[sessionID]
		c.mu.RUnlock()
		if ok {
			return sess, nil
		}

		c.mu.Lock()
		defer c.mu.Unlock()
		// Double-check locking
		if sess, ok := c.sessions[sessionID]; ok {
			return sess, nil
		}
	}

	sess, err := c.client.CreateSession(ctx, &copilot.SessionConfig{
		Model:               model,
		Streaming:           true,
		OnPermissionRequest: copilot.PermissionHandler.ApproveAll,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create copilot session: %w", err)
	}

	if sessionID != "" {
		c.sessions[sessionID] = sess
	}

	return sess, nil
}

// Chat handles a non-streaming request to GitHub Models
func (c *GitHubClient) Chat(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error) {
	sess, err := c.getOrCreateSession(ctx, req.SessionID, req.Model)
	if err != nil {
		return nil, err
	}

	if req.Model != "" && req.Model != c.defaultModel {
		if err := sess.SetModel(ctx, req.Model); err != nil {
			return nil, fmt.Errorf("failed to set model: %w", err)
		}
	}

	var latestMessage string
	if len(req.Messages) > 0 {
		latestMessage = req.Messages[len(req.Messages)-1].Content
	}

	event, err := sess.SendAndWait(ctx, copilot.MessageOptions{
		Prompt: latestMessage,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to send message: %w", err)
	}

	var content string
	if event != nil && event.Data.Content != nil {
		content = *event.Data.Content
	}

	return &ai.ChatResponse{
		BaseResponse: &ai.BaseResponse{
			Done: true,
		},
		Message: ai.Message{
			Role:    ai.MessageRoleAssistant,
			Content: content,
		},
	}, nil
}

// ChatStreamed handles the streaming request to GitHub Models
func (c *GitHubClient) ChatStreamed(ctx context.Context, req ai.ChatRequest, ch chan *ai.ChatResponse) error {
	defer close(ch)

	sess, err := c.getOrCreateSession(ctx, req.SessionID, req.Model)
	if err != nil {
		return err
	}

	if req.Model != "" && req.Model != c.defaultModel {
		if err := sess.SetModel(ctx, req.Model); err != nil {
			return fmt.Errorf("failed to set model: %w", err)
		}
	}

	var latestMessage string
	if len(req.Messages) > 0 {
		latestMessage = req.Messages[len(req.Messages)-1].Content
	}

	var currentMsgID string
	var msgIDMu sync.RWMutex

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
			if event.Data.DeltaContent != nil {
				ch <- &ai.ChatResponse{
					BaseResponse: &ai.BaseResponse{
						Done: false,
					},
					Message: ai.Message{
						Role:    ai.MessageRoleAssistant,
						Content: *event.Data.DeltaContent,
					},
				}
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
		case copilot.SessionEventTypeSessionIdle, copilot.SessionEventTypeAssistantTurnEnd:
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
		Prompt: latestMessage,
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

// Verify interface compliance
var _ ai.ChatClientInterface = (*GitHubClient)(nil)
