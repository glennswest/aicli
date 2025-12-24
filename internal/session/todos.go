package session

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type TodoItem struct {
	Content   string
	Status    string // "pending", "in_progress", "completed"
	CreatedAt time.Time
}

type TodoFile struct {
	projectDir string
	filePath   string
	items      []TodoItem
}

// NewTodoFile creates or loads a TODOS.md file in the project root
func NewTodoFile(projectDir string) *TodoFile {
	filePath := filepath.Join(projectDir, "TODOS.md")
	tf := &TodoFile{
		projectDir: projectDir,
		filePath:   filePath,
		items:      make([]TodoItem, 0),
	}
	tf.Load()
	return tf
}

// AddTodo adds a new pending todo item
func (tf *TodoFile) AddTodo(content string) {
	// Don't add duplicates
	for _, item := range tf.items {
		if item.Content == content && item.Status != "completed" {
			return
		}
	}
	tf.items = append([]TodoItem{{
		Content:   content,
		Status:    "pending",
		CreatedAt: time.Now(),
	}}, tf.items...)
	tf.Save()
}

// SetInProgress marks a todo as in progress by index
func (tf *TodoFile) SetInProgress(index int) {
	if index >= 0 && index < len(tf.items) {
		tf.items[index].Status = "in_progress"
		tf.Save()
	}
}

// Complete marks a todo as completed by index
func (tf *TodoFile) Complete(index int) {
	if index >= 0 && index < len(tf.items) {
		tf.items[index].Status = "completed"
		tf.Save()
	}
}

// CompleteByContent marks the first matching todo as completed
func (tf *TodoFile) CompleteByContent(substr string) {
	for i, item := range tf.items {
		if strings.Contains(item.Content, substr) && item.Status != "completed" {
			tf.items[i].Status = "completed"
			tf.Save()
			return
		}
	}
}

// Remove removes a todo by index
func (tf *TodoFile) Remove(index int) {
	if index >= 0 && index < len(tf.items) {
		tf.items = append(tf.items[:index], tf.items[index+1:]...)
		tf.Save()
	}
}

// RemoveByContent removes todos containing the given substring
func (tf *TodoFile) RemoveByContent(substr string) {
	filtered := tf.items[:0]
	for _, item := range tf.items {
		if !strings.Contains(item.Content, substr) {
			filtered = append(filtered, item)
		}
	}
	tf.items = filtered
	tf.Save()
}

// GetPending returns all pending and in_progress items
func (tf *TodoFile) GetPending() []TodoItem {
	var pending []TodoItem
	for _, item := range tf.items {
		if item.Status == "pending" || item.Status == "in_progress" {
			pending = append(pending, item)
		}
	}
	return pending
}

// GetAll returns all items
func (tf *TodoFile) GetAll() []TodoItem {
	return tf.items
}

// Clear removes all todos
func (tf *TodoFile) Clear() {
	tf.items = make([]TodoItem, 0)
	tf.Save()
}

// ClearCompleted removes only completed todos
func (tf *TodoFile) ClearCompleted() {
	filtered := tf.items[:0]
	for _, item := range tf.items {
		if item.Status != "completed" {
			filtered = append(filtered, item)
		}
	}
	tf.items = filtered
	tf.Save()
}

// PopFirst removes and returns the first pending/in_progress item
func (tf *TodoFile) PopFirst() string {
	for i, item := range tf.items {
		if item.Status == "pending" || item.Status == "in_progress" {
			tf.items[i].Status = "completed"
			tf.Save()
			return item.Content
		}
	}
	return ""
}

// Save writes the todos to TODOS.md in GitHub-flavored markdown
func (tf *TodoFile) Save() error {
	var sb strings.Builder
	sb.WriteString("# AICLI Todos\n\n")

	// Group by status
	var inProgress, pending, completed []TodoItem
	for _, item := range tf.items {
		switch item.Status {
		case "in_progress":
			inProgress = append(inProgress, item)
		case "pending":
			pending = append(pending, item)
		case "completed":
			completed = append(completed, item)
		}
	}

	// Write In Progress section
	if len(inProgress) > 0 {
		sb.WriteString("## In Progress\n\n")
		for _, item := range inProgress {
			sb.WriteString(fmt.Sprintf("- [ ] %s\n", item.Content))
		}
		sb.WriteString("\n")
	}

	// Write Pending section
	if len(pending) > 0 {
		sb.WriteString("## Pending\n\n")
		for _, item := range pending {
			sb.WriteString(fmt.Sprintf("- [ ] %s\n", item.Content))
		}
		sb.WriteString("\n")
	}

	// Write Completed section
	if len(completed) > 0 {
		sb.WriteString("## Completed\n\n")
		for _, item := range completed {
			dateStr := item.CreatedAt.Format("2006-01-02")
			sb.WriteString(fmt.Sprintf("- [x] %s *(completed %s)*\n", item.Content, dateStr))
		}
		sb.WriteString("\n")
	}

	// Only write file if there are todos
	if len(tf.items) == 0 {
		// Remove file if no todos
		os.Remove(tf.filePath)
		return nil
	}

	return os.WriteFile(tf.filePath, []byte(sb.String()), 0644)
}

// Load reads todos from TODOS.md
func (tf *TodoFile) Load() error {
	file, err := os.Open(tf.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No file yet, that's OK
		}
		return err
	}
	defer file.Close()

	tf.items = make([]TodoItem, 0)
	scanner := bufio.NewScanner(file)

	// Regex to match todo items: - [ ] content or - [x] content
	todoRegex := regexp.MustCompile(`^-\s+\[([ x])\]\s+(.+?)(?:\s*\*\(completed [\d-]+\)\*)?$`)

	currentSection := ""
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Detect section headers
		if strings.HasPrefix(line, "## In Progress") {
			currentSection = "in_progress"
			continue
		} else if strings.HasPrefix(line, "## Pending") {
			currentSection = "pending"
			continue
		} else if strings.HasPrefix(line, "## Completed") {
			currentSection = "completed"
			continue
		}

		// Parse todo items
		if matches := todoRegex.FindStringSubmatch(line); matches != nil {
			checkbox := matches[1]
			content := matches[2]

			status := currentSection
			if status == "" {
				// Fallback to checkbox state
				if checkbox == "x" {
					status = "completed"
				} else {
					status = "pending"
				}
			}

			tf.items = append(tf.items, TodoItem{
				Content:   content,
				Status:    status,
				CreatedAt: time.Now(), // We don't persist creation time in markdown
			})
		}
	}

	return scanner.Err()
}

// FilePath returns the path to the TODOS.md file
func (tf *TodoFile) FilePath() string {
	return tf.filePath
}

// HasPending returns true if there are pending todos
func (tf *TodoFile) HasPending() bool {
	return len(tf.GetPending()) > 0
}
