# 2026-10-14 The cem-verdict gate for a merged-forward lane, and directory touch keys (V1-1356)

Branch `claude/v1-1356-cem-verdict-merge-forward` from `origin/main`
adf587fdb65f4d87dc94a5e374a4fda1f87c5e7f, in a plain clone. Ticket V1-1356 amends the sealed
cem/1.0 verdict gate contract (`TCB-V0-007..019`, PRs #858 and #863). Requirements `TCB-V0-020..023`
in `docs/specs/tasks-criterion-binding-v0.md` are accepted under the owner's standing auto-accept
(2026-10-11). The rest of that proposed contract is not thereby accepted or promoted. The stable 1.0
spec-driven development scope is `PRS-V1-018`; V1-1356 is not one of its five clauses and this lane
does not advance them.

## Intent

V1-1348's first native completion through `cem-verdict` ran the gate on the merge commit and was
refused `sealed-tree-mismatch` (receipt 000000005362), because `main` had moved during the lane. The
contract did not say which commit a merged-forward lane is gated and completed at. A second gap:
a touch key without a trailing `/` that names a directory governed only that exact path at the gate,
so a claim of `src` silently governed nothing under `src/`.

## Decision: the candidate is the seal commit (TCB-V0-020..022)

The lane claims at the base its sealed map names, submits the seal commit's tree, runs the gate with
the seal commit as the worktree HEAD, and completes with `complete --commit <seal>` once `main`
contains it. Under the merge queue's merge-commit method (AFP-V0-016) the seal commit is reachable
from `main` through the merge's second parent, so CAL-V0-017 (`git merge-base --is-ancestor` plus
`CommitTree == CandidateTreeOid`) accepts it. No Tasks code changes: TCB-V0-021 is pinned by a test
of the existing rule. `sealed-tree-mismatch` stays the refusal for any altered tree, which covers the
merge commit and every post-seal commit that changes the tree. Both checks bind trees, not commit
IDs: a later commit with the seal's tree (an empty commit, or a change reverted to the sealed tree)
presents the same sealed content and is treated as the seal commit.

Limit. A GRANT proves the sealed lane change, not the merge result. A post-seal commit or a merge
resolution that alters lane content is not seen by the gate, and Tasks does not refuse completion at
the seal because of it. Integration effects belong to merge-queue CI
(`docs/build-log/2026-10-12-merge-queue-enabled.md`).

Rejected alternatives:

- Merge-aware rule (accept a merge commit by reconstructing the sealed tree from its parents).
  Rejected: it widens the trusted surface to merge reconstruction and conflict resolution, which the
  sealed map does not bind.
- Quiescent-`main` requirement (refuse unless `main` is still the lane base). Rejected: unworkable
  under the merge queue; every lane integrated on 2026-10-13 had a moved `main`.

## Decision: directory keys refuse, not normalize (TCB-V0-023)

Under `touched-anchored`, a key without a trailing `/` that names a directory at the base or the
candidate refuses `CEM_VERDICT_INPUT_INVALID` with reason `touch-key-names-directory`, after the
inputs and candidate tree are pinned and before the map is read. Both trees are listed with
`git ls-tree -r -d -z --full-tree`; a failed listing, or one that is not a sequence of
NUL-terminated `<mode> tree <oid>\t<path>` records, refuses
`CEM_VERDICT_UNVERIFIABLE`/`git-read-failed`. Keys under `all-anchored` govern nothing and are not
checked. Normalizing to a prefix key was rejected: it would give the gate a key meaning Tasks does not
share, since CAL-V0-021 makes such a key exact for claims and resource collisions. CAL-V0-211's
`EXACT_KEY_NAMES_DIRECTORY` warning at set-effects and claim time stays advisory and unchanged.

## Criterion 2: the real native completion (cited, not repeated)

V1-1348 completed natively through this rule, recorded in
`docs/build-log/2026-10-14-v1-1283-native-completion-through-cem-verdict.md`: claim at lane base
d97f00b64; gate at the merge 5163d1ed4 refused `sealed-tree-mismatch` (receipt 000000005362); gate at
seal 460fc31ae GRANTed (receipt 000000005368); `complete --commit 460fc31ae` returned OK at
2026-10-14T00:11:15Z; `receipt audit` at head sequence 5369 reported CONSISTENT. This lane creates no
other completion; the orchestrator completes V1-1356.

## Failure modes

| Code | Reason | Cause |
| --- | --- | --- |
| `CEM_VERDICT_PRODUCER_MISMATCH` | `sealed-tree-mismatch` | Candidate is a merge or a post-seal commit that changes the tree (unchanged rule) |
| `CEM_VERDICT_INPUT_INVALID` | `touch-key-names-directory` | A `touched-anchored` key without `/` names a directory at base or candidate |
| `CEM_VERDICT_UNVERIFIABLE` | `git-read-failed` | The directory listing of base or candidate failed or was malformed |
| Tasks `STALE_TREE` | | `complete --commit` names the merge or a post-seal commit that changes the tree |
| Tasks `STALE_TREE` + `COMMIT_NOT_INTEGRATED` | | `complete --commit <seal>` before `main` contains the seal |

## Acceptance evidence

- `TestTCBV0020GateGrantsOnlyAtTheSealCommitOfAMergedForwardLane` (`internal/taskman`): over a moved
  `main`, the `--no-ff` merge and a post-seal forward merge refuse `sealed-tree-mismatch` naming the
  lane's sealed map with no verdict; the seal, the merge's second-parent ancestor, grants, and so
  does a tree-identical empty commit after it.
- `TestTCBV0021_CompletePinsTheSealCommitOfAMergedForwardLane` (`internal/tasks/store`): the seal
  refuses not-integrated before the merge; the merge and forward commits refuse `STALE_TREE` alone
  and the store digest is unchanged; the seal then completes and the receipt audit passes.
- `TestTCBV0023ExactKeyNamingADirectoryRefuses` and the amended `TestTCBV0011…`: directory keys at
  both, base-only and candidate-only refuse pinned before the map; file and absent keys grant;
  `all-anchored` is unaffected; an unlistable candidate refuses `git-read-failed`, and the same
  candidate without an exact key reads on to `CEM_VERDICT_MAP_MISSING`.
  `TestTCBV0023DirectoryListingFramingFailsClosed` refuses unterminated, empty-record, empty-path,
  non-tree and tab-less listings.

## Retained limits and NOT_PRODUCED

- No new live native completion: NOT_PRODUCED by design (criterion 2 cites V1-1348).
- The gate does not prove the merge result; merge-queue CI owns integration effects.
- V1-1359 (separating the predicate domain from claim scope) is not taken on. It may change which
  keys reach `--touch-paths`; TCB-V0-023 applies to whatever keys arrive.

## Independent review

One Codex read-only round over the pre-review commit 72c22ca42 (amended; log kept outside the repository) reported
two findings, both fixed in this change:

- Medium: the spec and build log said every post-seal commit refuses, but the gate and completion
  bind trees, so a tree-identical later commit grants and completes. Fixed by stating the tree
  binding in TCB-V0-020..022 and pinning it with an empty-commit case in
  `TestTCBV0020…`. Commit-ID binding was not added: it would not change what is proven, since the
  tree is the sealed content.
- Low: the directory listing parser accepted an unterminated listing and empty records. Fixed by
  requiring NUL termination and a nonempty path, with `TestTCBV0023DirectoryListingFramingFailsClosed`.

## Rollback

Delete `exactKeyNamesDirectory` and its call in `internal/taskman/cem_verdict_gate.go`, the two new
test files, the `touch-key-names-directory` expectation in `TestTCBV0011…`, and `TCB-V0-020..023`
with their spec rows. No Tasks code, schema, wire contract or CEM byte changed.
