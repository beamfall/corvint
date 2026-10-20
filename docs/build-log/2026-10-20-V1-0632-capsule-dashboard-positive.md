# V1-0632: capsule and dashboard cem/1.0 positive cases (gap 1)

## Problem

The 2026-10-17 delivered-open audit (`2026-10-17-delivered-open-audit.md`, gap 1) found that the
capsule (C15) and dashboard (C17) families had no actual `cem/1.0` positive case. The local-authority
capsule refused every stable enrollment with `capsule CEM base` (`CEM-V1-167`). The dashboard
refused every bundle flag and profile (`CEM-V1-168`, `CEM-V1-217`), and its frozen snapshot schema,
hash-pinned registry and closed error profile had no place for a stable universe.

## Decisions

- Dashboard: a separate `cem-stable` subcommand writes its own `corvint-dashboard-cem-stable/0`
  document, with a `corvint-dashboard-cem-stable-refusal/0` document on stderr. The snapshot
  schema, registry hash, error profile and retired bundle flags are unchanged, so no new registry
  version or conformance vector is needed. The view embeds the exact original map bytes and the
  verifier's whole `verify.StableResult`; nothing is reshaped. Completion and criterion
  satisfaction stay `NOT_OBSERVED`.
- Capsule: port the object demand to the stable map instead of building a capsule-native verifier.
  The closure accounts the six artifact roles by digest only and never reads artifact bytes. It
  binds the committed sidecar blob to the exact enrolled bytes and stays a precondition, not
  authority. Authority comes only from `verify.Stable` over the published minimal evidence view
  with a separately supplied artifact root (C15: "no graph/hash-only shortcut to authority").
- Requirements `CEM-V1-290..299` amend `CEM-V1-167` and the failure mode that said both families
  lacked a stable path. The amendments are recorded in place.

## Measurements

- The dashboard positive and controls tests take about 5 s. The capsule positive and controls
  tests take about 6 s, building real producer changes through `stablefixture`.
- A minimal capsule (both commits, all trees, changed blobs, the evidence blob and the sidecar) is
  enough for the stable verifier to return `ACCEPT` over the published view. History and unchanged
  blobs are not needed.

## Evidence

- `TestCEMV1290DashboardCEMStableReadsRealUniverse` runs with and without a coverage witness.
  `TestCEMV1293DashboardCEMStableRefusesStaleOrIncompleteBindings` covers seven binding controls and
  four argument controls.
- `TestCEMV1295CapsuleAdmitsStableClosure`, `TestCEMV1297CapsuleRefusesStableClosureControls` and
  the amended `TestCEMV1167CapsuleRefusesStableEnrollment` pass.
- The full suites pass for `cmd/corvint-dashboard-snapshot`, `tools/local-authority`,
  `conformance/dashboard-snapshot-v0` and `internal/companionrelease`. The last two are the
  binary-exec consumers that `corvint affected` selected.
- Fail-without-fix:
  - With the base `main.go` (no dispatch), the dashboard tests get `DASHBOARD_INVALID_ARGUMENT`.
  - With the base `capsule_closure.go` plus a compile stub, the positive test gets
    `capsule CEM base` and the role control fails.
- `go vet` is clean on both packages.
- One macOS detail: the stable boundary refuses a symlinked directory chain. The published-view
  test therefore resolves `t.TempDir()` (`/var` to `/private/var`), as `stablefixture` already does.

## Codex review round 1 (PR #1287)

Three P2 findings, all fixed in one follow-up commit:

1. The input reader ran `Lstat` on the final path only, then a separate `Open`. A symlinked parent
   (`root/linked -> /elsewhere`) read outside root, and a FIFO swapped in between the two calls
   blocked the open. The reader now walks each directory through its parent's `os.Root`
   descriptor, requiring the opened directory to be the one `Lstat` saw. It opens the file with
   `O_NONBLOCK` (Unix; nothing on other hosts) and requires the descriptor to be the regular file
   `Lstat` saw. New tests: `TestCEMV1292DashboardCEMStableInputStaysUnderRoot` (symlinked parent
   for the map and the OCM, symlinked final component, FIFO) and
   `TestCEMV1292DashboardCEMStableRefusesSwappedFIFO`, which opens a FIFO against a regular file's
   `Lstat` result. With the first reader, the symlinked parent was admitted (exit 0) and the
   swapped FIFO open blocked past 10 s.
2. Encoding `map` as a `json.RawMessage` compacted it, so the emitted bytes lost the final LF and no
   longer matched `mapSha256`. The test hid this by compacting its expected value. The document now
   carries `mapBase64`, the exact bytes in standard base64, and the test compares the decoded bytes
   with the on-disk map and its digest. With the first encoding, the raw comparison failed.
3. The capsule tests set no checks, so the CEM-V1-297 driver and subject demands never ran. Every
   check path in the fixture was also demanded by the changed-path or evidence rule, so a check
   control alone could not tell. `stablefixture.NewChangeWithUnchanged` adds an unchanged, uncited
   `tests/check.sh`. `NewChange` passes nil, so its repositories are unchanged. New controls:
   - the positive test with two checks;
   - a missing driver blob and a missing subject blob, each `changed blob missing`;
   - an absent driver path, `required capsule path missing`;
   - an evidence OID that names another carried blob, `capsule CEM blob binding`.

   Mutations: without the check demand the driver control admits; without the subject demand the
   subject control admits; without the OID comparison the mismatch reaches `capsule CEM sidecar
   binding`.

Rerun after the fix: the full suites for `cmd/corvint-dashboard-snapshot`, `tools/local-authority`
and `conformance/dashboard-snapshot-v0` pass, and so do the other `stablefixture` consumers
`internal/console`, `internal/extevidence` and `internal/mcp/bridge`. The `cmd/corvint` consumers
were not rerun: `NewChange` builds the same repository as before.

## NOT_RUN

- A full protected enrollment of a stable change through `loadEnrollment`, `publishEvidence` and
  `publish`. It needs the root-owned authority store and group, and the protected command also
  requires a codec-package-only target tree (`onlyCodecPackage`). The closure, published view and
  verifier were exercised directly.
- The C15 stale-live-gate control. The capsule holds no live Tasks gate, so there is nothing to go
  stale. Native authority stays `NOT_OBSERVED`.
- `make gate`, which the lane charter excludes.

## Residuals

- The dashboard refusal has no verifier result when the sidecar is not exact, the OCM fails, or the
  map is retired: `VerifyStableUniverse` returns a plain code in those cases.
- The enrollment format does not carry artifact bytes. The artifact root is supplied downstream.

## Rollback

Revert the change commit. The dashboard loses `cem-stable`, and the capsule refuses `cem/1.0`
with `capsule CEM base` again.
