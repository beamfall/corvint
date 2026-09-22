# Decision 0086 — exact DR-0026 grants and one prospective Packet 5 retry

Date: 2026-09-11. Status: accepted. Authority: repository owner, “Execute the handoff”
for `docs/plans/release-path-handoff-2026-09-11.md`, under the recorded delegated instruction
“you should be able to make the correct calls on this to get to Corvint release”.

## Decision and evidence

1. Import the untouched receipt
   `conformance/perf-v0/results/packet-5-current-pin-requalification-retry-2026-09-11-formal/report.json`
   through the dated amendment of decision 0037. Its status stays `NOT_RUN`: ten valid rows,
   two invalid Beamfall user-prompt rows, ten accepted divergences, 100 samples per runtime,
   five warmups, and global outcome `insufficient_evidence`. This is evidence, not qualification.
2. The invalid rows are `beamfall-harness-user-prompt` and its snapshot-present mirror.
   The two `diagnose` executions retained in `conformance/perf-v0/testdata/dr-0026/cold.txt`
   and `conformance/perf-v0/testdata/dr-0026/snapshot.txt` reproduce exactly the same stdout pair:
   oracle SHA-256 `7d18ce2eafd164a6b1cf5f92e6569bb224f88988c74ea3d3cc5397c9e54df6ff`
   (5,543 bytes), candidate `cddea558337c499a3f71637000c52a0f0be099a7b8afcd119c7770488b5364a0`
   (5,641 bytes). The actual stdout files are retained beside those diagnostics.
   Only `$.context.coverage.uncertainty` (zero versus one member) and its consequent
   `$.context.coverage.packet_bytes` (4844 versus 4942) differ. The added line is
   `9 test-path symbol candidates withheld from query ranking; the context verb serves test evidence`.
   This is registered DR-0026 under GPK-V0-052, not a new output waiver.
3. Ratify exactly two source-held DR-0026 grants for those task IDs with
   `harnessEventArgv("user-prompt")`, relaxing only `unequal-stdout`. Each unequal measured
   pair MUST match both full stdout hashes above; otherwise the unrelaxable reason
   `accepted-divergence-stdout-mismatch` invalidates the row. Other process, stderr, exit,
   stability and corpus predicates remain binding. The independent review found that the old
   task/argv/reason-only mechanism did not enforce the handoff's promised byte boundary;
   the exact-pair discriminator closes that gap without changing emitted or consumed output.
   `TestExactDR0026GrantRejectsOtherStdout` checks the actual diagnosed pair and changed bytes.
4. The independent performance blocker remains: cold Corvint compact session-start candidate p95
   1053.3 ms (95% CI 1030.4–1061.9), oracle 2064.8 ms, GPK-V0-017(c) limit 1032.4 ms.
   The 20.9 ms miss is inside that CI; the prior host load was 15.75. Cold (a) being
   `REPORTED` does not relax (c). Expected outcome before the new run: cold compact (c) was the only threshold failure
   among valid rows; the two DR-0026 rows remain unqualified in that receipt. PASS is plausible
   on an idle host, but these measurements do not establish that contention caused the miss.
5. This record supplies new owner ratification under P5R-V0-004; item 2 is its harness
   discriminator and item 3 its source-closed exact grant. Commit these changes first as the
   new Corvint pin. Then prospectively register `conformance/perf-v0/packet-5-retry-2-manifest.json`
   with a fresh timestamp, both Corvint corpus revisions and trees updated to that pin, and a new
   claim-scope ID under unchanged decision 0016. Preserve all twelve row invocations, stdin,
   classes, caps, corpus setup, Beamfall pin, 100 samples, five warmups and 120-second timeout.
   The two Beamfall user-prompt rows additionally select `acceptedDivergences: ["DR-0026"]`;
   copying the old rows without these keys would not apply the new grants. Admit the raw
   manifest hash by source and record it here before running any samples.
6. Execute that preregistration once from a clean worktree at or after the new pin, with host
   load below 3 and no competing lane gates or Go tests, into a fresh formal output directory.
   Commit the receipt untouched. A `FAIL` is a valid terminal result; it does not authorize a
   wider cap or a further retry without a new record. Any new divergence is terminal `NOT_RUN`:
   diagnose and register it before considering any further grant. Never overwrite either old
   formal receipt or import a second execution of a preregistration.
7. On PASS, import the new receipt through decision 0037, update release notes and BUILD-LOG,
   then bind wheel and Go archive witnesses to final HEAD, check, move the local tag and push
   only on the handoff's three PASS conditions. On FAIL, record terminal disposition, retain
   the untagged release and refresh the other witnesses. No corpus or cap amendment is authorized.
   Lane integration starts only after the tag is pushed.

## Scope and rollback

This is Packet 5 timing evidence only. Global outcome stays `insufficient_evidence`;
Packet 4, compatibility, FULL, publication, promotion and query retirement remain unqualified.
The missing DR-0026 CLI parity case stays `NOT_YET_AUTHORED` and query promotion remains blocked
under GPK-V0-034. Actual harness discriminator fixtures do not claim that broader qualification.

Rollback removes this record's two grants, exact-pair check and new manifest admission; retains
all historical run and diagnostic bytes; and restores decision 0037's prior report selection.
No pinned native executable or hook is modified. The owner may reverse this delegated decision.

## Prospective registration follow-up — 2026-09-11T17:01:18Z

The new Corvint pin is `b9d1be2dc2e6bf2e9818c8d1faae2976d998295a`, tree `7ba5987d0baaffa89408142fc9e1675d66a989bb`.
The retry-2 manifest is registered at `2026-09-11T17:01:18Z`, claim-scope ID
`packet-5-current-pin-requalification-retry-2-2026-09-11`, raw SHA-256
`1b0fc8b74b94b5be6afeac284fed6c0dc2a87e943d9aa6d937e71f2228d12920`. Both Corvint corpora use that pin/tree;
the two Beamfall user-prompt rows select the exact DR-0026 grants. No sample has run under this
registration. Formal output is exclusively
`conformance/perf-v0/results/packet-5-current-pin-requalification-retry-2-2026-09-11-formal`.
