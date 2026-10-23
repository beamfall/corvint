# Read hot spots: scope matching and stat-routed revision pins for GitHub #1610

Date: 2026-10-23. Lane: opencode/read-hotspots-gh1610, based on PR #1609's head (9708bc5dd) with
origin/main (77a49dfa2) merged in for the `CORVINT_CPUPROFILE` knob of #1608. Requirements
`DCP-V1-450..453` in `docs/specs/documentation-corpus-v1.md` are proposed and experimental.

## Causes

- `doccorpus.inScope` built `scope + "/"` on every call. It ran inside scope × path loops: the
  compiler's scope check over every tracked and skipped path, live freshness over every live
  path, and manifest admission over every input. A manifest of one scope per file makes each loop
  quadratic.
- A corpus build pins every revision it names through `contextindex.BuildRevisionContext`, twice
  (before and after compiling). Each pass opened, read and hashed every tracked file of that
  checkout. On the measurement host the cost was the leaf `openat` (12.1 s of the 12.5 s in
  `pinWorktree`). `gitBlobHash` did not appear in the profile, so skipping only the hash for
  stat-matching files, as the issue first suggested, would have saved nothing measurable.
- Since #1609, `liveFreshness` no longer pins anything (`ObserveTree`). The pin cost now falls on
  `build`, `validate`, `--verify-rederive` and an unattested builder's reads.

## Decisions

- `DCP-V1-450`: `inScope` allocates nothing. Each loop instead walks the path's ancestors
  through a per-revision scope set (`scopeSet.each`/`holds`), the prefix-walk option the issue
  named. The walk visits exactly the scopes the pairwise comparison admitted, once each. The
  compiler keeps its refusal precedence by tallying first and then checking scopes in manifest
  order. Live freshness notes a path once per holding scope, and its reasons are sorted by the
  caller as before. The inventory's nested-scope refusal still names the byte-least outer scope,
  then the byte-least inner one. `TestDCPV1450ScopeMatchingMatchesPairwise` compares all of this
  with the old pairwise loops over 200 random scope sets, using path bytes on both sides of
  `/` ("a", "a-b", "a.b", "a/b").
- `DCP-V1-452`: the Git index is used to route each entry to a source, never to trust a worktree
  file. When an entry's index record is stage 0 and unflagged, and its lstat matches the record
  (size, mtime, ctime, inode, uid, gid, executable bit) and is not racy (mtime strictly older than
  the index file's), Git vouches that the file holds the indexed blob. Opening that file could
  at best repeat a committed object. So the entry is pinned from the tree entry's own committed
  blob in one batched `cat-file`, which is immutable content. Every other eligible entry takes
  the unchanged open-and-hash pass. As a result, worktree bytes are still used only when they
  hash to the pinned blob, and the stat-trust risk the issue raised (clean/smudge filters, CRLF,
  `ident` or LFS making a stat-clean file differ from its blob) cannot reach a pin.
- Routing applies only to `BuildRevisionContext`, via a context mark. `Build`, `BuildContext` and
  `BuildEval` keep their accepted design, in which a clean repository reads no committed blob
  (`TestBuildEvalFetchesNoBlobForACleanRepository`). Their worktree cost was not part of this
  issue.
- `DCP-V1-453`: routing fails closed. The following vouch for nothing: an unreadable, missing,
  symlinked, oversized, truncated or checksum-failing index; version other than 2, 3 or 4; a
  split index (`link`); an unknown required extension; or a platform without lstat inode and
  ctime (`index_stat_other.go`). A failed or incomplete blob batch returns its entries to the
  hashed pass. An all-zero `index.skipHash` trailer is parsed under the same bounds rather than
  refused, because vouching only chooses where the tree blob is read from.
- Independent review (one read-only reviewer pass) found no HIGH or MED issue. Its LOW items were
  repaired: the spec and comment now say that vouching is a choice of source only; skipHash
  indexes are accepted; tests were added for refusal precedence in both orders, for the
  two-nested-scope reason count, and for the parser (a fuzz test).
- Not done: a cross-pass memo of pinned bytes. The issue's "reuse between Build and
  liveFreshness" has no second consumer since #1609, and after routing the build's repeated pin
  pass costs one batched object read. No memory is retained across passes.

## Measurements

Fixture: `CORVINT_READ_FIXTURE_DIR=… CORVINT_READ_FIXTURE_SCOPES=many go test -run
TestWriteLargeReadFixture ./internal/doccorpus`. It has 4,000 native pages plus 6,000 sources in
the primary repository, and four additional repositories of 1,500 pinned files each. That is
10,001 inputs and 10,001 scopes, one per input, in a 67.6 MB artifact. Each binary was built from
a clean commit, so its builder is attested. Each rebuilt `corpus.json`, then ran `docs corpus
info` (verified read) and `docs corpus validate` (rederivation) with `CORVINT_CPUPROFILE`.
Host: darwin/arm64, 12 CPUs, go1.27.3. Before is bb4d9790d; after is 7b48f3c0f.

| Read | Metric | Before | After |
|---|---|---|---|
| `info` (verified) | wall | 1.84 s | 0.35–0.43 s |
| | CPU samples | 2.54 s | 0.77 s |
| | `inScope` (cum) | 0.90 s | not sampled |
| | `runtime.concatstring2` (cum) | 0.75 s | not sampled |
| | `validateManifest` (cum) | 1.00 s | 0.04 s |
| | `liveRepository` (cum) | 0.96 s | 0.06 s |
| | peak RSS | 237 MB | 226 MB |
| `validate` (rederive) | wall | 7.98 s | 3.74–3.89 s |
| | CPU samples | 18.18 s | 2.53 s |
| | `pinWorktree` (cum) | 12.55 s | 0.18 s (0.12 s of it `os.Lstat`) |
| | `worktreePin` → `openat` (cum) | 12.54 s | not sampled (no worktree file opened) |
| | `(*compiler).pin` (cum) | 2.06 s | 0.39 s |
| | `inScope` (cum) | 1.81 s | not sampled |
| | peak RSS | 644 MB | 597–690 MB (run to run) |

Byte identity: both binaries built artifacts whose records are identical, with only `builder` and
`sha256` excluded. `search --query ledger` receipts differ only in the builder revision and the
digests derived from it.

Benchmarks, at 2,400 single-file scopes, 4,101 tracked paths and 5 iterations, before and after on
the same host:

| Benchmark | Before | After |
|---|---|---|
| `BenchmarkDCPV1451ManyScopesVerifiedRead` | 388 ms/op | 207 ms/op |
| `BenchmarkDCPV1451ManyScopesRederivedRead` | 3.78 s/op | 2.73 s/op |

## Verification

- `go test -count=1 ./internal/contextindex/ ./internal/doccorpus/`: PASS after the analyzer
  input pin was repinned by `script/analyzer-schema.sh bump`. The output digest was unchanged, so
  the schema number stays `corvint-analyzer/147`.
- New tests: `TestDCPV1450ScopeMatchingMatchesPairwise`,
  `TestDCPV1450NestedScopesKeepRefusalsAndReasons`, `TestDCPV1450NewLiveFileNotedPerHoldingScope`,
  `TestDCPV1451ManyScopesReadMatchesRederivation`, `TestDCPV1452GitIndexVouchesUnchangedEntries`,
  `TestDCPV1452RacyEntryIsNotVouched`, `TestDCPV1452IndexVersionsAndFlags`,
  `TestDCPV1453UnusableIndexVouchesNothing`, `FuzzParseGitIndex` (also fuzzed 15 s, about 1.05M
  executions, no failure), `TestDCPV1453FailedBlobBatchFallsBackToWorktree`,
  `TestDCPV1453RoutedPinMatchesParityRepository`.

## Limits

- Measured on macOS only. On Linux, where `openat` is cheaper, a batched object read may cost
  about as much as the worktree pass it replaces. The result is identical either way.
- On a checkout with a loose (unpacked) object store, the batch opens loose objects inside Git
  instead of worktree files.
- A build still runs its second pin pass, and `BuildRevisionContext` still compiles the full
  revision index each time. That is the remaining rederivation cost, and #1598 owns it.
