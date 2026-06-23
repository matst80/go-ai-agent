package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds the application configuration.
type Config struct {
	API         APIConfig         `yaml:"api"`
	Model       string            `yaml:"model"`
	SystemPrompt string           `yaml:"system_prompt"`
	Temperature  *float64         `yaml:"temperature"`
	Tools        []ToolConfig      `yaml:"tools"`
}

type APIConfig struct {
	BaseURL string `yaml:"base_url"`
	APIKey  string `yaml:"api_key"`
}

type ToolConfig struct {
	Name        string                 `yaml:"name"`
	Description string                 `yaml:"description"`
	Parameters  map[string]interface{} `yaml:"parameters"`
	Enabled     bool                   `yaml:"enabled"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	temp := 0.7
	return &Config{
		Model:        "gpt-4o",
		SystemPrompt: "You are a helpful assistant.",
		Temperature:  &temp,
	}
}

// Load loads configuration from a YAML file, environment variables, and applies defaults.
func Load() (*Config, error) {
	cfg := DefaultConfig()

	// Try to load from config file
	configPath := findConfigPath()
	if configPath != "" {
		data, err := os.ReadFile(configPath)
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("failed to read config file: %w", err)
			}
		} else if len(bytes.TrimSpace(data)) > 0 {
			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, fmt.Errorf("failed to parse config file: %w", err)
			}
		}
	}

	// Environment variables override config file
	if v := os.Getenv("AI_API_KEY"); v != "" {
		cfg.API.APIKey = v
	}
	if v := os.Getenv("AI_BASE_URL"); v != "" {
		cfg.API.BaseURL = v
	}
	if v := os.Getenv("AI_MODEL"); v != "" {
		cfg.Model = v
	}
	if v := os.Getenv("AI_SYSTEM_PROMPT"); v != "" {
		cfg.SystemPrompt = v
	}
	if v := os.Getenv("AI_TEMPERATURE"); v != "" {
		if temp := parseFloat(v); temp >= 0 && temp <= 2 {
			cfg.Temperature = &temp
		}
	}

	// Validate
	if cfg.API.APIKey == "" {
		return nil, fmt.Errorf("API key is required (set via AI_API_KEY env var or config file)")
	}
	if cfg.API.BaseURL == "" {
		cfg.API.BaseURL = "https://api.openai.com/v1"
	}

	return cfg, nil
}

// Save saves the configuration to a YAML file.
func Save(cfg *Config) error {
	configPath := findConfigPath()
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	return os.WriteFile(configPath, data, 0o600)
}

// findConfigPath returns the path to the config file, creating the directory if needed.
func findConfigPath() string {
	home, _ := os.UserHomeDir()

	// XDG path
	xdg := os.Getenv("XDG_CONFIG_HOME")
	if xdg == "" {
		xdg = filepath.Join(home, ".config")
	}
	xdgPath := filepath.Join(xdg, "go-ai-agent", "config.yaml")
	if _, err := os.Stat(xdgPath); err == nil {
		return xdgPath
	}

	// User home directory
	homePath := filepath.Join(home, ".ai", "config.yaml")
	if _, err := os.Stat(homePath); err == nil {
		return homePath
	}

	// Current directory
	localPath := "./.ai/config.yaml"
	if _, err := os.Stat(localPath); err == nil {
		return localPath
	}

	// Default to XDG path for saving
	if err := os.MkdirAll(filepath.Dir(xdgPath), 0o755); err != nil {
		return homePath
	}
	return xdgPath
}

func parseFloat(s string) float64 {
	var v float64
	fmt.Sscanf(s, "%f", &v)
	return v
}
