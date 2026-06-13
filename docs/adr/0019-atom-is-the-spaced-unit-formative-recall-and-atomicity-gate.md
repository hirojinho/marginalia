# 0019 — The atom is the spaced unit: formative recall, and an atomicity gate replaces the confidence-threshold mastery gate

- **Status:** Accepted
- **Date:** 2026-06-13

## Context

[ADR 0007](0007-knowledge-component-as-atomic-note.md) (2026-05-27) decided that
mastery, confidence, and retrieval are tracked against the **Knowledge
Component** — a learner-authored atomic note that lives *below* the task — and
that `confidence_log` must migrate off the plan-task UUID identifier. The
glossary records the same.

A review of two real study sessions (DDIA #62, CE-297 #63) on 2026-06-13 found
the implementation never honored 0007. The retrieval/confidence path stayed on
the **task UUID**:

- Rule 3 instructs `claw-cli confidence log --kc <active task id>`.
- The mastery gate checks `HasConfidenceAtLeast(task.ID, threshold)` (the shipped
  "S2" hard-gate, `mastery_threshold` default 0.7).
- Only two real `knowledge_components` rows were ever created in months of use.
- `confidence_log` / `retrieval_queue` accreted **four incompatible id schemes**
  (bare integers, namespaced `task:ddia:20`, plan-task UUIDs, a base-UUID+suffix
  scheme); the same idea (write skew) fragmented across three keys; one session
  mis-logged an FHA recall against the write-skew key. `retrieve due` returned
  only `id\tconfidence` with no titles, so the tutor reverse-engineered topics
  from raw text every session.

So the subsystem ran *against its own accepted architecture*. Two implementation
questions 0007 left open had to be resolved before re-keying:

1. In-session recall happens continuously (session-open recall, post-chunk
   boundary recall), but an atom usually has not been distilled yet at that
   moment. What feeds the spaced queue?
2. The mastery gate's confidence threshold (0.7 on the task) repeatedly blocked
   completion of a just-read task (observed: 0.45 → "mark it complete" → refused
   → "force complete"). Mastery comes from *spaced* repetition over days, so a
   threshold cannot be satisfied in the session that reads the material —
   contradicting the single-task daily-session model ([ADR 0009](0009-session-single-task-spaced-unit.md)).

## Decision

**1. The atom is the spaced unit; in-session recall is formative.** Recall
during a Session (explain-back, boundary checks, cueing) is **formative** — it
coaches understanding in the moment and is neither scored into a trajectory nor
scheduled. The **only** thing that enters the spaced-retrieval queue is a
**learner-authored Knowledge Component**, scored against its body at creation and
on each resurfacing. The session-open recall (Rule 6) quizzes *due atoms*, not
tasks.

**2. Completing a Task requires distilling ≥1 atom — an atomicity gate, not a
confidence threshold.** The completion gate's job shrinks to "did the generative
act happen": a Task may be completed once at least one Knowledge Component has
been authored from it. **No confidence score blocks completion.** Durability is
the job of spaced retrieval afterward — a weak atom *resurfaces until mastered*
rather than the Task refusing to close. This **supersedes the S2
confidence-threshold mastery gate** on task completion.

**3. Clean-start migration.** No atoms are fabricated from history (0007: the
learner authors the body). The two genuinely-authored atoms keep their
trajectories by re-keying their confidence from the task id onto the atom id.
The rest of `confidence_log` is retained as inert history but dropped from the
queue; the queue is rebuilt from atom-keyed rows only.

**4. Search-before-create.** Before authoring an atom the tutor searches existing
atoms and offers to reuse/refine a near-match — the lightweight version of the
identity-drift control 0007 deferred.

**5. Active-course-first surfacing.** Atoms are global (an atom is not owned by a
course). Session-open recall leads with the active course's due atoms and
*offers* cross-course due atoms rather than front-loading them, preserving
interleaving as an opt-in nudge instead of an ambush.

## Consequences

- The confidence-threshold mastery gate (S2) is removed from the completion path;
  `mastery_threshold` as a *completion blocker* is retired. (Per-atom mastery
  still exists — as a property of the atom's spaced trajectory, not a gate.)
- Rule 3 / Rule 6 / Rule 9 are rewritten: confidence logs against an atom id;
  `retrieve due` resolves and shows titles; atom capture is search-first.
- The daily loop is unchanged in feel (formative recall dominates a session);
  what changes is that *only distilled atoms persist and recur*.
- Friction observed in the review is dissolved at the root: no force-complete
  fight (gate is atomicity, not score), no cross-course ambush (active-first), no
  per-session topic archaeology (titles resolve).
- **Accepted cost:** distilling an atom to complete a task is required generative
  friction — deliberate, per 0007.
- **Deferred (unchanged from 0007):** the link graph and semantic dedup remain
  later tickets.
- If usage shows that *nothing* gets distilled (tasks completed with throwaway
  atoms to clear the gate), revisit — the gate may need a minimal quality check.

## References

- [ADR 0007](0007-knowledge-component-as-atomic-note.md) — atom as learner-authored note
- [ADR 0009](0009-session-single-task-spaced-unit.md) — single-task daily session / spacing
- [CONTEXT.md](../../CONTEXT.md) — *Knowledge Component*, *Recall: formative vs spaced*
- Plan: `docs/superpowers/plans/2026-06-13-retrieval-integrity-and-resolution.md`
