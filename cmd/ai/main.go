package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/matst80/go-ai-agent/internal/api"
	"github.com/matst80/go-ai-agent/internal/chat"
	"github.com/matst80/go-ai-agent/internal/config"
	"github.com/matst80/go-ai-agent/internal/tools"
	"github.com/matst80/go-ai-agent/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Error loading config: %v\n", err)
	}

	client := api.New(cfg.API.BaseURL, cfg.API.APIKey)
	convo := chat.NewConversation(cfg.Model, cfg.SystemPrompt)
	reg := tools.NewRegistry()
	setupBuiltInTools(reg)

	inputCh := make(chan string, 10)
	assistantCh := make(chan string, 100)
	toolCallCh := make(chan string, 10)
	toolResultCh := make(chan string, 10)
	errorCh := make(chan error, 10)
	doneCh := make(chan struct{})

	model := ui.NewModel(cfg.Model, inputCh)
	p := tea.NewProgram(model, tea.WithAltScreen())

	go func() {
		if _, err := p.Run(); err != nil {
			log.Printf("TUI error: %v", err)
		}
		close(doneCh)
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-doneCh:
			return
		case <-sigChan:
			cancel()
			p.Quit()
			<-doneCh
			return
		case input := <-inputCh:
			if strings.HasPrefix(input, "/") {
				handleCommand(input, convo, reg, assistantCh, errorCh)
				continue
			}
			convo.AddUserMessage(input)
			go processResponse(ctx, client, convo, reg, assistantCh, toolCallCh, toolResultCh, errorCh, p)
		case content := <-assistantCh:
			p.Send(ui.AssistantMsg{Content: content})
		case name := <-toolCallCh:
			p.Send(ui.ToolCallMsg{Name: name})
		case content := <-toolResultCh:
			p.Send(ui.ToolResultMsg{Content: content})
		case err := <-errorCh:
			p.Send(ui.ErrorMsg{Msg: err.Error()})
		}
	}
}

func handleCommand(cmd string, convo *chat.Conversation, reg *tools.Registry, assistantCh chan<- string, errorCh chan<- error) {
	switch cmd {
	case "/clear":
		convo.Clear()
		assistantCh <- ""
	case "/tools":
		var sb strings.Builder
		for _, t := range reg.GetAll() {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", t.Name, t.Description))
		}
		assistantCh <- sb.String()
	case "/config":
		assistantCh <- "Configuration loaded successfully."
	case "/quit":
		os.Exit(0)
	case "/?", "/help":
		assistantCh <- "Available commands:\n  /clear - Clear conversation\n  /tools - List tools\n  /config - Show config\n  /quit - Exit\n  /? - Help"
	default:
		errorCh <- fmt.Errorf("unknown command: %s", cmd)
	}
}

func processResponse(ctx context.Context, client *api.Client, convo *chat.Conversation, reg *tools.Registry, assistantCh chan<- string, toolCallCh chan<- string, toolResultCh chan<- string, errorCh chan<- error, p *tea.Program) {
	apiMessages := make([]api.ChatMessage, len(convo.Messages()))
	for i, msg := range convo.Messages() {
		apiMessages[i] = api.ChatMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	var toolsList []api.Tool
	for _, t := range reg.GetAll() {
		toolsList = append(toolsList, api.Tool{
			Type: "function",
			Function: api.ToolFunc{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		})
	}

	req := api.ChatRequest{
		Model:    convo.Model,
		Messages: apiMessages,
		Tools:    toolsList,
	}

	acc := api.NewAccumulator()

	chunkCh, err := client.ChatStream(ctx, req)
	if err != nil {
		errorCh <- err
		return
	}

	var fullContent strings.Builder
	var toolCalls []api.ToolCall

	for chunk := range chunkCh {
		isDone := acc.ProcessChunk(chunk)

		if chunk.Choices != nil && len(chunk.Choices) > 0 {
			delta := chunk.Choices[0].Delta
			if delta.Content != "" {
				assistantCh <- delta.Content
				fullContent.WriteString(delta.Content)
			}
		}

		if isDone {
			break
		}
	}

	finalCalls := acc.Finalize()
	if len(finalCalls) > 0 {
		toolCalls = finalCalls

		convo.AddAssistantMessage(fullContent.String(), toChatToolCalls(finalCalls))

		for _, tc := range toolCalls {
			toolCallCh <- tc.Function.Name

			result, err := reg.Call(ctx, tc.Function.Name, tc.Function.Arguments)
			if err != nil {
				toolResultCh <- fmt.Sprintf("Error: %v", err)
				errorCh <- err
				continue
			}

			toolResultCh <- result.Content
			convo.AddToolResult(tc.ID, result.Content)
		}

		processResponse(ctx, client, convo, reg, assistantCh, toolCallCh, toolResultCh, errorCh, p)
		return
	}

	convo.AddAssistantMessage(fullContent.String(), nil)
	p.Send(ui.ResponseDoneMsg{})
}

func setupBuiltInTools(reg *tools.Registry) {
	reg.RegisterFunc(
		"help",
		"Show available commands",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"command": map[string]interface{}{
					"type":        "string",
					"description": "The command to get help for",
				},
			},
		},
		func(ctx context.Context, args json.RawMessage) (tools.Result, error) {
			help := "Available commands:\n"
			help += "  /clear  - Clear conversation history\n"
			help += "  /tools  - List available tools\n"
			help += "  /config - Show current configuration\n"
			help += "  /quit   - Exit the application\n"
			help += "  /?      - Show this help\n"
			return tools.Result{Content: help}, nil
		},
	)

	reg.RegisterFunc(
		"list_tools",
		"List all available tools",
		map[string]interface{}{
			"type": "object",
		},
		func(ctx context.Context, args json.RawMessage) (tools.Result, error) {
			var sb strings.Builder
			for _, t := range reg.GetAll() {
				sb.WriteString(fmt.Sprintf("- %s: %s\n", t.Name, t.Description))
			}
			return tools.Result{Content: sb.String()}, nil
		},
	)

	reg.RegisterFunc(
		"clear_history",
		"Clear the conversation history",
		map[string]interface{}{
			"type": "object",
		},
		func(ctx context.Context, args json.RawMessage) (tools.Result, error) {
			return tools.Result{Content: "Conversation history cleared."}, nil
		},
	)

	reg.RegisterFunc(
		"web_search",
		"Search the web for information",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "The search query",
				},
			},
			"required": []string{"query"},
		},
		func(ctx context.Context, args json.RawMessage) (tools.Result, error) {
			return tools.Result{Content: "Web search is not yet implemented."}, nil
		},
	)

	reg.RegisterFunc(
		"get_time",
		"Get the current date and time",
		map[string]interface{}{
			"type": "object",
		},
		func(ctx context.Context, args json.RawMessage) (tools.Result, error) {
			return tools.Result{Content: fmt.Sprintf("Current time: %s", time.Now())}, nil
		},
	)
}

func toChatToolCalls(calls []api.ToolCall) []chat.ToolCall {
	result := make([]chat.ToolCall, len(calls))
	for i, tc := range calls {
		result[i] = chat.ToolCall{
			ID:   tc.ID,
			Name: tc.Function.Name,
			Args: tc.Function.Arguments,
		}
	}
	return result
}
