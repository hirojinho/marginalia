# Documentation

Quick index of everything under `docs/`.

## Specs

Active specs describe how the app works today. Proposed specs describe accepted designs not yet implemented; they flip to active once shipped.

| Spec | Subject | Status |
|---|---|---|
| [`specs/architecture.md`](specs/architecture.md) | Top-level architecture and dependency boundaries. | active |
| [`specs/rag-search.md`](specs/rag-search.md) | Vector-store layout and retrieval flow. | active |
| [`specs/pdf-viewer.md`](specs/pdf-viewer.md) | Embedded pdf.js viewer + annotation persistence. | active |
| [`specs/tools.md`](specs/tools.md) | Function-call tools the LLM can invoke. | active |
| [`specs/agent-runtime-pi.md`](specs/agent-runtime-pi.md) | Pi-based embedded agent runtime + Go `claw-cli`. | proposed (2026-05-10) |

Implementation plans for in-flight specs live alongside them, dated. See [`specs/2026-05-10-agent-runtime-pi-impl.md`](specs/2026-05-10-agent-runtime-pi-impl.md) for the Pi runtime build sequence.

Historical plans and superseded specs live in [`specs/archive/`](specs/archive/).

## ADRs — why we chose what we chose

Numbered, immutable decision records. Index in [`adr/README.md`](adr/README.md). Use the [template](adr/template.md) when adding a new one.

## Skills

- [`claw-skills/`](claw-skills/) — skills exposed to an external agent client.
- [`superpowers/specs/`](superpowers/specs/) — dated design specs for skill rollouts.

## Product journal

- Top-level [`CHANGELOG.md`](../CHANGELOG.md) — what shipped, when.
- Top-level [`ROADMAP.md`](../ROADMAP.md) — Now / Next / Later.
