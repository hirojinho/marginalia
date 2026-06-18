# S4 — Two-Step Reveal on Partial Recall — Design

**Date:** 2026-06-02
**Status:** Approved, pending implementation plan
**Part of:** the pedagogy-consolidation series (S1 ✅, S2 ✅, S3 ✅, **S4** — the finale). Prompt-only; no DB, no Go logic beyond a rule string + its test.

## Problem

In DDIA session #56 the learner recalled 3 of 4 lost-update solutions. Instead of cueing the missing one and giving him a chance to retrieve it, the tutor immediately printed the complete list. That throws away the highest-value moment — the retrieval-effort on the gap. The skill's Step 0 already says "surface gaps explicitly," but "explicitly" was interpreted as *reveal*, not *cue*.

## Goal

On any partial recall/answer, the tutor gives a minimal cue toward the gap and lets the learner attempt retrieval **before** revealing the full answer. Make this an always-on rule (so it governs every turn, not just the completion skill).

## Design

### 1. New pedagogy rule in `agent/sandbox.go` (always-on)

Add a rule to the mandatory pedagogy block in `studyTuningSections` (the same block S1's Rule 3 and S2's gate note live in). Wording intent:

> **On a partial answer, cue — don't complete.** When the learner recalls or answers *part* of something, do NOT immediately supply the missing pieces. First give a minimal cue toward the gap (a hint, a category, "there's one more — think about X") and let them attempt retrieval. Reveal the full answer only after they try again or explicitly pass. The struggle on the gap is the learning (Bjork & Bjork 2011, desirable difficulties; Slamecka & Graf 1978, generation effect).

This is the always-on enforcement Pi sees every turn — the durable fix, since the failure was the model defaulting to reveal.

### 2. Sharpen `study-step-complete` SKILL.md Step 0

Step 0 currently: *"Gaps between recalled material and the actual content are the highest-value pedagogic signal; surface them explicitly. Do not paraphrase or correct prematurely."* Amend so "surface" is unambiguously a **two-step**: on a gap, cue toward the missing piece and invite a second retrieval attempt; reveal/confirm only after the learner tries or passes. Cross-reference the always-on rule so the two stay consistent.

## Non-goals

- No change to logging, the gate, or any Go logic beyond the rule string + a presence test.
- Not touching the legacy mirrors (gone in S3).
- Not adding a new skill or DB field.

## Testing

- `agent/sandbox_test.go`: a test asserting `studyTuningSections("ddia")` output contains the two-step-reveal guidance (a stable substring, e.g. `"cue"` within the new rule, or a distinctive phrase like `"cue — don't complete"`). Mirrors the existing `TestRule3UsesClawCLIConfidenceLog` / `TestAgentsMDMentionsMasteryGate` pattern.
- `go test ./... && go build .` green.
- The SKILL.md edit is a static mounted file (no Go test); verify by reading the rendered file.

Manual acceptance: deploy; next study completion with a partial recall, confirm the tutor cues the gap and waits rather than dumping the full answer.

## Deploy

Two artifacts, because they ship differently:
1. **`study-app` binary** — carries the always-on Rule (compiled from `sandbox.go`); `AGENTS.md` regenerates next turn. Standard cross-compile + scp + restart.
2. **`SKILL.md`** — a mounted file, NOT compiled in. Pi reads skills from `SKILLS_DIR=$VAULT_ROOT/skills` (verified). The binary deploy does NOT sync it (the VPS copy is stale, dated 2026-05-30). So **explicitly** `scp skills/study-step-complete/SKILL.md nanoclaw:$VAULT_ROOT/skills/study-step-complete/SKILL.md`. No restart needed for the skill file (read per Pi launch), but restart anyway for the binary.
