# Tasks submit --keep-reservation and widen-after-release warning (V1-5601)

Ticket V1-5601 in `queue:corvint:main`, filed by the Beamfall 1.0 orchestrator on 2026-11-15
against corvint-tasks 1.0.0-rc.3.dev+build.146. The work is in beamfall/corvint-tasks.

## Diagnosis

The reported "widen narrowed a 56-key scope to X" was V1-5595's automatic release (corvint-tasks
`CAL-V0-391`): a verify-only resubmit of an integrated tree emptied the attempt's path scope, so the
later `widen --scope X` extended an empty scope, and another agent's claim took the released keys.
`widen` itself unions.

## Owner decision (2026-11-15)

Keep the automatic release; add `submit --keep-reservation` for an attempt that keeps building
(a lander stack that takes a repair commit after a red gate); make a widen of a released attempt
warn with the released keys and receipt; make `RESOURCE_COLLISION` name the holding attempt and
keys. V1-5601's original criterion 2 (always refuse a claim overlapping a SUBMITTED attempt)
conflicts with V1-5595 and was dropped; the ticket's acceptance was rescoped at revision 2.

## Behavior

- `submit --keep-reservation`: scope-checked, a same-tree resubmit is a no-change, the flag joins
  the request digest only when present, and other verbs refuse it `MALFORMED`.
- `WIDEN_AFTER_RELEASE`: a completed or refused widen of an attempt whose path reservation an
  earlier submit released names the release receipt and up to 20 released keys. The lookup reads
  at most 4096 receipts and never changes the outcome.
- Governing requirement: corvint-tasks `CAL-V0-394`.

## Evidence

- Delivered in https://github.com/beamfall/corvint-tasks/pull/8 (commits `bbe29bb`, `7a7afbf`).
- Tests: `TestCALV0394_KeepReservationHoldsAnIntegratedSubmit`, `TestCALV0394_WidenAfterReleaseWarns`
  (store); `TestCALV0391_VerifyOnlySubmitReleasesCollisions` (cli). Same runs and review as the
  V1-5567 entry.
- Installed locally as corvint-tasks 1.0.0-rc.3.dev+build.148.
