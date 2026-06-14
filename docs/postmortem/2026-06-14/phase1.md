Here's the triage analysis.

---

## 1. Ticket Table (sorted by ticket date)

| id | repo-state | outcome | exit | wall_s | cost | one-line note |
|---|---|---|---|---|---|---|
| 2026-05-21-debug-version-endpoint | done | shipped | 0 | 138 | 3.5MB | Trivial debug/version endpoint; clean pass |
| 2026-05-21-session-stats-endpoint | done | shipped | 0 | 117 | 3.9MB | Gate retry fixed NULL→int scan; shipped |
| 2026-05-25-stable-task-ids | done | shipped | 0 | 115 | 2.1MB | Stable plan IDs; smallest diff (38 lines) |
| 2026-05-25-courses-first-class | done | shipped | 0 | 182 | 5.0MB | Courses CRUD first-class entity |
| 2026-05-26-r4-confidence-log | done | failed-impl | 4 | 402 | 30.2MB | Diff budget overrun 355>350; impl complete, tests pass |
| 2026-05-26-rewrite-plan-tool | done | failed-gate | 13 | 311 | 2.9MB | Tool compiles+tests pass but not registered in runtime |
| 2026-05-27-rename-knowledge-component-id | done | failed-gate | 11 | 234 | 3.4MB | go build failed: unused import `"strings"` |
| 2026-05-28-knowledge-components-entity | done | shipped | 0 | 318 | 3.1MB | KC table entity with title column; shipped |
| 2026-05-29-knowledge-create-tool | done | shipped | 0 | 199 | 4.2MB | knowledge_create tool registration; shipped |
| 2026-05-30-retrieval-practice-loop | done | shipped | 0 | 285 | 3.2MB | Retrieval practice banding logic; shipped |
| 2026-06-01-sm2-spaced-review | failed | shipped | ? | 0 | 63.2MB | Non-standard result; gate.log shows build fail; manual fix deployed |
| 2026-06-02-fix-tool-panel-wipe | done | failed-gate | 10 | 278 | 15.1MB | Pre-baseline unexpectedly passed — spec already live? |
| 2026-06-04-r11-r12-session-rules | done | failed-gate | 10 | 101 | 27.4MB | Pre-baseline unexpectedly passed |
| 2026-06-05-r10-pretesting | done | failed-gate | 10 | 81 | 22.3MB | Pre-baseline unexpectedly passed |
| 2026-06-05-remove-plan-drawer | done | shipped | 0 | 104 | 22.5MB | Removed plan drawer from UI; gate PASSED despite 5 post-FAILs |
| 2026-06-08-bloom-enforcement | done | failed-gate | 10 | 296 | 7.0MB | Pre-baseline unexpectedly passed |
| 2026-06-08-kc-capture-box | done | failed-gate | 10 | 281 | 12.1MB | Pre-baseline unexpectedly passed (verifier `! grep -q` logic flagged) |
| 2026-06-10-r9-probe-infra | done | shipped | 0 | 245 | 5.7MB | Probe question infra + SM-2 integration; shipped |
| 2026-06-13-r9-full-pipeline | none | failed-gate | 12 | 352 | 34.3MB | Syntax error `unexpected %` + TestRule6OneLightOpener fail |

## 2. Discrepancies

### Result ↔ repo-state mismatches

| id | result.json outcome | spec dir | conflict |
|---|---|---|---|
| 2026-05-26-r4-confidence-log | `failed-impl` | `done/` | Pipeline rejected (budget), but spec filed under done |
| 2026-05-26-rewrite-plan-tool | `failed-gate` | `done/` | Gate exit 13 (post-acceptance fail), but spec in done |
| 2026-05-27-rename-knowledge-component-id | `failed-gate` | `done/` | Gate exit 11 (build fail), but spec in done |
| 2026-06-01-sm2-spaced-review | `shipped` | `failed/` | Result claims shipped, but spec in failed (manual fix, non-standard result.json) |
| 2026-06-02-fix-tool-panel-wipe | `failed-gate` | `done/` | Gate exit 10 (pre-baseline pass), but spec in done |
| 2026-06-04-r11-r12-session-rules | `failed-gate` | `done/` | Gate exit 10 (pre-baseline pass), but spec in done |
| 2026-06-05-r10-pretesting | `failed-gate` | `done/` | Gate exit 10 (pre-baseline pass), but spec in done |
| 2026-06-08-bloom-enforcement | `failed-gate` | `done/` | Gate exit 10 (pre-baseline pass), but spec in done |
| 2026-06-08-kc-capture-box | `failed-gate` | `done/` | Gate exit 10 (pre-baseline pass), but spec in done |

### Ticket with no spec dir at all

| id | result.json outcome | notes |
|---|---|---|
| 2026-06-13-r9-full-pipeline | `failed-gate` | Present in results but absent from `specs/done`, `specs/failed`, and `specs/queue` |

### Cost outliers (>5× median of 5.7MB)

| id | cost | ×median | notes |
|---|---|---|---|
| 2026-06-01-sm2-spaced-review | 63.2MB | 11.1× | Massive transcript; manual intervention rounds suspected |
| 2026-05-26-r4-confidence-log | 30.2MB | 5.3× | pi retried extensively within budget despite diff overrun |
| 2026-06-13-r9-full-pipeline | 34.3MB | 6.0× | High-effort implement-then-fail cycle; syntax + logic bugs |

## 3. Idle heartbeats

7 idle heartbeats from 2026-05-21 to 2026-06-13 (`idle-20260521T164207Z` through `idle-20260613T050020Z`).

---

PRIMARY HARD FAILURE: **2026-06-13-r9-full-pipeline**

---

# Verified corrections (Claude Code, against raw bundle)

Pi's triage is mostly accurate; verified the strong claims:

- **sm2-spaced-review** — result.json `outcome: shipped` with `fix_commit_sha 5c9f87e`, theory: `go build failed: handler/debug.go:85 still referenced removed RetrievalIntervalDays; fixed by replacing with ConfidenceToGrade`. gate.log confirms: pre-baseline FAILed correctly, then `go build` FAILED on the orphaned cross-package reference. So sm2 was a **build-break failure, MANUALLY SALVAGED + deployed 2026-06-03**. Spec still in `failed/` = bookkeeping not completed (should be `done/`). The "implementation that was supposed to happen" for sm2 ALREADY SHIPPED.

- **r9-full-pipeline** — ran **TODAY 2026-06-14 05:00**; pi_run OK (495 diff lines, 4 files, branch `agent/2026-06-13-r9-full-pipeline`), **gate FAILED exit 12 (`go test`)**, `branch_pushed: ""`, in **NO spec dir** (orphaned — bookkeeping bug: failed-gate should move spec to `failed/`). This is the genuine, fresh, UNSHIPPED implementation.

- **exit-10 cluster (6 tickets)** — root pattern found in kc-capture-box gate.log: `[WARN] pre-baseline verifier uses '! grep -q' pattern — this inverts the expected exit code`. So the pre-baseline verifier returns "pass" (feature looks already-present) → gate aborts exit 10 ("already satisfied or verifier broken"). A **systemic spec-authoring verifier bug**, not a feature problem. Orchestrator warns but aborts anyway.

- **remove-plan-drawer** — Pi's note "gate PASSED despite 5 post-FAILs" is **INACCURATE**: result.json shows all gate_steps exit 0, diff_lines 1, clean deploy. (Pi triage error — logged.)

## Three real problems (verified)
1. **r9-full-pipeline**: fresh hard `go test` failure, 495-line impl unshipped, spec orphaned. ← the true "implementation that was supposed to happen".
2. **Bookkeeping bug**: failure/salvage paths don't maintain spec dir state (r9 orphaned; sm2 stuck in failed/ after a successful salvage).
3. **Systemic verifier bug**: `! grep -q` inverted pre-baseline across ≥6 specs → gate self-aborts (exit 10).
