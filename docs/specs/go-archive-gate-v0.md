# Go archive gate V0

Owner: Russell Lewis
Date: 2026-09-12
Requirement prefix: `GAG-V0`
Intent status: accepted (decision 0105 amendment 2026-09-12)
Delivery status: implemented
Authoritative inputs: `../../AGENTS.md` invariant 8, `../SPEC-DRIVEN-DEVELOPMENT.md` "Required
capability-spec shape", `release-artifact-integrity-v0.md` and `go-production-kernel-migration-v0.md`
(which own the artifact contract this gate invokes), `optional-artifact-check-scripts-v0.md`
(which explicitly disclaims this script), and `../agent-memory/fixes.md` (which lists the gate
scripts still without an owning spec).

## Agent digest
- Claim: Every `make gate` run rebuilds the five release archives from the committed revision hermetically and proves the result is the expected closed set.
- Status: accepted (decision 0105 amendment 2026-09-12) / implemented
- Exists: `script/go-archive-gate` and `script/go-archive-gate_test.sh`, `make` targets `go-archive-gate` and `go-archive-gate-test`, both wired into `make gate`; `script/go-archive-gate-injection_test.sh` (`make go-archive-gate-injection-test`, on demand, not in `make gate`) injects negative outputs into real producer output.
- Blocked on: no implementation change. The real gate requires an exact local `go1.27.0`; a bare `GOTOOLCHAIN=local make go-archive-gate` fails whenever the host's `PATH` resolves `go` to a different point release (observed on 2026-09-12 with `go1.27.1`), but the pinned version was already installed alongside it and re-ran to `PASS` once `PATH` was pointed at it first — see README.md's toolchain-selection note. Real-command negative artifact injection ran on 2026-09-13 (`script/go-archive-gate-injection_test.sh`, exit 0): extra, missing and symlinked outputs of the real producer and a real-build interrupt each failed or cleaned up as specified. Archive content corruption is not this gate's check (non-goal); it belongs to `release-artifact-integrity-v0.md`.
- Read next: Requirements; Trust boundary, limits, and failure modes.

### Original status-marker classification (2026-09-12)

The 11 original literal occurrences classify as: category (a), one (`GAG-V0-008`'s failing-gate
execution, observed by the real command below); category (b), five (the GAG-V0-001, 002, 004 and
005 wrapper-execution rows, including GAG-V0-004's duplicate acceptance/traceability disclosure),
now covered by three focused cases added to `script/go-archive-gate_test.sh`; category (c), five
(the two aggregate real-gate negative disclosures, the release-checklist status literal,
GAG-V0-005's impossible successful dirty-worktree case, and GAG-V0-006's real-producer fault
injection). Category (c) is retained only where the exact unavailable evidence and reason are
stated below. On 2026-09-13 GAG-V0-006's real-producer fault injection moved to category (a):
`script/go-archive-gate-injection_test.sh` executes it.

## Human intent and scope

The release archives are the artifact a user installs. `release-artifact-integrity-v0.md` owns what
a correct archive set is; this script is the thing that makes producing one a precondition of every
`make gate` run rather than a release-day discovery. It exists because an archive build breaks
silently: nothing in the Go test suite builds the cross-target archives, so a change that breaks
`GOOS=windows` packaging or adds a stray file to the output directory is invisible until a release.

The script's distinguishing job is hermeticity. It builds in a private temporary environment with
`HOME`, `TMPDIR`, `GOTMPDIR`, and `GOCACHE` redirected and the module proxy, checksum service, Go
environment file, and toolchain switching all disabled, so a passing gate is evidence about the
committed sources and the pinned local toolchain rather than about the developer's caches or
network.

Affected user: an agent or engineer changing anything the archive build reads. Measurable job: for
the current `HEAD` commit, rebuild the archive set offline into a private directory, fail unless
the output is exactly the seven expected top-level regular files, and fail unless the private
witness the build recorded reports `PASS` for that exact revision and tree.

The owner instruction of 2026-09-07 places repository gate tooling in scope for AGENTS.md
invariant 8. This spec discharges that for the archive gate wrapper only; the archive semantics
themselves remain owned by `release-artifact-integrity-v0.md`.

## Verified current state

At `5b496119` the script was wired into `make gate` through the `go-archive-gate` target (historical
`Makefile@5b496119`). The current target is `Makefile:99@df81c156`; it runs
`go run ./conformance/release-artifact-v0 archive --revision HEAD` (`script/go-archive-gate:36`)
followed by an `archive-status` cross-check against a witness under the repository's Git directory.

The gate was first executed at `f9dc568b`, the commit that closed GAG-V0-007: `make go-archive-gate`
exited zero in 4m02s, printing `SUMMARY revision=f9dc568b08a7202539bcb700e9bab56952f5a068 targets=5
verdict=PASS`, and the run recorded its own 853-byte witness, which its `archive-status` cross-check
then accepted. That covers the passing path of GAG-V0-001 through GAG-V0-008 on one host; the
negative cases below — a missing artifact, an extra one, a symlink, an interrupted run — were then
`NOT_RUN` for the real gate because its archive producer has no test-only seam for substituting or
mutating the private output before the wrapper checks it. They ran on 2026-09-13 at
`2ea60b4527b251a6c522d835a91c8e8939b6a121` without adding such a seam:
`PATH=/opt/homebrew/Cellar/go/1.27.0/bin:$PATH ./script/go-archive-gate-injection_test.sh` exited 0
in 245s. It clones `HEAD` into a temporary directory and runs the unchanged gate under `bash -x`
with a `go` shim first on `PATH` that forwards every Go command to the real toolchain. The real
`archive` command built the five targets once (`SUMMARY ... targets=5 verdict=PASS`, gate exit 0);
the shim kept that output and the witness it recorded and replayed them into later runs, where an
unmodified replay also passed. Injected after production and before the wrapper checks, an extra
regular file, directory, or symlink each exited 1 at `test 8 = 7` (all-entry count), a
`linux_arm64` archive replaced by a symlink to its real bytes exited 1 at `test 6 = 7`
(regular-file count), and a removed `windows_amd64` archive exited 1 at the `test -f` presence
check; none reached `archive-status`. A SIGINT to the gate's process group while the real `archive`
command had a child build running exited 1, left no private parent, and left no process in the
group. No case changed the clone's worktree or the source checkout's witness. The replay reuses one
real build, so the negative cases observe the wrapper checks on real bytes, not a second producer
run per case.

At `76a5328222659de3ace732f1ea8bb782937e4018`,
`GOTOOLCHAIN=local make go-archive-gate` was attempted on 2026-09-12 and exited 2 after 9 seconds:
the selected local toolchain was `go1.27.1`, while the frozen manifest requires `go1.27.0`.
That run is `FAIL`, not passing archive evidence; its non-zero propagation is observed evidence for
GAG-V0-008's failure path. The exact `go1.27.0` toolchain was already installed on the same host
(Homebrew Cellar, alongside the linked `go1.27.1`), so the cause was `PATH` order, not a missing
toolchain; rerunning the same command with that Cellar directory prepended to `PATH` at
`c937bc669f47d9df1feb8cd0e2bcd43d53fc2201` on 2026-09-12 —
`PATH=/opt/homebrew/Cellar/go/1.27.0/bin:$PATH GOTOOLCHAIN=local make go-archive-gate` — exited 0
in 291s, printing `SUMMARY revision=c937bc669f47d9df1feb8cd0e2bcd43d53fc2201 targets=5 verdict=PASS`.
README.md now documents selecting the pinned toolchain by `PATH` without relinking Homebrew.

For decision 0105 the gate was rerun at `3c41cad3`: `make go-archive-gate` exited zero in 5m10s and
printed `SUMMARY revision=3c41cad31bb2daf13ffeb3a36947a9eee7a9b649 targets=5 verdict=PASS`. The
wrapper's negative paths were then exercised once with `go` stubbed on `PATH`, as a scratch check that
is not committed: a missing archive, an extra regular file, an expected name as a symlink, a failing
build (exit 3 preserved), and a wrong-revision witness each failed; a read-only directory under the
cache and a `TERM` during the build (exit 143) each left no private parent; no case changed the
fixture worktree. An extra top-level directory and an extra symlink both exited zero, which
`GAG-V0-006` forbids. The gate now counts every top-level entry as well as regular files, and
`script/go-archive-gate_test.sh` case 3 requires both extras to fail.

On the fresh-clone question that `decision-number-gate-v0.md` pins: this script does not have the
worktree-enumeration defect. It passes `--revision HEAD` and derives `HEAD^{commit}` and
`HEAD^{tree}` from Git, so it builds and verifies committed content. An uncommitted change does not
pass silently either: the delegated `archive` command refuses unless `git status --porcelain
--untracked-files=all` is empty (`conformance/release-artifact-v0/archive_run.go:422`), so any
staged, unstaged, or untracked change fails this gate until it is committed or removed.

GAG-V0-007 requires that a witness the current run did not record cannot satisfy the status check.
The `archive` subcommand writes the witness best-effort: on failure it prints
`release-artifact-v0 archive: private witness not recorded` to stderr and leaves the verdict
unchanged (`conformance/release-artifact-v0/archive_cli.go:29`), and the archive gate test asserts
that a witness failure does not relabel the verdict
(`conformance/release-artifact-v0/archive_gate_test.go:206`). The witness is persistent private
state under the Git directory, so a failed write used to leave a witness for the same revision and
tree from an earlier run for `archive-status` to read, and the script reported `PASS` on evidence
the current run did not produce. The wrapper now removes any pre-existing witness before the build
(`script/go-archive-gate:35`), so the status check can only read a witness this run recorded and
an unrecorded one fails the check instead. The command-side alternative, failing `archive` when
its witness write fails, was not taken: it changes the release command that
`release-artifact-integrity-v0.md` owns, while the wrapper fix is local to this gate.

## Requirements

- `GAG-V0-001`: The gate MUST create one private parent directory under `${TMPDIR:-/tmp}` with a
  randomised name, restrict it to owner access only, and create exactly four subdirectories inside
  it for the home, temporary, Go temporary, and Go cache roots, each equally restricted. No build
  output, cache, or temporary file may be written outside that parent.
- `GAG-V0-002`: The gate MUST export `HOME`, `TMPDIR`, `GOTMPDIR`, and `GOCACHE` to those private
  subdirectories and MUST disable the Go environment file, module proxy, checksum service, and
  toolchain switching, and require read-only module resolution, before invoking any Go command, so
  that the build neither reads the caller's Go configuration and caches nor reaches the network.
- `GAG-V0-003`: The gate MUST pin the toolchain to the locally installed one rather than allowing
  Go to download a different version, so the archive is built by the version `go-version` asserts.
- `GAG-V0-004`: The gate MUST install a cleanup handler for normal exit and for hangup, interrupt,
  and terminate, which restores owner write permission across the parent before removing it, so a
  build that leaves read-only directories still cleans up. Cleanup MUST NOT change the gate's exit
  status. The hangup, interrupt and terminate handlers MUST NOT return into the script: they exit
  129, 130 and 143 and the normal-exit handler cleans up, so a gate signalled while its delegated
  command still succeeds never reaches a later check or a passing exit (decision 0248).
- `GAG-V0-005`: The gate MUST build the archive set from the committed revision `HEAD`, not from
  the worktree, into a directory inside its private parent, and MUST fail when that build fails.
- `GAG-V0-006`: The gate MUST require the output directory to contain exactly the seven expected
  top-level entries: a checksum file, a verification report, and five archives covering darwin
  amd64 and arm64, linux amd64 and arm64, and windows amd64. Each MUST be present as a regular
  file, and both the count of top-level entries of any type and the count of top-level regular
  files MUST equal seven, so that a missing artifact and an unexpected extra artifact, including a
  directory or symlink, all fail. The check MUST be a closed set, not a subset test.
- `GAG-V0-007`: The gate MUST cross-check the build against the private witness the build recorded,
  requiring a `PASS` verdict bound to the exact `HEAD` commit and tree it resolved. A witness the
  current run did not successfully record MUST NOT be able to satisfy this check, even when a
  witness for the same revision and tree survives from an earlier run.
- `GAG-V0-008`: The gate MUST exit non-zero when any of its build, closed-set, or witness checks
  fail, and MUST exit zero only when all of them pass, so that `make gate`, which runs this check
  through the `go-archive-gate` target, fails with it. It MUST abort on the first failing command
  rather than continuing, and MUST write nothing into the working tree.

## Non-goals and simpler baseline

The simpler baseline is building the archives only at release time, which is what this gate exists
to replace, and a weaker variant is asserting only that the expected files exist, which admits a
stray extra artifact; GAG-V0-006 closes the set for that reason. The gate does not verify archive
contents, signatures, or cross-builder reproducibility, does not compare against a previously
published release, does not execute the built binaries, and does not check any target this
repository does not ship. Byte-level reproducibility of a loose binary is a separate, optional
check owned by `optional-artifact-check-scripts-v0.md`; that script's execution is not evidence
for this one, and this one's execution is not evidence for it.

## Trust boundary, limits, and failure modes

The committed tree at `HEAD`, the local Go toolchain, Git, and the shell and its utilities are
trusted inputs; this gate is a build-integrity check, not a sandbox against a hostile toolchain or
a malicious `conformance/release-artifact-v0`. Disabling the module proxy and checksum service is
not an operating-system network sandbox, and a dependency already vendored or already present in
the repository is resolved without any network refusal being exercised.

On enumeration: this gate reads Git, not a directory listing, for the revision under test, so it
does not carry the worktree-glob defect. Its closed-set check is a directory listing, but of the
build's own private output directory, which is the correct scope for it. Because `-type f` does not
follow symlinks while `test -f` does, an expected name materialising as a symlink to a regular file
satisfies the presence check but is not counted by the closed-set count, so the counts disagree
and the gate fails; this is the safe direction.

The private parent is created before the cleanup handler is installed, so a failure between those
two points leaks the directory. `GOTOOLCHAIN=local` is set for the build but the script does not
itself assert the resulting version; `go-version` is the separate gate member that does, and this
script's evidence depends on that target also running.

The witness is state under the Git directory that outlives the run, which is why the gate removes
any pre-existing one before building. It is derived private state, never an input to ranking,
evidence, or authority, and a removed witness costs nothing a rerun cannot rebuild: the only other
reader, `script/release-checklist:47-48@74e4657d`, reports `NOT_RUN` when it is absent. That word is the
checklist's prescribed status token, not an unexecuted-evidence marker.

Both the gate and the checklist resolve the witness path from `git rev-parse --absolute-git-dir`,
which names the calling checkout's own administrative directory. For a linked worktree that is the
per-worktree directory under `<common-dir>/worktrees/<name>`, not the repository's common Git
directory, so a witness recorded by `make go-archive-gate` in one worktree or clone is invisible to
`script/release-checklist` run from another worktree or clone, even at an identical or descendant
revision; that case is indistinguishable from the "fresh clone without the private witness" failure
mode `release-artifact-integrity-v0.md` already documents for `ARTIFACT-RDY-V0-001..009`. There is
no portable receipt to copy between checkouts. To make the checklist observe a real `PASS`, run
`PATH=/opt/homebrew/Cellar/go/1.27.0/bin:$PATH GOTOOLCHAIN=local make go-archive-gate` (or
`script/go-archive-gate` directly) in the exact same checkout `script/release-checklist` will be run
from, with a clean working tree, once the candidate commit is final; a commit made after recording
moves `HEAD` and the tag/publication rows have their own revision-binding rules, but note this
gate's own witness immediately reports `STALE`/`NOT_RUN` again for the new `HEAD` (GAG-V0-007). Then
run `script/release-checklist` from the same checkout before it, or any later commit, ages the
witness out. Confirmed on 2026-09-12 at `56913d3480011b86936074ff9f117e9e601d23c5`: the checklist
reported `NOT_RUN go-archive ARTIFACT-GO-V0-001..007 private archive verification witness is
absent` in a worktree that had never run the gate, even though `make go-archive-gate` had separately
`PASS`ed at ancestor revision `c937bc669f47d9df1feb8cd0e2bcd43d53fc2201` in a different checkout;
running `PATH=/opt/homebrew/Cellar/go/1.27.0/bin:$PATH GOTOOLCHAIN=local make go-archive-gate` in
the same worktree as the checklist (exit 0, ~250s, `SUMMARY revision=56913d3480011b86936074ff9f117e9e601d23c5
targets=5 verdict=PASS`) made the subsequent `script/release-checklist` run report
`PASS go-archive ARTIFACT-GO-V0-001..007 current-revision archive build and verification pass`.

| Failure | Behavior |
|---|---|
| The archive build fails for any target | fail, with the build's own diagnostics |
| An expected artifact is missing from the output | fail (exit 1) at the `test -f` presence check for that path, which prints no diagnostic of its own |
| An unexpected extra top-level regular file is present | fail at the closed-set count, although every expected name is present |
| An unexpected extra top-level directory or symlink is present | fail at the all-entry count, although every expected name is present as a regular file |
| An expected artifact is a symlink rather than a regular file | fail, because the count excludes it |
| The witness reports a verdict other than `PASS`, or names a different revision or tree | fail at the status comparison |
| The witness write failed, including when a same-revision witness survived an earlier run | fail: the pre-existing witness was removed before the build, so the status check finds none |
| The run is interrupted | the private parent is removed by the cleanup handler and the gate exits non-zero, 128 plus the signal number when the gate itself is signalled; no working-tree change exists to undo |

## Acceptance criteria and testing matrix

`script/go-archive-gate_test.sh`, the `go-archive-gate-test` target, covers private-root creation and
mode, the offline environment, the `HEAD` and private-output arguments, normal/failure/interrupt
cleanup, build-failure status preservation, GAG-V0-007 (unrecorded, non-`PASS` and other-revision witnesses), and
cases of GAG-V0-006 on a fixture repository with the archive build and status command stubbed on
`PATH`. `GOTOOLCHAIN=local ./script/go-archive-gate_test.sh` passed on 2026-09-12 (exit 0). The
passing path of the real gate was measured once at `f9dc568b`. `script/go-archive-gate-injection_test.sh`
(`make go-archive-gate-injection-test`) covers GAG-V0-006's negative cases and GAG-V0-004's interrupt
against real producer output; it needs the exact `go1.27.0` first on `PATH`, performs one real
five-target build, and is not a `make gate` prerequisite because `go-archive-gate` already performs
one real build per run. The delegated archive command
has Go tests; those cover the command, not this wrapper.

| Requirement | Evidence |
|---|---|
| GAG-V0-001, GAG-V0-002, GAG-V0-003 | the private directory creation, permission changes, and exported environment in `script/go-archive-gate`; one hermetic build executed offline at `f9dc568b` |
| GAG-V0-004 | PASS on 2026-09-12: `GOTOOLCHAIN=local ./script/go-archive-gate_test.sh` exited 0 after proving normal, exit-23 failure, and `TERM` cleanup remove the observed private parent, including a read-only child; PASS on 2026-09-13 against the real producer: `script/go-archive-gate-injection_test.sh` case 4 sent SIGINT to the gate's process group during the real build (gate exit 1, no private parent, no surviving process) |
| GAG-V0-005 | the wrapper's `--revision HEAD` and private `--output` arguments PASS in `GOTOOLCHAIN=local ./script/go-archive-gate_test.sh` on 2026-09-12 (exit 0). A successful worktree-edit-invisibility run is category (c): the delegated real command intentionally refuses a dirty worktree before building, so only clean committed-revision builds can succeed |
| GAG-V0-006 | met: the expected-name list, the per-name presence tests, and the all-entry and regular-file counts at `script/go-archive-gate:53-54`; `GOTOOLCHAIN=local ./script/go-archive-gate_test.sh` passed its extra-directory and extra-symlink cases on 2026-09-12 (exit 0). PASS on 2026-09-13 at `2ea60b45` against real producer output: `PATH=/opt/homebrew/Cellar/go/1.27.0/bin:$PATH ./script/go-archive-gate-injection_test.sh` exited 0; extra regular file, directory and symlink, a symlinked archive, and a missing archive each exited 1 at the expected count or presence check, after an unmodified replay passed |
| GAG-V0-007 | met: `script/go-archive-gate_test.sh` runs the gate twice on a fixture repository, once recording a witness and once with the write failing while that witness survives, and requires the second run to exit non-zero; the best-effort write it defends against is `conformance/release-artifact-v0/archive_cli.go:29` |
| GAG-V0-008 | PASS on 2026-09-12: `PATH=/opt/homebrew/Cellar/go/1.27.0/bin:$PATH GOTOOLCHAIN=local make go-archive-gate` exited 0 in 291s at `c937bc669f47d9df1feb8cd0e2bcd43d53fc2201`, printing `SUMMARY ... verdict=PASS`; the same command without that `PATH` prefix exited 2 on the exact-toolchain mismatch, which is separately observed evidence for the non-zero propagation half of this requirement; `GOTOOLCHAIN=local ./script/go-archive-gate_test.sh` exited 0 while checking build exit 23 is preserved |

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| GAG-V0-001 | the `mktemp -d` parent, its `chmod 0700`, and the four-subdirectory loop in `script/go-archive-gate` | PASS: `GOTOOLCHAIN=local ./script/go-archive-gate_test.sh` (2026-09-12, exit 0) observed one parent and four private mode-0700 roots |
| GAG-V0-002 | the `HOME`, `TMPDIR`, `GOTMPDIR`, `GOCACHE`, `GOENV`, `GOFLAGS`, `GOPROXY`, and `GOSUMDB` exports | PASS: `GOTOOLCHAIN=local ./script/go-archive-gate_test.sh` (2026-09-12, exit 0) observed the redirected roots and exact offline settings in the delegated command |
| GAG-V0-003 | the `GOTOOLCHAIN=local` export, paired with the separate `go-version` target | `script/go-archive-gate_test.sh` case 1 requires `GOTOOLCHAIN=local` in the delegated command's environment; the resulting version is asserted only by `go-version` |
| GAG-V0-004 | the `cleanup` function and its `trap` in `script/go-archive-gate` | PASS: `GOTOOLCHAIN=local ./script/go-archive-gate_test.sh` (2026-09-12, exit 0), normal/failure/interrupt cleanup; case 6 (2026-09-13, exit 0; the returning trap let the gate exit 0) signals the gate alone during a passing status check and requires exit 143 with the parent removed; `script/go-archive-gate-injection_test.sh` case 4 (2026-09-13, exit 0), SIGINT during the real build |
| GAG-V0-005 | the `archive --revision HEAD --output` invocation | PASS: `GOTOOLCHAIN=local ./script/go-archive-gate_test.sh` (2026-09-12, exit 0), exact revision/private output arguments and exit-23 build failure |
| GAG-V0-006 | the `expected` array, the `test -f` loop, and the two `find`/`wc -l` count comparisons | `script/go-archive-gate_test.sh` case 3: an extra top-level directory, symlink or regular file, a missing artifact, and an expected name that is a symlink each fail; `script/go-archive-gate-injection_test.sh` case 3 repeats them on real producer output and names the failing check; `script/go-archive-gate-injection-interrupt_test.sh` proves an interrupted harness exits 130 and removes its temporary directory, so the harness's exit 0 is evidence only of a run that finished |
| GAG-V0-007 | the pre-build `rm -f "$witness"` at `script/go-archive-gate:35` and the `archive-status --witness --revision --tree` invocation with its `PASS` comparison; `conformance/release-artifact-v0/archive_witness.go` writes the witness | `script/go-archive-gate_test.sh` cases 2 and 3b: an unrecorded witness, a recorded non-`PASS` verdict, and a witness bound to another revision each fail |
| GAG-V0-008 | the `Makefile` `go-archive-gate` target and its listing in the `gate` target | `make gate` membership; PASS at `c937bc669f47d9df1feb8cd0e2bcd43d53fc2201` on 2026-09-12 (exit 0, 291s) with the pinned toolchain first on `PATH` |

## Rollout, rollback, and drift

This document is additive ownership over an existing gate member; it changes no behavior. Rollback
removes the document and its index locators and alters no script. Dropping the gate itself would
mean removing the `go-archive-gate` prerequisite from `make gate`, which returns archive breakage
to release-day discovery. Rolling back the real-producer injection evidence removes
`script/go-archive-gate-injection_test.sh` and its `make` target, changes no gate behavior, and
returns GAG-V0-006's real-producer negative cases to `NOT_RUN`.

Drift rule: the expected-artifact list is a closed set and MUST stay one. A new shipped target
MUST be added to the list in the same change that ships it, and an artifact MUST NOT be admitted
by relaxing the count comparison into a subset test, because the count is what catches an
unintended extra file. The revision under test MUST stay a Git revision and MUST NOT become a
worktree build, for the fresh-clone reason `decision-number-gate-v0.md` pins.

## Unresolved

The GAG-V0-007 repair was taken in the wrapper, which removes any pre-existing witness before the
build. Whether `archive` should additionally fail when its own witness write fails, protecting
callers that do not go through this wrapper, is still open and belongs to
`release-artifact-integrity-v0.md`, which owns that command. Whether this gate should additionally
assert the toolchain version itself, rather than depending on `go-version` also running, is
likewise open.
