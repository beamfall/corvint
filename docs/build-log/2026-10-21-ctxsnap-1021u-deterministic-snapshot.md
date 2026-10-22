# 2026-10-21 ctxsnap-1021u: deterministic snapshot bytes and output-equivalent schema edits

Tickets: V1-2290 (delivered), V1-2289 (delivered), V1-2232 (not delivered, measured). Requirements
`IDX-SNAP-V0-106` and `IDX-SNAP-V0-107` in `docs/specs/index-snapshot-v0.md`, accepted under the
owner's standing auto-accept (2026-10-11). Base `origin/main` 9ae946a5c.

## Problem

- V1-2290: `encoding/gob` writes maps in random order. Three `corvint index` runs of one tree with
  the base binary wrote three different files (sha256 prefixes `94e688cb`, `08f7acf1`, `a402d5a1`),
  so a file hash could not identify content and any content-addressed reuse was impossible.
- V1-2289: `TestAnalyzerSchemaInputs` pins a digest of every non-test analyzer input, so a
  perf-only edit forced a schema bump and invalidated every pack even when output was unchanged.
- V1-2232: every re-index writes the full snapshot.

## Decisions

- Minimal mechanism for V1-2290: every map in the gob, sectioned and pack encodings is written as
  key-sorted pairs (`internal/contextindex/snapshot_wire.go`), including nested maps inside record
  fields (`wireAnyMap`). A `Present` bit keeps nil and empty maps distinct. `warmGobTypes` encodes
  sample values at init so gob type ids do not depend on the first value a process encodes. No new
  infrastructure, no new file layout. The gob format string moves to `corvint-index-snapshot/2`; a
  `/1` file is a format mismatch and rebuilds (the gob snapshot is executable-keyed, so no binary
  reads both). The pack sections change bytes, so the analyzer schema moves 140 -> 147 (141 to 146
  are reserved by other lanes).
- V1-2289: an output pin, not a weaker input rule. `TestAnalyzerSchemaOutput` hashes the pack a
  fixed golden corpus compiles to; `script/analyzer-schema.sh` reads that pin, `bump` runs the test
  to get the real output digest and keeps REF's number when it equals REF's, and `check` accepts
  REF's number with another input digest only when the output pins agree. The Go test stays the
  authority that the pinned output digest is real; the script never trusts an unverified pin
  except through CI running that test.
- V1-2232: skipped. Deterministic bytes are the precondition for a delta or content-addressed
  segment format, but that format is a new encoding needing its own design and the
  deployment-neutral gate; it does not fit this lane's budget.

## Evidence

- `TestSnapshotBytesAreDeterministic` (IDX-SNAP-V0-106): fails on base ("build 1 wrote different
  bytes: gob equal=false pack equal=true"), passes with the change.
- `TestSnapshotWireRoundTripsMaps`: eight encodes byte-identical, DeepEqual round trip, nil vs empty
  preserved, malformed pairs refused.
- `TestAnalyzerSchemaOutput` (IDX-SNAP-V0-107): pinned `bf311663...`; stable across processes. A
  temporary mutation of the snapshot format string made it fail with the bump recovery.
- `script/analyzer-schema_test.sh`: new cases (perf-only edit keeps the number, REF's number with
  another output digest refused, output change bumps once and repins, missing output digest
  refuses) pass; the base script fails at "perf-only edit bumped corvint-analyzer/9 ->
  corvint-analyzer/10".
- Format gate on this repository (host load average 24 to 40, so timings are noisy), base binary
  -> this change:

  | Measure | Base | This change |
  | --- | --- | --- |
  | snapshot bytes | 182,633,829 | 182,598,748 |
  | cold `index`, 3 runs, median | 2.85 s | 3.12 s |
  | cold max RSS range | 1,396-1,538 MB | 1,366-1,564 MB |
  | reuse `index`, median | 1.85 s | 1.91 s |
  | 20 `context` loads p50 / p95 | 0.46 s / 0.57 s | 0.49 s / 0.67 s |
  | `context` max RSS | 912 MB | 914 MB |
  | sha256 of 3 writes of one tree | 3 different | `c268c468...` x3 |

  Cross-format identity: canonical JSON (`IDX-SNAP-V0-022` form) of the index decoded from the base
  `/1` gob, the new `/2` gob and the new pack: one sha256 `554e18859f8d...` (230,890,448 bytes).
  The decoded form differs from the in-memory build by `[]` versus `null` for empty slices nested
  in record fields; the base binary shows the identical difference, so it is pre-existing gob
  behaviour, not a regression.
- V1-2232 measurement (before and after): a one-file commit re-index wrote 182,633,830 bytes with
  the base binary and 182,598,749 bytes with this change; reuse cut nothing from the write.

## NOT_RUN

- `make gate` (owner preference; batch CI runs it). Tier 2 of `corvint affected` is left to batch CI.
- Blob shards (`IDX-SNAP-V0-016`, off by default) are not in the golden pack.

## Residuals

- Incremental or content-addressed snapshot segments for V1-2232.
- Golden corpus coverage: no markers are produced by the corpus; facts it does not exercise can
  change without moving the output digest.

## Rollback

Revert the commit. Files written as `/2` then miss and rebuild under `/1`; packs written at
`corvint-analyzer/147` miss under the reverted schema id. The output pin and its script handling
can be removed alone: the script then behaves as `IDX-SNAP-V0-060`/`061`.
