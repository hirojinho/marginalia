# OSS Distribution — Stage 1 Design

**Date:** 2026-06-17
**Status:** Approved (grilled)
**Branch:** `oss-prep`

## Goal

Make `github.com/hirojinho/claw-study` **publicly presentable and lightly runnable** as
a portfolio / reference piece. This is **Stage 1** of a staged OSS effort:

- **Stage 1 (this spec):** presentation-complete + a ~5-minute "run it against a bundled
  sample corpus" path. A reader who lands on the repo can understand it, read clean code
  without personal cruft, *see* it working (screenshot), and run it if they want.
- **Stage 2 (deferred, not specified here):** broad adopter onboarding — issue templates,
  an open demo, contributor tooling, wider runtime-recipe coverage.

### Out of scope for Stage 1

Changing runtime behavior, refactoring the app, the gated production demo, contributor
tooling, and any Stage 2 onboarding work.

## Decisions (resolved during grilling)

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Distribution goal | Portfolio now, adopters later (staged) | Repo already pitches "the architecture is the part worth a look"; demo is deliberately gated. |
| Stage 1 bar | Presentation + light runnability | A reader can see it work *and* run it in ~5 min, without it being a full adopter onramp. |
| Runtime target | OpenAI happy-path + Ollama local alt | OpenAI = universal "paste a key, chat + embeddings work"; Ollama = $0/local. Zen gateway rejected (tied to personal account, may not serve `/embeddings`). |
| Sample corpus | Tiny openly-licensed doc(s), in-repo | Deterministic, zero-setup demo; keeps repo light. |
| License | MIT | Maximally permissive; no SaaS moat to protect, so AGPL's copyleft only deters the adopters/employers Stage 2 wants. |
| Cleanliness philosophy | Surgical scrub, keep process artifacts | `specs/done/`, `docs/adr/`, `docs/postmortem/`, `skills/` ARE the "operated by an AI agent" showcase — keep them; remove only genuinely personal/machine-local files. |
| Work location | Fresh clone on the Mac | Dev box has a browser (screenshot); leaves the ThinkPad worktree + live `study-app.service` untouched. |
| Pipeline coordination | `oss-prep` branch, merge at end | Overnight pipeline keeps owning `main`; we rebase once and merge. No mid-edit collisions. |

## Worklist (Stage 1 deliverables)

### Legal & hygiene
1. **Add `LICENSE`** — MIT, `Copyright (c) 2026 Eduardo Hiroji`. Fix the README License
   section to match (the TOC already links it).
2. **Untrack `CLAUDE.local.md`** — `git rm --cached`, add to `.gitignore`. Machine-local file.
3. **Genericize `seed-memory/main.go`** — drop the hardcoded
   `$HOME/.claude/projects/-Users-eduardohiroji-…/memory` default; default `-source` to
   `./memory` (non-breaking, tool still runs). Keep the tool.
4. **Personal-leakage grep pass** — sweep `CLAUDE.md`, `AGENTS.md`,
   `memory/study-context.md`, and docs for `hiroji`, real name, `claw-study.xyz`,
   `192.168.*`, `/home/hiro`, `/Users/eduardo*`; genericize hits. Keep
   specs/ADRs/postmortems/skills.

### Runnability (the 5-minute path)
5. **Bundle `examples/sample-corpus/`** — one or two tiny openly-licensed docs
   (CC/public-domain PDF or markdown), with a one-line provenance/license note.
6. **Document two recipes** in README "Running locally":
   - **OpenAI happy-path** — chat + embeddings, paste `LLM_API_KEY`, point `VAULT_ROOT`
     at the sample corpus.
   - **Ollama local alt** — zero-cost, fully local (chat model + `nomic-embed-text`).
   Verify both actually work.

### Presentation
7. **Screenshot/screencast** — run locally against the sample corpus, capture chat + plan
   + PDF viewer, drop into `docs/assets/`, embed in README (resolves the existing TODO).
8. **README/docs coherence pass** — no broken links a reader clicks; pitch matches reality.

## Sequencing

1. Clone on the Mac (`…/projects/claw-study`), branch `oss-prep` off `main`. *(done)*
2. Items 1–4 (legal/hygiene) — fast, no running needed.
3. Item 5 — pick + commit the sample corpus.
4. Item 6 — run locally against the sample corpus with an OpenAI key to **verify** the
   happy path; capture item 7 (screenshot) from that same instance.
5. Item 8 (coherence pass), then self-review.
6. Merge `oss-prep` → `main`, rebasing once if the overnight pipeline landed commits. Push.

## Verification gates

- `go build ./...` succeeds.
- The 5-minute OpenAI recipe genuinely indexes the sample corpus and answers a question
  (behavioral, not just "it compiles").
- `git ls-files` shows no personal paths and `CLAUDE.local.md` is no longer tracked.

## Dependencies / inputs needed

- An OpenAI API key (or OpenAI-compatible endpoint) to verify the recipe and capture the
  screenshot — needed at step 4, can be held until then.
