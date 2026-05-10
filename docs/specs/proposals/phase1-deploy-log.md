# Phase 1 Deploy Log

Date: 2026-05-10
Operator: Eduardo (via Claude)
Target: `nanoclaw` VPS, service `study-app.service`, URL `https://study.claw-study.xyz`

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
- Health check: `https://study.claw-study.xyz/debug/health` returned `200`.

## Schema migration

`InitSchema` ran on startup and created the new `agent_memory` table. Verified via `sqlite3` (using Python's `sqlite3` module since the CLI binary is not installed on the VPS) — table is present alongside the existing `pdfs`, `sessions`, `messages`, `corpus_chunks`, `meta`.

## Seed run

- Source: `~/.claude/projects/-Users-eduardohiroji-Documents-ITA-Mestrado/memory/` tarred and extracted to `/home/eduardo/stack/study-app/data/memory/` (74 markdown files).
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
