# Study Agent — Architecture Spec

## Vision

A specialist study agent that owns the full study lifecycle: orientation, note-taking, review, self-testing, and progress tracking. It is its own powerhouse — independent from Claw, with deep domain knowledge on *how to study*, not just course content.

Claw's role: build, maintain, and improve this agent. Claw is the dev team; the study agent is the product.

## Current State (Post Phase 1 Refactor)

```
Study App
├── main.go               — Thin HTTP handlers (routing, form parsing)
├── agent/                — Extracted agent package
│   ├── agent.go           — Global state, system prompt, vault helpers
│   ├── llm.go             — CallLLMNonStreaming (titles, session summaries)
│   ├── db.go              — DB init, message save/load, session helpers
│   └── types.go           — Shared types (Session, PDFEntry, JSONPlan, etc.)
├── static/index.html     — Frontend (session sidebar, scoped chat, PDF viewer)
├── data/
│   ├── study.db           — SQLite (sessions, messages, pdfs, meta)
│   ├── pdf-files/         — Uploaded PDF binaries
│   ├── pdf-texts/         — Cached extracted text (auto-generated)
│   ├── plans/             — Study plan JSON files
│   └── corpus/
│       ├── study-methods/ — 6 study method reference cards
│       ├── courses/        — (empty, for Phase 2)
│       └── meta/           — 2 meta documents
└── go.mod                 — Dependencies: modernc.org/sqlite, ledongthuc/pdf, html-to-markdown
```

**Runtime config:**
- Model: `opencode-go/qwen3.6-plus` (via opencode-go API)
- API: `https://opencode.ai/zen/go/v1` (OpenAI-compatible)
- Auth: `OPENCODE_API_KEY` → `LLM_API_KEY` (same credential as Claw)
- Env: `LLM_API_KEY`, `LLM_API_URL`, `LLM_MODEL`, `VAULT_ROOT`

## Target Architecture

```
┌─────────────────────────────────────────────────┐
│                  Study App                       │
│         (Frontend — static HTML/JS)              │
│  sessions │ pdf viewer │ plan viewer │ chat      │
└────────────────────┬────────────────────────────┘
                     │ HTTP/SSE
                     ▼
┌─────────────────────────────────────────────────┐
│              Study Agent (Go)                    │
│                                                  │
│  ┌──────────────┐  ┌─────────────────┐          │
│  │  Agent Core   │  │  API Gateway     │          │
│  │  (orchestrator│  │  /chat-v2,       │          │
│  │   + Pi runtime)│ │  /sessions, /api/*│         │
│  └──────┬───────┘  └─────────────────┘          │
│         │                                        │
│  ┌──────▼───────────────────────────┐            │
│  │        Tool Suite (7 tools)       │            │
│  │  ┌─────────┐  ┌────────────┐    │            │
│  │  │read_file │  │search_files│    │            │
│  │  ├─────────┤  ├────────────┤    │            │
│  │  │save_note │  │list_files  │    │            │
│  │  ├─────────┤  ├────────────┤    │            │
│  │  │web_fetch │  │pdf_extract │    │            │
│  │  ├─────────┤  └────────────┘    │            │
│  │  │study_skill│                   │            │
│  │  └─────────┘                     │            │
│  └──────────────────────────────────┘            │
│                                                  │
│  ┌──────────────────────────────────┐            │
│  │        Knowledge Base             │            │
│  │  ┌────────┐ ┌────────┐          │            │
│  │  │Course  │ │Study   │          │            │
│  │  │Corpus  │ │Methods │          │            │
│  │  │(md)    │ │Corpus │          │            │
│  │  │(Phase2)│ │(6 md)  │          │            │
│  │  └────────┘ └────────┘          │            │
│  │  ┌────────────────┐             │            │
│  │  │Personal Context │             │            │
│  │  │(plans, interests│             │            │
│  │  │ fleeting notes)  │             │            │
│  │  └────────────────┘             │            │
│  └──────────────────────────────────┘            │
│                                                  │
│  ┌──────────────────────────────────┐            │
│  │        SQLite (data layer)        │            │
│  │  sessions │ messages │ pdfs │ meta│            │
│  └──────────────────────────────────┘            │
│                                                  │
│  ┌──────────────────────────────────┐            │
│  │   Skills Engine (4 skills)       │            │
│  │  orientation │ study_notes       │            │
│  │  self_test   │ review            │            │
│  └──────────────────────────────────┘            │
└──────────────────────────────────────────────────┘
         │                              │
         │ A2A (optional, rare)         │ sync/config
         ▼                              ▼
   ┌──────────┐                  ┌──────────┐
   │   Claw    │                  │  Admin   │
   │ (builder/ │                  │  tool    │
   │ supervisor│                  │          │
   └──────────┘                  └──────────┘
```

## Core Concepts

### 1. Knowledge Corpus

The agent's domain knowledge lives in structured markdown, injected into prompts per session context.

**Current structure (Phase 2 started):**
```
data/corpus/
├── study-methods/
│   ├── active-recall.md       ✅ Active recall methods and techniques
│   ├── spaced-repetition.md   ✅ SR schedules and integration
│   ├── feynman-technique.md   ✅ Self-explanation framework
│   ├── error-diagnosis.md     ✅ Classifying & learning from mistakes
│   ├── orientation.md          ✅ Pre-reading strategy protocol
│   └── note-templates.md       ✅ Cornell, two-column, concept maps, etc.
├── courses/
│   ├── biology/
│   │   ├── stamp.md            ✅ STAMP control-theoretic model
│   │   ├── stpa.md             ✅ STPA four-step method
│   │   ├── cast.md             ✅ CAST retrospective analysis
│   │   ├── process-models.md   ✅ Controller belief states
│   │   └── coordination-failures.md ✅ Multi-controller conflicts
│   └── cs101/
│       ├── replication.md      ✅ Replication strategies and lag
│       ├── transaction-isolation.md ✅ Isolation levels and MVCC
│       ├── storage-engines.md  ✅ LSM-trees vs B-trees
│       ├── consensus.md        ✅ Raft, 2PC, FLP
│       ├── partitioning.md     ✅ Partitioning strategies
│       └── stream-processing.md ✅ CDC and event sourcing
└── meta/
    ├── how-to-study.md         ✅ Agent's study philosophy
    └── session-workflow.md     ✅ What happens in each session phase
```

**Retrieval strategy (progressive):**
1. **v0 (current):** Skills inject relevant corpus content into prompts via `study_skill` tool + system prompt includes tool descriptions
2. **v1:** Keywords + regex search scoped to corpus (via existing `search_files` tool)
3. **v2:** Vector embeddings + similarity search (local embedding model)

### 2. Skills Engine (Deployed)

Study-specific prompt chains that orchestrate multi-step interactions. Each skill is a Go function that assembles a structured prompt from templates + corpus data:

| Skill | Status | Input | Output |
|-------|--------|-------|--------|
| `orientation` | ✅ deployed | Topic + course | Pre-reading guide (prerequisites, key concepts, watch points, questions) |
| `study_notes` | ✅ deployed | Topic + course + optional content | Structured notes (summary, key concepts, formulas, review questions) |
| `self_test` | ✅ deployed | Topic + course + count | Exam-style questions with hints and answers |
| `review` | ✅ deployed | Topic + course | Spaced repetition assessment (quick recall questions, adaptive) |

Skills are invoked via the `study_skill` tool. The LLM decides when to call them based on user intent.

### 3. Session Lifecycle

```
1. Create session (course + topic)
2. Agent orients user (pre-reading primer from corpus)
3. User studies (reads, takes notes, asks questions)
4. Agent assists (answer questions, clarify, connect to corpus)
5. User can invoke skills (orientation, self-test, review)
6. Session ends → chat history preserved in SQLite
```

### 4. Tools

| Tool | Status | Notes |
|------|--------|-------|
| `read_file` | ✅ exists | Read workspace/vault files |
| `search_files` | ✅ exists | Ripgrep over files |
| `list_files` | ✅ exists | Directory listing |
| `save_note` | ✅ exists | Write notes to vault |
| `web_fetch` | ✅ implemented | Fetch web pages → markdown (rate limited: 5/min) |
| `pdf_extract` | ✅ implemented | Extract text from uploaded PDFs (with caching to pdf-texts/) |
| `study_skill` | ✅ implemented | Invoke named skill (orientation, study_notes, self_test, review) |
| `rag_search` | 🔲 Phase 2 | Semantic search over corpus chunks |

### 5. A2A Protocol (Claw ↔ Study Agent)

The A2A channel is for **infrastructure operations**, not runtime chat:

- **Deploy new skills** — Claw pushes a new skill definition
- **Update corpus** — Claw adds/updates course reference material
- **Sync config** — Model changes, prompt updates, tool configurations
- **Health check** — Is the agent alive? What version?
- **Debug** — Claw inspects agent state, session logs

The study agent does NOT call Claw mid-session. It's self-sufficient.

### 6. Model Strategy

- Current: `opencode-go/qwen3.6-plus` via local `opencode serve` (`http://127.0.0.1:4096`)
- Auth: the local opencode server reads `OPENCODE_API_KEY` from the environment — the study app does not need to manage credentials
- Configurable at runtime: `LLM_MODEL` env var, `LLM_API_URL` (default: `http://127.0.0.1:4096`), `LLM_API_KEY`
- Both streaming and non-streaming completions use the same endpoint (`/chat/completions`)
- Embeddings: configurable via `EMBEDDING_MODEL` (default: `nomic-ai/nomic-embed-text-v1.5`)
- Target: different models per skill type (future)

## Refactoring Plan

### Phase 0 — Foundation ✅ COMPLETE
- [x] Monolithic Go app with basic chat
- [x] Session-per-topic with course context
- [x] PDF viewer, plan viewer
- [x] 4 basic tools (read_file, search_files, list_files, save_note)

### Phase 1 — Modularize & Expand Tools ✅ COMPLETE
- [x] Extract agent core from HTTP handler into its own package (`agent/`)
- [x] Switch from OpenRouter to opencode-go API (`https://opencode.ai/zen/go/v1`)
- [x] Set `opencode-go/qwen3.6-plus` as default model (shares auth with Claw)
- [x] Add `pdf_extract` tool (ledongthuc/pdf, cached to data/pdf-texts/)
- [x] Add `web_fetch` tool (html-to-markdown, rate limited 5/min)
- [x] Add `study_skill` tool (4 skills: orientation, study_notes, self_test, review)
- [x] Build initial corpus: 6 study-methods files + 2 meta files
- [x] Skills: orientation, study_notes, self_test, review (all deployed)
- [x] System prompt updated with tool awareness

### Phase 2 — Corpus & RAG (In Progress)
- [x] Expand corpus with course concept cards (biology: stamp, stpa, cast, process-models, coordination-failures; cs101: replication, transaction-isolation, storage-engines, consensus, partitioning, stream-processing)
- [x] Wire course corpus into skill prompts (course-specific concept cards injected alongside study-methods)
- [ ] Add `rag_search` tool (vector similarity over corpus chunks)
- [ ] Local embedding model (sentence-transformers in Python sidecar, or WASM)
- [ ] PDF text auto-extraction on upload (trigger pdf_extract in background)
- [ ] Session summaries on close

### Phase 3 — A2A & Independence
- [ ] Define A2A protocol schema (JSON-RPC over HTTP)
- [ ] Implement A2A endpoints on study agent
- [ ] Implement A2A client in Claw
- [ ] Deploy as separate container
- [ ] Health checks, versioning, corpus update pipeline

### Phase 4 — Polish & Intelligence
- [ ] Knowledge graph construction (concept → prerequisite mappings)
- [ ] Progress analytics (time per topic, mastery estimates)
- [ ] Multi-model support (different models for different skills)
- [ ] Model selector in frontend header
- [ ] Review tracking (spaced repetition scheduling in DB)

## Data Model (Current)

### SQLite Schema
```sql
CREATE TABLE pdfs (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    filename      TEXT NOT NULL,
    original_name TEXT NOT NULL,
    course_id     TEXT,
    pages         INTEGER NOT NULL DEFAULT 0,
    last_page     INTEGER NOT NULL DEFAULT 1,
    uploaded_at   TEXT NOT NULL,
    last_read_at  TEXT
);

CREATE TABLE sessions (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    course_id   TEXT,
    topic       TEXT NOT NULL,
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL,
    last_pdf_id INTEGER,
    last_page   INTEGER DEFAULT 1
);

CREATE TABLE messages (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id INTEGER NOT NULL,
    role       TEXT NOT NULL,
    content    TEXT NOT NULL,
    created_at TEXT NOT NULL,
    FOREIGN KEY (session_id) REFERENCES sessions(id)
);

CREATE TABLE meta (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
```

### Target Additions (Phase 2+)
```sql
-- Corpus indexing
CREATE TABLE corpus_chunks (
    id INTEGER PRIMARY KEY,
    path TEXT NOT NULL,
    chunk_index INTEGER,
    content TEXT NOT NULL,
    embedding BLOB,
    course_id TEXT,
    category TEXT,  -- 'study-method', 'concept', 'pattern', 'pitfall'
    created_at TEXT
);

-- Review tracking
CREATE TABLE reviews (
    id INTEGER PRIMARY KEY,
    session_id INTEGER,
    topic TEXT NOT NULL,
    ease_factor REAL DEFAULT 2.5,
    interval_days INTEGER DEFAULT 1,
    next_review TEXT,
    review_count INTEGER DEFAULT 0,
    FOREIGN KEY (session_id) REFERENCES sessions(id)
);
```

### File-based Data
```
data/
├── study.db              — SQLite database
├── pdf-files/{id}.pdf    — Uploaded PDFs
├── pdf-texts/{id}.txt    — Cached extracted text (auto-generated)
├── plans/{id}.json       — Study plan files
└── corpus/
    ├── study-methods/    — 6 reference cards
    ├── courses/           — Course-specific concepts (Phase 2)
    └── meta/             — 2 workflow documents
```

## Key Decisions

1. **Corpus in markdown first** ✅ — Started with 8 files, upgrade retrieval later
2. **Skills are prompt templates** ✅ — Go functions that assemble prompts from templates + corpus data
3. **A2A is for infrastructure** — The study agent is self-sufficient at runtime
4. **One container for now** — Separate container comes in Phase 3, when A2A is ready
5. **Shares auth with Claw** ✅ — Uses `OPENCODE_API_KEY` via opencode-go API, no separate provider needed
6. **Agent package extracted** ✅ — `agent/` package with tools, LLM, DB, types, skills
7. **PDF text caching** ✅ — Extracted text cached to `data/pdf-texts/{id}.txt`, reused on subsequent queries