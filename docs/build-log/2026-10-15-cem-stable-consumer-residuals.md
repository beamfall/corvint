# cem/1.0 stable consumer residuals: drift charge, protocol pointers, three decisions

Date: 2026-10-15. Base: `253d6341f22242a1778876649708222a4a5bc33c` (origin/main).
Tickets: V1-1295, V1-1296, V1-1297, V1-1299, V1-1307 (`docs/specs/cem-stable-v1.md`,
`docs/specs/daily-change-evidence-workflow-v0.md`, decision 0531).

New requirements `CEM-V1-240`, `CEM-V1-241` and `DCW-V0-069` are accepted under the owner's standing
auto-accept of 2026-10-11. The choices for V1-1297, V1-1299 and V1-1307 were delegated to the lane.
No accepted requirement's behaviour changes; the three amendments that would are listed below as
owner decisions.

## V1-1295: the two consumers cut drift short at different rows

Both consumers keep the partial drift list in an `UNSUPPORTED` result (the 2026-10-14 V1-1295
decision). They differed in the drift stage's per-record Git charge. The native verifier read each
changed record's base blob again before reading the target blob, three logical operations per
record. The portable consumer searched the span bytes the evidence stage had authenticated, two
operations. Under the 1,024-operation limit the native verifier therefore stopped earlier.

`CEM-V1-240` pins the portable charge: one target path lookup per record, one target blob read only
for a changed regular-file entry, and no base blob read. The native `checkDrift` now takes the spans
`checkEvidence` returns, and a record whose authenticated span length differs from its declared span
fails closed `evidence-unavailable`. `ALGORITHMS.md` item 12 states the charge.

The shared case `protocol/cem-1.0/stable/drift-resource-limit/` (13 hunks, 26 `spec.txt` records,
every cited span moved in the target) declares a lowered limit of 113 operations. Each consumer applies
it only through an in-package test seam (`stableSeam.operations`, `stableHooks.callLimit`). The
production limit stays 1,024. A full-limit case was tried first: 340 records spent the 1,024
operations. The native test took 845 s and passed (load 55 to 71). The portable consumer reached the
30-minute verification deadline at 119 rows, `verification-timeout`, at load 65 to 76. That is too
slow for a test.

| Run | Native | Portable | Load (1/5/15 min) |
| --- | --- | --- | --- |
| f0fda542 map, BASE | 68 rows | 102 rows, `92747a60…` | about 62 to 67 |
| f0fda542 map, fixed | 102 rows, byte-identical to portable | 102 rows | about 62 to 67 |
| 340-record map, 1,024 ops, BASE | 103 rows, `ce14cecf…` | not comparable (timeout) | 65 to 76 |
| 340-record map, 1,024 ops, fixed | 154 rows, `c1735ec8…` | `verification-timeout` at 119 rows | 65 to 76 |
| shared case, 113 ops, fixed | 13 rows, 89.6 s | 13 rows, 156.9 s, byte-identical, `a5611d4d…` | 63.35 59.31 62.80 to 61.77 59.96 62.02 |
| shared case, base re-read restored | 8 rows, test FAIL | n/a | 57.57 58.99 61.51 |

The case's operation ledger on the portable consumer has 86 operations before drift (52 in the
evidence stage), one drift target tree read, and two per record after that. With the limit at 113,
13 records complete and the fourteenth record's lookup, operation 114, is refused.

## V1-1296: dangling protocol pointer

`CEM-V1-241` adds a pointer scan to `tools/cem-stable-regen --check`. A string member `K` with a
sibling `K`+`SHA256`/`Sha256`, or a `path` starting `../` beside `sha256`, under `protocol/cem-1.0`,
must resolve inside the repository to a file with that digest. CEM maps are skipped. The scan found
three dangling pointers, all in sealed, digest-pinned packet bytes that already mark them
informative:

- `stable/MIGRATION.json#/parentPlan`
- the same member in `repository-envelope-packet/inherited/s0e/MIGRATION.json`
- `repository-envelope-packet/INPUTS.json#/repair6Routing/sourceReview/path`

They are recorded by exact text; a record that resolves, changes or disappears is refused.
Removing them rewrites sealed bytes and re-pins two manifests and `INPUTS.json`, so that is left to
the owner. `--check` passes with the record and reports exactly three problems without it. With the
gate call removed, `TestCEMV1241PointerProblems` and `TestCEMV1241RepositoryPointersRecorded` fail.

## V1-1297 and V1-1299: kept as recorded

- V1-1297: a missing pinned object keeps `git-read-failed`. The sealed S0E cells `missing-base` and
  `shallow-missing` pin that code on both consumers. A distinct code needs an object-existence probe,
  which changes the sealed operation ledgers. `CEM-V1-103` cites decision 0531.
- V1-1299: no `CEM-V1-101` refusal code. The converter was removed (`CEM-V1-204`), so the code would
  be unreachable. The `CEM-V1-104` recorded limit stays and cites decision 0531.

## V1-1307: refused base `cem status` read

`DCW-V0-069` documents the current output. The base digest is the digest of the base verifier's
stdout whatever its exit status, so a refused read records the empty-output digest `sha256:e3b0c442…`.
A successful `cem status` always writes its canonical line, so that digest never stands for a
successful empty read. `TestCheckDCWV0069RefusedBaseStatusDigest` pins the
`base-independence=NOT_PROVED base=sha256:e3b0… tree=… declared=DCW-V0-069` line. Hashing stdout
and stderr together instead makes it fail.

## Owner decisions proposed (not applied)

1. Remove the three recorded dangling pointers. Rewrite the two `MIGRATION.json` files and
   `INPUTS.json`, then re-pin the stable manifest, the packet manifest and
   `INPUTS.json` `originalS0EPublicFiles`.
2. Give a missing pinned object its own repository code (amends `CEM-V1-103`). Both consumers probe
   object existence, and the sealed cells `missing-base` and `shallow-missing` and their ledgers are
   regenerated.
3. Amend `DCW-V0-047` so a refused base read records `NOT_PRODUCED` and its refusal code instead of
   the base digest.

## Verification

Focused packages from `corvint affected --base 253d6341f`: `internal/cem/verify`, `internal/cem/...`,
`internal/dogfoodflow`, `tools/cem-stable-regen`, and `interop/cem01-go` with its own `go test` and
`go vet`. Doc gates as in the CI list. `make gate` is `NOT_RUN` (owner preference for scoped issue
work).

## Review

One Codex round; findings below.
