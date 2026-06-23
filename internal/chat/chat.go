package chat

import (
	"encoding/json"
	"sync"
	"time"
)

// Message represents a chat message.
type Message struct {
	Role      string          `json:"role"`
	Content   interface{}     `json:"content"`
	ToolCalls []ToolCall      `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
	Timestamp  time.Time      `json:"timestamp"`
}

// ToolCall represents a tool call from the model.
type ToolCall struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Args     json.RawMessage `json:"args"`
	Result   string          `json:"result,omitempty"`
}

// Conversation holds a chat conversation with history.
type Conversation struct {
	mu       sync.RWMutex
	messages []*Message
	Model    string
}

// NewConversation creates a new conversation.
func NewConversation(model string, systemPrompt string) *Conversation {
	c := &Conversation{
		messages: make([]*Message, 0),
		Model:    model,
	}

	// Add system message if provided
	if systemPrompt != "" {
		c.AddSystemMessage(systemPrompt)
	}

	return c
}

// AddUserMessage adds a user message.
func (c *Conversation) AddUserMessage(content string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages = append(c.messages, &Message{
		Role:      "user",
		Content:   content,
		Timestamp: time.Now(),
	})
}

// AddAssistantMessage adds an assistant message.
func (c *Conversation) AddAssistantMessage(content string, toolCalls []ToolCall) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages = append(c.messages, &Message{
		Role:      "assistant",
		Content:   content,
		ToolCalls: toolCalls,
		Timestamp: time.Now(),
	})
}

// AddToolResult adds a tool result message.
func (c *Conversation) AddToolResult(toolCallID, result string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages = append(c.messages, &Message{
		Role:      "tool",
		Content:   result,
		ToolCallID: toolCallID,
		Timestamp: time.Now(),
	})
}

// AddSystemMessage adds a system message.
func (c *Conversation) AddSystemMessage(content string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages = append(c.messages, &Message{
		Role:      "system",
		Content:   content,
		Timestamp: time.Now(),
	})
}

// Messages returns all messages in the conversation.
func (c *Conversation) Messages() []*Message {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.messages
}

// GetMessages returns all messages in the conversation.
func (c *Conversation) GetMessages() []*Message {
	return c.Messages()
}

// GetAPIMessages converts messages to API format.
func (c *Conversation) GetAPIMessages() []map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	apiMessages := make([]map[string]interface{}, 0, len(c.messages))
	for _, msg := range c.messages {
		apiMsg := map[string]interface{}{
			"role":    msg.Role,
			"content": msg.Content,
		}

		if len(msg.ToolCalls) > 0 {
			apiMsg["tool_calls"] = msg.ToolCalls
		}
		if msg.ToolCallID != "" {
			apiMsg["tool_call_id"] = msg.ToolCallID
		}

		apiMessages = append(apiMessages, apiMsg)
	}

	return apiMessages
}

// Clear clears all messages.
func (c *Conversation) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages = c.messages[:0]
}

// Count returns the number of messages.
func (c *Conversation) Count() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.messages)
}
