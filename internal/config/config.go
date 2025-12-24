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

	// Tool permissions: "always", "ask", or "never" per tool
	// Tools: write_file, run_command, git_commit, git_add, screenshot, set_version
	ToolPermissions map[string]string `json:"tool_permissions,omitempty"`

	// UserInterrupts: if true, inject user messages to nudge model on errors
	// Smarter models (qwen2.5:72b) don't need this; weaker models might
	UserInterrupts bool `json:"user_interrupts,omitempty"`

	// Internal: tracks which config file was loaded
	loadedFrom string
}

// Permission constants
const (
	PermissionAlways = "always"
	PermissionAsk    = "ask"
	PermissionNever  = "never"
)

// GetToolPermission returns the permission for a tool, defaulting to "ask"
func (c *Config) GetToolPermission(tool string) string {
	if c.ToolPermissions == nil {
		return PermissionAsk
	}
	if perm, ok := c.ToolPermissions[tool]; ok {
		return perm
	}
	return PermissionAsk
}

// SetToolPermission sets the permission for a tool and saves config
func (c *Config) SetToolPermission(tool, permission string) {
	if c.ToolPermissions == nil {
		c.ToolPermissions = make(map[string]string)
	}
	c.ToolPermissions[tool] = permission
}

func DefaultConfig() *Config {
	return &Config{
		APIEndpoint: "http://localhost:11434/v1",
		APIKey:      "",
		Model:       "default",
		MaxTokens:   4096,
		Temperature: 0.7,
		SystemPrompt: `You are an expert coding assistant. You MUST use tools to perform actions - never just show code in markdown blocks.

PLANNING PHASE - For any non-trivial task:
1. First, use list_files to see what already exists in the working directory
2. Analyze what the user is asking for
3. List the steps needed (e.g., "Step 1: Create main file, Step 2: Build...")
4. Execute ONE step at a time
5. After each step, READ THE TOOL RESULT and verify it succeeded
6. If a step fails, you will receive a REQUIRED TODO - complete it before proceeding

CRITICAL - WORKING DIRECTORY:
- You are already in the user's project directory
- Do NOT create subdirectories - work in the current directory
- Create files directly in the working directory (e.g., "main.go" not "myproject/main.go")
- Do NOT use mkdir commands
- Use list_files first to see what exists

CRITICAL - ERROR HANDLING WITH REQUIRED TODOs:
When a command fails, you will receive a message containing "REQUIRED TODO".
This is a BLOCKING task - you MUST complete it before doing anything else.

Example of a failed command response:
  COMMAND FAILED (exit 1)
  === STOP - DO NOT PROCEED ===
  REQUIRED TODO:
  1. [BLOCKING] Fix error: missing go.sum entry
  2. Re-run the command to verify the fix works

When you see this pattern:
1. STOP immediately - do not run any other commands
2. Read the error message carefully
3. Fix the issue (e.g., run "go mod tidy" for missing dependencies)
4. Re-run the original command to verify success
5. Only then continue to the next step

NEVER:
- Claim success after seeing "COMMAND FAILED"
- Run the next step after a failure
- Ignore REQUIRED TODO items
- Try to run an executable that failed to build

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
3. Execute ONE step, wait for result, VERIFY SUCCESS, then proceed
4. If you see "REQUIRED TODO", complete it before anything else
5. Execute tools in logical order (create file, then build, then run)
6. After build, ONLY run the executable if the build succeeded`,
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

// AutoConfigModel selects a model, preferring running models over available ones.
// If the current model is "default" or not in any list, it picks the first running
// model, or falls back to the first available model. Returns true if changed.
func (c *Config) AutoConfigModel(runningModels, availableModels []string) bool {
	if len(runningModels) == 0 && len(availableModels) == 0 {
		return false
	}

	// Helper to check if model exists in a list
	modelInList := func(model string, list []string) bool {
		for _, m := range list {
			if m == model {
				return true
			}
		}
		return false
	}

	// Check if current model needs auto-configuration
	needsConfig := c.Model == "default"
	if !needsConfig {
		// Check if current model exists in running or available models
		needsConfig = !modelInList(c.Model, runningModels) && !modelInList(c.Model, availableModels)
	}

	if needsConfig {
		// Prefer running models, fall back to available models
		if len(runningModels) > 0 {
			c.Model = runningModels[0]
		} else if len(availableModels) > 0 {
			c.Model = availableModels[0]
		} else {
			return false
		}
		return true
	}
	return false
}
