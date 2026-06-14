# Harness redesign — session handoff (pick up here)

**What this initiative is:** redesign the Pi-based agentic-development harness
(local planning Pi + remote executor Pi, embodied in the `pi-claw-pipeline`
package) so it **drives and steers agentic development** reliably on small/local
models with few tokens. We model it with **systems-theoretic techniques as a
lens** (STPA / MAPE-K / VSM). **NOT a safety project** — "hazard/unsafe" language
from the sources is repurposed as "goal-divergence / ineffective control."

**Where it came from:** grew out of the 2026-06-14 Pi-driven postmortem
(`docs/postmortem/2026-06-14*`). That run exposed that the failures were *control*
failures (missing actuator = the hang, stale process model = drift + orphaned spec,
inverted feedback = `! grep`), which is what motivated modeling the harness as a
control system.

## Locked design constraints (hard definitions, agreed)
1. **Systems-theoretic + hierarchical control loops + traceability.** The control
   structure is *declared*, not buried in prose.
2. **Maintain Pi minimalism** (4-tool core + composable Extensions/Skills/Prompts/
   Packages; don't bloat the system prompt).
3. **Control plane = CODE / small-model / low-token. Generative core = model, may
   escalate.** Boundary (derived from MAPE-K's delegation rule): a loop is **code
   iff its decision is non-novel / non-customized**; genuinely novel/customized
   decisions stay model.
4. **Prefer code controls over generative controls** everywhere the decision is
   non-novel. Convert prose rules → executable checks / `tool_call` hooks (0 tokens,
   deterministic, small-model-proof). Example template: the `git add -A` prose rule
   should become a `tool_call` block.
5. **Acceptance test:** swap `deepseek-v4-pro` for a small local model and run a
   ticket — whatever breaks shows where control is still smuggled into the model.

## The two exercises (they are DUAL — run 2 → 1)
- **Exercise 2 (control model):** build the **control-loop manifest**
  `{layer, constraint (context-qualified), controller-type (code|model), check,
  on-violation ∈ UCA{1,2,3,4}}`. This single artifact IS the systems-theory model
  + the traceability record + the plugin registry.
- **Exercise 1 (Pi plugins):** derive plug-and-play Pi units (Extensions/Skills/
  Prompts/Package bins) from the manifest, biased toward code. "Is this cleanly
  pluggable?" is the diagnostic for single-loop vs cross-cutting concern.
- Duality: Ex2 finds the loops (and the missing ones) → Ex1 builds them; Ex1's
  decoupling test validates Ex2's layer boundaries.

## Research grounding (read first next session)
`docs/harness-design/2026-06-14-control-system-research.md` — cited, verified. Five
net design rules + the honest caveats (Ashby floor: narrow each check's scope so
its requisite variety is small; verification asymmetry is contextual → semantic
core stays model; code catches only *encoded* invariants → postmortems add checks;
the thesis is *unproven* for open-ended dev = our regime = our contribution).

## NEXT STEP (start here)
Run a **grill-with-docs** session to draft the **control-loop manifest** as the
design spine — enumerate the harness's control loops across the hierarchy
(human → orchestrator → gate → executor Pi → codebase, plus the planning-Pi
authoring control structure), and for each decide: constraint (+context),
controller-type (code|model per rule #3), the check, and the on-violation UCA type.
Grill each loop against the research + claw-study's `CONTEXT.md` / `docs/adr/`.
Then derive the Pi plugin set (Exercise 1).

## Candidate loops already identified (seed the manifest)
From the postmortem + the "promote my hand-workarounds" backlog:
- timeout/liveness (code, outside Pi — orchestrator); design-gate non-interactive
  approval token (code, executor); ADR auto-injection `before_agent_start` (code,
  planning); commit-hygiene `git add -A`/`*.bak` block (code, executor); verifier
  linter rejecting `! grep` at authoring (code, planning); re-gate-on-merge when
  main moved (code, orchestrator); spec↔ADR semantic reconciliation (MODEL —
  irreducible, planning); the actual implementation (MODEL — irreducible, executor).
