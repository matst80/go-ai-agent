package gemini

import (
	"encoding/json"

	"github.com/matst80/go-ai-agent/pkg/ai"
)

// GeminiRequest represents the request body for Gemini API
type GeminiRequest struct {
	Contents          []GeminiContent   `json:"contents"`
	SystemInstruction *GeminiContent    `json:"systemInstruction,omitempty"`
	GenerationConfig  *GenerationConfig `json:"generationConfig,omitempty"`
	Tools             []GeminiTool      `json:"tools,omitempty"`
}

type GeminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []GeminiPart `json:"parts"`
}

type GeminiPart struct {
	Text             string                  `json:"text,omitempty"`
	InlineData       *InlineData             `json:"inlineData,omitempty"`
	FunctionCall     *GeminiFunctionCall     `json:"functionCall,omitempty"`
	FunctionResponse *GeminiFunctionResponse `json:"functionResponse,omitempty"`
	ThoughtSignature string                  `json:"thoughtSignature,omitempty"`
}

type InlineData struct {
	MimeType string `json:"mime_type"`
	Data     string `json:"data"`
}

type GeminiFunctionCall struct {
	Id   string         `json:"id"`
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

type GeminiFunctionResponse struct {
	Name     string         `json:"name"`
	Response map[string]any `json:"response"`
}

type GenerationConfig struct {
	Temperature      *float64 `json:"temperature,omitempty"`
	TopP             *float64 `json:"topP,omitempty"`
	TopK             *int     `json:"topK,omitempty"`
	MaxOutputTokens  *int     `json:"maxOutputTokens,omitempty"`
	ResponseMimeType string   `json:"responseMimeType,omitempty"`
	StopSequences    []string `json:"stopSequences,omitempty"`
}

type GeminiTool struct {
	FunctionDeclarations []ai.Function `json:"functionDeclarations,omitempty"`
}

// GeminiResponse represents the response from Gemini API
type GeminiResponse struct {
	Candidates     []GeminiCandidate `json:"candidates"`
	UsageMetadata  *UsageMetadata    `json:"usageMetadata,omitempty"`
	PromptFeedback *PromptFeedback   `json:"promptFeedback,omitempty"`
}

type GeminiCandidate struct {
	Content       GeminiContent        `json:"content"`
	FinishReason  string               `json:"finishReason,omitempty"`
	Index         int                  `json:"index"`
	SafetyRatings []GeminiSafetyRating `json:"safetyRatings,omitempty"`
}

/*

{"candidates": [
{"content": {"parts": [{"functionCall": {"name": "run","args": {"command": "df -h /"},"id": "8qww9c6v"},"thoughtSignature": "EjQKMgG+Pvb7a+BCB+Y31VaoyKyrqwNLWNCEjl8bzr3GjnEO+CN/0LyOo3wgix+8iI0P9G7P"}],"role": "model"},"index": 0}],"usageMetadata": {"promptTokenCount": 57,"candidatesTokenCount": 17,"totalTokenCount": 74,"promptTokensDetails": [{"modality": "TEXT","tokenCount": 57}]},"modelVersion": "gemini-3.1-flash-lite-preview","responseId": "PACzadi9H6aQ_uMPhruWuA0"}

*/

type GeminiSafetyRating struct {
	Category    string `json:"category"`
	Probability string `json:"probability"`
}

type UsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

type PromptFeedback struct {
	SafetyRatings []GeminiSafetyRating `json:"safetyRatings,omitempty"`
}

// Helper to convert ai.ChatRequest to GeminiRequest
func ToGeminiRequest(req ai.ChatRequest) GeminiRequest {
	contents := make([]GeminiContent, 0)
	var systemInstr *GeminiContent

	for _, msg := range req.Messages {
		if msg.Role == ai.MessageRoleSystem {
			systemInstr = &GeminiContent{
				Parts: []GeminiPart{{Text: msg.Content}},
			}
			continue
		}

		role := string(msg.Role)
		if role == string(ai.MessageRoleAssistant) {
			role = "model"
		} else if role == string(ai.MessageRoleTool) {
			role = "user"
		}

		parts := make([]GeminiPart, 0)
		if msg.Content != "" {
			parts = append(parts, GeminiPart{Text: msg.Content})
		}

		for _, tc := range msg.ToolCalls {
			var args map[string]any
			json.Unmarshal(tc.Function.Arguments, &args)
			parts = append(parts, GeminiPart{
				FunctionCall: &GeminiFunctionCall{
					Id:   tc.ID,
					Name: tc.Function.Name,
					Args: args,
				},
				ThoughtSignature: tc.ThoughtSignature,
			})
		}

		if msg.Role == ai.MessageRoleTool {
			var response map[string]any
			if err := json.Unmarshal([]byte(msg.Content), &response); err != nil {
				response = map[string]any{"result": msg.Content}
			}
			parts = append(parts, GeminiPart{
				FunctionResponse: &GeminiFunctionResponse{
					Name:     msg.ToolCallID,
					Response: response,
				},
			})
		}

		contents = append(contents, GeminiContent{
			Role:  role,
			Parts: parts,
		})
	}

	var tools []GeminiTool
	if len(req.Tools) > 0 {
		funcs := make([]ai.Function, 0, len(req.Tools))
		for _, t := range req.Tools {
			funcs = append(funcs, t.Function)
		}
		tools = append(tools, GeminiTool{FunctionDeclarations: funcs})
	}

	config := &GenerationConfig{}
	if req.Format != nil {
		if *req.Format == ai.ResponseFormatJson {
			config.ResponseMimeType = "application/json"
		}
	}

	return GeminiRequest{
		Contents:          contents,
		SystemInstruction: systemInstr,
		Tools:             tools,
		GenerationConfig:  config,
	}
}

// Helper to convert GeminiResponse to ai.ChatResponse
func (gr *GeminiResponse) ToChatResponse() *ai.ChatResponse {
	if gr == nil || len(gr.Candidates) == 0 {
		return &ai.ChatResponse{
			BaseResponse: &ai.BaseResponse{
				Done: true,
			},
		}
	}

	cand := gr.Candidates[0]
	content := ""
	var toolCalls []ai.ToolCall

	for _, part := range cand.Content.Parts {
		if part.Text != "" {
			content += part.Text
		}
		if part.FunctionCall != nil {
			args, _ := json.Marshal(part.FunctionCall.Args)
			toolCalls = append(toolCalls, ai.ToolCall{
				ID: part.FunctionCall.Id,
				Function: ai.FunctionCall{
					Name:      part.FunctionCall.Name,
					Arguments: args,
				},
				ThoughtSignature: part.ThoughtSignature,
			})
		}
	}

	role := ai.MessageRoleAssistant
	if cand.Content.Role == "user" {
		role = ai.MessageRoleUser
	}

	return &ai.ChatResponse{
		BaseResponse: &ai.BaseResponse{
			Done: cand.FinishReason != "" && cand.FinishReason != "NONE",
		},
		Message: ai.Message{
			Role:      role,
			Content:   content,
			ToolCalls: toolCalls,
		},
	}
}
