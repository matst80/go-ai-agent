package terminal

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/matst80/go-ai-agent/pkg/ai"
)

var ansiRegex = regexp.MustCompile(`\033\[[0-9;]*[mKJKH]`)

// LiveUpdater handles rendering streaming AI output to the terminal by diffing
// with previous state and using ANSI escape codes for in-place updates.
// It accounts for line-wrapping based on terminal width to prevent duplicate lines.
type LiveUpdater struct {
	lastLines    []string
	out          io.Writer
	hideThinking bool
	width        int // manual override for testing or fixed-width terminals
}

// NewLiveUpdater creates a new LiveUpdater writing to os.Stdout.
func NewLiveUpdater() *LiveUpdater {
	return NewLiveUpdaterTo(os.Stdout)
}

// NewLiveUpdaterTo creates a new LiveUpdater writing to w.
func NewLiveUpdaterTo(w io.Writer) *LiveUpdater {
	return &LiveUpdater{out: w}
}

// WithHideThinking returns the updater configured to hide thinking once completed.
func (u *LiveUpdater) WithHideThinking(hide bool) *LiveUpdater {
	u.hideThinking = hide
	return u
}

// Handle processes an AccumulatedResponse and updates the terminal.
// It returns true if the stream should be considered finished for this turn.
func (u *LiveUpdater) Handle(res ai.AccumulatedResponse) bool {
	finished := u.IsFinished(res)

	if res.Content != "" || res.ReasoningContent != "" {
		if finished && u.hideThinking {
			// Clear thinking by rendering only the final content.
			u.Render(res.Content)
		} else {
			u.Render(u.format(res.ReasoningContent, res.Content))
		}
	}

	return finished
}

func (u *LiveUpdater) format(reasoning, content string) string {
	if reasoning == "" {
		return content
	}

	var sb strings.Builder
	// \033[2m: Dim/Faint
	sb.WriteString("\033[2m")
	lines := strings.Split(strings.TrimRight(reasoning, "\n"), "\n")
	for _, l := range lines {
		sb.WriteString("┃ ")
		sb.WriteString(l)
		sb.WriteString("\n")
	}
	sb.WriteString("\033[0m") // Reset

	if content != "" {
		sb.WriteString(content)
	}
	return sb.String()
}

// WithWidth sets a manual terminal width override.
func (u *LiveUpdater) WithWidth(width int) *LiveUpdater {
	u.width = width
	return u
}

// Render updates the terminal with new content string, diffing against previous lines.
func (u *LiveUpdater) Render(content string) {
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")

	diffLine := 0
	for diffLine < len(lines) && diffLine < len(u.lastLines) && lines[diffLine] == u.lastLines[diffLine] {
		diffLine++
	}

	// If nothing changed, just return
	if diffLine == len(lines) && len(lines) == len(u.lastLines) {
		return
	}

	// Move cursor up to the first differing line and clear screen from there.
	if len(u.lastLines) > 0 {
		width := u.getTermWidth()
		moveUpRows := 0
		// Calculate how many physical terminal rows to move up based on previous content's wrap count.
		for i := diffLine; i < len(u.lastLines); i++ {
			moveUpRows += u.countPhysicalRows(u.lastLines[i], width)
		}

		if moveUpRows > 0 {
			// \033[%dA: move up N lines
			// \r: return to column 1
			// \033[J: clear from cursor to end of screen
			fmt.Fprintf(u.out, "\033[%dA\r\033[J", moveUpRows)
		}
	}

	for i := diffLine; i < len(lines); i++ {
		fmt.Fprintln(u.out, lines[i])
	}
	u.lastLines = lines
}

func (u *LiveUpdater) getTermWidth() int {
	if u.width > 0 {
		return u.width
	}

	// Try using 'tput cols' which is quite standard on Unix/Mac
	cmd := exec.Command("tput", "cols")
	cmd.Stdin = os.Stdin
	if out, err := cmd.Output(); err == nil {
		if w, err := strconv.Atoi(strings.TrimSpace(string(out))); err == nil {
			return w
		}
	}

	// Fallback to stty size
	cmd = exec.Command("stty", "size")
	cmd.Stdin = os.Stdin
	if out, err := cmd.Output(); err == nil {
		parts := strings.Fields(string(out))
		if len(parts) >= 2 {
			if w, err := strconv.Atoi(parts[1]); err == nil {
				return w
			}
		}
	}

	return 80 // final fallback
}

func (u *LiveUpdater) countPhysicalRows(line string, width int) int {
	// Strip ANSI codes as they don't consume visual width
	stripped := ansiRegex.ReplaceAllString(line, "")
	length := utf8.RuneCountInString(stripped)
	if length == 0 {
		return 1
	}
	// Logic: (len-1)/width + 1 is the number of rows a line occupies.
	return (length + width - 1) / width
}

// IsFinished returns true if the response indicates the turn is complete.
func (u *LiveUpdater) IsFinished(res ai.AccumulatedResponse) bool {
	// Only truly finished if it's done AND no trailing tool calls exist.
	return res.Chunk != nil && res.Chunk.Done && len(res.ToolCalls) == 0
}

// Reset clears the internal line buffer for a fresh response turn.
func (u *LiveUpdater) Reset() {
	u.lastLines = nil
}
