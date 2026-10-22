# Declared flow inventory for docs flows generate (V1-2313, GH #1478)

Lane flowinv-1021q, base ef2a01db52d5436c262725d136060c9eb95e8580.

## Problem

`docs flows generate` turned every extracted declaration in `--scope` into a flow (56 flows for one
Rails service directory), so a project's own reviewed flow inventory could not drive `generate` and
`check`.

## Decisions

- Optional `--flows FILE` (`corvint-flow-inventory/1`), strictly decoded and refused on any
  malformed, duplicate or overlapping input (AFU-V1-101, AFU-V1-106). Requirements AFU-V1-101..106
  are accepted under the owner's standing auto-accept (2026-10-11); numbered from 101 to leave room
  for the concurrent #1479 lane in the same spec.
- Each declared flow with a resolved member is one flow with identity `inventory:<id>` that
  aggregates members' calls, claims and corpus context; claim and paragraph identities are keyed by
  flow id, member path and declaration name (AFU-V1-102).
- Unresolved members are reported with reasons; a flow with none resolved is reported, not generated
  (AFU-V1-103). Unassigned declarations are listed, never turned into flows (AFU-V1-104).
- The declared inventory is retained in the generation manifest (`inventory`, omitted when absent),
  so Open and check rederive the same identities (AFU-V1-105).
- The per-declaration emission moved unchanged into `flowBuilder` (`flowbuild.go`) so both modes share
  it. Inter-flow same-file and same-name relations are not derived for inventory flows (gap recorded).

## Evidence

- `TestFlowDocsInventoryAggregatesDeclaredFlows`, `TestFlowDocsInventoryCheckIdentities`,
  `TestFlowDocsInventoryRefusals`, `TestFlowDocsCLIInventory` pass; with the check rederivation or the
  unresolved report removed, the first two fail.
- Byte parity: the base and new binaries produce identical output directories without `--flows` for
  `conformance/ruby-profile-v0/rails-app`, `integrations/opencode/src` and `extensions/vscode/src`;
  the existing flowdocs suite passes unchanged.

## NOT_RUN

Tier 2 affected units are left to batch CI; the full `cmd/corvint` package and `make gate` were not
run.

## Rollback

Revert this change: remove `--flows`, `flowinventory.go` and the spec section. `flowbuild.go` can stay
or be inlined; manifests without an inventory are unaffected.
