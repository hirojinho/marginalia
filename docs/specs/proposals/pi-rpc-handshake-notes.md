# Pi RPC Handshake Notes — Phase 0 probe

> **Date:** 2026-05-10. Probe of `pi --mode rpc` against OpenCode Go on the
> claw-study VPS, gating decision for [agent runtime impl plan](../2026-05-10-agent-runtime-pi-impl.md).

## Install

VPS already had `node v22.22.2` and `npm 10.9.7`. Default npm prefix was `/usr`
(would have required sudo for a global install), so a user-scoped prefix was
configured first:

```bash
mkdir -p ~/.npm-global
npm config set prefix ~/.npm-global
echo 'export PATH=$HOME/.npm-global/bin:$PATH' >> ~/.bashrc
npm install -g @earendil-works/pi-coding-agent@0.74.0
```

- Install path: `~/.npm-global/lib/node_modules/@earendil-works/pi-coding-agent`
- Binary: `~/.npm-global/bin/pi` (255 packages, ~18 s install).
- Pinned: `0.74.0` (current latest).
- Quirk: `node-domexception@1.0.0` deprecation warning, harmless.
- Pi created `~/.pi/agent/` on first run for sessions/extensions/skills (we passed
  `--no-session`, `--no-skills`, `--no-extensions`, `--no-context-files`,
  `--no-prompt-templates` for clean cold-start measurements).

## Authentication

Pi reads `OPENCODE_API_KEY` from the environment for the `opencode-go` provider
(also `opencode` and `opencode-zen`). No config file or login flow needed for
this probe. The probe script exports the key from the existing app `.env`:

```bash
export OPENCODE_API_KEY=$(grep ^LLM_API_KEY= ~/stack/study-app/.env | cut -d= -f2)
pi --mode rpc --provider opencode-go --model <model-id>
```

`~/stack/study-app/.env` was **read only** — not modified. Key never written to
disk by the probe and not committed.

## Available models (relevant slice of `pi --list-models`)

The OpenCode Go catalog still lists all three target models under the
`opencode-go/` provider prefix:

```
opencode-go  deepseek-v4-pro  1M    384K
opencode-go  kimi-k2.6        262K  65K
opencode-go  glm-5.1          203K  33K
```

(There is also an `opencode/` provider with different pricing/scope; we used
`opencode-go/` to match what the live app already pays for.)

## Probe method

For each model, three trials of:

1. Spawn `pi --mode rpc --no-session --provider opencode-go --model <id>
   --no-context-files --no-skills --no-prompt-templates --no-extensions`.
2. Immediately write one JSONL line:
   `{"id":"p1","type":"prompt","message":"List files in /tmp"}` to stdin.
3. Stream stdout, log wall-clock to first event and to terminal `agent_end`.
4. Inspect events for `tool_execution_start` (or `toolcall_end` delta) to
   confirm a real tool call.

Probe script: `/tmp/pi-probe.py` on the VPS (kept around as a reference).

## Per-model results

Latencies in milliseconds, three trials, median bolded.

| Model | First-event ms (T1/T2/T3) | Total ms (T1/T2/T3) | Median total | Tool call? | Stop reason |
|---|---|---|---|---|---|
| `deepseek-v4-pro` | 780 / 728 / 812 | 16183 / 18660 / 18003 | **~18 000** | yes — `bash`, `command="ls -la /tmp"` | `stop` |
| `kimi-k2.6` | 806 / 768 / 835 | 10442 / 9021 / 8088 | **~9 000** | yes — `bash`, `command="ls -la /tmp"` | `stop` |
| `glm-5.1` | 783 / 812 / 847 | 3995 / 6732 / 8083 | **~6 700** | yes — `bash`, `command="ls -la /tmp"` | `stop` |

**Cold-start cost is consistent** across all three: ~750–850 ms from spawn to
first stdout event, dominated by Node + Pi boot. The remaining wall-clock is
LLM streaming time, which is what differentiates the models. No retry
events, no compaction, no errors on any trial.

### Sample event stream (deepseek-v4-pro, trial 3 — non-delta events only)

The full stream contained 435 lines, mostly `message_update / *_delta` chunks.
The skeleton is identical across all three models:

```jsonl
{"type":"response","command":"prompt","success":true,"id":"p1"}
{"type":"agent_start"}
{"type":"turn_start"}
{"type":"message_start","message":{...}}                 // brief outer wrapper
{"type":"message_end","message":{...}}
{"type":"message_start","message":{...}}                 // assistant turn
{"type":"message_update","assistantMessageEvent":{"type":"thinking_start",...}}
... thinking_delta x N ...
{"type":"message_update","assistantMessageEvent":{"type":"thinking_end",...}}
{"type":"message_update","assistantMessageEvent":{"type":"toolcall_start",...}}
{"type":"message_update","assistantMessageEvent":{"type":"toolcall_end","toolCall":{"name":"bash","arguments":{"command":"ls -la /tmp"}}}}
{"type":"message_update","assistantMessageEvent":{"type":"done","reason":"toolUse"}}
{"type":"message_end","message":{...}}
{"type":"tool_execution_start","toolCallId":"...","toolName":"bash","args":{"command":"ls -la /tmp"}}
{"type":"tool_execution_end","toolCallId":"...","toolName":"bash","isError":false,"result":{...}}
{"type":"turn_end","message":{...},"toolResults":[...],"message.stopReason":"toolUse"}
{"type":"turn_start"}
... thinking + text_delta chunks (final natural-language summary) ...
{"type":"turn_end","message":{...},"message.stopReason":"stop"}
{"type":"agent_end","messages":[...]}
```

Per-model quirks:

- **deepseek-v4-pro.** Slowest (~18 s total) but produced the longest, most
  thorough thinking and final summary (~360–435 events). Strong tool-call
  formatting with no warmup turns.
- **kimi-k2.6.** Mid-speed (~9 s), high event count (~470–630) — emits more
  `thinking_delta` granularity than the other two. Tool call clean.
- **glm-5.1.** Fastest (~4–8 s, high variance), shortest events (~73–116) —
  shorter chain-of-thought, terser final summary. Tool call still clean and
  identical (`bash`, `ls -la /tmp`). Variance suggests provider-side latency
  jitter rather than Pi overhead.

All three converged on the same tool name and bash command on the first turn,
emitted `stopReason: "toolUse"` then `stopReason: "stop"` on the second turn,
and produced a coherent natural-language summary of the directory contents.

## Go/no-go assessment

**Go.** The two assumptions the spec was worried about both held:

1. **Cold-start is not unbearable.** ~750–850 ms before any event lands is well
   under the 2–3 s budget in [agent-runtime-pi.md §Pi process lifecycle], and
   well under the spec's 10 s pool-trigger threshold. Per-turn spawn is fine
   for v1; no need to design a process pool now.

2. **All three OpenCode Go models tool-call cleanly** on a baseline prompt,
   despite the model catalog marking only kimi as "tools=yes" — Pi's tool
   plumbing works regardless. No model emitted free-form text instead of a tool
   call. No malformed JSON arguments observed.

End-to-end turn time (8–18 s) lands inside the spec's 15 s typical-turn target
for kimi/glm and slightly over for deepseek. That is acceptable for an
interactive study chat, especially since the deepseek "extra" time is real
work (longer summary), not stalls. UI-side: token streaming begins ~1 s after
spawn, so perceived responsiveness is good.

**Caveats / things to watch in Phase 5+:**

- This was a single trivial tool call. Multi-step turns with `claw-cli rag
  search` + a follow-up tool call will multiply the LLM round-trip count;
  budget accordingly. A two-tool turn on deepseek likely lands near 30 s.
- Tool-call discipline was tested only on the simplest prompt. If non-Claude
  models mis-format `claw-cli` invocations on more elaborate grammars (the
  Phase-1 risk in the impl plan), restrict v1 to deepseek-v4-pro.
- glm-5.1's wall-clock variance (4–8 s) is provider-side; not a blocker but
  worth a wider sample before pinning it as the cheap-quick-lookup default.

**Recommendation:** Proceed with Phase 1. Default model selection per the spec
(deepseek for chat, kimi for note-heavy skills, glm for quick lookups) looks
viable. Skip the process-pool work scoped for v2.
