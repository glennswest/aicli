package session

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type HistoryEntry struct {
	Timestamp   time.Time
	Type        string // "request", "todo", "change", "commit"
	Description string
	Details     string // Additional details (e.g., file list, todo status)
}

type HistoryFile struct {
	projectDir string
	filePath   string
	entries    []HistoryEntry
}

// NewHistoryFile creates or loads a HISTORY.md file in the project root
func NewHistoryFile(projectDir string) *HistoryFile {
	filePath := filepath.Join(projectDir, "HISTORY.md")
	hf := &HistoryFile{
		projectDir: projectDir,
		filePath:   filePath,
		entries:    make([]HistoryEntry, 0),
	}
	hf.Load()
	return hf
}

// AddRequest adds a user request to the history
func (hf *HistoryFile) AddRequest(request string) {
	hf.entries = append(hf.entries, HistoryEntry{
		Timestamp:   time.Now(),
		Type:        "request",
		Description: request,
	})
	hf.Save()
}

// AddTodo adds a todo event to the history
func (hf *HistoryFile) AddTodo(action, status string) {
	hf.entries = append(hf.entries, HistoryEntry{
		Timestamp:   time.Now(),
		Type:        "todo",
		Description: action,
		Details:     status,
	})
	hf.Save()
}

// AddChange adds a file change to the history
func (hf *HistoryFile) AddChange(description string, files []string) {
	details := ""
	if len(files) > 0 {
		details = strings.Join(files, ", ")
	}
	hf.entries = append(hf.entries, HistoryEntry{
		Timestamp:   time.Now(),
		Type:        "change",
		Description: description,
		Details:     details,
	})
	hf.Save()
}

// AddCommit adds a git commit to the history
func (hf *HistoryFile) AddCommit(message, hash string) {
	hf.entries = append(hf.entries, HistoryEntry{
		Timestamp:   time.Now(),
		Type:        "commit",
		Description: message,
		Details:     hash,
	})
	hf.Save()
}

// GetRecent returns the most recent n entries
func (hf *HistoryFile) GetRecent(n int) []HistoryEntry {
	if n > len(hf.entries) {
		n = len(hf.entries)
	}
	// Return from end (most recent)
	start := len(hf.entries) - n
	if start < 0 {
		start = 0
	}
	result := make([]HistoryEntry, n)
	copy(result, hf.entries[start:])
	// Reverse for newest first
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return result
}

// Save writes the history to HISTORY.md
func (hf *HistoryFile) Save() error {
	var sb strings.Builder
	sb.WriteString("# Project History\n\n")
	sb.WriteString("Activity log for this project.\n\n")

	if len(hf.entries) == 0 {
		return nil // Don't create empty file
	}

	// Group by date
	dateGroups := make(map[string][]HistoryEntry)
	var dates []string

	for _, entry := range hf.entries {
		dateStr := entry.Timestamp.Format("2006-01-02")
		if _, exists := dateGroups[dateStr]; !exists {
			dates = append(dates, dateStr)
		}
		dateGroups[dateStr] = append(dateGroups[dateStr], entry)
	}

	// Reverse dates to show newest first
	for i, j := 0, len(dates)-1; i < j; i, j = i+1, j-1 {
		dates[i], dates[j] = dates[j], dates[i]
	}

	for _, date := range dates {
		sb.WriteString(fmt.Sprintf("## %s\n\n", date))

		entries := dateGroups[date]
		// Reverse entries within date for newest first
		for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
			entries[i], entries[j] = entries[j], entries[i]
		}

		for _, entry := range entries {
			timeStr := entry.Timestamp.Format("15:04")
			icon := getIcon(entry.Type)

			switch entry.Type {
			case "request":
				sb.WriteString(fmt.Sprintf("### %s %s Request\n\n", icon, timeStr))
				sb.WriteString(fmt.Sprintf("> %s\n\n", entry.Description))
			case "todo":
				sb.WriteString(fmt.Sprintf("- %s `%s` **%s** - %s\n", icon, timeStr, entry.Details, entry.Description))
			case "change":
				if entry.Details != "" {
					sb.WriteString(fmt.Sprintf("- %s `%s` %s *(files: %s)*\n", icon, timeStr, entry.Description, entry.Details))
				} else {
					sb.WriteString(fmt.Sprintf("- %s `%s` %s\n", icon, timeStr, entry.Description))
				}
			case "commit":
				if entry.Details != "" {
					sb.WriteString(fmt.Sprintf("- %s `%s` **Commit** `%s`: %s\n", icon, timeStr, entry.Details[:7], entry.Description))
				} else {
					sb.WriteString(fmt.Sprintf("- %s `%s` **Commit**: %s\n", icon, timeStr, entry.Description))
				}
			}
		}
		sb.WriteString("\n")
	}

	return os.WriteFile(hf.filePath, []byte(sb.String()), 0644)
}

// Load reads the history from HISTORY.md
func (hf *HistoryFile) Load() error {
	file, err := os.Open(hf.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	// For now, we don't parse history back in - it's primarily an output file
	// The session JSON files contain the authoritative record
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		// Just consume the file to validate it's readable
		_ = scanner.Text()
	}

	return scanner.Err()
}

// FilePath returns the path to the HISTORY.md file
func (hf *HistoryFile) FilePath() string {
	return hf.filePath
}

// Clear removes all history entries
func (hf *HistoryFile) Clear() {
	hf.entries = make([]HistoryEntry, 0)
	os.Remove(hf.filePath)
}

func getIcon(entryType string) string {
	switch entryType {
	case "request":
		return ">"
	case "todo":
		return "-"
	case "change":
		return "*"
	case "commit":
		return "#"
	default:
		return "-"
	}
}
