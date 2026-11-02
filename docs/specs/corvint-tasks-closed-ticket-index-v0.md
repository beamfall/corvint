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
- Status: experimental under the owner's standing lane auto-accept (2026-10-11; V1-3459); implemented and tested in tree.
- Exists: `internal/tasks/intent/closed_index.go`, `internal/tasks/ticket/lazy.go`, the indexed read path in `internal/tasks/cli`, the audit digest share in `internal/tasks/journal/audit.go`, `fixture.WriteWritableState` and `corvint-tasks doctor --rebuild-closed-index` (`TCL-V0-738`).
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
  unchanged. A trusted entry's size still counts toward the intent tree bound, so a capture whose
  read and trusted files together exceed it is refused with `LIMIT_EXCEEDED`.
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
- `TCX-V0-988`: The shipped command `corvint-tasks doctor --rebuild-closed-index` (`TCL-V0-738`)
  writes the index of the repository at the working directory whether or not its state directory
  exists, at the path the CLI would use, so a journal-free copy of a store (a benchmark, a lane
  host snapshot) can be indexed. For the same intent tree its entries equal those of the index the
  CLI writes on the journaled store. It replaces the former development tool
  `internal/tasks/cmd/closed-index`, which V1-3485 retired.

### Write-path summaries (V1-4229, V1-1148)

The owner lifted the V1-0910 cache hold on 2026-11-01 for this scope (recorded in
`docs/build-log/2026-11-02-msc-u34-tasks-write-path.md`). Every write decoded every ticket in
three of its passes: the CLI's pre-mutation load, the writer model's canonical inventory, and the
index rewrite, which read and decoded every open ticket because only closed ones were indexed. At
500 tickets those passes were 54% of a `ticket create`'s CPU. The blocks below let each pass skip
the decode of a ticket whose bytes are exactly those an index entry was admitted for. The writer
journal audit (`TCX-V0-985`) is unchanged and outside these blocks. Status for `TCX-V0-845` to
`TCX-V0-848`: experimental under the standing lane auto-accept; V1-4229, V1-1148.

- `TCX-V0-845`: The index format becomes `corvint-tasks-closed-index/3` and holds an entry for
  every ticket file, open or closed. A rewrite admits a newly read file only after its decoded
  record re-encodes to exactly the file's bytes (`Encode(Decode(b)) == b`), so every entry's
  digest is the SHA-256 of a canonical encoding that the writer's re-encode check accepts. A
  rewrite keeps any entry, open or closed, whose stat key still matches (`TCX-V0-984`) and reads,
  decodes and checks every other ticket file. Reads trust only COMPLETED and ARCHIVED entries,
  so `TCX-V0-981` to `TCX-V0-983` and `TCX-V0-986` are unchanged for them. A `/2` index is not
  trusted and is replaced by the next rewrite.
- `TCX-V0-846`: The native mutation writer gives the model the index's entries when the index
  governs the proof's queue and policy (`TCX-V0-983`). For every canonical ticket other than the
  request's target whose bytes' SHA-256 and length equal an entry's, the model holds the entry's
  summary and decodes the record from those same in-memory canonical bytes only on its first
  `Get`. The model's writer re-encode check is kept: in the base model a decoded unselected
  ticket must have `FileDigest()`, the SHA-256 of its re-encoding, equal to its projection's
  digest. For an indexed ticket, `TCX-V0-845` admission ran `Encode(Decode(b)) == b` on bytes
  byte-identical (by SHA-256 and length) to the canonical bytes, so `FileDigest()` equals that
  digest; the model still requires the projection's digest and length to equal it, and still
  refuses a missing projection, a divergent unselected projection and an incomplete inventory
  exactly as before. The target, every ticket without a matching entry, and every route other
  than the native mutation writer decode as before. A lazy decode that fails is a `MALFORMED`
  refusal of the request, never a panic.
- `TCX-V0-847`: The CLI's pre-mutation load, which feeds only intent checks and warnings, reads
  and digests every intent file as before and holds a ticket as its entry's summary when the
  file's SHA-256 and length equal the entry's; its record is decoded from the bytes already read
  on first `Get`. Every warning and refusal is byte-identical to the full load.
- `TCX-V0-848`: Derived-state argument and measurement. The index is derived, rebuildable state
  in the private state directory beside the journal (invariant 4: only writes rewrite it, reads
  never do). It is not authority: it selects which decode to skip, never which bytes count, and a
  summary is used only for bytes whose digest equals the one it was derived from. Its checksum
  guards against corruption, not against an editor of the state directory, who could equally
  rewrite the journal; summaries therefore carry the same trust as the journal they sit beside.
  The journal and every command output are byte-identical with and without the index. Target, set
  before the build: a `ticket create` at 2,000 tickets costs under 20% more than at 500 tickets
  in these three passes, measured on the same host, interleaved, with load recorded, and
  `tasks-scale gen --tickets 2000` runs to completion and is recorded. The writer journal audit's
  share is recorded separately as owned by its lane. Non-goals: a change to the journal audit,
  to any read's output or to which bytes are authority. Rollback: restore format `/2`, and the
  writer model and the CLI preload decode every ticket again. Acceptance:
  `TestTCXV0845IndexAdmitsOnlyReencodedBytes`, `TestTCXV0845ReadsTrustOnlyClosedEntries`,
  `TestTCXV0846ModelSkipsOnlyByteIdenticalIndexedTickets`, `TestTCXV0846ModelOutputMatchesFullDecode`,
  `TestTCXV0847PreloadMatchesFullLoad`, `TestTCXV0848JournalByteIdenticalWithIndex`.
- `TCX-V0-849`: (experimental under the standing lane auto-accept; V1-4229, round 3) The index
  rewrite is incremental. It keeps, byte for byte, the encoded line of every entry whose stat key
  still matches. It reads, decodes and checks only the other ticket files (file name, queue and
  policy gates, as a full load checks them), without building the store's inventory, and admits
  each one by `TCX-V0-845`. Lines are written in path order, so the file is byte-identical to a full
  re-encode of the same entries. When no entry line changes, the file is left as it is. A process
  parses one index content at most once: a read reads the file and reuses the earlier parse only
  when the bytes' SHA-256 equals the bytes that parse decoded, so no timestamp, however coarse,
  can serve a stale or invalid file. Amended after review (Codex r1 P2): the parse was first
  reused by device, inode, size, modification and change time. The checksum and every
  trust rule of `TCX-V0-981` to `TCX-V0-984` still apply. Target, set before the build: index
  rewrite plus index reads at most 13 ms per create at 2,000 tickets, half of the round-2 26 ms.
  The `TCX-V0-848` target is unchanged. Acceptance: `TestTCXV0849IncrementalRewriteMatchesFullEncode`,
  `TestTCXV0849UnchangedIndexNotRewritten`, `TestTCXV0849ParsedIndexReusedPerVersion`,
  `TestTCXV0849SameStatRewriteReparsed`.
- `TCX-V0-850`: (experimental under the standing lane auto-accept; V1-1148) `tasks-scale gen`
  takes `--pause-after K` and `--max-seconds S`. Either makes a run stop cleanly after a whole
  ticket, recording its place in `.git/scale-progress.json`, outside the store's commit. `gen
  --resume` with the same `--root` and `--tickets` continues from that place through the same
  product writers, so a store too large for one bounded command is built across several. Every
  request ID gen sends is derived from the ticket number, and every mutation carries the one
  `issuedAt` the gen recorded at its start, so a ticket redone after a kill replays byte for byte.
  The first checkpoint is saved before the first mutation (init), and later ones every 50 tickets
  and at each pause. Each save writes a temporary file in the same directory, syncs it and renames
  it over the old one, so an interrupted save leaves the previous checkpoint. A resume whose
  checkpoint still names init skips init when its receipt is present. The manifest's receipt count
  is the journal head's sequence. Amended after review (Codex r4: replay timestamps, atomic save,
  test coverage).
  The manifest records `chunkSeconds`, the wall time of each 500 tickets across segments. A
  finished resumed store has the same tickets, statuses and receipt count as an unpaused one.
  Target, set before the build: ticket creation is roughly linear, the last 500-ticket chunk of a
  2,000-ticket gen taking at most 1.5 times the first. Acceptance:
  `TestTCXV0850PausedGenResumesToTheSameStore`, `TestTCXV0850ResumeAfterInitSkipsInit`; the chunk
  times are in the build log.

### V1-4315 gated batch-close write path

Status: proposed (V1-4315; store-performance lane bcp-w40, 2026-11-02); implemented with focused
tests and a paired measurement on the WSL2 benchmark host, live qualification on the owner's store
NOT_RUN. Human-owned input: native ticket V1-4315. `ticket batch-close` paid one complete
administrative write for every gated step of every row: claim, submit, gate run and complete each
took the complete lease route, which audited the whole journal, built the complete inventory and
decoded every ticket record, so closing 58 tickets on a 4,332-ticket, 19,197-receipt store took
more than 30 minutes (about 6.4 s per receipt). Intent: a gated step costs what the other
writer-route verbs cost, with the same admission decision and the same bytes, and a row's cost
stops growing with the receipt history and with the decode of the whole ticket inventory. These
requirements live here, beside the write-path summaries above, because the agent-leases spec's
Requirements section is at the OCM obligation bound (SRG-V0-740); `CAL-V0-116` is amended by
`TCX-V0-970` as `CAL-V0-205` amended it.

- `TCX-V0-970`: (proposed; V1-4315) `SUBMIT`, `GATE_RUN` and `COMPLETE` MUST be served by the
  `CAL-V0-116` writer route under that route's rules: it observes, models and commits under one
  writer lock, rechecks the head, the intent tree and the primary branch before effects, replays a
  retained request and reports a model result that plans no transaction without a write
  (`CAL-V0-190`). While the store holds a bound writer checkpoint whose tail the route accepts,
  the three verbs MUST NOT run the complete-history audit. Every decline of that route, including
  a model that read an elided path, MUST hand the request to the complete route unchanged, with
  nothing written. A committed write MUST write the same bytes and report as the complete route,
  and a refused one MUST be answered as the complete route answers it. This amends the
  `CAL-V0-116` lease-verb scope to include the three verbs and extends that route's accepted
  guarantee change (a prefix rewrite is not detected by a fast writer) to them.
- `TCX-V0-971`: (proposed; V1-4315) The lease writer route MUST give the model the index's
  entries exactly as `TCX-V0-846` gives them to the native mutation writer: only when the index
  governs the audited queue and policy; a ticket is held as its entry's summary only for canonical
  bytes whose SHA-256 and length equal the entry's; every projection check and refusal of the
  decoding route is kept; a lazy decode that fails refuses `MALFORMED`. A liveness lease, which
  reads no ticket, is given none. The index is never authority: a store without an index, or whose
  index does not govern, decodes every record as before, and a committed lease write writes the
  same bytes and report with and without the index.
- `TCX-V0-972`: (proposed; V1-4315) A gated `ticket batch-close` that will write at least one row
  rewrites the index, as the post-command refresh does (`TCX-V0-984`), before its first row, so
  every row's lease writes hold the unchanged tickets as summaries instead of decoding the whole
  inventory at every step. The rewrite is best effort and the index file is its only write; a
  `--dry-run` writes none (`TCL-V0-821` unchanged), and a manual close writes none before its rows.
- `TCX-V0-973`: (proposed; V1-4315) The gate run's pre-read, from which it decides runnability,
  the candidate, the gate definition and a reusable result before the program starts, MAY resume
  the read-side journal audit from the retained read checkpoint (`CAL-V0-059`), else from the
  bound writer checkpoint (`LGL-V0-807`), bound to the head it observed, with one capture of the
  intent tree handed to the audit as the outer snapshot (`TM-V0-008`). A pre-read that fails, is
  pending, finds staging present or observes a head that moved MUST fall back to the complete read
  audit, so every refusal is the complete audit's. Nothing is decided from it finally: the lease
  write that records the run audits again on its own route and rechecks under the lock.
- `TCX-V0-974`: (proposed; V1-4315) The intent-tree capture (`intent.TreeDigest`, its stat pass
  and the journal audit's directory listing) MAY read, stat and hash entries concurrently on at
  most min(GOMAXPROCS, 8) workers, provided the tree, its total bytes and digest, and every
  refusal, in listing order, are exactly those of the serial walk. The lease writer route MUST
  capture the intent tree itself, under the writer lock, and hand it to its audit as the outer
  snapshot exactly as the mutation writer takes the CLI's preload (`LGL-V0-845`): the audit takes
  a file's bytes from the capture while the file still has that size and lists the rest, and the
  pre-effect recheck validates every current entry again (reading and hashing every file unless
  `TCX-V0-977` admits process-local reuse), so the capture is never the
  evidence that the modeled bytes are current. A liveness write captures nothing (`JSG-V0-001`).
  The route hands a committed claim's capture to the claim's know-how delivery read
  (`TCX-V0-975`), since a claim changes no touch path and no note.
- `TCX-V0-975`: (proposed; V1-4315) The claim's know-how delivery read MUST load the inventory as
  the CLI preload loads it (`TCX-V0-847`: a record the index vouches for by digest and length is
  held undecoded; every other record is decoded and checked as the full load checks it, so an
  unreadable inventory still delivers `UNAVAILABLE` with the load's code, `KHN-V0-014`), over the
  tree the writer route captured for the claim when it served it, and MUST materialize only the
  claimed ticket and the records whose canonical bytes carry the `knowHow` key: a canonical record
  with know-how always carries it, so the selection never drops a note, and a record selected by a
  look-alike byte sequence decodes as before. The notes delivered, their order and their freshness
  resolution are those the selection over every record delivers.
- `TCX-V0-976`: (proposed; V1-4315) The manifest entry cost (`archive.EntryCost`), which depends
  on the entry's path and size alone, MAY be memoized by them in the process; a refusal is never
  memoized and a full memo is dropped whole. The inventory's capacity measure and its refusals are
  unchanged.

- `TCX-V0-977`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4315)
  One sequential gated batch-close command MAY retain one bounded native intent capture in
  memory, shared also with the mutation CLI preload while preserving its validation and warnings.
  Each capture, including the pre-effect binding and the gate pre-read, MUST freshly list
  the closed layout and stat every entry through root-confined, no-follow descriptors, enforcing
  all existing layout, count, type and byte bounds. Reuse of an entry's immutable bytes and
  SHA-256 requires equal device/inode, mode, size, nanosecond mtime AND kernel-controlled ctime;
  the ctime MUST predate the retained capture's START by more than two seconds and have
  nonzero nanoseconds to exclude racy or coarse timestamps. A changed, recent or unverifiable entry MUST be read and content-hashed;
  unsupported platforms MUST read and hash all entries. This is a native Linux filesystem
  optimization, not protection against privileged kernel/raw-device timestamp forgery. Restoring
  mtime alone, replacing an inode, adding, removing or renaming a file MUST NOT hide a change.
  Failed captures MUST discard retained entries. Only a capture made under the continuously held
  writer lock may also supply that audit's intent listing and inventory metadata; external
  preloads and unlocked read audits MUST retain independent listings. A batch gate pre-read MAY
  retain that native capture's raw projection bytes only if, after its audit and before any gate
  decision/program uses them, it takes another validated capture and re-reads the current head,
  requiring both digests to equal the audit. Any mismatch/read failure MUST fall back to the
  complete pre-read; an external preload's claimed digest MUST NOT qualify. This preserves
  TCX-V0-973 while removing its redundant projection opens/hashes. Immediately before effects
  the writer MUST rebind the current head and tree with a fresh validated capture and compare
  their digests to the audit, without releasing the lock. Thus CAL-V0-116, LGL-V0-845 and
  CAL-V0-070 retain content validation and confinement; the persisted read index's mtime key
  (`TCX-V0-981`) MUST NOT serve writes. This amends TCX-V0-974's unconditional second content
  read while retaining its independent pre-effect current-content check.
- `TCX-V0-978`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4315)
  The native mutation and lease writers MAY carry the SHA-256 already computed by their
  audit alongside the exact immutable canonical ticket bytes into the pure model, in matching
  order. The model MAY use
  it in place of hashing those same bytes again for index lookup and projection binding. An external
  preload's claimed digest MUST NOT qualify: mutation borrowing requires the route's own native
  capture under its held lock; ordinary external preloads MUST still hash their raw bytes. All
  length, target exclusion, canonical-set and physical projection checks MUST remain; an
  absent or incomplete digest vector MUST use content hashing. Digests are caller-supplied
  observations, never taken from the index as authority over canonical bytes. A command MAY retain
  an admitted immutable index and its queue/policy-scoped digest map across writes: only exact
  audited digest/length matches remain eligible, changed tickets decode normally, and a different
  store or governing queue/policy MUST invalidate or rebuild the scoped map. Indexed and
  decoding routes MUST retain the same plans, capacity checks and refusals.
- `TCX-V0-979`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4315)
  Within that serialized command the writer MAY update an owned summarized inventory by
  comparing every freshly audited entry and validating and charging only changed entries,
  undoing updates on failure. Previous model/plan users MUST be finished before reuse. Reuse
  requires the same store, immutable audited checkpoint prefix, elided counts/costs and state
  directory set; otherwise it MUST rebuild with NewSummarizedInventory. Duplicate, unknown,
  malformed, over-bound, missing-parent and overlapping elided entries MUST refuse as a rebuild
  does. Every capacity prefix and receipt/plan MUST equal the fresh-inventory result. The cache
  MUST be process-local, bounded by the inventory limits, discarded on failure, and excluded
  from journal authority and persistent formats.

Non-goals of this slice: the other lease verbs keep their routes; no change to the three verbs'
models, refusals or wire results; no collapse of a row's steps into fewer receipts (receipts and
journal semantics stay byte-compatible); no faster complete route when no bound checkpoint
exists; no use of the persisted read index's stat key on the write path (`TCX-V0-981` stays a read-path rule).

Failure modes of this slice: without a bound writer checkpoint (a new store, a head below the
checkpoint minimum, a tail beyond the route's bound, an unsettled journal) the three verbs run
the complete audit as every lease verb does, outside the preparation gate (`CAL-V0-263`). A
ticket projection rewritten outside the journal (stale or tampered) is refused by the writer
audit's projection comparison or by the model's projection check, with or without an index, and
the complete route refuses it alike; nothing is written. A store without an index decodes every
ticket per step until `TCX-V0-972` refreshes it. Every write still validates the current layout and file identities before effects; unchanged
old entries may retain their content capture only under TCX-V0-977. The measured growth from
4,000 to 8,000 tickets is recorded in
`docs/build-log/2026-11-02-bcp-w40.md`, not hidden.

Rollback of this slice: remove the three verbs from `writerLeaseVerb`, the index and the preload
from `leaseWriter`, the checkpointed pre-read, the batch-close refresh, the selective know-how
load, the entry-cost memo and the command-scoped capture/inventory reuse, and revert the tests
and this section. Removing WithBatchWrites restores full content reads and inventory rebuilds. Stores are unchanged: both
routes write the same bytes, and the index is derived state.

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
| Rebuild run on a store without intent, or whose tree does not load | Refused with the read's code; nothing written (`TCX-V0-988`). |
| Index write fails or races a writer | Old or no index; readers stay correct (`TCX-V0-983`, `TCX-V0-984`). |

## Acceptance evidence

Focused `internal/tasks/intent`, `internal/tasks/ticket`, `internal/tasks/journal` and
`internal/tasks/cli` tests, and the measured before/after timings in
`docs/build-log/2026-10-31-archive-speed-w1.md`.

## Rollback

Delete `<git common dir>/taskman.closed-index`; every read falls back to a full scan. To remove
the feature, revert the implementing commit; the index file is then ignored and can be deleted.
`doctor --rebuild-closed-index` writes only the index file; reverting it leaves any written index,
which readers validate. `fixture.WriteWritableState` is a test aid; reverting its commit removes it
with no effect on stores.

## Traceability

| Requirement | Evidence |
| --- | --- |
| `TCX-V0-980` | `TestTCXV0_LazyInventoryMatchesFull`, `TestTCXV0_IndexedReadParity` |
| `TCX-V0-981` | `TestTCXV0_ClosedIndexRoundTrip`, `TestTCXV0_ChangedFileNotTrusted`, `TestTCXV0_CachedEntriesCountTowardTreeBound` |
| `TCX-V0-982` | `TestTCXV0_CorruptIndexFullScan`, `TestTCXV0_MovedClosedRecordFails`, `TestTCXV0_IndexedReadParity` |
| `TCX-V0-983` | `TestTCXV0_PolicyMismatchNotTrusted`, `TestTCXV0_IndexedReadParity` |
| `TCX-V0-984` | `TestTCXV0_RacyEntryLeftOut` |
| `TCX-V0-985` | `TestTCXV0_IndexedReadParity` (journaled store, `receipt audit`) |
| `TCX-V0-986` | `TestTCXV0_ClosedRoadmapRowsMatchView`, `TestTCXV0_IndexedReadParity` (`roadmap`) |
| `TCX-V0-987` | `TestTCXV0_RefreshAfterRealMutation`, `TestTCXV0_PlainFixtureStateRefusesWrites` |
| `TCX-V0-988` | `TestTCLV0738_RebuildClosedIndexMatchesJournaled`; live-store comparisons in `docs/build-log/2026-10-31-archive-speed-w1.md` and `docs/build-log/2026-11-01-tcl-w31.md` |
| `TCX-V0-845` | `TestTCXV0845IndexAdmitsOnlyReencodedBytes`, `TestTCXV0845ReadsTrustOnlyClosedEntries` |
| `TCX-V0-846` | `TestTCXV0846ModelSkipsOnlyByteIdenticalIndexedTickets`, `TestTCXV0846ModelOutputMatchesFullDecode` |
| `TCX-V0-847` | `TestTCXV0847PreloadMatchesFullLoad` |
| `TCX-V0-848` | `TestTCXV0848JournalByteIdenticalWithIndex`; before and after numbers in `docs/build-log/2026-11-02-msc-u34-tasks-write-path.md` |
| `TCX-V0-849` | `TestTCXV0849IncrementalRewriteMatchesFullEncode`, `TestTCXV0849UnchangedIndexNotRewritten`, `TestTCXV0849ParsedIndexReusedPerVersion`, `TestTCXV0849SameStatRewriteReparsed` |
| `TCX-V0-850` | `TestTCXV0850PausedGenResumesToTheSameStore`, `TestTCXV0850ResumeAfterInitSkipsInit`; chunk times in the build log |
| `TCX-V0-970` | `TestTCXV0970_GateVerbsTakeWriterRoute`, `TestTCXV0970_TamperedProjectionRefusesGateWrite` (`internal/tasks/store`); `TestCALV0070_MutateRefusesMappedWriteAfterMergedAudit`, the batch-close tests (`internal/tasks/cli`) |
| `TCX-V0-971` | `TestTCXV0971_LeaseWriterIndexParity` (`internal/tasks/store`); `TestTCXV0846ModelSkipsOnlyByteIdenticalIndexedTickets` |
| `TCX-V0-972` | `TestTCXV0972_BatchCloseRefreshesIndexBeforeFirstRow` (`internal/tasks/cli`) |
| `TCX-V0-973` | `TestTCXV0973_CheckpointedPreReadMatchesCompleteAudit` (`internal/tasks/store`) |
| `TCX-V0-974` | `TestTCXV0974_ParallelCaptureMatchesSerial` (`internal/tasks/intent`); `TestTCXV0970_GateVerbsTakeWriterRoute`, `TestCALV0116_WriterRouteParity` (`internal/tasks/store`); the `intent` and `journal` packages whole |
| `TCX-V0-975` | `TestTCXV0975_KnowHowSelectionAmongCandidatesMatchesFull` (`internal/tasks/store`); `TestTCXV0975_LoadSummarizedTreeMatchesLoadSummarized` (`internal/tasks/intent`); `TestKHNV0014_UnreadableInventoryDeliversUnavailable` and the know-how claim tests (`internal/tasks/cli`, `internal/tasks/store`) |
| `TCX-V0-976` | `TestTCXV0976_EntryCostMemoMatchesUncached` (`internal/tasks/archive`) |
| V1-4315 measurement | `TestV14315_BatchCloseProfile` (opt-in, `internal/tasks/store`); paired numbers in `docs/build-log/2026-11-02-bcp-w40.md` |
| `TCX-V0-977` | `TestTCXV0977_ConcurrentTamperBetweenWritesRefused`, `TestTCXV0977_PreEffectTamperRefused`, `TestTCXV0977_CaptureLayoutChanges`, `TestTCXV0977_CaptureReusesOnlyStableContent`, `TestTCXV0977_InventoryFingerprintEncodingParity`, `TestTCXV0977_GatePreReadRebindsCapturedBytes` (`internal/tasks/store`) |
| `TCX-V0-978` | `TestTCXV0978_AuditedDigestAndBatchInventoryParity`, `TestTCXV0978_AuditDigestsPreservePlansAndRefusals`, `TestTCXV0978_ExternalPreloadCannotForgeAuditedDigest`, `TestTCXV0978_RetainedIndexScope`, existing TCX-V0-971 indexed/decoding parity (`internal/tasks/store`) |
| `TCX-V0-979` | `TestTCXV0979_IncrementalInventoryMatchesRebuild` (`internal/tasks/store`); paired 4k/8k `TestV14315_BatchCloseProfile` evidence in the lane build log |
