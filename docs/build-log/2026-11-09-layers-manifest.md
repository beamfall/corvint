# V1-5233: the area manifest keyed to the five layers, cross-layer imports and per-layer test time

Experimental, under the owner's standing lane auto-accept; base
`9cb7692432a23f469c2e14aaaf63169301485c9e`, branch `claude/layers-manifest-5233`. Decision 0594
item 8 chose layers over repositories and asked that per-layer test time and cross-layer imports
be measured before the import rule is enforced, `cmd/corvint`'s test package is split, CI is
sharded by layer and each binary is released on its own. This entry is that measurement.

## What changed

`tools/import-ratchet/areas.json` names five areas instead of the panel's eleven: `shared`
(layer 0), `analyzers`, `tests` and `docs` (layer 1) and `release` (layer 2). The rule is
unchanged (IRT-V0-002): a package may import its own layer or a strictly lower one, so shared
imports no other layer, the three products import only shared, and release may import any layer.
`tools/import-ratchet/baseline.json` is regenerated from the scanner (298 production packages):
44 wrong-way edges. The IRT-V0-005 control now builds its added edge into the highest non-entry
layer instead of the removed `cli` area; `docs/specs/import-ratchet-v0.md` records both.

Keying principles, in order: a package is keyed by its role in the decision's layer list; a
leaf that two products need sits in `shared` (`pythongrammar`, `pythonsyntax`, the ruby, shader
and web dialects, `langplugin/decl`, `langplugin/dialect`, `langplugin/runnernames`); each `cmd`
binary is keyed to its product (`corvint`, `corvint-mcp`, `corvint-test-validity-mcp`, the test
providers, the test runner and `corvint-cli` to `tests`; `corvint-docs-mcp` and
`corvint-web-flows` to `docs`; every `corvint-analyzer-*` to `analyzers`; the release, update
and receipt binaries to `release`); `.github`, `tools`, `script`, `conformance`, `benchmarks`
and `examples` are `release`. `internal/cem` is one `shared` prefix, so the narrow `corvint cem`
being restored in parallel maps without a manifest edit. `internal/mcp` is `shared` plumbing
with its product bridges keyed out (`testvaliditybridge` and `cli` to `tests`, `corpusbridge`
and `docsbridge` to `docs`). `workqueue`, `worksource` and `worklistadapter` are keyed `shared`
only because no kept binary reaches them; they are Tasks leftovers for the cut to remove.

## Cross-layer imports (production, 44 edges)

| From | To | Edges | Importing packages |
|---|---|---|---|
| shared | tests | 14 | `appflows` (8), `appmap` (2), `flowcoverage`, `mcp/bridge`, `mcp/toolgroup` (2) |
| tests | docs | 10 | `cmd/corvint` (7: the docs verbs, `doccorpus`, `flowdocs`, `mcp/corpusbridge`), `mcp/cli` (3) |
| shared | docs | 8 | `appflows`, `appmap`, `flowcoverage` (2), `flowcoverage/testfixture`, `mcp/toolgroup` (3) |
| docs | tests | 8 | `doccorpus` (5: `jstestprovider`, `liveverify/affected`, `testvalidity`, `testvaliditydoc`), `command/flows`, the two docs binaries (`swiftimpact`) |
| tests | analyzers | 2 | `liveverify/rbmutate` to `analyzerruby`; `testrunner/registry` to `analyzerrust` |
| tests | release | 2 | `cmd/corvint` and `sessiontest` read `.github/cishards` |

Test-only (`_test.go`) imports that cross the same way are not in the baseline (IRT-V0 non-goal)
but matter for "run a layer's tests plus dependents": shared tests reach tests (16) and docs (4);
tests reach docs (5) and release (1); docs reach tests (5); analyzers reach tests (1).

The two largest debts are the flows and E2E mapping packages (`appflows`, `appmap`,
`flowcoverage`), which the decision places in shared but which import the test-selection and
corpus layers, and `cmd/corvint`, which still carries the docs verbs until the `corvint-docs`
binary exists. No analyzer package imports another layer, and no docs or tests package imports
analyzers except the two edges above.

## Per-layer test time

Source: `.github/cishards/package-costs.json`, the hosted `go test` wall time per package that
CI sharding already uses, from run `42328363436` at revision `b75053a5b`, go1.27.3. That run
predates decision 0594, so packages cut since are excluded and kept packages whose tests the cut
removed are over-counted. Packages keyed by the longest manifest prefix; the test-only package
`protocol/integrations` counts as shared.

| Layer | Packages | With tests | Priced | Hosted test time | Tested but unpriced |
|---|---|---|---|---|---|
| shared | 73 | 60 | 65 | 910 s (15.2 min) | 6 |
| analyzers | 31 | 31 | 31 | 304 s (5.1 min) | 0 |
| tests | 88 | 79 | 74 | 3124 s (52.1 min) | 12 |
| docs | 13 | 13 | 13 | 614 s (10.2 min) | 0 |
| release | 100 | 86 | 86 | 1043 s (17.4 min) | 12 |
| total | 305 | 269 | 269 | 5996 s (99.9 min) | 30 |

Largest packages: tests `cmd/corvint` 1147 s and `liveverify/affected` 1085 s (71 percent of the
layer); shared `contextindex` 179 s, `appmap` 172 s, `procgroup` 106 s, `secretscreen` 101 s;
docs `doccorpus` 341 s; release `tools/cem-stable-regen` 153 s, `tools/corvint-pr-tests` 114 s;
analyzers `analyzernativebridge` 79 s, `analyzerkotlinandroid` 73 s. The unpriced tested
packages are small CLI, runner-protocol and tooling packages that the source run did not list.

Post-cut local timing is `NOT_OBSERVED`: a whole-suite run exceeds one lane command's bound
(`cmd/corvint` alone runs about 591 s on a quiet host), and the lane may not start background
jobs. The next complete CI run on `main` refreshes the table through `tools/ci-shard-costs`.

## Commands

- `script/heavy.sh GOTOOLCHAIN=local go test -count=1 -v ./tools/import-ratchet/`: PASS,
  TestImportRatchet 0.09 s, 298 packages, 44 baselined edges (before the re-key: 37 stale
  prefixes and 22 stale baseline edges failed it).
- `GOTOOLCHAIN=local go vet ./tools/import-ratchet/`, `gofmt -l tools/import-ratchet`: clean.

## Non-goals and rollback

No package moves, no new edge is removed, no CI or release change; the follow-ups enforce the
rule, split `cmd/corvint`'s tests, shard by layer and release per binary. Rollback: revert this
change; the eleven-area manifest and its 62-edge baseline return (both already failed on the
post-cut tree).
