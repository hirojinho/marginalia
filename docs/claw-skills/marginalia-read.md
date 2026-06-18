---
name: marginalia-read
description: Use when the user asks read-only questions about their marginalia state — plan progress on a course (biology, cs101, algorithms, history, research), recent session activity, what page they were on in a PDF, or to find notes in their memory/corpus. Triggers on "what's next on", "how much is left", "last session", "what did I cover", "what page was I on", "find my notes about", or any course ID.
---

You answer questions about the user's marginalia app by reading from the shared mount at `/workspace/marginalia/`. **Read-only. Never write.**

## Course aliases

| Phrasing | Course ID |
| --- | --- |
| "Biology", "bio", "cells" | `biology` |
| "CS 101", "intro CS", "computer science" | `cs101` |
| "Algorithms", "algo" | `algorithms` |
| "World History", "history" | `history` |
| "Research", "independent research" | `research` |

If the user names a course not in this table, list the known IDs and ask which they meant.

## Query routing

Pick the source by query class:

- **Plan progress** ("what's next on X", "how much of X is left")
  Read `/workspace/marginalia/data/plans/<id>.json` and `/workspace/marginalia/memory/courses/<id>/study-plan.md`. The JSON gives `done/total` and the next incomplete tasks; the markdown adds narrative context.

- **Session list / counts** ("how many sessions on cs101", "list my recent sessions")
  ```
  sqlite3 /workspace/marginalia/data/study.db "SELECT id, course_id, topic, updated_at FROM sessions ORDER BY updated_at DESC LIMIT 10"
  ```

- **Last messages of a session** ("what did I cover last on recursion")
  Find the session id first, then:
  ```
  sqlite3 /workspace/marginalia/data/study.db "SELECT role, content FROM messages WHERE session_id=? ORDER BY id DESC LIMIT 6"
  ```

- **PDF reading state** ("what page was I on", "last PDF")
  ```
  sqlite3 /workspace/marginalia/data/study.db "SELECT id, original_name, last_page, last_read_at FROM pdfs ORDER BY last_read_at DESC LIMIT 5"
  ```

- **Memory / corpus search** ("find my notes about recursion", "do I have anything on Big-O")
  ```
  grep -ril "<term>" /workspace/marginalia/memory /workspace/marginalia/data/corpus
  ```
  Then `head -20` the most relevant hits and quote 1–2 lines of context each.

## Output style

Short plain text suitable for a chat message. No JSON dumps, no code fences for routine answers, no headings unless the answer needs sections.

Examples:

```
Plan: Biology — 12/30 done.
Next 3:
- Read the photosynthesis chapter
- Diagram the Calvin cycle
- Write up cellular-respiration notes
```

```
Last cs101 session: 2026-05-08 — "Recursion". 14 messages.
```

```
3 notes mention recursion:
- memory/courses/cs101/recursion.md — "Base case stops the recursion..."
- data/corpus/courses/cs101/algorithms.md — "Recursion vs iteration"
- memory/research/complexity.md — "Recursion and Big-O analysis"
```

## Errors

- **`database is locked`**: marginalia is mid-write. Wait 1 second, retry once. If it fails again, reply "DB busy, try again in a moment."
- **No matching row / file**: plain English ("no PDF reading history yet"), no stack traces.
- **`sqlite3` not available**: fall back to `python3 -c "import sqlite3; ..."` — the container has both.

## Hard refusal

If the user asks you to **mutate** anything — mark a task done, save a note, edit a plan, write a fleeting note, deploy code, run a build — refuse. Reply: "I can only read here. Mutating skill (`marginalia-notes`) is on the roadmap." Do not run any command outside the read paths above. Do not attempt edits via shell tricks.
