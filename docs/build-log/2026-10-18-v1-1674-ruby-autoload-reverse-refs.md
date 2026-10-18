# 2026-10-18 V1-1674: Ruby named-constant reverse references by autoload convention

GitHub #1077. In a Rails/Zeitwerk application a controller names `Thing` with no `require`,
so syntax-profile impact on `app/models/thing.rb` returned only the path and the baseline
`dynamic-dispatch-unresolved` / `reverse-import-rule-unavailable` rows. This lane extends the
existing `non-go-syntax-v0` path impact (no parallel path) with named-constant reverse
references.

The new requirements `NGI-V0-016..021` in `docs/specs/non-go-impact-v0.md` are accepted under
the owner's standing auto-accept of lane requirements (2026-10-11); V1-1674. No CAL-V0 change.
IDs were checked against origin/main (highest `NGI-V0-015`) and open PRs #1081, #1084..#1087
(none touch the NGI spec or `internal/contextindex`).

## Design

- `internal/contextindex/ruby_autoload.go` maps each indexed Ruby path below an autoload root
  to the constant its path names (ActiveSupport camelize with literal acronyms and Zeitwerk
  whole-basename overrides). Default roots: `app/<dir>` except `assets`, `javascript` and
  `views`, `app/<dir>/concerns`, and `lib` with `lib/assets` and `lib/tasks` ignored.
- Minimal config surface: an optional committed `.corvint/ruby-autoload.json`
  (`ruby-autoload/0`: `roots`, `inflections`). It replaces the default roots; an invalid one
  fails closed (maps nothing, `ruby-autoload-declaration-invalid`) instead of silently falling
  back. Literal `inflect.acronym "API"` and `.inflect("html_parser" => "HTMLParser")` calls are
  read from non-test source; non-literal forms, `collapse`, and undeclared root configuration
  are counted in `ruby-autoload-config-unresolved`. Ruby is never evaluated.
- References are scanned from the shared Ruby lexical mask (`FlowCodeLines`), with a bounded
  stack of `module`/`class` and other `end`-closed blocks, so `Invoice` inside `module Billing`
  resolves to `Billing::Invoice` only when that constant is mapped. An unbalanced file widens
  (resolves under every namespace it opens) rather than dropping a reference.
- A referencing file is one result of kind `reference` (575) or `test` (525), strictly below
  every import score (direct 700/650, type-only 600/550), evidence confidence `medium`,
  authority `syntax`. `constantize`, `const_get`, `send` and friends, constant-shaped strings
  and association symbols are counted unknowns, never references.
- The receipt's `ruby_autoload` member names the roots, ignored directories, roots source and
  every inflection applied with its path and line. It appears only for a syntax-profile request
  with a Ruby path; the historical default receipt and range receipts are unchanged.
- All non-test `internal/contextindex` files are analyzer inputs, so `analyzerSchemaID` moves
  to `corvint-analyzer/122` with the `IDX-SNAP-V0-017` audit digest repinned. Pack facts and
  encoding are unchanged.
- `impact --help` names the Ruby row's autoload constant references (path, profile only)
  (`NGI-V0-005`). The MCP description is unchanged: it still states, accurately, that Ruby
  reverse imports are unknown; the MCP tools-list goldens keep their bytes.

## Real CLI output

Fixture: a fresh Git repository with `app/models/thing.rb` (`class Thing`),
`app/controllers/things_controller.rb` (`Thing.find`), `app/controllers/api/things_controller.rb`
(`module API`, `::Thing.all`, `"Thing".constantize`), `config/initializers/inflections.rb`
(`inflect.acronym "API"`) and `spec/models/thing_spec.rb`; no `require`. Binary built from this
branch; `corvint index`, then `corvint impact --language-profile non-go-syntax-v0
app/models/thing.rb` (rc 0). Results (kind, id, score, first evidence line/confidence/authority):

```text
path      app/models/thing.rb                       1000  1 authoritative git-tree
reference app/controllers/api/things_controller.rb   575  4 medium syntax
reference app/controllers/things_controller.rb       575  3 medium syntax
test      spec/models/thing_spec.rb                  525  1 medium syntax
```

`ruby_autoload`: `{"ignored":["lib/assets","lib/tasks"],"inflections":[{"constant":"API",
"kind":"acronym","line":2,"path":"config/initializers/inflections.rb","word":"api"}],
"roots":["app/controllers","app/models"],"roots_source":"default"}`. Unknowns:
`dynamic-dispatch-unresolved` 1, `reverse-import-rule-unavailable` 1,
`ruby-constant-lookup-dynamic` 1, `ruby-constant-name-literal` 1. A repeat run was
byte-identical (`cmp`). The default `corvint impact app/models/thing.rb` receipt returned only
the path row and no `ruby_autoload` member.

## Limits

Syntax authority only: no ancestor-chain or `include` lookup, no collapsed directories, no
runtime-configured roots, no `require` resolution; acronyms apply to every camelized word.
`isTestPath` does not classify `spec/` or `_spec.rb` Ruby paths, so the lane uses a local
`rubyTestPath`. Rollback: remove `ruby_autoload.go` and its `impact` wiring.
