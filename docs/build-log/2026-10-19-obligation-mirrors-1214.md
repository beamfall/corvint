# 2026-10-19 V1-1966: obligation mirrors and the #1177 unblock follow-up

This lane implements GitHub beamfall/corvint#1214 (V1-1966) in the corvint-tasks obligation ledger.
An obligation on ticket A can be declared satisfied by an obligation on ticket B. A derives its
state from B at read time. The dispatcher stops offering a ticket whose open items are all decided
elsewhere. It also carries the GitHub #1177 follow-up: unblocking an item releases a parked ticket,
and doctor names a parked ticket that still has open items. Spec:
`docs/specs/corvint-tasks-obligation-ledger-v0.md` TOL-V0-120..125, plus a doctor remedy row in
`docs/specs/corvint-tasks-doctor-v0.md`.

## Acceptance

- TOL-V0-120..125 are agent-drafted and accepted under the owner's standing auto-accept of lane
  requirements (2026-10-11). Delivery is experimental.
- Each acceptance criterion of #1214 maps to a test:
  - B witnessed → A's mirror reads `WITNESSED` with B's event as `source` and no review:
    `TestTOLV0121_ResolveMirrors`, `TestTOLV0120_MirrorReadsAndAcceptance`.
  - A ticket with only decided mirrors is excluded from dispatch and listed as completable:
    `TestTOLV0123_MirrorsCompletable`, `TestTOLV0123_MirrorsDecidedLeaveWindow`,
    `TestTOLV0123_MirrorsDecidedIsNotDispatched` and the CLI acceptance test.
  - A missing or renamed owner obligation is reported (`OWNER_OBLIGATION_MISSING`,
    `MIRROR-OWNER-UNRESOLVED`), not silently treated as open: `TestTOLV0121_ResolveMirrors`,
    `TestTOLV0122_MirrorFindings` and the CLI acceptance test.
- The #1177 follow-up maps to `TestTOLV0124_UnblocksCounted`,
  `TestTOLV0124_UnblockReleasesParkedTicket` and
  `TestTOLV0124_UnblockObservedAndDoctorReportsParked`.

## Decisions (made in the lane, no owner question asked)

- **Where the mirror lives.** `satisfiedBy` is one optional member on the `set` change and the
  folded entry, a closed `{ticketId, id}` object; `null` clears it. Ledgers without it keep their
  exact bytes, which follows the pattern of `candidate` (TOL-V0-084). There is no new verb and no
  new event type.
- **No write-time owner check.** Owners can be created, renamed or removed after the mirror is
  written, so a write-time check would still go stale. Reads report the owner's status. Only a
  self-mirror is refused at write, as `OBLIGATION_MIRROR_SELF:`, because it can never resolve.
- **Derivation is read-side and one hop.** The stored ledger state, its counts and its high-water
  mark are untouched, so witness credit, the TOL-V0-017 fingerprint and audits keep their
  meaning. One hop keeps resolution bounded and makes cycles harmless: an owner that is itself only
  a derived mirror reads `OWNER_UNDECIDED`.
- **What counts as decided.** The owner entry must be `WITNESSED` or `DEFERRED`, and the local
  entry must still be pending. `DEFECT` and `BLOCKED` are not terminal, so they leave the mirror
  open with `OWNER_UNDECIDED`. A locally terminal entry is never overridden.
- **Source, not review.** A derived item's `source` is the owner's evidence `eventSha256` (null for
  a deferral). The mirror carries no review of its own, as #1214 asks.
- **Missing-owner reasons.** There are four: `OWNER_TICKET_MISSING`, `OWNER_LEDGER_MISSING`,
  `OWNER_LEDGER_UNKNOWN` and `OWNER_OBLIGATION_MISSING`. A renamed owner obligation reads as
  `OWNER_OBLIGATION_MISSING`. An owner in another queue is not resolved and reads as
  `OWNER_TICKET_MISSING` (a retained limit). `OWNER_UNDECIDED` is a normal wait and not a finding.
- **Where findings appear.**
  - `show` adds a `mirrors` member.
  - `queue status` adds `obligationMirrors`.
  - `worklist` adds `findings`, and excludes decided mirrors with `MIRROR_DECIDED_ELSEWHERE`.

  Each member appears only when non-empty, so existing outputs are byte-identical. Status and
  worklist fold the owner ledgers on each read. The dispatcher caches folds by ticket and head.
- **Dispatch.** A completable ledger (at least one decided mirror, and every pending entry decided)
  is deferred `MIRRORS_DECIDED_ELSEWHERE`.
  - The deferral is dispatcher-only, like `WORK_STATE_HELD`, so `plan preview` and `claim` behave
    as before.
  - The roster skips the ticket for every role, including roles without a `planSelected` focus.
  - The dispatcher emits one `needs-owner` event when the reason first appears. The owner, not a
    worker, completes the ticket.
- **Unparking (#1177).** The fold counts entries leaving `BLOCKED`. The fingerprint adds
  `obligations-unblocked|<n>` only when n > 0, which extends TOL-V0-017 additively. The existing
  CAL-V0-057 unpark therefore releases the key, and no new unpark path is needed. An unknown fold
  never unparks a key or counts as progress.
  - Rollout effect: a ticket with a past unblock changes its fingerprint once under the new binary,
    which at most releases a parked key one time. This is benign.
- **Doctor.** The new finding is `PARKED_WITH_OPEN_ITEMS`: one per parked key of a configured
  dispatcher whose ledger ticket has open (non-deferred, unwitnessed) items. It uses the record's
  reference counts, so the doctor folds no chain.
- **Read-only.** None of these reads writes the store.
- **IDs.** The IDs are TOL-V0-120..125. TOL-V0-100..109 already exist on main (#1212), so there is
  no collision.
