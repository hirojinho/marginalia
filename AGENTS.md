# Authoring rules — local planning Pi

These rules govern the **local planning Pi** that authors specs for the overnight
queue. (The VPS executor runs with `--no-context-files` and ignores this file; it
obeys the spec + the `implement-from-spec` skill only.)

- **Always grill before writing a spec.** Use `/grill <draft>`. One question at a
  time, multiple-choice preferred. Explore the code to answer before asking.
- **Spec = WHAT + how we verify. ADR = WHY.** Record an ADR at
  `docs/adr/NNNN-<slug>.md` only when a decision is hard-to-reverse AND surprising
  AND a real trade-off. Don't ADR routine choices.
- **Reconcile terminology against `CONTEXT.md`** (ubiquitous language); rename for
  consistency as decisions crystallise.
- **The committed spec file in `specs/queue/` is the sole handoff to the VPS.**
  No file paths or code in the spec body — except a snippet that encodes a
  decision more precisely than prose (state machine, reducer, schema, type shape).
- **Every spec needs a `## Verification recipe` whose `### Pre-baseline` genuinely
  FAILS on current main.** That failing check is the contract with the executor;
  a spec without it is broken.
- **Pre-baseline verifiers must use the form `if grep -q PATTERN FILE; then echo
  UNEXPECTED; exit 0; fi` ending with `exit 1` — NEVER `! grep -q` (it inverts
  the exit code, so the gate reads the spec as already-satisfied and aborts with
  exit 10).**
- **When a spec changes behavior that an ADR or a guard test protects: cite the
  ADR it honors or amends, reconcile MEANING (not just terminology) against that
  ADR, and update or explicitly retire the guard test IN THE SAME SPEC.** A spec
  that breaks a guard test it never mentions is the drift failure mode.
- **Pick the cost tier in frontmatter:** `model: deepseek-v4-flash` default;
  `deepseek-v4-pro` / `thinking: high` only when a ticket needs frontier reasoning.
