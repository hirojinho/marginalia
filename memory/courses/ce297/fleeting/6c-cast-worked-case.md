# 6C Fleeting — CAST Worked Case (Hospital Adverse Events)

**Source:** Leveson, Samost, Dekker, Finkelstein, Raman (2020) "A Systems Approach to Analyzing and Preventing Hospital Adverse Events", *J. Patient Safety* 16(2):162–167. MIT preprint: http://sunnyday.mit.edu/papers/CAST-JPS.pdf
**Read:** 2026-04-25
**Consolidates into:** Block 6D (forward/backward γ pair; this paper supplies the concrete instance the formalization needs)
**Status:** raw inputs, not formal notes

---

## 1. The per-controller template, filled

> _The plan's reading frame: pick one (controller, responsibility, context, process-model flaw) tuple where the paper completes the §11.5–§11.6 template explicitly. This is the concrete instance 6D's γ\*/γ divergence claim needs to point at — without it, the formal section reads as theory-only. Capture controller name, the slot-1 phrasing of responsibility (what γ\*_i was meant to enforce), the slot-2 phrasing of context (state of γ_i at the moment of departure), and the slot-3 named flaw (the specific belief/world divergence). Page numbers if available._

**Controller:**

**Responsibility (what γ\*_i was meant to enforce):**

**Context (state of γ_i at the moment of departure):**

**Process-model flaw (named divergence):**

**Page(s):**

---

## 2. Evidence for naming a process-model flaw

> _Process-model flaws are imputations from observed outcomes — the place CAST is most exposed to hindsight bias (§11.7). For each named flaw, the authors are doing one of: (a) triangulating from interviews, training docs, prior near-misses, or institutional artifacts; (b) inferring backward from the bad outcome alone (post-hoc); (c) something else. Capture the dominant pattern and one or two concrete instances where the inference chain was visible. The contrast between (a) and (b) is what 6D needs to flag — process-model-flaw imputation is judgment-laden in a way forward STPA is not (per `6b-cast-mechanics.md` slot 5)._

**Dominant inference pattern:**

**Triangulation cases (a):**

**Post-hoc cases (b):**

**Implication for 6D's claim that backward reasoning is judgment-laden:**

---

## 3. The aggregation move (n=30)

> _Textbook CAST analyzes one trajectory σ. This paper aggregates 30 cardiovascular surgery adverse events into a population-level analysis. This is methodologically novel — what is the abstracted γ\*? Are they reading a single γ\* from 30 trajectories (an averaged designed control structure), or are they identifying recurring γ_i flaws across 30 distinct γ\*s? Capture how the aggregation is justified (or whether the move is silent). This matters for 6D because the coalgebraic frame defaults to one γ\* per system; population aggregation needs a separate move._

**What is the abstracted γ\*?**

**How is the aggregation justified (verbatim or paraphrase):**

**Implication for 6D (does the coalgebraic frame need a population-level extension, or is the move bracketable):**

---

## 4. Regress-stopping rule

> _§11.7 demands every CAST analysis name where it stops climbing the control hierarchy. Where do these authors stop — at the surgical team, the unit, the hospital admin, the regulator, the legal/cultural layer? And what reason do they give for halting? This is the operational form of `sec:blame` for the worked case: the externally imported stopping rule (Haddon's criterion) made concrete. If they don't name their stopping rule, that's also data — it shows the §11.7 guardrail being honored implicitly rather than explicitly._

**Highest layer reached:**

**Stated stopping reason (verbatim or paraphrase):**

**Implication for 6D (does the coalgebraic frame need a stopping criterion, or is it the analyst's judgment call):**

---

## 5. Pointwise vs. drift

> _Per the orientation: cardiac surgery adverse events are typically pointwise (a wrong action at a moment), but the §11.9 drift case (where no single γ_i has a pointwise departure but the joint trajectory exits the safety region) is what 6D needs to flag as the modal-upgrade case. Did any of the 30 events look drift-like? If yes, this is the place to mark the case for 6D to build modal logic around. If all 30 are pointwise, that is also useful: it tells you the hospital paper does not extend the example pool to drift, and 6D's modal-upgrade claim has to lean on §11.9 alone._

**Drift-like cases (if any):**

**All-pointwise reading (if so, note):**

**Implication for 6D:**

---

## 6. The "did anything surprise me" slot

> _The thing you'd otherwise lose between now and 6D. Could be a domain detail that opened a coalgebraic angle, a tension between the textbook CAST and the worked case, a place where the authors slipped past the hindsight-bias guardrail, or a recommendation that would not have come from the chain model. One paragraph, raw._

---

## 7. Margin notes for the coalgebraic reading (carry into 6D)

> _Per the plan: this is where ω-bisim-at-a-node and `rem:forward-stamp`'s backward dual become operational. Capture any specific moment where the worked case made the γ/γ\* divergence concrete enough to formalize, and any moment where the coalgebraic reading strained (other than drift, which is slot 5)._

**Cases where the ω-bisim-at-a-node reading clicked:**

**Cases where it strained (non-drift):**

**Implication for 6D's formal section structure:**
