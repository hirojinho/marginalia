---
id: 2026-06-02-fix-tool-panel-wipe
title: Fix tool panel visual wipe when streaming resumes after bash
max_wall_clock_minutes: 20
max_diff_lines: 40
max_retries: 1
max_tokens: 40000
model: deepseek-v4-flash
thinking: off
requires_visual_approval: false
allow_web_search: false
---

## Goal

Fix a rendering bug in `static/chat.js`: when the agent uses a bash tool during
streaming, the tool panel appears correctly, but when `token` events resume, the
bash output disappears and the text takes its place with no spacing. The root
cause is `currentSegmentEl.innerHTML = renderMarkdown(rawAnswer)` on every token
event — it replaces the entire `.answer-segment` contents, destroying any tool
panel `<details>` elements that were appended as children.

## Implementation plan

### Step 1 — Add `currentTextEl` tracking variable

In the `initChat` function, after `let currentSegmentEl = null;`, add:

```js
let currentTextEl = null;  // inner .answer-text — survives tool panels
```

### Step 2 — Update `ensureAnswerSegment()` to create inner `.answer-text`

Replace the existing `ensureAnswerSegment()` body. Currently it creates only
`.answer-segment` and sets `currentSegmentEl`. Change it to also create an inner
`.answer-text` div and set `currentTextEl`:

```js
function ensureAnswerSegment() {
  if (currentSegmentType !== 'answer') {
    const seg = document.createElement('div');
    seg.className = 'answer-segment';
    const text = document.createElement('div');
    text.className = 'answer-text';
    seg.appendChild(text);
    currentAssistantMsg.appendChild(seg);
    currentSegmentEl = seg;
    currentTextEl = text;
    currentSegmentType = 'answer';
    rawAnswer = '';
  }
}
```

### Step 3 — Update `token` handler to write into `.answer-text`

Find the `token` event handler block (the `else if (eventType === 'token')`
branch). Two changes:

**3a.** In the segment-creation block (when `currentSegmentType !== 'answer'`),
apply the same inner-container pattern. After creating the `.answer-segment` div
and before appending to `currentAssistantMsg`, add:

```js
const text = document.createElement('div');
text.className = 'answer-text';
seg.appendChild(text);
```

And after setting `currentSegmentEl = seg;`, add `currentTextEl = text;`.

**3b.** Replace:
```js
currentSegmentEl.innerHTML = renderMarkdown(rawAnswer);
```
with:
```js
if (currentTextEl) currentTextEl.innerHTML = renderMarkdown(rawAnswer);
```

### Step 4 — Update `tool_start` handler to insert after `.answer-text`

In the `tool_start` handler, replace:
```js
currentSegmentEl.appendChild(panel.el);
```
with:
```js
if (currentTextEl) {
  currentTextEl.after(panel.el);
} else {
  currentSegmentEl.appendChild(panel.el);
}
```

This places the tool panel as a sibling *after* the `.answer-text` div, so it
survives future `innerHTML` updates to `.answer-text`.

## Verification recipe

### Pre-baseline (must FAIL on current main)

```bash
# The bug exists: token handler writes into currentSegmentEl.innerHTML directly,
# and ensureAnswerSegment does not create an inner .answer-text.
grep -q 'currentSegmentEl.innerHTML = renderMarkdown' static/chat.js \
  && echo "PASS: pre-baseline 1 — token handler still uses currentSegmentEl.innerHTML" \
  || echo "FAIL: pre-baseline 1 — already fixed?"

grep -q 'currentTextEl' static/chat.js \
  && echo "FAIL: pre-baseline 2 — currentTextEl already exists" \
  || echo "PASS: pre-baseline 2 — no currentTextEl yet"
```

### Post-acceptance (must PASS after implementation)

```bash
# 1. currentTextEl declared.
grep -q 'let currentTextEl' static/chat.js \
  && echo "PASS: currentTextEl declared" \
  || echo "FAIL: currentTextEl not found"

# 2. ensureAnswerSegment creates .answer-text.
grep -q "answer-text" static/chat.js \
  && echo "PASS: answer-text class referenced" \
  || echo "FAIL: answer-text not found"

# 3. token handler writes to currentTextEl, not currentSegmentEl.
grep -q 'currentTextEl.innerHTML = renderMarkdown' static/chat.js \
  && echo "PASS: token uses currentTextEl" \
  || echo "FAIL: token still uses currentSegmentEl"

# 4. No more dangerous currentSegmentEl.innerHTML for markdown.
! grep -q 'currentSegmentEl.innerHTML = renderMarkdown' static/chat.js \
  && echo "PASS: no currentSegmentEl.innerHTML for markdown" \
  || echo "FAIL: dangerous pattern still present"

# 5. tool_start uses .after() for safe insertion.
grep -q 'currentTextEl.after' static/chat.js \
  && echo "PASS: tool_start uses .after()" \
  || echo "FAIL: tool_start uses .appendChild"
```

### Human-eyeball notes

- Open a session with an active agent. Send a message that triggers a bash tool
  mid-stream (e.g., "list the files in the current directory"). Observe:
  - Bash output appears as a collapsible panel.
  - When text streaming resumes, the bash panel stays visible.
  - Text renders below/around the panel with normal spacing.
- No visual regressions: thinking blocks, skill chips, compaction notices still
  render correctly.

## Done criteria

- [ ] `currentTextEl` tracking variable added next to `currentSegmentEl`.
- [ ] `ensureAnswerSegment()` creates both `.answer-segment` and inner `.answer-text`.
- [ ] `token` handler segment-creation creates `.answer-text` too.
- [ ] `token` handler writes to `currentTextEl.innerHTML`, not `currentSegmentEl.innerHTML`.
- [ ] `tool_start` handler inserts panels after `currentTextEl` (`.after()`).
- [ ] Pre-baseline fails on current main; post-acceptance passes on the branch.
