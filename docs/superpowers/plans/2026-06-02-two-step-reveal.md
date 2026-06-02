# Two-Step Reveal — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.

**Goal:** On a partial recall/answer, the tutor cues the gap and lets the learner retry before revealing — enforced by an always-on pedagogy rule plus a sharpened `study-step-complete` Step 0.

**Architecture:** One new rule string in `agent/sandbox.go`'s pedagogy block (compiled into the binary, seen every Pi turn) + a Step 0 edit in `skills/study-step-complete/SKILL.md` (mounted file). One presence test. Prompt-only.

**Tech Stack:** Go 1.26 (`/opt/homebrew/bin/go`); mounted Markdown skill.

**Spec:** `docs/superpowers/specs/2026-06-02-two-step-reveal-design.md`

**Conventions:** build/test `/opt/homebrew/bin/go`; commit `git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho commit -m "..."`; branch main.

---

### Task 1: Always-on rule + Step 0 refinement

**Files:**
- Modify: `agent/sandbox.go` (the `pedagogySection` string in `studyTuningSections`)
- Modify: `skills/study-step-complete/SKILL.md` (Step 0)
- Test: `agent/sandbox_test.go`

Context: `studyTuningSections` builds a `pedagogySection` string concatenating numbered rules 1–10 then a `\n### Interest log — surface once per session\n...` subsection (it's the block that already contains S1's Rule 3 and S2's mastery-gate note). The new rule goes into that concatenation, before the `### Interest log` part. The test mirrors the existing `TestRule3UsesClawCLIConfidenceLog` / `TestAgentsMDMentionsMasteryGate` pattern (zero-value `SandboxManager`, call `studyTuningSections("ddia")`, assert a substring).

- [ ] **Step 1: Write the failing test**

Add to `agent/sandbox_test.go`:
```go
func TestPedagogyHasTwoStepReveal(t *testing.T) {
	var sm SandboxManager
	out := string(sm.studyTuningSections("ddia"))
	if !strings.Contains(out, "cue — don't complete") {
		t.Fatalf("pedagogy rules must include the two-step-reveal rule ('cue — don't complete')")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `/opt/homebrew/bin/go test ./agent/ -run TestPedagogyHasTwoStepReveal -v`
Expected: FAIL (rule absent).

- [ ] **Step 3: Add the rule to `pedagogySection`**

In `agent/sandbox.go`, find the `pedagogySection` concatenation in `studyTuningSections`. Immediately before the `"\n### Interest log — surface once per session\n..."` segment (i.e., after the last numbered rule / the rule10 segment), add this concatenated line:

```go
		"11. **On a partial answer, cue — don't complete.** When the learner recalls or answers *part* of something, do NOT immediately supply the missing pieces. First give a minimal cue toward the gap (a hint, a category, \"there's one more — think about X\") and let them attempt retrieval; reveal the full answer only after they try again or explicitly pass. The effort on the gap is the learning (Bjork & Bjork 1992, desirable difficulties; Slamecka & Graf 1978, generation effect).\n" +
```

Ensure it concatenates cleanly (the preceding segment ends in `\n" +` and the following `### Interest log` segment begins with `"\n### Interest log...`). Keep Go string escaping valid (escape `"`, leave backticks literal — there are none here).

- [ ] **Step 4: Run to verify it passes**

Run: `/opt/homebrew/bin/go test ./agent/ -run TestPedagogyHasTwoStepReveal -v`
Expected: PASS.

- [ ] **Step 5: Sharpen `study-step-complete` Step 0**

In `skills/study-step-complete/SKILL.md`, Step 0 currently contains:
> - Use the recall — not what's on the page — to drive Step 2's anchoring conversation. Gaps between recalled material and the actual content are the highest-value pedagogic signal available; surface them explicitly.
> - Do not paraphrase or correct prematurely. Let the user produce their version first, then anchor.

Replace those two bullets with (making "surface" an explicit two-step):
```markdown
- Use the recall — not what's on the page — to drive Step 2's anchoring conversation. Gaps between recalled material and the actual content are the highest-value pedagogic signal available.
- **Surface a gap as a two-step reveal, never a dump.** When recall is partial, first give a minimal *cue* toward the missing piece (a hint or category — "you've got three; there's one more, think about how the DB detects a conflict") and invite a second retrieval attempt. Reveal or confirm the full answer only after the learner tries again or explicitly passes. This matches the always-on "cue — don't complete" pedagogy rule. (Bjork & Bjork 1992, desirable difficulties.)
- Do not paraphrase or correct prematurely. Let the user produce their version first, then anchor.
```

- [ ] **Step 6: Full suite + build**

Run: `/opt/homebrew/bin/go test ./... && /opt/homebrew/bin/go build .`
Expected: green.

- [ ] **Step 7: Commit**

```bash
git add agent/sandbox.go agent/sandbox_test.go skills/study-step-complete/SKILL.md
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho commit -m "feat(pedagogy): two-step reveal — cue the gap before revealing on partial recall"
```

---

### Task 2: Build, deploy (binary + SKILL.md), verify

**Files:** none (operational). Note: the SKILL.md is a mounted file, NOT compiled in — it must be scp'd separately.

- [ ] **Step 1: Cross-compile the server**
```bash
cd ~/Documents/ITA/claw-study
GOOS=linux GOARCH=amd64 /opt/homebrew/bin/go build -o /tmp/study-app-linux .
```

- [ ] **Step 2: Deploy the binary (back up, restart)**
```bash
scp /tmp/study-app-linux nanoclaw:/home/eduardo/stack/study-app/bin/study-app.new
ssh nanoclaw 'cd ~/stack/study-app/bin && cp study-app study-app.bak && mv study-app.new study-app && chmod +x study-app && export XDG_RUNTIME_DIR=/run/user/$(id -u) && systemctl --user restart study-app.service && sleep 3 && systemctl --user is-active study-app.service'
```
Expected: `active`.

- [ ] **Step 3: Sync the SKILL.md (mounted file — not in the binary)**
```bash
scp skills/study-step-complete/SKILL.md nanoclaw:/home/eduardo/stack/study-app/skills/study-step-complete/SKILL.md
ssh nanoclaw 'grep -c "two-step reveal" ~/stack/study-app/skills/study-step-complete/SKILL.md'
```
Expected: `1` (the new text is on the VPS).

- [ ] **Step 4: Verify the rule is in the deployed binary**
```bash
ssh nanoclaw "grep -c \"cue — don't complete\" ~/stack/study-app/bin/study-app"
```
Expected: `1` (compiled-in rule present).

- [ ] **Step 5: Confirm service healthy**
```bash
URL=https://study.claw-study.xyz
TOKEN=$(ssh nanoclaw 'grep ^AUTH_TOKEN= ~/stack/study-app/.env | cut -d= -f2')
rtk proxy curl -s -o /dev/null -w "health=%{http_code}\n" -H "Authorization: Bearer $TOKEN" "$URL/debug/health"
```
Expected: `health=200`.

Manual acceptance: next study completion with a partial recall, confirm the tutor cues the gap and waits rather than dumping the answer.

---

## Self-Review

- **Spec coverage:** always-on rule → Task 1 Steps 1–4; Step 0 sharpening → Task 1 Step 5; tests → Task 1 Step 1; binary deploy → Task 2 Steps 1–2,4; SKILL.md sync (the easy-to-miss part) → Task 2 Step 3.
- **Placeholders:** none — exact rule string, exact Step 0 replacement, exact commands.
- **Consistency:** test substring `"cue — don't complete"` matches the rule string in Step 3 exactly (including the em-dash and apostrophe). The SKILL.md edit references the same rule name for cross-consistency. Binary grep in Task 2 Step 4 uses the same substring.
