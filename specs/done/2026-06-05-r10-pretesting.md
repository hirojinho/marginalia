---
id: 2026-06-05-r10-pretesting
title: Add pre-testing pedagogy rule — probe prior knowledge at session open
max_wall_clock_minutes: 20
max_diff_lines: 30
max_retries: 1
max_tokens: 20000
model: deepseek-v4-flash
thinking: off
requires_visual_approval: false
allow_web_search: false
---

## Goal

Add a pre-testing rule (R10) to the sandbox AGENTS.md template. After the
session-open recall (Rule 6) and before the pre-read prediction, the agent
asks 2–3 probing questions about the upcoming topic to activate prior
knowledge. Even wrong answers create a "fertile void" that improves later
learning (Kornell et al. 2009, pretesting effect). This is distinct from
Rule 7's pre-read prediction ("what do you think the key idea will be?") —
pre-testing asks specific, content-probing questions about the topic.

Pure prompt rule — no code beyond the string constant in `agent/sandbox.go`
and its mirror in `CLAUDE.local.md`.

## Implementation plan

### Step 1 — Add the rule string constant in `agent/sandbox.go`

In the `writeAgentsMD` function, locate the pedagogy section assembly. Find the
end of the Rule 6 (session-open recall) string and the beginning of the Rule 7
(pre-read prediction) string. Insert a new string constant between them:

```go
pretestingRule := "Pre-testing. After the session-open recall (Rule 6) and before opening the first 🔴 Read task, ask 2–3 probing questions about the upcoming topic to activate prior knowledge — generate them from the task title and description, no tool call. Make them open-ended and content-specific, not generic. Example for a task on STPA hazard analysis: *\"What kinds of hazards do you think a safety analysis should catch? How would you start identifying them?\"* Even wrong or incomplete answers create a 'fertile void' that improves later learning — struggling with a question before seeing the content makes the answer stick (Kornell et al. 2009, pretesting effect; Richland, Kornell & Kao 2009, generation effect).\n\nAfter he responds, do NOT grade, correct, or reveal the answers — say *\"Good — keep those ideas in mind as you read\"* and move directly to the Pre-Read prediction step. The questions are a probe, not a quiz.\n\n"
```

**Placement:** append `pretestingRule` to the pedagogy section string, between
the Rule 6 entry and the Rule 7 entry. Since this runs after the R11+R12 spec
(which adds rules 12 and 13 at the end), the existing rule numbering is
1–6, then this rule, then 7–13. Do NOT renumber existing rules — the new rule
stands without a number between 6 and 7, matching the agent's natural turn flow.

Specifically, in the pedagogy section assembly, change:

```go
rule6 +
"7. **Pre-Read prediction.** Before opening any new 🔴 Read task…\n" +
```

to:

```go
rule6 +
pretestingRule +
"7. **Pre-Read prediction.** Before opening any new 🔴 Read task…\n" +
```

The `pretestingRule` constant is defined above (or inline) before the
pedagogy section assembly.

### Step 2 — Mirror in `CLAUDE.local.md`

In `~/claw-study/CLAUDE.local.md`, find Rule 6
(session-open recall) and the Pre-Read prediction rule. Insert the following
between them:

```
**Pre-testing (no rule number — fires after Rule 6, before Pre-Read prediction).** After the session-open recall and before opening the first 🔴 Read task, ask 2–3 probing questions about the upcoming topic to activate prior knowledge — generate them from the task title and description, no tool call. Make them open-ended and content-specific, not generic. Example for a task on STPA hazard analysis: *"What kinds of hazards do you think a safety analysis should catch? How would you start identifying them?"* Even wrong or incomplete answers create a 'fertile void' that improves later learning — struggling with a question before seeing the content makes the answer stick (Kornell et al. 2009, pretesting effect; Richland, Kornell & Kao 2009, generation effect).

After he responds, do NOT grade, correct, or reveal the answers — say *"Good — keep those ideas in mind as you read"* and move directly to the Pre-Read prediction step. The questions are a probe, not a quiz.
```

### Step 3 — Verify build

No new functions, imports, or types. `go build ./...` and `go vet ./...` should
pass without changes.

## Verification recipe

### Pre-baseline (must FAIL on current main)

```bash
# No pre-testing rule exists in the sandbox template.
if grep -q 'fertile void\|Pre-testing. After the session-open recall\|2–3 probing questions' agent/sandbox.go; then
  echo "PRE-FAIL: pre-testing text already exists on main (spec already shipped?)"
  exit 0
fi
# Rule absent — fix needed. Exit non-zero to signal pre-baseline failure.
exit 1
```

### Post-acceptance (must PASS after implementation)

```bash
# 1. Pre-testing rule present with Kornell evidence citation.
grep -q 'Kornell.*2009.*pretesting' agent/sandbox.go \
  && echo "PASS: post-1 — pre-testing rule present with evidence" \
  || echo "FAIL: post-1 — missing pre-testing rule"

# 2. Pre-testing sits between Rule 6 and Rule 7 in the template.
#    (check that the session-open recall line appears before pretesting,
#     and pre-read prediction appears after)
grep -n 'session-open recall\|pretesting\|Pre-Read prediction' agent/sandbox.go \
  | awk -F: '{print NR, $0}'

# 3. Go build passes.
go build ./...

# 4. Go vet passes.
go vet ./...

# 5. No regressions in tests.
go test ./...
```

### Human-eyeball notes

- The pre-testing questions are open-ended probes, not graded quizzes — the
  agent should not correct or score the answers.
- Questions are generated from the task title/description inline — no
  `pdf extract` or tool call needed.
- The rule fires once per session, at the first task, not before every task.
- It's placed after the retrieval round (Rule 6) and before the pre-read
  prediction (Rule 7), to avoid disrupting the scored-recall flow.

## Done criteria

- [ ] Pre-testing prompt rule inserted in `agent/sandbox.go` between Rule 6
      and Rule 7 in the pedagogy section.
- [ ] Same rule mirrored in `CLAUDE.local.md` between Rule 6 and Pre-Read
      prediction.
- [ ] Rule text includes Kornell et al. 2009 citation.
- [ ] Rule specifies: no grading, no correction — probe, not quiz.
- [ ] `go build ./...` passes.
- [ ] `go vet ./...` passes.
- [ ] `go test ./...` passes.
- [ ] Pre-baseline fails on current main; post-acceptance passes on the branch.

## Rollback notes

Pure prompt text. Revert the string constant addition in `agent/sandbox.go` and
the paragraph in `CLAUDE.local.md`. No schema, migration, or data impact.
