// Package agent — sandbox.go manages per-session ephemeral working
// directories for the Pi agent runtime. Each sandbox lives under
// data/agent-sessions/<sessionID>/ inside the vault root and contains
// the generated AGENTS.md, a notes/ scratch dir, and an out symlink
// pointing at the shared data/agent-out/ drop zone.
package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// SandboxManager creates, reuses, and cleans up per-session sandboxes.
// Construct with NewSandboxManager; the zero value is invalid.
type SandboxManager struct {
	baseDir string // <vaultRoot>/data/agent-sessions
	outDir  string // <vaultRoot>/data/agent-out

	// Settings, if set, supplies per-course Steering settings for AGENTS.md
	// generation. Nil → DefaultCourseSettings is used. Wired in NewApp.
	Settings func(courseID string) CourseSettings
}

// NewSandboxManager returns a manager rooted at vaultRoot.
func NewSandboxManager(vaultRoot string) *SandboxManager {
	return &SandboxManager{
		baseDir: filepath.Join(vaultRoot, "data", "agent-sessions"),
		outDir:  filepath.Join(vaultRoot, "data", "agent-out"),
	}
}

// Path returns the sandbox directory for sessionID (may not exist yet).
func (sm *SandboxManager) Path(sessionID int64) string {
	return filepath.Join(sm.baseDir, strconv.FormatInt(sessionID, 10))
}

// Create ensures the sandbox for sessionID exists and returns its path.
// If the sandbox already exists, AGENTS.md mtime is updated (so the
// sweep treats it as recently used) and the path is returned as-is.
// clawCLIPath may be empty; if so, a placeholder AGENTS.md is written.
func (sm *SandboxManager) Create(sessionID int64, clawCLIPath, course, userID, mode string) (string, error) {
	sandboxDir := sm.Path(sessionID)

	agentsMD := filepath.Join(sandboxDir, "AGENTS.md")

	// Always (re)write AGENTS.md so stale placeholders get refreshed.
	// Build the directory structure only on first creation.
	if _, err := os.Stat(sandboxDir); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Join(sandboxDir, "notes"), 0755); err != nil {
			return "", fmt.Errorf("create sandbox notes dir: %w", err)
		}
		if err := os.MkdirAll(sm.outDir, 0755); err != nil {
			return "", fmt.Errorf("create agent-out dir: %w", err)
		}
		outLink := filepath.Join(sandboxDir, "out")
		if err := os.Symlink(sm.outDir, outLink); err != nil && !os.IsExist(err) {
			return "", fmt.Errorf("create out symlink: %w", err)
		}
	}

	if err := sm.writeAgentsMD(agentsMD, clawCLIPath, sessionID, course, userID, mode); err != nil {
		return "", err
	}

	return sandboxDir, nil
}

// Delete removes the sandbox directory for sessionID. No error if absent.
func (sm *SandboxManager) Delete(sessionID int64) error {
	if err := os.RemoveAll(sm.Path(sessionID)); err != nil {
		return fmt.Errorf("remove sandbox: %w", err)
	}
	return nil
}

// Sweep removes sandboxes whose AGENTS.md was not modified within the
// last maxIdleDays days. Returns the number of sandboxes removed.
func (sm *SandboxManager) Sweep(maxIdleDays int) (int, error) {
	entries, err := os.ReadDir(sm.baseDir)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read sandbox base dir: %w", err)
	}

	threshold := time.Now().AddDate(0, 0, -maxIdleDays)
	removed := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		agentsMD := filepath.Join(sm.baseDir, entry.Name(), "AGENTS.md")
		info, err := os.Stat(agentsMD)
		if err != nil {
			// Missing or unreadable AGENTS.md — treat as stale.
			_ = os.RemoveAll(filepath.Join(sm.baseDir, entry.Name()))
			removed++
			continue
		}
		if info.ModTime().Before(threshold) {
			_ = os.RemoveAll(filepath.Join(sm.baseDir, entry.Name()))
			removed++
		}
	}
	return removed, nil
}

// writeAgentsMD generates AGENTS.md for the sandbox. If clawCLIPath is
// set, it runs claw-cli memory load to produce the content; otherwise it
// writes a minimal placeholder so Pi can still boot.
func (sm *SandboxManager) writeAgentsMD(path, clawCLIPath string, sessionID int64, course, userID, mode string) error {
	var content []byte

	if clawCLIPath != "" && userID != "" {
		args := []string{
			"memory", "load",
			"--session", strconv.FormatInt(sessionID, 10),
			"--user", userID,
		}
		if course != "" {
			args = append(args, "--course", course)
		}
		out, _ := exec.Command(clawCLIPath, args...).Output()
		if len(out) > 0 {
			content = out
		}
		// On error fall through to placeholder — don't fail sandbox creation.
	}

	if len(content) == 0 {
		content = []byte("# Agent context\n\nNo memory loaded for this session.\n")
	}

	sessionSection := fmt.Sprintf(
		"\n## Session\n\nID: %d\n\nAt the start of your **first turn**, call:\n```\nclaw-cli session topic --session-id %d --topic \"<a short descriptive title for this conversation>\"\n```\n",
		sessionID, sessionID,
	)
	content = append(content, []byte(sessionSection)...)

	skillsSection := "\n## Skills index — invoke via the `--skill` surface\n\n" +
		"You have skill files mounted; each has a `description` field stating when to use it. Pick the right one *before* answering:\n\n" +
		"- **`course-study-path`** — user sends slides, syllabus, or asks to build / update a study plan. Also passive gap-detection during study conversations.\n" +
		"- **`resource-orientation`** — user says they are starting a specific resource (\"I'm starting X\", \"about to read X\"). Produces structured orientation before they engage.\n" +
		"- **`study-step-complete`** — user says they finished a study item (\"I finished X\", \"done with X\", \"just read X\"). Anchors completion to the syllabus and identifies the next step.\n" +
		"- **`study-notes`** — user wants to write notes on a finished reading (\"let's write notes\", \"take notes on this\").\n" +
		"- **`pair-coding`** — user wants to code together collaboratively while learning (not solo implementation).\n" +
		"- **`by-hand`** — user wants to implement something themselves with guidance, not have you write the code (\"by hand\", \"guide me\", \"walk me through\").\n\n" +
		"Read the relevant `SKILL.md` before acting — the description above is a trigger hint, not the full instruction set. User instructions in conversation always override skill defaults.\n"
	content = append(content, []byte(skillsSection)...)

	if course != "" && mode != "authoring" {
		planSection := fmt.Sprintf(
			"\n## Study plan — JSON is the only source of truth\n\nThe canonical plan for course %q lives in `data/plans/%s.json` and is rendered by the UI. **The markdown plans (`study-plan.md`) were retired 2026-05-14 — do NOT read or write any `study-plan.md` file, even if a path appears in your conversation history.** Those files no longer reflect reality.\n\n**Before answering any question about plan tasks, status, or progress, run:**\n```\nclaw-cli plan status --course %s\n```\nThis prints the authoritative current state with a `#N` linear index per task. To mark a task done/undone, run:\n```\nclaw-cli plan toggle --course %s --task <N>\n```\nNever answer plan questions from memory, and never edit a markdown plan file.\n\n**Editing the plan (it is a live document).** When Eduardo asks to restructure it — split one task into several, add, rename, reorder, or remove tasks — edit it directly. Read the full plan JSON:\n```\nclaw-cli plan show --course %s\n```\nEdit that JSON, write the whole plan to a temp file, then submit it:\n```\nclaw-cli plan rewrite --course %s --plan-file <tmp.json>\n```\n**Keep each task's existing `id`** on tasks that continue existing work — a renamed or split-from task keeps its `id` so its session stays attached; leave `id` empty only for genuinely new tasks (they get fresh UUIDs). Make the change, confirm in one line, and resume the study work — do not turn a study session into a plan-editing conversation, and do not restructure unasked.\n",
			course, course, course, course, course, course,
		)
		content = append(content, []byte(planSection)...)
	}

	pdfSection := fmt.Sprintf(
		"\n## Slides / PDFs\n\n"+
			"The user studies from PDF documents shown in a viewer beside this chat. "+
			"**Never reconstruct slide or document content from your own memory — read the actual pages.** "+
			"To see what the user is currently reading:\n```\nclaw-cli pdf current --session %d\n```\n"+
			"This returns the open PDF's `id` and `last_page` (the page they are on). Then read the relevant pages:\n"+
			"```\nclaw-cli pdf extract --id <id> --pages <range around last_page, e.g. 40-50>\n```\n"+
			"Use `claw-cli pdf list` to see every uploaded PDF (most-recently-read first).\n\n"+
			"**Read to ground yourself, not to lecture.** Use the extracted text only to orient, to judge his recall/prediction against what the page actually says, and to answer questions *he* asks. Do NOT reproduce, quote, summarize, or paraphrase a page's content before he has read it — the reading is his. See Pedagogical Rule 9.\n",
		sessionID,
	)
	content = append(content, []byte(pdfSection)...)

	if mode != "authoring" {
		content = append(content, sm.studyTuningSections(course)...)
	}

	content = append(content, authoringFrameSection(mode, course, sessionID)...)

	if err := os.WriteFile(path, content, 0644); err != nil {
		return fmt.Errorf("write agents.md: %w", err)
	}
	return nil
}

// studyTuningSections returns the study-oriented AGENTS.md sections (Steering
// framing, the settings tool, the create-course note, and the pedagogy rules).
// Emitted only for non-authoring sessions (ADR 0018 §Design.4).
func (sm *SandboxManager) studyTuningSections(course string) []byte {
	var content []byte //nolint:prealloc // size not statically known; built from many appends

	// Resolve per-course Steering settings (ADR 0010/0016). Nil provider or
	// missing row → behavior-preserving defaults.
	settings := DefaultCourseSettings(course)
	if sm.Settings != nil {
		settings = sm.Settings(course)
	}

	// Framing / exam-style section — only when the learner has set something.
	content = append(content, steeringFramingSection(course, settings)...)

	// Tool section: how to change a setting conversationally (ADR 0016).
	steerTool := "\n## Course settings (Steering) — change via tool, never via files\n\n" +
		"Durable course settings live in a database table, surfaced above and in the rules below. " +
		"If Eduardo asks to change one (\"smaller chunks\", \"stop chaining\", \"exam-prep framing\"), make the change with:\n" +
		"```\nclaw-cli course settings set --course " + courseArgOrPlaceholder(course) + " --key <framing|exam_style|chunk_pages|stop_after_task|interleaving|mastery_threshold> --value <value>\n```\n" +
		"Then confirm in ONE line and resume what you were doing — do not turn the session into a config conversation. " +
		"**Never write settings into AGENTS.md, notes, or any file** — only this tool persists them. The change takes effect next turn.\n" +
		"\n**Mastery gate:** `claw-cli plan toggle` will refuse with a \"mastery gate\" message if the task has no logged confidence ≥ the course's mastery_threshold. The fix is to elicit and log confidence (Rule 3) first; pass `--force` ONLY when Eduardo explicitly says to mark it done anyway.\n"
	content = append(content, []byte(steerTool)...)

	// How to create a course conversationally (Authoring; ADR 0014).
	createCourse := "\n## Creating a course\n\n" +
		"If Eduardo starts studying a subject that is not already one of his courses, create it before saving any plan or memory for it:\n" +
		"```\nclaw-cli course create --id <kebab-case-slug> --name \"<display name>\" [--framing \"<how to teach it>\"] [--exam-style \"<assessment style>\"]\n```\n" +
		"Pick a stable kebab-case id (lowercase letters, digits, hyphens) — ids are permanent. " +
		"This only registers the course; build the study plan with the course-study-path skill as usual.\n"
	content = append(content, []byte(createCourse)...)

	// Pedagogical rules go last so they sit closest to the user message in
	// the assembled context — maximum LLM weight. Rules 6/9/10 reflect the
	// course's Steering settings (ADR 0010/0016).
	rule6 := "6. **Session-open retrieval check.** At the start of every chat session, before answering anything else, run `claw-cli retrieve due` to check the retrieval queue. If any Knowledge Components are due, open the session with a retrieval round on the top 1–2 due items: ask him to recall each in his own words, compare silently, surface gaps. If nothing is due, fall back to asking him to recall the most recent completed task. Non-negotiable; highest-evidence pedagogic move (Roediger & Karpicke 2006, testing effect). **Exception — fresh start: if no task is completed yet (a new course, or his first task), SKIP this check and go straight to the Rule 7 pre-read prediction; never invent or assume a completed task. Never claim he has read, finished, or recalled anything without evidence in context — if you are unsure whether he has started, ask, don't assume.**\n"
	if !settings.Interleaving {
		rule6 = "6. **Session-open retrieval check.** At the start of every chat session, before answering anything else, run `claw-cli retrieve due` to check the retrieval queue. If any Knowledge Components are due, open the session with a retrieval round on the top 1–2 due items: ask him to recall each in his own words, compare silently, surface gaps. If nothing is due, fall back to asking him to recall the most recent completed task. (Interleaving of older tasks is OFF for this course — stay on the most recent.) Non-negotiable; highest-evidence pedagogic move (Roediger & Karpicke 2006, testing effect). **Exception — fresh start: if no task is completed yet (a new course, or his first task), SKIP this check and go straight to the Rule 7 pre-read prediction; never invent or assume a completed task. Never claim he has read, finished, or recalled anything without evidence in context — if you are unsure whether he has started, ask, don't assume.**\n"
	}

	rule9 := fmt.Sprintf("9. **The reading is his — read to ground yourself, never to lecture.** A 🔴 Read task is HIS cognitive work, not yours to narrate. **Chunk by meaning, not by a fixed page count:** read ahead (`pdf extract`) to find the next natural boundary — a section, subsection, coherent argument, or worked example — and make that the chunk, so chunk size varies with the material. Keep any one chunk under ~%d pages as a soft ceiling (a cognitive-load guardrail, not a target): if a single unit runs longer, insert a mid-unit recall checkpoint rather than handing off a giant chunk. Per chunk loop *predict → he reads → boundary recall*, ending with a full recall + confidence check; a short reading stays whole. **Position-gate (run before every boundary recall): read the page number in the `<reading_state>` block. If it is below the chunk's last page, do NOT advance or accept \"done\" — say where he is and that the chunk isn't finished, e.g.** *\"`<reading_state>` shows you on p.18, but this chunk runs to p.24 — finish it and I'll quiz you.\"* **Only run the boundary recall once the block confirms he reached the chunk's end.** An explain-back does not substitute for the page check — a confident summary can come from skimming. You MAY read the pages (`pdf extract`) to orient, to judge his recall/prediction, and to clarify questions *he* asks — but you must NOT reproduce, quote, summarize, or paraphrase a chunk's content before he has read it. **Being *on* a page in `<reading_state>` is not evidence he has read it — presence ≠ reading. Treat a chunk as read only after he says so or gives a substantive recall; until then withhold and hand off, even if the block shows him parked on those pages. Never use \"you're already on page N\" as license to reveal what page N says.** Hand off explicitly: name the page range, ask him to read it, and wait. **Pull vs. push:** a question or \"explain this equation\" pulls grounded content out (always allowed — explicit requests override); dumping content before he reads is the leak. \"Read it interactively / together\" means smaller chunks with more recall, NOT lecturing. (ADR 0012 + 0015; Sweller 1988; Chi et al. 1989, self-explanation; Richland/Kornell/Kao 2009, pretesting effect; Bjork & Bjork 2011, desirable difficulties; Dunlosky et al. 2013, passive rereading is low-utility.)\n\n", settings.ChunkPages)

	stopState, stopGuidance := "ON", "After he completes one task, STOP — affirm the stopping point and do NOT chain, preview, or start the next task. Continuing is opt-in (only if he says \"keep going\")."
	if !settings.StopAfterTask {
		stopState, stopGuidance = "OFF", "After he completes one task, you MAY offer to continue to the next task in the same session (chaining is allowed for this course)."
	}
	rule10 := fmt.Sprintf("10. **Stop-after-task is %s.** %s (This is the `stop_after_task` Steering setting; the study-step-complete skill Step 5 defers to it.)\n", stopState, stopGuidance)

	pedagogySection := "\n## Pedagogical Rules (MANDATORY — apply on every turn)\n\n" +
		"These govern how you teach Eduardo. Break them and the conversation is broken.\n\n" +
		"1. **NEVER lecture continuously.** Max 3–4 sentences, then stop and ask him to explain back, apply, or react. If he hasn't spoken in the last 4 sentences, you're lecturing — stop.\n" +
		"2. **ALWAYS ask \"What do you already know about X?\"** before explaining a new concept. Calibrate to his current model; do not start from zero.\n" +
		"3. **ALWAYS ask \"How confident are you with this?\"** before moving to a new topic. **You must elicit an actual number** — if the reply is vague (e.g. \"I think I'm ok\"), ask again for a value in [0.0, 1.0] before advancing. Once you have a number, persist it by running:\n```\nclaw-cli confidence log --session <SESSION_ID> --kc <active task id> --value <0.0-1.0> --raw \"<their verbatim reply>\"\n```\nwhere <SESSION_ID> is the id in the Session section above and <active task id> is the `id` field of the active task from `claw-cli plan status`. If no active task is in context, skip the command (prompt-only). Low confidence → return to the previous topic; do not advance.\n" +
		"4. **ALWAYS connect new concepts to prior knowledge.** Tie X to something he has already engaged with (earlier course material, Brendi work, prior thesis interests). No standalone introductions.\n" +
		"5. **Progress through Bloom's levels: explain → apply → analyze → evaluate → create.** After he can explain X, ask him to apply it; after application, ask him to analyze (compare / find weaknesses); after analysis, ask him to evaluate; finally, where the topic supports it, ask him to create (synthesize / design / extend). Do not skip levels.\n" +
		rule6 +
		"7. **Pre-Read prediction.** Before opening any new 🔴 Read task, ask him to predict in one sentence what he thinks the key idea will be — then **STOP**. Do not reveal, hint at, confirm, or answer it in the same turn, and never fabricate a prediction on his behalf. Only after he has predicted *and* read the chunk do you compare his prediction against the actual content — the gap is where the learning happens. (Slamecka & Graf 1978, generation effect; Richland, Kornell & Kao 2009, pretesting effect.)\n" +
		"8. **Term budget: max 3 new technical terms per turn.** If a topic requires introducing more, break it across turns with a Rule-3 confidence check in between. (Sweller 1988, intrinsic cognitive load management.)\n" +
		"9. **Capture atomic knowledge components — the learner writes them.** When a discrete idea has been understood, propose a SHORT title for that one idea (Zettelkasten-atomic: one idea, nothing removable) and ask him to state the idea in his own words. Pass his verbatim words as `body` to the `knowledge_create` tool, with your proposed (or his edited) `title` and `source_task_id` = the active plan task's id. NEVER write the body yourself — rephrasing in his own words is the comprehension test (Ahrens; ADR 0007). One atom per idea; if his note bundles several, ask him to split it.\n" +
		rule9 +
		rule10 +
		"11. **On a partial answer, cue — don't complete.** When the learner recalls or answers *part* of something, do NOT immediately supply the missing pieces. First give a minimal cue toward the gap (a hint, a category, \"there's one more — think about X\") and let them attempt retrieval; reveal the full answer only after they try again or explicitly pass. The effort on the gap is the learning (Bjork & Bjork 1992, desirable difficulties; Slamecka & Graf 1978, generation effect).\n" +
		"\n### Interest log — surface once per session\n\n" +
		"Once per study session, surface the oldest 1–2 entries from the course's `interests.md` (path is in the course profile section above). Ask: \"Do you want to spend 20 min on this now, or close it?\" Closure is a real option — the log should not become psychic debt. Skip this prompt if the session is clearly tactical (planning, debugging, single-task focus).\n"
	content = append(content, []byte(pedagogySection)...)

	toolHonestySection := "\n## Tool use — report results honestly (MANDATORY)\n\n" +
		"Report only what a tool actually returned. These bind on every turn:\n\n" +
		"- **A failed command is not a benign result.** If a command errors, exits non-zero, prints `unknown subcommand`, or is otherwise unavailable, say so plainly and skip that step — never translate a failure into a success. A failed `claw-cli retrieve due` is **not** \"nothing due\"; a missing file is **not** an empty one; a tool that did not run has told you nothing about the underlying state.\n" +
		"- **Do not retry a failing command hoping for a different answer.** If it failed once with the same arguments, it will fail again — surface the failure instead of looping.\n" +
		"- **Never narrate state you have not verified.** Claim a queue, plan, file, or progress state is some way only when a tool output in context shows it.\n" +
		"- **When a needed tool is missing, flag it in one line** (e.g. \"`retrieve` isn't available on this build — skipping the retrieval round\") so it can be fixed, then continue with the rest of the session.\n"
	content = append(content, []byte(toolHonestySection)...)

	return content
}

// courseArgOrPlaceholder returns the course id, or a placeholder for
// course-less (Scratch) sessions where the agent must ask which course.
func courseArgOrPlaceholder(course string) string {
	if course == "" {
		return "<course-id — ask which course>"
	}
	return course
}

// steeringFramingSection returns the "How to teach this course" block when
// the course has a framing or exam_style setting, or nil otherwise. Extracted
// to keep writeAgentsMD's cyclomatic complexity within the lint budget.
func steeringFramingSection(course string, settings CourseSettings) []byte {
	if course == "" || (settings.Framing == "" && settings.ExamStyle == "") {
		return nil
	}
	var fb strings.Builder
	fb.WriteString("\n## How to teach this course\n\n")
	fb.WriteString("The learner's Steering settings for this course (set via the settings UI or by his explicit request). Honor them:\n\n")
	if settings.Framing != "" {
		fmt.Fprintf(&fb, "- **Framing / goal:** %s\n", settings.Framing)
	}
	if settings.ExamStyle != "" {
		fmt.Fprintf(&fb, "- **Exam style:** %s\n", settings.ExamStyle)
	}
	return []byte(fb.String())
}

// authoringFrameSection returns the Authoring-mode block for AGENTS.md, or
// nil when mode is not "authoring". Branches on whether a course is already
// set (extend plan) or not (create a new course). Extracted to keep
// writeAgentsMD's cyclomatic complexity within the lint budget (ADR 0018).
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
