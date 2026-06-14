# Systems-theory grounding for a code-first agentic harness (research, 2026-06-14)

> **Framing — this is NOT a safety project.** The goal is to **drive and steer
> agentic development** toward correct, low-token, small-model-viable outcomes.
> We *borrow* systems-theoretic techniques (STPA, MAPE-K, VSM) purely as a
> **modeling lens** — we want their *structure* (hierarchical control loops, the
> control-loop anatomy, the failure-mode *type checklist*, MAPE-K per-phase
> delegation, Ashby's variety limits), not hazard analysis. Wherever the sources
> say "hazard / loss / unsafe control action," read it as **"goal-divergence /
> ineffective or incorrect control action"** for *our* objective (steering
> development), not as a safety claim. The frameworks are a toolkit, not the point.

Distilled from a deep-research sweep (26 sources, 117 claims, 25 adversarially
verified → 22 confirmed / 3 refuted). Grounds the harness redesign:
model the pipeline as a hierarchical control system and maximize **code**
(deterministic) control loops over **generative** (LLM) ones, so it runs on
small/local models with few tokens.

## Reusable vocabulary & taxonomies (confirmed, high-confidence)

### 1. STPA control-loop anatomy → the manifest backbone
A system is a **hierarchical control structure** of nested feedback loops. Each
loop: `{controller (runs a control algorithm), control action, controlled process
(may itself be a lower controller — recursion), feedback, process model,
constraint}`. (Leveson & Thomas, STPA Handbook 2018; corroborated MIT OCW 16.63J,
arXiv 2506.01782.) Our manifest fields map 1:1 onto these elements.
**Live prior art for AI:** STPA is already applied to frontier/agentic AI with
developers as hierarchical controllers and the AI deployment as controlled process
(arXiv 2506.01782 "Systematic Hazard Analysis for Frontier AI using STPA", 2025;
2512.17600; 2503.12043). We are not reinventing.

### 2. Constraints are CONTEXT-QUALIFIED (UCA) → the `check`/`constraint` semantics
An **Unsafe Control Action** is unsafe "in a particular context and worst-case
environment" — almost never unconditionally. Formal structure `{Control Action,
Type, Context}`. A guardrail fires only against **context-qualified** outputs, not
blanket bans. (Our r9 case is textbook: the Rule-6 wording is unsafe *in the
context of* an empty retrieval queue, not always.) So manifest `constraint` must
carry its context; `check` fires conditionally.

### 3. The four UCA types → the `on-violation` axis (finite, complete checklist)
How a control action can be unsafe: **(1)** not provided when needed; **(2)**
provided when it shouldn't be / provided incorrectly; **(3)** wrong timing or
order; **(4)** wrong duration (lasts too long / stopped too soon — *continuous
actions only*). A discrete tool call faces 1–3; a looping/sustained action also
faces 4. Every failure in the 2026-06-14 postmortem maps to one of these (hang =
type 1 missing actuator; deploy-on-stale-base = type 2; `! grep` inversion =
corrupted feedback → type 3-ish; orphaned spec = process-model staleness).

### 4. MAPE-K per-phase delegation → the `controller-type (code|model)` rule
MAPE-K loop = **Monitor → Analyze → Plan → Execute** over a shared **Knowledge**
base. Each phase can be independently assigned to a controller that is **human /
deterministic-infrastructure / the managed component itself**; the system's degree
of autonomy *is* that per-phase distribution. **The delegation rule (decisive for
our thesis):** a phase can go to a cheap automatic (code) controller *only when it
is always applied WITHOUT customization*; novel/customized decisions need the
costly (human ≈ LLM) controller. (ICSA-C 2022 autonomic-microservices, Kubernetes
example.) → **This IS our "irreducible generative boundary," derived: non-novel /
non-customized ⇒ code; genuinely novel ⇒ model.**

### 5. Cybernetic limits → why "push to code" has a floor (Ashby / Good Regulator)
- **Ashby's Law of Requisite Variety:** a controller must have ≥ the variety of the
  disturbances it handles ("only variety destroys variety"). → A deterministic
  checker **cannot be arbitrarily simple** if the space of possible bad LLM outputs
  is large. **Design move:** to make a control code-able, *narrow its constraint so
  the per-check disturbance space is small.* The art of "push to code" = decomposing
  one fuzzy constraint into many narrow, scoped checks.
- **Good Regulator Theorem (Conant & Ashby 1970):** every good regulator must be a
  model of the system it regulates — controllers embed process models. (Proof rigor
  is contested in recent work, but attribution/limit stands.)

## The thesis: "push control into code so small models suffice"

### Strongest FOR
- **VeriGuard** (arXiv 2510.05156, Google/DeepMind 2025): dual-stage — OFFLINE
  formal verification compiles control logic into a verified code policy; ONLINE a
  lightweight monitor checks each agent action against it *before execution*. Drove
  attack-success to **0.0%** across 4 attack types × 4 backbones and **beat an
  LLM-judge guardrail on the safety/utility trade-off** (e.g. TSR 85.1 vs 68.3 on
  Claude). Direct evidence a **code checker can outperform a model checker.** Its
  refine-until-verified loop is the generate-and-check asymmetry in action.
- **Life-Harness** (arXiv 2605.22166): adapting the *harness/interface* (not weights)
  improves frozen agents; harnesses evolved from one 4B model transfer to 17 others
  → control captures **model-agnostic, environment-side structure**.
- **Self-Healing Orchestrators** (arXiv 2606.01416): explicit **data plane vs
  reliability control plane** (MAPE-K-inspired monitor-detect-diagnose-recover-verify);
  structured *action selection* beats undifferentiated retry (94.0% vs 85.3%).

### Honest AGAINST / scope limits (these discipline the design)
- **All strong evidence is in DETERMINISTIC, rule-governed domains** (tau-bench,
  agent-security-bench). None demonstrates it for **open-ended software dev**
  (multi-file refactors, ambiguous specs) — i.e. exactly r9's regime. We are at the
  edge of the evidence.
- **Harness transfer proves model-AGNOSTICISM, not small-model task adequacy.** The
  control *plane* can be code/small-model; the generative *core* (writing 495 lines)
  may still need a capable model. (Matches our agreed boundary.)
- **Verification asymmetry is CONTEXTUAL, not universal** (Jason Wei's "verifier's
  law"; arXiv 2509.17995): strong for tasks with cheap deterministic checks, *degrades*
  for hard problems and strong generators making subtle errors. → generate-and-check
  economics hold for our **structural/policy** checks, NOT for "is this implementation
  correct" (near-symmetric, stays model).
- **REFUTED claims (0-3 / 1-2):** (a) a deterministic verifier eliminates *semantic
  silent failures* to 0.0% — **refuted**: code checks catch only the invariants you
  have *encoded*; unencoded semantic drift still slips. (b) competent regulation needs
  no internal model — **refuted** (Good Regulator stands). (c) interface mismatch is
  the *primary* cause of agent failure — **refuted** (don't over-claim the harness).

## Net design rules (what this licenses)
1. Manifest = `{layer, constraint (context-qualified), controller-type (code|model),
   check, on-violation ∈ UCA{1,2,3,4}}` — every field framework-grounded.
2. **controller-type = code iff the decision is non-novel/non-customized** (MAPE-K
   rule); else model. This is the generative-boundary test, applied per loop.
3. **To code-ify a control, narrow its scope** until its requisite variety is small
   (Ashby). Big fuzzy constraint → many small scoped checks.
4. Generate-and-check is the small-model lever for **structural** checks; the
   **semantic core stays model** (and escalates), because asymmetry is contextual.
5. **Code checks catch only ENCODED invariants** — so each postmortem *adds* checks
   (the guard test `TestRule6` is the pattern); the planning Pi stays essential for
   catching *novel* semantic conflicts. No deterministic plane "eliminates" drift.

## Open questions (no source resolves; for our design to answer)
- Does the thesis hold OUTSIDE rule-governed domains (open-ended dev)? Unproven.
- Minimum requisite variety of a checker vs the model size it lets you shrink to —
  no quantified trade-off curve exists.
- Hand-written vs LLM-synthesized-then-verified control logic (VeriGuard) for the
  small-model regime — unisolated.
- End-to-end instantiation of the manifest for an agentic *coding* harness — nobody
  has published one. **That's our contribution.**

## Key sources
STPA Handbook (Leveson & Thomas 2018); arXiv 2506.01782 (STPA for frontier AI);
ICSA-C 2022 autonomic microservices (MAPE-K delegation); arXiv 1409.7475 +
Conant & Ashby 1970 (requisite variety / good regulator); arXiv 2510.05156
(VeriGuard); 2605.22166 (Life-Harness); 2606.01416 (self-healing orchestrators);
2509.17995 + jasonwei.net (verification asymmetry / verifier's law).
