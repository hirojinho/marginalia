---
id: 2026-06-04-r11-r12-session-rules
title: Add session-close free recall and elaborative interrogation pedagogy rules
max_wall_clock_minutes: 20
max_diff_lines: 40
max_retries: 1
max_tokens: 20000
model: deepseek-v4-flash
thinking: off
requires_visual_approval: false
allow_web_search: false
---

## Goal

Add two pedagogy rules to the sandbox AGENTS.md template:

1. **Session-close free recall (R11)** — When a study session ends, the agent
   prompts "write down everything you remember from this session" and compares
   against the source material to highlight what was missed. One of the
   highest-evidence retrieval strategies (Roediger & Karpicke 2006, testing
   effect; Karpicke & Blunt 2011, free recall vs. concept mapping).

2. **Elaborative interrogation (R12)** — After the learner states a fact or
   explanation, the agent systematically follows with "why is this true?" or
   "why does that follow?" — not a one-off but a pattern across the session.
   Presses the chain of reasoning deeper (Pressley et al. 1987).

Both are pure prompt-rule additions to the sandbox AGENTS.md template and the
local CLAUDE.local.md copy. No code changes beyond the string constants.

## Implementation plan

### Step 1 — Add Rule 12 and Rule 13 string constants in `agent/sandbox.go`

In the `writeAgentsMD` function, after the Rule 11 constant (the "partial
answer cue" rule), add two new string constants:

```go
rule12 := "12. **Session-close free recall.** When a study session is ending — he signals completion, or you detect a natural stopping point after a task — before closing, prompt: *\"Before we wrap up — write down everything you remember from this session. Don't look at notes.\"* After he responds, compare his recall against the source material for the session and point out what was missed or thin. Do not quiz or re-test the gaps; just name what didn't come up so he knows what needs revisiting later. This is among the highest-impact retrieval strategies (Roediger & Karpicke 2006, testing effect; Karpicke & Blunt 2011, free recall outperforms concept mapping).\n\n"

rule13 := "13. **Elaborative interrogation — follow 'why'.**
```

Wait — let me write this properly. The string constants need to be complete
and match the existing style (numbered, evidence-cited, actionable).

**Rule 12 (session-close free recall):**

```go
rule12 := "12. **Session-close free recall.** When a study session is clearly ending (he says he's done, the task is complete and you've paused per Rule 10, or he signals a wrap-up), prompt: *\"Before we wrap up — write down everything you remember from this session. Don't look at notes.\"* After he responds, compare his recall against the source material covered in this session and point out what was missed or thin: *\"Good — you covered X and Y. You didn't mention Z, which we discussed when [context].\"* Do NOT quiz or re-test the gaps; just name them so he knows what needs revisiting. Skip this prompt if the session was purely tactical (planning, debugging, authoring) with no study content. Free recall is among the highest-impact retrieval strategies (Roediger & Karpicke 2006, testing effect; Karpicke & Blunt 2011, free recall vs. restudy/review).\n\n"
```

**Rule 13 (elaborative interrogation):**

```go
rule13 := "13. **Elaborative interrogation.** When he states a fact, definition, or causal explanation, follow up with *\"Why is this true?\"* or *\"Why does that follow?\"* — not as a one-off, but systematically across the session. When he gives the *why*, press one layer deeper if the topic allows it (*\"And why is that?\"*). The goal is not a correctness test — it's to force the chain of reasoning past surface recognition into the connective tissue that makes the fact stick. Acknowledge good reasoning; if the chain breaks, supply the missing link and move on. Stop pressing when he signals fatigue or when the why-chain reaches a self-evident foundation (definitional, axiomatic, or outside scope). (Pressley et al. 1987, elaborative interrogation; Chi et al. 1994, self-explanation effect.)\n\n"
```

### Step 2 — Append the new rules to the pedagogy section

Find the line in `writeAgentsMD` where the pedagogy section string is assembled.
Currently it appends rules 1–11 inside a string literal ending with Rule 11:

```go
"11. **On a partial answer, cue — don't complete.** …\n" +
```

After this line, append:

```go
rule12 +
rule13 +
```

These sit between Rule 11 and the interest-log section. Since they're
session-level rules (close + deepening), they belong right before the
interest-log note, which is also session-level.

### Step 3 — Mirror in `CLAUDE.local.md`

The local study-agent prompt at `/Users/eduardohiroji/Documents/ITA/claw-study/CLAUDE.local.md` (gitignored, ops-only) is the manual copy that the VPS study
agent reads directly. The sandbox template in `agent/sandbox.go` is what the
**local planning Pi** injects into **sandbox sessions** — two different
code paths, same pedagogy rules.

In `CLAUDE.local.md`, find Rule 11 (the "On a partial answer, cue" rule) and
append Rule 12 and Rule 13 after it, using the same prose as the Go constants
above but formatted for markdown:

```
12. **Session-close free recall.** When a study session is clearly ending (he says he's done, the task is complete and you've paused per Rule 10, or he signals a wrap-up), prompt: *"Before we wrap up — write down everything you remember from this session. Don't look at notes."* After he responds, compare his recall against the source material covered in this session and point out what was missed or thin: *"Good — you covered X and Y. You didn't mention Z, which we discussed when [context]."* Do NOT quiz or re-test the gaps; just name them so he knows what needs revisiting. Skip this prompt if the session was purely tactical (planning, debugging, authoring) with no study content. Free recall is among the highest-impact retrieval strategies (Roediger & Karpicke 2006, testing effect; Karpicke & Blunt 2011, free recall vs. restudy/review).

13. **Elaborative interrogation.** When he states a fact, definition, or causal explanation, follow up with *"Why is this true?"* or *"Why does that follow?"* — not as a one-off, but systematically across the session. When he gives the *why*, press one layer deeper if the topic allows it (*"And why is that?"*). The goal is not a correctness test — it's to force the chain of reasoning past surface recognition into the connective tissue that makes the fact stick. Acknowledge good reasoning; if the chain breaks, supply the missing link and move on. Stop pressing when he signals fatigue or when the why-chain reaches a self-evident foundation (definitional, axiomatic, or outside scope). (Pressley et al. 1987, elaborative interrogation; Chi et al. 1994, self-explanation effect.)
```

### Step 4 — Update `go vet`

No new types or functions — string constants only. `go vet` should pass without
changes.

## Verification recipe

### Pre-baseline (must FAIL on current main)

```bash
# 1. No session-close free recall rule exists.
grep -q 'Session-close free recall' agent/sandbox.go \
  && echo "FAIL: pre-baseline 1 — Rule 12 already exists in sandbox.go" \
  || echo "PASS: pre-baseline 1 — no session-close rule yet"

# 2. No elaborative interrogation rule exists.
grep -q 'Elaborative interrogation' agent/sandbox.go \
  && echo "FAIL: pre-baseline 2 — Rule 13 already exists in sandbox.go" \
  || echo "PASS: pre-baseline 2 — no elaborative interrogation yet"
```

### Post-acceptance (must PASS after implementation)

```bash
# 1. Session-close free recall rule present with evidence citation.
grep -q 'Session-close free recall.*Roediger' agent/sandbox.go \
  && echo "PASS: post-1 — Rule 12 present with evidence" \
  || echo "FAIL: post-1 — missing or incomplete Rule 12"

# 2. Elaborative interrogation rule present with evidence citation.
grep -q 'Elaborative interrogation.*Pressley' agent/sandbox.go \
  && echo "PASS: post-2 — Rule 13 present with evidence" \
  || echo "FAIL: post-2 — missing or incomplete Rule 13"

# 3. Go build passes.
go build ./...

# 4. Go vet passes.
go vet ./...

# 5. No regressions in tests.
go test ./...
```

### Human-eyeball notes

- The session-close prompt only fires when content was studied — not after
  planning/debugging sessions.
- The elaborative interrogation has an explicit stop condition (fatigue or
  self-evident foundation) — verify the agent doesn't infinite-chain "why".
- Both rules land only in `agent/sandbox.go` and `CLAUDE.local.md` — no new
  tables, no new tools, no UI changes.

## Done criteria

- [ ] Rule 12 (session-close free recall) string constant in `agent/sandbox.go`,
      appended to the pedagogy section after Rule 11.
- [ ] Rule 13 (elaborative interrogation) string constant in `agent/sandbox.go`,
      appended after Rule 12.
- [ ] Both rules mirrored in `CLAUDE.local.md` after Rule 11.
- [ ] `go build ./...` passes.
- [ ] `go vet ./...` passes.
- [ ] `go test ./...` passes.
- [ ] Pre-baseline fails on current main; post-acceptance passes on the branch.

## Rollback notes

Both rules are pure prompt text — removing them is a revert of the two string
constants and the two lines in CLAUDE.local.md. No schema, migration, or data
impact.
