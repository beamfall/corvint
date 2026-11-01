# Cross-repository breakage map V0

Owner: Russell Lewis
Date: 2026-09-29
Intent status: proposed
Delivery status: experimental
Authoritative inputs: owner-approved six-feature wow slice, native ticket V1-0503,
`AGENTS.md`, `docs/specs/external-evidence-provider-v1.md`,
`docs/specs/external-evidence-provider-v2.md`.

## Agent digest
- Claim: A selected API maps pinned Go syntax references and original declared provider relationships across explicit local repositories.
- Status: proposed/experimental
- Exists: `internal/breakagemap`, public `breakage` CLI dispatch/help and focused tests.
- Blocked on: Behavioral breakage, adequate test coverage, complete inventory and V1-0417 promotion remain unqualified.
- Read next: Requirements; Failure modes; Acceptance evidence and rollback.

## Human-owned intent and user job

Given a selected changed API, show the exact caller and test reference sites and the
explicit flow/document relationships in a locally declared repository set. Expose all
unresolved edges and unsupported language boundaries. The user needs evidence to inspect,
not an invented promise that every downstream behavior will break or every test covers it.
The measured job is one actual two-repository fixture plus source use from Corvint and
the Beamfall shell consumer, with its relationship explicitly declared rather than
presented as discovery by the Go analyzer.

## Input and output

```
corvint [--root DIR] breakage --manifest FILE --api REPO:PATH:SYMBOL \
  --repository REPO=CHECKOUT [--repository OTHER=CHECKOUT] [--base FULL_COMMIT]
corvint [--root DIR] breakage manifest --repository REPO=CHECKOUT \
  --source REPO:PATH [--source REPO:PATH ...]
```

Manifest schema `corvint-breakage-manifest/0` contains `repositories` (each `id`,
`origin`, `commit`, `tree`), `sources` (each `repository`, `path`, `blob`, `start`,
`end`) and `providers` (embedded original `external-evidence-provider/1`, `/2` or `/3`
records). Origins are root commits. Every source span is nonempty and 1-based inclusive.
Locations are operator arguments, not repository identities or manifest authority.
Relative manifest and checkout locations resolve against `--root`.

Output `corvint-breakage-map/0` contains the manifest digest, exact API selection,
repository bindings without local directory disclosure, change classification,
typed edges, unknowns, bounds and truncation. Each source anchor pins repository,
commit, tree, path, blob, inclusive span and SHA-256 of newline-joined span bytes.
`syntax-call`, `syntax-reference`, test-prefixed variants, `import-only`, and
`provider-relationship` remain distinct. A test reference is not covering-test evidence.
Provider edges retain the original relation, evidence category, rule and reference,
with the embedded provider bytes' SHA-256. Entity declarations are not source anchors.

## Requirements

- `BKM-V0-001`: The command MUST be explicit, read-only, local and experimental. It
  MUST NOT execute providers, discover checkout directories, clone, fetch, run tests,
  start services, change ranking or change the existing impact receipt.
- `BKM-V0-002`: The manifest MUST be one strict bounded UTF-8 JSON document with no
  duplicate or unknown members. Repository identities, immutable commit/tree/blob
  pins and unique exact relative source paths MUST be validated. Checkout bindings
  MUST name canonical Git top levels with the declared origin in both the selected
  commit and HEAD histories. Invalid pins MUST produce refusal or unresolved evidence,
  never witnesses derived from replacement worktree bytes.
- `BKM-V0-003`: Each emitted source witness MUST name an immutable Git blob and
  exact span/hash. Unrelated dirty files MUST NOT be read. Nonregular Git modes,
  missing objects, unavailable repositories, invalid spans and unsupported sources
  MUST remain unavailable or refused. HEAD differing from the pinned commit MUST be
  disclosed as historical; the report describes the explicit pinned bytes.
  Go anchors MUST use physical blob lines irrespective of `//line` directives. Every
  declaration, reference and provider anchor MUST be ordered and lie within both the blob
  and its supplied source span before any bytes are sliced; otherwise it stays unresolved.
- `BKM-V0-004`: Go references MUST match the selected function's pinned module
  import path and parsed package name, including explicit aliases. The selected API's
  bounded ancestor go.mod metadata MUST be checked; an undeclared nested module
  withholds syntax mapping. Shadowed qualifiers, dot imports, receivers, build constraints,
  malformed sources and ambiguous declarations MUST NOT become resolved callers.
  Import-only candidates remain separate. Same-package unqualified calls are syntactic;
  non-call name ambiguity, references within the selected file, dependency replacement,
  type resolution and complete module inventories remain explicit gaps.
  Build-directive text inside literals, block comments or explanatory comments MUST
  NOT count as a build constraint. Filename constraints use trailing recognized
  GOOS/GOARCH components before optional `_test`; interior platform words MUST NOT
  withhold syntax mapping.
  Directive recognition MUST respect the leading file header, including the legacy
  `+build` blank-line boundary; directive-shaped comments after source begins MUST
  NOT withhold syntax mapping.
- `BKM-V0-005`: EEP V1 repository/entity composition and V2 path composition MUST
  reuse the existing strict EEP decoder and preserve original relation categories.
  Provider repository pins MUST agree with the manifest. Path endpoints MUST resolve
  only through admitted pinned sources; mismatches and missing endpoints MUST remain
  unresolved and MUST NOT advance traversal. Traversal may inspect a relation in either
  direction for association, but MUST retain its original direction and MUST NOT infer
  causal impact. Directory scopes are unsupported, not recursively expanded.
  Every provider projection MUST disclose that file/entity association does not
  establish dependency on the selected symbol merely through shared file membership.
- `BKM-V0-006`: Byte verification MUST NOT establish assertion truth, behavioral
  breakage, test execution, adequacy or complete coverage. Entity-only witnesses retain
  declaration state. Flow and documentation relationships are existing provider
  declarations; they MUST NOT acquire accepted intent, reviewed status or observed
  execution by being included. Unsupported languages retain declaration-only gaps.
- `BKM-V0-007`: `--base` MUST identify an available ancestor of the selected API
  repository's pinned commit. The selected package function's declaration bytes are
  compared at that same path. Unchanged declarations, changed declarations, unavailable
  base/target paths and unresolved declarations MUST be distinguished. Deletion,
  addition and rename MUST NOT be guessed from similarity. This command does not
  implement automatic changed-declaration inventory or close V1-0417. When either
  declaration is unresolved, including every target without a trusted declaration
  extractor (Go, or JS/TS under `BKM-V0-011`), the change is bounded only by whole-blob
  identity as amended by `BKM-V0-009`.
- `BKM-V0-008`: Limits MUST be enforced: 8 repositories, 256 source files, 32 path
  components, 1 MiB manifest/blob, 16 MiB aggregate source, 4 existing bounded EEP
  records, 2000 edges and 2000 unique unknowns, a 30-second request context and
  10-second bounded contained Git calls. Input overflow refuses; output overflow
  marks truncation. Output is deterministic for the same bytes/bindings, always
  `scope: INCOMPLETE`, and never claims that the explicit inventory is exhaustive.
- `BKM-V0-009`: When the selected declaration cannot be resolved at the base or target
  (deleted, added, renamed or an unsupported language), the change MUST be
  `declaration-unresolved-file-unchanged` when the whole selected blob is byte-identical
  at `--base`, otherwise `declaration-unresolved-file-changed`, and the unresolved
  declaration unknown MUST remain. A file-level classification MUST NOT be presented as
  symbol-level change detection: an unchanged file bounds the symbol as unchanged, while a
  changed file does not establish that the named symbol changed. Status: experimental under
  the standing lane auto-accept (GitHub issue #1898, V1-2801).
- `BKM-V0-010`: `breakage manifest` MUST pin only explicitly bound checkouts and explicitly
  named `REPO:PATH` sources: each repository's HEAD commit, tree and lowest root commit as
  origin, and each source's regular UTF-8 blob with the whole-file span. It MUST read Git
  objects only, never discover checkouts or sources, never write files, enforce the
  `BKM-V0-008` repository and source limits, and refuse (exit 2, no stdout) any unbound
  repository, unsafe or missing path or non-top-level checkout. Its output MUST decode under
  the `BKM-V0-002` manifest decoder; providers stay an explicit empty list for the operator to
  supply. Status: experimental under the standing lane auto-accept (GitHub issue #1898,
  V1-2801).
- `BKM-V0-011`: `--base` change detection MUST also resolve the selected declaration in
  `.js`, `.mjs`, `.cjs`, `.ts`, `.mts` and `.cts` blobs through the shared JS/TS lexer: exactly
  one top-level `function`, `class`, `interface`, `enum` or `namespace`/`module` declaration,
  or one `const|let|var NAME =` binding to a function expression or a brace-bodied arrow. Its
  change identity is the declaration's whole physical lines at the same path. It MUST fail
  closed to the `BKM-V0-009` file-level bound and its unknowns on lexer uncertainty, unbalanced
  brackets, overloads or any second top-level declaration of the name, any assignment to the
  name, a decorated class, an expression-bodied arrow, a type-annotated binding, a body brace
  it cannot tell from a type literal, and every `.jsx`, `.tsx` or other-language file. JS/TS
  caller and test reference mapping stays unsupported, so the unsupported-language unknown
  remains. Status: experimental under the standing lane auto-accept (GitHub issue #1898,
  V1-2813).
- `BKM-V0-012`: The manifest MUST also admit `external-evidence-provider/3` records
  (`EEP-V2-014`) through the strict EEP decoder; a `/1` or `/2` record carrying an endpoint
  `span` or `symbol` MUST refuse. A `/3` path endpoint `span` narrows its anchor to that span
  and a `symbol` narrows it to the one declaration the Go profile or `BKM-V0-011` resolves in
  the endpoint's pinned blob; with both, the declaration MUST lie within the span. Narrowing
  resolves only through admitted pinned source bytes. An unresolved symbol, a declaration
  outside its span, or a narrowed anchor outside the blob or supplied source span MUST leave
  the edge unresolved with an unknown and MUST NOT advance traversal. Traversal stays
  file-level, and a narrowed edge's reason MUST say so: narrowing does not establish
  dependency on the selected symbol or assertion truth. Status: experimental under the
  standing lane auto-accept (GitHub issue #1898, V1-2813).
- `BKM-V0-092`: (standing lane auto-accept, 2026-10-28; V1-2915) With `--base`, the report MUST carry `declared_span_change`, separate from `change`: the selected source's declared lines (`sources[].start`..`end`) compared positionally between the base and the target blob as `declared-span-changed`, `declared-span-unchanged` or `span-unavailable` (base path or lines missing). It is available for every language, resolves no declaration, and the unsupported-language unknown stays. EEP /3 endpoint spans are compared under `BKM-V0-099`.
- `BKM-V0-093`: (standing lane auto-accept, 2026-10-28; V1-2915) A manifest provider record of schema /1 or /2 carrying an endpoint `span` or `symbol` MUST refuse naming the field and that it needs `external-evidence-provider/3`; when any provider endpoint is narrowed, the provider unknown MUST say so instead of claiming associations bind only files/entities.
- `BKM-V0-094`: (standing lane auto-accept, 2026-10-28; V1-2915) `breakage manifest` MUST accept `--repository ID=CHECKOUT@COMMIT` (full commit id; tree and origin derive from it), `--source ID:PATH:START-END` (that line span, default whole file), and `--provider-skeleton`, which appends one empty `external-evidence-provider/3` record whose repositories equal the manifest's pins.
- `BKM-V0-099`: (standing lane auto-accept, 2026-10-28; V1-2915) With `--base`, the report MUST also carry `endpoint_span_changes`: one `{start, end, change}` per distinct line span that a relation endpoint of an EEP /3 record whose repository pins match the manifest declares on the selected source, sorted by start then end, each compared positionally as in `BKM-V0-092` (`declared-span-changed`, `declared-span-unchanged`, or `span-unavailable` when the base path, the target source or either blob's lines are missing). A symbol-only endpoint declares no span and is not compared; without `--base` the field is absent. It resolves no declaration and changes no edge.
- `BKM-V0-955`: (standing lane auto-accept, 2026-11-01; V1-3529) When `BKM-V0-004` withholds Go syntax mapping for the selected API, the unknown MUST name the exact ancestor go.mod path(s) and why each failed: tracked at the pinned commit but not a declared manifest source (with the `breakage manifest --source REPO:PATH` remedy), declared with a blob that differs from the pinned tree, or none tracked at any probed ancestor path (each listed). The withholding rule itself is unchanged: a module boundary is admitted only through declared, pinned go.mod sources. Non-goal: auto-declaring go.mod. Rollback: restore the fixed reason text.
- `BKM-V0-710`: (standing lane auto-accept, 2026-11-01; V1-3529) When the selected Go API's bounded ancestor `go.mod` check withholds the syntax mapping, the unknown MUST keep its `module boundary` prefix and name the exact `REPOSITORY:PATH` it rests on and why: a `go.mod` tracked at the pinned commit but not declared as a manifest source (with the `--source REPOSITORY:PATH` that declares it), a declared `go.mod` whose pinned bytes could not be read (its own unknown says why, for example a blob that differs from the pinned tree's), no `go.mod` tracked in the source's directory or any ancestor, or a pinned tree that cannot be listed. The check and its withholding are unchanged; only the reason is named.

## Failure modes and non-goals

Untrusted manifests carry data, never commands. Secrets-shaped manifest content is
refused. Git subprocesses use the existing contained process runner and sanitized
environment, disabling network/lazy fetching, credentials, replacement objects and
optional locks. Only named immutable blobs and bounded ancestor module metadata are
read; no new language scanner or whole-repository index is introduced.

The Go profile recognizes package functions, not receiver dispatch or general symbol
semantics. Package matches are syntax evidence; actual dependency versions and runtime
bindings remain unverified. Provider text is untrusted external declaration. A fresh
blob does not validate its author's relationship claim. No source inference creates
flow intent, accepted documentation or a passing test. Existing optional console,
flow providers and documentation corpus contracts retain their own authority.

## Acceptance and rollback

`TestCrossRepositoryBreakageMap` creates separate Git histories and checks aliases,
direct test references, import-only candidates, shadowing/dot/method/build constraints,
malformed sources, original flow/docs declarations, immutable spans, deterministic
results and clean worktrees. `TestBreakagePinsFailClosed` challenges origin/tree/blob,
unbound checkout, endpoint blob and provider revision pins. `TestBreakageManifestRefusals`
challenges ambiguous JSON and paths. `TestBreakageSymlinkAndHistoricalPins` verifies
mode refusal and historical disclosure. `TestBreakageV1EntityComposition` covers
the V1 entity seam and missing entities. `TestBreakageBaseAndModuleGaps` covers
unchanged/renamed/deleted API states, foreign bases and hidden nested modules.
`TestBreakageEdgeBound` checks truncation. CLI argument tests exercise the same runner.
`TestBreakageBuildConstraintLookalikes` retains callers for directive literals and
interior platform filename components. `TestBreakageBuildConditional` distinguishes
actual directive comments and platform suffixes from ordinary comment text and names.
`TestBreakageLineDirectivesPinPhysicalLines` pins declaration and caller anchors to
physical lines around inner and preceding `//line` directives, and
`TestBreakageLineDirectivesOutsideNarrowedSpan` keeps directive-shifted spans that leave a
narrowed supplied span unresolved. `TestBreakageAnchorRejectsInvalidPhysicalSpans`,
`TestBreakageSuppliedSourceSpanBounds`, `TestBreakageInvalidProviderAnchorCannotAdvance`
and `TestBreakageOversizeProviderAnchorCannotAdvance` refuse reversed, zero and oversize spans.

Retain an actual source-bound Corvint map and an explicitly declared Beamfall shell
association with commit/tree/blob/span pins, the source manifest, output and candidate
identity. The shell consumer's references to `corvint` and `corvint-harness-event/0`
do not establish Go semantic discovery, deployment qualification or V1-0417 closure.
Root integration owns live receipts, independent review, CEM and native completion.
Focused tests do not imply a full repository gate was run.

`TestBuildManifestMatchesHandPins` proves the helper pins exactly what hand-written pins
carry and refuses unbound, unsafe, missing and non-top-level inputs;
`TestReadSpanDigestsPhysicalLines` binds span digests to anchor digests;
`TestBreakageNonGoChangeIsFileBounded` covers `BKM-V0-009`; and
`TestBreakageManifestCLIPinsNamedSources` compiles a CLI-built manifest end to end.
`TestJSDeclarationSpans` and `TestBreakageTypeScriptDeclarationChange` cover `BKM-V0-011`
resolved shapes, fail-closed shapes and the TypeScript end-to-end change classification;
`TestBreakageProviderV3SpanSymbol` covers `BKM-V0-012` span and Go symbol narrowing, the
unresolved, off-span and out-of-source refusals, and `/2` refusal of a symbol.
`TestBreakageDeclaredSpanChangeIsSeparateFromDeclaration` covers `BKM-V0-092`,
`TestBreakageEEPV2SpanNamesField` covers `BKM-V0-093` and
`TestBuildManifestPinsOlderCommitSpanAndSkeleton` covers `BKM-V0-094`;
`TestBreakageDeclaredSpanUnavailable` adds the `BKM-V0-092` unavailable case and
`TestBreakageEndpointSpanChange` covers `BKM-V0-099`, the narrowed provider unknown and the
`/2` span refusal through `Decode` (`BKM-V0-093`).
`TestBreakageModuleBoundaryNamesMissingGoMod` covers `BKM-V0-955` across two repositories:
an undeclared tracked SDK-root go.mod, a stale declared blob, and no tracked go.mod each name
the path, and declaring the go.mod yields the caller edge.
`TestModuleBoundaryNamesTheMissingGoMod_V13529` (`module_boundary_v13529_test.go`) covers
`BKM-V0-710` with two repositories: an omitted root `go.mod` is named with its `--source`, a
declared one yields syntax edges, and one declared with a stale blob is named as such. The
reasons come from the same `moduleBoundary` code as `BKM-V0-955`. Rollback of
`BKM-V0-710`: return the bare reason from `moduleBoundary` again.

Non-goals of `BKM-V0-009` to `BKM-V0-012`: no parser for languages other than Go and the
JS/TS declaration shapes above, no JSX/TSX, no dotted or member symbol resolution (a dotted
`/3` symbol stays unresolved), no JS/TS caller or test reference mapping, no multiple spans
per endpoint, no symbol-level traversal, no source discovery and no provider generation.

Rollback removes the optional command registration and this package. There is no
persistent state, migration, daemon or changed default ranking to unwind.

## Named diagnostics

| Code | Meaning |
|---|---|
| `checkout-unbound` | A declared repository has no explicit checkout binding; retain an unresolved repository rather than discovering or fetching a checkout. |
