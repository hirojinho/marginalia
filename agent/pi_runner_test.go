package agent

import (
	"testing"
)

func TestParsePiLineTextDelta(t *testing.T) {
	line := []byte(`{"type":"message_update","assistantMessageEvent":{"type":"text_delta","delta":"Hello"}}`)
	ev, ok := parsePiLine(line)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if ev.Kind != "token" || ev.Delta != "Hello" {
		t.Errorf("got Kind=%q Delta=%q, want token/Hello", ev.Kind, ev.Delta)
	}
}

func TestParsePiLineThinkingDelta(t *testing.T) {
	line := []byte(`{"type":"message_update","assistantMessageEvent":{"type":"thinking_delta","delta":"hmm"}}`)
	ev, ok := parsePiLine(line)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if ev.Kind != "reasoning" || ev.Delta != "hmm" {
		t.Errorf("got Kind=%q Delta=%q, want reasoning/hmm", ev.Kind, ev.Delta)
	}
}

func TestParsePiLineToolcallEnd(t *testing.T) {
	line := []byte(`{"type":"message_update","assistantMessageEvent":{"type":"toolcall_end","toolCall":{"name":"bash","arguments":{"command":"ls"}}}}`)
	ev, ok := parsePiLine(line)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if ev.Kind != "tool_start" || ev.ToolName != "bash" {
		t.Errorf("got Kind=%q ToolName=%q, want tool_start/bash", ev.Kind, ev.ToolName)
	}
	if ev.InputSummary == "" {
		t.Errorf("InputSummary should not be empty")
	}
}

func TestParsePiLineToolExecutionEnd(t *testing.T) {
	line := []byte(`{"type":"tool_execution_end","toolName":"bash","isError":false,"result":{"output":"file1\nfile2"}}`)
	ev, ok := parsePiLine(line)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if ev.Kind != "tool_end" || ev.ToolName != "bash" || !ev.OK {
		t.Errorf("got Kind=%q ToolName=%q OK=%v, want tool_end/bash/true", ev.Kind, ev.ToolName, ev.OK)
	}
}

func TestParsePiLineToolExecutionEndIsError(t *testing.T) {
	line := []byte(`{"type":"tool_execution_end","toolName":"bash","isError":true,"result":{}}`)
	ev, ok := parsePiLine(line)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if ev.OK {
		t.Errorf("expected OK=false for isError=true")
	}
}

func TestParsePiLineSkillInvocation(t *testing.T) {
	line := []byte(`{"type":"skill_invocation","name":"study-notes"}`)
	ev, ok := parsePiLine(line)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if ev.Kind != "skill_start" || ev.SkillName != "study-notes" {
		t.Errorf("got Kind=%q SkillName=%q, want skill_start/study-notes", ev.Kind, ev.SkillName)
	}
}

func TestParsePiLineCompaction(t *testing.T) {
	line := []byte(`{"type":"compaction","reason":"context limit"}`)
	ev, ok := parsePiLine(line)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if ev.Kind != "compaction" || ev.Reason != "context limit" {
		t.Errorf("got Kind=%q Reason=%q, want compaction/context limit", ev.Kind, ev.Reason)
	}
}

func TestParsePiLineModelChange(t *testing.T) {
	line := []byte(`{"type":"model_change","from":"deepseek-v4-pro","to":"kimi-k2.6"}`)
	ev, ok := parsePiLine(line)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if ev.Kind != "model_change" || ev.From != "deepseek-v4-pro" || ev.To != "kimi-k2.6" {
		t.Errorf("got Kind=%q From=%q To=%q", ev.Kind, ev.From, ev.To)
	}
}

func TestParsePiLineAgentEnd(t *testing.T) {
	line := []byte(`{"type":"agent_end","messages":[]}`)
	ev, ok := parsePiLine(line)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if ev.Kind != "done" {
		t.Errorf("got Kind=%q, want done", ev.Kind)
	}
}

func TestParsePiLineIgnoredTypes(t *testing.T) {
	for _, line := range []string{
		`{"type":"response","success":true}`,
		`{"type":"agent_start"}`,
		`{"type":"turn_start"}`,
		`{"type":"turn_end"}`,
		`{"type":"message_start"}`,
		`{"type":"message_end"}`,
		`{"type":"tool_execution_start","toolName":"bash"}`,
		`{"type":"message_update","assistantMessageEvent":{"type":"thinking_start"}}`,
		`{"type":"message_update","assistantMessageEvent":{"type":"thinking_end"}}`,
		`{"type":"message_update","assistantMessageEvent":{"type":"toolcall_start"}}`,
		`{"type":"message_update","assistantMessageEvent":{"type":"done","reason":"stop"}}`,
	} {
		_, ok := parsePiLine([]byte(line))
		if ok {
			t.Errorf("expected ok=false for ignored line: %s", line)
		}
	}
}

func TestParsePiLineInvalidJSON(t *testing.T) {
	_, ok := parsePiLine([]byte(`not-json`))
	if ok {
		t.Errorf("expected ok=false for invalid JSON")
	}
}

func TestParsePiLineTruncatesLongInputSummary(t *testing.T) {
	long := `{"command":"` + string(make([]byte, 200)) + `"}`
	line := []byte(`{"type":"message_update","assistantMessageEvent":{"type":"toolcall_end","toolCall":{"name":"bash","arguments":` + long + `}}}`)
	ev, ok := parsePiLine(line)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if len(ev.InputSummary) > 84 { // 80 chars + "…" (3 bytes UTF-8)
		t.Errorf("InputSummary not truncated: len=%d", len(ev.InputSummary))
	}
}
