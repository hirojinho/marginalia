# 0002 — No service/repository layer split

- **Status:** Accepted
- **Date:** 2026-05-08

## Context

The original stability plan (Phases 2.3 and 2.4) prescribed introducing a `service/` layer (`SessionService`, `PDFService`, `PlanService`) and a `repository/` layer (mockable persistence) on top of the new `agent.App`. Standard advice for testable backends, and would let handlers depend on interfaces rather than `*App`. Counter-pressure: ~5,500 LOC, one developer, no immediate plan to swap the persistence backend or run the same logic against a different DB.

## Decision

Skip the split. `agent.App` methods are themselves the persistence + orchestration surface. `handler/` calls into `App` directly. Tests use an in-memory SQLite instance instead of mock interfaces.

## Consequences

- Two layers (`agent/`, `handler/`) instead of four. Less navigation, fewer files to keep in sync.
- Concrete dependency on SQLite leaks slightly into `App` method signatures — acceptable while there's only one storage backend.
- No mockable repository means handler tests need real DB fixtures. Cheap with in-memory SQLite; would hurt at higher LOC or with team contributors.
- Revisit if the codebase doubles, or if a second persistence backend ever shows up.
