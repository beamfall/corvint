# V1-4733: smoke Git cancellation repair

Requirement: `GAG-V0-9962`. Base: `9db7a94169821d9b28701eaa88866aee2fec1083`.
Repair starts at `cd3d0eff36f14ca9b0c8646641c0eb810e7053d4`.

The review found that fixture creation and status Git commands bypassed subprocess
containment. An inherited `GIT_CONFIG_COUNT` can select a commit or fsmonitor hook even
when fixture Git overrides the global and system config files. A hook descendant holding
stdout open survives cancellation of Git alone, so output collection can block the target
worker join and prevent speculative suffix cleanup.

Decision: apply the existing loose-command containment and cleanup to every fixture Git
command and to status Git. On qualified Unix hosts this supplies a private process group,
group cancellation, the existing five-second output wait bound and final group retirement.
Always run cleanup after command completion, preserving the original command error when
one exists. Unsupported hosts keep the existing loose-command behavior. No wire, field,
error code, exit code, existing diagnostic format, golden or existing assertion changes.
This restores the accepted cancellation contract; it does not introduce a new capability.

`TestParallelLooseCancellationReapsSmokeGitAndRemovesSuffix` uses real Git, inherited
`core.hooksPath` and `core.fsmonitor` settings, and a hook helper whose child inherits stdout.
The earlier worker fails only after the helper reports its child PID. Both cases must join
within ten seconds, prove the descendant no longer exists and prove the speculative binary,
cache, temporary directory, smoke fixture and checksums are absent. Emergency helper cleanup
bounds the negative experiment without masking the failed join assertion. The spec's existing
requirement-to-test row now includes this witness.

## Verification

All Go tests used `script/heavy.sh`, one package per command, with
`TMPDIR=/home/russell/lanes/archpar.tmp` and no identity exports or lock overrides.

- `corvint affected --base 9db7a94169821d9b28701eaa88866aee2fec1083`: exit 0;
  touched package selected by `DIRECT_SOURCE_CHANGE`. Installed binary reports
  `self-binary-stale`; its plan is retained with scope UNKNOWN: 92 selected, 564 excluded,
  79 unknown (78 LANGUAGE_FRONTIER, one UNOWNED_DIRTY_PATH). It is not current-tree
  selection qualification and does not authorize wider testing.
- Against the pre-repair `smoke.go`, the new focused test failed both cases at ten seconds
  with the join assertion (20.174 s package runtime, exit 1). Original source was restored
  to the repaired version before subsequent tests. This demonstrates regression sensitivity.
- The first full-package run failed because the new fixture used `Target.id()` (a slash)
  to name speculative files, unlike the production hyphenated names. Corrected test setup
  only; no assertion weakened or removed. The failed run remains retained.
- Final `go test -count=1 -timeout 30m ./conformance/release-artifact-v0`: PASS,
  14.429 s package runtime, 15.2 s admitted run, no queue. Includes all existing package tests.
- `go test -race -count=1 -timeout 30m -run '^(TestTargetWorker|TestParallel|TestSmokeGitHookProcessHelper)' ./conformance/release-artifact-v0`:
  PASS, 4.126 s package runtime, 5.3 s admitted run, no queue.
- `make spec-requirements`: PASS, generated indexes unchanged.
- `script/check-line-citations.sh --repin`: no moved unchanged citations; existing basename
  skips retained. `make -s spec-requirements-check line-citations-check`: PASS.
- Local Go version check: `go1.27.3`, PASS.

Pre-change capture is reused under the repair instruction. Final CEM rebind/check follows
these final source, test, spec and evidence edits; its exact result belongs in the external
handback. Independent review, repository-wide gates, selection baseline, archive live gate
rerun, Windows/Darwin execution and frozen evaluations are NOT_RUN under the repair boundary;
the host orchestrator owns follow-up verification. No new timing claim replaces the prior
live-gate measurement. Publication, native task writes and seal are NOT_PRODUCED by instruction.
Optional guarding-test mutation receipts remain NOT_PRODUCED because that runner was not run.

## Retained evidence

Raw logs stay in `/home/russell/lanes/archpar-evidence/self-review/`.

| File | SHA-256 |
| --- | --- |
| `affected.json` | `43b4a036b12009f7103c2e47d3f5b85d64ab173125dcdd9afb096b24849e93f8` |
| `regression-before.log` | `9f3c1b9290876958d01f0050e0a36772070446daec7d3ea7c0e4acebcb002a86` |
| `package-test.log` | `4d955548c51ac366993a67e896ebe6548d19c921cc0188e5e4eac242da8ff8c1` |
| `package-test-final.log` | `85545b7f8d444910757f09fb8c04c3c2cca98eb5fb693ebac49b2a26a4787a39` |
| `race-test.log` | `1d407807ae5aac5a50dac817b2c23c3945a07f0c3a7ff0607d0d6369e1301967` |
| `static-check.log` | `57a0bfb9d0b0b1e15f92627ebc564e802942177c411ed2aa995baed822a7a92f` |

## Repair friction

1. Installed `corvint affected --base` emits the existing `self-binary-stale` degradation
   (binary built at `d436dff76de1`, 54 differing Go paths). Reproduce with the affected command
   above. It exits zero with a degraded plan; do not claim current-tree plan qualification.
2. The first full-package run exposed the test's slash-versus-hyphen output setup error.
   Reproduce at the intermediate test setup by naming artifacts with `native.id()` rather
   than `native.GOOS + "-" + native.GOARCH`. Fixed the setup and reran the whole package.
