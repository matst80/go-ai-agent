package ai

import (
	"context"
	"strings"
	"testing"
)

func TestChunkReader_PreservesSpaces(t *testing.T) {
	input := "data: {\"content\": \"Hi \"}\n" +
		"data: {\"content\": \"Mats!\"}\n"

	var result strings.Builder
	handler := DataJsonChunkReader(func(data *Message) bool {
		result.WriteString(data.Content)
		return false
	})

	reader := strings.NewReader(input)
	err := ChunkReader(context.Background(), reader, handler)
	if err != nil {
		t.Fatalf("ChunkReader failed: %v", err)
	}

	expected := "Hi Mats!"
	if result.String() != expected {
		t.Errorf("expected %q, got %q", expected, result.String())
	}
}

func TestChunkReader_PreservesLeadingSpaces(t *testing.T) {
	input := "data: {\"content\": \"Hello\"}\n" +
		"data: {\"content\": \" how\"}\n" +
		"data: {\"content\": \" are\"}\n" +
		"data: {\"content\": \" you?\"}\n"

	var result strings.Builder
	handler := DataJsonChunkReader(func(data *Message) bool {
		result.WriteString(data.Content)
		return false
	})

	reader := strings.NewReader(input)
	err := ChunkReader(context.Background(), reader, handler)
	if err != nil {
		t.Fatalf("ChunkReader failed: %v", err)
	}

	expected := "Hello how are you?"
	if result.String() != expected {
		t.Errorf("expected %q, got %q", expected, result.String())
	}
}

func TestChunkReader_NoSpaceAfterData(t *testing.T) {
	input := "data:{\"content\":\"Missing\"}\n" +
		"data: {\"content\":\" Space\"}\n"

	var result strings.Builder
	handler := DataJsonChunkReader(func(data *Message) bool {
		result.WriteString(data.Content)
		return false
	})

	reader := strings.NewReader(input)
	err := ChunkReader(context.Background(), reader, handler)
	if err != nil {
		t.Fatalf("ChunkReader failed: %v", err)
	}

	expected := "Missing Space"
	if result.String() != expected {
		t.Errorf("expected %q, got %q", expected, result.String())
	}
}

func TestChunkReader_RawTextSpaces(t *testing.T) {
	input := "Hi\n" +
		" \n" +
		"Mats!\n"

	var results []string
	handler := func(line []byte) bool {
		results = append(results, string(line))
		return false
	}

	reader := strings.NewReader(input)
	err := ChunkReader(context.Background(), reader, handler)
	if err != nil {
		t.Fatalf("ChunkReader failed: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d: %q", len(results), results)
	}
	if results[1] != " " {
		t.Errorf("expected [ ], got %q", results[1])
	}
}
