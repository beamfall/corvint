# 2026-10-21 perf-leftovers-1021: corvint-pr-tests admits go1.27.3 (V1-2194)

## Problem

`tools/corvint-pr-tests/main.go` pinned `id.GoVersion != "go1.27.2"`, so the trusted PR driver
refused the go1.27.3 patch revision that decision 0547 admits beside the go1.27.2 pin.

## Decisions

- `AFP-V0-296` in `docs/specs/affected-plan-v0.md` is accepted under the owner's standing auto-accept
  (2026-10-11). One definition, `admittedGoVersions`, holds `go1.27.2` and `go1.27.3`. Every other
  version is refused with the existing `unsupported Go version` error.
- `.github/cishards` `SourceGoVersions` and the hosted `setup-go` pin are unchanged. `.github/` is
  out of lane scope, and decision 0547 leaves CI pinned at go1.27.2.

## Evidence

- `TestToolIdentityRequiresCurrentGoVersion` admits go1.27.2 and go1.27.3 and refuses go1.27.1,
  go1.27.4 and go1.28.2.
  - It passed under the host go1.27.3 and under the module-cache go1.27.2 toolchain (`go env GOVERSION` = go1.27.2).
  - With the old pin restored, the go1.27.3 subtest fails.
- The whole `./tools/corvint-pr-tests` package passed under go1.27.3 (531 s at load about 100). Go vet passed.

## NOT_RUN

`make gate` (lane policy). A hosted CI run under go1.27.3 is not applicable, because CI stays pinned at go1.27.2.

## Lane tickets not delivered here

- V1-1766: SKIPPED. Reproduced on c9ca8beff: a new `testdata/x/policy.json` selects 51 PATH_LITERAL readers. The cause is that the `namesFileOnlyWithin` vocabulary places `testdata/x` from any `"testdata"` or `"x"` literal. Narrowing that rule can drop real readers, and turning the namers into unknowns changes the plan's unknown set. Both need a selection-safety design, not a lane patch.
- V1-0923: SKIPPED. Reproduced: a new build-log file yields `UNOWNED_DIRTY_PATH`. The fix needs a decision: either a repository-declared documentation non-source class (like `ClassTaskStoreProjection`, AFP-V0-088), or hard-coding Corvint's own `docs/build-log` layout into the product.
- V1-0922: SKIPPED. Reproduced: `interop/cem01-go/*.go` yields `UNINDEXED_SOURCE_PATH`. Indexing a nested module, or adding a new nested-module unknown reason, is an affected-plan wire/register change.
- V1-1010 and the optional list: NOT-STARTED (budget).

## Rollback

Set `admittedGoVersions` back to `go1.27.2` alone.
