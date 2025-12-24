package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Entry struct {
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"` // "user", "assistant", "tool_call", "tool_result"
	Content   string    `json:"content"`
	ToolName  string    `json:"tool_name,omitempty"`
	ToolArgs  string    `json:"tool_args,omitempty"`
}

type Session struct {
	ProjectDir string    `json:"project_dir"`
	StartTime  time.Time `json:"start_time"`
	Entries    []Entry   `json:"entries"`
}

type Recorder struct {
	session    *Session
	sessionDir string
	filePath   string
}

func NewRecorder(projectDir string) *Recorder {
	sessionDir := filepath.Join(projectDir, ".aicli")
	os.MkdirAll(sessionDir, 0755)

	// Create session file with timestamp
	fileName := fmt.Sprintf("session_%s.json", time.Now().Format("20060102_150405"))
	filePath := filepath.Join(sessionDir, fileName)

	return &Recorder{
		session: &Session{
			ProjectDir: projectDir,
			StartTime:  time.Now(),
			Entries:    make([]Entry, 0),
		},
		sessionDir: sessionDir,
		filePath:   filePath,
	}
}

func (r *Recorder) RecordUser(content string) {
	r.session.Entries = append(r.session.Entries, Entry{
		Timestamp: time.Now(),
		Type:      "user",
		Content:   content,
	})
	r.save()
}

func (r *Recorder) RecordAssistant(content string) {
	r.session.Entries = append(r.session.Entries, Entry{
		Timestamp: time.Now(),
		Type:      "assistant",
		Content:   content,
	})
	r.save()
}

func (r *Recorder) RecordToolCall(name, args string) {
	r.session.Entries = append(r.session.Entries, Entry{
		Timestamp: time.Now(),
		Type:      "tool_call",
		ToolName:  name,
		ToolArgs:  args,
	})
	r.save()
}

func (r *Recorder) RecordToolResult(name, result string) {
	r.session.Entries = append(r.session.Entries, Entry{
		Timestamp: time.Now(),
		Type:      "tool_result",
		ToolName:  name,
		Content:   result,
	})
	r.save()
}

func (r *Recorder) save() error {
	data, err := json.MarshalIndent(r.session, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(r.filePath, data, 0644)
}

func (r *Recorder) SessionPath() string {
	return r.filePath
}

// ListSessions returns all session files for a project
func ListSessions(projectDir string) ([]string, error) {
	sessionDir := filepath.Join(projectDir, ".aicli")
	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var sessions []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			sessions = append(sessions, filepath.Join(sessionDir, e.Name()))
		}
	}
	return sessions, nil
}

// LoadSession loads a session file
func LoadSession(path string) (*Session, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}
	return &session, nil
}

// Playback represents a session ready for replay
type Playback struct {
	session *Session
	index   int
}

func NewPlayback(path string) (*Playback, error) {
	session, err := LoadSession(path)
	if err != nil {
		return nil, err
	}
	return &Playback{session: session, index: 0}, nil
}

func (p *Playback) Next() (*Entry, bool) {
	if p.index >= len(p.session.Entries) {
		return nil, false
	}
	entry := &p.session.Entries[p.index]
	p.index++
	return entry, true
}

func (p *Playback) Reset() {
	p.index = 0
}

func (p *Playback) Total() int {
	return len(p.session.Entries)
}

func (p *Playback) Current() int {
	return p.index
}

// GetUserInputs returns only user input entries for replay
func (p *Playback) GetUserInputs() []string {
	var inputs []string
	for _, e := range p.session.Entries {
		if e.Type == "user" {
			inputs = append(inputs, e.Content)
		}
	}
	return inputs
}

// GetLatestSession returns the most recent session file for a project
func GetLatestSession(projectDir string) (string, error) {
	sessions, err := ListSessions(projectDir)
	if err != nil || len(sessions) == 0 {
		return "", err
	}
	// Sessions are named with timestamps, so the last one alphabetically is newest
	latest := sessions[0]
	for _, s := range sessions[1:] {
		if s > latest {
			latest = s
		}
	}
	return latest, nil
}

// IsSessionIncomplete checks if the last session ended with the model
// narrating an action but not executing it (suggesting interrupted work)
func IsSessionIncomplete(session *Session) bool {
	if len(session.Entries) == 0 {
		return false
	}

	// Find the last assistant message
	var lastAssistant *Entry
	for i := len(session.Entries) - 1; i >= 0; i-- {
		if session.Entries[i].Type == "assistant" {
			lastAssistant = &session.Entries[i]
			break
		}
	}

	if lastAssistant == nil {
		return false
	}

	// Check if it contains intent phrases suggesting incomplete work
	content := strings.ToLower(lastAssistant.Content)
	intentPhrases := []string{
		"let's create", "let's write", "let's add", "let's update",
		"let me create", "let me write", "let me add", "let me update",
		"here's the code", "here is the code", "with this content",
		"i'll create", "i'll write", "i will create", "i will write",
	}

	for _, phrase := range intentPhrases {
		if strings.Contains(content, phrase) {
			return true
		}
	}

	// Check for markdown code blocks (model showed code instead of writing)
	if strings.Contains(lastAssistant.Content, "```") {
		return true
	}

	return false
}

// GetEntries returns all entries in the session
func (s *Session) GetEntries() []Entry {
	return s.Entries
}
