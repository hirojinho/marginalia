# Weekly Study Digest

**Date:** 2026-05-12
**Status:** Approved

## Goal

Every Monday at 09:00 São Paulo time, the study app runs a full Pi agent turn that reads the past week's usage metrics and the active study plans, produces a concise digest with suggestions, and delivers it to Eduardo via Telegram through NanoClaw (`@HiroClawdBot`).

## Flow

```
weekly goroutine (main.go, polls hourly)
  → check: Monday 09:00–10:00 SP and last_digest_at < this week's Monday
  → QueryEventSummary(since=7 days ago)
  → CreateSession("", "Weekly Digest YYYY-MM-DD")
  → SaveMessage(session, "user", digest_prompt)
  → RunPiTurn(messages) → drain events → collect text + reasoning
  → SaveAssistantMessage(session, text, reasoning)
  → InjectNanoclawTask(NANOCLAW_INBOUND_DB, relay_prompt)
  → setMetaInt("last_digest_at", now_ms)
```

## Pi Prompt (user message)

```
Weekly study digest — please review the past 7 days and give me actionable suggestions.

Usage metrics (last 7 days):
  Chat turns: {TurnCount}  |  Avg latency: {AvgLatencyMs}ms  |  p95: {P95LatencyMs}ms
  Tokens: {InputTokens} in / {OutputTokens} out
  Tool calls: {ToolCounts}
  Sessions by course: {CourseCounts}
  Plan toggles: {PlanDone} done / {PlanUndone} undone
  PDF opens: {PDFOpens}

Steps:
1. Read the study plans for all active courses (ce297, ddia, dsa-interview, software-arch, thesis). Use read_file or rag_search as needed.
2. Cross-reference what I actually worked on (CourseCounts, plan toggles) against what the plans say I should be doing.
3. Write a weekly digest formatted for Telegram (no markdown headers, emoji sparingly, under 350 words) containing:
   - A 2–3 line "last week" summary
   - 3–5 specific suggestions for this week
   - Any ⚠️ flags: courses with pending plan tasks but zero sessions this week
```

## NanoClaw Relay Prompt (content injected into inbound.db)

```json
{
  "prompt": "Relay this weekly study digest to Eduardo via Telegram, verbatim and without modification:\n\n{PI_RESPONSE_TEXT}",
  "script": null
}
```

NanoClaw's agent (DeepSeek) receives this as a `task` and sends the text as-is to Telegram.

## Scheduling Logic

```go
// In main.go goroutine — checks every hour:
spLoc := time.FixedZone("America/Sao_Paulo", -3*60*60)
now := time.Now().In(spLoc)
thisMonday := /* truncate to most recent Monday 00:00 SP */
if now.Weekday() == time.Monday &&
   now.Hour() == 9 &&
   lastDigestAt < thisMonday.UnixMilli() {
    go runAndInjectDigest(ctx, app, cfg)
}
```

`last_digest_at` is stored in the existing `meta` table (key `"last_digest_at"`). The goroutine is disabled when `NANOCLAW_INBOUND_DB` env var is unset.

## NanoClaw Task Row Schema

Matches the existing HEARTBEAT task format in `messages_in`:

| Column | Value |
|---|---|
| `id` | `"study-digest-{unix_ms}"` |
| `seq` | `MAX(seq) + 1` (computed in same transaction) |
| `kind` | `"task"` |
| `timestamp` | ISO 8601 now |
| `status` | `"pending"` |
| `process_after` | ISO 8601 now |
| `recurrence` | `NULL` (one-shot) |
| `series_id` | `"study-weekly-digest"` |
| `tries` | `0` |
| `trigger` | `1` |
| `platform_id` | `"telegram:1177542513"` |
| `channel_type` | `"telegram"` |
| `thread_id` | `""` |
| `content` | JSON relay prompt (above) |

## Digest Session

- Created via `app.CreateSession("", "Weekly Digest YYYY-MM-DD")`
- Visible in the session list under the "General" course group
- Contains one user message (the digest prompt with formatted metrics) and one assistant message (Pi's response + reasoning)
- Not set as the active session — `RunWeeklyDigest` saves the current `ActiveSessionID()` before calling `CreateSession`, then restores it via `SetActiveSessionIDInMemory` after the turn completes

## DB API

```go
// agent/digest.go
func RunWeeklyDigest(ctx context.Context, app *App, cfg DigestConfig) (string, error)

type DigestConfig struct {
    PiConfig   PiRunnerConfig  // existing type — model, tools, etc.
    NanoclawDB string          // path to inbound.db; empty = disabled
}

// agent/nanoclaw.go
func InjectNanoclawTask(dbPath, prompt string) error
```

`RunWeeklyDigest` creates the session, builds messages, invokes Pi (draining events into a discard writer), saves both messages, and returns the response text. It does not inject into NanoClaw — the caller does that so the goroutine can log errors independently.

`InjectNanoclawTask` opens NanoClaw's SQLite with a short write transaction. Uses `modernc.org/sqlite` (already a dependency).

## Files Touched

| File | Change |
|---|---|
| `agent/digest.go` | New: `RunWeeklyDigest`, `DigestConfig`, metrics formatter |
| `agent/nanoclaw.go` | New: `InjectNanoclawTask` |
| `agent/nanoclaw_test.go` | New: test for `InjectNanoclawTask` using in-memory SQLite |
| `main.go` | Add hourly goroutine + `runAndInjectDigest` |

Estimated ~150 lines of new code.

## Environment

```
NANOCLAW_INBOUND_DB=/path/to/nanoclaw-v2/data/v2-sessions/ag-1777924890168-e1etn1/sess-1777924890171-zaqikj/inbound.db
```

Added to `$VAULT_ROOT/.env` on the VPS. When unset, the goroutine logs a single startup notice and exits — no digest runs in local dev.

## Error Handling

- Pi turn error: log `slog.Warn`, skip injection, do not update `last_digest_at` (will retry next Monday)
- NanoClaw injection error: log `slog.Warn`, update `last_digest_at` anyway (Pi already ran; we don't want to re-send next hour)
- Empty Pi response: treat as Pi error (skip injection)

## Out of Scope

- Configurable schedule (day/time are hardcoded; change requires redeploy)
- Digest history UI (sessions are visible in the session list already)
- Retry logic for NanoClaw injection beyond the next Monday window
- Multiple recipients
