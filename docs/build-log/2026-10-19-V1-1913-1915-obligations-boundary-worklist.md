# V1-1913 and V1-1915: obligation test boundaries, scope decisions and the shortfall worklist

Date: 2026-10-19
Tickets: V1-1913 (GitHub issue beamfall/corvint#1180) and V1-1915 (beamfall/corvint#1182).
Requirements: TOL-V0-079..083 and TOL-V0-084..088 (`docs/specs/corvint-tasks-obligation-ledger-v0.md`).
They are accepted under the owner's standing auto-accept of lane requirements (2026-10-11). No
accepted requirement is amended.

## Decisions

- **Reuse, no new profile.** `cites`, `scope` and `candidate` are optional OBLIGATIONS_SET change and
  entry members, and `finding` is an optional verdict member. A ledger that never records one keeps
  its exact bytes. `boundary` and `worklist` are reads.
- **No citations is UNKNOWN, not NO_TEST_BOUNDARY.** The issue asks for NO-TEST-BOUNDARY when an
  obligation "cites no tracked code at all". An entry that records cites, none of them tracked, is
  `NO_TEST_BOUNDARY/NO_TRACKED_CODE`. An entry with no `cites` member is `UNKNOWN/NO_CITATIONS`,
  because missing evidence yields uncertainty (product invariant 2). An unlisted repository is
  likewise `UNKNOWN/REPOSITORY_UNAVAILABLE`.
- **Owner-gated scope decision.** PROPOSE records who, when, why and an evidence reference and
  leaves `core` unchanged. ACCEPT needs a pending proposal for the same core value, the OWNER role
  and an actor id other than the proposer's. Only an accepted decision moves `core`, so the existing
  counts (`coreTotal`, `coreWitnessed`) exclude the entry with no new counting path. `show` lists
  `outOfScope` and `scopeProposals`. Under the default §3.2 role matrix only OWNER may issue
  OBLIGATIONS_SET at all, so the separation that matters in practice is the distinct actor id; the
  explicit OWNER-only ACCEPT check is defence in depth for a future role grant.
- **Plain core changes stay admitted by default.** Refusing every plain `core` change would amend
  the accepted TOL-V0-007/015. It ships as the opt-in policy key `obligations.requireScopeDecision`
  (TOL-V0-082). Making it the default is an owner decision.
- **Re-scope candidates.** `boundary` lists an entry that is out of scope by an accepted decision
  and now TESTABLE.
- **Worklist.** Only pending entries whose latest review is HELD are considered. In order: a
  `NO_FALSIFIABLE_TEXT` finding goes to `needsObligationText`; then a candidate, BLOCKED state, an
  unreadable test repository or a changed subject blob excludes the entry with a reason; the rest
  are ranked items. The default rank is priority, then nearest completion (fewest pending core
  entries). `--next K` batches by test repository and path.
- **Candidate lifetime.** A recorded candidate is cleared by any later REVIEWED witness (CREDITED or
  HELD) or credit. A HELD entry can already be re-reviewed on the same blob (TOL-V0-076 refuses only
  a same-blob re-review of a REVIEWED-witnessed entry), so recording a candidate never blocks the
  next review.
- **Stub-risk notes.** These list every reviewer whose risk is not `none`, including reviewers
  whose verdict was `correct`, because an on-path stub is the note a worker needs.

## Evidence

- The branch is stacked on #1189: BASE is `2326a3bfe`, the merge of #1189's head `9378caa0e` into
  `origin/main` `714393264`. The worklist reads each review's test repository from #1189's
  `review.testRepository` record member, the same source `show` uses.
- Focused tests, through heavy.sh, one package per command, with `-race`:
  - `internal/tasks/obligation`: ok, 1.2s (load averages 36.60 36.83 36.17).
  - `internal/tasks/ticket`: ok (36.60 36.83 36.17).
  - `internal/tasks/mutation`: ok (34.71 36.44 36.03).
  - `internal/tasks/intent`: ok, 56.9s (39.51 37.36 36.36).
  - `internal/tasks/cli`, the whole package: ok, 901s (33.19 35.95 35.91 before, 33.63 28.99 30.34
    after). This ran on the code before the Codex fixes.
  - After the Codex fixes: `obligation` ok (35.39 28.91 30.36), `mutation` ok (31.89 29.24 30.38),
    and `cli -run 'TestTOLV00(7|8)'` ok, 15.7s (33.74 30.83 30.94).
  - BASE has none of the new tests, so the BASE count is 0.
- 20 mutations, each run against its own test, and all 20 bite. Load averages were 22.62 28.27 29.98
  before and 25.02 26.24 28.51 after.
  - The 16 from the first build: BoundaryVerdict's tests-present check, re-scope verdict,
    JavaScript test classifier, self-accept, no-proposal, accept sets core, propose keeps core,
    requireScopeDecision, worklist candidate exclusion, candidate cleared on review, needs-text,
    finding beside a correct verdict, `--next` batch key, nearest rank, unknown rank key, stale
    subject.
  - 4 from the Codex fixes: the trailing-slash cite, the unborn test-repository `HEAD`, the empty
    finding, and the OWNER-only ACCEPT guard.
- The script is a scratch file and is not retained.

## Codex review (one round)

There was one `codex exec -s read-only` round over `2326a3bfe..653a48d62`. It found no legacy-byte
regression and no authority or self-acceptance bypass. All four findings are fixed:

1. **P2: a directory cite ending in `/` read as `NO_TRACKED_CODE`.** Fixed: cites are matched
   without the trailing `/` (`boundaryCitePath`). The spec says a trailing `/` names the same
   directory.
2. **P2: an unborn `HEAD` in a test repository read as `SUBJECT_CHANGED`.** `FilesAtCommit` returns
   an empty commit and no error. Fixed: the worklist treats it as `SUBJECT_UNVERIFIED`, and TOL-V0-086
   says so. Test: `TestTOLV0086_WorklistTestRepositoryUnverified`.
3. **P3: `"finding":""` passed the CLI and was dropped at serialisation.** Fixed: the CLI reads
   `finding` as the closed enum. The empty case is now tested.
4. **P2: the CLI test could not reach the OWNER-only ACCEPT guard,** because the default role
   matrix already refuses OPERATOR. A policy can only remove rows, so no fixture can grant it.
   Fixed: an internal test, `TestTOLV0081_ScopeAcceptOwnerOnly`, calls `obligationAuthority`
   directly.

## Residuals

- Actor ids and roles are recorded claims (decision 0003). One operator acting under two ids is not
  detected.
- The runnable-test classifier is a closed naming heuristic. A matching file that no runner
  executes still counts, and a provider outside the set does not.
- Cite repository labels are not validated at write. An undeclared label reads UNKNOWN.
- The boundary and staleness reads look at each repository's HEAD, not at a reviewed commit.
- Rank keys are fixed (priority, nearest, age). There is no per-policy default.
- #1189's `witnessReviewValue` reads an unborn `HEAD` in a declared test repository the same way:
  as a resolved repository whose subject is absent. So it reports `stale: true` rather than
  `TEST_REPOSITORY_UNRESOLVED`. The credit is withheld either way. This is #1189's code and is left
  for that change.
