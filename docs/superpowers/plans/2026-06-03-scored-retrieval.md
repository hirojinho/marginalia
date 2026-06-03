# Scored Retrieval — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Replace self-rated confidence with a tutor-measured retrieval score, and make the session-open scored recall reliably fire — all prompt-only, reusing the existing `confidence_log`/gate/scheduler plumbing.

**Architecture:** Rewrite Rules 3, 6, and 8's cross-reference in `agent/sandbox.go`'s `pedagogySection`/`rule6`, and the `study-step-complete` Step 0, so the [0,1] value is `(key idea-units recalled ÷ total)` measured by the tutor rather than self-reported, and the session-open recall fires even when the retrieval queue is empty. No DB/Go-logic/rename change.

**Tech Stack:** Go 1.26 (`/opt/homebrew/bin/go`); mounted Markdown skill.

**Spec:** `docs/superpowers/specs/2026-06-03-scored-retrieval-design.md`

**Conventions:** build/test `/opt/homebrew/bin/go`; commit `git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho commit -m "..."`; branch main.

**Compatibility note:** the existing S1 test `TestRule3UsesClawCLIConfidenceLog` asserts the output CONTAINS `"claw-cli confidence log"` and does NOT contain `"call the log_confidence tool"`. The new Rule 3 keeps the `claw-cli confidence log` command and never mentions `log_confidence tool`, so that test stays green — do not break it. S2 (`mastery_threshold`), S4 (`cue — don't complete`) presence tests must also stay green.

---

### Task 1: Rewrite Rules 3, 6, 8 + Step 0

**Files:**
- Modify: `agent/sandbox.go` (`rule6` var ~line 227–229; Rule 3 string ~line 244; Rule 8 string ~line 249)
- Modify: `skills/study-step-complete/SKILL.md` (Step 0)
- Test: `agent/sandbox_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `agent/sandbox_test.go`:
```go
func TestRule3ScoresRetrievalNotSelfRating(t *testing.T) {
	var sm SandboxManager
	out := string(sm.studyTuningSections("ddia"))
	if strings.Contains(out, "How confident are you") {
		t.Fatalf("Rule 3 must NOT ask for a self-rating")
	}
	if !strings.Contains(out, "key idea-units") {
		t.Fatalf("Rule 3 must instruct scoring the recall by key idea-units")
	}
	// plumbing unchanged — still logs via the same command
	if !strings.Contains(out, "claw-cli confidence log") {
		t.Fatalf("Rule 3 must still log via claw-cli confidence log")
	}
}

func TestRule6RecallMandatoryWhenQueueEmpty(t *testing.T) {
	var sm SandboxManager
	out := string(sm.studyTuningSections("ddia"))
	if !strings.Contains(out, "does NOT license skipping") {
		t.Fatalf("Rule 6 must make an empty queue NOT a license to skip the recall")
	}
	if !strings.Contains(out, "most recent completed task") {
		t.Fatalf("Rule 6 must keep the most-recent-completed-task fallback")
	}
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `/opt/homebrew/bin/go test ./agent/ -run "TestRule3ScoresRetrieval|TestRule6Recall" -v`
Expected: FAIL — current Rule 3 says "How confident are you" (no "key idea-units"); Rule 6 lacks "does NOT license skipping".

- [ ] **Step 3: Replace Rule 3**

In `agent/sandbox.go`, replace the entire current Rule 3 string (the line beginning `"3. **ALWAYS ask \"How confident are you with this?\"**`) with:

```go
		"3. **Score retrieval — never ask for a self-rating.** Do NOT ask \"how confident are you?\" or request a 0–1 number. After a recall or explain-back, score it yourself: read the source (`pdf extract`) to fix the handful of **key idea-units** for the task, count how many he actually produced, and log `value = produced ÷ total` (round to 2 dp) via:\n```\nclaw-cli confidence log --session <SESSION_ID> --kc <active task id> --value <0.0-1.0> --raw \"<what he recalled, verbatim>\"\n```\nwhere <SESSION_ID> is the id in the Session section above and <active task id> is the `id` of the active task from `claw-cli plan status`. Report hits and misses as a two-step reveal (Rule 11). A low score → return to the topic; do not advance. The value is a **measurement of retrieval**, not a feeling (Karpicke & Blunt 2011; self-rated confidence is miscalibrated — Dunlosky et al. 2013). If no active task is in context, skip the command.\n" +
```

- [ ] **Step 4: Replace `rule6` (both variants)**

In `agent/sandbox.go`, replace the `rule6 := "..."` assignment (line ~227) with:

```go
	rule6 := "6. **Session-open recall (MANDATORY).** Before answering anything else, run `claw-cli retrieve due`. **(a)** If items are due → run a scored retrieval round on the top 1–2 (he recalls in his own words → score per Rule 3 → cue gaps per Rule 11). **(b)** If nothing is due BUT a task has been completed → you MUST still open with a scored free-recall of the **most recent completed task** before the Rule 7 prediction. An empty queue is the normal case (SM-2 future-dates fresh items); it does NOT license skipping the recall. **(c)** ONLY if no task is completed at all (a brand-new course, or his very first task) do you skip to the Rule 7 pre-read prediction. Never invent or assume a completed task; never claim he has read, finished, or recalled anything without evidence — if unsure whether he has started, ask. Non-negotiable; highest-evidence pedagogic move (Roediger & Karpicke 2006, testing effect).\n"
```

And replace the `!settings.Interleaving` variant (line ~229) with the same text plus its interleaving clause — insert `(Interleaving of older tasks is OFF for this course — for branch (b) stay on the most recent.) ` immediately before `Non-negotiable;`:

```go
	if !settings.Interleaving {
		rule6 = "6. **Session-open recall (MANDATORY).** Before answering anything else, run `claw-cli retrieve due`. **(a)** If items are due → run a scored retrieval round on the top 1–2 (he recalls in his own words → score per Rule 3 → cue gaps per Rule 11). **(b)** If nothing is due BUT a task has been completed → you MUST still open with a scored free-recall of the **most recent completed task** before the Rule 7 prediction. An empty queue is the normal case (SM-2 future-dates fresh items); it does NOT license skipping the recall. **(c)** ONLY if no task is completed at all (a brand-new course, or his very first task) do you skip to the Rule 7 pre-read prediction. Never invent or assume a completed task; never claim he has read, finished, or recalled anything without evidence — if unsure whether he has started, ask. (Interleaving of older tasks is OFF for this course — for branch (b) stay on the most recent.) Non-negotiable; highest-evidence pedagogic move (Roediger & Karpicke 2006, testing effect).\n"
	}
```

- [ ] **Step 5: Fix Rule 8 cross-reference**

In `agent/sandbox.go` Rule 8 string, change `break it across turns with a Rule-3 confidence check in between` to `break it across turns with a Rule-3 recall check in between`.

- [ ] **Step 6: Run to verify the new tests pass + the S1/S2/S4 tests stay green**

Run: `/opt/homebrew/bin/go test ./agent/ -run "TestRule3|TestRule6Recall|TestAgentsMDMentionsMasteryGate|TestPedagogyHasTwoStepReveal" -v`
Expected: PASS — `TestRule3ScoresRetrievalNotSelfRating`, `TestRule6RecallMandatoryWhenQueueEmpty`, AND the existing `TestRule3UsesClawCLIConfidenceLog`, `TestAgentsMDMentionsMasteryGate`, `TestPedagogyHasTwoStepReveal` all green.

- [ ] **Step 7: Sharpen `study-step-complete` Step 0 (scored recall)**

In `skills/study-step-complete/SKILL.md`, Step 0, after the existing recall bullets (the "Without looking..." prompt and the two-step-reveal bullet added in S4), add a bullet:
```markdown
- **Score the recall, don't ask him to.** After he recalls, identify the section's key idea-units yourself and log `value = idea-units he produced ÷ total` via `claw-cli confidence log --session <id> --kc <task id> --value <0.0-1.0> --raw "<his recall>"` — the same value the mastery gate and scheduler consume. Never ask him to self-rate a confidence number; the score is your measurement of his retrieval (Karpicke & Blunt 2011).
```

- [ ] **Step 8: Full suite + build**

Run: `/opt/homebrew/bin/go test ./... && /opt/homebrew/bin/go build .`
Expected: green.

- [ ] **Step 9: Commit**

```bash
git add agent/sandbox.go agent/sandbox_test.go skills/study-step-complete/SKILL.md
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho commit -m "feat(pedagogy): scored retrieval replaces self-rated confidence; reliable session-open recall"
```

---

### Task 2: Build, deploy (binary + SKILL.md), verify

**Files:** none (operational). The SKILL.md is mounted, not compiled — scp it separately.

- [ ] **Step 1: Cross-compile the server**
```bash
cd ~/Documents/ITA/claw-study
GOOS=linux GOARCH=amd64 /opt/homebrew/bin/go build -o /tmp/study-app-linux .
```

- [ ] **Step 2: Deploy binary + SKILL.md (back up, restart)**
```bash
scp /tmp/study-app-linux nanoclaw:/home/eduardo/stack/study-app/bin/study-app.new
scp skills/study-step-complete/SKILL.md nanoclaw:/home/eduardo/stack/study-app/skills/study-step-complete/SKILL.md
ssh nanoclaw 'cd ~/stack/study-app/bin && cp study-app study-app.bak && mv study-app.new study-app && chmod +x study-app && export XDG_RUNTIME_DIR=/run/user/$(id -u) && systemctl --user restart study-app.service && sleep 3 && systemctl --user is-active study-app.service'
```
Expected: `active`.

- [ ] **Step 3: Verify the new rules are in the deployed binary**
```bash
ssh nanoclaw 'grep -c "Score retrieval — never ask" ~/stack/study-app/bin/study-app && grep -c "does NOT license skipping" ~/stack/study-app/bin/study-app && grep -c "Score the recall, don" ~/stack/study-app/skills/study-step-complete/SKILL.md'
```
Expected: `1`, `1`, `1`.

- [ ] **Step 4: Confirm the old self-rating prompt is gone from the binary**
```bash
ssh nanoclaw 'grep -c "How confident are you" ~/stack/study-app/bin/study-app || echo 0'
```
Expected: `0`.

- [ ] **Step 5: Health check**
```bash
URL=https://study.claw-study.xyz
TOKEN=$(ssh nanoclaw 'grep ^AUTH_TOKEN= ~/stack/study-app/.env | cut -d= -f2')
rtk proxy curl -s -o /dev/null -w "health=%{http_code}\n" -H "Authorization: Bearer $TOKEN" "$URL/debug/health"
```
Expected: `health=200`.

Manual acceptance: next DDIA session-open, confirm the tutor opens with a recall of the last completed task (even with an empty queue), scores it against the section's key points, logs a `confidence_log` row with that measured value, and never asks for a self-rated number.

---

## Self-Review

- **Spec coverage:** Rule 3 scoring → Task 1 Step 3; Rule 6 reliability → Step 4; Rule 8 cross-ref → Step 5; Step 0 scored recall → Step 7; tests → Step 1; deploy (binary + SKILL.md) → Task 2.
- **Placeholders:** none — exact replacement strings and commands. `<SESSION_ID>`/`<active task id>` are runtime placeholders the agent fills (literal text in the rule), not plan gaps.
- **Consistency:** new test substrings (`"key idea-units"`, `"does NOT license skipping"`, `"most recent completed task"`) appear verbatim in the Step 3/Step 4 rule strings; the binary-grep in Task 2 Step 3 uses substrings (`"Score retrieval — never ask"`, `"does NOT license skipping"`, `"Score the recall, don"`) that appear verbatim in those edits. The S1 test compatibility (keep `claw-cli confidence log`, never `log_confidence tool`) holds — Step 3's string contains the command and not the forbidden phrase.
