# 2026-10-06: rc.3 core performance

## Intent

For rc.3 the `corvint` binary should be faster and leaner without changing any command's output.
Measured on rc.2 against the Corvint repository: `index --if-stale` took 2.26 s cold and 0.18 s warm,
`affected --base` took 1.34 s, and `context --task` took 0.30-0.40 s at about 330 MB RSS. The
user-prompt hook reported "does not fit the hook deadline".

Scope is `cmd/corvint` and `internal/**`, except `internal/tasks/**`. Out of bounds:
- the immutable snapshot format (invariant 7: a change may only be proposed);
- ranking, selection and scoring logic, including the `internal/contextindex` ranking code owned by
  the V1-0859 lane;
- mutation by any read command (invariant 4).

Related tickets: V1-0328, V1-0418, V1-0416, V1-0372 and V1-0554.

## Change

- **`internal/gitstatus/worktree_posix.go`: V1-0388 FIFO refusal (`worktreeInputsOpen`).**
  - Before: every status read did one `os.Root.Lstat` for each `.gitignore` and `.gitattributes`
    path of every index directory. Each `os.Root` call re-walks the path from the root.
  - Now: `worktreeInputModes` visits the directories in a subtree-contiguous order and keeps an
    `os.Root` handle open for each directory on the current path. It Lstats the two base names
    through that handle.
  - A sub-root is opened only for a component that is a plain single name and that the parent's
    `Lstat` reports as a real directory, and only while fewer than `maxInputHandles` (32) handles
    are open. Anything else falls back to the old full-path `worktree.Lstat` for that directory and
    every directory below it: a symlink, a missing directory, a file, or a deeper directory.
  - The refusal is still the first blocking input in the original `worktreeInputs` order, with the
    same message and class.
- **`internal/liveverify/affected/readers.go`: AFP-V0-021 path-literal readers.**
  - `componentRuns` used to be recomputed for every unit, token and dirty path. It is a pure
    function of the token, so one selection now memoises it in a `runCache` shared across its dirty
    paths.
  - `namesPath` keeps its uncached signature for `resolves` and for the mirror in
    `tools/gate-affected-select`.
- **`internal/liveverify/affected/golang/golang.go` and `source.go`: Go directory observation.**
  - A disk source is observed by a bounded pool of `min(GOMAXPROCS, 8, directories)` workers.
  - Each result is stored at its directory's sorted offset, and the caller merges units, edges,
    frontier reasons and the first error in that same order.
  - `Source.ConcurrentReads` admits only a disk source. An FS source, which keeps a shared walk cache
    and retains failures, stays serial.
- **`cmd/corvint/repository_guidance.go`: `review`, `features` and `overview`.**
  - Every source line was run through two backtracking regexps.
  - `guidanceMayMatch` now skips a line that lacks a literal the pattern requires:
    - the marker pattern needs a `:`;
    - the registration pattern needs a `(`, a quote, and a substring of one of its names.
  - These are necessary conditions, so a skipped line could never have matched, and receipts are
    unchanged.

- **The analyzer schema moves to `corvint-analyzer/104`.** `gitstatus` is an input pinned by
  `TestAnalyzerSchemaInputs`, and its rule is to bump on any change to it. Extracted facts
  are unchanged, and existing analyzer packs are rebuilt once. The V1-0859 orientation change
  also bumped to `104`, so the rc.3 integration batch lands both as `corvint-analyzer/105` with
  one repinned audit digest.

## Decisions

- **Snapshot load is unchanged; a concurrent trailer digest was tried and withdrawn.**
  - Two versions moved the SHA-256 off the decode's path. The first hashed an independent second
    read of the file (review finding 1 below). The second hashed copies of exactly the decoded bytes
    on another goroutine.
  - On `context --task` over the Corvint corpus, eight alternating runs each:
    - main: 0.29-0.35 s, 316-319 MB;
    - second-read digest: 0.23-0.28 s, 318-397 MB;
    - decoded-bytes digest: 0.27-0.33 s, 335-406 MB.
  - The GC trace shows the same heap peak (295-310 MB) in all three. The extra RSS appears when the
    heap overshoots its goal during concurrent marking: 7 of 8 runs of the decoded-bytes variant,
    3 of 8 of the second-read variant, and none of main.
  - 20-60 ms per load was not worth up to 85 MB more RSS on a command whose RSS was a stated
    complaint, so `decodeSnapshotValue` stays as on main. This is recorded as deferred potential.
    It includes warm `index --if-stale`, which took 0.10-0.12 s with the second-read variant against
    0.14-0.15 s on main.
- **The snapshot format is unchanged.** The remaining load cost is in gob itself.
  - gob reads the roughly 120 MB Index message through `saferio.ReadData`, which grows its buffer in
    10 MB appends. That costs about 500 MB of allocation per load in the allocation profile.
  - Avoiding this needs a forked gob or a different layout. This is proposed as a deferred format
    change: promote the experimental `CORVINT_SNAPSHOT_FORMAT=sectioned|pack` under its
    benchmark/format gate. It is not done here.
- **Query observation brackets are kept.** A query runs about four `git status` calls of 70-85 ms
  each, in scratch metadata copies, as deliberate before/after observation.
  - Collapsing them is the proposed `CORVINT_QUERY_SHARED_OBSERVATION=1` (GPK-V0-065), which is not
    accepted, so it is deferred.
  - This change only makes each status call cheaper: about 110 ms became about 80 ms on the Corvint
    corpus, and the input refusal alone went from 62 ms to 15 ms.
- **Ranking-lane hotspots are recorded, not touched.**
  - `evalSymbolTerms.of` in `internal/contextindex/eval_query.go` builds a map union for each symbol.
    In a Corvint query that costs about 160 ms of CPU, 130 ms of it in `memclr`.
  - V1-0372 (repeated PageRank degree sums in `internal/contextindex/ppr.go`) applies only with
    `CORVINT_CONTEXT_GRAPH=on`.
  - Both are in V1-0859's ranking code, so both are left to that lane.
- **The concurrent Go observation raises the `affected` peak RSS by 8-17 MB** (40-56 MB became
  46-72 MB). This is kept for the 30-50% wall-clock gain.
- **A whole-repository walk was not used as the prefilter's maintained test.** Reading the tree from
  a `cmd/corvint` test would make that package an unbounded reader for `affected`. The maintained
  test uses seeds plus a fuzz target, and the corpus walk was a one-off scratch check (see Evidence).

## Evidence

Machine: Darwin, 12 CPUs, load average 10-17 throughout.
- `old` is `origin/main` 855077fc, built with `-trimpath -ldflags "-X main.build=0"`.
- `new` is this branch, built the same way.

Each binary indexed its own private clones:
- `corpus/corvint`: Corvint at 855077fc, 1.58 M lines.
- `corpus/prom`: Prometheus, 0.58 M lines.

Wall time and RSS are medians of 5 interleaved runs of each command, with the old and new order
alternating.

| command | old wall / RSS | new wall / RSS |
|---|---|---|
| c-query-1 (`query --task`) | 0.690 s / 342 MB | 0.522 s / 341 MB |
| c-context-1 (`context --task`) | 0.345 s / 319 MB | 0.345 s / 320 MB |
| c-context-4 (`context --lsp off`) | 0.366 s / 322 MB | 0.343 s / 317 MB |
| c-impact-3 (`impact --base`) | 1.609 s / 319 MB | 1.083 s / 315 MB |
| c-affected-1 (`affected --base`, 1 commit) | 1.486 s / 55 MB | 0.760 s / 71 MB |
| c-affected-2 (`affected --base`, 5 commits) | 2.646 s / 57 MB | 1.824 s / 71 MB |
| c-review-2 (`review --base`) | 1.894 s / 47 MB | 1.047 s / 48 MB |
| c-features | 0.962 s / 47 MB | 0.454 s / 48 MB |
| c-overview | 1.031 s / 361 MB | 0.542 s / 361 MB |
| c-harness-1 (user-prompt hook) | 0.857 s / 323 MB | 0.551 s / 323-391 MB |
| p-query-1 | 0.390 s / 107 MB | 0.330 s / 106 MB |
| p-affected-2 | 0.752 s / 40 MB | 0.495 s / 48 MB |
| p-review-1 | 1.224 s / 55 MB | 0.638 s / 55 MB |
| p-overview | 0.859 s / 120 MB | 0.412 s / 120 MB |
| `index --if-stale`, warm, Corvint (5 interleaved runs) | 0.14-0.15 s | 0.14-0.16 s |
| `index`, cold, Corvint (3 interleaved runs, first commit) | 1.87-2.18 s / 0.92-1.0 GB | 1.83-1.97 s / 0.88-1.0 GB |

The table is the final binary, after the review fixes. The warm `--if-stale` gain of the first commit
(0.16 s to 0.11 s) came from the withdrawn snapshot digest change, so it is not claimed.
Across all 36 commands the new/old wall-time ratio ranged from 0.47 to 1.00, with no regression.
The harness RSS varies with GC timing in both binaries: over eight alternating runs it was 325-390 MB
for old and 324-399 MB for new.

- **Output parity.** The corpus held 36 commands over both repositories:
  - query, context, impact, affected, review, features, overview, `dogfood status` and the harness
    user-prompt hook;
  - plus 9 commands re-run after a tracked edit and an untracked file.

  Results:
  - For all 45, stdout, exit code and stderr were byte-identical between old and new.
  - The one exception is `d-c-impact-2`, which differs only in `identity_sha256`. Two old runs
    differ in that field in the same way.
  - The 45 commands were run for the first commit and again for the final binary, with the same
    result.
  - `index` output was equal apart from the engine digest and path. Snapshot sizes were identical:
    124501338 and 25723101 bytes.
- **Frozen retrieval eval.**
  - Command: `corvint eval --goldens testing/context-retrieval-goldens.json`.
  - Repository: a private local clone of the manifest's `beamfall` repository at 6e82abd8.
  - Old and new outputs were structurally identical except for `latency_ms`. It was 8391 ms for
    old and 2902 ms for the final binary, but the final run had a warm index, so the latency is not
    comparable.
- **Maintained tests and benchmarks.** All PASS.
  - `TestWorktreeInputModesMatchRootLstat`
    - Covers symlinks that stay in, leave and climb out of their parent, a symlink out of the root,
      dangling and looping links, a file in place of a directory, and FIFOs at any depth.
    - Also covers `..` and absolute names and both name orders. It compares against the per-input
      Lstat oracle and the old refusal.
    - `BenchmarkWorktreeInputsOpen` (512 directories): 53.4 ms/op became 12.6 ms/op; allocations rose
      from 12 k to 25 k per op.
  - `TestNamersWithSharedRunCacheMatchesUncached` compares the cached match with the uncached one
    over anchored, climbing, partial, lone-component and shared tokens, in both dirty-path orders.
  - `TestConcurrentDirectoryObservationMatchesSerial`
    - Compares `GOMAXPROCS` 2, 8 and 16 with 1 over 40 packages carrying import, test-import,
      path-literal, cgo and unparsed-source frontiers. It passes under `-race`.
    - `BenchmarkUnitsSourceDisk` (400 packages): 40-49 ms/op became 32-35 ms/op.
  - `TestGuidancePrefilterIsANecessaryCondition` and `FuzzGuidancePrefilter`
    - A 60 s fuzz run did 253309 executions and found no failure.
    - A scratch check confirmed the prefilter over every line of both corpora (2.16 M lines, 1090
      matching).
- **Test runs.**
  - Touched packages (`internal/gitstatus`, `internal/contextindex`, `internal/liveverify/affected/...`,
    `cmd/corvint`) ran with `GOMAXPROCS=2 GOTOOLCHAIN=local go test -p 1 -count=1 -timeout 30m`.
  - `go vet` ran for darwin, linux and windows, and `gofmt -l` was clean.

## Independent review

Codex (`codex exec -s read-only`, diff 855077fc..0ae4de01) reported three P2 findings. Each was
checked before it was fixed; the fixes and the withdrawal are in the second commit.

1. **Snapshot digest no longer of the decoded bytes (`snapshot.go`).** Confirmed by reading: the
   first version hashed a second, independent read of the file. A same-length in-place rewrite with
   its modification time restored could pass both the digest and the re-`Stat`. A fix that hashed
   copies of the decoded bytes kept the guarantee but raised RSS (see Decisions), so the change was
   withdrawn and `snapshot.go` is identical to main.
2. **Symlink budget reset per component (`worktree_posix.go`).** Reproduced: two chains of five
   directory symlinks resolve in their own sub-roots but exceed the root's budget as one path, so
   the first version reported a FIFO the old code did not. The case is now in
   `TestWorktreeInputModesMatchRootLstat` and fails on the first version. Fixed by opening a
   sub-root only for a real directory.
3. **One descriptor per ancestor (`worktree_posix.go`).** Confirmed by reading: the handles were
   bounded only by the directory depth. The old code also ignored an `Lstat` error, such as
   EMFILE, but used no lasting descriptors. Fixed with the `maxInputHandles` cap; the test adds a
   directory 40 levels deep whose FIFO must still be found.

A narrow race remains: a directory replaced by a symlink between the parent's `Lstat` and
`OpenRoot`. A status read is not atomic against concurrent edits before or after this change.

## NOT_RUN

- Frozen retrieval eval for the public manifest corpora (flask, cobra, zod, execa). There is no local
  checkout, and fetching them needs a network download that was not approved.
- The exhaustive gate (`make gate`, full `go test ./...`). Owner preference for scoped work is
  focused tests.
- The dogfood CEM bind/check/seal loop: the session hook reported
  `corvint-event-rejected:dogfood-event-deadline`.

## Rollback

Revert this branch's commits. No snapshot, trace, ledger or wire format changes. Each change is local
to one function and keeps its old behaviour as the oracle in its test, so any one can be reverted
alone.
