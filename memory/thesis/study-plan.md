# Thesis — Phase 1 Survey

## Phase 1 — Cluster 1: Structural complexity (weeks 1–3)

- [x] **1** Watch: hackerdashery, "P vs. NP and the Computational Complexity Zoo" (~10 min, YouTube — `https://www.youtube.com/watch?v=YX40hbAHx3s`). The canonical popularization. Optional deepening: Michael Sipser, "Beyond Computation: The P vs NP Problem" (full ~1 hr lecture by the textbook's author); Aaronson "P =? NP" survey at `scottaaronson.com/papers/pnp.pdf` (long, 120pp — later reference, not pop-sci).
- [x] **2** Read book (substrate): Sipser, *Introduction to the Theory of Computation*, ch. 3 "The Church–Turing Thesis" (~25 pp). The TM definition itself — states, tape alphabet, transition function, configurations, deciders vs recognizers, multitape and nondeterministic variants. This is the vocabulary Cook 1971 silently assumes; lay it before touching Cook.
- [x] **3** §3.1 Turing Machines — formal definition, configurations, decider vs recognizer
- [x] **4** §3.2 Variants of Turing Machines — multitape, nondeterministic TM, enumerators
- [x] **5** §3.3 The Definition of Algorithm — Hilbert's 10th, Church–Turing thesis statement
- [ ] **6** Read book: Sipser, *Introduction to the Theory of Computation*, ch. 7 "Time Complexity" (~50 pp). Modern restatement of what Cook 1971 invented. Split into 3 merged sessions (revised 2026-05-02 — sections paired by pedagogical arc, not page count):
- [x] **7** Session A — §7.1 + §7.2: Cost model + Class P (~1.5 hr, done 2026-05-02). Asymptotic notation, time complexity for TMs, simulation theorems (multi-tape O(t²), NTM 2^O(t)), Class P + robustness, examples (PATH, RELPRIME, every CFL).
- [ ] **8** Session B — §7.3 + §7.4: NP + Cook-Levin (~2 hr, densest). Verifier ↔ NTM equivalence, polynomial reductions, Cook-Levin theorem (SAT is NP-complete).
- [ ] **9** Session C — §7.5 + proof attempt: Reductions (~1.5 hr). Reduction chain (3SAT, CLIQUE, VC, HAMPATH, SUBSET-SUM); flows directly into the SAT → 3SAT proof attempt below.
- [ ] **10** Read paper: Cook 1971, "The Complexity of Theorem-Proving Procedures," STOC '71 pp. 151–158 (~7–8 pp). Now read as the historical artifact of the Cook–Levin theorem just learned. Goal: see how the universal encoding (SAT-as-NP-complete) is constructed in raw discovery-mode prose. Cook's "query machine" = oracle TM (Turing reduction), more powerful than the many-one reduction Sipser uses; recognize the difference.
- [ ] **11** Proof attempt: Reduce SAT to 3-SAT by clause splitting. Prove correctness in your own words.
- [ ] **12** Exercise: Encode a 4-coloring instance for a small graph (~6 vertices) as SAT; run z3 or MiniSAT; observe runtime growth on instance size.
- [ ] **13** Felt-data note 1 — write the 1-page note (5-question template at the end of this file) immediately at end-of-cluster.
- [ ] **14** ADR 1 — write a 1-page ADR titled "Why structural complexity IS / IS NOT my thesis home." Constraint: use only vocabulary from this cluster (reductions, NP-completeness, decision problems). Take a position; defend it.

## Phase 2 — Cluster 2: Type theory & foundations (weeks 4–6)

- [ ] **15** Watch: Bartosz Milewski "Category Theory 1.1: Motivation and Philosophy" (~50 min, YouTube). Browse HoTT Book §1 intro for taste (the full HoTT book is hard; just read the intro chapter for now).
- [ ] **16** Read paper: Wadler 2015, "Propositions as Types," *CACM* 58(12):75–84 (~10 pp). The accessible Curry-Howard correspondence paper — connects logic, λ-calculus, and types.
- [ ] **17** Read book: Pierce, *Types and Programming Languages*, ch. 5 "The Untyped Lambda-Calculus" (~14 pp) and ch. 9 "Simply Typed Lambda-Calculus" (~14 pp). *Note:* ch. 3 is "Untyped Arithmetic Expressions" — useful warm-up if syntax/AST formalism is unfamiliar (see interests log: AST gap surfaced earlier).
- [ ] **18** Proof attempt: Prove subject reduction (preservation) for simply-typed λ-calculus. The proof is by induction on typing derivations + a substitution lemma — feel both.
- [ ] **19** Exercise: Implement β-reduction for untyped λ in any language; verify it works on Church numerals (encode 2 + 3 = 5).
- [ ] **20** Felt-data note 2.
- [ ] **21** ADR 2 — "Why type theory / foundations IS / IS NOT my thesis home." Vocabulary: types, terms, β-reduction, Curry-Howard, judgments. Distinguish *research interest* from *hobby*: would I want to publish papers in this, or just read them?

## Phase 3 — Cluster 3: Combinatorial optimization & matroids (weeks 7–9)

- [ ] **22** Watch: Lovász Abel Prize 2021 lecture "Graphs and Geometry" (~45 min, YouTube). Big-picture, accessible.
- [ ] **23** Read paper: Edmonds 1970, "Submodular Functions, Matroids, and Certain Polyhedra," in *Combinatorial Structures and Their Applications* (Calgary 1969 proc.), pp. 69–87 (~19 pp). Foundational. Skim §1–2; deep-read the matroid/greedy section.
- [ ] **24** Read book: Korte & Vygen, *Combinatorial Optimization*, ch. 13 "Matroids" (~30–35 pp). Read §13.1–13.3 carefully (definitions, greedy algorithm, matroid axioms); skim later sections.
- [ ] **25** Proof attempt: Prove the Edmonds-Rado theorem — greedy by weight is optimal iff the underlying structure is a matroid. Both directions.
- [ ] **26** Exercise: Take a small graph (~5–7 vertices); verify that the spanning forests form a matroid by explicitly checking the exchange axiom on representative pairs.
- [ ] **27** Felt-data note 3.
- [ ] **28** ADR 3 — "Why combinatorial optimization / matroid structure IS / IS NOT my thesis home." Vocabulary: matroid, exchange axiom, greedy, dual, polytope.

## Phase 4 — Cluster 4: Logic-meets-computation (weeks 10–12)

- [ ] **29** Watch: Wigderson ICM 2006 plenary "P, NP and Mathematics — A Computational Complexity Perspective" (YouTube; or his book talks for *Mathematics and Computation*, 2019). Optional: search YouTube for a Moshe Vardi public lecture on logic + computation.
- [ ] **30** Read paper: Fagin 1974, "Generalized First-Order Spectra and Polynomial-Time Recognizable Sets," in *Complexity of Computation* (R. Karp ed., SIAM-AMS Proc. vol. 7), pp. 43–73. Establishes NP = ∃SO. Dense; aim for the theorem statement and proof sketch.
- [ ] **31** Read book: Immerman, *Descriptive Complexity*, ch. 1–2 (FO logic + complexity background; verify exact chapter titles when opening — exact wording uncertain in my notes).
- [ ] **32** Proof attempt: (a) Show "graph has a triangle" is FO-expressible (write the formula). (b) Show "graph is connected" is NOT FO-expressible — use Ehrenfeucht-Fraïssé games or a direct compactness argument.
- [ ] **33** Exercise: Encode a structural property (e.g., "graph has an even number of vertices" or "graph is 3-colorable") in both FO and ∃SO; identify which complexity class it captures via Fagin's theorem.
- [ ] **34** Felt-data note 4.
- [ ] **35** ADR 4 — "Why logic-meets-computation IS / IS NOT my thesis home." Vocabulary: FO, ∃SO, definability, EF games, descriptive complexity.

## Phase 5 — Cluster 5: Process calculi & concurrency (weeks 13–15)

- [ ] **36** Read: Milner 1991 Turing Award Lecture "Elements of Interaction," *CACM* 36(1):78–89 (1993, ~12 pp). Use as the pop-sci-substitute (the video may not be widely available; the published text is canonical).
- [ ] **37** Read paper: Milner, Parrow, Walker 1992, "A Calculus of Mobile Processes, I" *and* "II," *Information and Computation* 100(1):1–40 and 41–77. Two companion papers. Read part I in depth; part II as deepening.
- [ ] **38** Read book: Sangiorgi & Walker, *The π-calculus: A Theory of Mobile Processes*, ch. 1 (~40 pp; introduces π syntax + operational semantics; exact ch. title verify on opening).
- [ ] **39** Proof attempt: Strong bisimulation between two simple π-calculus processes — e.g., a buffered channel `c(x).c̄⟨x⟩` vs. an unbuffered handshake. Construct the bisimulation relation explicitly; verify the closure conditions.
- [ ] **40** Exercise: Encode a small two-process handshake protocol in π-calculus syntax; analyze whether it deadlocks under the operational semantics.
- [ ] **41** Felt-data note 5.
- [ ] **42** ADR 5 — "Why process calculi / concurrency IS / IS NOT my thesis home." Vocabulary: process, channel, reduction, bisimulation, structural congruence.

## Phase 6 — Cluster 6: Theoretical ML / learning theory (weeks 16–18)

- [ ] **43** Watch: Vapnik on Lex Fridman Podcast #71 (2020, ~1h 45min) — VC theory + SVM history. Optional: Aaronson Shtetl-Optimized blog posts on PAC (search the blog).
- [ ] **44** Read paper: Valiant 1984, "A Theory of the Learnable," *CACM* 27(11):1134–1142 (~9 pp). Foundational PAC paper.
- [ ] **45** Read book: Shalev-Shwartz & Ben-David, *Understanding Machine Learning*, ch. 2 "A Gentle Start" (~10 pp), ch. 3 "A Formal Learning Model" (~14 pp), ch. 6 "The VC-Dimension" (~14 pp).
- [ ] **46** Proof attempt: Prove PAC-learnability of conjunctions over Boolean variables, with explicit sample-complexity bound. The proof technique (union bound + finite hypothesis class size) is the key transferable structure.
- [ ] **47** Exercise: Compute the VC dimension of axis-aligned rectangles in ℝ²; verify the PAC bound on a small synthetic dataset.
- [ ] **48** Felt-data note 6.
- [ ] **49** ADR 6 — "Why theoretical ML IS / IS NOT my thesis home." Vocabulary: hypothesis class, PAC, sample complexity, VC dimension, growth function.

## Phase 7 — Convergence (weeks 19–22)

- [ ] **50** Re-read the 6 felt-data notes side-by-side.
- [ ] **51** Re-read the 6 ADRs side-by-side. Note where vocabulary felt natural vs strained.
- [ ] **52** Score each cluster on five axes (1–5 each):
  - **Gut excitement** — when I think about doing this for 5 years, lean forward or away?
  - **Proof-doing satisfaction** — did the proof attempt feel alive in my hands?
  - **Problem curiosity** — did I leave unfinished questions I still want to chew on?
  - **Knowledge gap vs maturity** — manageable to deepen, or do I need a year of prerequisites first?
  - **PhD-targeting fit** — supervisors and venues exist; recognizable to admissions.
- [ ] **53** Pick one. Write the 2–3 page convergence memo containing:
  1. Chosen neighborhood + why (decisive, ~½ page).
  2. 2–3 candidate thesis-question shapes within it.
  3. 2–3 candidate international co-advisors in that neighborhood (no outreach yet — just the shortlist).
  4. The math toolkit needed; gap with current preparation.
  5. Phase 2 sketch (deep dive scope, paper target, ~6–9 month plan).
- [ ] **54** Rewrite identity memory files (`thesis_direction.md`, `phd_strategy.md`, and whatever `thesis_coalgebra_direction.md` becomes after renaming).
- [ ] **55** Begin Phase 2.

