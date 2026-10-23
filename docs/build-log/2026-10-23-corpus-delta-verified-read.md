# Verified delta reads and the unchanged-pair delta budget for GitHub #1634

Date: 2026-10-23. Lane: opencode/corpus-delta-fast-gh1634, based on origin/main c652d1639.
Requirements `DCP-V1-460..463` in `docs/specs/documentation-corpus-v1.md` and `PRB-V0-050..052` in
`docs/specs/performance-profiling-and-read-budget-v0.md` are proposed and experimental.

## Cause

`docs corpus delta --artifact` opened its current side with `doccorpus.OpenDeltaCurrent`, which
called `Open` and so `Build` on every run: a full rederivation that #1609's verified read
(`OpenRead`) had removed from every other read. The previous side used the serial closed decoder
(`ParseArtifact`). `delta` admitted neither `--timings` nor `--verify-rederive`.

## Decisions

- `DCP-V1-460`: the current side goes through `OpenDeltaRead`, which is `OpenRead` (verified by
  default, rederived with `--verify-rederive` or an unattested builder) unless a declared
  additional checkout is unavailable. In that case the `DCP-V1-425` path runs as before. That
  checkout observation runs as `OpenRead`'s header callback, so it overlaps the record decode. The
  branch is chosen from the checkouts alone. A failed verified read is never retried on the
  unverified path. `OpenDeltaCurrent` stays as the rederiving form. Independent review found
  that the rederiving branch (also taken by an unattested build) examined the checkouts of an
  unvalidated manifest, and could then admit it through the `DCP-V1-425` path. The verified branch
  refused the same bytes. The rederiving branch now validates the manifest first.
  `TestDCPV1460DeltaVerifiedReadUnavailableCheckout` fails without the fix. It also covers the
  `DCP-V1-425` path reached from the verified branch, which no earlier test did.
- `DCP-V1-461`: the issue asked for the previous side to use the same verified path. Its
  builder and pin checks would contradict `DCP-V1-424`: a previous corpus built by another builder
  must still give the same worklist, and today such a delta succeeds. So the previous side uses
  only the verified read's decode (`decodeArtifactParallel` plus the digest), which admits exactly
  what `ParseArtifact` admits. It is not rederived, even with `--verify-rederive`, which would need
  the same builder. This is a deliberate narrowing of the issue's wording, kept visible in the
  spec and pull request.
- While measuring, `decodeArtifactParallel(data, nil)` refused every schema `/2` artifact over
  4 MiB with `output bound exceeded`. Without a header callback, the header members (schema
  included) were deferred until after the bound was computed, so the schema-dependent bound fell
  back to `MaxBytes`. Only tests had called it that way before. It now always decodes the header
  members during the scan. `TestDCPV1461PreviousOverDefaultBound` fails without the fix.
- `DCP-V1-462`: the timings line is the `DCP-V1-415` line plus `compare_ms`, written only for a
  delta, so the `info` line is unchanged. `DeltaOptions.Timings` carries the comparison phase, and
  the stream's encode phase, out of `doccorpus`.
- `doccorpus.BuildsForTest` counts `build` calls. `TestDCPV1460DeltaCLIVerifiedRead` uses it to
  prove that the default delta path runs no build. With only the CLI's `OpenDeltaRead` call
  reverted to `OpenDeltaCurrent`, that test fails with `[--limit 256]: delta ran 1 builds on the
  verified path`.
- `PRB-V0-050..052`: a seventh budget scenario, `unchanged-delta`, is placed after the six
  #1602 scenarios so the edit is additive next to open PR #1612. Its wall ceiling is 2x its
  baseline p95, not the 3x of `PRB-V0-032`. With 3x the ceiling would be 8 s, above most of the
  measured rederiving deltas (6.0 to 7.8 s), so it would not catch a bypass. Its baseline entry was recorded separately
  from the other six; the next `--record-baseline` run replaces both.

## Measurement

Host: darwin/arm64, 12 cores, go1.27.2. The PRB-V0-021 full fixture (10,000 inputs, 96,350
records, 97,045,592-byte current artifact, 3 additional repositories) was a clone of an existing
generator `/1` fixture. "Before" is origin/main c652d1639 (`git archive`, builder `unrecorded`).
"After" is a86d55627, built from a clean checkout, so the builder is attested. The later review
repair only adds a manifest validation to the rederiving branch, so the default path measured
here is unchanged. Each binary built
its own artifacts. Wall time and peak RSS are from `/usr/bin/time -l`. These numbers are host
diagnostics; the issue's 15.7 s host figure was not reproduced or claimed.

Manual runs (3 each, 1-minute load 7 to 14):

| Command | Before | After (verified) | After `--verify-rederive` |
|---|---|---|---|
| unchanged pair, `--format jsonl` (33,001 lines, 35 MB) | 6.13 to 7.84 s, 0.99 to 1.08 GB | 1.58 to 1.99 s, 0.68 to 0.72 GB | 6.32 to 7.01 s, 1.04 to 1.07 GB |
| changed pair, `--limit 256` | 5.91 to 7.31 s (one 19.99 s outlier), 0.96 to 1.06 GB | 1.52 to 2.44 s, 0.61 to 0.71 GB | 6.14 to 7.45 s, 0.99 to 1.02 GB |
| changed pair, `--format jsonl` (33,351 lines) | not run | 1.58 to 1.73 s | 5.95 to 6.29 s |

`delta --timings` on the unchanged pair, after: `validation=artifact-verified parse_ms=446
verify_ms=124..173 encode_ms=72..92 compare_ms=1224..1269 total_ms=1546..1601`. With
`--verify-rederive`: `validation=source-rederived verify_ms=4491..4841 compare_ms=1238..1550
total_ms=6281..6965`. Classification (`compare_ms`) is now most of the remaining time.

Equality: the verified and `--verify-rederive` stdout have the same SHA-256 in every row above.
The before and after jsonl entry lines (all lines after the header) are byte-identical. The
headers differ only in the builder revision and the artifact digests, because each binary built
its own artifacts.

Harness (`perf-budget --fixture … --corvint …`, budgets as committed, 3 samples):

| Run | unchanged-delta samples | p95 | Verdict |
|---|---|---|---|
| baseline, after a86d55627, 2026-10-23T15:39:25Z, load 7.0/11.5/16.8 | 2510.30, 1520.91, 1488.30 ms | 2510.30 ms, 730 MB RSS | recorded |
| before c652d1639, 15:42:03Z, load 12.6 (contended) | 7640.19, 5998.30, 6683.78 ms | 7640.19 ms | `exceeded` (ceiling 5500 ms) |
| after a86d55627, 15:44:48Z, load 8.2 | 2510.87, 1536.33, 1519.60 ms | 2510.87 ms | `pass` |

In the same harness runs, `full-delta` went from p95 6616.63 ms to 1645.79 ms. The MCP scenarios
returned `corpus-refused` in these ad hoc runs, for the unmodified origin/main binaries as well.
Their binaries were not built by `make perf-budget` from one tree, and the cause was not
investigated; this change does not touch the MCP server.

## Tests

- `go test -count=1 -timeout 30m ./internal/doccorpus`: ok (519 s).
- `go test -count=1 -race -run 'Delta|DCPV141|DCPV1460' ./internal/doccorpus`: ok; after the
  review repair, `-race -run 'TestDCPV146|TestDCPV1425'`: ok.
- `go test -count=1 -run 'TestDCPV1460|TestDCPV1360DeltaCLI|TestDCPV1421|TestDCPV1425|TestDocsCorpusOperationFlagMatrix|TestDocsCorpusOperationHelpPrintsFlags|TestDCPV1413|TestDocsCorpusHelp' ./cmd/corvint`: ok.
- `go test -count=1 -timeout 30m ./internal/perfbudget/...`: ok, including the smoke harness run.

## Second review repair

Independent review of 8c9a8dbd8 found two issues.

- P1, `DCP-V1-464`: with every checkout available, the verified current side checked only the
  pins, tree and digests, never each input's `(revision, path) -> blob` binding. A current artifact
  whose changed input `src/report.py` named the previous blob and SHA-256, with its manifest and
  content digests recomputed, was admitted and classified `carry-forward` ("anchored blob is
  unchanged at the current revision"); `--verify-rederive` refused it. `OpenDeltaRead` now runs
  `verifyAvailableInputs` (the `DCP-V1-425` batched per-revision path walk: path present, regular
  file, blob ID, primary identity; each additional repository's identity is already checked at
  every pin by `unavailableCheckouts`) in the header callback when no checkout is missing, so it
  overlaps the record decode, and refuses before classification. `TestDCPV1464DeltaVerifiedReadChecksInputBindings`
  failed on 8c9a8dbd8 (`verified delta current side admitted a swapped input blob: <nil>`) and
  passes now. A recorded input SHA-256 is still not rehashed from its blob; only
  `--verify-rederive` catches an artifact whose SHA-256 alone is wrong.
- P3, `PRB-V0-053`: the `unchanged-delta` baseline samples sat under a baseline whose revision,
  time, host and `fixture.builder_sha256` belong to the 03:08 dd0d09db run. A full
  `--record-baseline` regeneration was not done: on this host the two MCP scenarios return
  `corpus-refused` for binaries passed with `--corvint` (unchanged from the first measurement, cause
  not investigated), and `--record-baseline` refuses a run with an unmeasured scenario; it would
  also re-derive the six existing ceilings. Instead each baseline measurement may carry
  `provenance` (start time, the measured executable's clean revision and SHA-256, host
  observation), validated on load. The `unchanged-delta` entry now names its own run from that
  run's report: 2026-10-23T15:39:25Z, executable a86d55627 (`vcs.modified=false`, SHA-256
  `9763911a…3fde1`, which also built the fixture artifacts it read), load 7.01/11.49/16.8,
  uncontended. The harness labelled that run `a86d55627…+dirty` because it ran from a dirty
  checkout with `--corvint`; the executable's embedded revision is recorded instead.
- `TestPRBV0050UnchangedDeltaBudget` cannot detect a restored rederivation, because its rederiving
  measurements are constants. The spec now names the build counter of
  `TestDCPV1460DeltaCLIVerifiedRead` as the deterministic guard.

Unchanged-pair `--format jsonl --timings` on the same full fixture, each binary built from a clean
checkout and building its own artifact, runs alternated, 1-minute load 5.8 to 6.1:

| Binary | verify_ms | compare_ms | total_ms | wall | peak memory |
|---|---|---|---|---|---|
| before repair, 8c9a8dbd8 | 135.7, 138.1, 208.7 | 1171.4, 1352.9, 2272.9 | 1510.5, 1704.2, 2769.5 | 1.54, 1.74, 2.87 s | 680 to 702 MB |
| after repair, a374f21f9 (also merges origin/main) | 334.7, 350.1, 362.7 | 1146.9, 1195.7, 1207.7 | 1662.6, 1746.5, 1752.2 | 1.69, 1.78, 1.82 s | 665 to 684 MB |

The binding check adds about 200 ms to `verify_ms`, partly hidden behind the record decode; the
median wall time went from 1.74 s to 1.78 s, under the 5,500 ms `unchanged-delta` ceiling. The six
outputs' entry lines are byte-identical. The committed baseline and ceilings were not re-recorded.

Repair tests: `go test -count=1 -timeout 30m ./internal/doccorpus`: ok (405 s);
`go test -count=1 -run 'TestPRBV005|TestPRBV003' ./internal/perfbudget`: ok.
