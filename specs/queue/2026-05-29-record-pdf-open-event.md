---
id: 2026-05-29-record-pdf-open-event
title: Record a pdf_open event when a PDF is opened for viewing
estimated_complexity: small
max_wall_clock_minutes: 30
max_diff_lines: 150
max_retries: 1
max_tokens: 100000
requires_visual_approval: false
allow_web_search: false
created_at: 2026-05-29
created_by: laptop-cc + eduardo
---

## Goal

Record a `pdf_open` event whenever a PDF is **opened for viewing**, not only when it is uploaded. **Why:** today `pdf_open` is emitted in exactly one place — the *upload* handler (`handler/pdf.go`, in `handlePDFUpload`). So re-opening an already-uploaded PDF records nothing, and the `PDF Opens` metric (`/debug/metrics`) plus the overnight digest are blind to all real reading. Empirically the learner reads heavily in-app (one PDF is on page 72, another on page 258/611) yet the metrics show ~2 opens ever. This ticket adds a dedicated open endpoint that the viewer calls once per open, so reading becomes visible to the metrics/observability layer.

This is the minimal, deterministically-verifiable backend slice of the larger reading-experience work (see ADR 0012); it does not touch the tutor prompt or the PDF↔session linkage (those are separate, non-pipeline work).

## References

- `handler/pdf.go` — `handlePDFUpload` already records the canonical `pdf_open` event (search for `Kind:      "pdf_open"`). Match that `RecordEvent` block exactly for session/course attribution.
- `handler/pdf.go` — `handlePDFFile` shows the existing `/pdf/file/` path-suffix parsing pattern (`parseInt64(pathSuffix(r.URL.Path, "/pdf/file/"), "id")`) and the `methodNotAllowed` guard. Mirror it.
- `handler/handler.go` lines ~55–61 — the `/pdf/*` route registration block. The new route goes here.
- `agent/db.go` — `GetPDF(id)` returns the PDF record (including `CourseID`); `RecordEvent(agent.Event{...})` inserts one event. `QueryEventSummary` populates `.PDFOpens` from `kind='pdf_open'`, which is what `/debug/metrics` renders under the `PDF Opens` heading.
- `static/pdf.js` — `openPdf(id)` (around line 148) is the single once-per-open frontend function. Do NOT hook `/pdf/file/` for the event: `http.ServeFile` serves HTTP range requests, so `/pdf/file/` is hit many times per open and would massively over-count.

## Implementation plan

1. **Add `handlePDFOpenEvent` to `handler/pdf.go`.** Signature: `func (h *Handler) handlePDFOpenEvent(w http.ResponseWriter, r *http.Request)`.
   - Guard method: `if methodNotAllowed(w, r, http.MethodPost) { return }`.
   - Parse the id: `id, err := parseInt64(pathSuffix(r.URL.Path, "/pdf/open/"), "id")`; on error `writeError(w, http.StatusBadRequest, err.Error()); return`.
   - Look up the PDF for course attribution and existence: `pdf, err := h.App.GetPDF(id)`; on error `http.Error(w, "not found", http.StatusNotFound); return`.
   - Resolve the active session exactly like `handlePDFUpload` does:
     ```go
     activeSess := h.App.ActiveSessionID()
     var sessPtr *int64
     if activeSess != 0 {
         sessPtr = &activeSess
     }
     ```
   - Record the event (mirror the upload handler's block):
     ```go
     if err := h.App.RecordEvent(agent.Event{
         Kind:      "pdf_open",
         SessionID: sessPtr,
         CourseID:  pdf.CourseID,
         CreatedAt: time.Now().UnixMilli(),
     }); err != nil {
         slog.Warn("record pdf_open event", "err", err)
     }
     writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
     ```
     (If the `GetPDF` return value's course field is named differently than `CourseID`, use the actual field; if there is no readily-available course field, pass `CourseID: ""` — it is optional in `agent.Event`.)

   **Edge cases to handle:**
   - Non-POST method: `methodNotAllowed` returns 405 (handled by the guard above).
   - Unparseable id: 400 via `writeError`.
   - Nonexistent PDF id: 404 via the `GetPDF` error path. Do NOT record an event for a missing PDF.

2. **Register the route** in `handler/handler.go`, in the `/pdf/*` block (right after the `/pdf/last` registration, ~line 61):
   ```go
   mux.HandleFunc("/pdf/open/", h.handlePDFOpenEvent)
   ```
   The trailing slash matters — the id is the path tail.

3. **Call it once per open from the frontend.** In `static/pdf.js`, inside `openPdf(id)` (~line 148), add a fire-and-forget POST near the top of the function (after `id` is known, before/around the existing `/pdf/progress/` PUT). It must NOT block or break opening if it fails:
   ```js
   fetch('/pdf/open/' + id, { method: 'POST' }).catch(() => {});
   ```
   Do not `await` it. Place it so it runs exactly once per `openPdf` call.

4. **Add a unit test** in `handler/pdf_test.go` (create it if absent; follow the setup in the existing `handler/*_test.go` files and `testutil_test.go` for building a test `*Handler` with a temp DB and a seeded PDF row). Assert:
   - `POST /pdf/open/<existing-id>` returns **200** and body contains `"ok":true`.
   - After that POST, an `events` row with `kind = 'pdf_open'` exists (query the test DB).
   - `GET /pdf/open/<id>` returns **405** (wrong method).
   - `POST /pdf/open/999999` (nonexistent) returns **404** and records **no** new `pdf_open` event.

   **Computed expectation (don't make the model count):** seed exactly one PDF and zero pre-existing events; after one successful `POST /pdf/open/<id>`, the `pdf_open` event count is `1`.

## Verification recipe

The same script is used for both phases: it asserts the open endpoint exists and responds correctly, so it FAILS on current main (the endpoint 404s) and PASSES on staging after implementation. HTTP only — no DB-path coupling — so it runs identically against prod (red step) and staging (gate).

> The endpoint's **side effect** (a `pdf_open` event row is inserted) is proven by the unit test in step 4 of the plan, which runs in the gate's `go test ./...` step. We do **not** verify the event via `/debug/metrics` here because that endpoint is currently broken on prod (a separate pre-existing bug: `QueryEventSummary` scans `COALESCE(AVG(duration_ms),0)` — a float — into an int64 and 500s). Do not depend on `/debug/metrics` in this verifier.

### Pre-baseline (must FAIL on current main)

```bash
set -uo pipefail
: "${STAGING_URL:?STAGING_URL required}"
: "${STAGING_TOKEN:?STAGING_TOKEN required}"
AUTH="Authorization: Bearer ${STAGING_TOKEN}"
PDF_ID=1

resp=$(curl -s -H "$AUTH" -X POST "${STAGING_URL}/pdf/open/${PDF_ID}" -w $'\n%{http_code}')
code=$(printf '%s' "$resp" | tail -n1)
body=$(printf '%s' "$resp" | sed '$d')

if [ "$code" != "200" ]; then
  echo "FAIL: POST /pdf/open/${PDF_ID} returned ${code}, want 200"
  exit 1
fi
case "$body" in
  *'"ok":true'*) echo "OK: POST /pdf/open/${PDF_ID} -> 200 ok:true"; exit 0 ;;
  *) echo "FAIL: 200 but body missing \"ok\":true -> ${body}"; exit 1 ;;
esac
```

### Post-acceptance (must PASS after implementation)

Identical to the pre-baseline script above. Against staging with the new binary it must exit 0.

### Human-eyeball notes

- Not part of the gate: open a PDF in the browser; confirm in the `events` table that exactly one new `pdf_open` row appears per open (and NOT one per page turn — only once per `openPdf`). `/debug/metrics`'s `PDF Opens` count will reflect these once the separate metrics-500 bug is fixed.
- `PDF_ID=1` (the DDIA PDF) exists in both prod and the staging DB copy. If a future DB reset removes it, point `PDF_ID` at any row from `/pdf/list`.

## Done criteria

- [ ] `POST /pdf/open/{id}` route registered and handled; returns 200 `{"ok":true}` for an existing PDF.
- [ ] A `pdf_open` event row is recorded on open (proven by the unit test; will surface in `/debug/metrics` `PDF Opens` once the separate metrics-500 bug is fixed).
- [ ] `openPdf()` in `static/pdf.js` fires the POST once per open, non-blocking.
- [ ] 405 on GET, 400 on bad id, 404 on missing PDF; no event recorded for missing PDF.
- [ ] Unit test in `handler/pdf_test.go` covers 200 + event-recorded, 405, and 404-no-event.
- [ ] `go build .` and `go test ./...` green; verifier exits 0 against staging.

## Rollback notes

No schema change, no migration. Pure additive: a new route + handler + one frontend call. `git revert <sha>` + binary redeploy fully reverts; no data cleanup needed (extra `pdf_open` events are harmless observability rows).
