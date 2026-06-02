package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Message struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	Reasoning string `json:"reasoning,omitempty"`
}

// LLMClient is the minimal interface for talking to an
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

const titlePrompt = `You generate a concise title for a study chat session, given the user's opening message. Reply with ONLY the title: 3 to 7 words, no surrounding quotes, no trailing punctuation, no preamble.`

// cleanTitle normalizes a raw model title: trims, strips wrapping quotes,
// collapses internal whitespace, drops a trailing period, and caps length.
func cleanTitle(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Join(strings.Fields(s), " ") // collapse whitespace/newlines
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			s = strings.TrimSpace(s[1 : len(s)-1])
		}
	}
	s = strings.TrimRight(s, ".")
	s = strings.TrimSpace(s)
	const maxRunes = 60
	runes := []rune(s)
	if len(runes) > maxRunes {
		s = string(runes[:maxRunes]) + "…"
	}
	return s
}

// GenerateTitle produces a short session title from the opening user message.
func (c *LLMClient) GenerateTitle(ctx context.Context, firstMessage string) (string, error) {
	msgs := []Message{
		{Role: "system", Content: titlePrompt},
		{Role: "user", Content: firstMessage},
	}
	raw, err := c.CallLLMNonStreaming(ctx, msgs)
	if err != nil {
		return "", err
	}
	return cleanTitle(raw), nil
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
