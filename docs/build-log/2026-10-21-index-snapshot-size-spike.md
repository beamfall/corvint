# Index snapshot size: measured composition and a 1.0 format-gate decision (V1-2140)

Date: 2026-10-21. Base: `5cf75daf0` (`origin/main`). Requirements: none new. `IDX-SNAP-V0-007`
(entry and 1 GiB byte bound) and the invariant-7 benchmark/format gate are unchanged.

## Question

V1-2140 reports that one gob snapshot of the Corvint tree is about 187 MB. It asks for a measured
design for a smaller encoding with a format-gate decision, or for a recorded decision that the
current size stands for 1.0 with its bound. The ticket is format-gated, so this lane wrote the
spike and the decision only. The snapshot encoding is unchanged.

## Measurement

Subject: `.git/corvint/index/sha1-39f5c02...-a7211d4bcfa2e3bd.gob`, the published snapshot of
tree `39f5c02bc`. It holds 11,417 tracked paths and 8,876 indexed sources. A scratch test decoded
the file and re-encoded each `Index` member alone with `encoding/gob`, which gives the
member's share of the stream. Host: Darwin arm64, go1.27.2, one run on a quiet host. The test
was deleted and never committed.

| Member | Bytes | Share |
|---|---:|---:|
| whole file (header, index, sha256 trailer) | 170,328,336 | 100% |
| source bodies (`Sources[].Content`) | 110,242,187 | 64.7% |
| vocabulary (all members below) | 49,424,731 | 29.0% |
| - `Terms` postings | 24,413,714 | |
| - `Words` | 18,051,832 | |
| - `SymbolWindows` | 4,157,030 | |
| - `IdentGraph` | 2,010,671 | |
| - `Paths`, `PathTerms` | 791,509 | |
| symbols | 6,615,590 | 3.9% |
| source metadata without bodies | 1,378,229 | 0.8% |
| documents | 1,249,212 | 0.7% |
| imports | 716,846 | 0.4% |
| tracked list | 551,393 | 0.3% |
| exclusions | 111,758 | 0.1% |

The largest 20 bodies total 12.9 MB. Decoding the whole file took 99 ms.

Generic compression (`compress/flate`) over the same bytes:

| Input | Level | Output | Ratio | Encode time |
|---|---|---:|---:|---:|
| whole file | 1 | 58,980,992 | 34.6% | 643 ms |
| whole file | default | 50,571,674 | 29.7% | 1.32 s |
| sources only | 1 | 32.8 MB of 111.7 MB | 29% | not timed |
| vocabulary only | 1 | 24.5 MB of 49.4 MB | 50% | not timed |

The Git object store already holds the bodies. The whole repository packs to 77.80 MiB
(`size-pack`). One `git cat-file --batch` pass over all 11,417 tree blobs (130,674,009 bytes)
took about 0.24 s.

## Candidate designs

The bodies are byte copies of immutable Git blobs, and each source already records its
`BlobHash` (invariant 1). That explains most of the size.

- **(a) Git-backed bodies.** Store `BlobHash` only and read bodies through one `git cat-file
  --batch` process when a reader needs them. This removes about 65% of the bytes (about 110 MB
  here) without changing what ranking sees. Costs: every read verb gains a Git subprocess and a
  failure mode (object unavailable, partial clone), and the 99 ms load path gets a measured
  read-latency regression to bound. It also changes what `context` can serve when the object
  store is pruned, so it needs an abstention rule.
- **(b) Compressed or delta-encoded postings.** Deflate halves the vocabulary (about 25 MB saved).
  Varint delta postings in the existing opt-in sectioned layout (`IDX-SNAP-V0-014`) would likely
  do better without a whole-file inflate. This is an inference; it was not measured here.
- **(c) Whole-file compression.** This cuts the file to 30–35% of its size, but it adds 0.6–1.3 s
  to every write, an inflate on every load, and it defeats the mmap path. It is rejected as the
  primary design.

## Decision

For 1.0, the current size stands with its existing bound. That is the conservative option, decided
under the delegated in-task authority (owner, 2026-10-05). `IDX-SNAP-V0-007` keeps at most eight
published snapshots and at most 1 GiB of `*.gob` per store, and the shared store serves every
linked worktree (`DIRTY-CACHE-013`). So the worst case on this repository is about six snapshots
(about 1 GiB), not unbounded growth. The receipt names every eviction (`IDX-SNAP-V0-025`).

Design (a), followed by (b), is the recommended post-1.0 candidate. Before promotion it needs the
invariant-7 benchmark/format gate:
- a format-version bump;
- byte-identical receipts on the pinned corpora;
- measured load and read latency against the 99 ms decode baseline;
- an abstention rule for unavailable objects.

No encoding, requirement, fixture or analyzer input changed.

Rollback: none needed; no behavior changed.
