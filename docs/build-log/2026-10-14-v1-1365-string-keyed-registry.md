# 2026-10-14 V1-1365: impact resolves declared string-keyed registries to the tests that pass the key

GitHub #871. A TS/JS module registered in an exported object literal (`export const registry = {
'ctx:x': new X(), ... }`) and reached by tests only through an accessor taking the literal key
(`dataProvider.createTestData('ctx:x')`) had no import edge to those tests: `impact` returned the
path, the registry as a reverse import and `dynamic-dispatch-unresolved`, and no tests. This lane
adds the narrow, declared `string-key-registry/0` dialect to syntax-profile path impact.

1.0 scope: `PRS-V1-018`. The new requirements `NGI-V0-007..011` in
`docs/specs/non-go-impact-v0.md` are Corvint 1.0 requirements, accepted under the owner's
standing auto-accept of lane requirements (2026-10-11); V1-1365 (REQUIREMENTS.tsv acceptance
`standing`, acceptedBy `2026-10-11; V1-1365`).

## Design

- Declaration path: router dialects are declared in the committed `application-map-manifest/0`
  document (`internal/appmap`, V1-1131). Registries reuse it as an optional `registries` member
  (`path`, `export`, `dialect: "string-key-registry/0"`, `accessors`). `appmap` validates the
  member with the shared closed check (`contextindex.ValidateRegistryDeclarations`) and refuses an
  invalid one with `appmap-invalid-manifest`; it does not otherwise read it. `appflows` imports
  `contextindex`, so impact cannot call the map's decoder; it reads every indexed manifest
  itself with the same strictness (`internal/cem/wire.Parse` refuses duplicate members, then a
  `DisallowUnknownFields` decode of the member) and counts every failure as
  `registry-declaration-invalid`.
- Parsing reuses the existing web lexer (`webimports.go`); the only lexer change is a
  `template` flag so a substitution-free template literal yields its body. No new parser
  dependency. Because `webimports.go` is an analyzer input, `analyzerSchemaID` moves to
  `corvint-analyzer/118` with the audit digest repinned.
- `internal/contextindex/registry_dialect.go` reads the declared export's object literal, gives
  key -> module edges for entries whose value is an imported binding (`X`, `ns.X`, `new X()`,
  `new ns.X()`, optionally `as`/`satisfies`), and finds accessor call sites (by name or an
  import alias) in indexed JS/TS sources. Edge modules resolve through the existing
  reverse-import rules, so no new resolution semantics are introduced.
- `impact.go` runs the dialect only when `syntaxFrontier` is set (positional
  `--language-profile non-go-syntax-v0` and MCP). The historical default receipt and range impact
  are byte-identical (`TestRegistryDialectDefaultReceiptUnchanged`).
- Precise rows: `test` (or `reference`) at score 640 with two `high`/`syntax` evidence rows, the
  registry entry line and the call-site line, each pinned to its Git blob. Fail-closed rows: score
  600 with `medium` call-site evidence for every caller of the registry's accessors when the
  registry itself changed, its export is unresolved, an entry is unresolved, or any call passes a
  non-literal key. New unknown codes: `registry-declaration-invalid`,
  `registry-export-unresolved`, `registry-key-unresolved`, `accessor-key-nonliteral` (with
  `registry` and `accessor`), `registry-caller-unavailable`.

## Real CLI output (retained)

Binary built from the change tree (`go build -o bin/corvint ./cmd/corvint`, reports
`Corvint 1.0.0-rc.3 (build 0)`). Fixture: a scratch Git repository shaped like the issue, commit
`9b30c9344b1185d5e657a32c442815214047cc32` (`corvint/appmap.json` declaring
`tests/data/registry.ts` export `registry` with accessor `createTestData`; three context modules;
three specs using literal keys `'ctx:alpha'`, `` `ctx:beta` ``, `"ctx:gamma"`), then
`a6e1f3af57fa253fcfe8e72c51814d3fbb3e9055` adding `tests/dynamic.spec.ts`, which passes a
computed key. Rendering: `kind id score`, then `path:line confidence authority blob - reason`.

`corvint impact --language-profile non-go-syntax-v0 src/contexts/alpha.ts` at `9b30c93`:

```text
path src/contexts/alpha.ts 1000
    src/contexts/alpha.ts:1 authoritative git-tree 091ac1407408 - requested changed path
reverse-import tests/data/registry.ts 650
    tests/data/registry.ts:1 high syntax 2c6b32d9fb15 - imports ../../src/contexts/alpha
test tests/alpha.spec.ts 640
    tests/data/registry.ts:6 high syntax 2c6b32d9fb15 - registry registry maps key ctx:alpha to src/contexts/alpha.ts
    tests/alpha.spec.ts:4 high syntax d46307759a76 - passes key ctx:alpha to accessor createTestData
unknown {"code": "dynamic-dispatch-unresolved", "count": 1, "language": "typescript", "line": 0, "path": "src/contexts/alpha.ts"}
```

`corvint impact --language-profile non-go-syntax-v0 src/contexts/beta.ts` at `a6e1f3a` (computed
key present):

```text
path src/contexts/beta.ts 1000
    src/contexts/beta.ts:1 authoritative git-tree 9df6917c1506 - requested changed path
reverse-import tests/data/registry.ts 650
    tests/data/registry.ts:2 high syntax 2c6b32d9fb15 - imports ../../src/contexts/beta
test tests/beta.spec.ts 640
    tests/data/registry.ts:7 high syntax 2c6b32d9fb15 - registry registry maps key ctx:beta to src/contexts/beta.ts
    tests/beta.spec.ts:3 high syntax 93d0ef286761 - passes key ctx:beta to accessor createTestData
test tests/alpha.spec.ts 600
    tests/alpha.spec.ts:4 medium syntax d46307759a76 - calls accessor createTestData while another call of a tests/data/registry.ts accessor passes a non-literal key
test tests/dynamic.spec.ts 600
    tests/dynamic.spec.ts:3 medium syntax e2c057f27657 - calls accessor createTestData with a non-literal key
test tests/gamma.spec.ts 600
    tests/gamma.spec.ts:3 medium syntax bd05ae9ad172 - calls accessor createTestData while another call of a tests/data/registry.ts accessor passes a non-literal key
unknown {"code": "dynamic-dispatch-unresolved", "count": 1, "language": "typescript", "line": 0, "path": "src/contexts/beta.ts"}
unknown {"accessor": "createTestData", "code": "accessor-key-nonliteral", "count": 1, "language": "typescript", "line": 0, "path": "src/contexts/beta.ts", "registry": "tests/data/registry.ts"}
```

`corvint impact --language-profile non-go-syntax-v0 tests/data/registry.ts` at `a6e1f3a` (the
registry file itself changed):

```text
path tests/data/registry.ts 1000
    tests/data/registry.ts:1 authoritative git-tree 2c6b32d9fb15 - requested changed path
test tests/alpha.spec.ts 600
    tests/alpha.spec.ts:4 medium syntax d46307759a76 - calls accessor createTestData of changed registry tests/data/registry.ts
test tests/beta.spec.ts 600
    tests/beta.spec.ts:3 medium syntax 93d0ef286761 - calls accessor createTestData of changed registry tests/data/registry.ts
test tests/dynamic.spec.ts 600
    tests/dynamic.spec.ts:3 medium syntax e2c057f27657 - calls accessor createTestData of changed registry tests/data/registry.ts
test tests/gamma.spec.ts 600
    tests/gamma.spec.ts:3 medium syntax bd05ae9ad172 - calls accessor createTestData of changed registry tests/data/registry.ts
unknown {"code": "dynamic-dispatch-unresolved", "count": 1, "language": "typescript", "line": 0, "path": "tests/data/registry.ts"}
unknown {"accessor": "createTestData", "code": "accessor-key-nonliteral", "count": 1, "language": "typescript", "line": 0, "path": "tests/data/registry.ts", "registry": "tests/data/registry.ts"}
```

Default profile, `corvint impact src/contexts/beta.ts` at `a6e1f3a`, is unchanged (path and
reverse-import rows only, no unknowns):

```text
path src/contexts/beta.ts 1000
    src/contexts/beta.ts:1 authoritative git-tree 9df6917c1506 - requested changed path
reverse-import tests/data/registry.ts 650
    tests/data/registry.ts:2 high syntax 2c6b32d9fb15 - imports ../../src/contexts/beta
```

## Verification

- `corvint affected --base adf587fdb65f4d87dc94a5e374a4fda1f87c5e7f` selected 115 Go packages
  (`contextindex` is a hub). Run: `internal/contextindex` focused
  (`-run 'Registry|NonGo|Impact|Web|Analyzer|Template|Syntax'`) PASS; `internal/appmap` focused
  (`-run 'Manifest|Registr|Router|Decode|Validate'`) PASS; `-race` on the dialect, non-Go impact
  and manifest tests in both packages PASS; `cmd/corvint` and `internal/mcp/...` with
  `-run Impact` PASS; `go vet` on `internal/contextindex`, `internal/appmap`, `cmd/corvint` clean;
  `gofmt -l` empty; the 12 CI doc gates pass.
- Full-package runs of `internal/contextindex` and `internal/appmap` hit the 30-minute alarm on a
  host at load average about 59 (still-running tests were `TestContextSpansDefaultBytes` at 5s and
  `TestAMAPV0025InjectedTableThroughBarrel` at 2m23s, so neither is a hang in this change). Before
  the alarm they reported two failures: `TestAnalyzerSchemaInputs` (a real finding, fixed by the
  analyzer schema bump above) and `TestBuildQuerySelectiveAllocationRatchet` (506448 bytes/op
  against 500000). The ratchet also fails at BASE on the same host (538320 bytes/op) and passed
  on the change tree when rerun alone, so it is retained as pre-existing load noise. The remaining
  103 affected packages are `NOT_RUN` (focused-test policy; they consume `contextindex` without
  changed behavior outside the syntax profile).
- Go toolchain on the host is go1.27.2; AGENTS.md still names go1.27.1.

## Independent review (Codex, one round)

Codex (read-only) found three correctness issues and one verification gap; all four were
confirmed and fixed in this change:

1. High: a manifest spelling the schema with an escaped solidus was skipped, and a duplicate
   `registries` member silently won. Fixed: discovery matches `application-map-manifest`, a
   manifest-naming document that mentions `registries` but is over 256 KiB or not strict JSON
   (duplicate members refused by `wire.Parse`) counts as `registry-declaration-invalid`
   (`TestRegistryDialectStrictManifest`). Retained limit: a schema name written with `\u` escapes
   of its letters is still not discovered.
2. Medium: an overlong identifier key was treated as resolved. Fixed: identifier keys use the
   same 256-byte bound and count as `registry-key-unresolved` (`long-key` case).
3. Medium: `.git/...` and backslash registry paths passed validation. Fixed: the check mirrors the
   application map's `safeRelative` (`git dir`, `backslash` cases).
4. Medium: the default-byte regression was untested and this entry was missing. Fixed:
   `TestRegistryDialectDefaultReceiptUnchanged` compares canonical default receipts with and
   without a `registries` member (revision normalized), and this entry retains the CLI output.

## Retained limits and non-goals

- No runtime evaluation, no inference of accessors, and no cross-file constant folding beyond a
  literal. Destructured or reassigned accessors, bracket-string member calls and registries
  assembled at runtime are not followed.
- A registry or accessor name shared across registries can over-select; it never under-selects.
- `flows appmap` still requires `routers`, so a registries-only manifest is invalid for the map
  while impact reads its `registries` member.
- Default-profile impact and range impact are unchanged by design.

Rollback: revert the change commit; the `registries` member, the dialect and the five unknown
codes disappear together and the default receipt is unaffected.
