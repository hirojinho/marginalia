# Software Architecture

## Before You Start (5 min)

- [ ] **1** Read: ADR format — Michael Nygard (cognitect.com/blog/2011/11/15/documenting-architecture-decisions, ~5 min)
  - The format: Title / Context / Decision / Status / Consequences
  - You will write one ADR per phase. This is the articulation training.

## Phase 1 — Vocabulary Foundation

### Design Patterns (GoF core)

- [ ] **2** Read: Strategy — Refactoring.Guru (~15 min)
  - Intent: extract family of algorithms into interchangeable objects
  - Hands-on: find a Strategy in your codebase, or write a minimal example
- [ ] **3** Read: Observer — Refactoring.Guru (~15 min)
  - Intent: subject notifies subscribers on state change without knowing who they are
  - Hands-on: find an Observer in your codebase (event bus, webhooks, pub-sub)
- [ ] **4** Read: Decorator — Refactoring.Guru (~20 min)
  - Intent: wrap an object to add behavior; composable at runtime
  - Hands-on: implement a Decorator pipeline (e.g., logging → auth → rate-limit around a handler)
- [ ] **5** Read: Command — Refactoring.Guru (~20 min)
  - Intent: encapsulate a request as an object; enables queuing, undo, history
  - Hands-on: model one agent tool call as a Command object — what is the invoker, receiver, command?
- [ ] **6** Read: Composite — Refactoring.Guru (~15 min)
  - Intent: treat individual objects and compositions uniformly; tree structures
- [ ] **7** Read: Adapter — Refactoring.Guru (~15 min)
  - Intent: convert one interface into another; makes incompatible things compatible
  - Hands-on: identify an Adapter in one of your microservices (a client wrapping an external API)

### Clean Architecture

- [ ] **8** Read: "The Clean Architecture" — Uncle Bob (blog.cleancoder.com, ~8 min)
  - The four rings: Entities → Use Cases → Interface Adapters → Frameworks & Drivers
  - The rule: source code dependencies point inward only, always
- [ ] **9** Hands-on: Sketch the layer diagram of one of your current microservices (~20 min)
  - Label each class/module by ring
  - Mark every place the dependency rule is violated (domain code importing from a framework = violation)
- [ ] **10** Read: "The Dependency Rule" in practice — find 1 blog post or example showing DIP at boundaries (~15 min)
  - Focus: what does an interface at the boundary look like in your language of choice?
- [ ] **11** Hands-on: Find one place where domain logic leaked into a controller or handler in your codebase (~15 min)
  - Write a 3-sentence note: what is it, why is it wrong, how would you fix it?

## Phase 2 — Elegant Structure

### Cluster A: Ports & Adapters (read together — they reinforce each other)

- [ ] **12** Read: "Hexagonal Architecture" — Alistair Cockburn (~20 min)
  - Port = technology-neutral interface; Adapter = technology-specific implementation
  - Left side (driven by tests/UI) vs. right side (drives DB/email/external services)
  - The key rule: the application never reaches outward through a port
- [ ] **13** Read: "Ready for Changes with Hexagonal Architecture" — Netflix Tech Blog (~12 min)
  - Production example: MovieRepository port + multiple adapter implementations
  - How they swapped adapters (REST → GraphQL → Kafka) without touching business logic
  - Read this immediately after Cockburn — it makes the abstract concrete

### Cluster B: Functional Architecture (read as a sequence — each builds on the prior)

- [ ] **14** Watch: "Functional architecture — the pits of success" — Mark Seemann, NDC (~60 min — split into 3 sessions of 20 min)
  - 0–20 min: pure functions as the default; impurity isolated at edges; Haskell IO as proof
  - 20–40 min: ports and adapters = functional architecture; they are the same insight
  - 40–60 min: dependency injection via partial application; F# and C# examples
- [ ] **15** Read: "Functional core, imperative shell" — Gary Bernhardt, BOUNDARIES talk (~45 min — or just read the concept summary, ~10 min)
  - Core = all decisions, pure functions, no side effects; Shell = thin, handles all I/O
  - The ratio: ~80% core, ~20% shell
- [ ] **16** Read: "Impureim sandwich" — Mark Seemann (blog.ploeh.dk, ~8 min)
  - The pattern: impure (gather) → pure (decide) → impure (write)
  - Limitation: when you can't separate IO from computation (this is where monadic composition comes in)
- [ ] **17** Read: "Railway Oriented Programming" — Scott Wlaschin (fsharpforfunandprofit.com, ~25 min)
  - Two-track design: success path and failure path as explicit types
  - Bind/map combinators for composing error-handling pipelines
  - Note: this is the Either monad — Wlaschin deliberately avoids that name for accessibility
- [ ] **18** Hands-on: Implement one use case as a pure pipeline with explicit error types (~30 min)
  - Pick a functional-friendly language: F#, Haskell, Rust, Scala, or TypeScript with fp-ts
  - The pipeline should have: input validation → business logic → persistence call — all explicit Result types

### Cluster C: Domain Modeling

- [ ] **19** Read: Domain Modeling Made Functional — Ch. 1: Introducing DDD (~45 min)
  - Ubiquitous language, bounded contexts, domain events — no code yet
- [ ] **20** Read: Ch. 2: Understanding the Domain (~45 min)
  - Working with domain experts; identifying workflows and data flows
- [ ] **21** Read: Ch. 3: A Functional Architecture (~35 min)
  - How DDD maps onto functional design; ports and adapters as the outer layer
  - *This chapter closes the loop: DDD + hexagonal + functional are the same architecture*
- [ ] **22** Read: Ch. 4: Understanding Types (~60 min)
  - Product types, sum types (discriminated unions), F# type system
- [ ] **23** Read: Ch. 5: Domain Modeling with Types (~70 min)
  - Making illegal states unrepresentable; value objects vs. entities; smart constructors
  - This is where the "constraints → elegant solutions" aesthetic lives most fully
- [ ] **24** Hands-on: Model one aggregate from your agents system using only types (~30 min)
  - No classes, no ORM, no framework — pure type definitions
  - Can the type system make invalid states impossible?
- [ ] **25** Read: "Bounded contexts" — Martin Fowler (martinfowler.com, ~10 min)
  - The strategic pattern: where does this model stop being valid?
- [ ] **26** Hands-on: Draw a bounded context map for one system you own (~20 min)
  - Label each context; label the relationships (conformist, anti-corruption layer, shared kernel)

## Phase 2.5 — Data Architecture

### Cluster A: The Persistence Problem

- [ ] **27** Read: "ORM Hate" — Martin Fowler (martinfowler.com, ~6 min)
  - The root cause: impedance mismatch between in-memory objects and relational rows is fundamentally hard
  - ORMs don't create the problem — they expose it
- [ ] **28** Read: Active Record catalog entry — Fowler PEAA (~2 min entry; skim ~10 min)
  - Object wraps a DB row; carries both data and DB access logic; schema = object
- [ ] **29** Read: Data Mapper catalog entry — Fowler PEAA (~2 min entry; skim ~10 min)
  - Separate layer maps between objects and DB; objects unaware of persistence
  - The contrast: use Active Record when domain ≈ schema; use Data Mapper when they must evolve independently
- [ ] **30** Read: Repository catalog entry — Fowler PEAA (~2 min entry; skim ~10 min)
  - Repository sits on top of Data Mapper; provides collection-like interface to domain
  - Domain layer depends on the Repository *interface*, not the implementation → dependency rule satisfied
- [ ] **31** Hands-on: Audit one service's data access layer (~20 min)
  - Is it Active Record, Data Mapper, or hybrid? Does the domain depend on persistence details?

### Cluster B: Aggregate Design Meets Persistence

- [ ] **32** Read: "Effective Aggregate Design, Part I" — Vaughn Vernon (~30 min)
  - Aggregate = consistency boundary around true invariants; root entity as sole access point
  - The "large cluster" antipattern: oversized aggregates cause contention and complexity
- [ ] **33** Read: "Effective Aggregate Design, Part II" — Vaughn Vernon (~30 min)
  - Reference aggregates by identity (not object reference); eventual consistency between aggregates
  - Domain events as the mechanism for cross-aggregate communication
- [ ] **34** Read: "Effective Aggregate Design, Part III" — Vaughn Vernon (~30 min)
  - Estimating aggregate boundaries from use case analysis
  - How tactical DDD (aggregate) serves strategic DDD (bounded context)
- [ ] **35** Hands-on: Take the aggregate you modeled in Phase 2 (types only) — now decide how to persist it (~25 min)
  - Write the schema (relational) OR event list (event-sourced) OR document structure
  - Justify the choice based on Vernon's consistency boundary rules

### Cluster C: Choosing the Right Store

- [ ] **36** Read: "Polyglot Persistence" — Martin Fowler (martinfowler.com, ~4 min)
  - Different data types are suited to different stores; service boundaries = storage boundaries
- [ ] **37** Read: NoSQL Distilled — Ch. 1: Why NoSQL? (~30 min)
  - Impedance mismatch, cluster scaling, relational model limitations
- [ ] **38** Read: Ch. 2: Aggregate Data Models (~40 min)
  - Aggregate as unit of interaction; ACID vs. BASE; aggregate orientation
  - *This chapter directly connects to Vernon's aggregate design — same concept, different community*
- [ ] **39** Read: Ch. 3: More Details on Data Models (~40 min)
  - Key-value, document, column-family, graph — when each fits
- [ ] **40** Read: Designing Data-Intensive Applications — Ch. 2: Data Models and Query Languages — Kleppmann (~90 min — split into 2 sessions)
  - Session 1: Relational vs. document model; many-to-one and many-to-many relationships; are document DBs repeating history?
  - Session 2: Graph-like data models; query languages — Cypher, SPARQL, Datalog
  - *This is the single best chapter on the relational/document/graph decision*
- [ ] **41** Read: DDIA — Ch. 3: Storage and Retrieval (first half only) — Kleppmann (~45 min)
  - SSTables and LSM-trees vs. B-trees — the intuition, not the implementation
  - Why LSM = good for writes, B-tree = good for reads; how this shapes your DB choice
- [ ] **42** Reflection: Build your personal decision matrix (~20 min)
  - For each DB type (relational, document, key-value, column-family, graph, vector), write 1 sentence: "Use this when..."

### Cluster D: Vector Stores and Agent Data

- [ ] **43** Read: "What is a vector database?" — Pinecone or Weaviate intro docs (~15 min)
  - Embeddings as the data model; approximate nearest neighbor search
- [ ] **44** Reflection: In your in-house agents, where is retrieval happening? (~15 min)
  - Keyword, semantic, or hybrid? What stores are in play? What would you change knowing what you know now?
- [ ] **45** Hands-on: Design the data layer for one of your agents — on paper (~25 min)
  - List every store needed and justify each choice using your decision matrix

## Phase 3 — Distributed Thinking

### Cluster A: Event-Driven Disambiguation (read first — it clarifies everything else in this phase)

- [ ] **46** Read: "What do you mean by Event-Driven?" — Martin Fowler (~9 min)
  - The four distinct patterns often conflated under "event-driven":
  1. Event Notification (no data payload, loose coupling, low observability)
  2. Event-Carried State Transfer (full data, no callback, replication tradeoff)
  3. Event Sourcing (event as system of record)
  4. CQRS (model split, not inherently event-based)
  - Read this before CQRS and Event Sourcing articles — it disambiguates both

### Cluster B: CQRS and Event Sourcing

- [ ] **47** Read: "CQRS" — Martin Fowler (~6 min)
  - Separate command model (writes) from query model (reads); attributed to Greg Young
  - The article is deliberately cautionary: CQRS adds complexity; only justified by domain complexity
- [ ] **48** Read: "Event Sourcing" — Martin Fowler (~18 min)
  - Event log as source of truth; complete rebuild, temporal query, event replay
  - Snapshotting; idempotency concerns during replay; side effects on external systems
- [ ] **49** Hands-on: Design an event-sourced version of one entity in your current system (~25 min)
  - What events exist? What is the aggregate state after replaying them?
  - What projections / read models would you build from that event stream?

### Cluster C: Distributed Patterns Catalog

- [ ] **50** Read: Patterns of Distributed Systems — intro catalog page — Unmesh Joshi (martinfowler.com, ~10 min)
  - Get the lay of the land; 30 patterns, each a standalone article
- [ ] **51** Read: Write-Ahead Log (~20 min)
  - Annotate: where have you seen this in your systems (Postgres WAL, Kafka log)?
- [ ] **52** Read: Replicated Log (~20 min)
  - Annotate: what service in your stack implements this?
- [ ] **53** Read: Leader and Followers (~20 min)
  - Annotate: primary/replica in your DB, leader election in your message broker
- [ ] **54** Read: Two-Phase Commit (~20 min)
  - Concept only; annotate: why don't we use this in microservices?
- [ ] **55** Reflection: For each pattern, write 1 sentence: "I've seen this as X in my systems" (~15 min)

### Cluster D: Sagas

- [ ] **56** Read: "Effective Aggregate Design, Part II" — revisit the domain events section (~10 min, already read)
  - Notice: domain events between aggregates = the same mechanism as saga events between services
- [ ] **57** Read: Saga pattern — Chris Richardson (microservices.io, ~10 min)
  - Context: database-per-service makes cross-service transactions need a substitute for 2PC
  - Choreography-based saga (services react to events) vs. orchestration-based saga (central coordinator)
  - Compensating transactions; lack of isolation; Transactional Outbox as required companion
- [ ] **58** Hands-on: Map one multi-service workflow in your system to a Saga (~25 min)
  - Which services participate? What are the compensating transactions if step 3 fails?
  - Is it choreography or orchestration in your current impl? Which would you prefer and why?

## Phase 4 — Agents Architecture

### Cluster A: Your Agent as an Integration Architecture

- [ ] **59** Read: "Building effective agents" — Anthropic (anthropic.com/engineering, ~20 min)
  - Workflows (predefined) vs. Agents (LLM-directed)
  - Five workflow patterns: prompt chaining, routing, parallelization, orchestrator-workers, evaluator-optimizer
  - When NOT to use agents: start with the simplest thing that works
- [ ] **60** Read: Pipes and Filters — EIP, Gregor Hohpe (~20 min)
  - Chain of independent processors connected by channels; each step = a filter; channels = the pipes
  - Reflection: is your agent's tool-calling pipeline a Pipes and Filters pattern?
- [ ] **61** Read: Process Manager — EIP (~30 min)
  - Central component maintains state of a multi-step workflow; routes messages based on a state machine
  - Enables stateful orchestration; compensating actions for failure
  - Reflection: is your agent orchestrator a Process Manager? What state does it hold?
- [ ] **62** Read: Correlation Identifier — EIP (~15 min)
  - Embed a unique ID in each request; return it in the reply to match async responses
  - Reflection: how do you correlate tool responses back to the right agent turn?
- [ ] **63** Reflection: Map EIP patterns onto your in-house agent architecture (~20 min)
  - What did you build that corresponds to Pipes and Filters? Process Manager? Correlation Identifier?
  - What did you reinvent without knowing the name?

### Cluster B: Formalizing the Loop

- [ ] **64** Read: ReAct paper — Yao et al. (arXiv:2210.03629, ~60 min — split into 2 sessions)
  - Session 1 (~30 min): Abstract + Introduction + Section 3 (knowledge-intensive reasoning)
  - The Thought/Action/Observation loop; how it reduces hallucination via external grounding
  - Session 2 (~30 min): Section 4 (decision making) + Discussion + Related Work
  - Limitations: reasoning not always useful; finite context; interpretability tradeoffs
- [ ] **65** Hands-on: Draw a formal state machine for one of your current agents (~30 min)
  - States: Thinking / Acting / Observing / Done / Failed
  - Transitions: what triggers each? Where does state leak or become implicit?
- [ ] **66** Reflection: Compare your in-house loop to ReAct (~15 min)
  - What did you converge on independently? What did you do differently? What did the paper get wrong for your use case?

### Cluster C: Tool Use as Architecture

- [ ] **67** Hands-on: Audit your tool-use layer (~25 min)
  - Does each tool follow the Adapter pattern? (Tool interface = port; LLM-facing wrapper = adapter)
  - Is the tool's interface technology-neutral, or does it leak implementation details to the LLM?
  - Refactor one tool to make the boundary explicit

### Cluster D: Reliability

- [ ] **68** Read: Circuit Breaker — Martin Fowler (martinfowler.com, ~15 min)
  - Prevents calling a failing service repeatedly; three states (Closed / Open / Half-Open)
- [ ] **69** Read: Retry with exponential backoff — any focused article (~10 min)
  - Applied specifically to LLM API calls: when to retry vs. when to fail fast
- [ ] **70** Hands-on: Reliability audit of one agent (~25 min)
  - What happens on: LLM timeout? Rate limit? Malformed JSON output? Tool failure? Missing memory?
  - Map the gaps. Which failure mode is most dangerous in production?
- [ ] **71** Hands-on: Implement one reliability improvement (~30 min)
  - Options: structured output validation with explicit error state, circuit breaker around LLM calls, fallback to simpler workflow, idempotent tool calls

## Phase 5 — Articulation (ongoing throughout)

- [ ] **72** Read ADR format (done before Phase 1)
- [ ] **73** Phase 1 ADR: Design decision using GoF + dependency rule vocabulary
- [ ] **74** Phase 2 ADR: Internal service structure choice (hexagonal / functional core / layered)
- [ ] **75** Phase 2.5 ADR: Data store choice using aggregate boundaries + access patterns
- [ ] **76** Phase 3 ADR: Service communication decision (sync/async, events, saga style)
- [ ] **77** Phase 4 ADR: Agent orchestration architecture
- [ ] **78** Optional reference: *Fundamentals of Software Architecture* — Richards & Ford
  - Use as vocabulary lookup, not cover-to-cover
  - High-value chapters: Ch. 1 (what architecture is), Ch. 3 (modularity, connascence), Ch. 4–5 (architecture characteristics), Ch. 17 (microservices), Ch. 19 (ADRs), Ch. 21 (diagramming)

