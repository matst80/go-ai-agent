package ai

import (
	"context"
	"fmt"
	"strings"
)

// FenceParser parses streamed fenced blocks using the exact form:
//
// ```type
// ...block body...
// ```
type FenceParser struct {
	buf strings.Builder
}

func NewFenceParser() *FenceParser {
	return &FenceParser{}
}

// ParseBlocks accepts accumulated streamed content and emits StreamedBlock values
// whenever complete fenced blocks have been received.
func (p *FenceParser) ParseBlocks(ctx context.Context, res *AccumulatedResponse) ([]*StreamedBlock, error) {
	_ = ctx

	if res == nil || res.Chunk == nil {
		return nil, nil
	}

	p.buf.WriteString(res.Chunk.Message.Content)
	input := p.buf.String()

	var out []*StreamedBlock

	for {
		start := strings.Index(input, "```")
		if start == -1 {
			break
		}

		if start > 0 {
			input = input[start:]
		}

		newlineOffset := strings.IndexByte(input, '\n')
		if newlineOffset == -1 {
			break
		}

		header := strings.TrimSpace(input[3:newlineOffset])
		if header == "" || strings.ContainsAny(header, " \t") || strings.ContainsAny(header, "`") {
			input = input[3:]
			continue
		}
		if header != "diff" {
			input = input[3:]
			continue
		}

		bodyAndTail := input[newlineOffset+1:]
		endOffset := strings.Index(bodyAndTail, "\n```")
		if endOffset == -1 {
			break
		}

		body := bodyAndTail[:endOffset+1]
		out = append(out, &StreamedBlock{
			Type:    header,
			Content: body,
		})

		input = bodyAndTail[endOffset+len("\n```"):]
	}

	p.buf.Reset()
	p.buf.WriteString(input)

	return out, nil
}

// AttachBlockParserToAccumulator feeds accumulated assistant output into the
// block parser, dispatches any parsed blocks to the provided handler, and
// forwards the original accumulated responses unchanged.
func AttachBlockParserToAccumulator(
	ctx context.Context,
	input <-chan *AccumulatedResponse,
	parser BlockParser,
	handler BlockHandler,
) <-chan *AccumulatedResponse {
	out := make(chan *AccumulatedResponse)

	go func() {
		defer close(out)

		for res := range input {
			if res != nil && parser != nil && handler != nil {
				blocks, err := parser.ParseBlocks(ctx, res)
				if err != nil {
					fmt.Printf("block parser error: %v\n", err)
				} else {
					for _, block := range blocks {
						if err := handler.HandleBlock(ctx, block); err != nil {
							fmt.Printf("block handler error: %v\n", err)
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
