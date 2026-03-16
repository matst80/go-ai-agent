package ai

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Integration test: feed a fenced git diff block through the generic block
// parser/handler flow and verify the diff is applied.
func TestFenceIntegration_ApplyDiffThroughBlockHandler(t *testing.T) {
	tmp := t.TempDir()
	repo := tmp

	if out, err := exec.Command("git", "-C", repo, "init").CombinedOutput(); err != nil {
		t.Fatalf("git init in tmp failed: %v: %s", err, string(out))
	}
	if out, err := exec.Command("git", "-C", repo, "config", "user.name", "Test").CombinedOutput(); err != nil {
		t.Fatalf("git config user.name failed: %v: %s", err, string(out))
	}
	if out, err := exec.Command("git", "-C", repo, "config", "user.email", "test@example.com").CombinedOutput(); err != nil {
		t.Fatalf("git config user.email failed: %v: %s", err, string(out))
	}

	original := "package main\n\nfunc add(a int, b int) int {\n\treturn a + b\n}\n"
	mainPath := filepath.Join(repo, "main.go")
	if err := os.WriteFile(mainPath, []byte(original), 0o644); err != nil {
		t.Fatalf("write seed file failed: %v", err)
	}
	if out, err := exec.Command("git", "-C", repo, "add", "main.go").CombinedOutput(); err != nil {
		t.Fatalf("git add seed file failed: %v: %s", err, string(out))
	}
	if out, err := exec.Command("git", "-C", repo, "commit", "-m", "seed").CombinedOutput(); err != nil {
		t.Fatalf("git commit seed file failed: %v: %s", err, string(out))
	}

	parser := NewFenceParser()
	dp := NewDiffParser(repo)
	handler := NewGitDiffBlockHandler(dp)

	diff := "```diff\n" +
		"--- a/main.go\n" +
		"+++ b/main.go\n" +
		"@@ -1,5 +1,6 @@\n" +
		" package main\n" +
		" \n" +
		" func add(a int, b int) int {\n" +
		"+\t// Computes the sum of two integer arguments\n" +
		" \treturn a + b\n" +
		" }\n" +
		"```"

	acc := make(chan *AccumulatedResponse, 2)
	out := AttachBlockParserToAccumulator(context.Background(), acc, parser, handler)

	acc <- &AccumulatedResponse{
		Chunk: &ChatResponse{
			Message: Message{Content: diff},
		},
	}
	close(acc)

	for range out {
	}

	got, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatalf("expected patched file: %v", err)
	}
	wantFragment := "\t// Computes the sum of two integer arguments\n\treturn a + b\n"
	if !strings.Contains(string(got), wantFragment) {
		t.Fatalf("patched content missing fragment %q in %q", wantFragment, string(got))
	}
}
