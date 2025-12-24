package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"aicli/internal/chat"
	"aicli/internal/client"
	"aicli/internal/config"
	"aicli/internal/executor"
	"aicli/internal/session"
)

// Version is set at build time via ldflags
var version = "dev"

var (
	endpoint     string
	apiKey       string
	model        string
	maxTokens    int
	temperature  float64
	prompt       string
	fileArgs     []string
	showConfig   bool
	initConfig   bool
	playbackFile string
	listSessions bool
	showVersion  bool
	autoMode     bool
)

func init() {
	flag.StringVar(&endpoint, "endpoint", "", "API endpoint URL (default: http://localhost:8000/v1)")
	flag.StringVar(&endpoint, "e", "", "API endpoint URL (shorthand)")
	flag.StringVar(&apiKey, "key", "", "API key")
	flag.StringVar(&apiKey, "k", "", "API key (shorthand)")
	flag.StringVar(&model, "model", "", "Model name")
	flag.StringVar(&model, "m", "", "Model name (shorthand)")
	flag.IntVar(&maxTokens, "max-tokens", 0, "Maximum tokens in response")
	flag.Float64Var(&temperature, "temperature", 0, "Temperature (0.0-2.0)")
	flag.Float64Var(&temperature, "t", 0, "Temperature (shorthand)")
	flag.StringVar(&prompt, "prompt", "", "Single prompt (non-interactive mode)")
	flag.StringVar(&prompt, "p", "", "Single prompt (shorthand)")
	flag.BoolVar(&showConfig, "config", false, "Show current configuration")
	flag.BoolVar(&initConfig, "init", false, "Initialize config file and VERSION")
	flag.StringVar(&playbackFile, "playback", "", "Replay a session file")
	flag.BoolVar(&listSessions, "sessions", false, "List recorded sessions")
	flag.BoolVar(&showVersion, "version", false, "Show project version")
	flag.BoolVar(&showVersion, "v", false, "Show project version (shorthand)")
	flag.BoolVar(&autoMode, "auto", false, "Auto-execute mode (skip confirmations)")
}

func main() {
	flag.Parse()
	fileArgs = flag.Args()

	// Set the app version for other packages to use
	config.AppVersion = version

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Override config with flags
	if endpoint != "" {
		cfg.APIEndpoint = endpoint
	}
	if apiKey != "" {
		cfg.APIKey = apiKey
	}
	if model != "" {
		cfg.Model = model
	}
	if maxTokens > 0 {
		cfg.MaxTokens = maxTokens
	}
	if temperature > 0 {
		cfg.Temperature = temperature
	}

	// Auto-configure model if needed (handles "default" or model mismatch)
	autoConfigModel(cfg)

	workDir, _ := os.Getwd()
	exec := executor.New(workDir)

	// Handle --version
	if showVersion {
		fmt.Printf("aicli version: %s\n", version)
		v, _ := exec.GetVersion()
		fmt.Printf("Project version: %s\n", v.String())
		return
	}

	// Handle --init
	if initConfig {
		if err := cfg.Save(); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
			os.Exit(1)
		}
		path, _ := config.ConfigPath()
		fmt.Printf("Config saved to: %s\n", path)

		// Initialize VERSION file
		if err := exec.InitVersion(); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating VERSION: %v\n", err)
			os.Exit(1)
		}
		v, _ := exec.GetVersion()
		fmt.Printf("VERSION initialized: %s\n", v.String())
		return
	}

	// Handle --config
	if showConfig {
		path, _ := config.ConfigPath()
		v, _ := exec.GetVersion()
		fmt.Printf("Config file:  %s\n", path)
		fmt.Printf("Endpoint:     %s\n", cfg.APIEndpoint)
		fmt.Printf("Model:        %s\n", cfg.Model)
		fmt.Printf("Max Tokens:   %d\n", cfg.MaxTokens)
		fmt.Printf("Temperature:  %.2f\n", cfg.Temperature)
		fmt.Printf("Version:      %s\n", v.String())
		if cfg.APIKey != "" && len(cfg.APIKey) > 8 {
			fmt.Printf("API Key:      %s...%s\n", cfg.APIKey[:4], cfg.APIKey[len(cfg.APIKey)-4:])
		} else if cfg.APIKey != "" {
			fmt.Printf("API Key:      (set)\n")
		} else {
			fmt.Printf("API Key:      (not set)\n")
		}
		return
	}

	// Handle --sessions
	if listSessions {
		sessions, err := session.ListSessions(workDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if len(sessions) == 0 {
			fmt.Println("No sessions found in .aicli/")
			return
		}
		fmt.Println("Sessions:")
		for _, s := range sessions {
			fmt.Printf("  %s\n", filepath.Base(s))
		}
		return
	}

	// Handle --playback
	if playbackFile != "" {
		sessionPath := playbackFile
		if !filepath.IsAbs(sessionPath) {
			// Check if it's just a filename (look in .aicli/)
			if !strings.Contains(sessionPath, string(os.PathSeparator)) {
				sessionPath = filepath.Join(workDir, ".aicli", sessionPath)
			} else {
				sessionPath = filepath.Join(workDir, sessionPath)
			}
		}

		c, err := chat.NewPlaybackMode(cfg, sessionPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading session: %v\n", err)
			os.Exit(1)
		}

		if err := c.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Single prompt mode
	if prompt != "" {
		ensureModelLoaded(cfg)
		runSinglePrompt(cfg, prompt)
		return
	}

	// Check for piped input
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		ensureModelLoaded(cfg)
		runPipedInput(cfg)
		return
	}

	// Interactive chat mode
	ensureModelLoaded(cfg)
	runInteractive(cfg)
}

func runSinglePrompt(cfg *config.Config, prompt string) {
	// Add file context if provided
	if len(fileArgs) > 0 {
		var contextParts []string
		for _, path := range fileArgs {
			content, err := os.ReadFile(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", path, err)
				continue
			}
			contextParts = append(contextParts, fmt.Sprintf("File `%s`:\n```\n%s\n```", path, string(content)))
		}
		if len(contextParts) > 0 {
			prompt = strings.Join(contextParts, "\n\n") + "\n\n" + prompt
		}
	}

	// Use non-interactive Chat for tool support
	c, err := chat.NewNonInteractive(cfg, autoMode)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := c.RunSingle(prompt); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runPipedInput(cfg *config.Config) {
	var input strings.Builder
	buf := make([]byte, 4096)
	for {
		n, err := os.Stdin.Read(buf)
		if n > 0 {
			input.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}

	prompt := input.String()
	if prompt == "" {
		fmt.Fprintln(os.Stderr, "No input provided")
		os.Exit(1)
	}

	c := client.New(cfg)
	_, err := c.Complete(prompt, true, func(token string) {
		fmt.Print(token)
	})
	fmt.Println()

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runInteractive(cfg *config.Config) {
	c, err := chat.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error starting chat: %v\n", err)
		os.Exit(1)
	}

	if err := c.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func autoConfigModel(cfg *config.Config) {
	c := client.New(cfg)

	// First, try to get running models (preferred)
	runningModels, _ := c.ListRunningModels()

	// Then get all available models as fallback
	availableModels, err := c.ListModels()
	if err != nil {
		// Silently skip autoconfig if API is unavailable
		return
	}

	if cfg.AutoConfigModel(runningModels, availableModels) {
		fmt.Printf("Auto-configured model: %s\n", cfg.Model)
		// Save to local project config
		if err := cfg.Save(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to save config: %v\n", err)
		}
	}
}

// ensureModelLoaded checks if the model is running and loads it if not
func ensureModelLoaded(cfg *config.Config) {
	c := client.New(cfg)

	// Check if model is already running
	if c.IsModelRunning(cfg.Model) {
		fmt.Printf("\033[32m✓ Model %s is ready\033[0m\n", cfg.Model)
		return
	}

	// Model not running - need to load it
	fmt.Printf("\033[33m⏳ Loading model %s (this may take a moment)...\033[0m", cfg.Model)

	// Load the model with 24h keep-alive
	err := c.LoadModel(cfg.Model, "24h")
	if err != nil {
		fmt.Printf("\n\033[31m✗ Failed to load model: %v\033[0m\n", err)
		return
	}

	fmt.Printf("\r\033[K\033[32m✓ Model %s is ready\033[0m\n", cfg.Model)
}
