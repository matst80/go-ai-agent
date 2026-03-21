package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/matst80/go-ai-agent/pkg/ai"
)

// GeminiClient handles interaction with the Google Gemini API using REST
type GeminiClient struct {
	client *ai.ApiClient
}

// NewGeminiClient creates a new Gemini client
func NewGeminiClient(apiKey string) *GeminiClient {
	// Base URL for Gemini
	baseUrl := "https://generativelanguage.googleapis.com"
	return &GeminiClient{
		client: ai.NewApiClient(baseUrl, map[string]string{"x-goog-api-key": apiKey}),
	}
}

// Chat handles a non-streaming request to Gemini and returns the full ChatResponse
func (c *GeminiClient) Chat(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error) {
	geminiReq := ToGeminiRequest(req)

	endpoint := fmt.Sprintf("v1beta/models/%s:generateContent", req.Model)

	resp, err := c.client.PostJson(ctx, endpoint, geminiReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error struct {
				Message string `json:"message"`
				Status  string `json:"status"`
			} `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&errResp)
		if errResp.Error.Message != "" {
			return nil, fmt.Errorf("Gemini API error: %s (%s)", errResp.Error.Message, errResp.Error.Status)
		}
		return nil, fmt.Errorf("Gemini request failed with status %d", resp.StatusCode)
	}

	var geminiResp GeminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
		return nil, fmt.Errorf("failed to decode Gemini response: %w", err)
	}

	return geminiResp.ToChatResponse(), nil
}

// ChatStreamed handles the streaming request to Gemini
func (c *GeminiClient) ChatStreamed(ctx context.Context, req ai.ChatRequest, ch chan *ai.ChatResponse) error {
	defer close(ch)

	geminiReq := ToGeminiRequest(req)
	// Gemini streaming endpoint (using alt=sse for Server-Sent Events)
	endpoint := fmt.Sprintf("v1beta/models/%s:streamGenerateContent?alt=sse", req.Model)

	resp, err := c.client.PostJson(ctx, endpoint, geminiReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyText, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Gemini streaming request failed with status %d: %s", resp.StatusCode, bodyText)
	}

	handler := ai.DataJsonChunkReader(func(chunk *GeminiResponse) bool {
		ch <- chunk.ToChatResponse()
		return false
	})

	// Gemini SSE format sends "data: {...}" lines
	// The DataJsonChunkReader in pkg/ai/chunk_reader.go should handle this if it's set up correctly.
	if err := ai.ChunkReader(ctx, resp.Body, handler); err != nil {
		// If it's a context cancellation, we don't return error
		if ctx.Err() != nil {
			return nil
		}
		return err
	}

	return nil
}

// Verify interface compliance
var _ ai.ChatClientInterface = (*GeminiClient)(nil)
