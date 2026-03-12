package openai

import (
	"encoding/json"
	"time"

	"github.com/matst80/go-ai-agent/pkg/ai"
)

// ChatCompletionChunk represents an OpenRouter chat completion chunk
// based on the response format from models like StepFun or DeepSeek.
type ChatCompletionChunk struct {
	ID       string   `json:"id"`
	Object   string   `json:"object"`
	Created  int64    `json:"created"`
	Model    string   `json:"model"`
	Provider string   `json:"provider"`
	Choices  []Choice `json:"choices"`
}

// Choice represents a choice in the completion chunk
type Choice struct {
	Index              int     `json:"index"`
	Delta              Delta   `json:"delta"`
	FinishReason       *string `json:"finish_reason"`
	NativeFinishReason *string `json:"native_finish_reason"`
}

// Delta represents the delta content in the completion chunk
type Delta struct {
	Content          string            `json:"content"`
	Role             string            `json:"role"`
	Reasoning        string            `json:"reasoning,omitempty"`
	ReasoningDetails []ReasoningDetail `json:"reasoning_details,omitempty"`
	ToolCalls        []DeltaToolCall   `json:"tool_calls,omitempty"`
}

// DeltaToolCall represents a tool call in a delta
type DeltaToolCall struct {
	Index    int           `json:"index"`
	ID       string        `json:"id,omitempty"`
	Type     string        `json:"type,omitempty"`
	Function DeltaFunction `json:"function"`
}

// DeltaFunction represents a function call in a delta
type DeltaFunction struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

// ReasoningDetail represents the reasoning details in the delta
type ReasoningDetail struct {
	Type   string `json:"type"`
	Text   string `json:"text"`
	Format string `json:"format"`
	Index  int    `json:"index"`
}

// ToChatResponse converts a ChatCompletionChunk to an ai.ChatResponse.
// It maps OpenRouter specific fields like reasoning to the unified ai.Message structure.
func (c *ChatCompletionChunk) ToChatResponse() *ai.ChatResponse {
	if len(c.Choices) == 0 {
		return &ai.ChatResponse{
			BaseResponse: &ai.BaseResponse{
				Model:     c.Model,
				CreatedAt: time.Unix(c.Created, 0),
			},
		}
	}

	choice := c.Choices[0]

	// Map role, defaulting to assistant for delta chunks
	role := ai.MessageRoleAssistant
	if choice.Delta.Role != "" {
		role = ai.MessageRole(choice.Delta.Role)
	}

	msg := ai.Message{
		Role:             role,
		Content:          choice.Delta.Content,
		ReasoningContent: choice.Delta.Reasoning,
	}

	// If reasoning is empty but reasoning_details has content, accumulate it
	if msg.ReasoningContent == "" && len(choice.Delta.ReasoningDetails) > 0 {
		for _, detail := range choice.Delta.ReasoningDetails {
			msg.ReasoningContent += detail.Text
		}
	}

	// Handle ToolCalls
	if len(choice.Delta.ToolCalls) > 0 {
		for _, dtc := range choice.Delta.ToolCalls {
			idx := dtc.Index
			msg.ToolCalls = append(msg.ToolCalls, ai.ToolCall{
				Index: &idx,
				ID:    dtc.ID,
				Type:  dtc.Type,
				Function: ai.FunctionCall{
					Name:      dtc.Function.Name,
					Arguments: json.RawMessage(dtc.Function.Arguments),
				},
			})
		}
	}

	resp := &ai.ChatResponse{
		BaseResponse: &ai.BaseResponse{
			Model:     c.Model,
			CreatedAt: time.Unix(c.Created, 0),
		},
		Message: msg,
	}

	if choice.FinishReason != nil {
		resp.Done = true
		resp.DoneReason = *choice.FinishReason
	}

	return resp
}
