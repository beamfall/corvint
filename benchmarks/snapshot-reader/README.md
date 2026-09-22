# snapshot-reader (AT-04 isolated reader-format experiment)

Compares four ways to encode and re-read a synthetic corpus shaped like
`internal/contextindex.Index`: (a) `encoding/gob`, one value, the current
`internal/contextindex/snapshot.go` style; (b) immutable SQLite
(`modernc.org/sqlite`, pure Go), one table per record kind, path-indexed;
(c) a sectioned pack -- a manifest (path list + per-kind, per-path block
offsets/lengths/SHA-256) written last via temp-file-then-rename, followed by
independently checksummed, independently gob-decodable blocks, read through
a counting `io.ReaderAt`; (d) pack v2 (`packv2.go`, added 2026-09-12) -- the
same per-path, per-section blocks, but a fixed 372-byte, self-checksummed
manifest that is O(sections) and a static checksummed tree over a
fixed-width sorted path table, with one shared gob type descriptor per
section instead of one per block. See "Pack v2" below.

This is a standalone Go module (`github.com/corvint-context/corvint/benchmarks/snapshot-reader`)
and does not import `internal/contextindex` (not importable from another
module); it exists to gather format evidence for AT-04 against
`DNIP-IDX-004..009`, not to change production code.

## Commands

```sh
cd benchmarks/snapshot-reader
GOTOOLCHAIN=local go mod tidy   # resolves modernc.org/sqlite v1.58.0
GOTOOLCHAIN=local go test -count=1 ./...
GOTOOLCHAIN=local go vet ./...
GOTOOLCHAIN=local go test -run '^$' -bench . -benchmem -benchtime=3x -count=3 ./...
# pack v2 rerun (one-path cells only, all four encodings):
GOTOOLCHAIN=local go test -count=5 -timeout 30m -run '^$' -bench 'BenchmarkSnapshotRead/files=/./one-path' -benchmem -benchtime=3x ./...
```

The results below use `-count=3`: three independent runs of every named
benchmark (each itself `-benchtime=3x`, so 9 timed iterations per cell), so
a single lucky/unlucky sample can't stand in for the number. The full
`-count=3` run (both scale cells, all encodings, all reads, cold+warm) took
**141.0s** wall clock on the machine below -- under the 4-minute cap this
ticket allows for a tripled count -- so `symbolsPerFile`/`importsPerFile`/
`markersPerFile` were left at the ticket's stated ~8/~5 counts.

## Machine

- `uname -m`: `arm64`
- CPU (`sysctl -n machdep.cpu.brand_string`): `Apple M2 Max`
- `go env GOVERSION`: `go1.27.0`

## Corpus and disk footprint

Deterministic seed 42. Per file: 1 path, 1 blob hash, 8 symbols
(name + line), 5 imports, 3 document markers. Plus a shared 50,000-term
vocabulary table.

| files   | gob (.gob) | pack (data.bin + manifest.gob) | pack v2 (data.bin + manifest.v2) | sqlite (.db, indexed + VACUUMed) |
|---------|-----------:|--------------------------------:|---------------------------------:|----------------------------------:|
| 10,000  | 4.2 MB     | 7.6 MB                           | 5.8 MB (5,817,969 + 372 B)       | 13 MB                             |
| 100,000 | 38 MB      | 74 MB                            | 54 MB (54,326,432 + 372 B)       | 122 MB                            |

Pack and SQLite are larger on disk than gob mainly because of per-block/
per-row framing: pack gives every one of its ~400k+1 blocks its own
independent gob type descriptor (no shared dictionary), and SQLite pays
per-row and B-tree index overhead.

## Results (`-benchtime=3x -count=3`, 12 logical CPUs)

`ns/op` is reported as **min–max across the 3 independent `-count=3` runs**
(each run's own number is `go test -bench`'s mean over its 3 `-benchtime=3x`
iterations), not a single aggregate -- so the spread below is real run-to-run
variance, not noise hidden inside one number. `bytes/op`, `B/op`, and
`allocs/op` did not vary meaningfully across the 3 runs (well under 0.1%),
so one representative value is shown for each. **`B/op` is bytes allocated
by Go's allocator (`-benchmem`), not resident set size (RSS) -- RSS is not
measured anywhere in this experiment.**

`bytes/op` is bytes actually read from the encoding's file(s), through one
`CountingReaderAt` per operation for both gob and pack -- including, for
pack, the one-time manifest read at `OpenPack`, and for gob, the single
whole-file read every read pattern requires. Earlier measurements of this
experiment counted pack's manifest read outside the counter and gob's read
via raw file size, which understated pack's true bytes-touched for scoped
reads by orders of magnitude (see "one-path" below: 26MB now vs. a
previously reported 478B, because the 100k-file manifest itself is ~26MB of
gob-encoded block offsets/lengths/checksums). **SQLite's bytes/op is still
not reported** (`-`): `modernc.org/sqlite` exposes no pluggable byte-level
I/O hook, so only ns/op and `-benchmem`'s alloc counters are directly
comparable for it; see "What this experiment does NOT establish" below.

### files=10,000

| encoding | read      | mode | ns/op (min–max)          | bytes/op  | B/op        | allocs/op |
|----------|-----------|------|--------------------------:|----------:|------------:|----------:|
| gob      | whole     | cold | 12,391,278 – 12,952,625   | 4,367,741 | 21,664,170  | 330,316   |
| gob      | one-table | cold | 12,450,764 – 12,671,833   | 4,367,741 | 22,451,290  | 330,350   |
| gob      | one-table | warm | 253,792 – 290,639         | 4,367,741 | 787,072     | 33        |
| gob      | one-path  | cold | 12,205,055 – 12,256,653   | 4,367,741 | 21,664,160  | 330,316   |
| gob      | one-path  | warm | 333 – 528                 | 4,367,741 | 0           | 0         |
| sqlite   | whole     | cold | 84,939,944 – 85,968,639   | -         | 39,795,800  | 1,580,852 |
| sqlite   | whole     | warm | 88,115,208 – 91,408,333   | -         | 39,790,309  | 1,580,805 |
| sqlite   | one-table | cold | 17,970,680 – 18,197,555   | -         | 8,774,800   | 340,157   |
| sqlite   | one-table | warm | 17,898,070 – 18,244,292   | -         | 8,770,042   | 340,112   |
| sqlite   | one-path  | cold | 170,708 – 194,945         | -         | 9,437       | 209       |
| sqlite   | one-path  | warm | 73,847 – 88,750           | -         | 5,280       | 166       |
| pack     | whole     | cold | 273,848,333 – 281,777,945 | 8,006,927 | 261,777,938 | 5,520,501 |
| pack     | whole     | warm | 255,225,806 – 262,355,431 | 6,287,258 | 251,271,309 | 5,470,209 |
| pack     | one-table | cold | 88,376,903 – 89,975,653   | 3,209,503 | 81,453,701  | 1,640,325 |
| pack     | one-table | warm | 71,710,570 – 72,215,070   | 1,489,834 | 70,947,194  | 1,590,034 |
| pack     | one-path  | cold | 17,053,375 – 17,413,000   | 2,579,981 | 10,531,080  | 50,833    |
| pack     | one-path  | warm | 33,306 – 35,347           | 860,312   | 24,568      | 542       |

`gob whole/warm` is omitted: see "Warm-cell comparability" below.

### files=100,000

| encoding | read      | mode | ns/op (min–max)               | bytes/op   | B/op          | allocs/op  |
|----------|-----------|------|-------------------------------:|-----------:|--------------:|-----------:|
| gob      | whole     | cold | 133,076,139 – 174,990,472      | 39,823,423 | 268,851,242   | 2,850,544  |
| gob      | one-table | cold | 133,551,347 – 139,252,514      | 39,823,423 | 275,147,984   | 2,850,802  |
| gob      | one-table | warm | 4,257,236 – 8,692,181          | 39,823,423 | 6,296,704     | 257        |
| gob      | one-path  | cold | 129,023,611 – 136,723,458      | 39,823,423 | 268,851,226   | 2,850,544  |
| gob      | one-path  | warm | 514 – 625                      | 39,823,423 | 0             | 0          |
| sqlite   | whole     | cold | 809,368,833 – 813,448,000      | -          | 334,399,250   | 14,455,625 |
| sqlite   | whole     | warm | 810,623,292 – 823,277,222      | -          | 334,393,861   | 14,455,577 |
| sqlite   | one-table | cold | 185,946,764 – 195,100,361      | -          | 84,594,133    | 3,400,609  |
| sqlite   | one-table | warm | 182,385,861 – 185,536,167      | -          | 84,589,152    | 3,400,562  |
| sqlite   | one-path  | cold | 215,944 – 227,958              | -          | 9,288         | 210        |
| sqlite   | one-path  | warm | 94,500 – 100,514               | -          | 5,280         | 166        |
| pack     | whole     | cold | 2,743,146,764 – 2,796,049,986  | 76,518,171 | 2,646,608,456 | 54,750,961 |
| pack     | whole     | warm | 2,604,190,639 – 2,713,134,875  | 59,121,459 | 2,489,600,493 | 54,250,443 |
| pack     | one-table | cold | 904,688,514 – 943,232,320      | 32,395,068 | 864,904,773   | 16,400,776 |
| pack     | one-table | warm | 729,564,986 – 765,537,667      | 14,998,356 | 707,896,746   | 15,900,258 |
| pack     | one-path  | cold | 173,096,389 – 176,307,486      | 26,095,546 | 157,032,584   | 501,060    |
| pack     | one-path  | warm | 47,139 – 52,055                | 8,698,834  | 24,568        | 542        |

`gob whole/warm` is omitted: see "Warm-cell comparability" below.

"cold" opens a fresh reader/connection/file handle per op (fresh manifest
decode for pack, fresh `*sql.DB` for SQLite, a fresh whole-file read for
gob) **within one already-warm benchmark process**, so the OS page cache for
every file is warm going in; "warm" additionally reuses one already-open
reader/connection across the loop. Neither drops the real OS page cache (no
privileges available to do that here), so **both "cold" and "warm" here mean
warm-OS-cache** -- "cold" isolates the per-operation open/decode cost within
a warm process, "warm" additionally amortizes that cost across repeats. This
is not a true cold-disk (cold page cache) measurement.

### Warm-cell comparability

gob has no random access and no persistent reader to reuse -- every gob read
pattern is "read the whole file, decode the whole value, then optionally
project." A `whole/warm` cell for gob would therefore either (a) redo the
identical whole-file read+decode every iteration -- indistinguishable from
`whole/cold` once the OS cache is warm, so it would report nothing new -- or
(b) hold onto the already-decoded value and do zero work, which is what
earlier versions of this benchmark did and is not comparable to SQLite/pack's
warm cells, which genuinely re-query and re-decode against a reused open
handle. `gob whole/warm` is dropped from the tables above rather than
published as if it meant the same thing as the other two encodings' warm
cells. `gob one-table/warm` and `gob one-path/warm` are kept because they do
real per-iteration work (projecting a map, or indexing into it) against a
value decoded once outside the timed loop, which is closer in kind to what
SQLite/pack's warm cells measure.

## Correctness, corruption, and atomicity tests (all passing)

Pack v2 has a twin of each pack test below (`TestPackV2CorruptionRefused`,
`TestPackV2TruncationRefused`, `TestPackV2ManifestLastAtomicWrite`,
`TestPackV2RepublishFailureKeepsOldSnapshotReadable`), plus
`TestPackV2ManifestCorruptionRefused` (a flipped manifest byte fails with
`errChecksumMismatch`), `TestPackV2OversizedLengthRefusedBeforeAllocation`
(a re-checksummed manifest claiming a block over 16 MiB fails with
`errPackV2Malformed`), and `TestPackV2OnePathLookupFindsEveryPathAndRefusesAbsentOnes`
(3,000 paths, height-3 tree, absent keys). The round-trip test includes
pack v2. Disabling the checksum comparison and the length bound in
`readVerified` makes the corruption and oversized-length tests fail.

- `TestAllEncodingsRoundTripTheSameCorpus` -- writes one corpus through all
  three encodings, reads the full corpus back from each, and compares a
  canonical content digest (every record's fields, sorted, not path order or
  seed) across gob, SQLite, and pack.
- `TestPackCorruptionRefused` -- mutates one byte inside a checksummed
  block's trailing string content (not its leading gob type/length framing),
  confirms the mutated bytes still gob-decode cleanly on their own, then
  asserts the pack read fails with `errChecksumMismatch` specifically --
  not merely "some error" -- so removing the checksum check would be caught
  even though the corruption alone doesn't break gob's wire format
  (DNIP-IDX-005).
- `TestPackTruncationRefused` -- truncates the pack's data file to half its
  length; `ReadWhole` fails rather than returning a partial corpus.
- `TestPackManifestLastAtomicWrite` -- with only partial section data and no
  manifest, `OpenPack` returns `errNoSnapshot` ("no snapshot", never a
  partial read); after `BuildPack` completes, the `manifest.gob.tmp` file is
  gone (renamed into place) and a full read succeeds (DNIP-IDX-008).
- `TestPackRepublishFailureKeepsOldSnapshotReadable` -- after one successful
  `BuildPack`, simulates a crashed re-publish (new section data written
  under a new name, a new `manifest.gob.tmp` written but never renamed) and
  asserts the original manifest still resolves the original, untouched
  section file with the original content. This only holds because
  `BuildPack` now writes every publish's sections to a freshly named file
  (`os.CreateTemp`, never a reused fixed name) -- the earlier fixed
  `data.bin` name meant a re-publish's `os.Create` truncated the previous
  snapshot's sections before the new manifest was ever renamed into place,
  so a crashed re-publish could leave the old (still-published) manifest
  pointing at truncated data.

## Reading

Inconclusive on a production selection, and the three encodings do not
agree on a single winner across read patterns. Pack still reads far fewer
*data* bytes for scoped queries than gob's mandatory full-file read
(39.8MB at 100k for every gob pattern) -- but once its one-time manifest
read is counted (previously excluded, see "Results" above), a pack one-path
lookup at 100k reads 26.1MB cold, not 478 bytes, because the manifest itself
(~400k blocks' worth of path/offset/length/SHA-256 entries) dominates.
Pack's true scoped-read advantage over gob only shows up once amortized
across many reads against one already-open reader: `one-path/warm` reads
only 8.7MB total (manifest, counted once at `Open`, divided by `b.N`) and
runs in 47–52µs, and `one-table/warm` similarly drops to 15.0MB/op. A
single cold pack open still has to pay for decoding that whole manifest
before touching any requested content, which is also why pack is the
slowest of the three encodings on wall clock for every read pattern except
a warm, repeated one-path lookup: each of its ~400k blocks carries an
independent gob type descriptor with no shared dictionary, so a cold open's
manifest decode alone costs proportional to path count (order of 170ms at
100k, comparable to gob's whole-file decode), before "whole"/"one-table"
additionally pay that per-block framing overhead on every block touched --
pack's whole-read cold time (2.74–2.80s at 100k) is over 3x SQLite's
(809–813ms) and over 15x gob's (133–175ms), making **pack the slowest
encoding for a genuine whole-corpus load, not SQLite**. SQLite is the
strongest all-around performer for one-path point lookups (216–228µs cold,
95–101µs warm at 100k, using its own B-tree path index) and is a legitimate
DNIP-IDX-009 safety baseline, but its bytes-touched are not instrumented
here (see above). gob is fastest for a warmed one-path/one-table lookup (it
is, after all, already fully decoded in memory) but is the only encoding
that always reads and decodes the entire file regardless of what the caller
wants, which is exactly the ceiling docs/agent-memory/optimizations.md's
2026-09-04 entry describes. A fairer verdict on pack would need a version
that shares gob type descriptors across blocks (or drops gob for a flatter
per-block format, and shrinks the manifest itself) before its wall-clock and
bytes-read cost can be compared to SQLite on equal footing.

## Pack v2: O(sections) manifest (2026-09-12)

Layout (`packv2.go`). The data file (unique name per publish, as for pack
v1) holds, per section: a descriptor (the section encoder's encoding of a
fixed primer value, carrying the gob type descriptor once), then one value
message per path; then the vocabulary block; then a static tree. Leaf rows
are `(path zero-padded to the corpus's max path length, 4 x (offset u64,
length u32, SHA-256))`, 32 rows per page, sorted: the fixed-width path table.
Interior rows are `(first key of child page, child locator)`, 64 per page.
`manifest.v2` is fixed-size: magic, data file name, key width, height, root,
vocabulary and four descriptor locators, and a SHA-256 over all of it,
renamed into place last. A one-path lookup reads the manifest, the four
descriptors, one verified page per tree level (binary search within each
page, over `io.ReaderAt`), and the path's four blocks. Every length is
bounded before allocation and every range is checksum-verified before
decoding; the manifest's own checksum is verified before any locator in it
is used.

Byte accounting is confirmed by arithmetic, not only by the counter: at both
scale cells the key width is 20 and the tree has height 3, so the 10k and
100k one-path reads differ only in the root page's row count (5 vs 49 rows of
64 bytes); 44 x 64 = 2,816 = 14,388 - 11,572 bytes.

### One-path results, four encodings (`-benchtime=3x -count=5`)

Median and min–max of the five runs' `ns/op`. `bytes/op`, `B/op` and
`allocs/op` were identical across runs for gob and both packs (SQLite `B/op`
varied by under 10%). The run took 78 s wall clock.

**Load-contaminated.** The host has 12 logical CPUs; `uptime` load averages
were 44.25 (1 min) before the run, 56.86 after, and 42.48–55.13 (median
52.44) over five samples taken every 15 s during it -- load exceeded cores
throughout. Spreads of 2–10x inside one cell (e.g. pack v1 cold 100k:
321 ms – 1.18 s; SQLite warm at 100k almost equal to SQLite cold, 418 vs
432 µs median, where the 2026-09-04 run had warm under half of cold) are
that contamination, not codec behaviour. Treat latency
as order-of-magnitude only; `bytes/op` and `allocs/op` are deterministic and
not affected by load.

| files   | encoding | mode | ns/op median | ns/op min–max           | bytes/op   | B/op        | allocs/op |
|---------|----------|------|-------------:|------------------------:|-----------:|------------:|----------:|
| 10,000  | gob      | cold | 31,612,389   | 19,066,889 – 55,004,736 | 4,367,741  | 21,664,160  | 330,316   |
| 10,000  | gob      | warm | 1,042        | 611 – 1,097             | 4,367,741  | 0           | 0         |
| 10,000  | sqlite   | cold | 781,083      | 498,681 – 1,452,472     | -          | 9,362       | 210       |
| 10,000  | sqlite   | warm | 235,931      | 136,417 – 334,625       | -          | 5,280       | 166       |
| 10,000  | pack v1  | cold | 107,444,431  | 30,128,861 – 124,324,736| 2,579,981  | 10,531,080  | 50,833    |
| 10,000  | pack v1  | warm | 165,264      | 83,875 – 716,778        | 860,312    | 24,568      | 542       |
| 10,000  | pack v2  | cold | 190,708      | 115,820 – 305,736       | 11,572     | 54,688      | 594       |
| 10,000  | pack v2  | warm | 136,056      | 67,292 – 163,500        | 11,195     | 52,960      | 578       |
| 100,000 | gob      | cold | 412,632,472  | 333,592,597 – 596,066,417 | 39,823,423 | 268,851,285 | 2,850,544 |
| 100,000 | gob      | warm | 986          | 694 – 42,361            | 39,823,423 | 0           | 0         |
| 100,000 | sqlite   | cold | 432,361      | 384,764 – 700,986       | -          | 9,069       | 209       |
| 100,000 | sqlite   | warm | 417,875      | 329,069 – 589,125       | -          | 5,285       | 166       |
| 100,000 | pack v1  | cold | 848,464,945  | 321,273,167 – 1,179,535,542 | 26,095,545 | 157,032,600 | 501,060 |
| 100,000 | pack v1  | warm | 133,792      | 54,930 – 1,065,389      | 8,698,834  | 24,568      | 542       |
| 100,000 | pack v2  | cold | 181,486      | 144,445 – 207,944       | 14,388     | 57,568      | 594       |
| 100,000 | pack v2  | warm | 86,431       | 67,111 – 142,667        | 14,011     | 55,840      | 578       |

Warm `bytes/op` for both packs is total bytes since `Open` divided by `b.N`
(3), so it still carries a third of the open-time read. Pack v2's cold and
warm differ by only ~377 bytes because its open-time read (manifest plus four
descriptors) is a few hundred bytes; per lookup it reads ~14 KB of tree pages
and blocks.

### Reading (pack v2)

The stated experiment is done: a cold one-path lookup at 100k now reads
14,388 bytes, not 26,095,545 (1,814x fewer), and its bytes grow with tree
height rather than with path count. Cold one-path latency drops from
hundreds of milliseconds (pack v1, dominated by decoding the manifest) to
~0.2 ms, in the same order as SQLite's B-tree point lookup on this run.
Pack v2's lower medians than SQLite here are **not** a win under
DNIP-IDX-009: the run is load-contaminated, SQLite bytes-read is still not
instrumented, and a one-path point lookup is one read pattern. Pack v2
also removes one gob type descriptor per block: its total footprint is 29%
smaller than pack v1's at 100k (54.3 MB vs 76.5 MB) even though ~20 MB of it
is the fixed-width leaf path table. Pack v2's `whole` and `one-table` cells
exist in the harness but were NOT_RUN in this rerun; its per-block decode
replays the section descriptor into a fresh gob decoder, so those cells may
be slower per block than their size suggests.

## What this experiment does NOT establish

- **Not the real `Index` type.** The corpus is a synthetic approximation
  (path, blob hash, 8 symbols, 5 imports, 3 markers, a 50k-term vocabulary)
  of `internal/contextindex.Index`'s shape, not that struct itself -- this
  module cannot import `internal/contextindex`.
- **No ranking.** Nothing here touches search, scoring, or query
  relevance -- only raw table/record retrieval.
- **No million-file cell.** Only `files=10_000` and `files=100_000` were
  measured, as declared; the roadmap's own note that "small corpora cannot
  satisfy million-file gates" applies directly.
- **No production selection.** AT-04's ticket text is explicit that
  production-format work is specified only after a selection is made from
  accepted evidence; this experiment supplies evidence, not a decision, and
  the pack implementation here is a first-cut, not a candidate optimized
  for production adoption.
- **SQLite bytes-read is not instrumented.** `modernc.org/sqlite` does not
  expose a pluggable `ReaderAt`/byte-counting hook, so its rows in the
  results tables above show `-` in the `bytes/op` column; only ns/op and
  `-benchmem`'s allocation counters are directly comparable for it.
- **Not a crash-injection test.** The atomicity tests simulate a crash by
  constructing partial section/manifest files by hand (both a first publish
  interrupted before any manifest exists, and a re-publish interrupted
  before its new manifest is renamed into place); they do not kill a real
  writer process mid-write.
- **No DNIP selection.** Pack v2 changes no gate status on its own. Still
  owed before any pack-versus-SQLite selection: instrumented SQLite
  bytes-read; RSS; an unloaded host; >=100 fresh-process samples with
  p50/p95/p99 (DNIP-XM-003); a real cold page cache; a million-file cell;
  the real `Index` shape and the current `.aip` format on actual receipts
  (the storage audit's AT-04 step 1 -- this benchmark pack is a different
  format); whole/one-table cells for pack v2; real crash injection; and a
  manifest digest pinned by an external root or receipt.
- **Pack v2's manifest is self-checksummed, not externally bound.** Its
  trailing SHA-256 detects a corrupted manifest and binds every page and
  block below it, but nothing outside the pack pins that digest, so a
  wholesale substituted pack v2 is not detected. The two bullets below
  apply to pack v1 only; pack v2 bounds every length before allocating
  (`TestPackV2OversizedLengthRefusedBeforeAllocation`).
- **No Merkle-root binding of the manifest.** The pack manifest is trusted
  as-is once its own bytes are read; nothing here binds it to a
  content-addressed root the way DNIP-IDX-004..006's design intent expects,
  so a manifest that is itself corrupted or substituted (as opposed to a
  data block) is not detected by this prototype.
- **Manifest lengths are not bounded before allocation.** `OpenPack`
  allocates a buffer sized by the manifest file's on-disk length, and
  `readBlock` allocates a buffer sized by each block's `Length` field, with
  no upper bound checked against either before allocating -- a corrupted or
  adversarial manifest could request an arbitrarily large allocation before
  any checksum is ever verified. Combined with the missing Merkle-root
  binding above, DNIP-IDX-004..006 are not satisfied by this prototype as
  it stands.
