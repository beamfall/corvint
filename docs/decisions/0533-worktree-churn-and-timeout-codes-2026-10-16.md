# Decision 0533 — Worktree churn and timeout refusals get their own codes

Date: 2026-10-16. Status: accepted. Authority: Owner delegated to the orchestrator on 2026-10-16; decided by the orchestrator.
Tickets: V1-0880, V1-1432. Supersedes, for the native surface's repository-drift code only, the
"`dogfood-event-unavailable` refusal ... as before" sentence that decision 0439 accepted in
`LCP-V0-016`.

## Context

`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-15-v1-1432-timeout-sites.md` (PR #968) kept three refusal mappings unchanged,
because changing them amends accepted rows, and listed them as owner decisions:

1. **V1-0880.** A dogfood event whose read is refused because the worktree moved
   (`repository-state-unstable`, `dogfood-event-repository-drift`, `dogfood-event-context-drift`)
   reached the native surface as the opaque `dogfood-event-unavailable`. Host adapters then
   recorded the same code for genuine read failures and for churn.
2. **V1-1432 sites (2) and (2b).** The MCP bridge reported a CEM Git read that ran out of its own
   bound (`cemcode.GitTimeout`, `cemcode.GitDiffTimeout`), and a context-index refusal caused by its
   own Git deadline, as `repository-unavailable`. That made a slow, healthy repository look missing.
3. **V1-1432 site (3).** The go-live provider's deferred release on an early exit mapped every
   release failure to `ErrAuthorityDrift`, including one whose only failure was its own expired
   deadline.

## Decision

1. **`dogfood-event-worktree-churn`.** The event surface maps three read refusals to a new fixed
   code, `dogfood-event-worktree-churn`: `repository-state-unstable`,
   `dogfood-event-repository-drift` and `dogfood-event-context-drift`.
   `dogfood-event-policy-drift` passes through unchanged. Every other read failure stays
   `dogfood-event-unavailable`. The code joins the `SOL-V0-010` closed adapter-rejection registry,
   so a ledger row keeps it. Amends `LCP-V0-008` and `LCP-V0-016`.
2. **MCP Git timeouts.** These reuse the existing `MCPV0-038` code `repository-probe-timeout`:
   `cemcode.GitTimeout`, `cemcode.GitDiffTimeout`, and a `contextindex` refusal whose typed cause
   is `context.DeadlineExceeded` while the request context is live. Caller cancellation or a caller
   deadline keeps precedence and stays `cancelled`. Every other Git failure keeps
   `repository-unavailable`. No new code is added. Amends `MCPV0-025` and `MCPV0-038`.
3. **Early-exit release.** The deferred release on an early exit is classified as one post-run
   call (`postRunFailure`). If its only failure is its own expired deadline, it reports
   `ErrAuthorityUnavailable: release execution authority deadline expired`. A typed drift report,
   even one returned late, any failure returned in time, and any release failure on an exit that
   already observed drift keep `ErrAuthorityDrift`. Every case
   stays `INCOMPLETE`, so the refusal remains fail-closed. The pre-launch `go-live-error/0`
   detail can move from `PARENT_AUTHORITY_DRIFT` to the existing
   `PARENT_AUTHORITY_UNAVAILABLE`. Amends `GLTP-V0-054`.

## Limits

- The docs bridge (`SDD-V0-011`, PR #968) names its index deadline `repository-index-timeout`.
  This decision keeps the main MCP bridge on the existing `repository-probe-timeout`, so the two
  surfaces use different names for the same class of timeout.
- `dogfood-event-worktree-churn` is not a time bound. `conformance/host-lifecycle-v1`'s time-bound
  set is therefore unchanged.

## Rollback

1. Revert this decision, the amended sentences in `LCP-V0-008`, `LCP-V0-016`, `MCPV0-025`,
   `MCPV0-038` and `GLTP-V0-054`, and their evidence rows.
2. Revert `dogfoodReadFailureCode` in `cmd/corvint/local_completion_event.go`, the registry entry
   in `internal/observations/observations.go`, the bridge's `cemFailures` timeout rows and
   `indexDeadline`, and the provider's early-exit `postRunFailure` release.
3. Regenerate `docs/specs/REQUIREMENTS.tsv`.

Nothing persistent depends on the new code. Ledger rows that already carry it are bounded and
private, and degrade to `corvint-event-rejected` once the registry entry is removed.
