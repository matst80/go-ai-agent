package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
)

// Handler is the function signature for tool handlers.
type Handler func(ctx context.Context, args json.RawMessage) (Result, error)

// Tool represents a registered tool.
type Tool struct {
	Name        string
	Description string
	Parameters  map[string]interface{}
	Handler     Handler
}

// Result is the return value from a tool execution.
type Result struct {
	Content string
	Error   error
}

// Registry manages tool registration and execution.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]*Tool
}

// NewRegistry creates a new tool registry.
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]*Tool),
	}
}

// Register adds a tool to the registry.
func (r *Registry) Register(t *Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[t.Name] = t
}

// RegisterFunc registers a tool from a function.
func (r *Registry) RegisterFunc(name, description string, params map[string]interface{}, handler Handler) {
	r.Register(&Tool{
		Name:        name,
		Description: description,
		Parameters:  params,
		Handler:     handler,
	})
}

// Get returns a tool by name.
func (r *Registry) Get(name string) *Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.tools[name]
}

// GetAll returns all registered tools.
func (r *Registry) GetAll() []*Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*Tool, 0, len(r.tools))
	for _, t := range r.tools {
		result = append(result, t)
	}
	return result
}

// Call executes a tool by name with the given arguments.
func (r *Registry) Call(ctx context.Context, name string, args json.RawMessage) (Result, error) {
	tool := r.Get(name)
	if tool == nil {
		return Result{}, fmt.Errorf("tool not found: %s", name)
	}
	return tool.Handler(ctx, args)
}

// Builder helps construct tools.
type Builder struct {
	name        string
	description string
	params      map[string]interface{}
	handler     Handler
}

// NewBuilder creates a new tool builder.
func NewBuilder(name string) *Builder {
	return &Builder{
		name:   name,
		params: map[string]interface{}{
			"type": "object",
			"properties":      make(map[string]interface{}),
			"required": []string{},
		},
	}
}

// WithDescription sets the tool description.
func (b *Builder) WithDescription(desc string) *Builder {
	b.description = desc
	return b
}

// WithParam adds a parameter to the tool.
func (b *Builder) WithParam(name, ptype, desc string, required bool) *Builder {
	props := b.params["properties"].(map[string]interface{})
	props[name] = map[string]interface{}{
		"type":        ptype,
		"description": desc,
	}
	if required {
		req := b.params["required"].([]string)
		b.params["required"] = append(req, name)
	}
	return b
}

// WithHandler sets the tool handler function.
func (b *Builder) WithHandler(h Handler) *Builder {
	b.handler = h
	return b
}

// Build returns the constructed tool.
func (b *Builder) Build() *Tool {
	return &Tool{
		Name:        b.name,
		Description: b.description,
		Parameters:  b.params,
		Handler:     b.handler,
	}
}

// JSONSchemaGenerator helps generate JSON schemas from Go structs.
type JSONSchemaGenerator struct {
}

// Generate creates a JSON schema from a Go struct type.
func (g *JSONSchemaGenerator) Generate(t reflect.Type) map[string]interface{} {
	schema := map[string]interface{}{
		"type":       "object",
		"properties": make(map[string]interface{}),
		"required":   []string{},
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		name := field.Name

		// Handle JSON tag
		jsonTag := field.Tag.Get("json")
		if jsonTag != "" && jsonTag != "-" {
			name = jsonTag
		}

		fieldType := field.Type
		prop := map[string]interface{}{
			"description": field.Tag.Get("description"),
		}

		// Determine type
		switch fieldType.Kind() {
		case reflect.String:
			prop["type"] = "string"
		case reflect.Int, reflect.Int64:
			prop["type"] = "integer"
		case reflect.Float64:
			prop["type"] = "number"
		case reflect.Bool:
			prop["type"] = "boolean"
		default:
			prop["type"] = "string"
		}

		props := schema["properties"].(map[string]interface{})
		props[name] = prop

		// Check if required (no pointer type)
		if fieldType.Kind() != reflect.Ptr && field.Tag.Get("required") == "true" {
			req := schema["required"].([]string)
			schema["required"] = append(req, name)
		}
	}

	return schema
}
