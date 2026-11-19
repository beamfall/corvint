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
