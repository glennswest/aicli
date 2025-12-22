package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	APIEndpoint  string  `json:"api_endpoint"`
	APIKey       string  `json:"api_key"`
	Model        string  `json:"model"`
	MaxTokens    int     `json:"max_tokens"`
	Temperature  float64 `json:"temperature"`
	SystemPrompt string  `json:"system_prompt"`

	// Internal: tracks which config file was loaded
	loadedFrom string
}

func DefaultConfig() *Config {
	return &Config{
		APIEndpoint: "http://localhost:8000/v1",
		APIKey:      "",
		Model:       "default",
		MaxTokens:   4096,
		Temperature: 0.7,
		SystemPrompt: `You are an expert coding assistant. You MUST use tools to perform actions - never just show code in markdown blocks.

CRITICAL: To perform ANY action, you MUST use this EXACT format:

<tool_call>
{"name": "TOOL_NAME", "arguments": {"param1": "value1"}}
</tool_call>

Available tools:
- write_file: Create/modify files. Args: path, content
- read_file: Read file contents. Args: path
- run_command: Execute shell commands. Args: command
- list_files: List files in directory. Args: pattern
- git_status, git_diff, git_add, git_commit, git_log

Example - To create a file:
<tool_call>
{"name": "write_file", "arguments": {"path": "main.go", "content": "package main\n\nfunc main() {\n\tprintln(\"Hello\")\n}"}}
</tool_call>

Example - To run a command:
<tool_call>
{"name": "run_command", "arguments": {"command": "go build -o app ."}}
</tool_call>

RULES:
1. ALWAYS use <tool_call> tags - NEVER just show code blocks
2. One tool call per <tool_call> block
3. Use multiple blocks for multiple actions
4. Execute tools in logical order (create file, then build, then run)
5. Wait for tool results before proceeding`,
	}
}

// LocalConfigPath returns the path to the local project config file
func LocalConfigPath() string {
	return filepath.Join(".aicli", "config.json")
}

// GlobalConfigPath returns the path to the global config file
func GlobalConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "aicli", "config.json"), nil
}

// ConfigPath returns the path to use for saving (prefers local if it exists)
// Deprecated: use LocalConfigPath or GlobalConfigPath
func ConfigPath() (string, error) {
	return GlobalConfigPath()
}

// Load loads config, checking local first then falling back to global
func Load() (*Config, error) {
	// First check for local config in current directory
	localPath := LocalConfigPath()
	if data, err := os.ReadFile(localPath); err == nil {
		cfg := DefaultConfig()
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("invalid local config: %w", err)
		}
		cfg.loadedFrom = localPath
		return cfg, nil
	}

	// Fall back to global config
	globalPath, err := GlobalConfigPath()
	if err != nil {
		return DefaultConfig(), nil
	}

	data, err := os.ReadFile(globalPath)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, err
	}

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	cfg.loadedFrom = globalPath

	return cfg, nil
}

// Save saves config to the local project directory
func (c *Config) Save() error {
	localPath := LocalConfigPath()
	dir := filepath.Dir(localPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(localPath, data, 0600)
}

// SaveGlobal saves config to the global config directory
func (c *Config) SaveGlobal() error {
	path, err := GlobalConfigPath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

// LoadedFrom returns the path the config was loaded from, or empty if defaults
func (c *Config) LoadedFrom() string {
	return c.loadedFrom
}

// AutoConfigModel selects the first available model if the current model
// is "default" or not in the available models list. Returns true if changed.
func (c *Config) AutoConfigModel(availableModels []string) bool {
	if len(availableModels) == 0 {
		return false
	}

	// Check if current model needs auto-configuration
	needsConfig := c.Model == "default"
	if !needsConfig {
		// Check if current model exists in available models
		found := false
		for _, m := range availableModels {
			if m == c.Model {
				found = true
				break
			}
		}
		needsConfig = !found
	}

	if needsConfig {
		c.Model = availableModels[0]
		return true
	}
	return false
}
