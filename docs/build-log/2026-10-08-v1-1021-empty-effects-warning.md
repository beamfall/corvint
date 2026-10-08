# 2026-10-08: unbounded effects warning and serial-fallback deferral (V1-1021)

## Intent

Issue 679 (native ticket V1-1021) reports that `ticket create` with empty effects, which is the
CREATE template's default, commits silently. The planner and claim then give that ticket the
WHOLE_REPOSITORY fallback scope, so it serializes against every other attempt, and no read said
why it was deferred. The change adds two proposed requirements to
`docs/specs/corvint-tasks-agent-leases-v0.md`, pending owner acceptance:

- CAL-V0-192: an `EFFECTS_UNBOUNDED` warning on create, refine and set-effects.
- CAL-V0-193: a `serialFallback` entry member and a `serialFallbackDeferred` list in plan preview,
  plus the same list in queue status.

## Decisions

- **Warn, do not refuse.** `ticket create` still commits. The warning is an envelope `warnings`
  entry added by the CLI after a fresh transaction, following the CAL-V0-186 precedent. Replays,
  no-change results and refusals add nothing. No result code, receipt, journal record or replay
  rule changes.
- **The planner's own predicate.** The warning fires when `transaction.UnboundedEffects` holds:
  `externalUnbounded` is false and no PATH scope is declared under QUALIFIED coverage. This is the
  same condition under which `planEntry` and claim fall back to WHOLE_REPOSITORY. It is broader
  than "touchPaths and resources both empty" because coverage UNKNOWN with touchPaths, and
  QUALIFIED coverage with only non-PATH resources, also reserve WHOLE_REPOSITORY.
  `externalUnbounded` tickets are ineligible (EXTERNAL_UNBOUNDED), so they get no warning.
- **Policy value named.** The text names the policy `serialFallback`. Claim and plan skip the
  COVERAGE_UNKNOWN blocker, so an external claim takes WHOLE_REPOSITORY under both values.
- **Deferred only by the fallback.** An entry is marked when all of these hold:
  - `choose` defers it for RESOURCE_COLLISION;
  - its own closure is incomplete, so its scope is the fallback;
  - no reservation or earlier selection holds WHOLE_REPOSITORY or collides with the entry's
    non-PATH resources;
  - capacity would admit one more attempt.

  Otherwise declaring a PATH scope would not admit the entry, so it is not marked. The independent
  review found the capacity and non-PATH conditions missing from the first version; the
  `spare capacity then exhausted` and `kept non-PATH resource collides` subtests failed against
  that version (`listed with capacity exhausted`, `listed behind a DATABASE collision`) and pass
  with the fix.
- **Additive output.** The plan members appear only when an entry qualifies, so existing plans
  render byte-identically. `queue status` always carries `serialFallbackDeferred`: an array, or
  null without a journal. It runs the planner only when an OPEN or HELD ticket has unbounded
  effects. The `--summary` key sets gain the member, which amends CAL-V0-167 in the same way that
  CAL-V0-184 did.

## Evidence

Focused tests in `internal/tasks/cli`: `TestCALV0192_UnboundedEffectsWarn`,
`TestCALV0193_SerialFallbackDeferralIsReported` and `TestCALV0167_SummaryShapes`.

- Failing before (source stashed, tests kept):
  - `TestCALV0167_SummaryShapes` failed because the summary key set lacked
    `serialFallbackDeferred`.
  - `TestCALV0192_UnboundedEffectsWarn` failed with `empty effects warnings = []`.
  - `TestCALV0193_SerialFallbackDeferralIsReported` failed with
    `serialFallbackDeferred = null`.
- Passing after: all three pass. The focused packages also pass:
  - `internal/tasks/transaction`
  - `internal/tasks/mutation`
  - `internal/tasks/ticket`
  - `internal/tasks/store`
  - `internal/tasks/cli`
- Live check: a corvint-tasks binary built from this branch, including the review fix, ran
  against a scratch store in a fresh `git init` repository under the lane's temp directory. It used
  the fixture queue and policy (serialFallback BLOCK) with `maxActiveAttempts` 2.
  - Creating a ticket with touchPaths `["src/"]` (P1) returned only the actor-binding warning.
  - Creating a ticket with empty effects (P3) also returned
    `EFFECTS_UNBOUNDED: ticket ticket:acme:main:AT-0005 ... (policy serialFallback BLOCK) ...`.
  - `queue status --summary` reported `"serialFallbackDeferred":["ticket:acme:main:AT-0005"]`.
  - `plan preview --summary` marked AT-0005 `RESOURCE_COLLISION` with
    `"serialFallback":"WHOLE_REPOSITORY"`.

## Non-goals

- `ticket create --effects-from` or any other effects derivation (out of scope per issue 679).
- Refusing an unscoped create, or an `--allow-whole-repository` opt-in.
- Warnings on batch `ticket refine --from-file`, admin console verbs or Core imports.
- Changing the serial fallback or plan selection.
- `plan preview --selected-only` output.
- The dispatcher.

## Failure modes and rollback

- The REFINE warning reads the pre-write record, so a concurrent effects change can make it stale.
  The warning is advisory.
- Rollback removes the warning, the two plan members and the queue status member. No store,
  journal or receipt state depends on them.

## Not run

- Live fleet or dispatcher qualification: NOT_RUN.
- Linux: NOT_RUN.
- Repository-wide `go test ./...` and `make gate`: NOT_RUN (shared host; focused packages only).
- Owner acceptance of CAL-V0-192..193: pending.
