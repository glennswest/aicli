package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"aicli/internal/chat"
	"aicli/internal/client"
	"aicli/internal/config"
	"aicli/internal/discovery"
	"aicli/internal/executor"
	"aicli/internal/session"
	"aicli/internal/update"
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
	insecure     bool
	checkUpdate  bool
	debugMode    bool
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
	flag.BoolVar(&insecure, "insecure", false, "Skip TLS certificate verification")
	flag.BoolVar(&checkUpdate, "update", false, "Check for updates and install if available")
	flag.BoolVar(&debugMode, "debug", false, "Enable debug logging for discovery")
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

	// Apply insecure setting from config or command line flag
	if cfg.Insecure || insecure {
		discovery.InsecureSkipVerify = true
		client.InsecureSkipVerify = true
	}

	// Handle --version early (no Ollama needed)
	if showVersion {
		workDir, _ := os.Getwd()
		exec := executor.New(workDir)
		fmt.Printf("aicli version: %s\n", version)
		v, _ := exec.GetVersion()
		fmt.Printf("Project version: %s\n", v.String())
		return
	}

	// Handle --update early (no Ollama needed)
	if checkUpdate {
		handleUpdate()
		return
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

	// Set debug mode for discovery
	if debugMode {
		discovery.Debug = true
	}

	// Auto-discover Ollama if no config was loaded and endpoint wasn't overridden
	if cfg.LoadedFrom() == "" && endpoint == "" {
		autoDiscoverEndpoint(cfg)
	}

	// Warn if using unencrypted connection (except for localhost)
	warnIfUnencrypted(cfg.APIEndpoint)

	// Auto-configure model if needed (handles "default" or model mismatch)
	autoConfigModel(cfg)

	workDir, _ := os.Getwd()
	exec := executor.New(workDir)

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
		if cfg.PreloadModel {
			ensureModelLoaded(cfg)
		}
		runSinglePrompt(cfg, prompt)
		return
	}

	// Check for piped input
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		if cfg.PreloadModel {
			ensureModelLoaded(cfg)
		}
		runPipedInput(cfg)
		return
	}

	// Interactive chat mode
	if cfg.PreloadModel {
		ensureModelLoaded(cfg)
	}
	checkUpdateOnStartup()
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

// autoDiscoverEndpoint attempts to find an Ollama instance via mDNS
// if no local Ollama is available
func autoDiscoverEndpoint(cfg *config.Config) {
	// Check if local Ollama is available first
	if discovery.CheckLocalOllama() {
		return
	}

	fmt.Print("\033[33m🔍 No local Ollama found, searching network...\033[0m")

	endpoint, host, useTLS, needsInsecure := discovery.AutoDiscover()
	if endpoint == "" {
		fmt.Printf("\r\033[K\033[31m✗ No Ollama instances found on network\033[0m\n")
		return
	}

	cfg.APIEndpoint = endpoint
	protoIcon := "🔓"
	if useTLS {
		protoIcon = "🔒"
	}
	fmt.Printf("\r\033[K\033[32m✓ Discovered Ollama at %s %s\033[0m\n", host, protoIcon)

	// If the endpoint uses a self-signed certificate, enable insecure mode
	if needsInsecure {
		cfg.Insecure = true
		discovery.InsecureSkipVerify = true
		client.InsecureSkipVerify = true
		fmt.Printf("\033[33m⚠ Warning: Using self-signed certificate (TLS verification disabled)\033[0m\n")
	}

	// Save the discovered endpoint to local config
	if err := cfg.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to save config: %v\n", err)
	}
}

// warnIfUnencrypted warns if the endpoint uses HTTP instead of HTTPS
// Localhost connections are exempt from the warning
func warnIfUnencrypted(endpoint string) {
	// Skip warning for localhost
	if strings.Contains(endpoint, "localhost") || strings.Contains(endpoint, "127.0.0.1") {
		return
	}

	if !discovery.IsEncrypted(endpoint) {
		fmt.Printf("\033[33m⚠ Warning: Connection is not encrypted (using HTTP)\033[0m\n")
		fmt.Printf("\033[33m  Data sent to %s may be visible on the network\033[0m\n", endpoint)
	}
}

// checkUpdateOnStartup silently checks for updates and notifies user if available
func checkUpdateOnStartup() {
	info, err := update.CheckForUpdate(version)
	if err != nil {
		return // Silently fail - don't interrupt startup
	}

	if update.IsNewerVersion(info.CurrentVersion, info.LatestVersion) {
		fmt.Printf("\033[33m⬆ Update available: %s → %s (run with --update to install)\033[0m\n\n", info.CurrentVersion, info.LatestVersion)
	}
}

// handleUpdate checks for updates and prompts user to install
func handleUpdate() {
	fmt.Printf("Checking for updates...\n")

	info, err := update.CheckForUpdate(version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\033[31m✗ Failed to check for updates: %v\033[0m\n", err)
		os.Exit(1)
	}

	if !update.IsNewerVersion(info.CurrentVersion, info.LatestVersion) {
		fmt.Printf("\033[32m✓ You are running the latest version (%s)\033[0m\n", version)
		return
	}

	fmt.Printf("\n\033[33m⬆ Update available!\033[0m\n")
	fmt.Printf("  Current version: %s\n", info.CurrentVersion)
	fmt.Printf("  Latest version:  %s\n", info.LatestVersion)
	fmt.Printf("  Download size:   %.2f MB\n", float64(info.AssetSize)/(1024*1024))

	if info.ReleaseNotes != "" {
		fmt.Printf("\nRelease notes:\n")
		// Show first few lines of release notes
		lines := strings.Split(info.ReleaseNotes, "\n")
		for i, line := range lines {
			if i >= 10 {
				fmt.Printf("  ...\n")
				break
			}
			fmt.Printf("  %s\n", line)
		}
	}

	fmt.Printf("\nDo you want to update? [y/N]: ")
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
		os.Exit(1)
	}

	response = strings.TrimSpace(strings.ToLower(response))
	if response != "y" && response != "yes" {
		fmt.Println("Update cancelled.")
		return
	}

	fmt.Printf("\nDownloading update...")

	err = update.DownloadAndInstall(info, func(downloaded, total int64) {
		pct := float64(downloaded) / float64(total) * 100
		fmt.Printf("\rDownloading update... %.1f%%", pct)
	})

	if err != nil {
		fmt.Printf("\n\033[31m✗ Update failed: %v\033[0m\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n\033[32m✓ Successfully updated to version %s\033[0m\n", info.LatestVersion)
	fmt.Println("Please restart aicli to use the new version.")
}
