package github

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/matst80/go-ai-agent/pkg/ai"
)

func TestGitHubClient_Integration_Chat(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := NewGitHubClient()
	
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Try gpt-4o which is commonly available in GH Models / Copilot
	req := ai.NewChatRequest("gpt-4o")
	req.Messages = []ai.Message{
		{Role: ai.MessageRoleUser, Content: "Hello, what is the capital of France? Answer in one word."},
	}

	fmt.Println("Starting Chat request...")
	resp, err := client.Chat(ctx, *req)
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	if resp == nil {
		t.Fatal("expected response, got nil")
	}

	fmt.Printf("Response: %s\n", resp.Message.Content)
}

func TestGitHubClient_Integration_GetModels(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := NewGitHubClient()
	
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fmt.Println("Listing models...")
	models, err := client.GetModels(ctx)
	if err != nil {
		t.Fatalf("GetModels failed: %v", err)
	}

	fmt.Printf("Number of models found: %d\n", len(models))
	for _, m := range models {
		fmt.Printf("- %s: %s\n", m.ID, m.Name)
	}
}

func TestGitHubClient_Integration_ChatStreamed(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := NewGitHubClient()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req := ai.NewChatRequest("gpt-4o")
	req.Messages = []ai.Message{
		{Role: ai.MessageRoleUser, Content: "Say 'The streaming works!' and then stop."},
	}

	fmt.Println("Starting ChatStreamed request...")
	ch := make(chan *ai.ChatResponse, 100)

	// Since ChatStreamed closes the channel when it returns, we can just range over it
	errCh := make(chan error, 1)
	go func() {
		errCh <- client.ChatStreamed(ctx, *req, ch)
	}()

	var content string
	for resp := range ch {
		if resp.Message.Content != "" {
			fmt.Printf("Chunk: %q\n", resp.Message.Content)
			content += resp.Message.Content
		}
		if resp.Done {
			fmt.Println("Stream DONE marker received")
		}
	}

	if err := <-errCh; err != nil {
		t.Fatalf("ChatStreamed failed: %v", err)
	}

	fmt.Printf("Total Response: %s\n", content)
	if content == "" {
		t.Error("expected non-empty response content")
	}
}

func TestGitHubClient_Integration_SessionPersistence(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := NewGitHubClient()
	sessionID := "test-session-" + time.Now().Format("20060102150405")

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// 1. First message: "My name is Mats."
	req1 := ai.NewChatRequest("gpt-4o")
	req1.SessionID = sessionID
	req1.AddMessage(ai.MessageRoleUser, "My name is Mats.")

	fmt.Println("Sending first message...")
	_, err := client.Chat(ctx, *req1)
	if err != nil {
		t.Fatalf("Chat 1 failed: %v", err)
	}

	// 2. Second message: "What is my name?"
	req2 := ai.NewChatRequest("gpt-4o")
	req2.SessionID = sessionID
	req2.AddMessage(ai.MessageRoleUser, "What is my name?")

	fmt.Println("Sending second message...")
	resp, err := client.Chat(ctx, *req2)
	if err != nil {
		t.Fatalf("Chat 2 failed: %v", err)
	}

	fmt.Printf("Response: %s\n", resp.Message.Content)
	if !strings.Contains(strings.ToLower(resp.Message.Content), "mats") {
		t.Errorf("expected response to contain 'Mats', got %s", resp.Message.Content)
	}
}
