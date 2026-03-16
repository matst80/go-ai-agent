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

// SystemPromptNDJSON is a reusable system prompt that instructs models to
// emit newline-delimited JSON (NDJSON) objects (one JSON object per line)
// describing file operations. This is provided for backwards compatibility
// and for models that prefer emitting JSON lines.
var SystemPromptNDJSON = "Output machine-actionable file changes as newline-delimited JSON (NDJSON).\n" +
	"Each line must be a single JSON object with fields: type, and for type=file: op,path,content_encoding,content,sha256(optional),atomic(optional).\n" +
	"For large text files you may emit chunk objects with file_id,chunk_index,total_chunks,data_encoding=utf-8 and data as the chunk body.\n" +
	"Commit objects should be emitted as: {\"type\":\"commit\", \"message\":\"...\", \"finalize\":true}.\n" +
	"Only emit NDJSON lines (no surrounding prose) when performing edits. After processing, the system will emit a [diff-report] summary."
