# Documentation

Quick index of everything under `docs/`.

## Specs — current behavior

Authoritative reference for how the app works today.

| Spec | Subject |
|---|---|
| [`specs/architecture.md`](specs/architecture.md) | Top-level architecture and dependency boundaries. |
| [`specs/rag-search.md`](specs/rag-search.md) | Vector-store layout and retrieval flow. |
| [`specs/pdf-viewer.md`](specs/pdf-viewer.md) | Embedded pdf.js viewer + annotation persistence. |
| [`specs/tools.md`](specs/tools.md) | Function-call tools the LLM can invoke. |

Historical plans and superseded specs live in [`specs/archive/`](specs/archive/).

## ADRs — why we chose what we chose

Numbered, immutable decision records. Index in [`adr/README.md`](adr/README.md). Use the [template](adr/template.md) when adding a new one.

## Skills

- [`claw-skills/`](claw-skills/) — skills exposed to Claw (the Telegram agent client).
- [`superpowers/specs/`](superpowers/specs/) — dated design specs for skill rollouts.

## Product journal

- Top-level [`CHANGELOG.md`](../CHANGELOG.md) — what shipped, when.
- Top-level [`ROADMAP.md`](../ROADMAP.md) — Now / Next / Later.
