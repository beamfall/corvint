# Unbounded lane self-review repair — 2026-11-05

Owner intent: fix only the three P1 self-review findings for the unbounded lane against
base `9db7a94169821d9b28701eaa88866aee2fec1083`. Requirements: AZU-V0-9931 and AFP-V0-023.
The existing AZU-V0-9930 comparison proof is unchanged. No ticket IDs were supplied;
native Tasks writes and publication are forbidden.

## Findings and decision

Remove four declarations from `.corvint/test-read-scopes.json` and retain their existing
entries and reasons in `.corvint/unbounded-readers.json`:

- `internal/companionrelease`: `TestCurrentExportedTasksSmokeGenerator` and
  `TestCurrentTasksSourceArchiveBuildsOffline` call `exportSource` against the real checkout.
  It resolves HEAD and HEAD's tree, enumerates every tracked entry and reads Git blobs in two
  independent passes. A source directory grant cannot supply that Git authority.
- `internal/opencodequalification`: `TestCommandInterruption` builds `tools/qualify-opencode`
  with `cleanEnvironment()`, which clears GOFLAGS and discards the wrapper's
  `-buildvcs=false`. VCS stamping then needs the checkout's denied Git metadata. Choose the
  requested conservative floor instead of changing production environment handling or build flags.
- `internal/dashboard/repository` and `conformance/dashboard-snapshot-v0`: the repository
  authority starts directory binding at `/`, where `bindRootDirectory` reads `root.Open(".")`.
  The existing wrapper cannot grant that directory read without granting a recursive root read.
  Keep both packages unbounded until a separately scoped authority implementation is qualified.

This satisfies AZU-V0-9931's requirement that consumers without admitted confinement proof
stay unbounded. All three repair findings are fixed. Their intended declaration narrowing is
withdrawn; the original four audit rows and six-declaration replay are historical evidence of
that overlay, not qualification or performance measurements for this tree. The earlier Tasks CLI
PARTIAL disposition remains unchanged. Only the original postmergehost declaration remains added
relative to the lane base; its independent qualification is outside these three repair findings.
No wire shape, field name, error/exit/stderr contract, golden, core-freeze output, assertion,
Git authority code or confinement rule changes. Rollback of this repair alone would restore
known unsound declarations and is not an admitted promotion path.

## Verification

Reuse the existing pre-change capture; no start capture is rerun. Before tests, the existing
checkout-built binary's `affected --base` plan selected all eight tested packages:
360 selected, 510 excluded, three unknowns (LANGUAGE_FRONTIER=1 and
UNSELECTED_BINARY_CONSUMER=2), overall UNKNOWN. Both dashboard packages have an
UNBOUNDED_READER witness; companion release and OpenCode have BINARY_EXEC witnesses.
Absence of all four declarations and presence of all four existing floor entries/reasons were
also checked directly. The affected plan does not imply exhaustive test coverage.

The existing `.github/testconfine/cmd` wrapper was built through `script/heavy.sh`, with
`-trimpath -buildvcs=false`, and reported Landlock ABI 7. The four repaired packages run via
`go test -exec` with this wrapper and `CORVINT_CONFINE_ROOT` set. Because their declarations
are absent, the wrapper intentionally executes them unconfined, matching CI's restored path;
these passes do not claim that these packages are safe under Landlock confinement.

Every test command uses `script/heavy.sh`, one package per command, the existing full package,
`GOTOOLCHAIN=local go test -count=1 -timeout 30m`, and no `-run` filter. TMPDIR is
`/home/russell/lanes/unbounded.tmp` (0700); no lock-directory override or identity export is used
for tests. Toolchain is go1.27.3. Canonical verification commands are retained outside the
repository in `unbounded.tmp/self-review-verify.txt`.

| Package | Outcome | Package time | Log SHA-256 |
|---|---|---:|---|
| `internal/companionrelease` | FAIL: sandbox ancestor Git directory | 102.874s | `75f825ba77c657f252948a49d4f1231f2c25d38defbc033fbaf483aaf29247e8` |
| `internal/opencodequalification` | PASS | 12.695s | `ef4499b1a27132a95d779eab9f194c0bd1bc4451af1fd6aaa48fad3c55f833f5` |
| `internal/dashboard/repository` | PASS | 26.542s | `55a220d8f7471e2c571ed040f134fde4c04366872c102e4311cfa7f39f7d6f55` |
| `conformance/dashboard-snapshot-v0` | PASS | 12.869s | `140d11567f94d87361eb12a1836dc067ea8d7f0575c2e75e71109de90c9fe829` |
| `internal/liveverify/affected/golang` | PASS | 16.748s | `0de128a1caf1463457d57806074973c88834a5b32661ddb42861d33a68ad1419` |
| `internal/gateselect` | PASS | 3.082s | `3faba0777f4b8484fb0086a9699c4f02e622180c14a00e16c265e322d6ab717f` |
| `tools/test-read-scopes-check` | PASS | 3.719s | `644c144f8a14a392ccd498392170c1cdee01ff033ce4b985d46f1acb6b17a048` |
| `internal/liveverify/affected` | PASS | 113.248s | `28ae92ba7b76cb3f46d9dc39985e5f5394ac304b244e4c72244a242ee4960708` |

The real declaration checker (`go run ./tools/test-read-scopes-check`, also through heavy.sh)
passes. All declaration removals are exactly reversions of lane-added entries; no existing test
assertion or baseline was changed. The four wrapper package invocations include the named P1
positive tests; only companion release fails, after successfully obtaining real checkout authority.

## External blocker and retained limits

Companion release qualification is PARTIAL due to the sandbox's read-only empty directory
`/home/russell/lanes/.git`. Both exported-source tests now pass checkout export, then their
closed-environment builds discover that ancestor Git marker from the mandated TMPDIR and fail
`error obtaining VCS status: exit status 128`. The directory is outside the authorized Git
metadata roots and cannot be removed or changed here. A standalone trivial Go module under
`unbounded.tmp/self-review-vcs-repro` reproduces the same failure with `go build`; the same build
with `-buildvcs=false` passes. Both control commands use heavy.sh. No test, product environment,
source build flag or TMPDIR was changed to hide this failure. The host verifier must rerun the
whole companion release package outside this synthetic ancestor-marker obstruction.

The seven other full packages pass. Linux evidence does not qualify Darwin/Windows behavior.
`TestCurrentExportedTasksAndVSIX` requires an explicitly supplied populated npm proof cache;
that optional live qualification was not supplied, and its pre-existing skip stays intact.
Repository-wide selections/tests, doc-gates, selection-baseline-check, gate-affected, corvint test,
model-backed review, Tasks writes, publication and seal are NOT_RUN as directed. No spec or
numeric line citation changed in this repair, so spec/citation regeneration is unnecessary.
Independent review is orchestrator-owned. The CEM finish records local outcome `blocked` for
the companion release verification blocker; structural dogfood success must not imply all tests
passed. The original replay deltas are NOT_REMEASURED for the repaired tree.

Raw evidence is outside the repository under `/home/russell/lanes/unbounded-evidence/`.
Additional immutable local evidence bindings:

| File | Bytes | SHA-256 |
|---|---:|---|
| `self-review-affected.json` | 142822 | `758f0a292b44266146d243f3ea475f16710d13bdb0f000fcb7d33b656209a3bd` |
| `self-review-test-confine` | 4096977 | `c027e60f2b2eabe15e5dc764cc63be9d0713f6225d8fd834f4caf0028f4525f4` |
| `self-review-read-scopes-check.log` | 436 | `3f73e2ae8fdb60337bb1750bb975c18684f3e4dcc5be59d18847499e0fa54d8a` |
| `self-review-vcs-repro.log` | 540 | `e152349d39f6aea9d15acbc5d342030638afbcee1d341b4474393315e5952fa7` |
| `self-review-vcs-control.log` | 408 | `fd4dfea974ba3de83f128c2ab6143066bfcb09e63a3822abb2b1400bc184a1ab` |
