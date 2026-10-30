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

# Additive member `verdictSha256` (2026-10-12)

Base: the public packet with manifest sha256
2e69ef83a8fe94d59afbcdfb16c590936d630dfb09ba46f5cf98dbc6ec6ae186.
Driver: V1-1265 / decision 0523 P1 (CEM-V1-066): SHA-256 of the canonical JSON value of the result minus runtime, repositoryEnvelope and verdictSha256, on every outcome
Produced by `make cem-stable-regen` (tools/cem-stable-regen, CEM-V1-091).

- Member: `verdictSha256`, after `repositoryEnvelope` in RESULT.schema.json `properties` and
  `required` and in every expectedResult.
- Schema fragment sha256: 980dbd0a7a812034e0a2178b7a355dc0c4e2ea10091b5665879c07b3d6166b46.
- Values captured from the native verifier on go1.27.2 for 54 cases.
- Values from the override file (sha256 fc82580378606bbff7a3a02ddcd6dde2df10ed48843328fe617c1e1543570f4b) for: go1.24.13-nonstructural-positive, go1.24.13-structural-refusal.
- Envelope cells (inherited/s0e/REPOSITORY-ENVELOPE.cells.json and its ../REPOSITORY-ENVELOPE.cells.json copy):
  19 cells take the value of the cases with the same exit and frozen keys.
- Cell values from the cell override file (sha256 be817ae04d08452d00dddf3c06ad5c62367fdff851227e0fc36d2b74456dcdac) for: ordinary-sha256, origin-sha256, linked-sha256, relative-sha256, attributes, shallow-missing, missing-base, corrupt-base, linked-attributes, missing-promisor.
- No frozen key, expectedExit, recipe or status changed; every captured
  result matched its frozen expectation on all 22 frozen keys and its exit
  (runtime.goVersion only through a sealed structural-tuple substitution).
  The case count stays 56.

## Changed files

| File | Old sha256 | New sha256 |
| --- | --- | --- |
| RESULT.schema.json | fabb8d1a3773f9881b20b39ae16256ce960a6ffbc68d45c18bdde450482d0e3a | 2334f05dfbc4cb226ae53b5df22eba4ba0f94f3f7136b13e04b610e37bc14f2a |
| FULL-RESULT-CASES.json | f7f02ec13d77c90080c13160b1b5ab47a61baf3c9bde0fee5b14d439a257e4df | 25d4155c2235c3034146093d0c844e0e448281429c74bad5f19abf215d672146 |
| inherited/s0e/REPOSITORY-ENVELOPE.cells.json | c45e4e65b9a7465f309ff19c41d48b3542a61aa5bc2e4a4368a914dd028e43e9 | 03c55a3222d694df390724587b192170dca17bf6799bae80d86e7d706f670811 |
| CHANGES.md | 57bbd50aa21b64b6a77e4e1962a1f15aeeec64e9342f4c4fd4e1ab76cb48ffb7 | See manifest.json; this file cannot contain its own final digest. |
| manifest.json | 2e69ef83a8fe94d59afbcdfb16c590936d630dfb09ba46f5cf98dbc6ec6ae186 | See ../manifest.json publicAmendmentManifestSHA256; manifest.json is not listed inside its own file list. |

Files not listed above are byte-identical to the base packet. INPUTS.json
keeps its repair-era pins as history.

## Re-bound files outside the packet

These files are re-bound to the digests above; their new digests depend on this
file and are recorded in Git, in ../manifest.json and in each tuple manifest.

| File | Predecessor sha256 |
| --- | --- |
| protocol/cem-1.0/stable/REPOSITORY-ENVELOPE.cells.json | ecbc86f8aab33a9548ae9cf9cb4f6e02d3198ff13d9d01a92814a78ccb1609c5 |
| protocol/cem-1.0/stable/REPOSITORY-ENVELOPE.md | 89385b7ae4a910a4f2588264547f75353b0e30f144b7ae0851e118678b111a25 |
| protocol/cem-1.0/stable/manifest.json | c48f07ea68c136eb2db3d948883f29206cc31bb0ffec3fe498cf9a2b9ddcc679 |
| protocol/cem-1.0/structural-tuples/go1.27.2/TUPLE.json | 707309ef2145c56f3acc1f3583fb9f6b63659f949d4c9317ca6bf6e5624e8d76 |
| protocol/cem-1.0/structural-tuples/go1.27.2/manifest.json | 13ba55835e1349fb05e6c231eae4805c608fc81a6b02bb4b54a836a056202b55 |
| protocol/cem-1.0/stable/RESULT.schema.json | identical to protocol/cem-1.0/stable/repository-envelope-packet/RESULT.schema.json |

# Recorded value re-emission of `citation`, `verdictSha256` (2026-10-15)

Base: the public packet with manifest sha256
9423e789c58b20079ffdc5f6d73de1a703ecd0b2f155d5b988e1fbeaafd05f07.
Reason: CEM-V1-080/081/082 amendments approved by the owner on 2026-10-15 (decision 0530): concentratedHunks is 0 when maxFanOut is below 2 (V1-1331), so every sealed value whose citation counted a concentrated hunk at fan-out 1 is re-emitted with concentratedHunks 0 and its recomputed verdictSha256; recorded re-emission approved by the owner on 2026-10-15 (option (a)).
Record file sha256: aba7c010bd47772949dbf00016baae65b06b2f38aa00b4bfa51c9c0b4c0a5fa5.
Produced by `make cem-stable-regen-replace` (tools/cem-stable-regen, CEM-V1-091
as amended by decision 0530).

- Members re-emitted in place: `citation`, `verdictSha256`. No key is added or removed and
  RESULT.schema.json is unchanged.
- Values captured from the native verifier on go1.27.2 for 54 cases.
- Values from the override file (sha256 c8c837ae36134ebd1c6e8046ec31c9f30c0aacf0bd8d3ea9ba125f8282347ba5) for: go1.24.13-nonstructural-positive, go1.24.13-structural-refusal.
- Envelope cells (inherited/s0e/REPOSITORY-ENVELOPE.cells.json and its ../REPOSITORY-ENVELOPE.cells.json copy):
  19 cells take the values of the cases with the same exit and other keys.
- Cell values from the cell override file (sha256 ce08087f9ea547dfb9ebf2fa8854f456e59d8a1c5e49b370df4184a00c2184b6) for: ordinary-sha256, origin-sha256, linked-sha256, relative-sha256, attributes, shallow-missing, missing-base, corrupt-base, linked-attributes, missing-promisor.
- Every other key and expectedExit is unchanged; every captured result
  matched its frozen expectation on them (runtime.goVersion only through a
  sealed structural-tuple substitution). The case count stays 56.

## Re-emitted values

Each digest is the sha256 of the value's compact JSON.

| Kind | Id | Member | Predecessor sha256 | New sha256 |
| --- | --- | --- | --- | --- |
| case | normal-exit-lingering-descendant-contained | citation | 5c0b2fa6ba8a806151afdd83718471934ac262955fb9fb9b789b879a0de1a387 | 3118a3c4d75ffde1583c9bbf69fabb04fc39bcdcbb63c1790d7c1b03b9dbca8f |
| case | normal-exit-lingering-descendant-contained | verdictSha256 | 9049c8dde3fc2229b9d11b28f52616d151c4c6f767c8ffdf4884dc9248b67039 | 25341552c6c1efdef200be504ead7d476a8121ce1b5d2eb92df675e92b0e1f11 |
| case | go1.24.13-nonstructural-positive | citation | 0e3296fbb0e2a6c14a0d658a77159e853861e146a5317051ee4a090aede161a8 | 503dfa2da5ef8f833e335921c0658647c8fd6b243d706178876961ffd818b5ce |
| case | go1.24.13-nonstructural-positive | verdictSha256 | 7839bbcac5693cbfd553b915401431a7a93790492370d2af1f6e8623f5cc9e2c | 2ec89552aba0406b03ec0e3c322262ec9ecde228197ad7c6901c6c6a284835aa |
| case | ledger-336-evidence | citation | d00418c18f225bc8310ab2c8e1beb1d3aa2587d5f2c12bd27e59dc011c0fd027 | 81d58a163267c297f06a6022b8dcff8b7f66c9ec334b95b97a31ccaf2cea5df9 |
| case | ledger-336-evidence | verdictSha256 | d8f0c3fd3dfe6a504ad8594fac0f782c5faa71703e2224393d8332de607e04bb | bfaaa9546b0cda23c875bba6c192c9c31f2488fb30181bc7d2372c84e480febc |
| cell | ordinary-sha1 | citation | 5c0b2fa6ba8a806151afdd83718471934ac262955fb9fb9b789b879a0de1a387 | 3118a3c4d75ffde1583c9bbf69fabb04fc39bcdcbb63c1790d7c1b03b9dbca8f |
| cell | ordinary-sha1 | verdictSha256 | 9049c8dde3fc2229b9d11b28f52616d151c4c6f767c8ffdf4884dc9248b67039 | 25341552c6c1efdef200be504ead7d476a8121ce1b5d2eb92df675e92b0e1f11 |
| cell | origin-sha1 | citation | 5c0b2fa6ba8a806151afdd83718471934ac262955fb9fb9b789b879a0de1a387 | 3118a3c4d75ffde1583c9bbf69fabb04fc39bcdcbb63c1790d7c1b03b9dbca8f |
| cell | origin-sha1 | verdictSha256 | 9049c8dde3fc2229b9d11b28f52616d151c4c6f767c8ffdf4884dc9248b67039 | 25341552c6c1efdef200be504ead7d476a8121ce1b5d2eb92df675e92b0e1f11 |
| cell | linked-sha1 | citation | 5c0b2fa6ba8a806151afdd83718471934ac262955fb9fb9b789b879a0de1a387 | 3118a3c4d75ffde1583c9bbf69fabb04fc39bcdcbb63c1790d7c1b03b9dbca8f |
| cell | linked-sha1 | verdictSha256 | 9049c8dde3fc2229b9d11b28f52616d151c4c6f767c8ffdf4884dc9248b67039 | 25341552c6c1efdef200be504ead7d476a8121ce1b5d2eb92df675e92b0e1f11 |
| cell | relative-sha1 | citation | 5c0b2fa6ba8a806151afdd83718471934ac262955fb9fb9b789b879a0de1a387 | 3118a3c4d75ffde1583c9bbf69fabb04fc39bcdcbb63c1790d7c1b03b9dbca8f |
| cell | relative-sha1 | verdictSha256 | 9049c8dde3fc2229b9d11b28f52616d151c4c6f767c8ffdf4884dc9248b67039 | 25341552c6c1efdef200be504ead7d476a8121ce1b5d2eb92df675e92b0e1f11 |
| cell | ordinary-sha256 | citation | 79738518c3cea1ed314910aa1cd6d160d55b9ba4e97c9d4feecd7e6d71788001 | 24adfe1b3e382d2b0b8ab4abdb7fcad789a41f61e17f855c1fbf9cdd37c27e90 |
| cell | ordinary-sha256 | verdictSha256 | e26d39c58e1794dbad64e28ec05896593a367f8eb14b84dd81c0c0700fec5ca9 | 5f374fa32236c88089c6644ddb4d8d0b256f87819ac8540f10014da962c0514e |
| cell | origin-sha256 | citation | 79738518c3cea1ed314910aa1cd6d160d55b9ba4e97c9d4feecd7e6d71788001 | 24adfe1b3e382d2b0b8ab4abdb7fcad789a41f61e17f855c1fbf9cdd37c27e90 |
| cell | origin-sha256 | verdictSha256 | e26d39c58e1794dbad64e28ec05896593a367f8eb14b84dd81c0c0700fec5ca9 | 5f374fa32236c88089c6644ddb4d8d0b256f87819ac8540f10014da962c0514e |
| cell | linked-sha256 | citation | 79738518c3cea1ed314910aa1cd6d160d55b9ba4e97c9d4feecd7e6d71788001 | 24adfe1b3e382d2b0b8ab4abdb7fcad789a41f61e17f855c1fbf9cdd37c27e90 |
| cell | linked-sha256 | verdictSha256 | e26d39c58e1794dbad64e28ec05896593a367f8eb14b84dd81c0c0700fec5ca9 | 5f374fa32236c88089c6644ddb4d8d0b256f87819ac8540f10014da962c0514e |
| cell | relative-sha256 | citation | 79738518c3cea1ed314910aa1cd6d160d55b9ba4e97c9d4feecd7e6d71788001 | 24adfe1b3e382d2b0b8ab4abdb7fcad789a41f61e17f855c1fbf9cdd37c27e90 |
| cell | relative-sha256 | verdictSha256 | e26d39c58e1794dbad64e28ec05896593a367f8eb14b84dd81c0c0700fec5ca9 | 5f374fa32236c88089c6644ddb4d8d0b256f87819ac8540f10014da962c0514e |
| cell | comments | citation | 5c0b2fa6ba8a806151afdd83718471934ac262955fb9fb9b789b879a0de1a387 | 3118a3c4d75ffde1583c9bbf69fabb04fc39bcdcbb63c1790d7c1b03b9dbca8f |
| cell | comments | verdictSha256 | 9049c8dde3fc2229b9d11b28f52616d151c4c6f767c8ffdf4884dc9248b67039 | 25341552c6c1efdef200be504ead7d476a8121ce1b5d2eb92df675e92b0e1f11 |
| cell | shallow-complete | citation | 5c0b2fa6ba8a806151afdd83718471934ac262955fb9fb9b789b879a0de1a387 | 3118a3c4d75ffde1583c9bbf69fabb04fc39bcdcbb63c1790d7c1b03b9dbca8f |
| cell | shallow-complete | verdictSha256 | 9049c8dde3fc2229b9d11b28f52616d151c4c6f767c8ffdf4884dc9248b67039 | 25341552c6c1efdef200be504ead7d476a8121ce1b5d2eb92df675e92b0e1f11 |
| cell | hostile | citation | 5c0b2fa6ba8a806151afdd83718471934ac262955fb9fb9b789b879a0de1a387 | 3118a3c4d75ffde1583c9bbf69fabb04fc39bcdcbb63c1790d7c1b03b9dbca8f |
| cell | hostile | verdictSha256 | 9049c8dde3fc2229b9d11b28f52616d151c4c6f767c8ffdf4884dc9248b67039 | 25341552c6c1efdef200be504ead7d476a8121ce1b5d2eb92df675e92b0e1f11 |

## Changed files

| File | Old sha256 | New sha256 |
| --- | --- | --- |
| FULL-RESULT-CASES.json | 25d4155c2235c3034146093d0c844e0e448281429c74bad5f19abf215d672146 | 045b8fd32d81d09d79d9d157213bcbb6cf133c006f6b6f102b3a0f680816819c |
| inherited/s0e/REPOSITORY-ENVELOPE.cells.json | 03c55a3222d694df390724587b192170dca17bf6799bae80d86e7d706f670811 | d89a96828e888c285085b92cb339eb34e09b0558617ce04f76feba633969c5f3 |
| README.md | 34f8ca92caec4392da9d731fd5941ea8c0f58403da8d697b56286879fcbf7c00 | 47febbf4da14a86b2733015731ed91d916817a2b1d3a5818f46838cdb5f93f96 |
| CHANGES.md | 6f0673a52f0df49d3065d7e16ff0cab83c0ef27afdedaef462e1bb559adf5d2a | See manifest.json; this file cannot contain its own final digest. |
| manifest.json | 9423e789c58b20079ffdc5f6d73de1a703ecd0b2f155d5b988e1fbeaafd05f07 | See ../manifest.json publicAmendmentManifestSHA256; manifest.json is not listed inside its own file list. |

Files not listed above are byte-identical to the base packet. INPUTS.json
keeps its repair-era pins as history.

## Re-bound files outside the packet

These files are re-bound to the digests above; their new digests depend on this
file and are recorded in Git, in ../manifest.json and in each tuple manifest.

| File | Predecessor sha256 |
| --- | --- |
| protocol/cem-1.0/stable/REPOSITORY-ENVELOPE.cells.json | 4baad2604240eb80f77c468ff1d11eccb76e885344756552b9a6cf841a6b4239 |
| protocol/cem-1.0/stable/REPOSITORY-ENVELOPE.md | 2c160dd526905f48c65dbfb1fc732b947dfcd429b236a1cd4b6dbffc78b6fe38 |
| protocol/cem-1.0/stable/manifest.json | 87a0baa70505c162f19998da1ad41caef5508e7d38f1c4e0fecd1537741e0e71 |
| protocol/cem-1.0/structural-tuples/go1.27.2/TUPLE.json | 4c048beee50d0ef5ae9fc32073138c54923e6f94e8e11504c988dc12346abfdf |
| protocol/cem-1.0/structural-tuples/go1.27.2/manifest.json | 61f7961652c94b3019fe492fd1995f13f6f06bf59b1c71da9d6b87890ca2da43 |

# Recorded value re-emission of `code`, `verdictSha256` (2026-10-16)

Base: the public packet with manifest sha256
259a294acb88a797e6f3b6f628c895f86871898177f102969826b125f6f3b4fa.
Reason: V1-1297 (decision 0535): a pinned Git object that is absent locally is repository-object-unavailable, not git-read-failed; the cells whose pinned object is absent re-emit code and the verdict digest that binds it.
Record file sha256: a4b12a309a161f1f4a4cc326a84a1c563e42457f819adf3f6109d30636a70ec6.
Produced by `make cem-stable-regen-replace` (tools/cem-stable-regen, CEM-V1-091
as amended by decision 0530).

- Members re-emitted in place: `code`, `verdictSha256`. No key is added or removed and
  RESULT.schema.json is unchanged.
- Values captured from the native verifier on go1.27.2 for 54 cases.
- Values from the override file (sha256 67c5b8063feb8dba964d86b836aec8773ae0c7efef3fe1e2239b796a3de9f15f) for: go1.24.13-nonstructural-positive, go1.24.13-structural-refusal.
- Envelope cells (inherited/s0e/REPOSITORY-ENVELOPE.cells.json and its ../REPOSITORY-ENVELOPE.cells.json copy):
  7 cells take the values of the cases with the same exit and other keys.
- Cell values from the cell override file (sha256 6c8abfc161b90db9a643aeae0f3845921e9f851dca4b16b240b3ed419b5c3da2) for: ordinary-sha256, origin-sha256, linked-sha256, relative-sha256, attributes, alternates, bad-marker, object-link, root-link, shallow-missing, missing-base, corrupt-base, ambient-alternates, linked-wrong-backpointer, linked-missing-common, linked-attributes, pack-link, info-link, missing-promisor, missing-target-tree, missing-target-blob, corrupt-target-blob.
- Every other key and expectedExit is unchanged; every captured result
  matched its frozen expectation on them (runtime.goVersion only through a
  sealed structural-tuple substitution). The case count stays 56.

## Re-emitted values

Each digest is the sha256 of the value's compact JSON.

| Kind | Id | Member | Predecessor sha256 | New sha256 |
| --- | --- | --- | --- | --- |
| cell | shallow-missing | code | 8c87ea14986d150d0ee6f8c97e3646df1813b544c1148d59e0fa4a508f9d1f67 | 9b53d41cef49cffaf469ffc4698a4e5240de871c936e46c478815279763d7e00 |
| cell | shallow-missing | verdictSha256 | b77d442c21f7c2fadde9c9d44e7e49fce055bbb8e35d8084a055184055f478c8 | ea6fba19161e2dcc738dd626e0c0872d27d866959f0312b76d9054c9f44e2e01 |
| cell | missing-base | code | 8c87ea14986d150d0ee6f8c97e3646df1813b544c1148d59e0fa4a508f9d1f67 | 9b53d41cef49cffaf469ffc4698a4e5240de871c936e46c478815279763d7e00 |
| cell | missing-base | verdictSha256 | b77d442c21f7c2fadde9c9d44e7e49fce055bbb8e35d8084a055184055f478c8 | ea6fba19161e2dcc738dd626e0c0872d27d866959f0312b76d9054c9f44e2e01 |
| cell | missing-promisor | code | 8c87ea14986d150d0ee6f8c97e3646df1813b544c1148d59e0fa4a508f9d1f67 | 9b53d41cef49cffaf469ffc4698a4e5240de871c936e46c478815279763d7e00 |
| cell | missing-promisor | verdictSha256 | b77d442c21f7c2fadde9c9d44e7e49fce055bbb8e35d8084a055184055f478c8 | ea6fba19161e2dcc738dd626e0c0872d27d866959f0312b76d9054c9f44e2e01 |

## Changed files

| File | Old sha256 | New sha256 |
| --- | --- | --- |
| inherited/s0e/REPOSITORY-ENVELOPE.cells.json | d89a96828e888c285085b92cb339eb34e09b0558617ce04f76feba633969c5f3 | aff583ede3f3dd083aaaf4321efb9cc7d3d6413a280058db445b6df322cb6d5b |
| CHANGES.md | 43fce8bacebaf36510f4208861bcf83ac35e328d8a06c06b72584dfc78051a02 | See manifest.json; this file cannot contain its own final digest. |
| manifest.json | 259a294acb88a797e6f3b6f628c895f86871898177f102969826b125f6f3b4fa | See ../manifest.json publicAmendmentManifestSHA256; manifest.json is not listed inside its own file list. |

Files not listed above are byte-identical to the base packet. INPUTS.json
keeps its repair-era pins as history.

## Re-bound files outside the packet

These files are re-bound to the digests above; their new digests depend on this
file and are recorded in Git, in ../manifest.json and in each tuple manifest.

| File | Predecessor sha256 |
| --- | --- |
| protocol/cem-1.0/stable/REPOSITORY-ENVELOPE.cells.json | 443a350e0c7f14ada93b74141b181e8be3d77f268b9ccb8b6472145ceb0835bd |
| protocol/cem-1.0/stable/REPOSITORY-ENVELOPE.md | 1be9e931e7ef3fa9e1a184250a073d9ef8d802a280f0675d2f947939d790c175 |
| protocol/cem-1.0/stable/manifest.json | 90c71102e2f03631f70d0382dc18fffb976c35b042384cb601183b0d610ab1d3 |
| protocol/cem-1.0/structural-tuples/go1.27.2/TUPLE.json | 31d7adcd19c95f2fbcbfef62a0b86c7d69c44630403c45205e285c01efef8cb7 |
| protocol/cem-1.0/structural-tuples/go1.27.2/manifest.json | 2172d7ce6511ace2212b37b3ce3acef1af042e60972df010a80b4ac7072e34d8 |

# Recorded value re-emission of `runtime` (2026-10-22)

Base: the public packet with manifest sha256
f0cfb65c79a4621811eb8ae790787eaa7b99f1568fb23cf0125b437de71b4c16.
Reason: V1-2327, decision 0551: the go1.27.1 structural corpus qualified the predicates; runtime.structuralQualification is QUALIFIED_EXACT_TUPLE on every admitted tuple (CEM-V1-371)
Record file sha256: e4e052bde1fbd26811a8a28a626dda102a9755ed96619a30d28a8bd1dd536ac0.
Produced by `make cem-stable-regen-replace` (tools/cem-stable-regen, CEM-V1-091
as amended by decision 0530).

- Members re-emitted in place: `runtime`. No key is added or removed and
  RESULT.schema.json is unchanged.
- Values captured from the native verifier on go1.27.3 for 54 cases.
- Values from the override file (sha256 1ec46acfcc49c59c654cac0e451f3c84c8cb192ca4c281763f48a67f58a235a2) for: go1.24.13-nonstructural-positive, go1.24.13-structural-refusal.
- Envelope cells (inherited/s0e/REPOSITORY-ENVELOPE.cells.json and its ../REPOSITORY-ENVELOPE.cells.json copy):
  22 cells take the values of the cases with the same exit and other keys.
- Cell values from the cell override file (sha256 fad1aff75a4b170fbe35439511c057de174caed5d569109ce3bfdf456d15deaa) for: ordinary-sha256, origin-sha256, linked-sha256, relative-sha256, attributes, corrupt-base, linked-attributes.
- Every other key and expectedExit is unchanged; every captured result
  matched its frozen expectation on them (runtime.goVersion only through a
  sealed structural-tuple substitution). The case count stays 56.

## Re-emitted values

Each digest is the sha256 of the value's compact JSON.

| Kind | Id | Member | Predecessor sha256 | New sha256 |
| --- | --- | --- | --- | --- |
| case | cancel-before-content | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| case | cancel-during-target-resolution | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| case | cancel-during-base-sidecar | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| case | cancel-during-initial-target-entry | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| case | cancel-during-target-sidecar-body | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| case | cancel-during-canonical-diff | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| case | cancel-during-evidence-or-structural | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| case | cancel-before-first-drift-item | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| case | cancel-during-artifacts | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| case | root-replaced-before-final-check | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| case | cancel-at-final-publication | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| case | owner-failure-before-first-content-progress | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| case | owner-failure-after-provisional-complete | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| case | normal-exit-lingering-descendant-contained | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| case | canonical-per-op-timeout | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| case | canonical-output-limit | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| case | canonical-nonzero-exit | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| case | simultaneous-outer-cancel-and-overflow | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| case | simultaneous-overflow-and-nonzero | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| case | simultaneous-owner-failure-and-cancel | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| case | optional-attributes-EACCES | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| case | optional-objects-info-ELOOP | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| case | optional-commondir-EIO | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| case | metadata-leaf-replaced | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| case | admin-ancestor-symlink | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| case | empty-commondir | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| case | absolute-commondir | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| case | metadata-CRLF | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| case | metadata-two-final-LF | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| case | metadata-invalid-UTF8 | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| case | metadata-over4096 | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| case | empty-http-alternates | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| case | target-sidecar-blob-missing-child-exit | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| case | target-sidecar-blob-corrupt-compressed-child-exit | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| case | target-sidecar-blob-wrong-complete-hash | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| case | target-sidecar-blob-launch-failure | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| case | evidence-blob-missing-child-exit | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| case | evidence-blob-corrupt-compressed-child-exit | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| case | evidence-blob-wrong-complete-hash | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| case | evidence-blob-launch-failure | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| case | repeated-target-sidecar-lookup-cancel | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| case | canonical-complete-parser-and-nonzero | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| case | canonical-per-op-timeout-and-overflow | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| case | canonical-valid-framing-limit-plus-one-and-nonzero | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| case | canonical-complete-malformed-before-limit-and-overflow | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| case | outer-expiry-content-observed | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| case | outer-expiry-content-unobserved | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| case | outer-expiry-normal-session-close-observed | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| case | outer-expiry-normal-session-close-unobserved | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| case | ledger-336-evidence | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| case | ledger-337-evidence | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| case | partial-drift-one-item-cancel | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| case | per-op-timeout-returned-then-outer-expiry-in-teardown | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| case | per-op-timeout-unreturned-then-outer-expiry-in-teardown | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| cell | ordinary-sha1 | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| cell | origin-sha1 | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| cell | linked-sha1 | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| cell | relative-sha1 | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| cell | ordinary-sha256 | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| cell | origin-sha256 | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| cell | linked-sha256 | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| cell | relative-sha256 | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| cell | attributes | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| cell | comments | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| cell | alternates | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| cell | bad-marker | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| cell | object-link | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| cell | root-link | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| cell | shallow-complete | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| cell | shallow-missing | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| cell | missing-base | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| cell | corrupt-base | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| cell | hostile | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| cell | ambient-alternates | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| cell | linked-wrong-backpointer | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| cell | linked-missing-common | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| cell | linked-attributes | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| cell | pack-link | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| cell | info-link | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| cell | missing-promisor | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| cell | missing-target-tree | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| cell | missing-target-blob | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |
| cell | corrupt-target-blob | runtime | a1bcb5397051dbdf1ef7b989dfbcb816debca97960cf0e64e9c314fc19fba4ed | dfb50cb0d15bb83f6d2aed5150ce554bb7861e3885b57139782c2d5992f60c37 |

## Changed files

| File | Old sha256 | New sha256 |
| --- | --- | --- |
| FULL-RESULT-CASES.json | 045b8fd32d81d09d79d9d157213bcbb6cf133c006f6b6f102b3a0f680816819c | 06fb20c0a1903921c7c5f83459bf790313c5e33379538e867982ab9a5e6603be |
| inherited/s0e/REPOSITORY-ENVELOPE.cells.json | aff583ede3f3dd083aaaf4321efb9cc7d3d6413a280058db445b6df322cb6d5b | b59262dca59a6f815477c599f6fc173d2665bcf83d8d128e7c0c92160e0c4fdc |
| CHANGES.md | 8f7ed45631c927ffb9b65c2185d7fa261f26962b7c58b2582ff477fc9e419ccb | See manifest.json; this file cannot contain its own final digest. |
| manifest.json | f0cfb65c79a4621811eb8ae790787eaa7b99f1568fb23cf0125b437de71b4c16 | See ../manifest.json publicAmendmentManifestSHA256; manifest.json is not listed inside its own file list. |

Files not listed above are byte-identical to the base packet. INPUTS.json
keeps its repair-era pins as history.

## Re-bound files outside the packet

These files are re-bound to the digests above; their new digests depend on this
file and are recorded in Git, in ../manifest.json and in each tuple manifest.

| File | Predecessor sha256 |
| --- | --- |
| protocol/cem-1.0/stable/REPOSITORY-ENVELOPE.cells.json | 3678fcca54082ed8b2919bcfa9d1f6240c34d8f761eee1a7364a9d0a9e42b23d |
| protocol/cem-1.0/stable/REPOSITORY-ENVELOPE.md | 5d54f6f53f4bddc8582635398656f97cd3e4db582dcda410be77ea93ac7843c3 |
| protocol/cem-1.0/stable/manifest.json | f9498bddc7764801cc1179c5996b6a9730f2834b0d9a70cc1ac93c08aa405174 |
| protocol/cem-1.0/structural-tuples/go1.27.2/TUPLE.json | ec302a1f4745bb1e7b620df1b580fc2a54371244d4243710376f0516aeee21e5 |
| protocol/cem-1.0/structural-tuples/go1.27.2/manifest.json | 49f4e95921aeebcbcccb87136a8b55cf70aa6e24cbb8c1fbbc555b3a2da33b52 |
| protocol/cem-1.0/structural-tuples/go1.27.3/TUPLE.json | cd9e83cb4a997369dd5cedaf74c8b622181c7246fbf599ab58f56de03ce62750 |
| protocol/cem-1.0/structural-tuples/go1.27.3/manifest.json | 1240a0b0b4fb5525f16899e387940097b9cdd3c852da3bbabeeaaf2d53789a2e |

# Recorded schema enum widening of `/$defs/runnerReceipt/properties/profile` (2026-10-24)

Base: the public packet with manifest sha256
c6a5cbe0899fe31968f42e740db00da6005989c3e7ce1954c1e832e5045834f4.
Reason: V1-2107 (Codex P2 on beamfall/corvint#1338): CEM-V1-310..313 admit the opaque/0 runner receipt label for maps whose runner receipt was not admitted, so a verify result that references such a receipt is ACCEPT but failed RESULT.schema.json, which pinned references.runnerReceipts[].profile to corvint-test-runner-receipt/0; the node admits exactly the two labels stable.schema.json already admits (CEM-V1-621).
Record file sha256: 08deed4e4bb28eab94ba18e4d9ee8be631023f0f42bed916c2526aed78cc1e25.
Produced by `make cem-stable-regen-widen` (tools/cem-stable-regen, CEM-V1-620).

- RESULT.schema.json `/$defs/runnerReceipt/properties/profile` admitted `corvint-test-runner-receipt/0`; it now admits `corvint-test-runner-receipt/0`, `opaque/0`.
- Widened node sha256 (compact JSON): ee614710112c1b723eb3e032a0f7a103be2470b0c2e405cceb870d4ad3cf9f47 -> 2dcc5dcebe9c2714f5f0d3afb39a407590ae28c4dab65430a439b7d1b03f41bb.
- No member, case, cell, exit or other schema byte changed; every frozen
  expectation stays valid because no value was removed.

## Changed files

| File | Old sha256 | New sha256 |
| --- | --- | --- |
| RESULT.schema.json | 2334f05dfbc4cb226ae53b5df22eba4ba0f94f3f7136b13e04b610e37bc14f2a | 5849832edd23b92c8cc70aa7ea20f26f51dd8d0cfb8f288b5518515b56aa2fdb |
| README.md | 47febbf4da14a86b2733015731ed91d916817a2b1d3a5818f46838cdb5f93f96 | dd48c32b5b8ca50ce0f7d345fc9b43c70d4b58f45a1b5cf5a89ee0c24f534de6 |
| CHANGES.md | 6bde75e56857096345bb59c6ba484c9c1155c213d6c446cec63d3469ce8afc82 | See manifest.json; this file cannot contain its own final digest. |
| manifest.json | c6a5cbe0899fe31968f42e740db00da6005989c3e7ce1954c1e832e5045834f4 | See ../manifest.json publicAmendmentManifestSHA256; manifest.json is not listed inside its own file list. |

Files not listed above are byte-identical to the base packet. INPUTS.json
keeps its repair-era pins as history.

## Re-bound files outside the packet

These files are re-bound to the digests above; their new digests depend on this
file and are recorded in Git, in ../manifest.json and in each tuple manifest.

| File | Predecessor sha256 |
| --- | --- |
| protocol/cem-1.0/stable/REPOSITORY-ENVELOPE.cells.json | bf8244b77897400fbb8c28a463d24d190bba98cadbc38627af23639d6609dd60 |
| protocol/cem-1.0/stable/REPOSITORY-ENVELOPE.md | d667043b39855fbe4933c428be7e18e8630b3e84a122bc91438ba028e3c3aad8 |
| protocol/cem-1.0/stable/manifest.json | bd5016fe00f39add50e67fb7556b813b2869480bf9ef10cfc6965da8646fbc97 |
| protocol/cem-1.0/structural-tuples/go1.27.2/TUPLE.json | 4bb2c6de30a65df944ed7a6530f0ac6c431b2ee25f24d90ea178fa229d53fc67 |
| protocol/cem-1.0/structural-tuples/go1.27.2/manifest.json | ae87785b03bb10ff8461f86c93207b74ccad8ba104a9b7438aeedcd37db63abd |
| protocol/cem-1.0/structural-tuples/go1.27.3/TUPLE.json | 2eb10d8258eddcb272185e35296e423167e47353ee35d94fdb22f3a10c20f32e |
| protocol/cem-1.0/structural-tuples/go1.27.3/manifest.json | 912c59cbdffc35c9031491c74c6f41bb9bba4377c076cce8fcb411f48d88a692 |
| protocol/cem-1.0/stable/RESULT.schema.json | identical to protocol/cem-1.0/stable/repository-envelope-packet/RESULT.schema.json |
