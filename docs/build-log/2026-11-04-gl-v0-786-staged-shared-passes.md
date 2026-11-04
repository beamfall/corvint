# 2026-11-04 rescue-b fix round: staged shared-digest passes (GL-V0-786)

BASE 24d8c04f9f5ffaaed86759951e520cb775dc2eff (origin/main). No requirement IDs were added. The
round amends `GL-V0-786` in `docs/specs/gate-ledger-v0.md` and its trace row.

## Independent review finding (P1)

A shared-digest pass became reusable before it was validated. Every batch-precheck step under
`GATE_LEDGER_SHARED_DIGEST` wrote its pass straight into the ledger. That record was keyed on the
digest taken at the start of the run. Until `verify_shared` ran, three consumers could reuse a pass
whose step might have read other content: another worktree, a concurrent run, or a run interrupted
before verify. If the run was interrupted, verify never ran at all.

## Fix

- `gate-ledger digest FILE` now creates `FILE.stage`. The stage must be a plain directory with mode
  0700, owned by the invoking user, and not a symlink. Creating it also empties any stage abandoned
  under the same file. A process on the shared digest writes passes only to that stage. Lookups
  never read the stage, so no other process can use an unverified pass.
- After a passing step, the process digests the worktree again (`moved`). If the tree no longer
  matches the shared tree, it prints `RECORD STEP skipped: the worktree moved since the shared
  digest` and stages nothing. Each pass is therefore bound to the tree that was present before and
  after the step.
- If `digest --verify FILE` finds the tree unchanged, it promotes only well-formed staged records
  into the ledger: correct schema, key matching the file name, and tree matching the shared tree.
  If the tree moved, verify discards the stage, removes records written under the shared tree since
  FILE was written, and exits 1.
- On an interruption (EXIT, HUP, INT or TERM), batch-precheck's existing cleanup trap removes
  `$dir`. That removes the stage with it, so unverified passes are discarded and never promoted.
- On a host without Unix ownership, `ownedByInvokingUser` is false, so `digest FILE` refuses the
  stage and writes no shared digest. batch-precheck then falls back to a digest per step, as it
  already did when the shared digest was unavailable. No shared-run pass is recorded on such a host.
- Cost: one extra worktree digest, paid only by a step that ran and passed. HIT steps pay nothing.
- Residual limit: an edit that is made and fully reverted while one step runs cannot be detected.
- Out of scope, and reported for a ticket: a process on its own per-step digest (no shared digest)
  still records its pass without digesting again after the step, as it did before this change. An
  edit made during such a step is therefore keyed on the tree from the step's start.

## Independent review finding (P2)

`TestChangeDWCV0756ImpactBesideQueryMatchesTheSequentialRun` used to prove overlap with elapsed
time and sleeps. Two channels now prove it instead: each command closes its own channel on entry,
and in concurrent mode each waits for the other. A one-minute timeout is used only to detect
deadlock.

## Evidence

- Go tests:
  - `TestSharedDigestStagesPassesUntilVerify_GLV0786`
  - `TestSharedDigestRefusesAPassWhoseTreeMovedDuringTheStep_GLV0786`
  - `TestSharedDigestVerifyForgetsRecordsOfAMovedTree_GLV0786` (updated)
- Shell test: `script/batch-precheck_test.sh` case 4b covers three cases:
  - a concurrent consumer sees no ledger record before verify;
  - verify promotes the staged passes;
  - a TERM before verify leaves no ledger record and no reuse.
- Mutations: with the stage removed from `openShared`, three Go tests fail, and all six new 4b
  assertions fail. With `moved` forced to "", the Refuses test fails. With the P2 command start
  forced to sequential, the DWC test fails at its deadlock timeout.
- `-race -count=20` on the DWC test: 20 passes.
- `script/batch-precheck_test.sh` still fails cases 1 and 9, which fail identically at BASE.
