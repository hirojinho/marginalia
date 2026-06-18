# 0006 — Embed Pi as the agent runtime; expose domain ops via Go `claw-cli`

- **Status:** Accepted
- **Date:** 2026-05-10

## Context

`/chat` today calls a single OpenCode-compatible LLM endpoint with a fixed list of function tools defined in `agent/tools_*.go`. That arrangement is bounded: the model does not run protocols, does not follow skills, does not accumulate memory across sessions, and cannot dispatch multi-step work. The 2026-05-10 grill-me session established that the gap between this app and the maintainer's day-to-day Claude Code workflow is the *runtime fabric* — skills, memory, protocols, verification — not the LLM itself. To replace that workflow, the app needs an embedded agent runtime, not a smarter prompt.

Three runtime options were weighed: build one in Go, embed Anthropic's Claude Code (closed source, per-token billing), or embed an open-source Claude-Code-style runtime. Pi (`@earendil-works/pi-coding-agent`, MIT, the substrate that powers OpenClaw) was chosen for its minimal surface, its embeddable design (RPC mode + SDK), its built-in OpenCode Go provider, and its standard Agent Skills format that the existing skills can port to.

`marginalia` is Go; Pi is TypeScript-only. Three integration paths exist: a Node sidecar wrapping Pi's SDK (Path A, OpenClaw's pattern), Pi RPC subprocess + Go CLI for custom tools (Path B), or Pi RPC + a TypeScript extension package (Path C). Path B was chosen because it keeps the repo monolingually Go, aligns with Pi's "use bash for everything that is not file CRUD" design bias, and is reversible into A or C if it hits a ceiling.

## Decision

Embed Pi as the agent runtime by spawning `pi --mode rpc` per chat turn from `marginalia`'s new `/chat-v2` handler. LLM calls go to OpenCode Go via Pi's built-in provider. Custom domain operations — RAG search, plan toggle, course context, fleeting-note save, study-skill dispatch, memory load — are exposed through a small Go binary `claw-cli` that Pi invokes via its `bash` tool. Per-session ephemeral sandbox `cwd` at `data/agent-sessions/<id>/` carries a generated `AGENTS.md` with user profile, course context, and active feedback memories. Six study-core skills (`study-notes`, `course-study-path`, `study-step-complete`, `resource-orientation`, `by-hand`, `pair-coding`) are ported to Pi's skill format under `skills/` in the repo and mounted into each session. Rollout is feature-flagged (`AGENT_RUNTIME=pi|legacy`) until stable.

Full design in [`docs/specs/agent-runtime-pi.md`](../specs/agent-runtime-pi.md). Implementation sequencing in [`docs/specs/2026-05-10-agent-runtime-pi-impl.md`](../specs/2026-05-10-agent-runtime-pi-impl.md).

## Consequences

- The LLM-call layer in `agent/llm.go` no longer drives chat directly; `/chat-v2` proxies to a Pi subprocess and translates its event stream into SSE for the browser.
- `agent/tools_*.go` stays as the implementation surface for custom operations — `claw-cli` is a thin wrapper over those Go functions, exposed as subcommands. No duplication of business logic.
- ADR 0003's "single static binary" stance gets a footnote: the deploy now includes an extra `pi` binary on the server. No Docker, no Node runtime, no compose file — the binary count goes from two (`marginalia`, `cloudflared`) to three (`+ pi`).
- Cost stays bounded by the existing $10/month OpenCode Go subscription. Per-turn latency rises to roughly 5–15 s (Pi cold-start plus LLM streaming) — accepted because per-turn isolation is the right default for v1.
- Skills become first-class artifacts checked into `skills/` in the repo. The six ported skills travel with the app and ship via the same deploy path as the binary.
- Memory becomes explicit, not ambient: the app owns its memory store, and a per-session `claw-cli memory load` produces the relevant slice. This is a smaller surface than Claude Code's auto-memory, deliberately.
- Capability ceiling: non-Claude OpenCode models (DeepSeek, Kimi, GLM) are weaker on protocol-heavy skills (`grill-me`, `by-hand`, `verification-before-completion`). v1 ships the six study-core skills and accepts that the protocol-heaviest workflows still benefit from running Claude Code on the laptop. The app does not aim to replace every interaction tonight.
- Reversible. Path A (Node sidecar) or Path C (TypeScript extensions) remain available if Pi RPC + `claw-cli` proves too brittle for the OpenCode model fleet.
