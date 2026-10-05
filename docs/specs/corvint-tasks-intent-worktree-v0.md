# Corvint Tasks intent worktree V0

Owner: Russell Lewis
Date: 2026-10-04
Intent status: accepted (owner decision 2026-10-04)
Delivery status: experimental
Authoritative inputs: owner request 2026-10-04 to close the V1-0720 follow-ups, ticket V1-0325,
`docs/build-log/2026-10-04-v1-0720-memory-workaround-layers.md` (row 3), the Corvint Tasks contract
TCP-00 (`beamfall/corvint-tasks` `docs/SPEC.md` §3.1 "Primary worktree" and `TM-V0-007`), decision
0397 (corvint-tasks built in tree), `docs/specs/corvint-tasks-store-init-v0.md`, and the in-tree
sources under `internal/tasks/intent`, `internal/tasks/store`, `internal/tasks/authority`,
`internal/tasks/cli`, `internal/tasks/journal` and `internal/tasks/archive`.

## Agent digest
- Claim: An agent on a feature branch files a ticket through one linked worktree on the intent branch, without switching the primary checkout.
- Status: accepted (owner decision 2026-10-04); experimental. CTW-V0-001 through CTW-V0-009 are implemented and tested; CTW-V0-010 and CTW-V0-011 are design and code-review requirements without runtime tests.
- Exists: intent-root resolution in `internal/tasks/intent`, the store and CLI redirection, the fixture-session pin of the linked root, repair text on the two intent refusals, and fixture plus real-Git tests.
- Blocked on: the matching edit to the external TCP-00 `docs/SPEC.md` §3.1 and `TM-V0-007` wording.
- Read next: Requirements; Failure modes; Acceptance evidence; Rollback.

## User and boundary

The intent projection `.taskman/` is Git-tracked; the journal under `<common>/taskman` is not.
TCP-00 §3.1 puts every intent-file read and write, branch check, intent tree digest and publication
observation in the primary worktree, and `TM-V0-007` refuses `INTENT_BRANCH_MISMATCH` unless the
primary's HEAD is on `queue.intentBranch`. An agent working on a feature branch in the primary
checkout therefore had to switch the primary to the intent branch, file the ticket, and switch back
(the "intent-branch checkout dance" recorded in the V1-0720 inventory). The switch moves the
operator's index and working tree and races any other agent that uses the primary.

This amendment adds one routing rule. When the primary is not on the intent branch, a single linked
worktree that is on it receives the projection instead. It changes no identity, no journal or wire
format, no error code and no behavior from an intent-branch primary. It amends TCP-00 §3.1 and
`TM-V0-007` by replacing "the primary worktree" with "the intent root (CTW-V0-001)" wherever those
clauses name the checkout holding `.taskman/` or the HEAD the branch guard reads. Their IDs and every
other clause are unchanged. The owner accepted this amendment on 2026-10-04; the external
`docs/SPEC.md` text is edited to match in its own change.

## Requirements

- `CTW-V0-001`: The intent root is the checkout whose `.taskman/` is the intent projection. Every
  read audit, intent-file write, `TM-V0-007` branch check, intent tree digest and publication
  observation that TCP-00 assigns to the primary worktree uses the intent root. The intent root is
  the primary worktree unless `CTW-V0-002` selects a linked worktree. The recorded
  `primaryWorktree` identity in `head.json` and the `INIT` receipt, the relocation refusal, the
  state dir, the journal and the lock stay bound to the primary worktree and common dir.
- `CTW-V0-002`: When the primary worktree's HEAD does not name the intent branch from
  `CTW-V0-003`, and exactly one linked worktree admitted by `CTW-V0-004` has HEAD
  `ref: refs/heads/<intentBranch>`, that linked worktree is the intent root. The branch guard then
  reads its HEAD file `<common>/worktrees/<id>/HEAD`, so `INTENT_BRANCH_MISMATCH` and
  `INTENT_DIVERGED` keep their meanings at the intent root. Selection depends only on repository
  files, never on the caller's working directory: the primary, the linked worktree and any other
  checkout of the repository resolve the same intent root.
- `CTW-V0-003`: The intent branch and queue ID used for routing are read from the primary worktree's
  `.taskman/queue.json`. If that file cannot be read or decoded, nothing is selected and the existing
  refusals apply unchanged. The hint only routes; the writer's queue, policy and divergence checks
  still run against the intent root's projection and the journal.
- `CTW-V0-004`: A linked worktree is admitted only when all of the following hold. It is among the
  first 256 entries of `<common>/worktrees` in name order, and its entry is a directory, not a
  symlink. Its `gitdir` file names a single `<root>/.git` path, the parent of `<root>` resolves,
  and `<root>` is a supported path and a directory. `<root>/.git` holds a single `gitdir:` line
  naming the same registration directory by file identity. `<root>/.taskman` has no symlink
  component. `<root>/.taskman/queue.json` decodes with the hint's queue ID and intent branch. An
  entry on the intent branch that fails a registration check is reported as stale, and one that
  fails a path or projection check as not admitted; neither ever becomes the intent root.
  Resolution runs no Git subprocess, takes no lock and opens nothing for writing.
- `CTW-V0-005`: When the primary is not on the intent branch and no admitted linked worktree holds
  it, the intent root stays the primary and today's refusal stands (`INTENT_BRANCH_MISMATCH`, or
  `INTENT_DIVERGED` when the projection audit refuses first). Its message gains a fix naming the
  primary's branch (or a detached or unreadable HEAD), the intent branch, any entry refused by
  `CTW-V0-004` together with `git worktree prune` or `git worktree repair`, and the exact command
  `git -C '<primary>' worktree add '<primary>-<branch>' '<branch>'`, where `/` in the branch becomes
  `-` in the path, or switching the primary to the intent branch. Paths and the branch in a repair
  command are POSIX single-quoted shell words (each `'` becomes `'\''`), so `$`, backticks and
  spaces paste literally.
- `CTW-V0-006`: When two or more admitted linked worktrees hold the intent branch, none is selected
  and the intent root stays the primary. The refusal fix names every candidate root, quoted as in
  `CTW-V0-005`, and `git worktree remove`.
- `CTW-V0-007`: Repair text is appended only to `INTENT_BRANCH_MISMATCH` and `INTENT_DIVERGED`
  refusals: to the error or report detail of the store writers `Init`, `Mutate` (ticket
  mutations), `PolicyUpdate`, `Release` and `Lease` (every lease command, claim included), and to
  a read's warning. Other writers (cutover, execution cutover, reconcile, import, barrier, lease
  recovery and redo) keep their refusal text without the fix. The code, location and
  original message are kept, with the fix after `; `. For `INTENT_DIVERGED` at a linked intent root
  the fix names the linked `.taskman` path, carrying or committing the changes, and
  `corvint-tasks reconcile inspect`. A dirty intent worktree, whose projection no longer equals the
  journal (uncommitted hand edits included), is refused by the existing audit; nothing is written.
  No wire code is added and the closed code set is unchanged.
- `CTW-V0-008`: A writer's fixture session retains a linked intent root as a second pinned root.
  It pins `.taskman/` under that root, and on every check reopens it by path and requires the same
  directory. It refuses `UNSUPPORTED_FILESYSTEM` when the linked root's mount differs from the
  primary's, because publication links from staging under the common dir. Re-resolution under the
  lock must yield the same intent root and HEAD path, or the session refuses. Because that
  comparison is by path, the pin then requires the path to still name the opened root directory
  and the `.git` file read through the opened root to point to the admitted registration by file
  identity, or it refuses `UNSUPPORTED_FILESYSTEM` and retains nothing.
- `CTW-V0-009`: When the primary's HEAD names the intent branch, or `CTW-V0-003` yields no hint,
  the intent root is the primary. Every read, write, refusal code and message, receipt, journal
  byte and projection byte is then identical to the behavior before this amendment, whatever
  linked worktrees exist.
- `CTW-V0-010`: The rule is cooperative routing among the operator's own checkouts, not a security
  boundary. HEAD files, worktree registrations and queue manifests are ordinary files that any
  local process with repository write access can change. A process that can rewrite them can
  already edit `.taskman/` or the journal directly. The rule guards against stale or mistaken
  layouts. The canonical checks (`TM-V0-007` divergence, `TM-V0-006` request IDs and actor roles)
  remain the authority.
- `CTW-V0-011`: Program and workflow runs, claim and lease Git observation, archive identity, and
  the `primaryWorktreeSha256` snapshot field keep the checkout they used before this amendment;
  the intent root never changes them. Only the intent projection moves. In particular a lease
  command reads Git from the caller's working directory, falling back to the primary only when
  none is given (`internal/tasks/cli/lease.go` passes the cwd as the lease root;
  `internal/tasks/store/lease.go` `leaseRoot` and `claimFacts`). A claim's base commit is
  `--base`, default `HEAD`, resolved there: run from an admitted linked intent worktree it is that
  worktree's intent-branch commit, and run from a feature-branch primary it is the feature HEAD.
  The intent root does not pin or rewrite the claim base.

Non-goals: moving or relocating the primary identity, a new error code or wire field, a journal or
receipt format change, running Git to resolve worktrees, cross-filesystem intent worktrees,
automatically committing, carrying or merging the projection between branches, choosing among
several intent worktrees, and changing which checkout program, workflow or lease commands read Git
from.

## Failure modes

| Situation | Behavior |
| --- | --- |
| Primary on the intent branch | Primary is the intent root; behavior unchanged (`CTW-V0-009`). |
| Primary on a feature branch, one admitted linked worktree on the intent branch | Writes publish into the linked `.taskman/`; the primary's HEAD, index and working tree are untouched (`CTW-V0-002`). |
| Primary on a feature branch, no linked worktree on the intent branch | `INTENT_BRANCH_MISMATCH` with the `git worktree add` fix; nothing written (`CTW-V0-005`). |
| Registered worktree deleted, moved or with a foreign `.git` back-pointer | Stale, not admitted; the fix names prune or repair and add (`CTW-V0-004`, `CTW-V0-005`). |
| Linked projection names another queue or branch, is missing, or is reached through a symlink | Not admitted; same fix (`CTW-V0-004`). |
| Two linked worktrees on the intent branch | No selection; the fix names both and `git worktree remove` (`CTW-V0-006`). |
| Linked projection edited or behind the journal | `INTENT_DIVERGED` naming the linked `.taskman` path; nothing written (`CTW-V0-007`). |
| Linked worktree on another mount | `UNSUPPORTED_FILESYSTEM`; nothing published (`CTW-V0-008`). |
| Linked worktree replaced during a transaction | The session check refuses (`CTW-V0-008`). |
| Linked worktree path swapped between lock-time re-resolution and the pin | The pin requires the path to name the opened directory and that directory's `.git` to point to the admitted registration; otherwise `UNSUPPORTED_FILESYSTEM` and nothing is retained (`CTW-V0-008`). |
| `git switch` in the linked intent worktree between the branch check and publication | Not closed by this amendment. The same race exists today for a `git switch` in an intent-branch primary: the session pins directories, not HEAD, so the branch guard is not re-read at publication (`CTW-V0-002`, `CTW-V0-009`). |
| Claim run from the linked intent worktree while the primary is on a feature branch | The claim base is resolved in the caller's checkout, so it is the intent-branch commit; pass `--base` to claim on another commit (`CTW-V0-011`). |
| Primary queue manifest unreadable | No routing; the existing refusals apply (`CTW-V0-003`). |
| More than 256 registrations | Only the first 256 in name order are examined, and the fix says so (`CTW-V0-004`). |

## Acceptance evidence

Run with `GOTOOLCHAIN=local go test -count=1` on the packages named below.

- A feature-branch primary plus one linked intent worktree: CREATE commits, the ticket file lands
  in the linked `.taskman/tickets/`, and the primary's HEAD bytes, index bytes and projection tree
  are unchanged. This is proven on a fixture layout and on a real `git worktree add` repository,
  through the store and through `corvint-tasks ticket create`.
- A missing, stale or dirty intent worktree refuses with the exact fix text and leaves the HEAD,
  the index, both projections and the state dir byte-identical.
- An intent-branch primary keeps its refusal text and publishes into the primary.
- The existing `internal/tasks/...` suites pass unchanged.

## Rollback

Revert the implementing commit. Resolution writes nothing, so no on-disk state depends on it. Tickets
filed into a linked intent worktree are ordinary projection files on the intent branch: commit them
there, or carry them as before. After the revert the primary is the only intent root again, and a
feature-branch primary refuses `INTENT_BRANCH_MISMATCH` as before. No journal, receipt or head
format changed, so no migration or repair is needed.

## Traceability

| Requirement | Evidence |
| --- | --- |
| `CTW-V0-001`, `CTW-V0-002` | `TestCTWV0002_FeatureBranchPrimarySelectsTheLinkedIntentWorktree`, `TestCTWV0002_FeatureBranchPrimaryFilesIntoTheIntentWorktree`, `TestCTWV0002_RealGitWorktreeFilesWithoutSwitchingThePrimary`, `TestCTWV0002_TicketCreateFromAFeatureBranch` |
| `CTW-V0-003` | `TestCTWV0003_NoHintKeepsThePrimary` |
| `CTW-V0-004` | `TestCTWV0004_StaleOrForeignIntentWorktreeIsNotAdmitted`, `TestCTWV0004_StaleIntentWorktreeRefusesWithTheFix` |
| `CTW-V0-005` | `TestCTWV0005_NoIntentWorktreeNamesTheFix`, `TestCTWV0005_MissingIntentWorktreeRefusesWithTheFix`, `TestCTWV0002_TicketCreateFromAFeatureBranch` |
| `CTW-V0-006` | `TestCTWV0006_TwoIntentWorktreesAreAmbiguous` |
| `CTW-V0-007` | `TestCTWV0007_DirtyIntentWorktreeRefusesWithTheFix`, `TestCTWV0005_NoIntentWorktreeNamesTheFix`, `TestCTWV0007_RepairCommandsUseShellQuoting` |
| `CTW-V0-008` | `TestCTWV0008_SessionPinsTheLinkedIntentWorktree`, `TestCTWV0008_LinkedIntentWorktreeOnAnotherMountIsRefused`, `TestCTWV0008_SwappedLinkedIntentWorktreeIsRefused` |
| `CTW-V0-009` | `TestCTWV0009_IntentBranchPrimaryKeepsThePrimaryRoot`, `TestCTWV0009_IntentBranchPrimaryRefusalsAreUnchanged`, `TestTMV0007_AS29_LinkedCallerUsesPrimaryHEAD` (the refusal test checks that repair text is absent, not byte equality of the whole message) |
| `CTW-V0-010` | Design statement; no runtime test asserts a trust boundary. |
| `CTW-V0-011` | Code review of the unchanged program, workflow and lease call sites; existing program and lease suites pass. |
