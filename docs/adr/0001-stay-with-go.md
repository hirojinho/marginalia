# 0001 — Stay with Go; restructure instead of rewrite

- **Status:** Accepted
- **Date:** 2026-05-08

## Context

The May 2026 stability review found ~5,160 LOC across 10 files (one 802-line `main.go`, one 2,160-line `index.html`) with zero tests, ~20 unhandled DB errors, no graceful shutdown, no request timeouts, and unprotected global state. A full rewrite was on the table. Alternatives weighed: stay with Go and restructure; Go + HTMX; Python + FastAPI + React; Go + SvelteKit. Estimated rewrite cost 40–60 h, restructure cost 25–35 h. App is single-developer, personal-use.

## Decision

Stay with Go. Restructure in place: introduce `agent.App`, split the `main.go` god-handler into `handler/` packages, add tests + structured logging + auth + named tunnel + restic backups. No language or framework change.

## Consequences

- No frontend framework — UI code stays plain HTML/JS plus `pdf.js`. More verbose, but no Node build step.
- Single static binary, embedded `static/`, low memory footprint on the VPS.
- Roughly half the time of a rewrite, without throwing away working RAG / PDF / tool-calling code.
- Tied to Go's standard library + `database/sql` + `mattn/go-sqlite3`; no benefit from richer JS-ecosystem libraries.
- The frontend will keep growing as plain JS until and unless ADR 0004 is revisited.
