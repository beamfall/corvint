# Repair6 changes

Base: a byte copy of the repair5 public packet (manifest sha256
bb76906bd468b432d84217c7780b2493a189d2c89833e15b9b58a964cb9eaeb1).
Driver: independent public-only review REVIEW-of-repair5.json (sha256
107fd1c50944bc4e86a567926cb7d0c90a068c7f03aee7fd8c830bd0bfb02a9e),
which rejected repair5.

Repair6 changes only public packet text/data. No runtime behavior is claimed. The
algorithm revision label remains cem-s0e-expanded-admission-r1; PROPOSED-RULES.md
adds a rank 1 "Repair6 rules" section that overrides repair5's ledger-boundary
details where they differed.

## Findings resolved

1. Ledger after refusal: LOGICAL-LEDGERS is now the attempted trace. Boundary-337
   ends at row 1025, the refused reservation, marked `refused-before-spawn`.
   There is no row after the refused reservation.
2. Drift ordering: inherited/ALGORITHMS.md says to retain drift records in
   ascending evidence ID order. Repair6 makes that bytewise evidence-ID order
   normative for processing and emission, independent of map input order.
3. Fixed prefix: repair6 keeps the sidecarless 12-hunk target prefix at 15 calls.
   The rejected 16th-call candidates are a canonical changed-level batch for
   `.corvint` or a per-file target inventory lookup. Section 3 already freezes a
   complete sidecarless positive at 18 reservations and requires the changed
   `.corvint` level for the target-sidecar case, not this absent-sidecar target.
4. Stale prose: README.md, manifest.json and this file now describe repair6 and do
   not reference absent packet files.

## Boundary arithmetic

The 12-hunk target has 11 supported evidence-backed hunks and one mechanical hunk.
Supported basis capacity remains `11 * 32 = 352`, enough for both 336 and 337
evidence records.

The fixed non-evidence prefix is 15 calls: 8 canonical/sidecar calls, 4 simulation
lookup/blob calls for `app.txt` and `sort.go`, 2 mechanical proof calls for
`sort.go`, and 1 drift-target resolve.

Evidence validation charges two calls per evidence record. Drift processing then
charges one target lookup per sorted evidence ID until completion or refusal.

Boundary-336 accepted arithmetic:

`15 + 2 * 336 + 336 = 1023`

Boundary-337 attempted arithmetic:

`15 + 2 * 337 + 335 admitted drift lookups + 1 refused reservation = 1025`

The complete theoretical boundary-337 trace would require one more drift lookup
(`1026` total), but repair6 does not put unattempted work in LOGICAL-LEDGERS.

Sorted-order effect for boundary-337: `evidence:sha256:f8863f6d...` is now in the
335 completed drift records. `evidence:sha256:ffb2f5ca...` is the refused sorted
drift item 336, and `evidence:sha256:ffd48e23...` is unattempted sorted item 337.
The partial-drift-one-item cancel case now retains the first sorted evidence ID,
`evidence:sha256:0001f810...`.

## Changed files

| File | Old sha256 | New sha256 |
| --- | --- | --- |
| PROPOSED-RULES.md | 12f75080ba1d3717cca140565dba3bf99ec0389dbcf1c84d1e58550d07d5f8f5 | e58f4fa795a589b66d3d16015c721a3cb95f1c47dd821d4da91ed9007949f3e9 |
| README.md | 3132b957e85be0c0af992da4c71527c4f8766c4bd996b390a8591bdfc930e3b2 | a2626a13b1dd3e11239ffda83edb5d39cc3b50ff5ff10d33dc38ecb8a03904e1 |
| FULL-RESULT-CASES.json | 73d9ce61b79f608be4809c5f4fa19185d6eff351cec51cf244b9fa46984266b6 | a59f490c33b02be55a576496be8ef670dd8b5e5765182f609c8a562b11e66ad1 |
| LOGICAL-LEDGERS.json | a003a7a0dc71739a6a3f598a2eeed422bf9b95d462b433ba4422bcba2485a1d0 | 10df72e24b43c5f64c42dd2ebf731cf6435fe3c13ec5623816dda2cbed61a2fb |
| INPUTS.json | c3a3f389b75b690ecd7c8a252bee95ed98b5fc13abff16507eb06d3f504a5230 | See final manifest and HANDBACK.json; this file was rebound after CHANGES.md changed. |
| CHANGES.md | 13e5133d8c0d8e559b7ec2d4e9b8e88858e19c260d2ce725673861603135f22f | See final manifest and HANDBACK.json; this file cannot contain its own final digest. |
| manifest.json | bb76906bd468b432d84217c7780b2493a189d2c89833e15b9b58a964cb9eaeb1 | See HANDBACK.json; manifest.json is not listed inside its own file list. |

Files not listed above are byte-identical to repair5.

## Verification status

Portable conformance was run after rebinding and logged to ../conformance.log.
Any portable-reader mismatch is reported in HANDBACK.json with the applicable
repair6 rule; the packet was not bent to match old reader behavior.
