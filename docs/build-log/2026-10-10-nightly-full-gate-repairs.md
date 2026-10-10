# Nightly full-gate repairs (V1-1092, V1-0960, V1-1112, V1-1113)

Date: 2026-10-10

The hosted release-gates run 38052349384 (main 64b5d174 lineage) failed on macos-15 and
ubuntu-24.04. This entry records the disposition of each failing step. No product behavior or
requirement changes; no decision is needed.

Fixed:

- V1-1092 (cross-vet, both hosts): `internal/tasks/dispatch/prompt_fragments_test.go` used
  `testConfig`, which is defined only in the `darwin || linux` test file, so the Windows
  type-check failed. The test file now carries the same build constraint. `make cross-vet` passes.
- V1-0960 (`internal/tasks/cli` TestCALV0131, macos-15): the test placed stores under the
  unresolved `t.TempDir()`. The store correctly refuses a symlinked ancestor
  (`UNSUPPORTED_FILESYSTEM`), and macOS `/tmp` and `/var` are symlinks. The test now resolves its
  temporary roots. `TestCALV0131_SymlinkedTempRoot` points TMPDIR at a symlink and runs the table
  in a subtest, because `t.TempDir` caches the root the parent test created first. It fails on the
  base and passes with the fix.
- V1-1112 (`cmd/corvint` TestIndexReceiptNamesTheSweptLegacyStore, macos-15): the receipt reports
  the resolved repository path, as intended; the test compared it with the unresolved one. The test
  now expects the resolved legacy store. A second test reaches the repository through a symlinked
  root and fails with the old expectation.
- V1-1113 (companion-release smoke, macos-15): the smoke expected the JS test provider's usage line
  to list `<unit|e2e>`, but the provider now dispatches `negate` and `qualify-keep-reporters` too.
  The check moved into `checkJSProviderUsage`, which expects the current usage line.
  `TestJSProviderUsageAdmitsTheBuiltProvider` builds the real provider and runs it, and fails with
  the stale expectation. A local `make companion-release-gate` passed with 33 checks, exit 0.

Not fixed, needs a decision:

- `TestHostAdapterJavaScriptHosts` (GOC-V0-008) failed on macos-15 in both `host-adapter-test` and
  `go-test`. Each failure was a different OpenCode subtest. Each returned
  `corvint-process-cleanup-unconfirmed` where `timeout` was expected.
  - The failure first appears in the run after the V1-0371 changes (7a4078b8, fd48aee7, c05bf0a4)
    landed. Those changes narrowed cleanup confirmation to an `ESRCH` group probe. The earlier
    check also accepted `EPERM` once the leader had exited.
  - It did not reproduce locally: 2/2 passes at load average about 94.
  - A local Darwin probe shows the expected sequence: `EPERM` while the group's zombies are
    unreaped, then `ESRCH` within about 2 ms of `close`.
  - Inference: on the hosted macos-15 VM, either reaping is slower than the immediate
    post-`close` probe, or the 100 ms reap timer completes the run before `close` arrives. Whether to retry the probe within a bound, or to accept `EPERM` after the leader is
    reaped, changes the AHI-050 cleanup contract. It is left to its own ticket rather than to this
    lane.
