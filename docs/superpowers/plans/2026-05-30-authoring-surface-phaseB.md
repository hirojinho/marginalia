# Authoring Surface — Phase B Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the existing-course Authoring flow — a per-course "Design plan" entry, a dedicated rail Design section, an authoring frame that *extends* an existing plan (not creates a course), and suppression of study framing for `mode=authoring`.

**Architecture:** `writeAgentsMD` extracts the study-tuned sections into a `studyTuningSections` helper called only when `mode != "authoring"`, and `authoringFrameSection` branches on `course` (extend vs create). The rail splits `mode=authoring` sessions out of Scratch into a Design bucket (below the plan) with a "+ design plan" entry mirroring Phase A's `startNewCourseAuthoring`. `study-app`-only; reuses CreateSession + the Phase A frame.

**Tech Stack:** Go 1.26 (`/opt/homebrew/bin/go`), vanilla-JS SPA. Tests: `SandboxManager` via `NewSandboxManager` + read AGENTS.md (`agent/sandbox_test.go`); frontend via local preview + curl/CDP.

---

## File Structure

- **`agent/sandbox.go`** — extract `studyTuningSections` (lines 178–235) + gate it on `mode`; gate `planSection`; branch `authoringFrameSection` on `course`. *(Task 1)*
- **`agent/sandbox_test.go`** — existing-course authoring frame + study-suppression tests. *(Task 1)*
- **`static/rail.js`** — `designSessions` split in `loadRailData`; Design bucket + "+ design plan" in `renderOther`; `startDesignPlan`. *(Task 2)*
- **`static/app.js`** — `design-plan` dispatcher case + import. *(Task 2)*

---

## Task 1: Mode-aware AGENTS.md (suppress study framing; branch the authoring frame)

**Files:** `agent/sandbox.go`, `agent/sandbox_test.go`

- [ ] **Step 1: Write the failing tests** — append to `agent/sandbox_test.go`:

```go
func TestWriteAgentsMDExistingCourseAuthoringFrame(t *testing.T) {
	sm := NewSandboxManager(t.TempDir())
	path, err := sm.Create(201, "", "ce297", "", "authoring")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(path, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	s := string(body)
	if !strings.Contains(s, "plan rewrite --course ce297") {
		t.Fatalf("existing-course authoring frame should reference plan rewrite for the course:\n%s", s)
	}
	if strings.Contains(s, "course create --session") {
		t.Fatalf("existing-course authoring must NOT tell the agent to create a course:\n%s", s)
	}
	if strings.Contains(s, "Pedagogical Rules (MANDATORY)") {
		t.Fatalf("authoring session must NOT carry the study pedagogy rules:\n%s", s)
	}
	if strings.Contains(s, "Course settings (Steering)") {
		t.Fatalf("authoring session must NOT carry the steering tool section:\n%s", s)
	}
}

func TestWriteAgentsMDStudyKeepsPedagogy(t *testing.T) {
	sm := NewSandboxManager(t.TempDir())
	path, err := sm.Create(202, "", "ce297", "", "study")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(path, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(body), "Pedagogical Rules (MANDATORY)") {
		t.Fatalf("study session must still carry the pedagogy rules (gating over-reached):\n%s", body)
	}
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `cd ~/Documents/ITA/claw-study && /opt/homebrew/bin/go test ./agent/ -run 'TestWriteAgentsMDExistingCourseAuthoringFrame|TestWriteAgentsMDStudyKeepsPedagogy' -v`
Expected: `TestWriteAgentsMDExistingCourseAuthoringFrame` FAILS (today an authoring session with a course still emits pedagogy + steering, and `authoringFrameSection` ignores `course` so it prints the new-course "course create --session" text). `TestWriteAgentsMDStudyKeepsPedagogy` passes already (study unaffected) — that's fine, it's a guard against the next step over-reaching.

- [ ] **Step 3: Branch `authoringFrameSection` on `course`**

In `agent/sandbox.go`, replace the whole `authoringFrameSection` function (currently `func authoringFrameSection(mode string, sessionID int64) []byte`) with:

```go
func authoringFrameSection(mode, course string, sessionID int64) []byte {
	if mode != "authoring" {
		return nil
	}
	if course == "" {
		frame := "\n## You are in an Authoring session (designing a course)\n\n" +
			"This is not a study session — Eduardo wants to design a course/plan with you, generatively. " +
			"Use the `course-study-path` skill: grill the intent, research the resources, and build the study plan.\n\n" +
			"When the course is ready, create it (this also re-tags THIS chat to the new course):\n" +
			"```\nclaw-cli course create --session " + strconv.FormatInt(sessionID, 10) + " --id <kebab-slug> --name \"<display name>\"\n```\n" +
			"Then seed the plan's tasks (read it back, edit JSON, submit the whole plan):\n" +
			"```\nclaw-cli plan rewrite --course <kebab-slug> --plan-file <tmp.json>\n```\n" +
			"Pick a stable kebab-case id (ids are permanent). Keep task `id`s stable on later edits. " +
			"Confirm what you created in one or two lines and ask Eduardo to review.\n"
		return []byte(frame)
	}
	frame := "\n## You are in an Authoring session (designing this course's plan)\n\n" +
		"This is not a study session — Eduardo wants to design or extend the plan for course `" + course + "`, generatively. " +
		"Use the `course-study-path` skill: grill the intent, research the resources, and shape the tasks. **Do NOT create a course — `" + course + "` already exists.**\n\n" +
		"Read the current plan, edit the JSON, and submit the whole plan:\n" +
		"```\nclaw-cli plan show --course " + course + "\nclaw-cli plan rewrite --course " + course + " --plan-file <tmp.json>\n```\n" +
		"Keep each task's existing `id` on tasks that continue existing work (so their sessions stay attached); leave `id` empty only for genuinely new tasks. Confirm what you changed in one or two lines and ask Eduardo to review.\n"
	return []byte(frame)
}
```

- [ ] **Step 4: Update the `authoringFrameSection` call site**

In `writeAgentsMD`, find the call (just after the pedagogy section append, near the end):
```go
	content = append(content, authoringFrameSection(mode, sessionID)...)
```
and change it to pass `course`:
```go
	content = append(content, authoringFrameSection(mode, course, sessionID)...)
```

- [ ] **Step 5: Extract the study-tuned block into a helper**

In `agent/sandbox.go`, MOVE the contiguous block currently spanning the `// Resolve per-course Steering settings (ADR 0010/0016).` comment (line ~178) through the `content = append(content, []byte(pedagogySection)...)` line (line ~235) — i.e. the settings resolution, `steeringFramingSection`, `steerTool`, the `createCourse` section, the `rule6`/`rule9`/`rule10` construction, and the `pedagogySection` assembly+append — VERBATIM into a new method. Add this method (e.g. right after `writeAgentsMD`):

```go
// studyTuningSections returns the study-oriented AGENTS.md sections (Steering
// framing, the settings tool, the create-course note, and the pedagogy rules).
// Emitted only for non-authoring sessions — an Authoring session gets the
// Authoring frame instead (ADR 0018 §Design.4).
func (sm *SandboxManager) studyTuningSections(course string, sessionID int64) []byte {
	var content []byte
	// <-- paste the moved block here, but change every `content = append(content, X...)`
	//     to append to THIS local `content`, and end with `return content`.
	return content
}
```
When moving: the block already appends to a `content` slice; in the helper the local `var content []byte` accumulates the same way, then `return content`. Do NOT change any section text, ordering, the `settings`/`sm.Settings` resolution, or the `rule6`/`rule9`/`rule10` conditional logic — this must be behavior-preserving for study sessions.

Then, where the block used to be in `writeAgentsMD`, call it conditionally (preserving position — it sits after `pdfSection`, before the authoring-frame append):
```go
	if mode != "authoring" {
		content = append(content, sm.studyTuningSections(course, sessionID)...)
	}
```

- [ ] **Step 6: Gate the plan section**

In `writeAgentsMD`, the plan section guard (line ~157) is `if course != "" {`. Change it to:
```go
	if course != "" && mode != "authoring" {
```
(So an authoring session — even one with a course — does not get the study "plan status/toggle" block; the authoring frame tells it to use `plan show`/`plan rewrite` instead.)

- [ ] **Step 7: Run the new tests + build**

Run: `cd ~/Documents/ITA/claw-study && /opt/homebrew/bin/go test ./agent/ -run 'TestWriteAgentsMD' -v && /opt/homebrew/bin/go build .`
Expected: all `TestWriteAgentsMD*` PASS (the Phase A `TestWriteAgentsMDAuthoringFrame` — course-less authoring — still finds `course create --session 101`; the two new tests pass), build clean. If `golangci-lint` (pre-commit) flags `writeAgentsMD` cyclomatic complexity, the extraction in Step 5 should have *reduced* it; if it still complains, extracting Step 5's block was the intended remedy — confirm the block truly moved out.

- [ ] **Step 8: Full agent suite**

Run: `/opt/homebrew/bin/go test ./agent/ ./handler/`
Expected: `ok` (the existing `TestWriteAgentsMDParameterizesSteering` / `TestWriteAgentsMDUsesDefaultsWhenNoProvider` still pass, proving `studyTuningSections` is behavior-preserving for study mode).

- [ ] **Step 9: Commit**
```bash
cd ~/Documents/ITA/claw-study
git add agent/sandbox.go agent/sandbox_test.go
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho commit -m "$(cat <<'EOF'
feat(agent): authoring frame extends existing plans; suppress study framing

writeAgentsMD skips the study-tuned sections (pedagogy rules, plan toggle
block, steering) for mode=authoring; authoringFrameSection branches on course
(extend an existing plan vs create a new course). Completes ADR 0018 §Design.4.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: Per-course "Design plan" entry + rail Design section

**Files:** `static/rail.js`, `static/app.js`

Read the current `loadRailData` (line 38), `renderOther` (line 169), `renderSessionLine` (165), `startNewCourseAuthoring` (193), and the `app.js` dispatcher `switch` (the `case 'new-course':` at line 65) before editing — mirror those patterns.

- [ ] **Step 1: Add a `designSessions` bucket variable**

In `static/rail.js`, beside `let scratchSessions = [];` (line 21) add:
```js
let designSessions = []; // task-less, mode='authoring' (course design chats)
```

- [ ] **Step 2: Split authoring out of Scratch in `loadRailData`**

In `loadRailData`, reset it alongside the others (near line 54 `scratchSessions = [];`):
```js
    designSessions = [];
```
Then in the task-less `else` branch (lines 65-71), change:
```js
      } else {
        // task-less = Scratch family; show global (no course) + this course's
        const inScope = !s.course_id || s.course_id === selectedCourse;
        if (!inScope) continue;
        if (s.archived) archivedSessions.push(s);
        else scratchSessions.push(s);
      }
```
to:
```js
      } else if (s.mode === 'authoring') {
        // Authoring (Design) chats are course-specific: a course-tagged one
        // shows under its course; a course-less (new-course) one shows under General.
        if (s.course_id === selectedCourse && !s.archived) designSessions.push(s);
        else if (s.archived && (!s.course_id || s.course_id === selectedCourse)) archivedSessions.push(s);
      } else {
        // task-less scratch = global (no course) + this course's
        const inScope = !s.course_id || s.course_id === selectedCourse;
        if (!inScope) continue;
        if (s.archived) archivedSessions.push(s);
        else scratchSessions.push(s);
      }
```

- [ ] **Step 3: Render the Design bucket in `renderOther`**

In `static/rail.js` `renderOther` (line 169), add the Design bucket as the FIRST thing (before the Scratch bucket), so it renders below the plan and above Scratch. Insert right after `let html = '';`:
```js
  if (selectedCourse !== '') {
    html +=
      '<div class="rail-bucket"><div class="rail-other-label">Design' +
      ' <button class="rail-design-add" data-action="design-plan" title="Design or extend this course\'s plan" aria-label="Design plan">+</button></div>';
    for (const s of designSessions) html += renderSessionLine(s);
    html += '</div>';
  }
```
(The "+ design plan" action shows whenever a real course is selected, even with zero design chats yet.)

- [ ] **Step 4: Add `startDesignPlan`**

In `static/rail.js`, after `startNewCourseAuthoring` (ends line 218), add a sibling that mirrors it but tags the selected course:
```js
export async function startDesignPlan() {
  if (!selectedCourse) return;
  const courseName = (courseMeta[selectedCourse] && courseMeta[selectedCourse].name) || selectedCourse;
  try {
    const resp = await apiFetch('/api/sessions', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        course_id: selectedCourse,
        task_id: '',
        mode: 'authoring',
        topic: 'Design ' + courseName + ' plan',
      }),
    });
    if (!resp.ok) {
      const text = await resp.text();
      showErrorBanner('Failed to start design session: ' + text);
      return;
    }
    const session = await resp.json();
    await switchSession(session.id);
    await loadRail();
    const input = document.getElementById('message-input');
    if (input) input.focus();
  } catch (err) {
    showErrorBanner('Failed to start design session: ' + err.message);
  }
}
```

- [ ] **Step 5: Wire the dispatcher in `static/app.js`**

Add `startDesignPlan` to the `rail.js` import (beside `startNewCourseAuthoring` at line 22), and add a case beside `new-course` (line 65):
```js
    case 'design-plan':
      startDesignPlan();
      break;
```

- [ ] **Step 6: Build + local preview**

```bash
cd ~/Documents/ITA/claw-study
VAULT_ROOT=/tmp/claw-phaseB-vault LISTEN_ADDR=127.0.0.1:8096 AUTH_TOKEN= LLM_API_KEY=dummy AGENT_RUNTIME=pi /opt/homebrew/bin/go run . &
```

- [ ] **Step 7: Verify**

- `curl -s -X POST 127.0.0.1:8096/api/sessions -H 'Content-Type: application/json' -d '{"course_id":"ce297","task_id":"","mode":"authoring","topic":"Design CE-297 plan"}'` → response includes `"mode":"authoring"` and `"course_id":"ce297"`.
- `curl -s 127.0.0.1:8096/api/sessions` → that session is present with `mode=authoring`, `course_id=ce297`, `task_id` null.
- (If CDP available) load `http://127.0.0.1:8096`, select CE-297 in the switcher, confirm a "Design" bucket with a "+" appears below the plan; click it; confirm the chat switches and `/api/sessions` shows the new ce297 authoring session. Capture the result.
- Read the final `rail.js`/`app.js` to confirm the wiring (button `data-action="design-plan"`, dispatcher case, `startDesignPlan` reuses `apiFetch`/`switchSession`/`loadRail`/`showErrorBanner`).
- Kill the server (and any chrome) when done.

- [ ] **Step 8: Commit**
```bash
cd ~/Documents/ITA/claw-study
git add static/rail.js static/app.js
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho commit -m "$(cat <<'EOF'
feat(rail): per-course "Design plan" entry + dedicated Design section

Splits mode=authoring sessions out of Scratch into a Design bucket below the
plan; "+ design plan" opens a course-tagged authoring chat (ADR 0018, Phase B).

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: Deploy

**Files:** none. `study-app`-only (sandbox.go + static JS are both in the server binary); claw-cli unchanged this phase.

- [ ] **Step 1: Pre-deploy sync** — `cd ~/Documents/ITA/claw-study && git fetch origin && git rev-list --left-right --count origin/main...HEAD`. If behind, `git merge origin/main --no-edit` and re-run `/opt/homebrew/bin/go test ./...`.

- [ ] **Step 2: Push (requires explicit user OK — direct-to-main is harness-gated)** — `git push origin main`.

- [ ] **Step 3: Build study-app**
```bash
cd ~/Documents/ITA/claw-study
GOOS=linux GOARCH=amd64 /opt/homebrew/bin/go build -o /tmp/study-app-linux .
ls -la /tmp/study-app-linux
```

- [ ] **Step 4: Deploy with backup + restart**
```bash
scp -q /tmp/study-app-linux nanoclaw:/home/eduardo/stack/study-app/bin/study-app.new
ssh nanoclaw 'cd ~/stack/study-app/bin && cp study-app study-app.bak.2026-05-30-authoring-B && mv study-app.new study-app && chmod +x study-app && export XDG_RUNTIME_DIR=/run/user/$(id -u) && systemctl --user restart study-app.service && sleep 2 && systemctl --user is-active study-app.service'
```
Expected: `active`.

- [ ] **Step 5: Live smoke**
```bash
TOKEN=$(ssh nanoclaw 'grep ^AUTH_TOKEN= ~/stack/study-app/.env | cut -d= -f2')
# existing-course design session via API:
rtk proxy curl -s -X POST -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d '{"course_id":"ce297","task_id":"","mode":"authoring","topic":"Design CE-297 plan"}' https://study.claw-study.xyz/api/sessions
# confirm it lists; then open the UI, select CE-297, confirm the Design section + "+".
# cleanup the smoke session:
ssh nanoclaw "sqlite3 /home/eduardo/stack/study-app/data/study.db \"DELETE FROM sessions WHERE topic='Design CE-297 plan' AND mode='authoring';\""
```
Expected: POST returns `mode:authoring, course_id:ce297`; cleanup removes it.

- [ ] **Step 6: Update memory** — flip the Phase B "pending" note in `claw_study_experience_redesign.md` to shipped: per-course Design entry + rail Design section + authoring-frame branch + study-framing suppression, deployed; the Authoring surface (ADR 0018) is now complete.

---

## Self-Review Notes

- **Spec coverage:** Task 1 = mode-aware AGENTS.md (spec §Design.1: gate study sections + branch frame). Task 2 = per-course entry + rail Design section (spec §Design.2). Task 3 = deploy (spec §Deploy). The "suppress study framing" decision and the frame branch are both in Task 1.
- **Signature/consistency:** `authoringFrameSection(mode, course string, sessionID int64)` updated at its one call site; `studyTuningSections(course string, sessionID int64)` is the extracted study block called only when `mode != "authoring"`. Frontend `startDesignPlan` mirrors `startNewCourseAuthoring` (same helpers `apiFetch`/`switchSession`/`loadRail`/`showErrorBanner`), differing only in `course_id`/topic; `designSessions` is reset+populated in `loadRailData` and consumed in `renderOther`; `design-plan` action ↔ dispatcher case ↔ `startDesignPlan` import all align.
- **Placeholders:** none — the only "paste the moved block" instruction (Task 1 Step 5) is a verbatim-move refactor with explicit behavior-preservation constraints, not an invent-the-code placeholder.
- **Behavior preservation risk (Task 1 Step 5):** the study-section extraction is the one judgment-heavy edit; the existing steering tests + `TestWriteAgentsMDStudyKeepsPedagogy` are the guard, and the spec reviewer must diff the moved block against the original.
