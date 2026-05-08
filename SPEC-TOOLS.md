# Study Agent — Tools & Skills Spec

## Overview

This spec defines the tools and skills to be added to the study agent in Phase 1, expanding it from a basic chat interface with 4 file tools into a capable study companion with PDF comprehension, web research, and structured study workflows.

## Current Tools

| Tool | Description |
|------|-------------|
| `read_file` | Read a file from the workspace |
| `search_files` | Ripgrep search over workspace files |
| `list_files` | List directory contents |
| `save_note` | Write a note to the vault |

## New Tools

### 1. `pdf_extract`

**Purpose:** Extract text content from an uploaded PDF so the agent can read, summarize, and answer questions about it.

**Input:**
```json
{
  "pdf_id": 3,                    // DB id of the PDF (from /pdf/list)
  "pages": [1, 5, "3-7"]         // optional: specific pages or ranges (default: all)
}
```

**Output:**
```json
{
  "original_name": "FMEA Handbook.pdf",
  "pages_extracted": 12,
  "total_pages": 45,
  "content": {
    "1": "Title page text...",
    "2": "Chapter 1 introduction...",
    "3-5": "Combined text of pages 3-5..."
  }
}
```

**Implementation notes:**
- Use a Go PDF text extraction library (e.g., `github.com/unidoc/unipdf` or `github.com/ledongthuc/pdf`) — no external binary dependency
- Cache extracted text in `data/pdf-texts/{id}.txt` so repeated queries don't re-extract
- For large PDFs (>50 pages), extract only requested pages to avoid context overflow
- The agent's system prompt should know which PDFs are available in the current session

**When the agent uses it:**
- User asks about content of a PDF they're viewing
- User shares a quote and asks for clarification
- Agent needs to reference course material during a study session

---

### 2. `web_fetch`

**Purpose:** Fetch and parse a web page for research, allowing the agent to look up information beyond the local corpus.

**Input:**
```json
{
  "url": "https://en.wikipedia.org/wiki/Fault_tree_analysis",
  "format": "markdown"              // "markdown" (default) or "text"
}
```

**Output:**
```json
{
  "url": "https://en.wikipedia.org/wiki/Fault_tree_analysis",
  "title": "Fault tree analysis - Wikipedia",
  "content": "# Fault tree analysis\n\nFault tree analysis (FTA) is a top-down...",
  "content_length": 4523
}
```

**Implementation notes:**
- Use Go's `net/http` client with a 30s timeout
- Parse HTML to markdown using a Go library (e.g., `github.com/JohannesKaufmann/html-to-markdown`)
- Truncate content at 50,000 characters to avoid context overflow
- Add a `User-Agent: StudyAgent/1.0` header
- Rate limit: max 5 requests per minute (prevent abuse)

**When the agent uses it:**
- User asks about something not in the local corpus
- Agent needs to look up a definition, paper, or concept
- User shares a URL and asks for summary/analysis

---

### 3. `study_skill`

**Purpose:** Invoke a named study skill — a structured prompt chain that orchestrates a multi-step study workflow.

**Input:**
```json
{
  "skill": "orientation",          // skill name
  "params": {
    "topic": "Fault tree analysis",
    "course_id": "ce297"
  }
}
```

**Output:**
```json
{
  "skill": "orientation",
  "result": "## Orientation: Fault Tree Analysis\n\n### Prerequisites\n- Boolean algebra basics\n- Understanding of FMEA...\n\n### Key Concepts\n1. Top-down deductive approach\n2. Boolean logic gates (AND, OR)\n..."
}
```

**Implementation notes:**
- Each skill is a Go function that takes params and returns a string
- Skills have access to the full tool suite (they can call other tools internally)
- The returned string gets injected into the conversation as a tool result
- Skills are registered by name in a map

**Available skills (Phase 1):**

| Skill | Description | Required Params |
|-------|-------------|-----------------|
| `orientation` | Pre-reading primer before studying a topic | `topic`, `course_id` |
| `study_notes` | Generate structured notes from a reading session | `topic`, `course_id`, `content` (optional) |
| `self_test` | Generate practice questions on a topic | `topic`, `course_id`, `count` (default: 5) |
| `review` | Assess understanding and suggest next steps | `topic`, `course_id` |

---

## Skills Detail

### `orientation`

**What it does:** Given a topic and course, produces a pre-reading guide with prerequisites, key concepts to watch for, common pitfalls, and suggested study approach.

**Prompt template:**
```
You are a study orientation assistant. The student is about to study "{topic}" 
for the course "{course_name}" (ID: {course_id}).

Using the course context below, produce a concise orientation guide:

1. **Prerequisites** — What should they already know?
2. **Key concepts** — 3-5 core ideas to focus on
3. **Watch points** — Common misconceptions or tricky parts
4. **Study approach** — Suggested method (read-then-solve, examples-first, etc.)
5. **Questions to ask while reading** — 3-5 questions to keep in mind

{course_interests}

Keep it practical and specific. No generic advice.
```

**Data sources:** Course interests file, fleeting notes (if CE-297), study plan progress.

---

### `study_notes`

**What it does:** Takes raw content (from a PDF, web page, or user summary) and produces structured study notes.

**Prompt template:**
```
You are a study note-taking assistant. The student has finished reading about "{topic}" 
for the course "{course_name}".

{content_or_pdf_excerpt}

Produce structured notes in this format:

## {Topic}

### Summary (2-3 sentences)

### Key Concepts
- Concept 1: brief explanation
- Concept 2: brief explanation

### Formulas / Definitions (if applicable)
- Formula/definition with context

### Connections to Other Topics
- How this relates to X and Y

### Questions for Review
1. Question that tests understanding
2. Another question

Keep notes concise and exam-focused.
```

---

### `self_test`

**What it does:** Generates practice questions on a topic, then evaluates the student's answers.

**Prompt template:**
```
You are a practice exam generator for "{course_name}". 
Generate {count} exam-style questions about "{topic}".

Mix these question types:
- Conceptual understanding
- Calculation/application
- Compare and contrast
- Identify the error

For each question, provide:
1. The question
2. A hint (in parentheses)
3. The expected answer (hidden until student responds)

Format as a numbered quiz. Do NOT reveal answers until the student attempts them.
```

---

### `review`

**What it does:** Assess understanding of a topic and suggest what to focus on next.

**Prompt template:**
```
You are a spaced repetition review assistant. The student wants to review "{topic}" 
for the course "{course_name}".

Based on the course materials:
{course_interests}

Ask 2-3 quick recall questions. Based on how well the student answers:
- If strong: suggest the next topic and mark this for later review
- If shaky: provide a focused refresher on weak areas
- If new: recommend starting with the orientation skill

Keep it conversational. One question at a time.
```

---

## Corpus Structure

When a skill runs, it injects relevant corpus content into the prompt:

```
corpus/
├── study-methods/
│   ├── active-recall.md
│   ├── spaced-repetition.md
│   ├── feynman-technique.md
│   ├── error-diagnosis.md
│   ├── orientation.md
│   └── note-templates.md
├── courses/
│   ├── ce297/
│   │   ├── concepts/          # Key concept cards
│   │   │   ├── fault-tree.md
│   │   │   ├── fmea.md
│   │   │   └── bow-tie.md
│   │   ├── patterns/          # Problem-solving patterns
│   │   │   └── risk-assessment.md
│   │   └── pitfalls/          # Common errors
│   │       └── confusion-matrix.md
│   ├── ddia/
│   │   ├── concepts/
│   │   └── patterns/
│   └── ...
└── meta/
    ├── how-to-study.md
    └── session-workflow.md
```

**Phase 1 corpus goal:** Populate `study-methods/` (universal, not course-specific) and `meta/`. Course-specific concept cards come in Phase 2.

---

## System Prompt Injection

When a session is active, the system prompt is assembled from:

1. **Base prompt** — `CLAUDE.local.md` + `study-context.md` (already done)
2. **Course context** — interests file + fleeting notes for the session's course (already done)
3. **Available tools** — tool definitions in the API call (already done)
4. **Corpus context** (NEW) — relevant corpus files read and injected based on topic keywords

The agent will use `search_files` to find relevant corpus content, or the `study_skill` tool will inject it directly.

---

## API Changes

### New endpoint: `GET /api/models`

Returns available models (for frontend model selector in the future):

```json
{
  "current": "thudm/glm-4.1:beta",
  "models": [
    { "id": "thudm/glm-4.1:beta", "name": "GLM-4.1" },
    { "id": "deepseek/deepseek-v4-flash", "name": "DeepSeek V4 Flash" },
    { "id": "anthropic/claude-sonnet-4", "name": "Claude Sonnet" }
  ]
}
```

### Updated `POST /chat`

Now accepts optional `model` form field to override the default model for that request.

### New tool responses

All three new tools return their results as tool results in the SSE stream, same as existing tools. No frontend changes needed for tool execution — the streaming protocol handles it.

---

## Frontend Changes (Minimal)

The frontend needs only one addition for Phase 1:

1. **Model selector in settings** — A small dropdown in the header to switch models mid-session (optional, can defer)

The `pdf_extract`, `web_fetch`, and `study_skill` tools are called by the agent automatically. The user just chats normally. The agent decides when to use them.