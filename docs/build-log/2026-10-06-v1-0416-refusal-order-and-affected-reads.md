# 2026-10-06: V1-0416 source-count refusal before the status scan; `affected` reads on the open descriptor

## Intent

Ticket V1-0416 (no GitHub issue): measure index, query and snapshot cost at 50,000 and 200,000
files and fix what does not change a contract. Owner goals (2026-10-06): large projects without
slowdowns; never burn CPU unless essential. Round 1 (`2026-10-06-v1-0881-single-event-bracket.md`)
fixed the hook bracket; this round covers two hot spots the coordinator selected: the source-count
refusal on a 200,000-file tree spent seconds in a status scan it never needed, and `affected` at
200,000 files spent 19% of its CPU resolving every source path twice.

## Change

1. `internal/contextindex/index.go`: `buildStableFrom` takes its fresh opening observation
   through `openingObservationWithTree`, which runs the status scan in a goroutine, reads the
   identity, then issues `ls-tree -r -l -z --full-tree <tree>` beside the scan. The tree is
   immutable content named by the identity, so the listing cannot depend on the scan; its
   source-count and byte-bound refusals now return while the scan is still running and before
   any `cat-file`. A listing failure cancels the scan and waits for it (no goroutine outlives the
   build); a listing error wins when both fail. `buildEvidence` is split into the listing and
   `buildEvidenceFrom` so nothing else moves. The carried loader observation on attempt 0 and the
   `IDX-SNAP-V0-016` shard path (`openingObservation`, `blob_shards.go`) are unchanged.
   Spec: `IDX-SNAP-V0-026` (proposed, not accepted) in `docs/specs/index-snapshot-v0.md`.
2. `internal/liveverify/affected`: `ReadSource` reads through `readSourceFile`. On darwin and
   linux (`read_unix.go`) it opens `O_RDONLY|O_NOFOLLOW|O_NONBLOCK`, maps `ELOOP`/`EMLINK` to
   `ErrInvalidUnit`, checks regular-file mode and `MaxSourceBytes` on `fstat` of the descriptor,
   and refuses a body that grows past the bound during the read. Other platforms (`read_other.go`)
   keep the `Lstat` + `ReadFile` pair. Spec: `AFP-V0-034` (proposed, not accepted) in
   `docs/specs/affected-plan-v0.md`.
3. `internal/contextindex/analyzer_schema_test.go`: `IDX-SNAP-V0-017` input-audit digest
   re-pinned for the `index.go` edit (consumer-only: `analyzerSchemaID` stays
   `corvint-analyzer/105`; no fact or encoding changed, the precedent is commit 43f3e40d).

## Decisions

- **Same refusal, earlier.** The coordinator ruled the pre-check is not a contract change: the
  code, message and limit are the ones `readTreeEntries` already returns. The build still closes
  the window with a full identity-and-status observation, so `GPK-V0-007`'s proof shape holds.
- **Listing error wins.** When the listing fails and the cancelled scan also reports an error, the
  listing's error is returned: it is the one that happened first and the one a caller can act on;
  the scan's error after cancellation is a consequence.
- **Only the walk's reader.** `ReadBounded`'s disk path still takes the `Lstat` pair; it is not on
  the measured hot path and is left for a follow-up rather than widened into this change.
- **`O_NOFOLLOW` only where it exists.** Build tags mirror `internal/delta/input_unix.go`; the
  fallback keeps the old behaviour byte for byte, so CI's `GOOS=windows` build is unchanged.
- **Peak memory on a full rebuild rises slightly.** The listing now overlaps the scan's buffers.
  r50k rebuild medians of three: 532 MiB before, 585 MiB after (round 1 measured 563-614 MiB on
  the unchanged binary, so the pairing is inside the earlier spread; not confirmed as a regression,
  retained as an open observation).

## Evidence

Measured on Darwin, 1-minute load average 8-11 (other lanes running), `GOMAXPROCS=3`; medians of
three (`TD/logs/measure2.txt`, `TD/logs/rebuild-probe.txt`). Before: the branch at `85800e50`;
after: this change. r50k and r200k are generated repositories of 50,000 and 200,000 tracked
files with a private `.git/corvint` store each.

| Repository | Command | Before wall s / CPU s / RSS MiB | After wall s / CPU s / RSS MiB | Result |
|---|---|---|---|---|
| r200k | `index --if-stale` | 2.39 / 5.13 / 132 | 0.54 / 0.65 / 134 | rc 2, same source-count refusal |
| r200k | `context --task ...` | 2.73 / 5.93 / 131 | 0.40 / 0.60 / 127 | rc 2, same refusal |
| r200k | `affected`, 1 dirty | 17.49 / 28.13 / 422 | 16.36 / 25.47 / 461 | rc 0 |
| r50k | `affected`, 1 dirty | 3.96 / 5.93 / 129 | 2.47 / 5.55 / 130 | rc 0 |
| r50k | `index --if-stale`, hit | 0.13 / 0.11 / 147 | 0.13 / 0.12 / 145 | rc 0, unchanged |
| r50k | `index`, full rebuild | 2.96 / 14.40 / 532 | 2.94 / 13.36 / 585 | rc 0 |

Round 1's "9-10 s" for the r200k refusal was host pressure (load 15-21); re-probed at load 8-10
the old binary refused in 2.3-2.7 s, 83-87% of its CPU in `gitstatus.StatusIn`'s native
pre-scan. The listing itself costs 0.22 s at 200,000 entries (`ls-tree -r -l -z`).

`affected` at r200k, CPU profile of the old binary (34.4 s wall under load, 19.7 s sampled,
`TD/tmp/affected-r200k.prof`), top three costs:

1. Whole-body Go source reads, `ReadSource` 13.9 s (70%): `os.ReadFile` 10.15 s of which
   `syscall.Open` 7.84 s, plus `os.Lstat` 3.73 s (19%). Path resolution, not bytes, dominates:
   open and lstat together are about 11.5 s. The `Lstat` half is removed by this change; the
   open half needs reads relative to directory handles or per-blob cached facts (follow-up).
2. TypeScript and Python unit sources, `typescript.unitsSource` 3.64 s.
3. The `gitstatus` scan 1.92 s and the `SourceFilesIncluding` walk 1.76 s.

Tests (`TMPDIR=TD/tmp GOMAXPROCS=3 GOTOOLCHAIN=local go test -p 1 -count=1 -timeout 30m`):
`TestBuildRefusesSourceCountBeforeStatusFinishesOrBlobsRead` PASS in 0.9 s on the new
`index.go` and FAIL after 11.4 s ("the refusal waited for the status scan") on the old one
(`TD/contextindex-refusal-order.log`, `-negative.log`);
`TestReadSourceRefusesNonRegularFilesOnOpenDescriptor` and the rest of
`./internal/liveverify/affected/...` PASS (`TD/affected-tests.log`); `TestAnalyzerSchemaInputs`
PASS after the re-pin. `go vet` clean on darwin, linux and windows. The frozen evaluations are
not touched (no ranking input moved); `dogfood-change` binding is left to the integrating session.

Not reproduced: the `cmd/corvint` fixture-setup "git signal: bus error" seen once in
`TestAFUV1FlowsCLIImportReportsNothingWritten` under load 15-21 passed on rerun
(`TD/cmd-flows-rerun.log`); attributed to host pressure, not to this change.

## Rollback

Revert the commit. No snapshot encoding, envelope byte, refusal code or bound changes; the
analyzer audit digest returns with the `index.go` bytes.
