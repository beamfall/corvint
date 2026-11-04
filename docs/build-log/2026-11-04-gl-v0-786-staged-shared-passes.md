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
- After a passing step, the process digests the worktree again (now `stepMoved`). If the tree no
  longer matches the shared tree, it stages nothing. The second review showed this check alone was
  insufficient; see "Second independent review" below.
- If `digest --verify FILE` finds the tree unchanged, it promotes only well-formed staged records
  into the ledger: correct schema, key matching the file name, and tree matching the shared tree.
  If the tree moved, verify discards the stage, removes records written under the shared tree since
  FILE was written, and exits 1.
- On an interruption (EXIT, HUP, INT or TERM), batch-precheck's existing cleanup trap removes
  `$dir`. That removes the stage with it, so unverified passes are discarded and never promoted.
- On a host without Unix ownership, `ownedByInvokingUser` is false, so `digest FILE` refuses the
  stage and writes no shared digest. batch-precheck then falls back to a digest per step, as it
  already did when the shared digest was unavailable. No shared-run pass is recorded on such a host.
- Residual limit: an edit that is made and fully reverted while one step runs cannot be detected.

## Second independent review (two P1 findings)

- Shared path. The check after the step did not bind execution. Digest A, edit an input to B before
  the step starts, let the step pass on B and restore A before it returns: the post-step digest saw
  A, verify promoted the pass, and the next run skipped testing A.
- Own-digest path. A process with no shared digest never checked again, so an edit made after its
  digest, or during its step, still recorded a pass keyed on A (ticket V1-4547).

Fix, in `tools/gate-ledger`, the same for both paths:

- `runStep` digests the worktree immediately before it executes a keyed step and derives the step's
  key again from that digest (`stepMoved`). When that matches, it does the same after the step
  passes. It records only when both keys equal the key it runs under. Otherwise it prints `RECORD
  STEP skipped: its inputs changed before the step started` or `... while the step ran`.
- The keys are compared rather than the whole tree, so a step that writes an untracked file outside
  its declared inputs still records. A first draft compared whole trees and failed three existing
  tests whose step writes an untracked `COUNT` file; that draft is the red check for the key
  comparison. A tree-keyed step still compares the whole tree.
- `runPackages` brackets its one `go test` batch with the whole-tree check (`treeMoved`), and only
  when some package in it can be recorded. The whole tree is a superset of every package's bound;
  deriving the package keys again would rerun `go list` and the selector, twice per batch. A batch
  that writes into the worktree records no package, which fails closed.
- Stated limit, kept in the spec: an edit made and undone entirely inside one step's execution is
  not detected, nor one made after the check before the step and undone before the check after it.
- Neither path needed to disable reuse; nothing is listed under Disabled for 1.0.
- Cost, paid only by a step that runs, never by a HIT: two worktree digests, the second only after a
  pass, plus the key's own derivation each time (a `go list` for a `@test:` or `@deps:` scope). The
  shared path paid one digest before. A warm digest of this repository (12,021 tracked files) took
  0.7 to 0.8 s on wsl-lanes, and the cold first run 2.5 s.

Evidence:

- `TestSharedDigestRefusesAPassOfAnEditRestoredBeforeTheStepReturns_GLV0786`: the reviewer's
  regression; neither staged nor promoted.
- `TestOwnDigestRefusesAPassWhoseInputChangedWhileTheStepRan_GLV0786`: the V1-4547 repro. Its
  unedited run records although it writes an untracked file.
- `TestOwnDigestRefusesAPassOfAnEditRestoredBeforeTheStepReturns_GLV0786`: the edit is made through
  the test-only `lockHook`, once the step holds its lock.
- `TestGoTestRecordsAPackagePassOnlyOnAnUnmovedTree_GLV0786`: both checks around the package batch.
- The Refuses test now expects the pre-execution message.
- Mutations, each red on `go test -run 'GLV0786|TestRunStepSkipsOnlyRecordedIdenticalInputs'`:
  - `runStep` pre check removed: three tests fail, the two restored-before-return tests and the
    Refuses test.
  - `runStep` post check removed: the V1-4547 test fails.
  - `runPackages` pre check removed, and separately its post check removed: the go-test test fails.
- The full package, `go test -count=1 ./tools/gate-ledger/`, passes.

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
