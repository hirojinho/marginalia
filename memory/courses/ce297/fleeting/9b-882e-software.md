# 9B — MIL-STD-882E software (Level-of-Rigor mechanism)

Source: MIL-STD-882E §4.4 (incl. Tables IV, V, VI). Read 2026-05-01.

## Pipeline

```
Software Control Category (Table IV)  ×  Severity (Table I)
            ↓
   Software Criticality Index (Table V)
            ↓
   Level-of-Rigor bundle (fixed task list)
            ↓ (only if bundle is incomplete)
   Risk Level (Table VI) → re-enters §4.3 acceptance chain
```

The §4.4 logic substitutes **control authority** for **probability**. Probability is unmeasurable for software, so the standard refuses to estimate it and replaces the axis. Severity stays — that is hardware-defined harm, and software inherits it.

## Table IV — Software Control Category (the autonomy axis)

| Level | Name | What the software does | Mitigation available? |
|---|---|---|---|
| 1 | Autonomous | acts on safety-significant hardware without human or independent safety mechanism in the loop | none |
| 2 | Semi-Autonomous | acts, but independent safety mechanisms can intervene in time | partial |
| 3 | Redundant Fault Tolerant | issues commands; redundant independent mechanisms cover each hazard | full, by design |
| 4 | Influential | informs an operator who decides; *no operator action required to avoid mishap* | operator-mediated, optional |
| 5 | No Safety Impact | no command, no safety data, no safety timing | not applicable |

- The ladder is **degree of in-the-loop human or redundant override**, not reliability or quality.
- *Influential* (level 4) versus *Autonomous* (level 1) reverses the §4.3 stance on operators: §4.3.3.b says operator-facing controls cannot move probability to *Eliminated*, yet here operator presence demotes the control category. Same operator, opposite role — *probability discount* (forbidden) vs *authority discount* (permitted).
- *No Safety Impact* (level 5) is an off-table category (compare *Eliminated* in the probability table): it exits the matrix entirely; the Level-of-Rigor bundle is empty.

## Table V — Software Safety Criticality Matrix (Software Control Category × Severity → Software Criticality Index)

| Control Category ↓ \\ Severity → | 1 Catastrophic | 2 Critical | 3 Marginal | 4 Negligible |
|---|---|---|---|---|
| 1 Autonomous | Index 1 | Index 1 | Index 3 | Index 4 |
| 2 Semi-Autonomous | Index 1 | Index 2 | Index 3 | Index 4 |
| 3 Redundant Fault Tolerant | Index 2 | Index 3 | Index 4 | Index 4 |
| 4 Influential | Index 3 | Index 4 | Index 4 | Index 4 |
| 5 No Safety Impact | Index 5 | Index 5 | Index 5 | Index 5 |

- Looks like the Risk Assessment Code matrix (Table III) — **is not**. §4.4.2 explicit: *"the Software Safety Criticality Matrix is not an assessment of risk."* Same shape, different ontological commitment: Table III ranks risk; Table V ranks **required engineering effort**.
- Asymmetric collapse: Catastrophic spans Criticality Indices 1–5, Negligible collapses to Indices 4–5. Catastrophic outcomes preserve control-category distinctions; negligible outcomes flatten them.
- Row 5 is constant — *No Safety Impact* software gets Criticality Index 5 regardless of severity (because by definition it cannot reach the hazard).
- Monotone in both axes (more autonomy → lower Criticality Index number = stricter; more severe → lower Index = stricter).

## Level-of-Rigor per Criticality Index (the bundle, embedded after Table V)

| Criticality Index | Required tasks |
|---|---|
| 1 | Analysis of **requirements + architecture + design + code**; in-depth safety-specific testing |
| 2 | Analysis of **requirements + architecture + design**; in-depth safety-specific testing |
| 3 | Analysis of **requirements + architecture**; in-depth safety-specific testing |
| 4 | Safety-specific testing only |
| 5 | Nothing (Not Safety) |

- The Level-of-Rigor bundle is a **fixed catalogue** — no program-specific tailoring inside the bundle. Tailoring is allowed *outside* (substitute the matrix), not inside (skip a layer of analysis).
- The ladder peels off analysis layers in reverse build order: Index 1 covers all four artifacts; each step down drops the lowest. Code analysis is the most demanding and is required only at Index 1.
- Testing is universal across Indices 1–4 (only "in-depth" qualifier varies), implying **testing is the floor** of safety-significant software per the standard.

## Table VI — incomplete Level-of-Rigor bundle → Risk Level (the legal escape hatch)

| Criticality Index | Risk Level if bundle incomplete | Program Manager decision |
|---|---|---|
| 1 | High | spend to complete OR formal risk acceptance |
| 2 | Serious | same |
| 3 | Medium | same |
| 4 | Low | same |
| 5 | Not Safety | no action |

- This is what makes the standard **operationally enforceable**: skipped Level-of-Rigor work does not vanish — it converts into a quantified risk that re-enters the §4.3 acceptance chain (Department of Defense Instruction 5000.02 signing authority by band).
- The mapping Criticality Index ↔ Risk Level is **identity** on bands (Index 1 ↔ High, …, Index 4 ↔ Low). Software risk re-enters the standard's hardware-risk machinery without translation.
- Note the conversion direction: *integrity by construction* is the default; *quantified risk* is the fallback. Opposite of the hardware path, where quantification is primary and engineering effort follows.

## Didactic example

**Scenario.** Quadcopter delivery drone. Software computes the geofence and issues motor cutoff if the drone crosses the fence over a school zone. No human pilot in the loop; no independent watchdog hardware to intervene if the cutoff is mis-issued or omitted. A failure (cutoff fires mid-air over the school, or fails to fire) can produce casualties.

**Walk:**
1. **Software Control Category?** Software acts on safety hardware (motors) without human or redundant-mechanism intervention available in the time window. → **Autonomous (level 1).**
2. **Severity?** Worst credible outcome: fatality. → **Catastrophic (level 1).**
3. **Software Criticality Index?** Autonomous × Catastrophic, Table V → **Index 1.**
4. **Level-of-Rigor bundle?** Analysis of requirements + architecture + design + code; in-depth safety-specific testing. *All four analysis layers, plus testing.*
5. **What if the program skips, say, code analysis?** Bundle incomplete → Table VI → contribution documented as **High** risk → Program Manager signs off (Department of Defense Instruction 5000.02 routes High to Component Acquisition Executive per the Table III chain) or funds the missing analysis.

Compare: same software but *fence-crossing alert* only, with mandatory human pilot pressing kill switch → Influential (level 4, operator-mediated) × Catastrophic (level 1) → Criticality Index 3 → drop code + design analysis. The autonomy reduction is the only thing that bought the relief.

## Connections to later parts

- **§4.4 = §4.3 with the probability axis removed and replaced by Software Control Category.** The chain-causality stance survives because severity (the harm-axis) is hardware-grounded; what gets re-engineered is the *likelihood* axis, which the standard refuses to estimate for software and substitutes with **observable structural autonomy**.
- **Integrity by construction** = the algebraic stance in pure form. Level-of-Rigor is a fixed dictionary; satisfying the prescribed tasks discharges criticality. Build software up from blocks tagged with their integrity. This is what STAMP rejects in 9E — STPA observes constraint enforcement at runtime; no fixed integrity dictionary, no Level-of-Rigor analogue.
- **STAMP's missing piece (per 9E preview).** STPA produces hazards and Component Safety Constraints (the safety-functional content) but no Software Criticality Index, no Level-of-Rigor bundle, no quantified risk band. STPA cannot route software through Department of Defense Instruction 5000.02 acceptance authority because it does not generate the integrity number the chain needs. That is the 882-vs-STPA compliance friction Leveson addresses.
- **The substitution itself is the seam.** Probability → Software Control Category is the standard's clearest admission that the chain model needs help when it runs through software. Coalgebraic framing: the control category is a partial coarse-grained observation of the software's interaction with its environment — exactly the kind of observation STAMP would generalize. Open thesis question: is the control category a degenerate coalgebra (constant functor at deployment time) and STAMP the full one?
