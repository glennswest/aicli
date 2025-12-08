package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	APIEndpoint string `json:"api_endpoint"`
	APIKey      string `json:"api_key"`
	Model       string `json:"model"`
	MaxTokens   int    `json:"max_tokens"`
	Temperature float64 `json:"temperature"`
	SystemPrompt string `json:"system_prompt"`
}

func DefaultConfig() *Config {
	return &Config{
		APIEndpoint:  "http://localhost:8000/v1",
		APIKey:       "",
		Model:        "default",
		MaxTokens:    4096,
		Temperature:  0.7,
		SystemPrompt: `You are an expert coding assistant with full access to development tools. You can:

Code & Files:
- Create/modify source code files and documentation
- Read files to understand the codebase
- Run shell commands (builds, tests, installs)

Git & Versioning:
- Git operations (status, diff, add, commit)
- Commits auto-bump the version (x.y.z format)
- Use bump:"minor" or "major" for significant changes

Research:
- Search the web for documentation and solutions
- Fetch and read web pages

Other:
- Capture screenshots when needed

Workflow:
1. Understand existing code before making changes
2. Make targeted, minimal modifications
3. Test changes with build/test commands
4. Commit with descriptive messages

Be concise. Use tools proactively.`,
	}
}

func ConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "aicli", "config.json"), nil
}

func Load() (*Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return DefaultConfig(), nil
	}

	data, err := os.ReadFile(path)
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

	return cfg, nil
}

func (c *Config) Save() error {
	path, err := ConfigPath()
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
