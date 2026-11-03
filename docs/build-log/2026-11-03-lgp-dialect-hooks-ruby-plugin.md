# Dialect algorithm bodies move behind plugin hooks; Ruby first (V1-4408)

Date: 2026-11-03. Ticket: V1-4408. Requirements: LGP-V0-023 to LGP-V0-027.

## Problem

After LGP-V0-015 to LGP-V0-022, `internal/contextindex` read each source's dialect from the
language plugins. It still held the dialect algorithm bodies, and they compared the dialect names
they implement. Those comparisons were 39 of the core-name ratchet literals: `ruby` 22,
`typescript` 8, `kotlin` 4, `swift` 3, `rspec` 1 and `minitest` 1.

## Decision

- **Hook leaf.** A stdlib-only leaf, `internal/langplugin/dialect`, holds `Impact` and `Region`
  hooks keyed by the dialect name a plugin declares. A plugin imports only the two provider-free
  leaves (`decl`, `dialect`), so LGP-V0-020 isolation holds. Corvint-tasks still links no affected
  provider (NGI-V0-988); `go list -deps ./cmd/corvint-tasks` shows the two leaves and
  `internal/rubydialect`, and the Tasks source export lists all three.
- **Rules.** Every impact and module dialect declares a `Rule` (`decl.Rule`), which `Validate`
  enforces. Core switches on that rule rather than on a name. This removed the kotlin, swift and
  typescript literals without moving their module readers. Those readers have no language-specific
  lexer of their own.
- **Ruby package.** `internal/rubydialect`, a new Ruby plugin directory, owns the following and
  registers them from `init`:
  - the scanner;
  - the impact profile;
  - the region normalizer and the symbol extractor;
  - the verification command, which reads the plugin's `Dependencies`.

  `internal/contextindex` blank-imports it. A blank import keeps registration compiled in, which
  invariant 7 requires; there is no run-time plugin load.
- **What stays in core.** The Ruby constant-reference rule (`ruby_autoload.go`) stays. It reads the
  scanner's exported word helpers rather than copying them. Moving it needs a hook for the reverse
  import graph, which is V1-4412.
- **Fail closed.** A declared dialect with no registered hook fails closed (LGP-V0-026).
  `ImplementedDialects` reads the registry, so an unlinked plugin fails the LGP-V0-016 conformance
  test.
- **Web lexer.** The web lexer stays in core. The flow, setup and registry readers share it. Core
  registers it by rule under the TypeScript plugin's declared dialects. Moving it is V1-4412.

## Evidence

- The ratchet (`TestCoreNameLiterals_LGPV0009`) now has no `internal/contextindex` entries except
  `go` 2 and `java` 1.
- `make plugin-test PLUGIN=ruby` and `PLUGIN=typescript` pass.
- `internal/contextindex` passes except `TestDeferredGobBodyTruncatedInPlaceIsRefused_IDXSNAPV0811`.
  That test fails identically at the base commit on darwin, which is V1-4413.
- `appmap`, `flowdocs`, `doccorpus`, `swiftimpact` (NGI-V0-988), `companionrelease` (the PUB-V0-012
  offline Tasks archive) and `make windows-vet` pass.

## Rollback

Revert the change. No state, wire format or persisted reading depends on the hooks.

## Remaining

The `internal/testsource` literals and the `cmd/corvint` literals keep their reasons under V1-4405.
