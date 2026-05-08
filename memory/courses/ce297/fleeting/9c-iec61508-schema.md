# 9C — IEC 61508 schema card

Source: exida "Back to Basics 07: Safety Lifecycle" + Wikipedia "IEC 61508" (SIL table). Read 2026-05-02. Genre context in `9-standards-genre.md`.

## 16-stage Overall Safety Lifecycle (slide chunking)

Five chunks, each doing one job:

### 1–5 — concept → allocation (analytical front-half)

| # | Stage | Job |
|---|---|---|
| 1 | Concept | What kind of equipment, what environment |
| 2 | Overall scope | What's in / out of the safety study |
| 3 | Hazard and risk analysis | Catalogue hazards |
| 4 | Overall safety requirements | What each safety function must do (in language) |
| 5 | Safety requirements allocation | Partition functions across subsystems + **pin the SIL** |

By the end of Stage 5: a list of (function, SIL) pairs the rest of the lifecycle must deliver. *The integrity number enters at Stage 5, not earlier.*

### 6–8 — three parallel planning documents (executed upfront)

| # | Stage | Plans for which downstream stage |
|---|---|---|
| 6 | Operation and maintenance planning | Stage 14 |
| 7 | Overall safety validation planning | Stage 13 |
| 8 | Installation and commissioning planning | Stage 12 |

Plan-then-execute, in pairs. Planning happens *before* building.

### 9–11 — three parallel realisation tracks (the build phase)

| # | Track | Example |
|---|---|---|
| 9 | E/E/PE safety-related systems realisation | Marcos's brake-by-wire firmware (Part 3 software invoked at assigned SIL) |
| 10 | Other-technology safety-related systems realisation | Mechanical pressure relief valves, hydraulic interlocks, fail-safe levers |
| 11 | External risk reduction facilities | Bunds around chemical tanks, exclusion zones, evacuation procedures |

Parallel, not sequential. A real system uses all three; Stage 5's allocation said which functions ride on which track.

### 12–14 — install / validate / operate

| # | Stage | Executes plan from |
|---|---|---|
| 12 | Installation and commissioning | Stage 8 |
| 13 | Overall safety validation | Stage 7 |
| 14 | Operation, maintenance, repair | Stage 6 |

Plan↔execute symmetry justifies Stages 6–8 as upfront work.

### 15–16 — end of life (recognition only, exam-light)

| # | Stage | Note |
|---|---|---|
| 15 | Modification and retrofit | Any change re-enters the lifecycle from the appropriate earlier stage |
| 16 | Decommissioning and disposal | Safe removal from service |

Adding adaptive cruise control to Marcos's brake-by-wire ECU re-runs Stages 4 onward — Stage 15 is the gate.

## SIL bands (two-mode table)

| SIL | Low-demand: PFDavg (per demand) | Continuous / high-demand: PFH (per hour) |
|---|---|---|
| 1 | 10⁻² ≤ x < 10⁻¹ | 10⁻⁶ ≤ x < 10⁻⁵ |
| 2 | 10⁻³ ≤ x < 10⁻² | 10⁻⁷ ≤ x < 10⁻⁶ |
| 3 | 10⁻⁴ ≤ x < 10⁻³ | 10⁻⁸ ≤ x < 10⁻⁷ |
| 4 | 10⁻⁵ ≤ x < 10⁻⁴ | 10⁻⁹ ≤ x < 10⁻⁸ |

- Bands half-open (lower included, upper excluded), like Avizienis severities.
- Lower SIL number = lower integrity. SIL 4 is hardest to achieve.

### Mode split rule

- Demand rate > 1 per year (or > twice the proof-test interval) → **continuous mode**, use PFH.
- Otherwise → **low-demand**, use PFDavg.

### Didactic anchors

**Low-demand — emergency shutdown valve, chemical plant.** Idle most of the time; demanded ~twice/year on a process upset. PFDavg = probability that *on the day you need it, it doesn't work*. SIL 3 ⇒ fewer than 1 in 10,000 demands fails ⇒ over 30-year plant life with 2 demands/year, expected failures ≈ 0.006. Effectively zero.

**Continuous — Marcos's brake-by-wire firmware.** Always running. PFH = dangerous failures per hour. ASIL D (≈ SIL 3) ⇒ PFH ≤ 10⁻⁷/h ≈ one dangerous failure per 1100 years per vehicle. Fleet of 10 million ⇒ ~9000 dangerous failures/year — ASIL D is the *bar*, not the target.

### Why the two columns matter

"SIL 3" alone is meaningless. SIL 3 low-demand (10⁻⁴–10⁻³ per demand) and SIL 3 continuous (10⁻⁸–10⁻⁷ per hour) are different numbers describing different failure phenomena. The slides assume you know which column you're reading.

## 7 normative parts (recognition-grade)

| Part | Topic |
|---|---|
| 1 | General requirements |
| 2 | Hardware requirements |
| 3 | Software requirements |
| 4 | Definitions and abbreviations |
| 5 | SIL determination examples |
| 6 | Application guidelines for Parts 2 and 3 |
| 7 | Techniques and measures overview |

Parts 2 and 3 are normative for builders; Parts 5 and 6 are guidance.

## Two-subsystem split + four subsystem types (recognition-grade)

- **Basic Process Control System (BPCS)** — runs the process normally. Not safety-rated.
- **Safety Instrumented System (SIS)** — sits on top, trips the process to a safe state when the BPCS or operators fail.

The SIS is what 61508 governs. Four subsystem types are sensor / logic solver / final element / operator interface — the standard architectural decomposition of a SIS loop.

## Closing line — waterfall by mandate

The 16 stages encode a **strict precedence**: Stage 9 cannot begin before Stages 1–8 produced their artifacts; Stage 13 cannot validate before Stages 9–11 built; Stage 14 cannot operate before Stage 13 validated. **Waterfall by mandate, not convention** — the standard refuses concurrent-engineering shortcuts. Connects to the slide "WATERFALL MODEL OF SOFTWARE DEVELOPMENT" and to the algebraic backbone of the chain camp: each stage's output is the input the next consumes (inductive, F(A) → A).

## Connections to later parts

- **9D** picks up the SIL bands as the integrity target Marcos's lifecycle (Stage 9) must hit. SIR/SFR factor a function into *behaviour × integrity*; SIR ≡ "the SIL band assigned in Stage 5."
- **9E** reads the strict precedence as the algebra/coalgebra translation problem made into a project schedule. STPA produces the SFR-equivalent (Controller Safety Constraints) but no SIR-equivalent — the SIL number has no STAMP analogue.
- **882E parallel:** 882E binds integrity to *task allocation* (Level of Rigor); 61508 binds it to *a number + technique catalogue* (SIL + Part 3). Same algebraic skeleton, different paperwork (per `9-standards-genre.md`).
