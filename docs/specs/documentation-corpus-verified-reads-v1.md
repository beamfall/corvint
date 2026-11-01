# Documentation corpus verified reads V1

Owner: Russell Lewis
Date: 2026-10-24
Intent status: proposed
Delivery status: experimental

Authoritative inputs: native ticket V1-2525 and GitHub issues
[#1598](https://github.com/beamfall/corvint/issues/1598),
[#1610](https://github.com/beamfall/corvint/issues/1610) and
[#1634](https://github.com/beamfall/corvint/issues/1634), and native ticket V1-2571. These
requirements were split unchanged from the [documentation corpus
contract](documentation-corpus-v1.md), whose Requirements section was over the 256-obligation OCM
limit (`maxObligations`). Each keeps its former DCP-V1 number under the DCR-V1 prefix, and the
parent's stub at each former section carries the crosswalk. The split changes no behavior, wire
format, error code or receipt; the parent's input and authority boundary, failure codes and
acceptance gates still govern them.

## Agent digest
- Claim: Corpus reads verify streamed, reused, parallel-scanned and delta members once, with stat-routed revision pins and bounded read-start freshness.
- Status: proposed; experimental; the blocks are implemented with focused tests and measured read timings.
- Exists: verified fast reads, streamed and reused verification, the parallel boundary scan of split members, a concurrent warm-read critical path, stat-routed revision pins, verified delta reads and read-start freshness, each with focused tests.
- Blocked on: owner acceptance and independent external utility qualification, as for the parent contract.
- Read next: Requirements; Failure modes; Traceability.

## User and current state

An agent reading a large corpus repeatedly needs each read to stay verified without re-hashing or
re-decoding the whole artifact. These blocks verify each member once, reuse that verification across
reads, scan split members in parallel, route revision pins by file stat and bound the work done at
read start. Former IDs: DCP-V1-NNN is DCR-V1-NNN for every ID listed here.

## Requirements

### Verified fast corpus reads (GitHub #1598)

Every CLI read used to rederive the whole artifact (`DCP-V1-011`) and then index each live
checkout in full, so one lookup cost more than a build (25 to 27 s and 1.7 to 2.0 GB on a
95 MB, 9,170-input corpus with four additional repositories). The owner's targets on an unchanged
checkout: a CLI read (`info`, `search`, `get`, `flow`, `coverage`, `locate`, `gaps`) under 1 s
cold and under 200 ms with warm reuse, with memory close to the artifact size; a corpus MCP first
tool call under 1 s; a warm MCP call under 100 ms. Status for `DCR-V1-410..419`: proposed,
experimental; GitHub #1598.

- `DCR-V1-410`: A read admits an artifact through the verified read, amending `DCP-V1-011`. It
  refuses unless the bytes are exactly the canonical encoding, match the recorded content digest,
  name the running builder (amended by `DCX-V1-471`: or an attested builder of the same schema,
  derivation version and engine), pair the artifact schema with the manifest schema, carry a valid
  manifest whose `manifest_sha256` and `profile_sha256` it recomputes, and every revision the
  manifest pins (repository, scopes, inputs, providers and additional repository pins) still
  resolves to a commit and tree in the checkout that owns it, the primary revision to the recorded
  `tree`. It then decodes the records without reading any pinned input body or rederiving any
  record. Every failure refuses the read (`corpus-refused`, or the native typed failure of an
  unresolvable revision); a read never falls back to serving unverified data. It applies to every
  `docs corpus` operation that opens `--artifact` except `validate`, `delta` and `carry-forward`,
  and to the corpus MCP tools. The content digest is integrity, not origin: a record edit whose
  digest was deliberately recomputed passes the verified read and is refused by the rederivation
  of `DCR-V1-413`. An equal builder label proves rebuild equivalence only when it is attested: a
  recorded source revision without uncommitted changes. Binaries built from different uncommitted
  changes share one `<revision>+dirty` label and binaries built without version-control metadata
  share `unrecorded`, and the artifact records no executable content identity that could tell
  them apart, so a running builder with such a label never takes the verified read: every read it
  makes runs the `DCP-V1-011` rederivation and reports `source-rederived`. Acceptance:
  `TestDCRV1410VerifiedReadSkipsRederivation`, `TestDCRV1410VerifiedReadRefusals`,
  `TestDCRV1410UnattestedBuilderRederives`, `TestDCRV1413CorpusReadsWithoutRederivation`.
- `DCR-V1-411`: Live freshness observes each declared checkout from Git's identity, its
  stat-cached status and one tree listing of blob modes and IDs, the checkouts in parallel. It
  reads and indexes no tracked file. Each checkout's identity is read, then its status while the
  identified tree is listed, then its identity again; the status is kept only when both identity
  reads agree, so the listing, identity and status describe one checkout state. When HEAD moved
  in between, the observation is repeated, at most three times, and then refused as a context
  build refuses (`repository HEAD changed`), never mixing two states. Git's index is the per-input stat stamp (path, size,
  modification time, inode and blob): status re-hashes only entries whose stat changed. States,
  limitations and the `DCP-V1-144`/`DCP-V1-280` reuse key are those the full live index gave.
  The CLI observes freshness from the decoded header members while the records are still being
  decoded and verified, and uses it only after the artifact is admitted. No sidecar or stamp file
  is written. Acceptance: `TestDCRV1411ObserveTreeMatchesBuildContext`,
  `TestDCRV1411ObserveTreeRetriesWhenHeadMoves`,
  `TestDCPV1280AdditionalRepositoryFreshnessReused`, `TestCorpusDeterministicRoundTrip`.
- `DCR-V1-412`: Symbol re-resolution (`DCP-V1-141`) builds the full live index of the primary
  checkout only when a cited Ruby or TypeScript/JavaScript symbol anchor's pinned blob changed or
  its path is dirty, once per overlay, and the corpus MCP server keeps it with the overlay. An
  index built from another checkout state than the overlay observed is refused, so the
  re-resolution is withheld and every cited symbol anchor is `unresolved` with the failure as a
  limitation (`DCP-V1-142`). Acceptance: `TestDCRV1412SymbolIndexBuiltOnlyWhenNeeded`,
  `TestSymbolAnchorReaderReceipt`.
- `DCR-V1-413`: `validate`, `--verify-rederive` on any read operation, and the corpus MCP
  `docs_validate` tool run the complete `DCP-V1-011` rederivation on every call. The `info` and
  `validate` result reports `validation` as `artifact-verified` or `source-rederived`. `build` is
  unchanged. Acceptance: `TestDCRV1410VerifiedReadSkipsRederivation`,
  `TestDCRV1413CorpusReadsWithoutRederivation`, `TestCorpusTypedCLIMCPEndToEnd`,
  `TestDocsCorpusOperationFlagMatrix`, `TestDocsCorpusOperationHelpPrintsFlags`.
- `DCR-V1-414`: The verified read finds the members of the input and the elements of every
  always-emitted record array in one scan, decodes the elements in parallel while the scan goes
  on, each compared with its own canonical encoding and separated by exactly one comma, and
  compares the rest with the canonical encoding of the artifact with those arrays empty: together
  exactly canonical byte equality. The content
  digest is hashed from the input bytes while they are decoded. It admits and refuses exactly what
  the closed decoder (`ParseArtifact`) admits and refuses. The rederivation compares its encoding
  with the input while encoding, without a second artifact-sized buffer. Acceptance:
  `TestDCRV1414ParallelDecodeMatchesClosedDecode`, `TestCorpusClosedInputsAndTamper`.
- `DCR-V1-415`: `--timings` on a read writes one line to stderr after the result:
  `corvint docs corpus timings: validation=V parse_ms=… verify_ms=… freshness_ms=… index_ms=…
  query_ms=… encode_ms=… total_ms=…`. Phases may overlap; times are host diagnostics, never a
  receipt field, and stdout is byte-identical with and without the flag. Acceptance:
  `TestDCRV1413CorpusReadsWithoutRederivation`.
- `DCR-V1-416`: The corpus MCP server's first tool call admits the artifact through the verified
  read, amending `DCP-V1-143`; `load` stays `first-call`, startup and the memo rules are
  unchanged. Acceptance: `TestDCRV1413CorpusReadsWithoutRederivation`,
  `TestDCRV1416CorpusMCPFirstAndWarmCalls`, `TestDCPV1143LazyLoadAndMemo`.
- `DCR-V1-417`: A warm corpus MCP call neither reads nor hashes the configured artifact while its
  file keeps the stat stamp (mode, size, inode, change and modification times) captured before
  the last read whose digest matched. A file changed within the `DCP-V1-314` racy window, or one
  that cannot be described, has no stamp and is read; a rewrite changes the stamp and is refused
  as before. The file is described through the artifact reader's own no-follow walk of every
  path component, so a symlink anywhere in the path, the leaf included, leaves no stamp: the call
  reads the file and is refused exactly as a fresh read is. Concurrent calls that each find no
  matching stamp share one artifact read and digest check: a call that finds a read in flight
  waits for it and takes its bytes or its error, so they hold one artifact copy and a changed
  artifact is refused for all of them as before (V1-3028). Acceptance:
  `TestDCRV1417WarmCallsSkipUnchangedArtifactRead`, `TestDCRV1417WarmStampRefusesParentSymlink`,
  `TestDCRV1417ConcurrentColdCallsShareOneRead`.
- `DCR-V1-418`: Acceptance at scale, on a generated fixture with a 4,000-page native scope, 6,000
  source files outside it and four additional repositories of 1,500 pinned files each (10,001
  inputs, a 67 MB artifact): a verified read with live freshness, and a corpus MCP first call, are
  each at least 3x faster than the rederivation they replace and answer identically. Wall time and
  peak memory before and after are recorded in the build log and pull request. Acceptance:
  `TestDCRV1418LargeFixtureVerifiedReadIsFast`, `TestDCRV1416CorpusMCPFirstAndWarmCalls`,
  `BenchmarkDCRV1418VerifiedRead`.
- `DCR-V1-419`: A read derives the same query index, receipts and refusals with less wall time.
  The index's record maps and term maps are built concurrently and its claim and subject terms
  are tokenized in blocks on worker goroutines, each block in record order and the blocks joined
  in order; its posting and unique-key-byte totals are taken from the finished maps. That
  derivation answers only when no reference refusal is possible (no empty or duplicate identity,
  no key over 65,536 bytes, no path at its fanout bound, both totals within their `DCP-V1-193`
  bounds); otherwise the serial reference derivation runs, so every refusal keeps its code,
  message and observed count (`DCP-V1-194`). Unindexed `search` tokenizes subjects in blocks the
  same way. The CLI derives the index while the live freshness observation of `DCR-V1-411`
  finishes, and still reports an index refusal before any freshness failure (`DCP-V1-195`). A
  bounded input file is read into one buffer sized from its opened size. No artifact or manifest
  format changes, nothing is persisted, and search ranking is unchanged. Acceptance:
  `TestDCRV1419FastIndexMatchesReference`, `TestDCRV1419FastIndexDeclinesEveryRefusal`,
  `TestDCRV1419BoundaryScanFindsStringContent`, `TestDCRV1413CorpusReadsWithoutRederivation`;
  CLI timings before and after in https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-23-corpus-verified-fast-reads.md.

### Streamed reads and verified reuse on the remaining paths (GitHub #1598 residual)

After `DCR-V1-410..419` the measured gaps on the `DCR-V1-418` fixture were: warm CLI `flow` and
`gaps` above 200 ms, the decode and canonical check repeated in every new process, peak memory
3 to 4 times the artifact, and several readers still rederiving. Status for `DCR-V1-430..434`:
proposed, experimental; GitHub #1598.

- `DCR-V1-430`: Residual acceptance. On the `DCR-V1-418` fixture the seven CLI reads (`info`,
  `search`, `get`, `flow`, `coverage`, `locate`, `gaps`) are measured before and after, each
  binary on an artifact it built itself, alternating binaries, at least three runs per read, with
  the median, range, peak resident memory and host load recorded; the corpus MCP first and warm
  calls are re-measured at the same scale. The memory bound is peak resident memory of at most
  3.5 times the artifact size for each of the seven reads on that fixture. The build log and the
  pull request record which owner targets the measurements meet and which they miss; a missed
  target stays named, never rounded into a pass. Acceptance:
  `TestDCRV1416CorpusMCPFirstAndWarmCalls` with `CORVINT_READ_FIXTURE_LARGE=1`; measurements in
  https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-23-corpus-read-residual.md.
- `DCR-V1-431`: The CLI's verified read, and a native read joined with `--corpus=FILE`, stream the
  artifact file instead of holding it whole. The file is opened as `ReadCorpusFile` opens it (the
  local path rule, the no-follow walk, a regular file whose identity holds from walk to open, and
  the 128 MiB bound checked on the opened size), read ahead in 1 MiB blocks on its own goroutine,
  hashed in read order and scanned window by window; each batch of record-array elements is
  decoded and compared with its canonical encoding from the window it was found in, the bytes
  outside every record array are copied once for the skeleton comparison, and a block buffer is
  reused once its window, its hash and every batch decoding from it are done. It admits and
  refuses exactly what `OpenRead` of the same bytes admits and refuses, with the same records,
  digest and refusal text, whichever block size splits the bytes; the digest holds back the last
  4 KiB, so every artifact the verified read can admit is hashed as `DCR-V1-414` hashes it. A
  read error is the read's `corpus-input-unavailable` refusal, never a refusal of the bytes read
  before it. A rederived read reads the whole file as before. The corpus MCP server keeps reading
  the artifact bytes it hashes for its stamp (`DCR-V1-417`) and decodes them through the same
  scanner as one window. Input that ends exactly where a window ends, inside a value or right
  after a scalar, refuses as invalid input and never panics: a refill moves the window even when
  it reads nothing, so the scan resumes from the moved position. Every exit of a CLI query read,
  a refusal of its query index or of its arguments included, cancels and joins the live freshness
  observation it started before returning, so none outlives the read or records its timing after
  the `--timings` summary. Acceptance: `TestDCRV1431StreamedDecodeMatchesInMemory`,
  `TestDCRV1431StreamedReadReusesBlocks`, `TestDCRV1431StreamedLongValue`,
  `TestDCRV1431OpenReadFile`, `TestDCRV1431StreamedTruncationAtWindowEnd`,
  `TestDCRV1431CorpusRefusalJoinsFreshness`, `TestDCRV1414ParallelDecodeMatchesClosedDecode`, `TestDCRV1413CorpusReadsWithoutRederivation`.
- `DCR-V1-432`: Every remaining reader of a retained artifact either takes the verified read of
  `DCR-V1-410` or rederives for a recorded reason (the `delta` exception of `DCR-V1-410` is
  amended by `DCR-V1-460`, not here):
  - a native read joined with `--corpus=FILE` (`query`, `context`, `impact`, `affected`,
    `test-validity`, `work` and `cem` reads) takes the streamed verified read: it attaches a
    `generated-documentation` receipt with no task authority, as a `docs corpus` read does;
  - the current side of `docs corpus delta --artifact` is superseded by `DCR-V1-460..464` (GitHub
    #1634), which deliver its verified read, `--verify-rederive`, `--timings` and the input blob
    binding check, with a receipt byte-identical on the verified and the rederived path; this
    requirement adds no delta behavior, flag or limitation of its own;
  - `carry-forward` keeps rederiving its current side: it emits provider records for the next
    build, so the records it copies forward must be the ones the pinned inputs give, not ones a
    rehashed edit could supply;
  - `docs flows generate` (`internal/flowdocs`) keeps rederiving the corpus it joins: the joined
    corpus is written into the generation manifest that `check` and `finalize` treat as
    source-validated;
  - flow coverage (`internal/flowcoverage`) keeps rederiving a `corpus` inventory: the inventory
    decides which flows are documented and the report carries a pass or fail verdict, so a
    rehashed record edit must not move that verdict;
  - the indexed companion build (`internal/corpusindex.Build`) keeps rederiving: its artifact
    declares `source-rederived-at-producer`;
  - republish (`internal/corpusrepublish`) keeps rederiving the previous artifact: its
    `VerifiedCorpus` token may only be minted by the complete source rederivation of `DCP-V1-040`.

  The content digest proves integrity, not origin (`DCR-V1-410`), so only readers whose output
  stays a read take the verified read; producers whose output is consumed as source-validated keep
  the rederivation. Acceptance: `TestDCRV1432JoinedCorpusReusesVerifiedArtifact`; the delta
  bullet's acceptance is that of `DCR-V1-460..464`.
- `DCR-V1-433`: A read persists and consumes no read index, posting set, identity map, sidecar or
  cache, and writes nothing beside the artifact. The query index is derived from the admitted
  records on every admission, as `DCP-V1-193..197` require. A persisted posting set could be
  trusted only after deriving it again (whether a term's postings are complete cannot be checked
  without tokenizing every record), so it would either be derived state deciding which records
  answer or save nothing; `build` writes only the artifact, to stdout. Acceptance:
  `TestDCRV1432JoinedCorpusReusesVerifiedArtifact`,
  `TestDCRV1413CorpusReadsWithoutRederivation`.
- `DCR-V1-434`: An indexed CLI read (`IndexedOperation`) derives the claim terms of its query
  index on the decode workers: once every claim of a batch is decoded and verified, the batch's
  claim-term postings, in claim order, and the text terms of each subject its claims are about
  are derived there. The index derivation of `DCR-V1-419` takes them when the batches cover every
  claim exactly, in order, and otherwise tokenizes as before; they are never used unless the
  artifact is admitted, and are dropped once taken. Exactly one index derivation of an artifact
  takes them, under a lock; a concurrent derivation of the same artifact tokenizes instead, and no
  query changes the shared artifact. The index, its posting and key-byte totals,
  its refusals (`DCP-V1-194`, `DCP-V1-195`) and every receipt are unchanged. Acceptance:
  `TestDCRV1434DecodeTimeClaimTermsMatchIndex`, `TestDCRV1434ConcurrentQueriesShareClaimTerms`,
  `TestDCRV1419FastIndexMatchesReference`,
  `TestDCRV1419FastIndexDeclinesEveryRefusal`, `TestDCRV1413CorpusReadsWithoutRederivation`.

### Parallel boundary scan of split members (V1-2571)

A warm CLI read decodes the artifact on parallel workers (`DCR-V1-414`), but one goroutine still
scans every split element's bounds byte by byte before it can hand the element to a worker, and on
the `DCR-V1-418` fixture that scan was the longest serial step of the decode. These requirements
split the scan itself across segments without changing what any decode returns. Status for
`DCR-V1-551..553`: proposed, experimental; V1-2571, part of GitHub #1598.

- `DCR-V1-551`: Before scanning a split member's next element sequentially, the decode may cut the
  next window region of at most 4 MiB into up to `GOMAXPROCS` segments of at least 32 KiB and
  scan them in parallel. The separator is a comma followed by the first element's bytes up to the
  colon after its first member name; it cannot occur inside a canonical JSON string, where every
  quote is escaped. The first segment starts at the scan position; each other segment guesses
  that its first element starts just after its first separator, and every segment scans whole
  elements with the sequential scan's own `scanValue`, taking an element only when one comma
  follows it directly. Runs are joined in order only while each run starts where its
  predecessor's next element does, so every element handed over is exactly one the sequential
  scan finds from the same position, in the same order; a wrong guess, an anomaly, a truncation
  or the end of the window only ends the join early, and the sequential scan then resumes after
  the last element handed over. A region is never scanned in parallel twice. Failure modes: a
  separator inside a string, an escape or backslash run cut by a segment bound, a nested array of
  objects that start with the same member name, an element that spans several segments, an
  element without a member name, corrupt, inserted or cut bytes, and a region shorter than two
  segments each fall back to (or stay a prefix of) the sequential scan. Acceptance:
  `TestDCRV1551ParallelElementsMatchScan`, `TestDCRV1551ParallelElementsCorrupt`.
- `DCR-V1-552`: With the parallel scan, a decode returns the same records, digest tail and refusal,
  with the same message, as with the sequential scan, in memory and streamed at every window size,
  so the refusal order of `DCR-V1-414` and `DCR-V1-419` (undecodable, then noncanonical, then the
  output bound) is unchanged. Acceptance: `TestDCRV1552ParallelScanMatchesSequential`, over the
  committed fixture, claims holding every adversarial text (escaped quotes, backslash runs of 31
  to 131 bytes, the separator, brackets, a claim longer than many segments, non-ASCII bytes),
  every refusal shape of the streamed-read tests, and the claims member truncated or carrying a
  quote, backslash, bracket, brace or comma at up to 1500 offsets; the existing
  `DCR-V1-410..419` and `DCR-V1-430..434` tests pass unchanged.
- `DCR-V1-553`: The parallel scan adds no on-disk state, cache, index or format change, and keeps
  the warm CLI read within the `DCR-V1-430` peak-RSS bound of 234 MB. Its latency is measured as
  interleaved warm `docs flow` and `docs gaps` reads of the `DCR-V1-418` fixture against the base
  binary, n=20 each, p50 and p99, with the host load average recorded, on linux/amd64 and darwin.
  Acceptance: the measurement in
  `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-24-corpus-parallel-boundary-scan.md`.

Non-goals: no change to the decode workers' JSON decoding or the canonical re-encoding check, no
SIMD or assembly, no persisted boundary index or other derived state (NEEDS-OWNER, V1-2572), and no
change to `DerivationVersion`. Rollback: make `boundaryScan` default to one segment, or remove
`parallelElements` and its call; the sequential scan is unchanged beneath it. No stored state needs
migration.

### Read hot spots: scope matching and stat-routed revision pins (GitHub #1610)

A CPU profile of one read of a 9,170-input corpus with one single-file scope per input put about
half of the read in two places: every scope check compared each scope with each path, building
`scope + "/"` on every comparison, and every pin of a revision opened, read and hashed each
tracked file of each checkout. Status for `DCR-V1-450..453`: proposed, experimental; GitHub #1610.

- `DCR-V1-450`: Scope membership allocates nothing, and each scope check finds the scopes holding
  a path by walking the path's ancestors through a per-revision scope set instead of comparing
  the path with every scope: the manifest's input admission, the compiler's scope check (an
  undeclared input, then an unsupported entry, then an empty scope, scopes in manifest order),
  live freshness's new-input, new-working-tree-input and unsupported-entry reasons (once per
  holding scope) and the
  inventory's nested-scope refusal (the byte-least containing scope, then the byte-least
  contained one). The walk visits exactly the scopes the pairwise comparison admitted, so every
  artifact, receipt, reason and refusal is byte-identical. Acceptance:
  `TestDCRV1450ScopeMatchingMatchesPairwise`, `TestDCRV1450NestedScopesKeepRefusalsAndReasons`,
  `TestDCRV1450NewLiveFileNotedPerHoldingScope`.
- `DCR-V1-451`: The read fixture of `DCR-V1-418` has a many-scopes shape, one single-file scope
  per native page and per additional-repository file, and its verified read answers exactly as
  its rederivation, fresh on the unchanged checkout. Benchmarks measure the verified and the
  rederived read over it, and CPU profiles before and after over the 10,000-scope persisted
  fixture (`CORVINT_READ_FIXTURE_SCOPES=many`) are recorded in the build log and pull request.
  Acceptance: `TestDCRV1451ManyScopesReadMatchesRederivation`,
  `BenchmarkDCRV1451ManyScopesVerifiedRead`, `BenchmarkDCRV1451ManyScopesRederivedRead`.
- `DCR-V1-452`: A corpus revision pin (`BuildRevisionContext`) reads the checkout's Git index
  (located by `git rev-parse --git-path index`, opened without following a symlink or blocking,
  at most 256 MiB, version 2, 3 or 4, trailing checksum verified unless it is Git's all-zero
  `index.skipHash` trailer). An eligible entry whose index
  record is stage 0, carries no assume-valid, skip-worktree or intent-to-add flag, and whose
  worktree file's lstat matches the record's size, modification and change times, inode, owner
  and executable bit, with the recorded modification time in a whole second strictly before the
  index file's modification second (Git's racy-clean rule with a one-second racy window, so no
  nanosecond timestamp is assumed), is vouched: it is pinned from the tree entry's committed blob in one
  batched object read and its worktree file is never opened. Every other eligible entry is
  opened and hashed as before and, when its bytes do not hash to the tree entry's blob, pinned
  from that blob. Worktree bytes are therefore used only when they hash to the pinned blob, and
  the pinned bytes, exclusions and records are byte-identical to an unrouted pin: vouching
  selects only where an entry's tree blob is read from, never what is pinned. Other index
  builds (`Build`, `BuildContext`, `BuildEval`) are unchanged and still read no committed blob
  for a clean repository. Acceptance: `TestDCRV1452GitIndexVouchesUnchangedEntries`,
  `TestDCRV1452RacyEntryIsNotVouched`, `TestDCRV1452IndexVersionsAndFlags`,
  `TestDCRV1453RoutedPinMatchesParityRepository`.
- `DCR-V1-453`: Routing fails closed to the hashed worktree pass: a missing, unreadable,
  symlinked, oversized, truncated or corrupt (checksum mismatch) index, an unsupported version,
  an entry with an extended flag other than skip-worktree or intent-to-add (Git refuses such an
  index), a decoded pathname longer than 4,096 bytes, decoded pathnames together exceeding eight
  times the index size (checked before each name is allocated, so version 4 prefix compression
  cannot expand without bound), a split index
  (`link`) or an unknown required extension vouches for nothing; a stat mismatch, a racy entry or
  a platform without lstat inode and change times leaves that entry unvouched; and a failed or
  incomplete blob batch returns its entries to the hashed worktree pass. No unverified worktree
  file is ever pinned. Acceptance: `TestDCRV1453UnusableIndexVouchesNothing` (including the
  unknown-extended-flag, long-pathname and expanded-pathnames cases),
  `TestDCRV1453IndexPathnameBounds`, `FuzzParseGitIndex`,
  `TestDCRV1453FailedBlobBatchFallsBackToWorktree`,
  `TestDCRV1453RoutedPinMatchesParityRepository`.

### Verified delta reads (GitHub #1634)

`docs corpus delta --artifact` still reopened and rederived the current artifact after the
verified read of `DCR-V1-410` landed: on a 95 MB, 9,170-input corpus the rederivation was 3.5 s of a
15.7 s jsonl stream while `info` took about 1 s, and `delta --timings` was refused. Status for
`DCR-V1-460..464`: proposed, experimental; GitHub #1634.

- `DCR-V1-460`: `delta` admits a retained `--artifact` current side through the verified read,
  amending `DCX-V1-360` and the `delta` exception of `DCR-V1-410`: it runs no build. With
  `--verify-rederive` (`DCR-V1-413`), or a running builder that is not attested, it runs the
  complete `DCP-V1-011` rederivation instead. Every refusal of the verified read or the
  rederivation refuses the delta (`corpus-refused`, or the native typed failure), with no output;
  a delta never falls back to serving an artifact the read refused. The one exception is the
  unchanged `DCX-V1-425` path for an unavailable additional checkout, which checks canonical
  bytes, the digest, a valid manifest, the primary identity and the available inputs' blobs, not
  the builder or the manifest and profile digests. Whether it runs depends only on that checkout,
  observed from the header members of a valid manifest while the records are decoded, never on
  whether the verified read succeeded; with `--verify-rederive` the manifest is validated before
  any checkout is examined. The receipt and the jsonl stream are byte-identical on the verified
  and the rederived path. `carry-forward` keeps the rederivation, and a `--manifest` current side is
  still built (`--verify-rederive` changes nothing there). Acceptance:
  `TestDCRV1460DeltaVerifiedReadMatchesRederived`, `TestDCRV1460DeltaVerifiedReadRefusesClosed`,
  `TestDCRV1460DeltaVerifiedReadUnavailableCheckout`, `TestDCRV1460DeltaCLIVerifiedRead`.
- `DCR-V1-461`: `--previous` of `delta` and `carry-forward` is admitted with the verified read's
  decode (`DCR-V1-414`): canonical bytes and the recorded content digest, exactly what
  `ParseArtifact` admits and refuses. It is not checked against the running builder or the pinned
  revisions and is never rederived, `--verify-rederive` included, because the worklist must not
  depend on which builder built the previous corpus (`DCX-V1-424`). It is decoded while the current
  side is opened; a previous-side refusal is reported before a current-side one, as before. The
  parallel decode reads the artifact schema before it applies the output bound whether or not a
  header callback is given, so a schema `/2` artifact over 4 MiB is admitted as `ParseArtifact`
  admits it. Acceptance: `TestDCRV1461PreviousOverDefaultBound`,
  `TestDCRV1460DeltaVerifiedReadRefusesClosed`, `TestDCRV1460DeltaVerifiedReadMatchesRederived`.
- `DCR-V1-462`: `delta` accepts `--timings` and `--verify-rederive` with the `info` help text;
  `carry-forward` accepts neither. `--timings` writes the `DCR-V1-415` line to stderr after the
  result, with `compare_ms=` (classification and worklist) before `total_ms=`: `parse_ms` holds
  both file reads and decodes, `verify_ms` the current side's verification or rederivation (or
  the `--manifest` build), and `encode_ms` the receipt or stream encoding. The previous side is
  decoded while the current side opens, so `parse_ms` may exceed `total_ms`. `validation` is
  `artifact-verified`, `source-rederived`, `checkout-unavailable` (`DCX-V1-425`) or
  `manifest-built`. stdout is byte-identical with and without the flag. Acceptance:
  `TestDCRV1460DeltaCLIVerifiedRead`, `TestDocsCorpusOperationFlagMatrix`,
  `TestDocsCorpusOperationHelpPrintsFlags`.
- `DCR-V1-463`: On the `PRB-V0-021` full-scale fixture (10,000 inputs, 96,350 records, a 97 MB
  artifact), the unchanged-pair jsonl delta and the changed-pair paged delta each take at most a
  third of the median wall time of the rederiving delta they replace, and the `PRB-V0-050` budget fails a delta that
  rederives. Before and after wall time and peak memory are recorded in
  https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-23-corpus-delta-verified-read.md. Acceptance:
  `TestPRBV0050UnchangedDeltaBudget`; measured in that build-log entry. That budget is wall-time
  evidence, not a deterministic proof: the deterministic guard that the default delta rederives
  nothing is the build counter of `TestDCRV1460DeltaCLIVerifiedRead`.
- `DCR-V1-464`: Before anything is classified, the verified current side of `delta` checks, with
  every declared checkout available, that each input names the regular-file blob its path holds in
  the tree of its pinned revision, in the checkout that holds the declared repository identity at
  that revision, with the same batched per-revision path walk as the `DCX-V1-425` path. The
  verified read alone checks only the pins and the recorded digests, and the classification trusts
  the recorded input blobs, so a current artifact whose changed input names the previous blob, with
  its manifest and content digests recomputed, would otherwise be classified `carry-forward`. A
  mismatch refuses the delta with no output (`does not name the blob its path holds`). The recorded
  SHA-256 of an input is not rehashed from its blob; a blob-consistent artifact whose recorded
  SHA-256 alone differs is caught only by `--verify-rederive`. Acceptance:
  `TestDCRV1464DeltaVerifiedReadChecksInputBindings`.

### Read-start freshness and decode allocation (GitHub #1598 residual)

After `DCR-V1-430..434` the warm CLI `flow` and `gaps` reads on the `DCR-V1-418` fixture still
took 205 to 248 ms median at host load 11 to 24, above the 200 ms owner target. The read's
freshness observation of the primary checkout only started when the artifact header had been
decoded, about 16 ms in, and then ran 150 to 210 ms, so it ended after the decode on most runs and
set the read's wall time. The decode's canonical check copied every record element before encoding
it, and the index join regrew every claim term's postings. Status for `DCR-V1-465..468`: proposed,
experimental; GitHub #1598 (ticket V1-2477).

- `DCR-V1-465`: A CLI query read (`corpusQueryOperation`) starts the live tree observation of its
  primary checkout, the read's own `--root`, when the read starts, before the artifact is opened
  (`WithPrimaryObservation`). The primary checkout needs nothing the artifact records, so the
  observation is the same `contextindex.ObserveTree` the freshness of `DCR-V1-411` runs; additional
  checkouts are still observed from the header's repository list. The freshness overlay, receipt
  and refusals are those of a freshness that observes the checkout itself; the checkout state is
  observed at the read's start instead of at its header, which is no earlier than the artifact
  decode it is paired with. `freshness_ms` still counts from the header callback, so it may
  understate the observation's wall time. Acceptance:
  `TestDCRV1465PrimaryObservationMatchesFreshRead`, `TestDCRV1413CorpusReadsWithoutRederivation`;
  CLI timings in https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-23-corpus-read-start-freshness.md.
- `DCR-V1-466`: The started observation answers at most one freshness, and only one of the same
  root; a later freshness under the same read, or one of another root, observes the checkout
  again, so no freshness reuses an observation older than the read. Acceptance:
  `TestDCRV1466PrimaryObservationUsedOnce`.
- `DCR-V1-467`: Every exit of the read, a refusal before or after the header included, cancels the
  started observation and joins it, after the header-started freshness of `DCR-V1-431` is joined,
  so no observation or Git process outlives the read or writes after the `--timings` summary.
  A freshness whose own context ends before the started observation does, such as one under a
  shorter derived deadline, stops waiting and fails with its context's error as a new
  observation would, while the read's cancel and join still end the observation. Acceptance:
  `TestDCRV1467CancelledPrimaryObservationJoins`,
  `TestDCRV1467ShorterDeadlineStopsWaitingForPrimary`,
  `TestDCRV1467CorpusPreHeaderRefusalJoinsPrimary`, `TestDCRV1431CorpusRefusalJoinsFreshness`.
- `DCR-V1-468`: The decode's canonical check (`DCR-V1-414`) encodes each decoded record element
  through its address instead of a boxed copy; the bytes compared, and every admission and refusal,
  are unchanged. Each batch's term map is sized from its claim count, and decode-time claim
  terms (`DCR-V1-434`) are joined with each term's postings
  allocated once at their final length, and a term only one batch holds keeps that batch's list;
  the index equals the reference derivation. Acceptance:
  `TestDCRV1431StreamedDecodeMatchesInMemory`, `TestDCRV1434DecodeTimeClaimTermsMatchIndex`,
  `TestDCRV1419FastIndexMatchesReference`.

### Warm read critical path (V1-2477, V1-2571)

After `DCR-V1-465..468` and `DCR-V1-551..553`, profiles of warm `docs flow` and `docs gaps` reads
of the `DCR-V1-418` fixture still showed serial steps on the read's critical path: the header
members decoded inline in the scan, the decoded chunks copied into their members one after
another before the skeleton was encoded, the query index's identity, path and link maps built on
one goroutine with its key accounting after them, a gaps query that mapped every claim to its
subject, and a recorded-identity check that validated the manifest a second time. These
requirements move or remove that work without changing any read's result. Status for
`DCR-V1-554..558`: proposed, experimental; V1-2477 and V1-2571, part of GitHub #1598.

- `DCR-V1-554`: Each header member the scan finds is decoded on its own goroutine while the scan
  goes on to the records, its window held out of the block pool until it is decoded. The header
  callback of `DCR-V1-411` starts only once every header member is decoded, and never when one is
  undecodable; after the scan the decode waits for the header members, and an undecodable one
  refuses as invalid, before the output bound and every canonical check, exactly as the inline
  decode did. Acceptance: `TestDCRV1554UndecodableHeaderMemberRefuses`,
  `TestDCRV1431StreamedDecodeMatchesInMemory`, `TestDCRV1552ParallelScanMatchesSequential`.
- `DCR-V1-555`: After the workers finish, the skeleton (the artifact with every split member
  empty) is encoded and compared on its own goroutine while the decoded chunks are copied into
  their members, the chunk runs of each member copied on up to `GOMAXPROCS` goroutines. The
  records, the skeleton bytes compared and the refusal order of `DCR-V1-414` are unchanged.
  Acceptance: `TestDCRV1431StreamedDecodeMatchesInMemory`,
  `TestDCRV1414ParallelDecodeMatchesClosedDecode`, the `DCR-V1-410..419` and `DCR-V1-430..434`
  tests unchanged, under the race detector.
- `DCR-V1-556`: The fast query index of `DCR-V1-419` builds its identity, path and link maps on
  three goroutines, visiting records in the reference order, sorts path references on the path
  goroutine, merges subject terms concurrently with claim terms, and computes the posting total
  and unique key bytes with one goroutine per map: a key counts once, in the first map in the
  reference charging order that holds it, unless it is an identity. The index equals the reference
  derivation whenever it is returned, and every case the reference refuses still declines to it.
  Acceptance: `TestDCRV1419FastIndexMatchesReference`,
  `TestDCRV1419FastIndexDeclinesEveryRefusal`, `TestDCRV1434DecodeTimeClaimTermsMatchIndex`.
- `DCR-V1-557`: The claim-audit gaps of a query map only the claims a current incorrect verdict
  judges to their subjects; the last claim with a judged ID still names the subject, so the gaps
  equal those of mapping every claim. Acceptance: `TestDCRV1557ClaimAuditGapsMatchReference`.
- `DCR-V1-558`: The recorded-identity check of `DCR-V1-410` does not validate the manifest again
  after the header callback validated it; validation only reads the manifest, so every check and
  refusal, and their order, are unchanged, and the builder, schema pair, manifest and profile
  digests and every pinned revision are still verified on every read. Acceptance:
  `TestDCRV1410VerifiedReadRefusals`, `TestDCRV1414ParallelDecodeMatchesClosedDecode`.

The warm reads are measured as interleaved `docs flow` and `docs gaps` reads of the
`DCR-V1-418` fixture against the base binary, after an idle check of the host, with peak RSS
against the `DCR-V1-430` bound of 234 MB, in
`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-25-corpus-warm-read.md`. Non-goals: no persisted index, cache or stamp
(`DCR-V1-433`), no skipped or weakened check, no change to the JSON decoding, the canonical
re-encoding or `DerivationVersion`. Rollback: revert the implementing change; no stored state
needs migration.

### Block pre-scan of split members (V1-2477)

After `DCR-V1-554..558`, a timeline of the warm `docs flow` and `docs gaps` reads showed the
decode goroutine's longest step was still the boundary scan of each 1 MiB window: the parallel
scan of `DCR-V1-551` runs on the goroutine that dispatches the elements, and under the decode
workers' saturation of every processor its fan-out waited more on scheduling than it scanned, so
its segment size made no measured difference. This requirement moves the scan of each block off
that goroutine. Status for `DCR-V1-559`: proposed, experimental under the standing lane
auto-accept; V1-2477, part of GitHub #1598.

- `DCR-V1-559`: A streamed decode whose boundary scan has more than one segment pre-scans each
  block the reader reads ahead on up to four scanner goroutines, under the separator of
  `DCR-V1-551` that the decode publishes when it opens a split member and withdraws when it closes
  one. A block's pre-scan finds its element runs as a `DCR-V1-551` segment does: a run starts
  where the block begins with the separator's element bytes or just after a separator, and holds
  the whole elements from there, scanned with the sequential scan's own `scanValue`, that one
  comma follows inside the block; a value that runs past the block ends the pre-scan. The block
  is held out of the pool until its pre-scan is done, and the window takes the block only once it
  is done. The decode joins a pre-scanned run only when the run starts exactly at the scan
  position, which is the first byte of an element, so every element handed over is exactly one
  the sequential scan finds from that position, in the same order, and the sequential scan then
  resumes after the last element handed over; a position with no such run falls back to the
  `DCR-V1-551` parallel scan and then to the sequential scan. The pre-scan reads no byte outside
  its block, adds no on-disk state, cache, index or format change, and holds at most the blocks
  the reader already reads ahead, so the warm CLI read stays within the `DCR-V1-430` peak-RSS
  bound of 234 MB. Failure modes: a block read ahead before the separator is published or after
  it is withdrawn, a separator of another split member, a wrong guess, a separator inside a
  string, an element that spans blocks, a window that is not pooled, the output bound
  (`DCR-V1-431`) and a sequential boundary scan each leave the decode to the `DCR-V1-551` or
  sequential scan, which returns the same records, digest tail and refusal. Acceptance:
  `TestDCRV1559BlockScanMatchesScan`, and `TestDCRV1552ParallelScanMatchesSequential`, whose
  streamed decodes with more than one segment run the pre-scan at every window size against the
  sequential reference, under the race detector; the `DCR-V1-410..419`, `DCR-V1-430..434` and
  `DCR-V1-551..558` tests pass unchanged. Latency and RSS are measured as in `DCR-V1-553`, in
  `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-27-corpus-block-prescan.md`.

Non-goals: no change to the sequential scan, the decode workers, the canonical re-encoding or
`DerivationVersion`; no persisted boundary index or other derived state (`DCR-V1-433`, V1-2572);
no SIMD or assembly. Rollback: do not create the block scanner in `decodeArtifactBlocksScan`, or
revert the implementing change; the `DCR-V1-551` scan and the sequential scan are unchanged
beneath it, and no stored state needs migration.

### One commit batch per checkout for pinned revisions (V1-2657)

The `DCR-V1-410` recorded-identity check resolved a checkout's pinned revisions one at a time,
each `CommitTree` starting its own `git cat-file --batch -z` process, so a checkout with six pinned
revisions started six. Status for `DCR-V1-975`: experimental under the standing lane auto-accept;
V1-2657.

- `DCR-V1-975`: The pinned-revision verification resolves every pinned revision of one checkout
  through a single `git cat-file --batch -z` process (`CommitTrees`): two requests per revision in
  sorted order, the commit peel and the tree peel, each record self-hashed and each present commit
  required to name the tree its tree peel returns, exactly the checks `CommitTree` makes for one
  revision. Checkouts are still verified in parallel, one process each. A revision that is not a
  full object ID refuses before Git starts; a missing or mismatched revision refuses with the
  message `CommitTree` gives, the first such revision in sorted order deciding; the primary
  revision must still resolve to the recorded tree, and a primary revision sorted before a refused
  one still refuses as a recorded tree difference, as the per-revision order did. Non-goals: no change to `gitauth.Open`, to the
  checkout grouping or to any refusal. Failure mode: one malformed record fails the whole batch,
  as it failed the one revision it belonged to. Acceptance:
  `TestDCRV1975PinnedRevisionsOneBatchPerCheckout` (a checkout with six pinned revisions verifies
  under the Git operation bound one revision needs, which six one-revision batches exceed),
  `TestDCRV1410VerifiedReadRefusals` and every other pin-verification refusal test unchanged.
  Rollback: call `CommitTree` per revision again.
- `DCR-V1-985`: (experimental under the owner's standing lane auto-accept (2026-10-11); V1-2745)
  Inside an object session, a `LookupTreeEntry` at a full object ID MUST be answered by the
  session's one `git cat-file --batch` co-process instead of a new one-shot batch per call: it reads
  the peeled commit, then the commit's root tree, then each directory tree by the object ID its
  verified parent names, and answers only when every record is present, self-hashes to its name,
  and the one-shot batch's trailing records (with an absent directory's remaining `missing`
  records) stay within `MaxTreeBytes`. Any other answer replays the original one-shot batch under
  the same operation reservation, so the lookup charges exactly one Git operation either way and
  every result, refusal, code and remaining budget is the one-shot lookup's. Motivation: each
  lookup paid one process spawn (about 2.2 ms on WSL2), so the per-path reference reads in the
  `DCP-V1-216` tests paid one spawn per path. Non-goal: caching tree bodies across lookups (a
  replaced loose object must be read again) or changing lookups at a ref or abbreviated name,
  which keep the one-shot batch. Failure mode: a co-process that cannot answer, refuses a record
  or is stopped falls back to the one-shot batch, which costs one spawn as before. Rollback: drop
  the `sessionTreeEntry` branch in `lookupTreeEntry`. Acceptance:
  `TestDCRV1985SessionLookupsShareOneCatFile` (120 lookups over present, absent, nested and
  non-tree paths at two revisions: one `cat-file` spawn in a session, one per lookup without, with
  equal answers and remaining operations) and `TestObjectSessionMatchesOneShotReads` (a forged
  directory tree read through the session fails as the one-shot lookup does).

## Non-goals

The parent contract's non-goals and simpler baseline apply; these blocks add none of their own.

## Failure modes

- Verified reads (`DCR-V1-410`): a noncanonical artifact, a digest mismatch, a different
  builder, a schema pair, manifest or profile digest that does not match, an unresolvable pinned
  revision or a recorded tree that differs each refuse the read; none falls back to unverified
  data. A dirty or unrecorded running builder rederives instead of verifying. HEAD moving during
  a freshness observation repeats it and, after three attempts, refuses (`DCR-V1-411`). A symlink
  in the configured MCP artifact's path voids its warm stamp (`DCR-V1-417`). A live index built from another state than the freshness observation withholds symbol
  re-resolution (`DCR-V1-412`).
- Streamed reads (`DCR-V1-431`): a file read error mid-stream is `corpus-input-unavailable`, not a
  refusal of a truncated artifact; a file over 128 MiB at open is `corpus-input-unavailable` before
  any byte is decoded, and bytes past the bound during the read refuse with `input bound
  exceeded`; bytes whose last `sha256` member starts more than 4 KiB before their end refuse as
  noncanonical; input truncated exactly at a window boundary refuses as invalid input, never a
  panic. A refused query read joins its started freshness observation before returning
  (`DCR-V1-431`). A refused or interrupted decode discards the claim terms it derived, and
  concurrent queries of one artifact take them at most once (`DCR-V1-434`).
- Read-start freshness (`DCR-V1-465..467`): a primary observation that fails, or is cancelled
  by a refused read, fails the freshness that takes it exactly as a new observation would; it is
  never taken by a second freshness or by a freshness of another root, and a refusal before the
  header still cancels and joins it.
- Stat-routed revision pins (`DCR-V1-452`, `DCR-V1-453`): an unusable Git index (including an
  unknown extended entry flag or pathnames past the length and expansion bounds), a stat
  mismatch, a racy entry or a failed blob batch never refuses a build; the entry is opened and
  hashed, and a worktree copy that does not hash to the pinned blob is replaced by that blob.

## Acceptance evidence

Each block names its acceptance tests, and the Traceability table maps every block to its
implementation and evidence. The parent contract's acceptance gates apply.

## Rollback

Revert the implementing change a block names. The split itself reverts by moving these blocks back
into the parent under their unchanged DCP-V1 numbers and renaming the DCRV1 test prefixes back to
DCPV1.

## Traceability

| Requirements | Implementation boundary | Required evidence |
| --- | --- | --- |
| DCR-V1-410..419 | `internal/doccorpus/read.go`, `artifact_decode.go`, `encoding.go`, `compile.go`, `query.go`, `query_index.go`, `query_index_fast.go`, `search_rank.go`, `symbol_anchor.go`, `internal/contextindex/observe_tree.go`, `internal/testvaliditydoc/read.go`, `internal/mcp/corpusbridge/corpusbridge.go`, `stamp.go`, `eval.go`, `cmd/corvint/docs_corpus.go`, `docs_corpus_help.go` | `TestDCRV1410VerifiedReadSkipsRederivation`, `TestDCRV1410VerifiedReadRefusals`, `TestDCRV1410UnattestedBuilderRederives`, `TestDCRV1411ObserveTreeMatchesBuildContext`, `TestDCRV1411ObserveTreeRetriesWhenHeadMoves`, `TestDCRV1412SymbolIndexBuiltOnlyWhenNeeded`, `TestDCRV1413CorpusReadsWithoutRederivation`, `TestDCRV1414ParallelDecodeMatchesClosedDecode`, `TestDCRV1416CorpusMCPFirstAndWarmCalls`, `TestDCRV1417WarmCallsSkipUnchangedArtifactRead`, `TestDCRV1417WarmStampRefusesParentSymlink`, `TestDCRV1418LargeFixtureVerifiedReadIsFast`, `BenchmarkDCRV1418VerifiedRead`, `TestDCRV1419FastIndexMatchesReference`, `TestDCRV1419FastIndexDeclinesEveryRefusal`, `TestDCRV1419BoundaryScanFindsStringContent`; measured in https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-23-corpus-verified-fast-reads.md |
| DCR-V1-430..434 | `internal/doccorpus/artifact_decode.go`, `read.go`, `encoding.go`, `query_index_fast.go`, `types.go`, `internal/testvaliditydoc/read.go`, `cmd/corvint/docs_corpus.go`, `corpus_integration.go` | `TestDCRV1431StreamedDecodeMatchesInMemory`, `TestDCRV1431StreamedReadReusesBlocks`, `TestDCRV1431OpenReadFile`, `TestDCRV1431StreamedTruncationAtWindowEnd`, `TestDCRV1431CorpusRefusalJoinsFreshness`, `TestDCRV1432JoinedCorpusReusesVerifiedArtifact`, `TestDCRV1434DecodeTimeClaimTermsMatchIndex`, `TestDCRV1434ConcurrentQueriesShareClaimTerms`, `TestDCRV1416CorpusMCPFirstAndWarmCalls`; measured in https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-23-corpus-read-residual.md |
| DCR-V1-551..553 | `internal/doccorpus/artifact_scan.go`, `artifact_decode.go` | `TestDCRV1551ParallelElementsMatchScan`, `TestDCRV1551ParallelElementsCorrupt`, `TestDCRV1552ParallelScanMatchesSequential`; measured in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-24-corpus-parallel-boundary-scan.md` |
| DCR-V1-465..467 | `internal/doccorpus/primary_observation.go`, `query.go`, `cmd/corvint/docs_corpus.go` | `TestDCRV1465PrimaryObservationMatchesFreshRead`, `TestDCRV1466PrimaryObservationUsedOnce`, `TestDCRV1467CancelledPrimaryObservationJoins`, `TestDCRV1467ShorterDeadlineStopsWaitingForPrimary`, `TestDCRV1467CorpusPreHeaderRefusalJoinsPrimary`, `TestDCRV1431CorpusRefusalJoinsFreshness`, `TestDCRV1413CorpusReadsWithoutRederivation`; measured in https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-23-corpus-read-start-freshness.md |
| DCR-V1-468 | `internal/doccorpus/artifact_decode.go`, `query_index_fast.go` | `TestDCRV1431StreamedDecodeMatchesInMemory`, `TestDCRV1434DecodeTimeClaimTermsMatchIndex`, `TestDCRV1419FastIndexMatchesReference` |
| DCR-V1-554..558 | `internal/doccorpus/artifact_decode.go`, `query_index_fast.go`, `claim_audit.go`, `read.go` | `TestDCRV1554UndecodableHeaderMemberRefuses`, `TestDCRV1431StreamedDecodeMatchesInMemory`, `TestDCRV1414ParallelDecodeMatchesClosedDecode`, `TestDCRV1419FastIndexMatchesReference`, `TestDCRV1419FastIndexDeclinesEveryRefusal`, `TestDCRV1557ClaimAuditGapsMatchReference`, `TestDCRV1410VerifiedReadRefusals`; measured in https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-25-corpus-warm-read.md |
| DCR-V1-559 | `internal/doccorpus/artifact_scan.go`, `artifact_decode.go` | `TestDCRV1559BlockScanMatchesScan`, `TestDCRV1552ParallelScanMatchesSequential`; measured in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-27-corpus-block-prescan.md` |
| DCR-V1-975 | `internal/doccorpus/read.go`, `internal/cem/gitauth/tree.go` | `TestDCRV1975PinnedRevisionsOneBatchPerCheckout`, `TestDCRV1410VerifiedReadRefusals` |
| DCR-V1-985 | `sessionTreeEntry` and `servedTreeEntry` in `internal/cem/gitauth/object_session_lookup.go`; `runCommitBatch` in `commitstream.go`; `RunReservedStream` and `RunReservationStream` in `internal/cem/gitrun/gitrun.go` | `TestDCRV1985SessionLookupsShareOneCatFile`, `TestObjectSessionMatchesOneShotReads` |
| DCR-V1-460..462 | `internal/doccorpus/delta_open.go`, `delta.go`, `read.go`, `artifact_decode.go`, `compile.go`, `cmd/corvint/docs_corpus_delta.go`, `docs_corpus_help.go` | `TestDCRV1460DeltaVerifiedReadMatchesRederived`, `TestDCRV1460DeltaVerifiedReadRefusesClosed`, `TestDCRV1460DeltaVerifiedReadUnavailableCheckout`, `TestDCRV1460DeltaCLIVerifiedRead`, `TestDCRV1461PreviousOverDefaultBound`, `TestDocsCorpusOperationFlagMatrix`, `TestDocsCorpusOperationHelpPrintsFlags` |
| DCR-V1-463 | `internal/perfbudget/budgets.json`, `harness.go` | `TestPRBV0050UnchangedDeltaBudget`, `TestDCRV1460DeltaCLIVerifiedRead`; measured in https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-23-corpus-delta-verified-read.md |
| DCR-V1-464 | `internal/doccorpus/delta_open.go` | `TestDCRV1464DeltaVerifiedReadChecksInputBindings` |
| DCR-V1-450 | `internal/doccorpus/compile.go`, `query.go` | `TestDCRV1450ScopeMatchingMatchesPairwise`, `TestDCRV1450NestedScopesKeepRefusalsAndReasons`, `TestDCRV1450NewLiveFileNotedPerHoldingScope` |
| DCR-V1-451 | `internal/doccorpus/read_fast_fixture_test.go`, `many_scopes_test.go` | `TestDCRV1451ManyScopesReadMatchesRederivation`, `BenchmarkDCRV1451ManyScopesVerifiedRead`, `BenchmarkDCRV1451ManyScopesRederivedRead`; profiles in https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-23-read-hotspots-scope-and-pin.md |
| DCR-V1-452..453 | `internal/contextindex/stat_routed_pin.go`, `index_stat_darwin.go`, `index_stat_linux.go`, `index_stat_unix.go`, `index_stat_other.go`, `index.go`, `corpus.go` | `TestDCRV1452GitIndexVouchesUnchangedEntries`, `TestDCRV1452RacyEntryIsNotVouched`, `TestDCRV1452IndexVersionsAndFlags`, `TestDCRV1453UnusableIndexVouchesNothing`, `TestDCRV1453IndexPathnameBounds`, `FuzzParseGitIndex`, `TestDCRV1453FailedBlobBatchFallsBackToWorktree`, `TestDCRV1453RoutedPinMatchesParityRepository` |
