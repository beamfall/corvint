# Corvint Tasks source containment V0

Owner: Russell Lewis
Date: 2026-10-19
Intent status: accepted under the owner's standing auto-accept of lane requirements (2026-10-11; V1-1920, V1-1907; GitHub #1186, #1177; SCC-V0-001..005 and SCC-V0-010..014)
Delivery status: experimental
Requirement prefix: SCC-V0
Authoritative inputs: owner issues [#1186](https://github.com/beamfall/corvint/issues/1186)
(patch-equivalent containment) and [#1177](https://github.com/beamfall/corvint/issues/1177)
(machine-checkable blocker conditions); the pure-read rule TM-V0-008; the HOLD and RELEASE
operations of [the agent leases contract](corvint-tasks-agent-leases-v0.md); the advisory
doctor of [the doctor contract](corvint-tasks-doctor-v0.md). No accepted requirement is amended
here. The build record is `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-19-containment-blockers-1920-1907-1771.md`.

## Agent digest
- Claim: `source contains` reports ancestor or patch-equivalent containment; a HOLD may carry a typed condition that status and doctor re-evaluate at each read.
- Status: accepted under the owner's standing auto-accept of lane requirements (2026-10-11; V1-1920, V1-1907; GitHub #1186, #1177; SCC-V0-001..005 and SCC-V0-010..014); experimental source implementation with focused store, mutation and CLI tests.
- Exists: `store.SourceContainmentOf` (ANCESTOR, EQUIVALENT, ABSENT, UNKNOWN); the read-only `source contains <rev> <commit>...` verb; the optional hold `condition` (COMMIT_CONTAINED, PATH_EXISTS, SYMBOL_DEFINED); queue status `holdConditions`; doctor HOLD_CONDITION_MET, HOLD_CONDITION_UNKNOWN and FREE_TEXT_HOLD_MET.
- Blocked on: #1177 criterion 4 (stale base pins in role prompts and operator notes), policy routing of an unblocked ticket, and a configurable deploy ref (an owner decision on TQD-V0-020/TQD-V0-001) are not delivered.
- Read next: Requirements; Containment-site inventory; Gaps and owner decision; Traceability.

## Intent and boundary

A ticket blocked on "commit X must reach main" stays blocked when X lands rebased or
cherry-picked, because a strict ancestry test cannot see a commit whose id changed. An
operator then releases the hold by hand, or worse, never notices it is met. The simpler
baseline is `git cherry` by hand for each blocker; it is unbounded, not recorded on the hold
and never re-run.

Containment here is a pure Git read. It never fetches, never writes a ref, the store, the
journal or a trace, and never releases a hold: a met condition is reported, and the operator or
a later policy acts on it. Every answer that cannot be observed (a missing ref or object,
shallow history, a failed read, an exceeded bound) is UNKNOWN, never ABSENT or met.

## Requirements

- `SCC-V0-001`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1920) `store.SourceContainmentOf(root, source, commit)` MUST answer ANCESTOR when `commit` is an ancestor of `source`; otherwise EQUIVALENT, naming the source commit, when a non-merge commit of `source` outside `commit`'s history has `commit`'s patch-id (Git's `--cherry-mark`, the `git cherry` rule); otherwise ABSENT or UNKNOWN per SCC-V0-002..004. A rebased or cherry-picked commit whose diff is unchanged is therefore EQUIVALENT.
- `SCC-V0-002`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1920) An ABSENT answer MUST carry a reason: `PATCH_DIFFERS` naming a source commit with `commit`'s author email and subject, or its `Change-Id` trailer, whose patch differs (a conflict-resolved rebase); `NO_EQUIVALENT` when no such commit exists; `MERGE_COMMIT` for a merge commit that is not an ancestor, since a merge has no patch-id and only ancestry can contain it.
- `SCC-V0-003`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1920) The patch-id scan MUST be bounded by `SourceContainmentScanCommits` (10000) non-merge source commits outside `commit`'s history, a 64 KiB record line and one 60-second deadline that covers every Git command of the answer (resolution, ancestry, depth, parents, count and log). The read MUST NOT start the repository's signature verifier (`log.showSignature`), hooks or file-system monitor. Exceeding the commit bound MUST answer UNKNOWN `SCAN_BOUND` naming the count and the bound; an over-long or malformed record, an exceeded deadline (named in the detail, never reported as a missing ref) or a failed Git read MUST answer UNKNOWN `GIT_FAILED`.
- `SCC-V0-004`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1920) A source or commit that does not resolve to a commit MUST answer UNKNOWN `MISSING_SOURCE` or `MISSING_COMMIT`; a shallow repository, or one whose depth is unobservable, MUST answer UNKNOWN `SHALLOW_HISTORY` once ancestry fails. Containment never fetches or writes a ref.
- `SCC-V0-005`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1920) `corvint-tasks source contains <rev> <commit>...` MUST be a read verb that opens no store and writes nothing, answering one item per commit with `source`, `commit`, `state` and, when present, `sourceResolved`, `commitResolved`, `equivalent`, `patchDiffers`, `reason` and `detail`. It MUST refuse more than 64 commits with LIMIT_EXCEEDED before any read, and refuse flags, empty arguments and arguments holding NUL, CR or LF as usage errors.
- `SCC-V0-010`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1907) A HOLD payload and a stored hold MAY carry one closed `condition` object, beside the free-text `reason` that stays required: `{"kind":"COMMIT_CONTAINED","ref","commit"}` (a full commit object id), `{"kind":"PATH_EXISTS","ref","path"}` or `{"kind":"SYMBOL_DEFINED","ref","path","symbol"}` (a file path; an identifier-shaped symbol of at most 128 bytes). `ref` MUST be a full refname under `refs/` of at most 256 bytes obeying Git's refname rules (no component is empty, starts with `.` or ends with `.lock`). An unknown kind, an extra or missing key or an invalid member MUST be refused by payload canonicalisation and envelope decode. A hold without a condition encodes exactly as before; `adopt` carries a file hold's condition and refuses a condition-only edit of an existing hold as ADOPT_UNSUPPORTED_FIELD.
- `SCC-V0-011`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1907) `store.EvaluateHoldCondition` MUST evaluate a condition at its ref's current commit: COMMIT_CONTAINED is UNBLOCKED for an ANCESTOR or EQUIVALENT answer, BLOCKED for ABSENT and UNKNOWN otherwise; PATH_EXISTS is UNBLOCKED or BLOCKED by the path's presence at that commit, and a path ending in `/` is present only as a directory; any unresolvable ref, object or path read is UNKNOWN with a reason naming it. An UNBLOCKED detail MUST read `condition met at <ref> <commit>`.
- `SCC-V0-012`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1907) SYMBOL_DEFINED MUST be UNBLOCKED only when the file at the ref's commit holds a line-leading declaration of the symbol (a bounded keyword heuristic covering Go, Rust, Python, Ruby, JavaScript/TypeScript, Java/Kotlin/C#-style and shell-function forms), BLOCKED when the text file lacks one or the path is absent or not a file, and UNKNOWN for a file over `HoldConditionMaxBlobBytes` (1 MiB), a binary file or a failed read. The heuristic is advisory evidence, not a parser.
- `SCC-V0-013`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1907) `queue status` MUST re-evaluate every typed condition of every HELD ticket at each read and report it in `holdConditions` (`ticketId`, `holdId`, `condition`, `state`, `refCommit` when resolved, `containment` for COMMIT_CONTAINED, `detail`), omitting the member when no typed hold exists. `doctor` MUST report HOLD_CONDITION_MET for UNBLOCKED and HOLD_CONDITION_UNKNOWN for UNKNOWN conditions. Neither read releases, edits or re-routes the hold. One read evaluates at most 64 conditions; the rest are UNKNOWN `not evaluated`.
- `SCC-V0-014`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1907) `doctor` MUST report FREE_TEXT_HOLD_MET for a HELD ticket's hold without a condition whose reason names a commit (7..64 lowercase hex characters holding a digit) contained, by SCC-V0-001, in `refs/remotes/origin/HEAD`, or a relative path existing at it, with a detail naming that ref and commit. URLs, `refs/` names and `..` paths are not candidates; each hold contributes at most 8 candidates and one read evaluates at most 128, warning how many it left. An unresolvable `refs/remotes/origin/HEAD` MUST yield no finding and one NOT_OBSERVED warning when some candidate exists; an unmet or unknown candidate yields nothing.

## Containment-site inventory

Every pre-existing ancestry test (`merge-base --is-ancestor`, `isAncestor`) found with
`git grep` at BASE `714393264114757adf858187dc0d5913e9830c57` was reviewed. None adopts
EQUIVALENT in this change; each is strict for the reason named.

| Site | Reason it stays strict |
| --- | --- |
| `internal/tasks/store/lease_gate.go` `intentIntegrated`, `repositoryIntegrated` (CAL-V0-017) | Completion binds the exact candidate tree and commit the gates ran on; a patch-equivalent commit is a different tree once neighbours differ, so equivalence would complete a ticket on evidence that never ran. |
| `internal/tasks/store/ticket_worktree.go` (TWT-V0-004) | Attaching an existing branch requires the declared base in the branch's history so the worktree's diff is relative to that base; an equivalent commit leaves the base's own history absent. |
| PSR-V0-081 pool reuse | An exact commit pair check, not ancestry; equivalence does not apply. |
| `cmd/corvint/runs_baseline.go`, `internal/appflows/repositories.go`, `internal/appflows/selection.go` | Baseline and selection descent order runs and repositories by history; a rebased commit has a different position. |
| `internal/dogfoodflow/{batch,check,finishlane,migration}.go` | CEM binding and seal checks bind exact commits and ranges (invariant 1). |
| `internal/extevidence/freshness.go`, `internal/witness/witness.go`, `internal/tracerecordrepo/adapter.go` | Evidence freshness is pinned to immutable content; equivalence would carry evidence across a changed tree. |
| `internal/localcompletion/storage.go`, `internal/upgradechanges/changes.go`, `internal/sharedpath/split.go` | Stored records and change ranges are history descents over exact commits. |
| `internal/postmergeconnector/git.go`, `internal/postmergemetrics/git.go` | Post-merge metrics attribute merged history; the merge itself is the event measured. |
| `internal/dashboard/repository/trace.go`, `conformance/dashboard-snapshot-v0/git_authority.go`, `conformance/release-artifact-v0/publication_status.go`, `tools/corvint-pr-tests/main.go` | Authority and publication checks require the exact published commit. |

New callers that ask "has this change landed" rather than "is this exact commit in history"
use `store.SourceContainmentOf`; the HOLD condition (SCC-V0-011) is the first.

## Failure modes

| Failure | Behaviour |
| --- | --- |
| Source or commit ref missing | UNKNOWN `MISSING_SOURCE`/`MISSING_COMMIT`; a hold condition is UNKNOWN, never met. |
| Shallow clone | UNKNOWN `SHALLOW_HISTORY` once ancestry fails. |
| Conflict-resolved rebase | ABSENT `PATCH_DIFFERS` naming the look-alike commit, so the operator can judge it. |
| Merge commit not in history | ABSENT `MERGE_COMMIT`. |
| More than 10000 source commits to scan | UNKNOWN `SCAN_BOUND`. |
| Git slow, failed or malformed output | UNKNOWN `GIT_FAILED` after at most 60 seconds. |
| More than 64 typed holds | The rest UNKNOWN `not evaluated`. |
| `refs/remotes/origin/HEAD` absent | No free-text findings; one NOT_OBSERVED warning. |
| Older binary reads a hold with `condition` | The closed hold decode refuses the record; see rollback. |

## Non-goals

- No fetch, no network and no ref writes; a stale local ref yields a stale answer.
- No automatic release, unpark or re-routing of a held ticket (reads never mutate, TM-V0-008).
- No language-aware symbol resolution; SCC-V0-012 is a declaration-line heuristic.
- No change to the strict sites inventoried above.

## Gaps and owner decision

- #1177 criterion 4 (report role prompts and operator notes pinning a base B once the deploy
  ref strictly descends from B) is not delivered.
- #1177 criterion 2's routing "to the work stage or operator per policy" is not delivered:
  status and doctor surface UNBLOCKED, and the hold stays until released.
- Owner decision: the deploy ref for free-text holds is fixed to `refs/remotes/origin/HEAD`.
  Making it configurable needs a doctor config member or argument, which amends the accepted
  TQD-V0-020 closed config key set or the TQD-V0-001 argument closure; that amendment is not
  covered by the standing auto-accept and is left to the owner.

## Acceptance evidence

Focused tests with `-race` in `internal/tasks/store`, `internal/tasks/mutation` and
`internal/tasks/cli` use real `git init` repositories in `t.TempDir`; each behaviour has a
recorded mutation that the tests kill (build log). The scenarios are rebase, cherry-pick,
conflict-resolved rebase, merge-only, bound-exceeded, missing refs and shallow history.

## Compatibility and rollback

The `condition` member is additive and optional; stores without it are byte-identical. Revert
the change to roll back. A store that already holds a hold with `condition` is refused by an
older binary's closed hold decode: release or re-place those holds without a condition before
downgrading. The `source contains` verb and the doctor kinds are read-only and need no
migration.

## Traceability

| Requirement | Evidence |
| --- | --- |
| SCC-V0-001 | `TestSCCV0001Ancestor`, `TestSCCV0001CherryPickIsEquivalent`, `TestSCCV0001RebaseIsEquivalent` |
| SCC-V0-002 | `TestSCCV0002ConflictResolvedRebaseIsPatchDiffers`, `TestSCCV0002PlainAbsent`, `TestSCCV0002ChangeIDMatchesPatchDiffers`, `TestSCCV0002MergeCommitIsAbsentOnlyByAncestry` |
| SCC-V0-003 | `TestSCCV0003ScanBoundIsUnknown`, `TestSCCV0003DeadlineIsUnknown`, `TestSCCV0003DeadlineCoversEveryCommand`, `TestSCCV0003SignedCommitStartsNoVerifier` |
| SCC-V0-004 | `TestSCCV0004MissingRefsAbstain`, `TestSCCV0004ShallowHistoryAbstains` |
| SCC-V0-005 | `TestSCCV0005_SourceContainsVerb` |
| SCC-V0-010 | `TestSCCV0010_HoldConditionRoundTrip`, `TestSCCV0010_HoldConditionRefusals`, `TestSCCV0010_AdoptCarriesHoldCondition` |
| SCC-V0-011 | `TestSCCV0011HoldConditionEvaluation` |
| SCC-V0-012 | `TestSCCV0011HoldConditionEvaluation`, `TestSCCV0011SymbolBinaryAndOversizeAbstain` |
| SCC-V0-013 | `TestSCCV0013_StatusReevaluatesHoldConditions`, `TestSCCV0013_EvaluationBound` |
| SCC-V0-014 | `TestSCCV0014_DoctorFreeTextHolds`, `TestSCCV0014_FreeTextCandidates`, `TestSCCV0014_FreeTextCandidateBound` |
