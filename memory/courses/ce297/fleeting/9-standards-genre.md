# 9 — Safety standards as a genre (882E + IEC 61508 context)

Cross-cutting note for Part 9. Sits above the per-standard schema cards (`9a-882e-schema.md`, `9b-882e-software.md`, future `9c-iec61508-schema.md`). Genre-level prose; the schema cards stay table-driven.

## What a safety standard *is*

A standard is a **rulebook**: a contractually or legally binding document prescribing *how* a safety-critical system must be developed, evaluated, and accepted, so that "is this safe?" becomes auditable.

- **Deliverable-driven.** It tells you which artifacts (hazard logs, risk matrices, V&V evidence, design records) you must produce and what each must contain. Compliance = artifacts exist + pass review.
- **Bureaucratic by design.** The purpose is making the safety claim *defensible to a third party*, not making the system safer in itself. Leveson's 9E critique lands exactly here: the algebraic shape baked in leaves system accidents invisible.
- **Read by navigation, not cover-to-cover.** §-numbered tables and lifecycle diagrams are load-bearing; connecting prose is filler. Engineers consult the standard; secondary sources (exida, Wikipedia, Young & Leveson) are what they *read*.

## MIL-STD-882E — System Safety

**Purpose.** Make safety risk a signed-off, tracked line item in DoD acquisition. Hazards catalogued, classified by severity × probability, mitigated, residual risk **formally accepted by the named authority whose seniority matches the risk band**.

**When used.** Mandatory by contract for any DoD acquisition program — aircraft, missiles, vehicles, ships, space, weapons, supporting infrastructure. Voluntarily adopted by NASA, FAA, NATO-aligned militaries. Spans concept → development → production → operation → disposal.

**Deliverables (what the contractor produces):**
- **System Safety Management / Program Plan** — how the 8-element process will be executed for *this* program.
- **Hazard Tracking System** — running ledger: each identified hazard, its current Risk Assessment Code, mitigation status, owner.
- **Hazard analysis reports** — outputs of Appendix A Tasks (Preliminary Hazard List → Preliminary Hazard Analysis → Subsystem / System Hazard Analyses → Operating & Support, Health, Functional Hazard Analyses).
- **Risk assessments** using Tables I/II/III; each hazard sits in a cell of the Risk Assessment Code matrix.
- **Risk Acceptance documents** — signed by the authority Table III routes to. *High* must go to Component Acquisition Executive; *Serious* to Program Executive Officer; *Medium* to Program Manager; *Low* to safety lead.
- **Software Safety package** — Software Control Category assignments (Table IV), Software Safety Criticality Index per function (Table V), Level-of-Rigor evidence (Tables VI/VII).
- **Safety Assessment Report** — consolidated hazard picture handed to the DoD acceptance authority before fielding.

**Force.** Contractual. Non-compliance = breach = system not delivered, not paid for, not fielded.

## IEC 61508 — Functional Safety of E/E/PE Safety-related Systems

**Purpose.** Give industries using programmable electronics a common way to **specify how reliably a safety function must perform** (the Safety Integrity Level) and **how to build the system to meet that target** (the lifecycle and the techniques mandated per SIL). Parent standard for every modern sector standard for software safety.

**When used.** Either directly (general industrial, machinery via IEC 62061) or through a sector child:
- Process industry (oil & gas, chemical, refining) → IEC 61511.
- Automotive → ISO 26262 (ASIL replaces SIL).
- Rail signalling/control → EN 50126 / 50128 / 50129.
- Medical devices → IEC 62304.
- Nuclear instrumentation → IEC 61513.

Not legally mandated in most jurisdictions, but the de facto market gate: an automotive ECU with no ISO 26262 evidence is uninsurable and unmarketable. Spans concept → decommissioning (the 16-stage Overall Safety Lifecycle).

**Deliverables:**
- **Overall Safety Plan** — how the 16-stage lifecycle will be run.
- **Hazard and Risk Analysis report** (Stage 3 output).
- **Overall Safety Requirements Specification** (Stage 4) — what each safety function must do.
- **Safety Requirements Allocation** (Stage 5) — partitions safety functions across E/E/PE / other-tech / external-risk-reduction.
- **SIL assignment per safety function** — the integrity target. Low-demand → PFDavg band; continuous / high-demand → PFH band.
- **E/E/PE realisation evidence** (Stage 9) — design, code, V&V, integration carried out at the techniques rigor level mandated for that SIL. Higher SIL → more techniques + stricter independence.
- **Software Safety Validation report** — evidence software-based functions meet the assigned SIL.
- **Functional Safety Assessment** — independent assessor's report confirming compliance.
- **Modification and decommissioning records** (Stages 15–16).

**Force.** Commercial / regulatory via the sector standards. Insurers, customers, certification bodies all assume 61508 (or its child) was followed.

## Concrete vignette — Sarah, software engineer at Lockheed Martin (882E)

Sarah owns the **lateral-axis flight control loop** of an unmanned combat drone — converts commanded roll angle into aileron deflection. Autonomous, no pilot override.

**How 882E grips her code:**
1. System safety engineer assigns the module **Software Control Category I** (autonomous, irreversible) per Table IV; worst hazard = **Severity I — Catastrophic** (loss of aircraft, possible ground casualties).
2. SCC I × Severity I in Table V → **Software Safety Criticality Index = 1** (highest tier).
3. Per Tables VI/VII, SwCI 1 mandates the full Level-of-Rigor task set: traceable safety requirements, formal architectural analysis, structural code coverage, safety-specific test cases, independent V&V, anomaly tracking.

**Daily reality:**
- Every commit to lateral-axis tagged with the SwCI in the message.
- Pull requests cannot merge without an attached LoR-tracking record showing which task this commit advances. Peer-review checklist explicitly references the LoR list.
- System safety engineer is a standing attendee in sprint reviews.
- Hazard Tracking System (DOORS or in-house DB) lists each hazard with current Risk Assessment Code, mitigation status, owner. Sarah named on three.
- One residual hazard sits at **Serious** RAC because mitigation is partial. Per Table III, *Serious cannot be accepted by the Program Manager* — must go to Component Acquisition Executive. PM has to brief up the chain.
- Before flight test, every artifact she produced feeds the **Safety Assessment Report**, which the DoD acceptance authority reads before signing acceptance.

The whole apparatus is the matrix routing the right signature to the right cell of Table III. Sarah's code earns its right to fly by producing the deliverables Tables V–VII demand.

## Concrete vignette — Marcos, embedded engineer at Bosch (IEC 61508 via ISO 26262)

Marcos owns the firmware on the **brake-by-wire ECU** — interprets pedal force, commands hydraulic actuators per wheel.

**How 61508/26262 grips his code:**
1. Hazard analysis (joint OEM + supplier) identifies "loss of brake function above 50 km/h" as worst hazard.
2. Risk-reduction allocates the safety function to the brake-by-wire ECU at **ASIL D** — automotive's highest, equivalent to SIL 3, PFH band 10⁻⁸–10⁻⁷ failures per hour.
3. Per ISO 26262 Part 6 (software), ASIL D mandates a *technique catalogue*:
   - MISRA-C coding rules enforced at compile time.
   - Semi-formal specification of safety requirements.
   - Modular architecture with explicit interface contracts.
   - MC/DC structural coverage (same bar as DO-178C Level A).
   - Third-party static analysis (Polyspace, Astrée).
   - Independent V&V by a separate Bosch team.
   - Runtime diagnostic coverage > 99%.

**Daily reality:**
- IDE runs MISRA analyzer on every save. CI rejects commits that don't hit 100% MC/DC on changed code.
- Requirements live in DOORS-NG with full traceability: hazard → safety goal → safety requirement → architectural element → code function → test case → coverage report. Trace web *is* the audit trail.
- Functional Safety Manager sits across Marcos's team — separate role, separate sign-off.
- Every two weeks the safety lifecycle tool (Polarion / Jama with safety extensions) gets updated against the 16-stage lifecycle. Stage 9 (E/E/PE realisation) is where Marcos's work lives.
- When firmware is feature-complete, an **independent Functional Safety Assessor** (TÜV SÜD, SGS-TÜV, exida) walks in. Samples deliverables, reads safety case, interviews team, signs the Functional Safety Assessment — or sends the team back. Without FSA sign-off, OEM cannot ship the car.

**Output the assessor reads:** a **Safety Case** — structured document arguing the ECU meets ASIL D, with full traces hazards → goals → requirements → architecture → code → tests → coverage → static analysis → reviews. The Safety Case integrates every artifact 26262 required Marcos to produce.

## Pattern across both

| | 882E (Sarah) | 61508 / 26262 (Marcos) |
|---|---|---|
| Top-level integrity binding | Software Safety Criticality Index (1–4) | SIL / ASIL (1–4 / A–D) |
| What integrity buys you | Required Level-of-Rigor task list | Required techniques + coverage targets |
| Daily artifact | Hazard Tracking System entry | DOORS-NG traceability link |
| Authority signing the system into service | Component Acquisition Executive (per Table III) | Independent Functional Safety Assessor (TÜV / exida) |
| Integrating final deliverable | Safety Assessment Report | Safety Case |

Same algebraic skeleton. Different paperwork: 882E binds integrity to **authority routing + task allocation**; 61508 binds it to **a number + a technique catalogue**.

## Hooks for 9E

- Both standards encode **integrity-by-construction** for software, allocated *before* deployment from a fixed dictionary. Satisfying the dictionary = the deliverable.
- STPA produces the SFR equivalent (Controller Safety Constraints) but **no SIR equivalent** — the integrity number has no STAMP analogue. The standards' machinery is exactly what STAMP cannot reproduce.
- This is the algebra↔coalgebra translation problem made operational and regulatory. Where 9E lands.
