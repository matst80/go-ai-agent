package ai

import (
	"context"
	"testing"
	"time"
)

type fakeChatClient struct {
	resp *ChatResponse
	err  error
}

func (f *fakeChatClient) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	return f.resp, f.err
}

func (f *fakeChatClient) ChatStreamed(ctx context.Context, req ChatRequest, ch chan *ChatResponse) error {
	close(ch)
	return nil
}

func TestNewSummarizeTruncatorDefaults(t *testing.T) {
	st := NewSummarizeTruncator(nil, nil)
	if st.Threshold != 50 {
		t.Fatalf("expected default threshold 50, got %d", st.Threshold)
	}
	if st.RemoveCount != 10 {
		t.Fatalf("expected default RemoveCount 10, got %d", st.RemoveCount)
	}
	if st.Timeout != 10*time.Second {
		t.Fatalf("expected default Timeout 10s, got %v", st.Timeout)
	}
	if st.TokenEstimateThreshold != 2000 {
		t.Fatalf("expected default TokenEstimateThreshold 2000, got %d", st.TokenEstimateThreshold)
	}
}

func TestSummarizeFallbackOnError(t *testing.T) {
	// Build messages with some non-system messages to be removed
	msgs := []Message{
		{Role: MessageRoleSystem, Content: "sys"},
		{Role: MessageRoleAssistant, Content: "a1"},
		{Role: MessageRoleAssistant, Content: "a2"},
		{Role: MessageRoleAssistant, Content: "a3"},
	}

	fake := &fakeChatClient{resp: nil, err: context.DeadlineExceeded}
	st := NewSummarizeTruncator(fake, &SummarizeOptions{Threshold: 1, RemoveCount: 2, Timeout: 10 * time.Millisecond})

	res, removed := st.Apply(msgs)
	if removed == 0 {
		t.Fatalf("expected some messages removed on fallback, got 0")
	}
	// ensure system message preserved
	if len(res) == 0 || res[0].Role != MessageRoleSystem {
		t.Fatalf("expected system message preserved after fallback")
	}
}
