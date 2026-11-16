# 2026-11-16: ownership replay through the lane base (V1-4685)

Requirements: AZU-V0-002, AZU-V0-009, AZU-V0-8900. This entry answers the
independent review's P2 finding on `2026-11-16-r1docs-ownership-replay.md`: that
replay stopped at the eight frozen merges, while AZU-V0-009 names those eight
**plus every subsequent merge**. It replays the same two pinned binaries over all
117 required merge trees through `df315e35056d15c7933b5ff97c605cedf5ef4919`.
V1-4685 criterion 3 stays **incomplete**: 81 of the 117 trees have no comparison.

## Inputs

- Required merges: the 8 lines of `script/affected-zero-unknown-merges.txt`, then
  `git rev-list --first-parent --merges --reverse 8e2c13aa926fcaf1731fae71b27c41c41c8f09af..df315e35056d15c7933b5ff97c605cedf5ef4919`
  (109 merges): 117 distinct merges, each planned against its first parent.
- Binaries (unchanged from the eight-merge entry): `corvint-azu-base`
  SHA-256 `90ce2940aaf7b6db53856c31d31db84a95cf05fe96fc4e52c967596b311e6f24`
  (source `8e2c13aa9`), `corvint-azu-candidate` SHA-256
  `5e77baf05a3d8678b3d45996c3567e0bde7e7a089b1f2c3d797a024b7477b5c6` (source `2eaaf9fba`).
- Planner argv per tree and binary, the one `tools/affected-zero-unknown-replay`
  uses (`invokePlanner`), with its single `--accept-stale-binary` retry on a
  `self-binary-stale` refusal (none occurred):
  `BINARY --root SNAPSHOT affected --no-cache --full --base FIRST_PARENT`, where
  SNAPSHOT is a standalone clone checked out detached at the merge.

The repository harness itself, run as
`script/affected-zero-unknown-replay.sh --corvint corvint-azu-candidate --baseline corvint-azu-base --through df315e35056d15c7933b5ff97c605cedf5ef4919`,
exits 1 (767.0 s) at the first planner refusal: its report holds 36 compared rows
and a 37th, merge `e19823d919cfa653ee0ed05708d03f5fd12ca929` (base `c3b29cbb5`),
with no plan. The resumable
external script `replay-all-v2.py` therefore runs every remaining tree with the
same argv and retains each refusal as a row instead of stopping. The first 36
trees reuse the harness run's raw plans (72 files) when both sides parsed; each
other tree ran fresh. Unknown entries are compared as `(reason, detail)` pairs,
the harness's `unknown` struct; rows 1 to 8 reproduce the eight-merge entry's
table exactly.

## Result

- **36 trees compared** (rows 1 to 36). **Zero baseline-selected units dropped**
  on every compared row. Every compared row adds selections (77 to 235 units).
- **81 trees not compared** (rows 37 to 117, `NOT_PRODUCED`). Both binaries refuse
  every one with exit 2, `unsupported-affected-graph`:
  - rows 37 to 61 (25 trees): `.corvint/runtime-edges.json: runs "python:.github.macos.darwin_select" names no unit`;
  - rows 62 to 117 (56 trees): `.corvint/runtime-readers.json: path ".corvint/documentation-paths.json"`.
  These historical binaries cannot read declarations those later trees added.
  A refusal is not a comparison and not a pass; no row was dropped.
- Unknown growth is retained: on rows 14, 16, 19 to 21 and 26 the candidate
  reports 66 more unknown entries than the baseline (baseline 1 to 3), and row 5
  grows from 63 to 78. Removed entries come with wider selection, not with
  permission to drop a reader, executor, importer or embedder.
- `accepted` stays false: historical full-suite and mutation receipts are not
  available (as in the eight-merge entry), and 81 trees have no comparison.

What remains for criterion 3: a before/after pair able to plan rows 37 to 117
(binaries that read those trees' declarations, or an owner ruling that the
refused trees are outside the ownership rule's replay), plus the safety receipts.

Follow-up: `2026-11-16-r1replay-ownership-replay-complete.md` supplies that pair. It is built
from the lane base, and its "before" switches off only the ownership rule. It compares all 117
rows, with zero dropped units. The safety receipts remain AZU-V0-009's own promotion condition,
not V1-4685's criterion 3.

## Compared rows

Unknown counts are plan `unknown` entries; selected counts are plan `selected`
unit IDs. "Removed" and "Added unknown" are the `(reason, detail)` set differences.

| # | Exact merge | Unknown before / after | Removed | Added unknown | Selected before / after | Added | Dropped |
|---:|---|---:|---:|---:|---:|---:|---:|
| 1 | `8e2c13aa926fcaf1731fae71b27c41c41c8f09af` | 92 / 87 | 7 | 2 | 266 / 490 | 224 | 0 |
| 2 | `39c03198086681fb24f509fbd00b34696d3268c4` | 84 / 73 | 13 | 2 | 66 / 181 | 115 | 0 |
| 3 | `d436dff76de1b8e75c427069ae7f80003ae99bc2` | 99 / 95 | 5 | 1 | 233 / 442 | 209 | 0 |
| 4 | `6c33df33bc5670131482f5263b410cb980580657` | 83 / 80 | 4 | 1 | 183 / 388 | 205 | 0 |
| 5 | `ea770fa17992b970aeda570ed74686a16dbc441e` | 63 / 78 | 3 | 18 | 75 / 181 | 106 | 0 |
| 6 | `24d8c04f9f5ffaaed86759951e520cb775dc2eff` | 79 / 68 | 13 | 2 | 38 / 118 | 80 | 0 |
| 7 | `f13f52b1607222033b089144a744fedae0d35239` | 86 / 82 | 6 | 2 | 46 / 131 | 85 | 0 |
| 8 | `9fdc2f02efc11ce9ed582d8529b63fe0478fcad2` | 79 / 69 | 13 | 3 | 36 / 113 | 77 | 0 |
| 9 | `cdeed7ee6396a8d6bf6225eb5b2b4cb4d601b419` | 208 / 206 | 4 | 2 | 267 / 485 | 218 | 0 |
| 10 | `28a3ff86a2c3d17662765da12cc9251c06466f5f` | 91 / 89 | 4 | 2 | 248 / 465 | 217 | 0 |
| 11 | `c5379b4cf039fa8dd2df2aa4ee6e5b7b897cf514` | 80 / 69 | 13 | 2 | 81 / 193 | 112 | 0 |
| 12 | `9db7a94169821d9b28701eaa88866aee2fec1083` | 79 / 68 | 13 | 2 | 43 / 133 | 90 | 0 |
| 13 | `9208cc7e25bb82503ef9d61b0ce6a5f02039e649` | 82 / 80 | 3 | 1 | 188 / 411 | 223 | 0 |
| 14 | `dba1f4d6c9a33098470ccf57794fb66f989f03c8` | 1 / 67 | 0 | 66 | 89 / 306 | 217 | 0 |
| 15 | `f50a0e3447a2bbadbfadbe7f8cc1019ae1636ac5` | 89 / 86 | 6 | 3 | 80 / 315 | 235 | 0 |
| 16 | `36e2da4e800ff1f35c3931c50f01ea020cbec1db` | 2 / 68 | 0 | 66 | 73 / 174 | 101 | 0 |
| 17 | `8fc0cf37642e7012f4c26671c9ed97a8706d6e09` | 92 / 87 | 7 | 2 | 199 / 336 | 137 | 0 |
| 18 | `d886ec70adaf7e4aa7494b72212a8288d8e5cc8a` | 80 / 69 | 12 | 1 | 94 / 313 | 219 | 0 |
| 19 | `9d0c56665c039876b78eb71b877122f86c4ccd25` | 2 / 68 | 0 | 66 | 170 / 271 | 101 | 0 |
| 20 | `7fd70eab2f6475074dd1c85c62a43cb5a6a7ff0e` | 2 / 68 | 0 | 66 | 114 / 232 | 118 | 0 |
| 21 | `654df5053cc3a5f36aa4ecb48af36c5eaea0a86c` | 3 / 69 | 0 | 66 | 113 / 230 | 117 | 0 |
| 22 | `044058f1bde98a27be3819c5aa9193951b241d24` | 85 / 72 | 16 | 3 | 83 / 209 | 126 | 0 |
| 23 | `093750bb2247c49237f3cf8ba69485224791f482` | 71 / 69 | 3 | 1 | 127 / 244 | 117 | 0 |
| 24 | `670559b9ec3ffbcd59aa6c6d02029ec0cb206a93` | 116 / 114 | 3 | 1 | 147 / 283 | 136 | 0 |
| 25 | `13342d85f6245f897c58023891824212617fe0a7` | 88 / 85 | 4 | 1 | 205 / 339 | 134 | 0 |
| 26 | `000d4c56c725f0f963b84bf503196f897239dddb` | 2 / 68 | 0 | 66 | 84 / 203 | 119 | 0 |
| 27 | `539b55331ab2c10a35963e324428b30746fb4b85` | 79 / 77 | 3 | 1 | 92 / 208 | 116 | 0 |
| 28 | `d74f54b2783b77d6e46d204b7c7164430dc3cf4e` | 83 / 71 | 13 | 1 | 60 / 181 | 121 | 0 |
| 29 | `1a401eb5ee68ff4a0541534b33a5d6538793e9ea` | 80 / 76 | 5 | 1 | 82 / 205 | 123 | 0 |
| 30 | `b75053a5b23cb0a4737a593d1fc1912aa3b79436` | 86 / 83 | 3 | 0 | 198 / 323 | 125 | 0 |
| 31 | `4052b8540b5fff818a819db37b55371ea5428c2c` | 79 / 76 | 3 | 0 | 83 / 203 | 120 | 0 |
| 32 | `538e823f4a1d220a3fb5b12c0529248f18924823` | 84 / 72 | 12 | 0 | 181 / 401 | 220 | 0 |
| 33 | `59b31f12ed984802627f3ef44fa918f3022045cd` | 85 / 79 | 7 | 1 | 216 / 345 | 129 | 0 |
| 34 | `fe73dc77d169b79bf0d1b2791bda50cb9ca077fe` | 81 / 77 | 4 | 0 | 106 / 200 | 94 | 0 |
| 35 | `cb403735903b1e54a8469373489960c0b2b89d99` | 90 / 85 | 6 | 1 | 108 / 244 | 136 | 0 |
| 36 | `c3b29cbb54d31c8526d3cce4ad74356b6fb9b943` | 107 / 83 | 24 | 0 | 234 / 467 | 233 | 0 |

## Every removed unknown entry

Each distinct `(reason, detail)` removed on some compared row, with the rows
(numbered as above) where it was removed. Backticks inside details are escaped.

- LANGUAGE_FRONTIER: swift:conditional-compilation (rows 1-2,6-8,11-12,15,22,28-29,33,35)
- LANGUAGE_FRONTIER: typescript:e2e-runtime-dependency:conformance/interactive-alpha/fixture/e2e.spec.js:1 &#96;@playwright/test&#96; (rows 1-13,15,17-18,22-25,27-36)
- LANGUAGE_FRONTIER: typescript:e2e-runtime-dependency:internal/tasks/obligation/obligation-qualification.spec.ts:6 &#96;@playwright/test&#96; (rows 1-13,15,17-18,22-25,27-36)
- LANGUAGE_FRONTIER: typescript:e2e-runtime-dependency:tools/web-flows/test/fixture/flows.spec.ts:1 &#96;@playwright/test&#96; (rows 1-13,15,17-18,22-25,27-36)
- UNOWNED_DIRTY_PATH: .github/workflows/ci-route-health.yml (rows 1)
- UNOWNED_DIRTY_PATH: .github/workflows/ci.yml (rows 1-2,6-7,11-12,15,22,33,35)
- UNOWNED_DIRTY_PATH: Makefile (rows 1,3-4,9-10,18,22,25,32-33,36)
- LANGUAGE_FRONTIER: cfamily:link-graph-unmodelled (rows 2,6,8,11-12,18,22,28,32)
- LANGUAGE_FRONTIER: cfamily:unresolved-include (rows 2,6,8,11-12,18,22,28,32)
- LANGUAGE_FRONTIER: dotnet:dynamic-test-discovery (rows 2,6,8,11-12,18,22,28,32)
- LANGUAGE_FRONTIER: dotnet:source-project-unresolved (rows 2,6,8,11-12,18,22,28,32)
- LANGUAGE_FRONTIER: dotnet:test-runner-filter-unresolved (rows 2,6,8,11-12,18,22,28,32)
- LANGUAGE_FRONTIER: ruby:implicit-test-dependency:conformance/ruby-profile-v0/rails-app/spec/requests/users_spec.rb (rows 2,6,8,11-12,18,22,28,32)
- LANGUAGE_FRONTIER: swift:exclusion-evidence-unrepresentable (rows 2,6,8,11-12,18,22,28,32)
- LANGUAGE_FRONTIER: swift:unmapped-source (rows 2,6,8,11-12,18,22,28,32)
- NO_SELECTABLE_TEST: shell:script/lane-host-heavy.sh (rows 3)
- UNOWNED_DIRTY_PATH: .github/workflows/cache-prune.yml (rows 7)
- UNOWNED_DIRTY_PATH: .github/workflows/release-gates.yml (rows 8,22)
- UNOWNED_DIRTY_PATH: .github/workflows/go-macos.yml (rows 15,22,28,33)
- UNOWNED_DIRTY_PATH: .agent-evidence/pk-affected-fix-round-1.json (rows 17)
- UNOWNED_DIRTY_PATH: .agent-evidence/pk-affected-verification.json (rows 17)
- UNOWNED_DIRTY_PATH: tools/str-parity/testdata/local-default.txt (rows 17)
- UNOWNED_DIRTY_PATH: tools/str-parity/testdata/local-full.json (rows 17)
- UNOWNED_DIRTY_PATH: .github/workflows/greptile-gate.yml (rows 29)
- UNOWNED_DIRTY_PATH: .corvint/.gitignore (rows 34)
- UNOWNED_DIRTY_PATH: .github/workflows/linecov-index.yml (rows 35)
- UNOWNED_DIRTY_PATH: .agent-evidence/cx-affected-ci-failures.json (rows 36)
- UNOWNED_DIRTY_PATH: .agent-evidence/cx-affected-selection-replay.json (rows 36)
- UNOWNED_DIRTY_PATH: .agent-evidence/cx-affected-verification.json (rows 36)
- UNOWNED_DIRTY_PATH: .agent-evidence/lanetools/collisions-test.log (rows 36)
- UNOWNED_DIRTY_PATH: .agent-evidence/lanetools/completion-checks.json (rows 36)
- UNOWNED_DIRTY_PATH: .agent-evidence/lanetools/fanout-completion.log (rows 36)
- UNOWNED_DIRTY_PATH: .agent-evidence/lanetools/fanout-fault.json (rows 36)
- UNOWNED_DIRTY_PATH: .agent-evidence/lanetools/fanout-fault.log (rows 36)
- UNOWNED_DIRTY_PATH: .agent-evidence/lanetools/fanout-final.log (rows 36)
- UNOWNED_DIRTY_PATH: .agent-evidence/lanetools/fanout-race.log (rows 36)
- UNOWNED_DIRTY_PATH: .agent-evidence/lanetools/fixtures-final.json (rows 36)
- UNOWNED_DIRTY_PATH: .agent-evidence/lanetools/go-measurements.json (rows 36)
- UNOWNED_DIRTY_PATH: .agent-evidence/lanetools/lane-scripts-test.log (rows 36)
- UNOWNED_DIRTY_PATH: .agent-evidence/lanetools/line-citations-final-test.log (rows 36)
- UNOWNED_DIRTY_PATH: .agent-evidence/lanetools/measure.txt (rows 36)
- UNOWNED_DIRTY_PATH: .agent-evidence/lanetools/measurements-final.json (rows 36)
- UNOWNED_DIRTY_PATH: .agent-evidence/lanetools/pi-namespace-fault.log (rows 36)
- UNOWNED_DIRTY_PATH: .agent-evidence/lanetools/pi-scratch-test.log (rows 36)
- UNOWNED_DIRTY_PATH: .agent-evidence/lanetools/prechange-final-test.log (rows 36)
- UNOWNED_DIRTY_PATH: .agent-evidence/lanetools/reservations-test.log (rows 36)

## Evidence bindings

Raw plans, refusals and the folded report remain under
`/home/agent/lanes/r1docs-evidence/` (not committed). SHA-256:

- `ownership-replay-all/report.json`: `028d9e446a0857778aca14ad3beb89c13657e53420201cd1534306393c2d8a24`.
- `replay-all-v2.py`: `e5b4907baa3440a53f448ebe1ba0161ed645a2b8565dea97a423358b061b80d1`.
- `replay-all-v2.log`: `a33c562fb1e1d5967621e18dbabf91a80bc47bfe83989162bd4b124f7e05cdb1`.
- `ownership-replay-review/report.json` (harness run, 37 rows): `737beeb80649221ceeaaf65b5abd7c74867aa468de87440db8deae9797e14916`.
- `ownership-replay-review.log` (harness run): `4bca229826a563f46c727154810b787ba58dba844d860c751f3b1d79ae711e91`.
