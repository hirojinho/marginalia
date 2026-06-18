# Agent Runtime — Pi Integration

> **Status:** Proposed (2026-05-10) — not yet implemented. Binding decisions in [ADR 0006](../adr/0006-embed-pi-as-agent-runtime.md). Implementation sequencing in [`2026-05-10-agent-runtime-pi-impl.md`](./2026-05-10-agent-runtime-pi-impl.md). Once shipped, this section's status flips to *Active* and the file becomes canonical reference.

## Motivation

Today's `/chat` is a single OpenCode call with a fixed list of function tools. It cannot run skill protocols, accumulate memory, dispatch multi-step work, or verify before completion. The May 2026 grill-me audit established that the gap between this app and Eduardo's Claude Code workflow is the runtime fabric, not the LLM. This spec defines how to embed an agent runtime — Pi, MIT-licensed, the substrate behind OpenClaw — to close that gap without committing to Anthropic's per-token billing or rebuilding the runtime in Go.

## Architecture

```
Browser ──HTTP/SSE──▶ claw-study (Go) ──spawn per turn──▶ pi --mode rpc ──▶ OpenCode Go API
                            │                                  │
                            │                                  ├─bash──▶ claw-cli (Go) ──▶ claw-study state
                            │                                  │
                            │                                  └─read/write/edit/bash──▶ data/agent-sessions/<id>/
                            │                                                            data/agent-out/
                            │
                            └─event translation──▶ Browser (SSE)
```

**Roles.**

- **`claw-study`** owns auth, sessions, plans, RAG corpus, PDF state, persistence, observability, the chat UI. It spawns Pi per chat turn and translates Pi's event stream into SSE.
- **Pi (RPC subprocess)** runs the agent loop: LLM call, tool dispatch, skill loading, compaction, session tree. One subprocess per `/chat-v2` request.
- **`claw-cli`** is a small Go binary exposing `claw-study`'s domain operations as CLI subcommands. Pi invokes it via its `bash` tool. It calls back into `claw-study` (over a localhost HTTP loopback or a shared SQLite file, depending on collocation choice in implementation).

## Pi process lifecycle

- **Per-turn spawn.** `/chat-v2`'s POST handler starts `pi --mode rpc --provider opencode-go --model <selected>`, sends a JSONL `prompt` command on stdin, streams events from stdout, terminates after the agent's turn ends.
- **Cold start budget.** Roughly 2–3 s for Node + Pi to boot, plus LLM streaming. Total per-turn latency target: under 15 s for a typical turn. Per-session warm process is a v2 optimisation.
- **Concurrency.** One concurrent Pi per `(user, session)` pair; second concurrent request rejected with 409. Single-user assumption holds for v1.
- **Failure modes.** Pi exits non-zero, stalls past 60 s, or its stdout closes unexpectedly. Each surfaces as a structured SSE `error` event; the chat UI shows the existing error banner. The Go side reaps the subprocess unconditionally on handler exit (deferred kill + waitpid).

## Sandbox shape

Per-session ephemeral working directory at `~/stack/study-app/data/agent-sessions/<session-id>/`:

```
data/agent-sessions/<session-id>/
├── AGENTS.md            # generated at sandbox creation by `claw-cli memory load`
├── notes/               # agent scratch space; not promoted anywhere
├── out -> ../agent-out  # symlink: where artifacts (PDFs, exports) get dropped
└── .pi-session.jsonl    # Pi's tree-shaped session file
```

- Pi launches with `cwd=<this dir>`, so `read`, `write`, `edit`, and `bash` are scoped here.
- The agent **cannot** read `study.db`, `data/corpus/`, or `data/plans/` directly. All structured state goes through `claw-cli`. The sandbox cwd never contains real DB files.
- Sandbox is created at the first `/chat-v2` turn for a session, reused for subsequent turns within that session, deleted when the session is deleted (or after N idle days, garbage-collected by a periodic job).

## Memory and `AGENTS.md`

Pi auto-loads `AGENTS.md` from `cwd` (and ancestor dirs). The per-session `AGENTS.md` is generated at sandbox creation:

```
claw-cli memory load --session <id> --course <id> --user <id> > AGENTS.md
```

Content assembled by the loader, in this order:

1. **User profile** — short, stable: who the user is, study program, role.
2. **Course context** — for the active session's course: study-plan summary, current focus area, learning objectives.
3. **Active feedback memories** — style, density, voice rules relevant to this course (e.g. for CE-297: formalization-hook rule, no-abbreviations rule; for DSA: descriptive variable names, data-structure-semantics-first).
4. **Recent activity slice** — last one or two sessions on this course, brief.
5. **Available skills** — list with one-line triggers, lifted from skill frontmatter.

`AGENTS.md` is regenerated for each new session. Within a session it is **not** re-generated mid-conversation — Pi treats it as static once loaded. Cap at roughly 3 KB; prefer recency-relevant and scope-relevant memories.

The memory store itself is owned by `claw-study`. New table in `study.db`:

```sql
CREATE TABLE agent_memory (
  id          INTEGER PRIMARY KEY,
  user_id     TEXT NOT NULL,
  course_id   TEXT,            -- nullable: scope-global memories
  kind        TEXT NOT NULL,   -- 'profile' | 'feedback' | 'project' | 'reference'
  title       TEXT,
  body        TEXT NOT NULL,
  created_at  INTEGER NOT NULL,
  updated_at  INTEGER NOT NULL
);
CREATE INDEX agent_memory_scope ON agent_memory (user_id, course_id, kind);
```

Initial seed: import the relevant memories from `~/.claude/projects/<project-slug>/` (the user-profile and feedback files, scoped course memories, course interests). Future writes happen via `claw-cli memory save`.

## Skill catalog (v1)

Six study-core skills, ported from `~/.claude/skills/` to Pi's skill format. Stored in `skills/` at the repo root and mounted via `pi --skills-dir`:

| Skill | Source | Status |
|---|---|---|
| `study-notes` | `~/.claude/skills/study-notes/SKILL.md` | port |
| `course-study-path` | `~/.claude/skills/course-study-path/SKILL.md` | port |
| `study-step-complete` | `~/.claude/skills/study-step-complete/SKILL.md` | port |
| `resource-orientation` | `~/.claude/skills/resource-orientation/SKILL.md` | port |
| `by-hand` | `~/.claude/skills/by-hand/SKILL.md` | port |
| `pair-coding` | `~/.claude/skills/pair-coding/SKILL.md` | port |

Pi's skill format is markdown plus YAML frontmatter; the existing skills are largely compatible. Porting work per skill: rewrite Claude-Code-specific tool references (`Skill`, `TodoWrite`, etc.) to Pi-equivalents (slash commands, `claw-cli`, bash), strip Claude-Code-specific protocol affordances that don't translate, validate the skill loads via `pi --skills-dir=./skills` and is discoverable via `/skills`.

Skills are auto-invoked by the model when their frontmatter description matches the user's intent, or manually via `/skill:name`.

**Out of scope for v1:** career skills (`interview-prep`, `resume-tailoring`, `candidate-profile`, `job-application-prep`, `job-hunt`) — port later. `commit-and-push` is permanently out of scope (the agent does not ship code from `claw-study`).

## `claw-cli` surface

A Go binary at `~/stack/study-app/bin/claw-cli`, on the agent's `$PATH`. Wraps existing `agent/tools_*.go` functions where they exist; introduces new helpers for memory and notes. All subcommands write JSON to stdout for parsing; errors go to stderr with non-zero exit.

| Subcommand | Wraps | Purpose |
|---|---|---|
| `memory load --session <id> --course <id> --user <id>` | new (this spec) | Generate AGENTS.md content |
| `memory save --kind <k> --course <c> --title <t>` | new | Store a feedback / project / reference memory |
| `memory search <query> [--course <c>]` | new | Search memory store |
| `rag search <query> [--course <c>] [--top-k N]` | `tools_rag.go` | RAG over corpus |
| `plan show --course <c>` | `tools_plan.go` | Read current plan as JSON |
| `plan toggle --course <c> --task <id>` | `tools_plan.go` | Mark a task done / undone |
| `course interests --course <c>` | `tools_skill.go` | Load course interests file |
| `note save --course <c> --kind fleeting --content <text>` | new | Save a fleeting note |
| `pdf extract --id <id> [--pages <range>]` | `tools_pdf.go` | Extract text from a stored PDF |
| `web fetch <url>` | `tools_web.go` | Fetch a URL (existing rate limit applies) |
| `skill dispatch --skill <name> --topic <t> --course <c>` | `tools_skill.go` | Run one of the existing prompt-template skills (legacy compat for v1) |

The CLI is the only path agent code takes to `claw-study` state.

## Model selection

Configured via `~/stack/study-app/.env`. Defaults:

| Skill / context | Default model |
|---|---|
| General chat | `deepseek-v4-pro` |
| Note-heavy skills (`study-notes`, `resource-orientation`) | `kimi-k2.6` |
| Cheap quick lookups | `glm-5.1` |

Per-skill override: `AGENT_MODEL_<skill_uppercase>=<model-id>` in `.env`. Per-turn UI footer exposes the model used, token usage, and estimated cost (input + output tokens × per-million rate from a local rate table).

## Streaming SSE event vocabulary

Today's `/chat` SSE events: `event: token`, `event: reasoning`, `event: done`.

`/chat-v2` translates Pi's RPC event stream into a richer set:

| SSE event | Source (Pi event type) | Payload |
|---|---|---|
| `token` | `text_delta` | `{delta: string}` |
| `reasoning` | `thinking` (delta) | `{delta: string}` |
| `tool_start` | `tool_call` | `{name, input_summary}` |
| `tool_end` | `tool_result` | `{name, output_summary, ok}` |
| `skill_start` | `skill_invocation` | `{name}` |
| `compaction` | `compaction` | `{reason}` |
| `model_change` | `model_change` | `{from, to}` |
| `done` | turn end | `{usage: {input, output, cost}}` |
| `error` | exit / timeout | `{message}` |

Browser renders accordingly: tool calls become a collapsible inline panel, skills get a chip header, reasoning interleaves with tokens (closes the inline-reasoning ROADMAP item naturally because Pi emits the events in production order). The "surface tool calls" ROADMAP item is also closed by this design.

## Rollout

- New endpoint `/chat-v2` ships behind the env var `AGENT_RUNTIME=pi|legacy`.
- `/chat` (legacy) stays untouched.
- Browser hits `/api/runtime` at boot to discover which mode is active.
- After two weeks of daily use of `pi` mode, cut over: delete the legacy `/chat`, remove the flag, rename `/chat-v2` to `/chat`.
- Rollback at any time: flip the env var, restart, legacy mode resumes.

## Out of scope (v1)

- **Per-session warm Pi process.** v2 perf optimisation; ship after measuring real cold-start cost.
- **Multi-user.** Single-tenant for now. The schema (`agent_memory.user_id`) is multi-user-ready.
- **Live edits to the user's laptop work tree.** Regime α — artifacts only. The agent generates new files into `data/agent-out/`; integrating them into `formal_notes.tex` stays a laptop-side activity.
- **Career skills.** Port later.
- **Web search beyond `web fetch <url>`.** Pi has no built-in search; defer.
- **HTML snippets, interactive templates, agent-generated PDFs, agent-administered test questions.** Separate ROADMAP items, orthogonal to the runtime decision; design them once the runtime exists.
