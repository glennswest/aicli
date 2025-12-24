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

type ChangelogEntry struct {
	Timestamp   time.Time
	Type        string // "Added", "Changed", "Fixed", "Removed"
	Description string
	Files       []string
}

type ChangelogFile struct {
	projectDir string
	filePath   string
	unreleased map[string][]ChangelogEntry // Type -> entries
	released   []ReleasedSection
}

type ReleasedSection struct {
	Date    string
	Entries map[string][]ChangelogEntry
}

// NewChangelogFile creates or loads a CHANGELOG.md file in the project root
func NewChangelogFile(projectDir string) *ChangelogFile {
	filePath := filepath.Join(projectDir, "CHANGELOG.md")
	cf := &ChangelogFile{
		projectDir: projectDir,
		filePath:   filePath,
		unreleased: make(map[string][]ChangelogEntry),
		released:   make([]ReleasedSection, 0),
	}
	cf.Load()
	return cf
}

// AddEntry adds a new changelog entry to the Unreleased section
func (cf *ChangelogFile) AddEntry(entryType, description string, files []string) {
	// Normalize entry type
	entryType = normalizeEntryType(entryType)

	entry := ChangelogEntry{
		Timestamp:   time.Now(),
		Type:        entryType,
		Description: description,
		Files:       files,
	}

	if cf.unreleased[entryType] == nil {
		cf.unreleased[entryType] = make([]ChangelogEntry, 0)
	}
	cf.unreleased[entryType] = append(cf.unreleased[entryType], entry)
	cf.Save()
}

// Release moves all unreleased entries to a new dated section
func (cf *ChangelogFile) Release(version string) {
	if len(cf.unreleased) == 0 {
		return
	}

	dateStr := time.Now().Format("2006-01-02")
	title := dateStr
	if version != "" {
		title = fmt.Sprintf("[%s] - %s", version, dateStr)
	}

	section := ReleasedSection{
		Date:    title,
		Entries: cf.unreleased,
	}

	cf.released = append([]ReleasedSection{section}, cf.released...)
	cf.unreleased = make(map[string][]ChangelogEntry)
	cf.Save()
}

// GetRecent returns the most recent n entries across all types
func (cf *ChangelogFile) GetRecent(n int) []ChangelogEntry {
	var all []ChangelogEntry

	// Collect from unreleased
	for _, entries := range cf.unreleased {
		all = append(all, entries...)
	}

	// Collect from released sections
	for _, section := range cf.released {
		for _, entries := range section.Entries {
			all = append(all, entries...)
		}
	}

	// Sort by timestamp (newest first) - simple bubble sort for small list
	for i := 0; i < len(all); i++ {
		for j := i + 1; j < len(all); j++ {
			if all[j].Timestamp.After(all[i].Timestamp) {
				all[i], all[j] = all[j], all[i]
			}
		}
	}

	if n > len(all) {
		n = len(all)
	}
	return all[:n]
}

// Save writes the changelog to CHANGELOG.md
func (cf *ChangelogFile) Save() error {
	var sb strings.Builder
	sb.WriteString("# Changelog\n\n")
	sb.WriteString("All notable changes to this project will be documented in this file.\n\n")

	// Write Unreleased section
	if len(cf.unreleased) > 0 {
		sb.WriteString("## [Unreleased]\n\n")
		writeEntrySection(&sb, cf.unreleased)
	}

	// Write released sections
	for i, section := range cf.released {
		if i == 0 && len(cf.unreleased) > 0 {
			sb.WriteString("---\n\n")
		}
		sb.WriteString(fmt.Sprintf("## %s\n\n", section.Date))
		writeEntrySection(&sb, section.Entries)
		if i < len(cf.released)-1 {
			sb.WriteString("---\n\n")
		}
	}

	// Only write file if there are entries
	if len(cf.unreleased) == 0 && len(cf.released) == 0 {
		return nil // Don't create empty changelog
	}

	return os.WriteFile(cf.filePath, []byte(sb.String()), 0644)
}

func writeEntrySection(sb *strings.Builder, entries map[string][]ChangelogEntry) {
	// Write in consistent order: Added, Changed, Fixed, Removed
	order := []string{"Added", "Changed", "Fixed", "Removed"}

	for _, entryType := range order {
		if items, ok := entries[entryType]; ok && len(items) > 0 {
			sb.WriteString(fmt.Sprintf("### %s\n\n", entryType))
			for _, item := range items {
				if len(item.Files) > 0 {
					sb.WriteString(fmt.Sprintf("- %s *(files: %s)*\n", item.Description, strings.Join(item.Files, ", ")))
				} else {
					sb.WriteString(fmt.Sprintf("- %s\n", item.Description))
				}
			}
			sb.WriteString("\n")
		}
	}
}

// Load reads the changelog from CHANGELOG.md
func (cf *ChangelogFile) Load() error {
	file, err := os.Open(cf.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No file yet
		}
		return err
	}
	defer file.Close()

	cf.unreleased = make(map[string][]ChangelogEntry)
	cf.released = make([]ReleasedSection, 0)

	scanner := bufio.NewScanner(file)

	// Regex patterns
	sectionRegex := regexp.MustCompile(`^##\s+(.+)$`)
	typeRegex := regexp.MustCompile(`^###\s+(Added|Changed|Fixed|Removed)$`)
	entryRegex := regexp.MustCompile(`^-\s+(.+?)(?:\s*\*\(files:\s*(.+?)\)\*)?$`)

	var currentSection string
	var currentType string
	var currentReleased *ReleasedSection

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Check for section header (## ...)
		if matches := sectionRegex.FindStringSubmatch(line); matches != nil {
			sectionTitle := matches[1]
			if sectionTitle == "[Unreleased]" {
				currentSection = "unreleased"
				currentReleased = nil
			} else {
				currentSection = "released"
				currentReleased = &ReleasedSection{
					Date:    sectionTitle,
					Entries: make(map[string][]ChangelogEntry),
				}
				cf.released = append(cf.released, *currentReleased)
			}
			currentType = ""
			continue
		}

		// Check for type header (### Added, etc)
		if matches := typeRegex.FindStringSubmatch(line); matches != nil {
			currentType = matches[1]
			continue
		}

		// Check for entry (- description)
		if matches := entryRegex.FindStringSubmatch(line); matches != nil && currentType != "" {
			description := matches[1]
			var files []string
			if matches[2] != "" {
				files = strings.Split(matches[2], ", ")
			}

			entry := ChangelogEntry{
				Timestamp:   time.Now(),
				Type:        currentType,
				Description: description,
				Files:       files,
			}

			if currentSection == "unreleased" {
				if cf.unreleased[currentType] == nil {
					cf.unreleased[currentType] = make([]ChangelogEntry, 0)
				}
				cf.unreleased[currentType] = append(cf.unreleased[currentType], entry)
			} else if currentReleased != nil {
				idx := len(cf.released) - 1
				if cf.released[idx].Entries[currentType] == nil {
					cf.released[idx].Entries[currentType] = make([]ChangelogEntry, 0)
				}
				cf.released[idx].Entries[currentType] = append(cf.released[idx].Entries[currentType], entry)
			}
		}
	}

	return scanner.Err()
}

// FilePath returns the path to the CHANGELOG.md file
func (cf *ChangelogFile) FilePath() string {
	return cf.filePath
}

// normalizeEntryType ensures entry type is capitalized correctly
func normalizeEntryType(t string) string {
	t = strings.ToLower(strings.TrimSpace(t))
	switch t {
	case "added", "add", "new":
		return "Added"
	case "changed", "change", "update", "updated":
		return "Changed"
	case "fixed", "fix", "bugfix":
		return "Fixed"
	case "removed", "remove", "delete", "deleted":
		return "Removed"
	default:
		return "Changed"
	}
}
