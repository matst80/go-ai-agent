package ai

import (
	"context"
	"strings"
	"testing"
	"time"
)

type testHooks struct {
	DefaultSessionHooks
	onChatRequestCalled    bool
	onThinkingCalled       bool
	onContentCalled        bool
	onBeforeToolCallCalled bool
	onAfterToolCallCalled  bool
	onBlockCalled          bool
	onDoneCalled           bool
	onErrorCalled          bool

	thinkingDeltas []string
	contentDeltas  []string
	blocks         []string
}

func (h *testHooks) OnChatRequest(ctx context.Context, req *ChatRequest) error {
	h.onChatRequestCalled = true
	return nil
}

func (h *testHooks) OnThinking(ctx context.Context, thinking string) {
	h.onThinkingCalled = true
	h.thinkingDeltas = append(h.thinkingDeltas, thinking)
}

func (h *testHooks) OnContent(ctx context.Context, content string) {
	h.onContentCalled = true
	h.contentDeltas = append(h.contentDeltas, content)
}

func (h *testHooks) OnBeforeToolCall(ctx context.Context, toolCalls []ToolCall) error {
	h.onBeforeToolCallCalled = true
	return nil
}

func (h *testHooks) OnAfterToolCall(ctx context.Context, toolCalls []ToolCall, messages []Message, results []AutoToolResult) {
	h.onAfterToolCallCalled = true
}

func (h *testHooks) OnBlock(ctx context.Context, blockType string, content string) {
	h.onBlockCalled = true
	h.blocks = append(h.blocks, content)
}

func (h *testHooks) OnDone(ctx context.Context, res AccumulatedResponse) {
	h.onDoneCalled = true
}

func (h *testHooks) OnError(ctx context.Context, err error) {
	h.onErrorCalled = true
}

func TestAgentSession_Hooks(t *testing.T) {
	ctx := context.Background()
	req := NewChatRequest("test-model")

	mockClient := &MockChatClient{
		ChatStreamedFunc: func(ctx context.Context, req ChatRequest, ch chan *ChatResponse) error {
			// thinking
			ch <- &ChatResponse{
				BaseResponse: &BaseResponse{Done: false},
				Message:      Message{Role: MessageRoleAssistant, ReasoningContent: "Thinking... "},
			}
			ch <- &ChatResponse{
				BaseResponse: &BaseResponse{Done: false},
				Message:      Message{Role: MessageRoleAssistant, ReasoningContent: "done."},
			}
			// content
			ch <- &ChatResponse{
				BaseResponse: &BaseResponse{Done: false},
				Message:      Message{Role: MessageRoleAssistant, Content: "Hello "},
			}
			// block start
			ch <- &ChatResponse{
				BaseResponse: &BaseResponse{Done: false},
				Message:      Message{Role: MessageRoleAssistant, Content: "here is a block:\n```test\n"},
			}
			ch <- &ChatResponse{
				BaseResponse: &BaseResponse{Done: false},
				Message:      Message{Role: MessageRoleAssistant, Content: "line 1\n```\n"},
			}
			// final
			ch <- &ChatResponse{
				BaseResponse: &BaseResponse{Done: true},
				Message:      Message{Role: MessageRoleAssistant, Content: "bye!"},
			}
			close(ch)
			return nil
		},
	}

	hooks := &testHooks{}
	session := NewAgentSession(ctx, mockClient, req, NewDefaultAgentState(), WithHooks(hooks))
	defer session.Stop()

	err := session.SendUserMessage(ctx, "Hi")
	if err != nil {
		t.Fatalf("SendUserMessage failed: %v", err)
	}

	// Consume messages
	for res := range session.Recv() {
		if res.Chunk != nil && res.Chunk.Done {
			break
		}
	}

	// Wait for hooks to be called (they are called in the accumulation goroutine)
	time.Sleep(100 * time.Millisecond)

	if !hooks.onChatRequestCalled {
		t.Error("OnChatRequest was not called")
	}
	if !hooks.onThinkingCalled {
		t.Error("OnThinking was not called")
	}
	if !hooks.onContentCalled {
		t.Error("OnContent was not called")
	}
	if !hooks.onBlockCalled {
		t.Error("OnBlock was not called")
	}
	if !hooks.onDoneCalled {
		t.Error("OnDone was not called")
	}

	// Check deltas
	expectedThinking := "Thinking... done."
	actualThinking := strings.Join(hooks.thinkingDeltas, "")
	if actualThinking != expectedThinking {
		t.Errorf("expected thinking %q, got %q", expectedThinking, actualThinking)
	}

	expectedContentDeltaCount := 4 // "Hello ", "here is a diff:\n```diff\n", "+ line\n```\n", "bye!"
	if len(hooks.contentDeltas) != expectedContentDeltaCount {
		t.Errorf("expected %d content deltas, got %d: %q", expectedContentDeltaCount, len(hooks.contentDeltas), hooks.contentDeltas)
	}

	if len(hooks.blocks) != 1 {
		t.Errorf("expected 1 block, got %d", len(hooks.blocks))
	} else if !strings.Contains(hooks.blocks[0], "line 1") {
		t.Errorf("expected block to contain 'line 1', got %q", hooks.blocks[0])
	}
}

func TestAgentSession_Hooks_Error(t *testing.T) {
	ctx := context.Background()
	req := NewChatRequest("test-model")

	mockClient := &MockChatClient{
		ChatStreamedFunc: func(ctx context.Context, req ChatRequest, ch chan *ChatResponse) error {
			errStr := "api error"
			ch <- &ChatResponse{
				BaseResponse: &BaseResponse{Done: true, Error: &errStr},
			}
			close(ch)
			return nil
		},
	}

	hooks := &testHooks{}
	session := NewAgentSession(ctx, mockClient, req, NewDefaultAgentState(), WithHooks(hooks))
	defer session.Stop()

	_ = session.SendUserMessage(ctx, "Hi")

	// Consume messages
	for res := range session.Recv() {
		if res.Chunk != nil && res.Chunk.Done {
			break
		}
	}

	time.Sleep(50 * time.Millisecond)

	if !hooks.onErrorCalled {
		t.Error("OnError was not called")
	}
}
