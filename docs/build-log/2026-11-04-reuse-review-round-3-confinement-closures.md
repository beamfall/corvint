# reuse: review round 3 (enforced confinement, job closure manifests and includes)

Base: `6f12f4704235dc8f9bea19fc12995bfb68c746c3`. Tickets: V1-1097, V1-1099. Requirements amended
in place, with IDs unchanged: AFP-V0-2320, AFP-V0-2322, AFP-V0-2326, AFP-V0-2327 and AFP-V0-2342
in `docs/specs/affected-plan-reuse-v0.md`. An independent review of the round-2 head `49fe9f75f`
returned three P1 findings. Each is fixed with a regression test that fails against the reviewed
implementation. Each fix refuses where it cannot prove soundness; none tries to model the
unbounded input.

## Findings and decisions

1. **Reuse assumed confinement that no runner applied (P1, AFP-V0-2320, AFP-V0-2322,
   AFP-V0-2326).** Eligibility relied on the declared read scope being enforced. In practice
   only CI's `go test -exec "$RUNNER_TEMP/test-confine"` enforces it. The local path
   (`heavy.sh`) runs plain `go test`, so a declared package could read `../data/other.txt` while
   unconfined. That file is in no digest, so it was reused after the file changed.
   Decision: the identity gains `confine`, the `sha256:` digest of the content of the
   `--confine PATH` wrapper (`ConfineDigest`). The digest is empty when the run was unconfined.
   A package is eligible only when `confine` is set; otherwise the reason is
   `UNCONFINED_EXECUTION`, checked after `SLICED_UNIT`. Only the wrapper's content is bound, not
   its path, because `$RUNNER_TEMP` varies between runners. A missing, non-regular or
   non-executable wrapper refuses the command with exit 3. `--argv` still refuses `-exec`.
   Rejected alternatives:
   - Binding the declared scope alone, which enforces nothing.
   - Detecting out-of-scope reads, which has no sound local detector.
   - Making heavy.sh confine its runs. Landlock is Linux-only and the Mac lane path has no
     equivalent, so the local suppression path now fails closed: it stays wired and reuses
     nothing.
   The binding is attested by the caller, as the argv already is. A record producer that names a
   wrapper it did not run under defeats it. AFP-V0-2327's protected collector is what makes that
   claim trustworthy in CI.
   Evidence:
   - `TestUnconfinedOutOfScopeReadRuns`. On the reviewed code the unconfined run recorded `a` and
     `b`, and decided `b` `REUSE RECORD_DIGEST_EQUAL` both before and after the out-of-scope file
     changed. After the fix it records nothing, decides `UNCONFINED_EXECUTION`, and the real rerun
     of `b` fails.
   - `TestConfinementWrapperBindsIdentity`: the same wrapper reuses, another wrapper gives
     `IDENTITY_CHANGED`, and a directory, a missing path or a non-executable file is refused.
   - The `UNCONFINED_EXECUTION` and wrapper-directory steps of `TestTestReuseCLI`.
2. **Job closures omitted dependency-module manifests (P1, AFP-V0-2342).** `GoDirectories` now
   reads each closure package's `Module` from `go list` and adds its module's `go.mod` and
   `go.sum` to the input map. A change to either is `GO_MODULE_CHANGED`, witnessed with the
   package and platform. Missing module metadata, or a module directory or manifest outside the
   repository or not the directory's `go.mod`, is `EVALUATION_ERROR` `module-input`.
   Evidence: `TestJobDependencyModuleLanguageFault`. Its fixture has an in-repository
   replacement module at `go 1.22` that uses range-over-int. The reviewed code gave
   `SKIP NO_DECLARED_INPUT_CHANGED` for a `go.mod` change. The fix gives `RUN GO_MODULE_CHANGED`
   for both `go.mod` and `go.sum`, and after a downgrade to `go 1.21` the real build fails
   ("requires go1.22 or later").
3. **Assembly and cgo includes escaped the package directories (P1, AFP-V0-2342).** `go list`
   does not name `#include` targets, so a header outside every listed directory could change
   with the job skipped. Decision: a closure package with `CgoFiles` or `SFiles` is
   `EVALUATION_ERROR` (`cgo-input`, `assembly-input`) on every path. This repository's job
   closures have no cgo or assembly packages, so the refusal costs nothing today.
   Evidence: `TestJobEscapingIncludeFault`. The fixture uses assembly on amd64 and arm64 and cgo
   elsewhere. On the reviewed code both the escaping-header change and an unrelated change gave
   `SKIP`. After the fix both give `RUN EVALUATION_ERROR`, and a forced rebuild with an invalid
   header really fails.

## Verification

Focused runs through `script/heavy.sh`, each offloaded to linux/amd64:
- `./internal/testreuse/`
- `./internal/jobinputs/`
- `./internal/capcatalog/`
- `./cmd/corvint/` with `TestAffectedJobsCLI`, `TestTestReuseCLI` and `TestCAPV0*`

`tools/ci-reuse-replay` unit tests also ran. The capability catalog was regenerated for the new
`--confine` usage.

## Limits

- The dormant CI record and decide steps in `.github/workflows/ci.yml` do not pass `--confine`.
  Wiring it is a `.github` change for a separate `ci-control-plane` commit. AFP-V0-2327 now
  requires it before the hold is lifted.
- `tools/ci-reuse-replay` does not pass `--confine`, so a rerun of it would now report no reuse.
  Its earlier savings figures assumed confined runs and were not re-measured.
- Local `HEAVY_REUSE` reuses nothing until local runs are confined. Hosted reuse remains
  `NOT_OBSERVED`.
