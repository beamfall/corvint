# Affected zero unknown: measurement and design

Date: 2026-11-04. Base: `8e2c13aa926fcaf1731fae71b27c41c41c8f09af`.
Authority: the owner's goal in chat on 2026-11-04 ("figure out how to get the unknown to 0
safely"). The orchestrator made the design choices under the owner's delegation of 2026-11-02.
Spec: `docs/specs/affected-zero-unknown-v0.md` (AZU-V0-001..009). Tickets: V1-4684..V1-4692.

## Measurement

The `8e2c13aa9` tree binary was run as `corvint affected --base M^1` over eight consecutive merges.
Every plan was `UNKNOWN`, with 63 to 99 entries each. Most were `LANGUAGE_FRONTIER`: 81 of 92 at
`8e2c13aa9`. Single-file probes then traced where the frontier count comes from:

- **Unowned-path cascade.** An unowned path such as `Makefile` or a workflow makes every
  non-participating plugin's path-open entries bear on it (AFP-V0-511).
- **Whole-plugin participation.** One TypeScript file makes all 69 TypeScript entries bear.
- **Inbound shell callers.** Unbounded-reader shell units carry
  `shell:foreign-invocation-unmodelled`. This also affects owned Go changes.

## Decision

Close each class soundly instead of narrowing. Only two outcomes are allowed:

- a selection, such as driver targets, frontier site units, declared callers, declared consumers
  or a package command;
- a stated proof, such as recording a unit as `uncovered` because no test can observe it.

Nothing that reads, executes, imports or embeds a changed path may be dropped. Plans may get wider
before they get narrower. Narrowing is later work, once the AZU-V0-009 replay shows zero unknowns
with zero safety misses.

Withdrawn approach: the V1-4679 lane narrowed frontier entries for unowned data paths. Its fix-only
review found an unsound case (V1-4683: a tsconfig alias can import JSON as data), so it was
withdrawn. AZU-V0-003 replaces it with site closure, which needs no proof that the alias misses.
