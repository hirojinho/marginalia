# Study Experience Redesign — Phased Implementation Plan

> **For agentic workers:** Use superpowers:subagent-driven-development to execute task-by-task. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Make study sessions small, daily, and spaced; move steering out of chat; make reading active and tutor-position-aware; and (later) make the Plan the navigation spine.

**Decisions:** [ADR 0009](../../adr/0009-session-single-task-spaced-unit.md) (session = single-task spaced unit; tutor stops), [ADR 0010](../../adr/0010-steering-via-settings-ui.md) (steering via settings UI), [ADR 0011](../../adr/0011-plan-is-navigation-spine.md) (Plan is the spine), [ADR 0012](../../adr/0012-segmented-active-reading.md) (segmented active reading + position-aware tutor). Glossary in [CONTEXT.md](../../CONTEXT.md).

**Sync constraint:** The Go binary carries `agent/*.go` changes (deploy by build + scp + restart). But `CLAUDE.local.md` and `skills/*/SKILL.md` are **disk files on the VPS** and are NOT carried by the binary deploy — they must be `scp`'d separately to `~/stack/study-app/` (and skills to their mounted path). Both repo and VPS copies are kept in sync.

**Phasing:** Phases 1–2 improve the *current* app and ship first. Phase 3 (IA rebuild) and 4 (settings UI) are larger and each get their own design pass. Phase 5 is deferred.

---

## PHASE 1 — Prompt-only behaviour wins (no UI, ship first)

### Task 1.1 — Stop, don't chain (study-step-complete Step 5)

**File:** `skills/study-step-complete/SKILL.md`

- [ ] **Step 1: Rewrite Step 5.** Replace the whole `### Step 5 — Recommend next step` section (the heading + its three bullets) with:

```markdown
### Step 5 — Mark a stopping point (default: STOP)

Completing one task is a complete session. The default is to **stop here, not
chain into the next task** — distributed practice beats massed sessions (Cepeda
2008).

- Affirm the stop: "Good stopping point. Come back tomorrow and we'll open with a
  quick recall on this." Name in one phrase what next time will open with, so the
  return has a hook.
- Do **not** recommend, preview, or start the next task.
- Continuing is **opt-in**: only if the learner explicitly says "keep going" do
  you proceed, treating the next task as a fresh task in this session.
```

- [ ] **Step 2: Fix the contradicting Red Flag row.** In the Red Flags table, replace the row `| Recommending next step without explaining the connection | Always say *why* it's next in the narrative |` with:

```markdown
| Chaining into the next task by default | Stop after one task; continuing is opt-in (the learner says "keep going") |
```

- [ ] **Step 3: Commit.**
```bash
git add skills/study-step-complete/SKILL.md
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho commit -m "feat(pedagogy): tutor stops after one task instead of chaining (ADR 0009)"
```

### Task 1.2 — Interleaved opener (Rule 6, three files)

Rule 6 currently says: open every session with a recall of the **most recent completed task**. Change it to: one recall check, *usually* the most recent task, *occasionally* an older task from an earlier phase (interleaving). **Exactly one check** either way (keep the opener small). Edit all three carriers, preserving each file's voice.

- [ ] **Step 1: `agent/sandbox.go`** (line ~181, the AGENTS.md template; "him/his" voice). Replace the existing rule-6 string with:

```go
		"6. **Session-open retrieval check.** At the start of every chat session, before answering anything else, run ONE recall check. Usually ask him to recall, in his own words, the main idea of his most recent completed task. Occasionally instead pick an OLDER completed task from an earlier phase and ask him to recall that (interleaved spaced retrieval — Rohrer 2007; Cepeda 2008) — useful when earlier material is at risk of fading. Exactly one check either way; keep the opener small. Compare his recall against the actual content silently — note gaps and surface them this turn. Non-negotiable; highest-evidence pedagogic move (Roediger & Karpicke 2006, testing effect).\n" +
```

- [ ] **Step 2: `agent/agent.go`** (line ~49, `toolsAndRulesPrompt`; "the user/their" voice). Replace rule 6 with:

```
6. **Session-open retrieval check.** At the start of every chat session, before answering anything else, run ONE recall check. Usually ask the user to recall, in their own words, the main idea of their most recent completed task. Occasionally instead pick an OLDER completed task from an earlier phase to recall (interleaved spaced retrieval — Rohrer 2007; Cepeda 2008). Exactly one check either way. Compare silently and surface gaps. (Roediger & Karpicke 2006.)
```

- [ ] **Step 3: `CLAUDE.local.md`** (line ~48; "Eduardo/his" voice). Replace rule 6 with:

```
6. **Session-open retrieval check.** At the start of every chat session, before answering anything else, run ONE recall check. Usually ask Eduardo to recall, in his own words, the main idea of his most recent completed task. Occasionally instead pick an OLDER completed task from an earlier phase to recall (interleaved spaced retrieval — Rohrer 2007; Cepeda 2008). Exactly one check either way. Compare silently and surface gaps. (Roediger & Karpicke 2006, testing effect.)
```

- [ ] **Step 4: Build + verify.** `/opt/homebrew/bin/go build . && /opt/homebrew/bin/go test ./agent/ ./handler/` → build OK, tests pass. (If a test snapshots the exact AGENTS.md/prompt text, update the expected string to match.)

- [ ] **Step 5: Commit.**
```bash
git add agent/sandbox.go agent/agent.go CLAUDE.local.md
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho commit -m "feat(pedagogy): interleave older material into the session-open recall (ADR 0009)"
```

### Task 1.3 — Deploy Phase 1 (binary + disk-file sync)

- [ ] **Step 1: Build linux binary.** `GOOS=linux GOARCH=amd64 /opt/homebrew/bin/go build -o /tmp/study-app-linux .`
- [ ] **Step 2: Deploy binary** (carries `sandbox.go`/`agent.go`): scp to `~/stack/study-app/bin/study-app.new`, back up, swap, `systemctl --user restart study-app.service`, confirm both services `active`.
- [ ] **Step 3: Sync disk files to VPS** (NOT carried by the binary):
```bash
scp CLAUDE.local.md nanoclaw:/home/eduardo/stack/study-app/CLAUDE.local.md
# study-step-complete skill → its mounted path on the VPS (confirm the live path first):
ssh nanoclaw 'find ~/stack/study-app -path "*skills/study-step-complete/SKILL.md"'
# scp skills/study-step-complete/SKILL.md to the path that command reveals
```
  Read [[claw_study_pedagogy_layer]] / `claw_study_service.md` to confirm the exact skill-mount path before copying.
- [ ] **Step 4: Verify live.** After one chat turn, dump the generated AGENTS.md (`ssh nanoclaw 'LATEST=$(ls -t ~/stack/study-app/data/agent-sessions/ | head -1); cat ~/stack/study-app/data/agent-sessions/$LATEST/AGENTS.md'`) and confirm Rule 6 shows the interleaving wording. Manually: finish a task → tutor stops (no "Next up"); open a new session → exactly one recall check.
- [ ] **Step 5: Push.** `git push`

---

## PHASE 2 — Reading flow + position-aware plumbing

This plumbing is shared with Phase 3 (reading-tied-to-task), so it lands once.

### Task 2.1 — Wire session ↔ PDF ↔ page

**Files:** `agent/db.go`, `agent/types.go`, `handler/pdf.go`, `static/pdf.js`
Currently `session.last_pdf_id` is never set; the frontend PUTs page turns to `/pdf/progress/{id}` (`handler/pdf.go:160` `handlePDFProgress` → `UpdatePDFProgress`), which updates only the `pdfs` table.

- [ ] Add `UpdateSessionPDF(sessionID, pdfID int64, page int) error` to `agent/db.go` (write `last_pdf_id`, `last_page` on `sessions`). Mirror the existing `UpdateSessionTopic` style.
- [ ] When the viewer opens a PDF inside an active session, persist the link. Read `static/pdf.js` to find where a PDF is opened/loaded and where the active session id is available (`getActiveSessionId()` from `sessions.js`); on open, call a small endpoint that invokes `UpdateSessionPDF`. Extend the progress PUT to also update the session's `last_page` when there's an active session, or add the session id to that call. Follow the existing handler/endpoint patterns; **read the current `handlePDFProgress` + `pdf.js` open path first.**
- [ ] Build + test; commit.

### Task 2.2 — `<reading_state>` block in the tutor prompt

**File:** `handler/chat_v2.go` (`buildPiPrompt`, ~line 330, which already builds `<plan_state>`)
- [ ] Extend `buildPiPrompt` to also emit a `<reading_state>` block when the session has a `last_pdf_id`: PDF name, current page, total pages — e.g. `<reading_state pdf="Chapter 8 - PHI ETA" page="72/104"/>`. Source the values from the session + `pdfs` row. Pass the needed data into `buildPiPrompt` (it currently takes `courseID, clawCLIPath, userMessage`; add reading state, sourced in the handler before the call).
- [ ] If the legacy `/chat` path's system prompt (`GetSessionSystemPrompt`) should also carry reading state, add it there too; otherwise note it as Pi-only.
- [ ] Build + test (update any prompt snapshot tests); commit.

### Task 2.3 — Segmented active-reading prompts

**Files:** `skills/resource-orientation/SKILL.md` (+ sync to VPS)
- [ ] Update the orientation skill so that, for a 🔴 Read task, the tutor: proposes **page-range chunks** for long readings (short ones stay whole); keeps the existing pre-read prediction per chunk (generation effect); after each chunk, prompts a **boundary recall / self-explanation** and **verifies the learner's page** (from `<reading_state>`) before accepting "done"; ends with a full recall + confidence. Explicitly discourage passive rereading (Dunlosky 2013). Reference position-verification (manual + position-verified per ADR 0012).
- [ ] Sync the skill file to the VPS mount path; commit.

### Task 2.4 — Fix `pdf_open` analytics (smaller, can trail)

**Files:** `handler/pdf.go`
- [ ] `pdf_open` currently fires only on **upload** (`handler/pdf.go:88`). Record a read/open event when a PDF is actually opened for viewing in a session (e.g. on the first progress PUT of a session-pdf pair, or a dedicated open call), without spamming one per page turn. Pick the lowest-noise hook; **read the open flow first**. Commit.

### Task 2.5 — Deploy Phase 2
- [ ] Build + deploy binary; sync `skills/resource-orientation/SKILL.md` (and any other touched disk file) to the VPS; verify a read task now runs segmented and the tutor references your page; push.

---

## PHASE 3 — IA rebuild (own design pass before code)

Plan rail as the navigation spine (course switcher → phase → task), the active task's workspace in the center, reading on the right auto-opened to the task's resource, a **Scratch** area for non-task chats; the flat session list retired. **Needs its own design/spec session**, principally the data model: anchor `sessions` to a task (a `task_id`/plan-task reference), migrate existing topic-named sessions into their tasks, route non-task sessions to Scratch, and decide how the plan rail reads plan + completion state. Reuses the ADR-0008 accordion mechanics for the rail. Hide `verifier-stats`/pipeline sessions from the human rail here.

## PHASE 4 — Settings UI (steering)

Per-course settings panel (goal/framing, exam style, tutor pace, "stop after each task") writing deterministically to source of truth (course JSON / `agent_memory`), read into AGENTS.md generation; remove the agent's config-writing-to-generated-files behavior (ADR 0010). After Phase 3.

## PHASE 5 — Deferred

Full retrieval queue (R1) + SM-2 spacing (R2) — promote the prompt-only interleaving to a persisted, scheduled queue once small daily sessions are habitual.

---

## Self-Review

- ADR 0009 → Tasks 1.1 (stop) + 1.2 (interleaved opener). ✓
- ADR 0012 → Tasks 2.1–2.3 (position plumbing + segmented prompts); 2.4 fixes the analytics blind spot. ✓
- ADR 0010 → Phase 4. ADR 0011 → Phase 3 (flagged for its own design pass — not faked as buildable here). ✓
- Sync constraint called out in every deploy task (binary vs disk files). ✓
- Phase 1 steps carry exact files, exact new text, and the three Rule-6 carriers are kept consistent. ✓
