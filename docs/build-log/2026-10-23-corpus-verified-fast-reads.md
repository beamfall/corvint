# Verified fast documentation-corpus reads for GitHub #1598

Date: 2026-10-23. Lane: opencode/corpus-read-reuse-gh1598 (base dd0d09db5). Ticket: V1-2471.
Requirements `DCP-V1-410..418` in `docs/specs/documentation-corpus-v1.md` are proposed and
experimental. They amend `DCP-V1-011` (a read verifies instead of rederiving) and `DCP-V1-143`
(the corpus MCP first call).

## Causes

- `doccorpus.Open` ran `ParseArtifact`, a full `Build` from the pinned inputs, `Encode` and a byte
  comparison on every read. Every read was therefore a compile.
- `liveFreshness` then ran `contextindex.BuildContext` once for the primary checkout and once for
  each additional repository, one after another. Each run read and indexed every tracked file.
- `ls-tree -l`, the listing behind that index, also opened every blob's object header to get its
  size.

## Decisions

- `DCP-V1-410`: a read now checks the canonical bytes and the content digest. It checks that the
  builder is the running one and that the artifact and manifest schemas pair. It recomputes the
  manifest and profile digests. It checks that every pinned revision resolves in the checkout that
  owns it, with the primary revision resolving to the recorded tree. Then it decodes. Every failure
  refuses. Full rederivation stays in `build`, in `validate`, in `--verify-rederive` on any read
  and in MCP `docs_validate` (`DCP-V1-413`). `info` names the path it took in its `validation`
  field: `artifact-verified` or `source-rederived`. The digest checks integrity, not origin. A
  record edit whose digest was deliberately recomputed passes the verified read; only rederivation
  refuses it. `TestDCPV1410VerifiedReadRefusals` pins both outcomes.
- Builder or manifest mismatch refuses outright, with no fallback to rederivation: `Build` stamps
  the running builder, so a rederivation could never reproduce those bytes.
- `DCP-V1-411`: freshness uses Git's identity, its stat-cached status and one `ls-tree` without
  sizes (`contextindex.ObserveTree`), with every checkout observed in parallel. Git's index is the
  per-input stat stamp the orchestrator asked for: it records path, size, mtime, inode and blob,
  and status re-hashes only entries whose stat changed. So no caller-named sidecar is needed and
  a read writes nothing. The CLI starts this observation from the header members (schema,
  builder, manifest, digests, tree) while the records are still being decoded.
- `DCP-V1-412`: symbol re-resolution builds the full live index only when a cited Ruby or
  TypeScript/JavaScript anchor's blob changed or its path is dirty. It refuses an index built from
  a state other than the one the overlay observed. In that case it withholds the states rather
  than mixing states.
- `DCP-V1-414`: one jsontext scan validates the input. Record-array elements then decode and
  compare against their own canonical encoding in parallel. The remainder is compared as a
  skeleton, the artifact with those arrays empty. The digest is hashed from the input bytes. For
  rederivation, the stopped run's streaming comparison (`rederivedEqual`) was kept.
- `DCP-V1-417`: a warm MCP call no longer reads and hashes the whole artifact while the artifact
  file's stat stamp is unchanged. The `DCP-V1-314` racy window still forces a read.
- Not done: no daemon or `--serve` (invariant 7). No persisted index or sidecar, because the cold
  target was met without one. `CORVINT_CPUPROFILE` plumbing is left to #1600. Search ranking is
  left to #1597 and `delta` paging to #1599. `delta` and `carry-forward` still rederive their
  current side.

## Measurements

The fixture comes from `TestWriteLargeReadFixture` (`CORVINT_READ_FIXTURE_DIR`): a 4,000-page
native docs scope (6 sections each), 6,000 Go files outside the scope, and four additional
repositories of 1,500 pinned files. That is 10,001 inputs and a 66.75 MB artifact, with every
repository packed. The host is a 10-core darwin/arm64 machine at load average 15 to 24 from other
lanes (8 to 24 across two rounds). Each CLI run is a new process with a warm OS page cache. Before means origin/main dd0d09db5
with each artifact built by its own binary.

| CLI read | Before wall / peak RSS | After wall / peak RSS (two rounds of 3 runs) |
| --- | --- | --- |
| `info` | 3.88 s / 715 MB | 0.17-0.22 s / 189-221 MB |
| `search --query ledger --limit 20` | 4.20 s / 714 MB | 0.23-0.28 s / 221-224 MB |
| `get --id adapter:lib0-1` | 3.36 s / 742 MB | 0.17-0.19 s / 189-221 MB |
| `flow --id adapter:lib0-1` | 3.61 s / 640 MB | 0.40-0.42 s / 230-267 MB |
| `coverage` | 3.39 s / 739 MB | 0.18-0.20 s / 189-222 MB |
| `locate --path docs/area01/feature00001.md` | 3.57 s / 675 MB | 0.18-0.20 s / 189-221 MB |
| `gaps` | 3.91 s / 754 MB | 0.40-0.43 s / 228-269 MB |

With the same fixture repositories still loose (not packed), the before reads took 9.7 to 10.5 s
and the after reads 0.4 to 0.75 s. `--timings` for a typical after read gave parse 170 to 290 ms
(scan, parallel decode and canonical comparison), freshness 65 to 130 ms overlapped with the
parse, and index 245 to 300 ms for the indexed operations (`flow`, `gaps`).

Corpus MCP results, from `TestDCPV1416CorpusMCPFirstAndWarmCalls` with
`CORVINT_READ_FIXTURE_LARGE=1` on the same fixture shape:
- The first `docs_search` took 382 ms in total: load 125 ms, freshness 147 ms, query 61 ms.
  The rederivation it replaces is the CLI before column above (3.4 to 4.2 s).
- Warm `docs_get` and `docs_info` took 20 to 23 ms once the fixture was past the 2 s racy window.
  Warm `docs_search` took 76 to 78 ms, of which 56 to 58 ms was the query. Calls inside the racy
  window ran Git and took up to 220 ms.

At the 15 MB test scale, `TestDCPV1418LargeFixtureVerifiedReadIsFast` logged 1.66 s for a
rederived read and 132 to 137 ms for a verified read.

## Against the targets

- CLI cold under 1 s: met for every row, at 0.17 to 0.43 s.
- CLI warm under 200 ms: the CLI has no separate warm state; every process takes the path above.
  `info`, `get`, `coverage` and `locate` are at the target (0.17 to 0.20 s). `search` is 0.23 to
  0.28 s. `flow` and `gaps` are about 0.41 s because the query index is rebuilt in each process.
  This is not met.
- Memory: 189 to 269 MB for a 66.75 MB artifact, which is 2.8 to 4.0 times the artifact size.
  That is down from 640 to 754 MB, but not close to the artifact size.
- MCP first call under 1 s: met (382 ms). MCP warm under 100 ms: met (20 to 78 ms in steady state).

## Repair round (review of PR #1609)

Three independent review findings, each fixed fail-closed with a regression test that fails
without the fix:

- Freshness could pair a listing and identity of one commit with the status of another
  (`DCP-V1-411`). `ObserveTree` now reads the identity, then the status while the identified tree
  is listed, then the identity again, and keeps the status only when both identity reads agree;
  otherwise it observes again, at most three times, then refuses with `repository HEAD changed`.
  `TestDCPV1411ObserveTreeRetriesWhenHeadMoves` commits inside the window through a test hook.
- A warm MCP call trusted the artifact's stat stamp from `os.Lstat` on the joined path, which
  follows symlinks in parent components (`DCP-V1-417`). The stamp now comes from
  `doccorpus.StatCorpusFile`, the artifact reader's own no-follow walk without the read, so a
  symlinked parent leaves no stamp and the call reads and is refused as a fresh read is.
  `TestDCPV1417WarmStampRefusesParentSymlink`.
- An equal `<revision>+dirty` or `unrecorded` builder label was taken as rebuild equivalence
  (`DCP-V1-410`). Such labels are shared by builds of different source and the artifact records no
  executable content identity, so a running builder with such a label now always rederives
  (`source-rederived`). Test binaries are always `unrecorded`; tests that exercise the verified
  read opt in through `TrustUnattestedBuilderForTest`, and
  `TestDCPV1410UnattestedBuilderRederives` turns it off. Consequence: a development binary built
  from a dirty tree pays the full rederivation on every read; release and clean builds do not.

Faster warm reads (`DCP-V1-419`), profiled with `--timings` and a CPU profile on the fixture
below: the query index cost 215 to 230 ms per indexed read, mostly term tokenization and
per-posting map work, and unindexed `search` spent 52 to 58 ms tokenizing every subject.

- The index's record maps and term maps are built concurrently, claims are tokenized in blocks on
  worker goroutines and joined in order, and the posting and key-byte totals come from the
  finished maps. This path answers only when no refusal is possible; otherwise the unchanged
  serial derivation runs, so refusals keep their code, message and observed count.
  `TestDCPV1419FastIndexMatchesReference` compares both derivations on four corpora;
  `TestDCPV1419FastIndexDeclinesEveryRefusal` covers each refusal and both bounds.
- Unindexed `search` tokenizes subjects in blocks on worker goroutines.
- The artifact scan finds member and element boundaries without validating them (every byte is
  still checked by the element decode or the skeleton comparison), and elements are decoded
  while the scan continues. `TestDCPV1419BoundaryScanFindsStringContent`.
- The CLI derives the index while live freshness finishes instead of after it.
- The bounded file reader sizes its buffer from the opened file instead of growing it.

Measured on the same fixture and host (10-core darwin/arm64, load average 5.5 to 9.9 from other
lanes), three CLI processes per row, each artifact built by its own clean (`vcs.modified=false`)
binary. Before is 52ae391ff; after is this change, built at 485789a02 before this entry was added. Wall time is the zsh `time` of the process,
including about 25 ms of process start (22 ms of package `init` outside this change):

| CLI read | Before wall (median, range) | After wall (median, range) | Peak RSS before / after |
| --- | --- | --- | --- |
| `info` | 193 ms (171-245) | 172 ms (171-180) | 193-220 / 209-210 MB |
| `search --query ledger --limit 20` | 233 ms (231-243) | 191 ms (184-193) | 227-229 / 236-237 MB |
| `get --id adapter:lib0-1` | 175 ms (174-191) | 179 ms (163-181) | 192-221 / 209-210 MB |
| `flow --id adapter:lib0-1` | 403 ms (389-420) | 217 ms (210-227) | 229-266 / 248-250 MB |
| `coverage` | 191 ms (174-201) | 171 ms (168-178) | 193-222 / 205-210 MB |
| `locate --path docs/area01/feature00001.md` | 199 ms (197-210) | 176 ms (174-196) | 194 / 207-209 MB |
| `gaps` | 417 ms (411-421) | 226 ms (205-233) | 230-269 / 248-264 MB |

`--timings` after: parse 118 to 150 ms (read, scan, parallel decode and canonical comparison),
index 45 to 85 ms (was 213 to 230), `search` query 16 to 17 ms (was 52 to 58), and freshness 122
to 183 ms in parallel (was 75 to 117; the closing identity read and the no longer concurrent
status read add one Git process each per checkout).

Against the 200 ms warm target: `info`, `search`, `get`, `coverage` and `locate` now meet it on
this fixture. `flow` and `gaps` do not: 205 to 233 ms. Their remaining critical path is the
artifact decode and canonical verification (about 120 to 140 ms in a new process) followed by the
query index (45 to 85 ms under this load); live freshness, which spawns Git in five checkouts,
runs in parallel and finishes at about the same time. Memory is unchanged in kind: 205 to 264 MB
for a 66.75 MB artifact. Within this round's limits (no daemon, no persisted index or sidecar, no
artifact format change) the next cut would have to remove decode or freshness work rather than
index work. The corpus MCP first and warm calls were not re-measured at the large scale.

## Verification

Tests: the whole `internal/doccorpus` package, `internal/mcp/corpusbridge`,
`cmd/corvint-corpus-mcp`, `cmd/corvint-corpus-http`, `TestDCPV1411` in `internal/contextindex`, and
the `cmd/corvint` docs-corpus tests selected with `-run`. Results are listed in the pull request.
