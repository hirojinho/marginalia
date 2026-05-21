---
id: 2026-05-21-debug-version-endpoint
title: Add /debug/version endpoint returning build commit + timestamp
estimated_complexity: small
max_wall_clock_minutes: 30
max_diff_lines: 150
max_retries: 1
max_tokens: 100000
requires_visual_approval: false
allow_web_search: false
created_at: 2026-05-21
created_by: laptop-cc + eduardo
---

## Goal

Add a `GET /debug/version` endpoint that returns the build's git commit SHA and build timestamp as JSON. **Why:** this is ticket 1 of the overnight pipeline's R2 vertical-slice validation — its job is to be the smallest possible real change that exercises the full pipeline end-to-end (spec → Pi implementation → gate → deploy → rollback drill). The endpoint itself is also genuinely useful: it lets the morning digest include the deployed commit, and lets manual smoke-tests confirm "which version is running" without SSH.

## References

- Existing handler patterns: `handler/debug.go` already implements `/debug/health` (returns 200). Match its style.
- AuthMiddleware gates all routes except `/login`. `/debug/version` is gated (must accept bearer token). This is intentional — version info is not for unauthenticated callers.
- Build-time injection of version via Go `-ldflags`: standard pattern, see https://pkg.go.dev/cmd/link (`-X importpath.name=value`).

## Implementation plan

1. **Declare two package-level variables in `main.go`** that ldflags will populate at build time:
   ```go
   var (
       buildCommit    = "unknown"
       buildTimestamp = "unknown"
   )
   ```
   Place these in `main.go` near the top, before `func main()`.

2. **Add a `versionHandler` method on `*Handler` in `handler/debug.go`.** Signature: `func (h *Handler) versionHandler(w http.ResponseWriter, r *http.Request)`. Returns JSON `{"commit": "<sha>", "built_at": "<iso8601>"}` with `Content-Type: application/json`. Reads from package-level vars in main; expose them via the existing pattern (either pass through Handler struct on construction, or use a small accessor — match what claw-study already does for build config).

3. **Register the route** in the routes registration block (find where `/debug/health` is registered). Add `/debug/version` next to it, also gated by AuthMiddleware (which wraps the whole mux per claw_study_service.md).

4. **Add unit test in `handler/debug_test.go`** asserting:
   - 200 status code with valid bearer token
   - Response body has `commit` and `built_at` fields
   - `commit` length is 40 chars (full SHA) when build vars are set; "unknown" otherwise (test the "set" case by injecting via the handler under test)
   - Missing/wrong token returns 401 (re-uses existing auth-middleware test pattern)

5. **Update the build command** in any build documentation to include `-ldflags`:
   ```
   GOOS=linux GOARCH=amd64 /opt/homebrew/bin/go build \
     -ldflags "-X main.buildCommit=$(git rev-parse HEAD) -X main.buildTimestamp=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
     -o /tmp/study-app-linux .
   ```
   The gate-runner.sh from the implementation plan must use these flags too — surface this in the gate-runner spec.

## Verification recipe

### Pre-baseline (must FAIL on current main)

```bash
set -euo pipefail

# Assumes STAGING_URL + STAGING_TOKEN are set by gate-runner; if running manually
# against prod, set them to the prod URL + AUTH_TOKEN.
: "${STAGING_URL:?STAGING_URL required}"
: "${STAGING_TOKEN:?STAGING_TOKEN required}"

# Expect 404 on current main (endpoint doesn't exist yet).
status=$(curl -s -o /dev/null -w "%{http_code}" \
  -H "Authorization: Bearer $STAGING_TOKEN" \
  "$STAGING_URL/debug/version")

if [ "$status" = "404" ]; then
  echo "OK: /debug/version returns 404 on current main (as expected)"
  exit 0
else
  echo "FAIL: expected 404, got $status — verifier or current main is wrong"
  exit 1
fi
```

**Note for Pi runner:** the gate inverts this exit code. Pre-baseline that exits 0 here means "current main does NOT have the feature, all good to proceed". Pi's gate-runner step 2 actually treats THIS exit 0 as the desired pre-baseline state (status=404 means the feature is missing as expected). If the script exits 1, the verifier is structurally wrong or the feature is already there — both are spec-author errors and Pi exits 10.

Actually clarification: the gate-runner contract from the impl plan is "pre-baseline must FAIL on current main (non-zero exit)". To honor that, this script must exit NON-ZERO when the endpoint is missing (current main state). Inverting the logic:

```bash
set -euo pipefail
: "${STAGING_URL:?STAGING_URL required}"
: "${STAGING_TOKEN:?STAGING_TOKEN required}"

# /debug/version should return 200 with valid JSON. On current main it returns 404.
# This script asserts the endpoint works correctly; it should FAIL on current main.

response=$(curl -sf -H "Authorization: Bearer $STAGING_TOKEN" "$STAGING_URL/debug/version")
commit_len=$(echo "$response" | jq -r '.commit | length')
[ "$commit_len" -eq 40 ] || { echo "FAIL: commit not 40 chars"; exit 1; }
echo "$response" | jq -e '.built_at | test("^[0-9]{4}-[0-9]{2}-[0-9]{2}T")' > /dev/null \
  || { echo "FAIL: built_at not ISO8601"; exit 1; }
echo "OK"
```

This is the **single canonical verifier**; pre-baseline runs it against current-main staging (expects exit 1 because `curl -sf` 404s), post-acceptance runs it against new-binary staging (expects exit 0).

### Post-acceptance (must PASS after Pi's implementation)

**Same script as above.** This is by design — one verifier, two contexts. Pi's gate-runner runs it twice with different binaries; we don't author two scripts that could drift from each other.

### Human-eyeball notes (NOT part of the gate)

- After deploy, manually `curl https://study.claw-study.xyz/debug/version` with the prod token and confirm the `commit` matches `git rev-parse origin/main` on the laptop. This is part of Phase 7 acceptance in the impl plan; it's redundant with the gate but builds your confidence in the pipeline's first run.

## Done criteria

- [ ] `handler/debug_test.go` includes `TestVersionHandler` and it passes locally
- [ ] `main.go` declares `buildCommit` + `buildTimestamp` package-level vars
- [ ] Route `/debug/version` registered alongside `/debug/health`, gated by AuthMiddleware
- [ ] Gate-runner build invocation uses `-ldflags` to inject real values (verifier requires 40-char commit)
- [ ] All existing tests still pass
- [ ] Diff stays under 150 lines (per frontmatter cap)
- [ ] After deploy, prod `/debug/version` returns 200 with `commit` matching HEAD
- [ ] Rollback drill (Phase 7.7 in impl plan) reverts cleanly: post-rollback `/debug/version` returns 404

## Rollback notes

No data migration. Binary swap + git revert is sufficient — the only state change is two new package-level vars and one new route, all undone by reverting the commit.
