# Phase 1 Deploy Log

Date: 2026-05-10
Operator: Eduardo (via Claude)
Target: `nanoclaw` VPS, service `study-app.service`, URL `https://your-host.example`

## Commit range

Phase 1 covers the 7 commits from `agent: add agent_memory table` through `seed-memory: import frontmatter-tagged memory`:

```
ff4181f seed-memory: import frontmatter-tagged memory into agent_memory
0e8e700 claw-cli: memory load subcommand emits AGENTS.md
875584e claw-cli: memory search subcommand
00bd66c claw-cli: skeleton + memory save subcommand
667e0ad agent: AGENTS.md assembler with recent-sessions + skills frontmatter
b90f9aa agent: add MemoryStore with save/search/load-by-scope
6739763 agent: add agent_memory table to InitSchema
```

## Build + deploy

- Cross-compiled three Linux/amd64 binaries on macOS with `/opt/homebrew/bin/go` (Go 1.26.3): `claw-cli` (10.9 MB), `seed-memory` (10.7 MB), `study-app` (17.5 MB).
- Hot-swapped the server: backed up old binary to `study-app.bak`, moved `study-app.new` to `study-app`, restarted via `systemctl --user restart study-app.service`. Service came up `active` on first try.
- Health check: `https://your-host.example/debug/health` returned `200`.

## Schema migration

`InitSchema` ran on startup and created the new `agent_memory` table. Verified via `sqlite3` (using Python's `sqlite3` module since the CLI binary is not installed on the VPS) — table is present alongside the existing `pdfs`, `sessions`, `messages`, `corpus_chunks`, `meta`.

## Seed run

- Source: `~/.claude/projects/<project-slug>/` tarred and extracted to `$VAULT_ROOT/data/memory/` (74 markdown files).
- `seed-memory` collected 24 candidate rows and seeded all 24. Matches the laptop dry-run.
- Kind breakdown after seed:

```
feedback|14
profile|1
project|7
reference|2
```

(Total 24 rows.)

## AGENTS.md verification

- Generated for course `ce297` and user `eduardo`.
- Final size: **2673 bytes** — well under the 3072-byte cap.
- All five sections present: `# AGENTS.md`, `## User profile`, `## Course context: ce297`, `## Active feedback rules`, `## Recent sessions`, `## Available skills`.
- Profile populated from real seed data (mentions Eduardo, ITA, advisor Juliana Bezerra, CE-297).
- Course context populated with ce297-scoped memories (AppSTPA tool setup).
- Feedback section populated with `feedback_concept_introduction` and consolidation rules.
- Skills section: `_(none yet)_` (expected — Phase 3 will populate).

## Smoke tests

- `claw-cli memory search --query "abbreviations"` returned two relevant feedback hits (`feedback_concept_introduction` id=10 and `feedback_notes_no_abbreviations` id=19).
- `claw-cli memory save` wrote a `phase1-smoke` feedback entry (id=25); re-running `memory load` confirmed it surfaced in the resulting AGENTS.md.

## Deviations / surprises

- VPS does not have `sqlite3` CLI installed; substituted Python's `sqlite3` module for verification. Not blocking.
- macOS `tar` emits noisy `LIBARCHIVE.xattr.com.apple.provenance` warnings; cosmetic only, files extract cleanly.

## Status

DONE. All Step 11 commits pushed below. Restic nightly will pick up the new `data/memory/` extracted tree and the 25-row `agent_memory` table on its next run.

## Pre-prep deploy 2026-05-10

Three Phase-2-prep fixes shipped. Commits: `9c346cb..65621aa`.

- TruncateRunes UTF-8-safe truncation: replaces 6 byte-slice sites
- claw-cli paths via CLAW_STUDY_ROOT: 3 handlers, missing-explicit-path errors
- seed-memory transactional + mtime-based created_at

Same hot-swap pattern as Phase 1: backed up `study-app` to `study-app.bak`, swapped in new build, `systemctl --user restart study-app.service` came up `active` on first try. Cross-compiled `study-app` (17.5 MB), `claw-cli` (10.9 MB), `seed-memory` (10.7 MB) for linux/amd64.

Appended `CLAW_STUDY_ROOT=$VAULT_ROOT` to `~/stack/study-app/.env` (the only .env mutation).

Verification:
- AGENTS.md from canonical cwd (`cd ~/stack/study-app`): 2664 bytes
- AGENTS.md from `/tmp` via `CLAW_STUDY_ROOT`: 2664 bytes (diff: identical) — confirms Pi-sandbox path-resolution fix
- Missing-explicit-DB error: stderr `database not found at "/tmp/no-such.db": stat /tmp/no-such.db: no such file or directory`, exit 1
- Seeded 24 rows; distinct `created_at`: 24 (range: 1774571985..1778387859) — confirms mtime-based reseed (not all `time.Now()`)
- Live `/debug/health`: 200

## Status

DONE.

## Phase 2 deploy 2026-05-10

**Commit range:** `47f7a04..9c1b67e`

```
9c1b67e claw-cli: strengthen skill dispatch test assertion
ae78496 claw-cli: skill dispatch subcommand
e5d2586 claw-cli: fix flag order in web fetch test
53d4b56 claw-cli: web fetch subcommand
5adace4 claw-cli: pdf extract subcommand
...
47f7a04 (phase 2 start)
```

### Build + deploy

- Cross-compiled two Linux/amd64 binaries on macOS with `/opt/homebrew/bin/go`: `study-app` (17.5 MB), `claw-cli` (15.1 MB).
- Hot-swapped: backed up old binary to `study-app.bak`, swapped `study-app.new → study-app`, restarted `systemctl --user restart study-app.service`. Service came up `active` on first try.
- `claw-cli` replaced in-place (no service restart needed for CLI binary).

### Subcommand smoke results

All commands run from `cd $VAULT_ROOT` with `.env` sourced (`set -a; source .env; set +a`).

| Subcommand | Result |
|---|---|
| `plan show --course ce297` | `plan not found for course "ce297"` (expected — no plan seeded on VPS) |
| `plan toggle --course ce297 --task 999` | `error: plan not found: ce297` (expected — intentional bad index) |
| `course interests --course ce297` | `interests not found at ".../memory/courses/ce297/interests.md"` (expected — file not present on VPS) |
| `note save --course ce297 --kind fleeting --content "phase2 smoke"` | `saved to .../memory/courses/ce297/fleeting/2026-05-10T214735.md` — OK |
| `pdf extract --id 999` | `error: PDF not found (id: 999)` (expected — intentional bad id) |
| `web fetch --db ./data/study.db https://example.com` | Returned title "Example Domain" and body excerpt — OK |
| `skill dispatch --skill orientation --topic STAMP --course ce297` | Printed full orientation prompt with Prerequisites/Key Concepts/Watch Points sections — OK |

Note: `plan show`, `plan toggle`, and `course interests` return expected "not found" errors (no corpus seeded on VPS); these are not regressions. `note save`, `pdf extract`, `web fetch`, and `skill dispatch` all behave correctly.

### RAG search

```
rag search --query "STAMP" --course ce297 --top-k 3
```

- 3 hits returned from `courses/ce297/cast.md` (CAST/STAMP-related chunks), all at score 0.500.
- Latency: **320 ms** wall-clock (real `0m0.320s`).

### Health check

- `GET /debug/health` with Bearer token → **200 OK**. Live `/chat` UI unaffected.

### Deviations / surprises

- Without sourcing `.env` first, `course interests` and `note save` fail with missing-env errors (`VAULT_ROOT or CLAW_STUDY_ROOT must be set` and `mkdir /workspace: permission denied`). This is expected behaviour — the service process itself has `.env` loaded via the systemd unit; the raw smoke script needed explicit sourcing.
- No panics, no "command not found", no regressions.

### Status

DONE.

## Phase 5 — Pi SSE proxy (2026-05-11)

Deployed commits eb6f21d..bfe9a2e to VPS. .env updated with PI_PATH,
SKILLS_DIR, AGENT_MODEL=deepseek-v4-pro.

Two bugs found and fixed during deploy:
- `--skills-dir` flag does not exist in pi v0.74.0; correct flag is `--skill <dir>` (agent/pi_runner.go).
- Closing stdin immediately after sending the prompt caused pi to exit before
  making the LLM call; stdin is now closed by the goroutine after agent_end
  is received (agent/pi_runner.go).

Both fixes were committed and redeployed on the same day.

Smoke results:
- POST /chat-v2 streams SSE with token events ✅
- event: done received at end of turn ✅
- reasoning events (thinking_delta) stream before text_delta tokens ✅
- /debug/health 200 ✅
- No panics or errors in service logs ✅

## Phase 4 — Sandbox manager (2026-05-10)

Deployed commits from Phase 4 to VPS.

Smoke results:
- POST /chat-v2 creates sandbox at data/agent-sessions/<id>/ ✅
- Second POST /chat-v2 reuses same path ✅
- DELETE /api/sessions removes sandbox ✅
- Sandbox contains AGENTS.md, notes/, out -> agent-out ✅
- /debug/health 200 ✅
