package ai

import (
	"testing"
	"time"
)

func TestAgeTruncator(t *testing.T) {
	now := time.Now()
	messages := []Message{
		{Role: MessageRoleSystem, Content: "System", CreatedAt: now.Add(-10 * time.Hour)},
		{Role: MessageRoleUser, Content: "Old User", CreatedAt: now.Add(-5 * time.Hour)},
		{Role: MessageRoleAssistant, Content: "Old Assistant", CreatedAt: now.Add(-4 * time.Hour)},
		{Role: MessageRoleUser, Content: "Recent User", CreatedAt: now.Add(-1 * time.Hour)},
		{Role: MessageRoleAssistant, Content: "Latest", CreatedAt: now},
	}

	// MaxAge = 2 hours. Should remove "Old User" and "Old Assistant"
	// But should keep "System" and "Latest"
	truncator := NewAgeTruncator(2*time.Hour, 0, nil)
	result, removed := truncator.Apply(messages)

	if removed != 2 {
		t.Errorf("Expected 2 removed, got %d", removed)
	}

	if len(result) != 3 {
		t.Errorf("Expected 3 remaining, got %d", len(result))
	}

	// Verify specific messages preserved
	expected := []string{"System", "Recent User", "Latest"}
	for i, msg := range result {
		if msg.Content != expected[i] {
			t.Errorf("At index %d: expected %q, got %q", i, expected[i], msg.Content)
		}
	}
}
