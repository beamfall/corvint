# Gate ledger V0

Owner: Russell Lewis
Date: 2026-09-21
Requirement prefix: `GL-V0`
Intent status: accepted (owner instruction 2026-09-21)
Delivery status: implemented
Authoritative inputs: `../../AGENTS.md` invariants 4 and 8 and its Verify block, the owner
instruction of 2026-09-21 that Corvint manage which gate work is already proven so parallel agents
stop repeating a full gate each, `go-only-cutover-v0.md` GOC-V0-010 for the receipt the gate
records, `affected-plan-v0.md` for the affected tier this ledger must not replace and for the
rules (a) to (d) whose index proves a resolved package's bound, and decision 0352 for the
per-package key (GL-V0-009).

## Agent digest
- Claim: `make gate` skips a step, or one resolved package, only when this user recorded a pass for byte-identical inputs, from any worktree, never narrowing a step.
- Status: accepted (owner instruction 2026-09-21; GL-V0-009 decision 0352) / implemented
- Exists: `tools/gate-ledger` (`run`, `go-test`, `plan`), the `ledger/STEP` targets in `Makefile`, `gate-affected-select -unresolved` and `-bounds`, `tools/gate-ledger/main_test.go`, the ledger-off branch of `script/gate-receipt_test.sh`.
- Blocked on: nothing. The unresolved packages (`cmd/corvint` among them) rerun on any tree change; narrowing them is the affected tier's job, not this ledger's. Resolved packages key per package across worktrees (GL-V0-009).
- Read next: Requirements; Non-goals; Trust boundary, limits, and failure modes; Unresolved.

## Human intent and scope

Several agents work on Corvint in parallel worktrees, and each one ran the whole of `make gate`
before every commit: 29 steps, of which `go-test` over 197 packages dominates. Almost all of that
work repeated a pass another worktree, or the same worktree a minute earlier, had already
established on identical content. Nothing recorded what had been proven, so nothing could refuse to
prove it again.

Measurable job: for one user on one host, run a gate step at most once per distinct content of the
inputs that step reads, across every worktree of the repository, and never let that skip weaken the
gate. A skipped step is one whose exact inputs a recorded pass already covers; every other step
runs unchanged.

The affected tier (`make gate-affected`, `affected-plan-v0.md`) answers a different question,
"which packages could this diff have changed?", and is gated by its own qualification before it can
stand in for a push gate. This ledger asks only "has this exact content passed this step before?"
and therefore needs no qualification: a hit is a replay of a recorded pass on the same bytes.

## Verified current state

At the base of this change, `Makefile` ran every step with `-count=1`, which discards Go's test
cache, so a rerun in the same worktree on the same content repeated every package. Measured on this
host, Go's test cache never hits across worktrees even with `-trimpath`, because its test log hashes
the absolute paths of files a test opens under the module root; cross-worktree deduplication must
therefore come from something keyed on content, not from Go's cache.
`gate-affected-select` indexes which packages read the tree without a literal bound (`runtime.Caller`,
`os.Getwd`, `..` to the root, git); 104 of 197 packages are unresolved by that index, and
`cmd/corvint` is one of them.

## Requirements

- `GL-V0-001`: A step MUST be skipped only when the ledger holds a pass whose key equals the key
  of the current run. The key MUST derive from the record schema, the step name, the ledger tool's
  own identity (its Go source digest and the pinned toolchain version), `GO_TEST_TIMEOUT`,
  `GOFLAGS`, and a digest of the step's declared inputs taken from the exact worktree content:
  tracked and untracked files with ignored files excluded, written as a Git tree through a private index so the
  repository index is never touched. Cached stat data or timestamps MUST NOT authorize content reuse.
  Tracked files remain included when ignored; ignored untracked files remain excluded.
  The key MUST NOT include the worktree path, branch, commit,
  host, user, or time, so that one recorded pass serves every worktree of this user. Every step's
  scope MUST include the gate's own tooling (`Makefile`, `go.mod`, `go.sum`, `script/`, `tools/`),
  so a change to how a step runs is a change to its inputs.
- `GL-V0-002`: Only a step that exits zero MUST be recorded, after it exits. The ledger MUST pass
  the step's exit status and output through unchanged, MUST NOT run a step's command differently
  on a miss than `make STEP` runs it, and MUST NOT record on the failure path.
- `GL-V0-003`: A step without a declared input scope MUST run and MUST NOT be recorded. A step
  declared `never` MUST always run; `go-archive-gate` is `never`, because GOC-V0-010 binds its
  witness to the HEAD the receipt names, which is not content the ledger keys on. A step declared
  `tree` MUST key on the whole worktree tree id.
- `GL-V0-004`: `ledger/go-test` MUST run every package `go test ./...` would run. Packages the
  affected-plan index lists as unresolved (`gate-affected-select -unresolved`) MUST run with
  `-count=1` under one record keyed on the whole tree id and the package list; every other package
  MUST run without `-count=1`, so Go's own test cache may answer it, with the same flags `make
  go-test` uses, under the per-package key of GL-V0-009. When the partition cannot be computed (no
  module path, `go list` or the index failing, a package outside the module), every package MUST
  run with `-count=1` under the tree key. `make go-test` and `make gate-affected` MUST keep
  running with `-count=1` unchanged.
- `GL-V0-005`: The ledger MUST refuse to digest, and therefore run without recording, any
  worktree whose content `git status` cannot see fully: a tracked file marked skip-worktree or
  assume-unchanged, an ignored `.go` file outside `.`-, `_`- and `testdata` directories that the
  build would still compile, or any failing git command. These are the same refusals GOC-V0-010
  makes for the receipt.
- `GL-V0-006`: Records MUST live in a per-user directory (`CORVINT_GATE_LEDGER_DIR`, else
  `$XDG_CACHE_HOME/corvint/gate-ledger`, else `~/.cache/corvint/gate-ledger`) that is a plain
  directory owned by the caller with no group or world permission; any other directory MUST make
  every step run without recording. The directory MUST hold at most 4096 records, oldest pruned.
  A record the tool cannot parse, or whose schema or key does not match, MUST count as absent.
  Records are private derived state: nothing in Corvint's product path, ranking, learning,
  evidence, or authority MAY read them (`AGENTS.md` invariant 4).
- `GL-V0-007`: `CORVINT_GATE_LEDGER=off` MUST make every `ledger/STEP` target exactly `make
  STEP`, and `make STEP` on its own MUST stay unchanged. `gate-ledger plan STEP...` MUST print one
  HIT or RUN line per step and MUST NOT execute or record anything. The `gate` target MUST keep
  GOC-V0-010's wiring: `gate-receipt-clear` first, `script/gate-receipt record` last, and no step
  starting before the clear, including under `make -j`.
- `GL-V0-008`: Two runs of one key at once MUST execute the step once: the second MUST wait on a
  per-key lock and read the record the first wrote. A lock the tool cannot take MUST make the
  step run without recording.
- `GL-V0-009`: A resolved package MUST run under its own key, step `go-test-package`, derived as
  GL-V0-001 derives every key from the package's import path and a digest of the worktree entries
  in its proven bound together with the gate tooling. The bound MUST be the union of every file
  `go list -deps -test` compiles or embeds into the package's test binary within the module and
  every worktree path whose change would select the package under `affected-plan-v0.md` rules (a)
  to (c), taken from the same index and relation `make gate-affected` uses
  (`gate-affected-select -bounds`); by construction of rule (d) that union covers every path a
  resolved package's tests can read. The record MUST state how the bound was proven: the `go
  list` file count, the path count per selector rule, and the number of entries digested. A
  package whose bound cannot be proven, because the selector attributes no path to it, `go list`
  does not list it, or `go list` names a file the worktree digest does not hold, MUST run in the
  same `go test` invocation and MUST NOT be recorded; when the bounds cannot be computed at all,
  every resolved package MUST run through Go's test cache unrecorded, as before this requirement.
  Resolved packages without a recorded pass MUST run in one `go test` invocation with the flags
  `make go-test` uses, each under its per-key lock (GL-V0-008), and each MUST be recorded only
  when that invocation exits zero. The record format MUST stay additive: `bound` is an optional
  field, and records without it keep matching.

## Non-goals and simpler baseline

The simpler baseline is the state before this change: every gate run repeats every step, and Go's
test cache is discarded by `-count=1`. It is correct and slow, and it is still what CI runs.

This ledger never narrows a step: a miss runs the same command `make STEP` runs, over the same
inputs. It is not the affected tier and does not replace it; the only principled way to run fewer
packages for a tree change is `affected-plan-v0.md`, with its own qualification. It does not share
records between users or hosts, does not run in CI, does not key unresolved packages per package
(their reads have no proven bound; see Unresolved), and does not change what the GOC-V0-010 receipt means: a
receipt still says every step passed for the HEAD the run started at, whether a step's pass was
executed or replayed from a record keyed on identical content.

## Trust boundary, limits, and failure modes

The tool reads the worktree through git, writes a temporary private index and the per-user record
directory, and spawns the step's command. It reads no record as an instruction: a record is a key,
a tree id, a time, a host and a package list, and only key equality matters.

| Failure | Behavior |
|---|---|
| No recorded pass for the key | run, record on exit zero (GL-V0-001, GL-V0-002) |
| The step exits non-zero | pass the status through, no record (GL-V0-002) |
| The step has no declared scope, or is `never` | run, no record (GL-V0-003) |
| skip-worktree or assume-unchanged entry, ignored compiled `.go`, git failure | run, no record (GL-V0-005) |
| Ledger directory absent, shared, symlinked, or another user's | run, no record (GL-V0-006) |
| Record unreadable or mismatched | treated as absent (GL-V0-006) |
| Partition unavailable for `go-test` | every package with `-count=1` under the tree key (GL-V0-004) |
| Bounds unavailable for `go-test` (selector, `go list`, module path or digest failing) | resolved packages through Go's test cache, no record (GL-V0-009) |
| A resolved package's bound cannot be proven | runs in the resolved batch, no record (GL-V0-009) |
| One package of the resolved batch fails | status through, no package of the batch recorded (GL-V0-002, GL-V0-009) |
| Per-key lock unavailable | run, no record (GL-V0-008) |
| `CORVINT_GATE_LEDGER=off` | every step runs as `make STEP`, nothing printed by the ledger (GL-V0-007) |

Limits. The ledger costs one worktree digest per step, about half a second on this repository, so
a full gate whose every step hits still spends roughly twenty seconds in digests and `go run`
start-up; that is the price of computing each key from content rather than trusting a cached
plan. The unresolved packages key on the whole tree, so any tree change reruns all of them; with
`cmd/corvint` unresolved, that is most of the suite's wall time. A step whose scope is declared
too narrowly would hit when it should run; every declared scope therefore errs wide (`tree` where
a script reads paths the declaration cannot enumerate), and the scope table is inputs to every
key, so widening a scope invalidates its records. The per-package bounds cost one selector index
and one `go list -deps -test` per `ledger/go-test`, about three and a half seconds on this
repository (`../BUILD-LOG.md`, 2026-09-22). A bound errs wide the same way a scope does: rule (c)
attributes every path a package's string literals name, so a package holding a broad path token
(`docs/`, `internal/`) is keyed on hundreds of paths and reruns more often than its tests need,
never less. A package at the module root encloses every path under rule (b) and therefore reruns
on any change. Tests that read the tree through git or the environment rather than a literal are
unresolved by rule (d) and stay on the tree key; a resolved package whose test reads a path no
literal names is outside rule (d)'s promise, not a gap this ledger can detect.

## Acceptance criteria and testing matrix

`tools/gate-ledger/main_test.go` runs the tool inside a fixture repository with a private ledger
directory; `tools/gate-affected-select/main_test.go` covers the `-unresolved` listing;
`script/gate-receipt_test.sh` covers the gate wiring with the ledger off.

| Requirement | Evidence |
|---|---|
| GL-V0-001 | `TestRunStepSkipsOnlyRecordedIdenticalInputs`: a rerun on identical content hits, a change outside the scope still hits, a change inside the scope runs; `TestRunStepRecordsFromLinkedWorktree`: a pass recorded in a `git worktree add` checkout hits from the main worktree; `TestWorktreeDigestIgnoresCachedStat`: same-size restored-time edits miss even with a newer index; `TestWorktreeDigestPreservesMembershipAndPaths`: tracked/ignored/staged/intent-to-add membership, executable/symlink modes and NUL-safe paths; `TestWorktreeDigestWithoutIndex`: unborn worktree |
| GL-V0-002 | the same test: a step exiting 3 returns 3 and records nothing |
| GL-V0-003 | the same test: an undeclared step runs and records nothing; `plan` reports `go-archive-gate: always runs` |
| GL-V0-004 | `TestGoTestFallsBackToOneUncachedRun`: the fallback passes `-count=1 ./...` and hits on an identical tree; `TestUnresolvedPackagesListsRootLocators`: the partition's input; measured partition on this repository in `../BUILD-LOG.md` |
| GL-V0-005 | `TestRunStepRefusesWhatItCannotDigest`: skip-worktree, assume-unchanged and an ignored `build/build.go` run and record nothing; `TestWorktreeDigestImportFailureRunsWithoutRecord`: private import failure runs without recording and removes the private index/lock |
| GL-V0-006 | the same test: a `0755` ledger directory runs and records nothing |
| GL-V0-007 | the same test: `CORVINT_GATE_LEDGER=off` runs silently; `TestRunStepSkipsOnlyRecordedIdenticalInputs`: `plan` leaves the run count unchanged; `script/gate-receipt_test.sh` ordering probe through the `ledger/` targets |
| GL-V0-008 | `lock` in `tools/gate-ledger/main.go`, flock per key with a second lookup after acquisition; inspection |
| GL-V0-009 | `TestGoTestKeysResolvedPackagesPerPackage`: three resolved packages record with a `bound` proof, hit from a `git worktree add` checkout, an edit to a package's source reruns it and its dependent while a reader package hits, an edit to a path a test literal names reruns only that reader, an edit outside every nested bound hits; `TestPackageBoundsAttributesResolvedPackages`: the `-bounds` listing per rule and the unresolved reason; `TestPathMatcherAgreesWithNamesPath`: the batched rule (c) matcher equals `namesPath`; measured cross-worktree hit in `../BUILD-LOG.md` (2026-09-22) |

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| GL-V0-001 | `key`, `inputs`, `worktreeDigest`, `tooling`, `scopes` in `tools/gate-ledger/main.go` | `TestRunStepSkipsOnlyRecordedIdenticalInputs`; `TestRunStepRecordsFromLinkedWorktree` |
| GL-V0-002 | `runStep`, `execute` | `TestRunStepSkipsOnlyRecordedIdenticalInputs` |
| GL-V0-003 | `scopes`, `key` | `TestRunStepSkipsOnlyRecordedIdenticalInputs` |
| GL-V0-004 | `goTest`, `partition`; `unresolvedPackages` in `tools/gate-affected-select/main.go`; `GO_TEST_FLAGS` and `ledger/go-test` in `Makefile` | `TestGoTestFallsBackToOneUncachedRun`; `TestUnresolvedPackagesListsRootLocators` |
| GL-V0-005 | `worktreeDigest`, `hasFlaggedEntry`, `compiledIgnored` | `TestRunStepRefusesWhatItCannotDigest` |
| GL-V0-006 | `ledgerDirectory`, `lookup`, `record`, `prune`; `ownedByInvokingUser` in `tools/gate-ledger/platform_unix.go` (`platform_other.go` refuses the directory on non-Unix hosts) | `TestRunStepRefusesWhatItCannotDigest` |
| GL-V0-007 | `open`, `plan`; the `ledger/%` targets and their `off` branch in `Makefile` | `TestRunStepRefusesWhatItCannotDigest`; `script/gate-receipt_test.sh` |
| GL-V0-008 | `lock`; `lockExclusive` in `tools/gate-ledger/platform_unix.go` (`platform_other.go` fails the lock on non-Unix hosts) | inspection |
| GL-V0-009 | `packageKeys`, `packageKey`, `selectorBounds`, `listedFiles`, `runPackages`, `digestKey` in `tools/gate-ledger/main.go`; `packageBounds`, `attribute`, `attributeStructure` in `tools/gate-affected-select/main.go` and `pathMatcher` in `tools/gate-affected-select/readers.go` | `TestGoTestKeysResolvedPackagesPerPackage`; `TestPackageBoundsAttributesResolvedPackages`; `TestPathMatcherAgreesWithNamesPath` |

## Rollout, rollback, and drift

The ledger is on from the commit that introduces it, and the first gate on any tree records rather
than hits. Rollback is `CORVINT_GATE_LEDGER=off` for one run, or removing the `ledger/` targets
and restoring `gate: gate-receipt-clear $(GATE_STEPS)`; records are derived state and can be
deleted at any time.

Drift rules. A new `GATE_STEPS` member without a `scopes` entry runs and is never recorded
(GL-V0-003), so forgetting the table costs time, never correctness; adding the entry is the
follow-up. A step that starts reading a path its scope does not name must widen its scope in the
same change. The tool identity in every key includes the tool's own source, so a change to the
scope table or the key derivation invalidates every record.

## Unresolved

- `cmd/corvint` is unresolved because of one `os.Getwd` in its dogfood recording; bounding that
  read would move the largest package under a per-package key (GL-V0-009) and let it hit across
  worktrees.
- A resolved batch is recorded package by package but run as one `go test` invocation, so one
  failing package leaves every package of the batch unrecorded; recording the packages `go test`
  reported as `ok` would need the tool to parse test output, which it does not do.
- The receipt records a pass whose steps may have been replayed; whether the receipt should name
  the records it replayed from is not decided here.
