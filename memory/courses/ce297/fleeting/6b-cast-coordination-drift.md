# 6B Fleeting — CAST Coordination & Drift (Steps 4–5)

**Source:** Leveson EASW Ch. 11 §§11.8–11.9 (pp. 378–383)
**Read:** 2026-04-25
**Consolidates into:** Block 6D (forward/backward γ pair; drift as the case requiring temporal modal logic over coalgebraic paths)
**Status:** raw inputs, not formal notes
**Companion:** `6b-cast-mechanics.md` (§§11.2–11.6, Steps 1–3)

---

## §11.8 — Coordination and Communication (slide Step 4)

### 1. Leveson's coordination failure taxonomy vs. your `sec:coordination-failures`

> _Block 5A established `sec:coordination-failures` (formal_notes.tex line 2563) as the formal-side vocabulary. §11.8 is the CAST application of the same idea. The orientation question is whether Leveson's CAST checklist matches that taxonomy 1-1, extends it, or carves it differently. Capture each failure mode she names — verbatim or tightest paraphrase — and tag it as `match` / `addition` / `re-framing` against what's already in formal_notes._

**Failure modes Leveson names (verbatim or paraphrase, with page):**

| Mode | Phrasing | Page | Tag (match/addition/re-frame) |
|------|----------|------|-------------------------------|
|      |          |      |                               |
|      |          |      |                               |
|      |          |      |                               |

**Net result for `sec:coordination-failures`:**

---

### 2. Coordination-as-handoff vs. coordination-as-shared-mental-model

> _Two distinct flavors of coordination failure live in the literature: discrete handoff failures (A passes to B; B drops it) and continuous shared-model failures (A and B disagree about what the joint state is). Does §11.8 distinguish them? If yes, capture her terms and which she emphasizes. If no, that's itself worth noting — it means CAST collapses the distinction at this level, and the coalgebraic frame in 6D needs to make the choice explicit._

**Distinction made? [ yes / no / partial ]**

**Her terms (if any):**

**Page:**

**If collapsed — what does she lose, and is that a feature or a gap?**

---

### 3. The "fault lives nowhere in any single controller" case

> _The hardest CAST move is naming a failure that cannot be localized to one γ_i. §11.8 should contain at least one example where the answer to "whose process model was wrong?" is "nobody's, individually — but their joint behavior violated the constraint." This is the pointwise/joint asymmetry that the per-component template from §§11.5–11.6 cannot resolve. If §11.8 has such an example, it's the cleanest course-side evidence for the interaction-functor move in 6D. If not, §11.9 has to carry that weight alone._

**Example present?**

**If yes — case + page:**

**Why it can't be localized to a single γ_i (her words):**

---

### 4. Leplat two-geometry tie-in (carry from 5A)

> _Part 5A's note on Leplat: geometric (designed) vs. functional (operated) topology of the control structure. Coordination failures are precisely the points where the two geometries don't align. Does §11.8 use vocabulary that maps cleanly onto this — explicit reference to design-vs-actual structure, missing channels, channels used differently than designed? Capture phrases that align._

**Aligned phrases (with page):**

**Implication for 6D — does Leveson supply the design/operated distinction in CAST terms, or do you still need to coin it?**

---

## §11.9 — Dynamics and Migration to a High-Risk State (slide Step 5)

### 5. Verbatim phrasing — drift, migration, high-risk state

> _§11.9 is the load-bearing section for 6D. Before reading anything *into* it, capture her exact vocabulary. The 6D subsection should adopt her terms wherever they fit; γ\* / γ are your coinages and should only be deployed where Leveson leaves a gap. Specifically: how does she name the *process* (drift / migration / decay / erosion / normalization), the *destination* (high-risk state / unsafe region / hazardous configuration), and the *driver* (pressure / efficiency / production goals)?_

**Process — her phrasing(s):**

**Destination — her phrasing:**

**Driver — her phrasing:**

**Pages:**

---

### 6. The named *mechanism* of migration

> _The interesting question is whether §11.9 advances a single mechanism or catalogs several. Common candidates from the broader literature: (a) Rasmussen's pressure-toward-the-boundary; (b) Vaughan's normalization of deviance; (c) erosion of safety constraints under production demands; (d) loss of feedback / observability over time; (e) adaptation of the safety control structure faster than the hazard model. Which of these does she name, and does she cite who? The answer determines what the coalgebraic frame in 6D needs to capture._

**Mechanism(s) Leveson names:**

**Citations (Rasmussen / Vaughan / others):**

**Page(s):**

**Implication for 6D — single-driver or multi-driver model needed?**

---

### 7. **The load-bearing slot — chain-reconstruction failure case**

> _This is the slot 6D depends on most. The thesis claim is that drift is the case where pointwise per-controller analysis (the §§11.5–11.6 template) breaks: no single γ_i has a node where the observed transition departs from γ\*_i, yet the joint trajectory exits the safety region. For this claim to land in 6D, you need at least one Leveson example where she shows — explicitly or implicitly — that walking back the chain doesn't find a fault, and that this is the point. Without a concrete case, 6D's modal-logic hook (eventually-always over coalgebraic paths) is unsupported decoration. **If §11.9 doesn't supply such a case, flag it here so 6D can recruit one from §§11.10's recommendations or from 6C's hospital case.**_

**Case present in §11.9? [ yes / no / partial ]**

**If yes — case + page + her words on why chain reconstruction fails:**

**If no — what does she offer instead, and which §11.10 / 6C case can stand in?**

---

### 8. Connection to existing `sec:drift` (formal_notes.tex line 1569)

> _`sec:drift` already formalizes safety margin drift via the metric on S_ext(t) (line 596). The orientation question is whether §11.9's prose-level account adds anything formalizable that `sec:drift` is missing — e.g., the role of the controller's process model in the drift, the regulatory layer, multi-controller coupling. If yes, 6D should extend `sec:drift` rather than just citing it. If no, 6D just cites it and moves on._

**Does §11.9 add formalizable content beyond `sec:drift`?**

**Specifically — what's new:**

**Implication for 6D — extend `sec:drift` or just cross-reference?**

---

### 9. The "did anything surprise me" slot

> _Same slot as in `6b-cast-mechanics.md` §5: one paragraph, raw, on whatever in §§11.8–11.9 felt unexpectedly sharp or unexpectedly thin. The thing you'd otherwise lose between now and 6D._

>

---

## 10. Margin notes for the coalgebraic reading (carry into 6D)

> _Per the plan: §11.8 is the place where coordination failures should naturally factor through a joint-observation functor (one γ_i alone doesn't have access to the joint state); §11.9 is the predicted strain point where pointwise per-controller bisimulation discharge breaks and modal logic must enter. Use this slot to capture the specific examples or framings from §§11.8–11.9 where these readings clicked or strained. Strained cases > clicked cases — they tell 6D what it has to address rather than paper over._

**§11.8 — joint-observation functor reading:**

  - Clicked (specific phrase / example):
  - Strained (where pointwise still feels adequate, or where the joint frame feels overkill):

**§11.9 — modal-logic-needed reading:**

  - Clicked (specific phrase / example showing trajectory-level property):
  - Strained (place where Leveson localizes drift to a single controller after all):

**Implication for 6D's structure:**
