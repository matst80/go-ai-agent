package ai

import (
	"bytes"
	"context"
	"testing"
)

type TestStruct struct {
	Field1 string `json:"field1"`
	Field2 string `json:"field2"`
}

func TestJsonChunkReader_NoObjectReuse(t *testing.T) {
	chunks := [][]byte{
		[]byte(`{"field1": "val1"}`),
		[]byte(`{"field2": "val2"}`),
	}

	var results []*TestStruct
	handler := JsonChunkReader(func(data *TestStruct) bool {
		results = append(results, data)
		return false
	})

	for _, chunk := range chunks {
		handler(chunk)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results) )
	}

	// Verify first chunk
	if results[0].Field1 != "val1" {
		t.Errorf("chunk 0 field1: expected val1, got %q", results[0].Field1)
	}
	if results[0].Field2 != "" {
		t.Errorf("chunk 0 field2: expected empty, got %q", results[0].Field2)
	}

	// Verify second chunk - IMPORTANT: should NOT have Field1 from first chunk
	if results[1].Field1 != "" {
		t.Errorf("chunk 1 field1: expected empty, got %q (OBJECT REUSE DETECTED)", results[1].Field1)
	}
	if results[1].Field2 != "val2" {
		t.Errorf("chunk 1 field2: expected val2, got %q", results[1].Field2)
	}
}

func TestDataJsonChunkReader_NoObjectReuse(t *testing.T) {
	chunks := [][]byte{
		[]byte(`data: {"field1": "val1"}`),
		[]byte(`data: {"field2": "val2"}`),
	}

	var results []*TestStruct
	handler := DataJsonChunkReader(func(data *TestStruct) bool {
		results = append(results, data)
		return false
	})

	for _, chunk := range chunks {
		handler(chunk)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Verify first chunk
	if results[0].Field1 != "val1" {
		t.Errorf("chunk 0 field1: expected val1, got %q", results[0].Field1)
	}
	if results[0].Field2 != "" {
		t.Errorf("chunk 0 field2: expected empty, got %q", results[0].Field2)
	}

	// Verify second chunk - IMPORTANT: should NOT have Field1 from first chunk
	if results[1].Field1 != "" {
		t.Errorf("chunk 1 field1: expected empty, got %q (OBJECT REUSE DETECTED)", results[1].Field1)
	}
	if results[1].Field2 != "val2" {
		t.Errorf("chunk 1 field2: expected val2, got %q", results[1].Field2)
	}
}

func TestChunkReader_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	r := bytes.NewReader([]byte("data: {}\n"))
	err := ChunkReader(ctx, r, func(line []byte) bool {
		return false
	})

	if err != context.Canceled {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}
