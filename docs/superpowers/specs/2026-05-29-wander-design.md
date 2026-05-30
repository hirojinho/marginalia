# Wander — Design

**Date:** 2026-05-29
**Status:** Approved, pending implementation plan

## Problem / motivation

Eduardo wants to use phone downtime ("time to kill") for studying, but the kind
of studying that fits that slot is *not* reading new slides (that wants focus
and a screen). What fits is **passive, serendipitous reading** — the way he
already enjoys Wikipedia rabbit-holes. Crucially, real Wikipedia is sometimes
above his current level on a given topic, so he wants a gentler on-ramp that
*links out* to the real article once he has footholds.

He prefers **Telegram** (via Claw) over the web app, wants it **pull**-based
(he asks; it never spams), **maximally adventurous** (free to roam off-syllabus),
and explicitly *not* a quiz — pure reading.

See the **Wander** entry in [CONTEXT.md](../../../CONTEXT.md) for the canonical
term and how it relates to *Studying*, *Steering*, and *Scratch*.

## Goal

A pull-triggered Telegram experience where Claw hands Eduardo one short,
level-appropriate explainer card seeded from his study material, lets him chase
rabbit-hole threads by replying, and points him at the real Wikipedia article as
the "exit."

## Non-goals

- **No pedagogy.** Wander deliberately bypasses the 8 mandatory tutoring rules
  (no recall/confidence/Bloom). Pure reading. (If it tried to teach Socratically
  it would become homework and he'd stop opening it.)
- **No Telegram buttons / no NanoClaw fork.** Navigation is numbered replies —
  see [ADR 0013](../../adr/0013-wander-numbered-reply-no-telegram-button-fork.md).
- **No new Go code and no `claw-cli` changes.** Everything needed already exists.
- **No push/scheduled delivery** in v1 (pull only). A HEARTBEAT-driven nudge is
  a possible later addition, explicitly out of scope here.
- **No web-app surface.** Telegram only.

## Design

### Nature of the artifact

A **pure Claw skill** — a `SKILL.md` of instructions that composes existing
capabilities, in the exact style of the shipped `claw-study-read` skill (file
reads + `sqlite3`/`grep`, **no `claw-cli`** — it isn't in Claw's container). It
uses:

- **Interest-logs from Claw's own group memory** — `memory/courses/<id>/interests.md`
  (`ce297`, `ddia`, `dsa-interview`, `software-arch`) and `memory/thesis/interests.md`.
  (The study-app `memory/courses` tree is empty on the VPS; the live interest-logs
  are Claw's, read natively.)
- **Corpus from the study-app mount** — `/workspace/study-app/data/corpus/`
  (`courses/`, `meta/`, `study-methods/`).
- `curl` to the Wikipedia API for the real article URL.
- Claw's own memory dir for the wander-log (plain markdown read/append).

### Ownership & deployment

Authored in the claw-study repo at `docs/claw-skills/wander.md` — the same
pattern as the shipped `docs/claw-skills/claw-study-read.md`. Deployed into
NanoClaw's runtime skills dir (`~/stack/nanoclaw-v2/.../skills/wander/SKILL.md`),
hot-reloaded by the skill-watcher (polls every ~5 s, restarts the container on a
new skill).

### Trigger

Natural language (NanoClaw has no slash commands). Frontmatter `description`
lists triggers: **"wander"** (primary), "give me something to read",
"surprise me", "feed me something".

### What one Wander does (data flow)

On a trigger:

1. **Read the wander-log** at `groups/dm-with-hiroji/memory/wander/log.md` to
   know which recent topics to avoid.
2. **Gather seeds** — read interest-logs from Claw's group memory
   (`memory/courses/{ce297,ddia,dsa-interview,software-arch}/interests.md` +
   `memory/thesis/interests.md`), plus a sampling of corpus topics under
   `/workspace/study-app/data/corpus/`.
3. **Hop sideways** — pick a seed, then jump *one step* into an adjacent or
   tangential topic (may be fully off-syllabus). Serendipity is the point; the
   seed is a launch pad, not the destination. Avoid anything in the wander-log.
4. **Write the card** — one idea, **~150–250 words**, in **English**, levelled
   "sharp but new to *this* topic: give footholds, not jargon."
5. **Resolve the exit link** — call
   `https://en.wikipedia.org/w/api.php?action=opensearch&search=<topic>&limit=1&format=json`
   and use the real URL it returns. **Never fabricate a URL**; if there is no
   confident match, omit the Wikipedia link for that card.
6. **Append the menu** — three numbered rabbit-hole threads plus a `w` link
   option: *"Reply **1–3** to go deeper · **w** for Wikipedia · or just ask
   anything."*
7. **Log it** — append the chosen topic + date to the wander-log.

### Navigation (numbered reply)

The learner's reply is an ordinary Telegram message through the existing path:

- `1`–`3` → Claw continues that thread (a fresh sideways hop / a deeper card on
  the chosen angle), itself ending with a new numbered menu.
- `w` → Claw returns the resolved Wikipedia link (if it was omitted, it says so).
- *"simpler"* (or any clarifying question) → Claw drops a rung and re-explains
  the **same** card's idea with more footholds. No configured level.
- Free text → answered as a normal question (truest rabbit-hole behaviour).

### State: the wander-log

`groups/dm-with-hiroji/memory/wander/log.md` — a plain markdown list of the last
~15 topics with dates. The skill reads it (step 1) and appends to it (step 7),
trimming to the most recent ~15. This survives the per-message container model
(containers are stateless; files persist). Enough to dodge an annoying
immediate rehash without heavier machinery.

### Error handling

- Wikipedia API unreachable or no confident match → **omit the link, ship the
  card anyway** (the card stands alone).
- A course's interest-log is empty/missing → skip that course's seeds.
- No usable seeds at all → fall back to a topic drawn from the corpus.
- Wander-log file absent → treat as empty and create it on first append.

## Components & boundaries

| Unit | Responsibility | Depends on |
|------|----------------|------------|
| `docs/claw-skills/wander.md` (SKILL.md) | the entire Wander behaviour: triggers, seed→hop→write→link→menu→log | Claw group-memory interest-logs, `/workspace/study-app/data/corpus/`, Wikipedia API, wander-log file |
| wander-log (`memory/wander/log.md`) | last-~15 topic memory for repeat-avoidance | filesystem only |
| `CONTEXT.md` Wander entry | canonical meaning of the term | — |
| ADR 0013 | records the no-buttons / numbered-reply decision | — |

## Acceptance (behavioural — a prompt has no unit tests)

Manual, over Telegram, after deploy:

1. Send "wander" → receive a ~150–250-word English card on a course-*adjacent*
   topic, ending with 3 numbered threads + a `w` option.
2. The Wikipedia link, when present, resolves to a **real** article (open it);
   when the topic has no good match, the card ships without a link rather than a
   dead one.
3. Reply `2` → Claw continues that thread with a new card + menu.
4. Reply "simpler" → Claw re-explains the same idea a rung lower.
5. `groups/dm-with-hiroji/memory/wander/log.md` gained the topic; a second
   "wander" does not repeat a logged topic.
6. The card is pure reading — no recall prompt, no confidence question, no Bloom
   tag (pedagogy stays off).

## Deploy

Author `docs/claw-skills/wander.md` in the repo, then place it into the NanoClaw
runtime skills dir and let the skill-watcher hot-reload (mirror however
`claw-study-read` was deployed). Seed the empty wander memory dir
(`groups/dm-with-hiroji/memory/wander/`). No `study-app` / `claw-cli` rebuild.

## References

- Existing skill pattern: `docs/claw-skills/claw-study-read.md`
- NanoClaw runtime facts (per-message containers, skill-watcher, Telegram via
  long-polling, study-app mount at `/workspace/study-app/`): memory
  `nanoclaw_self_host_plan.md`, `claw_study_service.md`.
- [ADR 0013](../../adr/0013-wander-numbered-reply-no-telegram-button-fork.md),
  [CONTEXT.md](../../../CONTEXT.md) Wander entry.
