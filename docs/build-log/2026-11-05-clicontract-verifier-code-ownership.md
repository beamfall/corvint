# clicontract verifier repair 1: refusal-code ownership

Owner intent: repair the host verifier's error-code ownership failure at
`07af7dfe4b7553f82cbfe51953ad1447d235013f`, against unchanged base
`d436dff76de1b8e75c427069ae7f80003ae99bc2`. Preserve every frozen native
contract, assertion and golden. Reuse the existing pre-change capture; finish
with a checked CEM, without sealing, publication or native ticket writes.

## Decision and scope

V1-3736 / ACX-V0-9301 already owns the explicitly selected CLI profile.
Document its existing `unknown-binary`, `catalog-unavailable`,
`native-protocol-required`, `cli-deadline` and `binary-unavailable` refusal
meanings in that spec's failure modes. This satisfies ECO-V0-002/003 without
adding an allowlist entry or changing code, diagnostics, exit status or shape.
V1-2685 / CAP-V0-9300 and V1-3738 / CAP-V0-9302 retain their prior deliveries;
this repair changes no runtime behavior. Rollback removes this documentation
addition and restores the reported ownership failure.

## Evidence and retained failures

Raw artifacts are outside the repository in
`/home/russell/lanes/clicontract-evidence/verifier-repair-1-ownership/`.
The host log SHA-256 is
`909fb5f95c8922a67deeeeebcfd9f69b87f6af4d8a2d8d7c92cd1ae803a0aae6`.
The required TMPDIR is private (0700); the configured heavy lock directory is
unchanged. No pre-change capture or make dogfood-change was rerun.

- `corvint affected --base FULL_BASE`: UNKNOWN, 232 selected, 92 unknown;
  every tested Go package is selected. Installed-binary staleness remains visible.
- Citation repin: no moved unchanged anchors; line-citations-check passes.
- Error-code ownership and full `script/heavy.sh --admit-only make doc-gates`:
  PASS; full gate 300.0 seconds, ownership subgate exit 0.
- Whole `internal/cliprofile`: PASS, 0.025 seconds.
- Whole `internal/liveverify/affected`: FAIL, 233.161 seconds. The originally
  reported live-worktree test does not fail in this run. The unchanged
  `TestIncrementalSelectionMeetsTheLiveBudget` fails at p95 195.587998 ms,
  5.49 probes against limits of 100 ms and 5.00 probes. No threshold is relaxed;
  the cause of this timing result is NOT_OBSERVED.
- Focused `TestLGLV0885_GateRunSerializesASharedResource`: FAIL, 0.720 seconds,
  `/gates/0/argv/2` exceeds the existing 128-byte bound (191 bytes here).
  The host log reproduces this at BASE too (177 bytes). Its fixture is unchanged.
- `script/spec-requirements-conflict_test.sh`: FAIL, unexpected README conflict,
  reproducing the third selected-run failure. The fixture and generator have
  identical Git blobs at BASE and HEAD; neither is changed by this lane.

| Artifact | SHA-256 |
|---|---|
| doc-gates.log | bdfe23c8b766245c824a76fdb79a2085e26b1a5a773410421e47b76d734d3632 |
| cliprofile-whole.log | dcfa599a6d6a662354649839f7c801eb6dd188a43462dd6c58f580ef4a75b0dd |
| affected-whole-retry.log | 455fab8e4e69133eae22a98f7562aee95fe4ac91de8a58deb88b349991eec069 |
| store-repro.log | 52a51e6821ca2e3fd012534f52305b0d23fff1fc62839067be802107cb4c0a73 |
| spec-conflict-repro.log | 94e88d2760992a6b94528d076afddb3476a77d5cd9f9b6144219293ad91487a0 |

The aggregate outcome stays **failed**. Verification remains PARTIAL on the
inherited store/shell failures and the observed performance failure; no native
ticket completion is claimed. The full selected campaign is NOT_RUN again:
its failing components were retried directly, and no Go package was modified.
Windows vet and exhaustive release checks are NOT_RUN for this scoped prose
repair. Independent review remains orchestrator-owned, NOT_RUN here.

Self-development routes used: affected advice and final CEM binding/checking;
pre-change context reuses the original capture. Retrieval/learning, external
providers and live interoperability are not applicable to this documentation
repair. Missing runtime/semantic evidence stays NOT_OBSERVED. Guarding tests,
per-hunk coverage and discrimination remain NOT_PRODUCED unless the native
report produces them; structural acceptance does not establish semantic review.
