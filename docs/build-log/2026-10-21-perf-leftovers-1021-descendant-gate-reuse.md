# 2026-10-21 perf-leftovers-1021: descendant full-gate reuse (V1-2193)

## Problem

V1-2193 (P1): a seal commit's own `full-gate` re-ran even when a later commit on `main` already
contained the seal and had passed the same gate. The same-tree reuse rule (`CAL-V0-240..243`,
V1-1415) only matched results at exactly the attempt's candidate tree, so every merged seal paid one
extra full gate (about two hours on this host).

## Decisions

- The requirements are `TCD-V0-151..153` in `docs/specs/corvint-tasks-closeout-debt-v0.md`, with a
  cross-reference amendment note in the CAL V1-1415 section. No new CAL ID is added. They are accepted
  under the owner's standing auto-accept (2026-10-11).
- No new policy field. The strict policy decoder would make a field a format change, so the existing
  `reusable: true` gate flag turns the rule on.
- The protected ref is the queue's `intentBranch` plus its configured remote-tracking upstream, the
  same refs `intentIntegrated` reads. The history is read once (`git log %H %T`, at most 100,000
  commits). Ancestry uses `merge-base --is-ancestor`, at most 16 pairs. Nothing is fetched or written.
- Defense in depth:
  - the store chooses the source;
  - the model refuses unless the store rechecked the ancestry under the lock (`BLOCKED STALE_TREE`);
  - `complete` rechecks it before accepting the evidence;
  - the receipt audit verifies it from git through an injected oracle, so `transaction` stays pure.

## Evidence

- Tests:
  - `TestTCDV0151_DescendantPassCoversAMergedSeal` covers reuse without execution, `reusedFrom`, `complete`, the audit and the fold, and the fold failing after the branch is rewound.
  - `TestTCDV0152_DescendantReuseRefusals` covers the not-on-branch and not-an-ancestor cases.
- Each test fails under a mutation that removes its fix:
  - 0151 fails when the descendant selection is disabled;
  - 0152 fails when the model refusal is disabled.
- Regression runs passed:
  - all of `./internal/tasks/transaction`;
  - `-run 'Gate|Reuse|Receipt|Binding|Complete|TCDV|CALV02'` in `./internal/tasks/store`;
  - `TestCALV024*` and `TestCALV0131_LiveFormatsCoverEveryDecodedProfile` in `./internal/tasks/cli`;
  - `TestImportDirection`;
  - go vet and gofmt.
- Synthetic before/after measurement (the fixture in `descendantCase`):
  - before: the seal's gate executed again, for 2 executions over the two attempts (mutation run);
  - after: 1 execution, and the seal's run is a reuse that writes the receipt with no command executed.
  - At the live full-gate cost, that saves one full gate per merged seal.

## NOT_RUN / NOT_PRODUCED

- A real seal-closeout timing on the live store is NOT_PRODUCED. It needs the operator's policy
  change below and a merged seal.
- `make gate` was NOT_RUN (lane policy).

## Operator enablement and compatibility

To turn the rule on, set `"reusable": true` on the `full-gate` entry of `.taskman/policy.json` through
the store `policy update` (version + 1). Upgrade every `corvint-tasks` that audits the store first.
An older binary's receipt audit fails a recorded descendant reuse as `JOURNAL_FORKED`. An older
`complete` refuses the result as `GATE_STALE`, which is safe.

## Rollback

Set `reusable` back to `false`. Revert the code only after any attempt with a recorded descendant
reuse has completed. There is no store format or wire change.
