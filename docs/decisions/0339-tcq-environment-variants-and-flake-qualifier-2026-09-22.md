# Decision 0339 — TCQ environment variants and same-revision flake qualifier

Date: 2026-09-22 (UTC). Status: accepted; experimental delivery under `TCQ-V0-048..050`.
Ticket: `V1-0091`. Assigned number 0323 was already taken by
`0323-gate-affected-change-evidence-readers-2026-09-19.md`; this record uses the number the coordinator reassigned after 0321 to 0333 were taken on origin/main.

## Context

Flake detection lived only in the JS test provider, which trusted the Playwright reporter's
`flaky` label, and a test observation carried no record of the environment it ran under. Two runs
of one test at one tree SHA with different outcomes therefore produced either a clean
`test-report-matched-v0` relation or a failed row, depending on which run the caller supplied, and
two observations from different platforms could not be told apart from two runs on one.

## Decision

1. An observation MAY declare an environment variant as ResultDB-style name/value pairs
   (`TCQ-V0-048`). The member is optional and additive: an undeclared environment is unknown, not
   empty; it is reported as undeclared through the library result and adds no wire member, so every
   frozen conformance vector stays byte-identical and old consumers read new documents unchanged.
   TCQ still reads and persists no process environment; `TCQ-V0-024` is untouched.
2. One shared flake rule, `tcq.Flaky` (`TCQ-V0-049`), decides flakiness for every provider: more
   than one distinct terminal status across two or more runs at one target and variant. The JS
   provider now derives its Playwright state and its `flaky-retry` reason from this rule over the
   recorded attempts; a reporter label can only add the qualification, never remove it. The JS
   provider keeps its existing `flaky-retry` reason label because that label is governed by
   `docs/specs/js-live-test-provider-v0.md`, outside this change.
3. A caller MAY pass up to 16 prior observations of the same target (`TCQ-V0-050`). A claim whose
   execution key diverges across comparable observations adds reason 18 `test-flaky` and loses its
   relation while keeping its own report state. The reason is reachable only through priors, which
   the frontier shim never supplies, so the closed CF-V0-016 vocabulary and the frontier seam test
   are unchanged; extending them is a separate decision.

## Alternatives weighed

- Deriving flakiness from repeated rows inside one report: rejected, `TCQ-V0-034` already treats
  repeated rows as ambiguity and one report cannot show two runs.
- Emitting `environment: {}` when undeclared: rejected, it would change frozen vectors and let an
  unknown environment compare equal to a declared empty one.
- Storing flake history inside Corvint: rejected, the caller owns prior observations; TCQ stays a
  pure function of its inputs.

## Rollback

Drop `Request.PriorObservations`, the observation `environment` member, and reason 18. No frozen
vector carries either, and the JS provider falls back to the reporter label it used before.
