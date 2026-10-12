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

# Additive member `citation` (2026-10-12)

Base: the public packet with manifest sha256
21655c5d73a4b9b9af9f91e6dacbf702a8d2f95cdc9b40368a4cc240dedbd535.
Driver: V1-1265 citation member (CEM-V1-092): the CEM-V1-082 block on ACCEPT, null on every other outcome
Produced by `make cem-stable-regen` (tools/cem-stable-regen, CEM-V1-091).

- Member: `citation`, after `artifactChecks` in RESULT.schema.json `properties` and
  `required` and in every expectedResult.
- Schema fragment sha256: b22e37c29d75aeb17788e73f4d3c4a04ddd9a015071e5b720165288f1a1cae86.
- Values captured from the native verifier on go1.27.2 for 54 cases.
- Values from the override file (sha256 ac37f037ca28ec4f68f786da10db01c98beedd984b2c7f2db356e4e03a17cebc) for: go1.24.13-nonstructural-positive, go1.24.13-structural-refusal.
- Envelope cells (inherited/s0e/REPOSITORY-ENVELOPE.cells.json and its ../REPOSITORY-ENVELOPE.cells.json copy):
  19 cells take the value of the cases with the same exit and frozen keys.
- Cell values from the cell override file (sha256 83382f3d22bf101021a4165b9131703cf9cce9037f25f725f227dcfa69edd080) for: ordinary-sha256, origin-sha256, linked-sha256, relative-sha256, attributes, shallow-missing, missing-base, corrupt-base, linked-attributes, missing-promisor.
- No frozen key, expectedExit, recipe or status changed; every captured
  result matched its frozen expectation on all 21 frozen keys and its exit
  (runtime.goVersion only through a sealed structural-tuple substitution).
  The case count stays 56.

## Changed files

| File | Old sha256 | New sha256 |
| --- | --- | --- |
| RESULT.schema.json | 6c157e59ae9ed2ac372724084080a7e74263b8d9d3963e995445035fcc89176f | fabb8d1a3773f9881b20b39ae16256ce960a6ffbc68d45c18bdde450482d0e3a |
| FULL-RESULT-CASES.json | a59f490c33b02be55a576496be8ef670dd8b5e5765182f609c8a562b11e66ad1 | f7f02ec13d77c90080c13160b1b5ab47a61baf3c9bde0fee5b14d439a257e4df |
| inherited/s0e/REPOSITORY-ENVELOPE.cells.json | 68432f3e11fe9dff1f47fb4672ed97351dc67fd4875d8405e2b45e1007d065d9 | c45e4e65b9a7465f309ff19c41d48b3542a61aa5bc2e4a4368a914dd028e43e9 |
| README.md | a2626a13b1dd3e11239ffda83edb5d39cc3b50ff5ff10d33dc38ecb8a03904e1 | 34f8ca92caec4392da9d731fd5941ea8c0f58403da8d697b56286879fcbf7c00 |
| CHANGES.md | 36d33faf7b62497e16e21de09ed36f0450d08e36c50851d1a7961de7aac233cc | See manifest.json; this file cannot contain its own final digest. |
| manifest.json | 21655c5d73a4b9b9af9f91e6dacbf702a8d2f95cdc9b40368a4cc240dedbd535 | See ../manifest.json publicAmendmentManifestSHA256; manifest.json is not listed inside its own file list. |

Files not listed above are byte-identical to the base packet. INPUTS.json
keeps its repair-era pins as history.

## Re-bound files outside the packet

These files are re-bound to the digests above; their new digests depend on this
file and are recorded in Git, in ../manifest.json and in each tuple manifest.

| File | Predecessor sha256 |
| --- | --- |
| protocol/cem-1.0/stable/REPOSITORY-ENVELOPE.cells.json | 6b3828ac703b8488cfacd17cf71e25dd7a795498e41b6d45681f2033dcb643b1 |
| protocol/cem-1.0/stable/REPOSITORY-ENVELOPE.md | dd33dcaf9362ce8d8914916d659891c6e8a635d18fafbc50d9dff91eded2e9e7 |
| protocol/cem-1.0/stable/manifest.json | 5302bbfcc5d1c5007d3720e0e2d48abedb01e8d6d908102831c3bba4475978c8 |
| protocol/cem-1.0/structural-tuples/go1.27.2/TUPLE.json | ea6bdc1a85ef99b195aa7d6945b03572344d3844bdfe51da25a4c07d2f51ea8a |
| protocol/cem-1.0/structural-tuples/go1.27.2/manifest.json | 5233bba946e67a1f2e225b9a287adad7953bfc4a2ed2ddedd9452ab403b8ae28 |
| protocol/cem-1.0/stable/RESULT.schema.json | identical to protocol/cem-1.0/stable/repository-envelope-packet/RESULT.schema.json |
