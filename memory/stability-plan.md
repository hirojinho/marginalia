# Study App — Stability & Quality Plan

> From Fragile Prototype to Reliable Personal Tool
> Created: May 8, 2026
> Status updated: June 17, 2026 — **somewhat stable** (was *architecturally fragile*)

## Status — 2026-06-17 (architectural review)

The app has moved from **architecturally fragile** to **somewhat stable**. The
bulk of this plan shipped; the app no longer loses data silently, no longer dies
ungracefully, and is no longer one giant file front and back. Reassessed against
the live code in an architectural-stability review on this date.

**Shipped (verified in code):**
- *Phase 0* — `ReadTimeout`/`WriteTimeout`/`IdleTimeout` set; SIGTERM/SIGINT
  graceful shutdown with a 15s drain; 50MB PDF cap; 4000-char message cap.
- *Phase 1* — structured JSON error responses (`writeError`); input validation;
  `context.Context` cancellation through to the LLM/agent; `errcheck` now in the
  linter so unhandled DB errors are caught at commit time, not runtime.
- *Phase 2* — globals collapsed into the `agent.App` struct; `ActiveSessionID`
  is an `atomic.Int64`; the full SQLite pragma set (WAL, `busy_timeout`,
  `foreign_keys`, `synchronous=NORMAL`, cache) is applied on open. *(The
  service/repository layer (2.3/2.4) was deliberately **rejected** — see
  [ADR 0002](../docs/adr/0002-no-service-repository-layer.md). Numbered migrations
  (2.6) remain open — tracked in ROADMAP "Next".)*
- *Phase 3* — `go test` suites exist (handlers + pure functions); `golangci-lint`
  v2 config (`errcheck`, `govet`, `staticcheck`, `gocyclo`, `gochecknoglobals`,
  `forbidigo`, …) enforced by a `scripts/git-hooks/pre-commit` hook that lints
  staged files `--new-from-rev HEAD`.
- *Phase 4* — the 2,160-line `index.html` is now ~13 vanilla-JS modules (largest
  `pdf.js` ≈730 lines); retry-with-backoff (`apiFetch.js`); error banner;
  `data-action` event delegation.

**Why "somewhat stable" and not "stable" — residual structural debt:**
1. **`agent/db.go` is a 1,503-line monolith** — the one piece of the Phase-2
   restructuring never done. It's explicitly excluded from `gocyclo` in
   `.golangci.yml`, i.e. the guardrails were told to ignore it. → ROADMAP "Next".
2. **No file/function-size guardrail** — nothing stopped `db.go` from growing and
   nothing stops the next one. `funlen` is not enabled and there's no file-length
   check. → ROADMAP "Next".
3. **No numbered migration system** — migrations are still inline + string-matched
   on SQLite error text (Phase 2.6, already in ROADMAP "Next").
4. **Plan state is dual-sourced** — plans live as JSON files, everything else in
   SQLite, with no transaction spanning the two and the JSON authoritative on
   restart; multi-step writes aren't wrapped in explicit transactions. (Newly
   surfaced 2026-06-17; not yet a roadmap item — flag if it bites.)
5. **No resource ceiling on the pi subprocess** (memory/CPU/disk/FD) beyond the
   10-min turn timeout; three independently-deployed binaries with no version
   pinning; hand-synced frontend↔backend contract. (Newly surfaced 2026-06-17.)

**Recommendation:** Stay with Go, but restructure. A rewrite would take 40-60 hours; fixing it takes 25-35.

## Executive Summary

The study app is functionally usable but architecturally fragile. This plan identifies 28 specific issues and prescribes a phased remediation strategy. Goal: confidence to add features without breaking existing functionality.

> Historical note: this section is the original May-8 assessment; see the
> 2026-06-17 status block above for the current standing.

## Current State

| Component | Files | Lines | Tests |
|-----------|-------|-------|-------|
| `main.go` (handlers) | 1 | 802 | 0 |
| `agent/` (core logic) | 8 | ~2,200 | 0 |
| `static/index.html` (frontend) | 1 | 2,160 | 0 |
| **Total** | **10** | **~5,160** | **0** |

## Critical Findings

### 1. Silent Data Loss (~20 unhandled DB errors)
- Most `DB.Exec()` calls ignore errors entirely
- Examples: `main.go:268, 280-281, 485`, `agent/db.go:15-16, 119-120`
- Impact: Database corruption, lost messages, orphaned sessions — all invisible

### 2. Unprotected Global State
- `ActiveSessionID` read/written without synchronization
- `LastAssistantContent` written during streaming, read separately
- `Mu` mutex protects only some shared state
- Summary generation goroutine has race window with message saving

### 3. No Input Validation
- No message length limit (could exhaust LLM tokens)
- No PDF file size limit (could fill disk)
- No page number validation
- All IDs parsed with `fmt.Sscanf` (silently returns 0 on invalid input)

### 4. No Graceful Shutdown
- `log.Fatal(http.ListenAndServe(...))` — no signal handling
- In-flight requests killed mid-stream on SIGTERM
- Background goroutines (summary, PDF extraction) abandoned

### 5. No Request Timeouts
- No `ReadTimeout`, `WriteTimeout`, `IdleTimeout`
- Slow client can hold connection indefinitely
- LLM client has 120s timeout but no context cancellation on disconnect

### 6. Hardcoded Paths
- `agent/agent.go` hardcodes `/workspace/study-app/` for system prompt files
- Bypasses `VaultRoot` — caused the placeholder sessions incident

### 7. Single 2,160-line Frontend File
- CSS, HTML, JavaScript all in one file
- No error boundaries, no loading states, no retry logic
- CDN dependencies (marked.js, pdf.js) — breaks if CDN unavailable

### 8. No Testing, No Linting, No CI
- Zero test coverage
- No `go vet`, no `staticcheck`
- No automated build verification

## Remediation Phases

### Phase 0: Quick Wins (2-3 hours)

| # | Change | Files | Impact |
|---|--------|-------|--------|
| 0.1 | Add ReadTimeout, WriteTimeout, IdleTimeout to HTTP server | `main.go` | Prevents connection exhaustion |
| 0.2 | Add graceful shutdown (SIGTERM/SIGINT handler) | `main.go` | Clean shutdown, no lost data |
| 0.3 | Add PDF file size limit (50MB max) | `main.go` | Prevents disk exhaustion |
| 0.4 | Add message length limit (4000 chars) | `main.go` | Prevents LLM token exhaustion |
| 0.5 | Replace `fmt.Sscanf` with `strconv.ParseInt` + error checking | `main.go` | Eliminates silent parsing failures |
| 0.6 | Add `defer rows.Close()` where missing | `main.go`, `db.go` | Prevents connection leaks |

### Phase 1: Error Handling & Validation (4-6 hours)

| # | Change | Files | Impact |
|---|--------|-------|--------|
| 1.1 | Check all `DB.Exec()` errors — log and return 500 on failure | All | Eliminates silent data loss |
| 1.2 | Check all `DB.QueryRow().Scan()` errors | All | Eliminates silent read failures |
| 1.3 | Check all `os.WriteFile()` errors | `tools.go` | Eliminates silent file write failures |
| 1.4 | Add input validation middleware | `main.go` | Prevents invalid data from reaching DB |
| 1.5 | Add structured error responses (JSON error objects) | `main.go` | Frontend can handle errors gracefully |
| 1.6 | Add `context.Context` to LLM calls — cancel on disconnect | `llm.go` | Prevents wasted API calls |

### Phase 2: Architecture & Concurrency (6-8 hours)

| # | Change | Files | Impact |
|---|--------|-------|--------|
| 2.1 | Introduce `App` struct to hold DB, config, mutex — eliminate globals | New: `app.go` | Enables testing, cleaner architecture |
| 2.2 | Protect `ActiveSessionID` with mutex or atomic operations | `main.go` | Eliminates race conditions |
| 2.3 | Introduce service layer (SessionService, PDFService, PlanService) | New: `service/` | Separates HTTP from business logic |
| 2.4 | Introduce repository layer for DB operations | New: `repository/` | Enables mock DB for testing |
| 2.5 | Fix hardcoded paths in `agent/agent.go` to use `VaultRoot` | `agent.go` | Eliminates path dependency bug |
| 2.6 | Replace inline migration with proper migration system | `db.go` | Reliable schema evolution |
| 2.7 | Add SQLite pragmas (WAL, busy timeout, foreign keys) | `db.go` | Better concurrency, data integrity |

**SQLite pragmas to add:**
```
PRAGMA journal_mode=WAL;
PRAGMA busy_timeout=5000;
PRAGMA foreign_keys=ON;
PRAGMA synchronous=NORMAL;
PRAGMA cache_size=-2000;
```

### Phase 3: Testing Foundation (4-6 hours)

| # | Change | Files | Impact |
|---|--------|-------|--------|
| 3.1 | Set up `go test` infrastructure with in-memory SQLite | New: `*_test.go` | Enables all testing |
| 3.2 | Test pure functions (CountTasks, LoadPlan, parsePageSelection) | `types_test.go` | Coverage for data manipulation |
| 3.3 | Test chunker (ChunkFile, splitLongChunks, inferCourseID) | `chunker_test.go` | Coverage for corpus indexing |
| 3.4 | Test vectorstore (CosineSimilarity, serialize/deserialize) | `vectorstore_test.go` | Coverage for RAG core |
| 3.5 | HTTP handler tests using `httptest` | `handler_test.go` | Coverage for API endpoints |
| 3.6 | Add `go vet`, `staticcheck`, `golangci-lint` to workflow | CI config | Catch bugs before runtime |

**Testing strategy:** Don't aim for 100% coverage. Focus on pure functions (easy, high ROI), API endpoints (regression prevention), and edge cases (where bugs hide).

### Phase 4: Frontend Restructuring (6-8 hours)

| # | Change | Files | Impact |
|---|--------|-------|--------|
| 4.1 | Split `index.html` into `style.css` and `app.js` | New files | Reduces change risk, enables caching |
| 4.2 | Add global error handler (`window.onerror`) with recovery UI | `app.js` | App survives JS errors |
| 4.3 | Add loading states for all API calls | `app.js` | Better UX, prevents double-submits |
| 4.4 | Add retry logic for failed API calls (3 retries with backoff) | `app.js` | Resilient to transient network errors |
| 4.5 | Replace inline `onclick` with `addEventListener` + event delegation | `app.js` | Cleaner, debuggable event handling |
| 4.6 | Bundle CDN dependencies into static files | `static/` | App works without internet |
| 4.7 | Add input validation in frontend | `app.js` | Prevents invalid requests |

### Phase 5: Deployment Hardening (3-4 hours)

| # | Change | Files | Impact |
|---|--------|-------|--------|
| 5.1 | Create `Dockerfile` for reproducible builds | New: `Dockerfile` | Consistent environment, easy rollback |
| 5.2 | Create `docker-compose.yml` with app + volume mounts | New: `docker-compose.yml` | One-command deploy |
| 5.3 | Add automated SQLite backups (cron, daily) | New: script | Recover from data loss |
| 5.4 | Add log rotation | New: config | Prevents disk exhaustion |
| 5.5 | Add startup validation script (env vars, DB, disk space) | New: script | Fail fast on misconfiguration |
| 5.6 | Add `go.mod` version pinning and `go.sum` verification | `go.mod` | Reproducible builds |

## Stack Change Considerations

| Option | Pros | Cons | Verdict |
|--------|------|------|---------|
| **Stay with Go** | Already working, single binary, low resource usage | No frontend framework, manual HTML/JS | **Recommended** |
| **Go + HTMX** | Keep Go backend, simplify frontend | Requires rewriting frontend, learning HTMX | Consider for Phase 4 |
| **Python + FastAPI + React** | Rich ecosystem, better testing | Complete rewrite, heavier | Not worth it |
| **Go + SvelteKit** | Type-safe frontend, components | Requires Node.js build step | Overkill for personal use |

## Success Metrics

| Metric | Current | Target |
|--------|---------|--------|
| Test coverage (pure functions) | 0% | 80%+ |
| Test coverage (handlers) | 0% | 60%+ |
| Unhandled DB errors | ~20 | 0 |
| Global mutable state | 5 variables | 0 (encapsulated in App struct) |
| Request timeout | None | 30s read, 120s write |
| Graceful shutdown | No | Yes (15s drain) |
| Frontend files | 1 (2,160 lines) | 3+ (CSS, JS, HTML separated) |
| CDN dependencies | 2 (marked, pdf.js) | 0 (bundled) |
| Automated backups | No | Daily |
| Linting in workflow | No | go vet + staticcheck |

## Priority Order

1. **Phase 0** (Quick Wins) — 2-3 hours, maximum impact
2. **Phase 1** (Error Handling) — 4-6 hours, eliminates silent data loss
3. **Phase 3** (Testing) — 4-6 hours, catch regressions before deployment
4. **Phase 2** (Architecture) — 6-8 hours, clean foundation for future features
5. **Phase 5** (Deployment) — 3-4 hours, operational reliability
6. **Phase 4** (Frontend) — 6-8 hours, lowest risk of breaking existing functionality

**Total estimated effort: 25-35 hours**
