<!-- SPDX-License-Identifier: Apache-2.0 -->
# scr-w55: portable lane tests, bounded scratch, and closeout replay measurement

Base: `5d1ed52b17d61147bd8c04b08afa6da39fa80afb` (`origin/main` at dispatch).
Branch: `claude/lane-scr-w55-1103`. Human-owned intent and acceptance are the lane charter's
V1-4329, V1-4333, V1-4334, V1-4319 and V1-3398 criteria. The builder may not commit, push,
run Tasks or invoke Codex. This is implementation/evidence handback, not native ticket completion.

## Decisions and previously delivered behavior

`git log --all --oneline --grep=V1-4329` identifies `c930d3e70` and `6263ea1cf` on main:
the dedicated `BATCH_PRECHECK_LANE_CLOSEOUT=1` mode, both real closeout recipe opt-ins,
NOT_RUN receipt binding, and refusal to reuse deferred evidence for batch push already exist
(`ASD-V0-1990..1992`). Group 17 of `script/batch-precheck_test.sh` owns both lane passes and
ordinary batch behavior; group 15 retains changed-input selection. No duplicate deferral change
is needed. The earlier build log explicitly lacked real timing; the measurement below adds it.

`ASD-V0-734` already gives local jobs private `heavy-run.XXXXXX` TMPDIRs, forwards INT/TERM/HUP,
waits for timeout/cap termination, and removes their directories. Group 32 already exercises real
`timeout 1` termination and cap cleanup. Its fixture now also owns an EXIT cleanup for `leak.*`,
chmods owner-write first, proves a failing fixture cleans up, and retains enclosing-root cleanup
for a shell killed before its trap can run. No production heavy runner behavior is broadened.

`DCW-V0-396` already refuses `dogfood inputs-check` run by a stale binary with exit 2 and
`self-binary-stale`, including the explicit `--corvint-bin` path used by CORVINT_BIN wrappers.
`TestDogfoodFlowRefusesStaleBinary` is the retained regression, including the inputs-check case.

Before adding requirements, the generator's static capacity check
`script/gen-spec-requirements.sh --check` passed and warned at **244/256** obligations in
`corvint-self-development-v0.md`. The same section-ID rule independently counted 244. Adding
only `ASD-V0-2190..2194` brings it to **249/256**, so a companion spec is unnecessary.
All new acceptance markers use the owner's standing approval, not an invented owner read-back.

The Bash 3.2 gate now checks shell-test portability as well as language syntax: explicit mktemp
patterns, whitespace-stripped string counts and make 3.81 option compatibility. Existing mistakes
in the lane suite and other selected shell tests are fixed. Numeric comparisons remain allowed.
Related fixture assertions share selectable names rather than silently executing prerequisite test
groups; numeric failure IDs remain stable. Unset/empty selects everything and unknown names refuse.
Failure logs remove wrapper banners before constructing the bounded excerpt.

The optional operator sweep is dry-run by default. Only immediate, caller-owned Corvint test
scratch and heavy per-job directories with exact six-character letter/digit/underscore suffixes qualify, and every entry
must be older than 24 hours. Symlinks, fresh/unreadable/foreign content, observed active use and
failed ps/lsof inspection keep the candidate. Apply rechecks use and age, then adds owner directory
permissions before removal. It never sweeps generic /tmp, caches or semaphore directories.
A process starting after the final inspection remains a stated race; documentation requires idle
lanes. Rollback stops the operator sweep and removes selectors/lint while retaining private-root
cleanup and the existing batch baseline check.

## Corvint use and limits

At change start, `make dogfood-change BASE=5d1ed52b17d61147bd8c04b08afa6da39fa80afb
TASK='Harden lane shell test portability, group selection and temporary cleanup; measure baseline
deferral' PATHS='script/lane-scripts_test.sh script/heavy.sh script/check-bash32.sh'` refused with
`mkdir: Read-only file system` in `.git` (exit 2). Direct `corvint affected --base BASE` and
`corvint context --task 'Harden lane shell test portability and temp cleanup' --subject
script/lane-scripts_test.sh` were retained under `/tmp/scr-w55/` before implementation.
The context command rebuilt its index in memory and retained omissions; no omitted evidence is
claimed examined. A meaningful dirty-diff affected run selected 67 units, excluded 552 and retained
two LANGUAGE_FRONTIER unknowns. Focused shell/spec tests and the stale-binary regression address
this task; the transitive selection is not represented as exhaustive behavioral coverage.

The real index is read-only (`git add docs/specs`: cannot create index.lock). Required staging
uses `GIT_INDEX_FILE=/tmp/scr-w55/index`, `GIT_OBJECT_DIRECTORY=/tmp/scr-w55/objects` and the
real object store as a read-only alternate; `git read-tree HEAD; git add docs/specs` succeeds.
Generation/checking use that same staged index. The integrator still must stage the delivered
files in the real index. CEM binding/checking/sealing, publication and native completion are
NOT_PRODUCED under the no-commit/no-Tasks charter; they are not silently waived or claimed passed.
The full repository gate, frozen product evaluations and Darwin/Windows qualification are NOT_RUN
for this scoped shell-only lane. No changed Go files require gofmt or Windows vet.

## Independent review

One independent read-only reviewer found selector prerequisites still scoped inside `core`, an
unset offload path in standalone reaping, an arithmetic-equality false positive, an awk hexadecimal
escape unsuitable for the portable rule, and a sweep readability check limited to directories.
The final implementation moves shared helpers/paths to common setup, limits the WC refusal to
string tests, uses a literal quote, and retains unreadable files as well as directories. Focused
reruns below cover these corrections. No broader sweep/path-escape finding was reported.

## Measurement

The narrow equivalent is **the production closeout pre-bind selection-baseline doc-gate row**,
including real planning/selection and the real Make target, at one existing Go-touching lane commit.
It excludes other closeout steps and the actual CEM seal commit, which the charter forbids. It is
not a full closeout/seal duration or a synthetic fixture timing. Both runs are in a throwaway shared
clone of this worktree checked out at `990f74e8a045ecece447053cebf733d645560ed7`, against its parent
`8dd7907fbbdc7333b735584285285ada6275a37e`. That real diff touches
`internal/tasks/cli/read_wait_test.go` and `script/dogfood-cache_test.sh`.

| Production pre-bind baseline span | Wall seconds (`/usr/bin/time -f '%e'`) | Result |
|---|---:|---|
| Before: only the V1-4329 deferral branch disabled | **536.85** | exit 0; real build/replay; floor passes |
| After: unmodified production deferral | **2.88** | exit 0; explicit NOT_RUN deferral; no baseline target/build/replay |
| Observed difference in this span | **533.97** | One paired observation, no general speedup or whole-seal claim |

The before run replayed all 58 fixed changes: `narrow=30/43 merged (rate 0.698) bounded=0 misses=0
over 26 failing packages of 15 failed CI runs; median cost 0.945; whole=9331 sliced=2
whole-source-and-tests=26`; `selection-baseline: OK floor narrow>=30 misses<=0`.
The after row is `printf "%s\\n" NOT_RUN:...` and prints
`NOT_RUN: lane closeout: selection-baseline-check deferred to the batch precheck (ASD-V0-1990)`.
Neither run waited for a heavy slot (queue=0.0 s). The before wrapper reports run=536.4 s; the after
reports run=2.7 s. The tool is `Corvint 1.0.0-rc.3.dev (build 49, commit
5d1ed52b17d61147bd8c04b08afa6da39fa80afb)`, Go `go1.27.3`, Linux, 32 CPUs. During the run,
`/proc/loadavg` was `25.14 32.56 22.18`; exact entry/exit load is NOT_OBSERVED. The task cache is
shared between paired runs and independent checks ran concurrently. No repeated-sample statistics,
cold-cache comparison or end-to-end seal savings are inferred.

The real planner retained two shell-frontier unknowns (including the untracked measurement
harness); the before path explicitly printed `selection-baseline-check runs (a Go input changed
and the plan is not used)`. The after path printed the dedicated deferral under the same conditions.
This tests the admitted fail-safe branch with real inputs and workload, without claiming the plan
narrowed successfully. Plain batch execution remains covered by the passing batch-precheck suite.

Exact setup and measured commands (from the task root; paths deliberately remain task-private):

```bash
mkdir -p /tmp/scr-w55/tmp /tmp/scr-w55/cache /tmp/scr-w55/locks
export TMPDIR=/tmp/scr-w55/tmp GOCACHE=/tmp/scr-w55/cache CORVINT_GOCACHE=/tmp/scr-w55/cache
export HEAVY_CONFIG=/dev/null HEAVY_LOCK_DIR=/tmp/scr-w55/locks
export HEAVY_SELECTION_LOG=/tmp/scr-w55/selection.log HEAVY_FALLBACK_LOG=/tmp/scr-w55/fallback.log
export HEAVY_REMOTE_HOST= HEAVY_REMOTE_HOSTS= HEAVY_MIN_FREE_GB=0 HEAVY_REFUSE_FREE_GB=0
export HEAVY_MAX_LOAD=99999 HEAVY_CACHE_TRIM_HOURS=0 HEAVY_TMP_FSTYPE=disk CORVINT_GATE_LEDGER=off
git clone --quiet --shared --no-checkout . /tmp/scr-w55/measure-clone
git -C /tmp/scr-w55/measure-clone checkout --quiet --detach 990f74e8a045ecece447053cebf733d645560ed7
```

The exact preparation script, invoked once with `before` and then with `after`, is:

```bash
#!/usr/bin/env bash
set -euo pipefail
cd /tmp/scr-w55/measure-clone
# Load the production precheck definitions, stopping before its CLI dispatch/main runner.
awk '/^case \$\{1:-\} in$/ { exit } { print }' script/batch-precheck.sh > script/scr-w55-closeout-measure.sh
if [[ $1 == before ]]; then
  # Remove only the already-landed V1-4329 selection-baseline deferral, preserving lane mode.
  sed '/^select_baseline()/,/^}/s/if \[\[ \$lane_closeout == 1 \]\]; then/if false; then/' script/scr-w55-closeout-measure.sh > /tmp/scr-w55/measure-before.sh
  cp /tmp/scr-w55/measure-before.sh script/scr-w55-closeout-measure.sh
fi
cat >> script/scr-w55-closeout-measure.sh <<'MEASURE'
base=$1
dir=$(mktemp -d "$TMPDIR/corvint-seal-measure.XXXXXX")
trap 'rm -rf "$dir"' EXIT
need_gocache
select_plan
select_report
# Execute exactly the production doc-gate row the closeout's pre-bind precheck would run.
row=$(doc_gate_rows '' | awk -F '\t' '$1 == "make selection-baseline-check" {print; found=1} END {if (!found) exit 1}') || exit 2
printf 'MEASURE row=%s\n' "$row"
bash -c "${row#*$'\t'}"
MEASURE
```

The timed invocations, sequentially and with each preparation completed first, were:

```bash
bash /tmp/scr-w55/prepare-measure.sh before
cd /tmp/scr-w55/measure-clone
base=$(git rev-parse HEAD^)
BATCH_PRECHECK_LANE_CLOSEOUT=1 BATCH_PRECHECK_SKIP_DRIFT=1 /usr/bin/time -f '%e' -o /tmp/scr-w55/before.seconds script/heavy.sh bash script/scr-w55-closeout-measure.sh "$base" > /tmp/scr-w55/measure-before.log 2>&1
bash /tmp/scr-w55/prepare-measure.sh after
BATCH_PRECHECK_LANE_CLOSEOUT=1 BATCH_PRECHECK_SKIP_DRIFT=1 /usr/bin/time -f '%e' -o /tmp/scr-w55/after.seconds script/heavy.sh bash script/scr-w55-closeout-measure.sh "$base" > /tmp/scr-w55/measure-after.log 2>&1
```

The first harness attempt failed with `dir: unbound variable` before executing a baseline row.
It is excluded from the comparison. The preparation script above includes the corrected private
plan directory. The before-only `if false` changes the already-landed branch in the extracted
production function, preserving all other lane-mode behavior and the exact Go commit; the after
uses the unmodified source. No task or measurement commit was created.

## Verification, retained failures, and friction

- Complete `script/batch-precheck_test.sh` through heavy.sh: PASS, exit 0, run=279.4 s.
- Stale binary regression: `script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m
  -run '^TestDogfoodFlowRefusesStaleBinary$' ./cmd/corvint`: PASS, package 0.429 s, wrapper 53.7 s.
- Focused lane names `admission portability sweep reap`: PASS, eight groups. The matrix gives
  identical one-group output for all eight TMPDIR/cache combinations.
- `env -u CORVINT_GATE_LEDGER bash script/check-bash32_test.sh`: PASS, including quoted/reversed
  WC equalities, numeric positive cases and GNU make long/short options.
- `make spec-requirements` with GO_TEST_COMMAND explicitly through heavy.sh, staged specs followed
  by `make spec-requirements-check`: PASS. Requirement collisions: no collision for five new IDs
  against 42 open refs. Capacity is 249/256.
- `make line-citations-check` after staging and `make flow-guide-check`: PASS (45 spans, 16 mapped
  refusal codes, 299 refusal sites). Citation checking retains 180 skipped non-root basenames.
- `bash script/pre-push-dogfood-seal_test.sh`: PASS. Runtime-environment regression through heavy.sh
  with `XDG_CACHE_HOME=/tmp/scr-w55/xdg`: PASS, wrapper 51.6 s. Its existing bind-range fixture
  first failed against read-only HOME cache; the retry's XDG cache symlink points at the task cache.
- First complete lane run: 60 passed / 1 failed, the newly added probe lost its lock override after
  the workflow group. Fixed by an explicit private probe lock. The next run also passed 60 / 1
  failed because a banner-filter edit accidentally changed an unrelated script-hash assertion;
  that assertion is restored. Neither failed run is promoted to PASS. Terminal result follows.
- Observed real heavy temp suffix `8_zJW_` includes underscores. The sweep allowlist and fixture
  explicitly cover that portable suffix alphabet; symlink-root spellings with a trailing slash
  are also refused. No existing host directories were swept.
- Final affected advice after the required root handback exists: dirty=12, selected=72,
  excluded=547, unknown=77 (76 LANGUAGE_FRONTIER and one UNOWNED_DIRTY_PATH for
  `scr-w55.handback.md`). Before that report artifact the meaningful code diff retained two shell
  frontier unknowns. These are advisory limits, not test failures or authority to waive a gate.
- Bash syntax and gofmt changed-file check: PASS / no changed Go files. No Windows vet is needed.
- Initial dogfood collection and real staging failed read-only; private staging is not a real
  index write or a CEM seal. Integration and native ticket completion remain with the coordinator.


Terminal result: `LANE_SCRIPTS_GROUPS=all bash script/lane-scripts_test.sh` **PASS, exit 0,
61 groups passed**. The expected malformed-cache refusal is fixture output, not a failed group.
All final reviewer findings are closed by code and passing checks; the reviewer performed only
read-only review. Final Bash syntax/lint and whitespace checks pass. No task commit or native
completion is claimed.

## Review fixes

An independent Codex review of the lane commit reported two findings. Both reproduced, and both are
fixed in place under the lane's existing requirement IDs.

- P1, `ASD-V0-2190`: the new `portable_test` path filter wrote `[^/]` inside a regex literal. BWK
  awk (macOS `/usr/bin/awk`) ends a regex literal at the first unescaped slash, even inside a
  bracket expression, so the whole scanner fails with "nonterminated character class" and
  `make bash32-check` cannot run on a Mac. POSIX awk lexing likewise requires `\/` inside an ERE
  token. The filter is now `[^\/]`. This host has no BWK awk (only gawk 5.3.2 and mawk 1.3.4), so
  the macOS failure was not executed here. Instead, `script/check-bash32_test.sh` now lexes the
  scanner program as BWK does and refuses any literal that leaves a bracket open. It reads all 28
  literals; an independent count of `~ /`, `( /` and `, /` openings also gives 28. It flags the
  HEAD form at line 47 of the program and passes the fixed one. A nested
  `script/sub/*_test.sh` fixture shows that the escaped form keeps its meaning. The test passes
  under gawk and under mawk (through a PATH shim).
- P2, `ASD-V0-2192`: selecting `workflow` alone failed group 20. `hv`, `conns`, the stub repos and
  the `gh` stub were defined only inside offload groups 8, 17 and 18. Without the `gh` stub, a
  standalone build ran the caller's real `gh` and logged `runs=2` from live CI, which breaks the
  no-real-CI rule of `ASD-V0-040`. Group 20's clean-index assertion also depended on group 17's wip
  commit. Selecting `tools` alone hit the unset `$d` and `$h` in group 55, which were defined only
  in policy group 27. A Bash pipeline subshell that dies on nounset runs the parent's EXIT trap,
  which deleted `$tmp` and its failure ledger, so the run reported "18 groups passed" with exit 0.
  Fixes:
  - The offload fixture now runs in one shared block for `all|offload|workflow`: stubs, checkout,
    `hv`, `conns` and `gh`.
  - The log regexes are shared constants `stamp_re` and `tail_re`, renamed because offload loops
    reuse `h`.
  - Group 20 compares the tracked status before and after its builds.
  - `summary` refuses with exit 2 when the ledger is missing. The new `--ledger-self-test` is
    checked at startup: it gives exit 0 without the change and exit 2 with it.
- Every advertised selector now passes alone, and so does the complete suite (61 groups).
