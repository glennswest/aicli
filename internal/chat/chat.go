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
	client    *client.Client
	cfg       *config.Config
	rl        *readline.Instance
	exec      *executor.Executor
	web       *web.WebSearch
	recorder  *session.Recorder
	todoFile  *session.TodoFile
	changelog *session.ChangelogFile
	history   *session.HistoryFile
	autoExec  bool
	playback  *session.Playback
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

	c := client.NewWithDebug(cfg, workDir)

	return &Chat{
		client:    c,
		cfg:       cfg,
		rl:        rl,
		exec:      exec,
		web:       web.NewSearch(),
		recorder:  session.NewRecorder(workDir),
		todoFile:  session.NewTodoFile(workDir),
		changelog: session.NewChangelogFile(workDir),
		history:   session.NewHistoryFile(workDir),
		autoExec:  false,
	}, nil
}

// NewNonInteractive creates a Chat instance for single-prompt mode without readline
func NewNonInteractive(cfg *config.Config, autoExec bool) (*Chat, error) {
	workDir, _ := os.Getwd()

	exec := executor.New(workDir)
	exec.InitVersion()

	c := client.NewWithDebug(cfg, workDir)

	return &Chat{
		client:    c,
		cfg:       cfg,
		rl:        nil, // No readline for non-interactive mode
		exec:      exec,
		web:       web.NewSearch(),
		recorder:  session.NewRecorder(workDir),
		todoFile:  session.NewTodoFile(workDir),
		changelog: session.NewChangelogFile(workDir),
		history:   session.NewHistoryFile(workDir),
		autoExec:  autoExec,
	}, nil
}

// RunSingle executes a single prompt with full tool support
func (c *Chat) RunSingle(prompt string) error {
	c.recorder.RecordUser(prompt)
	c.history.AddRequest(prompt)
	c.sendMessage(prompt)
	return nil
}

// pushTodo adds a required action to the todo list (persistent)
func (c *Chat) pushTodo(action string) {
	c.todoFile.AddTodo(action)
}

// clearTodosMatching removes all todos containing the given substring
func (c *Chat) clearTodosMatching(substr string) {
	c.todoFile.RemoveByContent(substr)
}

// popTodo marks the first todo as completed and returns its content
func (c *Chat) popTodo() string {
	return c.todoFile.PopFirst()
}

// clearTodos removes all todos
func (c *Chat) clearTodos() {
	c.todoFile.Clear()
}

// getTodoPrompt returns a prompt prefix if there are pending todos
func (c *Chat) getTodoPrompt() string {
	pending := c.todoFile.GetPending()
	if len(pending) == 0 {
		return ""
	}
	return fmt.Sprintf(`
=== BLOCKING TODO - YOU MUST DO THIS FIRST ===
%s

Execute this command NOW before doing anything else.
=====================================

`, pending[0].Content)
}

func NewPlaybackMode(cfg *config.Config, sessionPath string) (*Chat, error) {
	playback, err := session.NewPlayback(sessionPath)
	if err != nil {
		return nil, err
	}

	workDir, _ := os.Getwd()

	c := client.NewWithDebug(cfg, workDir)

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

	// Check for pending todos from previous session
	pending := c.todoFile.GetPending()
	if len(pending) > 0 {
		fmt.Printf("\033[33m>>> Found %d pending todo(s) from previous session:\033[0m\n", len(pending))
		for i, todo := range pending {
			status := "[ ]"
			if todo.Status == "in_progress" {
				status = "[>]"
			}
			fmt.Printf("  %s %d. %s\n", status, i+1, todo.Content)
		}
		fmt.Print("\n\033[33mResume this work? (y/n): \033[0m")
		line, err := c.rl.Readline()
		if err == nil && strings.ToLower(strings.TrimSpace(line)) == "y" {
			// Inject todos as context for the first message
			todoContext := "I need to continue working on these pending tasks:\n"
			for i, todo := range pending {
				todoContext += fmt.Sprintf("%d. %s\n", i+1, todo.Content)
			}
			todoContext += "\nPlease help me complete these tasks."
			c.recorder.RecordUser("[Resuming from previous session]")
			c.history.AddRequest("[Resume] Continuing pending tasks")
			c.sendMessage(todoContext)
		} else {
			fmt.Println("Starting fresh session. Use /todos to view pending items.")
		}
		fmt.Println()
	}

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
		c.history.AddRequest(line)
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

	case "/permissions", "/perms":
		c.handlePermissionsCommand(parts[1:])

	case "/todos", "/todo":
		c.handleTodosCommand(parts[1:])

	case "/changelog":
		c.handleChangelogCommand(parts[1:])

	case "/history":
		c.handleHistoryCommand(parts[1:])

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

func (c *Chat) handlePermissionsCommand(args []string) {
	toolNames := []string{"write_file", "run_command", "git_add", "git_commit", "screenshot", "set_version"}

	if len(args) == 0 {
		// Show current permissions
		fmt.Println("\nTool Permissions:")
		fmt.Println("─────────────────────────────────────")
		for _, tool := range toolNames {
			perm := c.cfg.GetToolPermission(tool)
			var permColor string
			switch perm {
			case config.PermissionAlways:
				permColor = "\033[32m" // green
			case config.PermissionNever:
				permColor = "\033[31m" // red
			default:
				permColor = "\033[33m" // yellow
			}
			fmt.Printf("  %-15s %s%s\033[0m\n", tool, permColor, perm)
		}
		fmt.Println("─────────────────────────────────────")
		fmt.Println("Usage: /permissions reset [tool]  - reset to 'ask'")
		fmt.Println("       /permissions set <tool> <always|ask|never>")
		return
	}

	switch args[0] {
	case "reset":
		if len(args) > 1 {
			// Reset specific tool
			tool := args[1]
			c.cfg.SetToolPermission(tool, config.PermissionAsk)
			if err := c.cfg.Save(); err != nil {
				fmt.Printf("Error saving config: %v\n", err)
				return
			}
			fmt.Printf("Reset %s to 'ask'\n", tool)
		} else {
			// Reset all
			c.cfg.ToolPermissions = nil
			if err := c.cfg.Save(); err != nil {
				fmt.Printf("Error saving config: %v\n", err)
				return
			}
			fmt.Println("Reset all permissions to 'ask'")
		}

	case "set":
		if len(args) < 3 {
			fmt.Println("Usage: /permissions set <tool> <always|ask|never>")
			return
		}
		tool := args[1]
		perm := args[2]
		if perm != config.PermissionAlways && perm != config.PermissionAsk && perm != config.PermissionNever {
			fmt.Println("Permission must be: always, ask, or never")
			return
		}
		c.cfg.SetToolPermission(tool, perm)
		if err := c.cfg.Save(); err != nil {
			fmt.Printf("Error saving config: %v\n", err)
			return
		}
		fmt.Printf("Set %s to '%s'\n", tool, perm)

	default:
		fmt.Println("Unknown subcommand. Use: /permissions [reset|set]")
	}
}

func (c *Chat) handleTodosCommand(args []string) {
	if len(args) == 0 {
		// Show all todos
		todos := c.todoFile.GetAll()
		if len(todos) == 0 {
			fmt.Println("No todos.")
			return
		}

		fmt.Println("\nTodos:")
		fmt.Println("─────────────────────────────────────")
		for i, todo := range todos {
			status := "[ ]"
			statusColor := "\033[33m" // yellow
			if todo.Status == "in_progress" {
				status = "[>]"
				statusColor = "\033[36m" // cyan
			} else if todo.Status == "completed" {
				status = "[x]"
				statusColor = "\033[32m" // green
			}
			fmt.Printf("  %s%s\033[0m %d. %s\n", statusColor, status, i+1, todo.Content)
		}
		fmt.Println("─────────────────────────────────────")
		fmt.Println("Usage: /todos clear       - clear all todos")
		fmt.Println("       /todos add <text>  - add a new todo")
		return
	}

	switch args[0] {
	case "clear":
		c.todoFile.Clear()
		fmt.Println("Cleared all todos.")

	case "add":
		if len(args) < 2 {
			fmt.Println("Usage: /todos add <text>")
			return
		}
		content := strings.Join(args[1:], " ")
		c.todoFile.AddTodo(content)
		c.history.AddTodo(content, "added")
		fmt.Printf("Added todo: %s\n", content)

	default:
		fmt.Println("Unknown subcommand. Use: /todos [clear|add]")
	}
}

func (c *Chat) handleChangelogCommand(args []string) {
	if len(args) == 0 {
		// Show recent changelog entries
		entries := c.changelog.GetRecent(10)
		if len(entries) == 0 {
			fmt.Println("No changelog entries.")
			return
		}

		fmt.Println("\nRecent Changes:")
		fmt.Println("─────────────────────────────────────")
		for _, entry := range entries {
			timeStr := entry.Timestamp.Format("2006-01-02 15:04")
			files := ""
			if len(entry.Files) > 0 {
				files = fmt.Sprintf(" (%s)", strings.Join(entry.Files, ", "))
			}
			fmt.Printf("  [%s] %s: %s%s\n", timeStr, entry.Type, entry.Description, files)
		}
		fmt.Println("─────────────────────────────────────")
		fmt.Printf("Full changelog: %s\n", c.changelog.FilePath())
		return
	}

	switch args[0] {
	case "add":
		if len(args) < 3 {
			fmt.Println("Usage: /changelog add <type> <description>")
			fmt.Println("Types: added, changed, fixed, removed")
			return
		}
		entryType := args[1]
		description := strings.Join(args[2:], " ")
		c.changelog.AddEntry(entryType, description, nil)
		fmt.Printf("Added changelog entry: [%s] %s\n", entryType, description)

	default:
		fmt.Println("Unknown subcommand. Use: /changelog [add]")
	}
}

func (c *Chat) handleHistoryCommand(args []string) {
	count := 10
	if len(args) > 0 {
		if n, err := fmt.Sscanf(args[0], "%d", &count); n == 1 && err == nil {
			// Use provided count
		}
	}

	entries := c.history.GetRecent(count)
	if len(entries) == 0 {
		fmt.Println("No history entries.")
		return
	}

	fmt.Println("\nProject History:")
	fmt.Println("─────────────────────────────────────")
	for _, entry := range entries {
		timeStr := entry.Timestamp.Format("2006-01-02 15:04")
		switch entry.Type {
		case "request":
			// Truncate long requests
			desc := entry.Description
			if len(desc) > 60 {
				desc = desc[:57] + "..."
			}
			fmt.Printf("  [%s] > %s\n", timeStr, desc)
		case "todo":
			fmt.Printf("  [%s] TODO %s: %s\n", timeStr, entry.Details, entry.Description)
		case "change":
			fmt.Printf("  [%s] * %s\n", timeStr, entry.Description)
		case "commit":
			fmt.Printf("  [%s] # %s\n", timeStr, entry.Description)
		}
	}
	fmt.Println("─────────────────────────────────────")
	fmt.Printf("Full history: %s\n", c.history.FilePath())
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
	os.Stdout.Sync()

	result, err := c.client.Chat(msg, true, func(token string) {
		tokenCount++
		fmt.Printf("\r\033[K\033[90mThinking... [%d tokens]\033[0m", tokenCount)
		os.Stdout.Sync()
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
		commandFailed := false
		var failedToolResult string
		for _, tc := range result.ToolCalls {
			c.recorder.RecordToolCall(tc.Function.Name, tc.Function.Arguments)
			toolResult := c.executeTool(tc)
			c.recorder.RecordToolResult(tc.Function.Name, toolResult)
			c.client.AddToolResult(tc.ID, toolResult)

			// Stop executing remaining tool calls if a command failed
			// This forces the LLM to address the error before continuing
			if strings.Contains(toolResult, "COMMAND FAILED") {
				commandFailed = true
				failedToolResult = toolResult
				break
			}
		}
		// If command failed, optionally inject user message to interrupt and force attention
		// Smarter models (qwen2.5:72b) don't need this; they follow the TODO in tool result
		if commandFailed {
			result.ToolCalls = nil

			if c.cfg.UserInterrupts {
				// Build user interrupt message with the first todo
				interruptMsg := "STOP. The command failed. "
				pending := c.todoFile.GetPending()
				if len(pending) > 0 {
					interruptMsg += fmt.Sprintf("You MUST run this command now: %s", pending[0].Content)
				} else {
					interruptMsg += "Read the error above and fix it before continuing."
				}
				c.client.AddUserInterrupt(interruptMsg)
				fmt.Printf("\033[33m[User interrupt: %s]\033[0m\n", interruptMsg)
			}
			_ = failedToolResult // Used for context
		}

		tokenCount = 0
		fmt.Print("\033[90mThinking...\033[0m")
		os.Stdout.Sync()
		result, err = c.client.ContinueWithToolResults(true, func(token string) {
			tokenCount++
			fmt.Printf("\r\033[K\033[90mThinking... [%d tokens]\033[0m", tokenCount)
			os.Stdout.Sync()
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

		if !c.confirmTool("run_command", fmt.Sprintf("Execute command: %s", a.Command)) {
			return "OPERATION FAILED: User declined to execute command. The command was NOT run."
		}

		result := c.exec.Run(a.Command)
		output := result.String()
		stderr := result.Error // Get stderr specifically
		// Output is already streamed during execution, no need to print again

		// Check stderr for errors - even if exit code is 0, stderr may have warnings/errors
		stderrHasError := stderr != "" && (strings.Contains(stderr, "error") ||
			strings.Contains(stderr, "Error") ||
			strings.Contains(stderr, "failed") ||
			strings.Contains(stderr, "not found") ||
			strings.Contains(stderr, "cannot find") ||
			strings.Contains(stderr, "undefined"))

		if result.Success() && !stderrHasError {
			// Check if this completes a pending todo - only pop if command is in the todo
			pendingItems := c.todoFile.GetPending()
			if len(pendingItems) > 0 {
				firstTodo := pendingItems[0].Content
				// Extract the command from the todo (after "Run: " or "Then re-run: ")
				todoCmd := firstTodo
				if strings.HasPrefix(todoCmd, "Run: ") {
					todoCmd = strings.TrimPrefix(todoCmd, "Run: ")
				} else if strings.HasPrefix(todoCmd, "Then re-run: ") {
					todoCmd = strings.TrimPrefix(todoCmd, "Then re-run: ")
				}
				// Only pop if the actual command matches
				if strings.Contains(a.Command, todoCmd) || strings.Contains(todoCmd, a.Command) {
					c.popTodo()
				}
			}

			// If there are remaining todos, optionally inject user interrupt to continue
			pendingItems = c.todoFile.GetPending()
			if len(pendingItems) > 0 && c.cfg.UserInterrupts {
				nextTodo := pendingItems[0].Content
				interruptMsg := fmt.Sprintf("Good. Now run the next command: %s", nextTodo)
				c.client.AddUserInterrupt(interruptMsg)
				fmt.Printf("\033[33m[User: %s]\033[0m\n", interruptMsg)
				return fmt.Sprintf("Command succeeded:\n%s", output)
			}
			return fmt.Sprintf("Command succeeded:\n%s", output)
		}

		// Command failed - push fix onto todo stack
		// Use stderr specifically for fix detection since that's where errors are
		errorSummary := extractErrorSummary(stderr, a.Command)
		if errorSummary == "" {
			errorSummary = extractErrorSummary(output, a.Command)
		}
		fixCmd, isConcrete := getFixCommand(stderr)
		if fixCmd == "" {
			fixCmd, isConcrete = getFixCommand(output)
		}

		// Clear old todos and set fresh ones for this error
		// Clear any existing todos - start fresh with the current fix
		c.clearTodos()

		// Check if the error indicates the command itself is wrong (not just missing prereqs)
		unfixable := isUnfixableByRerun(stderr) || isUnfixableByRerun(output)

		if fixCmd != "" {
			// Build todo list in order (pushTodo prepends, so add in reverse)
			if !unfixable {
				c.pushTodo(fmt.Sprintf("Then re-run: %s", a.Command))
			}
			if isConcrete {
				c.pushTodo(fmt.Sprintf("Run: %s", fixCmd))
			} else {
				c.pushTodo(fmt.Sprintf("Fix: %s", fixCmd))
			}
		} else if unfixable {
			// No fix command but error is unfixable - tell model to check the command
			c.pushTodo("Check the command - the package/version may not exist. Use 'go list -m -versions <module>@latest' to find valid versions")
		}

		todoList := ""
		todoItems := c.todoFile.GetPending()
		if len(todoItems) > 0 {
			todoList = "\n\nREQUIRED TODO STACK (do these in order):\n"
			for i, todo := range todoItems {
				todoList += fmt.Sprintf("%d. %s\n", i+1, todo.Content)
			}
		}

		// Build error message with stderr prominently displayed
		stderrSection := ""
		if stderr != "" {
			stderrSection = fmt.Sprintf("\n\nSTDERR OUTPUT:\n%s\n", stderr)
		}

		return fmt.Sprintf(`COMMAND FAILED (exit %d)
%s
=== STOP - YOU MUST FIX THIS ===
%s
Error Summary: %s

DO NOT run other commands. Execute the first TODO item NOW.`, result.ExitCode, stderrSection, todoList, errorSummary)

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

		if !c.confirmTool("screenshot", "Capture screenshot?") {
			return "OPERATION FAILED: User declined screenshot. No screenshot was taken."
		}

		result := c.exec.ScreenCapture(a.OutputPath, a.Interactive)
		fmt.Println(result.Output)
		return result.String()

	case "git_status":
		result := c.exec.GitStatus()
		// Output already streamed by executor
		return result.String()

	case "git_diff":
		var a tools.GitDiffArgs
		json.Unmarshal([]byte(args), &a)
		result := c.exec.GitDiff(a.Staged)
		// Output already streamed by executor
		return result.String()

	case "git_add":
		var a tools.GitAddArgs
		json.Unmarshal([]byte(args), &a)
		if len(a.Files) > 0 {
			fmt.Printf("\033[90mStaging: %v\033[0m\n", a.Files)
		} else {
			fmt.Printf("\033[90mStaging all changes\033[0m\n")
		}

		if !c.confirmTool("git_add", "Stage these files?") {
			return "OPERATION FAILED: User declined to stage files. No files were staged."
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

		if !c.confirmTool("git_commit", fmt.Sprintf("Create commit: %s", a.Message)) {
			return "OPERATION FAILED: User declined to commit. No commit was created."
		}

		result := c.exec.GitCommitWithVersion(a.Message, bump)
		output := result.String()
		fmt.Println(output)

		// Log successful commits to history and changelog
		if result.Success() {
			c.history.AddCommit(a.Message, "")
			c.changelog.Release("") // Move unreleased to dated section on commit
		}

		return output

	case "git_log":
		var a tools.GitLogArgs
		json.Unmarshal([]byte(args), &a)
		count := a.Count
		if count <= 0 {
			count = 10
		}
		result := c.exec.GitLog(count)
		// Output already streamed by executor
		return result.String()

	case "list_files":
		var a tools.ListFilesArgs
		json.Unmarshal([]byte(args), &a)
		result := c.exec.ListFiles(a.Pattern)
		// Output already streamed by executor
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

		if !c.confirmTool("set_version", fmt.Sprintf("Set version to %s?", a.Version)) {
			return "OPERATION FAILED: User declined to set version. Version was NOT changed."
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

	if !c.confirmTool("write_file", fmt.Sprintf("Write %s to %s (%d bytes)?", fileType, path, len(content))) {
		return fmt.Sprintf("OPERATION FAILED: User declined to write %s. The file was NOT created or modified.", fileType)
	}

	if err := c.exec.WriteFile(path, content); err != nil {
		fmt.Printf("\033[31mFailed to write %s: %v\033[0m\n", fileType, err)
		return fmt.Sprintf("Failed to write %s: %v", fileType, err)
	}
	fmt.Printf("\033[32m✓ Wrote %s (%d bytes)\033[0m\n", path, len(content))

	// Log to changelog and history
	desc := fmt.Sprintf("Modified %s", filepath.Base(path))
	c.changelog.AddEntry("Changed", desc, []string{path})
	c.history.AddChange(desc, []string{path})

	return fmt.Sprintf("Successfully wrote %d bytes to %s", len(content), path)
}

// confirmTool asks for permission to execute a tool with options:
// y = yes (once), n = no, a = always allow this tool
// Returns true if the tool should be executed
func (c *Chat) confirmTool(toolName, prompt string) bool {
	// Check if autoExec is enabled
	if c.autoExec {
		return true
	}

	// Check saved permission for this tool
	perm := c.cfg.GetToolPermission(toolName)
	switch perm {
	case config.PermissionAlways:
		fmt.Printf("\033[32m✓ Auto-approved: %s (permission: always)\033[0m\n", toolName)
		return true
	case config.PermissionNever:
		fmt.Printf("\033[31m✗ Auto-denied: %s (permission: never)\033[0m\n", toolName)
		return false
	}

	// In non-interactive mode, decline
	if c.rl == nil {
		fmt.Printf("\033[33m%s\033[0m\n", prompt)
		fmt.Println("\033[31m✗ Declined (non-interactive mode, use -auto flag)\033[0m")
		return false
	}

	// Show the prompt with options
	fmt.Println() // Ensure we're on a new line
	fmt.Printf("\033[33m╭─ %s\033[0m\n", prompt)
	fmt.Printf("\033[33m│ (y)es once, (n)o, (a)lways allow %s, (!) never allow\033[0m\n", toolName)
	fmt.Printf("\033[33m╰─▶ \033[0m")
	os.Stdout.Sync() // Flush output before reading

	line, err := c.rl.Readline()
	if err != nil {
		fmt.Println("\033[31m✗ Declined (read error)\033[0m")
		return false
	}

	line = strings.ToLower(strings.TrimSpace(line))

	switch line {
	case "y", "yes":
		fmt.Println("\033[32m✓ Approved\033[0m")
		return true
	case "a", "always":
		fmt.Printf("\033[32m✓ Approved (saving 'always' for %s)\033[0m\n", toolName)
		c.cfg.SetToolPermission(toolName, config.PermissionAlways)
		if err := c.cfg.Save(); err != nil {
			fmt.Printf("\033[33mWarning: could not save config: %v\033[0m\n", err)
		}
		return true
	case "!", "never":
		fmt.Printf("\033[31m✗ Denied (saving 'never' for %s)\033[0m\n", toolName)
		c.cfg.SetToolPermission(toolName, config.PermissionNever)
		if err := c.cfg.Save(); err != nil {
			fmt.Printf("\033[33mWarning: could not save config: %v\033[0m\n", err)
		}
		return false
	default:
		fmt.Println("\033[31m✗ Declined\033[0m")
		return false
	}
}

// confirm is a simple yes/no confirmation (for backward compatibility)
func (c *Chat) confirm(prompt string) bool {
	return c.confirmTool("general", prompt)
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
  /permissions     View/manage tool permissions
  /todos           View/manage persistent todos
  /changelog       View/add changelog entries
  /history [n]     View recent project history
  /search <query>  Search the web
  /screenshot      Capture a screenshot
  /sessions        List recorded sessions
  /playback <file> Replay a session
  /config          Show current configuration
  /models          List available models
  /model [name]    Show or switch current model

Tool Permissions:
  When a tool wants to execute, you'll be prompted with options:
    (y)es     - approve once
    (n)o      - decline once
    (a)lways  - always approve this tool type
    (!)       - never allow this tool type

  Use /permissions to view and manage saved permissions.

Project Files (in project root):
  TODOS.md     - Persistent todo list (survives across sessions)
  CHANGELOG.md - Track changes made during sessions
  HISTORY.md   - Complete activity log (requests, todos, changes, commits)

The AI can:
  - Execute shell commands (builds, tests, etc.)
  - Create/modify source code and documentation
  - Search the web and fetch pages
  - Capture screenshots
  - Run git operations with auto-versioning
  - Read project files

All sessions are automatically saved in .aicli/ for playback.
Version is auto-bumped on each commit (x.y.z format).
Pending todos are detected on startup - resume work where you left off!
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

// extractErrorSummary extracts a concise error description from command output
func extractErrorSummary(output, command string) string {
	lines := strings.Split(output, "\n")

	// Look for common error patterns
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Go errors
		if strings.Contains(line, "undefined:") ||
			strings.Contains(line, "cannot find") ||
			strings.Contains(line, "no required module") ||
			strings.Contains(line, "missing go.sum entry") {
			return line
		}

		// Build errors
		if strings.Contains(line, "error:") ||
			strings.Contains(line, "Error:") ||
			strings.Contains(line, "FAILED") ||
			strings.Contains(line, "fatal:") {
			return line
		}

		// Python errors
		if strings.Contains(line, "ModuleNotFoundError") ||
			strings.Contains(line, "ImportError") ||
			strings.Contains(line, "SyntaxError") {
			return line
		}

		// Node errors
		if strings.Contains(line, "Cannot find module") ||
			strings.Contains(line, "ERR!") {
			return line
		}

		// Command not found
		if strings.Contains(line, "command not found") ||
			strings.Contains(line, "not found") {
			return line
		}
	}

	// Check for build-specific commands
	if strings.Contains(command, "go build") {
		return "Go build failed - check for compilation errors above"
	}
	if strings.Contains(command, "npm") || strings.Contains(command, "yarn") {
		return "Node package operation failed"
	}
	if strings.Contains(command, "pip") {
		return "Python package operation failed"
	}
	if strings.Contains(command, "cargo") {
		return "Rust build failed"
	}

	// Default: first non-empty line or generic message
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && len(line) < 200 {
			return line
		}
	}

	return "Command exited with non-zero status"
}

// getFixCommand returns a specific command to run to fix the error
// Returns the command and a boolean indicating if it's a concrete command (vs template)
func getFixCommand(output string) (string, bool) {
	// Go-specific fixes
	if strings.Contains(output, "go.mod file not found") {
		return "go mod init myproject", true
	}
	if strings.Contains(output, "missing go.sum entry") {
		return "go mod tidy", true
	}
	if strings.Contains(output, "no required module provides") {
		return "go mod tidy", true
	}

	// Python fixes - these need the module name filled in
	if strings.Contains(output, "ModuleNotFoundError") || strings.Contains(output, "No module named") {
		return "pip install <missing-module>", false
	}

	// Node fixes
	if strings.Contains(output, "Cannot find module") {
		return "npm install", true
	}

	return "", false
}

// isUnfixableByRerun returns true if the error indicates the command itself is wrong
// and re-running it won't help (e.g., wrong package name, version doesn't exist)
func isUnfixableByRerun(output string) bool {
	unfixablePatterns := []string{
		"no matching versions",       // Go: package version doesn't exist
		"404 Not Found",              // HTTP: resource doesn't exist
		"could not read Username",    // Git: auth issue with bad URL
		"invalid version",            // Go: malformed version
		"unknown revision",           // Go: bad version/tag
		"malformed module path",      // Go: invalid import path
		"unrecognized import path",   // Go: package doesn't exist at all
		"already exists",             // File/resource already exists
		"file exists",                // Alternative "already exists" message
	}
	for _, pattern := range unfixablePatterns {
		if strings.Contains(output, pattern) {
			return true
		}
	}
	return false
}
