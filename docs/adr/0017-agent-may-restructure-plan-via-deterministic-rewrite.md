# 0017 — The tutor may restructure the Plan, but only through a deterministic rewrite of the plan store

- **Status:** Accepted
- **Date:** 2026-05-30
- **Amends:** [0016](0016-agent-may-write-steering-via-deterministic-tool.md) (which amends [0010](0010-steering-via-settings-ui.md))
- **Relates to:** [0011](0011-plan-is-navigation-spine.md), [0014](0014-phase3-task-anchored-sessions-data-model.md)

## Context

[ADR 0016](0016-agent-may-write-steering-via-deterministic-tool.md) established that
the tutor may change *Steering config* mid-Session through a deterministic typed write
to the source store (`claw-cli course settings set`), confirming and resuming rather
than accreting a config conversation. It scoped itself to **settings knobs** (framing,
chunk size, etc.).

The same friction exists one level up, on the **Plan** itself. In a real
critical-theory Session (session 46, 2026-05-30) the learner asked the tutor to split a
monolithic 51-page reading task into the semantic chunks they had just worked out
together. The tutor could not: the Pi `/chat-v2` agent acts only through `claw-cli` +
bash, and `claw-cli plan` exposed only `show | status | toggle`. The Go layer already
had a full write path (`ToolRewritePlan`, which validates and preserves task UUIDs),
but it was never wired to the CLI — the same shape of gap as the pre-0016 settings
case and the course-creation case.

The Plan is the navigation spine ([ADR 0011](0011-plan-is-navigation-spine.md)) and a
live document the learner reshapes as need emerges. Forcing plan edits out to a form,
or refusing them mid-Session, recreates exactly the "drive me to a menu" friction
0016 removed.

Two real risks had to be addressed, not waved away:

1. **Re-tangling Authoring into Studying.** The Phase-3 redesign ([ADR 0014](0014-phase3-task-anchored-sessions-data-model.md))
   deliberately separated generative plan *Authoring* from task-anchored *Studying*
   Sessions. Allowing structural edits mid-Session risks the accretion 0016 warned
   about.
2. **Detaching live Sessions.** Sessions anchor to tasks by UUID (`sessions.task_id`).
   A careless rewrite that regenerates UUIDs orphans in-flight Sessions into the
   Detached bucket.

## Decision

Add one subcommand — `claw-cli plan rewrite --course <id> --plan-file <path>` — that
wraps the existing `ToolRewritePlan`. The tutor's loop is read → edit → write:
`claw-cli plan show` returns the full plan JSON, the agent edits it freely (split,
add, rename, reorder, remove), writes a temp file, and submits the whole plan. This is
the **minimal surface that edits anything**, deliberately chosen over a fixed set of
bespoke primitives (split/add/rename) — fewer commands, and conformant to the Pi
philosophy of general tools over special-cased ones.

The durable invariants from 0010/0016 are preserved and carried up to plan structure:

- **Deterministic store write, never raw file narration.** The write goes through the
  validating `ToolRewritePlan` (parses as `JSONPlan`, requires `plan_json.id` ==
  course), not a freehand edit of `data/plans/<id>.json` by the agent. The plan is
  structured source-of-truth, like `course_settings`; the agent does not narrate it
  into a file.
- **Anchors survive by construction.** `inheritOrGenerateIDs` honors any `id` the
  agent carries forward, so a renamed/split-from task keeps its UUID (and its Session)
  when the agent preserves `id`; only blank-`id` tasks get fresh UUIDs. AGENTS.md
  instructs the agent to keep `id` on continuing work and omit it for genuinely new
  tasks.
- **One-shot, no accretion.** As in 0016: make the change, confirm in one line, resume
  the study work. The prohibition is on a Studying Session drifting into an open-ended
  plan-editing conversation, and on the tutor restructuring unasked — not on the
  edit itself.

Classification (per CONTEXT.md): a learner-directed declarative restructure (split
this, add/rename/reorder one) is **Steering**; the generative design that may precede
it (deriving a chunk map from a PDF) is Studying/Authoring. Both persist through this
one deterministic write.

## Consequences

- The Plan becomes editable in-flow, closing the session-46 gap, with the same
  ergonomics 0016 gave settings.
- The live rail refresh is free and surface-agnostic: `handler/chat_v2.go` fingerprints
  the plan before/after each turn and emits `plan_changed` on any change, so a CLI
  rewrite refreshes the rail exactly as a toggle does.
- Accepted risk: a rewrite *can* still orphan a Session if the agent drops an anchored
  `id`. We rely on the AGENTS.md `id`-preservation norm rather than a hard guard;
  detachment is recoverable (Detached bucket), not data loss. A "warn/refuse if the
  rewrite would orphan a non-archived Session" guard is a possible future hardening,
  not built now.
- `rewrite` also *creates* a plan when none exists (it `MkdirAll`s the dir), so Plan
  Authoring can seed through the same command — no separate create path needed.
- The change is a `claw-cli` subcommand plus an AGENTS.md block, so deploying it
  rebuilds **both** binaries (`study-app` and `claw-cli`).
