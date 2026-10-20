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
