# Language Plugins V0

Owner: Russell Lewis
Date: 2026-11-03
Intent status: accepted
Delivery status: experimental
Authoritative inputs: owner request of 2026-11-03 (each language acts as a plugin, each testing and
coverage tool as a plugin of that language, so Corvint core stays language-agnostic and only uses
the plugins); ticket V1-4404; `affected-plan-v0.md` (the `affected.Language` seam);
`test-runner-execution-v0.md` (TRE-V0-081); `map-coverage-join-v0.md`;
`live-proof-carrying-verification-v0.md` (LPCV-V0-950..963); `AGENTS.md` invariant 7.

## Agent digest
- Claim: One in-process manifest per language declares its paths, provider, analyzers, runners, coverage and mutation tools; a missing piece fails a test.
- Status: accepted / experimental
- Exists: `internal/langplugin` (manifest, `Validate`, `Providers`), the table-driven `internal/testrunner/registry`, and the conformance and core-name ratchet tests.
- Blocked on: nothing. Migrating the core packages' remaining name literals and adding non-Go coverage collectors are later slices.
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
  collector is a `coverage` sub-plugin of its language. Today only Go has one (`linecov-go`:
  `internal/linecov`, `internal/liveverify/gorunner`). A new collector, including the MCJ-V0 Ruby
  and TypeScript segment collectors, registers in its language's manifest and clears that
  language's `coverage` absence in the same change, rather than adding a separate per-language
  path that core names.
- `LGP-V0-014`: (accepted under the owner's standing auto-accept (2026-11-03); V1-4404) A mutation
  tool is a `mutation` sub-plugin: `mutate-go`, `pymutate`, `rbmutate`, and `extmutate` (shared by
  Ruby and TypeScript). A new test provider likewise registers as a `test-provider` sub-plugin.

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

The ratchet compares exact string literals only, so a name built by concatenation or held in a
longer literal (`"ruby:rspec:"`) is not counted, and only each listed directory's own files are read,
not its subpackages; it bounds growth, it does not prove absence. A new runner whose name already
appears as a core literal fails the ratchet when it is registered, and the baseline is raised then.

## Non-goals

- Moving packages into per-language directory trees, or renaming them.
- Loading plugins at run time (Go `plugin`, shared objects, a plugin daemon or network discovery):
  registration stays compiled in, keeping the single native-Go binary (invariant 7).
- Changing the `analyzercap` protocol, any wire format, selection, or runner argv.
- Migrating the existing core name literals in this slice; the ratchet stops their growth.
- Splitting the multi-language runner family packages (`dynamic`, `native`, `platform`).

## Acceptance evidence

- `internal/langplugin/langplugin_test.go`: `TestManifestsValidate_LGPV0007`,
  `TestValidateRefusesAnUndeclaredPiece_LGPV0007`, `TestManifestConformance_LGPV0008`,
  `TestCoreNameLiterals_LGPV0009`.
- `internal/testrunner/registry/table_test.go`: `TestFamiliesAreDisjointAndComplete_LGPV0010`;
  `rspec_prefix_test.go` (`TestRSpecOnlyMembersRefusedForOtherRunners`, unchanged) for LGP-V0-011.
- Existing `internal/testrunner/...`, `internal/liveverify/affected/languages` and affected
  selection tests passing unchanged (LGP-V0-012).

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| LGP-V0-001..003 | `internal/langplugin/langplugin.go` | `TestManifestsValidate_LGPV0007`, `TestManifestConformance_LGPV0008` |
| LGP-V0-004..006 | `langplugin.go`, `internal/liveverify/affected/languages/languages.go` | `TestManifestConformance_LGPV0008`, `languages_test.go` |
| LGP-V0-007, 008 | `langplugin.Validate` | `TestValidateRefusesAnUndeclaredPiece_LGPV0007`, `TestManifestConformance_LGPV0008` |
| LGP-V0-009 | ratchet baseline in `langplugin_test.go` | `TestCoreNameLiterals_LGPV0009` |
| LGP-V0-010, 011 | `internal/testrunner/registry/registry.go`, `internal/testrunner/dynamic/rspec_prefix.go` | `TestFamiliesAreDisjointAndComplete_LGPV0010`, `TestRSpecOnlyMembersRefusedForOtherRunners` |
| LGP-V0-012 | unchanged behavior | existing runner, registry and selection tests |
| LGP-V0-013, 014 | manifest `Coverage` and `Mutation` lists | `TestManifestConformance_LGPV0008` |

## Boundaries and rollback

The manifest is compiled-in data with no state, file format or wire contract. Rollback is a revert
of the change: restore the hand-written provider list in `languages.go` and the switch in
`registry.go`, and delete `internal/langplugin` and its read-scope entry. Nothing persisted reads
the manifest, so no migration is needed.

Maintenance: a change that adds a language, runner, analyzer binary, provider package, coverage
collector or mutation tool updates the manifest in the same commit; the conformance test enforces
it for runners, analyzers and providers.

## Promotion and later slices

- Migrate the `cmd/corvint`, `internal/contextindex` and `internal/testsource` name literals onto
  `langplugin` and the registry, lowering the ratchet baseline each time.
- Register the MCJ-V0 Ruby and TypeScript collectors as `coverage` sub-plugins (LGP-V0-013).
- Promotion to `implemented` needs the slice's tests green in CI; there is no external gate.
