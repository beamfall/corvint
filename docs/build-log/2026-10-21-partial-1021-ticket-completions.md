# Partly delivered tickets finished or dispositioned (lane partial-1021)

Date: 2026-10-21. Base: `c9ca8beff43d8f548b054d3b3d40e3054cecf816` (`origin/main`). New
requirements `CEM-V1-568`, `AFP-V0-356`, `ERI-V0-261` and `UCV0-217` are accepted under the
owner's standing auto-accept (2026-10-11). A verifier found tickets V1-0303, V1-0988, V1-2038,
V1-0595, V1-0752, V1-2053, V1-0798 and V1-1881 only partly delivered, and rejected V1-2133.

## Problem and decisions

- V1-2133 (`CEM-V1-568`): `diffExitError` re-classified every Git exit failure of the canonical
  diff as `repository-object-unavailable`, so an ordinary (non-promisor) repository with a
  missing blob lost the `git-diff-failed` cause that the envelope cells packet expects. The
  missing-object reading now applies only when an `objects/pack/*.promisor` marker exists.
  `CEM-V1-367` is closed and `CEM-V1-362` has no exception left; the native envelope cells test
  asserts all 29 cells exact (or `NOT_RUN` where a tool is absent).
- V1-2038: a regression test ties the `effectiveRequiredGates` that `ticket show` and `blockers`
  report to `complete` refusing with `MISSING_GATE` for the same gate (`CAL-V0-049`).
- V1-2053 (`AFP-V0-356`): `affected` MIXED warnings and refusals now name up to 8 sorted dirty
  paths and say that Corvint runs Git without global configuration (decision 0158), so a path
  excluded only by `core.excludesFile` is dirty, with the fix. The `impact` warning is left alone:
  the detail does not fit its `MinPacketBytes` envelope (`TestImpactMixedWarningBudget`). The MCP
  binding keeps its count and digest.
- V1-1881 (`ERI-V0-261`): ERI-V0-060 still decoded the whole snapshot before refusing.
  `RangeBoundRefusalBeforeLoad` now proves the same refusal from Git alone: clean status, a
  snapshot file present for HEAD's tree (not read), a slash-qualified root module admitted
  exactly as Build admits it, and an ancestor base. Anything unproved falls through unchanged. The
  analyzer schema was bumped to `corvint-analyzer/136` for the new `internal/contextindex` file.
- V1-0303 (`UCV0-217`): the README already says "in-repo second consumer" and states no per-job
  status. A test now fails when the UCV0 digest's job statuses disagree with the ledger.
  `INDEX.json` inherits the line through the generated digest.

## Evidence

- Envelope cells: `TestStableCanonicalEnvelopeCellsNative` reports 29/29 exact.
  `TestCanonicalDiffOrdinaryRepositoryMissingBlobIsDiffFailed` expects `git-diff-failed`, and
  `repository-object-unavailable` once the repository is marked as a promisor.
- `TestAFPV0356MixedNamesGloballyExcludedPath` and `TestAFPV0356DirtyDetailIsBounded`.
- `TestERIV0261RangeBoundRefusesWithoutDecodingTheSnapshot` empties the snapshot file and still
  gets the loaded path's exact error.
- Measured on this repository at `db28559f6` (load 27 to 60): `impact --base HEAD~15` (688 paths)
  took 0.51 to 0.54 s with installed `1.0.0-rc.3 (build 698)` and 0.29 to 0.31 s with the lane
  binary. `HEAD~40` (1,864 paths) took 0.58 to 0.59 s and 0.35 to 0.41 s. Stdout, stderr and
  exit 2 were byte-identical. A 300-commit range with an unsafe path falls through, as specified
  (about 11 s for both binaries).
- `TestUCV0217DigestStatusMatchesLedger` fails when the digest says "eighteen" or a wrong status
  (checked by a temporary edit, then reverted).

## Not delivered (SKIPPED)

- V1-0988, V1-0798: moving `CAL-V0-106` and `CAL-V0-073` inside `## Requirements` pushes the
  leases spec past the 256-obligation OCM bound (`ocm_spec_section_test.go`). The owner's
  2026-10-17 rule re-homes new requirements and does not raise the bound, so renumbering or
  narrowing these needs an owner decision. The check criterion is already met by `RDG-V0-021`.
- V1-0752: the required retained evidence is a live GitHub merge-queue case, and the fix belongs
  in `.github/`, which is owner-only.
- V1-0595 remainder: consumer parity is now closed by V1-2133. Stable LRF evaluation and the
  CLI/MCP route wait on an owner-accepted stable LRF profile (`cem-stable-v1.md`).

## Dogfood intent scope

The dogfood intents are `affected-plan-v0`, `cem-stable-v1`, `expanded-range-impact-v0` and
`use-case-conformance-v0`. `corvint-tasks-agent-leases-v0` is left out (DOGFOOD.md, choosing
intents) because it already holds 256 obligations, the aggregate bound on its own. This change
adds one evidence sentence to it for V1-2038 and no requirement. `CAL-V0-049` still applies and is
tested by `TestV12038_CompleteRefusesTheGateShowAndBlockersReport`. Leaving the spec out does not
claim that its obligations hold.

## NOT_RUN

`make gate` and the exhaustive `go test ./...` were not run, per the lane charter. Only focused
package tests ran.

## Rollback

Revert the lane commits. The changes are independent: `isPromisor` in `diffExitError`;
`affectedDirtyDetail` in three messages; the `RangeBoundRefusalBeforeLoad` call in the impact
route (and the analyzer schema bump); and the two new tests.
