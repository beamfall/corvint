# Source removal B1: dead leaves removed; reference proof blocked

Date: 2026-11-18
Ticket: V1-5694
Batch parent: `432f0112f260cf661610a2c90ce09841d1887fe2`
Selection base: `55decc6a041a1df3f8ae43e95ba21cea669e7d80`
Authority: source-removal G14 acceptance, owner 2026-11-18, PR #235; round-2 lane decision.
Disposition: B1 source cut implemented; full RMV-V1-003 proof is PARTIAL.
Task 4 stays unchecked. B2 and B3 have not started because the B1 proof is incomplete.

## Decisions and retained paths

The round-2 decision retains `tools/affected-edit-oracle` and its corpus for FIH-V1-011.
`path-verdicts.md` now records: "FIH-V1-011 parity oracle; cut needs a coordinated FIH change".
`TestGoParityWithOracle_FIH_V1_011` and `runParityCommand` remain unchanged.

Also retain `script/spec-toolchain-conformance.sh`. Its OCM comparison is already gone;
STA-V0-011 still requires its upstream validators and integration packaging. Its callers are
`make spec-toolchain-conformance` and the workflow B0 retained. The script header proves this
is not a dangling OCM script. Its new keep row records this failed premise, as round 2 permits.
No source, contract or assertion in that path changed. `perfbudget` stays unchanged (RMV-V1-029).

## Importer proof and test ordering

RMV-V1-004: before deletion, `git grep` at the batch parent searched each candidate's full Go
import path across tracked Go files. There were no kept Go importers, so RMV-V1-005 required
no kept-test rewrite. The only importers were inside the removal set:

- `internal/workqueue`: eight files in `conformance/work-queue-v0`: `cli_fixture_test.go`,
  `identity_preimages_test.go`, `local_evidence_test.go`, `optimizer_budget_test.go`,
  `proposal_witnesses_test.go`, `schema_registry_test.go`, `tuple_invalidation_test.go` and
  `work_queue_v0_test.go`.
- `conformance/work-queue-v0`: the four `main.go` adapters in its `cmd/cli-adapter`,
  `cmd/cli-prose`, `cmd/cli-replay` and `cmd/cli-scope` directories.
- All other removed Go package roots: no importers.

The non-Go reference search was separate and retained the script dependency above.
No kept test assertion, golden, public command or wire format was changed.

## Removal and orphan table

RMV-V1-007 and RMV-V1-031: 86 tracked files are removed. The surviving imports of deleted
Go files reach `internal/cem/wire`, `internal/contextindex`,
`internal/liveverify/affected/golang/typedreach` and `internal/procgroup`; each still has kept
importers. There is no newly orphaned Go dependency to remove.

| Removed path | Disposition and orphan handling |
| --- | --- |
| `internal/cliconformance` | Empty package marker; no importer |
| `conformance/work-queue-v0` | Retired Tasks conformance plus its four CLI adapters |
| `internal/workqueue` | Orphan once that conformance leaves; removed in this batch |
| `docs/worklist.json`, `.corvint/work-queue-policy.json` | Retired queue data; declaration classifier and runtime-reader registration removed |
| `protocol/step`, `protocol/delta` | Schemas of already retired capabilities; no code reader |
| `benchmarks/selfuse-batch` | Retired self-use harness; selection and read-scope entries removed |
| `tools/ci-reuse-replay` | Retired measurement; selection entry removed |
| `tools/fixture-maintenance-check` | No code caller; read-scope and tree-reader entries removed |
| `tools/gotest-evidence` | No code caller; GTE-V0 spec archived in place |
| `tools/pi-tui-fixture` | No Pi integration caller; unbounded-reader entries removed |
| `tools/plan-delta` | No code caller; literal-reader entry removed |
| `tools/typed-reach-measure` | One-off measurement; direct-Git allowlist entry removed |
| `script/citeperf-measure.py` | One-off measurement, no executable caller |
| `script/qualify-checkpoint-handoff.py` | Experimental pilot, no executable caller |
| `benchmarks` import-ratchet prefix | Last Go package removed; stale registration removed |

The design's `breakagemap`, `unplannedread`, `scopelease`, `worklistadapter`, `worksource`,
`tracerecordrepo`, `trace`, `tracerepopaths` and `docviews` internal packages, and
`script/runtime-environment_test.sh`, were already absent. No second deletion is claimed.
The ratchet's `internal/cliconformance` and `internal/workqueue` prefixes are removed; its
baseline has no corresponding edge and needs no edit.

GTE-V0-820 through GTE-V0-826 remain retired history rows. AFP-V0-2333, AFP-V0-2346,
AFP-V0-2378 and TSP-V0-666 gain retirement status lines for the removed measurement tools;
their original clauses and evidence rows remain. `make spec-requirements` regenerates the index.

## Verification

All commands set `TMPDIR=/home/russell/lanes/sr-b13.tmp`. Go tests run through `script/heavy.sh`,
one whole package per command, without exported commit identity or semaphore overrides.

- `corvint affected --base 55decc6a041a1df3f8ae43e95ba21cea669e7d80`: exit 0, scope UNKNOWN;
  99 dirty paths, 337 selected units, 352 excluded, 80 unknowns and 16 unresolved packages.
  Unknowns: LANGUAGE_FRONTIER 58, UNINDEXED_SOURCE_PATH 5, UNOWNED_DIRTY_PATH 17.
  This broad fallback was retained, not executed. The focused checks below cover touched
  surviving packages and the explicitly required ratchet.
- `script/heavy.sh go test -count=1 -timeout 30m ./tools/declaration-compat-check`: PASS, 0.258s.
- `script/heavy.sh go test -count=1 -timeout 30m ./tools/import-ratchet`: PASS, 0.845s.
- `script/heavy.sh go vet ./tools/import-ratchet ./tools/declaration-compat-check`: PASS.
- `make windows-vet`: PASS, root and interop modules, Go 1.27.4.
- `make changed-checks BASE=55decc6a041a1df3f8ae43e95ba21cea669e7d80`: PASS; citation,
  unbounded-reader and test-read-scope checks run.
- `script/check-line-citations.sh --repin`: PASS, no moved anchor; 184 existing basename-only
  citations skipped by the checker.
- `make -s spec-requirements-check line-citations-check openspec-check long-lines-check`: PASS.
  The spec checker also ran the whole `internal/specindex` package: PASS, 0.905s.
  OpenSpec retains 65 baselined findings and zero failures.

Earlier failures were repaired without weakening tests: deleting the only test owner left an
empty `benchmarks/**` selection entry, which was removed; the ratchet found the stale
`benchmarks` prefix; spec generation refused a requirement ID in a new retirement header,
which now names only the source-removal change.

## Remaining proof and ordering blocker

RMV-V1-003 item 4 does not pass under its exact exclusions. Historical path names remain in
immutable or out-of-scope evidence, including:

- `.taskman/tickets/V1-0404.json`, `V1-0506.json` and `V1-0473.json`; the charter forbids editing
  the task store. These alone prevent the literal zero-reference proof.
- The retained oracle corpus pins historical `baseSha` values and `mustSelect` expectations.
  Removing those expectations would weaken the retained oracle. The source cut does not
  change what those historical commits contained.
- CI outcome/selection fixtures carry historical tree and manifest digests; they and benchmark
  runs are retained without relabelling or rewriting the observations.
- `.corvint/requirement-links.json` is pinned derived history, not a live package registration.
- `.github` inventories and qualification evidence are outside this lane's write scope.

Additional prose and historical index references are included in the full per-path audit.
The handback retains that audit and the exact extra `.github/linecov/qualification.json` patch
for owner consideration; it does not pretend to have regenerated that pinned qualification.
The gate needs an explicit historical-reference disposition, coordinated with the retained
oracle and the task-store prohibition. This lane does not invent a new exclusion.

Whole-root Linux vet, recursive make dry runs, exhaustive gates and hosted CI are NOT_RUN
under the focused-only charter. Merge-head proof is NOT_PRODUCED: publication and merging are
not authorized in this lane. Independent review belongs to the orchestrator. Dogfood, CEM and
seal are retired under the charter override and are NOT_RUN.

RMV-V1-001: stop before B2 because B1's complete proof is not available; RMV-V1-008 through
RMV-V1-011 remain undelivered. RMV-V1-030: revert this batch commit to restore all removed
source and registrations. Recovery bytes are in the batch parent named above.

## Retained evidence digests

Raw evidence is outside the repository in `/home/russell/lanes/sr-b13-evidence/`.

| File | Bytes | SHA-256 |
| --- | --- | --- |
| `b1-importers.txt` | 1784 | `7f0102d589954a9ca3f9f904a023067dba7d0ad4c40749f6e123e3c2c4be4aa2` |
| `b1-zero-reference-audit.json` | 5633 | `72933a304c764fea67f12c6bf891d137a482b5c8ffdce7bc9761805c5ba8c7a1` |
| `b1-affected.json` | 285799 | `0dfe8375cec00fb5c727f501d6234283c2ff4071973a197905414aaead49cae8` |
| `b1-orphan-audit.txt` | 10941 | `7b322d4c3bc2cbbc1e59ffab9034f450c284e7912cb10e6eff2d68b2ccf7c0e9` |

## sr-b23: authorized history exclusions and active reference repair

Authority: owner-delegated 1.0 decision in the sr-b23 charter, 2026-11-18; V1-5694.
Input head: `8b94fc5e54b5c877249778e744caafd954bc0faf`.
Selection base: `83d95f840c4506c43d0314b339686ba7273224d6`.
This section supersedes the earlier statement that the task store and retained oracle are blockers.

RMV-V1-003 item 4 now excludes the append-only task store, the retained FIH oracle corpus, and
frozen CI outcome/selection fixtures bound by a digest. Both the capability spec and change delta
carry the same amendment. No new requirement ID or broader historical exclusion was added.
The remaining hits in the newly excluded files are enumerated below, with the existing history
exclusions as well. The externally retained JSON audit maps each file to every removed path it hits.

RMV-V1-007 and RMV-V1-031: removed stale implementation entries from the spec index, stale rows
from the derived requirement path map, the retired tool's vocabulary root, and the dead conformance
citation-list entry. Active prose and code comments now describe surviving behavior or identify
retirement. The pilot document retains its historical protocol but no longer offers missing scripts
as current runnable commands. Test assertions, wire shapes, errors, goldens and frozen fixtures are
unchanged. Only comments changed in Go and TypeScript source. The old import and orphan proof
above still applies: this follow-up removes no additional source or dependency.

### Item 1 to 5 proof disposition

1. Root and interop module `GOTOOLCHAIN=local go vet ./...`: PASS, through `script/heavy.sh`.
2. `make windows-vet`: PASS, root and interop modules; Go 1.27.4.
3. Recursive `make -n gate doc-gates shell-test release-binaries-check`: NOT_RUN. The charter
   prohibits doc-gates and whole-repository selection. Make runs recursive recipe lines even
   with `-n`; the ledger and timed doc recipes invoke programs before their recursive make.
   No equivalent proof is claimed. The host must run this check at the integration boundary.
4. Literal removed-path audit: FAIL, with 12 remaining files outside the exact exclusions.
   Four are under `.github/`, which this lane may not edit. Eight are frozen observations or
   corpus/admission records, including the untouched-repository evidence RMV-V1-036 retains.
   The charter names only the FIH corpus and digest-bound CI fixtures as new exclusions;
   it also prohibits changing frozen outputs. A request to exclude the retained benchmark and
   conformance records is pending; no approval or wider exclusion is inferred from silence.
5. Existing whole `tools/import-ratchet` package: PASS, 0.995s. Registrations remain consistent;
   this follow-up removes no new package and introduces no new baseline edge.

The existing whole `internal/contextindex` package also passed (33.647s). All Go tests used the
heavy wrapper, one package per command, with no filter, identity export or lock override.
Affected selected contextindex directly, the import ratchet by enclosing package, and specindex
as a literal reader. Its plan is UNKNOWN: 399 selected, 293 excluded, 79 unknowns and 9 unresolved
packages. Unknown reasons: LANGUAGE_FRONTIER 58, UNINDEXED_SOURCE_PATH 5, UNOWNED_DIRTY_PATH 14,
UNSELECTED_BINARY_CONSUMER 2. The broad fallback was not executed.

B1 stays PARTIAL; task 4 is not ticked. RMV-V1-001 requires the completed proof before B2;
therefore RMV-V1-008/009 and RMV-V1-010/011 remain NOT_STARTED. No B2/B3 deletion, provider parity,
reader migration or runner-protocol claim is made. The orchestrator owns independent review.
No publication, native task write, CEM, dogfood or seal was attempted. Rollback is a revert of
this follow-up; the initial B1 source cut has its separate recovery parent above.

### Remaining non-excluded files

- `.github/cishards/package-costs.json`
- `.github/cishards/testdata/run42408095814.json`
- `.github/cishards/walltime_test.go`
- `.github/linecov/qualification.json`
- `benchmarks/daily-loop-v0/corpus.json`
- `benchmarks/daily-loop-v0/runs/run-001.json`
- `benchmarks/daily-loop-v0/runs/run-002.json`
- `benchmarks/taskcontext-lookup/default-summary.json`
- `benchmarks/taskcontext-lookup/pack-summary.json`
- `benchmarks/untouched-repository-v1/corvint/runs/run-002.json`
- `benchmarks/untouched-repository-v1/viper/admission.json`
- `conformance/use-cases-v0/receipts/UC-TASK-ORIENTATION/corvint-dogfood/prechange-query.json`

### Excluded hit inventory

- `.agent-evidence/ctx-w58/verification/affected-final.json`
- `.agent-evidence/ctx-w58/verification/affected.json`
- `.agent-evidence/cx-affected-selection-replay.json`
- `.agent-evidence/lineidx-qualification-blocked.json`
- `.agent-evidence/native-taskman/affected-dirty.json`
- `.agent-evidence/native-taskman/gate-a-implementation.md`
- `.agent-evidence/pk-tstore-fix1/affected-final.json`
- `.agent-evidence/pk-tstore-fix1/affected-narrow-final.json`
- `.agent-evidence/pk-tstore-fix1/base-binding.json`
- `.agent-evidence/pk-tstore/affected-delivered.json`
- `.agent-evidence/pk-tstore/affected-start.json`
- `.corvint/changes/2903292b5abe8528e2d3c1fb6bb994391daffbf5.cem.json`
- `.corvint/changes/405f18aebbcc9a1ca81353dfad6a2335b08abda4.cem.json`
- `.corvint/changes/41f821ff6867a933e6205cf71672fcf5a6398ea8.cem.json`
- `.corvint/changes/4be7c81296b780e9195424e0975856db2dfcba94.cem.json`
- `.corvint/changes/4cb33a048d853b6c2e34eaf2a2493dffac3d1420.cem.json`
- `.corvint/changes/7b896f87cc6bcd5e054532936936be1c5148e383.cem.json`
- `.corvint/changes/a14de43f6af0506d5ce617a54ddad8e4d1d681d9.cem.json`
- `.corvint/changes/b0f69f0c1430925fe1df9f2585e4452ad25123e3.cem.json`
- `.corvint/changes/b9f67a6471db95f13bb502bf7fc80ed3785f6075.cem.json`
- `.taskman/tickets/V1-0404.json`
- `.taskman/tickets/V1-0473.json`
- `.taskman/tickets/V1-0506.json`
- `benchmarks/results/v2892-selection-baseline-2026-10-31-base.json`
- `benchmarks/results/v2892-selection-baseline-2026-10-31-lane.json`
- `benchmarks/results/v2892-selection-baseline-2026-10-31-merge.json`
- `benchmarks/results/v2892-selection-baseline-2026-10-31-pre14.json`
- `benchmarks/results/v2892-selection-baseline-2026-11-01-58.json`
- `benchmarks/results/v3348-source-and-test-slices-2026-10-31.json`
- `docs/build-log/2026-10-31-lsp-cem-w2.md`
- `docs/build-log/2026-10-31-scripts-w2.md`
- `docs/build-log/2026-11-01-afr-w31.md`
- `docs/build-log/2026-11-01-afs-w27.md`
- `docs/build-log/2026-11-01-ci-w34.md`
- `docs/build-log/2026-11-01-scr-w16.md`
- `docs/build-log/2026-11-03-floor-typed-reach.md`
- `docs/build-log/2026-11-03-reuse-verified-results.md`
- `docs/build-log/2026-11-04-floor-review-c.md`
- `docs/build-log/2026-11-04-floor-review-e.md`
- `docs/build-log/2026-11-04-reuse-review-round-3-confinement-closures.md`
- `docs/build-log/2026-11-04-reuse-review-round-soundness.md`
- `docs/build-log/2026-11-06-derived-cishards-membership.md`
- `docs/build-log/2026-11-06-mbrescue.md`
- `docs/build-log/2026-11-06-selgo.md`
- `docs/build-log/2026-11-06-unbounded-readers.md`
- `docs/build-log/2026-11-07-mbapproved-typedreach-review-fixes.md`
- `docs/build-log/2026-11-07-rscope.md`
- `docs/build-log/2026-11-07-rsdecl.md`
- `docs/build-log/2026-11-07-rspt11dn-self-review-repair.md`
- `docs/build-log/2026-11-08-citeperf.md`
- `docs/build-log/2026-11-08-tree-scope-predicates.md`
- `docs/build-log/2026-11-09-linequal.md`
- `docs/build-log/2026-11-11-affmeasure.md`
- `docs/build-log/2026-11-11-deadcode-ratchet.md`
- `docs/build-log/2026-11-11-nomiss.md`
- `docs/build-log/2026-11-11-orphans.md`
- `docs/build-log/2026-11-11-trial-harness-archive.md`
- `docs/build-log/2026-11-13-macgate1113.md`
- `docs/build-log/2026-11-13-selwalk1113.md`
- `docs/build-log/2026-11-16-r1replay-ownership-replay-complete.md`
- `docs/build-log/2026-11-18-source-removal-b1-benchmark-ownership-repair.md`
- `docs/build-log/2026-11-18-source-removal-b1.md`
- `docs/build-log/evidence/2026-11-11-relhyg-untested/classification.tsv`
- `docs/decisions/0048-four-owner-calls-2026-09-04.md`
- `docs/decisions/0250-source-view-native-port-gaps-2026-09-13.md`
- `docs/decisions/0308-single-name-corvint-identity-2026-09-17.md`
- `docs/decisions/0348-self-dogfood-mapping-qualifies-store-scope-2026-09-22.md`
- `docs/decisions/0364-compact-evidence-summaries-exact-drill-down-2026-09-23.md`
- `docs/specs/affected-plan-reuse-v0.md`
- `docs/specs/affected-plan-selection-v0.md`
- `docs/specs/authoring-step-scope-v0.md`
- `docs/specs/cem-content-keyed-claims-v0.md`
- `docs/specs/dogfood-citation-plan-precheck-v0.md`
- `docs/specs/experimental-source-views-v0.md`
- `docs/specs/go-test-evidence-v0.md`
- `docs/specs/immutable-delta-v0.md`
- `docs/specs/openspec/changes/source-removal/design.md`
- `docs/specs/openspec/changes/source-removal/path-verdicts.md`
- `docs/specs/openspec/changes/spec-set-cleanup/specs-verdicts.md`
- `docs/specs/openspec/changes/spec-set-cleanup/tasks.md`
- `docs/specs/test-selection-precision-v0.md`
- `docs/specs/work-queue-observation-v0.md`
- `tools/affected-edit-oracle/testdata/corpus.json`
- `tools/ci-forward-row/testdata/outcomes/shard-0.json`
- `tools/ci-forward-row/testdata/outcomes/shard-2.json`
- `tools/ci-forward-row/testdata/outcomes/shard-3.json`
- `tools/ci-forward-row/testdata/outcomes/shard-4.json`
- `tools/ci-forward-row/testdata/outcomes/shard-5.json`
- `tools/ci-forward-row/testdata/outcomes/shard-7.json`
- `tools/ci-forward-row/testdata/selection/shard-0.json`
- `tools/ci-forward-row/testdata/selection/shard-2.json`
- `tools/ci-forward-row/testdata/selection/shard-3.json`
- `tools/ci-forward-row/testdata/selection/shard-4.json`
- `tools/ci-forward-row/testdata/selection/shard-5.json`
- `tools/ci-forward-row/testdata/selection/shard-7.json`
- `tools/ci-forward-row/testdata/shadow/dry-run-3.json`
- `tools/ci-forward-row/testdata/shadow/manifest-3.json`
- `tools/ci-forward-row/testdata/streams/pr38476173623-shard7-attempt1.json`

### sr-b23 retained evidence

`b1-reference-active.json` (1990 bytes), SHA-256:
`0ab8e0b3a19df21677965c75f759148c9acdd2aca013394960b95f8a45e63498`.

`b1-reference-excluded.json` (12315 bytes), SHA-256:
`3cd8e2be69ebc86bd0cd1d9bab4a7a59b314f8b07016dfeb0ab923e38f6904dd`.

`b1-affected.json` (339411 bytes), SHA-256:
`e3d767d990d5ebfe8ab6700d096f26441f29e7cbd52184ba1fd817e3fce6ed35`.

`b1-github-active.patch` (4350 bytes), SHA-256:
`0a030af2f2f67ca2a5ae3ad5fc49a88103171177da1a72e76445cbfec4cc14db`.

Raw evidence directory: `/home/russell/lanes/sr-b23-evidence/`.

### Verification repair

The first final spec check rejected wrapping an Agent digest field over multiple lines
(`TestIndexCoversSpecsAndHeaders`). The detailed implementation list now lives in a normal body
section, with a one-line digest pointer. No test or checker was changed. The generated README
row remains generator-formatted; its table format exceeds 100 columns. Authored Markdown additions
otherwise stay within 100 columns. The failed run remains in `b1-doc-checks-final.log`.

Final focused documentation gate: PASS after the digest repair. The whole specindex package,
spec requirements, line citations, OpenSpec (65 existing baseline findings, zero failures), and
long-line check passed. `make changed-checks` also passed its citation, unbounded-reader and
read-scope checks. Repinning found no moved anchors. No code or spec inputs changed after that
successful final run; this paragraph records its result only.

The final affected plan retained the same counts and unknowns reported above (129 dirty paths).
`b1-affected-final.json` SHA-256:
`e6c1889e71b872413a1cbdf214af6eea8ed395a2ecd6dded0fae79f167807aac`.

## Round 3: B1 completion (2026-11-18)

The orchestrator amended RMV-V1-003 item 4 to exclude frozen measurement and receipt records.
The capability spec and change delta carry that decision. The pinned TBC-V1-012 export
`.corvint/requirement-links.json` is restored byte-for-byte from selection base
`9f4d5d89ab0648425db40e617f3d890598f9ff79`; its historical paths are expected.
This section supersedes the preceding PARTIAL disposition and the earlier filtered-export claim.

Newly excluded hits, by path:

- `.github/cishards/testdata/run42408095814.json`
- `benchmarks/daily-loop-v0/corpus.json`
- `benchmarks/daily-loop-v0/runs/run-001.json`
- `benchmarks/daily-loop-v0/runs/run-002.json`
- `benchmarks/taskcontext-lookup/default-summary.json`
- `benchmarks/taskcontext-lookup/pack-summary.json`
- `benchmarks/untouched-repository-v1/corvint/runs/run-002.json`
- `benchmarks/untouched-repository-v1/viper/admission.json`
- `conformance/use-cases-v0/receipts/UC-TASK-ORIENTATION/corvint-dogfood/prechange-query.json`
- `.corvint/requirement-links.json`

Item 3: `CORVINT_GATE_LEDGER=off make -n gate doc-gates shell-test release-binaries-check`
passed. Disabling ledger reuse makes recursive make inherit dry-run mode without executing the
ledger program. The documentation timing wrapper runs only recursive dry-run recipes. No tests,
gates or whole-repository selection ran. Raw output is `b1-make-dry-run.log` in lane evidence.

Item 4: zero removed-path hits outside the stated exclusions and `.github/`. The exact remaining
active files are `.github/cishards/package-costs.json`, `.github/cishards/walltime_test.go` and
`.github/linecov/qualification.json`, pending the separate authorized GitHub-only PR. The handback
carries its patch and the qualification regeneration command. Items 1, 2 and 5 reuse the preceding
successful code-bound runs: this round's B1 repair changes documentation and restores a historical
export only. Independent review and merge-head verification belong to the orchestrator.

RMV-V1-007/031 B1 is complete under the round-3 proof boundary; B2 may proceed. The retained
FIH-V1-011 oracle and prior orphan table are unchanged. Rollback: revert this repair commit.
