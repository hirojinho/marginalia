# DSA Interview — Curiosity Log

Topics that came up during problem walkthroughs and sparked curiosity beyond the interview-prep scope.
Not active tasks — a backlog for future exploration.

---

- **Order reconstruction from sorted observations (a.k.a. "learning a hidden total order")** — came up while studying 269. Alien Dictionary. Eduardo asked: what is the general class of problems where pairwise comparison between *adjacent* sorted items is enough to extract all extractable atomic-order information? We worked out a formal answer. The class is characterized by three properties of the compound-to-atomic order lifting $\ell : \mathcal{O}(A) \to \mathcal{O}(W(A))$:
  1. **Single-witness:** every compound comparison $w_1 <_W w_2$ is witnessed by a unique atomic pair $(a_1, a_2)$ with $a_1 <_A a_2$. (For lex: the characters at the first differing position.)
  2. **Monotonicity:** extending the atomic order only extends the compound order.
  3. **Degenerate cases decidable from $w_1, w_2$ alone** (e.g.\ the prefix rule).

  Under these, the set of atomic orders consistent with a sorted sequence $\bar w$ equals the set of orders extending the transitive closure of *direct* adjacent-pair constraints. The theorem is proved in two lines (soundness is trivial; completeness: if $<'_A$ contains all direct constraints, then $\bar w$ is automatically sorted under $\ell(<'_A)$, so no "hidden" constraint is forced). Topological sort then computes a linear extension of the minimum element of this up-set in $\mathcal{O}(A)$.

  **Areas of CS that handle this kind of problem:**
  1. **Order theory / poset theory** — the mathematical backbone. Linear extensions, up-sets in $\mathcal{O}(A)$, Dilworth-style combinatorics. Entry point: Davey & Priestley, *Introduction to Lattices and Order*.
  2. **Combinatorics on words** — lex order on free monoids $A^*$, prefix trees, Sturmian and Lyndon words. Entry point: Lothaire, *Combinatorics on Words*.
  3. **Computational learning theory (COLT), specifically "learning from queries" and "exact learning of orderings"** — Angluin-style query learning, Gold's identification in the limit. The alien dictionary input is a *passive sorted-example oracle* for the hidden alphabet order. Named subfield: **order learning / rank aggregation**.
  4. **Sorting from partial/noisy comparisons** — classical algorithmic order theory: Ford-Johnson, sorting under partial information (Fredman's lower bound), sorting from a pre-sorted sample. Closely connects the interview-style problem to a deep algorithmic literature.
  5. **Constraint satisfaction / model theory (logic sense)** — "what alphabet orders satisfy these constraints?" is a CSP. The completeness proof is exactly a soundness-and-completeness argument for a constraint-extraction procedure; the up-set structure is the model class.
  6. **Coalgebra and final coalgebras** — the lex comparator on $A^\omega$ is a coinductive function on a final coalgebra for $X \mapsto A \times X$; the single-witness property is the statement that the comparator terminates at the first step where it can. Direct methodological tie to Eduardo's thesis direction (coalgebraic semantics for STAMP). Entry point: Jacobs, *Introduction to Coalgebra*, §2–3.
  7. **Formal language theory / trie structures** — the prefix-rule invalidity (\texttt{["abc","ab"]}) is the condition that a word's position in the sort is consistent with its trie depth; this is the starting point for work on *prefix-sorted structures* (FM-index, Burrows-Wheeler transform, $r$-index).

  **The "right" home** for the formal theorem itself is probably **algorithmic order theory + COLT**: the object is a hidden order, the mechanism is adjacency-based extraction, the goal is efficient and complete characterization of the model class. The coalgebraic framing is structurally satisfying but over-powered for the finite-word case.

  **Threads worth pulling later:**
  - Generalize the theorem to other single-witness compound orders (tuples with priority, radix orderings) — does the up-set / linear-extension characterization survive?
  - Rank aggregation: what if adjacent comparisons can be *inconsistent* (noisy), and we want the most-likely atomic order? Connects to Kemeny-Young ranking, a classical NP-hard problem. The interview problem is the *noise-free exact* special case.
  - Connection to sorting-network lower bounds: adjacency-based extraction is similar in spirit to how sorting networks propagate order information through adjacent swaps.

---

- **Algebra of Programming / recursion schemes — the formal study of algorithm *classes*** — came up while studying 427. Construct Quad Tree. Eduardo asked: what area formally studies "extract the class of algorithm and prove it works?" The clean answer is **algebra of programming** (Bird-Meertens / Squiggol tradition): algorithms as morphisms on recursive data types, correctness by equational reasoning using universal properties of initial algebras.

  **Concrete link to 427:** the `Node` type is the initial algebra of the functor $F(X) = \mathsf{Bool} + X^4$. The algorithm is a **hylomorphism** — an anamorphism (grid $\to$ implicit full cell tree) composed with a catamorphism (tree $\to$ compacted quad tree via the merge algebra). The "recurse to leaves, merge on the way up" observation is the **hylomorphism fusion law**; the merge predicate is the algebra $F(\mathsf{Node}) \to \mathsf{Node}$. Correctness follows from the algebra's definition, not from per-problem induction.

  **Reading list (ordered for Eduardo's background):**
  1. Meijer, Fokkinga, Paterson — *Functional Programming with Bananas, Lenses, Envelopes and Barbed Wire* (1991). ~20pp. Canonical entry point: defines cata/ana/hylo/para and proves fusion laws.
  2. Bird & de Moor — *Algebra of Programming* (1997). Textbook. Chapters on Datatypes, Recursive programs, Optimization problems. Derives sorting, graph search, divide-and-conquer, DP as morphism equations.
  3. Bird — *Pearls of Functional Algorithm Design* (2010). Thirty worked derivations from spec to implementation by equational calculation. Several tree/quad-tree-adjacent.
  4. Hinze, Wu, Gibbons — *Unifying Structured Recursion Schemes* (2013). Modern categorical survey. Covers the full zoo (cata/ana/hylo/para/apo/zygo/histo/futu) via distributive laws. Direct bridge to coalgebra.
  5. Backhouse — *Program Construction: Calculating Implementations from Specifications* (2003). Heavier on derivation discipline, lighter on category theory.

  **Adjacent neighborhoods:**
  - **Recursion schemes** (practical face of AoP) — Haskell's `Data.Functor.Foldable`, Agda's $\mu$-types. Hinze / Milewski / Gibbons blog literature.
  - **Datatype-generic programming** (Jansson, Jeuring, Gibbons) — fold/map/traverse parameterized by functor.
  - **Deforestation / fusion** (Wadler, Chin, Gill) — theorem justifying "two passes can be fused."
  - **Algorithm synthesis à la Kestrel (KIDS)** — Doug Smith, *The Design of Divide-and-Conquer Algorithms*. Specification-refinement tradition, parallel goal.
  - **Coalgebra (final coalgebras + ana/apo)** — direct dual of the cata side; dovetails with thesis direction on coalgebraic STAMP semantics.

  **Interview-problem lens (immediate payoff):**
  | Pattern | Categorical name |
  |---|---|
  | Tree recursion combining child results | catamorphism on tree functor |
  | Top-down structure build | anamorphism |
  | Build-then-consume that fuses (DP, memoized recursion) | hylomorphism |
  | Recursion using intermediate results | paramorphism / histomorphism |
  | DP over subproblem DAG | catamorphism on that DAG |

  **Threads worth pulling later:**
  - Is DP = "catamorphism on subproblem DAG" exact or approximate? (Hinze et al. have a clean version via histomorphism.)
  - What is the categorical status of greedy algorithms? (Matroid theory, Bird-de Moor chapter.)
  - Coalgebraic duality: which interview algorithms are *ana*- rather than cata-morphic? (Process-like / observation-like problems.)
  - Is there a mechanical way to go from "I see a recursion on structure X" to "that structure is the initial algebra of functor F, and this algorithm is a fold with algebra $\varphi$"? Yes: this is the core exercise of Bird's *Pearls* --- worth making it a reflex.
  - The fusion theorem as a decision procedure: when does "build intermediate + consume" fuse into a single pass? Maps directly onto many interview optimizations.

---
