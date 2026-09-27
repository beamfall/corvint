## 2026-09-27 CTS-V0-003 V1-0323: shadow import commits in IMPORT_APPLY batches through the full writer

The owner accepted CTS-V0-003 on 2026-09-27 (relayed by the orchestrator session "Work progress
orchestration"): `corvint-tasks import` writes a foreign roadmap export as shadow `IMPORT` records
that read `CUTOVER_MISSING` until a separate cutover. The first consumer is a fixture rehearsal of
the Beamfall roadmap (about 2,894 tickets and 682 dependency edges) into a `ROADMAP`-written queue.

Decisions.

- One new stage operation, `IMPORT_APPLY`, under an `OWNER` or `OPERATOR` binding. Its receipt uses
  the `IMPORT_APPLY` kind already in the closed §3.4 receipt-kind set, with a null `ticketId`. The
  existing `MUTATE` stage carries exactly one ticket envelope, so it could not post a batch; no wire
  code, receipt kind, projection check or genesis byte changes. The transaction
  model re-checks every posted record on its own (`IMPORT` source, shadow overlay, revision chain,
  no native or other-source holder, non-`NATIVE` writer), so a batch built by anything other than
  the importer is still refused.
- The export format (`corvint-tasks-import/0`, JSON Lines) carries the 23 exporter-mapped ticket
  fields and the verbatim block. The importer owns identity, `source`, `shadowOverlay`, the
  revision chain and every timestamp, so a re-export can only change what the foreign side owns.
  `source` is acceptance-relevant, so a changed block always bumps `acceptanceRevision`.
- The whole export is decoded and planned against one audit, under one lock and one authority
  session, before the first write. A refusal therefore writes nothing. The records then commit in
  ascending `ticketId` order in batches packed to the existing stage limits (11 staged artifacts,
  8 inline receipt posts, a 2422-byte descriptor, measured on a synthetic widest descriptor), about
  seven records per batch. Each batch passes the whole §5.2 writer (guards, request-index replay,
  fresh audit, head check, model, branch check, observation binding, apply) and replays by a request
  ID derived from its records. A batch refused after earlier ones committed leaves those committed,
  and rerunning the same export resumes.
- A dependency cycle among imported records is recorded, not refused; reads report it as for any
  store. Missing dependency or supersession targets and gates not declared by policy refuse before
  the first write.
- No import-map writer, cutover, admission, attempt or drain. A non-fixture import writer is
  V1-0398, which V1-0184 depends on.

Cost. Every batch re-audits every ticket file and walks the whole receipt chain, as every §5.2
write already does, so a first import costs about (store size × batch count) and grows
quadratically. Measured on a host at load 80 to 150 on 12 CPUs with 2,894 synthetic items,
imported as six successive 500-item exports (the last 394) into one store: 72 batches per full
chunk, taking 41 s, 2 m 11 s, 4 m 55 s, 6 m 53 s, 7 m 37 s and 8 m 32 s (57 batches), about
30 m 48 s in total and 419 batches. Reusing the audit's inventory for the model removed one of the
two scans per batch. Re-importing the unchanged export is one audit, planned nothing and took 3.2 s.
The cost is accepted for the one-off fixture rehearsal. The chosen follow-up, not yet filed, carries
the audit across batches within one locked import session, re-checking the head and the files each
batch writes; a larger import stage would only shrink the constant.

Real export. The orchestrator session reported a fixture rehearsal of the real Beamfall export
(2,894 items, 14 MB) with a binary built from this branch before its rebase, into a throwaway
`ROADMAP`-written fixture store with 23 command gates: the first import committed in 415 receipts
in about 21 minutes at load 80 to 150, slowing from 3.5 to 1.4 items per second; the receipt audit
read `CONSISTENT` and `AGREES`; every record read `CUTOVER_MISSING`; the unchanged re-import took
3.3 s and wrote nothing. Six real-data issues (a prose identifier, empty titles, directory paths
ending in `/`, free-text repository names, dependencies on alias identifiers with nine unresolved
targets, and holds and completions without source values) were fixed in the Beamfall exporter, not
in the importer. This run is reported, not reproduced here.

Evidence: `TestCTSV0003_*` in `internal/tasks/store`, `internal/tasks/transaction` and
`internal/tasks/cli`, and the existing `IMPORT` source-rule tests
`TestTMV0003_AS02_FieldRelationships` and `TestTMV0004_AS05_EligibilityDerived`.

Rollback: remove the `import` verb, `internal/tasks/importer`, `internal/tasks/store/import.go` and
the `IMPORT_APPLY` stage operation. Records already imported stay valid, never-eligible `IMPORT`
shadow records, and their receipts stay decodable because the receipt kind predates this change.
Only a store left with a pending `IMPORT_APPLY` stage by an interrupted run needs the stage
operation, so finish that run (a rerun redoes it) before rolling back.
