# V1-1562: context notice verb and detached hand-off detail

Date: 2026-10-17. Base: `072a306f1dbd89c2f29b20a3434a0f25864ebbf7` (origin/main).
Ticket: V1-1562, from the residuals of V1-1549
(`docs/build-log/2026-10-17-v1-1549-dispatch-context-thresholds.md`). Spec:
`docs/specs/corvint-tasks-attempt-runner-v0.md`, ATR-V0-056..057, accepted under the owner's
standing auto-accept of lane requirements (2026-10-11). The failure-mode row for the context
record gains a cross-reference only.

## Decisions

- Read verb. `dispatch status --worker ID` was chosen over `attempt show`. The record belongs to
  the dispatcher's state directory, and a worker already has `CORVINT_DISPATCH_PROGRAM` and
  `CORVINT_DISPATCH_WORKER` in its environment. The ID must name a ledger worker, which also keeps
  it from naming a path outside the state directory.
- Marker version. The strict `/0` marker decoder of an earlier build removes a marker with an
  unknown member, which would lose the deferral. A marker of another version is held unread
  instead (CAL-V0-147). So the `context` member comes with a profile bump to `/1`, and `/0` stays
  readable. The CAL-V0-131 live set lists `/1`, and its format-version test now writes `/2`.
- The detail is the same three members the attached path records (`reason`, `contextTokens`,
  `hardTokens`). The decoder accepts exactly that closed shape.

## Evidence

- `TestATRV0056_DispatchStatusWorkerShowsContextNotice` (`internal/tasks/cli`) covers the notice,
  its crossings, that the record is unchanged, that the member is absent without `--worker`, and
  that an unknown ID is refused.
- `TestATRV0057_DeferredContextHandoffSurvivesRestart` (`internal/tasks/dispatch`) shows that the
  marker records the detail. After a reopen, with the worker gone from the ledger and no
  in-memory detail, the hand-off still carries `reason=CONTEXT`, `contextTokens` and
  `hardTokens`.
- `TestATRV0057_MarkerVersionsAndContextMember` covers decode acceptance and refusal per version
  and context shape. A later version is refused as `UNSUPPORTED_VERSION`.
- `TestCALV0147_MalformedMarkerIsRemoved` now uses `/2` as the other version.
- Linux: NOT_RUN.
