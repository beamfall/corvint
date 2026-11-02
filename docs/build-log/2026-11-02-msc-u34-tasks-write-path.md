# 2026-11-02 msc-u34 tasks write path (V1-1148, V1-4229)

Decision: the owner lifted the V1-0910 cache hold on 2026-11-01 ("you can lift it"), relayed by the
orchestrator for lane msc-u34 round 2. For this scope that supersedes the 2026-10-24 rejection
recorded in `docs/build-log/2026-10-24-owner-held-decisions.md`, which had kept decision 0549 F1. A
write-path or in-process structure is admitted; a persisted index needs its own derived-state
argument in the spec, and read commands still persist nothing (invariant 4).

BASE `35475f860d97bfde9bbaa6cccf1f2ba8f78ea44b`; host unraid-lanes (12 CPUs, btrfs, Go 1.27.3).
Lane tkp-u33 is editing `internal/tasks/store`, journal and wire on the same host (V1-4217,
V1-3275, V1-4243: 49 files, including `writer_checkpoint.go`, the segmented journal audit and
`wire/parse.go`/`encode.go` fast paths).

## Before numbers at BASE

`ticket create` through `tasks-scale run`, one cold process per sample, on gen-built stores:

| Store | wall ms | user ms | sys ms | allocated | load (1 min) |
|---|---|---|---|---|---|
| 100 tickets | 218, 224, 222 | 70, 68, 66 | 28, 35, 31 | 26.4-27.0 MB | 2.3-3.3 |
| 500 tickets | 410, 275, 277 | 197, 205, 202 | 182, 42, 59 | 80.6-81.1 MB | 2.3-3.3 |

CPU grows about 0.33 ms and 135 KB per ticket, so a create at 2,000 tickets projects to roughly
700 ms of CPU. The rest of the wall time is off-CPU (durable writes under host I/O load).

`tasks-scale gen` at BASE:

- `--tickets 100`: 36.9 s (load 4.4).
- `--tickets 500`: 256.4 s (load 3.3 to 2.3); 181.8 s in round 1 at load 2.4 to 2.6.
- `--tickets 2000`, bounded at 530 s (the lane's per-command bound): 500 tickets at 187 s, 1,000
  at 519 s (the second 500 took 332 s), and killed at 530 s with 1,014 tickets written. The full
  run is `NOT_OBSERVED`. The 500-ticket chunks grow roughly 187, 332, ... s, so the total projects
  to well over 20 minutes.

## Where a create's CPU goes at 500 tickets

Four creates, 1.40 s of samples:

| Pass | Package | Share |
|---|---|---|
| `ticket.Decode` in total, of which: | `ticket`/`wire` | 38.6% |
| - CLI preload `intent.Load` (`decodeTree`) | `cli` → `intent` | 16.4% |
| - model `transaction.loadTicket` | `transaction` | 17.1% |
| `Record.FileDigest` re-encode in `loadTicket` | `transaction` | 10.7% |
| writer journal audit `AuditForWriter` | `store` → `journal` | 15.7% |
| closed-index rewrite `WriteClosedIndex` (`LoadTreeIndexed` decodes open tickets) | `cli` → `intent` | 10.0% |

Every write runs five passes over every ticket: the CLI preload, the intent tree probe, the
writer's journal audit, the model decode and the closed-index rewrite. A create cost that differs
by under 20% between 500 and 2,000 tickets needs all of them gone or flattened, not only the
model's.

## Design (TCX-V0-845 to TCX-V0-848)

Per the orchestrator's 2026-11-02 decision (option A), the scope widened to `internal/tasks/cli` and
`internal/tasks/intent`. The writer journal audit (`journal.Reader.AuditForWriter`) is left to
tkp-u33.

- One write-path index: the existing `<state dir>.closed-index` becomes format `/3` and holds every
  ticket, open or closed. A rewrite admits a newly read file only when `Encode(Decode(b)) == b`,
  and still keeps any entry whose stat key matches (`TCX-V0-984`). Reads trust only closed entries,
  so every read path is unchanged.
- The model's re-encode writer check is kept. The base model requires an unselected decoded
  ticket's `FileDigest()` (the SHA-256 of its re-encoding) to equal its projection's digest. The
  model uses an entry only for canonical bytes whose SHA-256 and length equal the entry's. Those
  bytes are byte-identical to bytes that already passed `Encode(Decode(b)) == b`, so `FileDigest()`
  equals that digest, and the model still requires the projection's digest and length to equal
  it. The target, unmatched tickets and every other writer route decode as before. A lazy decode
  that fails is a `MALFORMED` refusal (a recover in `transaction.model`), never a panic.
- CLI preload: `intent.LoadSummarized` reads and digests every file, as `intent.Load` did, and
  holds a ticket as its entry's summary only when the digest and length match. The record is
  decoded from those same bytes on first `Get`.
- `mutation.allocateSerial` `Get`-ed every ticket only to read its ticket ID; it now reads the
  summary, which carries the same ID.

### Edits that overlap tkp-u33's area (store, journal, wire)

- `internal/tasks/store/writer_route.go`: three lines read the index and pass `TicketIndex` to
  `transaction.Model`.
- New test `internal/tasks/store/write_index_test.go`.
- No journal or wire file was touched.

Outside the original scope, all minimal:

- `internal/tasks/mutation/apply.go` (`allocateSerial`).
- `internal/tasks/intent/closed_index.go` and `store.go`.
- `internal/tasks/cli/mutate.go`.
- `internal/tasks/transaction/model.go`.
- `docs/specs/corvint-tasks-agent-leases-v0.md` (one line citation repinned, because the model's
  recover block moved the cited cutover check from line 789 to 806).

## After numbers

One cold `tasks-scale run` process per sample, BASE and lane binaries interleaved, n=7 per cell,
load 4.04 to 4.14. Each binary ran on its own relocated copy of the same store, so the `/2` and
`/3` index formats never alternate on one store. The 2,000-ticket store is the lane gen's 1,216
tickets plus 778 plain creates, because gen cannot finish 2,000 within the lane's per-command
bound (below).

| `ticket create` | wall p50 ms | CPU p50 ms (user+sys) | allocated p50 |
|---|---|---|---|
| BASE, 500 tickets | 210.9 | 223.3 | 80.1 MB |
| lane, 500 tickets | 150.4 | 100.8 | 27.9 MB |
| BASE, 2,000 tickets | 426.2 | 627.0 | 267.6 MB |
| lane, 2,000 tickets | 269.1 | 238.6 | 76.2 MB |

At 2,000 tickets a create is 37% faster in wall time, uses 62% less CPU and allocates 72% less.

CPU of the passes these blocks own, from CPU profiles (BASE 8 creates per store, load 2.9; lane 12
creates per store, load 3.8), per create:

| Pass | BASE 500 | BASE 2,000 | lane 500 | lane 2,000 |
|---|---|---|---|---|
| CLI preload (`intent.Load` / `intent.LoadSummarized`) | 31 ms | 110 ms | 8 ms | 32 ms |
| model decode (`transaction.loadTickets`) | 50 ms | 176 ms | 0 ms | 4 ms |
| index rewrite (`intent.WriteClosedIndex`) | 20 ms | 78 ms | 7 ms | 18 ms |
| index reads (`intent.ReadClosedIndex`, 3 per create) | - | - | 3 ms | 8 ms |
| total, these passes | 101 ms | 364 ms | 17.5 ms | 61 ms |
| writer journal audit (tkp-u33-owned) | 22 ms | 64 ms | 26 ms | 64 ms |

- The target is **not met.** These passes cost 61 ms at 2,000 tickets against 17.5 ms at 500,
  about 3.5 times, not under 1.2 times. They are 83% cheaper than BASE at 2,000 tickets, and their
  per-ticket slope fell from about 0.175 ms to 0.029 ms.
- What remains is O(n) by design:
  - The preload reads and hashes every ticket file. That is the byte-exact check the constraint
    requires, and it is 82% of the preload, about 26 ms at 2,000 tickets.
  - The single-file index is stat-walked, encoded and parsed whole on every rewrite, and parsed
    three times per create.
- Closing the gap needs one of these, each a decision for the orchestrator or owner:
  - a stat-trusted preload, as reads already use, which is not byte-exact;
  - an incremental index format;
  - one tree capture shared between the preload and the writer's journal read (tkp-u33's area).
- The writer journal audit, tkp-u33-owned, grew from 26 to 64 ms over the same range and is
  recorded here only for the combined re-measure at the 1101e merge.

`tasks-scale gen --tickets 2000`, lane binary, bounded at 540 s (load 3.7 to 3.3): 500 tickets at
166 s (BASE 187 s), 1,000 at 414 s (BASE 519 s), killed at 540 s with 1,216 tickets written (BASE
1,014 at 530 s). The second 500 took 248 s against 332 s at BASE. The full run is `NOT_OBSERVED`.
Each ticket costs about 2.2 durable writes, each taking about 0.15 s of wall time on this host
whatever the store size. So a 2,000-ticket gen is about 4,500 writes, more than the 9-minute
per-command bound even if a write cost nothing more as the store grows. `gen` keeps building the
store through the product's own CLI writers; the harness itself is unchanged.

## Tests

- `script/heavy.sh go test -count=1 -run 'TestTCXV084' ./internal/tasks/intent`: ok
  (`TestTCXV0845IndexAdmitsOnlyReencodedBytes`, `TestTCXV0845ReadsTrustOnlyClosedEntries`).
- `script/heavy.sh go test -count=1 -run 'TestTCXV0846|TestLGLV0014|TestLGLV0004' ./internal/tasks/transaction`:
  ok (`TestTCXV0846ModelSkipsOnlyByteIdenticalIndexedTickets`).
- `script/heavy.sh go test -count=1 -run 'TestTCXV084' ./internal/tasks/store`: ok
  (`TestTCXV0846ModelOutputMatchesFullDecode`, `TestTCXV0848JournalByteIdenticalWithIndex`: every
  report, journal, state and intent file byte-identical with and without the index over create,
  prioritize, complete-manual, archive and create).
- `script/heavy.sh go test -count=1 -run 'TestTCXV084|TCXV0_' ./internal/tasks/cli`: ok
  (`TestTCXV0847PreloadMatchesFullLoad` and every existing TCX parity test).
- Whole packages with `HEAVY_WHOLE_REASON`: `internal/tasks/intent` ok (51 s),
  `internal/tasks/transaction` ok (18 s), `internal/tasks/mutation` ok, `internal/tasks/ticket` ok,
  `internal/tasks/scale` ok (28 s), `internal/tasks/cmd/closed-index` ok.
- `internal/tasks/store` whole package exceeded the 8-minute bound (a timeout, not a failure); the
  slices `-run 'TestTMV|Mutate|Writer|TCX'` (108 s) and
  `-run 'Request|Replay|Guard|Archive|Complete|Prioritize|Create'` (165 s) are ok.
- `internal/tasks/cli` slices `-run 'TCX|Mutate|Create|Prioritize|Complete|Archive'` and
  `-run 'Warning|Intent|Override|Conflict|IssuedAt|Unmet|Dependenc'`: ok.
- Gates: `gofmt -l`, `make windows-vet`, `make test-read-scopes-check`, `make line-citations-check`
  (after the repin), `make shell-foreign-callers-check`, `make doc-gates`: pass.

## Hook git-status fix (owner-decision item)

The round-1 hook fix (V1-2240) re-hashes racily clean index entries once per process instead of
twice per hook event. Measured: Git time per hook event fell from about 460 ms to about 290 ms, and
the second isolated read from about 235 ms to about 30 ms on an 11,217-file fresh checkout.

It cannot hold accepted `EAF-V0-007` as written:

- A racy index costs an eighth Git process (`update-index -q --refresh`). The 7-process ceiling is
  pinned by `TestBuildQueryBlobAdmissionAndGitProcessCeiling`.
- The reused private copy carries the time Git gave it when refreshed, not the captured index
  modification time.

So, per the orchestrator, spec commit `67c803435` is reverted in `9678ef33a` and no code is on the
branch. The production diff is kept below for the owner.

The test changes it needs:

- new `internal/gitstatus/refresh_test.go`: three `TestGPKV0845*` tests;
- `settleIndex` in the gitstatus probe-reuse and simple-metadata tests;
- the timestamp test accepts the refreshed copy's own time;
- the genesis sparse-index count becomes `MaxProcesses-3`;
- the `contextindex` fake git lets `update-index` succeed.

```diff
diff --git a/internal/gitstatus/simple.go b/internal/gitstatus/simple.go
index 9436ee5b2..aca32f9fe 100644
--- a/internal/gitstatus/simple.go
+++ b/internal/gitstatus/simple.go
@@ -5,6 +5,7 @@ import (
 	"crypto/sha1"
 	"encoding/binary"
 	"strings"
+	"time"
 )
 
 // simpleConfig recognizes only unquoted, single-line settings in the sections
@@ -168,3 +169,24 @@ func simpleEntryLength(data []byte, version uint32) int {
 	}
 	return length
 }
+
+// racyEntries reports whether a simple index records an entry whose
+// modification second is not before the index file's own: Git, comparing
+// whole seconds, cannot trust such an entry's stat data and re-hashes it on
+// every status until a later index write records it again (GPK-V0-845).
+func racyEntries(data []byte, indexModTime time.Time) bool {
+	if !simpleIndex(data) {
+		return false
+	}
+	version := binary.BigEndian.Uint32(data[4:8])
+	indexSecond := uint32(indexModTime.Unix())
+	remaining := data[12 : len(data)-sha1.Size]
+	for count := binary.BigEndian.Uint32(data[8:12]); count > 0; count-- {
+		length := simpleEntryLength(remaining, version)
+		if binary.BigEndian.Uint32(remaining[8:12]) >= indexSecond {
+			return true
+		}
+		remaining = remaining[length:]
+	}
+	return false
+}
diff --git a/internal/gitstatus/status.go b/internal/gitstatus/status.go
index a2ab7d2d9..19897e659 100644
--- a/internal/gitstatus/status.go
+++ b/internal/gitstatus/status.go
@@ -100,9 +100,10 @@ func (memo probeMemo) remember() {
 
 // MaxProcesses is the most Git processes one Status call starts: a config
 // probe for each of config and config.worktree, the two index probes of
-// validateIndex, and the status run, plus a tree read and private gitlink status
-// when gitlinks are present. A caller's process budget covers it.
-const MaxProcesses = 7
+// validateIndex, the private index refresh of GPK-V0-845 and the status run,
+// plus a tree read and private gitlink status when gitlinks are present. A
+// caller's process budget covers it.
+const MaxProcesses = 8
 
 const metadataLimit = 32 << 20
 const snapshotLimit = 64 << 20
@@ -314,6 +315,7 @@ func statusIn(ctx context.Context, root string, temporaryParents []string, expli
 	simpleConfigs, sha1Config := true, false
 	reuse := probeReuse(ctx)
 	var index []byte
+	var indexModTime time.Time
 	var configs [][]byte
 	capturedBytes := 0
 	for _, file := range pointers {
@@ -330,7 +332,7 @@ func statusIn(ctx context.Context, root string, temporaryParents []string, expli
 		capturedBytes += len(file.data)
 		files = append(files, file)
 		if item.name == "index" {
-			index = file.data
+			index, indexModTime = file.data, file.modTime
 		}
 		if !file.present {
 			continue
@@ -421,6 +423,10 @@ func statusIn(ctx context.Context, root string, temporaryParents []string, expli
 			}
 		}
 	}
+	refreshKey := probeDigest(append(append([][]byte{[]byte("refresh"), []byte(root), []byte(gitdir)}, configs...), index)...)
+	if err := refreshPrivateIndex(ctx, temp, filepath.Join(private, "index"), prefix, index, indexModTime, refreshKey, run); err != nil {
+		return nil, err
+	}
 	if err := reader.unchanged(files); err != nil {
 		return nil, err
 	}
@@ -449,6 +455,75 @@ func statusIn(ctx context.Context, root string, temporaryParents []string, expli
 	return result, nil
 }
 
+// refreshedIndexes holds, in process memory only, the private index copy that
+// update-index refreshed for one captured state, by its digest (GPK-V0-845).
+// Its bound keeps a long-lived process from accumulating copies.
+var (
+	refreshedMu      sync.Mutex
+	refreshedIndexes = map[[sha256.Size]byte]refreshedIndex{}
+)
+
+const refreshedIndexBound = 8
+
+type refreshedIndex struct {
+	data    []byte
+	modTime time.Time
+}
+
+// refreshPrivateIndex replaces the private copy of index at target with the
+// copy refreshed earlier in this process for the same captured state, or,
+// when index records a racily clean entry, refreshes the copy in place so the
+// status run does not re-hash every such entry (GPK-V0-845). The live index is
+// never written. A failed refresh restores the live copy, and only a refreshed
+// copy that is still a simple index is remembered.
+func refreshPrivateIndex(ctx context.Context, temp, target string, prefix []string, index []byte, indexModTime time.Time, key [sha256.Size]byte, run Runner) error {
+	refreshedMu.Lock()
+	known, ok := refreshedIndexes[key]
+	refreshedMu.Unlock()
+	if ok {
+		return writeIndexCopy(target, known.data, known.modTime)
+	}
+	if !racyEntries(index, indexModTime) {
+		return nil
+	}
+	if _, err := run(ctx, temp, 4096, append(append([]string(nil), prefix...), "update-index", "-q", "--refresh")...); err != nil {
+		if ctx.Err() != nil {
+			return ctx.Err()
+		}
+		return writeIndexCopy(target, index, indexModTime)
+	}
+	data, err := os.ReadFile(target)
+	if err != nil {
+		return writeIndexCopy(target, index, indexModTime)
+	}
+	info, err := os.Stat(target)
+	if err != nil || !simpleIndex(data) {
+		return writeIndexCopy(target, index, indexModTime)
+	}
+	refreshedMu.Lock()
+	if len(refreshedIndexes) >= refreshedIndexBound {
+		clear(refreshedIndexes)
+	}
+	refreshedIndexes[key] = refreshedIndex{data: data, modTime: info.ModTime()}
+	refreshedMu.Unlock()
+	return nil
+}
+
+// writeIndexCopy writes data as the private index with modTime, which Git's
+// racily-clean check reads.
+func writeIndexCopy(target string, data []byte, modTime time.Time) error {
+	if err := os.WriteFile(target, data, 0o600); err != nil {
+		return copyWriteRefusal("index", err)
+	}
+	if err := os.Chtimes(target, modTime, modTime); err != nil {
+		return errIndexTime
+	}
+	if info, err := os.Stat(target); err != nil || !info.ModTime().Equal(modTime) {
+		return errIndexTime
+	}
+	return nil
+}
+
 func validateIndex(ctx context.Context, temp string, prefix []string, run Runner) (indexEntries, error) {
 	// The shared index is absent from private metadata, so a reference to one
 	// fails closed before any worktree or submodule traversal.
```

## Friction (round 2)

1. Ticket records for msc-u34 were never staged in the lane directory.
   - Repro: `ls ~/lanes | grep msc` on unraid-lanes.
   - Expected: the staged ticket records the charter names.
   - Actual: only other lanes' charters and ticket files; the round ran from the orchestrator's
     message alone.
2. `git log --all -S` for a 10-ID free check ran past the 2-minute tool limit.
   - Repro: `for i in $(seq 845 854); do git log --all --oneline -S "TCX-V0-$i"; done`.
   - Expected: `make requirement-ids-free` also covers `-S`, or the check finishes in seconds.
   - Actual: the harness moved the command to the background automatically. That is against the
     lane rule against background jobs, though it was not started as one.
3. A killed `tasks-scale gen` cannot be relocated by default.
   - Repro: kill `gen` mid-run, `cp -a` the store, then `tasks-scale relocate --store COPY`.
   - Expected: relocate from the root head.json pins.
   - Actual: it fails because `.scale-manifest.json` is written only at the end;
     `--from OLD_ROOT` is required.
4. `tasks-scale gen --tickets 2000` cannot finish inside the lane's 9-minute per-command bound.
   - Repro: `tasks-scale gen --root S --tickets 2000` on this host.
   - Expected: the ticket's measurement can be taken in one command.
   - Actual: about 4,500 durable writes at about 0.15 s each. The wall time is
     `NOT_OBSERVED` both before and after, and the 2,000-ticket store was finished with plain creates.
5. `make line-citations-check` checks committed or staged content, not the worktree.
   - Repro: edit a citation in a spec, run the check before `git add`.
   - Expected: the check sees the edit, or its message says which tree it read.
   - Actual: it reported the old citation until the edit was committed, and the message did not
     say why.
6. The whole `internal/tasks/store` package exceeds an 8-minute command bound.
   - Repro: `HEAVY_WHOLE_REASON=... script/heavy.sh go test -count=1 -timeout 8m -run '.' ./internal/tasks/store`.
   - Expected: a whole-package run the lane rules permit.
   - Actual: it timed out at 480 s and had to be split by `-run` pattern.
7. A request-ID conflict refusal costs 4 to 6 times a successful create.
   - Repro: `tasks-scale run --store S -- ticket create --request-id m2 ...` on a gen store, where
     `m2` was used by gen's complete-manual.
   - Expected: a cheap refusal.
   - Actual: 820 to 850 ms CPU and 310 MB, against about 65 ms and 28 MB for the successful
     create. This is a new hot spot outside these blocks.
8. `TestDCPV1281AdditionalRepositoryWarmCallsAtScale` fails under load spikes.
   - Repro: run it at load about 12 on unraid-lanes.
   - Expected: the warm p95 bound measures the warm path.
   - Actual: the test takes p95 of 20 samples as the 20th sorted sample, that is the maximum. So
     one pairing call's Git observation (1,304 ms at load 12, about 16 ms when quiet) failed the
     1 s bound once in round 1. BASE and lane both take exactly one such observation per run, so
     the flake predates this lane.

## Round 3 (orchestrator decision 2026-11-02)

Declined: a stat-trusted preload, because it would skip the byte-exact verification. To build:

- an incremental index (`TCX-V0-849`);
- one intent capture shared by the preload and the writer audit (`LGL-V0-845`), through the
  journal reader's existing `IntentTree` seam only;
- V1-4263, a request-ID conflict refused before the complete route (`LGL-V0-846`).

The `TCX-V0-848` target is unchanged; changing it is the owner's call.

### Round 3 before numbers

The conflict refusal: `ticket create --request-id m2` on a gen-built 500-ticket store, where `m2`
is gen's own complete-manual request. Six interleaved samples per side, load 1.97 to 2.05:

| Binary | wall p50 | CPU p50 | allocated p50 |
|---|---|---|---|
| BASE | 490.2 ms | 1,166 ms | 324.9 MB |
| round-2 head `d6e82da66` | 458.7 ms | 1,123 ms | 309.9 MB |

The output differs between samples only in the defaulted `issuedAt` (the current second) named in
the warning.

A stage trace of the round-2 head shows where the time goes:

- The writer route declines: `requests/...json: checkpoint unusable: request is not within the
  bound before the checkpoint`.
- The complete route then runs the full `AuditForMutation`.
- The CLI runs a second full audit in `RequestIndex.Lookup` to explain the conflict.

The three-pass numbers before this round are round 2's (17.5 ms at 500, 61 ms at 2,000).

### Round 3 design and seam edits

- `LGL-V0-846` (V1-4263): `journal.Reader.WriterReplay` now wraps a shared `writerRequest` helper
  that takes the walk bound. `WriterConflictRequest` calls the same helper with no bound. The
  journal lines touched, all in `internal/tasks/journal/writer_checkpoint.go`:
  - the `WriterReplay` body (now a one-line wrapper);
  - the new `WriterConflictRequest` and the `writerRequest` signature;
  - the one bound comparison (`last-seq > MaxWriterTail` becomes `last-seq > maxWalk`).

  No check was removed. `observeWriter` tries the unbounded binding only after the bounded one
  declines. `mutateWriter` refuses only on a differing digest, through the same `replayResult`
  model call the complete route uses. A same-digest replay declines with the original bounded
  decline text, so `TestCALV0190_WriterReplayBeyondBoundDeclines` is unchanged. The CLI's
  explanation takes the bound entry and ticket from `Report.ConflictBinding()`, an unexported
  field that never reaches output.
- `LGL-V0-845`: the CLI hands `store0.Tree` to `Mutate` (`store.WithPreloadTree`), and
  `observeWriter` sets the journal reader's existing `IntentTree` input. The journal side is
  unchanged.
  - Wiring only this regressed the writer route. The audit's physical observation lists only the
    files it read natively, so `writerInventory` re-read every shared ticket (2,000 tickets: 1.0 s
    against 0.11 s over 8 creates).
  - Fixed in the store: `writerInventory` takes an intent file the audit did not read from the
    preload at the preload's digest (`preloadObserved`). The capture reads every intent file it
    does not share, so an absent file was shared at exactly that digest.
- `TCX-V0-849`:
  - Index lines are kept per entry and reused for kept entries.
  - An unchanged body leaves the file in place.
  - `ReadClosedIndex` reuses a parse while the file's device, inode, size, modification and change
    times match.
  - The rewrite decodes and checks only the files it read (`decodeTicketFile`, factored from the
    load's own per-ticket checks), without `LoadTreeIndexed`'s inventory build.

### Round 3 after numbers

All measured on the same host and interleaved. Each binary ran on its own relocated copy of the
same 500- and 2,000-ticket stores, built by the round-2 gen and grown with plain creates.

`ticket create`, n=7 per cell, load 2.86 to 3.20:

| Binary | 500 wall / CPU / alloc | 2,000 wall / CPU / alloc |
|---|---|---|
| BASE | 177.1 ms / 192.5 ms / 79.6 MB | 379.7 ms / 544.1 ms / 264.9 MB |
| round 2 | 143.0 ms / 94.6 ms / 31.1 MB | 240.2 ms / 219.5 ms / 78.1 MB |
| round 3 | 134.6 ms / 81.9 ms / 27.8 MB | 210.2 ms / 174.5 ms / 66.2 MB |

CPU per create of the passes, from 12 profiled creates per cell at load 2.3:

| Pass | r2 500 | r2 2,000 | r3 500 | r3 2,000 |
|---|---|---|---|---|
| preload (byte-exact read and hash of every file) | 8.3 ms | 28.3 ms | 5.8 ms | 27.5 ms |
| model decode | 0.8 ms | 5.0 ms | 0 ms | 1.7 ms |
| index rewrite | 2.5 ms | 15.8 ms | 4.2 ms | 5.8 ms |
| index reads | under 1 ms | 11.7 ms | 2.5 ms | 4.2 ms |
| total, the three passes | about 12 ms | 61 ms | 12.5 ms | 39 ms |
| writer journal audit (tkp-u33-owned) | 31.7 ms | 60.0 ms | 15.8 ms | 35.8 ms |
| pre-effect recheck (`bindObservation`) | 5.0 ms | 23.3 ms | 9.2 ms | 26.7 ms |

- `TCX-V0-848` is **still not met.** The three passes grow from 12.5 ms to 39 ms, 3.1 times
  against under 1.2 times; the target was not changed. The measured floor is the preload's
  byte-exact read and hash of every intent file: 27.5 of the 39 ms at 2,000 tickets (70%), about
  14 µs per ticket file. The rest is about 10 ms of index work and 1.7 ms of model decode.
- `TCX-V0-849` is met: index rewrite plus reads take 10 ms per create at 2,000 tickets (target 13
  ms; round 2 took 27.5 ms).
- `LGL-V0-845` is met. At 2,000 tickets the capture's intent reads (`requiredRead` and its
  hashing) fell from 0.51 s to 0.02 s (`sharedIntent`) over 12 creates, -96% (target -80%). The
  whole writer audit fell from 60.0 to 35.8 ms per create.
- `LGL-V0-846` (V1-4263) is met. The conflict refusal at 500 tickets, n=6 interleaved, load 2.71
  to 2.86:

  | Binary | wall p50 | CPU p50 | allocated p50 |
  |---|---|---|---|
  | BASE | 420.1 ms | 979.0 ms | 324.9 MB |
  | round 2 | 419.1 ms | 969.3 ms | 309.9 MB |
  | round 3 | 148.9 ms | 176.7 ms | 84.5 MB |

  That is 18% of the base CPU (target at most 25%).

### Round 3 tests

All ran through `script/heavy.sh`; the store and CLI runs used `GOCACHE=~/.cache/go-build` and
`TMPDIR` under the lane scratch after friction 9.

- New and ok:
  - `TestLGLV0846ConflictRefusedFromCheckpoint` (store): writer route, no publish, the complete
    route's decision, and a binding equal to `RequestIndex.Lookup`.
  - `TestLGLV0846ConflictOutputMatchesCompleteRoute` (cli): result, codes and warnings
    byte-identical to the complete route, apart from the defaulted second.
  - `TestLGLV0845WriterTakesPreloadBytes` and `TestLGLV0845ChangedAfterPreloadRefused` (store).
  - `TestTCXV0849IncrementalRewriteMatchesFullEncode`, `TestTCXV0849UnchangedIndexNotRewritten`
    and `TestTCXV0849ParsedIndexReusedPerVersion` (intent).
- Existing and ok:
  - every `TestTCXV084*`, the `TCXV0_` tests and the `CAL-V0-115/116/190` writer-route tests;
  - whole packages `intent`, `transaction`, `mutation`, `ticket`, `scale` and `cmd/closed-index`;
  - `journal -run 'Writer|Replay|Checkpoint|Conflict|Request|Index'`;
  - store slices `TestTMV|Mutate|TCX|LGL` (137 s), `Writer|CAL` (352 s) and
    `Request|Replay|Guard|Archive|Complete|Prioritize|Create|Conflict` (174 s);
  - the two cli slices.

### Round 3 friction

9. The lane host's root filesystem filled during the store tests.
   - Repro: `script/heavy.sh go test ... ./internal/tasks/store` with the default `TMPDIR` on
     unraid-lanes while `/tmp` held 36 GB (28 GB of it the shared `/tmp/corvint-go-build-cache`).
   - Expected: test scratch with room, or a refusal that names the full filesystem.
   - Actual: about 20 store tests failed with `no space left on device` (586 MB free). The
     orchestrator cleared the shared cache mid-run, and the rerun used `GOCACHE=~/.cache/go-build`
     and `TMPDIR` under the lane scratch.
10. A cold build cache pushes store test slices past the 8-minute bound.
    - Repro: the same slice right after the shared cache was deleted.
    - Expected: the slice's usual 108 s.
    - Actual: a timeout at 480 s. Two smaller slices together then ran past the tool's 10-minute
      limit and were moved to the background automatically.
11. `make windows-vet` puts its build cache on the root filesystem, whatever the environment says.
    - Repro: `GOCACHE=~/.cache/go-build make windows-vet` on unraid-lanes.
    - Expected: the Go cache the environment names.
    - Actual: the Makefile sets `GOCACHE=/tmp/corvint-go-build-cache` itself, and recreated the
      cache the orchestrator had just removed to free the full root filesystem.

## Codex r1 fixes (two P2 findings)

1. `TCX-V0-849`, `internal/tasks/intent/closed_index.go`. The parse cache trusted stat fields, so
   a same-length in-place rewrite with its mtime restored could be served from the old parse on a
   coarse-ctime filesystem, even when the replacement was invalid.
   - Fix: `ReadClosedIndex` reads the file on every call and reuses a parse only when the bytes'
     SHA-256 equals the bytes that parse decoded. The stat-version comparison and the
     `closed_index_ctime_*.go` files are removed.
   - Test: `TestTCXV0849SameStatRewriteReparsed` (same size, same mtime, same file, one body byte
     changed: refused).
   - Mutation check: with the digest ignored, the test fails ("served from the earlier parse").
   - Cost: index reads at 2,000 tickets were not re-measured after this fix. Each read now also
     reads and hashes the index file, which is under 1 MB at 2,000 tickets.
2. `LGL-V0-846`, `internal/tasks/store/writer_route.go`. The writer-route conflict shortcut
   answered `REQUEST_ID_CONFLICT` before refusals that won at BASE. One example: a malformed file in
   an existing `requests/<shard>/`, which the complete audit's shard scan refuses first.
   - Fix: the shortcut is removed. `internal/tasks/journal/writer_checkpoint.go` is back to its BASE
     bytes; no journal line remains changed. A retained conflict beyond the tail declines and the
     complete route refuses it, exactly as at BASE.
   - The complete route now hands its audit's bound entry and ticket to the CLI
     (`Report.ConflictBinding`), so the CLI's explanation no longer runs a second full audit.
   - Test: `TestLGLV0846MalformedShardRefusedAsBase` gives the same refusal, and the same error
     text, as the complete-route oracle (`MALFORMED: requests/61/zz-not-a-request.json: forbidden
     receipt destination`).
   - Mutation check: with the shortcut restored from `ef8f31b83`, the test fails with
     `REQUEST_ID_CONFLICT`.
   - `TestLGLV0846ConflictRefusedFromCheckpoint` now asserts the BASE decline text, a conflict, no
     publish, a binding equal to `RequestIndex.Lookup`, and no binding on a replay.

Re-measured after fix 2: the conflict refusal at 500 tickets, n=6 interleaved, load 4.70 to 4.84.

| Binary | wall p50 | CPU p50 | allocated p50 |
|---|---|---|---|
| BASE | 469.8 ms | 1,113.9 ms | 324.9 MB |
| head after fix 2 | 283.9 ms | 572.9 ms | 162.9 MB |

`LGL-V0-846` is **no longer met**: the refusal costs 51% of the base CPU against a target of 25%
or less. The one full audit that remains is what keeps BASE's refusal precedence. Only the CLI's
second audit was removed.

Tests after both fixes, all ok:

- whole `internal/tasks/intent` (55 s);
- `store -run 'Request|Replay|Conflict|LGLV|TCXV|CALV0190|CALV0116'` (159 s);
- `cli -run 'TCX|Mutate|Create|Conflict|IssuedAt|Warning|LGL'`;
- `journal -run 'Writer|Replay|Request|Index'`.

### Per-ticket delivery

- V1-2443: FULL. DCR-V1-845 and 846, with criteria 2 and 3 met by measurement. The Windows-host
  cost is `NOT_OBSERVED`.
- V1-2240: FULL as a measured rejection: the untracked cache gave no win. The real cause found
  (racily clean entries) is an owner-decision item (EAF-V0-007), not delivered.
- V1-3717: FULL, already delivered on origin/main at `1d95a4beb`.
- V1-2745: FULL, already delivered at `1763e5060` (lane dck-u24).
- V1-4069: FULL, already delivered at `63551c530` (lane spt-u19).
- V1-4229: PARTIAL.
  - TCX-V0-845, 846, 847 and 849 are delivered.
  - TCX-V0-848 is not met: 3.1 times growth against under 1.2 times. The floor is the preload's
    byte-exact read.
  - LGL-V0-845 is met.
- V1-1148: PARTIAL. Every per-ticket write pass is cheaper, but `gen --tickets 2000` still cannot
  finish inside the lane's 9-minute bound, and its wall time is `NOT_OBSERVED`.
- V1-4263: PARTIAL. The refusal is byte-identical and 49% cheaper, but LGL-V0-846's 25% target is
  not met after the review fix.
