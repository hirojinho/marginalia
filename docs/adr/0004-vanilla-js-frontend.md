# 0004 — Vanilla JS frontend, no Node build step

- **Status:** Accepted
- **Date:** 2026-05-10

## Context

Phase 4 of the stability plan split the 2,160-line `index.html` god-file. Choices considered: keep vanilla JS (`addEventListener` + `fetch`); HTMX (server-rendered partials, minimal JS); React + a build pipeline; Svelte / SvelteKit. The app is solo-use, lightly interactive (chat, plan toggles, PDF viewer), already has working JS, and ships as a single Go binary embedding `static/`.

## Decision

Stay on vanilla JS. Split into `static/index.html`, `static/style.css`, `static/app.js`. Bundle CDN dependencies (`marked.min.js`) into `static/` so the app works without internet to a CDN. No Node toolchain, no bundler, no framework runtime.

## Consequences

- Build stays one command (`go build`). One binary, no `dist/` step, no `npm install`.
- UI code is more verbose than a framework-driven equivalent — accepted at current size.
- `apiFetch` wrapper handles retry/backoff for idempotent GETs; global `error` + `unhandledrejection` handlers surface failures via banner. No framework error boundaries needed.
- Reactive data binding has to be hand-rolled. Pushes back when components become stateful enough that imperative DOM updates start tangling — revisit then.
- All static assets cache-friendly under the same origin; no third-party CDN at runtime.
