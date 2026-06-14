# Phase 3 — Drift check (resolved during Phase 2)

The drift check collapsed into the Phase 2 finding. Verdict: **DRIFTED — spec-level.**

- Term/referent: r9 keys probes on `--kc <knowledge_component_id>` correctly (ADR-0019 aligned) — no atom-id drift here.
- BUT a NEW drift: the r9 spec's Rule-6 rewrite **semantically contradicts ADR-0020** (forces recall on empty queue vs. ADR-0020's prediction-is-opener), and the spec cites ADR-0020 while violating it. Task 4 added probe tests but did not update the `TestRule6OneLightOpener` guard it breaks.
- Impact on the fix: it BLOCKS a naive fix. Reverting (Pi's proposal) kills the feature; shipping as-authored violates ADR-0020. Requires a human pedagogy decision + an ADR amendment.
