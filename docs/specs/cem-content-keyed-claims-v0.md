# CEM Content-Keyed Claims V0

Owner: Russell Lewis
Date: 2026-11-01
Requirement prefix: `CKC-V0`
Intent status: proposed
Delivery status: experimental
Authoritative inputs: ticket V1-3980 (CEM 2.0 Phase 1a); `AGENTS.md` invariants 1, 2, 4 and 8; `docs/specs/daily-change-evidence-workflow-v0.md` DCW-V0-150, DCW-V0-350, DCW-V0-442 and DCW-V0-884; `internal/cem/wire/canonical.go` `HunkIdentity`.

## Agent digest
- Claim: Reviewed plan rows persist as content-keyed claims that compile to current hunk IDs across fixes and rebases, refusing every ambiguous match.
- Status: proposed/experimental
- Exists: `corvint dogfood record-claims`, the claims form of `corvint dogfood propose-citations --refresh`, `internal/dogfoodflow/claims.go` and its focused tests and replay harness.
- Blocked on: owner acceptance; adoption in `docs/DOGFOOD.md` (owned by another lane, proposed text only); live lane use.
- Read next: Requirements; Failure modes; Acceptance evidence.

## User and measurable job

A lane reviews one citation-plan row per hunk, then commits a fix or merges `origin/main`. A cem/1.0 hunk ID hashes the hunk's old and new line ranges (`wire.HunkIdentity`), so the fix re-keys every later hunk of the file and a new BASE re-keys most hunks. `--refresh PLAN` (DCW-V0-442) re-keys rows by path and content, or by path and BASE range, but it has three gaps:

- It carries a row onto a hunk whose content changed.
- It drops every row once BASE moved, because the old commits no longer descend from the new BASE (V1-3814).
- It redrafts rows the author deleted.

The job is to carry exactly the reviewed rows whose edit is provably the same, refuse every ambiguous case, and keep deleted rows deleted. The bars are at least 50% fewer manual row edits and at least 30% lower p95 time than the current flow on the same replays.

## Verified current state

At base `b7ffc233396da54e2ac5a094ec314dc3b067f6fb`:

- `internal/dogfoodflow/propose_refresh.go` `carry` resolves old hunk IDs only through commits that descend from the current BASE.
- Its second matcher (path and BASE range) carries rows across content changes, and notes "content changed, so review the row".
- A dropped hunk is drafted afresh, so a deleted drafted row comes back.
- The bind loop cites each plan row through `cem cite`/`cem mark` against a freshly prepared map. The plan grammar, not the CEM wire, is the only input this slice changes.

## Requirements

- `CKC-V0-001`: Claims file format.
  - A claims file is deterministic, indented JSON with schema `corvint-dogfood-claims/v0`: `base` (the full BASE commit ID), `claims` and `deferred`.
  - Each claim has these fields:
    - `path`: the hunk's display path.
    - `content`: the hunk's `contentSha256`, which covers its context, removed and added lines.
    - `oldStart` and `oldCount`: its BASE range.
    - `anchor`: the SHA-256 of the BASE lines that range covers.
    - `occurrences`: how many times those lines occur, line-aligned, in the BASE file. A zero-line range counts 1.
    - `rows` and `tombstones`. A claim holds at least one of them; a claim with only tombstones is CKC-V0-017's.
  - A cite row stores `path`, `lines` (BASE `START:END`), `relation` and `text`, the SHA-256 of the cited BASE lines. An unknown row stores `reason` and `note`.
  - Claims are sorted by path, BASE range and content, and rows keep plan order, so equal inputs give equal bytes.
  - The file is at most 4 MiB, with at most 256 claims and 256 rows in all. It is written mode 0600 through a temporary file and rename.
- `CKC-V0-002`: Recording a plan.
  - `corvint dogfood record-claims BASE --plan PLAN --claims FILE [--draft DRAFT]` reads the reviewed plan and resolves each row to one hunk of the canonical BASE..HEAD patch:
    - A full hunk ID resolves directly.
    - An ordinal resolves only through a prepared map whose BASE is BASE and which still matches the patch (DCW-V0-382).
  - Each row must be a valid four-field cite or reasoned unknown row (DCW-V0-019). Deferred rows are kept as written (DCW-V0-370).
  - Any unresolved, stale, malformed or over-bound row refuses the whole record by row number, and nothing is written.
  - A claim's cite rows are hashed from the BASE blob. A cited range outside its BASE file refuses.
- `CKC-V0-003`: Tombstones.
  - A tombstone is a cite row the author removed for an edit. Two sources record one:
    - A cite row of `--draft DRAFT` for a hunk that the reviewed plan does not hold for that hunk.
    - A row of an earlier claims file at `--claims FILE` whose claim matches the same hunk exactly (CKC-V0-004) and which the plan dropped.
  - An earlier tombstone stays on every hunk its claim matches or overlaps, unless the plan re-adds that row.
  - Unknown rows are never tombstoned: an unknown row is the draft's safe fallback.
- `CKC-V0-004`: Exact match.
  - `corvint dogfood propose-citations BASE --refresh FILE`, given a claims file, compiles each claim against the canonical patch.
  - A claim matches a hunk exactly when one current hunk has the same path and content and its BASE range is the claim's range. When BASE changed, that range is the claim's range relocated by its anchor.
  - With BASE unchanged, no anchor uniqueness is needed. When BASE changed, relocation needs the anchor lines to occur exactly once in both the claim's BASE file and the current BASE file. A zero-line range relocates only to itself.
  - A claim is never matched by hunk ID, ordinal or range alone.
- `CKC-V0-005`: Ambiguous matches are refused, and a refused claim carries no row. These classes are refused:
  - `duplicate`: two or more current hunks of the path have the claim's content, or two claims match one hunk.
  - `unlocated`: BASE changed and the anchor is absent from, or not unique in, either BASE file.
  - `split`: two or more current hunks overlap the located range.
  - `merged`: one overlapping hunk overlaps two or more claims.
  - `changed`: one overlapping hunk has other content.
  - `cite-unlocated`: a cite row's text is absent, ambiguous or changed in the current BASE.

  Each refused hunk is drafted afresh. stderr marks each refusal `REASSESS` with its class, the hunk and the claim's earlier rows. A claim that overlaps no hunk is `gone` and is noted.

  When a fix reverts every claimed edit, the canonical patch is empty. An empty patch that was read is a patch with no hunks, as the draft's patch read has it: every claim is `gone` and the refresh succeeds. Only a patch that could not be read or parsed refuses `claims-patch-unavailable`.
- `CKC-V0-006`: Cite relocation.
  - When BASE changed, a carried cite row's `START:END` is moved to the single line-aligned occurrence of its cited text in the current BASE file. The text hash must match the claim's BASE bytes.
  - With BASE unchanged, cite rows carry as recorded.
- `CKC-V0-007`: Tombstone filter. A drafted cite row equal to a tombstone of a claim that matches or overlaps the hunk is never printed. Equality is the same path and relation and the same SHA-256 of the cited BASE text, wherever BASE now numbers it. The hunk instead gets the draft's unknown fallback row, whose note names the removed row, so the precheck (DCW-V0-019) still passes.
- `CKC-V0-008`: Compiled output.
  - Compiled rows are keyed by full hunk ID, in canonical patch order, followed by the claims file's deferred rows.
  - The output is an ordinary cem/1.0 citation plan. Binding it gives the same map bytes as binding the equivalent hand-keyed plan.
  - Claims are never an input to `cem`, the CEM wire, the seal or any verifier, and the cem/1.0 wire does not change.
- `CKC-V0-009`: Read and write scope.
  - `propose-citations` stays read-only (DCW-V0-150).
  - `record-claims` writes only its `--claims` file, and refuses a path that is not a regular file or absent.
  - A claims file whose schema, base, bounds or row grammar is invalid refuses the refresh by name, and nothing is drafted.
- `CKC-V0-010`: The PROPOSED header of a claims refresh reports `claims=FILE carried=N reassess=N gone=N suppressed=N deferred=N`.
- `CKC-V0-011`: Evidence.
  - A differential test shows a compiled plan binds to byte-identical cem/1.0 maps.
  - Claims carried incrementally through fix steps compile to the same plan and map as claims recorded once at the final step.
  - A replay harness drives 20 real fix sequences of this repository plus injected duplicate, split, merge and reorder cases, and counts incorrect carries.
  - The same replays measure manual row edits and the plan-step time of both flows.
  - Unmeasured closeout time stays `NOT_OBSERVED`.
- `CKC-V0-012`: Batched blob reads.
  - `record-claims` reads every BASE file it hashes (the hunk paths and the cited paths of the plan and draft) through one `ls-tree` and one `cat-file --batch` call, not two Git processes a file.
  - A claims compile after BASE moved batches its claim and row paths at both commits the same way.
  - A batched read caches each name exactly as the per-file read would, including absent for a missing path, a directory, a symlink or a submodule.
  - A name with a pathspec or revision metacharacter, a blob past the flow's Git output bound and every name of a failed batch fall back to the per-file read.
  - Recorded claims are byte-identical with and without the batch.
- `CKC-V0-013`: In a refresh, each stderr hunk note (`hunk ID edits PATH at BASE ...`) ends with `; carried` when the hunk's rows were carried from the earlier plan or claims, and `; drafted` otherwise. A plain draft adds no mark.
- `CKC-V0-014`: Carried-only refresh skips the anchor search.
  - When a refresh (from claims or from a plan) carries rows for every owed hunk, the draft runs neither the BASE `git grep` for anchors nor the governing-spec read, since only drafted hunks use them.
  - The canonical patch is still read, because carrying needs its hunks.
  - The compiled plan and the stderr are byte-identical to the searching path.
  - A refresh that drafts any hunk searches as before.
- `CKC-V0-015`: Batch blob reader.
  - A draft or record reads each blob it has not prefetched through one long-lived `git cat-file --batch` reader per invocation, not an `ls-tree` and a `cat-file` process a blob.
  - The reader answers exactly as the per-file read: absent for a missing path, a directory, a symlink or a submodule, and the regular blob's bytes otherwise.
  - A commit or name that is not a clean literal, and every read after a reader failure, use the per-file read, which keeps that read's refusal.
  - Any object past the flow's Git output bound stops the reader before its body is read or allocated, whatever its mode or type: a blob, a symlink, a tree or a gitlink. The per-file read then answers, refusing an oversized blob and reading a non-regular entry as absent.
  - A Git whose `cat-file` lacks the `%(objectmode)` atom fails the reader's first read, and every read falls back with the same answers. The reader-stays-up assertion runs only where Git has the atom; the fallback's parity is tested on every Git.
  - The flow's boundary closes the reader, so no Git process outlives the command.
  - Drafts, recorded claims, record stderr and claims refreshes are byte-identical with and without the reader.
- `CKC-V0-016`: Draft anchor search skips data files.
  - The draft's BASE `git grep` excludes `**/*.json`, `**/*.jsonl` and `.agent-evidence/`, the data paths whose matches the draft already discards.
  - Its output is byte-identical to the unexcluded search.
  - One disclosed exception: a search that went past the Git output bound only through data files now completes instead of refusing.
  - The check-tier proposal keeps its own search unchanged.
- `CKC-V0-017`: Tombstone-only claims.
  - When the reviewed plan holds no row for an edit that has a tombstone (CKC-V0-003), `record-claims` still writes a claim for that edit, with empty `rows` and its tombstones.
  - A refresh compiles that claim like any other. It carries no row, so the edit is drafted afresh and marked `; drafted`, and the tombstones keep the removed cites out of the draft (CKC-V0-007).
  - Re-recording over such a claims file keeps its tombstones on every hunk the claim matches or overlaps.
- `CKC-V0-018`: Strict claims grammar.
  - A claim cites `START:END` line spans only. A plan row with a `bytes:` span refuses the record by row number, naming the byte span. A claims file holding one refuses the refresh, because a byte span has no text hash to relocate or tombstone by.
  - A claim whose BASE range is negative, starts before line 1 with a nonzero count, or ends past the largest int64 refuses the decode by name (`invalid BASE range`). The range reader checks its end by subtraction, so no range can overflow the slice.
  - The claims JSON document must be the file's only value; any data after it except whitespace refuses the decode by name.

## Failure modes

| Condition | Required observation |
|---|---|
| Fix commit shifts later hunks' new ranges | Exact carry, same rows |
| Fix commit changes a hunk's content | `changed` REASSESS, hunk drafted afresh, no carry |
| Fix splits or merges hunks | `split` / `merged` REASSESS, no carry |
| Same content twice in one file | `duplicate` REASSESS, no carry |
| BASE moved, anchor unique in both | Exact carry with relocated cite lines |
| BASE moved, anchor or cited text not unique or gone | `unlocated` / `cite-unlocated` REASSESS |
| Author deleted a drafted cite | Never redrafted; unknown fallback with note |
| Claims file invalid, oversized or for unreadable BASE blobs | Named refusal, nothing drafted or written |
| Plan row unresolved or stale at record | Named refusal by row, nothing written |
| Every owed hunk carried | No anchor search, output byte-identical to the searching path |
| Batched blob read fails, is over the bound, or a name is not batchable | Per-file read for those names, same bytes and absent cases |
| Batch reader fails mid-flow, or Git lacks `%(objectmode)` | Per-file reads from then on, same bytes and refusals |
| Blob past the Git output bound through the batch reader | Per-file read refuses with the existing over-bound code |
| Anchor matches only in excluded data files | Same draft; the search no longer refuses on data-only overflow |
| Symlink, tree or gitlink past the Git output bound through the batch reader | Reader stops before the body is read; the per-file read answers absent |
| Author removed every row of an edit that had a drafted cite | Tombstone-only claim; the edit is drafted afresh without the removed cites |
| Fix reverts every claimed edit (empty patch) | Every claim `gone`; no `claims-patch-unavailable` refusal |
| Claims file with a byte-span row, an invalid BASE range or data after the JSON document | Named `claims-unreadable` refusal, nothing drafted |
| Plan row with a byte span at record | Named refusal by row, nothing written |

## Acceptance evidence

These focused tests live in `internal/dogfoodflow/claims_test.go` and `internal/dogfoodflow/claims_strict_test.go`:

- `TestClaimsCompileByteIdenticalCEM` covers the AC1 differential test and the incremental-equals-full check.
- `TestClaimsAmbiguousRefused` covers the injected duplicate, split, merge, reorder and changed cases.
- `TestClaimsRebaseRelocates` covers relocation after BASE moves.
- `TestClaimsTombstones` covers AC4.
- `TestClaimsRecordRefusals` covers the record refusals.
- `TestClaimsPrefetchBlobsMatchesBlob` covers CKC-V0-012: the batched cache equals the per-file read for regular, empty, executable, missing, directory and symlink names, unbatchable names stay uncached, and the bound skips an oversized blob.
- `TestClaimsTombstones` also covers CKC-V0-013's carried and drafted marks.
- `TestClaimsAllCarriedSkipsAnchorSearch` covers CKC-V0-014 for a claims refresh and a plan refresh, and checks that a refresh with a drafted hunk still searches.
- `TestClaimsBatchBlobMatchesBlob` covers CKC-V0-015. It compares batched and per-file reads across two commits and an unknown commit, for regular, empty, executable, symlink, submodule, directory, missing, spaced, glob and unclean names. It checks the bound refusal, the fallback after it and the boundary close. It also checks that the draft, record stderr, claims bytes and a post-fix claims refresh are byte-identical with and without the reader. It asserts that the reader stays up only where Git's `cat-file` has `%(objectmode)`, and its `without objectmode` subtest swaps in a format Git rejects and checks the fallback's parity on every Git.
- `TestClaimsBatchBoundsEveryObject` covers CKC-V0-015's bound for a symlink and a tree past it: the reader stops before reading the object, and both reads answer absent.
- `TestClaimsEveryEditRevertedIsGone` covers CKC-V0-005's empty patch: every claim is `gone` and nothing refuses.
- `TestClaimsTombstoneOnlyClaim` covers CKC-V0-017.
- `TestClaimsStrictDecode` and `TestClaimsRangeRecordsBounds` cover CKC-V0-018.
- `TestClaimsDataExcludesKeepDraft` covers CKC-V0-016: a draft whose anchors also match JSON, JSONL and `.agent-evidence/` files is byte-identical with and without the excludes.

`TestClaimsReplay` runs only when `CKC_REPLAY_REPO` and `CKC_REPLAY_SEQUENCES` are set. It replays real history and prints the raw counts and timings recorded in `docs/build-log/2026-11-01-ckc-w15.md`. CLI wiring is covered in `cmd/corvint`.

## Non-goals and limits

- No cem/1.0 wire change and no CEM 2.0 encoding.
- No caching of cite or check results.
- No persisted anchor-search cache. A cache written by a read command sits outside invariant 4, and the replays measured no cross-step repeat of a search.
- No semantic judgment that a carried row still holds: an exact content match is the boundary, and the bind's precheck still judges every row.
- No automatic recording in the bind loop. The `docs/DOGFOOD.md` adoption is proposed text only.
- Relocation is line-aligned exact text. A BASE edit inside a hunk's range or context refuses rather than guesses.
- Full closeout time is not measured by this slice.

## Rollback

Remove `record-claims`, the claims branch of `--refresh` and their tests. Plan-file `--refresh` and every bind path are unchanged throughout. Claims files are private lane inputs, with no migration.
