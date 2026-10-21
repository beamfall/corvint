# 2026-10-21 gates-1021c: CI-run gate evidence and environment-value reuse key

Tickets: V1-2216 (P1), V1-1431, V1-0723, V1-0324. Spec: `docs/specs/corvint-tasks-closeout-debt-v0.md`
TCD-V0-154..158. These are accepted under the owner's standing auto-accept (2026-10-11).

## Problem

- V1-2216: the owner said a change "checked in the cloud" needs no local gate run. A gate could not
  accept a green merged CI run as evidence, so every merged ticket re-ran its required gates
  locally before `complete`.
- V1-1431 (confirmed): `gateEnvironment` digested `{name, present}` only. A reusable PASS recorded
  under another `PATH` or `GOTOOLCHAIN` was reused.

## Decisions

- **CI evidence is opt-in per gate.** It is declared in an optional policy key `ciEvidence`
  (TCD-V0-154): a gate ID, a GitHub `owner/name` repository and the required check names. Removing
  the key through `policy update` is the rollback. CI evidence does not require `reusable: true`.
- **The CI run is a retained record, and nothing is fetched.** The `taskman-ci-run/0` record is
  posted as an `evidence/<sha256>` blob. The gate result names that blob in `reusedFrom`, so the
  existing reuse paths in `complete` and the audit carry it. The verb never fetches and never
  queries GitHub (invariant 7). The record's facts are the operator's claim. Only the head
  commit's ancestry is re-observed, from local git.
- **Ancestry is checked at the commit level.** It is required even when the head tree equals the
  candidate tree. Then a rewound branch fails the audit (TCD-V0-157) instead of passing on tree
  equality alone.
- **The model is the only judge.** Policy admission, repository, conclusion and required checks
  are judged by the transaction model (`CIRunRefusal`). The store does not pre-check them. The
  same function runs in the receipt audit against the policy as last posted.
- **The environment value key applies only to reusable gates.** TCD-V0-158 adds a per-value
  SHA-256 for gates declared `reusable: true`. A non-reusable gate keeps the names-only digest, so
  no new value-derived material is recorded for gates that never reuse. A per-session `TMPDIR`
  declared on a reusable gate now prevents reuse across sessions; the spec says so.

## Evidence

All runs went through `heavy.sh`, one package per command.

- `internal/tasks/store`:
  - `TestTCDV0155_GreenCIRunClosesMergedTicketsWithoutLocalGates`: 3 tickets, zero `verify`
    executions, and all three complete. After a rewind the fold fails.
  - `TestTCDV0156_CIRunRefusals` (7 subtests).
  - `TestTCDV0155_ModelRefusesAForgedCIResult`.
  - `TestTCDV0158_ReuseKeyCoversDeclaredEnvironmentValues`.
  - Regression run of `TestTCDV015|TestCALV024|Gate|Reuse`: 59 pass, 2 skip (existing load tests).
- `internal/tasks/intent`: `TestTCDV0154_PolicyCIEvidence`.
- `internal/tasks/cli`: `TestCALV0131*`, with the new profile registered in `LiveFormats` and the
  format-version rows.
- `internal/tasks/transaction` and `internal/tasks/snapshot`: pass.
- Mutation proofs. Each mutation made the named test fail, and each was then restored:
  - the model's `CIRunRefusal` call skipped: 4 refusal subtests fail;
  - `commitDescent` always passing: 2 refusal subtests and the rewind assertion fail;
  - the audit's `CIDescent` check skipped: the rewind assertion fails;
  - the model's record-equality check weakened: the forged-record test fails;
  - the environment value dropped from the digest: `TestTCDV0158` fails.
- Criterion 3 of V1-2216 (zero local gate runs to close N merged tickets) is measured in the
  fixture: N=3, zero runs. On the live store it is NOT_PRODUCED, because no store has
  `ciEvidence` enabled.
- V1-0723: probably already delivered by 5db4ff66d (V1-0772, CAL-V0-086).
  - That commit records the supervised stage worktree, a path that can exceed 128 bytes under a
    long agent TMPDIR, as PathText instead of an Identifier. The fix names
    `TestCALV0072_MultiRepositoryGatesFailClosed`.
  - With a 175-byte TMPDIR, the program, workflow, multi-repository and CALV007x/008x store tests
    pass at base 0106d06c (391 s).
  - The full package under a long TMPDIR exceeds the 9-minute per-command budget and is NOT_RUN.
    The second failing test is not identified.

## NOT_RUN

- `make gate`: not run, by owner preference.
- The full `internal/tasks/store` package under a long TMPDIR: exceeds the 9-minute budget.
- Live-store CI closeout timing.
- V1-0324 (PROJECTION_STALE): not started within the lane budget. `cli/cli.go` is shared with peer
  PR #1430.

## Rollback

- Remove `ciEvidence` with a store `policy update`, then revert after every attempt with a
  CI-backed result has completed.
- Reverting TCD-V0-158 restores the names-only key. Results recorded with values then execute once
  more instead of being reused.
