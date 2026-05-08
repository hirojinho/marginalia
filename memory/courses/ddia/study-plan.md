# Designing Data-Intensive Applications

## Phase 1 — Foundations & Data Modeling

### Tasks

- [x] **1** 1.1 Read: Ch.1 "Reliable, Scalable, and Maintainable Applications" — sections "Thinking About Data Systems" and "Reliability" (~30 min). _Completed 2026-04-10. Notes: data systems as compositions of specialized tools; fault vs. failure distinction; fault taxonomy (hardware = redundancy, software = isolation/testing, human = abstractions/sandboxes)._
- [x] **2** 1.2 Read: Ch.1 — section "Scalability" (~25 min). _Completed 2026-04-10. Notes: "scalable" requires (i) a chosen load parameter and (ii) a chosen percentile; Twitter fan-out hybrid driven by follower-count skew, not tweets/sec; percentiles as distinct diagnostic instruments; Amazon p99.9 cutoff (slowest customers are most valuable; p99.99 measures noise)._
- [x] **3** 1.2b Read: Ch.1 — section "Maintainability" (~10 min). Covers operability, simplicity, and evolvability — the third leg of the triangle. Needed before the 1.3 reflection. _Completed 2026-04-13. Notes: essential vs accidental complexity; abstraction reduces accidental complexity but requires shared mental model to preserve evolvability; the three properties are interdependent._
- [x] **4** 1.3 Reflect: Think about Brendi's WhatsApp bot system. Where does it sit on the reliability/scalability/maintainability triangle? Which dimension causes the most pain today? Write 3-5 sentences. (~20 min) _Completed 2026-04-13. Notes: scalability handled by cloud functions; reliability a concern due to LLM non-determinism; maintainability is the sharpest problem — lack of shared patterns drives drift and limits evolvability._
- [x] **5** 1.4 Read: Ch.2 "Data Models and Query Languages" — sections "Relational Model Versus Document Model" through "Are Document Databases Repeating History?" (~35 min). This is your pain point. Focus on the object-relational mismatch and many-to-many relationships — compare against how you model data in Firebase. _Completed 2026-04-14. Notes: object-relational mismatch; document model locality advantage + schema-on-read flexibility; many-to-many wall kicks join problem to application layer; IMS→CODASYL→relational historical arc; god entity as dual failure mode (query complexity + semantic degradation). Brendi's Conversation type is a concrete case of both._
- [x] **6** 1.5 *(light-touch survey)* Read: Ch.2 — sections "Relational Versus Document Databases Today" and "Query Languages for Data" (~20 min). _Completed 2026-04-20. Notes already captured in `notes/ddia.tex`._
- [x] **7** 1.6 *(light-touch survey)* Read: Ch.2 — section "Graph-Like Data Models" through end (~20 min). Know property graphs / triple stores exist and what they buy you. Skim Cypher/Datalog syntax. Thin notes.
  - Completed 2026-05-06.
- [ ] **8** 1.7 *(light-touch survey)* Read: Ch.4 "Encoding and Evolution" — sections "Formats for Encoding Data" through "Avro" (~25 min). Encoding formats + forward/backward compatibility matter for infra work — slightly more depth than 1.5/1.6 but still survey mode.
- [ ] **9** 1.8 *(light-touch survey)* Read: Ch.4 — sections "The Merits of Schemas" and "Modes of Dataflow" through end (~20 min). REST/RPC/message-passing vocabulary. Thin notes.

## Phase 2 — Storage Engine Internals

### Tasks

- [ ] **10** 2.1 Read: Ch.3 "Storage and Retrieval" — section "Data Structures That Power Your Database": Hash Indexes (~30 min). Start simple: an append-only log with an in-memory hash map. This is literally how some key-value stores work.
- [ ] **11** 2.2 Read: Ch.3 — section "SSTables and LSM-Trees" (~35 min). This is how LevelDB, RocksDB, and Cassandra work under the hood. Understand the compaction trade-off (space amplification vs write amplification).
- [ ] **12** 2.3 Read: Ch.3 — section "B-Trees" and "Comparing B-Trees and LSM-Trees" (~35 min). The core trade-off: LSM-trees optimize for writes, B-trees for reads. Think about which your Firebase workloads favor.
- [ ] **13** 2.4 Read: Ch.3 — section "Other Indexing Structures" (~25 min). Multi-column indexes, full-text search, in-memory databases. Skim what's less relevant, but note covering indexes and multi-column indexes for later.
- [ ] **14** 2.5 *(light-touch survey — Camp B)* Read: Ch.3 — sections "Transaction Processing or Analytics?" and "Data Warehousing" (~20 min). OLTP vs OLAP vocabulary. Enough to know why "can we run analytics on production?" is a loaded question. Thin notes.
- [ ] **15** 2.6 *(light-touch survey — Camp B)* Read: Ch.3 — section "Column-Oriented Storage" through end (~20 min). Column compression, sort order, materialized views — know they exist and what they buy you. Thin notes. Sets up Phase 6 vocabulary.

## Phase 3 — Transactions & Correctness

### Tasks

- [ ] **16** 3.1 Read: Ch.7 "Transactions" — sections "The Slippery Concept of a Transaction" and "The Meaning of ACID" (~30 min). ACID is not as precise as people think. Focus on how Kleppmann distinguishes each letter — especially isolation.
- [ ] **17** 3.2 Read: Ch.7 — section "Single-Object and Multi-Object Operations" and "Handling Errors and Aborts" (~25 min). Atomicity at the single-object vs multi-object level. Think about Firebase's atomic operations vs what a relational DB offers.
- [ ] **18** 3.3 Read: Ch.7 — section "Weak Isolation Levels": "Read Committed" (~30 min). The most basic useful isolation level. Understand dirty reads and dirty writes — these are bugs you can have *right now* without knowing it.
- [ ] **19** 3.4 Read: Ch.7 — section "Snapshot Isolation and Repeatable Read" (~35 min). MVCC explained clearly. This is the default in PostgreSQL. Understand why "repeatable read" means different things in different databases.
- [ ] **20** 3.5 Read: Ch.7 — section "Preventing Lost Updates" (~30 min). Atomic writes, explicit locking, compare-and-set. Directly applicable to concurrent order updates in your system.
- [ ] **21** 3.6 Read: Ch.7 — section "Write Skew and Phantoms" (~35 min). The most subtle concurrency bug. Kleppmann's examples are excellent — meeting room booking, on-call doctors. Think about analogous scenarios in order processing.
- [ ] **22** 3.7 Read: Ch.7 — section "Serializability" through end (~40 min). Three approaches: actual serial execution, 2PL, SSI. This is the formal models content you enjoy. Understand why SSI is the exciting recent development.
- [ ] **23** 3.7v Watch: Martin Kleppmann — "Transactions: myths, surprises and opportunities" (Strange Loop 2015, ~45 min). Covers the same ground as Ch.7 but with live examples and audience questions. Good reinforcement after reading.

## Phase 4 — Replication & Partitioning

### Tasks

- [ ] **24** 4.1 Read: Ch.5 "Replication" — section "Leaders and Followers" through "Synchronous Versus Asynchronous Replication" (~30 min). The basic model. Understand the fundamental trade-off: synchronous = durable but slow; asynchronous = fast but can lose data.
- [ ] **25** 4.2 Read: Ch.5 — sections "Setting Up New Followers" and "Handling Node Outages" (~25 min). Follower catch-up and leader failover. Think about what happens to in-flight WhatsApp messages during a failover.
- [ ] **26** 4.3 Read: Ch.5 — section "Implementation of Replication Logs" (~25 min). WAL shipping, logical logs, statement-based. How the sausage is made.
- [ ] **27** 4.4 Read: Ch.5 — section "Problems with Replication Lag" (~35 min). Read-your-own-writes, monotonic reads, consistent prefix reads. These are real bugs — you've probably shipped them without knowing.
- [ ] **28** 4.5 Read: Ch.5 — section "Multi-Leader Replication" (~35 min). Conflict handling, topologies. The offline-capable clients use case is relevant to mobile-first products.
- [ ] **29** 4.6 Read: Ch.5 — section "Leaderless Replication" through end (~40 min). Dynamo-style quorums, sloppy quorums, concurrent write detection. The "happens-before" relationship and version vectors — this is where formal models start appearing.
- [ ] **30** 4.7 Read: Ch.6 "Partitioning" — sections "Partitioning of Key-Value Data" through "Skewed Workloads and Relieving Hot Spots" (~30 min). Key-range vs hash partitioning. Think about how your order IDs distribute across partitions.
- [ ] **31** 4.8 Read: Ch.6 — sections "Partitioning and Secondary Indexes" through "Request Routing" (~30 min). Document-partitioned vs term-partitioned secondary indexes, rebalancing strategies, service discovery. Completes the distribution picture.

## Phase 5 — Distributed Systems Theory

### Tasks

- [ ] **32** 5.1 Read: Ch.8 "The Trouble with Distributed Systems" — sections "Faults and Partial Failures" and "Unreliable Networks" through "Detecting Faults" (~30 min). The fundamental problem: partial failure. Unlike a single machine, a distributed system can be half-broken.
- [ ] **33** 5.2 Read: Ch.8 — sections "Timeouts and Unbounded Delays" and "Synchronous Versus Asynchronous Networks" (~30 min). Why you can't reliably detect failures. Network congestion, queueing, the impossibility of bounded timeouts.
- [ ] **34** 5.3 Read: Ch.8 — section "Unreliable Clocks" through "Process Pauses" (~35 min). Time is not what you think it is. Monotonic vs time-of-day clocks, clock skew, GC pauses. This changes how you think about timestamps in logs.
- [ ] **35** 5.4 Read: Ch.8 — section "Knowledge, Truth, and Lies" through end (~35 min). Fencing tokens, Byzantine faults, system models. The formal framework for reasoning about what a distributed system can guarantee.
- [ ] **36** 5.5 Read: Ch.9 "Consistency and Consensus" — sections "Consistency Guarantees" and "Linearizability" (~40 min). The strongest consistency guarantee. Understand what it means formally and why it's expensive. This is the formal models content you want.
- [ ] **37** 5.6 Read: Ch.9 — section "Ordering Guarantees" through "Total Order Broadcast" (~35 min). Causality, Lamport timestamps, the connection between ordering and consistency. Formal and elegant.
- [ ] **38** 5.7 Read: Ch.9 — section "Distributed Transactions and Consensus": "Atomic Commit and Two-Phase Commit (2PC)" and "Distributed Transactions in Practice" (~35 min). 2PC explained clearly — its guarantees and its failure modes.
- [ ] **39** 5.8 Read: Ch.9 — section "Fault-Tolerant Consensus" through end (~40 min). Raft, epoch numbering, limitations of consensus. The crown jewel of distributed systems theory. Take your time here.
- [ ] **40** 5.8v Watch: MIT 6.5840 Lecture 6 — "Fault Tolerance: Raft (1)" (~75 min, split into two sessions if needed). Complements Ch.9's consensus discussion with the actual Raft paper walkthrough.

## Phase 6 — Stream & Batch Processing

### Tasks

- [ ] **41** 6.1 Read: Ch.10 "Batch Processing" — sections "Batch Processing with Unix Tools" through "The Unix Philosophy" (~30 min). Unix pipes as the mental model for batch processing. Simple and powerful framing.
- [ ] **42** 6.2 Read: Ch.10 — section "MapReduce and Distributed Filesystems" through "Reduce-Side Joins and Grouping" (~35 min). MapReduce explained properly. The join patterns (sort-merge, broadcast, partitioned) are important for Phase 6.
- [ ] **43** 6.3 Read: Ch.10 — sections "Map-Side Joins" through "Comparing Hadoop to Distributed Databases" (~30 min). Deep on the join mechanics (sort-merge, broadcast, partitioned); light on the "philosophy of batch outputs" framing. Under new lens: you care about *how* the join works, not the Unix-philosophy sermon.
- [ ] **44** 6.4 Read: Ch.10 — section "Beyond MapReduce" through end (~30 min). Dataflow engines (Spark, Flink), graph processing. Focus on what Spark/Flink *do differently* at the engine level — DAG execution, in-memory shuffle, materialization boundaries.
- [ ] **45** 6.5 Read: Ch.11 "Stream Processing" — section "Transmitting Event Streams" through "Partitioned Logs" (~35 min). Messaging systems, message brokers, log-based messaging (Kafka). This maps directly to your event-driven architecture.
- [ ] **46** 6.6 Read: Ch.11 — section "Databases and Streams" through "State, Streams, and Immutability" (~35 min). Change data capture, event sourcing. The deep connection between databases and streams. This is conceptually powerful.
- [ ] **47** 6.6v Watch: Martin Kleppmann — "Event sourcing and stream processing at scale" (DDD Europe 2016, ~50 min). Excellent complement to Ch.11's CDC and event sourcing sections.
- [ ] **48** 6.7 Read: Ch.11 — section "Processing Streams" through "Fault Tolerance" (~35 min). Stream joins, time reasoning, exactly-once semantics. Microbatching vs checkpointing. The hardest practical problems in stream processing.
- [ ] **49** 6.8 Reflect: Map Brendi's current event flow (WhatsApp message → bot processing → order creation → delivery) to the stream processing concepts from Ch.11. Where are you doing implicit CDC? Where would explicit event sourcing help? (~30 min)

## Phase 7 — Synthesis & Future

### Tasks

- [ ] **50** 7.1 *(survey)* Read: Ch.12 — sections "Data Integration" and "Combining Specialized Tools by Deriving Data" (~20 min). Thin notes.
- [ ] **51** 7.2 *(survey)* Read: Ch.12 — section "Unbundling Databases" through "Designing Applications Around Dataflow" (~20 min). Thin notes.
- [ ] **52** 7.3 *(survey — one exception)* Read: Ch.12 — section "Aiming for Correctness" through "Trust, but Verify" (~20 min base; go deeper if end-to-end correctness arguments click with your formal instincts — this is the one Ch.12 section worth dwelling on).
- [ ] **53** 7.4 *(survey)* Read: Ch.12 — section "Doing the Right Thing" through end (~20 min). Ethics/privacy. Thin notes.

