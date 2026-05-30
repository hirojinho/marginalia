# 0016 — The tutor may change Steering config, but only through a deterministic typed write to the source store

- **Status:** Accepted
- **Date:** 2026-05-30
- **Amends:** [0010](0010-steering-via-settings-ui.md)

## Context

[ADR 0010](0010-steering-via-settings-ui.md) moved durable Steering config out of
chat and into a settings UI writing to a source-of-truth store, and stated as a
consequence that "the LLM is removed from the config *persistence* path." That was
the right fix for the session-34 failure, but it over-corrected: it would force the
learner out to a menu for every tweak, when the natural thing — mid-reading — is to
just tell the tutor "these chunks are too big, go smaller."

ADR 0010 itself left the door open: its final consequence says future conversational
config changes "must still land in the source store (e.g. via a tool that writes the
JSON/DB), never in a generated file — that invariant is the durable part of this
decision." This ADR walks through that door.

The session-34 pain had two roots, and neither was "the agent helped with config":
the agent wrote config into the **generated** `AGENTS.md` (so it regenerated away and
never stuck), and the learner could not **see or trust** what had landed where. The
`course_settings` table (Phase 4) plus the settings UI fix both — the store is
durable and the form shows current state.

## Decision

The tutor may change Steering config conversationally, in **any** surface (including
mid-Studying-Session), by calling a typed tool (`claw-cli course settings set`) that
writes the `course_settings` table — the **same** deterministic, validated write path
the settings form uses. The tutor confirms the change in one line and resumes; it does
**not** turn the session into a config conversation. The agent never narrates config
into a generated artifact (`AGENTS.md`), and there is no separate "agent config store"
— the form and the tool are two writers of one table.

The durable invariant from ADR 0010 is preserved and is the load-bearing part: **config
persists to the source store via a deterministic write; it is never narrated into a
generated file.** What changes is only *who may trigger* that write — the learner via
the form, or the tutor via the tool.

## Consequences

- Steering is reachable both ways: a form for deliberate review, a one-line ask for
  in-flow tweaks. The "don't force me to a menu" friction is gone.
- Changes take effect on the **next turn** for free: `AGENTS.md` is regenerated every
  turn from the table (`sandbox.go` `Create` → `writeAgentsMD`), so a tool write or a
  form save both bite immediately, with no session restart.
- Both writers share one validated DB function — the tool cannot write a knob the form
  can't, and validation lives in one place.
- Cost / accepted risk: a Studying Session *can* still drift into config chat. We rely
  on a prompt norm ("change it, confirm, resume") rather than a hard surface boundary.
  CONTEXT.md is amended to match — the prohibition is on *accretion*, not on a one-shot
  change.
- The tool is a `claw-cli` subcommand (the Pi agent's tool surface), so deploying this
  change rebuilds **both** binaries (`study-app` and `claw-cli`).
