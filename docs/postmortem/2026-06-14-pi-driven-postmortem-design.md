# Design — Pi-driven postmortem of the overnight pipeline

**Date:** 2026-06-14
**Status:** Approved (brainstorm/grill complete), pending implementation plan.

## Purpose

Run the usual error-analysis ritual on the claw-study overnight pipeline, but
drive the entire forensics + fix + deploy loop through **local Pi** instead of
Claude Code doing the work directly. Claude Code authors each prompt and
critiques how Pi performs each step. The goal is twofold:

1. **Repair** — understand what went wrong, fix the root cause, and ship the
   implementation that was supposed to happen, *through the real pipeline*.
2. **Harvest** — distill the methodology into a reusable Pi skill so local Pi
   can run this self-repair loop on its own next time.

## Object of study

- **Anchor failure:** `specs/failed/2026-06-01-sm2-spaced-review.md` — the one
  ticket in `failed/`, with a recoverable unshipped implementation (the SM-2
  expanding-interval scheduler). This is the bounded, evidence-backed failure we
  drive end-to-end.
- **Triage sweep first:** before committing to sm2, local Pi sweeps all ~20
  `results/*` dirs to confirm sm2 is the primary hard failure and surface
  anything nastier.
- **Out of scope:** the R9 / ADR-0007 semantic-drift genre is already
  root-caused (2026-06-13) and has its own remedy path (ADR-0019). We do a
  *drift check* on the sm2 spec (phase 3) but do not fold the broader drift
  remediation into this session.

## Actors, surfaces, safety

- **Driver:** Claude Code. Authors each prompt, invokes local Pi via
  `pi --mode json -p`, parses the event stream, critiques, authors the next
  prompt. Never `pi -p` alone (documented hang issues).
- **Actor:** local Pi (`deepseek-v4-pro`, thinking high). Does all the work.
- **Evidence surface (read-only):** `ssh nanoclaw` →
  `~/stack/claw-build/results/<id>/` (transcripts, gate output, diffs, exit
  codes) + `~/stack/claw-build/bin/` (orchestrator scripts).
- **Fix surface (write, gated):** fresh local clone/worktree of `claw-study`
  for the feature fix; the `pi-claw-pipeline` package for skill/prompt fixes;
  in-place edits on VPS `bin/*.sh` for orchestrator fixes.
- **Safety layering:**
  - Forensics phases are read-only — Pi needs no write tool.
  - Fix phases are gated by the `design-gate` extension: Pi cannot write until
    Claude Code issues `/approve-design` after reviewing the proposed diff.
  - Prod deploy is gated by the human's go/no-go (pause at gate-green +
    staging-verified, before the prod binary swap).
  - The remote executor Pi is never used to investigate itself.
- **Phase handoff = files, not long sessions.** Each phase is a self-contained
  Pi invocation that writes findings to
  `claw-study/docs/postmortem/2026-06-14/phaseN.md`; the next phase reads it.
  Keeps each invocation cheap (no giant cached-context replay), auditable, and
  mirrors the pipeline's own "handoff = committed file" philosophy.

## Phase / prompt sequence

| # | Phase | Pi does | Claude Code analyzes |
|---|---|---|---|
| 0 | Preflight | Verify `ssh nanoclaw` reach, claw-study clone fresh off `origin/main`, deepseek key live, design-gate active | Setup sane before spending tokens |
| 1 | Triage sweep | Enumerate all `results/*`; per ticket extract outcome / cost / wall-time / Pi's-theory; cross-ref repo `specs/{done,failed}/` → triage table | Did Pi find all ~20? classify correctly? flag sm2 + anything else? |
| 2 | sm2 deep-dive | Reconstruct the sm2 run from its results dir; pinpoint which gate step/impl phase went red; quote the log lines; classify cause-layer (spec / executor / orchestrator) | Is root cause evidence-backed? symptom vs cause? |
| 3 | Drift check | Verify sm2 spec against ADR-0007 / CONTEXT.md (does it inherit the knowledge_component_id task-id drift?) | Does Pi catch semantic drift, not just terminology? |
| 4 | Fix authoring (gated) | Author the layer-appropriate fix; re-validate pre-baseline locally | Review diff before `/approve-design` |
| 5 | Re-run through gate | Re-queue sm2; trigger `overnight-run.sh`/`gate-runner.sh` on VPS; stop at gate-green + staging-verified | Gate output real? |
| 6 | Go/no-go + deploy | Human approves → `deploy-swap.sh` + prod `/debug/health` + `/debug/version` | Shipped clean |
| 7 | Pipeline hardening (gated) | Fix the class: planning AGENTS.md rules + executor `implement-from-spec` skill/frontmatter, derived from the root cause | Review diffs before approve |
| 8 | Harvest skill | Distill the loop into `pi-claw-pipeline/skills/pipeline-postmortem/` + `/postmortem` prompt; version-bump | Reusable + faithful to what we did |
| 9 | Overview doc | Assemble phases 1–3 + fixes into the final overview | Coherent narrative |

## Deliverables

1. **Forensics overview** — `claw-study/docs/postmortem/2026-06-14-overview.md`
   (what happened across all runs + sm2 root cause).
2. **sm2 shipped** via the real pipeline (gate-green, human go/no-go,
   prod-verified).
3. **Layer-specific pipeline fixes** (planning rules + executor
   skill/frontmatter + orchestrator, as the cause dictates).
4. **`pipeline-postmortem` Pi skill** in `pi-claw-pipeline`, version-bumped,
   with a short "how to drive local Pi" addendum derived from the meta-analysis.

## Meta-analysis protocol

At every Pi turn, Claude Code records in a running log: (1) did Pi do what the
prompt asked? (2) where did Pi need correction / go off-rails? (3) what does that
teach about local Pi's config or our prompt design? This log feeds the pipeline
fixes (phase 7) and the "how to drive local Pi" addendum to the harvested skill
(phase 8).

## Decisions locked during grilling

- Anchor on sm2-spaced-review; triage sweep first to confirm.
- Scope = hard-failure genre only; drift is a check, not a remediation.
- Local Pi is the sole actor; Claude Code drives via `pi --mode json`.
- Ship sm2 through the real gate (re-queue → gate → deploy), not hand-fixed.
- Human go/no-go before the prod binary swap (first re-ship after a failure).
- Harvest a reusable `pipeline-postmortem` skill (not prompts-only).

## Related

- Memory: `claw-overnight-pipeline`, `pi-pipeline-architecture`,
  `claw-study-pipeline-drift-rootcause`.
- Execution log: `docs/specs/2026-05-21-first-overnight-run-log.md`.
- Package: `~/Documents/ITA/pi-claw-pipeline`.
