# Decision 0544 — Dispositions for six 1.0 forks delegated to the orchestrator

Date: 2026-10-20. Status: accepted. Authority: on 2026-10-20 the owner delegated these named forks
to the orchestrator ("you decide on those, keep going"). The delegation covers the six forks below
only. It is not a standing grant, and it does not cover tagging, publication, promotion or
hosted release-gate dispatch.

## Context

On 2026-10-20 four read-only audits looked at the open `v1-0` tickets on `origin/main`
c27d54e92. They found six open forks that block 1.0 closeout and need an owner call. This record
settles all six.

## Decisions

1. **Signing for `1.0.0-rc.3`.** An rc.3 candidate uses the same signing policy as decision 0433:
   "No signing: SHA256SUMS only; publisher identity NOT_VERIFIED". SRR-V1-009 already makes the
   operator supply the signing selection for any version it does not fix. For rc.3 the operator
   supplies `policy/signing` as NOT_RUN, decision 0544, with that reason. No code change is needed.
   Before 1.0.0, a later candidate either reuses this record or names its own.
2. **V1-0464, the Beamfall roadmap takeover.** The activation is the Core cutover recorded in
   Beamfall ADR-0227 (core-history commit 0f494e72b, 2026-10-19). It made Core's native Tasks
   queue the authority, imported the 807 open and partial items as native tickets, and deleted
   the dispatcher scripts. These deviations from the V1-0464 criteria are accepted:
   - C2: the 3,094-record history export with preserved IDs was not imported.
   - C3: there was no rehearsal.
   - C5: there was no recovery demonstration over a frozen export.

   V1-0436 (re-measuring the full import) is off the 1.0 path and its dependency edge is dropped.
   Two criteria remain:
   - C6: one native claim, submit, `gate run` and complete on a Core ticket, followed by a readback
     and a `receipt audit`.
   - C4: every read-only consumer that has not been rewired is filed as a Core ticket.
3. **V1-1038, the promisor timeout class.** The closed MCPV0-028 `reasonClass` set from decision
   0383 stays unchanged. The decision 0383 amendment is declined. If a promisor-specific signal is
   needed later, it is added as a closed `code`, following the V1-1427 precedent. V1-1038 is closed
   with no code change.
4. **Core finish-gate transcript ticket (CORE-FINISH-INTERRUPTED-TRANSCRIPT-PRESERVATION-1).** This
   is closed as obsolete. ADR-0227 deleted `script/roadmap.sh` and its `finish_gate_exit_cleanup`
   path, and the owning Core ticket was not carried over. Whether `corvint-tasks gate run`/`attempt
   run` keeps gate output after INT/TERM/HUP is NOT_OBSERVED. That question becomes its own Corvint
   ticket.
5. **V1-1315, the `corvint test` CI cutover.** This moves to `post-1-0` together with V1-0812. Its
   bar needs organic failing PR rows (3 of 5 on 2026-10-20) and the owner's AFP-V0-016 consent, and
   no lane can produce either. The CI call site stays shadow-only and experimental for 1.0.
6. **V1-1257, promoting the `corvint test` runner.** The non-latching AFP-V0-030 reading recorded in
   PR #988's evidence is adopted. The runner may be promoted to "implemented" and the CI call site
   stays experimental and shadow-only, so V1-1257 does not wait on V1-1315. "Implemented" is the
   status wording, not "delivered".

## Not decided here

These stay with the owner:
- **Hosted dispatch (V1-0374).** Dispatching Release gates on the frozen candidate. No candidate
  exists yet.
- **Trial run (V1-0835).** Running the blind v7 trial. Case construction, already granted by
  decision 0539, proceeds.
- **Promotion (V1-0591, V1-0595, V1-0596).** Promoting the stable CEM profile. Its prerequisites
  (external conformance CEM-V1-008, and the device tuples) are not met, so promotion now would be
  premature in any case.
- **The release itself (V1-0019/0020/0021).** Tag, publication and stable promotion.

## Rollback

Each item can be reversed by a later decision that supersedes it. Item 5 is reversed by restoring
V1-1315's `v1-0` milestone. Items 3 and 4 are reversed by reopening the tickets.
