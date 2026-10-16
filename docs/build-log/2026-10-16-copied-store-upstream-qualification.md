# 2026-10-16 Copied-store qualification of upstream completion (V1-1459), V1-0557 closure evidence, V1-1059 repair text

## V1-1459: CAL-V0-017 upstream reachability on a copied store

Runtime: `corvint-tasks version` = `0.0.0-tcp01-unverified+build.580` (goVersion `go1.27.2`,
`verification: NOT_RUN`), from `PATH` (`~/.local/bin/corvint-tasks`). Every `corvint-tasks` command
below ran with its working directory inside the scratch copy; none ran against the live store.

### Setup

- Copy: a fresh `git clone https://github.com/beamfall/corvint.git` in a private scratch directory.
  `refs/remotes/origin/main` = `c4a7522224bb83ae47d2e544eb03fa73b2af39fd` (merge of PR #964).
  Local `refs/heads/main` was reset to `bd1bc85db2f6d364fb999fd62d34cc81de58cc04` (merge of PR #965,
  `origin/main~2`), so `c4a75222` is not reachable from `refs/heads/main`
  (`git merge-base --is-ancestor` exit 1). It is reachable from `refs/remotes/origin/main` and from
  other remote-tracking refs only. `branch.main.remote = origin`, `branch.main.merge = refs/heads/main`.
- Store, first attempt (full copy): the live `.git/taskman` (head seq 6628, `head.json` sha256
  `a43269df…fb7ad`) and the live `.taskman` projection were copied read-only with `/bin/cp -R`. The copy
  was byte-identical (`diff -rq` empty). `corvint-tasks queue status` in the copy resolved the copy's
  state dir, but it refused `UNSUPPORTED_FILESYSTEM`: `head.primaryWorktree` names the live checkout
  path and "repository relocation is unsupported in this preview". A copied full store is therefore
  **unsupported** by this runtime. That copy was set aside unused.
- Store, used: a fresh store from `corvint-tasks init --role OWNER --request-id copy-init-1` over the
  copy's committed `.taskman/queue.json` and `.taskman/policy.json` at `bd1bc85db` (tickets and
  releases removed from the working tree). The live policy's gates, including `focused-docs`, were
  used unchanged. Init reported `stateDir` and `primaryWorktree` inside the copy. This is the
  "store initialized from the repository" fallback, not a migrated copy of the live journal.
- A detached `git worktree add --detach ../cand c4a75222…` for the gate (no ref created).

### Run (copy only)

| Step | Command | Result |
| --- | --- | --- |
| create | `ticket create --request-id copy-create-1` (kind DOC, `requiredGates: [focused-docs]`) | OK, `V1-0849` (copy-local ID), receipt 2 |
| claim | `claim V1-0849 --holder copy-qual --base 9d57fd1cd1ded936f47e1ea09077ecd2a9ab41d6 --lease-minutes 180` | OK, generation 1, receipt 3 |
| submit | `submit --tree 9d5a4935bdc0c2eadade14de0d7df57896021927` (tree of `c4a75222`) | OK, receipt 4 |
| gate | `gate run --gate focused-docs --worktree ../cand` | OK, result `0e670283…466d` `state: PASSED`, `executedTreeOid` = candidate tree, receipt 5 |
| negative | `complete --commit 7768af43048c85cb4cdce9c8de9013c7abdbb346` | exit 1, `REFUSED`, codes `[COMMIT_NOT_INTEGRATED, STALE_TREE]`; head stayed at seq 5 |
| positive | `complete --commit c4a7522224bb83ae47d2e544eb03fa73b2af39fd` | OK, receipt 6, ticket revision 2 |
| audit | `receipt audit` | `structuralConsistency: CONSISTENT`, `projectionAgreement: AGREES`, head seq 6 |

The negative commit `7768af43` was made with `git commit-tree` on the candidate tree, parent
`9d57fd1c`, and no ref. It has the same tree as the candidate, so the refusal can only come from
reachability. No ref contained it (`for-each-ref --contains` printed 0 refs). The refusal detail was:
"commit 7768af43… is not reachable from refs/heads/main or its upstream refs/remotes/origin/main;
fetch the upstream (git fetch) so refs/remotes/origin/main contains the commit, then complete again".

`ticket show V1-0849` after completion: `status: COMPLETED`, completion `kind: VERIFIED`, evidence
`[0e670283…466d]`, `manifestSha256: b8ba98f9ad1c93cf3eee9213d0ff251e8487ab315d787e21d7977152223aae16`.
The manifest binds `candidateTreeOid 9d5a4935…` (the tree of `c4a75222`), `baseCommit 9d57fd1c…`,
`gateResults [0e670283…]` and `scopeCheck WITHIN`. Its `mode` is `DEVELOPMENT`, and `cem` and
`docsResult` are null.

### Ref digests

`git for-each-ref --format='%(refname) %(objectname)' | shasum -a 256` over 144 refs:

- before the claim: `aacc4377640dd6b804734186de0aa33be12b9e9e54a3c6a0fed3ce24ea1532e7`
- after the refused complete: `aacc4377640dd6b804734186de0aa33be12b9e9e54a3c6a0fed3ce24ea1532e7`
- after the positive complete: `aacc4377640dd6b804734186de0aa33be12b9e9e54a3c6a0fed3ce24ea1532e7` (`cmp` identical)

No ref moved. The live `head.json` sha256 was `a43269df…fb7ad` before and after the whole run.

### Limits

- Unsupported: a relocated full-store copy (`UNSUPPORTED_FILESYSTEM`, above). The qualification
  uses a fresh store, so it does not prove the live 6,628-receipt chain completes from another path.
- The store ran in `DEVELOPMENT` mode (no execution cutover). `cemRequired` and `docsLane` were not
  enforced, so the manifest carries `cem: null` and `docsResult: null`. The live V1-1404 and V1-1407
  completions have the same mode.
- One positive and one negative run on one host; `runtimeQualification`, `liveness` and
  `historicalAcceptance` stay `NOT_OBSERVED`.

## V1-0557: closure evidence

| Criterion | Evidence |
| --- | --- |
| AC1: define and owner-accept the public integration/intent-branch authority and rollback path | Orchestrator disposition under the owner's 2026-10-16 delegation: public `origin/main` is the integration/intent authority through the CAL-V0-017 upstream reachability path, and rollback is the existing local-`main` path. Recorded in decision record orchestrator-dispositions-2026-10-16 (pending). Private state and active attempts are untouched: completion reads refs and never fetches or moves one. |
| AC2: qualify on representative copied stores, preserving receipt chain, claims, identities and candidate/gate binding | V1-1459 above: claim, submit, gate, complete and audit `CONSISTENT` on a copy, with the manifest binding the candidate tree and gate result. Retained limit: a full live-store copy is refused as a relocation, so the run used a fresh store over the repository's queue and policy. |
| AC3: native completion for a reviewed, publicly integrated candidate, read back with ticket and audit receipts | Live store, read-only reads on 2026-10-16. V1-1404 is `COMPLETED`/`VERIFIED`, manifest `c09904a3…3900`; V1-1407 is `COMPLETED`/`VERIFIED`, manifest `6767f4ab…1077`. Both manifests bind `candidateTreeOid f89a6be046b8bbdb305211a78edfbbf62a92221c` (the tree of PR #919's merge `3337f5f6db262a2e0f75b548814e7d06dbbcdd05`) and base `778bfcd3`. Their `focused-docs` results are `PASSED` at that tree (V1-1407's is a reuse of V1-1404's). The live `refs/heads/main` (`3beedd98`) does not contain `3337f5f6`; the live `refs/remotes/origin/main` (`6f5272b5`) does. These completions therefore used the upstream path. Live `receipt audit`: head seq 6628, `CONSISTENT`, `AGREES`. |
| AC4: retain observed divergence and unsupported/unknown qualifications | Live primary on 2026-10-16: `refs/heads/main` `3beedd981f0d71f604175860a7f09e6d869c5687`, `refs/remotes/origin/main` `6f5272b5f22fcb779c755f338b0bc0a3d3184ba9`. Their merge base is `3beedd98`; local main is 0 commits ahead and 1,864 behind. This is the same history (local main is an ancestor), not unrelated history. The upstream is `origin`/`refs/heads/main`. Unsupported: relocated full-store copy. Not observed: runtime qualification, liveness, historical acceptance. No runtime defect is claimed. |

## V1-1059: repair text on barrier and redo refusals

`CTW-V0-007` (amended in `docs/specs/corvint-tasks-intent-worktree-v0.md`) now lists `Barrier`
among the writers whose intent-location refusals carry the repair text, plus the redo refusal of
`Import`. The barrier itself has no branch guard. Its only intent-branch refusal is the
pending-receipt redo it settles first (`redoPendingFor` → `requireBranch`). `store.Barrier` now
defers `intentFix`, as `Mutate`, `Release` and `PolicyUpdate` do. `Import` wraps only its
`redoPending` refusal with `WithIntentRepair`, and its other refusals are unchanged. Redo under
`Mutate`, `Release`, `PolicyUpdate` and batch mutate was already covered by those writers' deferred
fix.

Tests (`internal/tasks/store/intent_worktree_test.go`):
`TestCTWV0007_BarrierRedoRefusesWithTheFix` and `TestCTWV0007_ImportRedoRefusesWithTheFix`. Each
holds a pending intent-writing receipt with the primary on `feature` and no intent worktree. It
asserts `INTENT_BRANCH_MISMATCH` with detail ending `primary intent branch differs; <IntentFix>`,
the `CTW-V0-005` `git worktree add` command, and an unchanged repository.

Mutations (reverted after use): removing the `intentFix` defer from `Barrier` fails only
`TestCTWV0007_BarrierRedoRefusesWithTheFix`. Removing the `WithIntentRepair` wrap in `Import` fails
only `TestCTWV0007_ImportRedoRefusesWithTheFix`. With both fixes in place, both tests pass.

Rollback: revert the commit. No wire code, store format or receipt changes.
