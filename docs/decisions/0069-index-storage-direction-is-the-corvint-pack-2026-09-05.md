# Decision 0069 — The index storage direction is the Corvint Pack: a zero-copy tree pack now, blob-addressed shards next

Date: 2026-09-05. Status: accepted (2026-09-05, delegated call; prototypes in flight, promotion gated). Authority: repository owner, delegated call; the owner instructed on 2026-09-05 that the panel figure out how to beat grep on performance and, if needed, design a next-generation storage and indexing technology, and that open calls be resolved through the expert panel. This record applies the storage memo (Code-Search Index Architect), the latency memo (performance-scalability engineer), the internal capability audit and the external technology scan, after independent review of their cited evidence.

## Context

Measured on this repository (2,301 tracked paths, 1,943 sources) with the BM25 lane's binary:

- A snapshot hit for `context --task ... --limit 20` is 70–80 ms wall, at ripgrep parity
  (`rg -l -i` over three terms is 80–110 ms on the same tree). The ranking is 8 µs; the
  47.6 MB gob decode (27–43 ms) is hidden behind the `git status` spawn.
- The snapshot key is object format, `HEAD^{tree}` and the executable digest. A dirty worktree
  does not miss (`IDX-SNAP-V0-004`); a new commit or a rebuilt binary does, and the miss is a
  full in-process rebuild of 480–560 ms wall and 1.25–1.86 s CPU. The last twenty commits changed
  0–31 files each (median 8.5), so a per-tree rebuild redoes about 99.5 % of work whose inputs,
  the blob OIDs, did not change. The earlier 0.74–0.83 s figure was this miss.
- A hit with a subject is 0.6–1.2 s wall at 60 ms CPU: co-change is recomputed by
  `git log -200` on every call although it is a pure function of the HEAD commit.
- The gob format decodes the whole value for any read and satisfies none of `DNIP-IDX-005`,
  `-006`, `-007`; the sectioned prototype (`IDX-SNAP-V0-014`, proposed) has the right skeleton
  but still verifies the whole 35 MB bodies section and gob-decodes every table into fresh heap.
- The field's measured wins (GitHub's code search, Cursor's layered index, Zoekt's mapped
  shards) key by blob or by commit plus overlay and read only touched postings; none combines
  Git-OID content addressing with co-located graph sections and a BM25 field model.

## Decision

1. The immutable embedded index encoding for the local product is the **Corvint Pack**: a
   per-tree file (`<objfmt>-<tree>-<engine>.aip`) of 64-byte-aligned sections with a 64 KiB block
   digest table per section, memory-mapped read-only on unix with a `ReaderAt` block-cache
   fallback elsewhere, verified block by block on first touch, zero-copy views over fixed-width
   term tables, blob table and symbol table, bodies mapped and paged in only where touched, small
   tables gob-inside-a-verified-section until measured otherwise, and a co-change section keyed
   by the HEAD commit that falls back to the spawn for that section only.
2. Order of work, each behind its own falsifier and proposed clause: (a) the zero-copy tree pack
   under `CORVINT_SNAPSHOT_FORMAT=pack` (proposed `IDX-SNAP-V0-015`); (b) the miss quick wins that
   need no format change (the loader's observation handed to the build, a pre-sized term counter,
   the residual blob read folded into the closing window); (c) the blob-facts shard store keyed
   by blob OID so a tree is a manifest and a post-commit miss re-reads only changed blobs;
   (d) native Git observation only behind a 10,000-state byte-parity falsifier against
   `git status --porcelain=v1 -z --untracked-files=all`.
3. Promotion of any encoding stays under `IDX-SNAP-V0`'s cross-format gate and `DNIP-IDX-009`'s
   immutable-SQLite baseline: the pack must win open-plus-query p95, heap and bytes or it is not
   selected. `context` output stays byte-identical across formats; no ranking gain is claimed from
   storage work.
4. The default path runs no new code until promotion; a refused pack falls to the gob, then the
   build (`IDX-SNAP-V0-003`), never to a partial index.

## Evidence and falsifier

Numbers to beat on this repository (n ≥ 10, idle host, min and p50 reported with load average):
hit without subject in-process open-rank-emit ≤ 10 ms p95 and ≤ 40 ms wall with the status
spawn, heap ≤ 8 MB (today 48 MB); hit with subject ≤ 50 ms wall (today 0.6–1.2 s); tree pack
≤ 4 KB per file; miss after a commit changing ≤ 10 files ≤ 120 ms wall and ≤ 150 ms CPU once the
shard store exists (today 520–560 ms, 1.25 s CPU); one flipped byte in every section and block, a
truncated file and an out-of-range offset each refuse the pack with byte-identical output through
the fallback and zero panics under the race detector. A cold first build cannot reach ripgrep's
80 ms (`ls-tree` alone is 66 ms and the build is 1.86 s CPU on twelve cores); the claim is for
the hit and the post-commit miss, not the first build of a fresh clone.

## Consequences

- The sectioned prototype's block layout, mmap files and section table are reused by the pack;
  `IDX-SNAP-V0-014` is superseded by `IDX-SNAP-V0-015` once the pack is measured.
- Moving the store under the Git common directory so linked worktrees share it changes
  `IDX-SNAP-V0-001/-005`'s path contract and is a separate accepted change.
- Replacing the executable digest with a build-id engine rule is a separate amendment to
  `IDX-SNAP-V0-001` for the new format only.

## Rollback

Delete the `.aip` files and the opt-in flag; the gob snapshot and the build path are unchanged
and remain the default until promotion.
