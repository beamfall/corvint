# Ruby route-dispatch edges get their own witness and tier 2

Date: 2026-10-22. Lane: claude/ruby-tiers-oc-1022 (base 14dc0e985). Ticket: V1-2448, GH #1565.
Requirement `RUBY-AFFECTED-V0-049` in `docs/specs/ruby-affected-test-adapter-v0.md` is accepted
under the owner's standing auto-accept (2026-10-11).

## Problem

In a Rails application, `corvint affected --path app/controllers/a_controller.rb` put every
route-dispatched spec in tier 1. A 545-spec monolith gave `tier0=0 tier1=555 tier2=1`. A reader
could not tell the boot fallback apart from a spec that names the controller
(`RSpec.describe Foo::BarController`).

## Cause

`bootEdges` (`RUBY-AFFECTED-V0-011`) gave every request, system, feature, integration, routing and
controller test an advisory edge to every file under `app/controllers/` and `app/helpers/`. The
traversal marked those edges `ADVISORY_CONSTANT_REFERENCE` with a two-unit chain. `AFP-V0-162`
puts that witness in tier 1, although no constant is involved.

## Fix

- `affected.Unit` has a new edge list, `RouteDispatchImports`. It is part of the digest only when
  non-empty (`routeDispatchImports`), so non-Rails graphs keep their bytes. `Select` traverses it
  after explicit and advisory edges at each depth. A chain that contains one is witnessed with the
  new kind `ROUTE_DISPATCH_BOOT`.
- The Ruby plugin puts the controller and helper fan-out on that list. It leaves out any target the
  test already reaches through an import or a constant, so the spec that names a controller keeps
  its advisory witness and stays in tier 1. The `config/` boot edges remain advisory.
- `tierAffectedSelections` is unchanged: a `ROUTE_DISPATCH_BOOT` unit falls to tier 2. Selection is
  unchanged.
- `conformance/ruby-profile-v0/rows.json`: the `request-spec` and `controller-spec` rows now expect
  `ROUTE_DISPATCH_BOOT` for `spec/requests/users_spec.rb`. That spec names no controller.
- Skipped (optional precision): narrowing a controller-type spec to its constant edge. A controller
  spec can render views or call helpers that belong to other controllers, so dropping the fan-out
  could miss a test.

## Evidence

- `TestRubyRouteDispatchSpecsAreTierTwo_RUBYAFFECTEDV0049` (`cmd/corvint`): with the base
  production files restored, it fails with `beta_flow_spec tier=1 kind=ADVISORY_CONSTANT_REFERENCE;
  tier0=0 tier1=3 tier2=0`. With the fix it passes.
- `TestRouteDispatchEdgesCarryTheirOwnWitness_RUBYAFFECTEDV0049` (`affected/ruby`), and
  `TestRubyProfileAffectedRows_RUBYAFFECTEDV0014` with the updated rows.
- Focused packages pass: `affected`, `affected/ruby`, `affected/languages`, `rubyprofile`,
  `conformance`, `plansnapshot`, `gateselect`, and `cmd/corvint -run 'Affected|Tier|Ruby|RUBY'`.
- V1-1851 is not met at base: `corvint impact` for `users_controller.rb` still lists no request spec
  (the impact rows are unchanged).

## Rollback

Revert the change. Route edges return to `advisoryImports` with `ADVISORY_CONSTANT_REFERENCE` and tier
1, and the two conformance rows go back to their old values. Selection does not change in either
direction.
