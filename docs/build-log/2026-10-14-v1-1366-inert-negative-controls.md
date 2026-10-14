# 2026-10-14 V1-1366: inert negative controls are NOT-APPLIED, not strength evidence

Lane V1-1366 (GitHub #872), Corvint 1.0 scope `PRS-V1-018`. Requirements `BBF-V0-015..017`
(`docs/specs/browser-behavior-falsification-v0.md`), accepted under the owner's standing
auto-accept (2026-10-11). Base adf587fdb65f4d87dc94a5e374a4fda1f87c5e7f.

## Intent

A hand-authored control can be inert. For example, a `page.route` guard can always fall back, so
the perturbation never happens and the run stays green. Today such a run reads as "the control did
not break the test", which is weakness evidence the control never earned. A control can now
declare an application marker. A run without that marker is `NOT-APPLIED`; it is never credited as
a kill or a survivor, and the strength axis abstains. The report lists inert controls separately,
and a control that stays inert across revisions is flagged as a control defect.

## Decisions

- **The owner is BBF, extended in place.** Control runs are classified in
  `internal/behaviorfalsify` (`classifyAttempt`). `internal/playwrightruns` only reads native
  Playwright runs, and `internal/stepnegation` owns generated step negations, not hand-authored
  controls. The marker check is a final stage of the existing classifier, not a parallel one.
  `internal/testacceptance` is changed only because it owns the strength axis that consumes BBF
  receipts.
- **There is no new status.** The BBF rule says a status-vocabulary change needs a new profile or
  an explicit compatibility rule, so the six statuses stay unchanged.
  - `NOT-APPLIED` is `invalid_control` with reason `control-not-applied`. That status already
    sits outside the mutation-score denominator and is already a coverage gap.
  - The four application values are a separate optional `application` field on attempt and
    control results.
  - `APPLIED-EARLY-FAIL` covers a marked attempt whose required setup failed. It stays
    `invalid_control`.
- **Compatibility rule (BBF-V0-016).** Every new member is optional and omitted when empty.
  - A plan, report or receipt without a marker is byte-identical to its prior form, so plan
    digests and legacy evidence receipts still verify. `TestBBFV0016LegacyWireUnchanged` checks
    this.
  - The marker is in the plan digest but not in the perturbation digest. Hook semantics are
    unchanged.
- **Applied means this control's own marker.** An observation counts only when its kind, type and
  `control_id` all match the control.
  - Duplicate observations count once.
  - A marker from another control, or of another type or kind, does not apply.
  - The observation list is bounded at 64 entries and validated, and `acceptedReceiptBound` grows
    by three strings per allowed observation.
  - A kill without the marker is also `NOT-APPLIED`, because the failure cannot be attributed to
    the perturbation.
- **"Unobserved" for unmarked controls is recorded by the consumer.**
  - Putting it in BBF report/0 would change legacy receipt verification. Instead, the
    acceptance consumer records `application_unobserved` (unmarked control IDs) and `not_applied`
    (the receipt's `inert_controls`) on its control record.
  - An inert control adds `negative-control-not-applied` and sets Strength to `NOT_MEASURED` with
    that reason, on both the /0 and the fresh path.
  - An unmarked control leaves the assessment byte-for-byte as before, as the charter requires.
- **Cross-revision history (BBF-V0-017).** `InertControlDefects` takes up to 64 caller-supplied
  receipts, each with its pinned plan and tool digests. No store is read or created.
  - Every receipt must pass `VerifyReceipt`, otherwise the history fails with
    `history-receipt-invalid`. An empty or oversized history fails with `history-bound-exceeded`.
  - "Same control" means the same contract, criterion, assertion, test, project, control ID, kind
    and marker. Hook and perturbation digests are excluded so that a control edited only in its
    hook bytes stays one identity.
  - A revision is the (application, test) revision pair.
  - A control `NOT-APPLIED` at two or more distinct revisions is flagged `repeated-not-applied`.
    Repeats within one revision are not flagged.

## Failure modes covered

- A marker is present but the target step was skipped.
- A marker is emitted twice.
- A marker comes from a different control, or has another type or kind.
- An early setup failure occurs, with or without a marker.
- Marker observations are malformed or exceed the bound.
- An infrastructure loss occurs (it keeps its class).
- An invalid plan marker is declared.
- A receipt is tampered with: the inert list, the control or attempt application, an inert
  control reclaimed as a survivor, or a marker injected into the bound outcome.
- A cross-revision repeat occurs, versus a same-revision repeat.
- The marker is changed, which makes a different identity.
- A history receipt is tampered with or has a stale plan digest.
- The history is out of bounds.

## Acceptance evidence

- `TestBBFV0015*`, `TestBBFV0016*` and `TestBBFV0017RepeatedNotAppliedAcrossRevisions` in
  `internal/behaviorfalsify`.
- `TestBBFV0016InertControlAbstainsOnStrength` and
  `TestBBFV0016ControlApplicationSeparatesInertAndUnobserved` in `internal/testacceptance`.
- Observed on this host at load average ~60:
  - The new BBF tests pass run alone (`-run 'BBFV001[5-7]'`, 18.8s), and so does the new NEA test
    under `-race`. `go vet` on the four touched packages is clean, `gofmt -l` prints nothing, and
    the 12 CI doc gates pass.
  - The full BBF and NEA package runs and the BBF `-race` run (twice) fail only on
    `descendant snapshot unavailable`. The pre-existing `BBFV0004`, `BBFV0013`, `BBFV0014` and
    `NEAV0006`/`0010` tests hit the same infrastructure loss. In the failing new-test runs the
    report counts carry `infrastructure_failed`.

## Independent review

One read-only Codex round produced one medium finding. `APPLIED-EARLY-FAIL` was assigned for any
failed setup observation, including one the control does not require. Fixed: only a failed setup
that the control lists in `required_setup` counts as an early failure. The hostile row
`undeclared failed setup is not an early failure` covers this. There were no other findings.

## Limits and residuals

- **Markers are hook-reported.** They are observations (BBF-V0-012), not independently parsed from
  native Playwright annotations or provider events. A dishonest hook can still claim application.
- **The CLI is not exposed.** `InertControlDefects` is a package API.
  `cmd/corvint-behavior-falsify` has no history subcommand in this lane because that is outside
  the charter scope.
- **Optional item 4 is deferred.** Recording the observed request shape (keys only) when a guard
  falls back is not done. It needs a new hook-receipt member with secret screening, so it is not
  small.
- **A live Playwright control is NOT_RUN.**
  - The repository has no BBF Playwright hook harness; live adopter evidence is already
    `NOT_OBSERVED` in BBF.
  - The conformance fixture has no installed `node_modules`, and installing one would need a
    network download outside this lane.
  - The host Node is v22.23.3, not the V1-0976 v24.11 tuple.
- **Load-sensitive live tests.** Under host load, `procgroup` can report
  `descendant snapshot unavailable`. That makes a live attempt `infrastructure_failed`, and a
  retained infrastructure-failed attempt with stdout does not verify, which predates this change.
  Like the existing live tests, the new live receipt tests depend on a host where descendant
  snapshots succeed.

## Rollback

Revert the change. Plans, reports and receipts without markers never carried the new members, so
the revert affects no existing evidence. Plans with markers would be refused as unknown fields by
the prior closed decoder.
