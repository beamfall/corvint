# 2026-10-21 tbugs-1021: shared spec-index reservations and the version-smoke blind interval

Lane tbugs-1021 (base `f284484ebc613c8b61995f7af3ac53d170973be1`). Tickets V1-2119 and V1-1651
delivered; V1-1568 skipped. Requirements PGO-V0-018 and NTP-V0-026 are accepted under the owner's
standing auto-accept (2026-10-11).

## Problem

- V1-1651: `corvint-update check --component core` failed its version smoke with "descendant
  snapshots were unavailable for 6.079s, beyond the 5s observation bound". Each periodic descendant
  snapshot may spend its 2.5 s timeout before it is lost, so two consecutive lost snapshots always
  exceeded the 5 s blind-interval bound on a loaded host, even when the smoke itself succeeded.
- V1-2119: a parked attempt (V1-0812) listing `docs/specs/REQUIREMENTS.tsv`, `INDEX.json` and
  `README.md` refused an unrelated lane's (V1-1639) widen RESOURCE_COLLISION, although those files
  are generated and regenerated at merge.

## Decisions

- PGO-V0-018: `descendantMaxGap` is four snapshot timeouts (10 s): three lost snapshots and a slow
  fourth are tolerated, a fourth loss still fails closed with the same error. Lost snapshots stay
  reported as limitations. No flag, env or field was added; a fixed robust bound keeps the
  fail-closed behaviour deterministic. Limit: the window in which an undetected detachment can hide
  grows from 5 s to 10 s.
- NTP-V0-026: the store policy declares `reservations.sharedPaths` once; overlapping PATH keys do not
  collide only when shared paths cover both sides. Hand-authored paths and uncovered directory keys
  stay exclusive. One function (`ticket.CollideShared`) serves claim/widen, expired-lease blocking
  and plan selection, so the plan and the lease writer agree. The rule lives in the planning spec
  that already owns `plan batches --shared-path`; no new CAL-V0 IDs.
- V1-1568 skipped: its uncovered paths route through `prepareLease`/`commitLease` and the journal
  resume fallback, which open PR #1415 (JSG segments) rewrites. Editing them here would conflict.

## Evidence

- `TestV11651VersionSmokeSurvivesTwoLostSnapshots` fails at the base with "unavailable for 6.06s,
  beyond the 5s observation bound" (the reported message) and passes with the fix; the existing
  AHI-032 beyond-bound case still fails closed.
- The four NTP-V0-026 tests fail with the shared-path rule neutered (unit and store regression) and
  pass with it; collision, widen, plan and CAL-V0-131/import-direction traps pass.

## NOT_RUN

- The reporter's live `corvint-update check --component core` rerun (needs a released build).
- The live `.taskman/policy.json` declaration: after merge the orchestrator must run a policy update
  adding `"reservations":{"sharedPaths":["docs/specs/INDEX.json","docs/specs/README.md","docs/specs/REQUIREMENTS.tsv"]}`
  with a rebuilt corvint-tasks; until then behaviour is unchanged.
- `make gate` (not required for scoped work).

## Rollback

Revert the commit. For NTP-V0-026, removing the policy key alone restores exclusive semantics; no
store migration is needed. For PGO-V0-018, restoring the 5 s bound is a one-line revert.
