# Stable Darwin shard ownership (V1-4732)

Self-review found a CIN-V0-4625 omission: partitioning each runner's selected
packages lets a native narrowing failure change package ownership. With equal
costs, inventory a,b,c,d belongs to shards 0,1,2,3, while narrowing to b alone
puts b on shard 0. If only shard 0 falls back to FULL, b executes nowhere.

Decision: partition the entire native inventory before intersecting each
shard's assignment with its admitted selection. Keep the existing selection
record and workflow invocation. Estimates still count only packages that the
leg would execute. The same immutable inventory and cost history now produce
the same ownership regardless of narrowing admission. FULL legs can execute
extra packages without taking affected packages away from narrowed legs.
Clarify that invariant and its regression witness in CIN-V0-4625; no new
requirement or wire contract is needed.

The regression calls the selector's selection function and the shard entry
point for all 16 combinations of FULL/NARROWED legs, with equal costs,
unequal costs and missing-cost round-robin fallback (48 scenarios). It checks
the assigned packages, retained package lists, affected-package execution
and absence of duplicate execution. Before the implementation repair, 36
scenarios failed. After the repair, all five shard tests and seven selector
tests pass, including the unchanged assertions. The intentional failed-module
fixture still verifies that subsequent modules execute.

Raw results are outside the repository in `~/lanes/macosci-evidence/`.
The pre-fix regression log SHA-256 is
`fe3b247b8516d038e2761f0b125b6e865e8b8b58acbf013e93958ecccfdfeaf5`.
Final focused Go checks, spec/citation checks and post-commit CEM/check evidence
are recorded in `~/lanes/macosci.handback.md`. Reuse the pre-change capture;
the orchestrator owns independent review and broad verification. Hosted
Darwin behavior and runtime remain NOT_OBSERVED. No publication, Tasks
writes or CEM seal are authorized.

Rollback: revert this repair's sharder, regression and spec clarification
together; that restores the known mixed-selection omission. Reverting the
original workflow extraction remains the capability rollback.
