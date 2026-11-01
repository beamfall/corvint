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
Admission amendments: `docs/decisions/0065-documentation-is-searchable-evidence-with-its-own-placement-2026-09-05.md`, `docs/decisions/0095-index-path-screen-is-the-go-set-2026-09-12.md` (`IDX-SNAP-V0-018`), `docs/decisions/0438-rc3-batch-3-requirements-accepted-2026-10-06.md` (`IDX-SNAP-V0-025`), `docs/decisions/0439-rc3-batch-4-requirements-accepted-2026-10-07.md` (`IDX-SNAP-V0-026`).

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
(`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/BUILD-LOG.md`, 2026-09-02). Every table of the index is a function of the committed tree
alone (`DIRTY-CACHE-001`: sources come only from `HEAD^{tree}`); the two fields that depend on the
worktree, `DirtyPaths` and `StatusSHA256`, are cheap to recompute. So the compiled index can be
written once per tree and read back, and the read costs the two Git observations a build already
makes plus a decode. This slice ships that file and the one verb that writes it. It does not change
what any packet says.

## Requirements

- `IDX-SNAP-V0-001`: `corvint [--root PATH] index` builds the index of the committed tree exactly as
  `context` would and writes it under `.corvint/index/` as
  `<object-format>-<tree-oid>-<engine>.gob`. Location amendment 2026-09-23 (V1-0212, pending owner
  review): the store is `corvint/index/` under the Git common directory, shared by linked worktrees,
  and `.corvint/index/` only when that directory cannot be resolved (`DIRTY-CACHE-013`); every
  mention of `.corvint/index/` in this spec names that store. Here `engine` is the first sixteen
  hex digits of the SHA-256 of the running executable. The file holds a header (format `corvint-index-snapshot/1`,
  object format, tree OID, engine) and the index with `Root`, `DirtyPaths`, and `StatusSHA256`
  cleared. The write is atomic (temporary file then rename). The receipt is one canonical JSON
  line with `mutates:true`, `path`, `bytes`, `tree`, `commit`, `engine`, `sources`, `symbols`,
  `evicted`. The searchable-text admission allowlist includes `.rst`, `.mdx`, `.txt`, `.mts` and
  `.cts` (IDX-SNAP-V0-108), and `.erb`, `.html`, `.htm`, `.haml`, `.slim`, `.hbs`, `.vue` and
  `.svelte` (IDX-SNAP-V0-109), and `.c`, `.h`, `.cc`, `.cpp`, `.cxx`, `.hh`, `.hpp`, `.hxx`,
  `.java`, `.scala`, `.pony`, `.adoc`, `.asciidoc`, `.css`, `.scss`, `.less`, `.pcss` and `.tpl`
  (IDX-SNAP-V0-110); these
  suffixes enter source and term tables without a symbol extractor. Clarifying amendment
  (2026-09-13, bug hunt): a failure closing the temporary file (a delayed write-back error
  surfacing only at `Close`) MUST be treated the same as an encode failure — the temporary is
  removed and the write refused — rather than renamed into place as if the buffered content had
  reached disk. Clarifying amendment (2026-09-13, decision 0210): the temporary file is synced to
  stable storage before it is closed and renamed, and a sync failure refuses the write in the same
  way. The gob decoder cannot detect a zeroed range inside a source body. Without the sync, a crash
  after the rename could publish such a range, and it would load as a wrong index. The sectioned
  and pack writers sync the same way. Amendment (accepted 2026-09-26, decision 0424; from decision
  0398; V1-0338): the file ends with the 32-byte SHA-256 of every byte before it (header and index
  message), so a body overwritten with the same number of bytes, or a zeroed range the gob decoder
  would accept, is detectable on read. Amendment (proposed 2026-10-06, V1-0870, pending owner
  review): the receipt also carries `store`, `store_shared`, `live_heads`, `live_trees` and
  `evicted_snapshots` (`IDX-SNAP-V0-025`). Amendment (accepted by decision 0440, V1-0928): when the writing worktree still has a superseded `.corvint/index`, it also
  carries `legacy_store`, `legacy_removed` and `legacy_left` (`IDX-SNAP-V0-027`).
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
  Amendment (accepted 2026-09-26, decision 0424; from decision 0398; V1-0338): a gob snapshot whose trailing SHA-256 does not
  match the bytes before it is undecodable, so every loader, and the `IDX-SNAP-V0-011` probe,
  treats a byte-corrupted file as this miss rather than serving text its blob does not contain.
- `IDX-SNAP-V0-004`: a dirty worktree reads the same snapshot; only `DirtyPaths` and
  `StatusSHA256` differ from the clean read (`DIRTY-CACHE-003`). The snapshot never holds worktree
  bytes. On both a hit and a miss, `DirtyPaths` is exactly the status snapshot's sorted path set.
  A status-clean path whose raw worktree bytes differ from its blob because of `ident`, `eol`, LFS,
  or another clean filter is read from the blob but is not added to `DirtyPaths`; the status
  snapshot is the dirty authority.
- `IDX-SNAP-V0-005`: `index` is the only verb that writes under `.corvint/index/`; `context` and every
  other read verb never create, touch, or delete a file there (invariant 4). The worktree fallback
  store ignores itself through a `.gitignore` it contains, so a repository with no rule for it stays
  clean; the shared store under the Git common directory is never tracked and has no such file.
  `index` writes that file only when it is absent or different, using temporary-file publication
  by rename rather than exposing a truncate-then-write interval to concurrent status readers. On its
  first write, `index` also creates `.corvint/.gitignore` only when that file is absent. The narrow
  file ignores itself, `index/`, `self-observations.jsonl`, and observation-writer temporaries; it
  MUST NOT hide CEM, work-queue, or other `.corvint/` paths and MUST NOT overwrite a repository-owned
  parent ignore file. This mutating bootstrap lets SOL-V0 attempt its ledger append without making a
  fresh repository dirty; no read verb may create the rule. Repository content is untrusted, so
  before and after creating the directory `index` refuses, with no write or eviction, when `.corvint`
  or `.corvint/index` exists as a symlink or any other non-directory (security amendment 2026-09-13),
  and in the shared store likewise when the common directory's `corvint` or `corvint/index` does.
  Every snapshot reader (`LoadSnapshot`, the event and context loaders, `LoadEventSnapshotObserved`,
  and the `IDX-SNAP-V0-011` probe) refuses the same shapes: a `.corvint` or `.corvint/index` that is a
  symlink or non-directory is the `IDX-SNAP-V0-003` miss, never a read of the bytes it points at
  (security amendment 2026-09-13, gosec G304 follow-up).
- `IDX-SNAP-V0-006`: a hit and a miss produce byte-identical `context` output for the same
  repository state and arguments.
- `IDX-SNAP-V0-007`: `index` keeps the newest eight snapshot files in the directory and removes
  the rest (`DIRTY-CACHE-007`'s entry bound). After a successful publish it also removes regular
  `snapshot-*.tmp` files at least ten minutes old, without counting them in the receipt's published
  snapshot `evicted` total; younger temporaries may belong to a concurrent writer and remain.
  Amendment (accepted 2026-09-26, decision 0424; from decision 0398; V1-0302): the temporary cutoff
  is ten minutes, since a writer holds its temporary only while it encodes and syncs an index
  already built. Beside the entry bound, the published `*.gob` files in one store stay within 1 GiB.
  Eviction orders files whose engine matches the snapshot just written first, newest first, then
  every other engine's files, newest first, and removes each file past the entry bound or past the
  byte budget; the snapshot just written is never removed, even when it alone exceeds the budget.
  Amendment (V1-0361): the same cutoff sweeps the other temporaries a killed writer can leave,
  `blob-*.tmp` in the store and in each `blobs/<engine>/` shard directory (reached through the
  shard writer's no-follow walk) and the `.gitignore-*.tmp` rewrite temporary.
  Amendment (accepted by decision 0438, V1-0870): `IDX-SNAP-V0-025` ranks
  the writing engine's snapshots of a live worktree HEAD's tree ahead of its others, within the
  same entry and byte bounds, and names every removed file.
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
  the identical `standaloneQueryContext` build (`cmd/corvint/prove.go:1112-1114@d473eb95`, reaching
  `deferredSnapshotIndex` and `snapshotIndex` at `cmd/corvint/main.go:1477-1478@c39315fe` and `cmd/corvint/harness_context.go:70-71@70282d2c`) before
  `compileProof` applies its own independent revision and dirty-set re-verification
  (`cmd/corvint/prove.go:632@7ad2b2af`, `cmd/corvint/prove.go:653-654@38c3dce4`, `cmd/corvint/prove.go:675-676@61b1bb0c`) and reads every cited blob at the
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
  receipt line with `mutates:false`, `state:"fresh"`, `path`, `tree`, `commit`, `engine` (and,
  under `IDX-SNAP-V0-025`, `store`, the directory holding `path`). Otherwise
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
  Superseded in part by decision 0517: for the stale-snapshot hook path only, the no-automatic-refresh /
  no-detached-child clause is replaced by `IDX-SNAP-V0-031` through `IDX-SNAP-V0-036`.
  Amendment authority: owner's 2026-09-08 cross-host lifecycle repair and every-child cleanup
  instruction; independent Gate A; partial supersession of decision 0049 item 3 only.
  (accepted amendment 2026-09-26, decision 0422; V1-0286) After it
  publishes a snapshot, the explicit `index` writer records how long its build took in one
  store-wide record, `build-cost.json` (`{"format":"corvint-index-build-cost/0",
  "buildMilliseconds":N}`), replacing the previous record by a synced temporary and rename. The
  write and the read pin each store directory without following links, the read opens the record
  without following a link or blocking, and the writer sweeps a stale record temporary as it
  sweeps snapshot ones. A failed record write does not fail `index` and adds no receipt field. The record is not a
  snapshot: it is outside `corvint-index-snapshot/1`, no snapshot byte depends on it, and eviction
  neither counts nor removes it. On a miss the hook's native read MAY read the record, running no
  Git process and writing nothing; when the recorded cost is at least the time left before the
  read's deadline, the read skips the in-memory build and reports the `AHI-031` stale-snapshot
  outcome at once. A missing, linked, non-regular, oversized or malformed record, including a cost
  longer than a Go duration can hold, means the miss builds as before, and so does a recorded cost
  that fits the time left, which keeps the in-budget build on a small repository. The record is a timing hint only and never an input to ranking, evidence or
  authority; a value recorded on a busier or quieter host can at worst skip a build that would
  have fit or start one that expires, and both outcomes already name the stale snapshot and its
  refresh argv. Rollback: remove the record read from the hook and the record write from `index`;
  a leftover `build-cost.json` is inert.
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
  Amended 2026-10-07 (proposed, V1-0983, `IDX-SNAP-V0-028`): eviction releases the retention's
  reference instead of only dropping it; the mapping is unmapped once every Index read from it has
  also been released, and stays mapped while one has not.
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
  Amended 2026-10-07 (proposed, V1-0983, `IDX-SNAP-V0-028` to `IDX-SNAP-V0-030`): eviction
  releases the retention's reference rather than only dropping it, and the mapping is unmapped once
  every Index read from it has also been released; an unreleased Index keeps it mapped as before.
  The strings the read hands out, the co-change entries included, now come from one heap copy of
  the verified string section per retained file and no longer alias the mapping.
- Amended 2026-10-07 (proposed, V1-0947; no GitHub issue), `IDX-SNAP-V0-014` and `IDX-SNAP-V0-015`
  unchanged in scope: the four-slot bound limits retained files, not live mappings. Values copied
  out of an Index (`Source.Data`, the `Source.Text` view, and before V1-0983 the string-table views
  such as `Symbol.Path`) alias the mapping without keeping the Index or any owner object reachable,
  so neither eviction nor the Index becoming unreachable proves the bytes unused. The process keeps
  each live mapping in a per-format table, keyed like the retention (path, size, modification time
  and header digest). A later read of an evicted file with the same key adopts that mapping and
  releases its own fresh one, which only the header read touched, so live mappings are bounded by
  the distinct keys a process reads, not by its cache misses (`TestEvictedReadsAdoptOneMappingPerFile`).
  A read adopts or registers the key's mapping before it decodes, so a concurrent first read whose
  retention is evicted during that decode shares the same mapping and the key never holds two
  (`TestRetainKeepsOneMappingWhenEvictedDuringDecode`). The key is sound without inode or change
  time: the header digest commits to every section digest, and every snapshot writer replaces the
  file by rename, never in place. Unmapping on eviction, or from a runtime cleanup of the Index or of
  the mapping owner, faults on those escaped values; `TestEvictedMappingsStayValidForEscapedAliases`
  guards that. A mapping leaves the table and is unmapped only when its last reference is released
  (V1-0983, `IDX-SNAP-V0-028` to `IDX-SNAP-V0-030`); while any Index read from it is unreleased, it
  stays, and a file the store has since deleted or replaced keeps its blocks allocated.
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
  exceeding the representation bounds. Source Git bounds still apply (the aggregate bound is withdrawn by `IDX-SNAP-V0-910`). At most four
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

### Proposed (2026-09-22, V1-0008, not accepted): lifecycle qualification

These two clauses record outcomes the current code already produces; they add no behaviour. They
qualify the default gob path only: the blob-shard path (`IDX-SNAP-V0-016`) stays proposed/off.

- `IDX-SNAP-V0-022`: (proposed 2026-09-22, V1-0008) for one committed tree, the index a snapshot
  hit serves MUST be byte-identical under a canonical encoding (JSON of the decoded `Index` with
  `Root`, `DirtyPaths` and `StatusSHA256` cleared, map keys ordered) whether the snapshot was written
  cold in a fresh clone or incrementally, after the same repository first indexed an earlier
  commit and then moved to this one. The moved repository MUST probe not fresh before the second
  `index` (`IDX-SNAP-V0-011`). This is identity of the served index, not of the file. (Amended
  2026-10-21 by `IDX-SNAP-V0-106`: the file bytes of two writes of one tree are now equal too; the
  transport-neutral encoding still stays behind the `deployment-neutral-index-platform-v0.md` gate.) Since 2026-10-20
  (`IDX-SNAP-V0-037`, V1-0328) the default path reuses verified per-source facts from the newest
  prior snapshot, so this identity now covers that reuse; the cold side of the test runs with
  `CORVINT_INDEX_REUSE=0` and reuses nothing. The reading below predates the reuse. Measured on this repository
  (target `1894b9e5`, prior `7e9b1856`, 3,819 tracked files): canonical index 87,658,954 bytes,
  sha256 `76b96397439184b02f7173942d76dfb87d5d7d35183a4d8b5c6bd0044ffdbfa9`, equal in three runs.
  Falsifier: any byte of the canonical encoding that differs between the cold and the incremental
  hit. Rollback: delete the test; nothing else depends on this clause.
- `IDX-SNAP-V0-023`: (proposed 2026-09-22, V1-0008) the snapshot lifecycle MUST end each hostile
  state below in the one stated outcome, with no panic and no error return:
  (a) unsupported input: a tracked source over `maxSourceBytes` is recorded as the single exclusion
  `source exceeds size bound`, a tracked file with an unadmitted suffix only raises
  `UnsupportedSuffixCount`, both stay in `Tracked` and out of `Sources`, a binary body under an
  admitted suffix is loaded but not valid text, and the snapshot round-trips equal to the build;
  (b) corruption: an empty file, a torn header, a torn body, a file one byte short, and a garbled
  header each make `LoadSnapshot` return no index, no hit and no error, `ProbeSnapshot` report not
  fresh with no error, and compact `LoadEventSnapshot` miss with no error (`IDX-SNAP-V0-003`,
  `IDX-SNAP-V0-011`); restoring the original bytes serves the original index again;
  (c) staleness: after a new commit changes the tree, every loader misses as in (b);
  (d) dirty state: a modified tracked file leaves the hit in place, lists only that path in
  `DirtyPaths`, serves the committed body, and writes nothing to `.corvint/index/`
  (`IDX-SNAP-V0-004`, `IDX-SNAP-V0-006`);
  (e) rollback: resetting the repository to an earlier commit whose snapshot is still retained
  (`IDX-SNAP-V0-007`) hits that snapshot, probes fresh, and serves an index byte-identical under
  `IDX-SNAP-V0-022`'s encoding to the one first served for that commit.
  Falsifier: a panic, an error return, or any other outcome in one of the five states. Rollback:
  delete the test; nothing else depends on this clause.

### Accepted under standing auto-accept (2026-10-11; V1-0328, 2026-10-20): verified prior-snapshot reuse

- `IDX-SNAP-V0-037`: (accepted under the owner's standing auto-accept (2026-10-11); 2026-10-20; V1-0328) when blob shards are off, `BuildForSnapshot`
  MAY reuse per-source facts from the newest prior snapshot of the same object format and engine in
  the shared store, and the served index MUST stay identical under `IDX-SNAP-V0-022`'s canonical
  encoding to a cold build of the same tree. Reuse is fail-closed at three boundaries:
  (a) the prior file is read only through `decodeSnapshot` (header, trailing digest, symbol windows,
  vocabulary check) under the identity its name states, and its name must round-trip through
  `snapshotPath`; any refusal, an engine or object-format mismatch, or a missing or symlinked store
  reuses nothing;
  (b) the prior's facts are regrouped per source only when the vocabulary paths equal the sorted
  sources, every postings table is canonical, every source is checked and named by its key, every
  symbol, marker, unparsed row and extraction note names a prior source, every symbol, marker and
  unparsed row carries that source's blob id (an import refusal carries none, as the extractor
  writes it), unparsed rows are unique per (path, facts), import sets are non-empty, and the per-source approximate-import flags sum to
  the prior's `ApproximateImports`; any failure reuses nothing;
  (c) a source is reused only when its prior blob id equals the new tree entry's id and its prior
  bytes have that entry's size and hash to that id under the object format (invariant 1); a reused
  source takes the new tree's mode. Every other source is pinned and read from Git as before.
  Whole-index products (document records, the term vocabulary order, symbol windows, the identifier
  graph) are recomputed; the vocabulary merges the prior postings, renumbered to the new source
  order with removed sources dropped, with freshly tokenised postings for the non-reused sources.
  Symbols sort by path, line, name, kind and end line so the order is independent of input order.
  The file format, bytes written and snapshot file name are unchanged: shrinking per-tree bytes
  needs a format change behind the `deployment-neutral-index-platform-v0.md` gate. Reuse is
  per-process derived input only; it never adds an authority, a row, or a confidence the cold build
  lacks. Measured 2026-10-20 (11,733-file clone of this repository at `a148e3a92`, `corvint index`,
  median of three, host load 30 to 45 on 12 cores): one-commit move 3.26 s real / 9.25 s CPU cold
  against 1.67 s / 3.46 s reused; thirty-commit move (1,435 changed files) 2.09 s / 4.34 s; bytes
  written 187,024,595 in every case (`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-20-v1-0328-incremental-reindex.md`).
  Falsifier: any canonical byte that differs from the cold build, a reused source whose bytes do
  not hash to the new tree's blob id, or any reuse from a prior that fails (a) or (b). Rollback:
  `CORVINT_INDEX_REUSE=0` restores the cold build per process; reverting
  `internal/contextindex/snapshot_reuse.go` and its two call sites removes the path; snapshot files
  are unchanged.

### Accepted (2026-09-25, decision 0394, panel blocker B9): non-UTF-8 tracked paths

- `IDX-SNAP-V0-024`: (accepted 2026-09-25 by decision 0394, panel blocker B9) a tracked path whose Git bytes are not
  UTF-8 MUST NOT refuse the index. The tree pass keeps it out of `Sources` and records it in
  `Exclusions` with the reason `unsafe-or-non-utf8-path` (the `init` gap name,
  `docs/specs/genesis-backfill.md`), under its display form: the path with each invalid byte run
  replaced by U+FFFD. The display form also stands in `Tracked` and `Skipped`, and in `DirtyPaths`
  when Git status reports the path, so a dirty non-UTF-8 path still makes freshness
  `mixed-worktree`. Every other path indexes and answers as before, and path `impact` and `prove`
  disclose the exclusion through the receipt's existing bounded exclusion sample. The `context`
  co-change slot leaves such a path out of its commit. Query history learning keeps its
  `unsupported-query-history` refusal (`GPK-V0`), and the `context` packet, which carries no
  exclusion member for any reason, gains none. A path that is only valid after the replacement is
  not recoverable from the display form; two distinct invalid paths can share one. Falsifier: a
  repository with one committed Latin-1 path that refuses `index` or path `impact`, or whose
  receipt omits the exclusion. Rollback: restore the three refusals in `internal/contextindex/git.go`
  and the parse flag in `history.go`.
- `IDX-SNAP-V0-026`: (accepted by decision 0439; V1-0416; no GitHub issue) on a fresh opening observation of the build's stability window
  (`buildStableFrom`: every retry, and the first attempt when no loader observation is carried)
  the tree listing (`git ls-tree -r -l -z --full-tree <tree>`) is issued beside the status scan,
  not after it. The listing reads immutable content named by the identity already read, so its
  result cannot depend on the scan's outcome, and the `GPK-V0-007` proof shape is unchanged: the
  closing observation still compares identity and dirty set against the opening one before the
  index is accepted. The source-count refusal (`Git tree exceeds the source-count limit`,
  `maxIndexedSources`) and the tree-byte bound therefore return while the scan is still running
  and before any `cat-file` is spawned; the build cancels the scan and waits for it before
  returning, so no observation outlives the build. When both fail, a scan that failed on its
  own (a git failure or the build's deadline, not the cancellation the listing issued) keeps
  its own refusal, code and message, as the sequential build reported it; the listing's error
  is reported only when the scan failed because the listing cancelled it; the scan classifies
  its failure at the moment it happens (`contextCancellation`: a Git command that returned after
  the context ended, or `StatusIn`'s own context exit), so a failure that preceded the
  cancellation is never re-read as one after `StatusIn`'s deferred cleanup. A carried loader
  observation on the first attempt is refused on its carried identity or status error before
  any listing, exactly as the sequential build refused it. Refusal codes, messages, limits and
  the `IDX-SNAP-V0-016` shard path (`openingObservation`) are unchanged. Measured on a 200,000-file
  repository (`index --if-stale`, medians of three under load 8-11): 2.39 s wall and 5.13 CPU s
  before, 0.54 s and 0.65 CPU s after, the same refusal; `context` 2.73 s / 5.93 CPU s to
  0.40 s / 0.60 CPU s. Falsifier: a build whose shimmed status reports completion before the
  refusal, any `cat-file` spawn before it, a carried observation whose identity or status read
  failed that still yields an index, or an independent status failure reported as the listing's
  refusal. Rollback: restore `openingObservation` followed by
  `buildEvidence` in the loop.

### Proposed (2026-10-06, V1-0870, pending owner review): named eviction and live worktree trees

- `IDX-SNAP-V0-025`: (accepted by decision 0438; V1-0870) the `index` receipt MUST name the store
  it wrote and every published file it removed. `store` is the snapshot directory and
  `store_shared` is true when that directory is the Git common directory's store every linked
  worktree reads (`DIRTY-CACHE-013`). `evicted_snapshots` lists each removed file in removal order
  as `{kind, path, bytes, tree, engine, live_head}`. `kind` is `snapshot` for a `*.gob` file,
  `sectioned` for its sectioned companion, and `pack` for an analyzer pack (`IDX-SNAP-V0-017`) or a
  legacy executable-keyed pack removed beside an evicted gob file. The list is empty, never absent, when
  nothing was removed, and `evicted` is its length. The swept writer temporaries are not
  published snapshots and are not listed. The `--if-stale` fresh receipt adds `store`. Before
  ranking, `index` reads the trees at the HEAD of every live worktree of the repository with
  `git worktree list --porcelain -z` and one `git cat-file --batch-check`. A worktree Git reports
  bare or prunable, or whose HEAD is unborn (all zeros), contributes no tree. `live_heads` is
  `OBSERVED` and `live_trees` the count when both reads succeed and every other listed HEAD
  resolves to a tree. When either read fails, or any such HEAD does not resolve (Git answers
  `missing` and exits zero), `live_heads` is `NOT_OBSERVED`, `live_trees` is 0, and eviction falls back to the `IDX-SNAP-V0-007` order. The
  gob ranking keeps the snapshot just written first. The writing engine's snapshots whose tree
  segment names a live tree come next, then the writing engine's others, then another engine's
  live trees, then the rest, each group newest first. Another engine's snapshot is never reused
  (`IDX-SNAP-V0-006`), so a live tree does not outrank a reusable snapshot only by being live. The entry bound and the
  1 GiB byte budget of `IDX-SNAP-V0-007` are unchanged. Live-tree protection is therefore
  bounded: a live tree past the bound is still removed and listed with `live_head:true`. The
  repository's live trees can need more than the budget, for example 143 worktrees at about
  120 MB each. Analyzer packs keep their own age order and only report `live_head`. Only the
  writer reads the live set, so no read verb gains a Git process (`AGENTS.md` invariant 4). A
  reused snapshot still matches the exact object format, tree and engine of its name. Falsifier:
  an `index` run that removes a file it does not list, or that removes a gob snapshot that fits
  the entry bound and the remaining byte budget while keeping one the ranking above places after
  it (a larger snapshot that would exceed the budget is skipped, and a smaller later one may stay). Rollback: drop the live set and the list from
  `WriteSnapshot`, `evictSnapshots` and `evictAnalyzerPacks`, and the new receipt keys from
  `runIndex`; the count-only `evicted` remains.

### Accepted (decision 0440, V1-0928): superseded per-worktree store

- `IDX-SNAP-V0-027`: (accepted by decision 0440; V1-0928) after an `index`
  write publishes to the shared store (`store_shared:true`, `DIRTY-CACHE-013`) and runs its
  `IDX-SNAP-V0-025` eviction, it MUST sweep the writing worktree's own `.corvint/index/`, the
  per-worktree store the shared one superseded (`IDX-SNAP-V0-001` location amendment). Only the
  writer does this; no read verb and no `index --if-stale` fresh probe removes anything (`AGENTS.md`
  invariant 4). The sweep inspects at most 256 entries (four times the shared store's entry cap)
  and removes only regular files it recognises: a snapshot, sectioned companion or pack named
  `<sha1|sha256>-<tree-oid>-<16-hex engine>.{gob,sect,aip}` (kinds `snapshot`, `sectioned`, `pack`),
  the `build-cost.json` record (`build-cost`), and a writer temporary (`snapshot-*.tmp`,
  `blob-*.tmp`, `.gitignore-*.tmp`) older than the existing ten-minute staleness cutoff
  (`temporary`). `.corvint` and `.corvint/index` are opened only when a no-follow `lstat` shows a
  real directory and the opened descriptor is that same directory, and every removal is relative to
  those descriptors, so a symlinked entry is never followed and swapping either directory for a
  link mid-sweep cannot redirect a file removal; the directory itself is removed with `rmdir`
  after re-checking that it is still the one opened, so a file or link swapped in is never deleted
  and at worst an empty directory swapped in at that path in the last instant is. When nothing is left, the store's own `.gitignore` is removed if it holds
  exactly `*\n` (`ignore`), then the empty directory (`directory`, 0 bytes). A `.corvint/index`
  that is a symlink or not a directory, a link, a subdirectory, a fresh temporary, any other
  name, an unreadable entry, a failed removal and an unfinished listing past the entry bound are
  left in place. The write is never refused by the sweep. When that directory exists, the receipt
  adds `legacy_store` (its path), `legacy_removed` (each removed file in the `evicted_snapshots`
  shape, `live_head:false`, tree and engine filled for the three published kinds) and
  `legacy_left` (`{path, reason}`); the three keys are absent when it does not, so the frozen
  receipt (`core-compatibility-freeze-v1.md`) is unchanged for a repository without one. The
  fallback store (`store_shared:false`) is the live store and is never swept. Falsifier: an `index`
  write that removes a file it does not list, removes anything outside that directory or behind a
  link, removes an unrecognised entry or a fresh temporary, touches the shared store beyond
  `IDX-SNAP-V0-025`, or any read verb that removes a legacy file. Rollback: delete
  `internal/contextindex/legacy_store.go`, the `sweepLegacyStore` call in `WriteSnapshot`, the
  three receipt fields and `legacyLeftPayload`; the legacy directory is then kept as before and
  can be removed by hand (it is disposable derived state).

### Proposed (2026-10-07, V1-0983, not accepted): bounded snapshot mapping lifetime

Pack and sectioned mappings were never unmapped: eviction dropped the ring's reference, and every
earlier Index, and every string or slice a caller took from it, could still alias the bytes. A
long-lived reader that opens more distinct snapshot files than the ring holds therefore kept every
mapping, and a deleted snapshot file's blocks stayed allocated until the process exited. Unmapping
on eviction, or from a collector cleanup when the Index became unreachable, faults on such an
alias, because a Go string or slice into a mapping keeps no Go object alive. Both were reproduced
as a SIGSEGV by `TestEvictedPackMappingKeepsEscapedAliasesValid`
(`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-07-v1-0983-snapshot-mapping-lifetime.md`).

- `IDX-SNAP-V0-028`: (proposed (V1-0983)) each pack or sectioned mapping MUST carry an explicit
  reference count: one reference for the opener, which the retention ring takes over when it
  retains the file, and one for each non-compact Index a read hands out, taken before the file is
  retained or, on a retained hit, under the ring's lock. Eviction, a lost retention race and a
  refused read release their reference, and `(*Index).Release` releases the Index's, once for the
  Index and every copy of it. The mapping is unmapped only when the count reaches zero, never on
  eviction alone and never because an Index became unreachable. Release is optional: an Index never
  released keeps its mapping valid for the process's life, exactly as before. A compact read
  copies its one field and takes no reference. Falsifier: an unreleased Index's aliases fault or
  change after eviction and collection, a released and evicted mapping keeps a reference, or the
  counts race. Tests: `TestEvictedPackMappingKeepsEscapedAliasesValid`,
  `TestReleasedEvictedPackMappingUnmaps`, `TestCompactPackReadTakesNoLease`,
  `TestConcurrentSnapshotReadReleaseEvict` (also under `-race`).
- `IDX-SNAP-V0-029`: (proposed (V1-0983)) every string a pack read hands out (Source `Path`,
  `BlobHash` and `Mode`, Symbol fields, `Tracked` and `Skipped` keys, `Vocabulary.Paths`, and the
  co-change history) MUST be heap-backed, served from one copy of the verified string section made
  once per mapped file, so it stays valid after the mapping is unmapped. The values that alias the
  mapping are lease-scoped: after `Release`, the caller MUST NOT read the Index's `Source.Data`,
  any `Source.Text` result, its `Vocabulary` term postings, or a value derived from them without
  copying. A sectioned read maps only `Source.Data`; its other fields are gob-decoded to the heap.
  `internal/doccorpus` already copies a source's text before keeping it. Falsifier: a released,
  evicted, unmapped read's strings fault or change. Test: `TestReleasedEvictedPackMappingUnmaps`
  (its negative control, serving strings from the mapping, faults).
- `IDX-SNAP-V0-030`: (proposed (V1-0983)) a process that reads and releases more distinct pack or
  sectioned files than its ring holds MUST keep at most the ring capacity (four) of that format
  mapped, and once a deleted file's retention is evicted and its reads released, nothing in the
  process may hold the file: the descriptor is closed when the read returns and the mapping is
  unmapped, so its blocks are freed. Falsifier: more live mappings than the capacity, or a mapping
  of a deleted, evicted, released file still live. Test:
  `TestDistinctSnapshotReadsKeepBoundedMappings`.

Callers wired: the analyzer pack probe (`probeAnalyzerPack`) releases the index it discards.
Unwired: the MCP bridge's `context` call returns a Result whose packet is serialized after the
call, and that packet may hold body text, so releasing there needs an escape audit first; until
then it keeps the earlier behaviour. One-shot CLI verbs exit instead. Non-goals: a process-wide
mapping per file identity (V1-0947) and any change to the file bytes, the refusal rules or the gob
fallback. Rollback: delete `internal/contextindex/snapshot_mapping.go` and the `owner`, `strings`
and `lease` fields, restore `retainedPack`/`retainPack`/`retainedSectioned`/`retainSectioned` and
`viewStrings` in `eventTables`; eviction then drops references without unmapping again.

### Accepted (2026-10-10, V1-1128, decision 0517): hook-triggered background refresh

This experimental amendment supersedes only the no-automatic-refresh / no-detached-child
clause of `IDX-SNAP-V0-012` (decision 0517). The rest of that clause remains recorded above; delivery
stays experimental. Human intent: after a commit or main move, the owner wants stale hook
snapshots refreshed automatically without a manual warmup.

- `IDX-SNAP-V0-031`: (accepted, decision 0517) On the existing stale-snapshot
  outcome, Claude Code and Codex adapters MAY start a separately exec'd copy of `os.Executable`
  with `--root ROOT index --if-stale`. Invariant 4 admission is limited to this explicit writer
  (`IDX-SNAP-V0-011`) and its coordination lock: the hook's evidence read remains non-mutating;
  neither ordinary reads nor fresh hook events acquire a writer. Lock and timing metadata never
  feed ranking, evidence or authority. Existing writer ignore maintenance and eviction remain
  writer-owned. This is an experimental proposal, not a new general read-command write exception.
- `IDX-SNAP-V0-032`: (accepted, decision 0517) A nonblocking flock on one
  `refresh.lock` in the shared snapshot store admits at most one background writer across its
  trees and linked worktrees. This stronger-than-per-tree bound limits host load without extra
  Git observations. The descriptor passes to the child; metadata records PID and start time.
  Completion clears metadata and releases the lock. The empty inode remains to prevent an
  unlink/recreate race. Dead or expired writers release their kernel lock; subsequent triggers
  overwrite stale metadata without trusting a recycled PID or stealing a live lock based on age.
  Acquisition and inherited-descriptor validation MUST open every store directory component
  relative to a pinned directory descriptor with O_NOFOLLOW|O_DIRECTORY, and open the lock leaf
  with O_NOFOLLOW|O_NONBLOCK. A component swapped for a symlink cannot redirect lock creation
  or metadata writes; linked leaves, hardlinks and non-regular lock files are refused.
- `IDX-SNAP-V0-033`: (accepted, decision 0517) The child owns a new session,
  redirects all standard streams to `/dev/null`, uses nice priority at least +10 and GOMAXPROCS=1,
  cancels work at 55 seconds and kills its process group at a hard 60-second wall bound (both
  bounds superseded by `IDX-SNAP-V0-912`: 55 CPU-seconds of work under a 10-minute wall). It
  refuses to build if lowering priority is denied; a started notice still acknowledges only spawn.
  Nice >= +10 covers every worker thread and inherited Git priority. On Linux a locked OS thread
  lowers its priority, then execs the same image once using an internal deadline/stage marker; fd 3 retains
  the flock across exec and becomes close-on-exec before Git dispatch. Darwin lowers process-wide
  priority without that extra exec. No external nice executable is required. Git
  descendants remain in that owned group. Normal completion and INT/TERM retire the group too.
  External SIGKILL of the worker PID alone cannot execute its timer or defers: its Git
  descendants may survive until their own exit. Operators needing forced retirement MUST kill
  the whole owned process group. This portable single-child design does not claim parent-death
  cleanup after leader-only SIGKILL; there is no surviving supervisor.
  The hook never waits for completion or builds synchronously and retains its existing work bound.
  Invariant 7 admits one finite child, no daemon, restart loop or supervisor. Under the V1-1120
  HLQ load envelope, scheduling can still exhaust a hook deadline: no latency/FULL qualification
  or safe load threshold is inferred. Busy stores skip spawning; later events may retry.
- `IDX-SNAP-V0-034`: (accepted, decision 0517) Publication, disk bounds and
  eviction remain `IDX-SNAP-V0-001`, `IDX-SNAP-V0-007`, `IDX-SNAP-V0-025` and `IDX-SNAP-V0-027`.
  Killing or expiring a writer publishes no partial snapshot. Complete files already renamed
  remain valid. Abandoned temporaries remain unreadable as snapshots and the next writer sweeps
  them at the existing ten-minute cutoff; no extra background sweeper is admitted.
- `IDX-SNAP-V0-035`: (accepted, decision 0517) `CORVINT_INDEX_AUTO_REFRESH=0`
  disables the spawn and retains the manual notice. Spawn failure, missing executable, unwritable
  or read-only store, lock refusal and unsupported platform all fail open with the existing stale
  code and exact manual argv. Notices distinguish started, already running, disabled and failed
  starts; started never means completed. There is no new configuration machinery or git hook install.
- `IDX-SNAP-V0-036`: (accepted, decision 0517) Acceptance requires a real
  commit-then-event regression whose next event consumes the completed snapshot, concurrent
  single-flight and stale-lock recovery, opt-out and failed-start checks, a slow-child hook-bound
  check, and killed/expired-writer checks including stale temporary sweep and descendant retirement.
  A leader-only SIGKILL test retains the descendant-cleanup residual and explicitly retires its
  surviving group. Tests own and reap every child. Cross-platform/live-host qualification remains separately named.
- `IDX-SNAP-V0-515`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0607) A
  writer that does not finish successfully stays visible to the next start. A worker that has
  validated its inherited lock and then exits nonzero, including when lowering its priority is
  refused, replaces the `refresh.lock` metadata with `{"pid","started","ended","outcome"}`, where
  `outcome` is `expired` when its 55-second work bound cancelled the build and `failed`
  otherwise; a completed writer still clears the metadata. A hard kill, crash or signal leaves
  the running record. A left record does not prove that no snapshot was published: a writer can
  publish and then fail or be killed in a later step, such as eviction or the build-cost record.
  When a hook then wins the lock and starts a writer, the stale notice (`AHI-053`) follows
  `Background index refresh started.` with one sentence: the previous refresh, started at its
  parsed UTC second, did not finish successfully, its closed cause (work limit under host load,
  failed, or stopped before recording an outcome), and that later events may also get no Corvint
  context until a refresh completes. The sentence is dropped, and the frame, status and manual
  argv kept, when it would push the hook output past its 8000-byte limit. Only the parsed start
  time and one of the three closed causes reach the notice, never the PID or raw bytes: an
  outcome other than `expired` or `failed` reads as stopped before recording an outcome, and a
  record without a parseable start names nothing. The read is at most 512 bytes of the lock the
  hook already holds. The record never feeds ranking, evidence or authority, and the priority,
  GOMAXPROCS, work and hard wall bounds of `IDX-SNAP-V0-033` are unchanged.
- `IDX-SNAP-V0-912`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3310;
  supersedes only the 55-second work and 60-second hard wall bounds of `IDX-SNAP-V0-033`) The
  writer's work bound is CPU time, not wall time: it cancels its build once the worker and its
  reaped Git children have used 55 CPU-seconds (user plus system, read with `getrusage` once a
  second), records `expired` and retires its process group, because
  compiling the index does not observe cancellation (the same retirement the hard wall uses). Its hard wall bound is 10 minutes: the work context
  also ends 50 seconds before it, and the group is killed at it. Rationale: the bound protects the
  host from a runaway writer, which CPU time measures, while a nice +10 writer under sustained
  normal-priority load is starved, not runaway. With 36 nice-0 busy loops on 12 CPUs it receives
  about 3.6% of one CPU, so a refresh that costs 8 CPU-seconds idle needs about 220 seconds of wall
  time and could never finish inside 60 (V1-0607 evidence: every writer killed, 0 of 20 hooks got
  context). A starved writer costs almost nothing while it waits and converges as CPU frees. While
  it runs it holds the single-flight lock (`IDX-SNAP-V0-032`), so later hooks report `already
  running` and start no other writer; a writer that exhausts its CPU budget needs that much work
  on an idle host too. The writer's build does not apply the shared 30-second Git deadline of
  `DCW-V0-075`: under the same load its evidence phase alone outlasts 30 seconds of wall time
  (observed: `Git repository index exceeded its deadline` after 266 s with 12.7 CPU-seconds used),
  so the writer's own CPU budget and wall bound govern its Git reads instead. The exemption is an
  in-process context mark the worker sets on itself (`contextindex.WithWriterBounds`); no flag or
  environment variable reaches it, and every other build and read keeps the 30-second deadline.
  Priority, GOMAXPROCS=1, the session, stream redirection, lock, record and
  publication rules of `IDX-SNAP-V0-031` to `IDX-SNAP-V0-036` and `IDX-SNAP-V0-515` are unchanged,
  and the child stays one finite process group (invariant 7). The `IDX-SNAP-V0-515` notice names the
  new limits: the 55-second CPU work limit for `expired`, the 10-minute hard limit for a record
  left running. Falsifier: under the recorded load a writer killed before its CPU budget or wall
  bound, or a second writer started while one runs. Rollback: restore the wall bounds
  (`snapshotRefreshWallBound = time.Minute`, the work deadline at 55 s) and drop the CPU ticker.
  Traceability: `TestSnapshotRefreshCPUBudgetCancelsWork_IDXSNAPV0912`,
  `TestSnapshotRefreshCPUBudgetRetiresWorkThatIgnoresCancel_IDXSNAPV0912`,
  `TestWriterBoundsReplaceTheGitDeadline_IDXSNAPV0912`,
  `TestSnapshotRefreshPreviousOutcome`; loaded qualification in
  `docs/build-log/2026-10-31-indexcli-w5.md`.

### Accepted (2026-10-21, V1-2130, V1-1732, owner's standing auto-accept 2026-10-11): analyzer schema maintenance

- `IDX-SNAP-V0-060`: (accepted under the owner's standing auto-accept (2026-10-11) (V1-2130)) The
  `IDX-SNAP-V0-017` schema rule keeps its scope: every edit to an audited analyzer input still
  bumps `analyzerSchemaID` and both pins of `TestAnalyzerSchemaInputs`. `script/analyzer-schema.sh
  bump [--base REF]` performs that bump as one command: it recomputes the audit digest exactly as
  the test does (reading the audited directory list from the test file, so the two cannot audit
  different inputs), and when the digest drifted or the number collides under `IDX-SNAP-V0-061`
  it sets the number to REF's number plus one, or keeps this branch's number when it is already
  above REF's, rewrites the schema constant and both test pins, then runs
  `script/repin-use-case-receipts.sh`. REF defaults to `origin/main`; an unresolvable REF refuses,
  writing nothing, so a bump never guesses its number. Repeated bumps on one branch keep one
  number. The test remains the authority: a digest the script computed wrongly fails it.
- `IDX-SNAP-V0-061`: (accepted under the owner's standing auto-accept (2026-10-11) (V1-1732))
  `script/analyzer-schema.sh check [--base REF]` (`make analyzer-schema-check`, which
  `make decision-numbers-check` and therefore CI's documentation step run) MUST refuse a tree whose
  audit digest differs from its pin, and a branch whose pins differ from both its merge base with
  REF and REF itself while its schema number is not greater than REF's: two pull requests that
  bump to the same number with different digests merge without a textual conflict on the number,
  and a reused number would let one pack name two analyzer contents. The refusal names REF's
  number and digest and the recovery, a rebump to REF's number plus one with `bump`. A branch
  whose pins equal its merge base's (it changed no analyzer input) is never refused for being
  behind REF. When REF does not resolve or shares no merge base, the collision comparison is
  reported `NOT_RUN` on stderr and only the digest check decides. Rollback: remove the
  `decision-numbers-check` prerequisite and the script; the in-tree pin test is unchanged.

Traceability: `script/analyzer-schema_test.sh` (`make analyzer-schema-test`) covers the drift
refusal, a first bump, a kept number on a second edit, a base that took the same number (refused,
then rebumped past it), a branch with unchanged pins behind a newer base, and an unresolvable base
(`NOT_RUN`, `bump` refuses); `TestAnalyzerSchemaInputs` passing on the script's pins proves the two
digests agree. Measured on a sample comment-only edit to `internal/contextindex/taskcontext.go`:
one command, 0.54 s wall, rewrote three lines in two files and left 18 receipts current, and the
pin test passed (`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-21-analyzer-schema-maintenance.md`).

### Accepted (2026-10-21, V1-1640, owner's standing auto-accept 2026-10-11): the miss names its build

Human intent: lanes in fresh clones called `context` without ever running `index`, so each call
paid a full in-memory build (measured 22.8 s wall, 7.25 s user, 813 MB at `c92ee0015` versus 5.8 s
with a snapshot) without being told; the build itself should also stop running the import regex
over bodies that cannot hold an import.

- `IDX-SNAP-V0-102`: (accepted under the owner's standing auto-accept (2026-10-11) (V1-1640)) When
  `context` or the path, range or working-tree `impact` and `feature` profiles find no usable
  snapshot and their in-memory build takes at least one second (`snapshotMissNoticeAfter`), they
  write exactly one stderr line naming the verb, that no snapshot matches the tree, the build's
  duration in milliseconds and the JSON argv `["corvint","--root",ROOT,"index"]`. Stdout bytes,
  the exit code and the snapshot store are unchanged; a faster build and a snapshot hit write
  nothing. The line is diagnostic: it is never evidence, ranking or authority input, and the read
  stays read-only (invariant 4). The one-second floor keeps fixture-sized builds silent.
- `IDX-SNAP-V0-103`: (accepted under the owner's standing auto-accept (2026-10-11) (V1-1640)) The
  generic import scanner runs its regular expression only over a body in which some `from` is
  followed by at least one of the expression's space runes and a quote, or some `import` by
  optional spaces, an optional `(` with optional spaces, and a quote (`mayHoldGenericImport`).
  That gate is exact on a match's opening, so it never skips a body the expression matches and
  every import edge, and therefore the index content, is unchanged; it bumps `analyzerSchemaID`
  under `IDX-SNAP-V0-017`/`IDX-SNAP-V0-060`.

Failure modes: a build just under one second stays silent; the notice reports wall time, which host
load inflates. Rollback: delete `noteSnapshotMissBuild` and its two calls, and `mayHoldGenericImport`
(restoring the quote-plus-keyword gate), with a schema rebump.

Traceability: `TestSnapshotMissNoticeNamesTheBuild` (notice text, unchanged stdout, silence under
the floor and on a hit, no store write), `TestGenericImportPrefilterLosesNoMatch` and
`FuzzGenericImportPrefilter`. Measured on this repository's 11,870 tracked files at `c9ca8beff`:
the gate skips 7,099 bodies (was 3,893) with identical extracted imports for every file, and
`corvint index` user CPU fell from 6.6 to 7.0 s to 5.8 to 6.0 s
(`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-21-lane-ctx-1021-context-index-impact.md`).

### Accepted (2026-10-21, V1-1640, owner's standing auto-accept 2026-10-11): `query` names its miss too

Human intent: V1-1640 asked that `context` "and other snapshot readers" name a snapshot-miss
build. `IDX-SNAP-V0-102` covered `context`, `impact` and `feature`; `query`, the other read the
pre-change step uses, still paid its miss build in silence (measured at `8def072e0`: 1.40 s wall
without a snapshot versus 0.57 s with one).

- `IDX-SNAP-V0-104`: (accepted under the owner's standing auto-accept (2026-10-11) (V1-1640)) The
  CLI `query` verb, on both its repository and authority-start paths, writes the
  `IDX-SNAP-V0-102` notice with verb `query` when it finds no usable snapshot and its in-memory
  build (`BuildEval` or `BuildQuery`) takes at least `snapshotMissNoticeAfter`, with the same
  text, floor, unchanged stdout and exit code, and no store write. The notice is armed only by the
  CLI adapter (`withSnapshotMissNotice`); the harness event path and `batch` share the build code
  and stay silent.

Failure modes: as `IDX-SNAP-V0-102`. Rollback: delete `withSnapshotMissNotice` and
`timedMissBuild` and restore the three bare build calls. Traceability:
`TestSnapshotMissNoticeNamesTheBuild` (both `query` paths).

### Accepted (2026-10-21, V1-2237, owner's standing auto-accept 2026-10-11): mapped snapshot decode
Human intent: the hook path decoded the 170 to 180 MB gob snapshot through read(2) in 270 to 390 ms;
the profile put about 22% in read syscalls and 17% in the trailer SHA-256, both serial with the
decode. The on-disk format stays unchanged (invariant 7), so no format gate applies; the edit to analyzer
inputs still bumps the schema (to `corvint-analyzer/139` in batch 1021h, shared with V1-0426 and V1-2276) under IDX-SNAP-V0-060.
- `IDX-SNAP-V0-105`: (accepted under the owner's standing auto-accept (2026-10-11) (V1-2237)) On a
  host where the snapshot file maps read-only, the default gob snapshot decode reads it through
  that mapping and hashes the payload for the IDX-SNAP-V0-003 trailer check on a second goroutine
  while the header and value decode, joining both before the mapping is released. It reads the
  same bytes and makes the same header, value and trailer checks, so every hit returns the value
  the read(2) decode returns and every mismatch is the same miss; no decoded value aliases the
  mapping. A memory fault while reading the mapping, such as a file truncated under it, is an
  error and so the IDX-SNAP-V0-003 miss, never a crash. A file that does not map takes the read(2)
  decode unchanged.
Failure modes: the remaining decode cost is gob's own message buffering and string copies, which
only a format change removes (left to the pack/sectioned formats and their gate). Rollback: delete
the `mapReadOnly` branch in `decodeSnapshotValue` and `snapshot_decode.go`.
Traceability: `TestMappedSnapshotDecodeMatchesStreamedDecode`, `TestMappedSnapshotDecodeFaultIsAMiss`
and the existing `TestSnapshotSameLengthBodyOverwriteIsAMiss`; measured in
`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-21-index-perf-1021i-hot-read-paths.md`.
- `IDX-SNAP-V0-108`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1088) The
  index admits the TypeScript module suffixes `.mts` and `.cts` as searchable text: they enter the
  source and term tables like `.rst`, with no symbol extractor. A reader that consumes indexed
  sources, such as the AMAP-V0-025 side-effect import closure, then reads such a module instead of
  failing closed on a file the index never held; a closure module that writes the table still
  fails closed. Failure modes: no symbol rule reads these suffixes yet, so `definition`, `impact`
  and `query` symbol evidence for them stays absent. Rollback: delete the two suffixes from
  `textSuffixes` and bump the analyzer schema.
Traceability: `TestAMAPV0025ChainBindingsFailClosed` (subtests `di closure reads .mts` and
`di closure reads .cts`).
- `IDX-SNAP-V0-109`: (accepted under the owner's standing auto-accept (2026-10-11); GH #1663)
  A Rake task file (`.rake`) is Ruby source: the index admits it as text and reads it with the
  Ruby symbol extractor, `#` comment markers and the Ruby region normalizer. The web and server
  template suffixes `.erb`, `.html`, `.htm`, `.haml`, `.slim`, `.hbs`, `.vue` and `.svelte` enter
  the source and term tables as searchable text, like `.rst`, with no symbol extractor and no
  marker syntax. A fixed allowlist is chosen over a repository-declared suffix map, which would
  add a project-authority configuration surface and key the cache on it; that remains a separate
  proposal. The golden analyzer corpus carries a `.rake` and an `.html.erb` file, so the schema
  number moves (IDX-SNAP-V0-107). Failure modes: a template's embedded code (`<% %>`, `<script>`,
  a Vue or Svelte script block) yields no symbols, imports or markers, so `definition`, `impact`
  and `query` symbol evidence for templates stays absent; a large generated HTML tree adds
  searchable text up to the existing source bounds. Rollback: delete the nine suffixes from
  `textSuffixes` and the `.rake` entries from `symbolExtractors`, `markerComments` and
  `SymbolRegionLanguage`, drop the two golden-corpus files and bump the analyzer schema.
Traceability: `TestTemplateAndRakeSuffixesAreIndexed_GH1663`, `TestAnalyzerSchemaOutput`.
- `IDX-SNAP-V0-110`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2716)
  The C-family suffixes `.c`, `.h`, `.cc`, `.cpp`, `.cxx`, `.hh`, `.hpp` and `.hxx`, the JVM
  suffixes `.java` and `.scala`, `.pony`, the AsciiDoc suffixes `.adoc` and `.asciidoc`, the
  stylesheet suffixes `.css`, `.scss`, `.less` and `.pcss`, and `.tpl` enter the source and term
  tables as searchable text, like `.rst`, with no symbol extractor and no marker syntax; `.adoc`
  and `.asciidoc` render as the `documentation` kind in task-context rows, as `.rst` does.
  Motivation: on the ContextBench full set 795 of the 4,540 gold files carried one of these
  suffixes and the index held none of them, so no ranking channel could return them. The analyzer
  golden corpus carries a `.java`, a `.css` and an `.adoc` file, so the audited output digest and
  the schema number move (IDX-SNAP-V0-107); packs built before this change are rebuilt rather than
  reused without these files. Failure modes: these sources yield no
  symbols, imports or markers, so `definition`, `impact` and `query` symbol evidence for them stays
  absent and only the lexical and task-context channels can rank them; a large vendored C or CSS
  tree not under a forbidden path adds searchable text up to the existing source bounds. Rollback:
  delete the eighteen suffixes from `textSuffixes` and the two AsciiDoc cases from the
  documentation-kind switch.
Traceability: `TestNativeJvmAndStylesheetSuffixesAreIndexed_V2716`, `TestAnalyzerSchemaOutput`.

### Accepted (2026-10-28, V1-3005, V1-2999, V1-3015, V1-3016, owner's standing auto-accept 2026-10-11): guarded mapped reads, verified sectioned probe, bounded symbol text

Human intent: a snapshot file that changes under its mapping, a corrupt sectioned body, or a source
with many symbols over one large declaration must cost a refusal or a bounded allocation, never a
process fault, a permanent miss or memory proportional to symbols times declaration bytes.

- `IDX-SNAP-V0-111`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3005;
  extends `IDX-SNAP-V0-105`) Every read of a mapped pack or sectioned snapshot, including the
  header digest, each section's SHA-256 check (on the goroutine that performs it) and the
  sectioned decode, runs under the mapped-read fault guard, so a file truncated under its mapping
  is a refusal and a cache miss, not a process fault. Failure modes: the guard covers reads during
  load only; strings a loaded index still aliases from the mapping are outside it (recorded in
  https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-28-ctxindex-bounded-readers-perf.md). Rollback: remove the guard calls.
- `IDX-SNAP-V0-112`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2999) The
  sectioned freshness probe reads every section and checks it against its digest, as the pack
  probe does, so a corrupt body is stale and the next build repairs it instead of the probe
  calling it fresh while every full load misses. Failure modes: the probe costs a full verified
  read. Rollback: restore the identity-only probe read.
- `IDX-SNAP-V0-113`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3015) A
  symbol extent's text is a substring of its one source text, the next-symbol line for an extent
  without a grammar end is a binary search over sorted symbol lines, identical symbol windows share
  one string, and the task store's know-how symbol table digests a shared declaration once, so S
  symbols over one B-byte declaration retain O(B) text and take O(S log S) time. Output is byte-for-
  byte the joined-lines reference. Rollback: restore per-symbol joins.
- `IDX-SNAP-V0-114`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3016) A
  symbol's context text is built by a capped join that stops encoding at the rune cap, so a window
  of long lines allocates O(cap), not O(window bytes); output equals truncating the full join,
  including U+FFFD replacement only on the truncating branch. Rollback: restore join-then-truncate.
Traceability: `TestPackMappedReadsRefuseATruncatedFile`, `TestSectionedMappedReadRefusesATruncatedFile`,
`TestSectionedProbeMissesACorruptBodyAndTheRebuildRepairsIt`, `TestSymbolExtentsMatchTheJoinedReference`,
`TestSymbolExtentsRetainOneDeclarationCopyPerSource`, `TestBuildSymbolWindowsSharesIdenticalWindows`,
`TestKnowHowSymbolTableDigestsSharedDeclarationsOnce`, `TestJoinLinesCappedMatchesTruncatedJoin`,
`TestSymbolContextTextAllocatesWithinTheCap`.

### Experimental (2026-10-29, V1-3133, standing lane auto-accept): the miss notice names the per-tree route
Human intent: a lane in a fresh linked worktree paid the full in-memory build on every `context` and
`impact` call, because each of its commits is a new tree that no snapshot matches, and the
`IDX-SNAP-V0-102` notice said to run `index` "once per checkout", which reads as one-time setup. A
snapshot is keyed by committed tree and binary in the store under the common Git directory, so every
worktree of the repository already shares it; a read cannot write it (invariant 4) and neither
permitted `.corvint/` ledger may hold it, so the warm route stays the operator's
`corvint index --if-stale`, run once per new tree.
- `IDX-SNAP-V0-115`: (experimental under the standing lane auto-accept; V1-3133; amends
  `IDX-SNAP-V0-102`) The snapshot-miss notice MUST name the argv
  `["corvint","--root",ROOT,"index","--if-stale"]` and end `once per committed tree, which every
  worktree of this repository then reuses`. A linked worktree whose HEAD tree another worktree of the
  repository indexed with the same binary MUST hit that snapshot with no index run of its own and no
  notice. A parser of the notice (`DCW-V0-377`) MUST admit this form and the `IDX-SNAP-V0-102` form
  an older binary writes, and no mix of the two. Stdout, the exit code, the store and the read-only
  contract are unchanged. Non-goals: a read that writes or refreshes the snapshot, an automatic
  refresh process (invariant 7), a per-worktree cache. Failure modes: an operator who skips the run
  keeps paying the build on every read of that tree; another binary's engine ID misses until it
  indexes. Evidence (2026-10-29, this repository, installed build 790): in a fresh linked worktree at
  a new tree, two `context` calls took 2.10 s and 2.16 s and two `impact` calls 2.02 s and 2.06 s,
  each printing the notice; `index --if-stale` took 1.69 s; then `context` took 0.40 s and 0.41 s and
  `impact` 0.31 s and 0.28 s. The base tree, already indexed from another worktree, took 0.65 s and
  0.51-0.56 s on the first call with no index run. `TestSnapshotMissNoticeNamesTheBuild` (new argv
  and wording; linked worktree hits silently) and `TestIDXSNAPV0115TreeNoticeKeepsTypedAbstention`
  (both forms admitted; mixed forms, another flag and trailing text refused). Rollback: restore the
  `IDX-SNAP-V0-102` argv and wording; the parser keeps admitting both.

### Accepted (2026-10-21, V1-2290, V1-2289, owner's standing auto-accept 2026-10-11): deterministic snapshot bytes and output-equivalent schema edits
Human intent: two writes of one tree produced different snapshot files because `encoding/gob`
writes maps in random order, so a file hash could not stand for its content (V1-2290); and every
edit to an audited analyzer input forced a schema bump, invalidating every pack, even when the
index it builds is byte-identical (V1-2289).
- `IDX-SNAP-V0-106`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2290) Two
  writes of the same committed tree by one binary MUST produce byte-identical snapshot files, for
  the default gob snapshot (format `corvint-index-snapshot/2`), the sectioned encoding and the
  experimental pack, whether the write was cold or reused prior facts (`IDX-SNAP-V0-037`). Every
  map the encodings carried is written as key-sorted pairs (`snapshot_wire.go`), nested maps inside
  record fields included, and the gob type ids are fixed at package init so they do not depend on
  which value a process encodes first. Decoding restores the same `Index`: nil and empty maps keep
  their distinction, and malformed pairs (lengths that disagree, values without the presence bit)
  are refused, which is the `IDX-SNAP-V0-003` miss. A `/1` file is refused as a format mismatch and
  rebuilt; the gob snapshot is executable-keyed, so no binary ever read both. This changes the
  encoding of the private cache, not its contract: it stays executable-keyed and is still not the
  immutable transport-neutral encoding `deployment-neutral-index-platform-v0.md` governs (Non-goals).
  The pack's sections change bytes, so the analyzer schema moves to `corvint-analyzer/145` (the lane's 147, renumbered by batch 1021v).
  Format gate evidence on this repository (9ae946a5c and a one-file commit on top, host load
  average 24 to 40), base binary against this change: snapshot 182,633,829 to 182,598,748 bytes;
  three cold `index` runs median 2.85 s to 3.12 s, max RSS 1,396 to 1,538 MB against 1,366 to
  1,564 MB; reuse `index` median 1.85 s to 1.91 s; 20 interleaved `context` loads p50 0.46 s to
  0.49 s, p95 0.57 s to 0.67 s, max RSS 912 to 914 MB; cross-format identity: the canonical JSON
  (`IDX-SNAP-V0-022`) of the index decoded from the base `/1` gob, the new `/2` gob and the new
  pack is one sha256 `554e1885...`; three writes of one tree hashed `c268c468...` each, where the
  base binary's three writes hashed three different values. Falsifier: two writes of one tree
  whose bytes differ. Rollback: revert `snapshot_wire.go` and its call sites and restore format
  `/1`; files written as `/2` then miss and rebuild.
- `IDX-SNAP-V0-107`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2289);
  amends `IDX-SNAP-V0-060` and `IDX-SNAP-V0-061`. `TestAnalyzerSchemaOutput` pins
  `auditedOutputSHA256`, the sha256 of the pack a fixed golden corpus (Go, Python, Ruby,
  TypeScript, JavaScript, feature and scenario records, an ADR, a spec, instructions, notes and a
  binary file) compiles to under a fixed engine, git identity and dates, and fails with the bump
  recovery when the bytes differ. When the test pins an output digest, `analyzer-schema.sh bump`
  runs that test (`CORVINT_ANALYZER_OUTPUT_COMMAND` overrides the command) and, when the output
  digest equals REF's, keeps REF's number and repins only the digests; otherwise it bumps as
  `IDX-SNAP-V0-060` says. `check` accepts REF's number with another input digest when both output
  pins are equal, and still refuses REF's number with another output digest. So a perf-only edit
  whose golden output is byte-identical needs no bump, and an output-changing edit still fails
  `TestAnalyzerSchemaOutput` and `check` until it bumps. A bump command that reports no output
  digest refuses. The golden corpus bounds the claim: a fact the corpus does not exercise can
  change without moving the output digest. Falsifier: an edit that changes the golden pack bytes
  and passes both the test and `check` at REF's number. Rollback: delete the output pin and test;
  the script then behaves as `IDX-SNAP-V0-060`/`061` (no output pin, every input edit bumps).
- `IDX-SNAP-V0-955`: (accepted under the owner's standing auto-accept (2026-11-01); V1-3574);
  amends `IDX-SNAP-V0-017`, `IDX-SNAP-V0-060`, `IDX-SNAP-V0-061` and `IDX-SNAP-V0-107`. The audited
  input digest is no longer pinned in source: `TestAnalyzerSchemaInputs` keeps the audited input
  set (it fails when a listed directory has no input) and the `auditedSchema` pin equal to
  `analyzerSchemaID`, and logs the input digest only. `auditedOutputSHA256` of
  `TestAnalyzerSchemaOutput` is the one content pin, so two branches that each edit an audited
  input (for example `internal/contextindex/receipt.go`) without changing the golden pack rewrite
  no pin line and cannot conflict on one. `TestAnalyzerSchemaOutputDetectsAnalyzerMutation` proves
  the remaining pin still fails when the analyzer's facts change: a mutated index of the same
  corpus encodes to another digest. `analyzer-schema.sh bump` runs the output test and, only when
  the output digest differs from the pin or the number collides, sets the number as
  `IDX-SNAP-V0-060`/`107` say and repins the output digest; `check` refuses a branch whose output
  pin differs from REF's while its number is not above REF's (`NOT_RUN` without REF, as before).
  It also prints, on stderr and without failing, `analyzer inputs changed since <merge base>`
  naming the changed audited paths, so the need to run `TestAnalyzerSchemaOutput` is visible at
  the start of a change. Residual: the golden corpus bounds the claim, as in `IDX-SNAP-V0-107`.
  Rollback: restore the `auditedSHA256` pin and the input-digest comparison in the script.
  Batch 1101a (2026-11-01): `IDX-SNAP-V0-890`..`892` delivered the same V1-3574 change first, and
  where the two differ they govern: the output pins also include `auditedShardSHA256`, and the
  changed-input note is the `IDX-SNAP-V0-890` stdout `NOTE` from `script/analyzer-inputs.sh`.
  This requirement keeps `TestAnalyzerSchemaOutputDetectsAnalyzerMutation`.
Failure modes: the golden corpus misses a fact class (residual, see the build log); the blob-shard
path (`IDX-SNAP-V0-016`, off by default) is not in the golden pack. V1-2232 (a re-index still
writes the whole file, 182,598,749 bytes after a one-file commit) is not addressed here:
deterministic bytes are the precondition for a content-addressed segment format, which needs its
own design and the deployment-neutral gate. Measured 2026-10-31 and declined for 1.0: chunking
would cut a one-file re-index's writes to about 2% but adds a per-read reassembly cost
(`docs/build-log/2026-10-31-lsp-cem-w2.md`).
Traceability: `TestSnapshotBytesAreDeterministic`, `TestSnapshotWireRoundTripsMaps`,
`TestAnalyzerSchemaOutput`, `script/analyzer-schema_test.sh`; evidence in
`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-21-ctxsnap-1021u-deterministic-snapshot.md`.
- `IDX-SNAP-V0-890`: (experimental under the standing lane auto-accept, 2026-10-31; V1-3574;
  amends `IDX-SNAP-V0-017`, `IDX-SNAP-V0-060`, `IDX-SNAP-V0-061` and `IDX-SNAP-V0-107`) The analyzer
  pins MUST be the schema number and the golden output only: `TestAnalyzerSchemaOutput` pins
  `auditedSchema`, `auditedOutputSHA256` and `auditedShardSHA256`, and `TestAnalyzerSchemaInputs`
  keeps discovering and reading every audited input (the same directories and files) and logs their
  digest but pins none, so an edit to an audited input that leaves the golden output unchanged moves
  no pin, and two branches that each make such an edit merge without a conflict on a pin line.
  `script/analyzer-schema.sh check` MUST note, on stdout, each such change (`NOTE N analyzer
  input(s) changed since MERGE_BASE, first PATH`) and keep the `IDX-SNAP-V0-061` collision refusal
  on the number and the output pins; `bump` MUST run the output test and rewrite the pins only when
  its digests differ from them, keeping REF's number when they equal REF's (`IDX-SNAP-V0-107`).
  Design: since `IDX-SNAP-V0-107` the output digest already decides whether a number moves, so the
  input pin only forced a mechanical one-line repin on every input edit, which every parallel lane
  rewrote differently. Non-goals: a wider golden corpus. Failure modes: an output change the corpus
  does not exercise keeps the number, as under `IDX-SNAP-V0-107`; an input edit that changes the
  output is still refused by `TestAnalyzerSchemaOutput` and by `check --output` until it bumps.
  Rollback: restore the input pin in `TestAnalyzerSchemaInputs` and the `audit_digest` comparison in
  the script.
- `IDX-SNAP-V0-891`: (experimental under the standing lane auto-accept, 2026-10-31; V1-2976,
  V1-3574) `script/analyzer-schema.sh check --output [--base REF]` MUST also run the output test and
  refuse, naming `script/analyzer-schema.sh bump`, when its digests differ from the pins.
  `script/dogfood-finish.sh` (`make dogfood-closeout`, `dogfood-finish`, `dogfood-inputs-check`)
  MUST, when the static check notes a changed analyzer input or fails, run `check --output --base
  BASE` before any build, prepare or bind and refuse as `REFUSE analyzer-schema-stale` with a `fix:`
  line naming the bump; `script/dogfood-change.sh` MUST print, at change start, `dogfood-change:
  NOTE analyzer-schema: N analyzer input(s) changed since BASE, first PATH: ...` from
  `script/analyzer-inputs.sh BASE`, the one lister of changed audited inputs, which the static
  check also uses and which runs no test, so `dogfood-change.sh` does not reach `script/heavy.sh`. `script/batch-regen.sh` bumps when
  `check --output` fails. Non-goals: adding the analyzer tests to `corvint test` selection. Failure
  mode: a repository without the analyzer schema files skips both steps.
- `IDX-SNAP-V0-892`: (experimental under the standing lane auto-accept, 2026-10-31; V1-2356) The
  golden corpus MUST yield at least one marker (it yields `feature:seed-demo`), and
  `TestAnalyzerSchemaOutput` MUST pin `auditedShardSHA256`, the SHA-256 over the blob-shard fact
  payload (`blobShardPayload`, the bytes `writeBlobShards` stores) of every golden source in path
  order under the fixed engine, each as `path NUL payload NUL`, so a shard-only output change moves
  a pin even though shards (`IDX-SNAP-V0-016`) stay off by default. A base whose test has no shard
  pin compares the pack digest only. `TestAnalyzerSchemaOutputCoversMarkersAndShards` MUST show that
  a marker-only index change moves the output digest and not the shard digest, and that one changed
  shard fact moves the shard digest. The default output command of `script/analyzer-schema.sh` MUST
  run through `script/heavy.sh` when it is executable. Failure mode: a shard fact the corpus does
  not exercise is not pinned.
Traceability for `IDX-SNAP-V0-890` to `IDX-SNAP-V0-892`: `script/analyzer-schema_test.sh` (`make
analyzer-schema-test`): two output-neutral `receipt.go` branches move no pin and merge cleanly, an
output change fails `check --output` and bumps once, a shard-only change bumps, a peer collision is
refused and rebumped, a base without the shard pin needs no bump, the default command runs through
a stub `script/heavy.sh`; `TestAnalyzerSchemaOutput` and
`TestAnalyzerSchemaOutputCoversMarkersAndShards` in `internal/contextindex`; the dogfood steps in
`docs/build-log/2026-10-31-frictionfix-w4.md`.

### Accepted (2026-10-31, V1-2881, owner direction 2026-10-27, standing auto-accept 2026-10-11): no aggregate admission wall

Human intent (owner, 2026-10-27): the 128 MiB aggregate admission bound is a brick wall that would
block larger projects; if speed is the worry, optimize how the index is built instead of refusing.

- `IDX-SNAP-V0-910`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2881) The
  index, query, feature and impact builds MUST NOT refuse a committed tree for the total size of its
  admitted sources. The aggregate refusal of decision 0424 (`unsupported-impact-repository`,
  `unsupported-feature-repository` or `unsupported-query-repository` naming the 128 MiB bound) is
  withdrawn. What still bounds a build: the per-source `maxSourceBytes` (1,000,000 bytes, larger
  blobs are `source-unreadable` exclusions), the vendor/generated path exclusions of
  `IDX-SNAP-V0-018`, the 64 MiB tree-listing read and the 200,000-entry source count (both
  unchanged: the count bounds per-entry bookkeeping before any blob is read, and no observed
  repository approaches it). Blob bodies are read in chunks: one `git cat-file --batch` call reads
  at most 32 MiB of framed output, its exact length known from the tree sizes before the call, so
  every reader stays bounded per call and no single read buffer scales with the repository. The
  compiled index still holds every admitted source in memory, so peak memory grows with admitted
  bytes; the build log records it on a 575 MB fixture. Output is unchanged for any tree the old
  bound admitted: chunking changes how bytes are read, never which bytes.
- `IDX-SNAP-V0-911`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2881) The
  `index` receipt reports, beside `sources`, `source_bytes` (the summed bytes of the admitted
  sources) and `build_ms` (the build's elapsed wall time in whole milliseconds, the measurement
  already recorded as the build cost). Both are report fields only: they enter no snapshot byte and
  never feed ranking, evidence or authority.

Falsifier: a tree whose admitted sources exceed 128 MiB refused for aggregate size, or a
`cat-file --batch` read whose limit is derived from the whole candidate set. Rollback: restore
`validateBlobAdmission` at its call sites and the single-batch `fetchBlobs`; the receipt fields
can be dropped independently. Traceability: `TestFetchBlobsReadsBoundedChunks_IDXSNAPV0910`,
`TestIndexBuildingCommandsDoNotRefuseAggregateSize_IDXSNAPV0910`, `TestIndexReceiptReportsSourceBytes_IDXSNAPV0911`;
measurements in `docs/build-log/2026-10-31-indexcli-w5.md`.

## Non-goals and authority

Accepted, decision 0517 (V1-1128): the adapter-only writer/lock exception in
`IDX-SNAP-V0-031` through `IDX-SNAP-V0-036` admits no daemon, git-hook installer, synchronous
hook build, supervision or new ranking/evidence authority. Ordinary read verbs keep the baseline.

No daemon, no watcher, no write from a read verb, no cross-repository store, no network. The
encoding is `encoding/gob` because the load is small against the target and the file is keyed,
not shared; since `IDX-SNAP-V0-106` its bytes are deterministic per tree, but it is not the immutable
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

Accepted, decision 0517 (V1-1128; IDX-SNAP-V0-031 through IDX-SNAP-V0-036): a missing
executable, spawn error, linked/unwritable lock or read-only store keeps the stale fail-open notice
and manual argv. A busy writer produces an already-running notice; killed/expired children leave
no partial published snapshot, release flock automatically and retain only stale metadata or
unpublished temporaries until retry/sweep. A tree moving again can still miss on the next hook.

Measured, V1-0607 (2026-10-30, linux/amd64, 12 CPUs): a cold refresh of the Corvint tree costs
about 8 CPU-seconds at `GOMAXPROCS=1`. Against sustained CPU load at normal priority (36 to 72
busy loops) the nice +10 writer can get too little CPU to finish within the 60-second bound:
writers were killed at the hard bound, never at the 55-second cancel, and published nothing.
Each later hook started another doomed writer, so context stayed lost for as long as the load
lasted. One 36-loop rerun finished in 35 s, so the outcome depends on scheduling. With the load
itself at nice 15 the writer finished in 27 s. A writer at nice 0 still took 88 s under 36 loops.
`IDX-SNAP-V0-515` makes the repeated expiry visible; recovering under such load needs either a
lower-priority load (V1-2440) or an owner amendment to the `IDX-SNAP-V0-033` bounds.
Measured, V1-3310 (2026-10-31, linux/amd64, 36 nice-0 loops pinned with the writer to 12 of 32
CPUs, host load 42): under `IDX-SNAP-V0-912` the one writer published after 336.8 s with its lock
record cleared, the hook during it reported `already running`, and the next hooks were no longer
stale (they met the expected `dogfood-event-deadline` time bound of LCP-V0-008 under that load).

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
  files per worktree, up to 64 and 1 GiB per shared store (`IDX-SNAP-V0-007`).
- More live worktree trees than the store's bound or byte budget holds: the newest live trees
  stay, and each older one is removed and listed with `live_head:true`, so that worktree's next
  event misses and reports a stale snapshot rather than evicting without trace
  (`IDX-SNAP-V0-025`).
- `git worktree list` or `cat-file` fails or exceeds its bound during `index`: the receipt reports
  `live_heads:"NOT_OBSERVED"` and eviction uses the engine-and-age order alone; the write is not
  refused.
- A superseded `.corvint/index` holding something the sweep does not recognise (an operator file,
  a `blobs/` shard directory an experimental build left, a link): those entries, the ignore file and
  the directory stay, every write names them in `legacy_left`, and the operator removes them by
  hand (`IDX-SNAP-V0-027`). More than 256 entries are finished over several writes only when the
  first batch is removable; a batch of unrecognised entries stalls it, reported as "entry bound
  reached". An older Corvint binary that still uses that store in the same worktree loses its
  snapshots and rebuilds; a temporary it is still writing is younger than the cutoff and kept.
  Another worktree's legacy store, and one in a repository whose fresh probe never writes, is
  swept only by its own next write.

## Acceptance evidence

Accepted, decision 0517 (V1-1128; `IDX-SNAP-V0-031` through `IDX-SNAP-V0-036`):
`TestSnapshotRefreshCommitThenEvent` witnesses both adapters consuming the refreshed snapshot;
the single-flight, stale-lock, opt-out/failure, slow-child and interruption tests below cover the
bounded lifecycle. The interruption fixture asserts a snapshot miss before fault injection and
at its readiness marker, then independently checks child retirement, lock recovery, no publication,
stale fail-open and temporary sweep. The test helper logs NOT_OBSERVED when its sandbox denies
nice priority and substitutes only that scheduler syscall; this does not qualify real niceness,
live host latency, Linux, or the V1-1120 load envelope. See the V1-1128 build-log entry for results.

V1-0607 (`IDX-SNAP-V0-515`): `TestSnapshotRefreshPreviousOutcome` pins the closed causes, names
nothing for malformed and PID-only records, and reports an unknown outcome as stopped before
recording one without echoing it; `TestSnapshotRefreshPriorityDenied` asserts the refused priority
setup records `failed`; `TestSnapshotRefreshNoticeNamesPreviousOutcome`
proves the hook notice carries the previous outcome and that the completing writer clears it;
`TestSnapshotRefreshInterrupted` asserts that every killed mode is reported as stopped before
recording an outcome. Linux live qualification through the real hook command, at idle and under
load, is in `docs/experiments/v1-0607-hook-refresh-qualification-20261030.json`.

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
`TestImpactRangeAndWorkingTreeProfilesReadTheSnapshotWithoutChangingAByte` for `IDX-SNAP-V0-021`; the timing readings in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/BUILD-LOG.md` (2026-09-02).
`TestColdAndIncrementalSnapshotsAreByteIdentical` and
`TestSnapshotLifecycleHostileStatesHaveBoundedOutcomes`
(`internal/contextindex/lifecycle_qualification_test.go`) for `IDX-SNAP-V0-022` and
`IDX-SNAP-V0-023`; the corpus subtest runs only when `CORVINT_LIFECYCLE_CORPUS` names a clone
(`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/BUILD-LOG.md`, 2026-09-22 V1-0008).
`TestSnapshotReuseMatchesColdBuild`, `TestSnapshotReuseRefusesUnverifiedPrior`,
`TestSnapshotReuseRefusesRowsBoundToOtherBlobs`,
`TestSnapshotReuseRecomputesIdentGraphForUnchangedReferrers` and
`TestMergeReusedPostingsEqualsFullInversion` (`internal/contextindex/snapshot_reuse_test.go`), with
the reuse counts asserted in `TestColdAndIncrementalSnapshotsAreByteIdentical`, cover
`IDX-SNAP-V0-037` (`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-20-v1-0328-incremental-reindex.md`).
`TestProbeSnapshotReadsOnlyTheMatchingHeader` (`internal/contextindex/snapshot_test.go`) probes a
written snapshot fresh and the same file truncated by one byte as a miss (`IDX-SNAP-V0-011`);
`TestSnapshotSameLengthBodyOverwriteIsAMiss` (`internal/contextindex/snapshot_test.go`) overwrites
one body with the same number of bytes and shows the probe and every loader miss
(`IDX-SNAP-V0-003`, `IDX-SNAP-V0-011`, V1-0338);
`TestBlobShardReadRefusesInRootLeafSymlink` (`internal/contextindex/blob_shards_open_test.go`)
refuses a shard name that is an in-root symlink (`IDX-SNAP-V0-016`).
`TestForbiddenPathScreenIsTheAcceptedSet` (`internal/contextindex/index_test.go`) pins
`IDX-SNAP-V0-018`'s component set, prefixes, pattern, reasons, and order.
`TestEvictSnapshotsNamesRemovalsAndKeepsLiveHeadTreesFirst` and
`TestLiveWorktreeTreesNamesEveryLiveHead` (`internal/contextindex/snapshot_test.go`) and
`TestIndexKeepsALinkedWorktreesSnapshotAndItsPromptReusesIt` (`cmd/corvint/index_snapshot_test.go`)
cover `IDX-SNAP-V0-025`. The last writes a linked worktree's snapshot, fills the shared store's
bound with newer non-live snapshots, and writes from the main worktree. The receipt names the one
evicted file, the linked snapshot survives, and the linked worktree's `index --if-stale` is fresh
on the same path. Its prompt event then succeeds with a context packet and no build. With the
live set disabled the unit test and this test both fail.

The 2026-09-08 lifecycle amendment is exercised by `TestClaudeNativeDogfoodLifecycle`, including
startup/resume/clear/compact on a cold fixture with unchanged repository/Git bytes and no refresh
invocation, plus the Python adapter's no-background and interruption regressions. Explicit
`index --if-stale` tests remain unchanged. GPK-V0-017(a)'s original automatic-refresh condition is
not satisfied by explicit warmup; this change claims no packet-5 promotion.

`TestIndexWriteSweepsTheSupersededWorktreeStore`, `TestIndexWriteLeavesUnrecognisedLegacyEntries`
(`internal/contextindex/legacy_store_test.go`) and `TestIndexReceiptNamesTheSweptLegacyStore`
(`cmd/corvint/index_snapshot_test.go`) for `IDX-SNAP-V0-027`: a seeded 1 MiB legacy
store is reclaimed in full and named, the shared snapshot still loads, links, a fresh temporary and
unrecognised entries are left with their reason, and the receipt keeps the frozen key set when no
legacy store exists (`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-07-legacy-index-store-sweep.md`).

## Rollback

Accepted, decision 0517 (V1-1128): set `CORVINT_INDEX_AUTO_REFRESH=0`; an already
started child remains bounded and exits. Reverting the amendment and its implementation restores
manual refresh argv behavior. An empty `refresh.lock` is inert; do not unlink a live flock inode.


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
| IDX-SNAP-V0-003 | `LoadSnapshot`, `decodeSnapshot`, `decodeSnapshotValue`, `TermTable.check` | `TestSnapshotRoundTripAppliesDirtyPathsAndMissesOnANewTree`, `TestSnapshotRefusesTermTableOffsetsOutsideTheirSlices`, `TestSnapshotSameLengthBodyOverwriteIsAMiss` |
| IDX-SNAP-V0-004 | `buildEvidence`, `buildQueryAttempt`, `adoptStatus`, `LoadSnapshot` | `TestSnapshotRoundTripAppliesDirtyPathsAndMissesOnANewTree`, `TestSnapshotHitAndMissUseStatusDirtyPaths` |
| IDX-SNAP-V0-005 | `runTaskContext` (no writer), `WriteSnapshot` (`.gitignore`), `snapshotDirectoryPresent` (readers) | `TestIndexWritesTheSnapshotThatContextReadsWithoutChangingAByte`, `TestWriteSnapshotDoesNotRewriteMatchingGitIgnore`, `TestWriteSnapshotRefusesCommittedSymlinkedSnapshotDirectory`, `TestSnapshotReadersMissThroughCommittedSymlinkedSnapshotDirectory` |
| IDX-SNAP-V0-006 | `runTaskContext`, status-only `DirtyPaths` builders and loader | `TestIndexWritesTheSnapshotThatContextReadsWithoutChangingAByte`, `TestSnapshotHitAndMissUseStatusDirtyPaths` |
| IDX-SNAP-V0-007 | `evictSnapshots` | `TestEvictSnapshotsKeepsNewestEightIncludingCurrent`, `TestEvictSnapshotsRemovesStaleTemporaries`, `TestEvictSnapshotsBoundsBytesAndEvictsOtherEnginesFirst`, `TestEvictSnapshotsRemovesStaleShardAndIgnoreTemporaries` |
| IDX-SNAP-V0-008 | `snapshotIndex`, `authorityStartQueryContext`, `repositoryQueryContext`, `evalLearnedCandidates` | `TestQueryVerbsReadTheSnapshotWithoutChangingAByte`, `TestEvalQueryAcceptsStatusCleanIdentCheckout` |
| IDX-SNAP-V0-009 | `LoadSnapshot` | measured by the Beamfall miss-path reading; no unit test yet |
| IDX-SNAP-V0-010 | `snapshotIndex`, `harnessIndexedContext` | `TestHarnessIndexBuildingEventsReadTheSnapshotWithoutChangingAByte` |
| IDX-SNAP-V0-012 | Claude native lifecycle adapter and explicit warmup guidance; build-cost record (accepted, decision 0422; `RecordBuildCost`/`RecordedBuildCost`, `dogfoodMissOutlastsDeadline`) | `TestClaudeNativeDogfoodLifecycle`; `tests/test_harness_claude.py` no-refresh and interruption regressions; `TestBuildCostRecordRoundTripsBesideTheSnapshots`, `TestBuildCostRefusesAnOverflowingRecordAndSweepsItsTemporary`, `TestRecordedBuildCostRejectsFIFOWithoutBlocking`, `TestDogfoodEventSnapshotMissUsesRecordedBuildCost` |
| IDX-SNAP-V0-013 | `pinnedFrom`, `pinnedEntry.exclusionReason`, `buildEvidence` | `TestBuildExcludesGitLFSPointerAndCarriesItThroughSnapshot` |
| IDX-SNAP-V0-018 | `forbiddenParts`, `generatedPath`, `forbiddenPath`, `admittedEntries` | `TestForbiddenPathScreenIsTheAcceptedSet`, `TestForbiddenPathExcludesAgentWorktreeCopies`, `TestTracePathScreenIsTheIndexScreen` |
| IDX-SNAP-V0-014 (proposed) | `encodeSectionedSnapshot`, `readSectionedSnapshot`, `readSnapshotIndex` | `TestSectionedSnapshotDecodesEverySectionToTheGobValues`, `TestSectionedFileChangeReadsOnlyItsSections`, `TestSectionedSnapshotRefusesACorruptSectionAsAMiss`, `TestSectionedSnapshotRefusesOutOfRangeOffsetsAsAMiss`, `TestSectionedRepeatedOpensRetainBoundedMappings`; timing gate not met (`docs/plans/sectioned-snapshot-prototype-2026-09-05.md`) |
| IDX-SNAP-V0-015 (proposed) | `encodePackSnapshot`, `readPackSnapshot`, `packTermView`, `cochangeSection`, `readSnapshotIndex`, `LoadContextSnapshotDeferred`, `LoadSnapshotDeferred`, `LoadEventSnapshotDeferred`, `overSnapshot`, `packBody` | `TestPackSnapshotDecodesEverySectionToTheGobValues`, `TestPackCochangeMatchesTheSpawnAfterAShallowCloneDeepens`, `TestPackCochangeMatchesTheSpawnAfterTheHistoryIsCutAtTheSameHead`, `TestPackTermViewFindMatchesTermPostings`, `TestPackSnapshotRefusesCorruptionTruncationAndOutOfRangeAsAMiss`, `TestPackSnapshotRefusesDuplicateOrOverlappingSections`, `TestPackCochangeServesTheSubjectHitAndFallsBackAfterAnAmend`, `TestPackSnapshotReadsWithoutChangingAByteOnTwentyTasks`, `TestPackRepeatedOpensRetainBoundedMappingsAndHistories`, `TestForgetPackHistoryReleasesRingSlot`, `TestPackContextLoadVerifiesOnlyTheBodiesAPacketReads`, `TestPackQueryAndEventLoadsVerifyABodyOnlyWhenItIsRead`, `TestPackDeferredBodyRefusalIsComputedOnceAcrossLaterLoads`, `TestPackQueryAndEventVerbsRereadARefusedBodyWithoutChangingAByte`; measurements in `docs/plans/corvint-pack-prototype-2026-09-05.md` |

| IDX-SNAP-V0-016 (proposed) | `buildWithBlobShards`, `writeBlobShards`, `BuildForSnapshot` | `TestBlobShardsReuseCommittedFactsAcrossChanges`, `TestBlobShardCorruptionRefusesAccelerationAndReadFallsBack`, `TestBlobShardRefusesFIFOWithoutBlocking`, `TestBlobShardPublicationPinsDirectoryAcrossSymlinkSwap`, `TestBlobShardRefusesForgedUnboundedFacts`, `TestBlobShardAggregateReadBudgetRefusesAcceleration`, `TestBlobShardsReadOnlyTwentyTaskMatrix`; actual-tree parity and idle-host timing falsified in the lane plan |

| IDX-SNAP-V0-017 (accepted within experimental pack) | `analyzerEngine`, `probeAnalyzerPack`, `WriteSnapshot`, `readSnapshotIndex` | `TestAnalyzerSchemaInputs`, `TestAnalyzerPackEngineIgnoresExecutableIdentity`, `TestAnalyzerPackProbeMissesACorruptBody`, `TestAnalyzerPacksStayBoundedAcrossTwelveTrees`, `TestAnalyzerPackEvictionReservesCurrentAndLeavesOtherFormats`, `TestPackSnapshotReadsWithoutChangingAByteOnTwentyTasks`; registered rebuild falsifier in `docs/plans/analyzer-schema-engine-2026-09-05.md` |

| IDX-SNAP-V0-019 (accepted, decision 0177) | `impact`/`feature` dispatch in `runContext`, `overSnapshot`, `deferredSnapshotIndex`, `snapshotIndex`, `standaloneImpactContext` | `TestImpactAndFeatureReadTheSnapshotWithoutChangingAByte`, `TestPackQueryAndEventVerbsRereadARefusedBodyWithoutChangingAByte` |
| IDX-SNAP-V0-020 (accepted, decision 0180) | `provePacket`, `runKernel`, `runWitness`, `snapshotOrBuild`, `overSnapshot` | `TestProveKernelAndWitnessReadTheSnapshotWithoutChangingAByte`, `TestPackQueryAndEventVerbsRereadARefusedBodyWithoutChangingAByte` |
| IDX-SNAP-V0-021 (accepted, decision 0182) | `impact` dispatch in `runContext`, `overSnapshot`, `snapshotIndex` | `TestImpactRangeAndWorkingTreeProfilesReadTheSnapshotWithoutChangingAByte` |
| IDX-SNAP-V0-022 (proposed) | `BuildForSnapshot`, `WriteSnapshot`, `LoadSnapshot`, `ProbeSnapshot` | `TestColdAndIncrementalSnapshotsAreByteIdentical` |
| IDX-SNAP-V0-023 (proposed) | `admittedEntries`, `LoadSnapshot`, `ProbeSnapshot`, `LoadEventSnapshot`, `evictSnapshots` | `TestSnapshotLifecycleHostileStatesHaveBoundedOutcomes` |
| IDX-SNAP-V0-024 | `displayPath`, `parseStatus`, `readTreeEntries`, `admittedEntries`, `parseHistory` | `TestNonUTF8TrackedPathIsExcludedAndTheRestIndexes`, `TestParseStatusNamesNonUTF8PathsInDisplayForm` |
| IDX-SNAP-V0-025 | `WriteSnapshot`, `liveWorktreeTrees`, `evictSnapshots`, `evictAnalyzerPacks`, `runIndex`, `evictedSnapshotsPayload` | `TestEvictSnapshotsNamesRemovalsAndKeepsLiveHeadTreesFirst`, `TestLiveWorktreeTreesNamesEveryLiveHead`, `TestIndexKeepsALinkedWorktreesSnapshotAndItsPromptReusesIt`, `TestIndexIfStaleReceiptsAndFreshSnapshotIsUntouched` |
| IDX-SNAP-V0-027 | `sweepLegacyStore`, `WriteSnapshot`, `runIndex`, `legacyLeftPayload` | `TestIndexWriteSweepsTheSupersededWorktreeStore`, `TestIndexWriteLeavesUnrecognisedLegacyEntries`, `TestIndexReceiptNamesTheSweptLegacyStore` |
| IDX-SNAP-V0-026 | `openingObservationWithTree`, `cancelledByListing`, `buildEvidenceFrom`, `buildStableFrom` | `TestBuildRefusesSourceCountBeforeStatusFinishesOrBlobsRead` (darwin/linux shim: 200,001-entry listing refused before the shimmed status finishes and with no `cat-file` spawn); `TestBuildReportsAnIndependentStatusFailureOverAnOverLimitListing` (shimmed status fails on its own before the over-limit listing: the status failure is reported, no `cat-file`); `TestBuildRefusesCarriedOpeningObservationErrors` (carried identity or status error refused, no index); `TestStandaloneStatusKeepsAnIndependentFailureCancelledDuringCleanup` (`contextCancellation`: a status failure classified when it happened keeps its own error when the scan context is cancelled during `StatusIn`'s deferred cleanup) |
| IDX-SNAP-V0-031 (accepted, decision 0517) | `startSnapshotRefresh`, `runSnapshotRefreshWorker` | `TestSnapshotRefreshCommitThenEvent` |
| IDX-SNAP-V0-032 (accepted, decision 0517) | `AcquireSnapshotRefreshLock`, `ValidateSnapshotRefreshLock` | `TestSnapshotRefreshSingleFlight`, `TestSnapshotRefreshStaleLock`, `TestSnapshotRefreshLockDirectorySwap`, `TestSnapshotRefreshLockConcurrentCreate` |
| IDX-SNAP-V0-033 (accepted, decision 0517) | `runSnapshotRefreshWorker` | `TestSnapshotRefreshSlowChild`, `TestSnapshotRefreshInterrupted`, `TestSnapshotRefreshEffectivePriority`, `TestSnapshotRefreshPriorityDenied` |
| IDX-SNAP-V0-034 (accepted, decision 0517) | `runIndex`, `WriteSnapshot` | `TestSnapshotRefreshInterrupted`, `TestEvictSnapshotsRemovesStaleTemporaries` |
| IDX-SNAP-V0-035 (accepted, decision 0517) | `snapshotRemediation` | `TestSnapshotRefreshDisabledAndFailures` |
| IDX-SNAP-V0-036 (accepted, decision 0517) | refresh regression matrix | `TestSnapshotRefreshCommitThenEvent`, `TestSnapshotRefreshSingleFlight`, `TestSnapshotRefreshInterrupted` |
| IDX-SNAP-V0-037 (standing auto-accept, V1-0328) | `BuildForSnapshot`, `snapshotReuseEnabled`, `loadReusePrior`, `priorFacts`, `takeUnchanged`, `attach`, `buildReusedVocabulary`, `mergeReusedPostings` | `TestSnapshotReuseMatchesColdBuild`, `TestSnapshotReuseRefusesUnverifiedPrior`, `TestSnapshotReuseRefusesRowsBoundToOtherBlobs`, `TestSnapshotReuseRecomputesIdentGraphForUnchangedReferrers`, `TestMergeReusedPostingsEqualsFullInversion`, `TestColdAndIncrementalSnapshotsAreByteIdentical` |
| IDX-SNAP-V0-515 (standing) | `previousSnapshotRefresh`, `writeSnapshotRefreshRecord`, `runSnapshotRefreshWorker`, `startSnapshotRefreshReporting`, `snapshotRemediation` | `TestSnapshotRefreshPreviousOutcome`, `TestSnapshotRefreshNoticeNamesPreviousOutcome`, `TestSnapshotRefreshInterrupted`; live run `docs/experiments/v1-0607-hook-refresh-qualification-20261030.json` |
| IDX-SNAP-V0-102 (standing auto-accept, V1-1640) | `noteSnapshotMissBuild`, `snapshotMissNoticeAfter`, `compileTaskContext` | `TestSnapshotMissNoticeNamesTheBuild` |
| IDX-SNAP-V0-103 (standing auto-accept, V1-1640) | `mayHoldGenericImport`, `skipGenericImportSpaces`, `sourceImports` | `TestGenericImportPrefilterLosesNoMatch`, `FuzzGenericImportPrefilter` |
| IDX-SNAP-V0-104 (standing auto-accept, V1-1640) | `withSnapshotMissNotice`, `timedMissBuild`, `repositoryQueryContext`, `authorityStartQueryContext` | `TestSnapshotMissNoticeNamesTheBuild` |
| IDX-SNAP-V0-105 (standing auto-accept, V1-2237) | `decodeSnapshotValue`, `decodeMappedSnapshotValue`, `guardMappedRead` | `TestMappedSnapshotDecodeMatchesStreamedDecode`, `TestMappedSnapshotDecodeFaultIsAMiss` |
| IDX-SNAP-V0-106 (standing auto-accept, V1-2290) | `snapshotWireOf`, `pairsOf`, `mapOf`, `wireValue`, `warmGobTypes` | `TestSnapshotBytesAreDeterministic`, `TestSnapshotWireRoundTripsMaps` |
| IDX-SNAP-V0-107 (standing auto-accept, V1-2289) | `script/analyzer-schema.sh` (`actual_output`, output pin in `pins`) | `TestAnalyzerSchemaOutput`, `script/analyzer-schema_test.sh` |
| IDX-SNAP-V0-108 (standing auto-accept, V1-1088) | `textSuffixes` | `TestAMAPV0025ChainBindingsFailClosed` |
| IDX-SNAP-V0-109 (standing auto-accept, GH #1663) | `textSuffixes`, `symbolExtractors`, `markerComments`, `SymbolRegionLanguage` | `TestTemplateAndRakeSuffixesAreIndexed_GH1663`, `TestAnalyzerSchemaOutput` |
| IDX-SNAP-V0-110 (standing auto-accept, V1-2716) | `textSuffixes` in `internal/contextindex/index.go`; documentation-kind switch in `internal/contextindex/taskcontext.go` | `TestNativeJvmAndStylesheetSuffixesAreIndexed_V2716`, `TestAnalyzerSchemaOutput` |
| IDX-SNAP-V0-111 (standing auto-accept, V1-3005) | `guardMapped`, `guardMappedRead` in `internal/contextindex/snapshot_decode.go`; `internal/contextindex/pack_reader.go`; `decodeSectionedSnapshot` in `internal/contextindex/snapshot_sectioned.go` | `TestPackMappedReadsRefuseATruncatedFile`, `TestSectionedMappedReadRefusesATruncatedFile` |
| IDX-SNAP-V0-112 (standing auto-accept, V1-2999) | `ProbeSnapshot`, `SnapshotFreshness` in `internal/contextindex/snapshot.go` | `TestSectionedProbeMissesACorruptBodyAndTheRebuildRepairsIt` |
| IDX-SNAP-V0-113 (standing auto-accept, V1-3015) | `newExtentSource`, `extent` in `internal/contextindex/symbol_extent.go`; `internal/contextindex/symbolwindows.go`; `internal/tasks/store/know_how_symbols.go` | `TestSymbolExtentsMatchTheJoinedReference`, `TestSymbolExtentsRetainOneDeclarationCopyPerSource`, `TestBuildSymbolWindowsSharesIdenticalWindows`, `TestKnowHowSymbolTableDigestsSharedDeclarationsOnce` |
| IDX-SNAP-V0-114 (standing auto-accept, V1-3016) | `joinLinesCapped`, `encodeLinesCapped` in `internal/contextindex/symbolwindows.go` | `TestJoinLinesCappedMatchesTruncatedJoin`, `TestSymbolContextTextAllocatesWithinTheCap` |
| IDX-SNAP-V0-115 (standing lane auto-accept, V1-3133) | `noteSnapshotMissBuild` in `cmd/corvint/index_snapshot.go`; `impactSnapshotMissNotice` in `internal/dogfoodflow/flow.go` | `TestSnapshotMissNoticeNamesTheBuild`, `TestIDXSNAPV0115TreeNoticeKeepsTypedAbstention` |
| IDX-SNAP-V0-955 (standing auto-accept, V1-3574) | `TestAnalyzerSchemaInputs` (no input pin), `script/analyzer-schema.sh` (`pins`, `check`, `bump`), `script/analyzer-inputs.sh` (IDX-SNAP-V0-890..892 govern) | `TestAnalyzerSchemaOutputDetectsAnalyzerMutation`, `TestAnalyzerSchemaOutput`, `script/analyzer-schema_test.sh` (two output-neutral `receipt.go` edits merge without conflict; changed inputs noted at check) |
