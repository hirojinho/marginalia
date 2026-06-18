# 0003 — No Docker; ship a static binary under systemd

- **Status:** Accepted
- **Date:** 2026-05-10

## Context

Phase 5 of the stability plan called for a `Dockerfile` + `docker-compose.yml` to make builds and deploys reproducible. Reviewing the actual deploy story: app is a single Linux/amd64 Go binary with `embed.FS`-bundled static assets, runs as a `systemd --user` unit on a single VPS, talks to SQLite on a local volume, with a `cloudflared` named tunnel beside it. There is no fleet, no orchestrator, no second host.

## Decision

Reject Docker and Compose. Build the binary on the laptop with `GOOS=linux GOARCH=amd64 go build`, scp it to the server, atomic-swap with the previous version, restart the systemd unit. Operational hardening lives in the boring base layer: journald for logs, restic for nightly snapshots of `/path/to/marginalia`, env vars in mode-0600 `.env`.

## Consequences

- Deploy is two commands (build + scp) and a restart. No image registry, no compose file, no daemon to keep running.
- Rollback is `mv marginalia.bak marginalia && systemctl --user restart` — a literal file swap.
- The binary runs on any modern glibc-compatible Linux without preparing the host. Portability comes from `embed.FS` + static linking, not containers.
- No build reproducibility from a clean room — laptop Go version matters. Mitigated by `go.mod` + `go.sum` pinning.
- Adding a second host or a sidecar process would push back toward containerisation; revisit then.
