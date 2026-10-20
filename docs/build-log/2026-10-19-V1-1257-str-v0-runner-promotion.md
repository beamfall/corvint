# 2026-10-19 STR-V0 runner promoted to implemented (V1-1257)

Human-owned intent: `docs/specs/session-test-runner-v0.md`, "Acceptance evidence: promotion and
kill criteria" and Rollout step 3. Owner-delegated decision 0544 item 6 (PR #1272, not yet on
`main` when this entry was written) adopts the non-latching AFP-V0-030 reading. The 2026-10-16
entry, `2026-10-16-v1-1257-str-v0-promotion-evidence.md`, met the measurements. It left two gaps:
STR-V0-004's versioned capability manifest and STR-V0-011, neither with executable evidence.

BASE: `c27d54e9245f0a15d581062b83a0452c03508703`.

## Problem

V1-1513 (`2026-10-16-v1-1513-str-v0-004-011-evidence.md`) had already added the adapter manifest
`corvint-test-adapter/0`, its pre-discovery refusal and the STR-V0-011 reader guard. Two gaps
remained:

- The version string did not identify its content. The operation set could change under the
  same version and every test would still pass.
- No receipt cited the version. A receipt could not show which manifest its adapter ran under.

## Decisions

- **STR-V0-058: version names content.** An append-only test table,
  `adapterManifestVersionPins` in `internal/sessiontest/adapter_test.go`, pins each shipped
  version to the canonical digest of `{operations, profile}`. The pin lives in the test, not in
  product code, so the pin cannot be bumped by the same edit that changes the operations without
  the reviewer seeing a pin-table change. A pin is never edited; new content needs a new version
  and a new pin.
- **STR-V0-059: receipts bind the version.**
  - `Invocation.AdapterManifest` (`adapterManifest`) is set in `Runner.Execute` from the manifest
    the pre-discovery check accepted.
  - `Verify` treats an absent, other or unregistered-adapter citation as `INCOMPLETE`.
- **Wire change: additive and fail-closed.** The member is additive to `corvint-test-run/0`.
  Receipts are per-run artifacts and `verify-shards` is their only reader (STR-V0-011). The strict
  reader refuses a receipt without the member as non-canonical, so a receipt from an older binary
  fails closed (`INCOMPLETE`) rather than verifying without a version. No profile bump: no retained
  receipt corpus is read across binaries, and mixed-binary fan-in already fails on manifest
  identity (STR-V0-009).
- **STR-V0-011 needs no new code.** `TestSTRV0011ReceiptsHaveNoOtherReader` covers product Go,
  scripts, workflows and Makefiles. Its only allowed reader is `runTestVerifyShards`. It was
  re-proved against a synthetic violation (below).
  - `tools/ci-shard-costs refresh --history` reads cost tables, not receipts. It is outside the
    guard's subject and feeds no ranking, selection, learning or authority.
- **Promotion scope.** The runner is `implemented`. The CI call site stays an experimental shadow,
  and balanced and measured placement stay experimental and unpromoted, as the spec already says.

## Evidence

Each mutation was applied in place, its focused test was run and the file was restored. The log
is at `/private/tmp/cem10-build/l1257-failproof.log`.

| Mutation | Test | Result |
| --- | --- | --- |
| M1: drop `recorded.AdapterManifest = ...` in `Execute` | TestSTRV0059ReceiptBindsAdapterManifestVersion | FAIL (receipt does not cite the version) |
| M2: disable the `Verify` manifest check | TestSTRV0059ReceiptBindsAdapterManifestVersion | FAIL (`verify PASS [], want INCOMPLETE`) |
| M3: add an operation, keep `corvint-test-adapter/0` | TestSTRV0058AdapterManifestVersionPinned | FAIL (`changed without a version bump`) |
| M4: add a `ReadReceipt` caller in `cmd/corvint/zz_mutation.go` | TestSTRV0011ReceiptsHaveNoOtherReader | FAIL (`new sites: [cmd/corvint/zz_mutation.go affectedPriorReceipt ReadReceipt]`) |

With the change, every test passed. All runs went through `heavy.sh`, `-count=1`, one package per
command:

- `./internal/sessiontest/`: full package.
- `./cmd/corvint/`: `-run 'TestSTRV0|TestSpec|Traceab'`. This includes TestSTRV0010CLIRunWritesReceipt,
  which now asserts that the CLI receipt cites `corvint-test-adapter/0`.
- Full packages: `./cmd/corvint-test/`, `./cmd/corvint-go-test-provider/`,
  `./tools/ci-shard-costs/`, `./tools/test-commands-check/`, `./tools/test-commands-run/`,
  `./tools/ci-forward-row/`, `./tools/corvint-pr-tests/` and `./tools/fixture-maintenance-check/`.

## NOT_RUN

- The units that `corvint affected --base BASE` widened through `UNBOUNDED_READER` and
  `PATH_LITERAL_READER` (73 units in all). These are generic repository walkers, and
  this change touches only `internal/sessiontest` and one CLI test.
  - The plan's unknowns are `LANGUAGE_FRONTIER`: `go:build-constraint-variants`,
    `go:nested-module-frontier` and `shell:foreign-invocation-unmodelled`.
- `make gate`: not required for scoped issue work.
- New hosted measurements: the 2026-10-16 entry stands.
- The balanced-placement 20% bar: unmeasured, and balanced placement stays unpromoted.

## Limits

- The STR-V0-011 guard is static. It inherits V1-1513's limits: reflection, or a reader in a
  language the guard does not scan, would escape it.
- The pin table is a test. A reviewer must still refuse an edited or removed pin.

## Rollback

Revert the commit.

- The delivery status returns to experimental.
- Receipts lose `adapterManifest`, and `Verify` loses the check.
- Receipts written by the promoted binary are then refused by the older strict reader. Mixed
  binaries therefore fail closed (`INCOMPLETE`) and never pass silently.
