# 2026-10-07: scoped WORKER grant for know-how ADD (V1-0987)

## Intent

GitHub issue beamfall/corvint#671 (ticket V1-0987) asks that the agent holding a claim be able to
record know-how on the ticket it is working, without an owner relaying it. The amendment is
specified in `docs/specs/corvint-tasks-know-how-notes-v0.md` as KHN-V0-016..018, which are proposed
and await owner acceptance. Delivery stays experimental.

## Decisions

- **Opt-in key.** Policy gains the optional closed member `knowHow: {workerAdd: boolean}`. Absent
  or false, a WORKER add is refused exactly as before: UNAUTHORIZED, detail
  `outside hypothetical role subset`, nothing written. True appends `KNOWHOW_ADD` to WORKER's
  default row only. An explicit `policy.roles.WORKER` row still outranks it, and a roles row naming
  `KNOWHOW_ADD` for WORKER stays undecodable, so roles rows still only remove operations.
- **Scope.** The add must name an attempt and generation. The transaction layer reads the
  ticket's attempt under the store lock (the store audits `attempts/` for this one request shape)
  and passes a small observation to `mutation`. The WORKER-only check
  (`workerKnowHowScope`) refuses, in order: supersede; missing attempt or generation; an absent,
  ended, expired or other-generation attempt; a lease holder other than the actor; an attempt on
  another ticket; an anchor outside `effects.touchPaths` (KHN-V0-006 matching). Each case reuses a
  closed §11 code and has a stable `KNOWHOW_WORKER_*` detail prefix. Details name fields, never
  values, because the secret screen runs afterwards. Caps and the secret screen are unchanged.
- **Policy prefixes.** Not implemented. The schema has no natural place for them and touchPaths
  already bound the claimed work; they remain a possible extension.
- **Record.** The entry records role WORKER plus the verified attempt and generation. The codec and
  the Core reader admit WORKER only on such a non-superseding ADD.
- **Admission order.** WORKER `KNOWHOW_ADD` is now admitted into the transaction model, which
  checks the canonical policy under the lock. The store first screens it before the lock against
  the committed intent tree's policy, so a disabled grant is refused before the store checks and
  §5.2 recovery, as before (the independent review found that the first version ran recovery
  first). The writer-checkpoint route declines the request because it models without attempts.
  Trade-off: once the key is removed, a retry of a committed WORKER add is refused, not replayed.

## Decisions that need amending

- Decision 0443 answered owner question 4 with "no WORKER grant". Accepting KHN-V0-016..018 needs
  an amendment recording that WORKER may hold a scoped, policy-opt-in ADD and still never
  supersedes or retracts.
- Decision 0444 assigns attempt/generation verification for every know-how writer to V1-0964.
  This slice verifies only WORKER adds; the amendment should record that V1-0964's generic check
  subsumes the WORKER attempt observation and keeps the WORKER holder, ticket and anchor checks.

## Limits

- A change to the `knowHow` key fences open handoffs (STALE_POLICY) like any policy change; it is
  not in `handoffIgnoredKeys`.
- No concurrency witness for a lease that expires during the add; the check reads the attempt under
  the same lock as the write.
- Rollback: remove the key. An older binary refuses a policy with `knowHow` and a record holding a
  WORKER entry.
