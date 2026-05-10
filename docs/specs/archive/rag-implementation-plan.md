# RAG Search — Implementation Plan

## Overview

Build a RAG (Retrieval-Augmented Generation) pipeline that replaces the current "load all corpus files" behavior with semantic retrieval. The pipeline chunks corpus markdown, embeds chunks via OpenRouter API, stores in SQLite, and retrieves relevant chunks during skill invocation.

## Implementation Steps

### Step 1: Corpus Chunker (`agent/chunker.go`)

**What**: Parse markdown files and split into semantic chunks by heading.

**File**: `agent/chunker.go`

**Functions**:

```go
type Chunk struct {
    Path          string
    Heading       string
    ParentHeading string
    Content       string
    CourseID      string
    Category      string
}

func ChunkFile(path string) ([]Chunk, error)
```

**Logic**:
1. Read markdown file
2. Split by `## ` headings (level 2 and below)
3. For each section:
   - Extract the heading text
   - Track the parent `#` heading for context
   - If section > 1500 chars, split into sub-chunks at paragraph boundaries (`\n\n`)
   - Infer `course_id` from path (e.g., `courses/ce297/stamp.md` → `ce297`)
   - Infer `category` from path:
     - `study-methods/` → `study-method`
     - `courses/` → `concept`
     - `meta/` → `study-method`

**Edge cases**:
- Files with no `##` headings → single chunk with empty heading
- Empty sections → skip
- Code blocks within sections → keep as part of chunk content

### Step 2: Embedding Client (`agent/embed.go`)

**What**: Call OpenRouter embedding API to get vector representations.

**File**: `agent/embed.go`

**Functions**:

```go
func EmbedText(text string) ([]float32, error)
func EmbedBatch(texts []string) ([][]float32, error)
```

**Implementation**:
- Use existing OpenRouter HTTP client (reuse from `agent/llm.go`)
- Endpoint: `POST /openai/deployments/{model}/embeddings` (OpenAI-compatible)
- Model: `nomic-ai/nomic-embed-text-v1.5` (configurable via env `EMBEDDING_MODEL`)
- Truncate text to 8192 tokens (model limit)
- Parse response: `data[0].embedding` → `[]float32`

**Env vars**:
- `EMBEDDING_MODEL` — default: `nomic-ai/nomic-embed-text-v1.5`
- Uses same `LLM_API_URL` and `LLM_API_KEY` as the main LLM

**Rate limiting**: Embeddings are cheap but add a 100ms delay between batch calls to avoid hitting rate limits.

### Step 3: Vector Store (`agent/vectorstore.go`)

**What**: Store and retrieve embeddings in SQLite.

**File**: `agent/vectorstore.go`

**Functions**:

```go
func InitVectorStore() error                          // Create corpus_chunks table
func IndexCorpus() error                              // Full re-index of corpus/
func IndexFile(path string) error                     // Index a single file
func Search(query string, course string, topK int) ([]SearchResult, error)
func NeedsReindex(path string) (bool, error)          // Check if file changed since last index
```

**Schema creation** (`InitVectorStore`):
```sql
CREATE TABLE IF NOT EXISTS corpus_chunks (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    path          TEXT NOT NULL,
    heading       TEXT NOT NULL DEFAULT '',
    parent_heading TEXT NOT NULL DEFAULT '',
    content       TEXT NOT NULL,
    embedding     BLOB,
    course_id     TEXT,
    category      TEXT NOT NULL DEFAULT 'concept',
    created_at    TEXT NOT NULL,
    updated_at    TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_corpus_chunks_course ON corpus_chunks(course_id);
CREATE INDEX IF NOT EXISTS idx_corpus_chunks_path ON corpus_chunks(path);
```

**Embedding storage**:
- Serialize `[]float32` to bytes: `binary.Write(buf, binary.LittleEndian, floats)`
- Deserialize: `binary.Read(bytes, binary.LittleEndian, &floats)`
- 768 dimensions × 4 bytes = 3072 bytes per chunk

**Indexing logic** (`IndexCorpus`):
1. Walk `data/corpus/**/*.md`
2. For each file:
   - Check `NeedsReindex` (compare file mtime vs max `updated_at` for that path)
   - If stale or new: `ChunkFile` → `EmbedBatch` → upsert rows
   - Delete rows for paths that no longer exist

**Search logic** (`Search`):
1. `EmbedText(query)` → query vector
2. `SELECT path, heading, parent_heading, content, embedding, course_id FROM corpus_chunks WHERE course_id = ? OR ? = ''`
3. For each row: compute cosine similarity with query vector
4. Sort by score descending, return top-k

**Cosine similarity** (pure Go, no dependencies):
```go
func CosineSimilarity(a, b []float32) float64 {
    var dot, normA, normB float64
    for i := range a {
        dot += float64(a[i]) * float64(b[i])
        normA += float64(a[i]) * float64(a[i])
        normB += float64(b[i]) * float64(b[i])
    }
    return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
```

**SearchResult type**:
```go
type SearchResult struct {
    SourceFile    string
    Heading       string
    ParentHeading string
    Content       string
    Score         float64
}
```

### Step 4: Integrate with Skills (`agent/tools.go`)

**What**: Replace "load all corpus" with RAG retrieval in skill prompt building.

**Changes to `executeStudySkill`**:

Current behavior:
```go
// Loads ALL study-methods files into corpusContent
corpusDir := filepath.Join(VaultRoot, "data", "corpus", "study-methods")
// ... reads every file ...
```

New behavior:
```go
// Build query from topic + course
query := topic
if courseID != "" {
    query = courseName + " " + topic
}

// RAG search
results, err := Search(query, courseID, 3)
var corpusContent string
if err == nil && len(results) > 0 {
    for _, r := range results {
        heading := r.Heading
        if heading == "" {
            heading = r.ParentHeading
        }
        corpusContent += "\n\n--- " + r.SourceFile + " (" + heading + ") ---\n" + r.Content
    }
} else {
    // Fallback: load all corpus (current behavior)
    corpusContent = loadAllCorpus(courseID)
}
```

**Changes to skill prompt generators** (`generateOrientation`, `generateStudyNotes`, `generateSelfTest`, `generateReview`):
- Already accept `corpusContent` parameter (from the corpus expansion change)
- No changes needed — they already inject `corpusContent` into the prompt when non-empty

**Fallback chain**:
1. RAG search succeeds with results → inject top-k chunks
2. RAG search returns no results → fall back to loading all course-specific corpus files
3. No course corpus available → fall back to loading all study-methods files
4. Embedding API fails → fall back to keyword search (`search_files`)

### Step 5: Startup Indexing (`main.go`)

**What**: Index corpus on app startup.

**Changes to `main.go`**:

In the `init()` or startup sequence (after DB is opened):
```go
func init() {
    // ... existing DB init ...
    agent.InitVectorStore()
    go func() {
        if err := agent.IndexCorpus(); err != nil {
            log.Printf("corpus indexing failed: %v", err)
        } else {
            log.Printf("corpus indexed successfully")
        }
    }()
}
```

Run indexing in a goroutine so it doesn't block server startup. The search function handles "no chunks yet" gracefully.

### Step 6: Expose as Tool (Optional)

**What**: Add `rag_search` as an explicit tool the LLM can call (in addition to automatic injection).

**File**: `agent/tools.go` — add to `GetTools()` and `ExecuteTool()`

```go
{
    Type: "function",
    Function: ToolFunc{
        Name:        "rag_search",
        Description: "Search the knowledge corpus using semantic similarity. Use this when you need to find relevant context for a topic.",
        Parameters: map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "query":   map[string]interface{}{"type": "string", "description": "Search query"},
                "course":  map[string]interface{}{"type": "string", "description": "Optional course ID to scope search"},
                "top_k":   map[string]interface{}{"type": "integer", "description": "Number of results (default: 3)"},
            },
            "required": []string{"query"},
        },
    },
}
```

**ExecuteTool case**:
```go
case "rag_search":
    var p struct {
        Query  string `json:"query"`
        Course string `json:"course"`
        TopK   int    `json:"top_k"`
    }
    json.Unmarshal(args, &p)
    if p.TopK <= 0 {
        p.TopK = 3
    }
    results, err := Search(p.Query, p.Course, p.TopK)
    if err != nil {
        return "error: " + err.Error()
    }
    var out string
    for _, r := range results {
        out += fmt.Sprintf("\n--- %s (%s) [score: %.3f] ---\n%s\n", r.SourceFile, r.Heading, r.Score, r.Content)
    }
    if out == "" {
        return "No relevant results found."
    }
    return out
```

**Decision**: Implement this but don't advertise it in the system prompt initially. The automatic injection handles 90% of cases. The explicit tool is there for edge cases where the LLM needs to do a targeted search mid-conversation.

## Testing Plan

### Unit Tests

1. **Chunker tests** (`agent/chunker_test.go`):
   - File with multiple `##` sections → correct splits
   - File with no `##` → single chunk
   - Section > 1500 chars → split at paragraph boundaries
   - Course ID inference from path

2. **Vector store tests** (`agent/vectorstore_test.go`):
   - Cosine similarity: identical vectors = 1.0, orthogonal = 0.0
   - Search returns results sorted by score
   - Course scoping filters correctly
   - Re-index detects changed files

3. **Embedding tests** (`agent/embed_test.go`):
   - Mock the HTTP call (use httptest)
   - Verify request format and response parsing
   - Test error handling (timeout, bad response)

### Integration Tests

1. Start app with test corpus
2. Verify `corpus_chunks` table is populated
3. Call a skill and verify relevant chunks are injected
4. Verify fallback behavior when embedding API is unavailable

## Rollout

1. **Build and test locally** — all 6 steps
2. **Index existing corpus** — run `IndexCorpus()` and verify chunk count (~100 chunks expected)
3. **Test with real sessions** — create study sessions for CE-297 and DDIA topics
4. **Compare prompts** — verify that injected chunks are more relevant than the previous "load all" approach
5. **Monitor** — check embedding API latency and cost in the first week

## Dependencies

| Dependency | Version | Purpose |
|---|---|---|
| `modernc.org/sqlite` | existing | SQLite driver (already in go.mod) |
| OpenRouter embedding API | — | Vector generation (no new package) |
| None | — | No new Go dependencies needed |

## Estimated Effort

| Step | Complexity | Time |
|---|---|---|
| 1. Chunker | Low | 1-2 hrs |
| 2. Embedding client | Low | 1 hr |
| 3. Vector store | Medium | 2-3 hrs |
| 4. Skill integration | Low | 1 hr |
| 5. Startup indexing | Low | 30 min |
| 6. Tool exposure | Low | 30 min |
| Tests | Medium | 2-3 hrs |
| **Total** | | **8-12 hrs** |
