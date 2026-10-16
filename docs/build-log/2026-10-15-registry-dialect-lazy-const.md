# 2026-10-15 V1-1455, V1-1456, V1-1457: registry dialect lazy entries, unexported registry, const keys

Extends the declared `string-key-registry/0` dialect (`docs/specs/non-go-impact-v0.md`,
NGI-V0-007..011, V1-1365) for GitHub #945, #946 and #947. Base `f81114713c96cf629462dfd9e91e7d370ee848c4`.

## Contracts

- `NGI-V0-012` (V1-1455, #945, P1). A registry with any unresolved entry now applies its
  `registry-key-unresolved` row and caller widening to every JavaScript or TypeScript changed
  path, not only to paths it imports. On BASE a module reached only by `require()` inside an
  entry got no reverse-import, no test row and no unknown: selection failed open, against
  NGI-V0-010 and invariant 2.
- `NGI-V0-013` (V1-1456, #946). Four lazy entry forms give a key -> module edge:
  `() => new (require(S).X)(...)`, `() => require(S).X` (including `.default`), and
  `() => import(S).then(...)`, optionally with a trailing `as`/`satisfies`. S must be a closed
  literal, and the specifier must be relative or a profile alias, resolved from the registry
  file by reverse-import rule (c) to an indexed JS/TS source. Bare specifiers, unindexed
  targets, parameterised or block arrows, and dynamic S stay unresolved, which under 012 widens.
- `NGI-V0-014` (V1-1456). The declared `export` may name a top-level `const` the file does not
  export. A nested or member-access binding is not the registry and stays
  `registry-export-unresolved`.
- `NGI-V0-015` (V1-1457, #947). An accessor argument that is a lone identifier is a literal key
  when the same file binds it by a top-level `const NAME = L` (optionally `as const`) ending the
  statement, and every other non-member occurrence of NAME is also a lone accessor argument.
  Shadowing, `let`/`var`, assignment, import, any other read, a type annotation, substitution,
  concatenation, a continued line and a file containing `with (...)` all keep it non-literal.

All four rows are new IDs (no collision on origin/main or in the open PRs). They are accepted under
the owner's standing auto-accept of lane requirements (2026-10-11).

## Decisions and owner items

- **Fork (#946): non-exported registry.** Accepted with no schema change. The existing `export`
  field names the binding, and only a top-level `const` qualifies. Lane decision under the
  delegated in-task forks.
- **Owner decision (reported, not self-accepted).** NGI-V0-008 says an entry gives an edge "only
  when" its value is an imported binding. NGI-V0-009 says a key is literal "only when" the
  argument is a quoted string or template. Rows 013..015 add new shapes beside those lists
  rather than editing their text. Any shape the accepted rows already recognised selects
  exactly as before. The owner should confirm that this additive reading does not amend the
  meaning of 008/009.
- **Existing test changed.** `TestRegistryDialectNonLiteralKeyWidens` declared its non-literal key
  as `const key = 'ctx:alpha'`. Under NGI-V0-015 that form is literal, so the test now uses
  `let key`, which keeps it non-literal and keeps the test's intent.

## Evidence

Load averages are 1/5/15-minute figures on a shared 12-core host.

| Check | Result |
|---|---|
| BASE reproduction: new tests run against the BASE `registry_dialect.go` | 6 FAIL: LazyRequireNeverFailsOpen, UnresolvedEntryAppliesToEveryPath, LazyEntriesResolve, LazyEntriesStayClosed (BASE counts 5 unresolved per case), NonExportedRegistry, ConstAccessorKey. StaysNonLiteral and the `let` NonLiteralKeyWidens pass, as expected. Load about 70.7. |
| Fixed: `go test -run Registry ./internal/contextindex` | 18/18 PASS, 478s, load 62.15/64.13/63.99 |
| Fixed: `-race -run Registry` | PASS, 424s, load 63.37 at start, 59.07 at end |
| M1 (012): `openEntry := false && ...` | fails LazyRequireNeverFailsOpen and UnresolvedEntryAppliesToEveryPath |
| M2 (013): `registryLazyBinding` result `&& false` | fails LazyEntriesResolve, load 70.72 |
| M3 (014): top-level `const` case disabled | fails NonExportedRegistry, load 77.74 |
| M4 (015): occurrence-check delete replaced by a no-op | fails ConstAccessorKeyStaysNonLiteral, load 87.75 |

Each mutation was restored and checked byte-identical against the fixed file.

## Real CLI output (retained)

Binaries `go build ./cmd/corvint` from BASE and from the change tree, both reporting
`Corvint 1.0.0-rc.3 (build 0)`. Each command is
`corvint impact --language-profile non-go-syntax-v0 <path>`. Rendering is `kind id score`,
then `path:line confidence authority blob - reason`. The baseline `dynamic-dispatch-unresolved`
row appears on every run and is omitted below.

### #945 fixture, commit `7af6e9a4a644`

Issue-shaped:

- an eager `alpha` entry;
- `zeta` loaded by `new (require(...).Czeta)()`;
- `eta` loaded by `import(...).then(...)`;
- three literal-key specs.

`src/contexts/zeta.ts`, BASE (fails open):

```text
path src/contexts/zeta.ts 1000
```

`src/contexts/zeta.ts`, fixed:

```text
path src/contexts/zeta.ts 1000
test tests/zeta.spec.ts 640
    tests/data/registry.ts:4 high syntax 3fe0c368a646 - registry registry maps key ctx:zeta to src/contexts/zeta.ts
    tests/zeta.spec.ts:2 high syntax 23609f2f59aa - passes key ctx:zeta to accessor createTestData
```

`src/contexts/eta.ts`:

- BASE: alpha, eta and zeta specs at 600 and `registry-key-unresolved` count 2.
- Fixed: `tests/eta.spec.ts` alone at 640, from the registry line 5 and spec line 2 pair.

`src/contexts/alpha.ts`:

- BASE: the alpha spec at 640, the eta/zeta specs at 600, `registry-key-unresolved` count 2.
- Fixed: `tests/alpha.spec.ts` alone at 640.

### Fail-closed check, commit `b14baf854f40`

This commit adds `'ctx:omega': () => require('omega-pkg').Comega`, a bare specifier, plus an
omega spec. `src/contexts/zeta.ts`, BASE (still fails open):

```text
path src/contexts/zeta.ts 1000
```

`src/contexts/zeta.ts`, fixed (widens):

```text
path src/contexts/zeta.ts 1000
test tests/zeta.spec.ts 640
    tests/data/registry.ts:4 high syntax fa17f87313f9 - registry registry maps key ctx:zeta to src/contexts/zeta.ts
    tests/zeta.spec.ts:2 high syntax 23609f2f59aa - passes key ctx:zeta to accessor createTestData
test tests/alpha.spec.ts 600
    tests/alpha.spec.ts:2 medium syntax d85eed94a7c0 - calls accessor createTestData while registry tests/data/registry.ts keys are not closed
test tests/eta.spec.ts 600
    tests/eta.spec.ts:2 medium syntax a7df9e11fd92 - calls accessor createTestData while registry tests/data/registry.ts keys are not closed
test tests/omega.spec.ts 600
    tests/omega.spec.ts:2 medium syntax fd190074b068 - calls accessor createTestData while registry tests/data/registry.ts keys are not closed
unknown {"code": "registry-key-unresolved", "count": 1, "language": "typescript", "line": 0, "path": "src/contexts/zeta.ts", "registry": "tests/data/registry.ts"}
```

### #947 fixture, commit `e11679a6740e`

The registry imports alpha, beta and delta. The alpha and beta specs pass literal keys.
`tests/delta.spec.ts` declares `const SCENARIO_CONTEXT = 'ctx:delta'` and passes the identifier.

`src/contexts/alpha.ts`, BASE:

```text
path src/contexts/alpha.ts 1000
reverse-import tests/data/registry.ts 650
    tests/data/registry.ts:1 high syntax 588f8338c1b5 - imports ../../src/contexts/alpha
test tests/alpha.spec.ts 640
    tests/data/registry.ts:5 high syntax 588f8338c1b5 - registry registry maps key ctx:alpha to src/contexts/alpha.ts
    tests/alpha.spec.ts:2 high syntax d85eed94a7c0 - passes key ctx:alpha to accessor createTestData
test tests/beta.spec.ts 600
    tests/beta.spec.ts:2 medium syntax d400104ab713 - calls accessor createTestData while another call of a tests/data/registry.ts accessor passes a non-literal key
test tests/delta.spec.ts 600
    tests/delta.spec.ts:3 medium syntax 9d87c20c3668 - calls accessor createTestData with a non-literal key
unknown {"accessor": "createTestData", "code": "accessor-key-nonliteral", "count": 1, "language": "typescript", "line": 0, "path": "src/contexts/alpha.ts", "registry": "tests/data/registry.ts"}
```

`src/contexts/alpha.ts`, fixed (only `tests/alpha.spec.ts`; no `accessor-key-nonliteral` row):

```text
path src/contexts/alpha.ts 1000
reverse-import tests/data/registry.ts 650
    tests/data/registry.ts:1 high syntax 588f8338c1b5 - imports ../../src/contexts/alpha
test tests/alpha.spec.ts 640
    tests/data/registry.ts:5 high syntax 588f8338c1b5 - registry registry maps key ctx:alpha to src/contexts/alpha.ts
    tests/alpha.spec.ts:2 high syntax d85eed94a7c0 - passes key ctx:alpha to accessor createTestData
```

`src/contexts/delta.ts`:

- BASE: every spec at 600.
- Fixed: `tests/delta.spec.ts` alone at 640, from the registry line 7 and spec line 3 pair.

## Retained limits

- **Template substitutions.** An accessor call inside a template substitution is invisible to
  the lexer. This was true before this change, and is a possible fail-open.
- **Late registry writes.** A registry written after its declaration (`registry['k'] = X`) is not
  observed.
- **Ruby paths.** `registry-export-unresolved` still applies to Ruby changed paths, although
  NGI-V0-010 says JS/TS. That behaviour predates this change; NGI-V0-012 itself is JS/TS only.
- **Imported const keys.** A const key imported from another module stays non-literal, as #947
  scopes it.
