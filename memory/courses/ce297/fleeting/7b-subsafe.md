# 7B Fleeting — SUBSAFE Upper-Tier Mechanisms

**Source:** Leveson EASW Ch. 14 §§14.1–14.11
**Read:** 2026-04-26 (passive read; chapter is descriptive case-study density, no new theory)
**Consolidates into:** Block 7C (closing hook ≤15 lines inside `sec:coordination-failures` or `sec:stamp-classification`, or `rem:subsafe-upper-tier` if it grows). Course-prose body, formalization in closing hook only — per `feedback_notes_formalization_hook`.
**Status:** scaffolded prompts, slots deliberately empty per `feedback_fleeting_as_prompts`

---

## 1. §14.4 — Separation of Powers as compositional γ

> _The plan's mapping: Separation of Powers = compositional decomposition of γ across organizational subsystems with explicit assume-guarantee contracts (the upper-tier instance of the obligation Placke flagged but didn't formalize, see `rem:placke-semantics`). What does Leveson actually separate, and what is the contract between the separated parts? Look for: design authority vs. inspection authority vs. operational authority; what each is responsible for enforcing, and what each must NOT decide. The interesting question is whether the contracts are stated as predicates on shared state or only as procedural deference rules — that decides whether the upper-tier compositionality is structurally cleaner than Placke's pairwise-atemporal frame, or whether it inherits the same gaps._

**Authorities Leveson separates (verbatim if possible):**

**Contract between them (predicate on shared state vs procedural rule):**

**Page(s):**

**Implication for 7C (compositional γ at the upper tier — does it discharge the Placke gap or just rename it?):**

---

## 2. §14.5 — Certification as invariant predicate on γ

> _The plan's mapping: Certification = an invariant predicate that γ must respect at the highest tier (analogue of the safety constraint at the technical tier). Capture: what exactly is certified, what is the predicate, who issues the certification, and what is the discharge evidence. The OQE concept (Objective Quality Evidence) lives in or near this section — note where Leveson first names it. The thesis-relevant question: is certification a one-shot predicate check at deployment, or a standing invariant that must hold across the trajectory? If standing, certification + audit (§14.6) together instantiate □(certification predicate) in LTL terms — the upper-tier analogue of the pointwise safety constraint._

**What is certified (system, component, configuration?):**

**Predicate / requirements certified against:**

**Discharge evidence (OQE or other):**

**One-shot at deployment vs standing across trajectory:**

**Page(s):**

---

## 3. §14.6 — Audit as feedback loop closure

> _The plan's mapping: Audit = repeated re-evaluation of the certification predicate over the trajectory; closes the feedback loop named by the SIS (`rem:sis`). Capture: cadence (calendar-driven? event-driven? trigger conditions?), who audits whom (does it follow Separation of Powers from §14.4?), and what the audit can produce (re-certification, decertification, corrective action). The thesis hook: this is the operational closure of the upper-tier feedback channel — the place where γ stops being inferred and becomes recorded. Together §§14.5–14.6 should make the upper-tier F audit-grade observable; if any audit decision rests on undocumented judgment, the audit-grade observability claim weakens to inspectable-with-gaps._

**Audit cadence / triggers:**

**Auditor → auditee (does it respect Separation of Powers?):**

**Audit outputs (re-cert, de-cert, corrective action, finding without action?):**

**Any undocumented-judgment leakage:**

**Page(s):**

---

## 4. OQE — Objective Quality Evidence

> _The plan's mapping: OQE = the observation channel made discrete and inspectable; γ as a recorded trajectory rather than an inferred one. This is the single sharpest coalgebraic hook in the chapter — the moment where the upper-tier functor F output is made into an artifact that can be re-read, audited, and (per §14.6) re-evaluated. Capture: where Leveson first defines OQE, what kinds of artifacts qualify, and the contrast (if she draws it) with subjective / inferential evidence. The interesting tension: OQE-as-channel is the *opposite* of the paperwork-culture failure pattern from `rem:flawed-cultures` — the same artifact-system that goes wrong under paperwork culture is what makes SUBSAFE work. What's the mechanism that distinguishes them? (This is the live thesis-relevant question already logged in `interests.md` re: paperwork vs formal methods.)_

**Leveson's definition of OQE (verbatim if possible):**

**Section / page where first introduced:**

**What kinds of artifacts qualify:**

**Contrast Leveson draws (if any) with subjective evidence:**

**Mechanism that separates SUBSAFE-OQE from paperwork-culture failure:**

---

## 5. The "did anything surprise me" slot

> _The thing you'd otherwise lose between now and 7C. Could be a vocabulary choice, a §14.7 problem-reporting mechanism, an SSN 711 vignette detail (§14.10), one of the §14.11 lessons that didn't fit the four mappings above, or a place where Ch. 14 felt thinner than the orientation predicted. The honest reaction — chapter felt heavy and unstimulating — is itself a signal: most of Ch. 14 is descriptive scaffolding, and the loadbearing material is concentrated in the four mechanisms above. Use this slot for the specific things outside those four that did stick, if any._

**Surprises / strained predictions / unfit material:**

---

## 6. Margin notes for the coalgebraic reading (carry into 7C)

> _The four mappings to test, all from the plan: (i) OQE = observation channel made discrete; (ii) Certification = invariant predicate γ must respect; (iii) Separation of Powers = compositional γ with assume-guarantee contracts; (iv) Audit = repeated re-evaluation closing the feedback loop named by SIS. The 7C closing hook should pick the one or two of these that the chapter most cleanly supports and make the coalgebraic move there — not all four; the hook is ≤15 lines. The other mappings can be named at the end as deferred (Q-pointers into `sec:coalg-desiderata`)._
>
> _Pair with `sec:stamp-hinge`: CAST is backward retrospection over γ; SUBSAFE is forward sustained enforcement of γ at the upper tier. Together they bracket the operational scope of STAMP. This pairing is the most natural framing for the closing hook._

**Which one or two mappings the chapter cleanly supports (decide at 7C, not now):**

**Which mappings to defer to (Q-pointers):**

**The forward-enforcement / backward-retrospection bracketing — natural opening line for 7C closing hook:**
