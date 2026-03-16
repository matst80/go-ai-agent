package ai

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Integration test: feed a fenced diffstream block via AttachMessageParserToAccumulator
// and verify DefaultOperationHandler wrote the file and committed when commit op is sent.
func TestFenceIntegration_WriteAndCommit(t *testing.T) {
	tmp := t.TempDir()
	repo := tmp
	// init git repo
	if out, err := exec.Command("git", "init").CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v: %s", err, string(out))
	}
	// make sure we run init in tmp
	if out, err := exec.Command("git", "-C", repo, "init").CombinedOutput(); err != nil {
		t.Fatalf("git init in tmp failed: %v: %s", err, string(out))
	}

	// create parser and handler
	dp := NewDiffParser(repo)
	dp.SetHandler(&DefaultOperationHandler{})

	// build fenced message that adds a file
	content := "hello world\n"
	fence := "```diffstream type=file op=add path=info.txt encoding=utf-8\n" + content + "\n```"
	acc := make(chan *AccumulatedResponse, 2)
	out := AttachMessageParserToAccumulator(context.Background(), acc, NewFenceParser(), dp)

	// send the accumulated response
	acc <- &AccumulatedResponse{Chunk: &ChatResponse{Message: Message{Content: fence}}}
	close(acc)

	// drain out channel
	for range out {
	}

	// verify file exists
	got, err := os.ReadFile(filepath.Join(repo, "info.txt"))
	if err != nil {
		t.Fatalf("expected file written: %v", err)
	}
	if string(got) != content+"\n" && string(got) != content {
		t.Fatalf("content mismatch: %q", string(got))
	}

	// now commit via a commit message fence
	acc2 := make(chan *AccumulatedResponse, 2)
	out2 := AttachMessageParserToAccumulator(context.Background(), acc2, NewFenceParser(), dp)
	acc2 <- &AccumulatedResponse{Chunk: &ChatResponse{Message: Message{Content: "```diffstream type=commit message=\"test commit\" finalize=true\n\n```"}}}
	close(acc2)
	for range out2 {
	}

	// check git log has commit
	if out, err := exec.Command("git", "-C", repo, "log", "--oneline").CombinedOutput(); err != nil {
		t.Fatalf("git log failed: %v: %s", err, string(out))
	} else {
		// expect at least one commit
		if len(out) == 0 {
			t.Fatalf("expected git commit, log empty")
		}
	}
}
