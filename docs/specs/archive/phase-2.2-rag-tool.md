# Phase 2.2 — RAG Search Tool

## Spec

### Overview

Expose the existing RAG search pipeline as an explicit tool (`rag_search`) that the LLM can call mid-conversation. The infrastructure already exists (`Search()`, `ChunkFile`, `EmbedText`, `vectorstore.go`), so this is purely about wiring it into the tool registry and adding an ExecuteTool case.

### Why

Currently RAG retrieval happens automatically inside `executeStudySkill` — the LLM has no direct way to search the corpus outside of skill invocation. An explicit tool lets the agent:

- Do targeted searches mid-conversation ("what does the corpus say about STPA step 2?")
- Combine RAG results with other tools (e.g., `rag_search` + `web_fetch`)
- Search across courses without invoking a specific skill

### Behavior

- **Tool name:** `rag_search`
- **Parameters:** `query` (required string), `course` (optional string), `top_k` (optional integer, default: 3)
- **Returns:** Formatted results with source file, heading, content, and similarity score
- **Fallback:** If embedding API is unavailable, falls back to keyword search (already handled by `Search()`)
- **Empty results:** Returns "No relevant results found."

### Output Format

```
--- study-methods/active-recall.md (Active Recall Methods) [score: 0.847] ---
[chunk content here]

--- courses/biology/stpa.md (The Four Steps) [score: 0.712] ---
[chunk content here]
```

---

## Implementation Plan

### Step 1: Add `rag_search` tool definition

**File:** `agent/tools.go` — `GetTools()`

Add a new entry to the tools slice (after `study_skill`):

```go
{
    Type: "function",
    Function: ToolFunc{
        Name:        "rag_search",
        Description: "Search the knowledge corpus using semantic similarity. Use this when you need to find relevant context for a topic or concept.",
        Parameters: map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "query":  map[string]interface{}{"type": "string", "description": "Search query (topic, concept, or question)"},
                "course": map[string]interface{}{"type": "string", "description": "Optional course ID to scope search (e.g. biology, cs101)"},
                "top_k":  map[string]interface{}{"type": "integer", "description": "Number of results (default: 3, max: 10)"},
            },
            "required": []string{"query"},
        },
    },
},
```

### Step 2: Add ExecuteTool case

**File:** `agent/tools.go` — `ExecuteTool()`

Add a new case in the switch statement:

```go
case "rag_search":
    return executeRAGSearch(args)
```

### Step 3: Implement `executeRAGSearch`

**File:** `agent/tools.go` — new function

```go
func executeRAGSearch(args json.RawMessage) string {
    var p struct {
        Query  string `json:"query"`
        Course string `json:"course"`
        TopK   int    `json:"top_k"`
    }
    if err := json.Unmarshal(args, &p); err != nil {
        return "error: " + err.Error()
    }

    if p.Query == "" {
        return "error: query is required"
    }

    if p.TopK <= 0 {
        p.TopK = 3
    }
    if p.TopK > 10 {
        p.TopK = 10
    }

    results, err := Search(p.Query, p.Course, p.TopK)
    if err != nil {
        return "error: " + err.Error()
    }

    if len(results) == 0 {
        return "No relevant results found for: " + p.Query
    }

    var out strings.Builder
    for _, r := range results {
        heading := r.Heading
        if heading == "" {
            heading = r.ParentHeading
        }
        if heading == "" {
            heading = r.SourceFile
        }
        fmt.Fprintf(&out, "\n--- %s (%s) [score: %.3f] ---\n%s\n",
            r.SourceFile, heading, r.Score, r.Content)
    }

    return strings.TrimPrefix(out.String(), "\n")
}
```

### Step 4: Update system prompt

**File:** `agent/agent.go` or wherever `LoadSystemPrompt()` lives

Add `rag_search` to the tools overview section. Current prompt mentions 7 tools — add an 8th:

```
- **rag_search** — Search the knowledge corpus using semantic similarity. Use when you need to find relevant context for a topic or concept.
```

### Step 5: Test

1. Build and restart the app
2. In a study session, ask: "Search the corpus for STPA"
3. Verify the agent calls `rag_search` and returns relevant chunks
4. Test course scoping: "Search for replication in cs101"
5. Test fallback: verify keyword search works if embedding API is down

---

## Summary

| Step | What | Time |
|------|------|------|
| 1 | Add tool definition to GetTools() | 5 min |
| 2 | Add ExecuteTool case | 2 min |
| 3 | Implement executeRAGSearch() | 15 min |
| 4 | Update system prompt | 5 min |
| 5 | Test | 15 min |
| **Total** | | **~45 min** |

**No new dependencies.** All RAG infrastructure (chunker, embed, vectorstore, Search) already exists.
