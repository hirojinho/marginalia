# Pi-Driven Pipeline Postmortem — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. **This plan is executed INLINE by Claude Code in a live session** — Claude Code is the driver, local Pi is the actor, and the human is the approver. Do NOT dispatch subagents; the whole point is that Claude Code reads every Pi transcript directly.

**Goal:** Drive local Pi through a full forensics → fix → deploy → harden loop on the claw-study overnight pipeline, anchored on the `sm2-spaced-review` failure, while Claude Code critiques how Pi performs each step.

**Architecture:** Claude Code authors each prompt and invokes local Pi via `pi -p --mode json`, capturing and parsing the stdout event stream. Forensics phases run Pi **read-only** (`-xt write,edit`); Pi returns findings as final text and Claude Code persists them. Fix phases use a **propose → review → apply** two-call protocol (the interactive `design-gate` extension can't gate print mode). Deploy pauses for human go/no-go before the prod swap. The loop is finally distilled into a reusable `pipeline-postmortem` Pi skill.

**Tech Stack:** local Pi (`@earendil-works/pi-coding-agent`, `deepseek-v4-pro`), `ssh nanoclaw` to the VPS, claw-study (Go), the `pi-claw-pipeline` Pi package.

---

## Shared conventions (read once before any task)

**Base Pi invocation (read-only forensics):**
```bash
pi -p --mode json \
  --provider deepseek --model deepseek-v4-pro --thinking high \
  --session-id pm-<phase> \
  -xt write,edit \
  "<PROMPT>" | tee ~/Documents/ITA/claw-study/docs/postmortem/2026-06-14/raw/pm-<phase>.jsonl
```
- `-xt write,edit` removes the write+edit tools → Pi can read + run bash (incl. `ssh nanoclaw`) but cannot mutate any file. Its findings come back as the final assistant message.
- `--session-id pm-<phase>` isolates each phase's session; within a phase, a follow-up call uses `--continue`.
- Always `tee` the raw JSONL so the transcript is auditable.

**Base Pi invocation (write, apply step only):** drop `-xt write,edit`, add `--tools read,write,edit,bash`, and feed the *already-approved* patch verbatim.

**Parsing the JSON stream:** each line is a JSON event. The final assistant text is the event with the terminal `agent_end` / final-message marker. Claude Code reads the `tee`'d file, extracts the final assistant message, and that becomes the phase finding.

**Directories (create in Task 0):**
- `docs/postmortem/2026-06-14/` — findings (`phaseN.md`, written by Claude Code from Pi output)
- `docs/postmortem/2026-06-14/raw/` — raw `pm-<phase>.jsonl` transcripts
- `docs/postmortem/2026-06-14/meta-log.md` — Claude Code's running meta-analysis

**Meta-analysis (after EVERY Pi call):** Claude Code appends to `meta-log.md`:
```
## pm-<phase> (<iso-ts>)
- Asked: <one line>
- Did Pi do it? <yes/partial/no + what>
- Off-rails / corrections: <what Pi got wrong, what I had to re-prompt>
- Lesson for Pi config / prompt design: <takeaway>
```

**Checkpoints with human:** explicit `>>> HUMAN CHECKPOINT` steps. Do not proceed past one without the human's reply.

---

## Task 0: Preflight

**Files:**
- Create: `docs/postmortem/2026-06-14/{,raw/}` dirs, `meta-log.md`

- [ ] **Step 1: Create the working dirs**

```bash
mkdir -p ~/Documents/ITA/claw-study/docs/postmortem/2026-06-14/raw
: > ~/Documents/ITA/claw-study/docs/postmortem/2026-06-14/meta-log.md
```

- [ ] **Step 2: Verify driver-side prerequisites (Claude Code runs directly, NOT Pi)**

```bash
ssh -o ConnectTimeout=8 nanoclaw 'echo OK; ls -d ~/stack/claw-build/results | head' \
  && pi --version \
  && pi --list-models deepseek 2>&1 | grep -i 'deepseek-v4-pro' \
  && test -n "$DEEPSEEK_API_KEY" && echo "KEY PRESENT"
```
Expected: `OK`, a results dir path, the pi version, a `deepseek-v4-pro` line, `KEY PRESENT`.
If `ssh nanoclaw` fails, stop — every forensics phase depends on it.

- [ ] **Step 3: Smoke-test the read-only Pi invocation**

```bash
pi -p --mode json --provider deepseek --model deepseek-v4-pro --thinking minimal \
  --session-id pm-smoke -xt write,edit \
  "Run 'ssh nanoclaw ls ~/stack/claw-build/results' and reply with ONLY the number of result directories you see." \
  | tee ~/Documents/ITA/claw-study/docs/postmortem/2026-06-14/raw/pm-smoke.jsonl
```
Expected: a JSON event stream ending with an assistant message containing a count. Confirms (a) print+json parses, (b) Pi can `ssh nanoclaw`, (c) write/edit are absent (Pi won't try to create files).

- [ ] **Step 4: Meta-log the smoke test**

Append the meta-analysis block for `pm-smoke` to `meta-log.md`. Note especially whether Pi tried to write a file despite `-xt write,edit` (it should not be able to).

---

## Task 1: Triage sweep

**Files:**
- Create: `docs/postmortem/2026-06-14/phase1.md` (Claude Code writes from Pi output)

- [ ] **Step 1: Send the triage prompt to Pi (read-only)**

```bash
pi -p --mode json --provider deepseek --model deepseek-v4-pro --thinking high \
  --session-id pm-p1 -xt write,edit \
  "You are auditing the claw-study overnight build pipeline. All run evidence is on a remote host reachable as 'ssh nanoclaw'. Do NOT modify anything; you are read-only.

  1. List every result directory: ssh nanoclaw 'ls -1 ~/stack/claw-build/results'.
  2. For EACH ticket id, inspect its results dir (ssh nanoclaw 'ls ~/stack/claw-build/results/<id>' then cat the relevant files — typically a result/summary json or .txt, the gate output, and any exit-code file). Extract: final outcome (shipped / failed-gate / failed-impl / paused / idle), exit code, wall-clock seconds, total cost if recorded, and any 'Pi theory' failure note.
  3. Cross-reference the repo at ~/Documents/ITA/claw-study: which ids are in specs/done/ vs specs/failed/ vs specs/queue/.
  4. Output a single markdown table sorted by date: columns [id | repo-state | outcome | exit | wall_s | cost | one-line note]. Below the table, list any DISCREPANCIES (e.g., a results dir says shipped but the spec is still in queue/, or a cost outlier), and state which single ticket is the primary HARD failure to deep-dive.

  Reply with ONLY the markdown (table + discrepancies + primary-failure line)." \
  | tee ~/Documents/ITA/claw-study/docs/postmortem/2026-06-14/raw/pm-p1.jsonl
```

- [ ] **Step 2: Parse and persist**

Read `raw/pm-p1.jsonl`, extract the final assistant message, write it verbatim to `docs/postmortem/2026-06-14/phase1.md` under a `# Phase 1 — Triage sweep` heading.

- [ ] **Step 3: Verify Pi's coverage (Claude Code, the "test")**

```bash
ssh nanoclaw 'ls -1 ~/stack/claw-build/results | wc -l'
```
Expected: the row count in Pi's table equals this number. If Pi missed dirs, re-prompt with `--continue` naming the missing ids. Confirm Pi's "primary hard failure" line names `2026-06-01-sm2-spaced-review`; if it names something else, STOP and surface it to the human before proceeding.

- [ ] **Step 4: Meta-log** the `pm-p1` block (coverage, classification accuracy, any re-prompts needed).

- [ ] **>>> HUMAN CHECKPOINT 1:** Present the triage table + the named primary failure. Get a nod that sm2 is the right anchor (or redirect).

---

## Task 2: sm2 deep-dive forensics

**Files:**
- Create: `docs/postmortem/2026-06-14/phase2.md`

- [ ] **Step 1: Send the deep-dive prompt (read-only)**

```bash
pi -p --mode json --provider deepseek --model deepseek-v4-pro --thinking high \
  --session-id pm-p2 -xt write,edit \
  "Read-only forensics on ONE failed overnight ticket: 2026-06-01-sm2-spaced-review.

  Evidence (all via 'ssh nanoclaw'):
  - Results dir: ~/stack/claw-build/results/2026-06-01-sm2-spaced-review/ — cat the full transcript, the gate output, the diff Pi produced, and the exit code.
  - Orchestrator scripts: ~/stack/claw-build/bin/overnight-run.sh and gate-runner.sh — read the parts that set exit codes (e.g. exit 11 = failed-gate) and the model-escalation ladder.
  The spec is in the repo at ~/Documents/ITA/claw-study/specs/failed/2026-06-01-sm2-spaced-review.md.

  Determine, WITH EVIDENCE (quote the exact log lines and their location):
  1. At which step the run failed: pre-baseline / go build / go test / staging spin / post-acceptance probe / implementation-incomplete (token|wall|crash).
  2. The PROXIMATE error (what the log literally shows failing).
  3. The ROOT CAUSE, classified into exactly ONE layer: SPEC (broken verifier or a plan bug Pi faithfully executed), EXECUTOR (deepseek-v4-flash + thinking:off couldn't do the SM-2 math/migration), or ORCHESTRATOR (a bug in the shell scripts/gate).
  4. The minimal fix at that layer.

  Reply in markdown with sections: ## Failed step, ## Proximate error (quoted), ## Root cause + layer, ## Minimal fix. Do not propose code yet — just the diagnosis." \
  | tee ~/Documents/ITA/claw-study/docs/postmortem/2026-06-14/raw/pm-p2.jsonl
```

- [ ] **Step 2: Parse and persist** the final message to `phase2.md`.

- [ ] **Step 3: Verify the diagnosis is evidence-backed (Claude Code)**

Read `phase2.md`. Reject (and re-prompt with `--continue`) if: the root cause has no quoted log line; the "failed step" contradicts the exit code Pi quoted; or Pi conflates symptom (e.g. "test failed") with cause (e.g. "migration column type mismatch"). Spot-check one quoted line:
```bash
ssh nanoclaw "grep -n '<a distinctive phrase Pi quoted>' ~/stack/claw-build/results/2026-06-01-sm2-spaced-review/*"
```
Expected: the phrase exists where Pi said it does.

- [ ] **Step 4: Meta-log** `pm-p2` (was the diagnosis crisp? did Pi distinguish symptom/cause? did it pick exactly one layer?).

---

## Task 3: Drift check

**Files:**
- Create: `docs/postmortem/2026-06-14/phase3.md`

- [ ] **Step 1: Send the drift-check prompt (read-only)**

```bash
pi -p --mode json --provider deepseek --model deepseek-v4-pro --thinking high \
  --session-id pm-p3 -xt write,edit \
  "Read-only semantic-drift check on the sm2 spec, against the project's own architecture.

  Read in the repo ~/Documents/ITA/claw-study:
  - docs/adr/0007*.md (the Knowledge Component / Zettelkasten-atom decision)
  - CONTEXT.md (glossary — find the 'knowledge_component_id' / Knowledge Component entries)
  - specs/failed/2026-06-01-sm2-spaced-review.md

  ADR-0007 mandates: the Knowledge Component (atom) is the unit that mastery/confidence/retrieval key on; knowledge_component_id must hold an ATOM id, not a plan-task id.

  Answer: does the sm2 spec USE knowledge_component_id consistently with its DEFINED REFERENT (an atom id), or does it inherit the legacy task-id drift (term reused but referent wrong)? Quote the exact spec lines that touch knowledge_component_id / retrieval_queue. Verdict: ALIGNED or DRIFTED, with the evidence. If DRIFTED, say whether the drift would break the fix we're about to ship.

  Reply in markdown: ## Verdict, ## Evidence (quoted lines), ## Impact on the sm2 fix." \
  | tee ~/Documents/ITA/claw-study/docs/postmortem/2026-06-14/raw/pm-p3.jsonl
```

- [ ] **Step 2: Parse and persist** to `phase3.md`.

- [ ] **Step 3: Verify (Claude Code)** that Pi quoted real ADR-0007 text and real spec lines (not paraphrase). If the verdict is DRIFTED and it would break the fix, fold the drift remedy into the Task 4 fix scope; if DRIFTED-but-harmless, note it and proceed.

- [ ] **Step 4: Meta-log** `pm-p3` (did Pi check MEANING vs just the term name?).

- [ ] **>>> HUMAN CHECKPOINT 2:** Present root cause (phase2) + drift verdict (phase3) + the proposed fix layer. Get a go to author the fix.

---

## Task 4: Fix authoring (propose → review → apply)

**Files:**
- Modify: depends on the layer from Task 2 — one of:
  - SPEC: `specs/failed/2026-06-01-sm2-spaced-review.md` (verifier/plan)
  - EXECUTOR: spec frontmatter (`model:`/`thinking:`) and/or `pi-claw-pipeline/skills/.../implement-from-spec` (on VPS: `~/.pi/.../implement-from-spec`)
  - ORCHESTRATOR: VPS `~/stack/claw-build/bin/*.sh`

- [ ] **Step 1: PROPOSE — ask Pi for the exact diff (read-only)**

```bash
pi -p --mode json --provider deepseek --model deepseek-v4-pro --thinking high \
  --session-id pm-p4 -xt write,edit \
  "Based on this root cause: <PASTE the 'Root cause + layer' + 'Minimal fix' from phase2.md, and any drift remedy from phase3.md>.

  Produce the EXACT change as a unified diff (or, for a new file, full contents with its path). Touch ONLY the layer named in the root cause. Constraints: the sm2 spec's gate contract (Pre-baseline must FAIL on main, Post-acceptance must PASS after) must stay intact; max_diff_lines and frontmatter limits must remain sane. Do NOT apply anything — output the diff only, in a single fenced \`\`\`diff block, preceded by a one-paragraph rationale." \
  | tee ~/Documents/ITA/claw-study/docs/postmortem/2026-06-14/raw/pm-p4-propose.jsonl
```

- [ ] **Step 2: Claude Code reviews the diff** — does it match the diagnosed layer? does it preserve the gate contract? is it minimal? If not, re-prompt with `--continue`. Persist the approved diff to `docs/postmortem/2026-06-14/phase4-fix.diff`.

- [ ] **>>> HUMAN CHECKPOINT 3:** Show the human the diff. Get explicit approval to apply.

- [ ] **Step 3: APPLY — Pi applies the approved diff verbatim (write enabled)**

```bash
pi -p --mode json --provider deepseek --model deepseek-v4-pro --thinking low \
  --session-id pm-p4 --continue --tools read,write,edit,bash \
  "Apply EXACTLY the diff you proposed and I approved (in phase4-fix.diff), changing nothing. Then show 'git -C <repo-or-path> diff' to confirm what landed. If the target is on the VPS, make a *.bak.20260614 backup first (the orchestrator convention) before editing in place via ssh." \
  | tee ~/Documents/ITA/claw-study/docs/postmortem/2026-06-14/raw/pm-p4-apply.jsonl
```

- [ ] **Step 4: Verify the applied change equals the approved diff (Claude Code)**

```bash
cd ~/Documents/ITA/claw-study && git diff   # for repo-side changes
# or, for VPS: ssh nanoclaw 'diff <file>.bak.20260614 <file>'
```
Expected: the working-tree diff is byte-equivalent to `phase4-fix.diff`. If Pi drifted, revert and re-apply.

- [ ] **Step 5: Re-validate the pre-baseline locally (the spec author's red step)**

If the fix was SPEC-side, run the spec's `### Pre-baseline` block against current main and confirm it still FAILs for the right reason (so the gate remains a real test). Capture output.

- [ ] **Step 6: Meta-log** `pm-p4` (did Pi apply faithfully? did propose/apply split work? any drift between proposed and applied?).

---

## Task 5: Re-queue and run through the real gate

**Files:**
- Modify: `git mv specs/failed/2026-06-01-sm2-spaced-review.md specs/queue/`

- [ ] **Step 1: Re-queue the spec (Claude Code runs git; commit is the human's call)**

```bash
cd ~/Documents/ITA/claw-study
git mv specs/failed/2026-06-01-sm2-spaced-review.md specs/queue/2026-06-01-sm2-spaced-review.md
git add -A && git status --short
```
Do NOT push yet — hold for the human checkpoint below (push is what the VPS pipeline consumes).

- [ ] **>>> HUMAN CHECKPOINT 4:** Confirm the requeue + the fix diff are ready to commit & push. On approval, commit (manually, per claw-study's push-to-main convention) and push.

- [ ] **Step 2: Trigger the gate on the VPS via Pi (Pi drives, bash enabled, no source writes needed)**

```bash
pi -p --mode json --provider deepseek --model deepseek-v4-pro --thinking low \
  --session-id pm-p5 --tools read,bash \
  "On the VPS via 'ssh nanoclaw', run the overnight pipeline for the single re-queued ticket 2026-06-01-sm2-spaced-review, but STOP before the production binary swap. Concretely: trigger ~/stack/claw-build/bin/overnight-run.sh (it picks the next queue item) OR, if it auto-deploys on green, instead run gate-runner.sh against a fresh agent/<id> worktree so the gate runs without deploying. Stream the run. Report: which gate steps ran, each step's pass/fail, the diff stats, gate timing, and whether it reached gate-green + staging-verified. Do NOT swap prod." \
  | tee ~/Documents/ITA/claw-study/docs/postmortem/2026-06-14/raw/pm-p5.jsonl
```
NOTE: confirm from phase2's reading of `overnight-run.sh` whether it auto-deploys on green. If it does, the gate-runner.sh path (no deploy) is mandatory here so we honor HUMAN CHECKPOINT 5.

- [ ] **Step 2b: VERIFY the gate result independently (Claude Code, not Pi)**

```bash
ssh nanoclaw 'cat ~/stack/claw-build/results/2026-06-01-sm2-spaced-review/*gate* 2>/dev/null | tail -40; echo "EXIT:"; cat ~/stack/claw-build/results/2026-06-01-sm2-spaced-review/exit* 2>/dev/null'
```
Expected: all gate steps green, exit 0. If red, loop back to Task 2 with the new evidence (the fix didn't close it).

- [ ] **Step 3: Persist** the gate report to `phase5.md`; **meta-log** `pm-p5`.

---

## Task 6: Human go/no-go + deploy

- [ ] **>>> HUMAN CHECKPOINT 5 (go/no-go):** Present gate-green + staging-verified evidence. The human says GO or NO-GO on the prod binary swap.

- [ ] **Step 1: On GO — Pi runs the deploy (write/bash on VPS)**

```bash
pi -p --mode json --provider deepseek --model deepseek-v4-pro --thinking low \
  --session-id pm-p6 --tools read,bash \
  "Human approved deploy of 2026-06-01-sm2-spaced-review. On the VPS via 'ssh nanoclaw': run the pipeline's deploy step (deploy-swap.sh with the gated binary), then probe prod: curl the /debug/health and /debug/version endpoints. Move the spec specs/in-progress|queue -> specs/done as the pipeline's green path does (or confirm the script already did). Report the deployed commit sha and the health/version responses." \
  | tee ~/Documents/ITA/claw-study/docs/postmortem/2026-06-14/raw/pm-p6.jsonl
```

- [ ] **Step 2: Independently confirm the ship (Claude Code)**

```bash
curl -s https://study.claw-study.xyz/debug/version
cd ~/Documents/ITA/claw-study && git fetch && git log --oneline -3 origin/main
```
Expected: `/debug/version` returns the new commit; sm2 spec now in `specs/done/`. Run the spec's `### Post-acceptance` recipe against prod/staging and confirm exit 0.

- [ ] **Step 3: Persist** ship record to `phase6.md`; **meta-log** `pm-p6`. On NO-GO, record why and stop here (feature stays unshipped; pipeline fixes still proceed).

---

## Task 7: Pipeline hardening (propose → review → apply)

**Files (as the root cause dictates):**
- Modify: `~/Documents/ITA/claw-study/AGENTS.md` (planning rules) and/or the executor `implement-from-spec` skill (on VPS) and/or VPS `bin/*.sh`.

- [ ] **Step 1: PROPOSE class-level fixes (read-only)**

```bash
pi -p --mode json --provider deepseek --model deepseek-v4-pro --thinking high \
  --session-id pm-p7 -xt write,edit \
  "Given the sm2 root cause <PASTE> and the drift finding <PASTE>, propose the MINIMAL set of changes that prevent this CLASS of error from recurring — not just this one ticket. Candidates, pick only what the root cause justifies:
  - planning side: a rule in ~/Documents/ITA/claw-study/AGENTS.md (e.g. semantic — not just terminological — reconciliation against CONTEXT.md; every spec must cite the ADR it honors/amends; verifier red-step discipline);
  - executor side: implement-from-spec skill wording, or a default frontmatter floor (model/thinking) for math/migration tickets;
  - orchestrator: a gate or escalation fix.
  For EACH proposed change output: the file, a unified diff, and one sentence on which failure mechanism it closes. Output diffs only; do not apply." \
  | tee ~/Documents/ITA/claw-study/docs/postmortem/2026-06-14/raw/pm-p7-propose.jsonl
```

- [ ] **Step 2: Claude Code reviews** each proposed diff against the root cause — reject any change not traceable to the diagnosed mechanism (no scope creep). Persist approved diffs to `phase7-fixes.diff`.

- [ ] **>>> HUMAN CHECKPOINT 6:** Show the human the hardening diffs. Approve/trim.

- [ ] **Step 3: APPLY approved diffs (write enabled)**

```bash
pi -p --mode json --provider deepseek --model deepseek-v4-pro --thinking low \
  --session-id pm-p7 --continue --tools read,write,edit,bash \
  "Apply EXACTLY the approved diffs in phase7-fixes.diff, verbatim. VPS files: *.bak.20260614 backup first. Then show the resulting git diff (repo) and ssh diffs (VPS). Change nothing beyond the approved diffs." \
  | tee ~/Documents/ITA/claw-study/docs/postmortem/2026-06-14/raw/pm-p7-apply.jsonl
```

- [ ] **Step 4: Verify** applied == approved (as Task 4 Step 4). **Meta-log** `pm-p7`.

---

## Task 8: Harvest the `pipeline-postmortem` skill

**Files:**
- Create: `~/Documents/ITA/pi-claw-pipeline/skills/pipeline-postmortem/SKILL.md`
- Create: `~/Documents/ITA/pi-claw-pipeline/prompts/postmortem.md` (`/postmortem <ticket>`)
- Modify: `~/Documents/ITA/pi-claw-pipeline/package.json` (version bump)

- [ ] **Step 1: PROPOSE the skill (read-only), grounded in what actually happened**

```bash
pi -p --mode json --provider deepseek --model deepseek-v4-pro --thinking high \
  --session-id pm-p8 -xt write,edit \
  "Read these postmortem findings: ~/Documents/ITA/claw-study/docs/postmortem/2026-06-14/phase1.md .. phase7-fixes.diff, and the meta-log.md. Also read the existing package skill format at ~/Documents/ITA/pi-claw-pipeline/skills/grill-with-docs/SKILL.md and prompt at prompts/grill.md.

  Distill the loop you just executed into a reusable Pi skill 'pipeline-postmortem' that local Pi can run on its own next time: triage sweep -> reconstruct failure with evidence -> classify cause-layer -> propose layer fix -> re-run through the real gate -> stop for human go/no-go -> harden the class. Encode the read-only-forensics / propose-review-apply / file-handoff conventions. Include a SHORT 'how to drive local Pi' note capturing the corrections from meta-log.md (where Pi needed re-prompting).

  Output: (1) full SKILL.md contents with proper frontmatter (name, description; mirror grill-with-docs style), (2) full prompts/postmortem.md contents with argument-hint for \$ARGUMENTS = ticket id, (3) the package.json version bump line. Output as files-to-write with their paths; do not apply." \
  | tee ~/Documents/ITA/claw-study/docs/postmortem/2026-06-14/raw/pm-p8-propose.jsonl
```

- [ ] **Step 2: Claude Code reviews** the skill for faithfulness (does it match what we actually did?) and the package conventions (`pi` key, `pi-package` keyword, frontmatter). Persist approved contents.

- [ ] **>>> HUMAN CHECKPOINT 7:** Show the skill + prompt. Approve.

- [ ] **Step 3: APPLY (write enabled)** — Pi writes the three files verbatim.

```bash
pi -p --mode json --provider deepseek --model deepseek-v4-pro --thinking low \
  --session-id pm-p8 --continue --tools read,write,edit,bash \
  "Write the three approved files verbatim (SKILL.md, prompts/postmortem.md, package.json bump). Then run 'pi list' to confirm the package still parses and the new skill/prompt are discovered." \
  | tee ~/Documents/ITA/claw-study/docs/postmortem/2026-06-14/raw/pm-p8-apply.jsonl
```

- [ ] **Step 4: Verify** the package still loads and the new skill is listed.

```bash
cd ~/Documents/ITA/pi-claw-pipeline && pi list 2>&1 | grep -i postmortem
```
Expected: `pipeline-postmortem` appears. **Meta-log** `pm-p8`.

---

## Task 9: Assemble the overview doc

**Files:**
- Create: `docs/postmortem/2026-06-14-overview.md`

- [ ] **Step 1: Have Pi assemble the narrative (read-only)**

```bash
pi -p --mode json --provider deepseek --model deepseek-v4-pro --thinking high \
  --session-id pm-p9 -xt write,edit \
  "Read all of ~/Documents/ITA/claw-study/docs/postmortem/2026-06-14/ (phase1..phase7 + the fix diffs + ship record). Write a tight overview titled 'Overnight pipeline postmortem — 2026-06-14' with sections: ## What happened (triage summary, the one-paragraph fleet picture), ## What went wrong (sm2 root cause, layer, evidence), ## Drift check, ## The fix and the re-ship (what shipped, prod commit), ## Pipeline hardening (the class-level changes + which mechanism each closes), ## What we learned about driving Pi (from the meta-log). Be concrete; quote shas and exit codes. Reply with ONLY the markdown." \
  | tee ~/Documents/ITA/claw-study/docs/postmortem/2026-06-14/raw/pm-p9.jsonl
```

- [ ] **Step 2: Claude Code persists** the final message to `docs/postmortem/2026-06-14-overview.md`, fixing any factual slips against the phase docs.

- [ ] **Step 3: Meta-log** `pm-p9`.

- [ ] **>>> HUMAN CHECKPOINT 8 (final):** Present the overview + the list of everything that landed (sm2 shipped, hardening diffs, new skill). Decide what to commit/push (claw-study repo) and whether to push the `pi-claw-pipeline` package.

---

## Notes for the executor

- **Never** run a forensics phase without `-xt write,edit`. The read-only guarantee is the safety belt that lets Pi roam the VPS.
- **Never** skip a `>>> HUMAN CHECKPOINT`. Deploy (CP5) and every write-to-prod are human-gated by design.
- If `ssh nanoclaw` or the deepseek key fails mid-run, stop and report — do not let Pi improvise around a broken surface.
- Cost: forensics is cache-read-light (fresh sessions, no giant context). If a phase balloons, that itself is a meta-log finding about prompt scoping.
- This plan loops: if Task 5's gate is still red, return to Task 2 with the new evidence rather than forcing the fix.
