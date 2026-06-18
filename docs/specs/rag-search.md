# RAG Search — Specification

## Problem Statement

The study agent currently loads **all** corpus files into every skill prompt. This:
- Wastes context window capacity
- Dilutes relevance (noise drowns signal)
- Doesn't scale as corpus grows

`rag_search` provides semantic retrieval: given a query, return only the corpus chunks that are actually relevant.

## Architecture

```
┌─────────────────────────────────────────────────┐
│              Study Agent (Go)                    │
│                                                  │
│  ┌──────────────────────────────────────────┐   │
│  │         Skill Invocation                  │   │
│  │  (orientation, study_notes, etc.)         │   │
│  └────────────────┬─────────────────────────┘   │
│                   │                              │
│  ┌────────────────▼─────────────────────────┐   │
│  │         RAG Pipeline                      │   │
│  │                                           │   │
│  │  1. Build query from topic + course       │   │
│  │  2. Embed query (OpenRouter API)          │   │
│  │  3. Cosine similarity vs chunk index      │   │
│  │  4. Return top-k chunks                   │   │
│  └────────────────┬─────────────────────────┘   │
│                   │                              │
│  ┌────────────────▼─────────────────────────┐   │
│  │         Prompt Builder                    │   │
│  │  Inject top-k chunks as context           │   │
│  │  (replaces "load all corpus" behavior)    │   │
│  └────────────────┬─────────────────────────┘   │
│                   │                              │
│  ┌────────────────▼─────────────────────────┐   │
│  │         LLM Call                          │   │
│  └───────────────────────────────────────────┘   │
│                                                  │
│  ┌──────────────────────────────────────────┐   │
│  │         SQLite (data layer)               │   │
│  │  corpus_chunks table (stored embeddings)  │   │
│  └───────────────────────────────────────────┘   │
└─────────────────────────────────────────────────┘
```

## Design Decisions

### 1. Embedding Source: OpenRouter API

**Decision**: Use OpenRouter embedding models (same API the agent already uses).

**Rationale**:
- No new infrastructure or process management
- Model-swappable via env config
- Low latency (~200ms per embed)
- Cost: fractions of a cent per search
- Can migrate to local embeddings in Phase 4 without changing the search interface

**Model**: `nomic-ai/nomic-embed-text-v1.5` via OpenRouter (8192 token context, 768-dim vectors, free tier available).

**Fallback**: If embedding API fails, fall back to keyword search (`search_files` behavior).

### 2. Chunking Strategy: By Heading

**Decision**: Each `##` section becomes a chunk. Split further if a section exceeds 1500 characters.

**Rationale**:
- Corpus is already well-structured markdown
- A `##` section is a natural semantic unit
- Preserves heading hierarchy for context
- Avoids cutting concepts mid-explanation

**Chunk format**:
```json
{
  "id": 1,
  "path": "courses/biology/stamp.md",
  "heading": "Core Idea",
  "parent_heading": "",
  "content": "Accidents are not caused by component failures...",
  "course_id": "biology",
  "category": "concept",
  "embedding": [0.12, -0.34, ...],
  "created_at": "2026-05-05T21:00:00Z"
}
```

### 3. Storage: SQLite

**Decision**: Store embeddings in SQLite alongside chunk text.

**Rationale**:
- App already uses SQLite — no new database
- Embeddings stored as BLOBs in a `corpus_chunks` table
- Similarity search via application-level cosine computation (no extension needed for Phase 2; corpus is small enough ~100 chunks)
- If corpus grows to 1000+ chunks, add `sqlite-vec` extension

### 4. Search Interface

```
rag_search(query, course?, top_k?)
→ [{chunk_text, source_file, heading, score}]
```

| Parameter | Type | Required | Default | Description |
|---|---|---|---|---|
| `query` | string | yes | — | Natural language search query |
| `course` | string | no | `""` | Scope to course ID (biology, cs101) |
| `top_k` | int | no | `3` | Number of results to return |

### 5. Integration: Automatic Injection

**Decision**: RAG runs automatically during skill prompt building, not as an LLM tool call.

**Rationale**:
- LLM shouldn't need to "remember" to search
- When a skill fires with topic + course, the system proactively retrieves relevant chunks
- Replaces the current "load all corpus files" behavior
- `search_files` remains available for explicit keyword searches

## Data Model

### New Table: `corpus_chunks`

```sql
CREATE TABLE corpus_chunks (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    path          TEXT NOT NULL,          -- relative path in corpus/
    heading       TEXT NOT NULL DEFAULT '', -- ## heading text
    parent_heading TEXT NOT NULL DEFAULT '', -- # heading text (context)
    content       TEXT NOT NULL,          -- chunk text
    embedding     BLOB,                   -- float32 array (768 dims)
    course_id     TEXT,                   -- 'biology', 'cs101', or NULL
    category      TEXT NOT NULL DEFAULT 'concept', -- 'study-method' | 'concept' | 'pattern' | 'pitfall'
    created_at    TEXT NOT NULL,
    updated_at    TEXT NOT NULL
);

CREATE INDEX idx_corpus_chunks_course ON corpus_chunks(course_id);
CREATE INDEX idx_corpus_chunks_path ON corpus_chunks(path);
```

## Corpus Indexing Pipeline

```
data/corpus/**/*.md
       ↓
  Parse markdown (split by ##)
       ↓
  For each chunk:
    1. Extract heading + parent heading
    2. Infer course_id from path
    3. Call embedding API
    4. Store in corpus_chunks
       ↓
  On startup: check if chunks are stale
    (compare file mtimes vs updated_at)
    Re-index changed files only
```

## Search Algorithm

```
1. Embed the query → vector q (768-dim)
2. Load all chunk embeddings from SQLite (or filter by course_id)
3. Compute cosine similarity: cos(q, e_i) for each chunk i
4. Sort by score descending
5. Return top-k chunks with metadata
```

Cosine similarity: `cos(a, b) = (a · b) / (||a|| * ||b||)`

## Error Handling

| Failure | Behavior |
|---|---|
| Embedding API unavailable | Fall back to keyword search (`search_files`) |
| No chunks indexed yet | Return empty, log warning |
| Query returns no relevant results (all scores < 0.3) | Return empty — don't inject noise |
| SQLite unavailable | Fatal error (same as current DB behavior) |

## Non-Goals (Phase 2)

- Re-ranking (cross-encoder)
- Multi-vector / hybrid search (BM25 + dense)
- Chunk-level access control
- Real-time re-indexing on file save
- Vector database migration (Pinecone, Weaviate, etc.)
