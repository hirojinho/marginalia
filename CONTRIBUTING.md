# Contributing to marginalia

marginalia is a personal project opened for reference, learning, and adaptation.
Contributions that improve the core product are welcome — especially bug fixes,
documentation, and runtime-recipe additions.

## Dev setup

**Prerequisites:** Go ≥ 1.24, Node ≥ 18 (for JS linting only — the app has no
Node runtime dependency).

```bash
git clone https://github.com/hirojinho/marginalia.git
cd marginalia

# Optional: install lint tooling
brew install golangci-lint    # or go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
npm install                    # ESLint + Prettier for the frontend

# Install the pre-commit hook (runs gofmt, golangci-lint, eslint, prettier)
bash scripts/install-hooks.sh
```

Build and test:

```bash
go build ./...     # builds marginalia, claw-cli, seed-memory, and convert
go test ./...      # handler, agent, claw-cli, and seed-memory test suites
```

## Style guide

All code follows [`STYLE.md`](STYLE.md) — read it before writing anything. The
pre-commit hook enforces the mechanically-checkable subset; conceptual rules
(file-header invariants, verb-noun error wraps, decomposition judgement) live in
prose and are enforced by reviewer attention.

Go specifics: [`docs/style/go.md`](docs/style/go.md).
JS specifics: [`docs/style/js.md`](docs/style/js.md).

## How changes land

marginalia follows [ADR 0005](docs/adr/0005-push-to-main-no-prs.md): **push to
main, no PRs.** This is a solo project — the workflow is:

1. Fork or branch from `main`
2. Make your change, keeping commits small and focused
3. Open a PR from your branch (or fork) to `main`
4. The PR is reviewed and merged

Keep PRs small. A PR that changes 300 lines across 20 files is hard to review;
one that changes 50 lines in 2 files is easy.

## Project structure

| Directory | Purpose |
|-----------|---------|
| `agent/` | Core logic: DB, tools, sandbox, embeddings, vector store, SM-2 scheduler |
| `handler/` | HTTP handlers: chat-v2 (Pi agent SSE), sessions, plans, PDFs, auth |
| `static/` | Vanilla JS SPA (ES modules, no framework) |
| `claw-cli/` | CLI the agent drives (`plan status`, `pdf extract`, `retrieve due`, …) |
| `convert/` | Corpus converter (markdown → chunks for RAG indexing) |
| `seed-memory/` | One-shot tool to seed agent memory from a markdown directory |
| `skills/` | Pi agent skills (pedagogy rules, study workflow) |
| `docs/adr/` | Architecture Decision Records (19 of them) |
| `docs/specs/` | Current-behavior specs (architecture, RAG, PDF viewer, tools) |
| `examples/` | Sample corpus for the 5-minute quickstart |
| `scripts/` | Git hooks, lint scripts |

## Docs

- [`CONTEXT.md`](CONTEXT.md) — the domain glossary. Read this first.
- [`docs/specs/architecture.md`](docs/specs/architecture.md) — how the app is put together.
- [`docs/adr/`](docs/adr/) — why decisions were made.

## License

MIT — see [`LICENSE`](LICENSE). By contributing, you agree your contributions
will be licensed under the same terms.
