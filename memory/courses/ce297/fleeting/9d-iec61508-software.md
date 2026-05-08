# 9D — IEC 61508 software branch (SFR / SIR / SIL)

Source: exida video "The Safety Lifecycle - IEC 61508 + IEC 61511". Watched 2026-05-03. Builds on `9c-iec61508-schema.md` (16-stage lifecycle + SIL band table).

## The two-axis factoring of a safety function

| Axis | Name | Question | Where it lives in the lifecycle |
|---|---|---|---|
| Behaviour | Safety Functional Requirements (SFR) | *What* must happen? | Stage 4 — Overall safety requirements |
| Integrity | Safety Integrity Requirements (SIR) | *How reliably* must it happen? | Stage 5 — Safety requirements allocation |

- Stage 4 produces the SFR catalogue (one functional spec per safety function, in language).
- Stage 5 attaches a SIL band to each SFR and partitions across subsystems. The SIR for a function **is** the SIL pinned at Stage 5 — i.e., a row of the PFDavg / PFH table from 9C.
- Every safety function downstream is a pair (SFR, SIR). Stages 9–11 (realisation) must deliver both: the right behaviour, hit at the right integrity.

### One automotive anchor

**Brake-by-wire firmware (Marcos's ECU, continuous mode).**
- **SFR:** "On detected brake-pedal travel ≥ θ at velocity v, command hydraulic actuator force F(v, θ) within 20 ms; on actuator non-response within 50 ms, engage independent backup channel."
- **SIR:** ASIL D (≈ SIL 3 continuous) ⇒ PFH ≤ 10⁻⁷/h.
- The SFR alone is a behavioural contract any embedded engineer could read. The SIR alone is a reliability target a quality engineer could read. Together they pin both *the function* and *the failure budget*.

## E/E/PE branch and Software branch — parallel realisation inside Stage 9

Stage 9 (E/E/PE realisation) is itself a sub-lifecycle. The video's load-bearing slide is the *parallel* execution of two branches under the same SIL:

| Branch | Governs | Normative IEC 61508 part |
|---|---|---|
| E/E/PE safety lifecycle | hardware (electronic / electrical / programmable-electronic) | Part 2 |
| Software safety lifecycle | embedded software running on the E/E/PE | Part 3 |

- The two branches are **fed the same (SFR, SIR) pair** from Stage 5; they execute independently and integrate at the end of Stage 9.
- Hardware branch hits SIR via redundancy, diagnostic coverage, proof-test intervals — quantitative reliability calculus.
- Software branch hits SIR via *prescribed techniques* keyed to the SIL number (Part 3 Annexes A/B). No probability calculation for software — same refusal as 882E §4.4, different machinery.

## The software safety lifecycle (V-model under Part 3)

The Part 3 software branch is a strict V-model:

```
Software safety requirements   ←→   Software validation
        ↓                                ↑
Software architecture           ←→   Software integration
        ↓                                ↑
Software system design          ←→   Software module integration
        ↓                                ↑
Software module design          ←→   Module testing
        ↓                                ↑
                Coding
```

- Each left-side artefact pairs with its right-side verification activity at the same level. The SIL number selects *which techniques* are required at each box (Part 3 Annexes — coding standards, dynamic analysis, formal methods, etc.).
- "Waterfall by mandate" (per 9C closing) shows up here as **V-by-mandate**: skipping a level is non-conformance, not a programme choice.

## How SIL routes engineering effort in the software branch (recognition-grade)

| SIL | Typical Part 3 demands (illustrative, not memorise) |
|---|---|
| 1 | Structured design, defensive programming, functional testing |
| 2 | + coding standards enforced, modular decomposition, equivalence-class testing |
| 3 | + semi-formal methods (state machines, sequence diagrams), data-flow analysis, boundary-value testing |
| 4 | + formal methods (typically *recommended*; *highly recommended* in safety-critical guidance) |

- The bands stack: SIL n inherits everything required at SIL n−1.
- Same algebraic skeleton as 882E's Level-of-Rigor bundle — a fixed dictionary keyed by an integrity number — with two surface differences: (i) keyed by SIL not Software Criticality Index, (ii) catalogue lives in Part 3 Annexes not §4.4 Tables.

## Didactic walk — Marcos's brake-by-wire through the software branch

1. **Stage 4 (SFR).** "Compute brake actuator force from pedal travel and vehicle state; engage backup channel on primary-channel non-response." → behavioural spec, no integrity yet.
2. **Stage 5 (SIR).** Hazard analysis (Stage 3) + ASIL D for severity / exposure / controllability → SIL 3 continuous ⇒ PFH ≤ 10⁻⁷/h. The pair (SFR, SIR=SIL 3) is now allocated to Marcos's ECU.
3. **Stage 9 split.** Hardware branch sizes the redundant channels and sets diagnostic coverage to hit PFH numerically. Software branch enters the V-model with SIL 3 selected.
4. **Software V at SIL 3.** Annex A/B require: semi-formal architectural specification (state machines for the channel-arbitration logic), enforced coding standards (MISRA-C-grade), data-flow analysis on the actuator-command path, structural testing to a defined coverage. No PFH number is computed for the software — *technique-completeness substitutes for measurable reliability*.
5. **Integration end of Stage 9.** Hardware (numeric PFH) + software (technique-completeness) integrated; if both branches discharged their obligations, the function is delivered at SIR = SIL 3. Stage 13 (validation) checks behaviour against Stage 4's SFR end-to-end.

## Connections to later parts

- **9E thesis hook (do not unfold here).** SFR + SIR factor a safety requirement into *behaviour × integrity*, both encoded inside the chain-causality model. STPA produces the SFR-equivalent — Controller Safety Constraints — but generates **no SIR-equivalent**: there is no STAMP-derived SIL number, no integrity band, no Part 3 technique catalogue. The integrity number is the chain camp's signature output and the coalgebraic side's gap. Same gap 882E surfaced via Software Criticality Index / Level-of-Rigor in 9B; 61508 surfaces it more cleanly because the integrity axis (SIL) is named first-class instead of derived.
- **9B parallel.** 882E re-engineered the *probability* axis (substitute Software Control Category for unmeasurable software probability); 61508 keeps probability for hardware (PFH/PFDavg via Part 2) and re-engineers it for software (substitute Part 3 technique-completeness). The substitution lives at different points but the move is identical: refuse to estimate, prescribe instead.
- **9C plan↔execute symmetry, extended.** SIR is the integrity contract Stage 5 hands forward; Stage 9 (software branch) discharges it; Stage 13 validates behaviour against Stage 4's SFR. The strict precedence of the 16-stage lifecycle is what permits this clean factoring — each stage's output is the input the next consumes (the algebraic backbone).
- **Stages 12–16 left light.** Slide weight says skip; the install/validate/operate trio executes the plans set in Stages 6–8 (already mapped in 9C) and adds nothing new for the SFR/SIR distinction.
