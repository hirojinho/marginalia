# CE-297 — Curiosity Log

Topics that came up during study and sparked curiosity beyond the course scope.
Not active tasks — a backlog for future exploration.

---

- **Side-channel attacks (Spectre, Meltdown)** — came up as an out-of-scope item in the STPA OS kernel analysis. Eduardo asked for an explanation. Key insight: they violate isolation not through the logical memory model but through physical hardware behavior (timing, cache state). Entry point: the original Spectre/Meltdown papers (2018), and Ge et al. "A Survey of Microarchitectural Timing Attacks" for a formal treatment.

- **Linear Temporal Logic (LTL) — full notation and theory** — came up naturally from the SC2 starvation-freedom constraint in the STPA homework. The □/◇ operators were unfamiliar; Eduardo asked for a full explanation. Natural entry point: Baier & Katoen *Principles of Model Checking* Ch. 5 (LTL semantics), freely available online. Connects directly to model checking and the safety/liveness distinction already in his study guidelines.

- **Virtual memory & page tables as formal structures** — came up while studying OS kernel control structure for STPA homework. The page table as a partial function `ProcessID → (VPage ⇀ PPage × Permissions)` and isolation as a disjointness invariant felt natural. Thread worth pulling: formal verification of OS memory isolation (seL4 is the canonical reference — fully verified microkernel with machine-checked isolation proofs in Isabelle/HOL).

- **Safety + formal methods + automated decision making as a thesis direction** — sparked during §2.4 of Leveson, from her mention of "dynamic" and "naturalistic" decision-making research (Edwards, Rasmussen, Klein). Eduardo noticed that the framework generalizes from human operators to automated decision makers and asked whether the three could combine into a thesis. Four intersecting entry points:
  1. **Formal verification of learning-based controllers** — Seshia (Berkeley), Topcu (UT Austin), Katz et al. (Marabou, neural network verification).
  2. **STPA extended to ML-enabled / autonomous systems** — Leveson's own lab has work here; central tension is that STAMP's "controller" abstraction strains when the controller is a learned policy with no nameable control actions.
  3. **Epistemic / belief-state formalization of the designer-vs-operator mental-model triad** from §2.4.4 (Kripke structures, POMDPs, belief-MDPs, epistemic planning) extended to a four-structure problem when the operator is itself automated.
  4. **ML-guided exploration of STAMP state space** — orthogonal to 1–3: uses ML as a *tool in service of* STAMP analysis rather than applying STAMP to ML. Research pattern is "learning-guided formal analysis." Closest neighbors: falsification of cyber-physical systems (Breach, S-TaLiRo — optimization/RL-based search for spec-violating trajectories); neural-guided model checking / SAT; automated STPA work by John Thomas (MIT PhD thesis 2013, *Extending and Automating a Systems-Theoretic Hazard Analysis*) and Asim Abdulkhaleq (STPA + model checking). This corner is less mature — a real opening. Critical discipline: keep ML as *heuristic* (search guidance, preserves soundness), not as *oracle* (concluding safety, breaks soundness).

  The big structural question any thesis here has to answer: **formalize STAMP itself, or use a different formal substrate (POMDPs, hybrid automata, epistemic logic) and treat STAMP as motivation?** STAMP's "state space" is not currently a well-defined object, so direction 4 has direction 3 (or equivalent) as a prerequisite. The pairing **3+4 (formal semantics for STAMP + ML-guided exploration) is the most coherent thesis spine identified so far** — the first piece gives you something to explore, the second lets you explore it at useful scale. This is the most concrete thesis-adjacent thread so far and connects directly to the formal-methods interests already logged.

- **Placke Conditions Table — formal gap inventory for 5C reflection** — surfaced reading Placke 2014 §§5.1–5.2 + Q&A pass. Placke fixes the *form* of multi-controller integration analysis (rows = control actions, columns = Required Conditions / Effect on System) but defers the *semantics* of what fills the cells, exactly mirroring the slot-vs-content pattern already named in `rem:stpa-elicitation` (Step 2) and `rem:csc-soundness` (Step 3). The Conditions Table is the third instance — and naming the *pattern* matters more than naming any single instance.

  ### Verification regime (the SMT/temporal framing)

  Operationally, Placke's conflict criterion is **SMT, not pure SAT, and only instantaneously**. If you fix a typed shared state space *S* (gear ∈ {P,R,N,D}, brake torque ∈ ℝ⁺, …), Effects *eₐ* as postcondition relations, Assumptions *A_b* as preconditions, then conflict ⇔ ∃s. *eₐ*(s) ⊭ *A_b* — typed satisfiability. But this collapses to a snapshot. Admitting time / action ordering / sequences lifts the regime to **LTL/CTL temporal model checking** on traces. Placke is silent on which regime he's in, which is itself part of the problem (and part of why the method is hard to validate). This regime question is the cleanest way to introduce the verification framing in 5C.

  ### Full gap inventory (root → surface)

  Three core gaps for the 5C remark:

  1. **No shared-state ontology** *(root gap)*. Placke does not commit to which variables get a column or in what space; "shared state" is left to engineer elicitation. Cells use natural language — "wheels free to roll" vs "no brake torque applied" can mean the same thing or not, with no rule to decide. Without a state space, none of the other gaps can even be stated as theorems.
  2. **Temporal / sequencing gap** *(biggest practical missed bug class)*. The table is atemporal — actions are pairwise, instantaneous. But many automotive conflicts are exactly *timing* problems ("ACC issues brake while AH is still releasing"). No clock, no ordering, no action duration. This is what lifts the regime from SMT to LTL.
  3. **Process-model vs world conflation** *(STAMP-distinguishing gap)*. Each controller acts on its *belief* of shared state (its process model), not the actual world. Required Conditions are written as if they're about reality — but in STAMP they're about the controller's process model. So a conflict can arise either because (a) Effect violates Assumption in the world (Placke's intended target) or (b) Required Condition holds in belief but not world (process-model inadequacy from `sec:stamp-model`). The table folds two distinct bug classes into one column. Coalgebraically: γ_i operates on ProcessModel_i, not World, and the handshake has to traverse a feedback channel that Placke doesn't represent.

  Four secondary gaps (mention briefly, don't expand in remark):

  4. **Frame conditions absent.** When *eₐ* "sets brake torque to X," what does it leave unchanged? Without a frame rule, you cannot decide which assumptions are even *eligible* to be violated. Classical frame problem.
  5. **Causality of state change unmarked.** If V changes between two action checks, was it due to *eₐ*, autonomous world dynamics, or another controller? No causal arrow.
  6. **Acceptable-conflict / priority gap.** Some conflicts are *by design* (ACC overrides AH). No formal notion of designed precedence — same artifact can't distinguish hazard from intended override.
  7. **Multi-way conflicts.** Pairwise checking misses three-way conflicts where individual pairings are conflict-free but joint application is hazardous. The 2n vs n² scaling claim implicitly assumes pairwise sufficiency.

  Symptom from earlier framing:

  - **No completeness/soundness claim.** Placke argues by construction. Cannot be stated as a theorem without gap (1). This is *consequence*, not root.
  - **Scaling argument (2n vs n²) silently breaks under implicit coupling.** It's only 2n if Effects don't have hidden cross-variable consequences. F (the coalgebraic shape: "which variables can change in one step, given which others") is implicit in Placke; making it explicit invalidates 2n unless coupling is bounded.

  ### SpecTRM-RL contextualization

  Placke gestures at SpecTRM-RL (Leveson's tabular formal notation, RSML-descended Mealy automata with AND/OR transition tables) as prior art. It does not subsume the Conditions Table gap because it operates **one level below**: SpecTRM-RL specifies *one controller's blackbox input/output behavior*; the Conditions Table is shaped for *cross-controller integration*. The coalgebraic move sits *between* — at the same level as Placke (system-wide composition) but with semantics. Stack: SpecTRM-RL inside boxes ⊂ Conditions Table across boxes ⊂ coalgebraic STAMP framing all of it. Worth one sentence in 5C: prior tabular formalisms (SpecTRM-RL) operate at controller granularity; the multi-controller composition layer is where prior art is silent.

  ### 5C deliverable framing

  Land gaps **(1) shared-state ontology**, **(2) temporal/sequencing**, **(3) process-model conflation** as the three load-bearing instances of "Placke fixes form, defers semantics," extending the same `rem:stpa-elicitation`/`rem:csc-soundness` pattern. Coalgebraic resolution: predicate-lifted assume-guarantee composition (D2) + per-controller invariant respect under γ_i (D3) supplies the missing F + missing semantics in one move; LTL-shaped functor variants address (2); the ProcessModel × World factoring addresses (3). Hold to ~30–40 lines per `feedback_notes_density`; keep the secondary gaps as a footnote-style remark, not a separate subsection.

  ### §5.3 concrete findings (closed 2026-04-24)

  **Ad-hoc ontology Placke implicitly committed to** (10 variables, recovered from notes):
  *brakes battery, brakes force, engine, idle torque, electrical power, auto stopped, EPB, gas speed, wheel rotating, driver presence.*

  Three structural observations for 5C:

  - **Heterogeneous types, untyped in the table.** Mix of continuous-physical (brake force, idle torque, electrical power), discrete state (engine, auto stopped, EPB, wheel rotating), boolean (driver presence), and *resource* (battery, electrical power). Placke never types them, never says which are continuous vs discrete vs boolean — yet the conflict criterion needs typed comparisons. Concrete evidence for gap (1).
  - **"Brakes battery" and "electrical power" are resource variables**, not control state. They're the silent enabler of every actuator action. They appear in the ontology, but the table's pairwise frame can't carry the additive-consumption invariant they need. Bridge to a sub-gap (see below).
  - **Loose naming** (e.g., "gas speed" — accelerator pedal? rate? velocity? ambiguous). Concrete instance of prose-cell ambiguity the no-ontology gap predicts.

  **Concrete temporal/multi-way conflict surfaced in §5.3** — *simultaneous draw on shared electrical capacity*. Multiple subsystems acting concurrently can jointly exceed the battery's draw budget. This is **not** captured by Placke's pairwise atemporal frame. Three reasons it's sharper than the original "temporal" gap framing:

  1. **Not pairwise.** No pair of actions exceeds budget; the *k*-way joint does. Pairwise check is structurally blind.
  2. **Not snapshot-state.** Resource consumption rates compose additively over a *time window*, not at an instant. Even with battery state in the ontology, "Effect violates Assumption" can't represent "sum of consumption rates over a window exceeds capacity."
  3. **Needs a different formal object.** It's a *resource budget invariant* — ∑ᵢ active_i · cᵢ ≤ B — structurally distinct from predicate violation.

  ### Refined 5C framing (post §5.3)

  The battery example collapses the original gaps (2) temporal and (7) multi-way into one shape: **"Placke's frame is pairwise + atomic + atemporal."** Three sub-symptoms:

  - **(2a)** Sequential conflicts (action *a* fires while *b*'s effect still propagates) — pure timing/ordering
  - **(2b)** Simultaneity / additive composition (joint resource draw — battery example) — coordination at an instant on shared bounded resources
  - **(2c)** Multi-way (3+ controllers jointly hazardous, each pair safe) — k-ary, not 2-ary

  All three are manifestations of the same scope limit. **Recommended 5C structure:** consolidate (2) and (7) under "narrow scope of the pairwise atemporal frame," cite the battery contention as the concrete §5.3 instance covering (2b)+(2c), and flag (2a) as the predicted-but-unsurfaced sub-case.

  **Coalgebraic resolution covers all three uniformly:** the coalgebra is on a *joint* state including all controllers (handles 2c — multi-way), F can encode multi-step traces or rate-bounded transitions (handles 2a, 2b), and resource budget invariants are predicates on the joint state preserved by predicate lifting (handles 2b directly). One semantic move addresses all three sub-symptoms; that's the load-bearing thesis claim for the remark.

  **Final 5C gap structure** (three core, one consolidated):
  - (1) No shared-state ontology — root
  - (2) Pairwise atemporal frame — three sub-symptoms (2a sequential, 2b additive, 2c multi-way), battery example anchors (2b)+(2c)
  - (3) Process-model vs world conflation — STAMP-distinguishing
  Secondary (footnote): frame conditions, causality, priority/override. Drop original (7) — absorbed into (2c).

- **Paperwork safety culture vs formal methods — thesis-motivation lever** — surfaced reading [L] §13.2. The slide and Leveson both treat *paperwork culture* as a flawed-culture pattern: the controller (an organisation, regulator, or certification regime) enforces "safety" by producing documentation rather than by tracking the controlled process. Eduardo asked the natural question: aren't formal methods structurally the same — proofs and certifications *are* paperwork — and don't they therefore feed the same failure mode?

  ### The distinction that resolves the question

  Paperwork culture is not "you produced documentation." It is **the documentation tracks process compliance rather than outcome enforcement.** "Did we run the FMEA? Yes ✓" is paperwork. "Does □¬H hold over γ?" is — *in intent* — outcome. Formal methods aim at the second; paperwork culture lives in the first.

  The slide is real, however: formal methods *do* slide into paperwork in practice, and the failure modes are documented in the formal-methods-in-industry literature.

  - **Model drift.** The proof is correct, but of an artifact that no longer reflects the running system. Ariane 5 is the canonical case — the proof was sound of the wrong model.
  - **Quiet assumption violation.** The proof says "if A, then □¬H," and A silently stops holding under operation. No mechanism re-checks A.
  - **Certification capture.** DO-178C Level A, IEC 61508 SIL 4 require formal artifacts as a *checkbox* in the certification chain. The artifact is produced once, signed off, and never re-evaluated.
  - **Artifact-as-deliverable, not as live property.** The proof becomes the thing handed to the auditor; the property it certifies decouples from the system that's running.

  All four are instances of one shape: **the formal artifact decouples from the system it claims to be about.** That is the paperwork failure mode, repackaged.

  ### The defence is the SIS / feedback channel

  This is exactly where the Safety Information System lives in Leveson's picture. A formal proof without the feedback loop *is* paperwork. A proof tied to an SIS — re-evaluated as the system evolves, with the assumption set itself part of the audit — is not. STAMP names the requirement; whether a verification regime satisfies it is what determines whether the regime is paperwork or outcome-tracking.

  ### Why this is a thesis-motivation lever

  Algebraic verification (FTA, classical model checking on a static model) freezes the system into a model and proves a property over it. The artifact is one-shot, prone to all four drift modes above. The form of the artifact is structurally exposed to the paperwork failure mode.

  Coalgebraic specification (γ : A → F(A)) is observational. The verification target is "what can be observed in one step at this state" — closer to a runtime invariant than a static lemma. Naturally compatible with the SIS / OQE / audit picture coming up in Ch. 14 (SUBSAFE): the formal artifact *is* the feedback channel, rather than something the feedback channel has to chase. Re-evaluation under operation is the same operation that defined the property in the first place; there is no static moment from which the artifact can decouple.

  This generalises one level: **the algebra/coalgebra split tracks the satisfaction-of-process / achievement-of-outcome split that Leveson uses to diagnose paperwork culture.** Algebraic verification is structurally vulnerable to the failure mode coalgebraic verification is structurally positioned to resist. That is a clean, course-derived motivation lever for the thesis direction — independent of the technical case in `sec:coordination-failures` and the Placke argument, and earlier in the syllabus arc than SUBSAFE makes it visible.

  ### Where this lands in the formalisation

  Not as a body argument — body stays in course prose per `feedback_notes_formalization_hook`. This is hook material: the kind of thing that earns one closing-paragraph mention in `sec:coalg-desiderata` once the desiderata are restated post-coalgebra-reading, and one earlier closing pointer in the SIS remark and the flawed-cultures remark of 7A. The full motivation argument belongs to a future chapter introduction — not yet in the notes.

  ### Open thread to test against Ch. 14 (SUBSAFE)

  SUBSAFE's OQE (Objective Quality Evidence) and audit machinery are the closest thing in the course to a verification regime that *does* keep its artifacts coupled to the system. The conjecture worth holding while reading 7B: SUBSAFE's success is exactly the case where the artifact-system coupling is enforced organisationally — a non-coalgebraic regime that operationally satisfies what a coalgebraic regime would deliver definitionally. If true, this strengthens the lever: SUBSAFE shows the property is *needed*; the coalgebraic framing shows it can be supplied from semantics rather than from organisational discipline.

- **Descriptive behavioural imports for human/organisational tiers** — surfaced reading [L] §13.2 alongside the paperwork-culture entry. The technical side of the STAMP hierarchy is well-served by engineering models; the human and organisational tiers are not, and the formalisation needs *quantitative descriptive* models (how humans and organisations actually behave) to populate the parts of $M$, $\mathrm{act}$, and $\mathrm{obs}$ that engineering cannot determine. Distinct from the principal-agent / mechanism-design family already canvassed: those are *normative* (how organisations should behave under rationality assumptions). Behavioural sciences are descriptive — they give parameters, distributions, and update rules.

  ### Operator tier (one human in the loop)

  - **Rasmussen's SRK + dynamic safety model** — already half-imported by Leveson (§2.6 drift is Rasmussen). SRK (skill–rules–knowledge) classifies operator processing at three cognitive depths with characteristic time constants and failure modes per level. The dynamic safety model parameterises the operator's working position relative to a "boundary of acceptable performance" against explicit pressures (workload, economic, safety margin). Quantified update rules for $M$; bias parameters for $\mathrm{act}$. **Entry point: Rasmussen (1997) "Risk Management in a Dynamic Society: A Modelling Problem,"** *Safety Science* — pre-STAMP but structurally compatible, Leveson cites it.
  - **Klein's Naturalistic Decision Making + Recognition-Primed Decision (RPD)** — structural model of expert decision-making: pattern-match against case base → mental simulation → action. Parameterizable (case-base size, simulation depth, match threshold). Populates $\mathrm{act}$ for experienced operators in ways engineering decision theory does not. Entry: Klein (1998) *Sources of Power*; Klein, Calderwood, Clinton-Cirocco (1986) on RPD.
  - **Prospect theory (Tversky–Kahneman)** — empirically estimated parameters for loss aversion and probability weighting. Action selection under uncertainty deviates systematically from expected-utility-optimal, and the deviation is parameterised. Supplies a non-rational $\mathrm{act}$ for choices under risk. Entry: Kahneman & Tversky (1979) "Prospect Theory"; Tversky & Kahneman (1992) "Advances in Prospect Theory."

  ### Organisational tier (where §13.2 lives)

  - **Vaughan's normalization of deviance** — *the* quantitative counterpart to §13.2 complacency. Mechanism: each successful operation under degraded conditions shifts the organisation's perceived hazard threshold incrementally. Writes as a parameterised update rule on $M^{(\ell)}$ — *memory window* + *threshold migration rate per success*. **This is structurally what is missing from §13.2** — Leveson names complacency as a state; Vaughan gives it dynamics. Entry: Vaughan (1996) *The Challenger Launch Decision*, especially Ch. 3 and Ch. 9; Vaughan (1999) "The Dark Side of Organizations" *Annual Review of Sociology*.
  - **Hollnagel's ETTO principle + FRAM** — ETTO (efficiency–thoroughness trade-off) parameterises every action as a trade between speed and rigour, with systematic under-supply of thoroughness when efficiency rewards are immediate. FRAM (Functional Resonance Analysis Method) is in direct dialogue with STPA as a competing program — its quantification work has already been done; the parameter structure could be lifted into STAMP without buying its analysis frame. Entry: Hollnagel (2009) *The ETTO Principle*; Hollnagel (2012) *FRAM: The Functional Resonance Analysis Method*.

  ### Human-factors engineering practice (operational, not academic)

  - **HRA — Human Reliability Analysis** — THERP (Swain & Guttmann 1983), ATHEANA, CREAM (Hollnagel 1998). Engineering-grade quantifications of human error probabilities under performance-shaping factors, used in nuclear safety with regulatory force. Base failure rates + modulation parameters for $\mathrm{act}$ at the human-controller level. CREAM is the most cognitive and the closest to a coalgebraic shape. Entry: NUREG/CR-1278 (THERP); Hollnagel (1998) *Cognitive Reliability and Error Analysis Method*.

  ### Shape of the import

  What these sciences supply is not a replacement for the human controller's coalgebra. It is the **parameters and update rules** that the coalgebra requires but engineering cannot determine. At human-occupied tiers, $F$ becomes inherently probabilistic or non-deterministic — distributions over $\mathrm{act}$ outputs, parameterised $M$ update dynamics, bias-shifted $\mathrm{obs}$. This lands directly on **(Q1)** of `sec:coalg-desiderata-list` — "which category is the carrier?" The answer at human tiers is something like $\mathbf{Meas}$ (measurable spaces, probabilistic coalgebras), and the behavioural sciences supply the actual measures. The category choice is not free; it is forced by the science available for the tier.

  ### Implication for the thesis

  The mathematical content can stay clean — coalgebraic STAMP can be developed without committing to any specific behavioural model. What is required is a **slot**: the formalism should make explicit where behavioural parameters enter and how the choice of behavioural model affects the safety analysis. The two strongest candidates to name as worked instances are **Rasmussen** (operator tier, drift dynamics) and **Vaughan** (organisational tier, normalization of deviance), because (a) Leveson already cites both, so the thesis stays inside the safety-engineering tradition rather than inventing a bridge, and (b) both come with crisp parametric structure rather than vague typology.

  ### Pairing with the paperwork-culture entry

  These two entries answer the same question from opposite directions. **Paperwork-culture / formal-methods** asks: when does the upper-tier verification artifact stay coupled to the system? — the SIS / feedback-loop side. **Behavioural imports** asks: where does the upper-tier $M$ get its content from when the engineering picture runs out? — the model-input side. Together they bracket the upper-tier formalisation: one defines what the channel must carry; the other defines what the model must update under. The thesis claim that ties them: a coalgebraic semantics for STAMP makes both the channel structure (D1, F-shape) and the model dynamics (Q1, the carrier category) into formal data, recoverable from semantics rather than supplied separately by organisational discipline (paperwork side) or by engineering judgement (behavioural side).

---
