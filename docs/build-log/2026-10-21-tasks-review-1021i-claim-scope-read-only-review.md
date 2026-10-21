# 2026-10-21 tasks-review-1021i: claim scope, read-only review sessions, loop refusals

Lane `tasks-review-1021i`, base `8def072e08e9730a6102657c887655f62993b31d`. Tickets V1-1321,
V1-1237, V1-1441 and V1-0161 (the `EXPECTED_REFUSAL` part). ATR-V0-210..212 are accepted under the
owner's standing auto-accept (2026-10-11).

## Problem

- V1-1321: a claim on a ticket with declared `QUALIFIED` touchPaths returned OK and silently dropped
  every requested `--scope` path the declared scope did not cover, on a first claim and on a
  re-claim after release; the claim result did not say what it reserved.
- V1-1237, V1-1441: a Codex review run with `codex exec -s read-only` still wrote the task store.
- V1-0161: in `benchmarks/daily-loop-v1/harness.py`, L3, L5 and L6 all expected the same generic
  `dogfood-report-drift` / "not complete" line, so none asserted its own cause and the L3c control
  could not show anything.

## Decisions

- V1-1321 takes the ticket's refusal branch: `OUT_OF_SCOPE` naming each uncovered requested path
  (ATR-V0-210), and every claim result carries `scope` (ATR-V0-211). CAL-V0-021 stays a precedence,
  which is why the 2026-10-15 attempt (PR #915) stopped; no amendment is needed for a refusal.
- V1-1237/V1-1441 root cause: the read-only seatbelt sandbox denies writes, so the store write came
  from a command Codex escalated outside the sandbox under the default `approval_policy=on-request`.
  `CODEX_SANDBOX=seatbelt` is set for both `read-only` and `workspace-write`, so it cannot be the
  signal. The guard is the explicit `CORVINT_TASKS_READ_ONLY` variable (ATR-V0-212): an allow-list of
  pure reads; anything else refuses `UNSUPPORTED` before the store opens. The supervision doc tells
  review recipes to set it and pass `-c approval_policy="never"`.
- V1-0161: L3 now expects the `cem-status unsupported-without-basis` issue code the rerun change
  step records, L5 the failing change row `local-outcome:outcome-input-not-provided`, L6
  `cem-cite:citation-plan-not-provided`. The harness reads those rows from the change step's output
  and the issue codes from `.git/corvint/cem-status.json`. A case whose expected refusal another
  case shares is recorded `discriminating: false`: L1 and L2 (both `dogfood-check: REFUSE
  dirty-worktree`). L3c records the case it controls and its discriminating state. No
  requirement was added; the harness is unsealed (PCCO-V0-015..017 stay proposed).

## Evidence

- Probe, codex-cli 0.162.0, no network: `codex sandbox -c sandbox_mode='"read-only"' -- /bin/sh -c ...`
  showed `CODEX_SANDBOX=seatbelt`, a denied write under `/private/tmp` ("Operation not permitted"),
  and a parent `CORVINT_TASKS_READ_ONLY=1` inherited inside the sandbox.
- Focused tests pass: `TestATRV0210_*`, `TestATRV0211_*` (`internal/tasks/store`),
  `TestATRV0211_ClaimResultListsEffectiveScope`, `TestATRV0212_ReadOnlySessionRefusesMutations`
  (`internal/tasks/cli`). Each fails with its fix disabled: the refusal (store and cli tests),
  `report.Scope` (store and cli), and the `readOnlyRefusal` hook (cli).
- Harness: `py_compile`, and a scratch exercise of `refusal`, `status_issues` and `expect` on
  synthetic output; `NOT_DISCRIMINATING` is exactly L1 and L2 (it held L3, L5 and L6 before).

## NOT_RUN

- A live `codex exec` review (network service); the escalated child's inheritance of the variable is
  inferred from Codex's default `shell_environment_policy`.
- A live daily-loop rehearsal: the L3/L5/L6 expectations are derived from
  `internal/dogfoodflow/change.go`; a mismatch shows up as `refusedAsExpected: false`.
- Linux; `make gate`.
- V1-1441 criterion 3, the owner's review of the V1-1375 operator note, stays with the owner.

## Rollback

Revert the commit. Claims again drop uncovered requested paths silently, results lose `scope`, the
variable has no effect, and the harness goes back to the shared expectation. No stored record or
receipt changes.
