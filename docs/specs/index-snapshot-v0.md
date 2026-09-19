# Index Snapshot V0

Owner: Russell Lewis
Date: 2026-09-02
Requirement prefix: `IDX-SNAP-V0`
Intent status: accepted (decision 0049, 2026-09-04)
Delivery status: experimental
Authoritative inputs: `docs/decisions/0032-index-snapshot-2026-09-02.md` (the owner's instruction and
the measured basis), `docs/specs/revision-cache-dirty-worktree-v0.md` (the cache key and the dirty
view, `DIRTY-CACHE-001` to `DIRTY-CACHE-004` and `DIRTY-CACHE-007`),
`docs/specs/deployment-neutral-index-platform-v0.md` (the immutable index direction),
`docs/specs/task-context-packet-v0.md` (the consumer), `AGENTS.md` invariants 1, 4, and 7.
Admission amendments: `docs/decisions/0065-documentation-is-searchable-evidence-with-its-own-placement-2026-09-05.md`, `docs/decisions/0095-index-path-screen-is-the-go-set-2026-09-12.md` (`IDX-SNAP-V0-018`).

## Agent digest
- Claim: `corvint index` writes the committed tree's index once; the packet and query verbs read it instead of rebuilding, unchanged, and never write it.
- Status: accepted (decision 0049, 2026-09-04)/experimental
- Exists: `internal/contextindex/snapshot.go` (`WriteSnapshot`, `LoadSnapshot`), `cmd/corvint/index_snapshot.go` (`snapshotIndex`), help topic `index`, the `context`, `query` and `user-prompt` read paths; the snapshot carries the term table (`termtable.go`, decision 0033).
- Blocked on: the invariant 7 benchmark/format gate (cold and warm p95, memory, size, cross-format identity) before the encoding is promoted; automatic-refresh-based packet-5 qualification remains unresolved by explicit warmup.
- Read next: Requirements; Failure modes.

Wave 1: `IDX-SNAP-V0-016` blob shards failed their first latency/parity evaluation and remain proposed/off (decision 0073); `IDX-SNAP-V0-017` analyzer identity is accepted only inside the experimental pack profile (decision 0074). Decision 0079 repairs cold/cached context agreement; corrected parity evidence is separate from the failed first experiment.

## Intent and scope

A packet call rebuilt the whole index from Git on every invocation: on a 3,233-file repository
about 770 ms warm, of which the build is about 450 ms, against a term listing's 55 to 120 ms
(`docs/BUILD-LOG.md`, 2026-09-02). Every table of the index is a function of the committed tree
alone (`DIRTY-CACHE-001`: sources come only from `HEAD^{tree}`); the two fields that depend on the
worktree, `DirtyPaths` and `StatusSHA256`, are cheap to recompute. So the compiled index can be
written once per tree and read back, and the read costs the two Git observations a build already
makes plus a decode. This slice ships that file and the one verb that writes it. It does not change
what any packet says.

## Requirements

- `IDX-SNAP-V0-001`: `corvint [--root PATH] index` builds the index of the committed tree exactly as
  `context` would and writes it under `.corvint/index/` as
  `<object-format>-<tree-oid>-<engine>.gob`, where `engine` is the first sixteen hex digits of the
  SHA-256 of the running executable. The file holds a header (format `corvint-index-snapshot/1`,
  object format, tree OID, engine) and the index with `Root`, `DirtyPaths`, and `StatusSHA256`
  cleared. The write is atomic (temporary file then rename). The receipt is one canonical JSON
  line with `mutates:true`, `path`, `bytes`, `tree`, `commit`, `engine`, `sources`, `symbols`,
  `evicted`. The searchable-text admission allowlist includes `.rst`, `.mdx`, and `.txt`; these
  suffixes enter source and term tables without a symbol extractor. Clarifying amendment
  (2026-09-13, bug hunt): a failure closing the temporary file (a delayed write-back error
  surfacing only at `Close`) MUST be treated the same as an encode failure — the temporary is
  removed and the write refused — rather than renamed into place as if the buffered content had
  reached disk. Clarifying amendment (2026-09-13, decision 0210): the temporary file is synced to
  stable storage before it is closed and renamed, and a sync failure refuses the write in the same
  way. The gob decoder cannot detect a zeroed range inside a source body. Without the sync, a crash
  after the rename could publish such a range, and it would load as a wrong index. The sectioned
  and pack writers sync the same way.
- `IDX-SNAP-V0-002`: `context` reads the repository's identity and status as a build's opening
  observation does, and when a file named by the current object format, tree OID, and engine
  exists with a matching header, uses it with `Root` set, `DirtyPaths` set to the status
  snapshot's sorted paths, and `StatusSHA256` set to its digest. After the status read completes,
  the loader reads identity once more and treats any identity change as a miss. Clarifying
  amendment (2026-09-12, Invariant 2 audit): the file is keyed by tree, so a later commit with the
  same tree hits it; a hit sets `CommitRevision` to the identity's HEAD commit, never the commit
  the snapshot was written at, as a miss's build would (`IDX-SNAP-V0-006`).
- `IDX-SNAP-V0-003`: any mismatch (no file, a different tree, a different engine, a different
  format, an undecodable file) is a miss: `context` builds the index as before and reports nothing
  about the miss. A miss is never an error.
  Clarifying amendment (2026-09-12, corrupt derived snapshots): a file is undecodable when its
  term table decodes but any key offset, posting offset or source id lies outside the slice it
  indexes, or the counted `Terms` table lacks one count per source. The decoder checks this once at
  open, never per lookup, and refuses the file as this miss; a corrupt snapshot never panics.
- `IDX-SNAP-V0-004`: a dirty worktree reads the same snapshot; only `DirtyPaths` and
  `StatusSHA256` differ from the clean read (`DIRTY-CACHE-003`). The snapshot never holds worktree
  bytes. On both a hit and a miss, `DirtyPaths` is exactly the status snapshot's sorted path set.
  A status-clean path whose raw worktree bytes differ from its blob because of `ident`, `eol`, LFS,
  or another clean filter is read from the blob but is not added to `DirtyPaths`; the status
  snapshot is the dirty authority.
- `IDX-SNAP-V0-005`: `index` is the only verb that writes under `.corvint/index/`; `context` and every
  other read verb never create, touch, or delete a file there (invariant 4). The directory ignores
  itself through a `.gitignore` it contains, so a repository with no rule for it stays clean.
  `index` writes that file only when it is absent or different, using temporary-file publication
  by rename rather than exposing a truncate-then-write interval to concurrent status readers. On its
  first write, `index` also creates `.corvint/.gitignore` only when that file is absent. The narrow
  file ignores itself, `index/`, `self-observations.jsonl`, and observation-writer temporaries; it
  MUST NOT hide CEM, work-queue, or other `.corvint/` paths and MUST NOT overwrite a repository-owned
  parent ignore file. This mutating bootstrap lets SOL-V0 attempt its ledger append without making a
  fresh repository dirty; no read verb may create the rule. Repository content is untrusted, so
  before and after creating the directory `index` refuses, with no write or eviction, when `.corvint`
  or `.corvint/index` exists as a symlink or any other non-directory (security amendment 2026-09-13).
  Every snapshot reader (`LoadSnapshot`, the event and context loaders, `LoadEventSnapshotObserved`,
  and the `IDX-SNAP-V0-011` probe) refuses the same shapes: a `.corvint` or `.corvint/index` that is a
  symlink or non-directory is the `IDX-SNAP-V0-003` miss, never a read of the bytes it points at
  (security amendment 2026-09-13, gosec G304 follow-up).
- `IDX-SNAP-V0-006`: a hit and a miss produce byte-identical `context` output for the same
  repository state and arguments.
- `IDX-SNAP-V0-007`: `index` keeps the newest eight snapshot files in the directory and removes
  the rest (`DIRTY-CACHE-007`'s entry bound). After a successful publish it also removes regular
  `snapshot-*.tmp` files at least one hour old, without counting them in the receipt's published
  snapshot `evicted` total; younger temporaries may belong to a concurrent writer and remain.
- `IDX-SNAP-V0-008`: the two per-prompt query verbs read the snapshot on the same terms as
  `context`: `corvint query` in place of its authority-only query build, and the harness
  `user-prompt` event (with the standalone repository and agent-tooling query intents that share
  its path) in place of its eval build. Each build compiles a strict subset of the snapshot's
  tables for its own ranking, so a hit ranks out of a wider index and must still emit byte-identical
  output. A read that cannot observe the repository is a miss, not an error: the verb falls back to
  its own build, which reports the failure in its own vocabulary, so a repository Corvint cannot read
  says exactly what it said before the snapshot existed. Neither verb writes under `.corvint/index/`
  (`IDX-SNAP-V0-005`). Clarifying amendment (2026-09-12, resolving the open question recorded in
  `docs/agent-memory/fixes.md`): the grant follows the shared build functions, not the invoking
  verb's name. `corvint prove --task`'s project-operations query mode compiles its packet through
  the identical `standaloneQueryContext` build (`cmd/corvint/prove.go:1059-1061@d473eb95`, reaching
  `deferredSnapshotIndex` and `snapshotIndex` at `cmd/corvint/main.go:1217-1218@c39315fe` and `cmd/corvint/harness_context.go:69-70@70282d2c`) before
  `compileProof` applies its own independent revision and dirty-set re-verification
  (`cmd/corvint/prove.go:521@7ad2b2af`, `cmd/corvint/prove.go:541-542@38c3dce4`, `cmd/corvint/prove.go:563-564@61b1bb0c`) and reads every cited blob at the
  re-confirmed revision; a snapshot hit there therefore carries no staleness risk beyond what
  `IDX-SNAP-V0-002` and `IDX-SNAP-V0-006` already bound, and is covered by this clause on the same
  terms as `corvint query`. `corvint prove --checkpoint` is not covered: it never reaches this
  build, and `FPK-V0-024` (`docs/specs/falsifiable-packet-v0.md:635`) separately requires it to
  make zero snapshot-loader calls because it judges against its own `base_commit`/`base_tree`
  rather than the current tree the snapshot always describes.
- `IDX-SNAP-V0-009`: a repository with no `.corvint/index/` directory is a miss without a Git
  observation. The file's name carries the tree OID, so asking Git for it would cost a repository
  that has never run `index` two process spawns per read to learn nothing.
- `IDX-SNAP-V0-010`: the two index-building harness events read the snapshot on the same terms as IDX-SNAP-V0-008: `harness event --event file-change` in place of its full build, and the compact `session-start` event in place of its full build. A hit MUST emit output byte-identical to the build path for the same tree and the same dirty set; the dirty set comes from the same status observation the loader already makes, never from the snapshot. A miss falls back to the full build and reports nothing new. This clause changes no output; it removes work.
- `IDX-SNAP-V0-011`: `corvint [--root PATH] index --if-stale` reads the header of the file
  named by the current object format, tree OID, and engine (the `IDX-SNAP-V0-002` probe) and walks
  the index message behind it without materializing the index (amended by decision 0193), so a
  truncated or torn body is a miss under `IDX-SNAP-V0-003`, never fresh. When the header matches
  and the message is complete it writes nothing and emits one canonical JSON
  receipt line with `mutates:false`, `state:"fresh"`, `path`, `tree`, `commit`, `engine`. Otherwise
  it builds and writes exactly as `index` does and emits the `IDX-SNAP-V0-001` receipt. Without the
  flag `index` is unchanged. A repository the verb cannot observe is the same error `index` reports
  today.
- `IDX-SNAP-V0-012`: the Claude Code plugin's lifecycle hooks do not automatically invoke a
  persistent index refresh and MUST NOT leave a detached index child after a hook returns.
  The hook's native read may use an existing snapshot or its existing synchronous in-memory
  build on a miss, within the read's deadline, output, identity and cleanup bounds; a miss
  alone is not a new refusal and does not write `.corvint/index/` (`IDX-SNAP-V0-003`,
  `IDX-SNAP-V0-005`, `IDX-SNAP-V0-008`, `IDX-SNAP-V0-010`). The operator may explicitly run
  `corvint --root ROOT index --if-stale` outside the hook under an owned foreground/supervised
  process lifetime with deadline and interruption cleanup. That writer retains
  `IDX-SNAP-V0-011`; no hook outcome asserts that a persistent refresh completed.
  Amendment authority: owner's 2026-09-08 cross-host lifecycle repair and every-child cleanup
  instruction; independent Gate A; partial supersession of decision 0049 item 3 only.
- `IDX-SNAP-V0-013`: During committed-source pinning, a blob whose bytes begin with
  `version https://git-lfs.github.com/spec/v1` is excluded from `Sources` as
  `git-lfs pointer, content not in the tree`. The exclusion is carried in the snapshot and emitted
  through the receipt's existing bounded exclusion sample. Git status remains the sole dirty
  authority: the exclusion does not make the path dirty and does not add a snapshot field.
- `IDX-SNAP-V0-018`: (accepted 2026-09-12 by decision 0095) Before suffix and size admission,
  every tracked path passes one closed path screen (`forbiddenPath`), checked in this order with
  the first match winning; a matched path is kept out of `Sources` and recorded in `Exclusions`
  with the named reason. (a) Any `/`-separated component byte-equal to one of `.git`, `vendor`,
  `node_modules`, `app-dist`, `dist`, `build`, `coverage`, `.next`, `.cache`, `target`, `.claude`:
  `vendor/build excluded`. Matching is whole-component and case-sensitive, so `internal/claude/`,
  `builds/`, and `Vendor/` are not excluded. (b) A path starting `internal/store/migrate/` or
  `internal/conformance/testdata/`: `protected path`. (c) A path matching
  `(?i)(?:^|/)(?:generated|gen)(?:/|$)|(?:\.gen\.|_generated\.)|(?:^|/)docs/api/(?:openapi\.json|reference\.md|llms(?:-full)?\.txt)$`:
  `generated path`. `eval` learned candidates pass the same screen. The set has no per-repository
  override; adding, removing, or respelling a member, prefix, pattern, or reason amends this clause
  and is an `internal/contextindex` production change under `IDX-SNAP-V0-017`'s schema rule. The
  learned-trace path screen (`internal/trace` record and stored-row admission, and the dashboard
  trace adapter) is this same screen, consumed through `contextindex.ForbiddenPathReason` with no
  copy (decision 0102; `LTPM-V0-011`), so an amendment here changes learned-trace admission too.

### Proposed (2026-09-05, not accepted): sectioned encoding

The prototype and its measurement are in `docs/plans/sectioned-snapshot-prototype-2026-09-05.md`.
Nothing below is accepted; the gob file stays the only accepted snapshot encoding, and the
sectioned file exists only behind the explicit opt-in `CORVINT_SNAPSHOT_FORMAT=sectioned`.

- `IDX-SNAP-V0-014`: under `CORVINT_SNAPSHOT_FORMAT=sectioned`, `index` writes beside the gob
  snapshot a second file (`.sect`, same name stem, same eviction) laid out as a fixed header slot
  carrying a section table (name, offset, length, sha256) followed by independently decodable
  sections, so that compact `session-start` reads only the identity section, `file-change` reads
  the tables `Impact` and its receipt consume and never `Tracked`, `Skipped` or `Vocabulary`, and
  `query`/`context` read every section. A read verifies the sha256 of every section it decodes and
  refuses the whole file on any mismatch as the `IDX-SNAP-V0-003` miss, never a partial index; a
  refused or absent sectioned file falls back to the gob snapshot. Identity, freshness, dirty-path
  and eviction rules are unchanged (`IDX-SNAP-V0-002`..`IDX-SNAP-V0-007`), and every decoded value
  MUST equal the gob decode of the same build. Acceptance is measured, not asserted:
  `harness event --event file-change` and compact `session-start` p95 at or below 100 ms on the
  pinned Beamfall corpus with the sectioned snapshot present, and byte-identical receipts to the
  gob path for every cli-parity harness case. The 2026-09-05 measurement did NOT meet the timing
  gate: the decode leaves the critical path but the kernel's `GPK-V0-007` observation bracket
  (three sequential Git stages per event) is the floor. Without that gate this clause cannot be
  promoted and the default product path does not change.
- Amended 2026-09-12, `IDX-SNAP-V0-014` unchanged in scope: because a successful read's Sources
  alias the mapping, the reader retains it under the `IDX-SNAP-V0-015` pack key (path, length,
  modification time and the digest the file stores over its section table), and a read of a file
  whose mapping is already retained decodes and re-verifies from that mapping, so repeated reads of
  one file retain one mapping. Retention is bounded at four mappings, evicting the oldest first;
  eviction drops the reference without unmapping. The file bytes, the refusal rules and the gob
  fallback are unchanged. Amended 2026-09-13 (decision 0210): when two first reads of one file race,
  the one that loses the retention releases its own mapping and decodes from the retained one. One
  file therefore keeps one mapping under concurrency too.
- Amended 2026-09-12 (second), `IDX-SNAP-V0-014` unchanged in scope: the section table and the
  source table are self-attested, so a matching digest does not bound their offsets. The read
  refuses the whole file as the `IDX-SNAP-V0-003` miss when a section's offset and length, or a
  source body's offset and length inside the bodies section, lie outside the bytes that hold them
  (checked without computing a wrapping `offset+length`), and when the `Vocabulary` section fails
  the `IDX-SNAP-V0-003` term-table check. Amended 2026-09-13 (bug hunt): the header parse also
  refuses a table that names a section twice, or whose sections, digest tables and header slot
  share any byte or run past the file, because a forged table could otherwise serve one section's
  verified bytes under another name with every digest in agreement.

### Proposed (2026-09-05, not accepted): zero-copy tree pack

The prototype and its measurement are in `docs/plans/corvint-pack-prototype-2026-09-05.md`. Nothing
below is accepted; the gob file stays the only accepted snapshot encoding, and the pack exists only
behind the explicit opt-in `CORVINT_SNAPSHOT_FORMAT=pack`.

- `IDX-SNAP-V0-015`: under `CORVINT_SNAPSHOT_FORMAT=pack`, `index` writes beside the gob snapshot a
  second file (`.aip`, same name stem, same engine id, same eviction) laid out as a fixed header
  slot carrying a section table (name, offset, length, the offset and sha256 of the section's
  64 KiB block digest table), then 64-byte-aligned sections, then the digest tables. The term
  tables (`Terms`, `Words`, `PathTerms`), the source table, the symbol table, `Tracked`, `Skipped`
  and the co-change history are fixed-width little-endian arrays over one string table, viewed in
  place from a read-only private mapping (`ReadAt` where the platform has none); bodies are the raw
  source bytes aliased from the mapping; the small tables, and the symbol-window postings
  `SymbolWindows` (`vocab.symbolwindows`, checked against the symbol count on read), stay gob
  values inside verified sections.
  A read verifies the block digest table of every section it touches and every block before any
  byte of it is decoded, and refuses the whole file on any digest mismatch, truncation, or
  out-of-range offset as the `IDX-SNAP-V0-003` miss, never a partial index; a refused or absent
  pack falls back to the gob snapshot, then to the build. The co-change section is keyed by the
  HEAD commit named in the header: a reader whose commit matches serves the cochange slot from it
  and spawns no `git log`; any other commit ignores that section only. A writer in a shallow
  repository writes no co-change section, because deepening the clone keeps HEAD but changes the
  history `git log` returns (amended 2026-09-13). For the same reason a writer whose repository
  carries a grafts file writes none, and a reader whose repository is shallow or carries a grafts
  file ignores the section even when the commit matches, so a complete clone later cut by a
  shallow fetch or a grafts file at the same HEAD spawns `git log` as before. The repository
  identity read reports both in its one `rev-parse` (`--is-shallow-repository`, and
  `--git-path info/grafts` checked by a stat); any stat error other than absence counts as a cut
  (amended 2026-09-13, bug hunt). Identity, freshness,
  dirty-path and eviction rules are unchanged (`IDX-SNAP-V0-002`..`IDX-SNAP-V0-007`), the engine id
  stays the executable digest, the receipt gains `pack_bytes`, and every decoded value MUST equal
  the gob decode of the same build. The default path runs no new code. Acceptance is measured,
  not asserted, against the falsifier rows in the plan: in-process open+rank+emit at or below
  10 ms p95 and 8 MB heap on a hit without subject, a hit with subject at or below 50 ms wall,
  at most 4 KB per file, byte-identical `context` output to the gob path with and without a
  subject on the twenty dogfood tasks, and refusal of every corrupted block with zero panics under
  the race detector.
- Amended 2026-09-11, `IDX-SNAP-V0-015` unchanged in scope: because a successful read's Index
  aliases the mapping, the reader retains it, keyed by pack path, length, modification time and the
  digest the file stores over its header table. A read of a pack whose mapping is already retained
  decodes from that mapping and registers the co-change entries it already decoded, so repeated
  reads of one pack retain one mapping and one history. Retention is bounded: at most four pack
  mappings and eight co-change registrations, each evicting the oldest first. Eviction drops the
  reference without unmapping, because an Index handed out earlier still aliases those bytes, and
  an evicted registration spawns `git log` exactly as an unregistered index does. Ranking inputs,
  the pack bytes and the refusal rules above are unchanged. Amended 2026-09-13 (decision 0210):
  when two first reads of one pack race, the one that loses the retention drops its history
  registration, releases its own mapping, and decodes from the retained one. Amended 2026-09-13
  (bug hunt): dropping the registration also clears its ring slot, so the index the loser never
  handed out is not kept reachable until a later registration reuses that slot.
- Amended 2026-09-12, `IDX-SNAP-V0-015` narrowed for one verb: the task-packet `context` verb's
  read (`LoadContextSnapshotDeferred`) verifies every section except `bodies` before decoding, as
  above, and verifies a body's blocks when the packet first reads that body, before any byte of it
  is returned; an unread body is never verified or returned. A source whose body is unverified has
  no `Data`, so a reader that bypasses `Text` sees no bytes. A body that fails verification reads
  as unreadable and the packet is withheld with `ErrSnapshotRefused`; the verb then rereads through
  the whole-file loader, which refuses the pack as the `IDX-SNAP-V0-003` miss and falls back to the
  gob snapshot, then to the build, so the output equals the build's. Every other load (query,
  event, compact, sectioned and gob) is unchanged and still verifies every block before decoding.
- Amended 2026-09-12 (second), `IDX-SNAP-V0-015` narrowed for the query and event loads: the
  body deferral above also governs the snapshot reads of `query`, `user-prompt` and `calibrate`
  (`LoadSnapshotDeferred`) and of the `file-change` and compact `session-start` harness events
  (`LoadEventSnapshotDeferred`). The header, the section table and every section other than
  `bodies` are still verified before any byte is decoded; a body's blocks are verified when the
  verb first reads that body, and a body never read is never verified. Each of these verbs computes
  over the deferred index and, when a body read failed verification (`SnapshotRefusal`), discards
  what it computed and computes again over the whole-file loader's read, which refuses the pack as
  the `IDX-SNAP-V0-003` miss and falls back to the gob snapshot, then to the build; a deferred miss
  goes straight to the build. The output equals the build's in every case. `prove --task`'s query
  mode shares this read through `standaloneQueryContext`. The batch and necessity reads and
  the sectioned and gob formats are unchanged; standalone path `impact` and `feature` share this
  read under `IDX-SNAP-V0-019`. Amended 2026-09-13 (bug hunt): a block or digest-table mismatch
  found in a retained mapping, including a deferred body's, marks that mapping refused, and every
  later read of the same pack bytes in the process refuses it as the `IDX-SNAP-V0-003` miss before
  decoding. A later deferred request therefore computes once over the gob fallback instead of
  computing over the pack, discarding the result and rereading.

### Proposed (2026-09-05, not accepted): blob facts shards

- `IDX-SNAP-V0-016`: under `CORVINT_INDEX_SHARDS=1`, `index` additionally publishes immutable,
  digest-verified blob facts under `.corvint/index/blobs/<engine>/`. A record key contains the Git
  object format, blob OID and full-path SHA-256; the header pins `corvint-blob-facts/2` and the
  reviewed analyzer engine identity from IDX-SNAP-V0-017. Full path is a conservative analyzer context: this
  prototype does not deduplicate one blob across filenames or reuse a renamed source's facts.
  `context` on a snapshot miss may read these records without writing any file. The current Git
  tree is the manifest: only admitted current entries are reused; only absent records fetch Git
  blobs. Bodies must match their Git OID and tree length; records exceeding 32 MB, symlink paths,
  corrupt digests, malformed encodings or mismatched identities refuse the whole accelerated
  attempt and fall back to the ordinary full build. Missing records are ordinary cold inputs.
  JSON payloads are screened before typed decode: at most 250,000 tokens per record, depth 32,
  eight million tokens and 512 MiB encoded bytes per accelerated attempt. The writer skips facts
  exceeding the representation bounds. Source and aggregate Git bounds still apply. At most four
  readers run concurrently; supported local platforms open confined no-follow paths nonblocking
  and validate regular-file mode after open, so a FIFO swap cannot block. Because `os.Root` follows
  a leaf symlink whose target stays inside the root, the opened file must also be the same file as
  a no-follow `Lstat` of its name, or the attempt refuses (decision 0193). Other platforms refuse
  shard reads and fall back. Local
  symbols, words, counted terms, markers, imports and extraction refusals are reused; profile,
  module, document references, path terms, global postings and ordering are derived for the
  current tree. Subject-specific import narrowing and live Git status are preserved. Only the
  explicit snapshot writer creates or replaces shards, by synced temporary file and rename relative to a directory descriptor reached with a
  no-follow component walk;
  reads never repair corrupt state. Other build profiles and all output wires remain unchanged.
  Falsifiers: zero differing bytes on twenty Corvint-tree tasks with and without subjects across
  full build, gob and shard-assisted miss (120 packets); post-commit `context` with at most twenty
  changed files has nearest-rank p50 wall <=120 ms on an idle host over thirty fresh-process
  samples, reporting min/p95/max, raw samples and load. Each run registers binary and input
  digests beforehand. The plan is `docs/plans/blob-facts-shards-2026-09-05.md`; until these pass,
  acceleration remains proposed and opt-in. The first registered actual-tree run on 2026-09-05
  failed both falsifiers: shard p50 680.97 ms versus paired full 530.25 ms (max one-minute load
  8.10), and 15/120 packet strings differed (3 shard/full, 12 gob/full), in explanation identifiers
  and gob test-candidate withheld counts. No promotion is justified; raw evidence and exact
  differences are in the lane plan's linked result directory. Rename/dedup reuse and bounded shard retention are
  follow-up requirements, not delivered claims; explicit deletion of `blobs/` is the rollback.

### Accepted within the experimental pack profile (2026-09-05): analyzer schema engine

- `IDX-SNAP-V0-017`: (accepted 2026-09-05 by decision 0074 within the experimental pack profile) Under
  `CORVINT_SNAPSHOT_FORMAT=pack`, the `.aip` filename and header replace the executable digest
  with the first sixteen hexadecimal digits of SHA-256 over the explicit `analyzerSchemaID`,
  a newline, and the running Go version. This amends only the proposed pack's engine rule:
  the gob and sectioned formats retain IDX-SNAP-V0-001's executable digest and the gob write
  receipt keeps that engine. A rebuilt executable with unchanged analyzer schema and Go
  toolchain MUST reuse the pack. An incompatible analyzer or encoding change MUST bump the
  schema; a changed schema or toolchain MUST refuse the old pack and fall back to the current
  executable's gob, then the build. `index --if-stale` checks the selected pack, returning its
  path and engine when fresh, and writes it when absent or refused; the check verifies and decodes
  every section a full load reads, so a pack any loader refuses is never fresh (amended 2026-09-13). A source audit test pins
  all production Go files across every platform in `internal/contextindex`, `gitstatus`, `projectprofile`,
  `pythongrammar`, `pythonsyntax`, and `secretscreen`, plus `go.mod` and `go.sum` when present;
  source additions and edits require deliberate schema/audit review. New extraction dependencies
  MUST join that audit. The runtime never reads source files or uses the audit digest as its key.
  The writer independently bounds regular `.aip` files across trees and schema versions to the
  newest eight, reserving one slot for the pack just written even if its timestamp is older.
  This cannot rely on executable-key gob companion names. The opt-in receipt's `evicted` count
  includes these independent pack deletions; default gob/sectioned eviction is unchanged.
  The Go version conservatively invalidates standard-library parser changes. Existing identity,
  dirty status, corruption fallback, read-only behavior, and every `context`, `query`, `eval`,
  and harness output remain unchanged. Falsifier: independently rebuild two executables with
  different digests and unchanged analyzer inputs; the second must hit the first's pack with
  byte-identical packets while the ordinary gob misses. A third build with changed extraction
  and a bumped schema must miss, and the audit must reject extraction changes without an audit
  update. The preregistration and measured result live in
  `docs/plans/analyzer-schema-engine-2026-09-05.md`. Failure retains the executable-key format;
  deleting the opt-in packs is rollback. This is no ranking or absolute-latency claim.

### Accepted (2026-09-13, decision 0177): standalone `impact` and `feature` read the snapshot

- `IDX-SNAP-V0-019`: (accepted 2026-09-13 by decision 0177) standalone path `impact` (no `--base`, no `--working-tree-untracked`) and
  `feature` read the committed tree's snapshot on the terms `IDX-SNAP-V0-008` gives the query
  verbs, instead of building. Under `CORVINT_SNAPSHOT_FORMAT=pack` they read through
  `LoadSnapshotDeferred` and reread a refused body through the whole-file loader exactly as
  `IDX-SNAP-V0-015`'s second 2026-09-12 amendment describes; otherwise they read the gob snapshot.
  A miss builds as before, and only a build failure appends the `SOL-V0-007` unsupported
  observation and maps `unsupported-impact-repository` to `unsupported-feature-repository`, so the
  ledger is written exactly when it was. Stdout, stderr and exit code equal the build's on every
  hit, miss and refusal; neither verb writes a snapshot. The range (`--base`) and working-tree
  (`--working-tree-untracked`) profiles read `Source.Data` directly and still built
  until `IDX-SNAP-V0-021`. `prove`'s
  impact and change modes, `kernel`, `witness` and `prove --checkpoint` (`FPK-V0-024`) are
  unchanged. Falsifier: any byte of an `impact` or `feature` answer that differs between a cold
  build and a warm `index` hit, or a refused pack body that changes the answer. Rollback: restore
  the unconditional build in the `impact`/`feature` dispatch in `cmd/corvint/main.go`.

### Accepted (2026-09-13, decision 0180): `prove` impact and change modes, `kernel` and `witness` read the snapshot

- `IDX-SNAP-V0-020`: (proposed 2026-09-13, accepted 2026-09-13 by decision 0180) `corvint prove`'s
  impact mode (`prove PATH...`) and change mode (`prove --base`), `corvint kernel`, `kernel
  verify`, and `corvint witness` read the committed tree's snapshot on the terms
  `IDX-SNAP-V0-008` gives the query verbs, instead of building. Impact mode reads through
  `overSnapshot` exactly as standalone path `impact` does under `IDX-SNAP-V0-019` (deferred pack
  bodies, whole-file reread on a refused body, gob, then build). Change mode, `kernel` and
  `witness` take only the whole-file read (`snapshotOrBuild`): `RangeImpact` parses
  `Source.Data`, which a deferred pack body leaves nil, and `witness.Compile` reaches
  `RangeImpact`. A miss, including an absent `.corvint/index/`, a snapshot of another tree (stale),
  another engine, or a corrupt file, builds with `contextindex.Build` exactly as before, and a
  build failure reports the same error. Stdout, stderr and exit code equal the build's on every
  hit, miss and refusal; none of these verbs writes a snapshot. A hit reports the live `HEAD` as
  `CommitRevision`, so `witness --head` and `prove`'s closing tree check judge the same revision a
  build would. `prove --checkpoint` still builds and makes no snapshot-loader call (`FPK-V0-024`);
  `prove`'s CEM mode, the range and working-tree `impact` profiles, and `affected` are unchanged.
  Measured on this repository (`55f905ec`, 63 MB gob) with built binaries, median of 5, `HEAD~3`
  as the range base: `prove internal/contextindex/pack_reader.go` 1.27 to 0.39 s, `prove --base`
  2.45 to 1.67 s, `kernel` 1.02 to 0.15 s, `witness --base` 1.89 to 1.05 s at load 39-65; under
  `CORVINT_SNAPSHOT_FORMAT=pack` 0.41, 1.52, 0.28 and 1.08 s at load 29-33. Absent, stale (a snapshot
  of `HEAD~1`) and current snapshots gave byte-identical stdout, stderr and exit across both
  binaries on all four verbs. Falsifier: any byte of stdout or stderr, or an exit code, that
  differs between a cold build, a stale snapshot and a current hit, or a refused pack body that
  changes a `prove` impact answer. Rollback: restore `contextindex.Build` in `provePacket`,
  `runKernel` and `runWitness` and delete `snapshotOrBuild`.

### Accepted (2026-09-13, decision 0182): the range and working-tree `impact` profiles read the snapshot

- `IDX-SNAP-V0-021`: (proposed 2026-09-13, accepted 2026-09-13 by decision 0182) `corvint impact
  --base` (both range profiles, default and `--range-profile expanded-256`) and `corvint impact
  --working-tree-untracked` read the committed tree's snapshot on the terms `IDX-SNAP-V0-008` gives
  the query verbs, instead of building. They take only the whole-file read, as `prove --base` does
  under `IDX-SNAP-V0-020`: `RangeImpact` and `worktreeimpact.Compile` read `Source.Data`, which a
  deferred pack body leaves nil. A miss, including an absent `.corvint/index/`, a snapshot of another
  tree (stale), another engine, or a corrupt file, builds exactly as before; a build failure appends
  the same `SOL-V0-007` observation and reports the same error. A hit rehydrates `DirtyPaths`,
  `StatusSHA256` and the live `HEAD` from Git, so the range profiles' clean-worktree check and the
  working-tree profile's closing comparison against a fresh build judge the same values a build
  would; the working-tree profile still performs that closing build. Stdout, stderr and exit code
  equal the build's on every hit and miss; neither profile writes a snapshot. `affected` and `prove
  --checkpoint` (`FPK-V0-024`) are unchanged. Measured on this repository (`dd3e978c`, `HEAD~3` as
  the base, untracked `internal/contextindex/zz_scratch.go` plus a modified `README.md` for the
  working-tree profile) with built binaries, median of 5 at load 20-26: `impact --base` 1.84 to
  0.98 s, `--range-profile expanded-256` 1.76 to 0.88 s, `--working-tree-untracked` 2.00 to 1.13 s;
  under `CORVINT_SNAPSHOT_FORMAT=pack` 0.85, 0.84 and 1.11 s. Absent, stale (a snapshot of `HEAD~1`),
  current gob and current pack snapshots gave byte-identical stdout, stderr and exit across both
  binaries on all three invocations. Falsifier: any byte of stdout or stderr, or an exit code, that
  differs between a cold build, a stale snapshot and a current hit. Rollback: restore the
  unconditional build for `impactWorktree`/`impactBaseSet` in the `impact` dispatch in
  `cmd/corvint/main.go`.

## Non-goals and authority

No daemon, no watcher, no write from a read verb, no cross-repository store, no network. The
encoding is `encoding/gob` because the load is small against the target and the file is keyed,
not shared; it is not byte-deterministic across writes (map order) and is not the immutable
transport-neutral encoding `deployment-neutral-index-platform-v0.md` governs. Promoting an
encoding requires that spec's gate. `prove --checkpoint` and the Python oracle do not read the snapshot;
`query`, harness `user-prompt`, `file-change`, and compact `session-start` do
(`IDX-SNAP-V0-008`, `IDX-SNAP-V0-010`); `impact` reads it inside `corvint batch` under
`SBQ-V0-003` (decision 0052), and standalone path `impact` and `feature` read it under
`IDX-SNAP-V0-019` (decision 0177), `prove`'s impact and change modes, `kernel` and `witness` read it under
`IDX-SNAP-V0-020` (decision 0180), and the range and working-tree `impact` profiles read it under
`IDX-SNAP-V0-021` (decision 0182). Nothing here makes a snapshot exist: a verb that finds none
pays the build it always paid, so a measurement protocol that never runs `index` measures the miss
path.

## Failure modes

- A rebuilt binary: the engine digest changes, every existing snapshot misses, `index` must be
  rerun. Development builds pay a build per binary.
- A snapshot written by a different Corvint version with the same engine digest: impossible by
  construction (the digest is of the executable).
- A repository that commits `.corvint` or `.corvint/index` as a symlink: `index` refuses before its
  first write, so it never repairs an ignore file, publishes a snapshot, or evicts a file outside
  the worktree (`IDX-SNAP-V0-005`). Every reader misses on the same link, so a snapshot planted
  outside the worktree under this tree's name is never served; `index --if-stale` then refuses as
  `index` does.
- Two concurrent `index` runs on the same tree: each writes its own temporary file and renames;
  the last rename wins and both files are the same index.
- The temporary file's `Close` fails after a successful encode (a delayed write-back error):
  the write is refused and the temporary removed, the same as an encode failure, so the rename
  never publishes a file whose buffered bytes may not have reached disk (`IDX-SNAP-V0-001`,
  clarifying amendment 2026-09-13).
- A truncated or corrupt file, including a digest-consistent one whose offsets lie outside the
  file, a section, or a term table's slices: decode fails, the read misses and builds
  (`IDX-SNAP-V0-003`, `IDX-SNAP-V0-014`).
- A gob file whose header is intact but whose body is truncated (a crash after the rename of an
  unsynced temporary file, or outside damage): every loader misses, and `index --if-stale` reports
  it stale and rewrites it rather than calling it fresh (`IDX-SNAP-V0-011`, decision 0193).
- A gob file whose header and length are intact but which holds a zeroed range: when the range
  falls inside a source body, the gob decode accepts it and serves a wrong body under the right
  blob hash. The writers sync before rename, so a crash cannot publish such a range
  (`IDX-SNAP-V0-001`, decision 0210). Outside damage to a published gob file stays undetected. The
  sectioned and pack readers refuse any zeroed section or block (`IDX-SNAP-V0-014`,
  `IDX-SNAP-V0-015`).
- Another process truncates a mapped `.sect` or `.aip` file in place: the next touch of an unmapped
  page raises SIGBUS. This is an accepted residual (decision 0210). Corvint publishes only by rename,
  so it never truncates an inode a reader has mapped. Amended 2026-09-13 (bug hunt): the same
  residual covers another process rewriting a mapped `.aip` in place without truncating it. The
  private mapping may show the new bytes, and a block verified before the write is not verified
  again, so a later read can return bytes no digest covered. Corvint never writes a published inode
  in place, and a rewrite that changes the size or modification time is a different retained pack.
- Pack source lengths are stored as u32. Index admission records a tree entry over 1,000,000 bytes
  as the exclusion `source exceeds size bound` (`maxSourceBytes` in `internal/contextindex`), so no
  body that reaches a pack can overflow the field (decision 0210).
- A pack body corrupted after a `context` read opened the pack: the read of that body fails, the
  packet is withheld, and the verb rereads through the whole-file loader, which refuses the pack
  and falls back to the gob snapshot, then the build (`IDX-SNAP-V0-015`).
- A pack body corrupted on disk and read by `query`, `user-prompt`, `calibrate`, path `impact`
  (`IDX-SNAP-V0-019`), `prove`'s impact mode (`IDX-SNAP-V0-020`) or the `file-change` event: that body's read fails verification, the verb discards the result it
  computed over the deferred read and computes again through the whole-file loader, which refuses
  the pack and falls back to the gob snapshot, then the build. A corrupt body the verb never reads
  is not detected by that verb; the next whole-file read refuses it (`IDX-SNAP-V0-015`).
- First-party source under a directory named by an `IDX-SNAP-V0-018` member (a Go package at
  `build/` or `internal/target/`) is absent from the index. It is recorded in `Exclusions` with its
  reason, so the omission is countable rather than silent; the repository cannot opt it back in.
- Disk: about the size of the source bodies (44 MiB on the 3,233-file repository); at most eight
  files per repository.

## Acceptance evidence

`internal/contextindex/snapshot_test.go` (round trip equals the built index, dirty paths applied
from status, a new tree misses); `cmd/corvint/index_snapshot_test.go` (`context` never creates
the directory, `index` writes it with a `mutates:true` receipt, hit and miss bytes identical;
`TestQueryVerbsReadTheSnapshotWithoutChangingAByte` for the two query verbs,
`TestHarnessIndexBuildingEventsReadTheSnapshotWithoutChangingAByte` for the two index-building
harness events); `TestPackQueryAndEventLoadsVerifyABodyOnlyWhenItIsRead`
(`internal/contextindex/pack_test.go`) and
`TestPackQueryAndEventVerbsRereadARefusedBodyWithoutChangingAByte`
(`cmd/corvint/pack_snapshot_test.go`) for the deferred query and event loads, measured by
`BenchmarkPackQueryAndEventLoads`; `TestImpactAndFeatureReadTheSnapshotWithoutChangingAByte`
(`cmd/corvint/index_snapshot_test.go`) and the same refusal test for `IDX-SNAP-V0-019`;
`TestProveKernelAndWitnessReadTheSnapshotWithoutChangingAByte` and the same refusal test for `IDX-SNAP-V0-020`;
`TestImpactRangeAndWorkingTreeProfilesReadTheSnapshotWithoutChangingAByte` for `IDX-SNAP-V0-021`; the timing readings in `docs/BUILD-LOG.md` (2026-09-02).
`TestProbeSnapshotReadsOnlyTheMatchingHeader` (`internal/contextindex/snapshot_test.go`) probes a
written snapshot fresh and the same file truncated by one byte as a miss (`IDX-SNAP-V0-011`);
`TestBlobShardReadRefusesInRootLeafSymlink` (`internal/contextindex/blob_shards_open_test.go`)
refuses a shard name that is an in-root symlink (`IDX-SNAP-V0-016`).
`TestForbiddenPathScreenIsTheAcceptedSet` (`internal/contextindex/index_test.go`) pins
`IDX-SNAP-V0-018`'s component set, prefixes, pattern, reasons, and order.

The 2026-09-08 lifecycle amendment is exercised by `TestClaudeNativeDogfoodLifecycle`, including
startup/resume/clear/compact on a cold fixture with unchanged repository/Git bytes and no refresh
invocation, plus the Python adapter's no-background and interruption regressions. Explicit
`index --if-stale` tests remain unchanged. GPK-V0-017(a)'s original automatic-refresh condition is
not satisfied by explicit warmup; this change claims no packet-5 promotion.

## Rollback

Rolling back the lifecycle amendment retains safe bounded reads and explicit supervised warmup;
it does not silently restore an unowned detached child. The broader original snapshot rollback is:

Delete `internal/contextindex/snapshot.go`, `cmd/corvint/index_snapshot.go`, their tests, the help
topic, the dispatch line in `cmd/corvint/main.go`, the two lines in `runTaskContext`, and the
`snapshotIndex` call in `authorityStartQueryContext` and `repositoryQueryContext`; remove
`.corvint/index/` from `.gitignore`. Snapshot files are disposable derived state.

## Traceability

| Requirement | Implementation | Test |
|---|---|---|
| IDX-SNAP-V0-001 | `textSuffixes`, `admittedEntries`, `WriteSnapshot`, `runIndex` | `TestTaskContextSearchesRstDocumentation`, `TestSnapshotRoundTripAppliesDirtyPathsAndMissesOnANewTree`, `TestIndexWritesTheSnapshotThatContextReadsWithoutChangingAByte` |
| IDX-SNAP-V0-002 | `LoadSnapshot` | `TestSnapshotRoundTripAppliesDirtyPathsAndMissesOnANewTree`, `TestSnapshotHitReportsTheLiveCommitForTheSameTree`, `TestLoadSnapshotMissesWhenIdentityChangesDuringStatusRead` |
| IDX-SNAP-V0-003 | `LoadSnapshot`, `decodeSnapshot`, `TermTable.check` | `TestSnapshotRoundTripAppliesDirtyPathsAndMissesOnANewTree`, `TestSnapshotRefusesTermTableOffsetsOutsideTheirSlices` |
| IDX-SNAP-V0-004 | `buildEvidence`, `buildQueryAttempt`, `adoptStatus`, `LoadSnapshot` | `TestSnapshotRoundTripAppliesDirtyPathsAndMissesOnANewTree`, `TestSnapshotHitAndMissUseStatusDirtyPaths` |
| IDX-SNAP-V0-005 | `runTaskContext` (no writer), `WriteSnapshot` (`.gitignore`), `snapshotDirectoryPresent` (readers) | `TestIndexWritesTheSnapshotThatContextReadsWithoutChangingAByte`, `TestWriteSnapshotDoesNotRewriteMatchingGitIgnore`, `TestWriteSnapshotRefusesCommittedSymlinkedSnapshotDirectory`, `TestSnapshotReadersMissThroughCommittedSymlinkedSnapshotDirectory` |
| IDX-SNAP-V0-006 | `runTaskContext`, status-only `DirtyPaths` builders and loader | `TestIndexWritesTheSnapshotThatContextReadsWithoutChangingAByte`, `TestSnapshotHitAndMissUseStatusDirtyPaths` |
| IDX-SNAP-V0-007 | `evictSnapshots` | `TestEvictSnapshotsKeepsNewestEightIncludingCurrent`, `TestEvictSnapshotsRemovesStaleTemporaries` |
| IDX-SNAP-V0-008 | `snapshotIndex`, `authorityStartQueryContext`, `repositoryQueryContext`, `evalLearnedCandidates` | `TestQueryVerbsReadTheSnapshotWithoutChangingAByte`, `TestEvalQueryAcceptsStatusCleanIdentCheckout` |
| IDX-SNAP-V0-009 | `LoadSnapshot` | measured by the Beamfall miss-path reading; no unit test yet |
| IDX-SNAP-V0-010 | `snapshotIndex`, `harnessIndexedContext` | `TestHarnessIndexBuildingEventsReadTheSnapshotWithoutChangingAByte` |
| IDX-SNAP-V0-012 | Claude native lifecycle adapter and explicit warmup guidance | `TestClaudeNativeDogfoodLifecycle`; `tests/test_harness_claude.py` no-refresh and interruption regressions |
| IDX-SNAP-V0-013 | `pinnedFrom`, `pinnedEntry.exclusionReason`, `buildEvidence` | `TestBuildExcludesGitLFSPointerAndCarriesItThroughSnapshot` |
| IDX-SNAP-V0-018 | `forbiddenParts`, `generatedPath`, `forbiddenPath`, `admittedEntries` | `TestForbiddenPathScreenIsTheAcceptedSet`, `TestForbiddenPathExcludesAgentWorktreeCopies`, `TestTracePathScreenIsTheIndexScreen` |
| IDX-SNAP-V0-014 (proposed) | `encodeSectionedSnapshot`, `readSectionedSnapshot`, `readSnapshotIndex` | `TestSectionedSnapshotDecodesEverySectionToTheGobValues`, `TestSectionedFileChangeReadsOnlyItsSections`, `TestSectionedSnapshotRefusesACorruptSectionAsAMiss`, `TestSectionedSnapshotRefusesOutOfRangeOffsetsAsAMiss`, `TestSectionedRepeatedOpensRetainBoundedMappings`; timing gate not met (`docs/plans/sectioned-snapshot-prototype-2026-09-05.md`) |
| IDX-SNAP-V0-015 (proposed) | `encodePackSnapshot`, `readPackSnapshot`, `packTermView`, `cochangeSection`, `readSnapshotIndex`, `LoadContextSnapshotDeferred`, `LoadSnapshotDeferred`, `LoadEventSnapshotDeferred`, `overSnapshot`, `packBody` | `TestPackSnapshotDecodesEverySectionToTheGobValues`, `TestPackCochangeMatchesTheSpawnAfterAShallowCloneDeepens`, `TestPackCochangeMatchesTheSpawnAfterTheHistoryIsCutAtTheSameHead`, `TestPackTermViewFindMatchesTermPostings`, `TestPackSnapshotRefusesCorruptionTruncationAndOutOfRangeAsAMiss`, `TestPackSnapshotRefusesDuplicateOrOverlappingSections`, `TestPackCochangeServesTheSubjectHitAndFallsBackAfterAnAmend`, `TestPackSnapshotReadsWithoutChangingAByteOnTwentyTasks`, `TestPackRepeatedOpensRetainBoundedMappingsAndHistories`, `TestForgetPackHistoryReleasesRingSlot`, `TestPackContextLoadVerifiesOnlyTheBodiesAPacketReads`, `TestPackQueryAndEventLoadsVerifyABodyOnlyWhenItIsRead`, `TestPackDeferredBodyRefusalIsComputedOnceAcrossLaterLoads`, `TestPackQueryAndEventVerbsRereadARefusedBodyWithoutChangingAByte`; measurements in `docs/plans/corvint-pack-prototype-2026-09-05.md` |

| IDX-SNAP-V0-016 (proposed) | `buildWithBlobShards`, `writeBlobShards`, `BuildForSnapshot` | `TestBlobShardsReuseCommittedFactsAcrossChanges`, `TestBlobShardCorruptionRefusesAccelerationAndReadFallsBack`, `TestBlobShardRefusesFIFOWithoutBlocking`, `TestBlobShardPublicationPinsDirectoryAcrossSymlinkSwap`, `TestBlobShardRefusesForgedUnboundedFacts`, `TestBlobShardAggregateReadBudgetRefusesAcceleration`, `TestBlobShardsReadOnlyTwentyTaskMatrix`; actual-tree parity and idle-host timing falsified in the lane plan |

| IDX-SNAP-V0-017 (accepted within experimental pack) | `analyzerEngine`, `probeAnalyzerPack`, `WriteSnapshot`, `readSnapshotIndex` | `TestAnalyzerSchemaInputs`, `TestAnalyzerPackEngineIgnoresExecutableIdentity`, `TestAnalyzerPackProbeMissesACorruptBody`, `TestAnalyzerPacksStayBoundedAcrossTwelveTrees`, `TestAnalyzerPackEvictionReservesCurrentAndLeavesOtherFormats`, `TestPackSnapshotReadsWithoutChangingAByteOnTwentyTasks`; registered rebuild falsifier in `docs/plans/analyzer-schema-engine-2026-09-05.md` |

| IDX-SNAP-V0-019 (accepted, decision 0177) | `impact`/`feature` dispatch in `runContext`, `overSnapshot`, `deferredSnapshotIndex`, `snapshotIndex`, `standaloneImpactContext` | `TestImpactAndFeatureReadTheSnapshotWithoutChangingAByte`, `TestPackQueryAndEventVerbsRereadARefusedBodyWithoutChangingAByte` |
| IDX-SNAP-V0-020 (accepted, decision 0180) | `provePacket`, `runKernel`, `runWitness`, `snapshotOrBuild`, `overSnapshot` | `TestProveKernelAndWitnessReadTheSnapshotWithoutChangingAByte`, `TestPackQueryAndEventVerbsRereadARefusedBodyWithoutChangingAByte` |
| IDX-SNAP-V0-021 (accepted, decision 0182) | `impact` dispatch in `runContext`, `overSnapshot`, `snapshotIndex` | `TestImpactRangeAndWorkingTreeProfilesReadTheSnapshotWithoutChangingAByte` |
