# 2026-10-07: snapshot mapping lifetime (V1-0983)

## Intent

Pack (`.aip`) and sectioned (`.sect`) snapshot mappings were never unmapped. The retention ring
(four per format) dropped its reference on eviction, but the bytes stayed mapped because an earlier
Index, and any string or slice a caller took from it, could still alias them. A long-lived reader
that opens more distinct snapshot files than the ring holds therefore kept every mapping. A deleted
snapshot file's blocks stayed allocated until the process exited. The change adds proposed
`IDX-SNAP-V0-028` to `IDX-SNAP-V0-030` to `docs/specs/index-snapshot-v0.md`, pending owner
acceptance.

## Reproduction

`TestEvictedPackMappingKeepsEscapedAliasesValid`
(`internal/contextindex/snapshot_mapping_test.go`) is the guard. It reads a pack and keeps what
callers keep:

- Symbol `Path`, `Name`, `Kind` and `BlobHash`;
- a Source value, as `internal/doccorpus` keeps one;
- that Source's `Text()`;
- a `Tracked` key;
- `Vocabulary.Paths[0]`.

It then drops the Index without releasing it, evicts the ring with four more distinct packs, runs
the collector twice, and reads every alias byte by byte. On the unchanged reader it passes. Both
naive fixes fault on the first alias read (`Symbol.Path`):

| Naive fix | Result |
|---|---|
| (a) `unmapReadOnly` on ring eviction | `unexpected fault address 0x1071c9621`, `fatal error: fault`, SIGSEGV code=0x2 |
| (b) `runtime.AddCleanup(packFile, unmapReadOnly, mapping)` | SIGSEGV at `0x105375621` in `checkAliases` |

Fix (b) shows why reachability cannot decide the lifetime: a string into a mapping keeps no Go
object alive. The cleanup runs as soon as the Index is unreachable, while the strings are still
held.

## Design: an explicit refcounted lease plus a heap copy of the string section

### Owner

`internal/contextindex/snapshot_mapping.go` holds the owner, apart from the readers.
`snapshotMapping` owns one mapping and an atomic reference count. The references are:

- one for the opener, which the ring takes over when it retains the file;
- one per non-compact Index a read hands out (`snapshotLease`). On a first read it is taken before
  the file is retained; on a retained hit it is taken under the ring's lock, so an eviction can
  never unmap between lookup and lease.

Eviction, a lost retention race and a refused read release their reference. `(*Index).Release()`
releases the Index's. The lease object holds the once-guard, so value copies of an Index share it
and releasing twice drops one reference. The mapping is unmapped when the count reaches zero.
Release is optional: an Index never released keeps its mapping for the process, exactly as before.
That is why the guard still passes, and why no current caller can crash.

### Heap strings

Every string a pack read hands out comes from the `strings` section:

- Source `Path`, `BlobHash` and `Mode`;
- Symbol fields;
- `Tracked` and `Skipped` keys;
- `Vocabulary.Paths`;
- the co-change history.

`packFile.heapStrings` copies the verified section once per mapped file, and every read of that
file builds strings over the copy. These strings therefore outlive the mapping without any caller
cooperation.

### Lease-scoped values

These alias the mapping and become invalid after `Release`:

- `Source.Data` and `Source.Text` results;
- the term postings in `Vocabulary`;
- deferred bodies.

No package outside `contextindex` reads `Vocabulary`. `internal/doccorpus` already copies a
source's text into `Data` before keeping it. A sectioned read maps only `Source.Data`; its other
fields are gob-decoded to the heap.

### Rejected alternatives

- **Unmapping on eviction, or from a GC cleanup.** It faults, as shown above.
- **Copying everything that aliases the mapping.** For this repository's pack (125,715,776 bytes)
  the bodies are 85.8 MB, and the term tables (`vocab.terms` 17.7 MB, `vocab.words` 12.9 MB) are
  another 30.9 MB. Copying them would put roughly 117 MB on the heap per read, against the pack
  spec's 8 MB heap target (`IDX-SNAP-V0-015`). The zero-copy profile exists to avoid exactly that
  cost.
- **Copying the strings section alone.** At 1.59 MB it fits that target, so it is the one copy
  made. It is made once per retained file, not once per read.

The lease keeps the remaining aliases zero-copy, and only callers that opt in pay for the bound.

## Measurements

This repository's pack (125,715,776 bytes) was measured with
`CORVINT_PACK_BENCH_ROOT=<clone> go test -run XXX_NONE -bench 'BenchmarkPack(FirstRead)?OpenRankEmit$|BenchmarkPackQueryAndEventLoads' -benchtime=30x -count=3`.
Each figure is the median of three runs, before (origin/main 0b5096ca) and after.

| Benchmark | B/op before | B/op after | allocs/op before | allocs/op after |
|---|---|---|---|---|
| `PackFirstReadOpenRankEmit` (new file each iteration) | 65,388,485 | 66,991,806 | 502,906 | 502,905 |
| `PackOpenRankEmit` (retained hit) | 65,291,019 | 65,346,918 | 502,677 | 502,678 |
| `open-full-deferred` | 20,930,051 | 20,989,420 | 82,587 | 82,590 |
| `open-event-deferred` | 10,972,788 | 10,979,861 | 82,204 | 82,206 |
| `query-deferred` | 142,859,864 | 142,880,620 | 1,340,037 | 1,340,036 |
| `impact-deferred` | 12,011,842 | 12,029,422 | 87,702 | 87,708 |

- **First read.** It pays the one string-section copy: +1.6 MB, which is the section's size, in
  one allocation.
- **Retained reads.** Every other load reuses the cached copy. Its bytes and allocations are
  unchanged within run-to-run noise, at most three allocations per op.
- **Latency.** Wall time could not be compared. The host's load average stayed at 48 to 52 on 12
  cores throughout. Spreads within one arm reached 2.3x (baseline `PackOpenRankEmit` 277 to
  639 ms; after 723 to 741 ms with package tests running alongside). The design adds no per-read
  work besides one atomic increment and, on the first read of a file, one 1.6 MB copy.
- **Mapping count.** `TestDistinctSnapshotReadsKeepBoundedMappings` reads 3 x capacity distinct
  files, releasing each Index. It holds live mappings at or below the ring capacity of 4, and at
  0 after the ring resets, for both packs and sectioned files. Without `Release`, live mappings grow
  by one per distinct file, as before.
- **Deleted blocks.** A temporary statfs probe (not committed) copied the pack to a scratch
  volume, read it fully, deleted the file and evicted the ring. With `Release`, free space rose
  124,583,936 and 124,706,816 bytes after eviction in two of three rounds; a third round's
  delta was masked by concurrent disk activity. Live mappings went 1 -> 0. Without `Release`, free
  space did not recover in any round, and live mappings went 1 -> 2 -> 3.

## Outcome

- **Callers wired.** `probeAnalyzerPack` releases the index it discards.
- **Not wired.** The MCP bridge's `context` call (`internal/mcp/bridge/bridge.go`, `callContext`)
  returns a Result whose packet map is serialized after the call returns. That packet can hold body
  text, so releasing there first needs an escape audit, or a copy of the canonical bytes. Until
  then the bridge keeps the earlier behaviour: nothing is unmapped, and nothing can fault.
- **CLI verbs.** These are one-shot processes and need no change.
- **File-identity dedupe.** The process-wide mapping per file identity (V1-0947) is left to that
  ticket. The owner type is the seam it can key by identity.
- **Rollback.** See the spec section. Removing the lease restores "eviction drops the reference
  without unmapping".
