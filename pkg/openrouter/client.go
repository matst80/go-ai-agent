package openrouter

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"os"

	"github.com/matst80/go-ollama-client/pkg/ai"
)

// OpenRouterClient handles interaction with the OpenRouter API
type OpenRouterClient struct {
	client  *ai.ApiClient
	logPath string
}

type OpenRouterEndpoint string

const (
	ChatEndpoint OpenRouterEndpoint = "api/v1/chat/completions"
)

// NewOpenRouterClient creates a new OpenRouter client
func NewOpenRouterClient(url string, apiKey string) *OpenRouterClient {
	return &OpenRouterClient{client: ai.NewApiClient(url, map[string]string{"Authorization": fmt.Sprintf("Bearer %s", apiKey)})}
}

// WithLogFile sets the path to the log file where all OpenRouter response lines will be stored
func (c *OpenRouterClient) WithLogFile(path string) *OpenRouterClient {
	c.logPath = path
	return c
}

// Chat handles a non-streaming request to OpenRouter and returns the full ChatResponse
func (c *OpenRouterClient) Chat(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error) {
	req.Stream = false

	resp, err := c.client.PostJson(ctx, string(ChatEndpoint), req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenRouter request failed with status %d", resp.StatusCode)
	}

	decoder := json.NewDecoder(resp.Body)
	var chatResp ai.ChatResponse
	if err := decoder.Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &chatResp, nil
}

var DATA_PREFIX = []byte("data: ")
var DONE = []byte("[DONE]")

// ChatStreamed handles the streaming request to OpenRouter and returns an error if the request or streaming fails.
func (c *OpenRouterClient) ChatStreamed(ctx context.Context, req ai.ChatRequest, ch chan *ai.ChatResponse) error {
	defer close(ch)

	resp, err := c.client.PostJson(ctx, string(ChatEndpoint), req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("OpenRouter request failed with status %d", resp.StatusCode)
	}

	reader := bufio.NewReader(resp.Body)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}

		if c.logPath != "" {
			if f, err := os.OpenFile(c.logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
				f.Write(line)
				f.Close()
			}
		}

		if bytes.Equal(line, DONE) {
			break
		}
		//log.Println(line)
		if !bytes.HasPrefix(line, DATA_PREFIX) {
			continue
		}
		line = line[len(DATA_PREFIX):]
		if len(line) == 0 {
			continue
		}
		//log.Printf("got: %s", cleanLine)

		var chatResp ChatCompletionChunk
		if err := json.Unmarshal(line, &chatResp); err != nil {
			continue
		}

		ch <- chatResp.ToChatResponse()

	}
	return nil
}
