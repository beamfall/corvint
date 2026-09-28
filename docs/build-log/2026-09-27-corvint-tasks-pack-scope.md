## 2026-09-27 CAL-V0-022/026: explicit pack scope derivation and lock measurement (S8 remainder)

The owner selected explicit pack opt-in for S8 and conservative default claims in this session.
The default gob snapshot key hashes the executable, so `corvint-tasks` cannot reuse one written
by `corvint`. A two-binary probe observed that miss, then loaded the same tree successfully
through the existing analyzer-schema-keyed pack with `CORVINT_SNAPSHOT_FORMAT=pack`.
This change keeps that format opt-in and experimental; it does not promote the index format.

CAL-V0-022 uses the existing context loader and TaskContext in process. It never builds an index,
changes ranking, invokes another Corvint executable, or writes outside task state. The base tree,
title, body, analyzer schema and returned packet are committed to the derivation digest.
Default format, absent/stale/dirty snapshots, unsupported context, omitted or withheld paths,
missing critical evidence, governance refusals and budget shortages all abstain to
`WHOLE_REPOSITORY`. A successful scope contains bounded, validated, pinned nongovernance paths.

Named claims keep DECLARED, REQUESTED, DERIVED, WHOLE_REPOSITORY precedence. `claim --next` uses
the existing conservative priority plan to select one ticket, derives that ticket only, and
binds the facts to its ID. The model independently selects again and checks the final scope's
collisions. A plan blocked by its conservative resources stays blocked; the deriver does not
skip a higher-priority ticket to improve parallelism.

CAL-V0-026 remains NOT_MET: each lease still inventories, audits and decodes the ticket store,
and there is no verified audit cache. The opt-in `TestCALV0026_LockHoldMeasurement` imports 3,000
synthetic tickets and records 20 claim, renew and release samples, with command duration and
actual successful flock-acquisition-to-release duration. A context-owned observer records no
store state and has no effect on admission. The measurement report retains the failed structural
clause regardless of latency. Host load and measured source revision accompany the run.

Independent review found the general snapshot loader could fall back to an executable-specific
gob after a pack miss. The dedicated pack-only loader now refuses that fallback, with missing
and corrupt pack regressions. The general loaders retain their existing fallback behavior.
CLI tests cover named and next claims, declared/requested precedence, conservative default
and unselective queries, and unchanged index files.

The first measurement at `156d9aab` stopped after claim/renew because the test supplied an
invalid free-text release reason. It produced no complete latency result. The fixture now
uses the existing release constructor, retains partial samples on failure, and has a three-ticket
`-short` smoke mode; only the normal 3,000-ticket mode counts as the planned measurement.

Measurement at source `ce679c0776aff999b3f7167ffebacf1245d1fd38`: 3000 synthetic tickets, 20 samples per verb; setup 191.012150 s. Claim p95 lock hold 7034.601292 ms; renew 7092.070708 ms.
Started 2026-09-28T02:19:56.578000+00:00; finished 2026-09-28T02:28:30.547445+00:00. Host: 12 CPUs.
Load averages (1/5/15 minutes): before `[14.86669921875, 29.21435546875, 35.623046875]`, after `[30.599609375, 23.40625, 28.33984375]`.
The starting one-minute load exceeded the CPU count, so the below-CPU-load qualification
condition is NOT_MET. These timings do not establish the required low-load bound.
The audit-cache/non-growing-work clause remains NOT_MET independently of these samples.

Verification: focused scope/claim/lock regressions, the required documentation checks and one
independent review. Repository-wide `make gate` is NOT_RUN under the owner's scoped-work rule.
Corvint query/affected and CEM/OCM bind this change; retrieval/ranking evaluations and optional
providers are not applicable because the existing ranking implementation is unchanged.

Rollback: revert S8. Claims without declared/requested paths revert to whole-repository scopes;
stored DERIVED attempts retain the existing scope codec and enforcement. Removing the observer
only removes timing evidence. No index format or store schema migration is introduced.

### Raw measurement samples

Durations in milliseconds, from the opt-in test. Each row is one claim, renew and release sequence.

| Sample | Claim lock | Claim command | Renew lock | Renew command | Release lock | Release command |
|---|---:|---:|---:|---:|---:|---:|
| 1 | 4010.764458 | 4017.563250 | 4025.846708 | 4032.374875 | 4218.080916 | 4223.048625 |
| 2 | 4386.229750 | 4392.083959 | 4012.246042 | 4017.967000 | 4094.262500 | 4100.234709 |
| 3 | 4084.855125 | 4091.486000 | 4093.150333 | 4098.829833 | 3755.960084 | 3761.296708 |
| 4 | 3989.447375 | 3995.838916 | 3978.745125 | 3984.336209 | 4031.087167 | 4036.316625 |
| 5 | 4413.057833 | 4421.273083 | 4813.310667 | 4819.510209 | 4627.737417 | 4634.216583 |
| 6 | 4481.376917 | 4488.690125 | 4683.589625 | 4690.161958 | 4708.678166 | 4714.563500 |
| 7 | 4607.947167 | 4613.794417 | 3925.630667 | 3931.646417 | 3121.156042 | 3127.430792 |
| 8 | 3474.077375 | 3479.612583 | 3190.331792 | 3196.089916 | 4620.991500 | 4626.542375 |
| 9 | 6571.754334 | 6577.698250 | 4703.924625 | 4710.138000 | 5884.623875 | 5890.280208 |
| 10 | 5261.754041 | 5267.766166 | 4911.038459 | 4916.855333 | 5170.728875 | 5176.304000 |
| 11 | 7034.601292 | 7042.894667 | 6868.544750 | 6874.033458 | 6547.005750 | 6553.438792 |
| 12 | 6736.103000 | 6743.859417 | 6324.235333 | 6351.307084 | 6843.143083 | 6849.238542 |
| 13 | 7146.504750 | 7153.551542 | 7092.070708 | 7098.577875 | 7268.641250 | 7275.111834 |
| 14 | 6937.818125 | 6945.691333 | 7954.489375 | 7960.812542 | 6490.750875 | 6498.443333 |
| 15 | 5386.454792 | 5392.775334 | 5013.185417 | 5019.065834 | 5529.162875 | 5534.178583 |
| 16 | 5687.119708 | 5693.433333 | 5266.443584 | 5272.663083 | 5563.607333 | 5569.652958 |
| 17 | 5532.045584 | 5538.659000 | 5726.859792 | 5731.570292 | 6388.075334 | 6394.600250 |
| 18 | 5287.475500 | 5296.494291 | 6124.375583 | 6131.236667 | 6385.108041 | 6390.621916 |
| 19 | 5334.070750 | 5340.630500 | 4946.946500 | 4953.828209 | 6816.829792 | 6823.253750 |
| 20 | 6789.032791 | 6795.248834 | 5843.181208 | 5849.148791 | 5602.854459 | 5609.750417 |

Corvint affected selected the tasks authority, CLI, scopes, store and transaction packages,
plus contextindex and its reverse dependents. Scope remains UNKNOWN because of the language
frontier: nested Go modules, dynamic imports and unsupported external language/configuration
paths. Focused checks cover the changed behavior; the omitted repository-wide tests and those
unknowns remain NOT_RUN/unresolved under the owner’s scoped-work instruction.

Retained affected-plan unknowns:

- `LANGUAGE_FRONTIER: dotnet:source-project-unresolved`
- `LANGUAGE_FRONTIER: go:build-constraint-variants`
- `LANGUAGE_FRONTIER: go:nested-module-frontier`
- `LANGUAGE_FRONTIER: kotlin:jvm-sibling-source-present`
- `LANGUAGE_FRONTIER: python:dynamic-import`
- `LANGUAGE_FRONTIER: swift:conditional-compilation`
- `LANGUAGE_FRONTIER: swift:exclusion-evidence-unrepresentable`
- `LANGUAGE_FRONTIER: swift:unmapped-source`
- `LANGUAGE_FRONTIER: typescript:dynamic-import`
- `LANGUAGE_FRONTIER: typescript:e2e-runtime-dependency`
- `LANGUAGE_FRONTIER: typescript:executable-config-unresolved`
- `LANGUAGE_FRONTIER: typescript:path-alias-unresolved`
- `LANGUAGE_FRONTIER: typescript:runtime-flags-unresolved`
- `UNOWNED_DIRTY_PATH: docs/build-log/2026-09-27-corvint-tasks-pack-scope.md`
- `UNOWNED_DIRTY_PATH: docs/specs/INDEX.json`
- `UNOWNED_DIRTY_PATH: docs/specs/README.md`
- `UNOWNED_DIRTY_PATH: docs/specs/REQUIREMENTS.tsv`
- `UNOWNED_DIRTY_PATH: docs/specs/corvint-tasks-agent-leases-v0.md`
