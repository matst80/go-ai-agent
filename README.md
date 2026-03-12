# go-ollama-client

A unified Go client library for Ollama, Gemini, OpenRouter, and other AI providers. It provides a simple, consistent interface for chat, streaming, and tool execution (function calling).

## Features

- **Unified Interface**: Use the same code to interact with different AI providers.
- **Streaming Support**: Direct support for streaming responses with `ChatStreamed`.
- **Agent Sessions**: High-level `AgentSession` for managing message history and complex interactions.
- **Tool Calling**: Built-in registry and executor for handling model tool calls.
- **Multi-Provider**: Support for Ollama, Gemini, OpenRouter, X.ai, and OpenAI.

## Installation

```bash
go get github.com/matst80/go-ai-agent
```

## Basic Usage

### Simple Chat (Ollama)

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/matst80/go-ai-agent/pkg/ai"
	"github.com/matst80/go-ai-agent/pkg/ollama"
)

func main() {
	client := ollama.NewOllamaClient("http://localhost:11434")
	
	req := ai.NewChatRequest("qwen3.5:4b").
		AddMessage(ai.MessageRoleUser, "Why is the sky blue?")

	resp, err := client.Chat(context.Background(), *req)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(resp.Message.Content)
}
```

### Streaming Chat

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/matst80/go-ai-agent/pkg/ai"
	"github.com/matst80/go-ai-agent/pkg/ollama"
)

func main() {
	client := ollama.NewOllamaClient("http://localhost:11434")
	
	req := ai.NewChatRequest("llama3").
		WithStreaming(true).
		AddMessage(ai.MessageRoleUser, "Tell me a story.")

	ch := make(chan *ai.ChatResponse)
	
	go func() {
		err := client.ChatStreamed(context.Background(), *req, ch)
		if err != nil {
			log.Fatal(err)
		}
	}()

	for resp := range ch {
		fmt.Print(resp.Message.Content)
		if resp.Done {
			fmt.Println()
		}
	}
}
```

### Agent Session (High-level API)

The `AgentSession` simplifies handling history and tool execution results.

```go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/matst80/go-ai-agent/pkg/ai"
	"github.com/matst80/go-ai-agent/pkg/gemini"
)

func main() {
	ctx := context.Background()
	client := gemini.NewGeminiClient(os.Getenv("GEMINI_API_KEY"))
	
	req := ai.NewChatRequest("gemini-3.1-flash-lite-preview").WithStreaming(true)
	
	// Create a session that automatically accumulates history
	session := ai.NewAgentSession(ctx, client, *req, ai.WithAccumulator())
	defer session.Stop()

	// Send a message
	session.SendUserMessage(ctx, "Hello!")

	// Receive as a stream of accumulated responses
	for res := range session.Recv() {
		if res.Chunk.Done {
			fmt.Printf("\nFull Response: %s\n", res.Content)
			break
		}
		fmt.Print(res.Chunk.Message.Content)
	}
}
```

### Using Tools

```go
// Define a tool
type DiskArgs struct {
	Path string `json:"path" tool:"Path to check"`
}

func CheckDisk(args DiskArgs) string {
	return "50GB free"
}

// Register and use
registry := tools.NewRegistry()
registry.Register("check_disk", &DiskArgs{}, CheckDisk)

req := ai.NewChatRequest("model").WithTools(registry.GetTools())
```

## Supported Providers

- **Ollama**: `ollama.NewOllamaClient(url)`
- **Gemini**: `gemini.NewGeminiClient(apiKey)`
- **OpenRouter**: `openrouter.NewOpenRouterClient(url, apiKey)`
- **OpenAI**: `openai.NewOpenAIClient(url, apiKey)`
- **X.ai**: `xai.NewXAIClient(url, apiKey)`

## License

MIT
