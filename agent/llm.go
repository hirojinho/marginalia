package agent

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

var LastAssistantContent string

func CallLLM(history []Message, tools []ToolDef, sysPrompt, model, apiKey, apiURL string) (*http.Response, error) {
	body := map[string]interface{}{
		"model":      model,
		"stream":     true,
		"max_tokens": 8192,
	}

	prompt := sysPrompt
	sysMsg := map[string]string{"role": "system", "content": prompt}
	if tools != nil {
		body["tools"] = tools
	}

	var msgs []interface{}
	msgs = append(msgs, sysMsg)
	for _, m := range history {
		msgs = append(msgs, map[string]string{"role": m.Role, "content": m.Content})
	}
	body["messages"] = msgs

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal error: %w", err)
	}

	req, err := http.NewRequest("POST", apiURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("request error: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("api error: %w", err)
	}

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("api status %d: %s", resp.StatusCode, string(respBody))
	}

	return resp, nil
}

func ProcessWithTools(history []Message, sysPrompt, model, apiKey, apiURL string, w http.ResponseWriter, flusher http.Flusher) error {
	tools := GetTools()

	for i := 0; i < 10; i++ {
		resp, err := CallLLM(history, tools, sysPrompt, model, apiKey, apiURL)
		if err != nil {
			return fmt.Errorf("api call failed: %w", err)
		}

		toolCalls, content, err := ParseStream(resp.Body, w, flusher)
		resp.Body.Close()
		if err != nil {
			return err
		}

		if len(toolCalls) == 0 {
			LastAssistantContent = content
			return nil
		}

		var toolResults []Message
		toolResults = append(toolResults, Message{Role: "assistant", Content: content})

		for _, tc := range toolCalls {
			result := ExecuteTool(tc.Name, tc.Args)
			toolResults = append(toolResults, Message{
				Role:    "tool",
				Content: result,
			})
			log.Printf("Tool %s: %d chars", tc.Name, len(result))
		}

		history = append(history, toolResults...)
	}

	return fmt.Errorf("too many tool call iterations")
}

func ParseStream(body io.Reader, w http.ResponseWriter, flusher http.Flusher) ([]ToolCall, string, error) {
	var toolCalls []ToolCall
	var fullContent strings.Builder
	var fullReasoning strings.Builder
	scanner := bufio.NewScanner(body)

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
					Content         *string `json:"content"`
					ReasoningContent *string `json:"reasoning_content"`
					ToolCalls []struct {
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

func CallLLMNonStreaming(messages []Message, model, apiKey, apiURL string) (string, error) {
	body := map[string]interface{}{
		"model":      model,
		"stream":     false,
		"max_tokens": 8192,
	}

	var msgs []interface{}
	for _, m := range messages {
		msgs = append(msgs, map[string]string{"role": m.Role, "content": m.Content})
	}
	body["messages"] = msgs

	payload, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal error: %w", err)
	}

	req, err := http.NewRequest("POST", apiURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("request error: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("api error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("api status %d: %s", resp.StatusCode, string(respBody))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read error: %w", err)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return "", fmt.Errorf("unmarshal error: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no response from LLM")
	}
	return result.Choices[0].Message.Content, nil
}

func GenerateSummary(history []Message, apiKey, apiURL, model string) (string, error) {
	prompt := `Summarize this study session conversation in 3-5 concise bullet points. Focus on:
- What topics were discussed
- Key concepts or insights learned
- Any decisions or next steps mentioned
- Questions still open

Be specific and concise. Do not include pleasantries.`

	msgs := []Message{{Role: "system", Content: prompt}}
	start := 0
	if len(history) > 30 {
		start = len(history) - 30
	}
	for _, m := range history[start:] {
		msgs = append(msgs, Message{Role: m.Role, Content: m.Content})
	}

	return CallLLMNonStreaming(msgs, model, apiKey, apiURL)
}