# 2026-10-14 V1-1266: consumer hunk-revert falsifier (`held`) and the guarding-test step

Decision 0523 P2 named the hunk-revert falsifier as the rc.3 headline. V1-1266 implements it as an
opt-in consumer step, `corvint cem verify --hunk-revert`, together with the producer step that lets
it fire. Requirements: `CEM-V1-088..089` (`cem-stable-v1.md`, section "Consumer hunk-revert
falsifier") and `TCQ-V0-060..061` (`test-claim-qualification-v0.md`, "Guarding-test citation").
All four are recorded under the owner's standing auto-accept (2026-10-11). `CEM-V1-090` belongs to
V1-1259 and is not used here. 1.0 scope: `PRS-V1-018` is merged, but its five clauses do not name
this work. The governing scope is decision 0523 P2 (rc.3) under the V1-0020 release dependency.

## Design

- **Placement.** The falsifier reports beside the result as the envelope member `hunkRevert`, never
  inside `stableVerification`. `verdictSha256` (`CEM-V1-066`) excludes only runtime,
  repositoryEnvelope and itself, so any in-result member would change it. The in-result `citation`
  block keeps `held` 0. Without the flag no output byte changes, and the 54 public cases and their
  `verdictSha256` are untouched (K6). Only `verify` takes the flag; `status` and `report` refuse it.
- **Candidates.** A candidate is a `supported` hunk of non-test Go source with a `test-claim` basis
  on a `_test.go` span. Its tests are the top-level `func TestX(t *testing.T)` declarations inside
  the cited span, run in the cited file's package directory. There is no new wire field.
- **Execution.**
  - The consumer exports the target tree from Git objects (`ls-tree` plus `cat-file --batch`, no
    optional locks, sanitized git environment) into a temporary directory under `os.TempDir()`. It
    removes that directory on every exit path.
  - For each candidate it reverse-applies exactly that one hunk, restores the file afterwards, and
    runs only the named tests. The `go test -json -count=1 -run ^(names)$ .` argv and the
    `GOTOOLCHAIN=local`, `GOPROXY=off` and `GOSUMDB=off` pins are pinned to `native.Build` by a
    parity test.
  - `held` requires a test that passed at the target and a test-level `fail` event for it that the
    consumer itself decoded with the hunk reverted.
  - A copy with no runnable candidate is never exported.
- **Dependencies.** The runner lives in `internal/cem/verify` on the standard library and
  `internal/groupreap`, because the CEM seam closure (`TestCEMSeamsDependOnlyOnStdlibAndGit`)
  forbids importing `internal/testrunner`.
- **Bounds.** Each run is limited to 300 s and the whole falsifier to 1200 s. The report records
  both limits and its elapsed seconds. One go test stream is capped at 32 MiB.
- **Producer step.** `docs/DOGFOOD.md` section 4 adds "Cite the guarding test". It is offered, not
  forced, and `unknown` stays allowed.

## Recorded limits (rc.3 release-notes text)

The repository has no rc.3 release-notes section yet: `docs/RELEASE-NOTES.md` stops at rc.2, and
that file is outside this lane's scope. Proposed text:

> `cem verify --hunk-revert` (opt-in) reverse-applies one cited hunk in a disposable copy and runs
> the tests its `test-claim` span declares under `go test` only. Every other runner is a recorded
> limit. The portable consumer (`interop/cem01-go`) does not re-execute. The run reuses the host
> Go module and build caches (`HOME`, `GOPATH`, `GOMODCACHE`, `GOCACHE` pass through), which the
> native executor's minimal environment does not. Offline, a dependency missing from the cache
> fails as `not-run`. A tree with a symlink or submodule is `not-run: infrastructure`. The flag
> executes the target's test code with the caller's privileges and no filesystem confinement, as
> `go test` would; run it only on a change whose tests you would run.

## Measured latency (K5 input)

These are wall-clock pairs on this host while seven other lanes were building and testing. The
plain runs vary by up to 5x on their own, so the falsifier's own `elapsedSeconds` is the
added-latency figure. Each pair runs `cem verify` with and without the flag on a committed dogfood
map, with target = the bind commit's parent.

| Map (bind commit) | Hunks | Candidates | Falsifier rows | Plain | With flag | `elapsedSeconds` |
| --- | --- | --- | --- | --- | --- | --- |
| `ff6e3b9d` | 19 | 0 | none | 1.31 s | 1.32 s | 0 |
| `283b25c0` | 10 | 1 | `no-named-test` | 23.15 s | 34.17 s | 0 |
| `69f61cf9` | 88 | 1 | `not-a-modification` | 135.91 s | 57.50 s | 0 |
| `1893e26b` | 183 | 1 | `survived` (`internal/dogfoodflow::TestFlowGitIgnoresAmbientConfigurationAndReplaceRefs`) | 7.94 s | 37.30 s | 26.93 |

- **Added latency.** About 0 s when no candidate is runnable (the default for today's maps: 4 of
  651 committed maps carry any candidate). One runnable package added 26.9 s on this loaded host:
  the full Corvint tree export plus one target run and one reverted run. After the review fixes,
  the same map gave the same `survived` row with `elapsedSeconds` 7.2 (13 s wall) on a less loaded
  host. The added latency is therefore about 7 to 27 s per runnable package on this host.
- **K5.** No dogfood command passes the flag, so default dogfood latency is unchanged. Turning the
  flag on adds roughly one export plus two `go test` runs per runnable package.
- **Fixtures.** The `internal/cem/workflow` fixture (three candidates, one module) took 18.6 s.

## Evidence limits

- **K1 stays open.** The kill test measures `held` across the next 30 dogfood changes after P2
  lands. It needs producers to cite guarding tests, and no measurement exists yet. The first real
  candidate above survived its revert.
- **Out-of-scope help text.** `cmd/corvint/help.go` documents `--citation` but not `--hunk-revert`.
  That file is outside this lane's paths.
- **Possible native defect (not confirmed).** A real `go test -json` build failure emits
  `build-output`/`build-fail` events that carry `ImportPath` and no `Package`. The native
  `parseGo` in `internal/testrunner/native` may refuse such a stream as "Go event package
  missing". The falsifier's own decoder handles both shapes; the native path was not exercised
  here.

## Independent review (Codex, one read-only round)

The prompt asked whether any path yields `held` without a consumer-observed failure and whether
verify can mutate the caller's repository. Log: the lane scratchpad `codex.log` (not committed).

- **Fixed, high: a printed failure made a row `held`.** A `TestMain` that prints `=== RUN`,
  `--- FAIL: TestGuard` and `PASS` framing lines and exits 0 produces a test-level `fail` event in a
  package whose result is `pass`. This was reproduced on go1.27.2. `held` now also requires the
  package's own `fail` result, which `go` derives from the binary's exit status; the inconsistent
  stream is `not-run: infrastructure`. The recorded stream is pinned in `TestCEMV1089ClassifyRevert`.
- **Fixed, medium: declaration-shaped text named a test.** The span was matched by a regular
  expression, so a comment or raw string in the span could name a test declared elsewhere.
  `declaredTests` now parses the cited base file (`go/parser`, standard library) and names only
  top-level declarations whose header lies inside the span. `TCQ-V0-061` was reworded to match.
- **Fixed, high: a revert write could follow a symlink.** A baseline test could replace the copied
  file, or a directory above it, with a symlink to a caller file before the revert and restore
  writes. Reads and writes now check every component with `Lstat`; the old entry is unlinked and
  the new file is created exclusively. Covered by `TestCEMV1088CopyWritesNeverFollowSymlinks`.
- **Fixed, medium: a Go cache inside the caller's repository.** A `GOCACHE`, `GOMODCACHE` or
  `GOPATH` that points into the caller's repository is moved into the disposable workspace, and
  `GOENV=off` stops a user go env file from redirecting a cache or adding flags.
- **Recorded, not fixed: test code runs unconfined.** Executing the target's tests is the
  falsifier's definition. Test code can write anywhere the caller can, and test code written to
  fail when its source is reverted makes a row `held`. Corvint's own operations only read the
  caller's repository. Confinement such as Landlock on Linux is a recorded limit (spec
  "Recorded limits" and the release-notes text above).

## Rollback

Revert the V1-1266 code commit. No committed map, seal, golden or public case depends on the
flag, the `hunkRevert` member or the DOGFOOD step.
