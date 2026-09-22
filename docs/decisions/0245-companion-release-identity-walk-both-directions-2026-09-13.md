# Decision 0245 — companion release judges scratch and output nesting by identity in both directions

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

Decision 0237 refused a scratch directory inside a checkout root and filed two gaps. First, a
checkout root inside the scratch directory passed `validateScratch`; `Run` later calls
`stageBuildSource` on `$scratch/corvint-build-src`, whose first step is `os.RemoveAll`, so a root at
that path would be cleared. Second, `validateOutputParent` compared `EvalSymlinks` results
lexically, and `EvalSymlinks` preserves case, so on a case-insensitive volume an output parent
`$base/CORVINT/out` passed for root `$base/corvint`. `TestRunRefusesCheckoutRootInsideScratch` and
`TestRunRefusesCaseAliasOutputParent` failed at the base (both reached the clean-tree check).

The call: `validateScratch` also refuses, when the scratch path already exists, a checkout root
that is or lies inside it, by running `gitstatus.ScratchOutside` with the symlink-resolved root as
the candidate and the resolved scratch as the only root. A not-yet-created scratch cannot contain
an existing root, so that direction is skipped for it. `validateOutputParent` drops the lexical
comparison and runs the same `ScratchOutside` walk from the nearest existing ancestor of the
resolved output path. No second copy of the walk is added; `internal/gitstatus` is unchanged.

Not changed: a checkout root inside the output parent is not refused. `retainBundle` never removes
or overwrites under the output parent (an existing `outputParent/bundleName` is refused), so that
direction cannot clear a root.

Consequences: `PUB-V0-011` text and its traceability row are amended; both `bugs.md` entries are
removed. `validateOutputParent` now takes a context and reports an unresolvable or missing checkout
root as "not provably outside" rather than a resolve error. No `analyzerSchemaID` bump: neither
package is under `internal/contextindex`.

Rollback: revert the commit. That restores the lexical `validateOutputParent`, the one-direction
`validateScratch`, and the 0237 text of `PUB-V0-011`.
