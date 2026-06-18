# PDF Viewer — Implementation Spec

## Audience

This spec is for a smaller coding model. It must be self-contained: read this file, then implement the feature. No ambiguous handoffs.

## Context

### The app

Study webapp for a graduate student. Stack: Go backend (`net/http`), single-page vanilla JS frontend (no framework), inline CSS, single binary deploy. No htmx — the existing app uses vanilla JS + fetch + SSE for all interactivity. No auth on local deploy.

Current pages: Chat (main), Plan (drawer). The PDF viewer will be a new view/tab accessible from the header.

Design language: Japanese minimalism, monochrome, MUJI-inspired. Soft white `#FAFAFA` backgrounds, `#1A1A1A` text, generous whitespace, no decorative elements. Full design spec in `DESIGN.md`.

### Why PDF matters

PDF reading is the **dominant friction** in study sessions. Today the student reads PDFs in Skim (macOS native viewer) alongside a terminal with nvim + AI chat. The goal is to bring the PDF viewer into the webapp so everything — reading, note-taking, AI conversation — lives in one interface.

### The study workflow (reference for all UX decisions)

1. **Upload a course PDF** — papers, textbook chapters, lecture slides
2. **Open a course PDF** — select from course-linked list
3. **Read continuously** — scroll through pages, jump to sections, flip back and forth
4. **Select key passages** — definitions, theorems, diagrams
5. **Ask the AI about what you just read** — "explain this theorem", "connect this to last week's concept"
6. **Take fleeting notes** — capture insights, questions, connections while reading
7. **Switch PDFs** — move between papers for the same course or across courses
8. **Resume where you left off** — PDF should remember last page per document

The viewer must support all of these flows without forcing the user into a rigid interaction pattern. The UX should emerge from this workflow, not from assumptions about "how PDF viewers work."

## Architecture: Storage & indexing

### PDF files → filesystem

PDF binary data is stored on disk under a configurable directory (default: `data/pdf-files/`). Each file is saved with a server-generated ID as the filename (e.g., `data/pdf-files/1.pdf`). This avoids naming collisions and keeps the DB small.

Rationale: PDFs are large binary blobs. Storing them in SQLite would bloat the DB, increase memory usage on reads (Go's `database/sql` loads full BLOBs into memory), and make backups harder. Filesystem storage is simpler, faster to stream, and standard practice for single-user local apps.

### PDF metadata → SQLite

A SQLite database (`data/study.db`) indexes all PDF metadata. This replaces the old JSON-file approach and gives us fast queries, course filtering, and progress tracking.

```sql
CREATE TABLE IF NOT EXISTS pdfs (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    filename    TEXT NOT NULL,           -- server-generated: "{id}.pdf"
    original_name TEXT NOT NULL,         -- user's upload filename
    course_id   TEXT,                    -- links to knownCourses (biology, cs101, etc.), nullable
    pages       INTEGER NOT NULL DEFAULT 0,
    last_page   INTEGER NOT NULL DEFAULT 1,
    uploaded_at TEXT NOT NULL,
    last_read_at TEXT
);
```

The `course_id` column links PDFs to courses from the study plan (same IDs: `biology`, `cs101`, `algorithms`, `history`, `research`). When uploading, the user picks a course from a dropdown (or leaves it uncategorized).

### Progress tracking

`last_page` and `last_read_at` are updated on page navigation. The "last opened" PDF is tracked via a single row in a key-value metadata table:

```sql
CREATE TABLE IF NOT EXISTS meta (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
-- row: key="last_opened_pdf", value="3"  (pdf id)
```

This replaces the old `data/pdf-progress.json` file.

## UX decisions (resolved)

These were decided in a design session with the user, grounded in their actual study workflow.

| Decision | Choice | Rationale |
|----------|--------|-----------|
| **Page surface** | Paper on desk | White page with soft shadow floating on `#FAFAFA` background. Generous margins. Feels like a physical document on a clean desk. Matches MUJI aesthetic. |
| **Toolbar** | Thin persistent bar (36px) | Always visible at top. Contains: filename, zoom controls, view-mode toggle, split toggle, close. Predictable, no discovery cost. Matches Skim's subdued bar. |
| **Split view** | Hard split, fixed 60/40 | 1px `--border` divider. PDF 60%, chat 40%. No drag, no resize. One toggle to switch modes. Clean, zero interaction tax. |
| **Text selection → chat** | Auto-draft to chat input | No popup. Selected text auto-populates into the chat input as a context prefix: `[p.42] "the controller ensures safety constraints..."`. User types their question after and sends. Zero extra clicks, flows naturally from read → ask. |
| **PDF list** | Auto-open last, course-grouped | When navigating to PDF, auto-open the last-read PDF at the last-read page. To switch: click filename → dropdown grouped by course. Uncategorised PDFs appear under "Library". |
| **Upload** | In-app drag & drop | Upload area in the "no PDF open" empty state. Drag-and-drop or file picker. On upload: extract page count, prompt for course assignment. No manual file placement needed. |
| **Page counter** | Bottom-right badge | Small semi-transparent badge in the bottom-right corner of the viewer: `42 / 187`. Doesn't overlap page content. The Skim default position. |

## What to build

### 1. PDF rendering

Use **PDF.js** (Mozilla's JS library, Apache 2.0). It renders PDFs to `<canvas>` elements in the browser. No server-side rendering needed.

- PDF.js is already vendored in `static/` as `pdf.mjs` and `pdf.worker.mjs`
- Render pages on demand (lazy — only render visible pages + 1 buffer above/below)
- Support: scroll view (continuous), single-page view, and a toggle between them
- **Paper-on-desk presentation**: each rendered page is a white `<canvas>` with `box-shadow: 0 1px 3px rgba(0,0,0,0.08)`, centered horizontally, with `margin: 24px auto` on the scroll container. Background of the scroll container is `var(--bg)` (`#FAFAFA`).
- Zoom: fit-to-width (default), fit-to-page, and manual +/- controls
- Keyboard shortcuts: `j`/`k` or arrow keys for page navigation, `+`/`-` for zoom, `/` for search

### 2. PDF upload & indexing

Upload is handled via a multipart form POST. The server extracts the page count using PDF.js on the server side (parse the PDF header for page count — a simple regex/binary scan is sufficient for well-formed PDFs, or use a lightweight Go library).

```
POST /pdf/upload          → Upload a PDF (multipart form: file + course_id)
GET  /pdf/list            → JSON list of all PDFs (optional ?course=biology filter)
GET  /pdf/file/:id        → Serve raw PDF file by DB id
GET  /pdf/progress/:id    → JSON: { id, last_page, last_read_at }
PUT  /pdf/progress/:id    → Body: { "page": 42 } → saves progress
```

Upload flow:
1. User drags a PDF onto the upload zone or clicks the file picker
2. JS sends `POST /pdf/upload` with the file and a `course_id` (selected from a dropdown of known courses, or empty for "Library")
3. Server saves the file to `data/pdf-files/{id}.pdf`, inserts a row in `pdfs` table with the page count
4. Server responds with the PDF metadata JSON
5. Frontend opens the newly uploaded PDF in the viewer

The upload zone appears in the empty state (when no PDF is open and no last-opened PDF exists). It can also be accessed from the filename dropdown (an "Upload PDF" entry at the top).

### 3. PDF view / tab

Add a **PDF** button in the header (alongside the existing "Plan" button). Clicking it enters the PDF view.

**Auto-open behavior:**
- Check `meta` table for `last_opened_pdf`
- If found: auto-open that PDF at its saved `last_page`
- If no PDF ever opened: show the empty state (upload zone + course-grouped PDF list)

**PDF open state:**
- Thin toolbar (36px height) at the top of the main content area, `background: var(--surface)`, `border-bottom: 1px solid var(--border)`
- Toolbar contents (left to right): close button (×), filename (clicking opens a dropdown of all PDFs grouped by course), view-mode toggle (continuous/single), zoom controls (−, %, +), split toggle button
- Below toolbar: the PDF scroll container, which renders the paper-on-desk pages
- Bottom-right: page counter badge (`42 / 187`), `background: rgba(250,250,250,0.8)`, `color: var(--text-dim)`, `font-size: 11px`, `padding: 2px 8px`, `border-radius: 4px`

### 4. Chat + PDF split view (critical feature)

When a PDF is open, the user should be able to see the chat **alongside** the PDF. This is the core workflow: read → ask → read → ask.

**Implementation:**
- When split is toggled on (via toolbar button or `s` key), the main area becomes a CSS flex row with two children:
  - Left: PDF viewer (flex: 3 — 60%)
  - Right: Chat panel (flex: 2 — 40%)
- Divider: `1px solid var(--border)` between the two panels
- The chat panel is the **same chat** as the main view — shared HTML, shared messages, shared SSE stream
- Three modes, cycled by the split toggle button or `s` key:
  1. Full PDF (no chat) — default when opening a PDF
  2. Split (PDF 60% + chat 40%)
  3. Full chat (no PDF) — same as Chat view but with PDF toolbar still showing filename
- On narrow screens (<768px), fall back to tab switching

### 5. Text selection → auto-draft to chat

When the user selects text in the PDF, it auto-populates into the chat input field.

**Implementation:**
- PDF.js creates a text layer (`<span>` elements) over each canvas for selection — this is built-in
- Listen for `mouseup` or `selectionchange` on the PDF viewer container
- On selection end, if `window.getSelection().toString().trim()` is non-empty:
  - Get the current page number
  - Format as: `[p.{page}] "{selected text}"`
  - Insert this as a prefix into the chat input field, followed by a space and the cursor
  - Do NOT clear previous input — append/prepend as appropriate
- If user clicks elsewhere (no selection), leave the drafted text in the input — they can still edit or clear it manually
- Do NOT auto-send. The user always confirms by pressing Send.

### 6. Keyboard navigation

The student works mouseless. The PDF viewer must support:

| Key | Action |
|-----|--------|
| `j` / `↓` / `Space` | Next page (or scroll down in continuous mode) |
| `k` / `↑` | Previous page (or scroll up) |
| `G` | Go to last page |
| `gg` | Go to first page |
| `:123` | Go to page 123 (vim-style command) |
| `+` / `=` | Zoom in |
| `-` | Zoom out |
| `0` | Reset zoom (fit-to-width) |
| `/` | Focus search bar (future — just focus chat input for now) |
| `q` | Close PDF / back to list |
| `s` | Toggle split view (full → split → full chat → full) |

These should only be active when the PDF viewer has focus (not when typing in chat input). Implementation: attach a `keydown` listener to the PDF viewer container, check `document.activeElement` is not the chat input.

### 7. Annotations (future, not MVP)

Annotations and highlights are explicitly out of scope. But don't paint into a corner:
- Reserve `PUT /pdf/annotations/:id` endpoint (returns 501 Not Implemented)
- PDF.js supports annotation layers — don't disable them, just don't build UI for creating annotations yet

### 8. PDF filename dropdown (course-grouped)

Clicking the filename in the toolbar opens a dropdown overlay listing all available PDFs from `/pdf/list`, grouped by course. Each group shows:
- Course name (e.g. "Intro to Computer Science (cs101)")
- Under each course: PDFs with original filename, page count, progress indicator ("p.42 / 187" or "not started")
- An "Upload PDF" entry at the top of the dropdown
- An "Library" group at the bottom for uncategorized PDFs

Clicking an entry opens that PDF (replaces current). Clicking outside or pressing `Escape` closes the dropdown.

## Technical constraints

- **No JavaScript framework.** Vanilla JS only (matching existing `index.html`). PDF.js is the only major JS dependency.
- **No htmx.** The existing app uses vanilla JS + fetch + SSE. Continue that pattern.
- **No build step.** PDF.js vendored as `static/pdf.mjs` and `static/pdf.worker.mjs`.
- **Go code** goes in the existing `main.go`. New SQLite logic in the same file for now (it's a monolith).
- **SQLite** via `modernc.org/sqlite` (pure Go, no CGO needed). Create `data/study.db` on first run. Use `database/sql` interface.
- **PDF files** stored in `data/pdf-files/` directory (created on first upload).
- **No auth** on local deploy (matching current app behavior).

## File structure after implementation

```
marginalia/
  static/
    index.html           (existing — add PDF viewer UI)
    pdf.mjs              (vendored PDF.js — already exists)
    pdf.worker.mjs        (vendored PDF.js worker — already exists)
  data/
    study.db              (new — SQLite metadata + progress)
    pdf-files/            (new — uploaded PDF binary storage)
      1.pdf
      2.pdf
    plans/                (existing)
      biology.json
      ...
  main.go                (modified — add PDF routes, SQLite init, upload handler)
  go.mod                 (modified — add sqlite dependency)
  SPEC-PDF-VIEWER.md     (this file)
  DESIGN.md              (existing)
```

## API endpoints to add

```
POST /pdf/upload          → Upload PDF (multipart: file + course_id). Returns PDF metadata JSON.
GET  /pdf/list            → JSON: [{ id, original_name, course_id, course_name, pages, last_page, last_read_at }]
                            Optional ?course=biology filter
GET  /pdf/file/:id        → Serve raw PDF file by DB id
GET  /pdf/progress/:id    → JSON: { id, last_page, last_read_at }
PUT  /pdf/progress/:id    → Body: { "page": 42 } → saves progress + updates last_opened_pdf
PUT  /pdf/annotations/:id → 501 Not Implemented (reserved for future)
```

## SQLite schema

```sql
CREATE TABLE IF NOT EXISTS pdfs (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    filename      TEXT NOT NULL,
    original_name TEXT NOT NULL,
    course_id     TEXT,
    pages         INTEGER NOT NULL DEFAULT 0,
    last_page     INTEGER NOT NULL DEFAULT 1,
    uploaded_at   TEXT NOT NULL,
    last_read_at  TEXT
);

CREATE TABLE IF NOT EXISTS meta (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
```

Known course IDs (matching the plan system): `biology`, `cs101`, `algorithms`, `history`, `research`.

## Design tokens for PDF viewer

All from the existing `DESIGN.md` palette — no new colors:

| Element | Token | Value |
|---------|-------|-------|
| Viewer background (scroll container) | `--bg` | `#FAFAFA` |
| PDF page (canvas) | — | `#FFFFFF` (white, PDF.js default) |
| PDF page shadow | — | `0 1px 3px rgba(0,0,0,0.08)` — paper-on-desk |
| PDF page margin | — | `24px auto` centered |
| Toolbar background | `--surface` | `#F5F5F5` |
| Toolbar border-bottom | `--border` | `1px solid #E0E0E0` |
| Toolbar height | — | `36px` |
| Toolbar button text | `--accent` | `#000000` |
| Toolbar button hover | `--accent-hover` | `#333333` |
| Page counter badge | `--text-dim` on `rgba(250,250,250,0.8)` | `#888888` |
| Page counter font | — | `11px`, uppercase, letter-spaced (matches timestamp style) |
| Split divider | `--border` | `1px solid #E0E0E0` |
| Filename dropdown bg | `--surface` | `#F5F5F5` |
| Filename dropdown border | `--border` | `1px solid #E0E0E0` |
| Selected-text draft prefix | `--text-dim` | `#888888` (prefix in chat input) |
| Upload zone border | `--border` | `2px dashed #E0E0E0` |
| Upload zone hover border | `--accent` | `#000000` |

## Acceptance criteria

1. Can upload a PDF via drag-and-drop or file picker in the empty state or dropdown
2. Upload assigns a course (or leaves uncategorized in "Library")
3. Navigating to PDF auto-opens the last-read PDF at the last-read page
4. PDF renders as white pages with soft shadow, centered on `#FAFAFA` background (paper-on-desk)
5. Can scroll/navigate with keyboard (j/k/G/gg/Space)
6. Can switch between continuous scroll and single-page view
7. Can zoom (fit-width default, manual +/-, fit-page)
8. Page counter shows in bottom-right corner as a semi-transparent badge
9. Page progress is saved on navigation and restored on reopen (SQLite)
10. Thin persistent toolbar (36px) shows: close, filename, view toggle, zoom, split toggle
11. Can toggle split view: full PDF → split (60/40) → full chat → back
12. Split-view chat is the same session as main chat
13. Selecting text in PDF auto-drafts `[p.X] "..."` into the chat input
14. Drafted text does NOT auto-send — user must press Send
15. Clicking filename opens dropdown of all PDFs grouped by course; clicking one opens it
16. Can close PDF and see empty state (upload zone + course-grouped list) if no last-opened PDF
17. All keyboard shortcuts work when PDF viewer has focus, not when chat input has focus
18. PDF viewer uses only B&W design tokens from DESIGN.md — no new colors
19. No JS framework — vanilla JS + PDF.js only (no htmx)
20. Works in Chrome and Safari (latest)
21. PDF list endpoint supports optional `?course=` filter
22. Upload endpoint extracts page count and stores in SQLite