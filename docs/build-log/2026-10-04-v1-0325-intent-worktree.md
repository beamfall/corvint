## 2026-10-04 V1-0325: intent-worktree rule for Corvint Tasks writes (accepted)

Human-owned intent: owner request 2026-10-04: close V1-0720 follow-ups. V1-0325 asks that an agent
on a feature branch can file a ticket without switching the primary checkout.

Status: accepted (owner decision 2026-10-04). The rule ships as `experimental` in
`docs/specs/corvint-tasks-intent-worktree-v0.md` (`CTW-V0-001` to `CTW-V0-011`). It amends the
wording of TCP-00 §3.1 ("primary worktree only") and `TM-V0-007` in the external contract
`corvint-tasks/docs/SPEC.md`; that external edit is its own change and is not made here.

### Decision

- The intent root is the checkout whose `.taskman/` is the intent projection (`CTW-V0-001`). It is
  the primary worktree unless the primary's HEAD differs from the intent branch named in the
  primary's `.taskman/queue.json` (`CTW-V0-003`) and exactly one admitted linked worktree holds that
  branch (`CTW-V0-002`).
- Admission (`CTW-V0-004`) is file-only: the registration's `gitdir` and the worktree's `.git`
  back-pointer must agree by file identity, `.taskman` has no symlink component, and its
  `queue.json` names the same queue and branch. Stale or foreign entries are never selected.
- Without an admitted intent worktree, today's refusal stands and gains the exact fix
  `git -C '<primary>' worktree add '<primary>-<branch>' '<branch>'` (`CTW-V0-005`). Two candidates
  are ambiguous and name `git worktree remove` (`CTW-V0-006`). A dirty intent worktree is refused
  `INTENT_DIVERGED` by the existing audit, with a fix naming the linked path and
  `corvint-tasks reconcile inspect` (`CTW-V0-007`). No wire code is added.
- The writer's fixture session pins the linked root, refuses another mount, and re-resolves under
  the lock (`CTW-V0-008`). From the intent-branch checkout every byte is unchanged (`CTW-V0-009`).
- The rule is cooperative routing, not a security boundary (`CTW-V0-010`). Identity, journal and
  lock stay on the primary; program, workflow and lease Git reads keep the checkout they used
  before, which for lease commands is the caller's cwd (`CTW-V0-001`, `CTW-V0-011`).

### Rejected alternatives

- An early guard in `writerGuards`: it would run before the existing audit and change refusal
  order for the intent-branch path.
- Repair text added only in the CLI: library writers and reports would lose the fix.
- A new `journal.Reader` field for the intent root: it widens a shared type for one caller.
- Resolving worktrees by running `git worktree list`: a subprocess on every read, and its output
  depends on the caller's environment.
- Changing the `guardFailure` signature: a deferred repair on the existing error paths is smaller.

### Evidence

- Fixture and real-Git tests in `internal/tasks/intent`, `store`, `authority` and `cli` (named in
  the spec's traceability table): a feature-branch primary files into the linked intent worktree
  while its HEAD, index, status and primary projection stay unchanged, and the missing, stale,
  dirty and ambiguous cases refuse with the exact fix.
- Observed while testing: a refused write still refreshes the derived checkpoint
  `<common>/taskman.checkpoint.json`. That is existing behavior, unchanged here, so the tests
  compare HEAD, index, projections and state-dir content rather than the whole common dir.
- Integration with V1-0751: `ticket create --template` and `submitMutation` load intent from
  `repo.IntentRoot()`, so the template agrees with the routed intent root.
- `make -k gate` on the merge with main (bc87a4dc) is not green. After the gofmt fix in this change,
  every step passes except failures that reproduce on main without this change: go-test (V1-0766;
  V1-0778 and V1-0779 in earlier runs), Windows `cross-vet` and `host-package-versions-check`.

### Review

One independent review of 40bf9e74 found no blocking issues. Its findings and their disposition:

- Fixed: the lock-time re-resolution compares paths, and the pin then opened the linked root by path
  without checking it was the admitted directory. The pin now requires the path to name the opened
  directory and the `.git` file read through the opened root to point to the admitted registration
  by file identity, else `UNSUPPORTED_FILESYSTEM` with nothing retained (`CTW-V0-008`,
  `TestCTWV0008_SwappedLinkedIntentWorktreeIsRefused`).
- Fixed: repair commands quoted paths with Go `%q`, which is not shell-safe for `$` or backticks.
  Paths and the branch are now POSIX single-quoted (`CTW-V0-005`, `CTW-V0-006`,
  `TestCTWV0007_RepairCommandsUseShellQuoting`).
- Documented: `CTW-V0-007` said "a writer's error", but the fix is applied only by `Init`,
  `Mutate`, `PolicyUpdate`, `Release` and `Lease`. The spec now names those writers and the
  exceptions (cutover, execution cutover, reconcile, import, barrier, lease recovery, redo); code
  coverage was not widened.
- Documented: the review asked what claim base a claim from the intent worktree uses while the
  primary is on a feature branch. Reading `internal/tasks/cli/lease.go` and
  `internal/tasks/store/lease.go` shows the base is `--base` (default `HEAD`) resolved in the
  caller's cwd, so from the intent worktree it is the intent-branch commit, not the primary's
  feature HEAD. `CTW-V0-011` previously said lease Git observation uses the primary, which was
  inaccurate; it now states the actual behavior. This is code-read evidence, not a new test.
- Documented: a `git switch` in the linked intent worktree between the branch check and publication
  is a pre-existing race shared with the intent-branch primary path; this change does not close it.
- Limits retained: `CTWV0009` tests check that the repair text is absent (substring absence), not
  full byte equality of every message; `CTW-V0-010` and `CTW-V0-011` are design and code-review
  requirements without runtime tests.

### Rollback

Revert the commit. Resolution then always returns the primary, the repair text disappears, and the
journal, receipts and projections are unaffected because the rule adds no stored state. Tickets
already filed through an intent worktree remain valid; commit or carry that worktree's `.taskman`
changes as usual.
