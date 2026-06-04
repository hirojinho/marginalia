---
id: 2026-06-05-remove-plan-drawer
title: Remove the right-side plan drawer — left rail is the single plan view
max_wall_clock_minutes: 20
max_diff_lines: 450
max_retries: 1
max_tokens: 30000
model: deepseek-v4-flash
thinking: off
requires_visual_approval: false
allow_web_search: false
---

## Goal

Delete the right-side plan drawer (`#drawer`, `#drawer-overlay`, and the `Plan`
header button). The left rail (`#session-sidebar`, powered by `rail.js`)
already provides the same plan view — course selection, task listing with
done toggles, PDF links — making the drawer fully redundant. Removing it
simplifies the UI to a single plan surface and eliminates 231 lines of dead
code.

## Implementation plan

### Step 1 — Remove HTML elements from `static/index.html`

Remove three things:
- The `#plan-btn` button in the header: `<button id="plan-btn" class="header-btn">Plan</button>`
- The `#drawer-overlay` div: `<div id="drawer-overlay"></div>`
- The `#drawer` div and all its children (lines from `<div id="drawer">` through its closing `</div>`)

### Step 2 — Remove plan.js imports and delegation from `static/app.js`

**2a.** Remove the import line:
```js
import { initPlan, openFullPlan, toggleTopic, openPdfFromDrawer } from './plan.js';
```

**2b.** Remove three switch cases from the click delegator:
```js
    case 'open-full-plan':
      openFullPlan(el.dataset.courseId);
      break;
```
```js
    case 'open-pdf-from-drawer':
      openPdfFromDrawer(parseInt(el.dataset.pdfId, 10));
      break;
```
```js
    case 'toggle-topic':
      toggleTopic(el.dataset.courseId, parseInt(el.dataset.idx, 10));
      break;
```

**2c.** Remove the `initPlan()` call near the bottom of the file.

### Step 3 — Delete `static/plan.js`

Delete the entire file. No other module imports from it after Step 2.

### Step 4 — Remove drawer-related CSS from `static/style.css`

Remove three blocks:
1. `/* === DRAWER === */` comment through the `#drawer-body code` rule (the
   entire `#drawer-overlay`, `#drawer`, `#drawer-header`, `#drawer-close`,
   `#drawer-body` section — the fixed-position right-side panel).
2. `.course-card` through `.empty-plan` (the course card, topic row, checkbox,
   and empty-state styles used only by the drawer's plan rendering).
3. `/* === PDFs section in drawer === */` through `.drawer-pdf-item .dpf-progress`
   (the drawer's PDF-item styles).

None of these classes appear in `rail.js` or any other JS module — they are
exclusive to `plan.js`.

## Verification recipe

### Pre-baseline (must FAIL on current main)

```bash
# 1. plan.js still exists.
test -f static/plan.js \
  && echo "PASS: pre-baseline 1 — plan.js exists" \
  || echo "FAIL: pre-baseline 1 — already deleted"

# 2. plan.js is imported in app.js.
grep -q "from './plan.js'" static/app.js \
  && echo "PASS: pre-baseline 2 — app.js imports plan.js" \
  || echo "FAIL: pre-baseline 2 — import already removed"

# 3. Drawer HTML elements still exist.
grep -q 'id="drawer"' static/index.html \
  && echo "PASS: pre-baseline 3 — drawer div exists" \
  || echo "FAIL: pre-baseline 3 — already removed"

# 4. Drawer CSS still exists.
grep -q '#drawer {' static/style.css \
  && echo "PASS: pre-baseline 4 — drawer CSS exists" \
  || echo "FAIL: pre-baseline 4 — already removed"
```

### Post-acceptance (must PASS after implementation)

```bash
# 1. plan.js is gone.
! test -f static/plan.js \
  && echo "PASS: post-1 — plan.js deleted" \
  || echo "FAIL: post-1 — plan.js still exists"

# 2. No plan.js import in app.js.
! grep -q "from './plan.js'" static/app.js \
  && echo "PASS: post-2 — no plan.js import" \
  || echo "FAIL: post-2 — import still present"

# 3. No drawer elements in index.html.
! grep -q 'id="drawer"' static/index.html \
  && echo "PASS: post-3 — drawer div removed" \
  || echo "FAIL: post-3 — drawer still in HTML"

# 4. No drawer CSS rules.
! grep -q '#drawer {' static/style.css \
  && echo "PASS: post-4 — drawer CSS removed" \
  || echo "FAIL: post-4 — drawer CSS still present"

# 5. No drawer-pdf-item CSS.
! grep -q 'drawer-pdf-item' static/style.css \
  && echo "PASS: post-5 — drawer-pdf CSS removed" \
  || echo "FAIL: post-5 — drawer-pdf CSS still present"

# 6. Go build passes (static files embedded).
go build ./...

# 7. Go vet passes.
go vet ./...

# 8. No regressions in tests.
go test ./...
```

### Human-eyeball notes

- The `Plan` button is gone from the header — the left sidebar (toggled by ☰)
  is now the only plan surface.
- The left rail's course selector and plan rendering should be unchanged.
- No layout shift: the drawer was fixed-position and didn't affect the main
  content flow.
- No remaining `plan.js` references in Go source or any other JS module.

## Done criteria

- [ ] `static/plan.js` deleted.
- [ ] `static/index.html`: `#plan-btn`, `#drawer-overlay`, `#drawer` removed.
- [ ] `static/app.js`: plan.js import, `open-full-plan`/`open-pdf-from-drawer`/
      `toggle-topic` cases, `initPlan()` call removed.
- [ ] `static/style.css`: `#drawer-*`, `.course-card`–`.empty-plan`,
      `.drawer-pdf-*` blocks removed.
- [ ] `go build ./...` passes.
- [ ] `go vet ./...` passes.
- [ ] `go test ./...` passes.
- [ ] Pre-baseline fails on current main; post-acceptance passes on the branch.

## Rollback notes

Revert the commit. No schema, migration, or data impact — all changes are
static frontend files only. The deleted `plan.js` can be restored from git
history.
