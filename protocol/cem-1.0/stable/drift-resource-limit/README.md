# Drift resource limit (CEM-V1-260)

Status: like the rest of `protocol/cem-1.0/stable/`, implemented and run by both
consumers; the technical contract is proposed and not promoted.

One `cem/1.0` map whose verification runs out of logical Git operations
(`ALGORITHMS.md` item 11) part way through the drift stage, with the one result
every stable consumer must write. The result is `UNSUPPORTED` at the `repository`
stage with code `unsupported-resource-limit`, and it keeps the drift rows completed
before the refused operation. Which rows those are depends only on the per-operation
charges before and during drift, in particular the per-record drift charge of
`ALGORITHMS.md` item 12 (`CEM-V1-260`): one target path lookup per record, plus one
target blob read for a changed regular-file entry, and no second read of a base
blob. A consumer that charges differently stops at a different row and writes a
different `drift` list and `verdictSha256`, which is how V1-1295 was found (68
against 102 rows on the f0fda542 map).

The case runs with the operation limit lowered from 1,024 to the `operations`
value in `EXPECTED.json` (113), through each consumer's in-package test seam, so
that it stays a few minutes long on a loaded host; spending the full 1,024
operations took 14 minutes natively and passed the 30-minute verification
deadline on the portable consumer under the same load. The protocol limit is
unchanged: the seam only moves the point at which the same accounting refuses.
With 113 operations, operations 1 to 86 precede the drift stage (52 of them the
evidence stage's two per record), operation 87 is the drift stage's target tree
read, operations 88 to 113 complete 13 records at two each, and operation 114,
the fourteenth record's path lookup, is refused. A consumer that read each base
blob again would complete 8 records. The native verifier
(`internal/cem/verify`) and the portable consumer (`interop/cem01-go`) both run
this directory; the tests are named `TestCEMV1260*` in both.

- `repository.recipe.json`: two flat commits, built exactly like
  `../neutral-criterion/repository.recipe.json`. The base `spec.txt` holds 340
  requirement lines; the target inserts one line before them, so every cited
  span moves and its record reads the changed target blob. Twelve `partNN.txt`
  files change one line each.
- `stable.json`: the map, 13 hunks and 26 evidence records, every record a
  single `spec.txt` line. Its producer-only arrays are empty, so it declares no
  artifacts.
- `EXPECTED.json`: profile `cem-stable-drift-resource-limit/1`: the revisions,
  the lowered `operations` limit, `exit`, `outcome`, `stage`, `code`, the number
  of drift rows, `verdictSha256` and `result`, the whole result line.
- `result.json`: the whole result line, `runtime.goVersion` written as
  `go1.27.2`; a consumer compares its own line with that one member replaced.
