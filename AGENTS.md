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
- **Pick the cost tier in frontmatter:** `model: deepseek-v4-flash` default;
  `deepseek-v4-pro` / `thinking: high` only when a ticket needs frontier reasoning.
