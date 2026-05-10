# claw-study

Personal study app — Go HTTP server with embedded SPA frontend, SQLite for sessions and PDF metadata, RAG over a local course corpus, and an LLM chat that calls tools to read plans, take notes, and toggle progress.

Public URL: `https://study.claw-study.xyz` (gated by bearer token).

## Architecture

- `main.go` — entry point; loads config, opens DB, builds `agent.App`, registers routes, starts the HTTP server with timeouts and graceful shutdown.
- `agent/` — domain logic. `App` owns the DB connection and config. Submodules: `db.go` (sqlite, WAL+busy_timeout+foreign_keys+synchronous=NORMAL pragmas), `llm.go` (OpenAI-compatible chat client), `tools.go` (the function-call tools the LLM can invoke), `vectorstore.go` + `chunker.go` (corpus indexing and cosine-similarity retrieval), `embed.go`.
- `handler/` — HTTP layer. Each domain (sessions, plan, pdf, static, auth) has its own file. All handlers hang off `*Handler` which carries `*App`, `*LLMClient`, and the embedded static FS.
- `static/` — single-page frontend (HTML + inline CSS/JS, plus pdf.js for the viewer).
- `convert/` — separate binary for one-off corpus conversion.

The runtime state lives under `VAULT_ROOT`:
```
$VAULT_ROOT/
├── data/
│   ├── study.db          # sessions, messages, plan toggles, PDF metadata
│   ├── corpus/           # source markdowns indexed for RAG
│   ├── pdf-files/        # uploaded PDFs
│   ├── pdf-texts/        # extracted text per PDF
│   └── plans/<id>.json   # per-course plans
└── memory/               # study notes, runbooks, project context
```

## Running locally

Requires Go ≥ 1.24 (for `github.com/ledongthuc/pdf`).

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

```bash
go test ./...
```

Coverage focuses on pure functions (`agent/chunker`, `agent/vectorstore`, `agent/embed`, plan/db helpers) and HTTP handlers (`handler/*_test.go`) using `httptest` against in-memory SQLite. Auth middleware has its own table-driven tests in `handler/auth_test.go`.

## Deploying

The app is shipped as a single Linux/amd64 binary, scp'd to the VPS, and managed by user systemd. Both the app and its named cloudflared tunnel run as `study-app.service` and `study-app-tunnel.service` under the `eduardo` user.

```bash
GOOS=linux GOARCH=amd64 go build -o /tmp/study-app-linux .
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

## Specs and design docs

| File | Subject |
|---|---|
| `SPEC-ARCHITECTURE.md` | Top-level architecture and dependency boundaries. |
| `SPEC-RAG-SEARCH.md` | Vector-store layout and retrieval flow. |
| `SPEC-PDF-VIEWER.md` | Embedded pdf.js viewer + annotation persistence. |
| `SPEC-TOOLS.md` | Function-call tools the LLM can invoke. |
| `SPEC-PHASE2.1.md`, `SPEC-PHASE2.2.md` | Architecture milestones during the App-struct refactor. |
| `DESIGN.md` | Older overall design notes. |
| `IMPLEMENTATION-PLAN.md`, `PLAN-RAG-IMPLEMENTATION.md` | Step-by-step plans (historical). |

## Stack

Go 1.24+, `database/sql` + `mattn/go-sqlite3`, `ledongthuc/pdf`, embedded `static/`, `slog` for structured logs. Frontend is plain HTML/JS plus `pdf.js`. Single binary, no Node build step.

## Repo conventions

- Commit straight to `main` (no PRs).
- `go vet` clean and `go test ./...` passing before push.
- Personal notes (`memory/courses/*`, `memory/thesis/*`) are gitignored — sensitive, not source.
