# V1-5087: conservative consumers and verifiable widening

Self-review repair of P1 (declared binary consumers omitted) and P2 (unresolvable
unbounded witnesses), under CIN-V1-940..942. The original implementation and its external
CI integration limit remain recorded in `2026-11-18-m8-narrow.md`.

Decision: every unit in the unresolved runtime-edge closure is conservatively rebuilt,
including a testless command reached from a holder test. The existing consumer machinery
therefore selects its declared runnable consumers and retains an
UNSELECTED_BINARY_CONSUMER warning for each consumer no selected test runs. No dirty-path
syntax or witness-kind heuristic can prove such a command unaffected.

The same closure is registered in the graph's unbounded-reader set, causes and chains.
Its graph-bound runtime degradation supplies the reason returned by UnboundedChain and
used by reader selection. Existing read scopes and name-only flags cannot narrow this
unresolved obligation. This retains the existing UNBOUNDED_READER and DECLARED_CONSUMER
wire forms, error codes, stderr and exit codes; no golden or existing assertion changed.

Class and closed admission: a declared consumer must have its exact declared argv and
final hop, with either its existing ordinary build proof or a one-unit command chain
registered as unbounded whose runtime degradation is present in the graph frontier.
Arbitrary unbounded causes do not establish a rebuilt binary. Unknown dirty-path forms
retain the whole conservative closure; unknown proof forms cannot certify a consumer.
This closes the consumer-propagation and unsupported-proof classes, rather than listing
specific source suffixes or adding more rejected forms.

`TestRuntimeWideningConsumersAndWitnesses_CINV1941` independently validates every selected
witness with affectedcheck.WitnessResolves. Its fixture has a holder, a terminal test
importer, a declared-scope reader, a testless command with runnable and non-runnable
consumers, and an unrelated unit. Holder source, holder test and an unknown input form
each select three whole test units plus one declared consumer and retain the non-runnable
consumer warning. A control with an unknown unbounded cause rejects a forged consumer
witness and keeps an ordinary holder-test change from rebuilding the command. The existing
CIN-V1-940 widening and strict-refusal regressions remain intact.

The tested source is retained by staged tree `dc6f1ca39fe67a15f06798512836d08715c20cb9`;
subsequent changes in this repair add evidence documentation only. Raw logs and the affected
plan are outside the repository in `~/lanes/m8-narrow-evidence/`. The final command manifest,
intent file and retired-CEM citation marker remain in `~/lanes/m8-narrow.tmp/`.

## Verification

All Go commands ran through script/heavy.sh with GOTOOLCHAIN=local, the configured lock
path and TMPDIR=/home/russell/lanes/m8-narrow.tmp. No Git identity was exported during
verification. Each test command names one package and uses -count=1 -timeout 30m.

| Whole package | Result |
| --- | --- |
| internal/liveverify/affected | PASS, 10.329s |
| internal/liveverify/affected/affectedcheck | PASS, no test files; regression runs in parent |
| internal/gateselect | PASS, 4.222s |
| tools/ci-forward-row | PASS, 66.911s |
| cmd/corvint | FAIL, 177.057s; same seven sandbox failures as the prior round |

The installed affected command refused self-binary-stale. A fresh local build produced
the required base selection, which selects all four lane packages with tests and remains
UNKNOWN: LANGUAGE_FRONTIER, NO_SELECTABLE_TEST, UNOWNED_DIRTY_PATH and
UNSELECTED_BINARY_CONSUMER. The touched verifier package has no tests of its own.

The focused runtime regressions pass (0.024s). Their first run rejected the new fixture's
noncanonical ../holder read scope; correcting it to holder/ fixed the fixture without
changing any assertion. Both runs remain retained. Whole CLI verification reproduces the
prior seven failures: TestAHI373MCPVerbOwnsItsArgumentsAndProfile,
TestGPKV0084ImpactCapabilitiesOutsideGitRoot, TestPRBV0008EveryNamedCommandWritesProfiles,
TestWatchHelpAndRefusals, TestAffectedRejectsNonRepositoryAndExtraArguments,
TestCoreVerbsRefuseAWorkingDirectoryOutsideTheRootAlike and TestConvertedRefusalDiagnostics.
The read-only ancestor /home/russell/lanes/.git makes temporary non-repository fixtures
appear to belong to that directory. Host verification remains necessary; no product
workaround or assertion change was made.

The requested spec-requirements-check and line-citations-check pass. Citation repinning
changed one line number with the same content hash; its second pass found no further move.
The owner excludes repository-wide gates, changed-checks and Windows vet in this repair.
No hosted or Darwin qualification was produced.

Logs and plan (SHA-256; raw files remain outside the repository):

| File | Bytes | SHA-256 |
| --- | ---: | --- |
| self-review-focused.log | 991 | `8c05917514e1a2b68de90e02f4db52ef0268a5175db67509e9ac992bbdd83a96` |
| self-review-focused-fixed.log | 771 | `d774adfff73beb0a4679b3200b23b39781f80f80923bebda6a26ec18f9348c91` |
| self-review-affected-whole.log | 773 | `845b0b30228a380371387c126a144b048a05bae6bc3f529031d65246a3795adc` |
| self-review-affectedcheck-whole.log | 601 | `e30d64dc67b4987590e969acc6a5775a538d771ab2e7670834693fb9e50edd60` |
| self-review-gateselect-whole.log | 569 | `394c3ebbf7ca94d42376a3eb5547db77ad478e8bf931831d7e6b6f2b841fd3c5` |
| self-review-ci-forward-row-whole.log | 572 | `ca0f8a816c820def31fe97b3594f62bd2f28679a5bfb494e19c33b80eee3329e` |
| self-review-cmd-corvint-whole.log | 6762 | `ee20e0e1371882abef35ccafc9c81366b7429d776cefe9c5ebf3336f7f5ca36a` |
| self-review-affected.json | 305301 | `05a1474f123a852fff922e92cc267bd8b0f708cb243edaa07d9ccdd2dda83674` |
| self-review-spec-citations.log | 909 | `5476fd9d0973c9d0ee00c144dd4917696d3ac6083b55eb8d465ad2b604760ec8` |

Rollback: revert this repair commit, restoring the prior widening implementation and its
known P1/P2 gaps; do not change frozen outputs to accommodate those gaps. The ticket remains
PARTIAL for its existing external hosted CI integration and host verification obligations.
The independent review is owned by the orchestrator. CEM binding and dogfood-check were
removed by decision 0594; `docs/DOGFOOD.md` records their retirement. No removed tooling is
restored and no seal, push, PR or task-store write is authorized in this lane.
