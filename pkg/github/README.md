# GitHub Models Inference API Client

This package provides a client for the GitHub Models Inference API, following the OpenAI-compatible chat completion format but with GitHub-specific headers and endpoints.

## Usage

```go
client := github.NewGitHubClient("https://models.github.ai", os.Getenv("GITHUB_TOKEN"), "2026-03-10")
req := ai.NewChatRequest("openai/gpt-4o")
// ...
```

## Testing with Curl

### List Models

```bash
curl -L \
  -H "Accept: application/vnd.github+json" \
  -H "Authorization: Bearer $GITHUB_TOKEN" \
  -H "X-GitHub-Api-Version: 2026-03-10" \
  https://models.github.ai/catalog/models
```

### Chat Completion with Tool Calls

To see how tool calls are delivered in the stream:

```bash
curl -L \
  -X POST \
  -H "Accept: application/vnd.github+json" \
  -H "Authorization: Bearer $GITHUB_TOKEN" \
  -H "X-GitHub-Api-Version: 2026-03-10" \
  -H "Content-Type: application/json" \
  https://models.github.ai/inference/chat/completions \
  -d '{
    "model": "openai/gpt-4o",
    "messages": [
      {"role": "user", "content": "What is the weather in Berlin?"}
    ],
    "stream": true,
    "tools": [
      {
        "type": "function",
        "function": {
          "name": "get_weather",
          "description": "Get the current weather in a given location",
          "parameters": {
            "type": "object",
            "properties": {
              "location": {"type": "string"}
            },
            "required": ["location"]
          }
        }
      }
    ]
  }'
```

The response chunks for tool calls will look like:

```text
data: {"id":"...","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"...","type":"function","function":{"name":"get_weather","arguments":""}}]}}]}
data: {"id":"...","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"loca"}}]}}]}
...
```

The `github` package correctly accumulates these chunks into a single `ai.ToolCall` with the complete JSON arguments.
