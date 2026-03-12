package openrouter

import (
	"encoding/json"
	"testing"

	"github.com/matst80/go-ollama-client/pkg/ai"
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
