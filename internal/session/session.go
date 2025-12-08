package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
