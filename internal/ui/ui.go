package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Styles defines all the colors and styles used in the UI.
type Styles struct {
	App        lipgloss.Style
	Title      lipgloss.Style
	ModelName  lipgloss.Style
	Welcome    lipgloss.Style
	Help       lipgloss.Style
	UserMsg    lipgloss.Style
	Assistant  lipgloss.Style
	Error      lipgloss.Style
	ToolCall   lipgloss.Style
	ToolResult lipgloss.Style
	Border     lipgloss.Style
	Cursor     lipgloss.Style
	Input      lipgloss.Style
	Thinking   lipgloss.Style
	ToolName   lipgloss.Style
	Streaming  lipgloss.Style
}

// NewStyles creates styled components with a default theme.
func NewStyles() Styles {
	s := Styles{}

	s.App = lipgloss.NewStyle().Padding(1, 2)
	s.Title = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#00BAB4")).MarginTop(1)
	s.ModelName = lipgloss.NewStyle().Foreground(lipgloss.Color("#888")).Italic(true)
	s.Welcome = lipgloss.NewStyle().Foreground(lipgloss.Color("#888")).MarginTop(1).MarginBottom(1)
	s.Help = lipgloss.NewStyle().Foreground(lipgloss.Color("#666")).MarginTop(1)
	s.UserMsg = lipgloss.NewStyle().Width(60).Padding(0, 1).Background(lipgloss.Color("#2C3E50")).Foreground(lipgloss.Color("#ECF0F1"))
	s.Assistant = lipgloss.NewStyle().Width(60).Padding(0, 1)
	s.Error = lipgloss.NewStyle().Foreground(lipgloss.Color("#E74C3C")).Bold(true)
	s.ToolCall = lipgloss.NewStyle().Foreground(lipgloss.Color("#F39C12")).Padding(0, 1)
	s.ToolResult = lipgloss.NewStyle().Foreground(lipgloss.Color("#27AE60")).Padding(0, 1)
	s.Border = lipgloss.NewStyle().Border(lipgloss.ThickBorder()).BorderForeground(lipgloss.Color("#34495E"))
	s.Cursor = lipgloss.NewStyle().Foreground(lipgloss.Color("#00BAB4"))
	s.Input = lipgloss.NewStyle().Padding(0, 1)
	s.Thinking = lipgloss.NewStyle().Foreground(lipgloss.Color("#888")).Italic(true)
	s.ToolName = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#3498DB"))
	s.Streaming = lipgloss.NewStyle().Foreground(lipgloss.Color("#00BAB4")).Bold(true)

	return s
}

// Message represents a displayable message.
type Message struct {
	Role     string
	Content  string
	ToolName string
	Error    bool
}

// QuitMsg is a Bubbletea message to quit the application.
type QuitMsg struct{}

// UserInputMsg is a message containing user input.
type UserInputMsg string

// AssistantMsg is a message from the assistant.
type AssistantMsg struct{ Content string }

// ErrorMsg is an error message.
type ErrorMsg struct {
	Msg string
}
func (e ErrorMsg) Error() string { return e.Msg }

// ToolCallMsg is a message indicating a tool call.
type ToolCallMsg struct{ Name string }

// ToolResultMsg is a message containing a tool result.
type ToolResultMsg struct{ Content string }

// ResponseDoneMsg is a message indicating response completion.
type ResponseDoneMsg struct{}

// Model is the main Bubbletea model.
type Model struct {
	styles      Styles
	textInput   textinput.Model
	messages    []Message
	responding  bool
	errMsg      string
	quitting    bool
	width       int
	height      int
	currentTool string
	showHelp    bool
	modelName   string
	sendInput   chan<- string
}

// NewModel creates a new UI model.
func NewModel(modelName string, inputCh chan<- string) *Model {
	ti := textinput.New()
	ti.Placeholder = "Type a message..."
	ti.Focus()

	return &Model{
		styles:    NewStyles(),
		modelName: modelName,
		textInput: ti,
		sendInput: inputCh,
	}
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd {
	return func() tea.Msg {
		return nil
	}
}

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.textInput.Width = msg.Width - 4

	case tea.QuitMsg:
		m.quitting = true
		return m, tea.Quit

	case tea.KeyMsg:
		// Handle Enter for submit before delegating to textInput
		if msg.Type == tea.KeyEnter {
			if m.responding {
				return m, nil
			}

			input := m.textInput.Value()
			if input == "" {
				return m, nil
			}

			// Add user message
			m.messages = append(m.messages, Message{
				Role:    "user",
				Content: input,
			})

			// Clear input
			m.textInput.SetValue("")
			m.responding = true

			// Send to parent goroutine for API processing
			if m.sendInput != nil {
				m.sendInput <- input
			}
		}

		switch msg.Type {
		case tea.KeyCtrlC:
			m.quitting = true
			return m, tea.Quit

		case tea.KeyCtrlH:
			m.showHelp = !m.showHelp
			return m, nil
		}

	// Handle user input response
	case UserInputMsg:
		m.responding = false
		m.errMsg = ""
		return m, nil

	case ErrorMsg:
		m.responding = false
		m.errMsg = msg.Error()
		return m, nil

	case AssistantMsg:
		if len(m.messages) == 0 || m.messages[len(m.messages)-1].Role != "assistant" {
			m.messages = append(m.messages, Message{
				Role:    "assistant",
				Content: msg.Content,
			})
		} else {
			// Append to last assistant message
			m.messages[len(m.messages)-1].Content += msg.Content
		}
		return m, nil

	case ToolCallMsg:
		m.currentTool = msg.Name
		m.messages = append(m.messages, Message{
			Role:     "assistant",
			ToolName: msg.Name,
			Content:  "Calling tool...",
		})
		return m, nil

	case ToolResultMsg:
		m.messages = append(m.messages, Message{
			Role:    "assistant",
			Content: "Tool result: " + msg.Content,
		})
		m.currentTool = ""
		return m, nil

	case ResponseDoneMsg:
		m.responding = false
		return m, nil
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

// View implements tea.Model.
func (m *Model) View() string {
	if m.quitting {
		return ""
	}

	var buf strings.Builder

	// Title
	buf.WriteString(m.styles.Title.Render("✦ AI Chat"))
	buf.WriteString("\n")
	buf.WriteString(m.styles.ModelName.Render("Connected to "+m.modelName))
	buf.WriteString("\n\n")

	// Help hint
	buf.WriteString(m.styles.Help.Render("Press Enter/Shift+Enter to send | Ctrl+H for help | Ctrl+C to quit"))
	buf.WriteString("\n\n")

	// Divider
	buf.WriteString(strings.Repeat("─", max(min(m.width-2, 70), 1)))
	buf.WriteString("\n")

	// Messages
	for _, msg := range m.messages {
		buf.WriteString(m.renderMessage(msg))
		buf.WriteString("\n\n")
	}

	// Current streaming/tool status
	if m.currentTool != "" {
		buf.WriteString(m.styles.ToolCall.Render(fmt.Sprintf("⚡ Calling: %s", m.currentTool)))
		buf.WriteString("\n\n")
	}

	if m.responding && m.currentTool == "" {
		buf.WriteString(m.styles.Streaming.Render("●"))
		buf.WriteString("\n\n")
	}

	// Error message
	if m.errMsg != "" {
		buf.WriteString(m.styles.Error.Render("⚠ " + m.errMsg))
		buf.WriteString("\n\n")
	}

	// Divider
	buf.WriteString(strings.Repeat("─", max(min(m.width-2, 70), 1)))
	buf.WriteString("\n")

	// Input
	buf.WriteString(m.styles.Cursor.Render("❯ "))
	buf.WriteString(m.textInput.View())

	return buf.String()
}

func (m *Model) renderMessage(msg Message) string {
	switch msg.Role {
	case "user":
		return m.styles.UserMsg.Render(msg.Content)
	case "assistant":
		if msg.ToolName != "" {
			return m.styles.ToolCall.Render(
				fmt.Sprintf("🔧 %s: %s",
					m.styles.ToolName.Render(msg.ToolName),
					msg.Content,
				),
			)
		}
		return m.styles.Assistant.Render(msg.Content)
	default:
		return msg.Content
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
