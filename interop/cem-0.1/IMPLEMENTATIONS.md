# CEM 0.1 implementation matrix

Status: no independent implementation has been demonstrated.

| ID | Role | Owner | Language | Commit | Corvint dependency | Result |
|---|---|---|---|---|---|---|
| corvint-reference | reference producer/consumer | Corvint | Python | repository HEAD | native | reference attestation only |
| corvint-portability | reference consumer portability probe | Corvint | Go | repository HEAD | none | internal 7/19/6 matrix plus 5 producer maps PASS; not independent |
| P1 | producer | unclaimed | — | — | must be none | pending |
| C1 | consumer | unclaimed | — | — | must be none | pending |
| C2 | consumer | unclaimed | — | — | must be none | pending |

Interoperability requires P1 output to pass C1, C2, and the Corvint reference, while C1 and C2 must
independently agree with every manifest case. Record implementation repository, exact commit,
dependency tree, author/organization, run log digest, elapsed engineer time, and every PASS/FAIL/
UNSUPPORTED cell before changing a row.

These do **not** count as independent: a Corvint wrapper or subprocess; code generated from,
translated from, copied from, or linked to the Corvint verifier; two CLIs over one protocol library;
two implementations by the same author/team; or Corvint-authored Go/TypeScript examples. They may be
useful portability probes but cannot satisfy P1/C1/C2. Shared use of the public specification,
manifest, raw fixtures, standard JSON/hash libraries, and the Git executable is allowed.

Freeze the tagged 0.1 manifest and artifacts once an external implementation starts. Any changed
field, canonicalization rule, parser decision, acceptance result, or drift rule creates `cem/0.2`;
it must not silently rewrite 0.1 evidence.

## Errata

Erratum 1 (2026-09-12, Corvint decision 0098 amendment) states the target entry kind in same-path
drift: a changed-OID entry that is not a `100644`/`100755` blob is `deleted` and is never searched.
It clarifies the existing rule that only regular-file blobs carry evidence content; it is not a
drift-rule change and creates no new profile. No independent implementation had started, and every
prior vector's fixture bytes and expected record are unchanged. The manifest adds one drift vector,
`deleted-symlink-changed` (`targets/symlink-changed.patch`), so its SHA-256 moves from
`324f1588c383b7c617474feb51ad8f50ad5926bd481f780e60235dfa86e67392` to
`2655258e73d569e35dffb36cc6d3ae2e738801848d9f57decb774463b92f34bd` and the matrix is 7/19/6 over
51 artifacts. The historical `runner.py` is revised only to pin that manifest digest and matrix, so
its SHA-256 moves from
`120b4b3eb13c3885ca9da07eedd8f168ae7ab429b9a533c1a09484df9f80b963` to
`8f25aecc9681c2b098e4022df2e74ad23973187421a04fad1b7ad7ab6647c0c6`;
receipts made before the erratum describe the earlier packet only.

Erratum 2 (2026-09-13, Corvint decision 0192) settles seven patch-parser readings on which Corvint's
two Go readers disagreed. The existing text decides four of them. A hunk-header suffix is any
bytes. Creates and deletes may carry several hunks. New positions are checked by the parser. A
similarity or dissimilarity line may appear without a rename. Three are owner calls. A percent has
no leading zero. Every body record and marker record ends in LF. A delete's mode equals the base
tree mode of its old path. `ALGORITHMS.md` states each one inline. No independent implementation
had started. Every manifest entry, fixture byte, expected record, and `runner.py` is unchanged.
