# 2026-10-16: Orchestrator dispositions (decision 0534), V1-1480 and V1-1490

## Intent

On 2026-10-16 the owner delegated the remaining open owner decisions to the orchestrator. Decision
0534 records the orchestrator's dispositions. This change also delivers the two documentation and
gate items those dispositions settle: V1-1480 and V1-1490 option (a).

## V1-1480: release notes join the citation gate's document set

`scanned_doc` in `script/check-line-citations.sh` now also matches `docs/RELEASE-NOTES-[^/]+.md`,
and the spec's `DCG-V0-001` lists `docs/RELEASE-NOTES-*.md`. The one tracked release-notes file,
`docs/RELEASE-NOTES-alpha.md`, holds a single citation, and it is pinned. The file is not on the
legacy allowlist, so every future citation in release notes needs an anchor. The unpinned count is
unchanged.

Mutation evidence (index-resolved, run locally): with one line prepended to
`docs/CHANGE-EVIDENCE-MAP.md` and staged, the amended gate exited 2. It reported
`docs/RELEASE-NOTES-alpha.md:127` "cited content changed: anchor @012d2dcc, now @90ff4281". The base
checker exited 0 on the same index. With the file restored, the gate exited 0. The new fixture case
23 in `script/check-line-citations_test.sh` covers the same behavior: it passes with the amended
checker and fails with the base checker ("the release-notes drift was not reported").

## V1-1490: the enrolled workflow ends at the bind commit

`docs/DOGFOOD.md`, "Enrolled local completion policy", now says the enrollment ends once `finish`
is satisfied at the bind commit. After `make dogfood-seal`, the `verify` that `status` offers must
not be run, and the next enrollment begins in a fresh worktree or clone. The documentation change
amends no requirement.

## Not done here

V1-1469 (another lane owns `.github/`), and the implementation of V1-1475, V1-0557, V1-0650 and
V1-1315. `make gate` was `NOT_RUN`.
