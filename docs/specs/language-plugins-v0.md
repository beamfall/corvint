# Language Plugins V0

Owner: Russell Lewis
Date: 2026-11-03
Intent status: accepted
Delivery status: experimental
Authoritative inputs: owner request of 2026-11-03 (each language acts as a plugin, each testing and
coverage tool as a plugin of that language, so Corvint core stays language-agnostic and only uses
the plugins); tickets V1-4404 and V1-4405 (owner, 2026-11-03: the other language-specific interactions belong in the plugins too, and a language and its tooling should be testable apart from the rest of Corvint); `affected-plan-v0.md` (the `affected.Language` seam);
`test-runner-execution-v0.md` (TRE-V0-081); `map-coverage-join-v0.md`;
`live-proof-carrying-verification-v0.md` (LPCV-V0-950..963); `AGENTS.md` invariant 7.

## Agent digest
- Claim: One in-process manifest per language declares its paths, tools and dialects, which core reads instead of naming languages; each plugin tests apart.
- Status: accepted / experimental
- Exists: `internal/langplugin` (manifest, `Validate`, `Providers`), the table-driven `internal/testrunner/registry`, and the conformance and core-name ratchet tests.
- Blocked on: nothing. Ruby dialect bodies live in `internal/rubydialect` behind `internal/langplugin/dialect` hooks; moving the web lexer and adding non-Go coverage collectors are later slices.
- Read next: Requirements; Failure modes; Boundaries and rollback.

## User job and verified baseline

The owner wants a language to be one plugin, and each test runner, coverage collector and mutation
tool to be a sub-plugin of its language, so that adding or changing a language touches its plugin
and core code reads languages only through the plugin seams.

Baseline at `origin/main` 463f0dd5f, from the code named here:

- `affected.Language` (`internal/liveverify/affected/unit.go`) is a good seam with optional
  capability interfaces, and `internal/liveverify/affected/languages` holds the one provider list.
- The `analyzercap` subprocess protocol is the external analyzer seam; each analyzer is a
  `cmd/corvint-analyzer-*` binary.
- `internal/testrunner/registry` dispatched by a hand-written switch over five runner families,
  duplicated in `Build` and `Parse`, and named `rspec` in its generic path.
- One language is spread over up to nine packages (Ruby: `analyzerruby`, its analyzer binary,
  `affected/ruby`, `rubytestprovider` and its binary, `rubyprofile`, `rubyautoload`,
  `liveverify/rbmutate`), and nothing records which packages belong to which language or checks
  that a language has every piece.
- Language and runner names are string literals in core packages, for example 44 in
  `internal/contextindex`, 37 in `internal/testsource` and 13 in `cmd/corvint` (non-test files,
  exact literals equal to a plugin or runner name).

The simpler alternative is to keep the providers list and the registry switch and document the
per-language packages in prose. It is rejected because prose cannot fail when a piece goes missing.

## Requirements

- `LGP-V0-001`: (accepted under the owner's standing auto-accept (2026-11-03); V1-4404) Each
  language is one `langplugin.Manifest` in `internal/langplugin`, registered in-process in one
  ordered list. A manifest names the plugin, the repository directories it owns, its
  affected-selection provider (`affected.Language`), its `analyzercap` analyzer binaries, and its
  sub-plugin lists: test runners (by testrunner registry name), test providers, coverage
  collectors and mutation tools. Languages with no affected provider (SQL, shader, HTML/CSS,
  structured data) are plugins too, so every analyzer and runner has a language.
- `LGP-V0-002`: (accepted under the owner's standing auto-accept (2026-11-03); V1-4404) The pieces
  are the closed set `provider`, `analyzer`, `runners`, `test-provider`, `coverage`, `mutation`.
  Every manifest declares each piece either present or in `Absent` with a non-empty reason, never
  both, so a missing piece is never silent.
- `LGP-V0-003`: (accepted under the owner's standing auto-accept (2026-11-03); V1-4404) Owned
  directories are disjoint across plugins and each exists. A directory several languages use
  (`internal/analyzernativebridge`, `internal/extmutate`, `internal/linecov`) is owned by none and
  appears only in sub-plugin paths, each of which exists.
- `LGP-V0-004`: (accepted under the owner's standing auto-accept (2026-11-03); V1-4404) Each
  declared analyzer is a `cmd/<name>` binary, declared by exactly one plugin, and every
  `cmd/corvint-analyzer-*` directory is declared by a plugin. Registration is in-process; the
  `analyzercap` protocol stays the analyzer's external form.
- `LGP-V0-005`: (accepted under the owner's standing auto-accept (2026-11-03); V1-4404)
  `languages.All()` returns `langplugin.Providers()`: one fresh provider per plugin that declares
  one, in registration order, which equals the previous hand-written list. A provider's `Name()`
  equals its plugin name, and every provider package under `internal/liveverify/affected/` (other
  than `languages` and `testdata`) is owned by a plugin.
- `LGP-V0-006`: (accepted under the owner's standing auto-accept (2026-11-03); V1-4404) Each
  declared runner resolves through `registry.Lookup` to a family with `Build` and `Parse`; output
  normalization stays the shared `testrunner.Normalize`. Every runner the registry registers is
  declared by exactly one plugin. `langplugin.RunnerLanguage` answers a runner's language.
- `LGP-V0-007`: (accepted under the owner's standing auto-accept (2026-11-03); V1-4404)
  `langplugin.Validate` refuses a manifest with an empty or padded name, no owned path, a piece
  neither present nor absent, a piece both present and absent, an absence without a reason, an
  unknown piece, a provider whose namespace differs from the plugin name, a sub-plugin with no
  path, or an empty runner name, naming the plugin and the piece.
- `LGP-V0-008`: (accepted under the owner's standing auto-accept (2026-11-03); V1-4404) A
  conformance test validates every registered manifest and binds it to the repository by
  LGP-V0-003 to LGP-V0-006; a fixture test shows `Validate` failing for each LGP-V0-007 case.
- `LGP-V0-009`: (accepted under the owner's standing auto-accept (2026-11-03); V1-4404) A ratchet
  test counts, per core package, the string literals in non-test Go files that equal a plugin or
  runner name. The checked packages are `cmd/corvint`, `internal/contextindex`,
  `internal/testsource`, `internal/liveverify/affected/languages` and
  `internal/testrunner/registry`; the last two have a baseline of zero. A count above its
  baseline fails, naming the literal and saying to read the name from `langplugin` or the
  registry; a count below its baseline also fails, saying to lower the baseline, so a migration
  is locked in.
- `LGP-V0-010`: (accepted under the owner's standing auto-accept (2026-11-03); V1-4404) Runner
  dispatch in `internal/testrunner/registry` is one table of families (`Family`: name, runner
  names, `Build`, `Parse`, the runners that admit exec members). `Build`, `Parse`, `Runners` and
  `Lookup` read only the table; no per-family switch remains. No runner is registered by two
  families, and a test checks it.
- `LGP-V0-011`: (accepted under the owner's standing auto-accept (2026-11-03); V1-4404) The
  ExecPrefix, PathMap and Formatter members (TRE-V0-081) are admitted by the runners their own
  family declares: `dynamic.ExecMemberRunners()` names `rspec`, and `dynamic.Build` uses the same
  list. The registry refuses those members for any other runner with
  `execPrefix, pathMap and formatter are admitted only for <runners>`, the admitting runners joined
  by `, `, before it resolves the family, as before.
- `LGP-V0-012`: (accepted under the owner's standing auto-accept (2026-11-03); V1-4404) Behavior is
  unchanged: the same runner set, the same family for each runner, the same `unsupported runner`
  error for an unknown runner, the same provider list and order, and every existing selection,
  runner and linecov test passing unchanged.
- `LGP-V0-013`: (accepted under the owner's standing auto-accept (2026-11-03); V1-4404) A coverage
  collector is a `coverage` sub-plugin of its language. Go has one (`linecov-go`:
  `internal/linecov`, `internal/liveverify/gorunner`), and Ruby (`linecov-ruby`: `internal/rubylinecov`). A new collector, including the MCJ-V0 Ruby
  and TypeScript segment collectors, registers in its language's manifest and clears that
  language's `coverage` absence in the same change, rather than adding a separate per-language
  path that core names.
- `LGP-V0-014`: (accepted under the owner's standing auto-accept (2026-11-03); V1-4404) A mutation
  tool is a `mutation` sub-plugin: `mutate-go`, `pymutate`, `rbmutate`, and `extmutate` (shared by
  Ruby and TypeScript). A new test provider likewise registers as a `test-provider` sub-plugin.

- `LGP-V0-015`: (accepted under the owner's standing auto-accept (2026-11-03); V1-4405) A manifest
  declares the core readings that apply to its sources as `Dialects`, each a kind, a dialect name
  and the path suffixes it reads. The kinds are `impact` (the bounded syntax profile of
  `contextindex.NonGoImpactLanguage`), `module` (`contextindex.ModuleRuleLanguage`) and `region`
  (`contextindex.SymbolRegionLanguage`). `langplugin.DialectFor(kind, path)` answers each of those
  functions from the manifests, with the same values as before: `ruby`, `javascript` and
  `typescript` impact dialects; `swift`, `kotlin` and `csharp` module dialects; `ruby` and `web`
  region dialects. `Validate` refuses an unknown kind, an empty dialect name or an empty or padded
  suffix, and the conformance test refuses a suffix two plugins declare for the same kind. The
  dialect and dependency tables live in the provider-free leaf `internal/langplugin/decl`, which
  the manifests read, so `contextindex` reads them without linking a provider into `corvint-tasks`
  (NGI-V0-988).
- `LGP-V0-016`: (accepted under the owner's standing auto-accept (2026-11-03); V1-4405) The core
  package that implements a hook kind lists the dialects it implements, and a test in that package
  fails when a manifest declares a dialect the core does not implement, or the core implements one
  no manifest declares. `contextindex.ImplementedDialects(kind)` is that list for the three kinds
  above.
- `LGP-V0-017`: (accepted under the owner's standing auto-accept (2026-11-03); V1-4405) A manifest
  declares `Dependencies`: an ecosystem package name (a gem, an npm package) mapped to one of the
  plugin's own runners. `Validate` refuses a dependency whose runner the plugin does not declare,
  or an empty package name. `langplugin.DependencyRunners(language)` returns the map. Ruby declares
  `rspec`, `rspec-core` and `rspec-rails` (runner `rspec`) and `minitest`; TypeScript declares the
  npm packages of its runners. The RSpec gem advice in `cmd/corvint`, the Ruby verification command
  reading in `internal/contextindex` and the JS runner discovery of `corvint tests unselected`
  (TSA-V0-141) read these maps instead of naming the packages, with unchanged results.
- `LGP-V0-018`: (accepted under the owner's standing auto-accept (2026-11-03); V1-4405) A mutation
  sub-plugin that Corvint runs in process declares the source `Suffixes` it mutates
  (`mutate-go` `.go`, `pymutate` `.py`, `rbmutate` `.rb`); `extmutate` declares none.
  `langplugin.MutationLanguage(path)` returns the plugin whose engine mutates the path's suffix, and
  `corvint prove` reads it instead of its own suffix switch.
- `LGP-V0-019`: (accepted under the owner's standing auto-accept (2026-11-03); V1-4405) The ratchet
  (LGP-V0-009) reads every non-test Go file under each checked directory, subpackages included and
  `testdata` excluded. Each non-zero baseline entry carries a recorded reason, and an entry without
  one fails the test. The baselines fall to the literals this change could not move, each with its
  reason (a wire field or output value, a language-syntax token, a toolchain argv word, or a
  dialect's own algorithm body).
- `LGP-V0-020`: (accepted under the owner's standing auto-accept (2026-11-03); V1-4405)
  `langplugin.Dirs(name)` lists a plugin's directories: its owned paths and every sub-plugin path,
  sorted and without duplicates. A test fails, per plugin, when a non-test Go file in those
  directories reaches `cmd/corvint`, `internal/contextindex`, `internal/testsource` or
  `internal/langplugin` through module-internal imports, so a plugin can be built and tested apart
  from the core. The one exemption is `internal/swiftimpact`, the Swift adapter that registers into
  `contextindex.RegisterSwiftPMReader`; the test records its reason and fails if it goes stale.
- `LGP-V0-021`: (accepted under the owner's standing auto-accept (2026-11-03); V1-4405) `make plugin-test
  PLUGIN=<name>` tests one language and its tooling apart from the rest of Corvint: it runs
  `go test` over `langplugin.Dirs(name)` only (printed by `go run ./tools/plugin-dirs <name>`, which
  refuses an unknown name) and the langplugin manifest, conformance and isolation subtests for that
  plugin, each named by the plugin. `tools/plugin-dirs` reads `decl.Dirs`, a provider-free copy of
  `langplugin.Dirs` that a test keeps equal, so the directory tests compile no other language's
  provider; only the closing registry subtests build `internal/langplugin` and every provider.
- `LGP-V0-022`: (accepted under the owner's standing auto-accept (2026-11-03); V1-4405) Behavior is
  unchanged by LGP-V0-015 to LGP-V0-021: every existing test passes unchanged.

- `LGP-V0-023`: (accepted under the owner's standing auto-accept (2026-11-03); V1-4408) `internal/langplugin/dialect`
  is the hook registry through which a plugin hands its dialect algorithm bodies to core. It imports
  only the standard library. A plugin registers an `Impact` profile (code-line mask, probes, load
  pattern, static-load judgement, optional verification command) or a `Region` (normalizer,
  optional symbol extractor) under a dialect name its declaration carries; a duplicate, empty or
  incomplete registration, including a probe without a code or pattern, panics at link time. Core looks hooks up by the declared dialect name and
  never by a language name of its own. A plugin package may import `internal/langplugin/decl` and
  `internal/langplugin/dialect`, which are provider-free leaves, and still no core package
  (LGP-V0-020).
- `LGP-V0-024`: (accepted under the owner's standing auto-accept (2026-11-03); V1-4408) An impact or module dialect
  declares the reverse-reference `Rule` core runs for it: `constant-references`, `module-imports`,
  `namespace-using`, `package-imports` or `target-modules`; a region dialect declares none.
  `Validate` refuses an impact or module dialect without a rule, a region dialect with one, and a
  rule core does not implement for the dialect's kind (`decl.RuleKind`).
  Core reads `decl.RuleFor(kind, path)` and `decl.RuleOf(kind, dialect)` and switches on the rule,
  so a second language sharing a rule needs no core edit.
- `LGP-V0-025`: (accepted under the owner's standing auto-accept (2026-11-03); V1-4408) The Ruby plugin's package
  `internal/rubydialect` owns the Ruby lexical scanner (comments, strings, percent literals,
  heredocs), the Ruby impact profile, the Ruby symbol-region normalizer and declaration extractor,
  and the Ruby verification command, which reads the plugin's `Dependencies` (LGP-V0-017). Its
  `init` registers a hook for every dialect the Ruby declaration names. `internal/contextindex`
  links it with a blank import and keeps no Ruby algorithm body except the constant-reference rule,
  which reads the scanner's word helpers.
- `LGP-V0-026`: (accepted under the owner's standing auto-accept (2026-11-03); V1-4408) A missing hook fails closed. A
  declared impact dialect with no registered profile reports `source-analysis-unavailable`; a
  region dialect with no extractor yields the extraction note "no symbol extractor registered for
  this dialect; file not scanned". `contextindex.ImplementedDialects` reads the registry for the
  impact and region kinds, so the LGP-V0-016 conformance test fails when a plugin's hooks are not
  linked.
- `LGP-V0-027`: (accepted under the owner's standing auto-accept (2026-11-03); V1-4408) The web lexer stays in
  `internal/contextindex`, because the flow, setup and registry readers there share it; core
  registers it under the TypeScript plugin's declared `module-imports` impact dialects and their
  region dialects, by rule rather than by name. The contextindex ratchet entries for `kotlin`,
  `swift`, `ruby`, `typescript`, `rspec` and `minitest` are removed. Behavior is unchanged and every
  existing test passes unchanged. (Amended by AFP-V0-2247 in
  [`affected-plan-selection-v0.md`](affected-plan-selection-v0.md): the impact profile's probes, load
  pattern and static-load rule live in the TypeScript plugin's `internal/webdialect`; core still
  owns the mask and registers the profile.)

## Failure modes

| Failure | Detection | Outcome |
|---|---|---|
| A language lacks a piece and does not declare it absent | `Validate` (LGP-V0-007) | test fails naming plugin and piece |
| A new runner, analyzer binary or provider package belongs to no plugin | conformance test (LGP-V0-008) | test fails naming it |
| A runner is registered by two families or declared by two plugins | table and conformance tests | test fails naming both |
| A declared path is renamed or deleted | conformance test | test fails naming the path |
| Core code adds a language or runner name literal | ratchet (LGP-V0-009) | test fails naming the package and literal |
| A core name literal is removed but the baseline is not lowered | ratchet | test fails asking to lower the baseline |
| An unknown runner reaches `Build` or `Parse` | registry | `unsupported runner`, unchanged (fail closed) |
| A manifest declares a dialect the core does not implement, or the reverse | dialect conformance test (LGP-V0-016) | test fails naming kind and dialect |
| A dependency maps to a runner its plugin does not declare | `Validate` (LGP-V0-017) | test fails naming plugin and package |
| A plugin package starts importing a core package | isolation test (LGP-V0-020) | test fails naming the plugin and import chain |
| A remaining core literal has no recorded reason | ratchet (LGP-V0-019) | test fails naming the entry |
| Two plugins register the same dialect hook, or one registers an incomplete hook | `dialect.Register*` (LGP-V0-023) | panic at init naming the dialect |
| An impact or module dialect declares no rule, or a rule core does not implement for its kind | `Validate` (LGP-V0-024) | test fails naming plugin and dialect |
| A declared dialect's plugin hooks are not linked | `ImplementedDialects` (LGP-V0-026) | `source-analysis-unavailable` or an extraction note; conformance test fails |

The ratchet compares exact string literals only, so a name built by concatenation or held in a
longer literal (`"ruby:rspec:"`) is not counted; it bounds growth, it does not prove absence. A new runner whose name already
appears as a core literal fails the ratchet when it is registered, and the baseline is raised then.

## Non-goals

- Moving packages into per-language directory trees, or renaming them.
- Loading plugins at run time (Go `plugin`, shared objects, a plugin daemon or network discovery):
  registration stays compiled in, keeping the single native-Go binary (invariant 7).
- Changing the `analyzercap` protocol, any wire format, selection, or runner argv.
- Moving the web lexer and the Ruby constant-reference rule out of `internal/contextindex`, or the
  runner readings out of `internal/testsource` (LGP-V0-027); their remaining literals carry
  recorded reasons.
- Splitting the multi-language runner family packages (`dynamic`, `native`, `platform`).

## Acceptance evidence

- `internal/langplugin/langplugin_test.go`: `TestManifestsValidate_LGPV0007`,
  `TestValidateRefusesAnUndeclaredPiece_LGPV0007`, `TestManifestConformance_LGPV0008`,
  `TestCoreNameLiterals_LGPV0009`.
- `internal/testrunner/registry/table_test.go`: `TestFamiliesAreDisjointAndComplete_LGPV0010`;
  `rspec_prefix_test.go` (`TestRSpecOnlyMembersRefusedForOtherRunners`, unchanged) for LGP-V0-011.
- Existing `internal/testrunner/...`, `internal/liveverify/affected/languages` and affected
  selection tests passing unchanged (LGP-V0-012).
- `TestDialectsAndDependencies_LGPV0015`, `TestPluginIsolation_LGPV0020` and the per-plugin
  subtests in `langplugin_test.go`; `TestPluginDialectsImplemented_LGPV0016` in
  `internal/contextindex`; `make plugin-test PLUGIN=ruby` (LGP-V0-021); existing tests passing
  unchanged (LGP-V0-022).
- `internal/rubydialect/rubydialect_test.go` (`_LGPV0025` tests); `TestManifestsValidate_LGPV0007`
  and `TestPluginDialectsImplemented_LGPV0016` over the rules and registry (LGP-V0-024, 026); the
  lowered ratchet (LGP-V0-027); `make plugin-test PLUGIN=ruby` and `PLUGIN=typescript`.

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| LGP-V0-001..003 | `internal/langplugin/langplugin.go` | `TestManifestsValidate_LGPV0007`, `TestManifestConformance_LGPV0008` |
| LGP-V0-004..006 | `langplugin.go`, `internal/liveverify/affected/languages/languages.go` | `TestManifestConformance_LGPV0008`, `languages_test.go` |
| LGP-V0-007, 008 | `langplugin.Validate` | `TestValidateRefusesAnUndeclaredPiece_LGPV0007`, `TestManifestConformance_LGPV0008` |
| LGP-V0-009 | ratchet baseline in `langplugin_test.go` | `TestCoreNameLiterals_LGPV0009` |
| LGP-V0-010, 011 | `internal/testrunner/registry/registry.go`, `internal/testrunner/dynamic/rspec_prefix.go` | `TestFamiliesAreDisjointAndComplete_LGPV0010`, `TestRSpecOnlyMembersRefusedForOtherRunners` |
| LGP-V0-012 | unchanged behavior | existing runner, registry and selection tests |
| LGP-V0-013, 014 | manifest `Coverage` and `Mutation` lists; ruby `Coverage` is `linecov-ruby` (`internal/rubylinecov`) | `TestManifestConformance_LGPV0008`, `make plugin-test PLUGIN=ruby` |
| LGP-V0-015, 017, 018 | manifest `Dialects`, `Dependencies`, `SubPlugin.Suffixes` | `TestDialectsAndDependencies_LGPV0015`, `TestValidateRefusesAnUndeclaredPiece_LGPV0007` |
| LGP-V0-016 | `contextindex.ImplementedDialects` | `TestPluginDialectsImplemented_LGPV0016` |
| LGP-V0-019 | ratchet in `langplugin_test.go` | `TestCoreNameLiterals_LGPV0009` |
| LGP-V0-020, 021 | `langplugin.Dirs`, `tools/plugin-dirs`, `make plugin-test` | `TestPluginIsolation_LGPV0020`, per-plugin subtests |
| LGP-V0-022 | unchanged behavior | existing tests |
| LGP-V0-023 | `internal/langplugin/dialect` | `TestPluginIsolation_LGPV0020`, `TestRegistersDeclaredDialects_LGPV0025`, `TestRegisterRefusesIncompleteHooks_LGPV0023` |
| LGP-V0-024 | `decl.Rule`, `langplugin.Validate`, rule switches in `internal/contextindex` | `TestManifestsValidate_LGPV0007`, existing impact tests |
| LGP-V0-025 | `internal/rubydialect` | `rubydialect_test.go`, existing Ruby impact and symbol tests |
| LGP-V0-026 | `non_go_impact.go`, `rubysymbols.go`, `ImplementedDialects` | `TestPluginDialectsImplemented_LGPV0016` |
| LGP-V0-027 | `contextindex/dialect_hooks.go`, ratchet | `TestCoreNameLiterals_LGPV0009`, existing tests |

## Boundaries and rollback

The manifest is compiled-in data with no state, file format or wire contract. Rollback is a revert
of the change: restore the hand-written provider list in `languages.go` and the switch in
`registry.go`, and delete `internal/langplugin` and its read-scope entry. Nothing persisted reads
the manifest, so no migration is needed.

Maintenance: a change that adds a language, runner, analyzer binary, provider package, coverage
collector or mutation tool updates the manifest in the same commit; the conformance test enforces
it for runners, analyzers and providers.

## Promotion and later slices

- Move the web lexer into the TypeScript plugin behind the LGP-V0-023 hooks, and the JavaScript and
  RSpec runner readings out of `internal/testsource`, lowering the ratchet baseline each time.
- Register the MCJ-V0 Ruby and TypeScript collectors as `coverage` sub-plugins (LGP-V0-013).
- Promotion to `implemented` needs the slice's tests green in CI; there is no external gate.
