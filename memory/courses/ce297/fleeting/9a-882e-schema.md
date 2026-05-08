# 9A — MIL-STD-882E schema card

Source: MIL-STD-882E §3.2, §4.1, §4.3 (incl. Tables I, II, III). Read 2026-05-01.

## Eight-element System Safety Process (§4.3, ordered)

1. Document the system safety approach (§4.3.1)
2. Identify and document hazards (§4.3.2)
3. Assess and document risk (§4.3.3) — *Tables I/II/III live here*
4. Identify and document risk mitigation measures (§4.3.4) — **catalogue**
5. Reduce risk (§4.3.5) — **selection**
6. Verify, validate, and document risk reduction (§4.3.6)
7. Accept risk and document (§4.3.7)
8. Manage life-cycle risk (§4.3.8)

The order is a claim. §4.3.4 / §4.3.5 split = the only non-obvious move: catalogue is hazard-stable and reusable; selection is program-specific and re-decided.

## Table I — Severity

| # | Category | People | Environment | Money |
|---|---|---|---|---|
| 1 | Catastrophic | death, permanent total disability | irreversible significant | ≥ $10M |
| 2 | Critical | permanent partial disability, hospitalization ≥3 | reversible significant | $1M – $10M |
| 3 | Marginal | injury w/ lost work day(s) | reversible moderate | $100K – $1M |
| 4 | Negligible | injury, no lost work day | minimal | < $100K |

- Three parallel impact axes (people / env / money); severity = **max** across axes (no inter-axis compensation).
- Money ladder is log₁₀ ($10M / $1M / $100K).
- Labels are decorative; the inequalities do all the work.

## Table II — Probability

| Lvl | Specific item | Fleet/inventory |
|---|---|---|
| A Frequent | likely to occur often | continuously experienced |
| B Probable | several times in life of item | occurs frequently |
| C Occasional | sometime in life of item | several times |
| D Remote | unlikely but possible | reasonably expected |
| E Improbable | assume not experienced (<1 in 10⁶) | unlikely but possible |
| F Eliminated | **incapable of occurrence** | **incapable of occurrence** |

- F is reachable **only** by architectural elimination — no doctrine, training, warning, caution, or PPE moves to F (§4.3.3.b explicit).
- A–E sit on the chain; F sits *off* the chain (term removed, not lowered).
- E ≈ 10⁻⁶/lifetime per item; the only quantitative anchor in the table.

## Table III — RAC matrix (severity × probability → risk level)

|  | 1 Catastrophic | 2 Critical | 3 Marginal | 4 Negligible |
|---|---|---|---|---|
| A Frequent | High | High | Serious | Medium |
| B Probable | High | High | Serious | Medium |
| C Occasional | High | Serious | Medium | Low |
| D Remote | Serious | Medium | Medium | Low |
| E Improbable | Medium | Medium | Medium | Low |

- F not in the matrix (no risk to assess).
- Monotone in both axes (partial-order respect).
- Asymmetric across the diagonal: severity dominates at extremes (1E = Medium > 4A = Medium tie at the band, but 1D = Serious > 4B = Medium).
- Output is 4 bands over 20 cells — the matrix is a routing function: hazard → RAC → required signing authority.
- Per DoDI 5000.02: High → CAE, Serious → PEO, Medium → PM, Low → safety lead. The matrix exists to name the legally accountable signer.

## Joint hazards (a hazard in more than one category)

**Within Table I (across impact axes):** apply max. §4.3.3.a explicit. Hydraulic-leak example: people Marginal, env Marginal, money Negligible → **Marginal** (the Negligible axis does not pull the rating down).

**Across distinct outcomes from one source** (e.g., "engine failure" → casualties / vehicle loss / runway damage depending on phase): standard does not mandate one move. Two practical paths:
- **Worst credible outcome** — rate at highest credible severity. Conservative; default in most programs.
- **Decompose into multiple hazards** — each with its own RAC. More precise; pushes work into Appendix A Task 208 (FHA) and Task 205 (SHA).

In practice: top-level hazard log uses worst-credible; design-level analysis uses decomposition.

## Connections to later parts

- **882E = the algebraic camp made operational.** Table III mechanizes the chain-causality decomposition (risk = severity × probability over identifiable failure events).
- **F's special status** is the standard's quiet admission that operator-facing controls don't change probability bands — only architectural elimination does. Off-chain move.
- **882E lacks a STAMP analogue** for constraint enforcement. STPA produces hazards and CSCs but no severity, no probability, no integrity number → 9E (Leveson, "STPA Compliance with MIL-STD-882").
- **Joint-hazards ambiguity** = where chain-causality strains. STAMP sidesteps by checking constraint enforcement (binary), not by ranking outcomes. Cost: no integrity number. Open thesis question.
