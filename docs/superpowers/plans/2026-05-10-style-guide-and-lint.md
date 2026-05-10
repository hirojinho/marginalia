# Code Style Guide + Lint Enforcement — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task.

**Goal:** Establish a project-wide code style for `claw-study` (Go + JS) with hard lint enforcement at pre-commit. Eduardo's locked-in preferences from a 2026-05-10 grill-me session are the source of truth.

**Architecture:** Single root `STYLE.md` carries the philosophy + 11 locked rules + embrace/ban list. Two language-specific files (`docs/style/{go,js}.md`) carry concrete examples. `CLAUDE.md` at root points agents at `STYLE.md`. `golangci-lint` + `prettier` + `eslint` enforce the *mechanically-checkable* subset of rules. A pre-commit hook (installed via tracked script) blocks commits that fail format or lint. Conceptual rules that no linter can verify (verb-noun error wraps, behavior-test naming, file-header invariants) live in prose only and are enforced by reviewer attention.

**Tech Stack:** Go 1.24+, vanilla JS, `golangci-lint` (Homebrew), `prettier` + `eslint` (npm devDeps via `package.json`).

---

## Locked decisions (verbatim from grill-me 2026-05-10)

1. **Scope:** Go + JS. `STYLE.md` (root) + `docs/style/go.md` + `docs/style/js.md`. `CLAUDE.md` (root) points agents at `STYLE.md`.
2. **Philosophy:** Idiomatic Go + hard type safety. One operation per line; no chained calls. Use `type CourseID string`-style typed identifiers for domain primitives. Avoid `any`/`interface{}` outside protocol boundaries (JSON, SQL `Scan`).
3. **Naming (hybrid):** Domain values descriptive (`store`, `memory`, `recentSessions`, `courseID`); plumbing short (`db`, `ctx`, `r` for `*http.Request`, `t *testing.T`); receivers 1–3 chars; loop indices `i` only in tight numeric loops, otherwise domain-meaningful.
4. **Comments (layered):** File-header paragraph stating responsibility + key invariant + non-obvious dependency on every non-trivial file. Doc comments on exported types/funcs in standard Go style. Inline comments only when answering "why", never restating "what". Code self-documents.
5. **Errors:** Wrap every fallible operation with `fmt.Errorf("verb noun: %w", err)`. One log site per request (HTTP handler / CLI `main`); intermediate layers never both log and return. Sentinel errors only when callers branch on them (YAGNI).
6. **Decomposition:** Smallest unit that names a concept. Struct only when ≥2 methods share non-trivial state. Method when receiver represents a meaningful subject (`store.Save(memory)`); free function when no natural subject (`AssembleAgentsMD(...)`). File splits when its header sentence needs an "and". Function splits when body has > ~3 distinct phases.
7. **Tests:** Behavior-first, real dependencies (`:memory:` SQLite, `t.TempDir()`, no mocks for things we own). Test names as English sentences (`TestSubjectVerbCondition`). Table tests only when N cases share assertions. Helper constructors at top of file (`newMemoryDB(t)`). Stdlib `testing` only — no assert libraries. No `t.Run` unless subtest isolation/parallelism is needed.
8. **JS structure:** ES modules (`<script type="module">`). File-per-concern split (`api.js`, `sessions.js`, `chat.js`, `plan.js`, `pdf.js`).
9. **JS objects:** Closure factories (`createChatStream()` returns `{ start, stop }`) + module-scope `let` for genuine singletons. **No classes.** **No prototypal inheritance.** Async always via `async/await`, never `.then()` chains. `const` by default; `let` only when reassignment is genuinely needed; **no `var`**.
10. **Enforcement:** Hard lint blocking pre-commit. Every rule that can be lint-enforced *will* be. Conceptual rules without lint coverage live in prose only.
11. **Embrace / ban list:**
    - **Embrace:** typed identifiers (`type X string`), behavior-named small interfaces (`Saver`, `Loader`), constructor functions (`NewMemoryStore`), pure free functions for transformations, total functions, named intermediates over chains, file-header docstrings.
    - **Ban:** `any`/`interface{}` outside protocol boundaries, `panic` outside `main` + unrecoverable init, `init()` functions, package-level mutable state in Go, `var` in JS, classes in JS, prototype mutation, mocks for things we own (DB, FS), comments that restate code, "smart" one-liners with chaining, bare `return err` without wrap, `console.log` in committed JS.

---

## File structure

| File | Status | Responsibility |
|---|---|---|
| `STYLE.md` | **create** | Single source of truth: 11 rules + embrace/ban list + pointer to language-specific files |
| `CLAUDE.md` | **create** | One-line pointer: "Before writing code, read STYLE.md." |
| `docs/style/go.md` | **create** | Go specifics with concrete examples drawn from existing repo |
| `docs/style/js.md` | **create** | JS specifics with concrete examples |
| `.golangci.yml` | **create** | Curated `golangci-lint` config matching the lintable Go rules |
| `package.json` | **create** | Pin `prettier` + `eslint` as devDependencies |
| `.prettierrc.json` | **create** | Format config (printWidth 100, single quotes, trailing commas) |
| `.eslintrc.cjs` | **create** | Lint config matching the lintable JS rules |
| `.gitignore` | modify | Append `node_modules/` |
| `scripts/install-hooks.sh` | **create** | Idempotent installer that symlinks the tracked hook into `.git/hooks/` |
| `scripts/git-hooks/pre-commit` | **create** | Runs `gofmt -s -d`, `golangci-lint run`, `npx prettier --check`, `npx eslint`, blocks on any failure |
| `README.md` | modify | Add "Setup" section with `npm install && bash scripts/install-hooks.sh` |

---

## Task 1 — `STYLE.md` (root) + `CLAUDE.md` (root)

**Files:**
- Create: `STYLE.md`
- Create: `CLAUDE.md`

- [ ] **Step 1: Write `STYLE.md`** at the repo root. Structure:

```markdown
# Code Style — claw-study

This is the source of truth for how code is written in this repo. The locked decisions below come from a 2026-05-10 grill-me session and are not up for re-litigation without an explicit follow-up session.

> **For agents:** read this file before writing any code. Lint enforces the mechanically-checkable subset (see `.golangci.yml`, `.eslintrc.cjs`); the prose rules below carry the rest. Pre-commit blocks on lint failures — do not bypass without permission.

## Philosophy

Code is read more than written. The rules below privilege the reader.

Two non-negotiable principles:
1. **One operation per line.** No chained calls. Named intermediates carry meaning.
2. **The smallest unit names a concept.** Methods name subject-verb-object sentences. Files have a one-paragraph responsibility statement.

## The 11 rules

[Number each rule from the locked-decisions list above. Each rule gets one paragraph of plain-English statement plus a one-line example of compliant vs non-compliant code where applicable. Do NOT inline the language-specific examples — those go in docs/style/{go,js}.md and are linked from here.]

## Embrace / ban

[Two columns. Each entry: the pattern, one-line rationale, link to the relevant rule number.]

## Where rules live

| Concern | File |
|---|---|
| Philosophy + rules | this file |
| Go specifics + examples | `docs/style/go.md` |
| JS specifics + examples | `docs/style/js.md` |
| Go lint config | `.golangci.yml` |
| JS lint config | `.eslintrc.cjs` |
| Format config (JS) | `.prettierrc.json` |
| Pre-commit hook | `scripts/git-hooks/pre-commit` |

## Enforcement

- `gofmt -s` and `prettier --check` run pre-commit. Format failures block.
- `golangci-lint run` and `npx eslint` run pre-commit. Lint failures block.
- Conceptual rules (file-header invariants, verb-noun error wraps, behavior-test naming, etc.) are not lint-enforceable — reviewer attention catches drift.

## Updating this file

Style changes require a fresh grill-me session. Single rule edits land via PR with rationale; sweeping changes need a sibling document explaining the new philosophy and what's superseded.
```

Target length: ~150-200 lines. Be concrete; no filler. Use the locked decisions verbatim where applicable.

- [ ] **Step 2: Write `CLAUDE.md`** at repo root (NEW — distinct from existing `CLAUDE.local.md` which is gitignored runtime guidance for the study agent itself).

```markdown
# CLAUDE.md — claw-study

**Before writing or modifying code, read [`STYLE.md`](./STYLE.md).**

It carries the project's code style decisions. Pre-commit lint enforces the mechanically-checkable subset; the prose carries the rest.

For runtime/study-agent behavior, see `CLAUDE.local.md` (gitignored, ops-only).
```

- [ ] **Step 3: Verify both files render and the cross-references resolve**

```bash
test -f ~/Documents/ITA/claw-study/STYLE.md
test -f ~/Documents/ITA/claw-study/CLAUDE.md
grep -q 'STYLE.md' ~/Documents/ITA/claw-study/CLAUDE.md
grep -q 'docs/style/go.md' ~/Documents/ITA/claw-study/STYLE.md
grep -q 'docs/style/js.md' ~/Documents/ITA/claw-study/STYLE.md
```

All commands must exit 0.

- [ ] **Step 4: Commit**

```bash
cd ~/Documents/ITA/claw-study
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho \
  add STYLE.md CLAUDE.md
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho \
  commit -m "docs: add STYLE.md as code-style source of truth"
```

---

## Task 2 — `docs/style/go.md`

**Files:**
- Create: `docs/style/go.md`

- [ ] **Step 1: Write `docs/style/go.md`** with these sections:

1. **Header** — one-paragraph responsibility statement.
2. **Naming** — hybrid rules with concrete examples:
   - Bad: `s.Save(m)` where `s` is `*MemoryStore`, `m` is `Memory`.
   - Good: `store.Save(memory)`.
   - Bad: `for memoryIndex := 0; memoryIndex < len(memories); memoryIndex++`.
   - Good: `for i, memory := range memories`.
   - Receivers: 1–3 chars (`s *MemoryStore`, `a *App`, `h *Handler`). Plumbing: `db`, `ctx`, `r`, `t *testing.T`.
3. **Type safety** — typed identifiers:
   - Bad: `func Save(courseID string, ...)`.
   - Good: `type CourseID string; func Save(course CourseID, ...)`.
   - When NOT to: ephemeral or boundary-layer values (`title string` is fine; SQL Scan targets stay raw).
4. **One operation per line** — no chains. Use named intermediates:
   - Bad: `return strings.ToLower(strings.TrimSpace(input))`.
   - Good:
     ```go
     trimmed := strings.TrimSpace(input)
     return strings.ToLower(trimmed)
     ```
5. **Errors** — `verb noun: %w` wrapping; one log site per request; sentinels on demand. Reference live examples from `agent/memory.go`.
6. **Decomposition** — when to make a struct/method/free function. Reference `MemoryStore` (struct earned its keep) vs. `AssembleAgentsMD` (free function — no natural subject).
7. **Comments** — file-header pattern with three real examples drawn from `agent/memory.go`, `claw-cli/main.go`, `seed-memory/main.go`. Doc-comment-on-exported rule. Anti-pattern: comment that restates code.
8. **Tests** — behavior-first, real DB, sentence names. Reference `agent/memory_test.go` patterns.
9. **Patterns embraced** — typed identifiers, behavior interfaces (`Saver`, `Loader`), constructors, pure helpers, total functions, named intermediates.
10. **Patterns banned** — `any`/`interface{}` outside boundaries, `panic` outside `main`, `init()`, package-level mutable state, mocks for things we own, comments that restate code, "smart" one-liners.

Each rule must include at least one **good** and one **bad** code snippet. Snippets should be short (≤10 lines). Where possible, draw from the live repo.

Target length: 250-400 lines. Skim-friendly, scannable, never aspirational without a concrete example.

- [ ] **Step 2: Verify**

```bash
test -f ~/Documents/ITA/claw-study/docs/style/go.md
wc -l ~/Documents/ITA/claw-study/docs/style/go.md   # should be 250-450
grep -c '^## ' ~/Documents/ITA/claw-study/docs/style/go.md  # should be ≥10
```

- [ ] **Step 3: Commit**

```bash
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho \
  add docs/style/go.md
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho \
  commit -m "docs: Go style guide with examples"
```

---

## Task 3 — `docs/style/js.md`

**Files:**
- Create: `docs/style/js.md`

- [ ] **Step 1: Write `docs/style/js.md`** with these sections:

1. **Header** — responsibility statement.
2. **Module structure** — ES modules, file-per-concern. Concrete file split for current `static/app.js` (1203 lines) → `static/js/{api,sessions,chat,plan,pdf,main}.js`. Show the `<script type="module" src="/static/js/main.js"></script>` change to `index.html`. Note: the actual split is a separate refactor task — this file just describes the target shape.
3. **Stateful units (closure factories)** — concrete example:
   ```js
   // Good
   export function createChatStream({ sessionId, onToken }) {
     let abortController = null;
     async function start(prompt) { /* ... */ }
     function stop() { abortController?.abort(); }
     return { start, stop };
   }
   ```
   Compare against the banned class form. Explain why `this` binding is avoided.
4. **Module-scope singletons** — when `let currentSession = null` at module top is right; when it's not.
5. **Async** — `async/await` always; ban on `.then()` chains. Show `apiFetch` integration.
6. **Variables** — `const` default, `let` when reassignment genuinely needed, `var` banned. Lint will catch.
7. **Naming** — hybrid (descriptive for domain, short for plumbing). DOM elements: `sessionListEl` not `el`. Event handlers: `onClick`, `onSubmit`.
8. **Errors** — try/catch only where you can do something useful. Surface to user via the existing error banner. No `console.log` in committed code.
9. **DOM patterns** — event delegation via `data-action` (already established in the repo). No inline `onclick`. Helpers for `el.querySelector` etc., not direct DOM access scattered everywhere.
10. **Patterns embraced** — closure factories, destructuring at function boundaries, template literals, optional chaining, `apiFetch` for all GETs.
11. **Patterns banned** — classes, prototypal inheritance, `var`, `this`-binding tricks, `.then()` chains, `console.log` in committed code, anonymous functions where named would help, deep destructuring as cleverness.

Each rule needs at least one good and one bad snippet.

Target length: 200-350 lines.

- [ ] **Step 2: Verify**

```bash
test -f ~/Documents/ITA/claw-study/docs/style/js.md
wc -l ~/Documents/ITA/claw-study/docs/style/js.md
grep -c '^## ' ~/Documents/ITA/claw-study/docs/style/js.md  # ≥10
```

- [ ] **Step 3: Commit**

```bash
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho \
  add docs/style/js.md
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho \
  commit -m "docs: JS style guide with examples"
```

---

## Task 4 — `.golangci.yml` + first lint pass

**Files:**
- Create: `.golangci.yml`

- [ ] **Step 1: Write `.golangci.yml`** with this config:

```yaml
run:
  timeout: 3m
  go: "1.24"

linters:
  disable-all: true
  enable:
    - errcheck       # unhandled errors
    - gosimple       # simpler constructions
    - govet          # vet bugs
    - ineffassign    # ineffective assignments
    - staticcheck    # general static analysis
    - unused         # unused code
    - revive         # naming + style (golint successor)
    - gocyclo        # cyclomatic complexity
    - misspell       # typos in comments and strings
    - unconvert      # unnecessary type conversions
    - prealloc       # slice prealloc opportunities

linters-settings:
  gocyclo:
    min-complexity: 15
  revive:
    rules:
      - name: var-naming
      - name: exported
        arguments: ["checkPrivateReceivers", "disableStutteringCheck"]
      - name: package-comments
      - name: unused-parameter
        disabled: true   # not aggressive enough; handled by `unused`
      - name: empty-block
      - name: superfluous-else

issues:
  exclude-rules:
    # Test files: allow longer functions, dot-imports if any
    - path: _test\.go
      linters: [gocyclo]
    # Generated/embedded SQL strings: don't flag every long line
    - path: agent/db\.go
      linters: [gocyclo]
```

- [ ] **Step 2: Install `golangci-lint` if not present**

```bash
which golangci-lint || brew install golangci-lint
golangci-lint --version
```

Expected: version 1.55+.

- [ ] **Step 3: Run against the repo and capture violations**

```bash
cd ~/Documents/ITA/claw-study
golangci-lint run ./... 2>&1 | tee /tmp/golangci-first-pass.log
echo "violations: $(grep -c '\.go:[0-9]' /tmp/golangci-first-pass.log)"
```

Record the count of violations. The repo was written before lint was enforced, so some are expected. **Do NOT fix them in this task** — Task 6 will gate new commits on lint, and the existing violations become Phase-2-prep cleanup work. Note them in the commit message.

- [ ] **Step 4: Commit**

```bash
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho \
  add .golangci.yml
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho \
  commit -m "lint: add .golangci.yml ($N existing violations to clean up later)"
```

(Replace `$N` with the actual count.)

---

## Task 5 — `package.json` + `.prettierrc.json` + `.eslintrc.cjs` + `.gitignore`

**Files:**
- Create: `package.json`
- Create: `.prettierrc.json`
- Create: `.eslintrc.cjs`
- Modify: `.gitignore`

- [ ] **Step 1: Write `package.json`** at repo root:

```json
{
  "name": "claw-study",
  "private": true,
  "description": "Devtools-only manifest. The app is Go; this exists to pin prettier + eslint for the vanilla-JS frontend.",
  "scripts": {
    "format": "prettier --check 'static/**/*.js' 'static/**/*.html' 'static/**/*.css'",
    "format:write": "prettier --write 'static/**/*.js' 'static/**/*.html' 'static/**/*.css'",
    "lint": "eslint 'static/**/*.js'"
  },
  "devDependencies": {
    "prettier": "^3.3.0",
    "eslint": "^9.0.0"
  }
}
```

- [ ] **Step 2: Write `.prettierrc.json`**:

```json
{
  "printWidth": 100,
  "singleQuote": true,
  "trailingComma": "all",
  "semi": true,
  "tabWidth": 2,
  "arrowParens": "always"
}
```

- [ ] **Step 3: Write `.eslintrc.cjs`**:

```js
module.exports = {
  root: true,
  env: {
    browser: true,
    es2022: true,
  },
  parserOptions: {
    ecmaVersion: 2022,
    sourceType: 'module',
  },
  extends: ['eslint:recommended'],
  rules: {
    'no-var': 'error',
    'prefer-const': 'error',
    'no-unused-vars': ['error', { argsIgnorePattern: '^_' }],
    'no-undef': 'error',
    'no-console': ['error', { allow: ['warn', 'error'] }],
    eqeqeq: ['error', 'always'],
    'no-implicit-globals': 'error',
    'no-prototype-builtins': 'error',
    'no-throw-literal': 'error',
  },
};
```

(Note: ESLint 9 uses flat config by default. If installation reveals flat-config requirement, convert to `eslint.config.js` flat format with the same rules.)

- [ ] **Step 4: Append to `.gitignore`**

```
node_modules/
```

(Check if it's already there; only append if missing.)

- [ ] **Step 5: Install and verify**

```bash
cd ~/Documents/ITA/claw-study
npm install
ls node_modules/prettier node_modules/eslint   # should both exist
npx prettier --version
npx eslint --version
```

- [ ] **Step 6: Run format check + lint against current static/**

```bash
cd ~/Documents/ITA/claw-study
npx prettier --check 'static/**/*.js' 'static/**/*.html' 'static/**/*.css' 2>&1 | tee /tmp/prettier-first.log
npx eslint 'static/**/*.js' 2>&1 | tee /tmp/eslint-first.log
```

Expect: existing files have format/lint violations (the project was written before this config). Record the counts. Do NOT fix in this task.

- [ ] **Step 7: Commit**

```bash
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho \
  add package.json package-lock.json .prettierrc.json .eslintrc.cjs .gitignore
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho \
  commit -m "lint: add prettier + eslint configs (existing JS to be cleaned later)"
```

If `package-lock.json` does not exist (npm v9+ should produce it), that's fine — adjust the `git add`.

---

## Task 6 — Pre-commit hook installer + smoke test

**Files:**
- Create: `scripts/install-hooks.sh`
- Create: `scripts/git-hooks/pre-commit`
- Modify: `README.md` (append a "Setup" section)

- [ ] **Step 1: Write `scripts/git-hooks/pre-commit`** (the tracked hook):

```bash
#!/usr/bin/env bash
# Pre-commit hook: blocks commits that fail format or lint.
# Installed via scripts/install-hooks.sh.
set -e

ROOT="$(git rev-parse --show-toplevel)"
cd "$ROOT"

echo "==> gofmt -s -d"
gofmt_out="$(gofmt -s -d $(find . -name '*.go' -not -path './node_modules/*' -not -path './.git/*'))"
if [ -n "$gofmt_out" ]; then
  echo "$gofmt_out"
  echo "FAIL: run 'gofmt -s -w .' to fix"
  exit 1
fi

echo "==> golangci-lint run"
if ! golangci-lint run ./...; then
  echo "FAIL: fix lint violations or commit a corresponding cleanup"
  exit 1
fi

if [ -d "$ROOT/node_modules" ]; then
  echo "==> prettier --check"
  if ! npx --no-install prettier --check 'static/**/*.js' 'static/**/*.html' 'static/**/*.css'; then
    echo "FAIL: run 'npm run format:write' to fix"
    exit 1
  fi
  echo "==> eslint"
  if ! npx --no-install eslint 'static/**/*.js'; then
    echo "FAIL: fix eslint violations"
    exit 1
  fi
else
  echo "WARN: node_modules missing; skipping prettier+eslint. Run 'npm install'."
fi

echo "OK: format + lint clean"
```

Make it executable: `chmod +x scripts/git-hooks/pre-commit`.

- [ ] **Step 2: Write `scripts/install-hooks.sh`**:

```bash
#!/usr/bin/env bash
# Idempotent installer: symlinks the tracked pre-commit hook into .git/hooks/.
set -e

ROOT="$(git rev-parse --show-toplevel)"
SRC="$ROOT/scripts/git-hooks/pre-commit"
DST="$ROOT/.git/hooks/pre-commit"

if [ ! -f "$SRC" ]; then
  echo "FAIL: $SRC not found"
  exit 1
fi

# Remove any existing hook (back it up if non-empty + not our symlink)
if [ -e "$DST" ] && [ ! -L "$DST" ]; then
  if [ -s "$DST" ]; then
    cp "$DST" "$DST.backup.$(date +%s)"
    echo "Backed up existing hook to $DST.backup.*"
  fi
  rm "$DST"
fi

ln -sfn "$SRC" "$DST"
chmod +x "$SRC"
echo "Installed pre-commit hook: $DST -> $SRC"
```

Make it executable: `chmod +x scripts/install-hooks.sh`.

- [ ] **Step 3: Run the installer**

```bash
cd ~/Documents/ITA/claw-study
bash scripts/install-hooks.sh
ls -la .git/hooks/pre-commit
```

Expected output: `.git/hooks/pre-commit -> scripts/git-hooks/pre-commit` (symlink).

- [ ] **Step 4: Smoke-test the hook BLOCKS bad commits**

Stage a deliberately malformed Go file, attempt to commit, expect block:

```bash
cd ~/Documents/ITA/claw-study
cat > /tmp/style_smoke.go << 'EOF'
package agent
import "fmt"
func   StyleSmokeTest( ) { fmt.Println( "bad" ) }
EOF
mv /tmp/style_smoke.go agent/style_smoke.go
git add agent/style_smoke.go
if git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho commit -m "smoke" 2>&1 | grep -q "FAIL: run 'gofmt"; then
  echo "OK: hook correctly blocked bad commit"
else
  echo "BLOCKED: hook did not fire as expected"
  exit 1
fi
git reset HEAD agent/style_smoke.go
rm agent/style_smoke.go
```

If the hook does not block, **STOP and report BLOCKED**.

- [ ] **Step 5: Append a "Setup" section to `README.md`**

Locate the existing README structure. Append (or insert near the top, after the project description) this section:

```markdown
## Setup (one-time)

```
git clone …
cd claw-study
npm install                          # prettier + eslint
brew install golangci-lint           # if not already installed
bash scripts/install-hooks.sh        # installs pre-commit hook
```

The pre-commit hook runs `gofmt`, `golangci-lint`, `prettier`, and `eslint`. Style rules and the philosophy behind them live in [STYLE.md](./STYLE.md).
```

- [ ] **Step 6: Commit (note: the hook will now run on this commit — it must pass)**

```bash
cd ~/Documents/ITA/claw-study
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho \
  add scripts/install-hooks.sh scripts/git-hooks/pre-commit README.md
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho \
  commit -m "tooling: pre-commit hook + installer for format/lint enforcement"
```

If the hook blocks this very commit because of pre-existing lint violations in unrelated Go files, **temporarily** add an exception in `.golangci.yml` (e.g. add the affected directories to `issues.exclude-rules`) — but only as a Phase-2-prep TODO recorded in the commit message. The goal here is to get the enforcement infrastructure landed; cleaning the existing repo is separate work.

- [ ] **Step 7: Push**

```bash
git push origin main
```

---

## Self-review checklist (controller)

- ✅ All 11 locked rules present in STYLE.md
- ✅ Both language guides have ≥10 sections each with good/bad examples
- ✅ `.golangci.yml`, `.prettierrc.json`, `.eslintrc.cjs` actually run on the laptop
- ✅ Pre-commit hook smoke-tested to block bad commits
- ✅ `node_modules/` in `.gitignore`
- ✅ README has the setup steps
- ⚠️ Existing repo lint violations are recorded but not fixed — Phase-2-prep work
