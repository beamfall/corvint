# Lineidx code split and Greptile repair — 2026-11-08

Owner intent: ship the lineidx lane as a code-only change and a stacked `.github` change, because `script/github-split-check.sh` (ASD-V0-706) refuses mixed ranges. Before publication, fix the P0/P1 findings of one Greptile review of the code-only diff. Narrowing remains SHADOW; this repair does not renew production qualification.

## Split

The code change carries the whole lane diff except these paths, which ship in the stacked `.github` change:

- `.github/**`
- `.corvint/test-selection.json`
- `docs/specs/affected-plan-v0.md`
- `script/ci-least-privilege-exceptions.tsv`
- the tests that read workflow files: the flaky and narrowing tests of tools/ci-forward-row, the linecov review, record expression and record workflow tests of tools/ci-reuse-plan, and the qualified workflow test of tools/linecov
- the workflow verifier repair build log

On the code side, the `GLS-V0-030..034` and `GLS-V0-040..047` traceability rows and the `tools/linecov` read scope omit the workflow tests and files.

Without `.github/linecov/qualification.json`, `linecov.CheckQualification` refuses admission, so the code change cannot narrow on its own.

## Findings and dispositions

- P1, V1-4365, GLS-V0-034: `QualifiedSources` omitted `internal/linecov/syscallenv.go`, which the collector injects into every test binary, and three other package sources. **Fixed.** All four are now bound. `TestQualifiedSourcesBindEveryPackageSource` requires every non-test source of the package to be bound. Binding more sources can only invalidate records.
- P1, V1-4366, LPCV-V0-7603: an explicit `qualify -test-timeout` could differ from the deadline in the index's runner identity, while the report kept that identity. **Fixed.** Qualify now refuses `test-deadline-mismatch`. The fixture tests collect under the deadline they qualify with, and `TestQualifyFixtureHasNoMiss` covers the refusal.
- P1, V1-4364, GLS-V0-020: watch runs omitted `-json`, so `testing.Verbose` differed from collection, and they used the index's deadline field instead of the runner's recorded runtime. **Fixed.** `watchTestArgs` runs `go test -json` under the recorded deadline. `TestWatchRunsRecordedTestRuntime` covers it.
- P2: a package crash with no named tests vanishes from qualify reports. **Not fixed in this change; retained for follow-up.**
- P2: an unsigned delta can overflow in compact index decoding. **Not fixed in this change; retained for follow-up.**

## Checks

Whole-package `internal/linecov` and `tools/linecov` tests ran through `script/heavy.sh`. `make doc-gates` and `make windows-vet` ran on the split code head before this repair. The CEM records the final verification.

## Rollback

Revert this repair commit as a unit. The split keeps narrowing disabled with or without it.
