# Phase 2.1 — Auto-Extract PDF Text & Session Summaries

## Spec

### 1. PDF Auto-Extraction on Upload

**What:** When a PDF is uploaded, immediately extract its full text and cache it to `data/pdf-texts/{id}.txt`. This makes the PDF content instantly available to the agent without requiring an explicit `pdf_extract` tool call.

**Why:** Currently the agent can only read PDF content if:
1. The user asks about it AND
2. The agent decides to call `pdf_extract`

This means the agent starts every PDF session blind — it doesn't know what the PDF contains until the user asks. Auto-extraction means:
- The system prompt can include a brief summary of the active PDF
- The agent can proactively reference PDF content
- `pdf_extract` still works for page-specific queries, but starts from cache (instant)

**Behavior:**
1. User uploads PDF → server saves file to `data/pdf-files/{id}.pdf`
2. Server immediately extracts text in a background goroutine → saves to `data/pdf-texts/{id}.txt`
3. If extraction fails (encrypted, image-only, corrupted), log a warning and skip. The file is still available for manual `pdf_extract` later.
4. The session's system prompt appends a brief excerpt (first ~2000 chars) of the active PDF when one is open

**New endpoint (optional, for frontend):**
```
GET /pdf/extracted/{id} — returns cached extracted text for a PDF
```

**Changes to existing:**
- `handlePDFUpload` — spawn a goroutine to extract text after saving
- `handlePDFProgress(PUT)` — when a PDF becomes active, also trigger extraction if cache is missing
- `GetSessionSystemPrompt` — if session has a `last_pdf_id`, append first 2000 chars of extracted text as context
- Frontend: no changes needed (transparent to user)

### 2. Session Summary on Close

**What:** When a session hasn't had activity for 30+ minutes and the user sends a new message, the agent automatically generates a brief summary of the previous conversation and prepends it as context. This makes resuming sessions seamless.

**Why:** Sessions accumulate long chat histories. When you come back the next day, the agent has to process the entire history to understand context. A summary at the top lets it quickly re-orient.

**Behavior:**
- After 20 messages in a session, the agent automatically generates a summary when the next message is sent
- The summary is stored in the `sessions` table as a new `summary` column
- On subsequent sessions, the summary is injected into the system prompt instead of the full history (old messages are still in DB but not sent to the LLM after the summary)
- The summary is regenerated every 20 messages

**Implementation approach:**
- Add `summary TEXT` and `summary_at INTEGER` columns to `sessions` table
- After saving a new user message, check if `count(messages where session_id = X) > summary_at + 20`
- If so, call the LLM with a special "summarize this conversation" prompt
- Store the summary in `sessions.summary` and update `summary_at`
- When loading session history, if a summary exists, send: system prompt + summary + recent messages (last 10)

---

## Implementation Plan

### Step 1: Add `summary` columns to sessions table

**File:** `agent/db.go`

1. Add to the schema in `InitSessionDB`:
   ```sql
   -- Migration: add summary columns
   ALTER TABLE sessions ADD COLUMN summary TEXT DEFAULT '';
   ALTER TABLE sessions ADD COLUMN summary_at INTEGER DEFAULT 0;
   ```
   
   Note: SQLite doesn't support `ALTER TABLE ... ADD COLUMN IF NOT EXISTS`. Handle this gracefully — try the ALTER TABLE and ignore "duplicate column" errors.

2. Add `Summary` and `SummaryAt` fields to the `Session` struct in `agent/types.go`:
   ```go
   type Session struct {
       ...
       Summary   string `json:"summary"`
       SummaryAt int    `json:"summary_at"`
   }
   ```

3. Update all `Scan` calls for sessions to include the new columns.

**Estimated time:** 15 min

---

### Step 2: Add auto-extraction after PDF upload

**File:** `main.go` — `handlePDFUpload`

1. After the existing upload logic (saving the file, inserting into DB, returning the response), add:
   ```go
   pdfID := id  // capture for goroutine
   go func() {
       pdfPath := filepath.Join(agent.VaultRoot, "data", "pdf-files", fmt.Sprintf("%d.pdf", pdfID))
       extractAndCachePDFText(pdfID, pdfPath)
   }()
   ```

2. Create `extractAndCachePDFText(id int64, pdfPath string)` function in `agent/tools.go`:
   ```go
   func ExtractAndCachePDFText(id int64, pdfPath string) {
       cacheDir := filepath.Join(VaultRoot, "data", "pdf-texts")
       os.MkdirAll(cacheDir, 0755)
       cachePath := filepath.Join(cacheDir, fmt.Sprintf("%d.txt", id))
       
       // Skip if already cached
       if _, err := os.Stat(cachePath); err == nil {
           return
       }
       
       f, r, err := pdf.Open(pdfPath)
       if err != nil {
           log.Printf("PDF auto-extract failed for id %d: %v", id, err)
           return
       }
       defer f.Close()
       
       var pageTexts []string
       for i := 1; i <= r.NumPage(); i++ {
           text, err := r.Page(i).GetPlainText(nil)
           if err != nil {
               pageTexts = append(pageTexts, "[error extracting page " + fmt.Sprintf("%d", i) + "]")
           } else {
               pageTexts = append(pageTexts, text)
           }
       }
       
       cached := strings.Join(pageTexts, "\n---PAGE BREAK---\n")
       os.WriteFile(cachePath, []byte(cached), 0644)
       log.Printf("PDF auto-extracted id %d (%d pages)", id, r.NumPage())
   }
   ```

3. Also trigger extraction in `handlePDFProgress` (PUT) when `last_pdf_id` changes. In `main.go`, after updating progress, check if cache exists and extract if not:
   ```go
   // After existing progress update
   cachePath := filepath.Join(agent.VaultRoot, "data", "pdf-texts", fmt.Sprintf("%d.txt", id))
   if _, err := os.Stat(cachePath); os.IsNotExist(err) {
       pdfPath := filepath.Join(agent.VaultRoot, "data", "pdf-files", fmt.Sprintf("%d.pdf", id))
       go agent.ExtractAndCachePDFText(id, pdfPath)
   }
   ```

**Estimated time:** 30 min

---

### Step 3: Inject PDF context into session system prompt

**File:** `agent/tools.go` — `GetSessionSystemPrompt`

1. After building the course-specific context, check if the session has a `last_pdf_id`:
   ```go
   var lastPdfID int64
   DB.QueryRow("SELECT last_pdf_id FROM sessions WHERE id = ?", sessionID).Scan(&lastPdfID)
   if lastPdfID > 0 {
       cachePath := filepath.Join(VaultRoot, "data", "pdf-texts", fmt.Sprintf("%d.txt", lastPdfID))
       if data, err := os.ReadFile(cachePath); err == nil {
           text := string(data)
           // Truncate to first 2000 chars for the system prompt
           if len(text) > 2000 {
               text = text[:2000] + "\n...[truncated, use pdf_extract for full content]"
           }
           var pdfName string
           DB.QueryRow("SELECT original_name FROM pdfs WHERE id = ?", lastPdfID).Scan(&pdfName)
           extra += fmt.Sprintf("\n\n---\n\nCurrent PDF: **%s**\n\nExcerpt:\n%s", pdfName, text)
       }
   }
   ```

2. This means every message in a session with an active PDF automatically includes a preview of the PDF content. The agent can then reference it without an explicit tool call, and still use `pdf_extract` for page-specific queries.

**Estimated time:** 15 min

---

### Step 4: Session summary generation

**File:** `agent/db.go` + `agent/llm.go` + `main.go`

1. **Add `GetMessageCount` function in `agent/db.go`:**
   ```go
   func GetMessageCount(sessionID int64) int {
       var count int
       DB.QueryRow("SELECT COUNT(*) FROM messages WHERE session_id = ?", sessionID).Scan(&count)
       return count
   }
   ```

2. **Add `GenerateSummary` function in `agent/llm.go`:**
   ```go
   func GenerateSummary(history []Message, apiKey, apiURL, model string) (string, error) {
       prompt := `Summarize this study session conversation in 3-5 concise bullet points. Focus on:
   - What topics were discussed
   - Key concepts or insights learned
   - Any decisions or next steps mentioned
   - Questions still open
   
   Be specific and concise. Do not include pleasantries.`
       
       // Build messages: system prompt + last 30 messages (or all if fewer)
       msgs := []Message{{Role: "system", Content: prompt}}
       start := 0
       if len(history) > 30 {
           start = len(history) - 30
       }
       for _, m := range history[start:] {
           msgs = append(msgs, Message{Role: m.Role, Content: m.Content})
       }
       
       // Non-streaming call
       resp, err := CallLLM(msgs, nil, prompt, model, apiKey, apiURL)
       if err != nil {
           return "", err
       }
       defer resp.Body.Close()
       
       body, err := io.ReadAll(resp.Body)
       if err != nil {
           return "", err
       }
       
       var result struct {
           Choices []struct {
               Message struct {
                   Content string `json:"content"`
               } `json:"message"`
           } `json:"choices"`
       }
       if err := json.Unmarshal(body, &result); err != nil {
           return "", err
       }
       if len(result.Choices) == 0 {
           return "", fmt.Errorf("no response from LLM")
       }
       return result.Choices[0].Message.Content, nil
   }
   ```
   
   Note: This needs a non-streaming variant of `CallLLM`. We'll add a `stream: false` path.

3. **Modify `CallLLM` to support non-streaming:**
   Add a `stream` parameter (or a separate `CallLLMNonStreaming` function). The simplest approach is to add `CallLLMOnce` that sets `"stream": false` and reads the full response.

4. **Modify `handleChat` in `main.go` to trigger summarization:**
   After saving the assistant message, check if summarization is needed:
   ```go
   // After saving assistant message
   msgCount := agent.GetMessageCount(sessionID)
   var summaryAt int
   agent.DB.QueryRow("SELECT summary_at FROM sessions WHERE id = ?", sessionID).Scan(&summaryAt)
   if msgCount > summaryAt + 20 && msgCount > 10 {
       // Generate summary asynchronously
       go generateSessionSummary(sessionID)
   }
   ```

5. **Modify session history loading to use summaries:**
   In `GetSessionHistory`, if a summary exists, prepend it and only send the last 10 messages:
   ```go
   func GetSessionHistoryWithSummary(sessionID int64) []Message {
       var summary string
       var summaryAt int
       DB.QueryRow("SELECT summary, summary_at FROM sessions WHERE id = ?", sessionID).Scan(&summary, &summaryAt)
       
       allMessages := GetSessionHistory(sessionID)
       
       if summary != "" && len(allMessages) > summaryAt + 10 {
           // Send: summary message + last 10 messages
           summaryMsg := Message{
               Role:    "system",
               Content: "Previous conversation summary:\n" + summary,
           }
           recentMessages := allMessages
           if len(allMessages) > 10 {
               recentMessages = allMessages[len(allMessages)-10:]
           }
           result := []Message{summaryMsg}
           result = append(result, recentMessages...)
           return result
       }
       
       return allMessages
   }
   ```

6. **Update `handleChat` to use `GetSessionHistoryWithSummary` instead of `GetSessionHistory`.**

**Estimated time:** 1.5 hrs

---

### Step 5: Add `/pdf/extracted/{id}` endpoint (optional)

**File:** `main.go`

```go
func handlePDFExtracted(w http.ResponseWriter, r *http.Request) {
    if r.Method != "GET" {
        http.Error(w, "method not allowed", 405)
        return
    }
    parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/pdf/extracted/"), "/")
    idStr := parts[0]
    var id int
    if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
        http.Error(w, "invalid id", 400)
        return
    }
    cachePath := filepath.Join(agent.VaultRoot, "data", "pdf-texts", fmt.Sprintf("%d.txt", id))
    data, err := os.ReadFile(cachePath)
    if err != nil {
        http.Error(w, "not extracted yet", 404)
        return
    }
    w.Header().Set("Content-Type", "text/plain; charset=utf-8")
    w.Write(data)
}
```

Add to `main()`: `http.HandleFunc("/pdf/extracted/", handlePDFExtracted)`

**Estimated time:** 10 min

---

### Step 6: Database migration

**File:** `agent/db.go` — `InitSessionDB`

Add safe migration after the existing schema:
```go
// Migrations — add columns if they don't exist
migrations := []string{
    "ALTER TABLE sessions ADD COLUMN summary TEXT DEFAULT ''",
    "ALTER TABLE sessions ADD COLUMN summary_at INTEGER DEFAULT 0",
}
for _, m := range migrations {
    // Ignore "duplicate column" errors — column already exists
    if _, err := db.Exec(m); err != nil && !strings.Contains(err.Error(), "duplicate column") {
        log.Printf("Migration warning: %v", err)
    }
}
```

**Estimated time:** 10 min

---

### Step 7: Test end-to-end

1. **Upload a PDF** → verify `data/pdf-texts/{id}.txt` is created automatically
2. **Open a session with a PDF** → verify system prompt includes PDF excerpt
3. **Send 25+ messages** → verify summary is generated (check `sessions.summary` column)
4. **Reopen the session** → verify summary is used instead of full history
5. **Call `pdf_extract` explicitly** → verify it uses the cached text (fast)
6. **GET `/pdf/extracted/1`** → verify it returns cached text

**Estimated time:** 30 min

---

## Summary

| Step | What | Time |
|------|------|------|
| 1 | DB schema migration (summary columns) | 15 min |
| 2 | Auto-extract PDF text on upload | 30 min |
| 3 | Inject PDF context into session prompt | 15 min |
| 4 | Session summary generation + loading | 1.5 hrs |
| 5 | `/pdf/extracted/` endpoint | 10 min |
| 6 | Database migration logic | 10 min |
| 7 | End-to-end testing | 30 min |
| **Total** | | **~3 hrs** |

**Priority order:** Step 2 → Step 3 → Step 1+6 → Step 4 → Step 5 → Step 7

Steps 2+3 (PDF auto-extract + context injection) give the biggest immediate impact — the agent becomes PDF-aware without any user action. Steps 1+4+6 (summaries) improve long-session UX.