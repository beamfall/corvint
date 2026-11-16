# 2026-11-16: retained ownership replay (V1-4685)

Requirements: AZU-V0-002, AZU-V0-009 (per-rule replay retention), AZU-V0-8900.
This is a fresh replay of the ownership implementation's original eight exact merge
trees, not a claim that today's entire AZU zero-unknown gate passes. The older
`2026-11-05-azu-new-replay-safety-repair.md` retained only aggregate counts and a
digest; this entry retains every removed unknown and the measured selection growth
in the repository. It does not copy the old host's measurements.

## Inputs and command

- Baseline source: `8e2c13aa926fcaf1731fae71b27c41c41c8f09af`.
- Candidate source: `2eaaf9fba87340641ead2ea37821f37c0e6a85e1`, the final historical
  ownership/caller repair named by the original entry.
- Replay harness and frozen list: lane base
  `df315e35056d15c7933b5ff97c605cedf5ef4919`.
- Each source extracted with `git archive REV` outside the repository; each binary
  built with `script/heavy.sh go -C DIR build -buildvcs=false -o BINARY ./cmd/corvint`,
  Go 1.27.4, Linux amd64. `-buildvcs=false` is necessary for source archives and is
  explicit in both builds. Binary digests below identify these executions.

```sh
TMPDIR=/home/agent/lanes/r1docs.tmp script/affected-zero-unknown-replay.sh \
  --corvint /home/agent/lanes/r1docs-evidence/corvint-azu-candidate \
  --baseline /home/agent/lanes/r1docs-evidence/corvint-azu-base \
  --through 8e2c13aa926fcaf1731fae71b27c41c41c8f09af \
  --out /home/agent/lanes/r1docs-evidence/ownership-replay
```

The through revision deliberately reproduces the historical eight-merge scope. It
does not include later merges through current main. The harness compares the same
unchanged historical tree with both binaries. No declaration is transplanted.

## Measured result

All eight merge rows were produced. Exit **1**, `accepted: false`: every row still
has unknowns and historical full-suite/changed-line mutation receipts are unavailable
(`NOT_PRODUCED`). An empty safety-misses array is not a measured zero-miss suite.
Independent set comparison of the two raw plans found **zero baseline selected
units dropped** on each row. The table's growth is added unit IDs; its wall time is
the candidate planner only, not a performance qualification on this shared host.

| Exact merge | Unknown before / after | Selected before / after | Added | Dropped | Tests after | Wall ms after |
|---|---:|---:|---:|---:|---:|---:|
| `8e2c13aa926fcaf1731fae71b27c41c41c8f09af` | 92 / 87 | 266 / 490 | 224 | 0 | 3054 | 10404 |
| `39c03198086681fb24f509fbd00b34696d3268c4` | 84 / 73 | 66 / 181 | 115 | 0 | 1489 | 21424 |
| `d436dff76de1b8e75c427069ae7f80003ae99bc2` | 99 / 95 | 233 / 442 | 209 | 0 | 2799 | 16147 |
| `6c33df33bc5670131482f5263b410cb980580657` | 83 / 80 | 183 / 388 | 205 | 0 | 2604 | 10431 |
| `ea770fa17992b970aeda570ed74686a16dbc441e` | 63 / 78 | 75 / 181 | 106 | 0 | 1542 | 8032 |
| `24d8c04f9f5ffaaed86759951e520cb775dc2eff` | 79 / 68 | 38 / 118 | 80 | 0 | 1264 | 8658 |
| `f13f52b1607222033b089144a744fedae0d35239` | 86 / 82 | 46 / 131 | 85 | 0 | 1299 | 8274 |
| `9fdc2f02efc11ce9ed582d8529b63fe0478fcad2` | 79 / 69 | 36 / 113 | 77 | 0 | 1243 | 8464 |

Unknown growth remains visible: the candidate also adds uncertainty in some trees,
including the net increase on ea770fa17. Removed entries are paired with wider
selection, not permission to discard a known reader/executor/importer/embedder.

## Every removed unknown entry

Each item below is the exact `reason` and `detail` from the set difference, grouped
by the full merge SHA in the table. Backticks inside details are escaped as HTML.

### 8e2c13aa926fcaf1731fae71b27c41c41c8f09af

- LANGUAGE_FRONTIER: swift:conditional-compilation
- LANGUAGE_FRONTIER: typescript:e2e-runtime-dependency:conformance/interactive-alpha/fixture/e2e.spec.js:1 &#96;@playwright/test&#96;
- LANGUAGE_FRONTIER: typescript:e2e-runtime-dependency:internal/tasks/obligation/obligation-qualification.spec.ts:6 &#96;@playwright/test&#96;
- LANGUAGE_FRONTIER: typescript:e2e-runtime-dependency:tools/web-flows/test/fixture/flows.spec.ts:1 &#96;@playwright/test&#96;
- UNOWNED_DIRTY_PATH: .github/workflows/ci-route-health.yml
- UNOWNED_DIRTY_PATH: .github/workflows/ci.yml
- UNOWNED_DIRTY_PATH: Makefile

### 39c03198086681fb24f509fbd00b34696d3268c4

- LANGUAGE_FRONTIER: cfamily:link-graph-unmodelled
- LANGUAGE_FRONTIER: cfamily:unresolved-include
- LANGUAGE_FRONTIER: dotnet:dynamic-test-discovery
- LANGUAGE_FRONTIER: dotnet:source-project-unresolved
- LANGUAGE_FRONTIER: dotnet:test-runner-filter-unresolved
- LANGUAGE_FRONTIER: ruby:implicit-test-dependency:conformance/ruby-profile-v0/rails-app/spec/requests/users_spec.rb
- LANGUAGE_FRONTIER: swift:conditional-compilation
- LANGUAGE_FRONTIER: swift:exclusion-evidence-unrepresentable
- LANGUAGE_FRONTIER: swift:unmapped-source
- LANGUAGE_FRONTIER: typescript:e2e-runtime-dependency:conformance/interactive-alpha/fixture/e2e.spec.js:1 &#96;@playwright/test&#96;
- LANGUAGE_FRONTIER: typescript:e2e-runtime-dependency:internal/tasks/obligation/obligation-qualification.spec.ts:6 &#96;@playwright/test&#96;
- LANGUAGE_FRONTIER: typescript:e2e-runtime-dependency:tools/web-flows/test/fixture/flows.spec.ts:1 &#96;@playwright/test&#96;
- UNOWNED_DIRTY_PATH: .github/workflows/ci.yml

### d436dff76de1b8e75c427069ae7f80003ae99bc2

- LANGUAGE_FRONTIER: typescript:e2e-runtime-dependency:conformance/interactive-alpha/fixture/e2e.spec.js:1 &#96;@playwright/test&#96;
- LANGUAGE_FRONTIER: typescript:e2e-runtime-dependency:internal/tasks/obligation/obligation-qualification.spec.ts:6 &#96;@playwright/test&#96;
- LANGUAGE_FRONTIER: typescript:e2e-runtime-dependency:tools/web-flows/test/fixture/flows.spec.ts:1 &#96;@playwright/test&#96;
- UNOWNED_DIRTY_PATH: Makefile
- NO_SELECTABLE_TEST: shell:script/lane-host-heavy.sh

### 6c33df33bc5670131482f5263b410cb980580657

- LANGUAGE_FRONTIER: typescript:e2e-runtime-dependency:conformance/interactive-alpha/fixture/e2e.spec.js:1 &#96;@playwright/test&#96;
- LANGUAGE_FRONTIER: typescript:e2e-runtime-dependency:internal/tasks/obligation/obligation-qualification.spec.ts:6 &#96;@playwright/test&#96;
- LANGUAGE_FRONTIER: typescript:e2e-runtime-dependency:tools/web-flows/test/fixture/flows.spec.ts:1 &#96;@playwright/test&#96;
- UNOWNED_DIRTY_PATH: Makefile

### ea770fa17992b970aeda570ed74686a16dbc441e

- LANGUAGE_FRONTIER: typescript:e2e-runtime-dependency:conformance/interactive-alpha/fixture/e2e.spec.js:1 &#96;@playwright/test&#96;
- LANGUAGE_FRONTIER: typescript:e2e-runtime-dependency:internal/tasks/obligation/obligation-qualification.spec.ts:6 &#96;@playwright/test&#96;
- LANGUAGE_FRONTIER: typescript:e2e-runtime-dependency:tools/web-flows/test/fixture/flows.spec.ts:1 &#96;@playwright/test&#96;

### 24d8c04f9f5ffaaed86759951e520cb775dc2eff

- LANGUAGE_FRONTIER: cfamily:link-graph-unmodelled
- LANGUAGE_FRONTIER: cfamily:unresolved-include
- LANGUAGE_FRONTIER: dotnet:dynamic-test-discovery
- LANGUAGE_FRONTIER: dotnet:source-project-unresolved
- LANGUAGE_FRONTIER: dotnet:test-runner-filter-unresolved
- LANGUAGE_FRONTIER: ruby:implicit-test-dependency:conformance/ruby-profile-v0/rails-app/spec/requests/users_spec.rb
- LANGUAGE_FRONTIER: swift:conditional-compilation
- LANGUAGE_FRONTIER: swift:exclusion-evidence-unrepresentable
- LANGUAGE_FRONTIER: swift:unmapped-source
- LANGUAGE_FRONTIER: typescript:e2e-runtime-dependency:conformance/interactive-alpha/fixture/e2e.spec.js:1 &#96;@playwright/test&#96;
- LANGUAGE_FRONTIER: typescript:e2e-runtime-dependency:internal/tasks/obligation/obligation-qualification.spec.ts:6 &#96;@playwright/test&#96;
- LANGUAGE_FRONTIER: typescript:e2e-runtime-dependency:tools/web-flows/test/fixture/flows.spec.ts:1 &#96;@playwright/test&#96;
- UNOWNED_DIRTY_PATH: .github/workflows/ci.yml

### f13f52b1607222033b089144a744fedae0d35239

- LANGUAGE_FRONTIER: swift:conditional-compilation
- LANGUAGE_FRONTIER: typescript:e2e-runtime-dependency:conformance/interactive-alpha/fixture/e2e.spec.js:1 &#96;@playwright/test&#96;
- LANGUAGE_FRONTIER: typescript:e2e-runtime-dependency:internal/tasks/obligation/obligation-qualification.spec.ts:6 &#96;@playwright/test&#96;
- LANGUAGE_FRONTIER: typescript:e2e-runtime-dependency:tools/web-flows/test/fixture/flows.spec.ts:1 &#96;@playwright/test&#96;
- UNOWNED_DIRTY_PATH: .github/workflows/cache-prune.yml
- UNOWNED_DIRTY_PATH: .github/workflows/ci.yml

### 9fdc2f02efc11ce9ed582d8529b63fe0478fcad2

- LANGUAGE_FRONTIER: cfamily:link-graph-unmodelled
- LANGUAGE_FRONTIER: cfamily:unresolved-include
- LANGUAGE_FRONTIER: dotnet:dynamic-test-discovery
- LANGUAGE_FRONTIER: dotnet:source-project-unresolved
- LANGUAGE_FRONTIER: dotnet:test-runner-filter-unresolved
- LANGUAGE_FRONTIER: ruby:implicit-test-dependency:conformance/ruby-profile-v0/rails-app/spec/requests/users_spec.rb
- LANGUAGE_FRONTIER: swift:conditional-compilation
- LANGUAGE_FRONTIER: swift:exclusion-evidence-unrepresentable
- LANGUAGE_FRONTIER: swift:unmapped-source
- LANGUAGE_FRONTIER: typescript:e2e-runtime-dependency:conformance/interactive-alpha/fixture/e2e.spec.js:1 &#96;@playwright/test&#96;
- LANGUAGE_FRONTIER: typescript:e2e-runtime-dependency:internal/tasks/obligation/obligation-qualification.spec.ts:6 &#96;@playwright/test&#96;
- LANGUAGE_FRONTIER: typescript:e2e-runtime-dependency:tools/web-flows/test/fixture/flows.spec.ts:1 &#96;@playwright/test&#96;
- UNOWNED_DIRTY_PATH: .github/workflows/release-gates.yml

## Current ownership and safety limits

A clean standalone clone at the lane base was audited with the installed `corvint`:
`corvint --root /home/agent/lanes/r1docs-evidence/audit-snapshot affected --ownership-audit`.
It returned `ok: true`, `tracked: 10545`, `unowned: []`, `mutates: false`, pinned to
`df315e35056d15c7933b5ff97c605cedf5ef4919`. The earlier dirty-worktree invocation
refused `unsupported-affected-mixed-worktree`; that refusal is not an audit pass.

The existing tests `TestOwnershipAuditRatchet_AZU_V0_002` and
`TestRepositoryOwnershipComplete_AZU_V0_8900` keep the empty audit and explicitly
introduce `azu-unclaimed-new-file.dat` to require its detection. No ownership rule,
declaration, baseline, test assertion or frozen output changes in this lane.
Historical unsafe caller drops and the conservative repair remain documented in
the 2026-11-05 entry. The full historical outcome/mutation receipts needed for
aggregate AZU-V0-009 acceptance remain external and NOT_PRODUCED.

## Evidence bindings

Raw plans and report remain under `/home/agent/lanes/r1docs-evidence/ownership-replay/`.
The table and complete removed-entry list above are the retained reviewable replay
record; the large raw plans are not committed. SHA-256 bindings:

- `ownership-replay/report.json`: `a8a1db9f9e1b26b3f9a63f23637fa35081595c8d00c5476719383493f9d189e3`.
- `ownership-replay.log`: `6a9d948bfab4d0ae23041f87bdad237dd1575bbc68e53e11f04b57546b708369`.
- `corvint-azu-base`: `90ce2940aaf7b6db53856c31d31db84a95cf05fe96fc4e52c967596b311e6f24`.
- `corvint-azu-candidate`: `5e77baf05a3d8678b3d45996c3567e0bde7e7a089b1f2c3d797a024b7477b5c6`.
- `ownership-audit-clean.json`: `32b2d92e4be1abec24e1aa238d08a50267602846787314f693415e63801aea5c`.

Frozen list SHA-256: `23e52a095a97042e9480d7e0c5b537a8e3507771d3b353bff45125d83abc5264`.

## Whole-package verification

The affected plan selected `internal/liveverify/affected/liverepo`; both invocations
ran the whole existing package through `script/heavy.sh`, with `-count=1 -timeout 30m`
and an explicit whole-package reason preserving the plan's UNKNOWN state.

- Working lane: **FAIL**, 292.266 s. The E2E intent test rejected `HEAD` as not a
  commit, the 295-dirty-path selection exceeded its unchanged budget, and a
  read-only check saw concurrent documentation edits (295 to 298 dirty paths).
- Clean standalone clone at the unchanged lane base: **FAIL**, 231.531 s, solely
  `TestRepositoryE2EFlowsAreBoundAndReviewed_AFUV11655` at e2eflows_repo_test.go:43:
  `intents at HEAD: evaluated revision is not a commit`. The ownership tests and
  other package tests emitted no failures. The clean run removes the concurrent
  edit and dirty-worktree budget confounders, but is not a whole-package pass.

The latter failure reproduces on the untouched base, so no unrelated flow-intent
behavior or test assertion is changed to make this documentation lane pass. The
host verifier must resolve that inherited failure. No repository-wide gate ran.

- `ownership-package-final.log` SHA-256: `c10a502da6bae1a58be57619ca6933a7dd5eb49f9380ac8698fc1cee978e5801`.
- `ownership-package-clean.log` SHA-256: `dd649037ea8540ae5fe9bcadf90231fce9e649132783c5eeef49d3c7ebf27e06`.
