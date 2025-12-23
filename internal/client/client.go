package client

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"aicli/internal/config"
	"aicli/internal/lang"
	"aicli/internal/tools"
)

// TextToolCall represents a tool call parsed from text output
type TextToolCall struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// extractJSON extracts a complete JSON object from a string starting at the given position
// It properly handles nested braces
func extractJSON(s string, start int) (string, int) {
	if start >= len(s) || s[start] != '{' {
		return "", -1
	}

	depth := 0
	inString := false
	escaped := false

	for i := start; i < len(s); i++ {
		c := s[i]

		if escaped {
			escaped = false
			continue
		}

		if c == '\\' && inString {
			escaped = true
			continue
		}

		if c == '"' {
			inString = !inString
			continue
		}

		if inString {
			continue
		}

		if c == '{' {
			depth++
		} else if c == '}' {
			depth--
			if depth == 0 {
				return s[start : i+1], i + 1
			}
		}
	}

	return "", -1
}

// knownToolNames contains all valid tool names for raw format parsing
var knownToolNames = []string{
	"run_command", "write_file", "write_doc", "read_file",
	"web_search", "fetch_url", "screenshot",
	"git_status", "git_diff", "git_add", "git_commit", "git_log",
	"list_files", "get_version", "set_version",
}

// ParseToolCallsFromText extracts tool calls from text output
// Supports two formats:
// 1. <tool_call>{"name": "...", "arguments": {...}}</tool_call>
// 2. tool_name\n{"arg": "value"} (raw format from some models)
func ParseToolCallsFromText(text string) ([]tools.ToolCall, string) {
	var toolCalls []tools.ToolCall
	cleanedText := text
	callIndex := 0

	// First try <tool_call> tag format
	re := regexp.MustCompile(`(?s)<tool_call>\s*`)
	closeTag := "</tool_call>"

	for {
		loc := re.FindStringIndex(cleanedText)
		if loc == nil {
			break
		}

		jsonStart := loc[1]
		closeIdx := strings.Index(cleanedText[jsonStart:], closeTag)
		if closeIdx == -1 {
			break
		}

		content := strings.TrimSpace(cleanedText[jsonStart : jsonStart+closeIdx])
		jsonStr, _ := extractJSON(content, 0)
		if jsonStr == "" {
			cleanedText = cleanedText[:loc[0]] + cleanedText[jsonStart+closeIdx+len(closeTag):]
			continue
		}

		var tc TextToolCall
		if err := json.Unmarshal([]byte(jsonStr), &tc); err != nil {
			cleanedText = cleanedText[:loc[0]] + cleanedText[jsonStart+closeIdx+len(closeTag):]
			continue
		}

		toolCall := tools.ToolCall{
			ID:   fmt.Sprintf("text_call_%d", callIndex),
			Type: "function",
		}
		toolCall.Function.Name = tc.Name
		toolCall.Function.Arguments = string(tc.Arguments)

		toolCalls = append(toolCalls, toolCall)
		callIndex++
		cleanedText = cleanedText[:loc[0]] + cleanedText[jsonStart+closeIdx+len(closeTag):]
	}

	// Then try raw format: tool_name\n{JSON} or tool_name{JSON}
	for _, toolName := range knownToolNames {
		// Find tool_name followed by whitespace then {
		pattern := regexp.MustCompile(`(?s)` + regexp.QuoteMeta(toolName) + `\s*\{`)

		for {
			match := pattern.FindStringIndex(cleanedText)
			if match == nil {
				break
			}

			// Find the start of JSON (the { character)
			jsonStart := match[1] - 1 // -1 because the pattern includes {

			// Use extractJSON to properly handle nested braces and strings
			jsonStr, jsonEnd := extractJSON(cleanedText, jsonStart)
			if jsonStr == "" {
				// No valid JSON found, skip
				break
			}

			// Validate it's proper JSON
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(jsonStr), &args); err != nil {
				// Invalid JSON, skip this match
				break
			}

			toolCall := tools.ToolCall{
				ID:   fmt.Sprintf("text_call_%d", callIndex),
				Type: "function",
			}
			toolCall.Function.Name = toolName
			toolCall.Function.Arguments = jsonStr

			toolCalls = append(toolCalls, toolCall)
			callIndex++

			// Remove the matched text (tool_name + whitespace + JSON)
			cleanedText = cleanedText[:match[0]] + cleanedText[jsonEnd:]
		}
	}

	cleanedText = strings.TrimSpace(cleanedText)
	return toolCalls, cleanedText
}

type Message struct {
	Role       string           `json:"role"`
	Content    string           `json:"content,omitempty"`
	ToolCalls  []tools.ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []Message     `json:"messages"`
	Tools       []tools.Tool  `json:"tools,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
	Stream      bool          `json:"stream"`
}

type ChatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role      string           `json:"role"`
			Content   string           `json:"content"`
			ToolCalls []tools.ToolCall `json:"tool_calls,omitempty"`
		} `json:"message"`
		Delta struct {
			Content   string           `json:"content"`
			ToolCalls []tools.ToolCall `json:"tool_calls,omitempty"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

type ChatResult struct {
	Content      string
	ToolCalls    []tools.ToolCall
	FinishReason string
}

type Client struct {
	cfg        *config.Config
	httpClient *http.Client
	history    []Message
	useTools   bool
	debugDir   string
	workDir    string
	requestNum int
}

type ModelsResponse struct {
	Data []ModelInfo `json:"data"`
}

type ModelInfo struct {
	ID string `json:"id"`
}

func New(cfg *config.Config) *Client {
	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Transport: &http.Transport{
				ResponseHeaderTimeout: 60 * time.Second,
			},
		},
		history:  make([]Message, 0),
		useTools: true,
	}
}

// NewWithDebug creates a client with debug logging and language detection enabled
func NewWithDebug(cfg *config.Config, workDir string) *Client {
	debugDir := filepath.Join(workDir, ".aicli", "debug")
	os.MkdirAll(debugDir, 0755)

	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Transport: &http.Transport{
				ResponseHeaderTimeout: 60 * time.Second,
			},
		},
		history:  make([]Message, 0),
		useTools: true,
		debugDir: debugDir,
		workDir:  workDir,
	}
}

// SetDebugDir enables debug logging to the specified directory
func (c *Client) SetDebugDir(workDir string) {
	c.debugDir = filepath.Join(workDir, ".aicli", "debug")
	os.MkdirAll(c.debugDir, 0755)
}

// logDebug writes request/response data to debug files
func (c *Client) logDebug(prefix string, data []byte) {
	if c.debugDir == "" {
		return
	}

	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("%s_%03d_%s.json", timestamp, c.requestNum, prefix)
	filepath := filepath.Join(c.debugDir, filename)

	// Pretty-print JSON if possible
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, data, "", "  "); err == nil {
		data = prettyJSON.Bytes()
	}

	os.WriteFile(filepath, data, 0644)
}

func (c *Client) ListModels() ([]string, error) {
	endpoint := strings.TrimSuffix(c.cfg.APIEndpoint, "/") + "/models"
	httpReq, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.cfg.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var modelsResp ModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("failed to decode models response: %w", err)
	}

	models := make([]string, len(modelsResp.Data))
	for i, m := range modelsResp.Data {
		models[i] = m.ID
	}
	return models, nil
}

func (c *Client) SetUseTools(use bool) {
	c.useTools = use
}

func (c *Client) ClearHistory() {
	c.history = make([]Message, 0)
}

func (c *Client) AddSystemPrompt() {
	if c.cfg.SystemPrompt != "" && len(c.history) == 0 {
		prompt := c.cfg.SystemPrompt

		// Always add language-specific error handling rules
		if c.workDir != "" {
			langs := lang.DetectMultipleLanguages(c.workDir)
			rules := lang.GetErrorRules(langs) // Returns LangUnknown rules if no langs detected
			prompt += "\n\n" + rules
		}

		c.history = append(c.history, Message{
			Role:    "system",
			Content: prompt,
		})
	}
}

func (c *Client) Chat(userMessage string, stream bool, onToken func(string)) (*ChatResult, error) {
	c.AddSystemPrompt()

	c.history = append(c.history, Message{
		Role:    "user",
		Content: userMessage,
	})

	return c.sendRequest(stream, onToken)
}

func (c *Client) AddToolResult(toolCallID, result string) {
	c.history = append(c.history, Message{
		Role:       "tool",
		Content:    result,
		ToolCallID: toolCallID,
	})
}

func (c *Client) ContinueWithToolResults(stream bool, onToken func(string)) (*ChatResult, error) {
	return c.sendRequest(stream, onToken)
}

func (c *Client) sendRequest(stream bool, onToken func(string)) (*ChatResult, error) {
	c.requestNum++

	req := ChatRequest{
		Model:       c.cfg.Model,
		Messages:    c.history,
		MaxTokens:   c.cfg.MaxTokens,
		Temperature: c.cfg.Temperature,
		Stream:      stream,
	}

	if c.useTools {
		req.Tools = tools.GetTools()
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Log the request
	c.logDebug("request", body)

	endpoint := strings.TrimSuffix(c.cfg.APIEndpoint, "/") + "/chat/completions"
	httpReq, err := http.NewRequest("POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.cfg.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		c.logDebug("error", bodyBytes)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result *ChatResult

	if stream {
		result, err = c.handleStreamResponse(resp.Body, onToken)
		if err != nil {
			return nil, err
		}
	} else {
		var chatResp ChatResponse
		respBody, _ := io.ReadAll(resp.Body)
		c.logDebug("response", respBody)
		if err := json.Unmarshal(respBody, &chatResp); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		result = &ChatResult{}
		if len(chatResp.Choices) > 0 {
			choice := chatResp.Choices[0]
			result.Content = choice.Message.Content
			result.ToolCalls = choice.Message.ToolCalls
			result.FinishReason = choice.FinishReason
		}
	}

	// Log the final result (especially useful for streaming)
	if resultJSON, err := json.Marshal(result); err == nil {
		c.logDebug("result", resultJSON)
	}

	// Add assistant message to history
	msg := Message{
		Role:    "assistant",
		Content: result.Content,
	}
	if len(result.ToolCalls) > 0 {
		msg.ToolCalls = result.ToolCalls
	}
	c.history = append(c.history, msg)

	return result, nil
}

func (c *Client) handleStreamResponse(body io.Reader, onToken func(string)) (*ChatResult, error) {
	scanner := bufio.NewScanner(body)
	// Increase buffer size for large responses
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	result := &ChatResult{}
	var contentBuilder strings.Builder
	toolCallsMap := make(map[int]*tools.ToolCall)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk ChatResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if len(chunk.Choices) > 0 {
			choice := chunk.Choices[0]

			// Handle content
			if choice.Delta.Content != "" {
				contentBuilder.WriteString(choice.Delta.Content)
				if onToken != nil {
					onToken(choice.Delta.Content)
				}
			}

			// Handle tool calls (streamed incrementally)
			for _, tc := range choice.Delta.ToolCalls {
				idx := tc.Index // Use index for streaming tool calls
				if existing, ok := toolCallsMap[idx]; ok {
					// Append arguments to existing tool call
					existing.Function.Arguments += tc.Function.Arguments
					// Update ID if provided (some APIs send ID in first chunk only)
					if tc.ID != "" {
						existing.ID = tc.ID
					}
				} else {
					// New tool call - store a copy
					newTC := tc
					// Generate an ID if not provided
					if newTC.ID == "" {
						newTC.ID = fmt.Sprintf("call_%d", idx)
					}
					toolCallsMap[idx] = &newTC
				}
			}

			result.FinishReason = choice.FinishReason
		}
	}

	result.Content = contentBuilder.String()
	for _, tc := range toolCallsMap {
		result.ToolCalls = append(result.ToolCalls, *tc)
	}

	return result, scanner.Err()
}

func (c *Client) Complete(prompt string, stream bool, onToken func(string)) (string, error) {
	// Temporarily disable tools for simple completion
	origUseTools := c.useTools
	c.useTools = false
	defer func() { c.useTools = origUseTools }()

	messages := []Message{
		{Role: "user", Content: prompt},
	}

	if c.cfg.SystemPrompt != "" {
		messages = append([]Message{{Role: "system", Content: c.cfg.SystemPrompt}}, messages...)
	}

	req := ChatRequest{
		Model:       c.cfg.Model,
		Messages:    messages,
		MaxTokens:   c.cfg.MaxTokens,
		Temperature: c.cfg.Temperature,
		Stream:      stream,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	endpoint := strings.TrimSuffix(c.cfg.APIEndpoint, "/") + "/chat/completions"
	httpReq, err := http.NewRequest("POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.cfg.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(bodyBytes))
	}

	if stream {
		result, err := c.handleStreamResponse(resp.Body, onToken)
		if err != nil {
			return "", err
		}
		return result.Content, nil
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(chatResp.Choices) > 0 {
		return chatResp.Choices[0].Message.Content, nil
	}

	return "", nil
}
