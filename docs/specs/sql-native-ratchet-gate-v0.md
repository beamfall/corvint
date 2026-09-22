# SQL-native ratchet gate V0

Owner: Russell Lewis
Date: 2026-09-07
Requirement prefix: `SNR-V0`
Intent status: accepted (owner instruction 2026-09-07)
Delivery status: partial
Authoritative inputs: `../../AGENTS.md` invariant 8, `../SPEC-DRIVEN-DEVELOPMENT.md`,
`analyzer-candidate-profiles.md` requirement `ACP-009` (proposed, not accepted — source of the
65,536-byte base source ceiling, the 6,291,456-byte base binary ceiling, and the sql-native
per-family allocation row of 98,304 B/op and 400 allocs/op), `../decisions/0226-sql-native-core-base-is-head-without-candidate-2026-09-13.md`
(the Core comparison base), `../agent-memory/fixes.md` (the
backlog entry naming this script as unspecified), and `../BUILD-LOG.md` heading "Gate tooling is in
scope for invariant 8" (the self-correction stating this script "has no caller").

## Agent digest
- Claim: An opt-in, content-pinned-toolchain ratchet bounds the experimental sql-native analyzer's budgets; its lost causal-parent comparison abstains.
- Status: accepted (owner instruction 2026-09-07) / partial
- Exists: `script/check-sql-native-ratchets.sh`, called only by the opt-in `make sql-native-ratchets` target (not a `gate` prerequisite) and absent from `.github/workflows/ci.yml`. `script/check-sql-native-ratchets_test.sh` now covers `SNR-V0-001`, `SNR-V0-002`, the `SNR-V0-005` source-byte ceiling boundary, the `SNR-V0-007` Core-base export, and the `SNR-V0-013` causal-parent presence guard, all against disposable fixture repositories. Decision 0226 replaced the stale `core_parent` pin with `HEAD` minus the candidate's two source trees.
- Blocked on: the `SNR-V0-013` parent-separation evidence cannot be produced here because `causal_parent` and its source checkpoint are absent from the object database — never claim it has been produced. With decision 0226's Core base, a real 2026-09-13 invocation completed and printed the `SNR-V0-014` line with the parent half `NOT_RUN` (see Verified current state). `SNR-V0-012`'s fixture copy into the parent export has never run.
- Read next: Verified current state; Requirements; Trust boundary/failure modes.

## Human intent and scope

`experimental/analyzers/sqlnative` and `cmd/corvint-analyzer-sql-native` are an unregistered
candidate analyzer (`analyzer-candidate-profiles.md`). A candidate in that lifecycle earns
promotion evidence, not trust by inspection: it must stay within an absolute source and binary
budget, it must not grow or alter the Core `corvint` binary it is not yet wired into, it must
build for every shipping target platform, and its measured allocation cost must both meet an
absolute ceiling and beat a specific historical implementation on an identical workload. This
script is the one place all of those checks are assembled and run together for this one candidate.

Affected user: an agent or engineer deciding whether the current `sql-native` candidate tree may be
carried forward. Measurable job: for one working tree, its `HEAD` without the candidate, and one
pinned historical commit, refuse to
report success unless every ceiling and every identity check named below holds, and — when they all
hold — print one line naming every measured value.

The owner instruction of 2026-09-07 places repository gate tooling in scope for AGENTS.md invariant
8. This spec discharges that for `check-sql-native-ratchets.sh` only. The decision-number,
requirement-definition, and traceability scripts have their own gate specs. The remaining Python
build/ratchet and release-reproducibility wrappers now have the proposed
`optional-artifact-check-scripts-v0.md` backfill; owner acceptance remains pending
(`../agent-memory/fixes.md`).

## Verified current state

`script/check-sql-native-ratchets.sh` is **not** a prerequisite of the `gate` target in `Makefile`.
Its one caller is the explicit opt-in `make sql-native-ratchets` target, whose `Makefile` comment
states it is excluded from `gate` because it builds `cmd/corvint` twice and runs 30 benchmark
samples; it does not appear in `.github/workflows/ci.yml`. The 2026-09-07 `../BUILD-LOG.md`
correction ("it has no caller") predates that target.

`script/check-sql-native-ratchets_test.sh` now exists (added 2026-09-13), sized like the sibling
`check-line-citations_test.sh`: it copies the script into a disposable fixture repository and
exercises `SNR-V0-001` (held-lock refusal), `SNR-V0-002` (`INVALID_LOCK_TEST` refusal and `spin`-mode
concurrent-run detection plus TERM-triggered lock release), the `SNR-V0-005` source-byte ceiling
boundary (at-ceiling proceeds past the Core-base export to the receipt build, which finds no main
module in the fixture; one byte over fails silently before that), the `SNR-V0-007` Core-base export
(the script's own `git archive` command keeps Core source and excludes both candidate trees), and the `SNR-V0-013` `git cat-file -e "$causal_parent^{commit}"` presence
guard (false for the pinned, absent `causal_parent`; true for a real commit). `SNR-V0-003`/`004`,
`006`–`012` (including the `SNR-V0-007` binary comparison), and `014` remain exercised only by a real invocation, per the script's own cost (two
`cmd/corvint` rebuilds and 30 benchmark samples).

A real invocation of `script/check-sql-native-ratchets.sh` via `make sql-native-ratchets` was run
2026-09-13 on this host (pinned toolchain at `/opt/homebrew/Cellar/go/1.27.0/bin`) and refused at the
`SNR-V0-005` source-byte ceiling: `source_bytes=65908` against the 65,536-byte ceiling (372 bytes
over), before any git archive, build, or benchmark step ran. Per-file breakdown (`wc -c` on every
non-test `.go` file under `experimental/analyzers/sqlnative` and `cmd/corvint-analyzer-sql-native`):
`read_windows.go` 402, `read_linux.go` 625, `cmd/corvint-analyzer-sql-native/main.go` 1014,
`read_darwin.go` 1088, `read_unix.go` 3482, `analyzer.go` 59297. This is the script correctly
enforcing its documented ceiling, not a script defect (lane rules forbid touching
`experimental/analyzers/sqlnative` source outside a defect a red test shows), so it is recorded here
and in `../agent-memory/bugs.md`, not fixed by this spec's delivery.

Later on 2026-09-13, `experimental/analyzers/sqlnative/analyzer.go` was shrunk with no output change
(59,297 to 57,542 bytes). The work removed dead guards, inlined single-use dispatch, shared the fact and
input-identity comparison, and merged duplicated lexer and literal branches. Emitted bytes and
allocations are unchanged: the package tests pass, and `BenchmarkAnalyzeCandidate` measured 393
allocs/op and about 93,467 B/op both before and after. The same script was then rerun on this host
with the pinned toolchain. It passed
`SNR-V0-003` (go binary digest and `GOVERSION=go1.27.0`), `SNR-V0-004` (receipt tool digest and
toolchain tree `{"Entries":17277,...}`, including the brackets around the candidate and both core
builds), `SNR-V0-005` (`source_bytes=64153`, 1,383 bytes under the ceiling), and `SNR-V0-006`
(`binary_bytes=2832594`). The `SNR-V0-013` guard set `causal_available=false`. It then refused
at `SNR-V0-007`, where `cmp` reported the current-tree and `core_parent` `cmd/corvint` binaries differ
at `char 17, line 1`. The cause is the pin, not this script or the candidate. `core_parent`
(`718dfc7d`, 2026-08-25) is 1,842 commits behind this tree, and `cmd/corvint`, `internal`, and
`go.mod` differ from it by 1,290 files. Byte identity with that commit cannot hold on the current tree.
`SNR-V0-008` onward did not run. Decision 0226 then replaced the pin: the Core base is `HEAD` with
`experimental/analyzers/sqlnative` and `cmd/corvint-analyzer-sql-native` excluded (see `SNR-V0-007`).

The script with decision 0226's Core base was then run on this host via `make sql-native-ratchets`
with the pinned toolchain. The lane's script and documentation edits were uncommitted, and none of
them is in the `cmd/corvint` build closure. It exited 0 and printed:
`sql-native-ratchets core_base=469c2478840858d38c3b91545016d949407cf34e causal_parent=a7db6200685c1ea7d4afc1798fb376ccc724acd1 fixture_sha256=13446028891448c376d6acfd0e12b70abf516a6c54c1bfa19dca68d1d1b0decc source_bytes=64153 binary_bytes=2832594 goroot_entries=17277 goroot_sha256=94b2c0ed86c9f62518348cc7fc3e4442e4e40a0194d10cfc8f6bc1be98447c33 core_binary=identical core_deps=identical complexity=delimiter-pairs-linear-old-parenAt-model-fails candidate_max_b_op=93462 candidate_max_allocs_op=393 causal_candidate_max_b_op=93462 causal_candidate_max_allocs_op=393 causal_parent_min_b_op=NOT_RUN causal_parent_min_allocs_op=NOT_RUN causal=NOT_RUN-causal-parent-unavailable latency=NOT_RUN`.
`SNR-V0-003`–`011` passed, and `SNR-V0-013` enforced the current-tree ceiling (393 allocs/op) while the
parent half abstained. A scratch commit then added
`cmd/corvint/zz_scratch_wiring.go` with a blank import of `experimental/analyzers/sqlnative`. The same
target exited 2 at the Core-base build: `no required module provides package
github.com/corvint-context/corvint/experimental/analyzers/sqlnative`. The scratch commit was then reset and
the file deleted.

ACP-009's per-executable check (`internal/analyzercap/source_ceiling_test.go`) measures a different
file set: the host build closure of `cmd/corvint-analyzer-sql-native` (4 production files, excluding
`read_linux.go` and `read_windows.go` on darwin). On darwin it measured 63,126 bytes against 65,536 after the
shrink, inside its 4,096-byte warning band but passing. `SNR-V0-005` counts every non-test `.go` file
for all platforms.

The Go symbols the script depends on are real: `TestSQLite351NestedWithDelimiterWorkRatchet` and
`BenchmarkAnalyzeCandidate` are defined in `experimental/analyzers/sqlnative/analyzer_test.go`, and
`BenchmarkAnalyzeCausalSQL` is defined in
`experimental/analyzers/sqlnative/causal_benchmark_test.go`.

Of the script's two former pinned comparison commits, `core_parent` (`718dfc7db3e9162f045669309e2f28dc73487aff`,
removed by decision 0226) resolved to a real commit in this repository. `causal_parent`
(`a7db6200685c1ea7d4afc1798fb376ccc724acd1`) does not: `git rev-list -1
a7db6200685c1ea7d4afc1798fb376ccc724acd1` fails with `fatal: bad object
a7db6200685c1ea7d4afc1798fb376ccc724acd1`, verified 2026-09-07 in this working tree. The script's
`git archive "$causal_parent"` step ran unconditionally before any ratchet assertion, so until
decision 0104 the script could not complete a run in this repository. This spec does not attempt to run the script
to confirm the exact downstream failure text; the object-database check above is sufficient and
read-only.

## Requirements

- `SNR-V0-001`: The gate MUST refuse a concurrent invocation. It MUST take an exclusive lock via
  `mkdir` under the repository's git directory before doing any other work, and MUST exit 75 with
  `reason=CONCURRENT_RUN` when the lock is already held. It MUST release the lock on every exit
  path, including `HUP`, `INT`, and `TERM`, via a trap.
- `SNR-V0-002`: The gate MUST support a test-only lock-hold mode selected by the
  `SQL_NATIVE_RATCHET_LOCK_TEST_HOLD` environment variable: unset or empty runs normally, the value
  `spin` busy-loops forever so a companion invocation can exercise `SNR-V0-001`'s
  `CONCURRENT_RUN` path, and any other non-empty value MUST exit 64 with `reason=INVALID_LOCK_TEST`.
- `SNR-V0-003`: The gate MUST discover the toolchain root as `GOTOOLCHAIN=local go env GOROOT` and
  MUST refuse to proceed unless that root is non-empty, its `bin/go` is executable, that binary's
  SHA-256 digest matches a pinned value, and its reported `GOVERSION` under `GOTOOLCHAIN=local` is
  exactly `go1.27.0`. Identity is pinned by content, not by path: any byte-identical installation
  passes wherever it lives, and no toolchain passes on its version string alone (decision 0104).
- `SNR-V0-004`: The gate MUST verify that the discovered Go toolchain root's content matches its pin
  by building a dedicated receipt tool from the current tree, verifying that tool's own SHA-256
  against a pinned value, then using it to hash the toolchain root (entry count plus a combined
  SHA-256) and comparing the result to a pinned value. The gate MUST repeat this verification
  immediately before and after every build and cross-compile step later in the run, so that a
  toolchain mutation occurring mid-run is caught rather than only checked once at the start.
- `SNR-V0-005`: The gate MUST sum the byte length of every non-test `.go` file under
  `experimental/analyzers/sqlnative` and `cmd/corvint-analyzer-sql-native` and MUST refuse when that
  total exceeds 65,536 bytes.
- `SNR-V0-006`: The gate MUST build `./cmd/corvint-analyzer-sql-native` with `-trimpath
  -buildvcs=false -ldflags='-s -w -buildid='` and MUST refuse when the resulting stripped binary
  exceeds 6,291,456 bytes.
- `SNR-V0-007`: The gate MUST build `./cmd/corvint` twice with identical build flags and
  equal-length output paths — once from the current working tree, once from a `git archive` export
  of `HEAD` (the Core base, `core_base`) that excludes exactly `experimental/analyzers/sqlnative`
  and `cmd/corvint-analyzer-sql-native` — and MUST refuse unless both builds succeed and the two
  resulting binaries are byte-identical. A Core that imports the candidate cannot build from the
  Core base and so refuses (decision 0226).
- `SNR-V0-008`: The gate MUST run `go list -deps` for `./cmd/corvint` in both the current tree and
  the Core-base export, sort each list, and MUST refuse unless the two sorted import lists are
  byte-identical. `SNR-V0-007` and `SNR-V0-008` together are the gate's evidence that the
  unreferenced `sql-native` candidate has not changed the Core binary or its dependency closure.
- `SNR-V0-009`: The gate MUST cross-compile `./cmd/corvint-analyzer-sql-native` for darwin/arm64,
  darwin/amd64, linux/amd64, linux/arm64, and windows/amd64, and MUST refuse if any of the five
  builds fails, with a toolchain-integrity check (`SNR-V0-004`) bracketing each target.
- `SNR-V0-010`: The gate MUST run `TestSQLite351NestedWithDelimiterWorkRatchet` in
  `experimental/analyzers/sqlnative` once and MUST refuse on any test failure. This is the gate's
  complexity-regression check for the analyzer's delimiter-handling model.
- `SNR-V0-011`: The gate MUST run `BenchmarkAnalyzeCandidate` ten times against the current tree at
  `-benchtime=100x`, take the maximum bytes/op and the maximum allocs/op across the ten samples, and
  MUST refuse unless that maximum is at most 98,304 bytes/op and at most 400 allocs/op.
- `SNR-V0-012`: When `causal_parent` is available (`SNR-V0-013`), the gate MUST copy the working
  tree's `causal_benchmark_test.go` into the `causal_parent` export before measuring, so that the identical fixture source runs against both
  sides of the comparison, and MUST record the SHA-256 of that fixture file in its report.
- `SNR-V0-013`: The gate MUST run `BenchmarkAnalyzeCausalSQL` ten times against the current tree and
  ten times against the `causal_parent` export, and MUST refuse unless the current tree's maximum
  allocs/op across its ten samples is at most 400 AND the `causal_parent` export's minimum allocs/op
  across its ten samples exceeds 400. This is a strict-separation check: every parent sample must be
  worse than the ceiling the candidate must meet in its worst sample. The gate measures but does not
  ratchet bytes/op for this comparison. When `causal_parent` is not a commit in the local object
  database, the gate MUST NOT substitute another commit: it still enforces the current tree's
  ceiling, skips the parent export and measurement, and reports both parent extremes and the verdict
  tag as `NOT_RUN` (decision 0104).
- `SNR-V0-014`: On completing every prior requirement without refusal, the gate MUST print exactly
  one structured `sql-native-ratchets ...` line to standard output naming the `core_base` commit and
the pinned `causal_parent` commit, the
  fixture digest, the measured source and binary byte totals, the pinned toolchain-root identity,
  the `SNR-V0-007`/`SNR-V0-008` identity verdicts, the `SNR-V0-010` complexity tag, all four
  `SNR-V0-011`/`SNR-V0-013` measured extremes (parent extremes `NOT_RUN` when abstained), the
  `SNR-V0-013` causal verdict tag or `causal=NOT_RUN-causal-parent-unavailable`, and an explicit
  `latency=NOT_RUN` marker naming a ratchet the script does not implement.
- `SNR-V0-015`: Every artifact the gate creates — the `SNR-V0-001` lock directory, the `git archive`
  exports, and every build, cache, and measurement directory — MUST live under the repository's git
  directory or under one `mktemp -d` scratch root, and MUST be removed by a trap on normal exit and
  on `HUP`/`INT`/`TERM`. The gate MUST NOT write to any tracked path in the working tree.

## Non-goals and simpler baseline

A far simpler baseline — the source-size check (`SNR-V0-005`), the binary-size check
(`SNR-V0-006`), and the candidate allocation ceiling (`SNR-V0-011`) alone — would catch gross size
and allocation regressions with none of the toolchain-identity, Core-isolation, or historical-parent
machinery. The full script additionally proves two things the simpler baseline cannot: that the
unreferenced candidate has caused zero byte or dependency change to the shipped Core binary
(`SNR-V0-007`, `SNR-V0-008`), and that the candidate's allocation profile is a measured improvement
over one specific named historical implementation on an identical fixture, not just under a static
budget (`SNR-V0-012`, `SNR-V0-013`).

This gate does not decide whether the `sql-native` candidate is promoted, registered, installed, or
launched — that lifecycle is `analyzer-candidate-profiles.md`'s, and this gate is one input to it.
It does not enforce that its own numeric thresholds only tighten over time: the four ceilings
(`SNR-V0-005`, `SNR-V0-006`, `SNR-V0-011`) are constants written into this one script, and nothing
here mechanically prevents a future edit from raising them; monotonic tightening is a review-time
convention borrowed from `ACP-009`'s per-candidate ceiling table, not a property this gate checks
for itself. It does not run on any host whose discovered `GOROOT` is not byte-identical to
the pinned installation (a darwin/arm64 Homebrew `go1.27.0`); the pins are platform-specific
content digests, which is the most plausible reason it was never added to the Linux-hosted CI
workflow. It does not measure or ratchet wall-clock latency (its own report says
`latency=NOT_RUN`), and for the causal comparison it does not ratchet bytes/op, only allocs/op, even
though bytes/op is measured and printed for both sides.

## Trust boundary, limits, and failure modes

The script reads the working tree, the local Go toolchain installation, `HEAD`, and one pinned
historical commit from the local object database; it treats none of them as adversarial, only as material that
must match pinned identities before any measurement is trusted. It writes only under `.git` (the
lock) and a `mktemp -d` scratch root (everything else), both removed by its exit trap. Failure is
almost entirely silent about *which* check failed: only the two lock-related failures
(`SNR-V0-001`, `SNR-V0-002`) print a structured `reason=`; every other refusal is a bare shell `test`
or `cmp` failing under `set -eu`, which stops the script with no `sql-native-ratchets ...` line and
no named reason, distinguishable from success only by its non-zero exit and whatever the failing
command itself wrote to standard error.

| Failure | Behavior |
|---|---|
| Concurrent invocation | fail, exit 75, `reason=CONCURRENT_RUN` |
| `SQL_NATIVE_RATCHET_LOCK_TEST_HOLD` set to an unrecognized value | fail, exit 64, `reason=INVALID_LOCK_TEST` |
| `go env GOROOT` fails or is empty; discovered go binary missing, wrong SHA-256, or wrong `GOVERSION` | fail, no structured reason |
| Toolchain-root identity mismatch at any `check_toolchain` call | fail, no structured reason |
| Source or binary byte total over ceiling | fail, no structured reason |
| Core imports the candidate, so the Core-base build or `go list` fails | fail, whatever `go build` or `go list` wrote to standard error |
| Core binary or dependency list differs from the Core base (including uncommitted or untracked Core changes in the working tree) | fail, no structured reason (a `cmp` diagnostic only); run on a committed tree |
| Candidate-driven change to shared root files such as `go.mod` | not detected: both sides share them; the candidate currently has no module requirement (decision 0226) |
| Any of the five cross-compiles fails | fail, whatever `go build` wrote to standard error |
| `TestSQLite351NestedWithDelimiterWorkRatchet` fails | fail, Go test output only |
| Candidate or causal allocation ratchet exceeded | fail, no structured reason |
| No `HEAD` commit | fail, a `git rev-parse` error |
| `causal_parent` commit absent from the local object database | parent half abstains: `causal_parent_min_*=NOT_RUN causal=NOT_RUN-causal-parent-unavailable`; confirmed absent in this repository 2026-09-12 |

## Acceptance criteria and testing matrix

`script/check-sql-native-ratchets_test.sh` (added 2026-09-13) is the dedicated test harness for
`SNR-V0-001`, `SNR-V0-002`, the `SNR-V0-005` ceiling boundary, the `SNR-V0-007` Core-base export, and
the `SNR-V0-013` presence guard. The first real 2026-09-13
invocation refused at `SNR-V0-005` (`source_bytes=65908`). After the `analyzer.go` shrink, the rerun
passed `SNR-V0-003`–`006` and refused at `SNR-V0-007` against the stale `core_parent` pin. After
decision 0226 the run completed with exit 0 and an `SNR-V0-014` line whose parent half is `NOT_RUN`
(see Verified current state and `../BUILD-LOG.md` 2026-09-13). `SNR-V0-012`'s fixture copy and the
`SNR-V0-013` parent measurement remain unexercised by a real run.

| Requirement | Evidence |
|---|---|
| SNR-V0-001, SNR-V0-002 | `sh script/check-sql-native-ratchets_test.sh`, exit 0, 2026-09-13 (fixture repositories; held lock, `INVALID_LOCK_TEST`, `spin`-mode `CONCURRENT_RUN`, TERM-triggered lock release) |
| SNR-V0-003, SNR-V0-004 | real 2026-09-13 post-shrink run: go binary digest, `GOVERSION=go1.27.0`, receipt digest, and every `check_toolchain` through both core builds matched |
| SNR-V0-005 | `sh script/check-sql-native-ratchets_test.sh`, exit 0 (ceiling boundary fixture); real 2026-09-13 runs: refusal at `source_bytes=65908`, then pass at `source_bytes=64153` after the `analyzer.go` shrink |
| SNR-V0-006 | real 2026-09-13 post-shrink run: `binary_bytes=2832594`, under 6,291,456 |
| SNR-V0-007, SNR-V0-008 | real 2026-09-13 decision-0226 run (`core_base=469c2478`): `core_binary=identical core_deps=identical`; `sh script/check-sql-native-ratchets_test.sh`, exit 0 (Core-base export keeps Core source, excludes both candidate trees); scratch commit wiring the candidate into `cmd/corvint` refused (see Verified current state). The earlier stale-pin refusal is superseded |
| SNR-V0-009 | real 2026-09-13 decision-0226 run (`core_base=469c2478`): all five cross-compiles succeeded |
| SNR-V0-010 | real 2026-09-13 decision-0226 run (`core_base=469c2478`): `complexity=delimiter-pairs-linear-old-parenAt-model-fails` |
| SNR-V0-011 | real 2026-09-13 decision-0226 run (`core_base=469c2478`): `candidate_max_b_op=93462 candidate_max_allocs_op=393` |
| SNR-V0-012, SNR-V0-013 | `SNR-V0-013`'s presence guard: `sh script/check-sql-native-ratchets_test.sh`, exit 0 (proves the guard is false for the pinned, absent `causal_parent` and true for a real commit); `SNR-V0-012`'s fixture-copy path and the parent-side measurement itself remain unexercised by a real run, and the parent-separation comparison this spec's evidence never claims has been produced; real 2026-09-13 decision-0226 run (`core_base=469c2478`) enforced the current-tree ceiling (`causal_candidate_max_allocs_op=393`) and abstained on the parent |
| SNR-V0-014 | real 2026-09-13 decision-0226 run (`core_base=469c2478`): exit 0, one `sql-native-ratchets` line with `causal=NOT_RUN-causal-parent-unavailable` |
| SNR-V0-015 | inspection of `script/check-sql-native-ratchets.sh`'s trap and `mktemp -d` usage; no tracked-tree write observed |

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| SNR-V0-001 | `script/check-sql-native-ratchets.sh:16@c6e1dd0e` (lock `mkdir`) and `:21-23@682a66e3` (cleanup trap) | `sh script/check-sql-native-ratchets_test.sh`, exit 0, 2026-09-13 |
| SNR-V0-002 | `script/check-sql-native-ratchets.sh:26-30@97a0bf90` (`SQL_NATIVE_RATCHET_LOCK_TEST_HOLD` case block) | `sh script/check-sql-native-ratchets_test.sh`, exit 0, 2026-09-13 |
| SNR-V0-003, SNR-V0-004 | `check_toolchain` and the receipt build in `script/check-sql-native-ratchets.sh` | real 2026-09-13 post-shrink run passed both (see Verified current state) |
| SNR-V0-005 | `script/check-sql-native-ratchets.sh:36-37@9aea4cd3` (`source_bytes` `find`/`wc -c` pipeline and ceiling test) | `sh script/check-sql-native-ratchets_test.sh`, exit 0 (ceiling-boundary fixture), plus the real 2026-09-13 invocations: refusal at `source_bytes=65908`, then pass at `source_bytes=64153` after the `analyzer.go` shrink (see `../BUILD-LOG.md` 2026-09-13) |
| SNR-V0-006 | the `build` function and `binary_bytes` check in `script/check-sql-native-ratchets.sh` | real 2026-09-13 post-shrink run: `binary_bytes=2832594` |
| SNR-V0-007, SNR-V0-008 | the `core_base` export and the `core-a`/`core-b` build and `deps` comparison block in `script/check-sql-native-ratchets.sh` | `sh script/check-sql-native-ratchets_test.sh`, exit 0 (Core-base export fixture); real 2026-09-13 decision-0226 run (`core_base=469c2478`): `core_binary=identical core_deps=identical`; scratch candidate-wiring commit refused |
| SNR-V0-009 | the cross-compile `for target in ...` loop in `script/check-sql-native-ratchets.sh` | real 2026-09-13 decision-0226 run (`core_base=469c2478`) |
| SNR-V0-010 | the `TestSQLite351NestedWithDelimiterWorkRatchet` invocation in `script/check-sql-native-ratchets.sh` | `experimental/analyzers/sqlnative/analyzer_test.go`; real 2026-09-13 decision-0226 run (`core_base=469c2478`) |
| SNR-V0-011 | the `measure` calls against `BenchmarkAnalyzeCandidate` in `script/check-sql-native-ratchets.sh` | `experimental/analyzers/sqlnative/analyzer_test.go`; ceilings from `analyzer-candidate-profiles.md`; real 2026-09-13 decision-0226 run (`core_base=469c2478`) |
| SNR-V0-012 | the fixture copy and `measure` calls against `BenchmarkAnalyzeCausalSQL` in `script/check-sql-native-ratchets.sh` | `experimental/analyzers/sqlnative/causal_benchmark_test.go`; not yet exercised by a real run |
| SNR-V0-013 | the causal_parent presence guard, `script/check-sql-native-ratchets.sh:53-60@92746b02` (`git cat-file -e "$causal_parent^{commit}"`) | `sh script/check-sql-native-ratchets_test.sh`, exit 0, 2026-09-13, proving the guard is false for the pinned, absent `causal_parent` and true for a real commit; the parent-separation comparison itself has not been produced (`causal_parent` absent from the object database) and this spec never claims it has |
| SNR-V0-014 | the final `printf` in `script/check-sql-native-ratchets.sh` | the gate invocation's stdout; real 2026-09-13 decision-0226 run (`core_base=469c2478`) |
| SNR-V0-015 | the `cleanup` trap and `ratchet_root` scoping in `script/check-sql-native-ratchets.sh` | inspection |

## Rollout, rollback, and drift

The script already exists and is unchanged by this spec; this document only records its delivered
behavior and its lack of a caller. Rollback is deleting the script; nothing in `Makefile` or CI
references it, so no other target would need to change. Drift risk is the object stated in Verified
current state: `causal_parent` is not a reachable commit in this repository, so every run here
abstains on the `SNR-V0-013` parent separation. The Core base follows `HEAD` (decision 0226), so it
does not go stale as main moves; rolling back that decision restores the unpassable `core_parent`
pin. A second drift risk is documentation, not code:
`../agent-memory/fixes.md` still describes this script as running in `make gate`, which
`../BUILD-LOG.md` has already corrected; that backlog entry should be updated to match the next time
someone edits it, but doing so is out of scope for this spec.

## Unresolved

Whether this script should ever gain a caller (`make gate`, a dedicated `make` target, or CI) is not
decided here — nothing in the repository currently exercises it. The `causal_parent` pin
(`a7db6200685c1ea7d4afc1798fb376ccc724acd1`) is kept as provenance, not replaced: the analyzer was
provenance-copied from rejected checkpoint `929d2ea` (BUILD-LOG "SQLite analyzer shadow-critical
split/rebuild"), which is also absent, and no reachable commit before `01f66571` carries
`experimental/analyzers/sqlnative`, so no substitute exists (decision 0104). Whether the monotonic-tightening
discipline for `SNR-V0-005`, `SNR-V0-006`, and `SNR-V0-011`'s thresholds should become a mechanical
check (comparing against a previously recorded value) rather than a review-time convention is not
decided here. Whether `SNR-V0-013`'s causal comparison should also ratchet bytes/op, not only
allocs/op, is not decided here. No provenance beyond `ACP-009` was found for any of the four
threshold constants; if one exists elsewhere in `docs/`, it was not located by this spec's search.
