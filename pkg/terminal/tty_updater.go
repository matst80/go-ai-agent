package terminal

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/matst80/go-ai-agent/pkg/ai"
)

// TTYUpdater handles rendering streaming AI output to the terminal by diffing
// with previous state and using ANSI escape codes for in-place updates.
// It accounts for line-wrapping based on terminal width to prevent duplicate lines.
type TTYUpdater struct {
	lastLines    []string
	out          io.Writer
	hideThinking bool
	width        int // manual override for testing or fixed-width terminals
}

// NewTTYUpdater creates a new TTYUpdater writing to os.Stdout.
func NewTTYUpdater() *TTYUpdater {
	return NewTTYUpdaterTo(os.Stdout)
}

// NewTTYUpdaterTo creates a new TTYUpdater writing to w.
func NewTTYUpdaterTo(w io.Writer) *TTYUpdater {
	return &TTYUpdater{out: w}
}

// WithHideThinking returns the updater configured to hide thinking once completed.
func (u *TTYUpdater) WithHideThinking(hide bool) *TTYUpdater {
	u.hideThinking = hide
	return u
}

// Handle processes an AccumulatedResponse and updates the terminal by printing
// chunk deltas. It returns true if the stream is finished for this turn.
func (u *TTYUpdater) Handle(res ai.AccumulatedResponse) bool {
	if res.Chunk != nil {
		if res.Chunk.Message.ReasoningContent != "" {
			fmt.Fprint(u.out, res.Chunk.Message.ReasoningContent)
		}
		if res.Chunk.Message.Content != "" {
			fmt.Fprint(u.out, res.Chunk.Message.Content)
		}

		// Handle synthetic chunks (like diff-reports) where Content is only in res.Content.
		// If Chunk has no message content but the summary Content is non-empty and it's the done chunk,
		// we check if it's the diff report and print it.
		if res.Chunk.Done && res.Chunk.Message.Content == "" && strings.Contains(res.Content, "[diff-report]") {
			// Find the report part and print it once.
			start := strings.Index(res.Content, "[diff-report]")
			if start != -1 {
				fmt.Fprint(u.out, res.Content[start:])
			}
		}
	}

	return u.IsFinished(res)
}

// Render updates the terminal with new content string.
func (u *TTYUpdater) Render(content string) {
	fmt.Fprint(u.out, content)
}

// IsFinished returns true if the response indicates the turn is complete.
func (u *TTYUpdater) IsFinished(res ai.AccumulatedResponse) bool {
	// Only truly finished if it's done AND no trailing tool calls exist.
	return res.Chunk != nil && res.Chunk.Done && len(res.ToolCalls) == 0
}
