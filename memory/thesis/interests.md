# Thesis — Curiosity Log

Tangents, gaps, and questions that surface during Phase 1 reading. Not blockers; not active tasks. Each entry is a fleeting prompt for later — most likely Phase 2 (after the neighborhood is chosen) or for the eventual formal write-up.

Format: `- [Topic] — [why it came up, suggested entry point if obvious]`

---

## Gaps surfaced (carried forward from previous direction)

- **Syntax trees / ASTs as a formal object** — surfaced reading Plotkin & Pretnar §1–2 in the previous (now superseded) PL/DB direction. Eduardo has informal CS background, no compilers course; the "computation as a tree built from operation nodes" framing is fresh. *In the new direction, this is even more relevant for cluster 2 (type theory)*: Pierce TaPL ch. 3 "Untyped Arithmetic Expressions" is the natural warm-up for the AST-as-formal-object idea before ch. 5 (untyped λ-calculus). Recommend reading ch. 3 as ~10-page warm-up if syntax/AST formalism feels unfamiliar.

## Conceptual threads to follow later

- **Chess and EXPTIME-completeness** — surfaced watching hackerdashery's pop-sci P-vs-NP video (Cluster 1, week 1). Pop-sci framing said "chess is too big"; the real distinction is between (a) standard 8×8 chess being *finite* (decidable trivially with unbounded compute, constant-time in the formal sense), and (b) generalized n×n chess being EXPTIME-complete (Fraenkel & Lichtenstein 1981) — one of the few cases where we have an *unconditional* lower bound, since P ⊊ EXPTIME is provably known. Entry points: Fraenkel-Lichtenstein 1981 paper; Aaronson lecture notes on game complexity; Papadimitriou Ch. 19 on games and complexity. Worth revisiting after Cluster 1 to feel the contrast: NP-completeness is *conditional* hardness (only matters if P ≠ NP), whereas EXPTIME-completeness gives *unconditional* hardness. The "conditional vs unconditional separation" thread is itself worth pulling on.
- **Where exactly is the P / exp line, and why P specifically?** — Pop-sci taxonomy felt loose. Real answer: the polynomial-time cutoff is the *finest* invariant cutoff under reasonable model changes (Cobham–Edmonds thesis: P-on-TM = P-on-RAM = P-on-λ up to polynomial overhead). Below P, distinctions become model-dependent. n^100 is "in P" but useless — galactic algorithms exist. The cleanness is theoretic, not pragmatic. Worth revisiting in Cluster 4 (descriptive complexity), where Fagin's theorem characterizes NP without referencing a machine model at all — a different, logical justification for the same cutoff.
- **Automata hierarchy and weaker computational tiers (Sipser ch. 1–2)** — surfaced reading Sipser §3.1 (Cluster 1, week 1); strong felt pull toward "other computational models." **Reopened reading §7.2 (2026-05-02)**: Eduardo named context-free languages explicitly as a long-standing curiosity ("I don't understand what CFLs are in the Chomsky hierarchy, but that's an interest I had for some time"). The §7.2 result that *every* CFL is in P is the trigger — connects the language-theoretic hierarchy (regular ⊊ CF ⊊ decidable) to the complexity hierarchy. Entry points: Sipser ch. 1 (DFAs / NFAs / regular langs / regex), ch. 2 (CFGs / pushdown automata), Hopcroft-Ullman *Introduction to Automata Theory* for the canonical Chomsky-hierarchy treatment. Same "model + language it recognizes" frame as TMs but with provable expressibility limits — and the hierarchy itself is exactly the kind of meta-relational structure Eduardo's hypothesis predicts will land.
- **Proof techniques for decidability / recognizability of languages** — surfaced reading Sipser §3.1 (Cluster 1, week 1); explicit ask to "learn how to show those properties for a given language." Two complementary techniques: (a) constructive — build a TM that decides/recognizes; (b) reductive — reduce from a known (un)decidable language. Entry: Sipser §3.2 (model equivalence by simulation) → ch. 4 (the halting problem and reductions). The reduction template here is structurally the *same* one that powers NP-completeness reductions later this cluster — worth feeling the continuity.
- **"State diagram is an algorithm" — the specification/computation collapse** — surfaced formalizing a TM in §3.1 (Cluster 1, week 1). The transition function *is* the program; the formal model collapses spec and execution into one object. Likely a recurring Phase-1 theme: it returns in Cluster 2 (λ-term as program, β-reduction as execution; Wadler 2015 names this explicitly via Curry–Howard), Cluster 5 (process calculi — process syntax = operational behavior), and inverts in Cluster 4 (descriptive complexity, where the "model" disappears and only the logical formula remains). Track in felt-data notes which cluster makes this collapse feel deepest — useful convergence signal.
- **Proofs of finite describability / encodability boundaries** — surfaced reading Sipser §3.3 (Cluster 1, week 1); strongest single felt-pull this session ("really interested"). Open question: what does it formally mean to *prove* an object can or cannot be encoded as a string? Three threads inside it: (a) cardinality arguments (Cantor: |ℝ| > |Σ*|, so most reals aren't encodable; only countably many TMs but uncountably many functions ℕ→ℕ); (b) constructive encodings (rationals, algebraic, computable reals — what makes a real "describable"); (c) the meta-question of what "describable" *means* formally — finite description, effective procedure, or something stronger? Entry points: Sipser ch. 4 (Cantor's diagonal in CS form, A_TM undecidability); Boolos & Jeffrey *Computability and Logic* ch. 1–3 for the foundational layer; Rogers *Theory of Recursive Functions* for the deep frame. Connects forward to Cluster 4 (descriptive complexity pins "definable" without a machine model — a completely different formalization of describability) and Cluster 2 (type inhabitation is a sibling describability question — "is there a term of type T?").
- **The syntactic / semantic decidability gap for TM properties** — surfaced reading Sipser §3.3 (Cluster 1, week 1). "Is ⟨M⟩ a well-formed TM?" is decidable (parse the encoding); "what does M do?" is generally undecidable (A_TM, halting, emptiness). Rice's theorem (ch. 4) makes this razor-sharp: every nontrivial *semantic* property of TMs is undecidable. Entry points: Sipser §4.1–4.2 + Rice's theorem; Sipser §5 for reduction-based undecidability proofs. The same syntactic/semantic gap returns in Cluster 2 (type-checking decidable, β-equivalence undecidable) — worth noting which proof template you used in each case.

---

## Patterns observed (working hypothesis — verify by end of Cluster 1)

The interests logged so far share a shape: **the proof-machinery of formal definability, encoding, and equivalence**, one level above the content itself.

- "How do we *show* a language is decidable?" — proof-technique question, not problem question.
- "How do we *show* an object can be encoded?" — describability question, not encoding question.
- "How do we *show* two models are equivalent?" — meta-relational question.
- "State diagram = algorithm" — interest in the formal collapse itself, not in any specific algorithm.

**Working hypothesis:** Eduardo is gravitating toward the meta-level proofs that *anchor* a formal definition — the step *after* the definition is given, where you have to establish it captures what it's supposed to and relates correctly to other formal objects. This territory: logic / model theory (definability), proof theory, foundations of mathematics, descriptive complexity (Cluster 4 — exactly the bridge field).

Felt-data layer, Eduardo's own words (2026-05-01): *"this kind of property and definition formalizations and proofs is a scratch that i dont seem to itch yet"* — interpretable as desire-without-muscle: drawn to the work, conscious of needing more tools. This is exactly the positive felt-data pattern Phase 1 is designed to detect.

**Test for this hypothesis:** if it holds through Cluster 2 (subject reduction is precisely this kind of meta-proof) and especially Cluster 4 (descriptive complexity is *entirely* meta-relational definability), it's a strong convergence signal toward the logic-meets-computation neighborhood. If Cluster 3 (combinatorial optimization, where the proofs are about objects, not about the machinery) feels duller by comparison, that's confirming evidence.

**Cluster-1 mid-cluster check (2026-05-02, after §7.1 + §7.2):** strongly confirming. Eduardo's own words on the session:

- *Boring:* practicality of algorithm analysis; proofs of model equivalence ("kinda boring, although the result is very interesting"); proofs that specific algorithms are in P ("mechanical — just find an algo"); P-class definition justified by "usefulness."
- *Interesting:* the *result* that decidability-equivalent TMs differ in complexity; the polynomial-invariance result itself; what the existence of a poly-time algorithm *says about* a problem.

The signature is the same one logged from §3.1–§3.3: meta-results alive, construction-machinery dull. Two new flavors confirm it sharpens further:

1. **Pragmatic justification falls flat too.** P-as-useful (Cobham–Edmonds engineering frame) didn't land. He wants the *structural* justification for the cutoff (which Cluster 4 provides via Fagin: NP characterized model-free as ∃SO).
2. **"What it says about" the problem.** The interest is not the proof and not the algorithm — it's the *meaning* of "X is in P" as a structural fact about X. This is squarely in the descriptive-complexity / model-theoretic flavor.

Cluster 4 prediction sharpens: expect strong fit. Cluster 3 prediction also sharpens: expect *negative* felt-data, since matroid-greedy proofs are exactly the construction-machinery he keeps reporting as dull.
