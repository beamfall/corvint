# Non-Go impact V0

Owner: Russell Lewis
Date: 2026-09-30
Intent status: proposed
Delivery status: experimental
Authoritative inputs: owner-requested GitHub issue #390 (native V1-0537), and the
2026-09-30 instruction to implement it. This technical contract proposes concrete
semantics; issue intent does not accept generated implementation detail.

## Agent digest
- Claim: Ruby, JavaScript and TypeScript path/range impact binds syntax evidence and explicitly names unresolved dynamic dispatch.
- Status: proposed/experimental; local source and focused conformance implemented; human acceptance and integration pending. NGI-V0-007..011 (declared `string-key-registry/0` dialect: literal registry keys to the tests passing them) are accepted under the owner's standing auto-accept (2026-10-11; V1-1365) and implemented over synthetic repositories.
- Exists: Ruby/JS/TS versioned positional, committed-range and MCP admission; bounded structured unknowns; existing JS/TS import rules; a declared string-keyed registry dialect for syntax-profile path impact.
- Blocked on: shared registry/CEM closeout, draft publication, integration and owner acceptance; no runtime-dispatch qualification.
- Read next: Requirements; Trust and failure boundaries; Acceptance and traceability.

## User and measurable job

A maintainer of Ruby/JS/TS product code needs immutable impact evidence for paths
or a complete committed range. The baseline CLI already admits these indexed
paths, but the range compiler requires a Go module and only qualifies Go hunks;
MCP admission requires .go. Syntax observations cannot establish runtime call or
test closure. The useful result is bounded evidence with an open semantic frontier.

## Requirements

- `NGI-V0-001`: Positional impact and MCP impact MUST admit tracked `.rb`, `.js`,
  `.jsx`, `.mjs`, `.cjs`, `.ts` and `.tsx` paths. MCP schema and runtime MUST agree
  on the same closed suffix set, retain path confinement, duplicates and bounds,
  and keep unchanged Go admission. CLI's broader indexed-path admission remains.
- `NGI-V0-002`: Default and expanded committed ranges MUST bind Ruby/JS/TS path
  membership and exact old/new textual hunk spans using the existing range
  identity, status, ancestry, path, source-size, deadline and drift checks.
  Go module/package/parser preconditions MUST apply only to changed Go members;
  root-level Ruby/JS/TS and repositories without Go modules MUST be admitted.
- `NGI-V0-003`: Every Ruby/JS/TS changed path MUST name a syntax-only dynamic
  dispatch limitation in structured unknowns independent of ranked-result limits.
  Specific detected computed dispatch, dynamic load, reflection or Rails autoload
  MUST add closed frontier codes. Detector absence MUST NOT establish closure.
  Positional CLI requests select `--language-profile non-go-syntax-v0`; the
  historical default path receipt remains byte-identical. MCP non-Go requests
  and non-Go range receipts select/name this experimental profile automatically.
  The selector is unavailable with `--base` or `--working-tree-untracked`.
  Ruby reverse-import resolution remains explicitly unavailable; JS/TS reuses
  existing relative/profile import rules, plus the tsconfig/jsconfig alias arm
  `GPK-V0-077` to `GPK-V0-081` propose (proposed, pending owner acceptance;
  V1-0958), without inferring runtime call semantics. A non-test JS/TS changed
  path in a repository with an unresolved bare import specifier carries
  `bare-import-unresolved` with the repository-wide count (`GPK-V0-080`).
- `NGI-V0-004`: Added paths MUST carry the same receipt/evidence envelope and
  target Git blob binding as Go evidence. Range markers/ADR relations MUST remain
  exact-hunk qualified and caller-authored authority MUST remain withheld.
  Result omissions, critical misses and unsupported language members MUST remain
  explicit. Legacy Go-only receipts and canonical digest preimages MUST retain
  their exact bytes when no new language members occur. Existing refusal
  conditions and codes remain. Module validation keeps historical precedence in
  repositories without admitted Ruby/JS/TS source, including the no-module
  daily-workflow abstention. In a repository admitting those languages it depends
  on changed Go membership and follows snapshot/base validation; exact precedence
  for multiply-invalid mixed repositories is not a compatibility claim. Frozen
  oracle expectations remain immutable and are never regenerated from a candidate.
- `NGI-V0-005`: `impact --help` and MCP description MUST publish a per-language
  capability table identifying path/range support, syntax-only import rules and
  unresolved dispatch. Working-tree-untracked remains the separately bounded Go
  profile. Help MUST distinguish receipt production from semantic completeness.
- `NGI-V0-006`: Conformance MUST include each new suffix/language, negative
  dynamic cases and comment/string twins, mixed Go/web ranges, root-level inputs,
  no-Go-module repositories, limits, dirty/drift/excluded/binary refusals, and
  byte-identical repeat runs. Tests MUST establish schema/runtime agreement and
  unchanged Go-only and historical default path bytes. The CLI selector MUST be
  tested for invalid/repeated values and mutually exclusive profiles. No test execution or qualification is implied
  by an impact receipt.

### Declared string-keyed registries (V1-1365; GitHub #871)

A module registered in an exported object literal and reached by tests only
through an accessor that takes the literal key has no import edge to those tests.
This narrow dialect, declared like the application map's router dialects, binds
the key instead. It is read only by syntax-profile path impact (positional
`--language-profile non-go-syntax-v0` and MCP); the historical default receipt
and range impact are unchanged.

- `NGI-V0-007`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1365) A
  registry MUST be declared as an entry of the optional `registries` member of a committed
  `application-map-manifest/0` document: `path` (a clean repository-relative JS/TS path with
  no backslash and no `.git` first segment),
  `export` (an identifier), `dialect` exactly `string-key-registry/0`, and 1..16 unique
  identifier `accessors`; at most 64 entries, unique by path and export. The application map
  validates the member with the same closed check and refuses an invalid one with
  `appmap-invalid-manifest` but does not read it. Impact reads every indexed manifest at the
  evaluated revision; a member that does not decode or validate, or a manifest that mentions
  `registries` but exceeds 256 KiB or is not strict JSON (a duplicate member included), is
  counted as `registry-declaration-invalid` on every JS/TS changed path and contributes no edge.
  No accessor or registry is ever inferred.
- `NGI-V0-008`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1365) The
  dialect MUST read the declared export as `export const|let|var NAME [: Type] = { ... }` with
  the existing web lexer. An entry gives a key -> module edge only when its key is a quoted
  string, a template literal without substitution, an identifier, or a computed key holding
  exactly one such literal, and its value is `X`, `ns.X`, `new X(...)` or `new ns.X(...)`
  (optionally followed by `as` or `satisfies`) where `X` or `ns` is a value binding of a static
  import; the edge's module is that import's specifier as resolved by the existing reverse-import
  rules. Every other entry (a spread, a computed key needing evaluation, a template with a
  substitution, a method, an escaped or overlong key, a value that is not an imported binding)
  is counted, never dropped. A declared file whose export is not such an object literal (for
  example a re-export) or is unreadable has no known key.
- `NGI-V0-009`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1365) A call
  site is a call of a declared accessor, by name or by a local alias from an import clause,
  in any indexed JS/TS source; a declaration of the method or function is not one, and an
  uncalled member reference (`dp.accessor` passed on) is a call site with a non-literal key. Its
  key is literal only when the first argument is a closed quoted string or substitution-free
  template followed by `,` or `)`. For a changed path the registry imports, impact MUST return each
  calling file whose literal key the registry maps to that path, as a `test` result (a test path)
  or `reference` result, with two `high`/`syntax` evidence rows pinned to Git blobs: the registry
  entry line and the call-site line. The same key in another registry selects only through that
  registry's accessors.
- `NGI-V0-010`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1365) Selection
  MUST fail closed. A non-literal key at any call site of a registry's accessor adds
  `accessor-key-nonliteral` (count of such calls, with `registry` and `accessor`); an
  unresolved entry adds `registry-key-unresolved` (count, with `registry`); an unresolved export
  adds `registry-export-unresolved` and applies to every JS/TS changed path. Each of these, and
  a change to the registry file itself, MUST widen selection to every caller of that registry's
  accessors with `medium` call-site evidence. A source that names an accessor but cannot be
  read or lexed completely adds `registry-caller-unavailable`. These rows sit in structured
  unknowns, independent of the result limit.
- `NGI-V0-011`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1365)
  Conformance MUST cover the issue-shaped fixture (several entries, literal-key specs, a spec
  with a computed key, a change to the registry file), a registered module under a test tree,
  and the hostile cases: a spread entry, a re-exported registry, an aliased accessor import,
  template keys with and without substitution, a key collision across two registries, an
  invalid declaration, and an unchanged default-profile receipt. The real CLI output on the
  fixture is retained in the build log.

## Trust and failure boundaries

Core performs bounded local immutable reads; it adds no network, process provider,
state write, execution, daemon or authority. Existing unsupported statuses/modes,
unsafe paths, binary content, excluded source and snapshot races remain refused.
A syntactic relation has syntax authority; only independently accepted intent has
project authority. Structured unknowns contain paths, counts and closed codes,
not excerpts or source-controlled diagnostic text. An output limit must never
remove the semantic frontier. Existing non-Go outside-profile omissions remain
for languages not admitted by this new range capability.

## Closed syntax unknown codes

| Code | Meaning |
|---|---|
| `dynamic-dispatch-unresolved` | Baseline: syntax does not establish runtime dispatch or test closure. |
| `reverse-import-rule-unavailable` | Ruby reverse-import semantics are unavailable. |
| `source-analysis-unavailable` | The requested source is missing, excluded or unavailable to syntax analysis. |
| `observed-reflective-dispatch` | A lexical reflection/metaprogramming candidate was observed. |
| `observed-computed-dispatch` | A computed-call candidate was observed. |
| `autoload-unresolved` | Ruby autoload was observed without resolved runtime semantics. |
| `dynamic-load-unresolved` | A load argument is dynamic, interpolated or concatenated. |
| `bare-import-unresolved` | Bare JS/TS import specifiers resolve to no repository file or declared package, so reverse importers are not closed; `count` is the repository-wide specifier count (`GPK-V0-080`, proposed). |
| `registry-declaration-invalid` | A manifest `registries` member does not decode or validate; `count` is the number of such manifests (`NGI-V0-007`). |
| `registry-key-unresolved` | Entries of the declared registry `registry` have no literal key or imported-binding value; selection is widened (`NGI-V0-008`, `NGI-V0-010`). |
| `registry-export-unresolved` | The declared export of `registry` is not a readable object literal, so none of its keys are known; selection is widened (`NGI-V0-008`, `NGI-V0-010`). |
| `accessor-key-nonliteral` | `count` calls of `accessor` pass a key that is not a literal; selection is widened to every caller (`NGI-V0-010`). |
| `registry-caller-unavailable` | Sources naming a declared accessor could not be read or lexed completely (`NGI-V0-010`). |

These codes are uncertainty, never execution evidence. The versioned receipt
names `language_profile: non-go-syntax-v0`; line 0 denotes a baseline frontier.
The registry rows' `registry` and `accessor` fields carry only a declared path
and a validated declared identifier, never source text.

## Acceptance and traceability

| Requirement | Implementation | Executable evidence |
|---|---|---|
| `NGI-V0-001` | shared suffix helper; MCP descriptor/validator | `TestNonGoImpactMCPAdmission` |
| `NGI-V0-002` | shared range compiler language classification | `TestNonGoRangeImpact` |
| `NGI-V0-003` | shared lexical mask and structured frontier; `bare-import-unresolved` in `nonGoImpactUnknowns` | `TestNonGoImpactDynamicUnknowns`; `TestImpactSyntaxReportsBareImportUnresolved` and `TestImpactKeepsFrozenBareReadingWithoutManifest` (`internal/contextindex/webresolve_test.go`) |
| `NGI-V0-004` | existing range bindings/reducer | `TestNonGoRangeImpactLegacyGoBytes`, `TestNonGoRangeImpactGoModuleValidationPriority`, `TestNonGoImpactCLIProfile` |
| `NGI-V0-005` | impactHelp and MCP description | `TestNonGoImpactMCPAdmission`; actual CLI help check in the build-log |
| `NGI-V0-006` | committed fixture conformance | `TestNonGoRangeImpactMixed`, `TestNonGoRangeImpactRefusals`; tests above and existing range/impact regressions |
| `NGI-V0-007` | `ValidateRegistryDeclarations`, `declaredRegistries` (`internal/contextindex/registry_dialect.go`); `decodeManifest` (`internal/appmap/manifest.go`) | `TestValidateRegistryDeclarations`, `TestRegistryDialectDeclarationFailures`, `TestManifestRegistriesMember` |
| `NGI-V0-008` | `parseRegistry`, `registryImports`, `registryEntryOf`; template body in `readTemplate` (`webimports.go`) | `TestRegistryDialectSelectsTestsPassingTheKey`, `TestRegistryDialectUnresolvedRegistryWidens`, `TestRegistryDialectReexportedRegistryIsUnresolved` |
| `NGI-V0-009` | `registryCallSites`, `registryDialect.impact`; wiring in `impact` | `TestRegistryDialectSelectsTestsPassingTheKey`, `TestRegistryDialectAliasedAccessorImport`, `TestRegistryDialectKeyCollisionAcrossRegistries` |
| `NGI-V0-010` | `registryDialect.impact`; `extra` rows in `nonGoImpactUnknowns` | `TestRegistryDialectNonLiteralKeyWidens`, `TestRegistryDialectChangedRegistrySelectsEveryCaller`, `TestRegistryDialectUnresolvedRegistryWidens`, `TestRegistryDialectReexportedRegistryIsUnresolved` |
| `NGI-V0-011` | `internal/contextindex/registry_dialect_test.go`, `internal/appmap/registries_test.go` | tests above; real CLI output in `../build-log/2026-10-14-v1-1365-string-keyed-registry.md` |

Names in this matrix identify evidence rather than implying passing execution.
Observed execution and independent findings are retained in
`../build-log/2026-09-30-non-go-impact.md`. No broad formal qualification or promotion is claimed.

## Non-goals, rollout and rollback

No Ruby runtime call-graph resolver, compiler/type-checker, complete dynamic
semantic impact, dynamic capacity increase, test executor, new provider transport
or working-tree non-Go admission. Existing wider positional suffix support is not
removed. The registry dialect performs no runtime evaluation, infers no accessor
or registry, and folds no constant across files or beyond a single literal; it
does not follow destructured or reassigned accessors, bracket-string member
calls, or registries assembled at runtime, and it does not change range impact.
Roll it back by removing the `registries` declarations or the dialect code. The implementation stays experimental pending human acceptance and
integration. Revert task-owned code/spec changes to roll back; preserve original
receipts and the failed/unsupported outcomes. Kill or repair the extension if it
changes legacy Go bytes, invents semantic closure or weakens immutable binding.
