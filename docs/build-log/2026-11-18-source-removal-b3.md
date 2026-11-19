# Source removal B3 — V1-5694

Authority: source-removal charter, RMV-V1-010/011, owner 2026-11-18.
RMV-V1-004 import audit: runnerpolicy has no Go importer;
runnerqualification is imported by runnerpolicy and tools/runner-qualification;
runnerprotocol is imported by tools/runner-plugin and tools/runner-reference.
There are no retained Go importers, so RMV-V1-005 needs no preparatory Go-import rewrite.
The exact per-file importer list is retained outside the repository in b3-importers.txt.

RMV-V1-010 removes those three internal packages and three tools, their package-local tests,
read-scope and runtime-reader registrations, direct-Git allowlist entries, documentation-path
registrations and capability rows. The ownership generator's existing repository test keeps
its exact remaining literal-read list. No .github file is edited; its active cost inventory and
qualification record require the separate PR described in the handback.

RSP-V0-025 now specifies the compiled-in interface. RSP-V0-026 through RSP-V0-032 and RSP-V0-047
carry explicit REMOVED status lines; the historical requirement definitions and IDs remain.
No code reads the retired discovery file after this batch. The zero-reference audit retains
only historical spec, change and build-log references.

RMV-V1-011 leaves registry.RunnerPlugin, runnerselection, testrunner and the held golang,
e2erunner, jsresolve and pyresolve packages unchanged. Vitest, Playwright, Jest, pytest, RSpec
and Minitest execution behavior is unchanged. The Jest test ticket write is NOT_RUN because
this lane forbids task-store writes; the orchestrator owns that integration step.

## Orphans and rollback

| Orphan | Disposition |
| --- | --- |
| runner protocol, policy and qualification source and tests | Removed together |
| three tool command-tree snapshots and README files | Removed with their tools |
| runtime/read-scope/area registrations for the six paths | Removed |
| documentation-path generator entries and pinned literal expectations | Removed together |
| direct-Git exemptions for deleted readers | Removed |

RMV-V1-030: revert this batch commit; its parent holds the recovery source.
RMV-V1-029 perfbudget remains present. No runner contract or provider envelope changes.

The plugin-envelope spec loses only the obsolete runner discovery-file example under
CPE-V0-007. Its generic declaration, digest, failure and process contracts are unchanged.

## Reference and target proof

RMV-V1-003 item 3: the four required make targets dry-run successfully, with
CORVINT_GATE_LEDGER=off so the dry-run writes no ledger. Item 4: zero active hits outside
the documented exclusions and .github; 33 excluded paths and two pending .github paths.
The runner-plugins.json audit has no code hits; all 14 hits are historical/retired spec text.

| Excluded hit path |
| --- |
| `.corvint/changes/3723dfa6baeec38b5862d59bd4370ca8c58e7379.cem.json` |
| `.corvint/changes/5fe7261afb0350358a6ebe14eced44bc1d5e8396.cem.json` |
| `.corvint/changes/7d16af44d60b8cf2f28f6b50110ddd4ef1631f94.cem.json` |
| `.corvint/changes/8db91c594213c4d38c1759d350997befeb4a3d52.cem.json` |
| `.corvint/changes/bc3d298e14545292b54dea608f53f40f337fe06e.cem.json` |
| `.corvint/changes/d32fd3c7a2caca43b8d27e1cb7265f47a9f80f6a.cem.json` |
| `docs/build-log/2026-11-07-rspfx-proto-repair.md` |
| `docs/build-log/2026-11-11-rsp-t10-adapter-host.md` |
| `docs/build-log/2026-11-13-macgate1113.md` |
| `docs/build-log/2026-11-16-r1replay-ownership-replay-complete.md` |
| `docs/specs/PRODUCT-PLAN-1.0.md` |
| `docs/specs/openspec/changes/runner-contract/proposal.md` |
| `docs/specs/openspec/changes/runner-contract/specs/runner-contract/spec.md` |
| `docs/specs/openspec/changes/source-removal/design.md` |
| `docs/specs/openspec/changes/source-removal/specs/source-removal/spec.md` |
| `docs/specs/runner-contract-v1.md` |
| `docs/specs/runner-selection-plugins-v0.md` |
| `docs/specs/source-removal-v1.md` |
| `tools/affected-edit-oracle/testdata/corpus.json` |
| `.corvint/changes/d1315d042c5724bff7aff0a30464a3668f05e584.cem.json` |
| `.corvint/changes/df13b4fe99f8f77dc5249a572220fa004d96eab3.cem.json` |
| `.corvint/requirement-links.json` |
| `docs/build-log/2026-11-08-rsp-affected-shadow-removal.md` |
| `docs/build-log/2026-11-08-rsp-t7-consumption.md` |
| `docs/build-log/2026-11-08-rspt7-self-review-repair.md` |
| `docs/build-log/2026-11-09-linequal.md` |
| `docs/build-log/2026-11-11-orphans.md` |
| `docs/build-log/2026-11-13-runner-contract-run-path.md` |
| `docs/build-log/TEMPLATE-rsp-baseline-update.md` |
| `docs/build-log/2026-11-07-rspfx-qualification-self-review.md` |
| `docs/build-log/2026-11-07-rspt14-successful-write-repair.md` |
| `docs/build-log/2026-11-18-source-removal-b3.md` |
| `docs/specs/openspec/changes/pytest-provider/proposal.md` |

## Evidence digests

- `b3-importers.txt`: `e0c923be9f9682619cfafc56d16dba1c168ea57fd7b3b5ea13831809ab8fb5c4`.
- `b3-round3-active.json`: `ca3d163bab055381827226140568f3bef7eaac187cebd76878e0b63e9e442356`.
- `b3-round3-excluded.json`: `4c4b5b5555a2ef32bff55f2c71ae4d67728527b038bb54dcbd66ce53f541e66d`.
- `b3-round3-github.json`: `455f1a8bd149a3bb90da7c44d6599e94fab8958703765df5821366ff33224dbf`.
- `b3-discovery-hits.txt`: `7a9ca7c4e22ca18a9cdc14fc9f909a56a23a9aeb32c2a79cdea7f46e1d7c4840`.
- `b3-make-dry-run.log`: `36410cb81bda9388227101380616f0339c6863a5a977d95dd85202f8e69870fa`.

Focused shell verification: documentation-path generation and ticket-gate fixtures pass.
The cutover fixture initially failed because its synthetic replacement path had no component
in the vocabulary; it now uses the retained gateselect root. Every original component, count,
ordering and missing-path assertion is unchanged. This is a B2 fixture repair, not a product change.

## Final focused verification

Root and interop vet, Windows vet, the required spec/citation/OpenSpec/long-line/unbounded-reader
checks, and changed-checks pass. OpenSpec retains 65 baselined findings and adds none.
Whole-package passes include release-binary (23.477s), import-ratchet (0.865s), capcatalog
(0.022s), capabilities-check (4.974s), compatibility-table (0.078s), specindex (1.516s),
jstestprovider (8.209s) and rubytestprovider (1.811s). B2's other focused passes remain recorded
in its entry. Every touched package ran its existing whole-package tests through heavy.sh.

The final whole core run (145.645s) retains only the seven nonrepository/profile failures caused
by /home/russell/lanes/.git above the mandated TMPDIR. The Ruby command package retains its
corresponding nonrepository failure. No diagnostic, golden or assertion was changed for them.
Provider-library checks pass; full Ruby command qualification remains blocked on the host rerun.

The repository-reader package passes (135.046s) after the two retained flow review anchors point
to their intent-change commit. The source-removal flow declaration also drops the removed flow
from its two consumers, preserving their Go/JS bindings. The affected plan no longer reports
E2E_FLOWS_INVALID. The compatibility page is regenerated from the reduced catalog and retains
its row for the two surviving MCP servers.

The final precommit affected plan is UNKNOWN: 444 selected, 249 excluded, 105 unknown entries,
with eight unresolved packages. Unknown reasons: LANGUAGE_FRONTIER 55, NO_SELECTABLE_TEST 1,
UNINDEXED_SOURCE_PATH 5, UNOWNED_DIRTY_PATH 42 and UNSELECTED_BINARY_CONSUMER 2. No broad fallback
suite runs here. Extra retained-rule/provider checks under RMV-V1-009 cover testvalidity and
jstestprovider beyond the planner-selected packages. No live-platform claim follows from these
Linux package tests; Darwin execution is NOT_RUN.

B3 keeps the held runner sources byte-identical to B2, including appflows/e2erunner. The pinned
requirement export remains exactly the supplied base; .github remains untouched. Its separate
patch passes git apply --check; the bound qualification report must be regenerated on the host.
A measured semaphore wait of 1338.3s preceded the 0.078s compatibility-table test; no override
was used. Earlier failed checks and the subsequent passes remain in external lane evidence.

Independent review, integration, ticket-store writes and publication are NOT_RUN by lane rule.
CEM bind and a valid citation plan are NOT_PRODUCED: the current map and workflow are absent
under decision 0594. The requested dogfood-check availability check is recorded in the handback;
no seal is attempted. This ticket's acceptance remains PARTIAL for those external closeout steps.

Final runtime and gate evidence is retained outside the repository; these digests bind the
reported numbers to those files:

- `postpin-affected.json`:
  `4ad37fa25a2c9fced403502450b143a469030c649d3b3108a2c55408c5b5e1af`.
- `b3-root-vet.log`:
  `bea6af2cadcae06956d34b17311ae50e834080631993557f7393058fa254a051`.
- `b3-interop-vet.log`:
  `acd065fee9ffe3c1f443dfa25e4f3b324d042f752d2c59c09442e15c10cadd17`.
- `b3-windows-vet.log`:
  `c4c439234f48f1ff79fce44a999770185016c15ab498012f4baf915e135ffee7`.
- `final-required-checks.log`:
  `bb46ad2b91c69b2b338a1856da89b0c6b12cc8481a7ce82927ded7fddaa5e372`.
- `final-changed-checks.log`:
  `aaff72429019b970f3ab674b1676688f6fb1cc8d429c06f42b9eea3f6db5f08e`.
- `test-cmd-corvint.log`:
  `16f02ed4a3eb00d27e9150edecf9a7f449be1e44e685ea93cd816a0c64033a17`.
- `test-internal-liveverify-affected-liverepo.log`:
  `3b3cef5ca7d75eaaae0cae3cefdf517672cb41510e95a1696614b1901406592f`.
- `test-internal-jstestprovider.log`:
  `d5d11440fd15ca828e4e5aa209e9cc01e15551249f631695f204750cc91d4f41`.
- `test-internal-rubytestprovider.log`:
  `c4e3a9746bcb244d7afc09a550b4e9f86bc174078e5d18c62af79aed7123e14a`.
- `test-tools-compatibility-table.log`:
  `ef156644bf7f4ab657cd3acaa72220a338ba409d30460efbae05b290c467b5f5`.
