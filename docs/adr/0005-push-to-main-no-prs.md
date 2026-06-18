# 0005 — Push to main directly, no PRs

- **Status:** Accepted
- **Date:** 2026-05-08

## Context

Personal repo, one developer, private GitHub project. PR review provides no second pair of eyes here, and feature-branching adds checkout overhead with no payoff. The deploy path is also direct — laptop builds, scp, restart — no CI gating merges.

## Decision

Commit straight to `main`. No feature branches, no pull requests. Local guardrails: `go vet ./...` clean and `go test ./...` passing before push.

## Consequences

- Minimum-friction workflow: edit, commit, push, deploy.
- No review safety net — bad commits land on `main`. Mitigated by small commits, fast revert (`git revert <sha>` + redeploy from binary backup), and the `marginalia.bak` rollback.
- Branch protection is off; force-push is technically possible. Don't.
- Convention breaks the moment a second contributor shows up — that's also the right time to introduce PRs.
