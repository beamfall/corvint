# 2026-10-07: tsconfig path aliases in impact reverse imports (V1-0958)

## Intent

GitHub #659 (native V1-0958): `corvint impact` on a TypeScript page object found only the specs that
import it by a relative path. Specs that import it through tsconfig `baseUrl`/`paths` were missed, so
the selection built on the packet came back empty instead of incomplete. Rule (c) of `GPK-V0-027`
resolved only relative and profile-alias specifiers, and `GPK-V0-069` had named `tsconfig` `paths`
as future work. Proposed `GPK-V0-077`..`GPK-V0-081` (`docs/specs/go-production-kernel-migration-v0.md`),
plus an `NGI-V0-003` amendment and the closed code `bare-import-unresolved`
(`docs/specs/non-go-impact-v0.md`), are pending owner acceptance.

## What main already had

- The `affected-plan/0` TypeScript adapter (`internal/liveverify/affected/typescript`) already raises
  the `typescript:path-alias-unresolved` frontier. On this issue's fixture it returns scope `UNKNOWN`
  with no selected units, so selection there already fails closed. Its Playwright profile
  (TJAA-V0-012) resolves nearest tsconfig `baseUrl`/`paths` without `extends`. It is left unchanged.
- `impact` had no alias resolution, no type-only distinction and no unresolved-bare disclosure.

## Decisions

- **Impact only.** The new alias arm adds rows only inside `impact`. `reverseImporters`, the oracle
  arm, is unchanged, so its other callers (context, span ranking, lookup, checkpoint, task frame)
  emit the same packets and frozen evaluations are not disturbed. The index, its format, its digest
  and its snapshots are unchanged.
- **Resolver API** (`internal/contextindex/webresolve.go`). `NewWebImportResolver(index)` returns a
  memoized resolver. `Resolve(importer, specifier)` returns `{Target, State}`, where the state is
  `repository`, `package` or `unresolved`. It also covers relative and profile-alias specifiers.
  `WebImportKinds(text)` classifies each specifier as type-only or not. The issue-657 lane should
  consume the import graph through these two functions.
- **Resolution follows TypeScript.**
  - Config: the nearest `tsconfig.json`, else `jsconfig.json`. `include`, `files` and project
    references are not evaluated.
  - Order: an exact `paths` key, else the wildcard with the longest prefix, trying its substitutions
    in order; then `baseUrl`; then the package test.
  - Files: `.js` is replaced by its `.ts`/`.tsx`/`.d.ts` sources first. Extensionless candidates
    get an extension added, and a directory resolves through its `index` file.
- **Fail-closed rules.** None of these produces a guessed target:
  - These make the whole config unknown: a cycle, a missing or unreadable relative extends, more
    than 16 links or 64 configs, `moduleSuffixes`, invalid `paths`, duplicate keys, or a config
    over 1 MiB.
  - A package-named extends makes only the fields no earlier config declares unknown.
  - These make the specifier unresolved: two matching wildcards with equal prefix length, or a
    target directory holding a `package.json` (whose entry point is not read).
- **Package rule (`GPK-V0-080`).** A bare specifier that resolves to no repository file counts as a
  package only in these cases:
  - it has a URL scheme;
  - it names a Node builtin;
  - it names a nested workspace package (already disclosed by `GPK-V0-069`);
  - it is declared, directly or through its `@types/` package, by a `package.json` at or above the
    importer.

  Everything else is unresolved and is counted repository-wide in an uncertainty line. The
  `non-go-syntax-v0` profile also gets a `bare-import-unresolved` row.
- **Type-only (`GPK-V0-079`).** `import type` and `export type` edges score 100 below their runtime
  equivalent, carry `medium` confidence, and keep the reason `imports <specifier>`, which the `prove`
  falsifier checks. Inline `import { type X }` stays a runtime edge.
- **Frozen-parity carve-out.** The `impact-web` parity fixture imports bare `widget/button` and has
  no manifest or config. Its expected bytes are immutable (`GPK-V0-033`, `NGI-V0-004`). So when the
  index holds no `package.json`, `tsconfig.json` or `jsconfig.json`, the default profile keeps the
  frozen reading and adds no line. The syntax profile still reports the unknown.
- **Fixture.** The committed fixture is one JSON document (`internal/contextindex/testdata/tsconfig-paths.json`)
  rather than loose `.ts` files. Loose files would put its deliberately unresolved aliases into this
  repository's own index.
- **Analyzer schema.** `analyzerSchemaID` moves to `corvint-analyzer/108`. `TestAnalyzerSchemaInputs`
  pins every production contextindex source, and this change edits `webimports.go` and adds
  `webresolve.go`. Snapshots therefore rebuild once.
- **Capability text.** The MCP impact description (and its two goldens) and `impact --help` now say
  relative/profile/tsconfig imports, and the MCP text says unresolved bare imports are unknown.

## Evidence

- Tests in `internal/contextindex/webresolve_test.go`:
  - `TestImpactResolvesTSConfigPathAliasImporters` lists all five alias importers. Three of them go
    through `@pages/*`, `.js` replacement and `baseUrl`; one goes through a two-link extends chain;
    one is type-only, scored 550 with medium confidence. The test also covers the directory index,
    the exact key and the second substitution, and checks the disclosure line.
  - With the alias merge disabled, the test fails with five missing importers (observed).
- The other new tests:
  - `TestWebImportResolverFixture`;
  - `TestWebImportResolverFailsClosed`: cycle, depth 17, missing extends, both package-extends
    cases, moduleSuffixes, ambiguous wildcard, two-star key, package directory, malformed and
    duplicate-key configs, escaping baseUrl, `${configDir}`, jsconfig;
  - `TestJSONCToJSON`;
  - `TestWebImportKinds`;
  - `TestImpactSyntaxReportsBareImportUnresolved`;
  - `TestImpactKeepsFrozenBareReadingWithoutManifest`.
- Timings. Medians of wall-clock CLI runs, including snapshot load. The repository is a generated
  20,005-file TypeScript repo with `package.json`, a JSONC `tsconfig.base.json` with wildcard paths,
  and a `tsconfig.json` that extends it. There are 10,000 lib modules, 5,000 pages and 5,000 specs,
  and one spec in three is `import type`.

  | Changed path | Binary | `index` | `impact` | Reverse imports |
  |---|---|---|---|---|
  | `src/pages/Page42.ts` | base | 1.087 s | 0.139 s | 0 (alias importers missed) |
  | `src/pages/Page42.ts` | new | 0.811 s | 0.153 s | 2, including 1 type-only, with the disclosure |
  | `src/lib/m0/util5.ts` | base | 0.790 s | 0.114 s | 1 |
  | `src/lib/m0/util5.ts` | new | 0.815 s | 0.159 s | 2 |

  Index build is unchanged within run-to-run noise, since the index code is untouched. One impact call
  costs about 14 to 45 ms more, from building the alias graph once over 20k sources.
- On this repository, `impact extensions/vscode/src/configuration.ts` takes 2.29 s with the new
  binary and 2.30 s with the base binary. The new binary returns the same rows plus the disclosure,
  "12 bare import specifiers in 8 sources". The specifiers are real: `@playwright/test` in test
  fixtures that have no `package.json`, and undeclared `@corvint/pi-*`, `@opencode/plugin/tui` and
  `@earendil-works/pi-tui` imports under `integrations/`.
- `corvint affected` on the fixture with `src/pages/LoginPage.ts` changed returns scope `UNKNOWN`
  with the `typescript:path-alias-unresolved` frontier.

## Dogfood

- At lane start, `corvint --version` reported `Corvint 1.0.0-rc.2 (build 360)`.
- `corvint affected --base 8af2bf62` on the empty pre-change diff returned scope `UNKNOWN` with no
  selected units.
- `corvint context` (task: tsconfig alias resolution) returned READY with 20 results, pointing at the
  TS adapter, `playwright_aliases.go`, the reverse-import tests and the TJAA spec.
- After the change, `corvint affected` returned scope `UNKNOWN` with 113 selected units, including the
  mandatory `make gate`, which is NOT_RUN by lane rule. Focused tests ran on the directly affected
  packages.
- No CEM was bound or sealed in the lane (`NOT_PRODUCED`: lane rule; the orchestrator binds per batch).

## Limits and NOT_RUN

- `make gate`: NOT_RUN, by lane rule.
- Committed-range impact (`impact --base`) is unchanged and does not use the alias arm.
- Not resolved:
  - package `exports`, `types` and `main` entry points;
  - `rootDirs` and project references;
  - Vite and webpack aliases;
  - `require()` edges.
- The unresolved count is repository-wide, so an unrelated undeclared import elsewhere marks every
  web impact incomplete. This is conservative by design; narrowing it is an owner question.
- When no result is admitted, the packet state stays `OUT_OF_SCOPE` and carries the disclosure. The
  state value itself does not change.

## Independent review

Codex (`gpt-6-astra`, read-only) round 1 reported six P2 findings and no P0 or P1 findings. Each one
was checked against the `moduleNameResolver` code of TypeScript 5.9.3 (`tryLoadModuleUsingPaths`,
`tryLoadModuleUsingOptionalResolutionSettings`, `tryAddingExtensions`). All six were accepted. Each
has a `TestWebImportResolverFailsClosed` case, and all seven new cases fail on the round-0
resolver (observed):

1. An unknown inherited `baseUrl` or `paths` fell through to the package test, so a declared name
   it might alias counted as a package. Now every bare specifier under such a config is unresolved.
2. A substitution naming a TypeScript extension is now tried as written before extension
   replacement.
3. A matched `paths` key whose substitutions all miss no longer falls back to `baseUrl`. TypeScript
   goes straight to `node_modules`.
4. The extension table now follows `tryAddingExtensions`: `.jsx`/`.tsx` also try `.ts` and `.js`,
   `.mts`/`.cts` and declaration forms map to their families, `.json` tries `.d.json.ts`, and other
   extensions try `.d<ext>.ts`.
5. An absolute or repository-escaping substitution makes the specifier unresolved. It used to be
   rebased into the repository or skipped.
6. The repository root resolves as a directory, through its `index` file.

On this repository, impact after these fixes still reports 12 specifiers in 8 sources.

Round 2 reported three P2 findings and one P3 finding. All four were checked against TypeScript
5.9.3 and accepted. Five `TestWebImportResolverFailsClosed` cases cover them, and all five fail on
the round 1 resolver (observed):

1. `moduleResolution` is not read, and `node10` tries TypeScript and declaration files in a first
   pass and JavaScript in a second, while `bundler`, `node16` and `nodenext` try every form at once.
   A specifier, relative or aliased, now resolves only when both orders pick the same file
   (cases "node10 and bundler orders disagree" and "relative orders disagree").
2. An empty `baseUrl` now means the declaring config's directory, as `normalizeNonListOptionValue`
   does, instead of no `baseUrl`.
3. Drive-letter, backslash and URL-shaped paths in `baseUrl`, substitutions and `extends` are now
   rooted, so they count as outside the repository and leave the specifier unresolved.
4. (P3) An empty `*` capture now leaves the substitution literal rather than substituting nothing.

Timings after round 1, re-measured under concurrent load from other lanes (7-run medians): Page42
impact 0.181 s base and 0.242 s new; util5 0.188 s base and 0.209 s new. The added cost stays at
tens of milliseconds and impact stays sub-second.

Out of scope: `TestSourceViewNativeSourceDigestsAreFrozen` in `cmd/corvint` fails on the base commit
8af2bf62 as well (the `cmd/corvint/source_handoff.go` digest is not re-frozen). This change does not
touch it.

Round 3 reported six P2 findings. Four were accepted, after checking them against TypeScript
5.9.3 (`createComputedCompilerOptions`, `nodeLoadModuleByRelativeName`, `loadModuleFromFile`,
`classicNameResolver`). Round 2's agreement rule is replaced by reading the mode. Twelve
`TestWebImportResolverFailsClosed` cases cover the accepted findings, and all twelve fail on the
round 2 resolver (observed). Two cases from round 2 changed their expectation, because the
default config is now known to be node10.

1. `.mts` and `.cts` files and their declaration forms are typed files, so node10 picks them in
   its first pass.
2. A trailing slash on a substitution, a relative specifier or a `baseUrl` lookup names a
   directory only.
3. Under node10, a target that only the JavaScript pass reaches is unresolved when the name is a
   declared package. node10 tries `node_modules` types before JavaScript, and `node_modules` is not
   indexed.
4. The mode is now read. It comes from `moduleResolution`, else `module`, else `target`, each
   inherited through extends. With a known mode, its own pass order replaces the agreement check.
   - node16 and nodenext treat a target reached by an added extension or a directory index as
     unresolved.
   - Classic leaves every bare specifier unresolved, because its ancestor-directory search is not
     modelled.
   - An unknown mode leaves everything unresolved. This includes a package extends that leaves the
     mode undecided.

Two findings were declined. Both are recorded in `GPK-V0-077`.

5. A substitution naming a stylesheet resolves to that file, although `tsc` leaves it unresolved.
6. A `.json` target resolves whether or not `resolveJsonModule` is set.

The reason for both: impact asks which files depend on the changed file, and a bundler or Node
imports the asset at run time. Dropping the edge would hide a real dependent. Keeping it at most
adds an importer to the selection.

Self-dogfood after round 3: `impact extensions/vscode/src/configuration.ts` takes 3.97 s, including
the index rebuild after the pin change. The repository's config uses `moduleResolution` `Node16`.
The result has the same three reverse imports and the same disclosure, "12 bare import specifiers
in 8 sources".

Round 4 reported two P2 findings. Both were accepted. Three new cases cover them, and all three
fail on the round 3 resolver (observed).

1. Under node10, a `typeRoots` declaration can supply a declaration before the JavaScript pass.
   So a JavaScript-pass-only alias target is now unresolved whenever `typeRoots` is declared or is
   left unknown by a package extends.
2. `moduleSuffixes` is now first-declaration-wins like the other fields. A package-named extends
   that comes before any `moduleSuffixes` declaration makes the whole config unknown, because the
   unread config may declare suffixes. A leaf that declares `[""]` overrides an inherited list,
   which round 3 still rejected.

The cost of this rule: a config that extends a package now leaves every bare specifier unresolved
unless the leaf itself declares `baseUrl`, `paths`, the mode and `moduleSuffixes`. This is an owner
question.
