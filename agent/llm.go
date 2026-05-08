package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"strings"
	"time"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// LLMClient is the minimal interface ProcessWithTools needs against an
// OpenAI-compatible API. It's wrapped in a struct so callers can
// configure the timeout and base URL once.
type LLMClient struct {
	APIKey string
	APIURL string
	Model  string
	HTTP   *http.Client
}

// NewLLMClient returns a client with sensible default timeouts.
func NewLLMClient(apiKey, apiURL, model string) *LLMClient {
	return &LLMClient{
		APIKey: apiKey,
		APIURL: apiURL,
		Model:  model,
		HTTP:   &http.Client{Timeout: 120 * time.Second},
	}
}

// CallLLM sends a chat completion request and returns the streaming
// response. Caller must Close the response body.
func (c *LLMClient) CallLLM(ctx context.Context, history []Message, tools []ToolDef, sysPrompt string) (*http.Response, error) {
	body := map[string]interface{}{
		"model":      c.Model,
		"stream":     true,
		"max_tokens": 8192,
	}
	if tools != nil {
		body["tools"] = tools
	}

	msgs := make([]interface{}, 0, len(history)+1)
	msgs = append(msgs, map[string]string{"role": "system", "content": sysPrompt})
	for _, m := range history {
		msgs = append(msgs, map[string]string{"role": m.Role, "content": m.Content})
	}
	body["messages"] = msgs

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.APIURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("api status %d: %s", resp.StatusCode, string(respBody))
	}
	return resp, nil
}

// ProcessWithTools runs a chat turn with tool-call iteration. Streams
// tokens to w as they arrive. Returns the final assistant content
// after tool calls have settled. Caller must persist the returned
// content if it should appear in history.
func (a *App) ProcessWithTools(ctx context.Context, c *LLMClient, history []Message, sysPrompt string, w http.ResponseWriter, flusher http.Flusher) (string, error) {
	tools := GetTools()

	for i := 0; i < 10; i++ {
		resp, err := c.CallLLM(ctx, history, tools, sysPrompt)
		if err != nil {
			return "", fmt.Errorf("call llm: %w", err)
		}

		toolCalls, content, err := ParseStream(resp.Body, w, flusher)
		resp.Body.Close()
		if err != nil {
			return "", err
		}

		if len(toolCalls) == 0 {
			return content, nil
		}

		var toolResults []Message
		toolResults = append(toolResults, Message{Role: "assistant", Content: content})

		for _, tc := range toolCalls {
			result := a.ExecuteTool(tc.Name, tc.Args)
			toolResults = append(toolResults, Message{
				Role:    "tool",
				Content: result,
			})
		}

		history = append(history, toolResults...)
	}

	return "", fmt.Errorf("too many tool call iterations")
}

func ParseStream(body io.Reader, w http.ResponseWriter, flusher http.Flusher) ([]ToolCall, string, error) {
	var toolCalls []ToolCall
	var fullContent strings.Builder
	var fullReasoning strings.Builder
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content          *string `json:"content"`
					ReasoningContent *string `json:"reasoning_content"`
					ToolCalls        []struct {
						Index    int `json:"index"`
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
				FinishReason *string `json:"finish_reason"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		c := chunk.Choices[0]

		if c.Delta.ReasoningContent != nil && *c.Delta.ReasoningContent != "" {
			fullReasoning.WriteString(*c.Delta.ReasoningContent)
			displayToken := html.EscapeString(*c.Delta.ReasoningContent)
			displayToken = strings.ReplaceAll(displayToken, "\n", "<br>")
			fmt.Fprintf(w, "event: reasoning\ndata: %s\n\n", jsonEscape(displayToken))
			flusher.Flush()
		}

		if c.Delta.Content != nil && *c.Delta.Content != "" {
			fullContent.WriteString(*c.Delta.Content)
			displayToken := html.EscapeString(*c.Delta.Content)
			displayToken = strings.ReplaceAll(displayToken, "\n", "<br>")
			fmt.Fprintf(w, "event: token\ndata: %s\n\n", jsonEscape(displayToken))
			flusher.Flush()
		}

		if c.Delta.ToolCalls != nil {
			for _, tc := range c.Delta.ToolCalls {
				for len(toolCalls) <= tc.Index {
					toolCalls = append(toolCalls, ToolCall{})
				}
				if tc.Function.Name != "" {
					toolCalls[tc.Index].Name = tc.Function.Name
				}
				if tc.Function.Arguments != "" {
					toolCalls[tc.Index].Args = append(toolCalls[tc.Index].Args, []byte(tc.Function.Arguments)...)
				}
			}
		}

		if c.FinishReason != nil && *c.FinishReason == "tool_calls" {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return toolCalls, fullContent.String(), fmt.Errorf("stream error: %w", err)
	}

	return toolCalls, fullContent.String(), nil
}

func jsonEscape(s string) string {
	data, _ := json.Marshal(s)
	return string(data)
}

// CallLLMNonStreaming sends a chat completion request and returns the
// concatenated assistant message text.
func (c *LLMClient) CallLLMNonStreaming(ctx context.Context, messages []Message) (string, error) {
	body := map[string]interface{}{
		"model":      c.Model,
		"stream":     false,
		"max_tokens": 8192,
	}
	msgs := make([]interface{}, 0, len(messages))
	for _, m := range messages {
		msgs = append(msgs, map[string]string{"role": m.Role, "content": m.Content})
	}
	body["messages"] = msgs

	payload, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.APIURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	// Use a shorter timeout for non-streaming summary calls.
	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("api status %d: %s", resp.StatusCode, string(respBody))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no response from LLM")
	}
	return result.Choices[0].Message.Content, nil
}

const summaryPrompt = `Summarize this study session conversation in 3-5 concise bullet points. Focus on:
- What topics were discussed
- Key concepts or insights learned
- Any decisions or next steps mentioned
- Questions still open

Be specific and concise. Do not include pleasantries.`

// GenerateSummary asks the LLM to summarize the last 30 messages.
func (c *LLMClient) GenerateSummary(ctx context.Context, history []Message) (string, error) {
	msgs := []Message{{Role: "system", Content: summaryPrompt}}
	start := 0
	if len(history) > 30 {
		start = len(history) - 30
	}
	msgs = append(msgs, history[start:]...)
	return c.CallLLMNonStreaming(ctx, msgs)
}
