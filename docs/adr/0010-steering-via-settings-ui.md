# 0010 — Steering is edited via a settings UI to source-of-truth stores; the agent never persists config to generated artifacts

- **Status:** Accepted
- **Date:** 2026-05-29

## Context

The learner's "Steering" work — adjusting how the system behaves: course framing
("exam-prep first, conceptual exam"), tutor pace/chunk size, plan tweaks — was
done inside the study chat, and it kept *failing*. In session 34 the learner
spent half a long session fighting it: "where did you add that?", "did you
really add those?", "why is it in agents and not interests?", "delete it."

Root cause: the agent wrote config into `AGENTS.md`, which is a **generated**
file — rebuilt every session from the memory DB + a template (see
[[claw_study_pedagogy_layer]] / `agent/sandbox.go`). Edits to a derived artifact
never stick; they regenerate. Meanwhile the real persistence layers (course plan
JSON, `interests.md`, the `agent_memory` DB) were ambiguous and the agent had no
reliable way to confirm what landed where. So Steering was both *misplaced*
(polluting study sessions) and *unreliable*.

See *Studying vs Steering* in [CONTEXT.md](../../CONTEXT.md).

## Decision

Durable Steering config — course framing, exam style, tutor pace, "stop after
each task," and similar knobs — is edited through a **direct settings UI** that
writes **deterministically to source-of-truth stores** (the course plan JSON
and/or the `agent_memory` DB). The LLM is removed from the config *persistence*
path: the agent may **read** config (it flows into the generated `AGENTS.md` on
the read side), but it must **never write** durable config into generated
artifacts.

Steering is thereby separated from Studying: study Sessions stay clean.

## Consequences

- Config becomes reliable and visible — what you set is what persists, and you
  can see current settings rather than interrogating the agent.
- Study Sessions stop accreting system-administration turns.
- Trade-off: less conversational flexibility for config. Note the boundary —
  *plan (re)structuring* (generating/reshaping the task list) is creative work
  and stays agent-driven via the `course-study-path` skill; it is distinct from
  the fixed-shape settings this ADR governs.
- Implementation cost: a settings UI + a defined config schema + wiring config
  into `AGENTS.md` generation on the read path. The agent's existing
  config-writing behavior must be removed/redirected.
- If a future need arises for conversational config changes, they must still
  land in the source store (e.g. via a tool that writes the JSON/DB), never in a
  generated file — that invariant is the durable part of this decision.
