# CLI contract repair round 1 — 2026-11-05

Owner scope: V1-2685, V1-3736 and V1-3738; incoming head `8dbea7e2c6b6b09a9504bb8df43deabcf4e9f903`,
base `cdeed7ee6396a8d6bf6225eb5b2b4cb4d601b419`. This entry records the four independent findings
and the host catalog failure. Native ticket disposition and independent re-review remain host-owned.

## Decisions and delivery

- CAP-V0-9300 / AFP-V0-023: add `internal/tasks/scale/`, `internal/tasks/testhooks/` and
  `internal/tasks/fixture/` to the Tasks CLI read scope. The last directory was discovered by
  checking the complete production-source walk, rather than just the two reported omissions.
  Keep the existing unbounded-reader floor. `TestTasksCLIRecursiveReadSelection` applies the
  production selector predicate to both reported subtrees and every production source the live
  format scanner walks, and asserts the floor remains declared. This is selection evidence,
  not confined-read qualification or evidence that the package can leave the floor.
- ACX-V0-9301 / CAP-V0-9300: exact command-node help is fully handled after rendering its
  synopsis. All other authored argv layouts return unhandled without writing either stream.
  This preserves the Pulse preamble's native exit 2, empty stdout and exact usage stderr.
  Removing every native synopsis was rejected by the whole conformance package: Core
  `cem gate-verdict` and `corvint-postmerge-workflow encode-id` still need the synopsis to
  satisfy existing help assertions. The final implementation retains those assertions and
  explicitly completes exact-node dispatch. It does not claim general native preamble help.
- CAP-V0-9300: the selected adapter separates option values, boolean flags and command words
  using existing authored synopsis placeholders. Inline values consume one argv element;
  separate values consume the next element even when it looks like help or a command.
  `--` stops option/help interpretation. No wire field, code, native stderr or golden changes.
- CAP-V0-045 / CAP-V0-9300: generate root machine help after all companion snapshots, including
  the discovery generator, then generate documentation. Extend the scratch Makefile regression
  with a newly generated binary, first-run root inclusion and second-run stability.
  Real `make capability-catalog` regenerated the full-depth catalog in `docs/CAPABILITIES.md`
  (`complete --commit REV`); the root JSON and other snapshots were already byte-identical.
  V1-3738's earlier typed-template, version and install delivery is retained; this round changes
  no install/update behavior. Whole process conformance rechecks the profile version controls.

## Validation and limits

All Go tests ran through `script/heavy.sh`, one selected package per invocation, with
`TMPDIR=/home/russell/lanes/clicontract.tmp` (0700), no exported commit identity and no semaphore
override. `corvint affected --base cdeed7ee6396a8d6bf6225eb5b2b4cb4d601b419` returned UNKNOWN:
247 selected, 92 unknown; every executed package is selected. No complete-selection claim.

Whole packages PASS: `internal/cliapp` (0.003s), `internal/cliprofile` (0.038s),
`tools/test-read-scopes-check` (3.453s), `tools/capabilities-check` (8.044s),
`internal/capcatalog` (0.066s), `conformance/cli-errors-v0` (99.053s).
The focused spec check also ran all existing `internal/specindex` tests (1.074s).
`make -s capabilities-check` PASS: 54 commands, 57 binaries, 44 MCP tools.
`script/capability-catalog_test.sh` PASS; the identical test with incoming Makefile FAILS
because first regeneration omits the new binary. The new adapter, native-help, process Pulse
and scope regressions all FAIL before their corresponding fixes; original failures are retained.
`make -s spec-requirements-check line-citations-check` PASS. Citation repin found no unchanged
content that moved, so no second repin was needed. Toolchain is `go1.27.3`.

Evidence directory: `/home/russell/lanes/clicontract-evidence/fix-round-1/`. No raw evidence is
committed. Frozen assertions and goldens are unchanged. The overbroad first draft of the scope
regression included test sources the production scanner excludes; corrected its walk to match
that scanner, while adding the genuinely missing production fixture directory.

Independent re-review, broad host qualification, native ticket completion, Windows vet and
publication are NOT_RUN here under the owner scope. No Tasks write, pre-change recapture,
model-backed reviewer, seal, push or PR. Bind/check will reuse the original pre-change receipts;
missing observations and any native NOT_PRODUCED reasons remain visible in the final handback.
Token/cost savings and Darwin qualification are NOT_OBSERVED. Self-development routes used:
affected selection and CEM review/binding. Mutation campaigns, optional services and external
interop are excluded because these repairs concern offline CLI and selection declarations.

## Evidence digests

| Artifact | SHA-256 |
|---|---|
| `catalog-before.log` | `3fc90648db66a4beb70d9fd62aae63134ef7b806c50dc25db842014e294a5e70` |
| `profile-before.log` | `4b2d33d0148c311453624d4118419bf4e9c7823e3901369278b81ae2ebd04895` |
| `cliapp-before.log` | `0ed4656f68c62a8275df6cdcc9739e3e238c8e4492034d79b2580f278806b733` |
| `pulse-before.log` | `8d9073ac65e933806e87d30bcb16c856e1a17493005445e89b8e7122e8c8e8ed` |
| `scopes-before.log` | `521dfcfe6d9a81d5a0667af382512ca8d7434b3b7ee39b8899e087fb6925fac9` |
| `conformance-final.log` | `55f93e6bfbe4497b6c1eae3ad2597c218815915928247869272397049bfaa560` |
| `capabilities-final.log` | `a9beb4c33680fb534ad016c58b49cfdfaf0abe141f1483c1b1dbefeee03b869d` |
| `scopes-final.log` | `a3c26c2142f8a79e05d37ebd52c8f76aaab2f21e7e8e6cdfb106914ab861148e` |

## Rollback

Restore the scoped source, read-scope and recipe changes, regenerate the catalog, and rerun the
same package checks. That rollback reopens the recorded findings; it does not authorize changing
frozen serializers or removing the selector safety floor. No new option or wire contract is added.
