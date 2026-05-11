// Package agent — pi_runner.go manages Pi agent subprocesses and translates
// their JSONL event stream into typed PiEvents for the HTTP handler.
//
// Pi is launched per chat turn via RunPi. Events are published on the returned
// channel, which is closed when the agent_end event is received or the process
// exits. parsePiLine is a pure function that converts a single JSONL line into
// a PiEvent — it can be unit-tested without spawning Pi.
package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
)

// PiEvent is a parsed Pi JSONL event translated into the SSE vocabulary.
// Only the fields relevant to Kind are populated.
type PiEvent struct {
	// Kind is the SSE event name.
	Kind string
	// Delta is the text content for token and reasoning events.
	Delta string
	// ToolName is the tool name for tool_start and tool_end events.
	ToolName string
	// InputSummary is a short representation of the tool arguments (tool_start).
	InputSummary string
	// OutputSummary is a short representation of the tool result (tool_end).
	OutputSummary string
	// OK indicates tool success for tool_end events.
	OK bool
	// SkillName is the skill name for skill_start events.
	SkillName string
	// Reason is the compaction reason for compaction events.
	Reason string
	// From and To are the old and new model IDs for model_change events.
	From string
	To   string
	// Message is the error description for error events.
	Message string
	// Usage holds token counts for done events.
	Usage PiUsage
}

// PiUsage holds token counts from the agent_end event.
type PiUsage struct {
	Input  int     `json:"input"`
	Output int     `json:"output"`
	Cost   float64 `json:"cost"`
}

// piRaw is the top-level structure of a Pi JSONL event.
type piRaw struct {
	Type                  string            `json:"type"`
	AssistantMessageEvent *piAssistantEvent `json:"assistantMessageEvent,omitempty"`
	ToolName              string            `json:"toolName,omitempty"`
	IsError               bool              `json:"isError,omitempty"`
	Result                json.RawMessage   `json:"result,omitempty"`
	Name                  string            `json:"name,omitempty"`
	Reason                string            `json:"reason,omitempty"`
	From                  string            `json:"from,omitempty"`
	To                    string            `json:"to,omitempty"`
	Usage                 *piRawUsage       `json:"usage,omitempty"`
}

type piAssistantEvent struct {
	Type     string      `json:"type"`
	Delta    string      `json:"delta,omitempty"`
	ToolCall *piToolCall `json:"toolCall,omitempty"`
}

type piToolCall struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type piRawUsage struct {
	InputTokens  int     `json:"inputTokens"`
	OutputTokens int     `json:"outputTokens"`
	Cost         float64 `json:"cost"`
}

type piPromptCmd struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Message string `json:"message"`
}

const piSummaryMaxBytes = 80

// parsePiLine converts one JSONL line from Pi's stdout into a PiEvent.
// Returns (PiEvent{}, false) for lines that should be ignored.
func parsePiLine(line []byte) (PiEvent, bool) {
	var raw piRaw
	if err := json.Unmarshal(line, &raw); err != nil {
		return PiEvent{}, false
	}
	switch raw.Type {
	case "message_update":
		if raw.AssistantMessageEvent == nil {
			return PiEvent{}, false
		}
		inner := raw.AssistantMessageEvent
		switch inner.Type {
		case "text_delta":
			return PiEvent{Kind: "token", Delta: inner.Delta}, true
		case "thinking_delta":
			return PiEvent{Kind: "reasoning", Delta: inner.Delta}, true
		case "toolcall_end":
			if inner.ToolCall == nil {
				return PiEvent{}, false
			}
			summary := truncatePiSummary(string(inner.ToolCall.Arguments))
			return PiEvent{Kind: "tool_start", ToolName: inner.ToolCall.Name, InputSummary: summary}, true
		}
		return PiEvent{}, false
	case "tool_execution_end":
		summary := truncatePiSummary(string(raw.Result))
		return PiEvent{Kind: "tool_end", ToolName: raw.ToolName, OutputSummary: summary, OK: !raw.IsError}, true
	case "skill_invocation":
		return PiEvent{Kind: "skill_start", SkillName: raw.Name}, true
	case "compaction":
		return PiEvent{Kind: "compaction", Reason: raw.Reason}, true
	case "model_change":
		return PiEvent{Kind: "model_change", From: raw.From, To: raw.To}, true
	case "agent_end":
		usage := PiUsage{}
		if raw.Usage != nil {
			usage = PiUsage{
				Input:  raw.Usage.InputTokens,
				Output: raw.Usage.OutputTokens,
				Cost:   raw.Usage.Cost,
			}
		}
		return PiEvent{Kind: "done", Usage: usage}, true
	}
	return PiEvent{}, false
}

// truncatePiSummary truncates s to piSummaryMaxBytes bytes at a rune
// boundary, appending "…" if cut. Uses TruncateRunes for UTF-8 safety.
func truncatePiSummary(s string) string {
	return TruncateRunes(s, piSummaryMaxBytes)
}

// RunPi spawns a Pi RPC subprocess in sandboxDir, sends message, and returns
// a channel of PiEvents. The channel is closed when agent_end is received or
// the process exits. The context controls the per-turn timeout; cancelling it
// kills Pi and drains the channel.
func RunPi(ctx context.Context, sandboxDir, message, model, piPath, skillsDir, apiKey string) (<-chan PiEvent, error) {
	args := []string{"--mode", "rpc", "--provider", "opencode-go", "--model", model}
	if skillsDir != "" {
		args = append(args, "--skill", skillsDir)
	}

	cmd := exec.CommandContext(ctx, piPath, args...)
	cmd.Dir = sandboxDir
	cmd.Env = append(os.Environ(), "OPENCODE_API_KEY="+apiKey)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("create stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start pi: %w", err)
	}

	prompt := piPromptCmd{ID: "m1", Type: "prompt", Message: message}
	promptJSON, _ := json.Marshal(prompt)
	_, _ = fmt.Fprintf(stdin, "%s\n", promptJSON)
	// Do NOT close stdin here — pi exits immediately on stdin EOF even if the
	// prompt is still being processed. Stdin is closed by the goroutine below
	// once agent_end is received or the context is cancelled.

	events := make(chan PiEvent, 64)
	go func() {
		defer close(events)
		defer func() { _ = stdin.Close() }()
		defer func() { _ = cmd.Wait() }()

		scanner := bufio.NewScanner(stdout)
		const maxLine = 1 * 1024 * 1024 // 1 MB per line
		scanner.Buffer(make([]byte, maxLine), maxLine)

		for scanner.Scan() {
			ev, ok := parsePiLine(scanner.Bytes())
			if !ok {
				continue
			}
			select {
			case events <- ev:
			case <-ctx.Done():
				return
			}
			if ev.Kind == "done" {
				return
			}
		}
		if scanErr := scanner.Err(); scanErr != nil {
			select {
			case events <- PiEvent{Kind: "error", Message: "pi read: " + scanErr.Error()}:
			default:
			}
			return
		}
		select {
		case events <- PiEvent{Kind: "error", Message: "pi exited without completing"}:
		default:
		}
	}()

	return events, nil
}
