# Selwide FULL discovery and build-context repair

The owner's self-review repair requires P1 (ordinary source directories omitted
from FULL) and P2 (build-excluded generators aborting discovery), under V1-5018 /
AFP-V0-9012. No exported wire fields, diagnostics, exit codes, goldens or frozen
outputs change. V1-4941 / AFP-V0-9013 and V1-4710 / AFP-V0-9014 are unchanged.

## Decision and closed exclusion classes

The shared Go graph/execution walk inherits only the closed metadata/vendor
exclusions `.git`, `.hg`, `.svn`, `vendor`. Every other generic skipped name is
admitted, including future additions; existing module, fixture, strict declared
hidden-output and bounded-walk rules remain. Unknown ordinary names are source
candidates. A bounded or otherwise unprovable universe still refuses.

Graph discovery retains every build variant. The execution toolchain's `go list`
uses the same race build context as the frozen execution prefix. Its private
JSON projection grants omission only for the exact build-constraint exclusion
for the candidate directory, with ignored Go files, no active Go/cgo/internal
or external test files, and no dependency errors. Every other candidate goes
through strict `go list`; unknown diagnostics and malformed constraints are
loading errors. This avoids implementing a second build-tag parser. The JSON
projection is internal and changes no persisted receipt or public wire shape.

## Regression evidence

`TestHiddenSkippedNamesFullExecution_AFPV09012` failed before the fix for each
of `venv`, `node_modules`, `site-packages`, in both one and four shards. It also
retains a failing test under `unknown-dependency-kind`, covering the default
admission beyond reported names. `TestPackageDirectoriesMatchGraph_AFPV09012`
checks the same admission in disk and immutable graphs.

`TestRuntimeBuildContext_AFPV09012` failed before the fix when ignored generator,
foreign-platform, unfamiliar custom-tag and race-only packages reached ordinary
listing. The repaired regression checks run, workflow-package and historical
modes, narrowed and FULL execution, race-only inclusion and malformed constraint
refusal. `TestRuntimeExclusionAllowlist_AFPV09012` checks that unknown diagnostics,
active source, mismatched directories and dependency errors cannot grant omission.
Workflow integration initially failed because its text-only `go list` stand-ins
did not implement the JSON build-context probe. Their setup now implements the
new exact probe and race-list argv, retaining explicit package/wildcard validation
and all existing outcome assertions; ordinary-directory coverage is expanded.
An intermediate regression attempt correctly refused its dirty fixture; the
fixture now commits its additions before asserting execution topology. The
whole-package run then exposed a missing fixture identity when the new assertion
verified that narrowing stayed narrowed; supplying the existing fixture identity
fixes setup without weakening the drift guard or that assertion.

Raw command logs remain outside the repository in `~/lanes/selwide-evidence/`
with prefix `self-review-6-`. Final package/check results and digests are recorded
below before the single final CEM binding. Host independent review, broad gates,
qualification and native ticket completion remain outstanding as previously
recorded; the owner forbids those actions in this lane round. Rollback is to
revert this repair's code/spec/test commit, restoring the known findings; do not
promote the resulting incomplete FULL universe as qualified.

## Final focused validation

Go 1.27.3 verified. All tests ran through `script/heavy.sh`, one package per
command, without exported commit identity. The affected plan selected all three
packages but retained UNKNOWN (71 unknowns) and an installed-binary-stale warning.

- `self-review-6-before.log`: FAIL github.com/Beamfall/corvint/tools/corvint-pr-tests 24.676s; heavy.sh: done queue=0.0s (load=0s slot=0.0s) run=25.7s rc=1.
  SHA-256: `63bcf88b139b0d7e5dc41e4f175a03f2f6d0883355c56a14013da6548f2f58dd`.
- `self-review-6-focused.log`: FAIL github.com/Beamfall/corvint/tools/corvint-pr-tests 29.660s; heavy.sh: done queue=0.0s (load=0s slot=0.0s) run=30.8s rc=1.
  SHA-256: `e9c2f836f7e0ebe7e18864111866ebf25fa550fc70f3c8aed184778cf8d3e381`.
- `self-review-6-whole-pr-tests.log`: FAIL github.com/Beamfall/corvint/tools/corvint-pr-tests 164.285s; heavy.sh: done queue=0.0s (load=0s slot=0.0s) run=165.2s rc=1.
  SHA-256: `b8fb2f76dd0532ec88f1c2b47cb6010a0507cded5572d3d9fd3a8953079c9224`.
- `self-review-6-whole-workflow.log`: FAIL github.com/Beamfall/corvint/tools/ci-forward-row 54.277s; heavy.sh: done queue=0.0s (load=0s slot=0.0s) run=55.4s rc=1.
  SHA-256: `cd5dd087f711f94b769a447d9ae52ab2a8183014ee5ec8f7b2b57ee553c9c2da`.
- `self-review-6-context-final.log`: ok   github.com/Beamfall/corvint/tools/corvint-pr-tests 16.506s; heavy.sh: done queue=0.0s (load=0s slot=0.0s) run=17.3s rc=0.
  SHA-256: `364c35c9e701c09560f5975755dbb7e23287b8952ec0c0540388a25d37bb25b1`.
- `self-review-6-whole-golang-final.log`: ok   github.com/Beamfall/corvint/internal/liveverify/affected/golang 9.448s; heavy.sh: done queue=0.0s (load=0s slot=0.0s) run=10.3s rc=0.
  SHA-256: `e4f8aa449d35432e6d45276026bbbcf6b26bd6d6d408874e5b8f56885e466ba8`.
- `self-review-6-whole-pr-tests-final.log`: ok   github.com/Beamfall/corvint/tools/corvint-pr-tests 158.417s; heavy.sh: done queue=0.0s (load=0s slot=0.0s) run=159.0s rc=0.
  SHA-256: `583d15979c938c49389c0e6e82a0428d53d40aa54fbd83f239f66b923a05293d`.
- `self-review-6-whole-workflow-final.log`: ok   github.com/Beamfall/corvint/tools/ci-forward-row 48.390s; heavy.sh: done queue=40.0s (load=0s slot=40.0s) run=49.5s rc=0.
  SHA-256: `bfb0fd96a0df963538ae1230887630881030293845c67de547e5b04de69a3b7a`.
- `self-review-6-spec-check-final.log`: ok   github.com/Beamfall/corvint/internal/specindex 0.921s; heavy.sh: done queue=0.0s (load=0s slot=0.0s) run=1.2s rc=0; ok   github.com/Beamfall/corvint/internal/lrfrepo 0.074s; heavy.sh: done queue=0.0s (load=0s slot=0.0s) run=0.5s rc=0.
  SHA-256: `c02c63054ca2137626770317140ce5cf7369b2fcd924044833157ca8bde6527b`.

The final whole packages are `./internal/liveverify/affected/golang`,
`./tools/corvint-pr-tests`, and `./tools/ci-forward-row`, each with
`go test -count=1 -timeout 30m`. The final spec/citation command was
`make -s spec-requirements-check line-citations-check` (PASS). Citation repin
found no moved unchanged citation; its 180 unmatched basename skips remain
visible. The host owns independent review, repository-wide gates and native
completion; none is implied by these passing scoped checks.
