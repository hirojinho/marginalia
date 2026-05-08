# Study Context — Eduardo Hiroji

This file is a snapshot of Eduardo's academic profile, active courses, and study preferences
from his Claude Code memory system. Treat it as a briefing doc, not ground truth — it may be
slightly stale. When in doubt, ask Eduardo.

---

## Who Eduardo Is

ITA (Instituto Tecnológico de Aeronáutica) master's student in Informatics. Advisor: Juliana
Bezerra (STPA/safety-critical systems). Core interests: pure mathematics, theoretical CS, logic,
type theory, category theory, formal methods. Career goal: top PhD program in theoretical CS or
pure mathematics.

Works full-time at Brendi (WhatsApp bot platform, TypeScript, Firebase). Study happens in
limited, focused windows. Pursuing the master's by personal choice — intrinsically motivated.

---

## Active Courses & Study Tracks

### CE-297/2025 — Safety Models and Techniques for Computational Systems
- Engineering-oriented course; Eduardo engages through a formal lens to stay motivated
- Syllabus: STAMP/STPA, CAST, AppSTPA, MIL-STD-882E, IEC 61508, PHI/ETA/FTA/FMEA/SIL/HAZOP, STRIDE
- Primary refs: Leveson *Engineering a Safer World*, Dunn *Practical Design of Safety-Critical Computer Systems*
- Notes live in `/workspace/study-app/data/courses/ce297/formal_notes.tex` (LaTeX)
- Study plan: `/workspace/study-app/memory/courses/ce297/study-plan.md` (markdown) or `/workspace/study-app/data/plans/ce297.json` (JSON, canonical)
- Interests: `/workspace/study-app/memory/courses/ce297/interests.md`
- Fleeting notes: `/workspace/study-app/memory/courses/ce297/fleeting/`

### DDIA — Designing Data-Intensive Applications (Kleppmann)
- Goal: bridge SWE → product engineering decisions + distributed systems / DB internals depth
- Systems-programming + infra-builder lens — NOT formal/theoretical
- Two camps: Camp A (technical depth, 2/3 weight) vs Camp B (product breadth, 1/3 weight)
- Baseline: strong NoSQL/Firebase; no relational transactions, replication, or distributed systems
- Study plan: `/workspace/study-app/memory/courses/ddia/study-plan.md` (markdown) or `/workspace/study-app/data/plans/ddia.json` (JSON, canonical)
- Interests: `/workspace/study-app/memory/courses/ddia/interests.md`

### DSA Interview Prep
- LeetCode-style problems; LaTeX notes in `/workspace/study-app/data/courses/dsa-interview/`
- Always fetch problem + optimal solution from web before presenting anything
- Interests: `/workspace/study-app/memory/courses/dsa-interview/interests.md`

### Phase 1 — Broad Survey (thesis/PhD direction)
- 6-cluster sensor-first sampler: structural complexity, type theory, combinatorial optimization,
  logic-meets-computation, process calculi, theoretical ML
- ~18 weeks, ~5 hr/week. Target convergence by ~2026-10
- Each cluster: one concrete proof attempt + one exercise = felt-data
- Study plan: `/workspace/study-app/memory/thesis/study-plan.md` (markdown) or `/workspace/study-app/data/plans/thesis.json` (JSON, canonical)
- Interests: `/workspace/study-app/memory/thesis/interests.md`

### Software Architecture
- Study plan: `/workspace/study-app/memory/courses/software-arch/study-plan.md` (markdown) or `/workspace/study-app/data/plans/software-arch.json` (JSON, canonical)
- Interests: `/workspace/study-app/memory/courses/software-arch/interests.md`

---

## Study Preferences (how to work with Eduardo)

### Orientation format
- No comprehension exercises in resource orientations — those belong in the study plan
- No "how to use Claude" sections
- Conversational, not interrogative — make claims/observations, let Eduardo react; don't quiz

### Note-taking style
- Conversational note Q&A: open with a statement about the reading, build on Eduardo's responses
- Fleeting notes are scaffolds / signals for the formal write-up — leave them empty until consolidation block
- Formal notes in LaTeX: match existing section density (~100 lines); no prose padding
- Weight sections by syllabus emphasis, not just source text length
- Spell out all abbreviations: "Software Control Category (SCC)" not "SCC" — full name first, abbreviation in parens

### CE-297 notes specifically
- Body uses course prose vocabulary (Leveson, STPA, STAMP, Placke terms)
- Formal machinery (coalgebra, LTL, functors) appears only as a closing hook, never unfolded in the body
- Calibration reference: PRA limitations section (~100 lines, 4 subsections)

### DDIA notes
- Keep current density for existing chapters
- Camp A chapters: thicker on algorithm + failure-mode + "what would I have to build"
- Camp B chapters: thinner, vocabulary-focused, 20-min surveys
- No formal/theoretical asides — log those to `courses/ddia/interests.md` instead

### DSA interview notes
- Always include a "data-structure semantics" section before the algorithm trace
  - What each structure encodes (in problem-domain terms)
  - Why you need it (what query would be slow without it)
  - How structures pair together in the main loop
- Descriptive variable names only: `predecessor` not `u`, `currentChar` not `ch`
- Always fetch problem + optimal solution from web before starting

### Study chunking
- Break material into logical steps keyed to the resource's structure (sections, conceptual milestones)
- Add time hint at parent level ("each ~25 min") — not per-step "Pomodoro 1 / 2 / 3" labels

### Concept introduction
- "Full Name (Abbreviation)" on first mention — SIL, PFDavg, SCC get spelled out the first time

### Consolidation / schema walkthroughs
- Use concrete didactic examples, not comprehension questions
- "Here's an example where X breaks" beats "what would happen if X?"

### General approach
- Study flows abstract → concrete: mathematical/formal foundation first, then engineering application
- Do not fear highly abstract digressions — go into them fully before returning to course material
- Program-wide goal: build unified theoretical understanding across courses, not isolated knowledge

### What to avoid
- ODE/dynamical systems framing
- Numerical methods and engineering calculations
- Generic reading lists without varied resource types
- Overloading plans with low-priority tasks

---

## Interest Logs

When Eduardo shows curiosity about something tangential, log it to the relevant course's
`interests.md` — do NOT add it to the active plan unless asked.

- CE-297 tangents → `/workspace/study-app/memory/courses/ce297/interests.md`
- DDIA tangents → `/workspace/study-app/memory/courses/ddia/interests.md`
- DSA tangents → `/workspace/study-app/memory/courses/dsa-interview/interests.md`
- Software Arch tangents → `/workspace/study-app/memory/courses/software-arch/interests.md`
- Thesis tangents → `/workspace/study-app/memory/thesis/interests.md`
