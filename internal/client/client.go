package client

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"aicli/internal/config"
	"aicli/internal/tools"
)

// TextToolCall represents a tool call parsed from text output
type TextToolCall struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// ParseToolCallsFromText extracts tool calls from <tool_call> tags in text
func ParseToolCallsFromText(text string) ([]tools.ToolCall, string) {
	re := regexp.MustCompile(`(?s)<tool_call>\s*(\{.*?\})\s*</tool_call>`)
	matches := re.FindAllStringSubmatch(text, -1)

	var toolCalls []tools.ToolCall
	cleanedText := text

	for i, match := range matches {
		if len(match) < 2 {
			continue
		}

		var tc TextToolCall
		if err := json.Unmarshal([]byte(match[1]), &tc); err != nil {
			continue
		}

		toolCall := tools.ToolCall{
			ID:   fmt.Sprintf("text_call_%d", i),
			Type: "function",
		}
		toolCall.Function.Name = tc.Name
		toolCall.Function.Arguments = string(tc.Arguments)

		toolCalls = append(toolCalls, toolCall)

		// Remove the tool call from displayed text
		cleanedText = strings.Replace(cleanedText, match[0], "", 1)
	}

	// Clean up extra whitespace
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
}

type ModelsResponse struct {
	Data []ModelInfo `json:"data"`
}

type ModelInfo struct {
	ID string `json:"id"`
}

func New(cfg *config.Config) *Client {
	return &Client{
		cfg:        cfg,
		httpClient: &http.Client{},
		history:    make([]Message, 0),
		useTools:   true,
	}
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
		c.history = append(c.history, Message{
			Role:    "system",
			Content: c.cfg.SystemPrompt,
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
		if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
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
				idx := tc.ID // Use ID as index for streaming
				if idx == "" {
					// Some APIs use index-based streaming
					continue
				}
				if existing, ok := toolCallsMap[0]; ok {
					existing.Function.Arguments += tc.Function.Arguments
				} else {
					newTC := tc
					toolCallsMap[0] = &newTC
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
