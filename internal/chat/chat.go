package chat

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/chzyer/readline"

	"aicli/internal/client"
	"aicli/internal/config"
	"aicli/internal/executor"
	"aicli/internal/session"
	"aicli/internal/tools"
	"aicli/internal/web"
)

type Chat struct {
	client   *client.Client
	cfg      *config.Config
	rl       *readline.Instance
	exec     *executor.Executor
	web      *web.WebSearch
	recorder *session.Recorder
	autoExec bool
	playback *session.Playback
}

func New(cfg *config.Config) (*Chat, error) {
	rl, err := readline.NewEx(&readline.Config{
		Prompt:          "\033[36m>>> \033[0m",
		HistoryFile:     getHistoryPath(),
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		return nil, err
	}

	workDir, _ := os.Getwd()

	// Initialize version file if not exists
	exec := executor.New(workDir)
	exec.InitVersion()

	c := client.New(cfg)

	// Auto-configure model if set to "default"
	if cfg.Model == "default" {
		if models, err := c.ListModels(); err == nil && len(models) > 0 {
			if cfg.AutoConfigModel(models) {
				cfg.Save() // Save the selected model for future sessions
			}
		}
	}

	return &Chat{
		client:   c,
		cfg:      cfg,
		rl:       rl,
		exec:     exec,
		web:      web.NewSearch(),
		recorder: session.NewRecorder(workDir),
		autoExec: false,
	}, nil
}

// NewNonInteractive creates a Chat instance for single-prompt mode without readline
func NewNonInteractive(cfg *config.Config, autoExec bool) (*Chat, error) {
	workDir, _ := os.Getwd()

	exec := executor.New(workDir)
	exec.InitVersion()

	c := client.New(cfg)

	// Auto-configure model if set to "default"
	if cfg.Model == "default" {
		if models, err := c.ListModels(); err == nil && len(models) > 0 {
			if cfg.AutoConfigModel(models) {
				cfg.Save()
			}
		}
	}

	return &Chat{
		client:   c,
		cfg:      cfg,
		rl:       nil, // No readline for non-interactive mode
		exec:     exec,
		web:      web.NewSearch(),
		recorder: session.NewRecorder(workDir),
		autoExec: autoExec,
	}, nil
}

// RunSingle executes a single prompt with full tool support
func (c *Chat) RunSingle(prompt string) error {
	c.recorder.RecordUser(prompt)
	c.sendMessage(prompt)
	return nil
}

func NewPlaybackMode(cfg *config.Config, sessionPath string) (*Chat, error) {
	playback, err := session.NewPlayback(sessionPath)
	if err != nil {
		return nil, err
	}

	workDir, _ := os.Getwd()

	c := client.New(cfg)

	// Auto-configure model if set to "default"
	if cfg.Model == "default" {
		if models, err := c.ListModels(); err == nil && len(models) > 0 {
			if cfg.AutoConfigModel(models) {
				cfg.Save()
			}
		}
	}

	return &Chat{
		client:   c,
		cfg:      cfg,
		exec:     executor.New(workDir),
		web:      web.NewSearch(),
		autoExec: true, // Auto-execute in playback mode
		playback: playback,
	}, nil
}

func getHistoryPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	dir := filepath.Join(home, ".config", "aicli")
	os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "history")
}

func (c *Chat) Run() error {
	if c.rl != nil {
		defer c.rl.Close()
	}

	// Check if playback mode
	if c.playback != nil {
		return c.runPlayback()
	}

	v, _ := c.exec.GetVersion()
	fmt.Printf("AI Coding Assistant (v%s)\n", v.String())
	fmt.Println("Commands: /help, /clear, /file, /auto, /models, /model, /quit")
	fmt.Printf("Working directory: %s\n", c.exec.WorkDir())
	fmt.Printf("Session: %s\n\n", c.recorder.SessionPath())

	for {
		line, err := c.rl.Readline()
		if err == readline.ErrInterrupt {
			continue
		}
		if err == io.EOF {
			break
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "/") {
			if c.handleCommand(line) {
				break
			}
			continue
		}

		c.recorder.RecordUser(line)
		c.sendMessage(line)
	}

	return nil
}

func (c *Chat) runPlayback() error {
	fmt.Printf("Playback mode: %d entries\n", c.playback.Total())
	fmt.Println("Press Enter to step through, 'q' to quit, 'a' to run all")

	inputs := c.playback.GetUserInputs()
	for i, input := range inputs {
		fmt.Printf("\n\033[36m[%d/%d] User: %s\033[0m\n", i+1, len(inputs), input)

		// Wait for user
		fmt.Print("(Enter to continue, 'q' to quit): ")
		var response string
		fmt.Scanln(&response)
		if response == "q" {
			break
		}

		c.sendMessage(input)
	}

	fmt.Println("\nPlayback complete.")
	return nil
}

func (c *Chat) handleCommand(cmd string) bool {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return false
	}

	switch parts[0] {
	case "/quit", "/exit", "/q":
		fmt.Println("Goodbye!")
		return true

	case "/clear", "/new":
		c.client.ClearHistory()
		fmt.Println("Conversation cleared.")

	case "/file", "/f":
		if len(parts) < 2 {
			fmt.Println("Usage: /file <path>")
			return false
		}
		c.addFileContext(parts[1])

	case "/files":
		if len(parts) < 2 {
			fmt.Println("Usage: /files <path1> <path2> ...")
			return false
		}
		for _, path := range parts[1:] {
			c.addFileContext(path)
		}

	case "/cd":
		if len(parts) < 2 {
			fmt.Printf("Current: %s\n", c.exec.WorkDir())
			return false
		}
		newDir := parts[1]
		if !filepath.IsAbs(newDir) {
			newDir = filepath.Join(c.exec.WorkDir(), newDir)
		}
		if info, err := os.Stat(newDir); err != nil || !info.IsDir() {
			fmt.Printf("Invalid directory: %s\n", newDir)
			return false
		}
		c.exec.SetWorkDir(newDir)
		fmt.Printf("Changed to: %s\n", newDir)

	case "/auto":
		c.autoExec = !c.autoExec
		if c.autoExec {
			fmt.Println("Auto-execute enabled (tools run without confirmation)")
		} else {
			fmt.Println("Auto-execute disabled (tools require confirmation)")
		}

	case "/run", "/!":
		if len(parts) < 2 {
			fmt.Println("Usage: /run <command>")
			return false
		}
		command := strings.Join(parts[1:], " ")
		result := c.exec.Run(command)
		fmt.Println(result.String())

	case "/git":
		if len(parts) < 2 {
			fmt.Println("Usage: /git status|diff|log|add|commit")
			return false
		}
		c.handleGitCommand(parts[1:])

	case "/version", "/v":
		v, _ := c.exec.GetVersion()
		fmt.Printf("Version: %s\n", v.String())

	case "/sessions":
		sessions, err := session.ListSessions(c.exec.WorkDir())
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return false
		}
		if len(sessions) == 0 {
			fmt.Println("No sessions found.")
			return false
		}
		fmt.Println("Sessions:")
		for _, s := range sessions {
			fmt.Printf("  %s\n", filepath.Base(s))
		}

	case "/playback":
		if len(parts) < 2 {
			fmt.Println("Usage: /playback <session_file>")
			return false
		}
		sessionPath := parts[1]
		if !filepath.IsAbs(sessionPath) {
			sessionPath = filepath.Join(c.exec.WorkDir(), ".aicli", sessionPath)
		}
		playback, err := session.NewPlayback(sessionPath)
		if err != nil {
			fmt.Printf("Error loading session: %v\n", err)
			return false
		}
		c.playback = playback
		c.autoExec = true
		c.runPlayback()
		c.playback = nil

	case "/search":
		if len(parts) < 2 {
			fmt.Println("Usage: /search <query>")
			return false
		}
		query := strings.Join(parts[1:], " ")
		results, err := c.web.Search(query, 5)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return false
		}
		for i, r := range results {
			fmt.Printf("%d. %s\n   %s\n   %s\n\n", i+1, r.Title, r.URL, r.Snippet)
		}

	case "/screenshot":
		outputPath := ""
		if len(parts) > 1 {
			outputPath = parts[1]
		}
		result := c.exec.ScreenCapture(outputPath, true)
		fmt.Println(result.String())

	case "/help", "/h", "/?":
		c.printHelp()

	case "/config":
		c.printConfig()

	case "/models":
		models, err := c.client.ListModels()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return false
		}
		fmt.Println("Available models:")
		for _, m := range models {
			if m == c.cfg.Model {
				fmt.Printf("  * %s (current)\n", m)
			} else {
				fmt.Printf("    %s\n", m)
			}
		}

	case "/model":
		if len(parts) < 2 {
			fmt.Printf("Current model: %s\n", c.cfg.Model)
			return false
		}
		newModel := parts[1]
		// Validate the model exists
		models, err := c.client.ListModels()
		if err != nil {
			fmt.Printf("Error fetching models: %v\n", err)
			return false
		}
		found := false
		for _, m := range models {
			if m == newModel {
				found = true
				break
			}
		}
		if !found {
			fmt.Printf("Model not found: %s\n", newModel)
			fmt.Println("Use /models to list available models")
			return false
		}
		c.cfg.Model = newModel
		fmt.Printf("Switched to model: %s\n", newModel)

	default:
		fmt.Printf("Unknown command: %s\n", parts[0])
	}

	return false
}

func (c *Chat) handleGitCommand(args []string) {
	var result *executor.Result
	switch args[0] {
	case "status", "st":
		result = c.exec.GitStatus()
	case "diff":
		staged := len(args) > 1 && args[1] == "--staged"
		result = c.exec.GitDiff(staged)
	case "log":
		result = c.exec.GitLog(10)
	case "add":
		if len(args) > 1 {
			result = c.exec.GitAdd(args[1:]...)
		} else {
			result = c.exec.GitAdd()
		}
	case "commit":
		if len(args) < 2 {
			fmt.Println("Usage: /git commit <message>")
			return
		}
		msg := strings.Join(args[1:], " ")
		result = c.exec.GitCommitWithVersion(msg, "patch")
	default:
		fmt.Printf("Unknown git command: %s\n", args[0])
		return
	}
	fmt.Println(result.String())
}

func (c *Chat) addFileContext(path string) {
	content, err := c.exec.ReadFile(path)
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		return
	}

	ext := filepath.Ext(path)
	lang := extToLang(ext)

	contextMsg := fmt.Sprintf("Here is the content of `%s`:\n\n```%s\n%s\n```", filepath.Base(path), lang, content)

	fmt.Printf("\033[33mAdded file: %s (%d bytes)\033[0m\n", path, len(content))

	c.recorder.RecordUser(fmt.Sprintf("[Added file: %s]", path))
	c.client.Chat(contextMsg, false, nil)
}

func extToLang(ext string) string {
	langs := map[string]string{
		".go": "go", ".py": "python", ".js": "javascript", ".ts": "typescript",
		".rs": "rust", ".c": "c", ".cpp": "cpp", ".h": "c", ".hpp": "cpp",
		".java": "java", ".rb": "ruby", ".sh": "bash", ".md": "markdown",
		".json": "json", ".yaml": "yaml", ".yml": "yaml", ".toml": "toml",
		".sql": "sql", ".html": "html", ".css": "css",
	}
	if lang, ok := langs[ext]; ok {
		return lang
	}
	return ""
}

func (c *Chat) sendMessage(msg string) {
	tokenCount := 0
	fmt.Print("\033[90mThinking...\033[0m")

	result, err := c.client.Chat(msg, true, func(token string) {
		tokenCount++
		fmt.Printf("\r\033[K\033[90mThinking... [%d tokens]\033[0m", tokenCount)
	})

	// Clear the "Thinking..." status
	fmt.Print("\r\033[K")

	if err != nil {
		fmt.Printf("\033[31mError: %v\033[0m\n", err)
		return
	}

	// Parse text-based tool calls from content
	textToolCalls, cleanedContent := client.ParseToolCallsFromText(result.Content)
	if len(textToolCalls) > 0 {
		result.ToolCalls = append(result.ToolCalls, textToolCalls...)
		result.Content = cleanedContent
	}

	if result.Content != "" {
		fmt.Print(result.Content)
		c.recorder.RecordAssistant(result.Content)
	}
	fmt.Println()

	for len(result.ToolCalls) > 0 {
		for _, tc := range result.ToolCalls {
			c.recorder.RecordToolCall(tc.Function.Name, tc.Function.Arguments)
			toolResult := c.executeTool(tc)
			c.recorder.RecordToolResult(tc.Function.Name, toolResult)
			c.client.AddToolResult(tc.ID, toolResult)
		}

		tokenCount = 0
		fmt.Print("\033[90mThinking...\033[0m")
		result, err = c.client.ContinueWithToolResults(true, func(token string) {
			tokenCount++
			fmt.Printf("\r\033[K\033[90mThinking... [%d tokens]\033[0m", tokenCount)
		})
		fmt.Print("\r\033[K")
		if err != nil {
			fmt.Printf("\033[31mError: %v\033[0m\n", err)
			return
		}

		// Parse text-based tool calls from continuation
		textToolCalls, cleanedContent = client.ParseToolCallsFromText(result.Content)
		if len(textToolCalls) > 0 {
			result.ToolCalls = append(result.ToolCalls, textToolCalls...)
			result.Content = cleanedContent
		}

		if result.Content != "" {
			fmt.Print(result.Content)
			c.recorder.RecordAssistant(result.Content)
		}
		fmt.Println()
	}

	fmt.Println()
}

func (c *Chat) executeTool(tc tools.ToolCall) string {
	name := tc.Function.Name
	args := tc.Function.Arguments

	fmt.Printf("\n\033[33m[Tool: %s]\033[0m\n", name)

	switch name {
	case "run_command":
		var a tools.RunCommandArgs
		json.Unmarshal([]byte(args), &a)
		fmt.Printf("\033[90m$ %s\033[0m\n", a.Command)

		if !c.autoExec && !c.confirm("Execute this command?") {
			return "User declined to execute command"
		}

		result := c.exec.Run(a.Command)
		output := result.String()
		if output != "" {
			fmt.Println(output)
		}
		if result.Success() {
			return fmt.Sprintf("Command succeeded:\n%s", output)
		}
		return fmt.Sprintf("Command failed (exit %d):\n%s", result.ExitCode, output)

	case "write_file":
		var a tools.WriteFileArgs
		json.Unmarshal([]byte(args), &a)
		return c.handleWriteFile(a.Path, a.Content, "file")

	case "write_doc":
		var a tools.WriteDocArgs
		json.Unmarshal([]byte(args), &a)
		return c.handleWriteFile(a.Path, a.Content, "documentation")

	case "read_file":
		var a tools.ReadFileArgs
		json.Unmarshal([]byte(args), &a)
		fmt.Printf("\033[90mReading: %s\033[0m\n", a.Path)

		content, err := c.exec.ReadFile(a.Path)
		if err != nil {
			return fmt.Sprintf("Failed to read file: %v", err)
		}
		return fmt.Sprintf("Contents of %s:\n```\n%s\n```", a.Path, content)

	case "web_search":
		var a tools.WebSearchArgs
		json.Unmarshal([]byte(args), &a)
		fmt.Printf("\033[90mSearching: %s\033[0m\n", a.Query)

		maxResults := a.MaxResults
		if maxResults <= 0 {
			maxResults = 5
		}

		results, err := c.web.Search(a.Query, maxResults)
		if err != nil {
			return fmt.Sprintf("Search failed: %v", err)
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Search results for '%s':\n\n", a.Query))
		for i, r := range results {
			sb.WriteString(fmt.Sprintf("%d. %s\n   URL: %s\n   %s\n\n", i+1, r.Title, r.URL, r.Snippet))
			fmt.Printf("%d. %s\n", i+1, r.Title)
		}
		return sb.String()

	case "fetch_url":
		var a tools.FetchURLArgs
		json.Unmarshal([]byte(args), &a)
		fmt.Printf("\033[90mFetching: %s\033[0m\n", a.URL)

		content, err := c.web.FetchPage(a.URL)
		if err != nil {
			return fmt.Sprintf("Fetch failed: %v", err)
		}
		return fmt.Sprintf("Content from %s:\n\n%s", a.URL, content)

	case "screenshot":
		var a tools.ScreenshotArgs
		json.Unmarshal([]byte(args), &a)
		fmt.Printf("\033[90mCapturing screenshot...\033[0m\n")

		if !c.autoExec && !c.confirm("Capture screenshot?") {
			return "User declined screenshot"
		}

		result := c.exec.ScreenCapture(a.OutputPath, a.Interactive)
		fmt.Println(result.Output)
		return result.String()

	case "git_status":
		result := c.exec.GitStatus()
		fmt.Println(result.String())
		return result.String()

	case "git_diff":
		var a tools.GitDiffArgs
		json.Unmarshal([]byte(args), &a)
		result := c.exec.GitDiff(a.Staged)
		output := result.String()
		if len(output) > 500 {
			fmt.Printf("%s\n... (truncated)\n", output[:500])
		} else {
			fmt.Println(output)
		}
		return output

	case "git_add":
		var a tools.GitAddArgs
		json.Unmarshal([]byte(args), &a)
		if len(a.Files) > 0 {
			fmt.Printf("\033[90mStaging: %v\033[0m\n", a.Files)
		} else {
			fmt.Printf("\033[90mStaging all changes\033[0m\n")
		}

		if !c.autoExec && !c.confirm("Stage these files?") {
			return "User declined to stage files"
		}

		result := c.exec.GitAdd(a.Files...)
		return result.String()

	case "git_commit":
		var a tools.GitCommitArgs
		json.Unmarshal([]byte(args), &a)
		bump := a.Bump
		if bump == "" {
			bump = "patch"
		}
		fmt.Printf("\033[90mMessage: %s (bump: %s)\033[0m\n", a.Message, bump)

		if !c.autoExec && !c.confirm("Create this commit?") {
			return "User declined to commit"
		}

		result := c.exec.GitCommitWithVersion(a.Message, bump)
		output := result.String()
		fmt.Println(output)
		return output

	case "git_log":
		var a tools.GitLogArgs
		json.Unmarshal([]byte(args), &a)
		count := a.Count
		if count <= 0 {
			count = 10
		}
		result := c.exec.GitLog(count)
		fmt.Println(result.String())
		return result.String()

	case "list_files":
		var a tools.ListFilesArgs
		json.Unmarshal([]byte(args), &a)
		result := c.exec.ListFiles(a.Pattern)
		fmt.Println(result.String())
		return result.String()

	case "get_version":
		v, err := c.exec.GetVersion()
		if err != nil {
			return fmt.Sprintf("Error: %v", err)
		}
		fmt.Printf("Version: %s\n", v.String())
		return fmt.Sprintf("Current version: %s", v.String())

	case "set_version":
		var a tools.SetVersionArgs
		json.Unmarshal([]byte(args), &a)
		fmt.Printf("\033[90mSetting version to: %s\033[0m\n", a.Version)

		if !c.autoExec && !c.confirm("Set this version?") {
			return "User declined"
		}

		v := executor.ParseVersion(a.Version)
		if err := c.exec.SetVersion(v); err != nil {
			return fmt.Sprintf("Error: %v", err)
		}
		return fmt.Sprintf("Version set to %s", v.String())

	default:
		return fmt.Sprintf("Unknown tool: %s", name)
	}
}

func (c *Chat) handleWriteFile(path, content, fileType string) string {
	fmt.Printf("\033[90mPath: %s\033[0m\n", path)
	fmt.Printf("\033[90mContent: %d bytes\033[0m\n", len(content))

	lines := strings.Split(content, "\n")
	if len(lines) > 10 {
		preview := lines[:10]
		fmt.Printf("\033[90m%s\n... (%d more lines)\033[0m\n", strings.Join(preview, "\n"), len(lines)-10)
	} else {
		fmt.Printf("\033[90m%s\033[0m\n", content)
	}

	if !c.autoExec && !c.confirm(fmt.Sprintf("Write this %s?", fileType)) {
		return fmt.Sprintf("User declined to write %s", fileType)
	}

	if err := c.exec.WriteFile(path, content); err != nil {
		fmt.Printf("\033[31mFailed to write %s: %v\033[0m\n", fileType, err)
		return fmt.Sprintf("Failed to write %s: %v", fileType, err)
	}
	fmt.Printf("\033[32m✓ Wrote %s (%d bytes)\033[0m\n", path, len(content))
	return fmt.Sprintf("Successfully wrote %d bytes to %s", len(content), path)
}

func (c *Chat) confirm(prompt string) bool {
	// In non-interactive mode, there's no readline - decline unless autoExec is set
	if c.rl == nil {
		fmt.Printf("\033[33m%s [y/N]: \033[0m(no input in non-interactive mode, use -auto to auto-execute)\n", prompt)
		return false
	}
	fmt.Printf("\033[33m%s [y/N]: \033[0m", prompt)
	line, err := c.rl.Readline()
	if err != nil {
		return false
	}
	line = strings.ToLower(strings.TrimSpace(line))
	return line == "y" || line == "yes"
}

func (c *Chat) printHelp() {
	fmt.Println(`
Commands:
  /help, /h        Show this help
  /quit, /q        Exit the chat
  /clear, /new     Clear conversation history
  /file <path>     Add file content as context
  /files <paths>   Add multiple files as context
  /cd <dir>        Change working directory
  /run <cmd>       Execute a shell command directly
  /git <cmd>       Git commands (status, diff, log, add, commit)
  /version         Show current project version
  /auto            Toggle auto-execute mode
  /search <query>  Search the web
  /screenshot      Capture a screenshot
  /sessions        List recorded sessions
  /playback <file> Replay a session
  /config          Show current configuration
  /models          List available models
  /model [name]    Show or switch current model

The AI can:
  - Execute shell commands (builds, tests, etc.)
  - Create/modify source code and documentation
  - Search the web and fetch pages
  - Capture screenshots
  - Run git operations with auto-versioning
  - Read project files

All sessions are automatically saved in .aicli/ for playback.
Version is auto-bumped on each commit (x.y.z format).
`)
}

func (c *Chat) printConfig() {
	v, _ := c.exec.GetVersion()
	fmt.Printf(`
Configuration:
  Endpoint:    %s
  Model:       %s
  Max Tokens:  %d
  Temperature: %.2f
  Working Dir: %s
  Version:     %s
  Auto-exec:   %v
  Session:     %s
`, c.cfg.APIEndpoint, c.cfg.Model, c.cfg.MaxTokens, c.cfg.Temperature,
		c.exec.WorkDir(), v.String(), c.autoExec, c.recorder.SessionPath())
}
