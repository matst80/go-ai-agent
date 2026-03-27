package ai

import (
	"testing"
	"time"
)

func TestMemoryTruncator(t *testing.T) {
	now := time.Now()
	messages := []Message{
		{Role: MessageRoleSystem, Content: "System", CreatedAt: now.Add(-10 * time.Hour)},
		{Role: MessageRoleAssistant, Content: "To Store", CreatedAt: now.Add(-5 * time.Hour)},
		{Role: MessageRoleAssistant, Content: "Latest", CreatedAt: now},
	}

	store := NewInMemoryMemoryStore()
	// Use MiddleTruncator that removes 1 message from middle
	inner := &MiddleTruncator{Threshold: 2, RemoveCount: 1}
	truncator := NewMemoryTruncator(inner, store, nil)

	result, removed := truncator.Apply(messages)

	if removed != 1 {
		t.Errorf("Expected 1 removed, got %d", removed)
	}

	if len(result) != 2 {
		t.Errorf("Expected 2 remaining, got %d", len(result))
	}

	// Verify it was stored
	stored, _ := store.RetrieveRelevant("", 10)
	if len(stored) != 1 {
		t.Errorf("Expected 1 stored, got %d", len(stored))
	} else if stored[0].Content != "To Store" {
		t.Errorf("Expected 'To Store' to be stored, got %q", stored[0].Content)
	}
}

func TestCompositeTruncator(t *testing.T) {
	now := time.Now()
	messages := []Message{
		{Role: MessageRoleSystem, Content: "System", CreatedAt: now.Add(-100 * time.Hour)},
		{Role: MessageRoleUser, Content: "Very Old", CreatedAt: now.Add(-50 * time.Hour)},
		{Role: MessageRoleAssistant, Content: "Middle 1", CreatedAt: now.Add(-5 * time.Hour)},
		{Role: MessageRoleAssistant, Content: "Middle 2", CreatedAt: now.Add(-4 * time.Hour)},
		{Role: MessageRoleAssistant, Content: "Latest", CreatedAt: now},
	}

	// Chain: AgeTruncator (MaxAge 10h) -> MiddleTruncator (Remove 1)
	truncator := NewCompositeTruncator(
		NewAgeTruncator(10*time.Hour, 0, nil),
		&MiddleTruncator{Threshold: 2, RemoveCount: 1},
	)

	result, removed := truncator.Apply(messages)

	// AgeTruncator removes "Very Old" (1)
	// Remaining: System, Middle 1, Middle 2, Latest (4)
	// MiddleTruncator removes 1 from middle (removable: Middle 1, Middle 2) (1)
	// Total removed: 2
	if removed != 2 {
		t.Errorf("Expected 2 removed, got %d", removed)
	}

	if len(result) != 3 {
		t.Errorf("Expected 3 remaining, got %d", len(result))
	}
}
