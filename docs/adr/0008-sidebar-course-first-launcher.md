# 0008 — Session sidebar is a course-first launcher, not a navigator

- **Status:** Superseded by [ADR 0011](0011-plan-is-navigation-spine.md)
- **Date:** 2026-05-29

> **Superseded (2026-05-29):** Usage data and an information-architecture review
> the same day showed the *core thesis here is wrong* — the session list should
> not be the primary navigator at all. The **Plan** is the navigation spine; a
> Session becomes a Task's workspace. See [ADR 0011](0011-plan-is-navigation-spine.md).
> What survives from this ADR: the truncation fix, the accordion tree mechanics
> (repurposed for the plan rail), and async titling (now for Scratch chats).

## Context

The original session sidebar was a fixed 260px drawer that grouped Sessions by
Course but never collapsed, never sorted by recency, and gated creation behind a
modal (pick course → type topic → create). In practice the learner **starts a
fresh Session for almost every task and seldom resumes an old one** (see the
*Session* entry in [CONTEXT.md](../../CONTEXT.md)). So the drawer optimised the
rare act (browsing old Sessions) and taxed the constant one (creating a new
Session). It also forced scrolling through every Course's every Session at once.

This inverts the usual chat-app assumption (ChatGPT/Claude foreground a
recency-ordered list because resumption is the norm there). It is not the norm
here.

## Decision

Treat the drawer as a **course-first launcher with an archive underneath**, not
a session navigator.

- **No recents/continue list.** Resumption is rare; a recents row is dead weight.
- **Single-open Course accordion.** All defined Courses are always listed (even
  empty ones, so any Course can be launched into) with a Session count. One
  Course expands at a time; expansion state persists across reloads; the active
  Session's Course is open on load.
- **Per-Course `+` creates instantly** — no modal, no required typing. The new
  Session becomes active immediately.
- **Titles are LLM-generated, async, from the first user message alone.** The
  row shows "Untitled" until the title returns (~1s). This keeps the archive
  scannable without taxing the learner — consistent with the Authoring
  principle (the app absorbs management friction). Manual inline rename remains.
- Session rows are title-only and newest-first within a Course.

## Consequences

- The constant action (create) drops to one click; the rare action (find an old
  Session) stays possible via the archive but is no longer privileged.
- One extra cheap LLM call (glm-5.1) per Session on first message. Fire-and-
  forget; a failed title just leaves "Untitled".
- Deterministic-title fallback (first-message truncation) was rejected for
  scannability; revisit if title cost or latency ever bites.
- If usage shifts toward resuming long-lived Sessions, this model is wrong and
  a recency-first list should return — the accordion would then be the surprise.
- Empty Courses are always shown so the `+` launcher is uniform; the Course set
  is the fixed list in `courseMeta`.
