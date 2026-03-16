package ai

// SystemPromptGitDiff is the preferred reusable system prompt. It instructs models to
// emit fenced `diff` blocks containing exact git unified diffs.
var SystemPromptGitDiff = "Output machine-actionable file changes using fenced `diff` blocks only. Do not emit surrounding prose when performing edits.\n" +
	"The contents of each fence must be an exact git unified diff that can be applied with git apply.\n" +
	"Use this form:\n" +
	"```diff\n" +
	"--- a/path/to/file\n" +
	"+++ b/path/to/file\n" +
	"@@ ...\n" +
	"...diff content...\n" +
	"```\n" +
	"For new files, use standard git diff format, for example:\n" +
	"```diff\n" +
	"diff --git a/newfile.txt b/newfile.txt\n" +
	"new file mode 100644\n" +
	"--- /dev/null\n" +
	"+++ b/newfile.txt\n" +
	"@@ -0,0 +1,2 @@\n" +
	"+first line\n" +
	"+second line\n" +
	"```\n" +
	"For deleted files, use standard git diff format with /dev/null on the new side.\n" +
	"Preserve exact patch text inside the fence. After processing, the system will emit a [diff-report] summary listing which operations succeeded or failed."
