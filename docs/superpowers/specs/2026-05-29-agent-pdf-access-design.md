# Agent PDF / Slide Access — Design

**Date:** 2026-05-29
**Status:** Approved, pending implementation plan

## Problem

The default chat path (Pi `/chat-v2`) cannot read the slides the user is
studying. During a CE-297 safety session the agent said *"the slides aren't
locally stored, so I'll draw on my knowledge of the Ch. 8 content"* and
reconstructed slide material from training data instead of reading the actual
PDF.

This is **not** a model issue (it happened on `glm-5.1`, persisted as a
pre-existing gap; it is independent of the `minimax-m2.7` swap done the same
day). The root cause is a wiring gap:

- The PDF text **is** extracted and cached server-side at
  `data/pdf-texts/<id>.txt` (per-page, split on `\n---PAGE BREAK---\n`,
  auto-populated on upload by `ExtractAndCachePDFText`).
- `claw-cli pdf extract --id N --pages X-Y` **already exists**
  (`claw-cli/main.go`, calls `App.ToolPDFExtract`) and reads that cache.
- **But** the Pi agent has (a) no way to *discover* which PDF id corresponds
  to the slides on screen, and (b) no instruction in `AGENTS.md` that PDFs
  exist or that it may read them. So `pdf extract` is unreachable in practice.

The uploaded PDFs live in a sibling dir of the Pi sandbox
(`data/pdf-files/`, sandbox is `data/agent-sessions/<id>/`), so the agent
cannot stumble onto them via the filesystem either.

## Goal

Give the Pi agent a reliable, instructed path to read the PDF the user is
currently viewing — without reconstructing content from memory.

## Non-goals

- No changes to `pdf extract` (already works).
- No corpus/RAG ingestion of slide PDFs (separate concern; the agent reading
  the cached text directly is sufficient and exact).
- No auto-injection of PDF text into every Pi turn (token-wasteful for a
  64-page PDF; on-demand reading is the chosen model).

## Design

### 1. Two new `claw-cli` subcommands (CLI wiring only — no new DB code)

Both reuse existing `*App` methods in `agent/db.go`. Added under the existing
`runPDF` dispatch in `claw-cli/main.go`, JSON to stdout, errors to stderr with
non-zero exit (matching every other subcommand).

**`claw-cli pdf list [--course <id>] [--db <path>]`**
- Calls `app.ListPDFs(course)` → `[]agent.PDFEntry`.
- Re-sorts the slice by `LastReadAt` **descending** (nil/empty sorts last) in
  the CLI, so the PDF currently being viewed is the first row. `ListPDFs`
  itself is left ordering by `uploaded_at DESC` because the UI depends on it.
- Emits `{"pdfs": [ {id, original_name, course_id, course_name, pages,
  last_page, last_read_at, uploaded_at}, ... ]}` (the `PDFEntry` JSON tags).

**`claw-cli pdf current [--session <id>] [--db <path>]`**
- Resolves the "current" PDF id:
  1. If `--session N` is provided and `app.GetSessionLastPDFID(N)` returns a
     positive id → use it (per-session precision).
  2. Otherwise fall back to `app.GetLastOpenedPDFID()`.
- If no id resolves (no PDF opened yet) → write a clear message to stderr
  (`pdf current: no PDF is currently open`) and exit 1.
- On success: `app.GetPDF(id)` → emit the single `PDFEntry` as JSON. The
  `last_page` field tells the agent which page the user is on.

`newAppFromEnv(resolvedDB, false)` is used (read-only, no API key required),
mirroring `pdfExtract`/`planStatus`.

### 2. `AGENTS.md` — new "Slides / PDFs" section

Added in `agent/sandbox.go:writeAgentsMD`, which already receives `sessionID`,
so the session id is baked literally into the instruction (same pattern as the
existing session-topic line). Placed before the pedagogy section.

Content (verbatim intent):

> ## Slides / PDFs
>
> The user studies from PDF documents shown in a viewer beside this chat.
> **Never reconstruct slide or document content from your own memory — read
> the actual pages.** To see what the user is currently reading:
> ```
> claw-cli pdf current --session <ID>
> ```
> This returns the open PDF's `id` and `last_page` (the page they're on).
> Then read the relevant pages:
> ```
> claw-cli pdf extract --id <id> --pages <range around last_page, e.g. 40-50>
> ```
> Use `claw-cli pdf list` to see every uploaded PDF (most-recently-read first).

### 3. Data caveat

The `pages` column is unreliable (the 64-page Ch.8 PDF stores `pages=1`). The
agent must rely on `last_page` for the user's position; `pdf extract` derives
the true page count from the cached text, so page ranges still resolve
correctly.

## Components & boundaries

| Unit | Responsibility | Depends on |
|------|----------------|------------|
| `pdf list` (CLI) | enumerate + sort PDFs, JSON out | `App.ListPDFs` |
| `pdf current` (CLI) | resolve current id, JSON out | `GetSessionLastPDFID`, `GetLastOpenedPDFID`, `GetPDF` |
| `writeAgentsMD` | instruct Pi how/when to read PDFs | (string only) |
| `ToolPDFExtract` | read page cache (unchanged) | `data/pdf-texts/` |

## Testing

Unit tests in `claw-cli/main_test.go` (mirroring existing subcommand tests),
using an in-test SQLite DB seeded with `pdfs` rows:

- `pdf list` — empty DB → `{"pdfs": []}`; populated → rows present, ordered by
  `last_read_at` desc (a recently-read row precedes an older-read row).
- `pdf current` — with a session that has a last PDF → returns that PDF;
  no session match but a last-opened PDF exists → returns the fallback;
  neither → exit 1 with the "no PDF is currently open" message.

Manual acceptance: build + deploy to `nanoclaw`, start a fresh Pi session on
the safety course while viewing `4.pdf`, ask about Ch. 8 ALARP, and confirm
the agent calls `pdf current` → `pdf extract` and quotes actual slide text
rather than reconstructing it.

## Deploy

Standard claw-study flow: `GOOS=linux GOARCH=amd64 go build` the server **and**
rebuild the `claw-cli` binary (the agent invokes the deployed `claw-cli`, so it
must be rebuilt and copied alongside `study-app`), then `systemctl --user
restart study-app.service`. Rebuilding `AGENTS.md` happens automatically on the
next session turn (it is re-written every `Create`).
