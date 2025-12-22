package executor

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Result struct {
	Command  string
	Output   string
	Error    string
	ExitCode int
	Duration time.Duration
}

type Executor struct {
	workDir string
	timeout time.Duration
}

func New(workDir string) *Executor {
	if workDir == "" {
		workDir, _ = os.Getwd()
	}
	return &Executor{
		workDir: workDir,
		timeout: 60 * time.Second,
	}
}

func (e *Executor) SetWorkDir(dir string) {
	e.workDir = dir
}

func (e *Executor) WorkDir() string {
	return e.workDir
}

func (e *Executor) Run(command string) *Result {
	start := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = e.workDir

	// Inherit environment and add common tool paths
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, e.getExtendedPath())

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := &Result{
		Command:  command,
		Output:   stdout.String(),
		Error:    stderr.String(),
		Duration: time.Since(start),
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
			result.Error = err.Error()
		}
	}

	return result
}

func (e *Executor) WriteFile(path, content string) error {
	fullPath := path
	if !filepath.IsAbs(path) {
		fullPath = filepath.Join(e.workDir, path)
	}

	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	return os.WriteFile(fullPath, []byte(content), 0644)
}

func (e *Executor) ReadFile(path string) (string, error) {
	fullPath := path
	if !filepath.IsAbs(path) {
		fullPath = filepath.Join(e.workDir, path)
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func (e *Executor) GitStatus() *Result {
	return e.Run("git status --porcelain")
}

func (e *Executor) GitDiff(staged bool) *Result {
	if staged {
		return e.Run("git diff --cached")
	}
	return e.Run("git diff")
}

func (e *Executor) GitAdd(files ...string) *Result {
	if len(files) == 0 {
		return e.Run("git add -A")
	}
	return e.Run("git add " + strings.Join(files, " "))
}

func (e *Executor) GitCommit(message string) *Result {
	// Escape message for shell
	message = strings.ReplaceAll(message, "'", "'\"'\"'")
	return e.Run(fmt.Sprintf("git commit -m '%s'", message))
}

func (e *Executor) GitLog(count int) *Result {
	return e.Run(fmt.Sprintf("git log --oneline -n %d", count))
}

func (e *Executor) GitBranch() *Result {
	return e.Run("git branch --show-current")
}

func (e *Executor) ListFiles(pattern string) *Result {
	if pattern == "" {
		pattern = "."
	}
	return e.Run(fmt.Sprintf("find %s -type f -name '*.go' -o -name '*.py' -o -name '*.js' -o -name '*.ts' -o -name '*.rs' -o -name '*.c' -o -name '*.cpp' -o -name '*.h' 2>/dev/null | head -50", pattern))
}

// ScreenCapture captures the screen or a window
func (e *Executor) ScreenCapture(outputPath string, interactive bool) *Result {
	if outputPath == "" {
		outputPath = filepath.Join(e.workDir, fmt.Sprintf("screenshot_%d.png", time.Now().Unix()))
	} else if !filepath.IsAbs(outputPath) {
		outputPath = filepath.Join(e.workDir, outputPath)
	}

	var cmd string
	if interactive {
		// Interactive mode - user selects area
		cmd = fmt.Sprintf("screencapture -i '%s'", outputPath)
	} else {
		// Capture entire screen
		cmd = fmt.Sprintf("screencapture -x '%s'", outputPath)
	}

	result := e.Run(cmd)
	if result.Success() {
		result.Output = fmt.Sprintf("Screenshot saved to: %s", outputPath)
	}
	return result
}

// Version management
type Version struct {
	Major int
	Minor int
	Patch int
}

func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

func ParseVersion(s string) Version {
	var v Version
	fmt.Sscanf(strings.TrimSpace(s), "%d.%d.%d", &v.Major, &v.Minor, &v.Patch)
	return v
}

func (e *Executor) GetVersion() (Version, error) {
	versionFile := filepath.Join(e.workDir, "VERSION")
	content, err := os.ReadFile(versionFile)
	if err != nil {
		if os.IsNotExist(err) {
			return Version{0, 1, 0}, nil
		}
		return Version{}, err
	}
	return ParseVersion(string(content)), nil
}

func (e *Executor) SetVersion(v Version) error {
	versionFile := filepath.Join(e.workDir, "VERSION")
	return os.WriteFile(versionFile, []byte(v.String()+"\n"), 0644)
}

func (e *Executor) BumpVersion(bumpType string) (Version, error) {
	v, err := e.GetVersion()
	if err != nil {
		return v, err
	}

	switch bumpType {
	case "major":
		v.Major++
		v.Minor = 0
		v.Patch = 0
	case "minor":
		v.Minor++
		v.Patch = 0
	case "patch", "":
		v.Patch++
	}

	return v, e.SetVersion(v)
}

// InitVersion creates VERSION file if it doesn't exist
func (e *Executor) InitVersion() error {
	versionFile := filepath.Join(e.workDir, "VERSION")
	if _, err := os.Stat(versionFile); os.IsNotExist(err) {
		return e.SetVersion(Version{0, 1, 0})
	}
	return nil
}

// GitCommitWithVersion bumps version and includes it in commit
func (e *Executor) GitCommitWithVersion(message string, bumpType string) *Result {
	v, err := e.BumpVersion(bumpType)
	if err != nil {
		return &Result{Error: fmt.Sprintf("Failed to bump version: %v", err), ExitCode: 1}
	}

	// Stage VERSION file
	e.GitAdd("VERSION")

	// Include version in commit message
	fullMessage := fmt.Sprintf("%s (v%s)", message, v.String())
	fullMessage = strings.ReplaceAll(fullMessage, "'", "'\"'\"'")

	return e.Run(fmt.Sprintf("git commit -m '%s'", fullMessage))
}

func (r *Result) String() string {
	var sb strings.Builder
	if r.Output != "" {
		sb.WriteString(r.Output)
	}
	if r.Error != "" {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("stderr: ")
		sb.WriteString(r.Error)
	}
	return sb.String()
}

func (r *Result) Success() bool {
	return r.ExitCode == 0
}

// getExtendedPath returns a PATH environment variable with common tool paths added
func (e *Executor) getExtendedPath() string {
	currentPath := os.Getenv("PATH")
	// Common paths for Go, Rust, Node, Python, etc.
	additionalPaths := []string{
		"/usr/local/go/bin",
		"/usr/local/bin",
		"/opt/go/bin",
		os.ExpandEnv("$HOME/go/bin"),
		os.ExpandEnv("$HOME/.local/bin"),
		os.ExpandEnv("$HOME/.cargo/bin"),
		"/snap/bin",
	}

	// Build new PATH with additional paths prepended
	var pathParts []string
	for _, p := range additionalPaths {
		if _, err := os.Stat(p); err == nil {
			pathParts = append(pathParts, p)
		}
	}
	pathParts = append(pathParts, currentPath)

	return "PATH=" + strings.Join(pathParts, ":")
}
