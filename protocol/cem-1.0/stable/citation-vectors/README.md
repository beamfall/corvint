# Citation vectors (CEM-V1-111)

Status: like the rest of `protocol/cem-1.0/stable/`, implemented and run by both
consumers; the technical contract is proposed and not promoted.

One clean `cem/1.0` map and seven refused copies of it. Every expected value in
`EXPECTED.json` was produced by the native verifier (`internal/cem/verify`),
not by the portable consumer. Both consumers load this directory and assert
every case: `TestCEMV1111CitationVectorsNative` and
`TestCEMV1111CitationVectorsPortable`. For each case they check the exit code,
`outcome`, `stage`, `code`, `issueCodes`, the drift `[path, status]` pairs,
`verdictSha256`, the top-level member order (`resultMembers`) and, for the
ACCEPT, the exact bytes of the citation block. They also compare the whole
result line with `results/<name>.json`, nested member order included
(CEM-V1-113). The rules these values follow are in `../ALGORITHMS.md`,
CEM-V1-108 to CEM-V1-110 and CEM-V1-113.

- `repository.recipe.json`: two flat SHA-1 commits, built exactly like
  `../neutral-criterion/repository.recipe.json`. The change edits code and
  notes, renames nothing, deletes `legacy.txt` and moves a cited line of
  `keys&values.txt`. `SPEC.md` defines requirement IDs with the
  ``- `ID`:`` line form, one of them inside a span that starts mid-line.
- `stable.json`: the clean map (case `clean`). It has every tier but `held`,
  both anchor kinds, one annotation hunk (`VERSION`), an evidence record
  shared by two hunks, a cited deletion hunk anchored through its base path, and nonzero
  `specificity` and `samePathCitations`. Its producer-only arrays are empty,
  so it declares no artifacts and the artifact root is this directory.
- `cases/<name>.json`: the refused maps. The drift cases recompute the IDs they
  change, so they reach the drift stage instead of failing identity checks.
  The wire cases are refused before any Git read.
- `EXPECTED.json`: profile `cem-stable-citation-vectors/1`; the base and target
  commit IDs the recipe must reproduce; the result member order; and one record
  per case, whose `citation` is the compact citation block as a JSON string, or
  `null` for a REJECT.
- `results/<name>.json`: the exact result line the native verifier wrote for
  each case, one compact object and its LF, with `runtime.goVersion`
  `go1.27.2`. Under another admitted toolchain only that value differs.
- `measures.json`: profile `cem-citation-measure-vectors/1`; library-level
  inputs of the CEM-V1-082 block (evidence, hunks, added lines, basis) and the
  exact block bytes for each, asserted by `TestCEMV1082MeasureVectorsNative`
  and `TestCEMV1082MeasureVectorsPortable`. They pin `concentratedHunks` at
  `maxFanOut` 1 (V1-1331), the annotation stop list (V1-1335) and the base
  path of a deletion hunk (V1-1332).

| Case | Class | Exit | Result |
| --- | --- | --- | --- |
| `clean` | control | 0 | ACCEPT, stage `complete` |
| `evidence-stale` | EvidenceDriftStale | 1 | REJECT, stage `drift`, `evidence-drift` |
| `same-path-deletion-cited` | EvidenceDriftDeleted | 1 | REJECT, stage `drift`, `evidence-drift` |
| `base-revision-mismatch` | BaseRevisionMismatch | 1 | REJECT, stage `binding` |
| `unsupported-spec` | UnsupportedSpec | 2 | REJECT, stage `wire`, `unsupported-spec` |
| `missing-field` | MissingField | 2 | REJECT, stage `wire`, `missing-field` |
| `duplicate-member` | InvalidJSON | 2 | REJECT, stage `wire`, `invalid-json` |
| `not-an-object` | NotAnObject | 2 | REJECT, stage `wire`, `invalid-field` |

## Misreadings these vectors catch

Each row is a reading of CEM-V1-108, CEM-V1-109 or CEM-V1-113 that a consumer
could plausibly make. The rows before the last three were each applied as a
one-line change to the native verifier, and the native vector test then failed
on the cases named (V1-1333 build log, 2026-10-14). The last three rows are the
portable consumer's own order before V1-1369. Each was restored separately, and
the portable test then failed on the cases named, plus the neutral criterion
ACCEPT (V1-1369 build log, 2026-10-14).

V1-1335 removed 21 code-word terms from the annotation stop list, so the
`NOTES.md` line of `clean` is no longer an annotation and `clean` no longer
catches the two stop-list rows. The library-level cases in `measures.json` now
catch them, and `removed-terms-single` catches a removed term put back. Each of
these was applied to both consumers (V1-1331 build log, 2026-10-15).

| Misreading | Failing cases |
| --- | --- |
| Added bytes joined with each line's LF removed | `clean` |
| Citation block members in another order | `clean` |
| Verdict digest written with `", "` and `": "` separators | all eight |
| Verdict digest with HTML escaping (`&` as `\u0026`) | `clean`, `evidence-stale`, `same-path-deletion-cited`, `base-revision-mismatch` |
| Top-level result members in another order | all eight |
| Defining line read from the blob line start instead of the span start | `clean` |
| A stop term missing from the list | `measures.json` `stop-list-complete`, `short-code-hunks` |
| Stop terms compared without lowercasing | `measures.json` `stop-list-complete`, `short-code-hunks` |
| Shift null pairing hunk `i` with hunk `i+1` | `clean` |
| `specificity` reported as the anchored count | `clean` |
| A deletion hunk given no path (post-image path only) | `clean` |
| `samePathCitations` counting distinct evidence instead of basis entries | `clean` |
| A cited file deleted by the change not treated as drift | `same-path-deletion-cited` |
| `targetSpan` written as `start`, `end` | `clean`, `evidence-stale`, `same-path-deletion-cited` |
| Hunk members in map order (`id`, `path`, `oldRange`, ...) instead of sorted | `clean`, `evidence-stale`, `same-path-deletion-cited`, `base-revision-mismatch` |
| `references` members in map order (`criterionBindings` first) | all eight |
