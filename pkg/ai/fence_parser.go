package ai

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// FenceParser parses fenced diffstream markdown blocks spanning arbitrary chunks.
type FenceParser struct {
	buf strings.Builder
}

func NewFenceParser() *FenceParser { return &FenceParser{} }

func atoiOrZero(s string) int {
	i, _ := strconv.Atoi(s)
	return i
}

func trimQuotes(s string) string {
	return strings.Trim(s, `"'`)
}

func parseHeaderAttrs(raw string) map[string]string {
	m := map[string]string{}
	parts := strings.Fields(strings.TrimSpace(raw))
	for _, tok := range parts {
		if kv := strings.SplitN(tok, "=", 2); len(kv) == 2 {
			v := trimQuotes(kv[1])
			m[kv[0]] = v
		} else {
			m[tok] = "true"
		}
	}
	return m
}

// Parse accepts an AccumulatedResponse and returns any complete StreamMessage events found.
func (p *FenceParser) Parse(ctx context.Context, res *AccumulatedResponse) ([]*StreamMessage, error) {
	if res == nil || res.Chunk == nil {
		return nil, nil
	}
	out := []*StreamMessage{}
	p.buf.WriteString(res.Chunk.Message.Content)
	s := p.buf.String()

	for {
		start := strings.Index(s, "```diffstream")
		if start == -1 {
			break
		}
		// find end of opening line
		nl := strings.IndexByte(s[start:], '\n')
		if nl == -1 {
			break
		}
		header := strings.TrimSpace(s[start : start+nl])
		bodyStart := start + nl + 1
		endIdx := strings.Index(s[bodyStart:], "\n```")
		if endIdx == -1 {
			break
		}
		body := s[bodyStart : bodyStart+endIdx]
		nextPos := bodyStart + endIdx + len("\n```")

		attrs := parseHeaderAttrs(strings.TrimSpace(header[len("```diffstream"):]))
		sm := &StreamMessage{}
		if v, ok := attrs["type"]; ok {
			sm.Type = v
		}
		if v, ok := attrs["op"]; ok {
			sm.Op = v
		}
		if v, ok := attrs["path"]; ok {
			sm.Path = v
		}
		if v, ok := attrs["encoding"]; ok {
			sm.ContentEncoding = v
		}
		if v, ok := attrs["content_encoding"]; ok {
			sm.ContentEncoding = v
		}
		if v, ok := attrs["file_id"]; ok {
			sm.FileID = v
		}
		if v, ok := attrs["data_encoding"]; ok {
			sm.DataEncoding = v
		}
		if v, ok := attrs["chunk_index"]; ok {
			sm.ChunkIndex = atoiOrZero(v)
		}
		if v, ok := attrs["total_chunks"]; ok {
			sm.TotalChunks = atoiOrZero(v)
		}
		if v, ok := attrs["message"]; ok {
			sm.Message = v
		}

		// decide where to put body
		if sm.Type == "chunk" || sm.DataEncoding != "" {
			sm.Data = strings.TrimSpace(body)
			if sm.DataEncoding == "" {
				sm.DataEncoding = "base64"
			}
		} else {
			sm.Content = body
			if sm.ContentEncoding == "" {
				sm.ContentEncoding = "utf-8"
			}
		}

		out = append(out, sm)
		s = s[nextPos:]
	}

	p.buf.Reset()
	p.buf.WriteString(s)
	return out, nil
}

// AttachMessageParserToAccumulator composes a generic MessageParser with a DiffParser.
// It feeds each AccumulatedResponse into the MessageParser; for every parsed
// StreamMessage it invokes diff.HandleMessage. The original AccumulatedResponse
// values are forwarded unchanged to the returned channel.
func AttachMessageParserToAccumulator(ctx context.Context, input <-chan *AccumulatedResponse, mp MessageParser, diff *DiffParser) <-chan *AccumulatedResponse {
	out := make(chan *AccumulatedResponse)
	go func() {
		defer close(out)
		for res := range input {
			if res != nil && mp != nil && diff != nil {
				msgs, err := mp.Parse(ctx, res)
				if err != nil {
					fmt.Printf("message parser error: %v\n", err)
				} else {
					for _, m := range msgs {
						if err := diff.HandleMessage(ctx, m); err != nil {
							fmt.Printf("diff handler error: %v\n", err)
						}
					}
				}
			}
			select {
			case out <- res:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}
