# Agent Onboarding — plugging a new tutor into marginalia

How marginalia hosts an AI agent, what the agent sees, and the questions you need
to answer before the tutor can work with a new learner.

---

## Architecture — how the agent plugs in

marginalia spawns one agent subprocess per chat turn. The browser sends a
message via SSE; the Go handler starts the agent, feeds it context, and streams
events back.

```
Browser ──SSE──▶ marginalia (Go) ──spawn──▶ pi --mode rpc ──▶ LLM API
                       │                          │
                       │                          ├── bash ──▶ claw-cli ──▶ marginalia DB
                       │                          │
                       │                          └── read/write ──▶ sandbox dir
```

The agent runs inside a **sandbox** — an ephemeral working directory at
`data/agent-sessions/<session-id>/` containing:

```
data/agent-sessions/<session-id>/
├── AGENTS.md          # generated context: memory, plan, skills, pedagogy rules
├── notes/             # scratch space
├── out -> ../../agent-out/  # symlink for generated artifacts
└── .pi-session.jsonl  # session tree
```

The agent is scoped to this directory. It cannot read the database, corpus, or
plan files directly — all structured state goes through `claw-cli`.

**What you need to swap in a different agent runtime:**

- The agent must speak JSON-RPC on stdin/stdout (the handler in
  `handler/chat_v2.go` writes a JSON prompt line and reads JSON event lines).
- The agent must have a `bash` tool so it can call `claw-cli`.
- The handler's `spawnPi` function in `handler/chat_v2.go` is the integration
  point — change the command and arguments there.
- The AGENTS.md template in `agent/sandbox.go` is the context generator — modify
  `writeAgentsMD` if your agent has different context expectations.

---

## What the agent receives (AGENTS.md)

Every session, `agent/sandbox.go` generates an `AGENTS.md` from these pieces,
in order:

1. **Course memory** — learner profile, observations, course context, feedback
   rules. Loaded via `claw-cli memory load --session <id> --user <id> --course <c>`.
   Populated by `seed-memory` from a directory of markdown files.

2. **Session block** — the session ID and a first-turn instruction to set the
   topic.

3. **Skill index** — one-line trigger hints for each pedagogy skill
   (`course-study-path`, `resource-orientation`, `study-step-complete`,
   `study-notes`, `pair-coding`, `by-hand`).

4. **Study plan section** (study sessions only) — how to read and mutate the
   plan via `claw-cli plan status` / `claw-cli plan toggle` /
   `claw-cli plan rewrite`. JSON-only, markdown plans are retired.

5. **PDF/slides section** — how to discover and extract PDF pages.

6. **Knowledge components** — how to search, create, and list atomic notes.

7. **Pedagogy rules** — 15 mandatory rules governing every turn (see below).

8. **Tool honesty section** — never fabricate tool output, flag missing tools.

---

## The learner profile — questions to answer before studying

Before the tutor can work with a new learner, the memory store needs a profile.
Seed it by writing markdown files and running `seed-memory`. The profile should
answer:

### Who they are

- **Academic level** (undergrad, master's, PhD, self-directed)?
- **Field and subfield** (e.g. CS → distributed systems, pure math → type theory)?
- **Career goal** (PhD program, industry role, personal enrichment)?
- **Time budget** (how many hours/week, what time of day, are sessions
  pre-scheduled or opportunistic)?

### How they learn

- **Abstract → concrete, or concrete → abstract?** Do they want the formal
  foundation first, or the engineering application?
- **Reading pace** — do they skim for structure or read every line?
- **Note-taking style** — formal LaTeX, conversational Q&A, fleeting scaffolds?
- **What burns them out?** Long lectures? Too many new terms? Too little
  structure?
- **What keeps them engaged?** Theoretical depth? Practical application?
  Connecting ideas across courses?

### What they're studying

- **Active courses** — one entry per course with:
  - Course name and syllabus summary
  - Primary resources (textbooks, papers, slide decks)
  - Learning goal (exam prep, conceptual understanding, skill building)
  - Where notes live (LaTeX file path, markdown directory, etc.)
  - Any specific framing ("teach this like interview prep, not academic survey")

### What to avoid

- Topics or framings that demotivate them (e.g. "don't use ODE/dynamical systems
  framing", "don't assign generic reading lists")
- Pace violations (e.g. "don't chain tasks without asking")
- Anything they've explicitly asked the tutor not to do

### Example memory file

```markdown
## Learner Profile

Master's student in CS. Advisor in formal methods. Core interests: type theory,
category theory, logic. Career goal: PhD in theoretical CS.

Works full-time at a software company. Study sessions are 25-45 min, 3-5x/week,
mostly evenings. Intrinsically motivated — this is by choice, not requirement.

## Study Preferences

- Abstract → concrete: formal foundation first, then application.
- Notes in LaTeX, conversational Q&A style. Fleeting notes are scaffolds.
- Avoid: ODE/dynamical systems framing, numerical methods, generic reading lists.
- Do not chain tasks — stop after one unless they say "keep going."

## Active Courses

### Algorithms
- CLRS + Kleinberg-Tardos. Goal: PhD qualifying-exam depth.
- Notes: data/courses/algorithms/notes.tex
- Framing: proof-first, then implementation. Skip LeetCode-style optimization.
```

---

## Course setup — what the agent needs per course

Each course the learner studies needs:

1. **A course registration** — `claw-cli course create --id <slug> --name "<name>"`
2. **A study plan** — a JSON file at `data/plans/<slug>.json` with phases and
   tasks. Each task has:
   - `id` (UUIDv4, stable across renames)
   - `title` and `description`
   - `bloom_level` (remember, understand, apply, analyze, evaluate, create)
   - `kind` (optional: `revisit` for spaced-retrieval interleaving)
3. **Corpus materials** — markdown files in `data/corpus/` (indexed on startup
   for RAG) or PDFs uploaded through the UI.
4. **Course settings** — `claw-cli course settings set --course <slug> --key <k> --value <v>`:
   - `framing` — how to teach this course (e.g. "exam-prep first, conceptual depth")
   - `exam_style` — assessment format (e.g. "proof-based written exam")
   - `chunk_pages` — max pages per reading chunk
   - `stop_after_task` — whether the tutor stops after each task completion
   - `interleaving` — whether the opener pulls from other courses
   - `mastery_threshold` — minimum confidence to gate completion (0.0–1.0)

5. **An interests log** — `memory/courses/<slug>/interests.md` for tangential
   curiosities to surface later.

---

## Pedagogy rules — what the agent must follow

The 15 rules in the generated AGENTS.md are non-negotiable. They implement
evidence-backed pedagogy from the learning-science literature. Here's what each
rule demands and what the learner author must know:

| Rule | What it does | Learner-side assumption |
|------|-------------|------------------------|
| 1 — No lecturing | Max 3–4 sentences, then ask learner to react | Learner wants Socratic dialogue, not monologue |
| 2 — Prior knowledge | Always ask "what do you already know?" first | Learner has relevant prior knowledge to activate |
| 3 — Scored retrieval | Score gist recall (produced ÷ total idea-units), log silently | Learner accepts formative scoring without seeing numbers |
| 4 — Connect to prior | Tie every new concept to something already covered | Courses have enough connective tissue to do this |
| 5 — Bloom progression | Explain → apply → analyze → evaluate → create; don't skip | Learner wants depth, not surface coverage |
| 6 — Session opener | One light move: retrieval if due, else prediction only | Learner tolerates the opener ritual every session |
| 7 — Pre-Read prediction | Predict key idea before reading; no reveal until after | Learner will actually predict, not skip to reading |
| 8 — Term budget | Max 3 new technical terms per turn | Learner absorbs vocabulary in small batches |
| 9 — Atomic capture | Learner writes KC body; one atom per idea; capture triggers | Learner is willing to distill ideas into their own words |
| 10 — Stop after task | Finish one task, then stop (or offer next if setting allows) | Sessions are one-task units by design |
| 11 — Cue, don't complete | On partial answer, give one cue before filling | Learner benefits from retrieval effort, not spoon-feeding |
| 12 — Session-close recall | Free recall at session end; name gaps, don't drill them | Learner will actually do the recall |
| 13 — Elaborative interrogation | Follow statements with "why is this true?" | Learner tolerates relentless why-chains |
| 14 — Sourced gap-fill | Fill gaps from corpus/PDF only, never from memory | Corpus is comprehensive enough to cover most gaps |
| 15 — Interleaved discrimination | When confusable concepts exist, test selection among them | Learner has built enough atoms for families to exist |

### Tuning rules for a different learner

Most rules are universal (evidence-backed, not preference-driven). Three are
tunable per course via Steering settings:

- **Rule 9 chunking** — `chunk_pages` (default 12). Lower for dense material,
  higher for narrative.
- **Rule 10 stop-after-task** — `stop_after_task` (default true). Turn off if
  the learner wants the tutor to chain tasks.
- **Rule 6 interleaving** — `interleaving` (default false). Turn on to pull
  retrieval from other courses at session open.

---

## Agent personality — voice and constraints

The tutor is **conversational, not clinical**. Its voice should be:

- **Direct** — short sentences, no academic hedging ("might be worth
  considering…"), no padding.
- **Curious** — asks why, presses one layer deeper, credits good reasoning.
- **Humble about gaps** — never fills a knowledge gap from its own memory
  without citing a source. If no source covers it, names the gap explicitly.
- **Aware of fatigue** — if the learner signals tiredness or stalls on multiple
  cues, the tutor steps back ("let's stop here and pick this up tomorrow").

The tutor is **never**:

- A cheerleader ("great job!" is fine; "you're doing AMAZING keep it up!!"
  is not).
- A lecturer who narrates content before the learner has read it.
- A quiz-master who announces scores or runs multi-round drills.
- An oracle who invents answers — every claim is grounded in a source or left
  as an open question.

---

## Quickstart — 5-minute new-learner setup

```bash
# 1. Write a learner profile (example above) to memory/profile.md
# 2. Create the first course
claw-cli course create --id algorithms --name "Algorithms"

# 3. Build a study plan (use the course-study-path skill or write JSON directly)
claw-cli plan rewrite --course algorithms --plan-file my-plan.json

# 4. Add corpus materials
mkdir -p $VAULT_ROOT/data/corpus/courses/algorithms
cp my-notes.md $VAULT_ROOT/data/corpus/courses/algorithms/

# 5. Seed the agent's memory
seed-memory --source ./memory --user default

# 6. Start the app and open a session
go run .   # then visit http://localhost:8081
```

---

## Environment — what the agent runtime needs

Set in `.env` (or the environment):

| Var | Purpose |
|-----|---------|
| `LLM_API_KEY` | Bearer token for the OpenAI-compatible chat + embeddings API |
| `LLM_API_URL` | Base URL (defaults to `https://api.openai.com/v1`) |
| `LLM_MODEL` | Chat model ID (e.g. `gpt-4o-mini`) |
| `EMBEDDING_MODEL` | Embedding model for the RAG corpus index |
| `VAULT_ROOT` | Root directory for `data/` (DB, corpus, plans), `memory/`, and agent sandboxes |
| `AUTH_TOKEN` | If set, gates all routes except `/login`. Empty = open (local dev) |
| `CLAW_CLI_PATH` | Path to the `claw-cli` binary (the agent calls this via bash) |
