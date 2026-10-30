# Lane script friction: N-1 default, RAM-backed TMPDIR, plain-clone offload (V1-3202, V1-3203, V1-3205, V1-3206)

Four lane-friction tickets from the 2026-10-30 friction sweep, built on the queued N-1
first-release and citation-replace branches (merge `8a692b79d`).

## Decisions

- **V1-3203, CCF-V1-007.** `make core-n1-replay` with `CORE_N1_TAG` unset no longer uses
  `git describe --exclude '*-*'`, which skipped every pre-release and left the default empty after
  rc.3. It takes `script/select-previous-release.sh`'s tag for `HEAD`, the selector
  `script/core-n1-source.sh` already uses, so pre-release tags count. Selector exit 3 prints
  `core-n1-replay: NOT_APPLICABLE FIRST_RELEASE ...` and exits 0; a selector refusal exits 2; an
  explicit empty `CORE_N1_TAG=` is still refused. The manual default does not check for a published
  release; `script/core-n1-source.sh` stays the CI path that does.
- **V1-3206 and V1-3205, ASD-V0-055.** Cause: this host's `/tmp` is a 16 GiB tmpfs. In the same
  batch-precheck run where `capability-catalog-check` failed `rc=2` and passed on its rerun,
  batch-drift's `TestCorvintBundledBinariesBuildTwiceIdentically` (eight cold builds, each with its
  own caches under `t.TempDir()`) hit `compile: writing output: ... no space left on device`. The
  catalog check compiles its test binaries and `go build`s `corvint-test` in the same `TMPDIR`.
  Six minutes earlier the same lane ran
  `bash script/batch-precheck_test.sh >/tmp/claude-1000/t.log 2>&1`: both it and
  `check-decision-numbers_test.sh` exited 1 with an empty log written to that full `/tmp`, and both
  passed when rerun. The empty log was the full filesystem swallowing the test's own FAIL line. No
  race of the test's own was found: 12 runs before the change and 20 after, four at a time through
  the host semaphore, all passed. Fix: batch-precheck moves its own and every step's `TMPDIR` off a
  tmpfs/ramfs to `${XDG_CACHE_HOME:-$HOME/.cache}/corvint-tmp` (or `BATCH_PRECHECK_TMPDIR`), and a
  `FLAKY` step prints its first attempt's last 20 lines, so the next one-off failure keeps its cause.
  The test's `fail` now names the numbered case and line, tails the checked output and prints the
  free space of `TMPDIR`; an exit that bypasses `fail` says so.
- **V1-3202, ASD-V0-056.** Every clone already carries `script/heavy.sh`. Offload failed in a plain
  clone because a `git clone` of a lane checkout has that checkout as `origin`. Its `origin/main` is
  the checkout's local `main`, an unpushed commit or the pre-snapshot lineage (decision 0331). The
  identity therefore mismatched the remote clone, or no merge base existed, and the offload returned
  97 silently and ran locally. Now an origin that is a local repository with its own origin is
  followed one hop, for the identity, the CI-guard repository and the `origin/main` base. Every
  local precondition failure prints `heavy.sh: offload unavailable from this checkout: REASON`.
  `HEAVY_LOCAL_REASON` is unchanged. LANE-ORCHESTRATION section 2 documents the fallback: the clone's
  own `script/heavy.sh`, with host settings in `HEAVY_CONFIG`.

## Evidence

- `script/core-n1-replay-default_test.sh` (new, in `make install-lifecycle-test`): no tag,
  tag only on HEAD, explicit empty, rc.3 predecessor selected over v0.8.1, explicit tag wins,
  selector refusal; PASS. `script/core-n1-source_test.sh` and
  `script/select-previous-release_test.sh` PASS.
- `script/batch-precheck_test.sh`: 32 of 32 PASS (12 before, 20 after), 4-way parallel; new
  section 10 and the section 8 `attempt 1:` line pass. A probe failure printed
  `FAIL case "2. A fixture repository, workflow and packages." line 117` with the output tail.
- `script/lane-scripts_test.sh`: 28 groups PASS; the plain-clone offload case fails against the
  parent commit's `script/heavy.sh` (it ran locally).
- `make capability-catalog-check` repeated under concurrent load: recorded in the lane report.

## Limits

NOT_OBSERVED: the ticket bodies (the four tickets are not in this host's Tasks projection); the
original failing catalog-check log (batch-precheck kept only the FLAKY line). The tmpfs cause is
inferred from the same run's `no space left on device` and was not reproduced by filling `/tmp`,
because that would break other lanes. The double build's own disk use is unchanged.

Rollback: revert this change; see the ASD-V0-055/056 and CCF-V1-007 rollback notes.
