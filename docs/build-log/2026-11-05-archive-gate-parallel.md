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
| After (default five workers) | `c2e9330b80f23bc05d19a03efbc22132377efce9` | 191.54 | 3181.71 | 631.78 | PASS, all five targets |

Before log SHA-256: `e9cea352a20800dcd9be7214fda59a692b34b1c14c9d6a24f89a721553034414`.
Before timing SHA-256: `13dfe508aa0e9d0cc599a5cce4e06f9514efc8fcdb4cec8c0fcc2d038b51dc13`.
The measured command is `/usr/bin/time -f 'elapsed=%e user=%U system=%S maxrss_kb=%M exit=%x'
make go-archive-gate` (with `-o` naming the external timing file and stdout/stderr retained there).
The after run took the configured heavy semaphore before starting its timer (zero admission
wait). Wall time fell by 334.05 s (63.56%), a 2.744x speedup. Total CPU time increased from
3423.29 to 3813.49 s; this is scheduling overlap, not less build work. These are one before/after
pair on a shared host, not a controlled benchmark distribution. The baseline command's peak
single-process RSS was 741900 KiB; after it was 1172580 KiB, not aggregate worker memory.
After log SHA-256: `ff1647ce658785353790c11593a665ad7f30f528f9fceced9a2a841a68ecc512`.
After timing SHA-256: `6135e5a6d1b6590b39e5461d46e627219338a9d30081a62948661831e527cd72`.
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

The focused concurrency tests also passed with `-race` (2.049 s). The required
`make -s spec-requirements-check line-citations-check` passed after the citation repair;
`make go-archive-gate-test` passed its existing private setup, cleanup, failure, interruption,
witness and closed-set checks. `make go-archive-gate-injection-test` first failed before real
production: the existing harnesses
used `/bin/sh` but require Bash job-control and `kill --` behavior. On Ubuntu's dash without a
terminal this prints `set: can't access tty; job control turned off` and `kill: Illegal number: -`.
Design decision: name Bash explicitly in both test harness shebangs, matching the gate's existing
Bash dependency and preserving every assertion. The rerun at
`2a74e84c2` passed in 278.85 s: real production and unmodified replay PASS; extra regular file,
directory and symlink each failed at 8 versus 7 entries; symlinked archive failed at 6 versus 7
regular files; missing archive failed its presence check; real interrupt exited 130 with no
private parent or group survivor. The source witness and clone worktree remained unchanged.
Injection log SHA-256: `cdfab7b89d8a51bf87e5fbf6808885208f003507afa22def81c6033d25133be5`.
Injection timing SHA-256: `5dd869e33a39a8d9cb84d9d944b897fa223ff1db18d4b55296a98728abe0e85b`.
Wrapper-test log SHA-256: `b658a62168b8fe869425ea619c39eadd0334f0ac16795571764c690550b26a31`.
Whole-package log SHA-256: `d5c06275b24cde08a1be0435aaaee56a54c2f61eb83c0e06a4029eb597ef1231`.
Race log SHA-256: `15f76586a2cdbb3e8e51246f2c47b040550048d246c6edab9eaeb0e3897cfcd9`.
Static-check log SHA-256 (before the harness documentation addition): `74fe5102626b9a8c2cdcc7fa2ef01364f737c489de63498f9ab6381cf85b91fa`.
Independent review is NOT_RUN in this lane; the host orchestrator owns it. Frozen ranking/selection
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
   current guard, preserving their meaning. The next repin required new spans to be hash-pinned
   and their three retired unpinned-inventory rows deleted. Those mechanical changes passed
   `script/check-line-citations.sh --repin` and the required final check.
4. `git commit` printed `Unable to create '/home/russell/projects/corvint/.git/packed-refs.lock':
   Read-only file system` during automatic maintenance, but committed successfully (exit 0).
   Repro: the implementation commit; the exact denied path is outside the writable Git subset.
   No metadata-permission override or repository-wide config change was used. The harness
   commit printed the same warning even with per-command maintenance disabled and also succeeded.
5. `make go-archive-gate-injection-test` initially exited 2 in 20.72 s because the two harnesses
   named `/bin/sh` while relying on Bash behavior. Repro: the target from a non-TTY command on this
   Ubuntu host (`/bin/sh` is dash). The first interrupt check failed before real producer testing.
   Both shebangs now use `#!/usr/bin/env bash`; no assertion, production script or wire text changed.
