Fenced git diff support
=======================

This repository accepts machine-actionable streamed file edits using fenced Git-style unified diffs.

Key points
- Preferred input: fenced blocks that start with ` ```diff `.
- The body of the fence must be an exact unified git diff.
- The parser reads fenced diff blocks across streamed chunks and emits typed `StreamedBlock` values.
- Parsed `diff` blocks are applied to the configured `repoRoot` with `git apply`.
- `AgentSession` is wired to use the fenced block parser and git diff block handler by default.
- The older `diffstream` and NDJSON parsing path has been removed from the fenced edit flow.

Fenced format overview
- Opening fence: ` ```diff `
- Body: exact unified git diff text
- Closing fence: ` ``` `

Example
-------

```diff
--- a/main.go
+++ b/main.go
@@ -12,5 +12,5 @@ func add(a int, b int) int {
 }
+// Computes the sum of two integer arguments
-// add is a simple function that returns the sum of two integers
 func main() {
```

The system should output only the exact git diff inside the fenced block when making edits.

Creating new files
------------------

Yes — a git diff can create files that do not already exist.

Use the standard unified diff form for new files, for example:

```diff
diff --git a/docs/example.txt b/docs/example.txt
new file mode 100644
--- /dev/null
+++ b/docs/example.txt
@@ -0,0 +1,3 @@
+first line
+second line
+third line
```

Deleting files is also supported with standard git diff syntax, for example:

```diff
diff --git a/old.txt b/old.txt
deleted file mode 100644
--- a/old.txt
+++ /dev/null
@@ -1,2 +0,0 @@
-old line 1
-old line 2
```

System prompt example
---------------------

Use the following system prompt as a starting point when creating agent system messages:

```text
Output machine-actionable file changes using fenced `diff` blocks only.
Do not emit surrounding prose; emit only fenced diff blocks when performing edits.

The contents of each fenced block must be an exact git unified diff that can be applied with `git apply`.

Example:
```diff
diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -12,5 +12,5 @@ func add(a int, b int) int {
 }
+// Computes the sum of two integer arguments
-// add is a simple function that returns the sum of two integers
 func main() {
```
```

Architecture
------------

The streamed edit pipeline is now organized around generic fenced blocks:

- `StreamedBlock`
  - a typed extracted block with:
    - `Type string`
    - `Content string`

- `BlockParser`
  - parses accumulated streamed output into typed blocks

- `BlockHandler`
  - handles parsed blocks by block type

Current concrete implementation:

- `FenceParser`
  - extracts exact fenced `diff` blocks from streamed model output

- `GitDiffBlockHandler`
  - handles `diff` blocks and forwards them to `DiffParser`

- `DiffParser`
  - applies unified diffs
  - records operation reports
  - can also commit repository changes

Notes
-----
- For modifications to existing files, emit a normal unified diff.
- For new files, use `--- /dev/null` and `+++ b/path/to/file`.
- For deleted files, use `--- a/path/to/file` and `+++ /dev/null`.
- Binary files are not a good fit for this fenced diff format; use another transport format if needed.
- After processing, the system may emit a `[diff-report]` summary listing which operations succeeded or failed.

Agent integration
-----------------
- `ai.WithRepoRoot(path)` sets the repository/workspace root where diffs are applied.
- `ai.WithOperationHandler(handler)` lets you provide a custom `OperationHandler`.
- The included `ai.DefaultOperationHandler` applies unified diffs with `git apply` and can commit staged changes.

Example:

```go
masterSession := ai.NewAgentSession(ctx, masterClient, masterReq,
    ai.WithRepoRoot("./test-repo"),
    ai.WithOperationHandler(&ai.DefaultOperationHandler{}),
)
```

Tests
-----
- Unit tests for the fenced block parser live in `pkg/ai/fence_parser_test.go`.
- Integration coverage for fenced diff application lives in `pkg/ai/fence_integration_test.go`.
- Diff application tests live in `pkg/ai/git_diff_test.go`.

The parser tests cover:
- exact `diff` fences
- chunked streamed fences across multiple calls
- multiple fenced blocks in one response
- ignoring invalid or non-exact fence headers
- preserving raw diff body content
- incomplete fence buffering

Files of interest
-----------------
- `pkg/ai/fence_parser.go` — fenced block parsing and block dispatch integration.
- `pkg/ai/git_diff.go` — unified diff application, commit handling, and git diff block handling.
- `pkg/ai/message_parser.go` — shared `StreamedBlock`, `BlockParser`, and `BlockHandler` types.
- `main.go` — example master session and prompts illustrating usage.

Current status
--------------
- The fenced parser path is fully block-based.
- The legacy `diffstream` parser abstraction has been removed from the fenced edit flow.
- NDJSON prompt aliases and compatibility parsing have been removed.
- Naming now reflects the git-diff-based design instead of the old `diffstream` terminology.

Future ideas
------------
- Extend `FenceParser` to support additional exact block types beyond `diff`.
- Add more block handlers for other machine-actionable fenced formats.
- Add more end-to-end tests for deletion patches and commit flows.