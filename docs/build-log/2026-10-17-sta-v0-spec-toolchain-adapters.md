# STA-V0 spec toolchain adapters: forms, context classification and host packaging

Date: 2026-10-17. Tickets: V1-1693 (spec), V1-1694, V1-1695, V1-1696. Decision 0540.

## What landed

- `STA-V0-002` to `STA-V0-006` and `STA-V0-010` (V1-1694): the `openspec-spec`, `openspec-delta`
  and `speckit-spec` OCM intent forms in `internal/lrfrepo/ocm_toolchain_forms.go`, the
  `nativeNames`, `excludedRequirements` and `nativeStatus` prepare envelope members, and fixture
  projects under `interop/spec-toolchains/fixtures/`. `internal/lrfrepo` gains the read scope
  `interop/spec-toolchains/fixtures/` for `TestSTAV0010_FixturesDerive`. `OIF-V0-001` lists the
  three forms. The use-case receipts were repinned mechanically for the `cmd/corvint/ocm.go` edit.
- `STA-V0-007` (V1-1695): path classification in `internal/contextindex/toolchain_status.go`, a
  `native_state` result member, and the analyzer schema bump to `corvint-analyzer/122`.
- `STA-V0-008` and `STA-V0-009` (V1-1696): `protocol/integrations/speckit/corvint/` and
  `protocol/integrations/openspec/`, with static tests in `protocol/integrations`.

The requirements were accepted under decision 0540 (owner, 2026-10-17); the readings below were
applied under the owner's 2026-10-11 standing auto-accept of lane requirement amendments.

## Readings and deviations

- OpenSpec names drop a closing `#` run only after a blank, as upstream; the spec regex was amended.
- Spec Kit headings accept the template's `*(mandatory)*` annotation; `nativeStatus` is trimmed and
  omitted when absent; the span is the whole blob; FR/SC IDs over 64 bytes fail
  `invalid-obligation-id`; a delta with only REMOVED/RENAMED entries fails `missing-requirements`;
  OpenSpec `## Requirements` is matched case-sensitively (stricter than upstream, refuses only).
- Context: an archive directory without the date prefix is still `archived`, with no date; Spec Kit
  presence is decided from admitted sources under `.specify/`; the Spec Kit status is the whole
  header text lowercased; only files already indexed as documents get a path status.
- Spec Kit `effect` lives under `extension:`. OpenSpec `apply` ignores `rules`, so the snippet
  carries the steps in `rules.tasks`, `context` and `operations.apply.guidance`. `corvint review`
  refuses a dirty tree; the commands report the refusal.

## Not run

- The upstream CLIs (`openspec validate --all --strict`, `specify extension add --dev`) were not
  run against the fixtures or packages; that is `STA-V0-011` (V1-1697).
- `corvint affected` selected 83, 120 and 77 units for the three lanes; only the owning packages'
  focused tests ran. `make gate` was not run (owner standing preference for scoped work).
- Delivery stays experimental while `OIF-V0` is proposed.
