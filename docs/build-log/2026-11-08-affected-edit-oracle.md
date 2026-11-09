# Controlled affected-plan edit oracle (V1-5181)

Date: 2026-11-08 (America/Toronto)
Intent: owner-directed controlled measurement, AEO-V0-001 through AEO-V0-008.
Outcome: harness verification passed; planner quality evaluation FAILED. This lane changes no
`internal/liveverify/affected` implementation. No Tasks command, PR, push or selector repair was run.

## Design and evidence boundary

A new `tools/affected-edit-oracle` separates interventions from the historical merge replay in
`tools/affected-zero-unknown-replay` and `tools/selection-baseline`, external analyzer qualification
in `tools/beamfall-shadow`, and the CI decision policy in `tools/gate-affected-select`. Those four
homes were read before implementation. The tool reads `plan.selected[].{kind,unitId,witness}` and
`summary.line` from the installed CLI; it imports process ownership, never selection logic.

The author wrote expectations from source/test relationships before observation. Direct tests are
required, Go production-import consumers are traced transitively, and test-only importer edges
terminate at their test package. Body cases permit additional importer tests; API cases require them.
Documentation/comment probes assert a semantic no-op, so source-inspection sentinels can appear as
OVER. These are corpus-relative excesses requiring adjudication, not proof that all 7,506 are useless.
No witness-wide allowances are used. Each case retains its rationale in `why`.

A first 79-case precision run had 8,073 selections, 134 missing required IDs and 7,410 excesses.
Self-review found its import traversal incorrectly propagated test-only imports to production
consumers. Source-derived correction removed those edges, without consulting selections to add
allowances. The final full run below re-executed all cases and all mutations. The initial corpus
and report remain local at `/private/tmp/aeo-corpus-initial.json` and `/private/tmp/aeo-precision.json`;
this paragraph retains the failed evaluation, not a promotion claim.

Single builder; independent review NOT_RUN under the owner's explicit no-subagents restriction.
No universal recall, test-filter completeness, mutation adequacy, performance savings or release
qualification is claimed. Mutation commands are trusted corpus inputs, not a security sandbox.

## Reproduction and identities

Local source clones used exactly `git clone --quiet --no-hardlinks --shared SOURCE DEST`, then a
detached checkout of the full observed SHA, under the owner-selected private `scratchpad/oracle-clones`
parent. Every case applied and committed its one deterministic edit before running the installed
`corvint affected --base HEAD~1 --format json`. Git identity/time are fixed, hooks/configuration are
neutralized, and results are sorted. `beamfall`'s uncommitted workflow edits never enter a clone.

Installed planner: `Corvint 1.0.0-rc.3.dev (build 137, commit 447bdcbd5963683f5f2b4934a45b8f48fc6742a3)`.
Toolchains: Go 1.27.3; Node v22.23.4 with `--experimental-strip-types` for the web assertion script.
The optional updater check returned freshness UNKNOWN because its process-cleanup probe was denied;
the official releases API read returned an empty list, so available release versions remain unknown.
The requested installed binary was preserved. Tasks version/checks were not invoked.

- Corpus SHA-256: `4716cf5f96dc19e36d052eb3994fde2acac9c14b8ef2ce83e62a687cf4c6667e`.
- Planner binary SHA-256: `fc0808c68898b4eae78203361bb9ed4528f186b84bc1142d4c2ab9e0ef7e13e8`.
- Harness executable SHA-256: `c7ddea433b42a546e77532c0037a65fbfd2cf0b58d457147975480a495497e2d`.
- Final JSON report SHA-256: `e6353baab8e8161f5da11483593e0496047f96eaf11b37e6215a473bb27cdc9f`.
- Initial corpus SHA-256: `59c57212a1da8386a89721e57b3f53954e13d88fe25725683d22756f92867619`.
- Initial JSON report SHA-256: `56c9e0755750a557848b63c3e0f231e2e9066c2e3fefe4f8ba15e03589ea227a`.

| Repository | Pinned base | Cases | Edit classes | Mutations |
| --- | --- | ---: | ---: | ---: |
| beamfall-workspace/beamfall | `45d0511ca65997c69480c82998cf1ff5f8060ebc` | 16 | 16 | 1 |
| beamfall-workspace/beamfall-cloud | `a8119da3c900b010bb73a9568485d588985d9da8` | 15 | 15 | 1 |
| beamfall-workspace/beamfall-plugin-sdk | `8f63b914a73c2051923906831fbe5257fed8e798` | 14 | 14 | 1 |
| beamfall-workspace/beamfall-web | `c1cd05fa57be28013f973b47e2692deab33a7d4f` | 15 | 15 | 1 |
| corvint | `447bdcbd5963683f5f2b4934a45b8f48fc6742a3` | 19 | 16 | 4 |

Web has no Go module, so applicable source/API/test cases use TS; SDK has no Makefile; cloud/SDK
have no TS/Swift/Kotlin production leaf. Corvint probes its VS Code TS leaf, excluding Swift/Kotlin
parser fixtures. The corpus has 14–16 distinct classes per repository (19 Corvint cases include four
body/mutation sites). The README explains applicability.

Both invocations used the explicit owner clone parent:
`/private/tmp/claude-501/-Users-russelllewis-projects-corvint/ffb5f99f-a0b4-4979-af00-7484072c3085/scratchpad/oracle-clones`.

```sh
/private/tmp/affected-edit-oracle --clones "$ORACLE_CLONES" --out /private/tmp/aeo-precision.json
/private/tmp/affected-edit-oracle-final --clones "$ORACLE_CLONES" --mutate --out /private/tmp/aeo-final.json
```

The first command measured the initial expectation draft. The second measured the final corpus:
precision comparisons on every case, plus baseline/kill/selection checks on all eight mutation
cases. Both exited 1 for quality findings, with zero execution errors. `--allow-over` deliberately
was not used; its unit test proves it cannot waive FAIL or SAFETY_FAIL. Raw final progress retains
every excess unit, witness and dirty path in `/private/tmp/aeo-final.log`; the JSON also retains
viaLength, summaries, missing IDs and failing test names. The tables below are the durable result.

## Final totals

Counts are case/unit occurrences, not unique test names across the corpus.

| Metric | Count |
| --- | ---: |
| cases | 79 |
| PASS | 5 |
| OVER | 73 |
| FAIL | 1 |
| ERROR | 0 |
| selected | 8073 |
| must | 190 |
| mustHit | 189 |
| missing | 1 |
| over | 7506 |
| SAFETY_PASS | 8 |
| SAFETY_FAIL | 0 |
| SAFETY_ERROR | 0 |

| Excess witness kind | Count |
| --- | ---: |
| BINARY_EXEC | 171 |
| DECLARED_CONSUMER | 10 |
| DECLARED_EXISTENCE_READER | 1 |
| DECLARED_READ_SCOPE | 109 |
| DEPENDENCY_PATH | 1979 |
| DIRECT_SOURCE_CHANGE | 4 |
| ENCLOSING_PACKAGE | 10 |
| FRONTIER_SITE | 2511 |
| IMPORT_GRAPH_READER | 85 |
| PATH_GLOB_READER | 50 |
| PATH_LITERAL_READER | 544 |
| UNBOUNDED_READER | 2032 |

## Per-case precision and safety

`hit` is required IDs hit/required IDs; `—` means no opt-in mutation for this case. An empty excess
cell means zero excess. Unknowns remain in the JSON and are counted here; they never prove safety.
Every empty-diff case selected zero units. All runs had zero materialization/command errors.

| Case | Status | Selected | hit | OVER by witness kind | Unknowns | Safety |
| --- | --- | ---: | ---: | --- | ---: | --- |
| beamfall-api | OVER | 174 | 45/45 | DEPENDENCY_PATH=28, FRONTIER_SITE=59, PATH_GLOB_READER=2, PATH_LITERAL_READER=4, UNBOUNDED_READER=36 | 0 | — |
| beamfall-body | OVER | 174 | 1/1 | DEPENDENCY_PATH=28, FRONTIER_SITE=59, PATH_GLOB_READER=2, PATH_LITERAL_READER=4, UNBOUNDED_READER=36 | 0 | PASS |
| beamfall-build-log | OVER | 146 | 0/0 | DEPENDENCY_PATH=88, FRONTIER_SITE=5, PATH_GLOB_READER=2, UNBOUNDED_READER=51 | 28 | — |
| beamfall-cem | OVER | 144 | 0/0 | DEPENDENCY_PATH=11, FRONTIER_SITE=65, PATH_GLOB_READER=2, UNBOUNDED_READER=66 | 27 | — |
| beamfall-cloud-api | OVER | 35 | 1/1 | DEPENDENCY_PATH=9, FRONTIER_SITE=12, IMPORT_GRAPH_READER=1, PATH_GLOB_READER=1, PATH_LITERAL_READER=5, UNBOUNDED_READER=6 | 1 | — |
| beamfall-cloud-body | OVER | 35 | 1/1 | DEPENDENCY_PATH=9, FRONTIER_SITE=12, IMPORT_GRAPH_READER=1, PATH_GLOB_READER=1, PATH_LITERAL_READER=5, UNBOUNDED_READER=6 | 1 | PASS |
| beamfall-cloud-build-log | OVER | 30 | 0/0 | DEPENDENCY_PATH=2, FRONTIER_SITE=18, PATH_GLOB_READER=1, UNBOUNDED_READER=9 | 2 | — |
| beamfall-cloud-cem | OVER | 30 | 0/0 | DEPENDENCY_PATH=2, FRONTIER_SITE=18, PATH_GLOB_READER=1, UNBOUNDED_READER=9 | 2 | — |
| beamfall-cloud-comment | OVER | 35 | 0/0 | DEPENDENCY_PATH=9, DIRECT_SOURCE_CHANGE=1, FRONTIER_SITE=12, IMPORT_GRAPH_READER=1, PATH_GLOB_READER=1, PATH_LITERAL_READER=5, UNBOUNDED_READER=6 | 1 | — |
| beamfall-cloud-config | OVER | 30 | 0/0 | DEPENDENCY_PATH=2, FRONTIER_SITE=18, PATH_GLOB_READER=1, UNBOUNDED_READER=9 | 2 | — |
| beamfall-cloud-data | OVER | 32 | 1/1 | DEPENDENCY_PATH=3, FRONTIER_SITE=15, PATH_LITERAL_READER=9, UNBOUNDED_READER=4 | 2 | — |
| beamfall-cloud-delete-data | OVER | 31 | 0/0 | DEPENDENCY_PATH=11, FRONTIER_SITE=11, PATH_GLOB_READER=1, UNBOUNDED_READER=8 | 2 | — |
| beamfall-cloud-empty | PASS | 0 | 0/0 | 0 | 0 | — |
| beamfall-cloud-go-mod | OVER | 32 | 0/0 | DEPENDENCY_PATH=9, FRONTIER_SITE=8, IMPORT_GRAPH_READER=1, PATH_LITERAL_READER=6, UNBOUNDED_READER=8 | 3 | — |
| beamfall-cloud-make | OVER | 30 | 1/1 | DEPENDENCY_PATH=3, FRONTIER_SITE=16, PATH_GLOB_READER=1, PATH_LITERAL_READER=1, UNBOUNDED_READER=8 | 1 | — |
| beamfall-cloud-new-test | OVER | 32 | 1/1 | DEPENDENCY_PATH=9, FRONTIER_SITE=12, IMPORT_GRAPH_READER=1, PATH_GLOB_READER=1, UNBOUNDED_READER=8 | 1 | — |
| beamfall-cloud-readme | OVER | 31 | 0/0 | DEPENDENCY_PATH=11, FRONTIER_SITE=11, PATH_GLOB_READER=1, UNBOUNDED_READER=8 | 2 | — |
| beamfall-cloud-shell | OVER | 31 | 0/0 | DEPENDENCY_PATH=10, FRONTIER_SITE=11, PATH_GLOB_READER=1, PATH_LITERAL_READER=2, UNBOUNDED_READER=7 | 3 | — |
| beamfall-cloud-workflow | OVER | 30 | 1/1 | DEPENDENCY_PATH=1, FRONTIER_SITE=18, PATH_GLOB_READER=1, PATH_LITERAL_READER=1, UNBOUNDED_READER=8 | 1 | — |
| beamfall-comment | OVER | 174 | 0/0 | DEPENDENCY_PATH=72, DIRECT_SOURCE_CHANGE=1, FRONTIER_SITE=59, PATH_GLOB_READER=2, PATH_LITERAL_READER=4, UNBOUNDED_READER=36 | 0 | — |
| beamfall-config | OVER | 144 | 0/0 | DEPENDENCY_PATH=11, FRONTIER_SITE=65, PATH_GLOB_READER=2, UNBOUNDED_READER=66 | 29 | — |
| beamfall-data | OVER | 150 | 1/1 | DEPENDENCY_PATH=87, FRONTIER_SITE=5, PATH_GLOB_READER=2, PATH_LITERAL_READER=18, UNBOUNDED_READER=37 | 20 | — |
| beamfall-delete-data | OVER | 148 | 0/0 | DEPENDENCY_PATH=89, FRONTIER_SITE=5, PATH_GLOB_READER=2, PATH_LITERAL_READER=5, UNBOUNDED_READER=47 | 30 | — |
| beamfall-empty | PASS | 0 | 0/0 | 0 | 0 | — |
| beamfall-go-mod | OVER | 147 | 0/0 | DEPENDENCY_PATH=76, FRONTIER_SITE=2, IMPORT_GRAPH_READER=1, PATH_LITERAL_READER=22, UNBOUNDED_READER=46 | 30 | — |
| beamfall-make | OVER | 144 | 1/1 | DEPENDENCY_PATH=18, FRONTIER_SITE=62, PATH_GLOB_READER=2, PATH_LITERAL_READER=2, UNBOUNDED_READER=59 | 1 | — |
| beamfall-new-test | OVER | 145 | 1/1 | DEPENDENCY_PATH=22, FRONTIER_SITE=65, IMPORT_GRAPH_READER=1, PATH_GLOB_READER=2, UNBOUNDED_READER=54 | 0 | — |
| beamfall-plugin-sdk-api | OVER | 9 | 3/3 | DEPENDENCY_PATH=3, FRONTIER_SITE=1, UNBOUNDED_READER=2 | 0 | — |
| beamfall-plugin-sdk-body | OVER | 9 | 1/1 | DEPENDENCY_PATH=3, FRONTIER_SITE=1, UNBOUNDED_READER=2 | 0 | PASS |
| beamfall-plugin-sdk-build-log | OVER | 6 | 0/0 | ENCLOSING_PACKAGE=1, FRONTIER_SITE=3, UNBOUNDED_READER=2 | 1 | — |
| beamfall-plugin-sdk-cem | OVER | 6 | 0/0 | ENCLOSING_PACKAGE=1, FRONTIER_SITE=3, UNBOUNDED_READER=2 | 1 | — |
| beamfall-plugin-sdk-comment | OVER | 9 | 0/0 | DEPENDENCY_PATH=5, DIRECT_SOURCE_CHANGE=1, FRONTIER_SITE=1, UNBOUNDED_READER=2 | 0 | — |
| beamfall-plugin-sdk-config | OVER | 9 | 0/0 | DEPENDENCY_PATH=5, ENCLOSING_PACKAGE=1, FRONTIER_SITE=1, UNBOUNDED_READER=2 | 1 | — |
| beamfall-plugin-sdk-data | OVER | 9 | 1/1 | DEPENDENCY_PATH=3, ENCLOSING_PACKAGE=1, FRONTIER_SITE=1, PATH_LITERAL_READER=1, UNBOUNDED_READER=2 | 2 | — |
| beamfall-plugin-sdk-delete-data | OVER | 9 | 0/0 | DEPENDENCY_PATH=5, ENCLOSING_PACKAGE=1, FRONTIER_SITE=1, UNBOUNDED_READER=2 | 2 | — |
| beamfall-plugin-sdk-empty | PASS | 0 | 0/0 | 0 | 0 | — |
| beamfall-plugin-sdk-go-mod | OVER | 9 | 0/0 | DEPENDENCY_PATH=5, ENCLOSING_PACKAGE=1, PATH_LITERAL_READER=2, UNBOUNDED_READER=1 | 1 | — |
| beamfall-plugin-sdk-new-test | OVER | 7 | 1/1 | DEPENDENCY_PATH=3, FRONTIER_SITE=1, UNBOUNDED_READER=2 | 0 | — |
| beamfall-plugin-sdk-readme | OVER | 9 | 0/0 | DEPENDENCY_PATH=5, ENCLOSING_PACKAGE=1, FRONTIER_SITE=1, UNBOUNDED_READER=2 | 2 | — |
| beamfall-plugin-sdk-shell | OVER | 8 | 1/1 | DEPENDENCY_PATH=3, ENCLOSING_PACKAGE=1, FRONTIER_SITE=1, UNBOUNDED_READER=2 | 1 | — |
| beamfall-plugin-sdk-workflow | OVER | 6 | 1/1 | DEPENDENCY_PATH=1, ENCLOSING_PACKAGE=1, FRONTIER_SITE=1, PATH_LITERAL_READER=2 | 0 | — |
| beamfall-readme | OVER | 148 | 0/0 | DEPENDENCY_PATH=89, FRONTIER_SITE=5, PATH_GLOB_READER=2, PATH_LITERAL_READER=5, UNBOUNDED_READER=47 | 30 | — |
| beamfall-shell | OVER | 145 | 1/1 | DEPENDENCY_PATH=24, FRONTIER_SITE=63, PATH_GLOB_READER=2, PATH_LITERAL_READER=8, UNBOUNDED_READER=47 | 1 | — |
| beamfall-ts-leaf | OVER | 179 | 0/0 | BINARY_EXEC=1, DEPENDENCY_PATH=50, ENCLOSING_PACKAGE=1, FRONTIER_SITE=60, PATH_GLOB_READER=2, PATH_LITERAL_READER=28, UNBOUNDED_READER=37 | 51 | — |
| beamfall-web-api | OVER | 22 | 1/1 | DEPENDENCY_PATH=4, FRONTIER_SITE=1, PATH_LITERAL_READER=16 | 47 | — |
| beamfall-web-body | OVER | 5 | 1/1 | DEPENDENCY_PATH=3, FRONTIER_SITE=1 | 47 | PASS |
| beamfall-web-build-log | OVER | 1 | 0/0 | FRONTIER_SITE=1 | 17 | — |
| beamfall-web-cem | OVER | 1 | 0/0 | FRONTIER_SITE=1 | 17 | — |
| beamfall-web-comment | OVER | 22 | 0/0 | DEPENDENCY_PATH=5, FRONTIER_SITE=1, PATH_LITERAL_READER=16 | 47 | — |
| beamfall-web-config | OVER | 1 | 0/0 | FRONTIER_SITE=1 | 17 | — |
| beamfall-web-data | FAIL | 1 | 0/1 | FRONTIER_SITE=1 | 17 | — |
| beamfall-web-delete-data | OVER | 2 | 0/0 | FRONTIER_SITE=1, PATH_LITERAL_READER=1 | 48 | — |
| beamfall-web-empty | PASS | 0 | 0/0 | 0 | 0 | — |
| beamfall-web-make | OVER | 2 | 1/1 | FRONTIER_SITE=1 | 1 | — |
| beamfall-web-new-test | OVER | 20 | 1/1 | FRONTIER_SITE=1, PATH_LITERAL_READER=18 | 47 | — |
| beamfall-web-readme | OVER | 2 | 0/0 | FRONTIER_SITE=1, PATH_LITERAL_READER=1 | 48 | — |
| beamfall-web-shell | OVER | 7 | 0/0 | FRONTIER_SITE=1, PATH_LITERAL_READER=6 | 3 | — |
| beamfall-web-ts-leaf | OVER | 22 | 1/1 | DEPENDENCY_PATH=4, FRONTIER_SITE=1, PATH_LITERAL_READER=16 | 47 | — |
| beamfall-web-workflow | OVER | 3 | 1/1 | FRONTIER_SITE=1, PATH_LITERAL_READER=1 | 1 | — |
| beamfall-workflow | OVER | 144 | 1/1 | DEPENDENCY_PATH=10, FRONTIER_SITE=65, PATH_GLOB_READER=2, UNBOUNDED_READER=66 | 0 | — |
| corvint-api | OVER | 371 | 108/108 | BINARY_EXEC=29, DECLARED_CONSUMER=2, DECLARED_READ_SCOPE=6, DEPENDENCY_PATH=57, FRONTIER_SITE=93, IMPORT_GRAPH_READER=6, PATH_GLOB_READER=1, PATH_LITERAL_READER=14, UNBOUNDED_READER=55 | 66 | — |
| corvint-body-archivepath | OVER | 271 | 1/1 | BINARY_EXEC=4, DECLARED_READ_SCOPE=7, DEPENDENCY_PATH=41, FRONTIER_SITE=93, IMPORT_GRAPH_READER=13, PATH_GLOB_READER=1, PATH_LITERAL_READER=18, UNBOUNDED_READER=80 | 66 | PASS |
| corvint-body-jsonend | OVER | 338 | 1/1 | BINARY_EXEC=27, DECLARED_CONSUMER=2, DECLARED_READ_SCOPE=7, DEPENDENCY_PATH=63, FRONTIER_SITE=93, IMPORT_GRAPH_READER=8, PATH_GLOB_READER=1, PATH_LITERAL_READER=14, UNBOUNDED_READER=58 | 66 | PASS |
| corvint-body-runtimeenv | OVER | 371 | 1/1 | BINARY_EXEC=29, DECLARED_CONSUMER=2, DECLARED_READ_SCOPE=6, DEPENDENCY_PATH=57, FRONTIER_SITE=93, IMPORT_GRAPH_READER=6, PATH_GLOB_READER=1, PATH_LITERAL_READER=14, UNBOUNDED_READER=55 | 66 | PASS |
| corvint-body-taskprojection | OVER | 412 | 1/1 | BINARY_EXEC=26, DECLARED_CONSUMER=2, DECLARED_READ_SCOPE=6, DEPENDENCY_PATH=68, FRONTIER_SITE=92, IMPORT_GRAPH_READER=5, PATH_GLOB_READER=1, PATH_LITERAL_READER=14, UNBOUNDED_READER=49 | 66 | PASS |
| corvint-build-log | OVER | 233 | 0/0 | BINARY_EXEC=6, DEPENDENCY_PATH=79, FRONTIER_SITE=81, PATH_LITERAL_READER=9, UNBOUNDED_READER=58 | 5 | — |
| corvint-cem | OVER | 213 | 0/0 | BINARY_EXEC=5, DECLARED_READ_SCOPE=1, DEPENDENCY_PATH=82, FRONTIER_SITE=115, PATH_LITERAL_READER=10 | 0 | — |
| corvint-comment | OVER | 371 | 0/0 | BINARY_EXEC=32, DECLARED_CONSUMER=2, DECLARED_READ_SCOPE=6, DEPENDENCY_PATH=161, DIRECT_SOURCE_CHANGE=1, FRONTIER_SITE=93, IMPORT_GRAPH_READER=6, PATH_GLOB_READER=1, PATH_LITERAL_READER=14, UNBOUNDED_READER=55 | 66 | — |
| corvint-config | OVER | 226 | 0/0 | DEPENDENCY_PATH=5, FRONTIER_SITE=93, UNBOUNDED_READER=128 | 76 | — |
| corvint-data | OVER | 247 | 1/1 | BINARY_EXEC=2, DEPENDENCY_PATH=50, FRONTIER_SITE=92, PATH_LITERAL_READER=29, UNBOUNDED_READER=73 | 74 | — |
| corvint-delete-data | OVER | 250 | 0/0 | BINARY_EXEC=3, DECLARED_EXISTENCE_READER=1, DEPENDENCY_PATH=97, FRONTIER_SITE=79, PATH_LITERAL_READER=23, UNBOUNDED_READER=47 | 76 | — |
| corvint-empty | PASS | 0 | 0/0 | 0 | 0 | — |
| corvint-go-mod | OVER | 312 | 0/0 | BINARY_EXEC=4, DECLARED_READ_SCOPE=26, DEPENDENCY_PATH=98, FRONTIER_SITE=89, IMPORT_GRAPH_READER=17, PATH_LITERAL_READER=58, UNBOUNDED_READER=20 | 77 | — |
| corvint-make | OVER | 245 | 1/1 | DECLARED_READ_SCOPE=2, DEPENDENCY_PATH=18, FRONTIER_SITE=92, PATH_GLOB_READER=1, PATH_LITERAL_READER=26, UNBOUNDED_READER=105 | 66 | — |
| corvint-new-test | OVER | 267 | 1/1 | DECLARED_READ_SCOPE=29, DEPENDENCY_PATH=36, FRONTIER_SITE=93, IMPORT_GRAPH_READER=17, PATH_GLOB_READER=2, PATH_LITERAL_READER=1, UNBOUNDED_READER=88 | 65 | — |
| corvint-readme | OVER | 249 | 0/0 | BINARY_EXEC=3, DEPENDENCY_PATH=97, FRONTIER_SITE=79, PATH_LITERAL_READER=23, UNBOUNDED_READER=47 | 76 | — |
| corvint-shell | OVER | 247 | 1/1 | DECLARED_READ_SCOPE=13, DEPENDENCY_PATH=38, FRONTIER_SITE=93, PATH_GLOB_READER=1, PATH_LITERAL_READER=22, UNBOUNDED_READER=79 | 66 | — |
| corvint-ts-leaf | OVER | 248 | 1/1 | DEPENDENCY_PATH=42, FRONTIER_SITE=93, PATH_LITERAL_READER=23, UNBOUNDED_READER=89 | 67 | — |
| corvint-workflow | OVER | 236 | 1/1 | DEPENDENCY_PATH=40, FRONTIER_SITE=89, PATH_LITERAL_READER=18, UNBOUNDED_READER=88 | 66 | — |

## Mutation observations

All eight baselines passed and each mutant produced an assertion failure before its failing unit
was checked against that mutant’s plan. No compilation/setup failure was counted as a kill. Each
command finished inside its enforced two-minute wall limit; this is a bound, not a timing benchmark.

| Case | Failing units | Failing tests | Safety |
| --- | --- | --- | --- |
| beamfall-body | `go:github.com/beamfall/core/internal/serverrole` | `go:github.com/beamfall/core/internal/serverrole#TestZeroRoleIsMainAndAllowsEveryFace` | PASS |
| beamfall-cloud-body | `go:github.com/beamfall/cloud/internal/push` | `go:github.com/beamfall/cloud/internal/push#TestPayloadContainsOnlyTypeAndOpaqueEventID` | PASS |
| beamfall-plugin-sdk-body | `go:github.com/beamfall/plugin-sdk` | `go:github.com/beamfall/plugin-sdk#TestL` | PASS |
| beamfall-web-body | `typescript:vitest:ui-catalog/semantics.test.ts` | `typescript:vitest:ui-catalog/semantics.test.ts` | PASS |
| corvint-body-archivepath | `go:github.com/Beamfall/corvint/internal/archivepath` | `go:github.com/Beamfall/corvint/internal/archivepath#TestV13179JoinKeepsLocalMembers` | PASS |
| corvint-body-jsonend | `go:github.com/Beamfall/corvint/internal/jsonend` | `go:github.com/Beamfall/corvint/internal/jsonend#TestDoneRefusesTrailingDelimitersMoreAccepts` | PASS |
| corvint-body-runtimeenv | `go:github.com/Beamfall/corvint/internal/runtimeenv` | `go:github.com/Beamfall/corvint/internal/runtimeenv#TestResolveNamespace`; `go:github.com/Beamfall/corvint/internal/runtimeenv#TestResolveNamespace/current` | PASS |
| corvint-body-taskprojection | `go:github.com/Beamfall/corvint/internal/taskprojection` | `go:github.com/Beamfall/corvint/internal/taskprojection#TestOnlyProjection` | PASS |

## Recall finding for the selection lanes

`beamfall-web-data` misses `typescript:vitest:test/tokenContract.test.ts`. The case appends JSON
whitespace to `test/fixtures/beamfall-tokens.json` at web base
`c1cd05fa57be28013f973b47e2692deab33a7d4f`. The test imports `test/token-contract.config.json`, whose
`artifact` names that fixture; it resolves `defaultJson` and copies it into its mutation fixtures.
This expected dependency was authored before the planner run. Reproduce with this case's exact
corpus edit and the pinned planner. No assertion failure is claimed for the whitespace edit: it is
a required dependency omission, not an observed SAFETY_FAIL. The web behavioral mutation at the
separate `ui-catalog/semantics.ts` site was killed and selected.

Final review also added a committed-path assertion: every non-empty edit must produce exactly its
requested path in the parent-to-HEAD diff. A synthetic ignored-file case proves a silently empty
commit is an ERROR. The full precision/mutation evaluation was repeated after this guard; the
pre-guard report is retained locally at `/private/tmp/aeo-before-path-check.json`. The final report
compared byte-for-byte equal with `cmp`, including every excess witness, mutation result and summary;
this supplies a full-corpus deterministic-output observation.

No selection repair or Tasks filing was made: the owner assigned selection changes to sibling lanes
and explicitly prohibited `corvint-tasks` in this lane.

## Verification and negative control

All local heavy calls after the initial admission mistake set `HEAVY_REMOTE=0`,
`HEAVY_LOCAL_REASON='owner requires local execution'` and `HEAVY_NICE=0`. The harness also strips
inherited remote host overrides and sets those controls itself. The first compile-only invocation
of `script/heavy.sh` automatically offloaded to configured `wsl-lanes` before this was noticed;
it returned `[no test files]`, and is NOT the verification evidence. This violated the requested
local-only boundary and was disclosed immediately. The later real tests and all oracle runs were
local. No push or hosted network write was made; the offload was an SSH scratch write.

| Command/check | Result |
| --- | --- |
| `corvint context --task ... --limit 8` | READY with omissions and unresolved ticket anchor retained; no Tasks lookup |
| `corvint affected --base 447bdcbd5963683f5f2b4934a45b8f48fc6742a3 --format json` before changes | BOUNDED, selected=0, unknown=0 |
| Same affected command on the substantive staged change | UNKNOWN, selected=355, unknown=78; new tool package selected; focused owner verification used |
| `script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m ./tools/affected-edit-oracle/` | PASS locally on final Go implementation |
| `GOTOOLCHAIN=local go vet ./tools/affected-edit-oracle/` | PASS |
| `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 GOTOOLCHAIN=local go vet ./tools/affected-edit-oracle/` | PASS on final Go implementation |
| `make windows-vet` | PASS before the last platform-independent receipt-validation test; final scoped Windows vet above also passed |
| `make spec-requirements` after staging | PASS on final rerun; initial generation wrote 8 rows but internal auto-stage hit index.lock Operation not permitted; direct staging recovered it |
| `make spec-requirements-check` | PASS after staging generated outputs |
| `make requirement-id-collisions` | PASS: 8 new IDs, no collisions across 77 open refs |
| `make test-commands-check` | PASS; existing Go-suite declaration already includes the new package |
| `make test-read-scopes-check` | PASS; new package registered with no repository read scope |
| `make unbounded-readers-check` | PASS; existing stale-entry advisory retained, no unrelated declaration edits |
| `make changed-checks BASE=447bdcbd5963683f5f2b4934a45b8f48fc6742a3` | PASS after appending the spec index row instead of shifting existing cited rows |
| Deliberately wrong comparator | Expected FAIL in `TestCompare_AEO_V0_003_005`; restored immediately, then full focused tests passed |
| Full final precision + mutation harness | Expected exit 1: one required omission and 7,506 excesses; eight mutation recall passes |
| `make dogfood-change BASE=447bdcbd5963683f5f2b4934a45b8f48fc6742a3` | BLOCKED: mkdir of private worktree Git/corvint directory denied by sandbox; CEM/check/seal NOT_PRODUCED |
| `make gate` / independent review / Tasks completion | NOT_RUN: owner-scoped checks, no-subagents and no-Tasks restrictions |

The negative control temporarily replaced `out.MustHit++` with `out.MustHit += 0` in `compare`.
The comparison test failed on its own semantic assertion with `MustHit:0` (expected 1), not a
fixture/compiler failure; process exit was 1. An EXIT trap restored the original bytes, `cmp`
confirmed restoration, and the full focused test suite subsequently passed. The synthetic Git
integration test separately demonstrates SAFETY_FAIL when its real killed Go mutation's unit is
removed from the protocol stub's plan. It also proves zero selection for an empty commit,
source preservation, deletion of all case clones, refusal to clone inside a source, rejection of an
ignored added file that Git would otherwise omit from the commit, and rejection
of a receipt missing `plan.selected`. Process cancellation is checked with a child `sleep` process.

The unbounded-reader advisory lists existing stale declarations in `internal/tasks/cli` and seven
shell tests. It does not fail the gate and was not changed by this lane. Update freshness remains
UNKNOWN; no verified newer component was installed or claimed. Optional provider/account telemetry
was unavailable; no usage or cost claim is made.

## Cleanup and remaining limits

The clone parent was empty after the final run. All four Beamfall source HEADs and dirty path lists
match the starting observations: web/cloud/SDK clean; beamfall still has only the original modified
`.github/workflows/release-gate.yml` and `.github/workflows/release.yml`. No source worktree content
was used as an edit input. No owned long-running harness process remains.

Remaining limits: (1) one fixture-reader omission plus unresolved corpus-relative excesses and
planner unknowns; (2) mutation recall covers eight package/file units, not all test filters,
platforms or consumers; (3) independent review and dogfood sealing are unproduced under the explicit
lane restriction and sandbox failure. This result is experimental measurement, not qualification.
