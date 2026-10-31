# Incremental forward-row listing — V1-3091

Problem: the scheduled verdict (`verdict.sh`, every 6 h) and every pull request's
narrowing-verdict acceptance (`accept.sh`) listed the repository's whole artifact store, newest
first, 100 per page, to find the forward rows. Measured read-only on 2026-10-30: 733 artifacts
(8 pages) growing at about 40 per hour; `accept.sh` had a 25-page budget, so it would answer FULL
once the store passed 2,500 artifacts. Two alternatives were measured and rejected: the
`?name=` filter of the artifacts API matches a whole name only, so it cannot select a
`ci-forward-*` prefix; listing per workflow run cost about 41 requests where the whole store
cost 8 pages.

Design (CIN-V0-003, CIN-V0-004): `list_names` takes a stop time and ends once a listed artifact is
older. The verdict stops at `FORWARD_EPOCH_SINCE`, no later than the merge that advanced `main` to
the epoch, and counts reaching it as covering the epoch. It also records `listedAt` and the
AFP-V0-030 window's row names; the acceptance lists back to one hour before `listedAt` and re-checks
the kill switch and `MISS` over those rows followed by the window rows, with a re-run row replacing
its older row. AFP-V0-097 semantics are unchanged; a verdict without both members, a malformed
member, an unreadable page or a listing that does not reach the stop time keeps the old path or
answers FULL with the limit named.

Request counts: on the replay fixture of 30 pages (past the old budget) the whole listing reads 31
pages and the incremental one 2, with the same MET verdict. Live, with a verdict at most 6 h old,
the acceptance reads at most about 7 h of artifacts (about 280, 3 pages) instead of the whole
store (8 pages today, unbounded with growth up to the 25-page FULL); the verdict reads only
epoch-2 artifacts instead of up to 400 pages.

Evidence: `TestCINV0003VerdictListsBackToTheEpochSince` and the `CIN-V0-004` cases of
`TestAFPV0097VerdictAcceptance` (`tools/ci-forward-row`, PASS; whole package PASS).
