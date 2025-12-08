package tools

import (
	"encoding/json"
)

type Tool struct {
	Type     string   `json:"type"`
	Function Function `json:"function"`
}

type Function struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

func GetTools() []Tool {
	return []Tool{
		{
			Type: "function",
			Function: Function{
				Name:        "run_command",
				Description: "Execute a shell command. Use for builds, tests, installing dependencies, etc.",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"command": {
							"type": "string",
							"description": "The shell command to execute"
						}
					},
					"required": ["command"]
				}`),
			},
		},
		{
			Type: "function",
			Function: Function{
				Name:        "write_file",
				Description: "Create or overwrite a file with the given content. Use for source code, config files, etc.",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"path": {
							"type": "string",
							"description": "File path (relative to working directory or absolute)"
						},
						"content": {
							"type": "string",
							"description": "The complete file content to write"
						}
					},
					"required": ["path", "content"]
				}`),
			},
		},
		{
			Type: "function",
			Function: Function{
				Name:        "write_doc",
				Description: "Write documentation files (README, docs, guides, etc.)",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"path": {
							"type": "string",
							"description": "Documentation file path (e.g., README.md, docs/guide.md)"
						},
						"content": {
							"type": "string",
							"description": "The documentation content in markdown format"
						}
					},
					"required": ["path", "content"]
				}`),
			},
		},
		{
			Type: "function",
			Function: Function{
				Name:        "read_file",
				Description: "Read the contents of a file",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"path": {
							"type": "string",
							"description": "File path to read"
						}
					},
					"required": ["path"]
				}`),
			},
		},
		{
			Type: "function",
			Function: Function{
				Name:        "web_search",
				Description: "Search the web for information. Use for finding documentation, solutions, APIs, etc.",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"query": {
							"type": "string",
							"description": "Search query"
						},
						"max_results": {
							"type": "integer",
							"description": "Maximum number of results (default 5)"
						}
					},
					"required": ["query"]
				}`),
			},
		},
		{
			Type: "function",
			Function: Function{
				Name:        "fetch_url",
				Description: "Fetch and read content from a URL",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"url": {
							"type": "string",
							"description": "URL to fetch"
						}
					},
					"required": ["url"]
				}`),
			},
		},
		{
			Type: "function",
			Function: Function{
				Name:        "screenshot",
				Description: "Capture a screenshot of the screen",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"output_path": {
							"type": "string",
							"description": "Path to save screenshot (optional, auto-generated if not provided)"
						},
						"interactive": {
							"type": "boolean",
							"description": "If true, allows user to select screen area"
						}
					}
				}`),
			},
		},
		{
			Type: "function",
			Function: Function{
				Name:        "git_status",
				Description: "Show git status of the repository",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {}
				}`),
			},
		},
		{
			Type: "function",
			Function: Function{
				Name:        "git_diff",
				Description: "Show git diff of changes",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"staged": {
							"type": "boolean",
							"description": "If true, show only staged changes"
						}
					}
				}`),
			},
		},
		{
			Type: "function",
			Function: Function{
				Name:        "git_add",
				Description: "Stage files for commit",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"files": {
							"type": "array",
							"items": {"type": "string"},
							"description": "Files to stage. Empty array means stage all changes."
						}
					}
				}`),
			},
		},
		{
			Type: "function",
			Function: Function{
				Name:        "git_commit",
				Description: "Create a git commit with staged changes. Automatically bumps version number.",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"message": {
							"type": "string",
							"description": "Commit message"
						},
						"bump": {
							"type": "string",
							"enum": ["patch", "minor", "major"],
							"description": "Version bump type (default: patch)"
						}
					},
					"required": ["message"]
				}`),
			},
		},
		{
			Type: "function",
			Function: Function{
				Name:        "git_log",
				Description: "Show recent git commits",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"count": {
							"type": "integer",
							"description": "Number of commits to show (default 10)"
						}
					}
				}`),
			},
		},
		{
			Type: "function",
			Function: Function{
				Name:        "list_files",
				Description: "List source code files in the project",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"pattern": {
							"type": "string",
							"description": "Directory or pattern to search (default: current directory)"
						}
					}
				}`),
			},
		},
		{
			Type: "function",
			Function: Function{
				Name:        "get_version",
				Description: "Get the current project version",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {}
				}`),
			},
		},
		{
			Type: "function",
			Function: Function{
				Name:        "set_version",
				Description: "Set the project version manually",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"version": {
							"type": "string",
							"description": "Version string in x.y.z format"
						}
					},
					"required": ["version"]
				}`),
			},
		},
	}
}

// Arguments structs for parsing
type RunCommandArgs struct {
	Command string `json:"command"`
}

type WriteFileArgs struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type WriteDocArgs struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type ReadFileArgs struct {
	Path string `json:"path"`
}

type WebSearchArgs struct {
	Query      string `json:"query"`
	MaxResults int    `json:"max_results"`
}

type FetchURLArgs struct {
	URL string `json:"url"`
}

type ScreenshotArgs struct {
	OutputPath  string `json:"output_path"`
	Interactive bool   `json:"interactive"`
}

type GitDiffArgs struct {
	Staged bool `json:"staged"`
}

type GitAddArgs struct {
	Files []string `json:"files"`
}

type GitCommitArgs struct {
	Message string `json:"message"`
	Bump    string `json:"bump"`
}

type GitLogArgs struct {
	Count int `json:"count"`
}

type ListFilesArgs struct {
	Pattern string `json:"pattern"`
}

type SetVersionArgs struct {
	Version string `json:"version"`
}
