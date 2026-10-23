# Affected graph reuse across calls and batch paths

Date: 2026-10-23. Lane: opencode/affected-reuse-gh1601 (base dd0d09db5). GitHub #1601, ticket
V1-2473. Requirements `AFP-V0-390..397` in `docs/specs/affected-plan-v0.md` are proposed.

## Problem

On a Rails checkout of about 12,100 tracked files (4,200 Ruby), each `affected --path P` cost about
11 s and 300 MB on a busy host. Repeated calls shared nothing. An agent that asks after each edit
paid the full graph build every time.

## Profile

The fixture was a generated Rails-shaped tree: 11,808 tracked files, 4,000 Ruby (models,
controllers, services, lib, and model, request and service specs that load `rails_helper` and
`spec_helper`), 7,800 views, JavaScript, locale and Markdown files, one commit. The profile came
from an in-process test harness: `runAffected` under `runtime/pprof`, five calls. It showed:

- `affected.Build` took 74% of CPU, and the Ruby plugin's `UnitsSource` took 58%.
- `open(2)` took 38% through `readSourceFile`. That was one open per Ruby file, plus one per
  helper per spec: `helperFramework` read and scanned `rails_helper.rb` and `spec_helper.rb` again
  for each of the 2,900 specs, about 60% of all opens.
- The rest was the two source walks (6%), the TypeScript plugin (7%), and the two worktree status
  reads (7%).

## Decision

- Cold path (AFP-V0-394). The Ruby plugin scans each loaded helper once per observation. A pool of
  eight concurrent Ruby readers was tried and dropped: wall time fell by under 10%, but the
  process used about three times the kernel CPU (1.9 s against 0.6 s of system time), which a
  busy host pays for.
- Reuse (AFP-V0-390..393). The cache is opt-in: `--cache DIR`, a directory the caller names. It is
  refused inside the repository root, through a case-folded or firmlinked path, or under any
  `.git` directory. An entry binds the root, the SHA-256 of the running executable, the language
  set, each plugin's raw result, and a stat stamp taken before each read. Stamps cover every
  directory the source walks listed and every path a plugin read, statted or found absent.
  - A directory's times move whenever one of its entries is added, removed, renamed or replaced.
    So a warm call restats about 8,500 paths and reads no directory or file.
  - A hit needs identical stamps, plus the recorded digest from a graph rebuilt through `build()`.
    Anything else rebuilds the graph and replaces the entry.
  - Paths changed within 2 s of the build are not pinned. Neither is a present Swift `.maestro`
    walk, nor a language that bypasses the recorded source.
  - Only `affected.Build` is replaced. Status, HEAD, ignored-path, advice, selection and the drift
    rechecks run on every call.
- Batch (AFP-V0-395/396). `--paths-from FILE` feeds `--path`. The help and `docs/CAPABILITIES.md`
  tell callers to batch.

The cache is keyed by stat stamps of what the build observed, not by the Git tree, because the disk
graph also reads ignored and untracked files that no tree pins. `--path` already refuses those
files, but the default form does not. The change time is the anchor: every write sets it to the
current time, and an unprivileged process cannot set it back.

Invariant 4 analysis: the entry lives at a location the caller named, outside repository content.
It is never under `.corvint/`. It is never evidence, and never an input to ranking, authority or
learning. It can only reproduce the graph a full build produces, and the receipt is byte-identical
whether it is used or not.

An independent read-only review found no HIGH issue. The fixes it prompted:
- the same-file root check and the case-folded `.git` check;
- resolving `--cache` before reading `--paths-from`;
- refusing a leading byte order mark;
- an `fstat` check on the opened list file;
- refusing to pin a language that bypasses the recorded source;
- a spec note on file systems whose change time only mirrors the modification time.

## Measured

Host: 12 cores, darwin/arm64, load average 10 to 20 during the final runs. The issue's host had a
load average of 14 to 22 on 10 cores. Times are wall clock, memory is peak RSS from
`/usr/bin/time -l`, single `--path app/models/thing5.rb`:

| Call | Runs | Wall time | Peak RSS |
|---|---|---|---|
| base binary (dd0d09db5), no cache | 3 | 1.19, 1.22, 1.78 s | 144-151 MB |
| this change, no cache | 3 | 1.03, 1.04, 1.84 s | 133-146 MB |
| this change, cold `--cache` (build + entry write) | 3 | 1.12, 1.16, 1.19 s | 157-163 MB |
| this change, warm `--cache` | 8 | 0.41 to 0.43 s | 145-167 MB |
| `--paths-from` with 10 paths, no cache / warm | 1 each | 1.05 s / 0.41 s | 155 / 151 MB |

At a load average of 20 to 40, warm calls took 0.43 to 0.55 s. Earlier runs at load 15 to 48 with
a re-walking verifier gave warm calls of 0.5 to 1.8 s, which is why directory stamps replaced
re-walks. Ten single-path calls without the cache took 17.1 s at a load average of about 42.

All receipts were byte-identical (`cmp`) across the base binary, no cache, cold and warm. The entry
is 16 MB for this tree, most of it the Rails route-dispatch edges. In a warm call, the remaining
time is mostly the two worktree status reads and the HEAD reads that every call keeps, plus
decoding the entry.

## Limits

- The cache works only on darwin and linux. Elsewhere every call is a full build.
- A warm entry is still read and decoded whole. Peak memory is similar to a cold build, not
  smaller.
- Any change to an observed file rebuilds the whole graph. There is no incremental update.
- The 11 s and 300 MB figures in the issue come from the reporter's real checkout, which was not
  re-measured here. The numbers above are for the generated fixture.
- A call within 2 s of an edit to an observed file or directory writes no entry. The next call
  stores one.
- On a file system whose change time only mirrors the modification time (exFAT, some FUSE mounts),
  a same-size write that restores the modification time is not detected.

## Independent review repair (PR #1607)

A second independent read-only review of `5078ba9df...c55ddc944` found two P1, four P2 and one P3
issue. Each fix has a regression test. With the fix disabled, the stamp, read-failure, directory
replacement, Git directory and FIFO entry tests were run and failed; the FIFO entry test blocked
until its 30 s bound. The race tests use controlled hooks at the exact point a racing process
would act.

- P1, stripped stamps. `verifyObservation` accepted an empty or partial stamp list, so a
  rechecksummed entry with `stamps: []` hit after a source edit. `completeStamps` now requires a
  strictly ordered, unique list that stamps the root listing, the documentation declaration,
  every unit source and test file, and every directory above one. Otherwise the entry misses.
- P1, separate Git directory. `--cache` at or under a `--separate-git-dir` directory passed.
  `resolveAffectedCache` now asks `git rev-parse --absolute-git-dir --git-common-dir` and refuses
  at, under or aliased to either. A Git directory that cannot be named refuses `--cache`.
- P2, directory replacement. Writes went through the pathname, so a directory swapped for a link
  into `.corvint/` after validation took the write. Every read and write now goes through one
  `os.Root` handle. The deepest existing ancestor is opened first, and each missing component is
  created and opened inside it. Each handle is rechecked against the root and both Git
  directories, by the kernel's path for the descriptor (`F_GETPATH` on darwin,
  `/proc/self/fd` on linux) and by file identity. The directory must also be owned by the user
  and not writable by group or other.
- P2, executable identity. Linux now hashes `/proc/self/exe`. Darwin has no handle on the
  running image, so the executable pathname is accepted only when its change and modification
  times are strictly before package initialization, and unchanged while it is hashed. Otherwise
  there is no identity and no cache.
- P2, transient read failures. A read, stat or bounded read that fails for any reason the stamp
  does not pin now leaves the observation unpinned. The pinned reasons are an absent path, a path
  that is not a regular file, and a file over its bound. Anything else (I/O or permission errors)
  stores no entry.
- P2, FIFO. The `--paths-from` file and the cache entry are opened nonblocking, and only the
  descriptor is checked. The pre-open stat is gone.
- P3, helper reads. The Ruby source loop and the helper scan now share one read per file. The test
  asserts exactly one open per helper, source and spec, so the requirement text did not need to
  change.

Coverage gaps closed: AFP-V0-393 parity now covers `--require-clean`, `--paths-from`, the strict
and coverage provider forms, `--appmap-impact`, and an unwritable or uncreatable cache directory.
AFP-V0-395 now has an over-4-MiB refusal.

Retained limits:

- The checksum is integrity, not authentication. A process that can write the cache directory can
  forge a self-consistent entry. That is why the directory must be private to the user. Stamp
  completeness also cannot cover reads outside unit files, such as a Gemfile.
- Each handle is checked at the moment it is opened. A process that renames the already opened
  directory into the repository afterwards is not caught, but that rename is itself a repository
  write.
- On darwin, a replacement installed between exec and package initialization (milliseconds) is not
  detected.
- The reporter's real checkout (GH #1601) is still not measured. Its warm time is `NOT_OBSERVED`.
