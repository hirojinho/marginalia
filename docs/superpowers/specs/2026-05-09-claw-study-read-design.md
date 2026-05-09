# claw-study-read skill — design

**Date:** 2026-05-09
**Status:** approved
**Tier:** A (read-only) of the larger claw-task-execute decomposition (A read, B notes, C HTTP, D deploy)

## Purpose

A SKILL.md hot-reloaded into Claw (the Telegram agent at `@HiroClawdBot`) so Eduardo can ask read-only questions about his claw-study state from his phone. No mutation, no deploys, no code execution beyond read commands.

## Install path

`/home/eduardo/stack/nanoclaw-v2/data/v2-sessions/ag-1777924890168-e1etn1/.claude-shared/skills/claw-study-read/SKILL.md`

The skill-watcher polls `.claude-shared/skills/` every 5s and kills the agent container when a new skill appears, so the next Telegram message picks it up. No restart needed.

## Activation

Frontmatter `description` triggers Claw on phrasings like:

- "what's next on", "what's my next task", "how much is left"
- "last session", "what did I cover"
- "what page was I on", "what PDF did I last open"
- "find my notes about", "do I have anything on"
- any of the known course IDs: `ce297`, `ddia`, `dsa-interview`, `software-arch`, `thesis`

## Behaviour

The skill instructs Claw to classify the query and pick a source:

| Query class            | Primary source                                                           |
| ---------------------- | ------------------------------------------------------------------------ |
| Plan progress          | `data/plans/<id>.json` + `memory/courses/<id>/study-plan.md`             |
| Session list / counts  | `sqlite3 data/study.db "SELECT ... FROM sessions"`                       |
| Last messages          | `sqlite3 data/study.db "SELECT ... FROM messages WHERE session_id=..."`  |
| PDF reading state      | `sqlite3 data/study.db "SELECT ... FROM pdfs ORDER BY last_read_at"`     |
| Memory / corpus search | `grep -ril <term> memory data/corpus`                                    |

All paths under `/workspace/study-app/` (the container's mount of the study-app vault).

HTTP API access is mentioned as a fallback for cases where listening on the host gateway is set up later, but for Tier A everything goes through the filesystem and SQLite. Defer HTTP to Tier C.

## Course alias resolution

The skill includes a small alias table so Claw maps natural phrasings to course IDs:

- "CE-297" / "Safety" / "STPA" → `ce297`
- "DDIA" / "Designing Data-Intensive" / "data-intensive" → `ddia`
- "DSA" / "interview prep" → `dsa-interview`
- "Software Architecture" / "softarch" → `software-arch`
- "Thesis" / "Phase 1" / "survey" → `thesis`

If a query doesn't resolve to a known course, Claw lists the known ones in the reply.

## Output style

Telegram-friendly: short plain text, no JSON dumps, no markdown code fences for routine answers. Examples:

```
Plan: CE-297 — 12/30 done.
Next 3:
- Read Leveson Chapter 4
- Run AppSTPA on RTOS Scheduler
- Write up CAST notes
```

```
Last DDIA session: 2026-05-08 — "Replication topology". 14 messages.
```

## Hard guardrails

The skill text MUST contain a refusal clause: if asked to mutate (mark a task done, save a note, edit a plan, deploy code), Claw refuses and points at the eventual Tier B `claw-study-notes` skill. No exceptions.

## Error handling

- `study.db` locked (study-app mid-write): retry once after 1s; if still locked, reply "DB busy, try again in a moment". With WAL + busy_timeout=5s, this is unlikely but documented.
- Course ID not in `KnownCourses`: list the known ones.
- File or row not found: plain English ("no PDF reading history yet"), not error stack traces.

## Testing

Purely manual via Telegram after install. Suggested probe set:

1. "What's next on CE-297?" → plan summary with next 3 tasks
2. "How many sessions did I have on DDIA?" → integer count + breakdown
3. "What page was I on in the last PDF?" → filename + page
4. "Find my notes on STPA" → list of paths + 1-line context per hit
5. (negative) "Mark task 3 of CE-297 done" → must refuse and reference Tier B

Pass criteria: all positive queries answer within ~5s, the negative case is refused.

## Deferred / out of scope

- Tier B (`claw-study-notes`): writing fleeting notes, toggling plan tasks, editing memory.
- Tier C (`claw-study-api`): hitting the HTTP API for things the filesystem doesn't expose well (RAG vector search, chat).
- Tier D (`claw-study-deploy`): build / scp / systemctl on the repo.
- Auth on the study-app HTTP layer (memory says this comes before the named cloudflared tunnel, separately tracked).
