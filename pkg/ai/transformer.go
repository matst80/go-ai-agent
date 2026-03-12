package ai

import (
	"context"
	"strings"
)

// mergeToolCalls merges incoming tool calls into the existing slice, updating matching entries by ID or Index.
func mergeToolCalls(existing []ToolCall, incoming []ToolCall) []ToolCall {
	if len(incoming) == 0 {
		return existing
	}

	for _, tc := range incoming {
		found := false
		for i, ex := range existing {
			// Match by ID if both have it, or by Index if available
			idMatch := ex.ID != "" && tc.ID != "" && ex.ID == tc.ID
			indexMatch := ex.Index != nil && tc.Index != nil && *ex.Index == *tc.Index

			if idMatch || indexMatch {
				// Update fields if they are provided in this chunk
				if tc.ID != "" {
					existing[i].ID = tc.ID
				}
				if tc.Function.Name != "" {
					existing[i].Function.Name = tc.Function.Name
				}
				if tc.Type != "" {
					existing[i].Type = tc.Type
				}

				// Accumulate arguments
				if len(tc.Function.Arguments) > 0 {
					if indexMatch {
						existing[i].Function.Arguments = append(existing[i].Function.Arguments, tc.Function.Arguments...)
					} else {
						existing[i].Function.Arguments = tc.Function.Arguments
					}
				}
				found = true
				break
			}
		}
		if !found {
			// Append new tool call when no match was found
			existing = append(existing, tc)
		}
	}

	return existing
}

// StreamAccumulator takes a receive-only channel of ChatResponse and returns a receive-only channel of AccumulatedResponse.
// It keeps track of the accumulated message (content, reasoning_content, and tool_calls).
func StreamAccumulator(ctx context.Context, input <-chan *ChatResponse, autoCloseMarkdown bool) <-chan *AccumulatedResponse {
	output := make(chan *AccumulatedResponse)

	go func() {
		defer close(output)

		var content strings.Builder
		var thinking strings.Builder
		toolCalls := make([]ToolCall, 0)

		for {
			select {
			case <-ctx.Done():
				return
			case chunk, ok := <-input:
				if !ok {
					return
				}

				// Accumulate content
				if chunk.Message.Content != "" {
					content.WriteString(chunk.Message.Content)
				}

				// Accumulate reasoning content
				if chunk.Message.ReasoningContent != "" {
					thinking.WriteString(chunk.Message.ReasoningContent)
				}

				// Accumulate tool calls via helper
				if len(chunk.Message.ToolCalls) > 0 {
					toolCalls = mergeToolCalls(toolCalls, chunk.Message.ToolCalls)
				}

				// Send the accumulated response
				// We create a copy to handle temporary markdown termination without affecting the actual accumulated content
				toSend := AccumulatedResponse{
					Chunk:            chunk,
					Content:          content.String(),
					ReasoningContent: thinking.String(),
					ToolCalls:        toolCalls,
				}

				if autoCloseMarkdown {
					if strings.Count(toSend.Content, "```")%2 != 0 {
						toSend.Content += "\n```"
					}
					if strings.Count(toSend.ReasoningContent, "```")%2 != 0 {
						toSend.ReasoningContent += "\n```"
					}
				}

				output <- &toSend
			}
		}
	}()

	return output
}
