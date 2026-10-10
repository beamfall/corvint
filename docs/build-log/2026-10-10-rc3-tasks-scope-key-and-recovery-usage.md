# 2026-10-10: exact PATH key refusal cause and worker recovery usage (V1-1090, V1-1063)

## Intent

This RC3 cut lane fixes two Corvint Tasks correctness bugs under decision 0493.

- V1-1090: `submit` refused `OUT_OF_SCOPE` for files under `.github/workflows` and
  `docs/build-log`, which the claim declared as `PATH` keys without a trailing `/`.
- V1-1063: a replacement owner recovering a dead owner's `STOPPING` program through the CAL-V0-074
  worker recovery evidence was refused `MALFORMED` when the stage's result class was not
  `NO_EXEC`. The program stayed `STOPPING`.

## Root cause

- V1-1090 is a contract question, not a matching defect. CAL-V0-021 says a key ending in `/`
  covers every path under it and any other key names one path. `wire.ParsePath`,
  `ticket.PathCovers`, claim collision and `widen` all apply that rule, so the refusal was
  correct. The refusal text named only the offending paths, so the holder could not tell that
  the cause was the missing `/`, and widened the scope one file at a time.
- V1-1063: `programTransition` (`internal/tasks/store/program.go`) rewrote a worker recovery to
  `FINISHED`/`PROVED` but kept the old `UsageKnown`. The recovery carries no host output, and
  `planProgram` requires a stop from `STOPPING` with a result other than `NO_EXEC` to derive usage
  from the output (`next.UsageKnown == old.UsageKnown && known`). The empty output makes `known`
  false, so the transaction refused the move. The CAL-V0-210 settled-stop branch already cleared
  `UsageKnown` in the same case; the worker recovery branch did not.

## Decision

- CAL-V0-024 (amended, decision 0493): the matching rule is unchanged. `planSubmit`
  (`internal/tasks/transaction/lease_gate.go`) appends a cause to the refusal. It names each
  `PATH` key without a trailing `/` that an offending path lies beneath, and states that only a
  directory key ending in `/` covers the paths beneath it. A wider match was rejected: it would
  change claim collision, `widen` and existing scopes, which the spec already fixes.
- CAL-V0-074 (amended, decision 0493): worker recovery from `STOPPING` with a result other than
  `NO_EXEC` records `usageKnown=false`. Token counters and turns are left unchanged, matching
  CAL-V0-210.

## Evidence

Each regression test fails on base `64b5d174` and passes with the fix, run with `-race`:

- `TestCALV0024_ExactPathKeyNamesTheDirectoryKeyCause`. On base, the refusal named only
  `docs/build-log/entry.md`. With the fix, it names the key. A `docs/build-log/` key admits the
  same tree.
- `TestCALV0074_WorkerRecoveryFromStopping`. A child owner exits at `refresh:STOPPING` with the
  worker recorded. On base, the reopen was refused `MALFORMED` and the program stayed `STOPPING`.
  With the fix, it records `FINISHED`/`PROVED` at epoch plus one with usage unobserved.

The transaction, service and ticket packages and the CAL-V0-015..025, CAL-V0-074 and CAL-V0-210
store tests pass with `-race`.

NOT_RUN:

- the whole store package (about 821s);
- the unbounded-reader units selected by `corvint affected`;
- `make gate`;
- live host qualification of the recovery.

`set-effects` and `claim` still accept, without warning, a key that lacks a trailing `/` but names
a directory. That is left as a proposed follow-up.
