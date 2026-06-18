<h1 align="center">claw-study</h1>

<p align="center">
  <b>An app designed to be operated by an AI agent — not just by a human clicking a UI.</b>
</p>

<p align="center">
  A personal study tutor (Go · SQLite · RAG) whose every operation is reachable through<br>
  three doors: a web UI, a token-gated HTTP API, and a CLI the agent drives.
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.24%2B-00ADD8?logo=go&logoColor=white" alt="Go 1.24+">
  <img src="https://img.shields.io/badge/SQLite-WAL-003B57?logo=sqlite&logoColor=white" alt="SQLite WAL">
  <img src="https://img.shields.io/badge/deploy-single%20binary-2b7489" alt="single binary">
  <img src="https://img.shields.io/badge/Docker-not%20required-2496ED?logo=docker&logoColor=white" alt="no Docker">
  <img src="https://img.shields.io/badge/frontend-vanilla%20JS-f0b35e" alt="vanilla JS">
</p>

<p align="center">
  <a href="#highlights">Highlights</a> ·
  <a href="#architecture">Architecture</a> ·
  <a href="#reading-the-source">Reading the source</a> ·
  <a href="#running-locally">Running locally</a> ·
  <a href="#documentation">Docs &amp; ADRs</a>
</p>

<!--
  TODO (highest-leverage addition for a first-time reader): drop a screenshot or a
  ~60s screencast of the chat + plan + PDF viewer into docs/assets/ and embed it here.
  A README pitch *tells*; an image *shows*.
-->

> **Demo:** runs at `https://study.claw-study.xyz` (bearer-token gated — it holds private
> study data, so it's a reference deployment, not an open demo). A screenshot/screencast
> is coming; until then the design is described below and in [`docs/specs/architecture.md`](docs/specs/architecture.md).

claw-study is a single Go binary: an embedded single-page frontend, SQLite for all state,
RAG over a local course corpus, and an LLM tutor that reads your slides, tracks a study
plan, and takes notes with you. The part worth a look is the architecture *around* the
tutor — see [Highlights](#highlights).

## Table of Contents

- [Highlights](#highlights)
- [Architecture](#architecture)
- [Reading the source](#reading-the-source)
- [Running locally](#running-locally)
- [Tests](#tests)
- [Deploying](#deploying)
- [Documentation](#documentation)
- [Stack](#stack)
- [Contributing & conventions](#contributing--conventions)
- [License](#license)

## Highlights

What makes this more than a CRUD app with a chat box:

- **One implementation, three callers.** Each operation is a single `agent.App` method.
  The browser reaches it over HTTP, an *external* agent reaches the same method over the
  token-gated JSON API, and the *in-sandbox* agent reaches it through `claw-cli`.
  `App.LoadPlan` / `App.SavePlan` / `App.UpdateSessionTopic` each have one body and
  two-or-three front doors — so the app never rots into a human-only UI with a brittle
  API bolted on.

- **The agent's context is rendered from the database, per turn.** Before each agentic
  turn, `agent/sandbox.go` runs `claw-cli memory load` to synthesize a fresh `AGENTS.md`
  briefing out of live state: profile, project/feedback memory, recent sessions, the
  skills index, plan status, open-PDF hints, course steering, and the pedagogy rules. The
  agent gets a briefing assembled from app state every time it runs, not a static prompt.

- **`claw-cli` is the agent's hands** (`claw-cli/main.go`, ~1,400 lines) — a real,
  scriptable surface built for a non-human operator: `plan show/status/toggle/rewrite`,
  `pdf list/current/extract`, `rag search`, `memory load/search/save`,
  `course settings get/set`, `knowledge create/show/list`, `confidence trajectory`,
  `note save`, `session topic`, `web fetch`, `skill dispatch`.

- **Two chat runtimes behind one chat box.** `/chat` is a legacy in-process loop
  (call LLM → parse tool calls → dispatch in Go → repeat). `/chat-v2` (the default) spawns
  a `pi` coding-agent subprocess in a per-session sandbox, streaming
  `token / reasoning / tool_start / tool_end / done` events to the browser over SSE.
  Both are live; the trade-off is recorded in
  [ADR&nbsp;0006](docs/adr/0006-embed-pi-as-agent-runtime.md).

- **Boring-on-purpose substrate.** Single binary, no Docker
  ([ADR&nbsp;0003](docs/adr/0003-no-docker-portability-first.md)), vanilla JS with no build
  step ([ADR&nbsp;0004](docs/adr/0004-vanilla-js-frontend.md)), no service/repository layer
  ([ADR&nbsp;0002](docs/adr/0002-no-service-repository-layer.md)), deliberately chosen
  SQLite pragmas. Shipped via `scp` + user systemd + a cloudflared tunnel, backed up with
  restic. It runs in production and has a rollback procedure.

## Architecture

- `main.go` — entry point; loads config, opens DB, builds `agent.App`, registers routes, starts the HTTP server with timeouts and graceful shutdown.
- `agent/` — domain logic. `App` owns the DB connection and config (no package globals). Submodules: `db.go` (sqlite, WAL+busy_timeout+foreign_keys+synchronous=NORMAL pragmas), `llm.go` (OpenAI-compatible chat client + tool loop), `tools.go` (the function-call tools the LLM can invoke), `pi_runner.go` + `sandbox.go` (the Pi subprocess runtime and its per-session AGENTS.md), `vectorstore.go` + `chunker.go` (corpus indexing and cosine-similarity retrieval), `embed.go`, `memory.go`.
- `handler/` — HTTP layer. Each domain (sessions, plan, pdf, courses, static, auth) has its own file. All handlers hang off `*Handler` which carries `*App`, `*LLMClient`, and the embedded static FS.
- `claw-cli/` — the command surface the in-sandbox agent uses; calls the same `agent.App` methods as the handlers.
- `static/` — single-page frontend (HTML + vanilla JS, plus pdf.js for the viewer).
- `convert/` — separate binary for one-off corpus conversion.

The runtime state lives under `VAULT_ROOT`:
```
$VAULT_ROOT/
├── data/
│   ├── study.db          # sessions, messages, plan toggles, PDF metadata
│   ├── corpus/           # source markdowns indexed for RAG
│   ├── pdf-files/        # uploaded PDFs
│   ├── pdf-texts/        # extracted text per PDF
│   ├── agent-sessions/   # per-session Pi sandboxes (AGENTS.md, pi-session/)
│   └── plans/<id>.json   # per-course plans
└── memory/               # study notes, runbooks, project context
```

## Reading the source

A short, ordered trail rather than browsing alphabetically:

1. [`CONTEXT.md`](./CONTEXT.md) — the glossary. `Course → Plan → Task → Session`, Knowledge Component, Studying/Authoring/Steering. The data model and UI fall out of these definitions; read this first.
2. [`docs/adr/`](docs/adr/) — the *why*. Skim all 17; read 0002, 0006, 0009, 0011, 0007.
3. `main.go` (~185 lines) — the whole wiring diagram in one file.
4. `handler/handler.go : Register()` — every route in one place. Pick one and follow it.
5. `handler/sessions.go : handleChat` → `agent/llm.go : ProcessWithTools` → `agent/tools.go` — the simple in-process brain.
6. `handler/chat_v2.go` → `agent/sandbox.go` → `agent/pi_runner.go` — the agentic brain.
7. `claw-cli/main.go` — the agent's hands; closes the loop.
8. `agent/db.go` (schema first) and `agent/vectorstore.go` + `chunker.go` + `embed.go` — data & RAG.

## Running locally

**Prerequisites:** Go ≥ 1.24 (for `github.com/ledongthuc/pdf`), Node ≥ 18.18 (for ESLint 9 flat config), Homebrew.

One-time setup:

```bash
git clone …
cd claw-study
npm install                          # prettier + eslint
brew install golangci-lint           # if not already installed
bash scripts/install-hooks.sh        # installs the pre-commit hook
```

The pre-commit hook runs `gofmt`, `golangci-lint`, `prettier`, and `eslint` on staged files only. Style rules and the philosophy behind them live in [STYLE.md](./STYLE.md).

Run it:

```bash
cp .env.example .env  # if missing, see env vars below
export $(grep -v '^#' .env | xargs)
go run .
```

Required env vars:

| Var | Purpose |
|---|---|
| `LLM_API_KEY` (or `OPENCODE_API_KEY`) | Bearer for the OpenAI-compatible chat endpoint. |
| `LLM_API_URL` | Chat completions URL. Defaults to OpenCode's. |
| `LLM_MODEL` | Model id sent in chat requests. |
| `EMBEDDING_MODEL` | Used for corpus embedding (passed through to the API). |
| `VAULT_ROOT` | Root for `data/` and `memory/`. Locally point at a scratch dir. |
| `LISTEN_ADDR` | Defaults to `:8081`. |
| `AUTH_TOKEN` | If set, gates all routes except `/login` (Bearer header or cookie). Empty = warn-and-allow (local dev). |

The app binds to `LISTEN_ADDR` and serves the embedded SPA at `/`. Browser flow: visit `/login?token=$AUTH_TOKEN` once and the cookie is set for a year (HttpOnly, Secure, SameSite=Strict).

## Tests

Run before every push:

```bash
go vet ./...
go test ./...
staticcheck ./...
```

`staticcheck` is a separate tool — install it once with:

```bash
go install honnef.co/go/tools/cmd/staticcheck@latest
```

Coverage focuses on pure functions (`agent/chunker`, `agent/vectorstore`, `agent/embed`, plan/db helpers) and HTTP handlers (`handler/*_test.go`) using `httptest` against in-memory SQLite. Auth middleware has its own table-driven tests in `handler/auth_test.go`.

## Deploying

The app is shipped as a single Linux/amd64 binary, scp'd to the VPS, and managed by user systemd. Both the app and its named cloudflared tunnel run as `study-app.service` and `study-app-tunnel.service` under the `eduardo` user.

```bash
GOOS=linux GOARCH=amd64 go build \
  -ldflags "-X main.buildCommit=$(git rev-parse HEAD) -X main.buildTimestamp=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  -o /tmp/study-app-linux .
scp /tmp/study-app-linux nanoclaw:/home/eduardo/stack/study-app/bin/study-app.new
ssh nanoclaw 'cd ~/stack/study-app/bin \
  && cp study-app study-app.bak \
  && mv study-app.new study-app \
  && chmod +x study-app \
  && export XDG_RUNTIME_DIR=/run/user/$(id -u) \
  && systemctl --user restart study-app.service'
```

To roll back: `mv study-app study-app.broken && mv study-app.bak study-app && systemctl --user restart study-app.service`.

The cloudflared tunnel (`tunnel: 7dede37f-...`) keeps `https://study.claw-study.xyz` mapped to `127.0.0.1:8081`. The tunnel survives app restarts and only reconnects when its own service is restarted.

## Documentation

- [`docs/`](docs/) — index of specs, ADRs, and skill docs.
- [`docs/specs/`](docs/specs/) — current behavior (architecture, RAG, PDF viewer, tools).
- [`docs/adr/`](docs/adr/) — architecture decision records (17 of them).
- [`CONTEXT.md`](./CONTEXT.md) — the domain glossary.
- [`CHANGELOG.md`](CHANGELOG.md) — what shipped, when.
- [`ROADMAP.md`](ROADMAP.md) — Now / Next / Later.

Historical phase plans and superseded designs live in [`docs/specs/archive/`](docs/specs/archive/).

## Stack

Go 1.24+, `database/sql` + `mattn/go-sqlite3`, `ledongthuc/pdf`, embedded `static/`, `slog` for structured logs. Frontend is plain HTML/JS plus `pdf.js`. Agentic runtime is `@earendil-works/pi-coding-agent` spawned per turn. Single binary, no Node build step, no Docker.

## Contributing & conventions

This is a solo project, run as a personal tool — issues and PRs aren't actively managed.
If you're reading the source, the conventions are:

- Commits go straight to `main` (no PRs — [ADR&nbsp;0005](docs/adr/0005-push-to-main-no-prs.md)).
- `go vet`, `go test ./...`, and `staticcheck ./...` must be clean before push.
- Code style is enforced by the pre-commit hook; see [STYLE.md](./STYLE.md).
- Personal notes (`memory/courses/*`, `memory/thesis/*`) are gitignored — sensitive, not source.

## License

[MIT](./LICENSE) © 2026 Eduardo Hiroji.
