# V1-4733: bounded concurrent archive targets

Recorded 2026-11-06; the filename follows the lane charter's requested evidence path.
Base: `9db7a94169821d9b28701eaa88866aee2fec1083`. Requirements:
`GAG-V0-9960` to `GAG-V0-9963` (standing approval, V1-4733).

The loose and archive phases now schedule independent targets in a bounded worker pool.
The default is the smaller of `runtime.NumCPU()` and the five targets; the optional positive
`CORVINT_ARCHIVE_WORKERS` override controls scheduling only. Workers own their findings and
collect in manifest order. A terminal error cancels higher indices, lets earlier ones finish,
and joins all workers before reporting the serial prefix. Loose report-level findings still
collect across every target, matching the original loop. Unix loose builds and smoke processes
now use the archive runner's existing process-group containment; speculative suffix builds are
removed after terminal errors. Unsupported hosts retain the previous loose-runner behavior;
the archive path still refuses hosts without qualified containment.

Design decision: do not reuse loose binaries. `ARTIFACT-GO-V0-001/003/005/007` and the existing
release-integrity traceability require live-root loose builds to be bound to separate raw-commit
builds. `TestIgnoredLiveSourcePassesLooseGateButArchiveBindingRefuses` demonstrates why identical
commit/tree/toolchain/manifest/flags do not prove identical live source: ignored `.go` files can
change the loose binary. All twenty cold target builds, independent caches, indexes, temporary
paths, assemblies, offline verification, clean-state rechecks and atomic promotion remain.
Source `.git` metadata is prepared before concurrent readers and is not rewritten during builds.
No JSON, wire schema, report field, error code, CLI flag, committed golden or CI file changes.

## Measurement

Same host: Linux amd64, Go 1.27.3, 32 online CPUs, approximately 31 GiB RAM. Commands use
`TMPDIR=/home/russell/lanes/archpar.tmp`; raw evidence is outside the repository under
`/home/russell/lanes/archpar-evidence/`. Each `make go-archive-gate` uses fresh private caches.

| Run | Revision | Wall seconds | User seconds | System seconds | Outcome |
| --- | --- | ---: | ---: | ---: | --- |
| Before | `9db7a94169821d9b28701eaa88866aee2fec1083` | 525.59 | 2815.07 | 608.22 | PASS, all five targets |
| After | pending committed implementation | NOT_RUN | NOT_RUN | NOT_RUN | pending measurement |

Before log SHA-256: `e9cea352a20800dcd9be7214fda59a692b34b1c14c9d6a24f89a721553034414`.
Before timing SHA-256: `13dfe508aa0e9d0cc599a5cce4e06f9514efc8fcdb4cec8c0fcc2d038b51dc13`.
The measured command is `/usr/bin/time -f 'elapsed=%e user=%U system=%S maxrss_kb=%M exit=%x'
make go-archive-gate` (with `-o` naming the external timing file and stdout/stderr retained there).
The after run will take the configured heavy semaphore before starting its timer.
A four-vCPU runner defaults to four target workers instead of five. Overlap can reduce idle
build/link and serial-target tail time; fourfold speedup is not implied because each Go build
already uses multiple CPUs. Four-vCPU CI timing remains `NOT_OBSERVED` until measured there.

## Verification and self-use

`make dogfood-change BASE=9db7a94169821d9b28701eaa88866aee2fec1083` recorded pre-change context
before any edit. The current-tree `make dogfood-prechange` retained context, query and path-impact
receipts; the installed binary first refused as stale, without changing the original task.
Receipt SHA-256 values: context `96587eda5a54a5b1b8d15d02b51e25b2d8031f7cb352f4786d558c3fc11573a9`,
query `55cc0c52ed1b010b2b0e245fc91f6240c0a4b18214ba9365c6d7354ec0f85ad2`, impact
`f3e48b16a3f780d4b860fdb098823d55f962794bb59a2f390d2a0d1568a03b89`.
`corvint affected --base 9db7a94169821d9b28701eaa88866aee2fec1083` selected
`conformance/release-artifact-v0` via DIRECT_SOURCE_CHANGE. Its retained scope is UNKNOWN:
277 selected, 592 excluded, 80 unknown (79 LANGUAGE_FRONTIER, one UNOWNED_DIRTY_PATH).
Those unknowns do not authorize the owner's excluded repository-wide selections.

The entire touched package passed through `script/heavy.sh`:
`go test -count=1 -timeout 30m ./conformance/release-artifact-v0` (16.295 s package runtime;
40 s load admission, 17.1 s total run). Existing assertions remain intact. New tests prove
bounded overlap, worker override behavior, serial-equivalent loose findings/checksums/summaries,
archive success aggregation, lowest-index failure with a faster later failure, suffix and parent
cancellation, joined workers and real descendant cleanup with no speculative suffix outputs.
The existing raw-export, ignored-source binding, cache-independence, clean-state and cancellation
cases run as part of that whole package.

Wrapper/injection checks and final static checks: pending. Independent review is performed by
the host orchestrator; no model-backed reviewer is run in this lane. Frozen ranking/selection
evaluations and repository-wide gates are NOT_RUN under the lane's explicit host-verifier boundary;
this change does not alter ranking or selection. Remote publication, native task writes and seal
are NOT_PRODUCED by explicit owner instruction. Optional self-development mutation, corpus,
learning, connector and external-outcome routes are not applicable to this release scheduler.

## Friction

1. Installed `corvint dogfood prechange` refused `self-binary-stale` (binary from `d436dff76de1`,
   54 differing Go paths). Repro: run the task's prechange command using the installed binary.
   Resolved using `make dogfood-prechange` and its current-tree build.
2. The package's heavy command queued 40 s at host load 57 (configured maximum 40). Repro:
   the package command above through `script/heavy.sh` on this loaded host. No semaphore overrides.
3. The first citation repin noted unstaged inputs; after staging, three old unpinned citations to
   `archive_run.go:422`/`422-424` could not move automatically and now hit blank lines. Repro:
   `script/check-line-citations.sh --repin` after staging the changed source. The old spans did
   not identify the clean-state guard even at BASE; corrected the three references to its actual
   current guard, preserving their meaning, then reran the repin and check.
