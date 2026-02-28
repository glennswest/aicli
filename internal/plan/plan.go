package plan

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ModelTier represents the complexity level for a task
type ModelTier string

const (
	TierPremium  ModelTier = "premium"  // Best reasoning model - architecture, complex planning
	TierStandard ModelTier = "standard" // Good coding model - implementation tasks
	TierEconomy  ModelTier = "economy"  // Fast/cheap model - docs, formatting, simple tasks
)

// Step represents a single step in the implementation plan
type Step struct {
	ID          int        `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Status      string     `json:"status"` // pending, in_progress, completed, failed
	ModelTier   ModelTier  `json:"model_tier"`
	Files       []string   `json:"files,omitempty"`
	Result      string     `json:"result,omitempty"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// Plan represents an implementation plan with ordered steps
type Plan struct {
	Goal      string    `json:"goal"`
	Analysis  string    `json:"analysis"`
	Steps     []Step    `json:"steps"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// PlanResponse is the expected JSON structure from the planning model
type PlanResponse struct {
	Analysis string `json:"analysis"`
	Steps    []struct {
		Title       string   `json:"title"`
		Description string   `json:"description"`
		Files       []string `json:"files,omitempty"`
		ModelTier   string   `json:"model_tier"`
	} `json:"steps"`
}

// New creates a new plan with the given goal
func New(goal, analysis string) *Plan {
	now := time.Now()
	return &Plan{
		Goal:      goal,
		Analysis:  analysis,
		Steps:     make([]Step, 0),
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// AddStep appends a step to the plan
func (p *Plan) AddStep(title, description string, tier ModelTier, files []string) {
	p.Steps = append(p.Steps, Step{
		ID:          len(p.Steps) + 1,
		Title:       title,
		Description: description,
		Status:      "pending",
		ModelTier:   tier,
		Files:       files,
	})
	p.UpdatedAt = time.Now()
}

// NextPending returns the first pending step, or nil if none
func (p *Plan) NextPending() *Step {
	for i := range p.Steps {
		if p.Steps[i].Status == "pending" {
			return &p.Steps[i]
		}
	}
	return nil
}

// GetStep returns a step by ID
func (p *Plan) GetStep(id int) *Step {
	for i := range p.Steps {
		if p.Steps[i].ID == id {
			return &p.Steps[i]
		}
	}
	return nil
}

// MarkInProgress marks a step as in-progress
func (p *Plan) MarkInProgress(id int) {
	for i := range p.Steps {
		if p.Steps[i].ID == id {
			p.Steps[i].Status = "in_progress"
			now := time.Now()
			p.Steps[i].StartedAt = &now
			p.UpdatedAt = now
			return
		}
	}
}

// MarkCompleted marks a step as completed with a result summary
func (p *Plan) MarkCompleted(id int, result string) {
	for i := range p.Steps {
		if p.Steps[i].ID == id {
			p.Steps[i].Status = "completed"
			p.Steps[i].Result = result
			now := time.Now()
			p.Steps[i].CompletedAt = &now
			p.UpdatedAt = now
			return
		}
	}
}

// MarkFailed marks a step as failed with an error description
func (p *Plan) MarkFailed(id int, result string) {
	for i := range p.Steps {
		if p.Steps[i].ID == id {
			p.Steps[i].Status = "failed"
			p.Steps[i].Result = result
			now := time.Now()
			p.Steps[i].CompletedAt = &now
			p.UpdatedAt = now
			return
		}
	}
}

// Progress returns plan completion stats
func (p *Plan) Progress() (total, completed, failed, inProgress, pending int) {
	total = len(p.Steps)
	for _, s := range p.Steps {
		switch s.Status {
		case "completed":
			completed++
		case "failed":
			failed++
		case "in_progress":
			inProgress++
		default:
			pending++
		}
	}
	return
}

// IsComplete returns true if all steps are completed or failed
func (p *Plan) IsComplete() bool {
	for _, s := range p.Steps {
		if s.Status == "pending" || s.Status == "in_progress" {
			return false
		}
	}
	return len(p.Steps) > 0
}

// Save writes the plan to .aicli/plan.json and plan.md in the work directory
func (p *Plan) Save(workDir string) error {
	// Save JSON for reliable loading
	jsonPath := filepath.Join(workDir, ".aicli", "plan.json")
	os.MkdirAll(filepath.Dir(jsonPath), 0755)

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal plan: %w", err)
	}
	if err := os.WriteFile(jsonPath, data, 0644); err != nil {
		return fmt.Errorf("write plan.json: %w", err)
	}

	// Save human-readable markdown
	mdPath := filepath.Join(workDir, "plan.md")
	if err := os.WriteFile(mdPath, []byte(p.RenderMarkdown()), 0644); err != nil {
		return fmt.Errorf("write plan.md: %w", err)
	}

	return nil
}

// Load reads a plan from .aicli/plan.json
func Load(workDir string) (*Plan, error) {
	jsonPath := filepath.Join(workDir, ".aicli", "plan.json")
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, err
	}

	var p Plan
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parse plan.json: %w", err)
	}
	return &p, nil
}

// Exists checks if a plan file exists in the work directory
func Exists(workDir string) bool {
	jsonPath := filepath.Join(workDir, ".aicli", "plan.json")
	_, err := os.Stat(jsonPath)
	return err == nil
}

// Remove deletes the plan files from the work directory
func Remove(workDir string) error {
	os.Remove(filepath.Join(workDir, ".aicli", "plan.json"))
	os.Remove(filepath.Join(workDir, "plan.md"))
	return nil
}

// RenderMarkdown generates plan.md content
func (p *Plan) RenderMarkdown() string {
	var sb strings.Builder

	sb.WriteString("# Implementation Plan\n\n")

	sb.WriteString("## Goal\n\n")
	sb.WriteString(p.Goal + "\n\n")

	sb.WriteString("## Analysis\n\n")
	sb.WriteString(p.Analysis + "\n\n")

	sb.WriteString("## Steps\n\n")

	for _, step := range p.Steps {
		var statusIcon string
		switch step.Status {
		case "completed":
			statusIcon = "[x]"
		case "in_progress":
			statusIcon = "[>]"
		case "failed":
			statusIcon = "[!]"
		default:
			statusIcon = "[ ]"
		}

		var tierLabel string
		switch step.ModelTier {
		case TierPremium:
			tierLabel = "PREMIUM"
		case TierStandard:
			tierLabel = "STANDARD"
		case TierEconomy:
			tierLabel = "ECONOMY"
		default:
			tierLabel = "STANDARD"
		}

		sb.WriteString(fmt.Sprintf("### %s Step %d: %s\n\n", statusIcon, step.ID, step.Title))
		sb.WriteString(fmt.Sprintf("- **Status**: %s\n", step.Status))
		sb.WriteString(fmt.Sprintf("- **Model Tier**: %s\n", tierLabel))
		if len(step.Files) > 0 {
			sb.WriteString(fmt.Sprintf("- **Files**: %s\n", strings.Join(step.Files, ", ")))
		}
		sb.WriteString(fmt.Sprintf("\n%s\n\n", step.Description))

		if step.Result != "" {
			sb.WriteString(fmt.Sprintf("> %s\n\n", step.Result))
		}
	}

	// Progress summary
	total, completed, failed, inProgress, pending := p.Progress()
	sb.WriteString("---\n\n")
	sb.WriteString("## Progress\n\n")
	sb.WriteString(fmt.Sprintf("| Total | Completed | Failed | In Progress | Pending |\n"))
	sb.WriteString(fmt.Sprintf("|-------|-----------|--------|-------------|--------|\n"))
	sb.WriteString(fmt.Sprintf("| %d | %d | %d | %d | %d |\n", total, completed, failed, inProgress, pending))

	sb.WriteString(fmt.Sprintf("\n*Updated: %s*\n", p.UpdatedAt.Format("2006-01-02 15:04:05")))

	return sb.String()
}

// ParsePlanResponse extracts a PlanResponse from AI model output
func ParsePlanResponse(content string) (*PlanResponse, error) {
	jsonStr := content

	// Strip markdown code blocks if present
	if idx := strings.Index(jsonStr, "```json"); idx >= 0 {
		jsonStr = jsonStr[idx+7:]
		if endIdx := strings.Index(jsonStr, "```"); endIdx >= 0 {
			jsonStr = jsonStr[:endIdx]
		}
	} else if idx := strings.Index(jsonStr, "```"); idx >= 0 {
		jsonStr = jsonStr[idx+3:]
		if endIdx := strings.Index(jsonStr, "```"); endIdx >= 0 {
			jsonStr = jsonStr[:endIdx]
		}
	}

	jsonStr = strings.TrimSpace(jsonStr)

	// If it doesn't start with {, try to find the JSON object
	if !strings.HasPrefix(jsonStr, "{") {
		start := strings.Index(content, "{")
		end := strings.LastIndex(content, "}")
		if start >= 0 && end > start {
			jsonStr = content[start : end+1]
		}
	}

	var resp PlanResponse
	if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
		// Truncate for error message
		preview := jsonStr
		if len(preview) > 300 {
			preview = preview[:300] + "..."
		}
		return nil, fmt.Errorf("failed to parse plan JSON: %w\nContent: %s", err, preview)
	}

	if len(resp.Steps) == 0 {
		return nil, fmt.Errorf("plan has no steps")
	}

	return &resp, nil
}

// BuildFromResponse creates a Plan from a PlanResponse
func BuildFromResponse(goal string, resp *PlanResponse) *Plan {
	p := New(goal, resp.Analysis)

	for _, s := range resp.Steps {
		tier := TierStandard
		switch strings.ToLower(s.ModelTier) {
		case "premium":
			tier = TierPremium
		case "economy":
			tier = TierEconomy
		}
		p.AddStep(s.Title, s.Description, tier, s.Files)
	}

	return p
}

// GetPlanningSystemPrompt returns the system prompt for the planning model
func GetPlanningSystemPrompt() string {
	return `You are a senior software architect. Your job is to analyze a project and create a concrete implementation plan.

Given a project's file structure, key source files, and a goal, you must:
1. Analyze the current state of the project
2. Break the goal into concrete, ordered implementation steps
3. Assign a model tier to each step based on complexity
4. Identify which files each step will create or modify

Output ONLY valid JSON with this exact structure (no other text):
{
  "analysis": "Brief analysis of the project and what needs to change",
  "steps": [
    {
      "title": "Short title for this step",
      "description": "Detailed instructions: what to do, what code to write, what patterns to follow. Be specific enough that a coding model can execute this without further clarification.",
      "files": ["path/to/file.go"],
      "model_tier": "standard"
    }
  ]
}

Model tiers (pick the cheapest that handles the task):
- "premium": Complex architecture, multi-file refactors, API design requiring deep reasoning
- "standard": Normal coding, feature implementation, tests, bug fixes
- "economy": Documentation, config changes, boilerplate, formatting

Rules:
- Each step must be independently executable in order
- Steps must be ordered by dependency
- Be very specific in descriptions - include function signatures, struct fields, patterns
- One logical change per step
- Output ONLY the JSON object, nothing else`
}

// BuildPlanningPrompt constructs the full prompt for the planning model
func BuildPlanningPrompt(goal, fileList, fileContents string) string {
	var sb strings.Builder

	sb.WriteString("## Project Structure\n\n")
	sb.WriteString("```\n")
	sb.WriteString(fileList)
	sb.WriteString("\n```\n\n")

	if fileContents != "" {
		sb.WriteString("## Key Files\n\n")
		sb.WriteString(fileContents)
		sb.WriteString("\n\n")
	}

	sb.WriteString("## Goal\n\n")
	sb.WriteString(goal)
	sb.WriteString("\n\nCreate a detailed implementation plan as JSON.")

	return sb.String()
}

// GetStepExecutionPrompt builds the prompt for executing a single plan step
func GetStepExecutionPrompt(step *Step, planGoal, planAnalysis string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("You are implementing step %d of an implementation plan.\n\n", step.ID))
	sb.WriteString(fmt.Sprintf("## Overall Goal\n%s\n\n", planGoal))
	sb.WriteString(fmt.Sprintf("## Project Context\n%s\n\n", planAnalysis))
	sb.WriteString(fmt.Sprintf("## Current Step: %s\n\n", step.Title))
	sb.WriteString(step.Description)
	sb.WriteString("\n\n")

	if len(step.Files) > 0 {
		sb.WriteString(fmt.Sprintf("Files to work with: %s\n\n", strings.Join(step.Files, ", ")))
	}

	sb.WriteString("Execute this step now using the available tools. Read existing files first if modifying them. Do not explain what you will do - just do it.")

	return sb.String()
}
