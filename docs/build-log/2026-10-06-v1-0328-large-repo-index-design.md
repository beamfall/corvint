# 2026-10-06: V1-0328 large-repository index design (proposal, not implemented)

## Intent

Ticket V1-0328 (no GitHub issue): a one-file commit on a 50,000-file repository rebuilds the
whole 61.6 MB snapshot (3.0 s wall, 13-14 CPU s), a 200,000-file tree is refused at the
source-count limit or, cold, at the 30 s deadline, and a kubernetes shallow clone is refused at
the 128 MiB aggregate bound. Owner bar (2026-10-06): really large projects must work without
slowdowns and nothing burns CPU unless it is essential; a refusal or a missed deadline at
200,000 files does not meet it. This entry is the design the coordinator asked for; it changes
no code. It builds on the measurements in `2026-10-06-v1-0881-single-event-bracket.md` and
`2026-10-06-v1-0416-refusal-order-and-affected-reads.md` and on the three experimental formats
the spec already carries. Where a number is an estimate it says so.

## What costs what today (measured, worktree at 26e9fbe0 unless noted)

| Repository | Operation | Now | Where |
|---|---|---|---|
| r5k (5,000 files) | cold `index` | 0.47 s, 1.5 CPU s, 6.0 MB snapshot | `index.go:277` `Build` |
| r50k | cold `index` / 1-file change | 2.9-4.4 s, 10-14 CPU s, 524-614 MiB RSS, 61.6 MB snapshot; a change is a full rebuild | `index.go:307` `buildStableFrom`, `git.go:561` `cat-file --batch` |
| r50k | `context` hit | 0.77 s, 1.2 CPU s, 281 MiB: the whole gob is hashed and decoded | `snapshot.go:862-880` `decodeSnapshotValue` |
| r50k | hook event hit (compact) | 1.0-1.3 s, about 200 MiB, two status scans | `snapshot.go:831` `LoadSnapshotObserved` |
| r200k | `index --if-stale` | refused, 0.54 s (after the V1-0416 pre-check; 2.4 s before) | `git.go:447-448` source count, `maxIndexedSources = 200_000` (`git.go:34`) |
| r200k | cold `index` with the count limit lifted (round 1) | deadline at 30.0 s after 20 CPU s | `git.go:26` `gitDeadline`, `git.go:300` |
| kubernetes (shallow, 20,485 admitted sources) | any index-backed verb | refused in 1.3-1.5 s: 169,044,137 source bytes over the 128 MiB bound | `git.go:670-681` `validateBlobAdmission`, `maxBatchBytes` (`git.go:31`) |

Snapshot size is linear, about 1.2 kB per file, so a 200,000-file gob would be about 250 MB and a
hit would decode it on every `context`, `impact` and index-backed event (estimate from the 5k and
50k points). The 1,000,000-byte per-file bound (`git.go:33`) and the 64 MiB tree-listing bound
(`git.go:30`, 0.22 s and about 14 MB at 200,000 entries) are not the problem.

## What the spec already has

- `IDX-SNAP-V0-014` (`snapshot_sectioned.go:32-55`, `:82-90`): a `.sect` file with a section
  table (name, offset, length, sha256) and `sectionsByLoad`, so a compact event reads only
  `identity`. Failed its 100 ms timing gate on 2026-09-05 because the bracket, not the decode,
  was the floor; the bracket is now one pair (V1-0881), so that gate is worth re-running.
- `IDX-SNAP-V0-015` (`pack_format.go:37-60`, `pack_reader.go`): the `.aip` pack, fixed-width
  little-endian tables over one string table, bodies aliased from a read-only mapping, 64 KiB
  block digests verified before any byte is decoded, at most four retained mappings. This is
  already the shape of a lazily loaded immutable index; it is opt-in and the gob is still the
  product path.
- `IDX-SNAP-V0-016` (`blob_shards.go:50-51`, `:56-105`): per-blob facts keyed by object format,
  blob OID and path digest, reused on a `context` miss. Its first evaluation failed
  (decision 0073: actual-tree parity and idle-host latency), because one file per blob under
  `.corvint/index/blobs/` costs an open per source on the cold path, the same cost `affected`
  shows at 200,000 files (open and lstat 11.5 of 19.7 sampled seconds).
- `IDX-SNAP-V0-017`: the analyzer identity pinned in every header, so a fact is reusable only
  under the engine that produced it. `IDX-SNAP-V0-022`: cold and incremental builds must be
  byte-identical (`TestColdAndIncrementalSnapshotsAreByteIdentical`).

The proposal composes these rather than adding a fourth format.

## Design

### 1. One sectioned, lazily loaded pack is the snapshot

Promote the `IDX-SNAP-V0-015` pack to the single on-disk snapshot, with the gob kept only as the
rollback encoding behind the existing `CORVINT_SNAPSHOT_FORMAT` switch until the format gate
passes. A hit maps the file read-only, verifies the header slot and section table, and decodes a
section's block digests only when a verb touches that section: a compact event touches
`identity`; `impact` touches `sources`, `tracked`, `skipped` and the symbol table; `context`
touches the vocabulary tables and the bodies of the results it returns, never every body. Bodies
stay aliased from the mapping (`IDX-SNAP-V0-015` already does this), so RSS on a hit is the
touched pages, not the file. The whole-file sha256 that `decodeSnapshotValue` streams
(`snapshot.go:871-880`) is replaced by the per-block digest table, which is what makes partial
reads sound: no byte is used before its block verifies, and any mismatch is the
`IDX-SNAP-V0-003` miss for the whole file.

New section: `facts`, one record per admitted source holding the per-blob analyzer output that
`collectBlobFact` (`blob_shards.go:299`) already defines (symbols, words, counted terms, markers,
imports, extraction refusal), keyed exactly as a shard is today: object format, blob OID,
sha256 of the full path, under the `IDX-SNAP-V0-017` engine in the header. The `sources` table
gains the offset of each source's fact record.

### 2. Incremental rebuild keyed by blob OID

On a tree change `index` runs the same stability window (`buildStableFrom`): opening
observation, evidence, closing observation, equal-or-retry. The evidence step changes:

1. List the tree (`ls-tree -r -l -z`, `git.go:412`, 0.22 s at 200,000 entries) and apply the
   admission rules, as now.
2. Open the previous snapshot for this repository, if any, under the same engine. For each
   admitted entry whose (object format, OID, path digest) matches a record in its `facts`
   section, copy that record's bytes into the new file without decoding them. The copy is
   verified by its block digests on read, as any other section.
3. Fetch only the remaining blobs with `cat-file --batch` (`git.go:561`) and analyze them as
   now (`compiledSources`), producing new fact records.
4. Recompile the global tables (vocabulary, symbol windows, identifier graph, co-change) from
   the full fact set and write the new pack. Write stays atomic and immutable: a new file
   named by tree and engine, evicted as today (`IDX-SNAP-V0-005`..`007`).

Byte identity (`IDX-SNAP-V0-022`) is the acceptance: the incremental file must equal the cold
file for the same tree, which holds when fact records are a pure function of (engine, OID,
path) and the global tables are a pure function of the fact set. That is what makes step 2 safe
to copy without re-analysis. A renamed file misses (the path digest is part of the key); that
is the conservative choice `IDX-SNAP-V0-016` already made and can be relaxed later by keying
analyzer context separately.

Step 4 is the part that does not shrink with the change set. If measurement shows it dominates
(estimate: a third to a half of today's compile at 50k), the vocabulary and graph tables can be
built as a merge of per-record postings rather than a scan of every body; that is a second
step, not required for the first gate.

### 3. Bounds: per stage, scaled, with a hard cap

Today one 30 s deadline covers the whole build and one 128 MiB bound covers every body the
build might fetch, so a repository is refused for its size even when the build would touch a
few blobs. Proposed:

- **Aggregate fetch bound applies to what this build fetches.** `validateBlobAdmission`
  (`git.go:670`) keeps its 128 MiB framed bound, applied per `cat-file --batch` invocation;
  blobs are fetched in batches of at most 128 MiB framed, so the bound limits a process's output
  (its purpose: `maxBatchBytes` is the output buffer) and no longer limits the repository. The
  hard cap moves to the index: admitted source bytes at most 1 GiB per snapshot and at most
  1,000,000 admitted sources, refused with the same `unsupported-impact-repository` code and a
  message that names the measured total. Kubernetes (161 MiB) is admitted in two batches; a
  repository over 1 GiB of admitted text is still refused.
- **Deadline per stage, hard total.** Each Git stage (listing, each fetch batch, each
  observation) keeps a 30 s deadline; the build as a whole gets a hard 5 min, and `index` reports
  the stage that exceeded. A cold 200,000-file build at r50k's rate is 40-60 CPU s, 15-20 s wall
  at `GOMAXPROCS=3` (estimate; the fetch is 4-6 batches), which the per-stage deadline admits and
  the old single deadline did not. The hook path keeps its 1.5 s adapter budget and never
  builds; the `--if-stale` loader keeps its 30 s.
- **Source count scales with memory, not a constant.** `maxIndexedSources` becomes the
  1,000,000 hard cap above, with the tree-listing bound (`maxTreeBytes`, 64 MiB, about 70 bytes
  per entry) as the practical limit it already is.

Every bound stays a constant in `git.go`, named in the refusal, so the fail-closed shape does
not change: no bound is read from the repository or the environment.

### 4. Invariants and gates

- AGENTS.md invariant 7: an immutable embedded index encoding is derived state, not a database,
  and needs the benchmark/format gate. The pack is immutable (new file per tree, never updated in
  place) and holds no state a verb cannot rebuild from Git. The gate, as `index-snapshot-v0.md`
  states it: cold and warm p95, memory, size and cross-format identity, measured on the pinned
  corpus and on r50k/r200k/kubernetes, before the pack replaces the gob on the default path.
- `docs/specs/deployment-neutral-index-platform-v0.md`: the pack must stay the transport-neutral
  immutable index contract (block digests, a header that names tree, engine and object format,
  no file-system state outside the file). Nothing here opens a daemon, watcher, account or
  network path (`GPK-V0-010`).
- `IDX-SNAP-V0-002`..`007`: identity, freshness, dirty paths, eviction unchanged.
  `IDX-SNAP-V0-003`: a damaged section is a whole-file miss, never a partial index.
  `IDX-SNAP-V0-017`: a fact record is reused only under the engine that wrote it; a schema
  change misses every record (the audit digest already pins the analyzer inputs).
  `IDX-SNAP-V0-022`: cold and incremental bytes equal, extended to the pack.
  `GPK-V0-007`: the build keeps the opening and closing observation around every read.
- Spec changes needed: amend `index-snapshot-v0.md` (`IDX-SNAP-V0-015` promoted from opt-in,
  a new requirement for the `facts` section and the incremental copy, a new requirement for the
  per-stage bounds; next free ID after `IDX-SNAP-V0-026`), `aggregate-local-outcome-v0.md:265`
  (the limits table), and the refusal message text in `git.go:680` with its consumers.

### 5. Rollback

Each step is behind the existing `CORVINT_SNAPSHOT_FORMAT` switch until the gate passes; the
gob writer and loader stay in the tree and remain the default until then. Rolling back after
promotion is removing the pack writer from the default path and evicting `.aip` files, which
the existing eviction already does for an unknown stem; no repository state, trace or refusal
code depends on the pack. The bounds change is a separate commit with its own rollback (restore
the four constants and the single deadline).

### 6. Expected numbers (estimates, to be replaced by the gate's measurements)

| Repository | Operation | Now | Expected |
|---|---|---|---|
| r50k | 1-file change | 2.9 s, 13-14 CPU s, full rebuild | 0.6-1.2 s, 2-4 CPU s: status 0.25 s, listing 0.05 s, one blob, record copy about 60 MB sequential, global tables recompiled (the dominant term) |
| r50k | `context` hit | 0.77 s, 281 MiB | 0.5-0.7 s, under 100 MiB: two status scans of 0.21-0.27 s each are the floor under the current bracket, plus 0.1-0.2 s for identity, sources, vocabulary sections and result bodies only |
| r200k | cold `index` | refused (count) or deadline at 30 s | 15-20 s wall, 40-60 CPU s, admitted under per-stage deadlines |
| r200k | 1-file change | refused | 2-3 s: the status scan (1.2-2 s) is the floor, as it is for hooks |
| r200k | `context` hit | refused | 2.5-4.5 s under the current bracket: two status scans of 1.2-2 s each are the floor, plus 0.3-0.6 s for the touched sections; 0.3-0.6 s only under the identity-only closing observation, an unqualified contract change (V1-0881 addendum). A 250 MB gob would have added 2-3 s and about 1 GiB RSS |
| kubernetes | cold `index` | refused at 128 MiB | about 5-8 s wall, 15-25 CPU s (161 MiB in two batches, at r50k's bytes-per-second); hooks then hit as r50k's do |

What this does not fix: the status scan floor on hooks at 200,000 files (two scans of 1.2-2 s
inside a 1.5 s budget; see the V1-0881 addendum for the two contract changes that would), and
`affected`, which does not use the index (its open-per-source cost needs reads relative to
directory handles or facts from this pack; follow-up).
