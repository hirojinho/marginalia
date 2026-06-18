# OSS Distribution Stage 1 — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `github.com/hirojinho/claw-study` publicly presentable and runnable in ~5 minutes against a bundled sample corpus — without changing app runtime behavior.

**Architecture:** Pure repo/doc/config work on the `oss-prep` branch of a fresh Mac clone. Add legal files (LICENSE), scrub personal data, bundle a markdown sample corpus, document two run recipes (OpenAI + Ollama), and embed a screenshot captured from a real local run. Merge to `main` at the end.

**Tech Stack:** Go 1.24+ (app), SQLite, vanilla JS SPA (embedded), Markdown (corpus + docs).

## Global Constraints

- Branch: `oss-prep` (cut from `main`); merge at the end, rebasing once if the overnight pipeline landed commits.
- Working dir: `/Users/eduardohiroji/Documents/ITA/Mestrado/projects/claw-study`.
- License: **MIT**, `Copyright (c) 2026 Eduardo Hiroji`.
- Do **not** change app runtime behavior or refactor the app. Docs/config/sample-data only.
- Keep process artifacts: `specs/`, `docs/adr/`, `docs/postmortem/`, `skills/` — they are the showcase. Surgical scrub only.
- RAG corpus = **markdown only** (`agent/vectorstore.go:63`); corpus dir is `$VAULT_ROOT/data/corpus/`; `IndexCorpus()` runs on startup (`main.go:83`).
- Every commit ends with the Co-Authored-By trailer:
  `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`

---

### Task 1: Add LICENSE + fix README License section

**Files:**
- Create: `LICENSE`
- Modify: `README.md` (the `## License` section, currently "No license is set yet…")

- [ ] **Step 1: Create `LICENSE` (MIT)**

```
MIT License

Copyright (c) 2026 Eduardo Hiroji

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

- [ ] **Step 2: Replace the README `## License` section**

Find the current block (starts with "No license is set yet, so default copyright applies…") and replace the body with:

```markdown
## License

[MIT](./LICENSE) © 2026 Eduardo Hiroji.
```

- [ ] **Step 3: Verify**

Run: `test -f LICENSE && grep -q "MIT License" LICENSE && echo OK`
Run: `grep -n "No license is set yet" README.md || echo "stale text gone"`
Expected: `OK` then `stale text gone`.

- [ ] **Step 4: Commit**

```bash
git add LICENSE README.md
git commit -m "$(printf 'docs: add MIT LICENSE and fix README license section\n\nCo-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>')"
```

---

### Task 2: Untrack `CLAUDE.local.md`

**Files:**
- Modify: `.gitignore`
- Remove from tracking (keep on disk): `CLAUDE.local.md`

- [ ] **Step 1: Stop tracking the file (keep working copy)**

```bash
git rm --cached CLAUDE.local.md
```

- [ ] **Step 2: Add it to `.gitignore`**

Append under the "editor/agent backup cruft" group:

```
# machine-local agent instructions (not app source)
/CLAUDE.local.md
```

- [ ] **Step 3: Verify it is no longer tracked but still present**

Run: `git ls-files CLAUDE.local.md` → expected: empty output.
Run: `test -f CLAUDE.local.md && git check-ignore CLAUDE.local.md` → expected: `CLAUDE.local.md`.

- [ ] **Step 4: Commit**

```bash
git add .gitignore
git commit -m "$(printf 'chore: stop tracking machine-local CLAUDE.local.md\n\nCo-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>')"
```

---

### Task 3: Genericize `seed-memory/main.go`

**Files:**
- Modify: `seed-memory/main.go` (header comment lines 1-3, `userID` const line 20, `-source` default line 23)

**Interfaces:**
- Produces: `seed-memory` binary with `-source` defaulting to `./memory` and no personal path baked in. `userID` becomes a `-user` flag (default `"default"`).

- [ ] **Step 1: Rewrite the header comment (lines 1-4)**

Replace:
```go
// Command seed-memory imports Eduardo's existing memory store at
// ~/.claude/projects/-Users-eduardohiroji-Documents-ITA-Mestrado/memory/
// into the agent_memory SQLite table. Idempotent: deletes all rows for
// the user before reseeding.
```
With:
```go
// Command seed-memory imports a directory of markdown memory files into the
// agent_memory SQLite table. Point -source at your memory directory.
// Idempotent: deletes all rows for the user before reseeding.
```

- [ ] **Step 2: Replace the hardcoded `-source` default (line 23)**

Replace:
```go
	source := flag.String("source", os.ExpandEnv("$HOME/.claude/projects/-Users-eduardohiroji-Documents-ITA-Mestrado/memory"), "source memory directory")
```
With:
```go
	source := flag.String("source", "./memory", "source memory directory")
```

- [ ] **Step 3: Make `userID` configurable (replace const at line 20)**

Replace:
```go
const userID = "eduardo"
```
With:
```go
var userID = "default"
```
Then add a flag inside `main()` immediately after the `source` line (before `dbPath`):
```go
	userFlag := flag.String("user", userID, "user id to seed memory under")
```
And after `flag.Parse()`, set:
```go
	userID = *userFlag
```

- [ ] **Step 4: Confirm `os` is still used; remove the import only if now unused**

Run: `cd seed-memory && grep -n "os\." main.go`
If `os.` no longer appears anywhere, remove the `"os"` import line; otherwise leave it.
(Note: `os.ExpandEnv` was likely the only `os` use — check and prune.)

- [ ] **Step 5: Build + verify the help text carries no personal path**

Run: `go build ./... && go run ./seed-memory -h 2>&1 | grep -- '-source'`
Expected: shows `(default "./memory")`, no `eduardoji`/`.claude` path.
Run: `grep -rn "eduardoji\|\.claude/projects" seed-memory/main.go || echo "clean"`
Expected: `clean`.

- [ ] **Step 6: Run existing seed-memory tests**

Run: `go test ./seed-memory/...`
Expected: PASS (the change is to flag defaults, not to `collect()` logic).

- [ ] **Step 7: Commit**

```bash
git add seed-memory/main.go
git commit -m "$(printf 'refactor(seed-memory): remove hardcoded personal path; add -user flag\n\nCo-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>')"
```

---

### Task 4: Personal-leakage scrub

**Files:**
- Modify (as hits dictate): `CLAUDE.md`, `AGENTS.md`, `memory/study-context.md`, and any doc under `docs/` or `memory/` that leaks private data.

- [ ] **Step 1: Find all leakage hits**

Run (from repo root):
```bash
grep -rniE 'hiroji|/home/hiro|/Users/eduardo|claw-study\.xyz|192\.168\.[0-9]+\.[0-9]+|eduardo\.hiroji@' \
  --include='*.md' --include='*.go' . \
  | grep -v -E '^\./(LICENSE|docs/superpowers/(specs|plans)/2026-06-17-oss)' 
```
Record each hit (file:line).

- [ ] **Step 2: Genericize each hit by category**

Apply these rules to each hit (edit in place):
- `https://study.claw-study.xyz` → keep ONLY where the README intentionally names the reference deployment; elsewhere replace with `https://your-host.example`.
- `/home/hiro/...` or `/Users/eduardo.../...` absolute paths → replace with a relative path or `$VAULT_ROOT/...` / `~/...` placeholder appropriate to the sentence.
- `192.168.x.y` → replace with `<your-host>` or remove the deployment-specific aside.
- `eduardo.hiroji@…` email → remove or replace with a neutral contact note.
- Personal first/last name in prose → keep author attribution (LICENSE, a single README credit line is fine); remove incidental mentions that read as private notes.

Do NOT touch `specs/done/`, `docs/adr/`, `docs/postmortem/` content unless a hit is a genuine secret/credential — those are showcase history and personal-but-harmless context is acceptable there.

- [ ] **Step 3: Re-run the grep to confirm only intentional mentions remain**

Run the Step 1 command again. Expected: zero hits, OR only the deliberately-kept README attribution/reference-deployment lines (list them in the commit body).

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "$(printf 'docs: scrub personal paths/host/email from tracked docs\n\nKept: <list any intentional mentions>\n\nCo-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>')"
```

---

### Task 5: Bundle a markdown sample corpus

**Files:**
- Create: `examples/sample-corpus/01-photosynthesis.md`
- Create: `examples/sample-corpus/02-cellular-respiration.md`
- Create: `examples/README.md`

(Original content we author → trivially MIT-licensed with the repo. Neutral topic so it reads as a believable study corpus.)

- [ ] **Step 1: Create `examples/sample-corpus/01-photosynthesis.md`**

```markdown
# Photosynthesis — Overview

Photosynthesis is the process by which plants, algae, and some bacteria convert
light energy into chemical energy stored in glucose. It occurs mainly in the
chloroplasts, using the green pigment chlorophyll to absorb light.

## The two stages

1. **Light-dependent reactions** — occur in the thylakoid membranes. Light
   splits water (photolysis), releasing oxygen as a by-product and producing
   ATP and NADPH.
2. **The Calvin cycle (light-independent reactions)** — occurs in the stroma.
   It uses ATP and NADPH to fix carbon dioxide into glucose via the enzyme
   RuBisCO.

## Overall equation

6 CO2 + 6 H2O + light energy -> C6H12O6 + 6 O2

## Why it matters

Photosynthesis is the primary entry point of energy into most food chains and
is responsible for the oxygen in Earth's atmosphere.
```

- [ ] **Step 2: Create `examples/sample-corpus/02-cellular-respiration.md`**

```markdown
# Cellular Respiration — Overview

Cellular respiration is the process by which cells break down glucose to release
energy stored as ATP. It is, in effect, the reverse of photosynthesis.

## The three stages

1. **Glycolysis** — in the cytoplasm; splits glucose into two pyruvate
   molecules, yielding a small amount of ATP and NADH.
2. **The Krebs cycle (citric acid cycle)** — in the mitochondrial matrix;
   produces NADH, FADH2, and CO2.
3. **Oxidative phosphorylation** — across the inner mitochondrial membrane;
   the electron transport chain uses NADH and FADH2 to produce most of the ATP,
   with oxygen as the final electron acceptor.

## Overall equation

C6H12O6 + 6 O2 -> 6 CO2 + 6 H2O + ATP

## Why it matters

Cellular respiration supplies the ATP that powers nearly every active process in
the cell, linking the energy captured by photosynthesis to the work of life.
```

- [ ] **Step 3: Create `examples/README.md`**

```markdown
# Examples

## `sample-corpus/`

A tiny, self-contained markdown corpus you can use to try claw-study in a few
minutes (see the project README's "Running locally" section). The documents are
original content written for this repository and are covered by the project's
[MIT license](../LICENSE) — free to copy, modify, and redistribute.

To use it, copy the markdown files into your vault's corpus directory:

\`\`\`bash
mkdir -p "$VAULT_ROOT/data/corpus"
cp examples/sample-corpus/*.md "$VAULT_ROOT/data/corpus/"
\`\`\`

claw-study indexes `$VAULT_ROOT/data/corpus/*.md` on startup, so the next launch
embeds these files and they become answerable through the chat/RAG tools.
```

- [ ] **Step 4: Verify files exist and are markdown**

Run: `ls examples/sample-corpus/*.md && wc -l examples/sample-corpus/*.md`
Expected: two files listed.

- [ ] **Step 5: Commit**

```bash
git add examples/
git commit -m "$(printf 'docs: add bundled markdown sample corpus for the 5-minute run\n\nCo-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>')"
```

---

### Task 6: `.env.example` + README 5-minute run recipes

**Files:**
- Create: `.env.example`
- Modify: `README.md` (the `## Running locally` section)

**Interfaces:**
- Consumes: sample corpus from Task 5 (`examples/sample-corpus/*.md`); env var names from `config.go` (`LLM_API_KEY`, `LLM_API_URL`, `LLM_MODEL`, `EMBEDDING_MODEL`, `VAULT_ROOT`, `LISTEN_ADDR`, `AUTH_TOKEN`).

- [ ] **Step 1: Create `.env.example` (OpenAI happy-path defaults)**

```bash
# --- LLM provider (OpenAI happy-path) ---
# Paste an OpenAI key (or any OpenAI-compatible bearer).
LLM_API_KEY=sk-your-key-here
LLM_API_URL=https://api.openai.com/v1
LLM_MODEL=gpt-4o-mini
EMBEDDING_MODEL=text-embedding-3-small

# --- App ---
# Local scratch vault. The app reads/writes $VAULT_ROOT/data and indexes
# $VAULT_ROOT/data/corpus/*.md on startup.
VAULT_ROOT=./vault
LISTEN_ADDR=:8081
# Local dev: leave empty to warn-and-allow (no auth). Set to gate all routes.
AUTH_TOKEN=
```

- [ ] **Step 2: Replace the README `## Running locally` body with a 5-minute path + two recipes**

Replace the existing `## Running locally` section (through just before `## Tests`) with:

````markdown
## Running locally

claw-study is a single Go binary. The fastest way to see it work is the
**5-minute path** below, using the bundled [sample corpus](examples/sample-corpus/).

**Prerequisites:** Go ≥ 1.24. (Contributor tooling — Node/ESLint/golangci-lint/git
hooks — is covered under [Contributing](#contributing--conventions); it is not
needed just to run the app.)

### 5-minute path (OpenAI)

```bash
git clone https://github.com/hirojinho/claw-study.git
cd claw-study

# 1. Config — fill in your OpenAI key
cp .env.example .env
$EDITOR .env                       # set LLM_API_KEY=sk-...

# 2. Load the sample corpus into the vault
mkdir -p ./vault/data/corpus
cp examples/sample-corpus/*.md ./vault/data/corpus/

# 3. Run (env vars from .env)
export $(grep -v '^#' .env | xargs)
go run .
```

Open `http://localhost:8081`. The app indexes the corpus on startup; ask the
tutor something answerable from it (e.g. *"What are the two stages of
photosynthesis?"*) to confirm RAG is working.

### Zero-cost local alternative (Ollama)

[Ollama](https://ollama.com) serves an OpenAI-compatible API locally, including
embeddings — no API key, no cost.

```bash
ollama pull llama3.1            # chat model
ollama pull nomic-embed-text    # embedding model
```

Then point the app at Ollama in `.env`:

```bash
LLM_API_KEY=ollama                       # any non-empty value; Ollama ignores it
LLM_API_URL=http://localhost:11434/v1
LLM_MODEL=llama3.1
EMBEDDING_MODEL=nomic-embed-text
```

Load the corpus and run exactly as in the 5-minute path (steps 2–3).

### Env vars

| Var | Purpose |
|---|---|
| `LLM_API_KEY` (or `OPENCODE_API_KEY`) | Bearer for the OpenAI-compatible chat + embeddings endpoint. |
| `LLM_API_URL` | Base URL for chat completions and `/embeddings`. |
| `LLM_MODEL` | Chat model id. |
| `EMBEDDING_MODEL` | Embedding model id (used to index the corpus). |
| `VAULT_ROOT` | Root for `data/` (corpus, db) and `memory/`. |
| `LISTEN_ADDR` | Defaults to `:8081`. |
| `AUTH_TOKEN` | If set, gates all routes except `/login`. Empty = warn-and-allow (local dev). |

The app binds to `LISTEN_ADDR` and serves the embedded SPA at `/`. With
`AUTH_TOKEN` set, visit `/login?token=$AUTH_TOKEN` once to set the cookie.
````

- [ ] **Step 3: Verify env names match config.go**

Run: `for v in LLM_API_KEY LLM_API_URL LLM_MODEL EMBEDDING_MODEL VAULT_ROOT LISTEN_ADDR AUTH_TOKEN; do grep -q "$v" config.go && echo "$v ok" || echo "$v MISSING"; done`
Expected: all `ok`.
Run: `test -f .env.example && echo OK`

- [ ] **Step 4: Confirm `.env.example` is not gitignored**

Run: `git check-ignore .env.example && echo "IGNORED — fix" || echo "trackable"`
Expected: `trackable`. (The `.gitignore` ignores `.env` patterns broadly — if `.env.example` is caught, add `!.env.example` exception.)

- [ ] **Step 5: Commit**

```bash
git add .env.example README.md .gitignore
git commit -m "$(printf 'docs: add .env.example and 5-minute run recipes (OpenAI + Ollama)\n\nCo-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>')"
```

---

### Task 7: Verify the run end-to-end + capture and embed screenshot

**Files:**
- Create: `docs/assets/screenshot-chat.png` (captured from the live run)
- Modify: `README.md` (replace the screenshot TODO comment + Demo note with the embedded image)

**Requires from the user:** an OpenAI API key (or OpenAI-compatible endpoint). This is the human-in-the-loop gate.

- [ ] **Step 1: Real run against the sample corpus**

```bash
cp .env.example .env          # then set LLM_API_KEY to a real key
mkdir -p ./vault/data/corpus && cp examples/sample-corpus/*.md ./vault/data/corpus/
export $(grep -v '^#' .env | xargs)
go run .
```
Expected logs include `indexed file` / `indexed chunks` for both sample files.

- [ ] **Step 2: Behavioral check (RAG actually answers)**

In the browser at `http://localhost:8081`, ask: *"What are the two stages of photosynthesis?"*
Expected: an answer naming the light-dependent reactions and the Calvin cycle, grounded in the corpus.
This is the verification gate — not "it compiled."

- [ ] **Step 3: Capture the screenshot**

Capture the chat view (ideally showing chat + plan + PDF/doc panel). Save as `docs/assets/screenshot-chat.png`.

```bash
mkdir -p docs/assets
# save the capture to docs/assets/screenshot-chat.png
```

- [ ] **Step 4: Embed it in the README + remove the TODO**

In `README.md`:
- Delete the HTML comment block that begins `<!--\n  TODO (highest-leverage addition...` and ends `-->`.
- Under the `## Highlights`-adjacent intro (right after the centered pitch links, before `## Highlights`), add:
```markdown
<p align="center">
  <img src="docs/assets/screenshot-chat.png" alt="claw-study chat with study plan and source corpus" width="900">
</p>
```
- In the `> **Demo:**` blockquote, change the trailing "A screenshot/screencast is coming; until then…" to "See the screenshot above; the design is detailed below and in [`docs/specs/architecture.md`](docs/specs/architecture.md)."

- [ ] **Step 5: Verify**

Run: `test -f docs/assets/screenshot-chat.png && echo OK`
Run: `grep -q 'screenshot-chat.png' README.md && echo embedded`
Run: `grep -q 'TODO (highest-leverage' README.md && echo "TODO STILL PRESENT — remove" || echo "todo removed"`

- [ ] **Step 6: Commit**

```bash
git add docs/assets/screenshot-chat.png README.md
git commit -m "$(printf 'docs: embed app screenshot from verified local run; drop screenshot TODO\n\nCo-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>')"
```

---

### Task 8: Coherence pass, self-review, and merge

**Files:**
- Modify: `README.md` / docs as link checks dictate.

- [ ] **Step 1: Build + full test suite still green**

Run: `go build ./... && go test ./...`
Expected: build succeeds, tests PASS.

- [ ] **Step 2: README internal link check**

Run:
```bash
grep -oE '\]\(([^)]+)\)' README.md | sed -E 's/\]\(|\)//g' | grep -vE '^https?:|^#' | while read p; do test -e "$p" && echo "ok  $p" || echo "BROKEN $p"; done
```
Expected: no `BROKEN` lines. Fix any (e.g. confirm `docs/specs/architecture.md`, `examples/sample-corpus/`, `LICENSE` all resolve).

- [ ] **Step 3: Final personal-path sweep on tracked files**

Run: `git ls-files | xargs grep -lniE 'hiroji|/home/hiro|/Users/eduardo|192\.168\.[0-9]' 2>/dev/null | grep -vE 'specs/done|docs/adr|docs/postmortem|docs/superpowers/(specs|plans)/2026-06-17-oss|LICENSE'`
Expected: empty (only intentional showcase/attribution mentions remain).

- [ ] **Step 4: Confirm `CLAUDE.local.md` untracked**

Run: `git ls-files CLAUDE.local.md` → expected empty.

- [ ] **Step 5: Rebase onto latest main and merge**

```bash
git fetch origin
git rebase origin/main           # resolve if the overnight pipeline landed commits
go build ./... && go test ./...  # re-verify after rebase
git checkout main && git merge --ff-only oss-prep
git push origin main
```

- [ ] **Step 6: Final confirmation**

Run: `git log --oneline -8`
Expected: the Stage 1 commits present on `main`; tree clean.

---

## Self-Review

**Spec coverage:**
- LICENSE (MIT) + README license fix → Task 1 ✓
- Untrack `CLAUDE.local.md` → Task 2 ✓
- Genericize `seed-memory` default → Task 3 ✓
- Personal-leakage scrub → Task 4 + Task 8 Step 3 ✓
- Bundle sample corpus → Task 5 ✓
- Two run recipes (OpenAI + Ollama) → Task 6 ✓
- Screenshot captured + embedded → Task 7 ✓
- README/docs coherence pass → Task 8 ✓
- Verification gates (build, behavioral RAG answer, no personal paths) → Tasks 6/7/8 ✓
- Merge with one rebase → Task 8 ✓

**Notes:** `.env.example` did not exist (README referenced it) — creation folded into Task 6. Sample corpus is original markdown (RAG indexes `.md` only), sidestepping third-party licensing. Task 7 is the only task needing the user (OpenAI key) and is the behavioral gate.
