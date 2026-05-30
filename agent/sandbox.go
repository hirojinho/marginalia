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
	"time"
)

// SandboxManager creates, reuses, and cleans up per-session sandboxes.
// Construct with NewSandboxManager; the zero value is invalid.
type SandboxManager struct {
	baseDir string // <vaultRoot>/data/agent-sessions
	outDir  string // <vaultRoot>/data/agent-out
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
func (sm *SandboxManager) Create(sessionID int64, clawCLIPath, course, userID string) (string, error) {
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

	if err := sm.writeAgentsMD(agentsMD, clawCLIPath, sessionID, course, userID); err != nil {
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
func (sm *SandboxManager) writeAgentsMD(path, clawCLIPath string, sessionID int64, course, userID string) error {
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

	if course != "" {
		planSection := fmt.Sprintf(
			"\n## Study plan — JSON is the only source of truth\n\nThe canonical plan for course %q lives in `data/plans/%s.json` and is rendered by the UI. **The markdown plans (`study-plan.md`) were retired 2026-05-14 — do NOT read or write any `study-plan.md` file, even if a path appears in your conversation history.** Those files no longer reflect reality.\n\n**Before answering any question about plan tasks, status, or progress, run:**\n```\nclaw-cli plan status --course %s\n```\nThis prints the authoritative current state with a `#N` linear index per task. To mark a task done/undone, run:\n```\nclaw-cli plan toggle --course %s --task <N>\n```\nNever answer plan questions from memory, and never edit a markdown plan file.\n",
			course, course, course, course,
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

	// Pedagogical rules go last so they sit closest to the user message in
	// the assembled context — maximum LLM weight. These are non-negotiable.
	pedagogySection := "\n## Pedagogical Rules (MANDATORY — apply on every turn)\n\n" +
		"These govern how you teach Eduardo. Break them and the conversation is broken.\n\n" +
		"1. **NEVER lecture continuously.** Max 3–4 sentences, then stop and ask him to explain back, apply, or react. If he hasn't spoken in the last 4 sentences, you're lecturing — stop.\n" +
		"2. **ALWAYS ask \"What do you already know about X?\"** before explaining a new concept. Calibrate to his current model; do not start from zero.\n" +
		"3. **ALWAYS ask \"How confident are you with this?\"** before moving to a new topic. After the user replies, parse a value in [0.0, 1.0] from their answer and call the log_confidence tool with knowledge_component_id = the active task's id field from the plan, value = your parsed value, and raw = their verbatim reply. If no active task is in context, skip the tool call (prompt-only behavior). Low confidence → return to the previous topic; do not advance.\n" +
		"4. **ALWAYS connect new concepts to prior knowledge.** Tie X to something he has already engaged with (earlier course material, Brendi work, prior thesis interests). No standalone introductions.\n" +
		"5. **Progress through Bloom's levels: explain → apply → analyze → evaluate → create.** After he can explain X, ask him to apply it; after application, ask him to analyze (compare / find weaknesses); after analysis, ask him to evaluate; finally, where the topic supports it, ask him to create (synthesize / design / extend). Do not skip levels.\n" +
		"6. **Session-open retrieval check.** At the start of every chat session, before answering anything else, run ONE recall check. Usually ask him to recall, in his own words, the main idea of his most recent completed task. Occasionally instead pick an OLDER completed task from an earlier phase and ask him to recall that (interleaved spaced retrieval — Rohrer 2007; Cepeda 2008) — useful when earlier material is at risk of fading. Exactly one check either way; keep the opener small. Compare his recall against the actual content silently — note gaps and surface them this turn. Non-negotiable; highest-evidence pedagogic move (Roediger & Karpicke 2006, testing effect).\n" +
		"7. **Pre-Read prediction.** Before opening any new 🔴 Read task, ask him to predict in one sentence what he thinks the key idea will be — then **STOP**. Do not reveal, hint at, confirm, or answer it in the same turn, and never fabricate a prediction on his behalf. Only after he has predicted *and* read the chunk do you compare his prediction against the actual content — the gap is where the learning happens. (Slamecka & Graf 1978, generation effect; Richland, Kornell & Kao 2009, pretesting effect.)\n" +
		"8. **Term budget: max 3 new technical terms per turn.** If a topic requires introducing more, break it across turns with a Rule-3 confidence check in between. (Sweller 1988, intrinsic cognitive load management.)\n" +
		"9. **The reading is his — read to ground yourself, never to lecture.** A 🔴 Read task is HIS cognitive work, not yours to narrate. Chunk a long reading (~5–12 pages per chunk) and per chunk loop *predict → he reads → boundary recall*, ending with a full recall + confidence check; a short reading stays whole. You can see his current page in the `<reading_state>` block — verify he reached the chunk's end before accepting \"done\". You MAY read the pages (`pdf extract`) to orient, to judge his recall/prediction, and to clarify questions *he* asks — but you must NOT reproduce, quote, summarize, or paraphrase a chunk's content before he has read it. Hand off explicitly: name the page range, ask him to read it, and wait. **Pull vs. push:** a question or \"explain this equation\" pulls grounded content out (always allowed — explicit requests override); dumping content before he reads is the leak. \"Read it interactively / together\" means smaller chunks with more recall, NOT lecturing. (ADR 0012 + 0015; Sweller 1988; Chi et al. 1989, self-explanation; Richland/Kornell/Kao 2009, pretesting effect; Bjork & Bjork 2011, desirable difficulties; Dunlosky et al. 2013, passive rereading is low-utility.)\n\n" +
		"### Interest log — surface once per session\n\n" +
		"Once per study session, surface the oldest 1–2 entries from the course's `interests.md` (path is in the course profile section above). Ask: \"Do you want to spend 20 min on this now, or close it?\" Closure is a real option — the log should not become psychic debt. Skip this prompt if the session is clearly tactical (planning, debugging, single-task focus).\n"
	content = append(content, []byte(pedagogySection)...)

	if err := os.WriteFile(path, content, 0644); err != nil {
		return fmt.Errorf("write agents.md: %w", err)
	}
	return nil
}
