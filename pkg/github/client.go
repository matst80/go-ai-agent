package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/matst80/go-ai-agent/pkg/ai"
	"github.com/matst80/go-ai-agent/pkg/openai"
)

// DefaultURL is the base URL for GitHub Models Inference API
const DefaultURL = "https://models.github.ai"

// GitHubClient handles interaction with the GitHub Models Inference API
type GitHubClient struct {
	client     *ai.ApiClient
	apiVersion string
}

// NewGitHubClient creates a new GitHub client
func NewGitHubClient(apiKey string, apiVersion string) *GitHubClient {

	if apiVersion == "" {
		apiVersion = "2026-03-10"
	}

	headers := map[string]string{
		"Authorization":        fmt.Sprintf("Bearer %s", apiKey),
		"Accept":               "application/vnd.github+json",
		"X-GitHub-Api-Version": apiVersion,
		"Content-Type":         "application/json",
	}

	return &GitHubClient{
		client:     ai.NewApiClient(DefaultURL, headers),
		apiVersion: apiVersion,
	}
}

// WithLogFile sets the path to the log file for the underlying API client
func (c *GitHubClient) WithLogFile(path string) *GitHubClient {
	if c.client != nil {
		c.client.WithLogFile(path)
	}
	return c
}

// Chat handles a non-streaming request to GitHub Models
func (c *GitHubClient) Chat(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error) {
	req.Stream = false
	oaReq := openai.ToOpenAIChatRequest(&req)

	resp, err := c.client.PostJson(ctx, "inference/chat/completions", oaReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub request failed with status %d", resp.StatusCode)
	}

	var chatResp openai.ChatCompletion
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return chatResp.ToChatResponse(), nil
}

// ChatStreamed handles the streaming request to GitHub Models
func (c *GitHubClient) ChatStreamed(ctx context.Context, req ai.ChatRequest, ch chan *ai.ChatResponse) error {
	req.Stream = true
	oaReq := openai.ToOpenAIChatRequest(&req)
	defer close(ch)

	resp, err := c.client.PostJson(ctx, "inference/chat/completions", oaReq)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub request failed with status %d", resp.StatusCode)
	}

	var chunk openai.ChatCompletionChunk
	handler := ai.DataJsonChunkReader(&chunk, func(cjk *openai.ChatCompletionChunk) bool {
		// Use the mapping logic from openai package
		ch <- cjk.ToChatResponse()
		// Reset chunk for next unmarshal to avoid picking up old fields
		*cjk = openai.ChatCompletionChunk{}
		return false
	})

	if err := ai.ChunkReader(ctx, resp.Body, handler); err != nil {
		return err
	}

	return nil
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
	resp, err := c.client.GetJson(ctx, "catalog/models")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get models: status %d", resp.StatusCode)
	}

	var models []ModelInfo
	if err := json.NewDecoder(resp.Body).Decode(&models); err != nil {
		return nil, fmt.Errorf("failed to decode models: %w", err)
	}

	return models, nil
}
