package openrouter

import (
	"encoding/json"
	"testing"

	"github.com/matst80/go-ai-agent/pkg/ai"
)

func TestChatCompletionChunk_ToChatResponse(t *testing.T) {
	jsonData := `{
		"id":"gen-1773266927-FLII5M5qbe9XBWvuHqjb",
		"object":"chat.completion.chunk",
		"created":1773266927,
		"model":"stepfun/step-3.5-flash:free",
		"provider":"StepFun",
		"choices":[
			{
				"index":0,
				"delta":{
					"content":"",
					"role":"assistant",
					"reasoning":" commonly used.\n\nLet",
					"reasoning_details":[
						{
							"type":"reasoning.text",
							"text":" commonly used.\n\nLet",
							"format":"unknown",
							"index":0
						}
					]
				},
				"finish_reason":null,
				"native_finish_reason":null
			}
		]
	}`

	var chunk ChatCompletionChunk
	err := json.Unmarshal([]byte(jsonData), &chunk)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	resp := chunk.ToChatResponse()

	if resp.Model != "stepfun/step-3.5-flash:free" {
		t.Errorf("Expected model stepfun/step-3.5-flash:free, got %s", resp.Model)
	}

	if resp.Message.Role != ai.MessageRoleAssistant {
		t.Errorf("Expected role assistant, got %s", resp.Message.Role)
	}

	if resp.Message.ReasoningContent != " commonly used.\n\nLet" {
		t.Errorf("Expected reasoning content ' commonly used.\n\nLet', got %q", resp.Message.ReasoningContent)
	}
}

func TestChatCompletionChunk_ToolCalls(t *testing.T) {
	jsonData := `{
		"id":"gen-1773267450-nJPMeX6R9paUySZtzAlK",
		"object":"chat.completion.chunk",
		"created":1773267450,
		"model":"stepfun/step-3.5-flash:free",
		"provider":"StepFun",
		"choices":[
			{
				"index":0,
				"delta":{
					"content":null,
					"role":"assistant",
					"tool_calls":[
						{
							"index":0,
							"id":"call_e1b835795de04660a021d04d",
							"type":"function",
							"function":{"name":"run","arguments":""}
						}
					]
				},
				"finish_reason":null,
				"native_finish_reason":null
			}
		]
	}	`

	var chunk ChatCompletionChunk
	err := json.Unmarshal([]byte(jsonData), &chunk)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	resp := chunk.ToChatResponse()

	if len(resp.Message.ToolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(resp.Message.ToolCalls))
	}

	tc := resp.Message.ToolCalls[0]
	if tc.ID != "call_e1b835795de04660a021d04d" {
		t.Errorf("Expected tool call ID call_e1b835795de04660a021d04d, got %s", tc.ID)
	}
	if tc.Function.Name != "run" {
		t.Errorf("Expected function name run, got %s", tc.Function.Name)
	}
}

func TestChatCompletionChunk_ToolCallsPartial(t *testing.T) {
	jsonData := `{
		"id":"gen-1773267450-nJPMeX6R9paUySZtzAlK",
		"object":"chat.completion.chunk",
		"created":1773267450,
		"model":"stepfun/step-3.5-flash:free",
		"provider":"StepFun",
		"choices":[
			{
				"index":0,
				"delta":{
					"content":null,
					"role":"assistant",
					"tool_calls":[
						{
							"index":0,
							"function":{"arguments":"{\"command\": \"df -h\"}"}
						}
					]
				},
				"finish_reason":null,
				"native_finish_reason":null
			}
		]
	}`

	var chunk ChatCompletionChunk
	err := json.Unmarshal([]byte(jsonData), &chunk)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	resp := chunk.ToChatResponse()

	if len(resp.Message.ToolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(resp.Message.ToolCalls))
	}

	tc := resp.Message.ToolCalls[0]
	if tc.ID != "" {
		t.Errorf("Expected empty tool call ID, got %s", tc.ID)
	}
	if string(tc.Function.Arguments) != `{"command": "df -h"}` {
		t.Errorf("Expected arguments {\"command\": \"df -h\"}, got %s", string(tc.Function.Arguments))
	}
}

func TestChatCompletionChunk_ToolCallsRaw(t *testing.T) {
	jsonData := `{
		"id":"gen-1773267450-nJPMeX6R9paUySZtzAlK",
		"object":"chat.completion.chunk",
		"created":1773267450,
		"model":"stepfun/step-3.5-flash:free",
		"provider":"StepFun",
		"choices":[
			{
				"index":0,
				"delta":{
					"content":null,
					"role":"assistant",
					"tool_calls":[
						{
							"index":0,
							"function":{"arguments":"{\"command\": \"df -h\"}"}
						}
					]
				},
				"finish_reason":null,
				"native_finish_reason":null
			}
		]
	}`

	var chunk ChatCompletionChunk
	err := json.Unmarshal([]byte(jsonData), &chunk)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	resp := chunk.ToChatResponse()

	if len(resp.Message.ToolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(resp.Message.ToolCalls))
	}

	tc := resp.Message.ToolCalls[0]
	if tc.Index == nil || *tc.Index != 0 {
		t.Errorf("Expected index 0, got %v", tc.Index)
	}
	if string(tc.Function.Arguments) != `{"command": "df -h"}` {
		t.Errorf("Expected arguments {\"command\": \"df -h\"}, got %s", string(tc.Function.Arguments))
	}
}

func TestToOpenRouterChatRequest(t *testing.T) {
	t.Run("SimpleText", func(t *testing.T) {
		req := ai.ChatRequest{
			Messages: []ai.Message{
				{Role: ai.MessageRoleUser, Content: "Hello"},
			},
		}
		orReq := ToOpenRouterChatRequest(&req)
		if len(orReq.Messages) != 1 {
			t.Fatalf("Expected 1 message, got %d", len(orReq.Messages))
		}
		if orReq.Messages[0].Content != "Hello" {
			t.Errorf("Expected content 'Hello', got %v", orReq.Messages[0].Content)
		}
	})

	t.Run("WithImages", func(t *testing.T) {
		req := ai.ChatRequest{
			Messages: []ai.Message{
				{
					Role:    ai.MessageRoleUser,
					Content: "What is this?",
					Images:  []string{"data:image/png;base64,abc"},
				},
			},
		}
		orReq := ToOpenRouterChatRequest(&req)
		if len(orReq.Messages) != 1 {
			t.Fatalf("Expected 1 message, got %d", len(orReq.Messages))
		}

		parts, ok := orReq.Messages[0].Content.([]OpenRouterContentPart)
		if !ok {
			t.Fatalf("Expected content to be []OpenRouterContentPart, got %T", orReq.Messages[0].Content)
		}

		if len(parts) != 2 {
			t.Fatalf("Expected 2 parts, got %d", len(parts))
		}

		if parts[0].Type != "text" || parts[0].Text != "What is this?" {
			t.Errorf("First part mismatch: %+v", parts[0])
		}

		if parts[1].Type != "image_url" || parts[1].ImageURL.URL != "data:image/png;base64,abc" {
			t.Errorf("Second part mismatch: %+v", parts[1])
		}
	})

	t.Run("ImageOnly", func(t *testing.T) {
		req := ai.ChatRequest{
			Messages: []ai.Message{
				{
					Role:   ai.MessageRoleUser,
					Images: []string{"data:image/png;base64,abc"},
				},
			},
		}
		orReq := ToOpenRouterChatRequest(&req)
		if len(orReq.Messages) != 1 {
			t.Fatalf("Expected 1 message, got %d", len(orReq.Messages))
		}

		parts, ok := orReq.Messages[0].Content.([]OpenRouterContentPart)
		if !ok {
			t.Fatalf("Expected content to be []OpenRouterContentPart, got %T", orReq.Messages[0].Content)
		}

		if len(parts) != 1 {
			t.Fatalf("Expected 1 part, got %d", len(parts))
		}

		if parts[0].Type != "image_url" || parts[0].ImageURL.URL != "data:image/png;base64,abc" {
			t.Errorf("Part mismatch: %+v", parts[0])
		}
	})
}
