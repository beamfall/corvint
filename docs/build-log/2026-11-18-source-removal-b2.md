# Source removal B2 — V1-5694

Authority: source-removal charter and round-3 retention decisions, 2026-11-18.
Base: B1 completion `c27cd0f3ef2ce89b3bd77e4ad325fb9b70896a75`.
RMV-V1-001/004/005: the import audit precedes deletion; preparatory test commit
`b73e29246a94cd893e180849fac33149776da067` passes focused vet. The retained Playwright provider
and flow-ingest assertions are unchanged; the deleted decoder leaves their comparison.

RMV-V1-008: removed the test-plan, runs, test-validity and tests lint/duplicates/validity
command paths, their dedicated tests, internal/testplan, internal/playwrightruns, the standalone
validity MCP server and bridge, and its conformance fixtures and installed-flow document.
Command snapshots and capability rows follow the reduced surface. Existing unknown-command
and invalid-argument diagnostics are unchanged. The previously skipped RVD-V1-008 removed-verb
check now executes, and TestRMVV1008RetiredCommandsRefuse covers each removed dispatch.
CMS-V0-915 now pins the two remaining flow readers.

RMV-V1-009: the projection, provider vocabulary and Flaky rule are unchanged. Pinned file reads
already lived in internal/pinnedread; the runner and external-origin callers now import it
directly. The decoder and compatibility wrappers remain for existing receipt readers. Shared
receipt helpers and test fixture helpers move out of deleted command files without changing
behavior. The bounded writer and strict-refusal regression fixtures remain test-only, with
all their assertions. Retained frozen goldens and core-freeze outputs are unchanged.

## Retained paths and design decisions

The current-base premise fails for three paths; the charter permits retention with a reason:

- The TCQ evaluator chain is named by immutable protocol migration records outside the allowed
  exclusions; changing those records would violate the frozen-contract instruction. Keep the
  chain and its assertions until a coordinated historical-record disposition is accepted.
- The validity source files serve the kept unselected adapter and teardown bound. Keep that
  shared reader and its assertions until its owners split that contract.
- The spec-coverage tool remains an input of the repository read-scope qualification test.

The exact paths and witnesses are recorded in the change's path-verdicts.md. No new authority,
wire format, error code or relaxed test assertion is introduced by these retention decisions.
TCN, MTV and TCQ specs carry B2 retirement status; retained provider contracts stay in force.

## Orphans and rollback

| Orphan | Disposition |
| --- | --- |
| test-plan command files and dedicated tests | Removed with their only dispatch |
| runs command files and dedicated tests | Removed with their only dispatch |
| standalone validity MCP command tree and catalog row | Removed with the binary |
| removed packages' area prefixes and read-scope registrations | Removed |
| shared session fixture helpers | Moved, because existing kept tests still call them |
| shared receipt reader helpers | Moved, because teardown and example-join still call them |

RMV-V1-030: revert the B2 deletion commit and then its preparatory test commit. Their parents
retain the original source. RMV-V1-031: no new production package is introduced. Perfbudget and
all RMV-V1-011 held runner packages remain untouched.

## Verification and limits

RMV-V1-003 item 3 passes with CORVINT_GATE_LEDGER=off and recursive make dry-run.
Item 4 has zero hits outside the documented exclusions and the pending .github PR.
Item 5 passes the existing whole import-ratchet package (1.562s). Whole appflows (15.894s),
closureguard (2.879s), testrunner (32.081s), testrunner/sql (0.020s), affected/golang (18.070s),
JS provider (1.891s) and Go provider (67.653s) passed. The retained decoder assertions and
provider golden files are unchanged. The catalog mutation tests now exercise surviving
subcommands with the same assertion counts and failure kinds; the reduced core tree has an
exact 50-node assertion instead of the former 60-node minimum.

External qualification blocker: /home/russell/lanes/.git exists outside the writable metadata
scope. Tests requiring a nonrepository scratch directory discover that ancestor through the
fixed TMPDIR. The whole Ruby provider package fails TestLPCVV0090RefusalsExitTwo for that
reason; the initial whole cmd/corvint run reports the same ancestor in profile/root-refusal
and frozen diagnostic tests. GIT_CEILING_DIRECTORIES does not affect the provider's filesystem
ancestor walk. No product root detection, fixture expectation, goldens or stderr were relaxed.
Host rerun with isolated scratch ancestry remains required. Provider parity is NOT_QUALIFIED
for Ruby; this external failure does not invalidate the source-removal premises.

Earlier repair failures are retained in lane evidence: stale guide/catalog mutation examples,
a removed MCP-server enum reader, and shared fixture helpers discovered by compilation.
The requested exhaustive tests, doc-gates and whole-repository selection remain NOT_RUN.
Independent review, publication and native task writes belong to the orchestrator.
Dogfood bind/check/seal are unavailable: decision 0594 removed their targets and implementation.

## Excluded reference inventory

Each row is a remaining hit file under RMV-V1-003's exclusions. The JSON audit outside the
repository maps each row to every removed path it names.

| Excluded hit path |
| --- |
| `docs/build-log/2026-11-18-source-removal-b2.md` |
| `.agent-evidence/ctx-w58/verification/affected-final.json` |
| `.agent-evidence/ctx-w58/verification/affected.json` |
| `.corvint/changes/0e1c734215466cfd81c759dc8a644bf2dce60e4a.cem.json` |
| `.corvint/changes/291c86c9ca1265bfd522bc7f05301fe839bb094b.cem.json` |
| `.corvint/changes/a38802bfa14833d21f1c494a5c99bab92dbcd445.cem.json` |
| `.corvint/changes/a3fbf3b8b63f8245b25edf3ba9e0716be532ced2.cem.json` |
| `.github/cishards/testdata/run42408095814.json` |
| `.taskman/tickets/V1-1667.json` |
| `.taskman/tickets/V1-1788.json` |
| `.taskman/tickets/V1-1807.json` |
| `benchmarks/results/v2892-selection-baseline-2026-10-31-base.json` |
| `benchmarks/results/v2892-selection-baseline-2026-10-31-lane.json` |
| `benchmarks/results/v2892-selection-baseline-2026-10-31-merge.json` |
| `benchmarks/results/v2892-selection-baseline-2026-10-31-pre14.json` |
| `benchmarks/results/v2892-selection-baseline-2026-11-01-58.json` |
| `docs/build-log/2026-10-31-internal-wsl.md` |
| `docs/build-log/2026-11-01-lnt-u32.md` |
| `docs/build-log/2026-11-01-sec-u19.md` |
| `docs/build-log/2026-11-09-flows-how-to-docs.md` |
| `docs/build-log/2026-11-16-r1replay-ownership-replay-complete.md` |
| `docs/decisions/0595-flows-and-how-move-to-docs-2026-11-09.md` |
| `docs/specs/PRODUCT-PLAN-1.0.md` |
| `docs/specs/application-navigation-ledger-v0.md` |
| `docs/specs/openspec/changes/source-removal/design.md` |
| `docs/specs/openspec/changes/source-removal/specs/source-removal/spec.md` |
| `docs/specs/openspec/changes/spec-set-cleanup/specs-verdicts.md` |
| `docs/specs/source-removal-v1.md` |
| `docs/specs/test-consolidation-planner-v0.md` |
| `tools/affected-edit-oracle/testdata/corpus.json` |
| `tools/ci-forward-row/testdata/outcomes/shard-1.json` |
| `tools/ci-forward-row/testdata/selection/shard-1.json` |
| `tools/ci-forward-row/testdata/shadow/manifest-3.json` |
| `.corvint/requirement-links.json` |
| `.taskman/tickets/V1-1235.json` |
| `.taskman/tickets/V1-1609.json` |
| `.taskman/tickets/V1-1610.json` |
| `.taskman/tickets/V1-1624.json` |
| `docs/build-log/2026-11-07-rsdecl.md` |
| `docs/build-log/2026-11-09-cut-e2e-proof-lab.md` |
| `docs/decisions/0593-cut-e2e-proof-lab-2026-11-09.md` |
| `docs/specs/application-flow-understanding-v1.md` |
| `docs/specs/observed-ui-evidence-v0.md` |
| `docs/specs/openspec/changes/archive/2026-11-13-run-evidence/proposal.md` |
| `docs/specs/playwright-run-evidence-v0.md` |
| `docs/specs/test-source-analysis-v0.md` |
| `tools/ci-forward-row/testdata/outcomes/shard-5.json` |
| `tools/ci-forward-row/testdata/selection/shard-5.json` |
| `.agent-evidence/pk-tstore-fix1/affected-final.json` |
| `.agent-evidence/pk-tstore-fix1/affected-narrow-final.json` |
| `.agent-evidence/pk-tstore/affected-delivered.json` |
| `.corvint/changes/3881134e7bb3dbbe5ee25b55eceb01d4a14f8081.cem.json` |
| `.corvint/changes/6320422bea57b70c3f3ac72c6b5e2de1c00172b0.cem.json` |
| `.corvint/changes/aab5212b564c43751705735963e9361633adf338.cem.json` |
| `.corvint/changes/b9f67a6471db95f13bb502bf7fc80ed3785f6075.cem.json` |
| `.corvint/changes/d4d04474f6ec12f1f8c2f3207356741142637cce.cem.json` |
| `benchmarks/daily-loop-v0/corpus.json` |
| `benchmarks/daily-loop-v0/runs/run-001.json` |
| `benchmarks/daily-loop-v0/runs/run-002.json` |
| `docs/build-log/2026-11-07-rscope.md` |
| `docs/build-log/2026-11-09-layer-docs-targets.md` |
| `docs/build-log/2026-11-11-layers-remaining-edges.md` |
| `docs/decisions/0147-mcp-test-validity-descendant-profile-2026-09-12.md` |
| `docs/decisions/0600-keep-swift-kotlin-providers-2026-11-16.md` |
| `docs/specs/affected-plan-v0.md` |
| `docs/specs/mcp-multi-root-v0.md` |
| `docs/specs/mcp-server-2026-07-28-v0.md` |
| `docs/specs/mcp-test-validity-profile-v0.md` |
| `docs/specs/mcp-tool-table-v1.md` |
| `docs/specs/openspec/changes/source-removal/path-verdicts.md` |
| `docs/specs/performance-profiling-and-read-budget-v0.md` |
| `tools/ci-forward-row/testdata/outcomes/shard-6.json` |
| `tools/ci-forward-row/testdata/selection/shard-6.json` |
| `.agent-evidence/native-taskman/affected-dirty.json` |
| `.taskman/tickets/V1-1781.json` |
| `.taskman/tickets/V1-1838.json` |
| `.taskman/tickets/V1-2092.json` |
| `benchmarks/untouched-repository-v1/corvint/runs/run-002.json` |
| `docs/specs/live-proof-carrying-verification-v0.md` |
| `tools/ci-forward-row/testdata/outcomes/shard-0.json` |
| `tools/ci-forward-row/testdata/selection/shard-0.json` |
| `.agent-evidence/cx-affected-selection-replay.json` |
| `.agent-evidence/pk-tstore-fix1/base-binding.json` |
| `.agent-evidence/pk-tstore/affected-start.json` |
| `docs/build-log/2026-10-31-win-w2-bugs.md` |
| `docs/build-log/2026-11-09-oneplan.md` |
| `docs/build-log/2026-11-11-compatibility-promise.md` |
| `docs/build-log/2026-11-13-macgate1113.md` |
| `docs/specs/affected-plan-selection-v0.md` |
| `docs/specs/public-release-v0.md` |
| `tools/ci-forward-row/testdata/outcomes/shard-4.json` |
| `tools/ci-forward-row/testdata/selection/shard-4.json` |
| `.corvint/changes/51d981b899911febbc5b567d8684d697f9025e12.cem.json` |
| `.corvint/changes/8b76e6f4b7708e29651b6da5224cc319ca19d96f.cem.json` |
| `docs/build-log/2026-11-04-pk-affected-serial-tests.tsv` |
| `.corvint/changes/7d16af44d60b8cf2f28f6b50110ddd4ef1631f94.cem.json` |
| `.corvint/changes/9d41070d1d3f120f357127208b35abb173affa23.cem.json` |
| `.corvint/changes/8e2a9fbfd9bdca1e6f158a6fbd7ce230525aeef1.cem.json` |
| `.corvint/changes/b084781271d1b19a6cfdde328f679af6262471c8.cem.json` |
| `.corvint/changes/e688bd44dc16fea862f252aaed17ba7f4be5c987.cem.json` |
| `docs/specs/go-production-kernel-migration-v0.md` |
| `.corvint/changes/f36fa9f14014870c35663280f7d82b8ad12dee66.cem.json` |
| `docs/build-log/2026-10-31-cmd-w1-bugs.md` |
| `docs/specs/secret-screen-writer-v0.md` |
| `.taskman/tickets/V1-2259.json` |
| `docs/decisions/0594-refocus-on-test-selection-2026-11-09.md` |
| `docs/specs/corvint-tasks-source-containment-v0.md` |
| `docs/specs/external-evidence-provider-v0.md` |

The OpenSpec batch checkboxes remain unticked: task 1 acceptance integration and task 3
(.github-only B0) remain open. Marking task 4 alone fails OSC-V1-005 ordering, so the B1
implementation proof lives in its build log without claiming those integrations are done.

Additional whole-package passes: internal/capcatalog 0.031s, tools/capabilities-check 6.236s,
internal/testvalidity 0.006s and internal/testvaliditydoc 0.304s. The catalog missing-option
fixture now uses instructions with the same three missing-option assertions. The help vocabulary
continues to enumerate all 58 retained reader codes; its existing completeness check is unchanged.
Line citations pass after repinning and pinning three formerly unresolved TCQ quotations.
OpenSpec passes with 65 existing baselined findings; long-lines and unbounded-readers pass.

## Retained evidence digests

- `b2-affected.json`: `52a77301ecc7afa8b579d32282dc3f5d8505b283e6df0e867b2cbf6b768daf25`.
- `b2-importers.txt`: `40d66cce796ab83a71c6352b9f644c176d9f336b1e9ec7feb34ca9ee1956aff5`.
- `b2-round3-active.json`: `ca3d163bab055381827226140568f3bef7eaac187cebd76878e0b63e9e442356`.
- `b2-round3-excluded.json`: `3514690470e6462d36f3019f8e518dea78d9dd9e9a9ac40fc9fcae8aca57eb59`.
- `b2-round3-github.json`: `afd8855084eb0afef471c3737c4d58d59f7f9ca674c47137ba4cb24a4ae89d54`.
- `b2-make-dry-run.log`: `0deaa69595b5da3b8ba165ff636bea8aead6bb1b903ebbe517f33fbfe96f02dc`.
- `b2-ruby-source-parity.txt`: `00256547946e73040afff955266043edfa8232b4b9c129177fc8e0ade7b530b9`.

Whole internal/testsource passes (13.303s); tools/mcp-server-doc passes (0.011s).
The last catalog repair regenerates the guide digest after its B2 wording change.
Dedicated Playwright-run comparison goldens are deleted with their removed package; no retained
provider golden changes. Earlier mismatched catalog and help results remain in external evidence.

Root vet, interop vet and Windows vet pass on the B2 source. The release catalog's existing
product/withheld assertions stay; its removed-binary expectation now requires absence. The
final whole-package rerun is required for that repair and the two restored generic tests
dispatch refusals. No retained assertion was relaxed to accommodate an environment failure.

The subsequent whole core run (156.814s) has only the seven ancestor-repository environment
failures already described. Catalog/MCP parity and retained reader-code completeness pass.
The latest test-only repairs are verified again at the final boundary with B3.
