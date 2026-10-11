# 2026-10-10: exact directory PATH key warnings at create and refine (V1-1125)

## Intent and scope

CAL-V0-214 in `docs/specs/corvint-tasks-agent-leases-v0.md` is **accepted, decision 0513
(V1-1125)**. It extends the advisory CAL-V0-211 warning to fresh CREATE and REFINE
transactions, including fresh batch-refine entries. CAL-V0-211's non-goals now point to this
requirement for create/refine; its accepted set-effects and claim behavior is preserved. No decision
file or acceptance declaration is added.

The earliest proof is the real CLI/store regression using a committed Git directory. CREATE uses
committed payload effects; REFINE cannot change effects and uses the committed ticket post. Both share
`store.EffectsDirectoryKeyWarnings` and check the caller's HEAD tree. The fresh-transaction guard
keeps replays and refusals warning-free. No warning changes a payload, stored effect, receipt,
code or outcome. Effects are retained in the transient store report under the writer lock by both
the complete-audit and writer-checkpoint paths; no persisted or wire field is added.

Rollback removes the CAL-V0-214 call sites and tests and regenerates the requirement index;
there is no stored-state migration. One builder and one bounded native read-only reviewer are
used, with the inherited model/effort for this routine implementation and source review. Runtime
tuning, billed usage and savings are NOT_OBSERVED.

## Acceptance evidence

`TestCALV0214_CreateAndRefineWarnOnExactDirectoryKey` exercises fresh create, single refine,
batch refine and each replay. Directory keys in touchPaths or PATH resources warn once;
duplicate keys across both sources warn once, including UNKNOWN coverage and externalUnbounded.
File keys, trailing-slash directory keys, absent keys and non-PATH resources do not warn.
Stored effects are compared with their original bytes after every mutation; complete ticket
records are compared before and after each replay.

The baseline CAL-V0-192/CAL-V0-211 CLI tests passed (3.288s). The new
CAL-V0-214 regression failed against the original call sites with missing directory warnings
(13.843s), then passed against the first implementation (6.551s).

Independent review identified stale pre-lock effects on REFINE: another SET_EFFECTS can commit
between CLI inventory load and writer-lock acquisition, or between batch chunks. The repair uses
the committed post retained under the lock. `TestCALV0214_CommittedEffectsStayBoundToTransaction`
exercises both writer routes, proves the preloaded inventory is stale, proves a later SET_EFFECTS
does not change the retained effects, and checks that replay retains none. The expanded race run
passed CLI (134.534s) but failed store (1045.616s) solely because this new test used the shared
50-ticket history initializer with a four-receipt expectation (`head 51, want 4`). The repair uses
`historyStoreAt` with one ticket and four receipts and explicitly proves the fast-path checkpoint
seed commits through the complete route before testing the checkpoint route. This fixture failure
is retained and is not counted as successful CAL-V0-214 evidence. A first repair compile failed
because the still-used `intent` import was removed; restoring it completed the fixture correction.
The independent reviewer checked the small fixture and route assertions and found no other issues.

## Verification

- Expanded focused race command: `go test -count=1 -race -timeout 30m
  ./internal/tasks/cli ./internal/tasks/store -run 'TestCALV0(106|116|117|119|190|192|211|214)_'`.
  CLI passed; the store fixture failure and repair are recorded above.
- All requested documentation targets passed: `spec-requirements-check`,
  `requirement-definitions-check`, `traceability-tests-check`, `decision-numbers-check`,
  `line-citations-check`, `error-code-ownership-check`, `unbounded-readers-check`,
  `use-case-receipts-check` and `use-case-receipts-test`. The last target ran without the temporary
  Git environment, so its fixture commits remained isolated.
- `spec-requirements` ran after staging the spec in the temporary index. After wrapping the
  proposal text, the generator and affected spec/definition/traceability checks passed again.
  The generated index adds only CAL-V0-214; existing requirement text is unchanged.
- Receipt fixture tests passed all five cases; explicit
  `sh script/repin-use-case-receipts.sh --check` passed with all 18 receipts current.
- Executable final results are retained under `/private/tmp/claude-501/gotmp/v1-1125-*`;
  `go test -count=1 -race -timeout 30m ./internal/tasks/store -run '^TestCALV0214_'`
  passed after the fixture/import repair (3.766s, status 0). The earlier expanded store run is
  retained as failed; it was not repeated in full for this test-only initializer correction.

## Corvint use and limits

Used the supplied context packet `/private/tmp/claude-501/gotmp/ctxA.txt`, then actual `query`
and tracked-path `impact` at base `71b2eb8fc3e56a9df368b8be8227bf5ad125f8c9`; retained outputs
are `/private/tmp/claude-501/gotmp/v1-1125-query.json` and `v1-1125-impact.json` in that directory.
Query retained omitted ranked results and 10 test-path symbol candidates without an import
relation; impact omitted 167 ranked results. Neither receipt proves exhaustive behavior.
`affected --base` was used before baseline testing and again for the final source diff;
`/private/tmp/claude-501/gotmp/v1-1125-affected.json` selects 60 units
with scope UNKNOWN, 328 units excluded for no dependency path, and retained language-frontier
and unowned-doc-path unknowns. Repository-wide `make gate` and `./...` checks are NOT_RUN under
the owner's explicit scoped-test instruction; this is not equivalent to exhaustive coverage.

Re-review of the implementation repair reported no findings. The reviewer did not run tests; executable results
are builder observations.

Installed Core is `1.0.0-rc.3 (build 407)` and standalone Tasks is
`0.0.0-tcp01-unverified+build.407`. Core and Tasks `corvint-update check --allow-network --component core|tasks` checks failed their
version smoke
with `process-after-start-failed: descendant snapshot unavailable`; official freshness is UNKNOWN,
publisher identity NOT_VERIFIED, and no update was installed. Index refresh failed because the
shared Git metadata's snapshot directory is outside the writable sandbox. No verification failure
was bypassed.

The shared Git index is also read-only. The changed spec is staged in the private temporary index
`/private/tmp/claude-501/gotmp/v1-1125-git-index`, with private objects and read-only alternates;
`make -s spec-requirements` runs against that staged content. GNU mktemp on the existing PATH
is used for gates because macOS mktemp's default system temporary directory is not writable here.
No repository scripts were changed for these environment limits. An initial race run accidentally
inherited the temporary Git variables into fixture commands, causing missing warnings, and was
stopped; its paired doc-gate run was interrupted at error-code ownership (exit 143). The temporary
index was rebuilt from the untouched shared index, all task changes were restaged, and corrected
runs separate staging variables from fixture/test environments. These invalidated checks are not
counted as acceptance evidence.

Committed CEM/OCM binding, dogfood check/seal, live-store qualification, publication and native
ticket completion are NOT_PRODUCED/NOT_RUN: the owner explicitly requested an uncommitted builder
patch and no `.taskman` writes. The task store is untouched, including friction filing. Frozen
retrieval/learning evaluations and process-lifecycle qualification are not applicable to this
advisory CLI extension. Owner acceptance is decision 0513; integration remained outside this builder result.
