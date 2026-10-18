# 2026-10-18 V1-1795: Ruby affected narrows by autoload constant edges

This is GitHub #1130 slice S2. On the #1130 Rails fixture, `corvint affected` raised the coarse
`ruby:rails-autoload` frontier and returned `UNKNOWN_SCOPE` for any Rails application with tests.
Because `Bar::BazService` reaches `FooClient` only through Zeitwerk, it could never select
`baz_service_spec` from a `foo_client.rb` change.

The new requirements `RUBY-AFFECTED-V0-009..012` (`docs/specs/ruby-affected-test-adapter-v0.md`)
are accepted under the owner's standing auto-accept of lane requirements (2026-10-11) for V1-1795.
There is no CAL-V0 change. The IDs were checked against origin/main `59cc560b9`, whose highest is
`RUBY-AFFECTED-V0-008`, and against open PRs #1105, #1141, #1142 and #1143, none of which carries a
RUBY-AFFECTED ID.

## Design

- **Seam (additive).**
  - `Unit.AdvisoryImports` is a sorted list of edges. It is part of the digest projection only when
    non-empty (`omitempty`), so existing Go and Python graphs keep their digests.
  - `Select` walks explicit edges first, then advisory edges, at each BFS level. A test reached
    through an advisory edge, directly or transitively, carries `ADVISORY_CONSTANT_REFERENCE`.
    `unitsReachingTests` follows both edge kinds.
- **Ruby plugin (`autoload.go`).**
  - It reuses the `internal/rubyautoload` mapper, the same one the V1-1794 impact path uses. For
    each observed file it masks comments and strings, then resolves constant references against
    the namespaces the file opens.
  - It resolves associations through a literal `class_name:` or the camelized (singular) name.
  - It adds `config/**` edges to every test. Route-dispatched tests (requests, system, features,
    integration, routing and controllers) also get `app/controllers/**` and `app/helpers/**` edges.
  - The frontier is kept only where the edges cannot bound the application:
    - `constantize`, `safe_constantize` or `const_get`;
    - a polymorphic, non-literal or unmapped association;
    - a missing or nested `config/application.rb`;
    - an invalid `.corvint/ruby-autoload.json`;
    - an unresolved inflection or autoload configuration.
- **Suite helpers.**
  - `rails_helper`, `spec_helper` and `test_helper` stay source units.
  - Each suite test imports its helpers and `support/**`.
  - The suite's own `support/**` glob is covered by those edges and no longer raises dynamic-load.
  - Shared examples and shared contexts in support files no longer raise the dynamic-test frontier.

## Evidence

- Affected plan: `corvint affected --base 59cc560b9...` returned `UNKNOWN_SCOPE` and selected 99
  Go packages, because the seam is widely imported. Its unknowns were
  `go:build-constraint-variants` and `go:nested-module-frontier`.
- Focused tests were run per the lane charter. Everything else in the plan is `NOT_RUN`.
- **Ruby package** (`./internal/liveverify/affected/ruby`, all tests, PASS):
  - the #1130 fixture selects `foo_client_spec` with `DEPENDENCY_PATH` and `baz_service_spec` with
    `ADVISORY_CONSTANT_REFERENCE`;
  - a `baz_service.rb` change selects only its own spec.
- **Seam package** (`./internal/liveverify/affected`, all tests, PASS): covers the digest-projection
  field parity and conformance tests.
- **Go/Python bytes:** `./internal/liveverify/affected/golang`, `/python`, `/languages`,
  `./tools/gate-affected-select`, and the `cmd/corvint` frozen-profile and affected-golden tests
  (`TestCoreVerbsEmitTheFrozenProfiles`, `TestAFPV0066*`, `TestAFUV1FlowsQueryGoldens`) all PASS
  unchanged.

## Residuals

- In a real application, `Bundler.require(*Rails.groups)` in `config/application.rb` still raises
  `ruby:dynamic-load`. Gemfile handling belongs to a concurrent lane.
- Heredoc bodies are not masked, and nesting is approximated by opened namespaces. Both cause
  over-selection only.
- Route-dispatched selection relies on directory convention. Views and templates remain unowned.

## Rollback

Revert this change. The Ruby plugin returns to the coarse frontier, and no non-Ruby digest moves.
