Here is the reconciled fix — output only.

### PART A — syntax error in `claw-cli/main.go`

```go
		learnerOut = fmt.Sprintf("%q", learnerAnswer)
```

### PART B — reconciled Rule‑6 block for `agent/sandbox.go`

```go
	rule6 := "6. **Session-open — ONE light move, then read (MANDATORY but brief).** Before anything else, run `claw-cli retrieve due --course <active course>`. Pick exactly ONE opener; do NOT stack phases. **(a)** If items are due → for each due KC (at most 2):\n" +
		"  (i) Read the KC body: `claw-cli knowledge show <kc_id>`\n" +
		"  (ii) Check for an existing cached question: `claw-cli probe show --kc <kc_id>`. If it returns a cached question, use it — skip generation.\n" +
		"  (iii) If no cached question exists, generate ONE understanding-first question from the KC body (ADR 0020): a why/how/when/what-breaks, or \"give me your own example\" — NEVER \"what is X,\" \"list everything,\" or \"what was the example in the material.\" Store it: `claw-cli probe store --kc <kc_id> --question \"<question>\" --expected \"<kc body>\"` (the --expected is the KC body at generation time).\n" +
		"  (iv) Present the question to Eduardo in chat. Ask for his answer.\n" +
		"  (v) When he answers, grade his answer against the expected answer on the SM-2 0–5 scale:\n" +
		"    - 0 = complete blackout — nothing correct or relevant\n" +
		"    - 1 = wrong, but would recognize the correct answer when shown\n" +
		"    - 2 = wrong, but the correct answer seems easy to recall (tip of the tongue)\n" +
		"    - 3 = correct, but with serious difficulty or major gaps\n" +
		"    - 4 = correct, after hesitation or minor gaps\n" +
		"    - 5 = perfect, immediate recall — complete and precise\n" +
		"  (vi) Record the probe: `claw-cli probe record --probe-id <id> --answer \"<his verbatim text>\" --grade <0-5>`\n" +
		"  (vii) Respond CONVERSATIONALLY — do NOT announce the raw 0–5 number (it is recorded silently for scheduling; ADR 0020). Credit the idea in his own words (paraphrase = full credit). At most ONE cue if he is short, then move on:\n" +
		"    - Low (0–2): one cue toward the gap; if still short, fill it in a sentence and move on.\n" +
		"    - Mid (3–4): note the one specific gap conversationally.\n" +
		"    - High (5): affirm briefly. Do not linger.\n" +
		"  (viii) If multiple KCs are due, repeat from (i) for the next one.\n" +
		"**(b)** If nothing is due (the normal case — SM-2 future-dates fresh atoms) → go straight to the Rule 7 pre-read prediction; the prediction IS your opener — do NOT also run a recall round. Never invent a completed task or claim he recalled anything without evidence.\n\n" +
		"Tailor question depth to the bloom_level of the upcoming task (visible in `claw-cli plan status`): remember/understand → key facts, definitions, mechanisms; apply → principles, formulas, procedures; analyze/evaluate → comparative frameworks, trade-offs, evaluation criteria (\"what are the trade-offs between X and Y?\" not \"what is X?\"); create → skip scored recall, the creation is the retrieval. If bloom_level is missing (older plans), default to understand-level. Non-negotiable; highest-evidence pedagogic move (Roediger & Karpicke 2006, testing effect; Endres et al. 2020, targeted short-answer preserves testing effect).\n"
```

**Checklist against the test assertions:**

| Assertion | Present? |
|---|---|
| `ONE light move` | ✓ header |
| `understanding-first` | ✓ step (a)(iii) |
| `give me your own example` | ✓ step (a)(iii) |
| `the prediction IS your opener` | ✓ clause (b) |
| `does NOT license skipping` (must be absent) | ✓ removed |
| Probe-tool loop preserved for clause (a) | ✓ steps (i)–(viii) intact |
| ADR‑0020 empty-queue routing restored for clause (b) | ✓ immediately to Rule 7 pre‑read prediction |
| Bloom‑level paragraph | ✓ r9’s expanded version with Endres et al. 2020 |
| Roediger & Karpicke 2006 citation | ✓ preserved |
