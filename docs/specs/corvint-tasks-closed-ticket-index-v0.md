# Corvint Tasks closed-ticket index V0

Owner: Russell Lewis
Date: 2026-10-31
Intent status: experimental under the owner's standing lane auto-accept (2026-10-11; V1-3459)
Delivery status: experimental
Authoritative inputs: native ticket V1-3459; `TM-V0-008` (snapshot reads) and `TM-V0-005`
(dependency structure) of the Corvint Tasks contract; AGENTS.md invariant 4 (reads do not
mutate); the design record `docs/build-log/2026-10-31-archive-speed-w1.md`.
Native tickets: V1-3459; V1-3473 (fixture mutations) and V1-3474 (index build tool).

## Agent digest
- Claim: OPEN-path reads take closed tickets' digests and summaries from a derived stat-keyed index, so their cost barely grows with the closed set.
- Status: experimental under the owner's standing lane auto-accept (2026-10-11; V1-3459); TCX-V0-980..988 implemented and tested in tree.
- Exists: `internal/tasks/intent/closed_index.go`, `internal/tasks/ticket/lazy.go`, the indexed read path in `internal/tasks/cli`, the audit digest share in `internal/tasks/journal/audit.go`, `fixture.WriteWritableState` and the development tool `internal/tasks/cmd/closed-index`.
- Blocked on: nothing.
- Read next: Requirements; Failure modes; Rollback.

## User and boundary

An agent reading the OPEN backlog of a long-lived store paid to parse every closed ticket too:
on a store of about 3,480 tickets, 2,456 of them closed, `ticket search --status OPEN` took about
3x what the same read took on an open-only store. This contract adds one derived file beside the
state directory and lets the snapshot read take a closed ticket's digest and summary from it.

The index is never authority. Every record, digest and receipt is still the projection's bytes.
The index adds no command, flag, error code or wire field, and every read's output is
byte-identical with and without it.

## Requirements

- `TCX-V0-980`: An inventory may hold a COMPLETED or ARCHIVED ticket as a summary: ticket ID,
  title, status, archivedFrom, priority, order, acceptanceRevision, kind, owner, milestone, labels,
  requiredGates, dependencies, updatedAt and completion time. Structure, cycles, dependency checks and planning
  order are computed from summaries and equal those of the full records. The whole record is
  decoded on its first `Get`, at most once per read.
- `TCX-V0-981`: The index file is `<state dir>.closed-index`, a sibling of the state directory. It
  holds a header (format `corvint-tasks-closed-index/2`, queue ID, policy digest, capture time and
  the body's SHA-256) and one line per indexed ticket file: path, size, modification time, SHA-256
  and the summary. A read trusts an entry only when the file's current size and modification time
  equal the entry's and that time is more than 2 seconds before the capture time. A trusted file is
  not read during the snapshot probe; its digest is the entry's, so the intent tree digest is
  unchanged.
- `TCX-V0-982`: Only `ticket search`, `roadmap`, `queue status`, `ticket show` and
  `ticket blockers` read with the index. Their scans of OPEN and HELD tickets skip closed
  summaries. Any index that is absent, unreadable, over the intent tree size bound, of another
  format, or fails its checksum is ignored and the read is a full scan. A closed record decoded on
  demand is re-read and re-hashed; bytes that differ from the snapshot digest fail the read with
  `SNAPSHOT_MOVED`, and a record whose ID or status differs from its summary fails the same way.
- `TCX-V0-983`: An index whose queue ID or policy digest differs from the snapshot's is not
  trusted for any entry: its files are read and decoded as in a full scan. Reads never write the
  index (invariant 4). After any command that is not a listed pure read (`ATR-V0-212`), the CLI
  rewrites it best effort; a failure leaves the old index, whose stale entries readers reject, or
  none. The CLI never indexes a snapshot bundle or a store without a state directory.
- `TCX-V0-984`: A rewrite reuses the entries whose stat keys still match and reads every other
  closed ticket. Entries whose modification time is within 2 seconds of the capture time are left
  out, so a same-second rewrite is read next time. The file is written to a temporary name and
  renamed.
- `TCX-V0-985`: The journal audit of a reader accepts a probed tree file taken from the index as
  sharing its digest when its recorded size equals the file's current size. A writer's audit
  always reads the bytes.
- `TCX-V0-986`: `roadmap` renders a COMPLETED or ARCHIVED ticket's row from its summary, without
  decoding its record. The row's eligibility is `BLOCKED`, its gate results `NOT_OBSERVED` and its
  next action the one `ticket show` derives from the status alone, so every row is byte-identical
  to the one rendered from the full record.
- `TCX-V0-987`: The test fixture's journaled store `fixture.WriteWritableState` also creates the
  directories `store.Init` creates in a new state directory (`evidence`, `pinned`, `requests`,
  `staging`), so a real CLI mutation commits on it. After `ticket archive` commits there, the CLI
  has rewritten the index; the newly closed ticket joins it on the first rewrite after its file
  leaves the 2-second window (`TCX-V0-984`), and every indexed read equals a full read.
  `fixture.WriteState` alone still has no `staging` directory, so the fixture writer refuses a
  mutation on it with `MALFORMED` ("fixture filesystem identity, role or lifetime refused"); tests
  that need only a read keep using it.
- `TCX-V0-988`: The development tool `go run ./internal/tasks/cmd/closed-index DIR` writes the index of
  the repository at `DIR` whether or not its state directory exists, at the path the CLI would
  use, so benchmarks can index journal-free copies of a store. It is not a shipped command. For
  the same intent tree its entries equal those of the index the CLI writes on the journaled store.

## Non-goals

- No change to the projection, the journal, receipts or any wire output.
- No index for OPEN or HELD tickets, and no network, daemon or database (invariant 7).
- A same-size rewrite that keeps the same modification time more than 2 seconds after the index
  capture is not detected, the limit Git's racy-index check also has. Corvint Tasks writers always
  change the modification time.

## Failure modes

| Situation | Behavior |
| --- | --- |
| Index absent, corrupt, truncated or checksum mismatch | Ignored; full scan (`TCX-V0-982`). |
| Indexed file edited, resized or touched | Entry not trusted; file read and decoded (`TCX-V0-981`). |
| Index of another queue or policy | No entry trusted (`TCX-V0-983`). |
| Indexed file changes between probe and decode | `SNAPSHOT_MOVED` (`TCX-V0-982`). |
| Tool run on a store without intent | Refused with the resolve error; nothing written (`TCX-V0-988`). |
| Index write fails or races a writer | Old or no index; readers stay correct (`TCX-V0-983`, `TCX-V0-984`). |

## Acceptance evidence

Focused `internal/tasks/intent`, `internal/tasks/ticket`, `internal/tasks/journal` and
`internal/tasks/cli` tests, and the measured before/after timings in
`docs/build-log/2026-10-31-archive-speed-w1.md`.

## Rollback

Delete `<git common dir>/taskman.closed-index`; every read falls back to a full scan. To remove
the feature, revert the implementing commit; the index file is then ignored and can be deleted.
`internal/tasks/cmd/closed-index` and `fixture.WriteWritableState` are test and benchmark aids; reverting
their commit removes them with no effect on stores.

## Traceability

| Requirement | Evidence |
| --- | --- |
| `TCX-V0-980` | `TestTCXV0_LazyInventoryMatchesFull`, `TestTCXV0_IndexedReadParity` |
| `TCX-V0-981` | `TestTCXV0_ClosedIndexRoundTrip`, `TestTCXV0_ChangedFileNotTrusted` |
| `TCX-V0-982` | `TestTCXV0_CorruptIndexFullScan`, `TestTCXV0_MovedClosedRecordFails`, `TestTCXV0_IndexedReadParity` |
| `TCX-V0-983` | `TestTCXV0_PolicyMismatchNotTrusted`, `TestTCXV0_IndexedReadParity` |
| `TCX-V0-984` | `TestTCXV0_RacyEntryLeftOut` |
| `TCX-V0-985` | `TestTCXV0_IndexedReadParity` (journaled store, `receipt audit`) |
| `TCX-V0-986` | `TestTCXV0_ClosedRoadmapRowsMatchView`, `TestTCXV0_IndexedReadParity` (`roadmap`) |
| `TCX-V0-987` | `TestTCXV0_RefreshAfterRealMutation`, `TestTCXV0_PlainFixtureStateRefusesWrites` |
| `TCX-V0-988` | `TestTCXV0_ToolIndexMatchesJournaled`; live-store comparison in the build log |
