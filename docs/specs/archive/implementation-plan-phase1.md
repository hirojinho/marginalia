# Study Agent — Implementation Plan (Phase 1)

## Objective

Add `pdf_extract`, `web_fetch`, and `study_skill` tools to the study agent, build the initial corpus, and wire skills into the chat flow. No architecture changes — just adding tools to the existing modular agent package.

---

## Step 1: Add `pdf_extract` tool

**Files to change:**
- `agent/tools.go` — add tool definition + execution
- `go.mod` — add PDF text extraction dependency

**Implementation:**

1. Add dependency: `github.com/ledongthuc/pdf` (pure Go, no CGO, Apache 2.0)
   ```
   go get github.com/ledongthuc/pdf
   ```

2. Add to `GetTools()`:
   ```go
   {
       Type: "function",
       Function: ToolFunc{
           Name:        "pdf_extract",
           Description: "Extract text content from an uploaded PDF",
           Parameters: map[string]interface{}{
               "type": "object",
               "properties": map[string]interface{}{
                   "pdf_id": map[string]interface{}{
                       "type":        "integer",
                       "description": "Database ID of the PDF (from /pdf/list)",
                   },
                   "pages": map[string]interface{}{
                       "type":        "string",
                       "description": "Optional page range, e.g. '1-5' or '1,3,7'. Default: all pages",
                   },
               },
               "required": []string{"pdf_id"},
           },
       },
   }
   ```

3. Add execution in `ExecuteTool`:
   ```go
   case "pdf_extract":
       var p struct {
           PdfID int64  `json:"pdf_id"`
           Pages string `json:"pages"` // e.g., "1-5" or "1,3,7"
       }
       json.Unmarshal(args, &p)
       // Look up PDF path from DB
       // Read and extract text using github.com/ledongthuc/pdf
       // Cache extracted text to data/pdf-texts/{id}.txt
       // Return extracted text as string
   ```

4. Create `data/pdf-texts/` directory on startup (in `main.go`)

5. Cache strategy:
   - First extraction: read PDF, extract text, save to `data/pdf-texts/{id}.txt`
   - Subsequent calls: read from cache if PDF hasn't changed
   - Cache invalidation: compare `uploaded_at` <-> cache file mtime

6. Page range parsing:
   - Parse `"1-5"` as pages 1-5
   - Parse `"1,3,7"` as pages 1, 3, 7
   - Empty string = all pages
   - Cap at 50 pages max per extraction

**Test:**
- Upload a PDF, then ask the agent "What does page 3 say about photosynthesis?"
- Verify the agent calls `pdf_extract` and returns relevant content
- Verify cached extraction is fast on second call

---

## Step 2: Add `web_fetch` tool

**Files to change:**
- `agent/tools.go` — add tool definition + execution
- `go.mod` — add HTML-to-markdown dependency

**Implementation:**

1. Add dependency: `github.com/JohannesKaufmann/html-to-markdown`
   ```
   go get github.com/JohannesKaufmann/html-to-markdown
   ```

2. Add to `GetTools()`:
   ```go
   {
       Type: "function",
       Function: ToolFunc{
           Name:        "web_fetch",
           Description: "Fetch a web page and convert it to readable markdown",
           Parameters: map[string]interface{}{
               "type": "object",
               "properties": map[string]interface{}{
                   "url": map[string]interface{}{
                       "type":        "string",
                       "description": "URL to fetch",
                   },
               },
               "required": []string{"url"},
           },
       },
   }
   ```

3. Add execution in `ExecuteTool`:
   ```go
   case "web_fetch":
       var p struct { URL string `json:"url"` }
       json.Unmarshal(args, &p)
       // Validate URL scheme (http/https only)
       // GET with 30s timeout, User-Agent: StudyAgent/1.0
       // Convert HTML to markdown using html-to-markdown
       // Truncate at 50,000 chars
       // Return: title + content
   ```

4. Rate limiter:
   - Add a simple time-based rate limiter (max 5 requests per minute)
   - Store last 5 call timestamps in a slice with mutex
   - Return "rate limited, try again in N seconds" if exceeded

5. URL validation:
   - Only allow `http://` and `https://` schemes
   - Block private IPs (127.0.0.0/8, 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16)
   - Max redirect depth: 5

**Test:**
- Ask the agent "What is cellular respiration? Look it up on Wikipedia"
- Verify the agent calls `web_fetch` and returns parsed content
- Verify rate limiting works (6 rapid calls = rejection)

---

## Step 3: Add `study_skill` tool

**Files to create:**
- `agent/skills.go` — skill registry + prompt templates
- `corpus/study-methods/` — initial corpus files (see Step 5)

**Files to change:**
- `agent/tools.go` — add tool definition + execution

**Implementation:**

1. Create `agent/skills.go` with a skill registry:

   ```go
   type Skill struct {
       Name        string
       Description string
       Params      []string   // required param names
       Optional    []string   // optional param names
       Generate    func(params map[string]string) string  // returns prompt
   }

   var skills = map[string]Skill{
       "orientation": { ... },
       "study_notes": { ... },
       "self_test":   { ... },
       "review":      { ... },
   }
   ```

2. Each skill's `Generate` function:
   - Reads relevant corpus files (using `os.ReadFile` + `VaultPath`)
   - Reads course interests (using existing path logic)
   - Assembles the full prompt from the template
   - Returns the prompt string

3. The `study_skill` tool:
   - Receives skill name + params
   - Looks up the skill in the registry
   - Validates required params
   - Calls `Generate(params)` to get the prompt
   - Returns the prompt as the tool result
   - The LLM then uses this prompt context to formulate its response

4. Tool definition:
   ```go
   {
       Type: "function",
       Function: ToolFunc{
           Name:        "study_skill",
           Description: "Invoke a study skill to get structured guidance",
           Parameters: map[string]interface{}{
               "type": "object",
               "properties": map[string]interface{}{
                   "skill": map[string]interface{}{
                       "type": "string",
                       "description": "Skill name: orientation, study_notes, self_test, review",
                   },
                   "params": map[string]interface{}{
                       "type": "object",
                       "description": "Skill parameters (topic, course_id, content, etc.)",
                   },
               },
               "required": []string{"skill"},
           },
       },
   }
   ```

**Test:**
- In a Biology session, type "orient me on cellular respiration"
- Verify the agent calls `study_skill` with `skill: "orientation"`, `params: {topic: "cellular respiration", course_id: "biology"}`
- Verify the response contains prerequisites, key concepts, and watch points
- Same for other skills

---

## Step 4: Update system prompt to mention available tools

**Files to change:**
- `agent/agent.go` — update `LoadSystemPrompt()` or add a tool-awareness section

**Implementation:**

Append a tools overview to the base system prompt:

```go
func LoadSystemPrompt() string {
    base := // ... existing logic ...
    
    toolsOverview := `

## Available Tools

You have access to these tools:
- **read_file** — Read any file in the workspace
- **search_files** — Search file contents with regex
- **list_files** — List directory contents
- **save_note** — Save notes to the vault
- **pdf_extract** — Extract text from uploaded PDFs (pass pdf_id from /pdf/list)
- **web_fetch** — Fetch and parse a web page as markdown
- **study_skill** — Invoke a study skill (orientation, study_notes, self_test, review)

When a user asks about a PDF they're viewing, use pdf_extract to read its content.
When a user asks about something not in your local knowledge, use web_fetch.
When a user wants to start studying, review, or test themselves, use study_skill.
`
    return base + toolsOverview
}
```

This makes the LLM aware of what it can do, so it calls the right tools proactively.

---

## Step 5: Build initial corpus

**Files to create:**
- `data/corpus/study-methods/active-recall.md`
- `data/corpus/study-methods/spaced-repetition.md`
- `data/corpus/study-methods/feynman-technique.md`
- `data/corpus/study-methods/error-diagnosis.md`
- `data/corpus/study-methods/orientation.md`
- `data/corpus/study-methods/note-templates.md`
- `data/corpus/meta/how-to-study.md`
- `data/corpus/meta/session-workflow.md`

**Content for each file:**

Write concise, practical reference cards (300-500 words each) covering:

1. **active-recall.md** — What active recall is, why it works, how to apply it in study sessions, practical techniques (self-testing, flashcards, blank page method)

2. **spaced-repetition.md** — The forgetting curve, optimal intervals (1d, 3d, 7d, 14d, 30d), Anki integration tips, when to review vs relearn

3. **feynman-technique.md** — The four steps, when to use it, how it differs from simply re-reading, common pitfalls (jargon crutch, surface explanation)

4. **error-diagnosis.md** — Classifying errors (conceptual gap, calculation mistake, misread question, knowledge boundary), what to do for each type, building an error log

5. **orientation.md** — Pre-reading strategy: scan headings, check prerequisites, set purpose questions, estimate time, identify difficulty level

6. **note-templates.md** — Cornell notes format, two-column method, concept maps, when to use each, how to structure equations vs definitions vs processes

7. **how-to-study.md** — The agent's study philosophy: active over passive, spaced over massed, understanding over memorization, interleaved practice

8. **session-workflow.md** — What happens in a session: orient → study → note → review → plan next

These files are stored under `VAULT_ROOT/data/corpus/` so they're inside the study app's data directory.

**Directory creation:**
- Add `os.MkdirAll` for `data/corpus/study-methods/` and `data/corpus/meta/` in `main.go` startup

---

## Step 6: Wiring and integration testing

**After all tools are implemented:**

1. **Rebuild and restart:**
   ```bash
   cd /workspace/marginalia && go build -o marginalia . && ./marginalia
   ```

2. **Test each tool individually:**
   - `pdf_extract`: Upload → ask "what is this PDF about?" → verify extraction
   - `web_fetch`: Ask "look up X on Wikipedia" → verify fetch + parse
   - `study_skill`: In a Biology session, type "orient me on photosynthesis" → verify skill invocation

3. **Test tool chaining:**
   - "Orient me on photosynthesis, and look up the Wikipedia page for context"
   - Expected: agent calls `study_skill(orientation)` + `web_fetch(wikipedia)` and combines results

4. **Test error handling:**
   - Invalid PDF id → graceful error message
   - Invalid URL → graceful error message
   - Unknown skill name → graceful error message
   - Rate limit exceeded → retry message

5. **Test cache:**
   - Extract same PDF twice → second call should be near-instant
   - Verify `data/pdf-texts/{id}.txt` cache files exist

---

## Dependency Summary

| Package | Purpose | License |
|---------|---------|---------|
| `github.com/ledongthuc/pdf` | PDF text extraction | Apache 2.0 |
| `github.com/JohannesKaufmann/html-to-markdown` | HTML to markdown | MIT |

Both are pure Go, no CGO, and compatible with the existing `modernc.org/sqlite` constraint.

---

## Estimated Effort

| Step | Time | Complexity |
|------|------|-------------|
| Step 1: pdf_extract | 1-2 hours | Medium |
| Step 2: web_fetch | 1 hour | Low |
| Step 3: study_skill | 1-2 hours | Medium |
| Step 4: System prompt update | 15 min | Low |
| Step 5: Build corpus | 1-2 hours | Low (writing) |
| Step 6: Integration testing | 1 hour | Low |
| **Total** | **5-8 hours** | |

---

## Execution Order

1. **pdf_extract** — most impactful tool, enables the agent to actually read uploaded PDFs
2. **web_fetch** — second most impactful, enables research beyond local files
3. **study_skill** — the structured workflow layer, depends on corpus being populated
4. **Corpus** — write the 8 reference files
5. **System prompt update** — quick win
6. **Integration testing** — verify everything works together