# Affected hidden-package integration: verifier repair 2

The host's whole `internal/liveverify/affected` run failed
`TestDriverSelectionCoversTheShadow_V13067`: gate selection rejected the read-scope
declaration for `.github/testconfine/cmd`. Repair 1 declared newly indexed hidden
packages, but the selector still excluded every dot and underscore directory.

## Decision and requirement traceability

For V1-4688 / AZU-V0-005, align selector source attribution and AFP-V0-023
declaration validation with explicitly addressable hidden Go packages. Keep the
bounded source walk, VCS metadata exclusions (`.git`, `.hg`, `.svn`), fixture
exclusion and nested-module boundaries. AFP-V0-050 immutable declaration checking
also follows the Go plugin's hidden-directory admission. Retain both project-owned
read scopes rather than removing the declarations to hide the integration error.

`TestAZUV0005HiddenPackageSelection` checks hidden and underscore packages,
import-dependent selection, test-only isolation, scoped documentation reads,
and metadata, fixture and nested-module exclusions. The existing committed-scope
parity test now requires the hidden fixture to produce a known declaration,
instead of permitting uncertainty, as AZU-V0-005 requires. Its validation and
map-equality assertions remain intact. No assertion is removed or loosened.

V1-4689 / AZU-V0-006 and V1-4690 / AZU-V0-007 are unchanged. No wire shape,
field name, error or exit code, stderr text, golden or core-freeze artifact changes.
Rollback is the selector repair commit; rolling back AZU-V0-005 itself requires
rolling back the two hidden-package declarations as well.

## Verification and retained limits

The installed `corvint affected --base
8e2c13aa926fcaf1731fae71b27c41c41c8f09af` selected both `internal/gateselect`
and `internal/liveverify/affected` before testing. All Go tests use the configured
`script/heavy.sh` semaphore, one whole package per command, `-count=1 -timeout 30m`.
Raw evidence remains outside the repository in
`/home/russell/lanes/azu-small-evidence/repair2/`; final results and digests follow.

The host's selected-run exit 3 remains INCOMPLETE: no qualified declared-consumer
adapter, and `go list ./...` omits explicitly addressable hidden packages. Repairing
that runner contract is outside this verifier failure. Whole-package execution
provides the requested fallback evidence; no successful selected-run claim is made.

Reuse the original pre-change capture. Preserve the earlier whole CLI failure,
external qualification limits and local outcome `failed`. Independent review is
reserved to the orchestrator. Repository-wide gate, selected-run execution,
Darwin qualification, publication, Tasks writes and sealing remain NOT_RUN under
the repair charter and existing evidence limits. Final CEM binding and dogfood-check
use the Russell Lewis identity and stop before sealing.

## Terminal evidence

- Whole `internal/gateselect`: PASS, 2.128s, including existing tests.
- Whole `internal/liveverify/affected`: PASS on retry, 106.775s, including
  `TestDriverSelectionCoversTheShadow_V13067` and the unchanged live-budget gate.
  The first run failed only `TestIncrementalSelectionMeetsTheLiveBudget`:
  best p95 151.373493ms and 5.34 probes, above 100ms and 5.00, under host contention.
  Its 180.359s package failure is retained; no threshold or assertion was weakened.
- `make unbounded-readers-check test-read-scopes-check line-citations-check
  requirement-definitions-check traceability-tests-check`: PASS under heavy admission,
  16.4s after 105s of configured host-load admission. No lock-directory override.
- `script/gen-spec-requirements.sh --check`: PASS; no generated metadata drift.
- Current-tree `affected --base` also selects both tested packages, retaining three
  unknowns (build-constraint variants, nested modules, unmodelled shell callers).
- `git diff --check`: PASS. Post-commit CEM/check results belong in the handback;
  this entry does not claim independent review or complete lane qualification.

SHA-256 digests (raw files remain in the external repair evidence directory):

- `test-gateselect.log` (592 bytes): `15244773f94b888b1735c13618d76e17fad8cddd90268b160d679ff8cca453a8`.
- `test-affected.log` (1734 bytes): `e9b142e0652ba7c252c9ce5e50000b27d8e97552688085f934da4a82df1aadc7`.
- `test-affected-retry.log` (798 bytes): `75866cb14b4aa10bec2f9e6af628caf1e51a0eaa911253a9d7d8beff180c2cf1`.
- `scoped-gates.log` (474 bytes): `1cfd1071669a67899186d94b3295b3fdb8f3baeb6f201c0a01981017ded76f66`.
- `affected-current.json` (88648 bytes): `56488ed084c5442d82a1545eb21e7b87b075782e3d201e1c47168b7bda44adb7`.
