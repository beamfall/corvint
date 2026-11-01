# Lane indexcli-r: context sufficiency, orientation ranking and docs-corpus tickets (2026-10-31)

Lane `claude/lane-indexcli-r-1101`, base fda7eb77fc50b93a3b4da40f389ffb11c9da1c16 (the indexcli-w5
lane head). Requirement-ID block 975-979. Host: WSL2 linux/amd64, 32 CPUs, go1.27.3; load average
28-58 throughout, so every timing below is under contention.

## V1-3457: a reserved spec row carries its sufficiency anchor (`TCP-V0-975`)

Decisions:

- Of the two rules the ticket offers, the reserved row satisfies the anchor; it does not get a span.
  An instruction reservation (`governing`, `spec-mentioned`, `instruction-routed`) already hands the
  agent the whole pinned document as project authority, and TCP-V0-025 deliberately gives those rows
  no core span. A span on a spec would spend the 240-line budget on prose that the reserved row
  already carries whole.
- The anchor is satisfied only through the reserved row itself. The same path carried as any other
  row, or not carried, keeps TCP-V0-028's span rule, and name anchors are unchanged, so missing
  evidence still never reads as sufficient. The anchor's detail names the relation instead of a line
  range.
- `TestTaskContextSpecCitedRowsKeepTheirCoreSpans_TCPV0995` pinned the old behaviour (spec path
  missing). It now asserts that nothing is missing and that the spec took no core span.

Evidence: `TestContextSufficiencyReservedInstructionAnchorIsCarried_TCPV0975`, covering the ticket's
repro (`satisfied`, nothing missing) plus not-carried, ordinary-row and governing-row cases.

## V1-0179: the spec citing the task's named paths ranks first (`GPK-V0-975`, `GPK-V0-976`, `DR-0047`)

Observed at base with a binary built from this worktree. `corvint query` on the V1-0148 task
("docs: SPEC-TOOLCHAIN-INTEGRATION.md:79 cites a stale DOGFOOD.md line range") ranked two openspec
interop fixtures first; decision 0136, which the ticket saw on 2026-09-23, no longer appears.
`corvint impact docs/SPEC-TOOLCHAIN-INTEGRATION.md` ranked `documentation-citation-gate-v0.md` fourth
at score 825, and the default 5952-byte budget dropped it ("2 ranked results omitted by packet
budget"). The document's `references` field did contain the path (24 references, under the cap of
100).

Decisions:

- Query: in `evalRankDocuments` a document earns 40 for each indexed path the task names that its
  own `references` cite, plus 40 more when it cites every named path and the task names two or
  more. Document ranking was bag-of-words, so the path tokens `docs`, `md` and `spec` decided it. A
  credit of 40 alone left the gate spec second, behind the 2,000-line kernel spec, which cites
  `DOGFOOD.md` and shares more words. The full-coverage bonus encodes the rule that the document
  governing every named file is the owner. Path naming reuses TCP-V0-003's `mentioned` reading (a
  full path, or an unambiguous basename with a dot). A document never earns credit for being named
  itself.
- Impact: at equal score, a referencing document that is not a `spec` gets order 2, after the
  changed document and any referencing spec. This changes ordering only, not scores or the row set.
  A budget that keeps one row now keeps the spec. Raising the default budget was rejected because it
  would move every default receipt's bytes.
- Both rules move the parity kernel away from the oracle, so they are registered as `DR-0047`
  (python-defect, known-divergent, no cli-parity row authored). `TestGPKV0002ManifestReplay` passes
  unchanged with both rules (umask 022; see Friction).
- The analyzer schema input pin moved (`script/analyzer-schema.sh bump --base <lane base>`). The
  output digest is unchanged, so the schema stays `corvint-analyzer/150` and only the input digest
  was repinned. The bump covers the V1-3457 and V1-0179 source edits together.

Evidence: `TestEvalQueryRanksTheSpecCitingEveryNamedPathFirst_GPKV0975` and
`TestImpactOrdersAReferencingSpecBeforeAReferencingDecision_GPKV0976`. Both failed against the old
ranking (long spec first; decision before spec). This is the retained retrieval case: a fixture with
the V1-0148 shape (a gate spec citing both named documents, a lexically broader spec citing one, a
decision citing the changed one). Over this repository at the lane head, `corvint query --limit 1` on
the V1-0148 task returns `docs/specs/documentation-citation-gate-v0.md` (score 194), and
`corvint impact docs/SPEC-TOOLCHAIN-INTEGRATION.md` lists it third, inside the default budget. Core
freeze goldens (`cmd/corvint/core_freeze_test.go`) and the `internal/contextindex` package pass. The
frozen retrieval-bench and benchmark-runner corpora were NOT_RUN: they need external checkouts this
lane host does not hold.

Review fix (Codex, relayed by the orchestrator, head df23fd53b): the lowercased full-path lookup
kept one path per case-folded key, so with `src/Foo.go` and `src/foo.go` both tracked, map
iteration order picked the surviving path and made ranking nondeterministic. A case-folded mention
now names every path it matches, in sorted order: the lowercased task cannot tell the two apart,
and picking either one would be a guess. A shared basename still names neither path.
`TestEvalNamedPathsKeepsEveryCaseFoldedMatch_GPKV0975` uses the existing case-fold fixture to
assert the named paths and 32 repeated `EvalQuery` packets byte for byte. GPK-V0-975 states the
rule. The analyzer input digest was repinned again; the schema stays at 150.

## V1-2362: ALREADY-DELIVERED

The per-path equivalence oracle cost was fixed by V1-2342's 1-in-10 stride sample, recorded in the
`DCP-V1-216..217` traceability row of `docs/specs/documentation-corpus-v1.md`. Each test still
bounds Git operations on its full input (5,000 anchors, 5,000 claims, 4,096 files, 2,000 files) and
compares bytes on a sample covering every directory. Because the batches have no chunk size, the
sample takes the same code path. The equivalence assertion is the unchanged `bytes.Equal` of the
batched and per-path output. Measured at the lane base, load average 28 to 35:
`TestDCPV1216SymbolAnchorsGitOpsIndependentOfAnchorCount` 11.5 s; the CEM projection test 2.5 s;
legacy discovery 2.2 s; Playwright discovery 1.5 s; the other three under 2 s each. Every test is
under the 30 s criterion. Nothing was rebuilt.

## V1-2657: one commit batch per checkout (`DCR-V1-975`)

Decisions:

- `gitauth.CommitTrees` sends two requests per revision (the commit peel and the tree peel) through
  the existing `streamCommitBatch`, which already accepted any number of requests in one
  `git cat-file --batch -z`. Each record makes exactly the checks `CommitTree` makes. The
  `internal/cem/gitauth` edit is one additive method.
- Refusal order is kept. `CommitTrees` returns the trees verified before the first failing revision
  beside the error, and `verifyPinnedRevisions` checks the primary-tree identity on that prefix
  first. A primary revision that sorts before a missing one therefore still refuses as
  "recorded tree differs", as the per-revision loop did. One remaining difference is recorded in
  the spec: a malformed object record fails the whole batch.

Evidence: `TestDCRV1975PinnedRevisionsOneBatchPerCheckout`. Opening the checkout costs no Git
operation in the budget, so one revision verifies in 1 operation and six revisions now verify in
the same 1; the old loop refused at that bound. The test also covers a missing revision, a
malformed revision and the primary-tree-before-missing order. Every existing pin and verified-read
refusal test in `internal/doccorpus` passes unchanged.

## V1-2668: one artifact encode per compile (`DCS-V1-976`)

Decisions:

- The output bound is applied to the length, with no second encode. The artifact's `sha256` member
  has no `omitempty`, so the digested encoding already holds `"sha256":""`, and the final encoding
  is exactly the digested length plus 64 bytes. The check `size + len(digest) > limit` refuses
  exactly where `Encode` refused. `TestDCSV1976OneArtifactEncodePerCompile` pins the length
  identity, so a future `omitempty` fails there first.
- The size reported on that refusal keeps the old formula (`+ len(digest) + 16`), because it feeds
  the DCS-V1-569 retry's overshoot. An exact figure would move the kept prefix and so the
  artifact's bytes.
- Encodes are counted by an `artifactEncodes` counter in the existing `BuildsForTest` and
  `PinsForTest` style, incremented in `Encode` and `EncodeTo` for artifact values.
- The build command's own write encode is out of scope; the criterion is per compile.

Evidence: a normal build is 1 compile with 1 artifact encode (it was 2); the over-bound build is 2
compiles with 2 encodes (it was 3). Artifact bytes are unchanged: `TestDCXV1471DerivationVersionGuard`
(fixture digest) and `TestDCSV1572RetryFromPinnedInputsIsByteIdentical` pass. The over-bound build
(the DCS-V1-569 fixture, MaxRecords+8 functions) was timed with 15 builds per run, before and after
interleaved, at load average 31 to 34: before, medians 274 ms and 302 ms; after, medians 251 ms and
242 ms.

## Friction

1. `corvint-tasks ticket show` refuses three of the five lane tickets (V1-3457, V1-2657, V1-2668)
   from both the primary checkout and the lane worktree: "not in this checkout's intent
   projection … no journal exists". This host's `.taskman` snapshot predates those tickets, and the
   WSL primary checkout holds no journal. Repro: `cd ~/projects/corvint && corvint-tasks ticket show
   V1-3457`. The texts were recovered from `~/lanes/tickets-indexcli-w5.md`.
2. The installed `corvint` is stale. `corvint test --base <sha> --dry-run` reports
   `DEGRADED self-binary-stale: this binary was built from commit dbc1713779fc and 25 Go source
   paths differ`. Every query and impact observation above needed a binary built from the worktree.
3. `TestGPKV0002ManifestReplay` fails at the lane base on this host's default umask 0002:
   `FAIL adopt-full-receipt fileModesBeforeSha256`. It passes under `umask 022`. A lane that does
   not know this reads it as a parity regression. Repro: `umask 0002; script/heavy.sh go test
   -count=1 -run TestGPKV0002ManifestReplay ./conformance/cli-parity-v0`.
4. A Git budget refusal names the default budget instead of the bound that applied: "Git operation
   budget of 1024 operations exhausted while verifying pinned revisions (git-budget-exceeded: Git
   operation budget exhausted at its 1-operation bound)". The outer number is wrong for a bounded
   budget. Repro: `verifyPinnedRevisions` under `boundCorpusGitOps(ctx, 1)` with two revisions in
   one checkout, at the lane base.
5. An unrelated ranking edit in `internal/contextindex` (`eval_query.go`, `impact.go`) trips
   `TestAnalyzerSchemaInputs`, and the fix is a schema-tool bump that repins digests for a change
   that alters no analyzer output. A lane learns this only from a whole-package run. Repro: edit
   any comment in `internal/contextindex/impact.go`, then run
   `go test -run TestAnalyzerSchemaInputs ./internal/contextindex`.
6. `heavy.sh` refuses a whole-package run without `HEAVY_WHOLE_REASON`, while
   `corvint test --dry-run` selected 181 units for a change that was mostly spec text, and its
   40 KB output overflows a terminal. With no narrow selection to reuse, a lane has to invent a
   reason string to run the one package it needs.
7. `DR-` numbers are allocated by `git grep` against the base only. Another unmerged lane can take
   `DR-0047` too, and two register entries with one number merge without a textual conflict. This
   lane used `DR-0047`.
