# 2026-10-18 V1-1794, V1-1700: Ruby autoload constant impact on by default, two hops, advisory

GitHub #1129 (and the #1130 fixture). V1-1674 added Ruby named-constant reverse references
only under `--language-profile non-go-syntax-v0`, one hop deep. On the #1130 Rails fixture the
default `corvint impact app/api_clients/foo_client.rb` therefore returned only the changed path
plus `reverse-import-rule-unavailable`, and never reached `spec/services/bar/baz_service_spec.rb`,
the spec of the service that calls `FooClient`. V1-1700: `isTestPath` did not classify RSpec paths,
so the Ruby reading carried its own private `rubyTestPath`.

The new requirements `NGI-V0-033..040` in `docs/specs/non-go-impact-v0.md` are accepted under the
owner's standing auto-accept of lane requirements (2026-10-11); V1-1794, V1-1700. They amend
`NGI-V0-003`/`NGI-V0-020` (the opt-in) and `NGI-V0-019` (the baseline unavailable-rule row). No
CAL-V0 change. IDs were checked against origin/main `70a21da5d` (highest `NGI-V0-021`) and the open
PRs: #1134 (V1-1796) reserves `NGI-V0-027` and `corvint-analyzer/124`, so this lane leaves a gap of
five and takes `NGI-V0-033..040` and `corvint-analyzer/125` (`NGI-V0-040` was added for the Codex
round-1 index bound below).

## Design

- Default on (`NGI-V0-033`): a path request naming any Ruby path is answered under the syntax
  frontier, byte-identical to `--language-profile non-go-syntax-v0`. Requests with no Ruby path
  keep their bytes; a mixed Ruby + JS request is answered wholly under the syntax profile.
- Second hop (`NGI-V0-034`): each first-hop referrer that the mapper itself maps to a constant D is
  followed once more to the sources naming D. Second-hop rows score 475 (`reference`) / 450
  (`test`), below every first hop, carry `via` (the first-hop path), confidence `low`, and a reason
  naming both constants. A path is reported once (its first hop wins); no third hop. References
  are indexed once per load (constant and namespace prefix to sites), and each constant is
  traversed once at the second hop however many paths map to it (`foo12.rb`, `foo_12.rb` and
  `foo1_2.rb` all name `Foo12`), so both hops are linear in the indexed references.
- Index bound (`NGI-V0-040`): an unbalanced file resolves a reference against at most 64 of the
  namespaces it opens, and the index retains at most 1,048,576 sites. A reference either bound
  cuts counts in the new `ruby-reference-index-bounded` unknown on every mapped changed Ruby path;
  nothing is dropped silently.
- Advisory (`NGI-V0-035`): every autoload-convention row carries `"advisory": true` and counts in
  `coverage.advisory_results`; path and range receipts add the uncertainty line "Ruby
  autoload-convention references are advisory syntax hints: a named constant does not establish
  load order, runtime dispatch or test closure". The learned-path advisory line is now added only
  when a learned-path row is present.
- Frontier (`NGI-V0-036`): a Ruby path the mapper resolves no longer carries
  `reverse-import-rule-unavailable`; unmapped Ruby paths (and every path under an invalid
  declaration) keep it. `dynamic-dispatch-unresolved` stays on every Ruby path.
- Range (`NGI-V0-037`): `compileRangeImpact` attaches the same rows (both hops), Ruby unknowns and
  `ruby_autoload` for each admitted Ruby path, merged by the existing range deduplication.
- `isRubyTestPath` (`NGI-V0-038`, V1-1700): for the Ruby reading's rows and the mapper's
  configuration scan, a `.rb` path named `*_spec.rb` or below a `spec/` directory is a test, on
  top of the shared rule. The shared `isTestPath` is unchanged (the first push changed it, which
  Codex round 1 found moved a Go receipt sharing a feature marker with `spec/example_spec.rb` from
  `medium`/`source-marker` to `high`/`test-marker`).
- Package (`NGI-V0-039`): the path-to-constant mapper moves to `internal/rubyautoload`
  (`New(Repository)`, `Mapper.Constant/Path/Known/Camelize/Report/DeclarationInvalid/
  ConfigUnresolved`, `DefaultRoots`). It performs no I/O: the caller supplies paths, the
  declaration text and sources, and the strict duplicate-key JSON check stays with the caller.
  V1-1795 can import it. The Tasks source-archive subset adds `internal/rubyautoload/`
  (decision 0397 addendum), and the analyzer audit (`IDX-SNAP-V0-017`) now includes it.

## Failure modes

- Invalid or unreadable declaration: maps nothing, keeps `reverse-import-rule-unavailable` and
  `ruby-autoload-declaration-invalid`; no fallback to the defaults.
- Unmapped changed path (outside a root, ignored, or non-camelizable): no rows, keeps
  `ruby-constant-unmapped` and `reverse-import-rule-unavailable`.
- Metaprogrammed names stay counted unknowns (`NGI-V0-019`), never rows, at either hop.
- A first-hop referrer that is not itself mapped (a spec, a script) is not followed.

## Receipt and analyzer byte changes (recorded per V1-1700 AC)

- Default path receipts with a Ruby path: now equal to the syntax profile (`language_profile`,
  `unknowns`, `ruby_autoload`, reverse-reference rows).
- Ruby autoload rows: new `advisory` member; `advisory_results` / `authoritative_results` move;
  new uncertainty line; new second-hop rows with `via`.
- Mapped Ruby paths drop the `reverse-import-rule-unavailable` unknown row.
- Range receipts admitting a Ruby path: Ruby rows, unknowns, `ruby_autoload`, uncertainty line.
- RSpec classification applies only to the Ruby reading's rows; every other `isTestPath` use
  keeps its base classification. `TestRubySpecPathNonRubyRequestBaseBytes` compares a Go default
  and a JS/TS syntax request (feature marker shared with `spec/example_spec.rb` and
  `spec/support/demo_helper.rb`) against canonical bytes produced by the base commit
  `a7ad984c9` (`testdata/ruby-spec-marker-base/receipts.json`); with the old shared rule it fails.
- `analyzerSchemaID` moves to `corvint-analyzer/125` (contextindex sources and the new
  `internal/rubyautoload` input); the `IDX-SNAP-V0-017` digest is repinned. Pack facts are
  otherwise unchanged.
- Go-only and JS/TS-only receipts keep their bytes: `conformance/cli-parity-v0` goldens
  (impact-go, impact-web, impact-python), `TestNonGoRangeImpactLegacyGoBytes`,
  `TestNonGoImpactCLIProfile` (JS/TS default unchanged; Go default equals profile) and
  `TestRubyAutoloadDefaultOn` (`web/util.js` default carries no profile members).

## Real CLI output

A scratch clone of the #1130 fixture (`/private/tmp/cem10-build/gh-ruby1130-fixture`, left
unmodified), `corvint` built from this branch, `corvint index` then
`corvint impact app/api_clients/foo_client.rb`:

```text
language_profile non-go-syntax-v0
ruby_autoload {"ignored":["lib/assets","lib/tasks"],"inflections":[],"roots":["app/api_clients","app/services"],"roots_source":"default"}
path      app/api_clients/foo_client.rb          1000
reference app/services/bar/baz_service.rb         525 advisory medium line 4
test      spec/api_clients/foo_client_spec.rb     500 advisory medium line 3
test      spec/services/bar/baz_service_spec.rb   450 advisory low    line 3 via app/services/bar/baz_service.rb
unknowns  [dynamic-dispatch-unresolved count 1]   (no reverse-import-rule-unavailable)
coverage  advisory_results 3, authoritative_results 1
```

`corvint impact --base 3306177aaa7fd53cc1bfaa34e8641e83fc036bba` (the fixture's one-commit range
over `foo_client.rb`) returned `range-impact` with the same four rows, the same `ruby_autoload`
block, `dynamic-dispatch-unresolved` only, `advisory_results` 3 and the advisory uncertainty line.

## Rollback

Revert the change: remove the `rubyImpactRequested` selection, the second-hop loop, the
`advisory` count and line, the `mapped` guard, `rangeRubyAutoload`, `isRubyTestPath`, the index
bounds and `ruby-reference-index-bounded`, restore the mapper into `ruby_autoload.go`, drop the
export prefix and the 0397 addendum, and restore `corvint-analyzer/123` with its digest.

## Residuals

- `reverseImportProfileGap` (keyed on the suffix-only `ImpactRuleNamed`, `GPK-V0-027`) still adds
  "reverse-import results for 1 changed paths with no named resolution rule are outside the native
  Go impact profile" for a mapped Ruby path; changing it moves the CLI budget fallback too.
- The MCP impact description (`internal/mcp/bridge/bridge.go`) is golden-pinned and still says
  "Ruby (.rb): path, markers, reverse imports unknown"; it is unchanged here.
- In a Ruby request, rows other than the reading's (feature markers, reverse imports) still use
  the shared rule, so an RSpec path among them is not classified as a test.
- `corvint affected` selects the fixture's `testdata/**/spec/*.rb` files as RSpec units of the
  Corvint repository.
