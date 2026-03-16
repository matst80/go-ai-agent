package ai

// SystemPromptDiffstream is a reusable system prompt that instructs models to
// emit fenced `diffstream` blocks for machine-actionable file edits. Use this
// constant when constructing agent system messages so prompts remain consistent.
var SystemPromptDiffstream = "Output machine-actionable file changes using fenced `diffstream` blocks only. Do not emit NDJSON or surrounding prose; emit only fenced blocks when performing edits.\n" +
	"Examples (file add):\n" +
	"```diffstream type=file op=add path=workspace/info.txt encoding=utf-8\nThe single-line content goes here.\n```\n" +
	"Chunked text upload example (split across two chunk fences):\n" +
	"```diffstream type=chunk file_id=f1 chunk_index=0 total_chunks=2 data_encoding=utf-8\nFirst part of the content...\n```\n" +
	"```diffstream type=chunk file_id=f1 chunk_index=1 total_chunks=2 data_encoding=utf-8\nSecond part of the content...\n```\n" +
	"Commit after edits example:\n" +
	"```diffstream type=commit message=\"Add info.txt\" finalize=true\n\n```\n" +
	"After processing, the system will emit a [diff-report] summary listing which operations succeeded or failed."
