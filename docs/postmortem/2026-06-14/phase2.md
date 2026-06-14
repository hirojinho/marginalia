## Failure 1: Go syntax error in `claw-cli/main.go`

### Proximate error

From gate.log:
```
claw-cli/main.go:1716:28: syntax error: unexpected %, expected expression
```

Offending source line from the diff (in `runProbeShow`):

```go
learnerOut = fmt.Sprintf(%q, learnerAnswer)
```

`%q` is a bare token, not a string literal — Go parses it as an expression identifier, which is invalid as the first argument to `fmt.Sprintf`.

### Precise fix

```go
-               learnerOut = fmt.Sprintf(%q, learnerAnswer)
+               learnerOut = fmt.Sprintf("%q", learnerAnswer)
```

Wrap the format verb in quotes so it is a `string` literal.

---

## Failure 2: `TestRule6OneLightOpener` FAIL in package `agent`

### Proximate error

From gate.log:
```
sandbox_test.go:388: Rule 6 (ADR 0020) missing "ONE light move"
```

### Why it fails

The test (pre-existing on `origin/main`; not touched by r9's diff — the diff only touches `agent/sandbox.go`, `agent/llm.go`, and `claw-cli/main.go`) asserts four required substrings must appear in the output of `studyTuningSections`:

| `want` string | Main text (passes) | r9's new text (fails) |
|---|---|---|
| `"ONE light move"` | ✅ `"Session-open — ONE light move, then read"` | ❌ r9 rewrote the header to `"Session-open retrieval with practice-testing probes"` — phrase deleted |
| `"understanding-first"` | ✅ | ✅ preserved |
| `"give me your own example"` | ✅ | ✅ preserved |
| `"the prediction IS your opener"` | ✅ | ❌ r9 expanded sections (b)/(c) and removed this exact fragment |

Additionally, the test has a retirement guard:

```go
if strings.Contains(out, "does NOT license skipping") {
    t.Fatalf("retired forced-recall-on-empty-queue wording still present")
}
```

r9's new text **introduces** this forbidden phrase in clause (b):
> *"An empty queue is the normal case …; it does NOT license skipping the recall."*

So r9's diff broke the test in **three** ways: removed two required guard strings and added a prohibited string. It regressed pre-existing text that was guarding ADR 0020 invariants.

### Precise fix

Restore the main `rule6` text verbatim (it already passes the test), then add the probe-tool instructions *without* removing the guard phrases. The minimal surgical fix to r9's text — showing the three affected lines — is:

**1. Restore `"ONE light move"` in the header:**

```go
-	rule6 := "6. **Session-open retrieval with practice-testing probes (MANDATORY).** Before answering anything else, …
+	rule6 := "6. **Session-open — ONE light move, then read (MANDATORY).** Before answering anything else, …
```

**2. Replace the offending `"does NOT license skipping"` clause with the original `"the prediction IS your opener"` wording:**

```go
-		"**(b)** If nothing is due BUT a task has been completed → you MUST still open with 2–3 targeted short-answer questions… An empty queue is the normal case (SM-2 future-dates fresh items); it does NOT license skipping the recall. …
+		"**(b)** If nothing is due (the normal case — SM-2 future-dates fresh atoms) → go straight to the Rule 7 pre-read prediction; the prediction IS your opener — do NOT also run a recall round. …
```

(Then merge r9's new `**(c)**` clause after it, or fold the task-completed case in separately.)

---

## Root cause

**EXECUTOR** — the coding agent shipped two independent errors: a bare `%q` typo in `fmt.Sprintf` (compilation trivially fails), and a full Rule 6 rewrite that discarded two ADR 0020 guard strings and introduced a prohibited phrase it would have caught by running `go test ./agent/...` before committing.

---

## Minimal fix

| Failure | File | Change |
|---|---|---|
| Syntax error | `claw-cli/main.go:~1716` | `fmt.Sprintf(%q, learnerAnswer)` → `fmt.Sprintf("%q", learnerAnswer)` |
| TestRule6OneLightOpener | `agent/sandbox.go:~236` | Restore "ONE light move" in header; replace "does NOT license skipping" clause with "the prediction IS your opener" wording; preserve r9's probe instructions around the invariant phrases |

---

## Is it bounded?

**Shippable tonight.** Both fixes are single-line (one literal quote wrapper) or single-paragraph (reverting to the known-good Rule 6 phrasing and weaving the probe instructions alongside). No ADR is unimplemented — the probe tools (`GenerateProbeQuestion`, `claw-cli probe`, `LogProbe`) are complete and correct in the diff; only the presentation text regressed a test invariant.

---

# Claude Code correction — root cause is SPEC, not EXECUTOR (verified against r9 spec)

Pi classified EXECUTOR. WRONG. Verified against the orphaned r9 spec (on branch `specs/failed/`):
- **Syntax error (claw-cli/main.go:1716)** — genuine EXECUTOR slip: `fmt.Sprintf(%q, …)` missing quotes. (The spec's own code blocks show `%q` correctly inside strings elsewhere, e.g. line 162 `"…:%q}", probeID, question` — so Pi mistyped this one.) Fix: `"%q"`. Uncontroversial.
- **TestRule6OneLightOpener** — NOT an accidental regression. The spec (line 338) ORDERS: *"Modify Rule 6 … Replace it with a structured probe flow"*, and dictates the exact new text (line 349/367) that r9 implemented verbatim. The new text drops `"ONE light move"` + `"the prediction IS your opener"` and adds the test-forbidden `"does NOT license skipping"`. **The executor obeyed the spec.** The spec contradicts ADR-0020 (which the test guards) AND its Task 4 never updated the guard test. → root cause = **SPEC** (spec↔ADR conflict, unreconciled). This is the [[claw-study-pipeline-drift-rootcause]] genre, live.

## The unresolved conflict (needs a human pedagogy decision)
- **ADR 0020 / current main:** empty retrieval queue → "the prediction IS your opener", do NOT also run a recall round ("ONE light move").
- **r9 spec / new text:** if a task was completed, "you MUST still open with 2–3 … questions … empty queue … does NOT license skipping the recall."
These are contradictory pedagogies. The syntax fix is safe to apply now; the Rule-6 conflict is NOT auto-fixable — it's a decision about how the study tool should behave.
