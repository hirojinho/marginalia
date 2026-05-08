# Safety Models and Techniques (CE-297)

## Chapter 1 — Foundations: Dependability, Fault Tolerance, Safety

### Dependability Taxonomy

- [x] **1** 🔴 Read — Avizienis, Laprie, Randell & Landwehr (2004) "Basic Concepts and Taxonomy of Dependable and Secure Computing"
  - Completed 2026-04-03. Full formal notes in `formal_notes.tex`.
- [x] **2** 🔴 Reflect — Fault/error/failure chain as a state transition system
  - Completed 2026-04-03 via `formal_notes.tex`. Covers: trajectory formalism with state spaces and projections, counterfactual causality (σ_a^{[t₀]}), fault model (F, α) with compound activation, all six dependability attributes classified by formal expressibility, fault-error-failure chain formalized, means mapped to formal operations.
  - **Remaining angles** (not blocking, but valuable to revisit):
  - Compositional reachability: decompose S_int into subsystem state machines, express error propagation as reachability across their product (the notes flag this gap in the "extent" discussion)
  - LTL classification: Reliability = safety property (∀□¬FailureEvent); Availability = quantitative measure, not pure safety/liveness

### Accidents & Safety Concepts

- [x] **3** 🔴 Read — [L] Ch. 2 "Questioning the Foundations of Traditional Safety Engineering"
  - Completed 2026-04-08. Covered §2.1–2.3 and §2.8 via `formal_notes.tex`: PRA limitations formalized (σ-algebra measurability, product measure independence, wrong abstraction for software/humans), chain model limitations (linearity, subjectivity, systemic boundary), and all eight goals for a new accident model anchored to the formal results.
  - §2.4–2.7 skipped on first pass (human factors, software, operator error, system accidents) — covered in Part 2 below.
- [ ] **4** 🟡 Read — [L] Ch. 7 "Fundamentals"
  - Short chapter; covers the precise definitions used throughout the rest of the book
  - Focus on:
  - §7.1 "Defining Accidents and Unacceptable Losses" — accident/incident distinction; connect to MIL-STD-882E
  - §7.2 "System Hazards" — hazard as a system state that is a precondition to an accident; formalize as: hazard ∈ Q such that ∃ path from hazard to harm in δ
  - §7.3 "System Safety Requirements and Constraints" — safety constraints as invariants on Q
  - Skip: §7.4 (safety control structure — belongs to STAMP chapters)

## Part 2 — Motivation: Why Traditional Safety Engineering Fails

### 2A — The Human & Software Gap (remaining Ch. 2 sections)

- [x] **5** 🔴 Read — [L] Ch. 2 §2.5 "The Role of Software in Accidents" (pp. 47–50)
  - Completed 2026-04-13. Formalized in `formal_notes.tex` (`sec:software`): designer's anticipation set $\mathcal{D}$, deterministic/frozen $\delta$, second fracture in $\sigma_a^{[t_0]}$ (connects to `rem:rel-not-safe`). Parallel structure to operator fracture but epistemically different — $\mathcal{D}$ is closed at deployment, $M_O$ is in principle reducible.
- [x] **6** 🟡 Read — [L] Ch. 2 §2.6 "Static versus Dynamic Views" + §2.7 "The Focus on Determining Blame" (pp. 51–56)
  - Faster read. Key idea: systems migrate toward unsafe states over time even with no component failure (Rasmussen's drift). Blame-focused investigation stops too early.
  - Completed 2026-04-13. Formalized in `formal_notes.tex` (`sec:drift`, `sec:blame`): drift as trajectory replacing t₀ snapshot (Rasmussen's optimization process); blame as truncated backward search with externally imported stopping rule (Challenger/Columbia case); legal vs. engineering causation (Haddon's criterion).
- [x] **7** 🔴 Watch — Leveson's STAMP Workshop 2019 keynote slides: http://sunnyday.mit.edu/workshop2019/STAMP-Intro2019.pdf
  - Covered via orientation + reflection: three traditional approaches (fail-safe, defense-in-depth, system safety) mapped to P1–P4 precondition violations in `sec:preconditions`
  - ~20 min
- [x] **8** 🔴 Reflect — Where does σ_a^{[t₀]} break?
  - Completed 2026-04-13. Formalized in `formal_notes.tex` (`sec:preconditions`): four preconditions (P1–P4) for counterfactual applicability; three violation remarks — interaction accidents violate P1 (Σ_c ∩ H ≠ ∅, empty chain), software errors violate P1 differently (D too small), operator errors violate P2 (correction doesn't hold — M_O discrepancy survives). Closing remark maps three traditional paradigms to these violations and states STAMP's replacement question.
  - ~30 min

### 2B — Systems Theory Bridge (Ch. 3)

- [x] **9** 🔴 Read — [L] Ch. 3 "Systems Theory and Its Relationship to Safety" (pp. 61–70)
  - Completed 2026-04-15. Formalized in `formal_notes.tex` (`sec:systems-theory`): emergence as predicate irreducibility over the product space (connects to compositionality gap in `sec:states`), hierarchical control with process model as belief state (generalizes operator $M_O$ and software $\mathcal{D}$), four structural pathways to control failure, accidents reframed as control-structure snapshots rather than event chains.
  - §3.5–3.6 skipped (systems engineering process — revisit with STPA)
  - ~40 min
- [x] **10** 🟡 Read — STPA Handbook Ch. 1 "Introduction" (pp. 4–13)
  - Completed 2026-04-18. Consolidation pass: confirmed P1-violation vignettes (Mars Polar Lander, 787 batteries, ferry cars, flight simulator pedals — interaction accidents with no failed component). Surfaced the crisp distinction: **STAMP** = accident causality *model* (what the course will formalize); **STPA** = proactive analysis *method* built on STAMP; **CAST** = retrospective method. The key rhetorical formula for the synthesis: *"safety as a dynamic control problem rather than a failure prevention problem."*
- [x] **11** 🔴 Reflect — Synthesize the motivation arc
  - Completed 2026-04-18. Formalized in `formal_notes.tex` as new section `sec:stamp-hinge` ("The STAMP Hinge"). Three parts: (1) `sec:chain-breaks` — event-reversal (σ_a^{[t₀]}) vs constraint-enforcement as the two counterfactuals, with `rem:chain-subsumption` showing the chain model is the restriction of STAMP to component-local constraints; (2) `sec:stamp-stpa-cast` — STAMP (model) vs STPA (proactive method) vs CAST (retrospective dual), forward/backward reasoning over the same model; (3) closing remarks `rem:thesis-target` (STAMP as coalgebraic, STPA/CAST as algorithms over the coalgebra) and `rem:forward-stamp` (what to test in Ch. 4).

## Part 3 — STAMP: The Control-Theoretic Accident Model (Syllabus item 4)

### 3A — The STAMP Definition (§§4.1–4.4)

- [x] **12** 🔴 Read — [L] Ch. 4 §§4.1–4.4 as one sustained block (pp. 76–92)
  - Completed 2026-04-18. Covers §4.1 Safety Constraints · §4.2 Hierarchical Control Structure · §4.3 Process Models · §4.4 STAMP (the synthesis).
- [x] **13** 🔴 Reflect — Extend `formal_notes.tex` with `sec:stamp-model` (consolidating section)
  - Completed 2026-04-18. STAMP controller tuple, process-model inadequacy unifying the three failure classes, and the coalgebra shape sketch.

### 3B — Accident Causes Classification (§4.5)

- [x] **14** 🟡 Skim — [L] Ch. 4 §4.5 (pp. 92–100)
  - Completed 2026-04-19. §4.5.1 and §4.5.3 formalized in `formal_notes.tex` as `sec:stamp-classification` with subsections `sec:controller-operation` (algorithm/model split mapped to the controller tuple; compounded-causation and updater-boundary remarks) and `sec:coordination-failures` (three archetypes as distributed-systems primitives; compositionality remark as thesis payoff). §4.5.2 and §4.5.4 skipped as planned.

## Part 4 — STPA: The Four-Step Method (Syllabus item 5, STPA half)

### 4A — Scoping (Steps 1 & 2)

- [x] **15** 🔴 Read — STPA Handbook Ch. 2 "Defining the purpose of the analysis" (pp. 15–24)
  - Completed 2026-04-20. Formalized in `formal_notes.tex` as `sec:stpa-method` with subsection `sec:stpa-step1`: three-layer artifact chain (losses → hazards → constraints), the four stylistic tips as filters on a missing definition, and two remarks — `rem:hazard-predicate` recovering the Ch.~7 predicate definition (H ⊆ Q, worst-case env closure, □¬H as enforcement obligation) and `rem:handbook-operational` naming the operational-vs-definitional trade and its soundness cost.
- [x] **16** 🔴 Read — STPA Handbook Ch. 2 "Modeling the control structure" (pp. 25–33)
  - Completed 2026-04-20. Formalized in `formal_notes.tex` as `sec:stpa-step2` under `sec:stpa-method`: seven-stage derivation chain (subsystems → controllers → responsibilities R-i[SC-j] → control actions → process-model variables → feedback → zoom); Table 2.2 three-column schema; four "common points of confusion" as definitional filters. Two remarks — `rem:stpa-step2-modes` (derivation mode vs annotation mode; soundness cost compounds in annotation mode; coalgebraic desiderata implicitly demand derivation mode) and `rem:stpa-elicitation` (STPA fixes slots + trace relations but not content; elicitation not calculus; functor F from D1 is where the silent elicitation would be forced explicit).

### 4B — UCA Analysis (Step 3)

- [x] **17** 🟡 Watch — John Thomas "Introduction to STPA" (YouTube, https://www.youtube.com/watch?v=2W-iqnPbhyc)
  - Completed 2026-04-20. Watched as visual primer for the four UCA types; no notes taken — redundant with the Handbook's §"Identifying Unsafe Control Actions". Scaffolding served; handbook is the primary source for Step 3.
- [x] **18** 🔴 Read — STPA Handbook Ch. 2 "Identifying Unsafe Control Actions" (pp. 34–41)
  - Completed 2026-04-20. Formalized in `formal_notes.tex` as `sec:stpa-step3` under `sec:stpa-method`: four UCA types framed as two crossed axes (presence × temporal shape); UCA four-slot sentence form; tips as filters on missing definition; UCA→CSC mechanical dualisation (type polarity flips, source and context preserved); level-shift interpretive move (Step 3 first step whose artifacts don't live on Q). Two remarks — `rem:uca-observation` (UCAs as coalgebraic observations; completeness-under-ontology; four types as projections along presence + temporal-shape axes of F's output) and `rem:csc-soundness` (SC vs CSC type asymmetry; the `⋀ CSC ⟹ SC` implication as an assume-guarantee obligation STPA declares but does not discharge; coalgebraic closure via (D2) predicate-lifted composition + (D3) per-node invariant respected by γ_i).

### 4C — Loss Scenarios (Step 4) + Outputs

- [x] **19** 🔴 Read — STPA Handbook Ch. 2 "Identifying loss scenarios" (pp. 42–53) + "Summary and a Look Forward"
  - Completed 2026-04-22. Formalized in `formal_notes.tex` as `sec:stpa-step4` under `sec:stpa-method`: two-class split by fault location with respect to the controller (class (a) interior: process-model inadequacy vs. algorithm inadequacy; class (b) channel-side: actuator/sensor/feedback/communication), full traceability chain Losses → Hazards → SC → UCA → CSC → Scenario → Requirement, deliverable framing. Two remarks — `rem:scenario-split` (the (a)/(b) split as interior/channel decomposition of the controller coalgebra γ; class (b) as the operational moment where `rem:chain-subsumption` pays out — chain model demoted to sub-case) and `rem:traceability-closure` (the full traceability chain as a stack of assume-guarantee obligations whose composition rule STPA fixes by tags rather than semantics; coalgebraic reading converts each arrow to a predicate-lifting entailment via D2/D3).

### 4D — Consolidation

- [x] **20** 🔴 Reflect — Run the full four-step pipeline on TDC using the course slides as scaffolding
  - Closed 2026-04-22 without the TDC worked example. The Step 4 consolidation work absorbed the pipeline-synthesis role: `sec:stpa-method` now carries four step-subsections (`sec:stpa-step1` through `sec:stpa-step4`), and the closing remarks of Step 4 (`rem:scenario-split`, `rem:traceability-closure`) deliver the pipeline-level synthesis the TDC run was meant to produce. Revisit if the exam explicitly requires a TDC-specific walkthrough.

## Part 5 — STPA with Multiple Controllers (Ch. 4 cont., extends Part 4)

### 5A — Leveson's coordination vocabulary (consolidation, ~30 min)

- [x] **21** 🟡 Read — [L] §4.5.3 "Coordination and Communication among Controllers and Decision Makers" (pp. 98–100) + §8.6.3 "Coordination Risks" (p. 237) + §9.4.7 "Coordination of Multiple Controller Process Models" (pp. 294–295)
  - Completed 2026-04-22. Reading done; no edits folded into `sec:coordination-failures` — treated the Leplat two-geometry correction and §8.6.3 two-pattern taxonomy as oral context, not warranting a new remark. Revisit if 5C thesis reflection needs the vocabulary sharpened.

### 5B — The Placke method (core new content, ~1h)

- [x] **22** 🔴 Read — [Placke 2014] Ch. 5 "Identifying Conflicts to Prevent Hazardous Interactions", §§5.1–5.2 "Overview and Background" + "Method for Analyzing Integrated Systems" (pp. 89–93)
  - Completed 2026-04-24. Reading + Q&A pass surfaced the full inventory of formal gaps in Placke's Conditions Table — logged in `courses/ce297/interests.md` under the Placke entry as 5C reflection material. Key findings: (i) Placke does not claim completeness, only argues by construction; (ii) no shared-state ontology committed; (iii) the method is operationally an SMT problem on shared variables, but only instantaneously — temporal/sequencing conflicts are unrepresented; (iv) SpecTRM-RL (Leveson's tabular formal notation) operates one level below and does not subsume the gap. Reading frame for §5.3 logged in interests.md (watch for ad-hoc shared variable commitments + temporal conflicts in #7/#16/#19).
- [x] **23** 🔴 Read — [Placke 2014] §5.3 "Application to Case Study", §§5.3.1–5.3.2 (pp. 94–107)
  - Completed 2026-04-24 (read together with §§5.1–5.2 in one continuous block). Concrete §5.3 findings recovered post-hoc: 10-variable ad-hoc ontology (brakes battery, brakes force, engine, idle torque, electrical power, auto stopped, EPB, gas speed, wheel rotating, driver presence) with mixed types and ambiguous names; battery contention conflict (simultaneous draw on shared electrical capacity) as concrete §5.3 instance covering both temporal (2b simultaneity) and multi-way (2c k-ary) sub-gaps. Full refined 5C framing logged in `courses/ce297/interests.md`: three core gaps (ontology, pairwise/atemporal frame with three sub-symptoms, process-model conflation) — the original gap (7) multi-way is absorbed into the consolidated (2). Coalgebraic resolution covers all sub-symptoms in one move via joint-state coalgebra + predicate-lifted resource invariants.

### 5C — Reconcile slide terminology + thesis reflection (~30 min)

- [x] **24** 🟡 Note for instructor — Closed 2026-04-24. Confirmed in conversation: R5 is the resolution of a specific Placke case-study conflict (no new citation needed); "complementarity to hazard" is not alternative nomenclature for "Effect violating Assumption" but a *distinct* conflict class (symmetric joint-effect on same component/same context) corresponding to gap (2b) from the `interests.md` inventory.
- [x] **25** 🔴 Reflect — Conditions Table as assume-guarantee without semantics
  - Completed 2026-04-24. Formalized in `formal_notes.tex` as `rem:placke-semantics` inside `sec:coordination-failures` (placed after `rem:coordination-compositionality`). Structure: opening frames the Conditions Table as the local contract of assume-guarantee reasoning (Required Conditions = A_i, Effect = G_i, conflict = G_j ⊭ A_i); three named gaps deferring semantics — (i) no shared-state ontology, (ii) pairwise/atomic/atemporal frame with sub-cases *sequential*, *joint-effect* (=course's "complementarity to hazard", anchored by §5.3 battery contention), *multi-way*, (iii) observation vs world conflation (process-model inadequacy vs world-level assumption violation); closing hook names what a semantics would supply (typed state space + observation channel + temporal regime + composition rule) and points at `sec:coalg-desiderata` without unfolding the machinery. Added `placke2014` to `references.bib`. Holds to ~55 lines, matching density of neighbouring remarks in the subsection.

## Part 6 — CAST: Retrospective Dual of STPA (Syllabus item 5, CAST half)

### 6A — Epistemic framing (~25 min)

- [x] **26** 🔴 Read — [L] Ch. 11 §11.1 "The General Process of Applying STAMP to Accident Analysis" (pp. 350–352) + §11.7 "A Few Words about Hindsight Bias and Examples" (pp. 372–378)
  - Completed 2026-04-25. Treated as scaffolding block per plan: no `formal_notes.tex` write-up — formalization deferred to 6D where the forward/backward γ pair gets a section. Fleeting notes landing in `fleeting/6a-cast-spine.md` (four items: seven-step list verbatim, one-sentence CAST thesis, read on §11.7's reasoning level — pointwise vs. trajectory — , FTA/RCA contrast). These feed 6D's reflection.

### 6B — The mechanics: Steps 1–5 (~1h 10min)

- [x] **27** 🔴 Read — [L] Ch. 11 §§11.2–11.6 "Proximal Event Chain" → "Analyzing the Higher Levels of the Safety Control Structure" (pp. 352–372)
  - Completed 2026-04-25. Treated as scaffolding-plus-mechanics block per plan: no `formal_notes.tex` write-up — formalization deferred to 6D where the forward/backward γ pair gets its section. Fleeting notes scaffolded in `fleeting/6b-cast-mechanics.md` (six slots: §11.2 chain-stop line, §11.4 as-designed/as-operated naming, §11.5 template phrasing, §11.6 hierarchy iteration, surprise slot, coalgebraic margin notes). Slot 5 filled in conversation: **CAST is structurally more open than STPA, and the openness is not stylistic** — STPA takes a finite specification and produces artifacts under fixed taxonomies (four UCA types, UCA→CSC dualisation, two scenario classes); CAST takes a trajectory σ and *explains* it. Taxonomy gap: STPA's four UCA types are projections of F's fixed output (`rem:uca-observation`); CAST asks "where did γ depart from γ\*?", and the shape depends on γ\*, so no finite taxonomy of process-model flaws is possible. **Consequence for 6D:** the forward/backward γ pair is real at the level of the coalgebra but breaks at the level of the *method* — STPA is a procedure over γ, CAST is a discipline over γ. The proximal event chain (§11.2) is the only procedural moment in CAST and is exactly where the classical chain camp briefly returns. Remaining slots (1–4, 6) deliberately empty per `feedback_fleeting_as_prompts` — answered during 6D consolidation.
- [x] **28** 🟡 Read — [L] Ch. 11 §§11.8–11.9 "Coordination and Communication" + "Dynamics and Migration to a High-Risk State" (pp. 378–383)
  - Read 2026-04-25. Fleeting note scaffolded at `fleeting/6b-cast-coordination-drift.md` — slots open for §11.8 coordination taxonomy vs. `sec:coordination-failures`, §11.9 drift vocabulary + chain-reconstruction-failure case, and the coalgebraic margin reading (joint-observation functor for §11.8, modal-logic-needed for §11.9). Capture pending; feeds 6D.
  - §11.8 (slide Step 4): cross-reference `sec:coordination-failures` (Part 3B) and the Leplat two-geometry context noted in 5A. CAST application of the same vocabulary.
  - §11.9 (slide Step 5): the drift case. Connect to `sec:drift` already in `formal_notes.tex`. This is where CAST formally outruns pointwise STPA: drift is a property of whole trajectories, not one-step controller states — the formal gap the thesis should flag, not paper over.
- [x] **29** 🟢 Skim — [L] Ch. 11 §11.10 "Generating Recommendations from the CAST Analysis" + §11.12 "Summary" (pp. 383–388, 390) — slide Steps 6–7 recap. Skimmed 2026-04-25. Slide Steps 6–7 recap captured at the margin level; if a recommendation taxonomy or §11.12-emphasis surprise was worth keeping, lands in `fleeting/6b-cast-coordination-drift.md` §10 (margin slot) rather than a new file. Skipped §11.11 (experimental comparisons; not on slides).

### 6C — Worked case (~45 min)

- [x] **30** 🟡 Read — Leveson et al. (2020) "A Systems Approach to Analyzing and Preventing Hospital Adverse Events" (MIT preprint `http://sunnyday.mit.edu/papers/CAST-JPS.pdf`)
  - Read 2026-04-25. Smallest end-to-end worked CAST example in the public literature; non-aviation domain (cardiovascular surgery adverse events, n=30 aggregated). Source for the concrete γ\*/γ divergence + process-model-flaw instance that 6D needs.
  - Margin-note capture (per-controller template fills, hindsight-bias regress-stops, the aggregation-across-30-events move) is the unfinished tail; resolve before or as part of 6D.
  - Original frame: ~30 min reading + 15 min margin notes.

### 6D — Thesis reflection (~30 min)

- [x] **31** 🔴 Reflect — CAST as backward projection over γ; pair with `rem:forward-stamp`
  - Completed 2026-04-25. Formalized in `formal_notes.tex` as `sec:cast-backward` (new subsection inside `sec:stamp-hinge`, after `rem:forward-stamp`). Body refines the symmetric framing of `sec:stamp-stpa-cast`: STPA is a *procedure* (input fixed, output enumerable under the four UCA types + two scenario classes), CAST is a *discipline* (input is a loss trajectory, output is per-controller departure analysis whose shape depends on the designed control structure of the system, no fixed taxonomy possible). Hindsight-bias and stopping-rule guardrails are framed as artifacts of CAST's openness — STPA needs no analogue. The §11.5–§11.6 template is named as a one-step, node-level question. One named remark — `rem:chain-return-cast` — argues §11.2's proximal event chain is the only procedural moment in CAST and ties it back to `rem:chain-subsumption` (chain as restriction of STAMP to component-local constraints). Closing hook (~9 lines) names the per-controller template as a one-step, node-level question over the coalgebra of `sec:stamp-model`, names drift (`sec:drift` + Leveson §11.9) as the predicted strain, and points to `(Q4)` of `sec:coalg-desiderata` for the modal-logic upgrade — no machinery unfolded in the body. Section runs ~63 lines, in line with neighbouring subsection density.

## Part 7 — Operating the Upper Tiers: Management, Culture, SUBSAFE (Syllabus item 5, sociotechnical extension)

### 7A — Management, Culture, the SIS as feedback channel (~50 min)

- [x] **32** 🔴 Read — [L] Ch. 13 §13.2 "General Requirements for Achieving Safety Goals"
  - Completed 2026-04-25. Two mechanics absorbed: SIS as the named operational embodiment of the upper-tier measuring channel; flawed-cultures triad (complacency, paperwork, secrecy/blame) as upper-tier instantiation of the §4.5.1 algorithm/process-model split, mapping fine-grainedly onto the loop's three elements (model, control law, $\mathrm{obs}$). Atmospheric material (Alcoa, Colonial Pipeline) skimmed, not formalized.
- [ ] **33** 🟡 Skim — [L] Ch. 13 §13.1 "Why Should Managers Care about and Invest in Safety?" + §13.3 "Final Thoughts" (incl. *Are Accidents Inevitable?*) + [L] Ch. 12 §§12.4–12.5 "Feedback Channels" + "Using the Feedback"
  - §13.1 + §13.3 are motivational/rhetorical — match slide titles but contain no new mechanics. Read fast for the rhetorical payoff lines used in the slides.
  - Ch. 12 §§12.4–12.5 is the *operational* counterpart to the SIS — feedback channel mechanics from the engineering side. Thin if time is tight; include only if 7A's reflection wants the engineering/management duality.
  - ~15 min
- [x] **34** 🟡 Reflect — Two short notes, no full section
  - Completed 2026-04-25. Both remarks landed as planned. (i) `rem:flawed-cultures` inside `sec:controller-operation` (after `rem:model-updater`) — three patterns mapped fine-grainedly onto $M$ / $\mathrm{act}$ / $\mathrm{obs}$ loop elements, body in course prose with single closing-hook pointer to `sec:coalg-desiderata-list` ($F$ must carry social pressures determining admissible observations and enforceable actions). (ii) `rem:sis` inserted at the end of `sec:stamp-hierarchy` (before `sec:process-models`) — SIS framed as the named operational embodiment of the upper-tier measuring channel, with closing-hook pointer to (Q3). The flawed-cultures remark cross-refs `rem:sis` to anchor the secrecy-as-channel-closure case.
  - **Thesis-relevant tangent surfaced** — paperwork-culture critique vs formal methods; logged in `interests.md` as a clean motivation lever (algebra vulnerable to artifact-system decoupling, coalgebra structurally resistant). To be tested against SUBSAFE / OQE in 7B.

### 7B — SUBSAFE as a forward worked example (~1h 15min)

- [x] **35** 🔴 Read — [L] Ch. 14 "SUBSAFE: An Example of a Successful Safety Program" (full chapter)
  - Completed 2026-04-26. Passive read; chapter felt dense and unstimulating, consistent with the plan's "case-study density, not new theory" framing — no `formal_notes.tex` write-up at this step. Fleeting file scaffolded at `fleeting/7b-subsafe.md` with six slots: (1) §14.4 Separation of Powers as compositional γ, (2) §14.5 Certification as invariant predicate, (3) §14.6 Audit as feedback loop closure, (4) OQE as observation channel made discrete, (5) surprise slot, (6) coalgebraic margin (the four mappings + forward/backward bracketing with CAST). Slots deliberately empty per `feedback_fleeting_as_prompts` — to be filled during 7C consolidation, where one or two of the four mappings get the closing hook and the rest get Q-pointers.
- [ ] **36** 🟢 Watch (optional) — A US Navy or MIT SUBSAFE briefing video if findable
  - Search YouTube for "USS Thresher SUBSAFE" or "SUBSAFE program briefing" — do **not** assume one exists; verify before clicking. Skip entirely if Ch. 14 is self-contained.
  - ~15 min if used

### 7C — Thesis reflection: the upper-tier functor, made operational (~25 min)

- [x] **37** 🔴 Reflect — SUBSAFE as the case where F at the management tier becomes audit-grade observable
  - Completed 2026-04-26. Formalized in `formal_notes.tex` as `rem:subsafe-upper-tier` inside `sec:coordination-failures`, placed immediately after `rem:placke-semantics` as the discharge of the gap Placke flagged. **Load-bearing distinction (driven by Eduardo, not the four-mappings checklist):** Separation of Powers ≠ partitioned responsibilities — it's *production–consumption asymmetry*. Paperwork-culture failure (`rem:flawed-cultures`) is the loop closed on document state because the producer and consumer of the document are the same body. SUBSAFE breaks the loop architecturally: production and consumption sit on distinct authorities with independent authority to act, and the consumer's $\mathrm{act}$ depends on whether the document tracks the controlled process — so the producer is forced back onto plant state by institutional refusal, not internal embarrassment. OQE (§14.5) is the typing the handshake imposes on the guarantee; Certification (§14.5) is the predicate the consumer evaluates; Audit (§14.6) is the repeated re-evaluation that keeps the handshake live across the trajectory. Closing hook frames this as the upper-tier instance of $G_j \models A_i$ with semantics supplied by organisational construction; pairs with `sec:cast-backward` to bracket STAMP's operational scope (CAST = backward retrospection over γ; SUBSAFE = forward sustained enforcement of γ at the upper tier); points to (Q4) of `sec:coalg-desiderata` for the standing-invariant ($\Box$) upgrade. Back-reference added to `rem:flawed-cultures`. The four-mappings checklist (OQE / Certification / Separation of Powers / Audit) collapsed onto Separation of Powers as the structural lever; the others are roles within the handshake. Ran ~38 lines in line with neighbouring remark density (`rem:placke-semantics` ~75, `rem:flawed-cultures` ~50). Fleeting prompts at `fleeting/7b-subsafe.md` served as agenda only — slots deliberately remain empty per updated `feedback_fleeting_as_prompts`.
- [ ] **38** 🟢 Read — [L] Ch. 12 §§12.1–12.3 + §§12.6–12.8 (operations based on STAMP, change management, occupational safety) — operational playbook content; pick up only if the exam tests Ch. 12 specifics
- [ ] **39** 🟢 Read — [L] §10.1 "The Role of Specifications and the Safety Information System" — engineering-side SIS treatment; complement to §13.2 if the SIS write-up wants both sides
- [ ] **40** 🟢 Read — [L] Ch. 8 §8.6 "Using STPA on Organizational Components of the Safety Control Structure" — STPA *applied* to organizations; relevant if the thesis wants a method, not just a model, at the upper tier
- [ ] **41** 🟢 Read — Leveson (2017) "Hospital Adverse Events" preprint already cited in 6C — has implicit upper-tier CAST that pairs naturally with SUBSAFE for a forward/backward case study

## Part 8 — STRIDE: Adversarial Extension of the Control Structure (Syllabus item 9)

### 8A — Bridge + STRIDE methodology (~1h, 🔴)

- [x] **42** 🔴 Read — Young & Leveson (2014) "An Integrated Approach to Safety and Security Based on Systems Theory" (CACM, 4 pp., free at `http://sunnyday.mit.edu/papers/cacm232.pdf`)
  - Completed 2026-04-26. Manifesto-level read; the strategy/tactics distinction and the unification claim absorbed without formal write-up — same γ as STPA, attacker-augmented context. Concrete artifact (matrices, mitigation bijection) lives in 8A2; thesis hook (NR + Authz as observation-channel extensions) deferred to 8C margin.
- [x] **43** 🔴 Read — OWASP Threat Modeling Cheat Sheet (`cheatsheetseries.owasp.org/cheatsheets/Threat_Modeling_Cheat_Sheet.html`, free, ~25 min)
  - Completed 2026-04-26. Page is methodology-agnostic — organised around Shostack's four questions (What working on / What can go wrong / What doing about it / Did we do good job), STRIDE listed alongside PASTA/LINDDUN, no matrices unfolded. Source-of-matrices gap: the per-element table and STRIDE→property bijection below are transcribed from the slide deck, NOT this page; memorise them from the plan itself. Spine absorbed; no formal write-up — the four-question shape goes to `interests.md` with 8C, not into `formal_notes.tex`.
  - Original framing (kept for reference):
  - Covers the Four-Step Framework (System Model → Threats → Mitigations → Verification), DFD basics, the six STRIDE categories with examples. This is the slide deck's spine in a single web page.
  - **Memorise** for the exam:
  - **STRIDE-per-Element matrix** (which categories apply to which DFD element type):
- [ ] **44** 🟡 Reflect (margin note, no formal write-up) — STRIDE as adversarial refinement of `Sec = Co ∧ In ∧ Av_auth`
  - Co/In/Av/Authenticity match directly. **Non-Repudiation and Authorization are new** — Avizienis does not name them as primary attributes. Both are epistemic / role-conditional: who can prove what, who is allowed to do what. Hold for 8C.
  - One-paragraph fleeting note; ~10 min.

### 8B — One worked example: HTTPS + CSRF (~40 min, 🔴)

- [x] **45** 🔴 Read — OWASP CSRF Prevention Cheat Sheet (`cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html`, free, ~15 min)
  - Completed 2026-04-26. Skim pass per the orientation: §§1–7 absorbed at the level of "what invariant does each defence enforce"; §§8–10 (framework code) skipped as planned. Defence list memorised: synchroniser tokens, signed double-submit cookies, SameSite, custom request headers, Origin/Referer, Fetch Metadata. Supplementary video on HTTP request internals watched alongside (covers the credential-attachment loop the cheat sheet assumes — feeds the 8B HTTPS reflection directly).
- [x] **46** 🔴 Reflect (margin note) — HTTPS as one protocol enforcing three properties simultaneously (~10 min)
  - Completed 2026-04-26. Margin note at `fleeting/8b-https-csrf.md`, populated through conversation rather than scaffolded — Eduardo flagged shaky absorption of HTTP request internals and we worked the consolidation conversationally before writing. Captures: (i) the TLS property table (cert chain → S, record MACs → T, session key → I, all *channel-scoped*); (ii) CSRF as the residual gap — TLS protects the channel, CSRF attacks the server's belief about consent; load-bearing SOP read/write asymmetry named; (iii) defenses by who supplies the signal (server-issued secret, browser-supplied metadata, browser-side enforcement); (iv) course-frame integration at slide level (STRIDE table population: Spoofing + Tampering) and the deferred STPA-Sec / Young-Leveson framing (Level 3 → `interests.md`); (v) Avizienis cross-link to `sec:dependability-attributes` and `sec:preconditions` (CSRF as P1-violation in the security setting — no single $t_0$). No `formal_notes.tex` write-up per Part 8 plan.
- [x] **47** 🟢 Watch (optional, only if certs feel fuzzy) — Computerphile "TLS Handshake Explained" or "HTTPS — How it Works", ~10 min on YouTube. Verify before clicking.
  - Completed 2026-04-26. Watched as background on HTTP request internals (Eduardo flagged unfamiliarity with the request mechanics the cheat sheet assumes). Scaffolds the 8B HTTPS reflection — credential-attachment behaviour and the TLS property split now both have concrete operational anchors.

### 8C — Thesis margin (~15 min, 🟡, optional but quick)

- [ ] **48** 🟡 Reflect (margin note in `interests.md`, NOT a formal `rem:` write-up — no time today)
  - One short paragraph: STRIDE's adversary lives on the observation channel `F`, not on the carrier `A`. NR and Authz are the two extensions Avizienis's attribute set doesn't carry — both epistemic/role-conditional, both connect to the process-model gap from `rem:placke-semantics`. STPA-Sec rides the *same* γ as STPA (Young & Leveson's actual claim) — coalgebraic framing for STAMP supplies STAMP-Sec for free.
  - Drop into `interests.md` under a "STRIDE coalgebra extension" entry. Formal write-up is post-exam work.

## Part 9 — Standards: MIL-STD-882E and IEC 61508 (Syllabus item 7)

### 9A — MIL-STD-882E core: process + risk tables (~40 min)

- [x] **49** 🔴 Read — MIL-STD-882E §3 "Definitions" (skim) + §4.1 "General" + §4.3 "System safety process" (the 8 elements)
  - Completed 2026-05-01. Read flagged as boring — typical for a standard; absorption happens through the schema card, not the prose. Eight elements registered in order; Tables I/II/III seen but not yet internalized — that's the next task's job.
  - Free PDF: https://www.nde-ed.org/NDEEngineering/SafeDesign/MIL-STD-882E.pdf
  - Focus: §4.3.1–4.3.8 — the eight elements (Document, Identify hazards, Assess risk, Identify mitigation, Reduce, V&V, Accept, Manage life-cycle)
  - **Especially:** §4.3.3 — Tables I (Severity), II (Probability), III (RAC matrix). These are the schema you'll be tested on
  - Skip: §1 Scope, §2 Applicable docs; pause on §4.4 (software) — that's 9B
  - ~25 min
- [x] **50** 🔴 Reflect — Schema card (~15 min). Three compact tables to a fleeting note (no prose, no padding — feedback_notes_density):
  - Completed 2026-05-01. Fleeting note at `fleeting/9a-882e-schema.md`. Walked the eight elements, Tables I/II/III conversationally with didactic examples (per new `feedback_consolidation_examples`); load-bearing distinctions surfaced — F as architectural-only / off-chain, RAC matrix as legal-routing function (DoDI 5000.02 sign-off authority), §4.3.4-vs-§4.3.5 catalogue/selection split. **Joint hazards** added beyond the original card scope: max-over-axes within Table I (§4.3.3.a explicit); worst-credible vs decomposition for one-source-many-outcomes (standard ambiguous, FHA/SHA tasks pick up the slack). 9E hooks pre-staged: Table III routes acceptance authority (STPA produces no analogue), F's off-chain status is the chain model's first crack, joint-hazards ambiguity is where chain-causality strains and STAMP sidesteps by checking constraint enforcement instead of ranking outcomes.
  - Severity (Catastrophic / Critical / Marginal / Negligible — categories I–IV)
  - Probability (Frequent / Probable / Occasional / Remote / Improbable / Eliminated — A–F)
  - RAC matrix (High / Serious / Medium / Low) and how mitigation priority is read from a cell
  - Plus a one-line note on §4.3 element ordering — why the standard sequences *Identify mitigation* (4.3.4) **before** *Reduce* (4.3.5): mitigation is the catalogue, reduction is the selection. Captures one of the few non-obvious moves in §4.3

### 9B — MIL-STD-882E software: the LoR mechanism (~35 min)

- [x] **51** 🔴 Read — MIL-STD-882E §4.4 "Software contribution to system risk" + Tables IV (SCC), V (SSCM), VI (SwCI ↔ Risk Level ↔ LoR completion conditions)
  - Completed 2026-05-01. Standard's §4.4 has three subsections (4.4.1 intro, 4.4.2 SSCM, 4.4.3 risk assessment) and three tables — IV/V/VI — *not* IV/V/VI/VII as the plan originally said. The "Table VII" the plan referenced was a misnumbering; the SwCI ↔ Risk Level ↔ LoR mapping lives in Table VI here. Read flagged as standard-prose dense; the load-bearing content is the SCC×Severity→SwCI matrix and the LoR description per SwCI level. Mapping card + integrity-by-construction note next.
  - Same PDF; this is the slide-heaviest block of the standard and the most thesis-relevant
  - Focus: how *Software Control Category* (SCC) crossed with *severity* yields *SwCI* (Software Safety Criticality Index); how SwCI then dictates the required *Level-of-Rigor* tasks
  - The exam-likely grasp: given SCC + severity, derive SwCI (Table V); given SwCI, name the required LoR tasks (Table VI)
  - ~25 min
- [x] **52** 🔴 Reflect — Mapping card (~10 min). Capture as a single combined table:
  - Completed 2026-05-01. Fleeting note at `fleeting/9b-882e-software.md`. Walked the SCC→SwCI→LoR→Table-VI pipeline with a quadcopter geofence didactic example (per `feedback_consolidation_examples`). Load-bearing observations surfaced: (i) §4.4 substitutes **SCC for probability** because software probability is unmeasurable — chain-causality stance preserved by re-engineering the likelihood axis; (ii) Table V is a *required-effort* matrix, not a risk matrix (§4.4.2 explicit), even though it has the same shape as Table III; (iii) Table VI converts skipped LoR back into a §4.3 risk band — integrity-by-construction is the default, quantified risk is the fallback (opposite of the hardware path); (iv) SCC's operator demotion contradicts §4.3.3.b's operator probability ban — same operator, opposite role. 9E thesis hooks pre-staged: SCC-for-probability is the chain model's clearest admission of trouble when it runs through software; STPA produces no SwCI/LoR/integrity number, hence Leveson's 882-compliance friction; open question whether SCC is a degenerate coalgebra and STAMP the full one.
  - Columns: SCC level (1–5) | Severity → SwCI (1–5) → Risk Level (High/Serious/Medium/Low) → LoR tasks required
  - One-line margin note on what makes this *integrity by construction*: LoR is allocated *before* deployment from a fixed dictionary; satisfying the prescribed tasks satisfies criticality. This is the classical algebraic stance — build software up from blocks tagged with their integrity. Contrast with STAMP's coalgebraic stance (observe constraint enforcement at runtime) reserved for 9E. No `formal_notes.tex` write-up here
- [x] **53** 🟡 Skim — MIL-STD-882E Appendix A Tasks 100/200/300/400 (names only)
  - Completed 2026-05-01. Four-series shape registered as lifecycle filter: 100 Management (102/103 = Doc/Plan), 200 Analysis (201–210, the technical heart — 201–205 = progressive refinement PHL→PHA→SRHA→SSHA→SHA; 206–210 = cross-cutting O&SHA/HHA/FHA/SoSHA/EHA), 300 Evaluation (SAR + reports), 400 Verification (mostly explosives). Maps onto §4.3 eight elements; recognition-grade only — no memorization of numbers.
  - 100 Management · 200 Analysis (the heavy series — PHA, SRHA, SSHA, SHA, O&SHA, HHA, FHA, SoSHA) · 300 Evaluation · 400 Verification
  - Do not read individual tasks — just register the four-series structure. Standards-internal vocabulary; the slides use it for orientation
  - ~5 min

### 9C — IEC 61508 scope + 16-stage Overall Safety Lifecycle (~40 min)

- [x] **54** 🔴 Read — exida "Back to Basics 07: Safety Lifecycle" (https://www.exida.com/Blog/back-to-the-basics-07-safety-lifecycle-iec-61508)
  - Completed 2026-05-02. First contact with IEC 61508 — recognition-grade pass. Schema card next; absorption happens there.
  - Free, ~10 min. Compact overview that gives you: the 7 normative parts (Part 1 General · Part 2 Hardware · Part 3 Software · Part 4 Definitions · Part 5 SIL determination examples · Part 6 Application guidelines · Part 7 Techniques overview), the two-subsystem split (BPCS / SIS), the four subsystem types, and the 16-stage Overall Safety Lifecycle named once
  - This single read covers the slides "Definitions / Two Subsystems / Major Concepts / Four Types / Aims / Parts of IEC 61508 / Goal of Safety Lifecycle"
- [x] **55** 🟡 Read — Wikipedia "IEC 61508" (https://en.wikipedia.org/wiki/IEC_61508)
  - Completed 2026-05-02. Read alongside exida; SIL table is the load-bearing piece — pulls into the schema card next.
  - Sanity check on definitions; **focus on the SIL table** — PFDavg (low-demand) and PFH (high-demand/continuous) bands for SIL 1–4. This is the canonical exam content
  - Skip the historical/regulatory sections
  - ~10 min
- [x] **56** 🔴 Reflect — Schema card (~20 min). Two compact tables, no prose:
  - Completed 2026-05-02. Fleeting note at `fleeting/9c-iec61508-schema.md`. Walked the 5-chunk grouping of the 16 stages and the two-mode SIL table conversationally with didactic examples (per `feedback_consolidation_examples`) — emergency shutdown valve for low-demand, Marcos's brake-by-wire for continuous. Load-bearing moves surfaced: integrity number enters at Stage 5, not earlier; Stages 6–8 plan-then-execute pair with Stages 12–14; "SIL 3" is mode-dependent (different numbers for different phenomena). Plan↔execute symmetry pre-staged for 9D (SIR ≡ SIL band from Stage 5) and 9E (strict precedence = algebra/coalgebra translation as project schedule). 7 parts + BPCS/SIS split captured at recognition grade per the slide-weight rubric.
  - **16-stage Overall Safety Lifecycle** in two columns (Stage 1 Concept → Stage 16 Decommissioning). Group the slide chunking — 1–5 (concept/scope/hazards/requirements/allocation), 6–8 (overall planning), 9–11 (E/E/PE realisation, other-tech realisation, external-risk-reduction), 12–14 (install / validate / operate), 15–16 (modify / decommission)
  - **SIL band table** (SIL 1–4, PFDavg, PFH). The two columns matter — low-demand mode vs continuous/high-demand mode is a load-bearing distinction the slides assume
  - One-line note: the 16 stages encode a strict precedence order; this is *waterfall by mandate*, not waterfall by convention. Connects to the slide "WATERFALL MODEL OF SOFTWARE DEVELOPMENT"

### 9D — IEC 61508 software, SIR/SFR, SILs (~35 min)

- [x] **57** 🔴 Watch — exida "The Safety Lifecycle - IEC 61508 + IEC 61511" (https://www.youtube.com/watch?v=nFfrT3JXpjg)
  - Completed 2026-05-03. Visual primer absorbed; reflect step (SIR/SFR card + SIL closing note) is the consolidation.
  - Visual primer for how the E/E/PE branch and the software branch sit inside the overall lifecycle — direct match for the slides "E/E/PE AND SOFTWARE SAFETY LIFECYCLES" and "SOFTWARE DEVELOPMENT AND INTEGRATION WITH IEC 61508"
  - Duration unverified before clicking — likely 25–40 min. **Stop after the first complete lifecycle pass** if it goes longer; do not watch the IEC 61511 deepening
  - ~25 min
- [x] **58** 🔴 Reflect — SIR/SFR card + SIL closing note (~10 min). Single fleeting capture:
  - Completed 2026-05-03. Fleeting note at `fleeting/9d-iec61508-software.md`. Captured the SFR/SIR two-axis factoring with the Marcos brake-by-wire didactic walk (Stages 4 → 5 → 9-split → integration → 13). Load-bearing moves surfaced: (i) SIR ≡ the SIL band pinned at Stage 5 — confirms the 9C pre-stage; (ii) E/E/PE and Software branches sit *parallel* inside Stage 9, both fed the same (SFR, SIR) pair from Part 2 and Part 3 respectively; (iii) software branch is V-by-mandate — the software-side echo of 9C's waterfall-by-mandate, with SIL number selecting *which* Part 3 Annex techniques are required at each V-box; (iv) the substitution move repeats — 61508 keeps probability for hardware (PFH/PFDavg via Part 2) and refuses it for software (substitute Part 3 technique-completeness), the same shape as 882E's Software Control Category-for-probability swap in 9B but cleaner because SIL is named first-class instead of derived from SCC × Severity. 9E thesis hooks pre-staged: STPA produces Controller Safety Constraints ≅ SFR but no SIR-equivalent — sharper here than in 9B because the integrity axis is explicitly labelled SIL, so the gap is named, not inferred; the substitution-of-axes move (probability → technique-completeness) is the chain camp's clearest admission that the model strains when run through software.
  - **SFR** (Safety Functional Requirements) = *what* the safety function does. Behavioural specification (e.g., "close valve V123 within 2s of pressure > 50 bar")
  - **SIR** (Safety Integrity Requirements) = *how reliably* it must do it. The SIL the function must meet — i.e., the PFDavg or PFH band from 9C's schema card
  - Anchor to the automotive examples from the slides — same shape, different domain
  - Closing line: SFR + SIR jointly factor the requirement into *behaviour × integrity*, both encoded inside the chain-causality model. STPA produces the SFR equivalent (CSC) but does not produce a SIR equivalent — the integrity number has no STAMP analogue. Reserve the thesis read of this for 9E

### 9E — Comparative + thesis hook (~25 min)

- [ ] **59** 🟡 Skim — USPAS lecture "Controlling Risks: Safety Lifecycle" (https://uspas.fnal.gov/materials/12UTA/06_lifecycle.pdf)
  - The only verified open document comparing MIL-STD-882D and IEC 61508 lifecycles side-by-side. Page count unverified — open and skim
  - **Focus:** any comparison table; the v-model / waterfall mapping. Skip reactor-specific content
  - Goal: see the two standards laid against each other once; do not take notes
  - ~10 min
- [ ] **60** 🔴 Read — Leveson, "STPA Compliance with MIL-STD-882" (http://sunnyday.mit.edu/compliance-with-882.pdf)
  - Short paper. Argues STPA more completely satisfies 882E hazard-analysis tasks than traditional techniques
  - Focus: the explicit pairing of 882 tasks ↔ STPA artifacts. What does 882 *require* that STPA *delivers* — and at what conceptual cost?
  - ~10 min
- [ ] **61** 🔴 Reflect — One short remark in `formal_notes.tex` (~5 min, NOT a full section)
  - Place inside `sec:stamp-classification` *or* as a closing remark in `sec:stamp-hinge` — **not** a new subsection. ~15–20 lines max
  - Body in course prose vocabulary (per feedback_notes_formalization_hook): the standards (882E, IEC 61508) operationally encode the chain model — risk = severity × probability over decomposable failure events; software handled by integrity-by-construction (LoR / SIL allocation) before deployment. The 882-compliance argument (Leveson) reads STPA artifacts as deliverables for 882 tasks: STPA hazards satisfy §4.3.2, UCAs satisfy §4.3.3, loss scenarios satisfy §4.3.4 — but no STPA artifact answers SIR. The integrity number is exactly the algebraic-side input that the coalgebraic side does not generate
  - Closing-hook line only (one or two sentences): this is the algebraic camp made standard; integrity-by-construction is the algebra/coalgebra translation problem named operationally. Point to `rem:thesis-target`, `rem:chain-subsumption`, and (if relevant) `rem:placke-semantics`. No coalgebra unfolding in body
- [ ] **62** 🟢 Read — IEC 61508-3 "Software requirements" full text. Paywalled. Useful for the thesis if it pursues SILs as integrity-allocation calculus; not exam-needed
- [ ] **63** 🟢 Read — Bozzano & Villafiorita "Formal Methods for Certification" chapter — covers DO-178/ARP 4754; thesis-rich for software-side certification, off-syllabus
- [ ] **64** 🟢 Read — ABB "Functional Safety: An IEC 61508 Introduction" (library.e.abb.com) — vendor-side overview; only if exida's piece left the lifecycle opaque
- [ ] **65** 🟢 Read — NHTSA report 812285 "Assessment of Safety Standards for Automotive Electronic Control Systems" — concrete 882 vs 61508 comparison in automotive; thesis-relevant for SIR/ASIL bridge but exam-marginal
- [ ] **66** 🟢 Read — Leveson EASW Ch. 2 §2.3 "Probabilistic Risk Assessment" — already done in Part 2, but worth re-reading once the standards are concrete; her PRA critique now reads as a critique of *exactly* the 882 risk matrix mechanic

