# 2026-11-18: rc.4 docs corpus and MCP search qualification (RCF-V1-008, RCF-V1-009)

Requirements: RCF-V1-007, RCF-V1-008, RCF-V1-009, RCF-V1-010. Change: `rc4-candidate`, tasks 2.3
and 2.4. Tickets: V1-5688 (multi-root limitation), V1-5689 (setup documentation).

## Result

PASS. Two runs of `TestRCFV1008Qualification` on the same commit produced byte-identical reports,
and DCS-V1-404 passed in both runs.

| Field | Value |
| --- | --- |
| Corvint commit under test | `1f35e9f28aa9d63475c8b1591a3d84265ffe9cf4` |
| Toolchain and host | go1.27.4, WSL lane host (Linux/amd64), `script/heavy.sh` local slot |
| Query set digest (`queries.sha256`) | `d1ade010f0c6f6b4c08b7fc2bafecef1e3e0780556100e50c3d8ad0d8ff1e5a0` |
| Report run 1 sha256 | `dea19bdfad900ea8ef1d0cee99ea5bb278a94f4eb23dd458114a326448363796` |
| Report run 2 sha256 | `dea19bdfad900ea8ef1d0cee99ea5bb278a94f4eb23dd458114a326448363796` |
| Run wall time | 134.24 s and 134.49 s |

| Repository | Pinned commit | Evidence | Queries | Passages verified | Unverified | Artifact sha256 |
| --- | --- | --- | --- | --- | --- | --- |
| corvint | `3dc980b7aec44eb4f880f3bdaec2fc2a23da2102` | full | 25 | 502 | 0 | `4af1653bae205c456b3bf9700dc80cfc3e4013b9ce0b7a551eb0e22e93a38e64` |
| beamfall | `947a28cc384b40643f7417b334f6b3490aa2b206` | digest-only | 25 | 570 | 0 | `30c4bd81dd9ad383bc0106244c710c5ea70159190816b80342118815a289dc13` |
| vue (`vuejs/core`) | `148fdacdc3a67aaa2b8d1b615da227c3fcc2df53` | full | 25 | 504 | 0 | `c154d7eb3a22ed18cd2b67d7419465a9dd93b4cddb2e4256dc3dd443f250d0e4` |

The `git status` digests before and after every repository's run are the empty-input digest
(`e3b0c442…b855`), so the runs left each checkout unchanged.

`TestDCSV1404SearchRankingEvalFixture` (`internal/mcp/corpusbridge`) passed in both runs (0.29 s and
0.28 s). The test evaluates the fixture twice in each run and asserts the same status and rank for
every case (`search_ranking_eval_test.go:165`), so rank identity is directly asserted.

## Third repository: prometheus replaced by vuejs/core

The frozen set first named `prometheus/prometheus` at `cb4e01dd`. Its history has two root commits,
and `CorpusRepositoryID` (`internal/contextindex/corpus.go:84`) requires exactly one, so
`docs corpus manifest` refused with "corpus requires one unambiguous root commit". This refusal is
deliberate and fail-closed, the same rule as EEP-V1-005. An earlier run on `d3cde8629` therefore
recorded prometheus as NOT_RUN, with corvint and beamfall PASS. The owner approved replacing it
with `vuejs/core` on 2026-11-18 ("Yes, swap to vue"). `vuejs/core` at `148fdacd` has one root,
`3401f6b4`. Commit `1f35e9f28` holds the new entry and digest:
- scopes `packages/reactivity` and `packages/runtime-core`;
- 8 file, 8 symbol and 9 prose queries, `vue-01` to `vue-25`.

The multi-root refusal remains a known rc.4 limitation that the release notes must state (V1-5688).
`pydantic/pydantic` is also two-root.

## Setup used

V1-5689 tracks documenting this setup.

- Build the binaries with `go build -buildvcs=false`. With a VCS-stamped binary, every query
  fails with "artifact cannot be rederived from current builder".
- For each repository:
  - Run `corvint-docs --root R docs corpus manifest --schema 2 --revision PIN --scope …
    --timestamp 2026-11-18T00:00:00Z --no-workspace`.
  - Copy the manifest to `R/.git/m.json`; `--manifest` must be relative to `--root`.
  - Run `corvint-docs --root R docs corpus build --manifest .git/m.json > R/.git/corpus.json`.
- The Corvint manifest lists file scopes that exclude the 7 generated-marker files (DCP-V1-607).
- Clones must be full, not `--reference` clones.
- The runner config is `{report, repositories: {name: {root, artifact: ".git/corpus.json"}}}`,
  passed through `CORVINT_RCF_QUALIFY`.

## Rerun trigger (RCF-V1-010, task 2.4)

`.corvint/test-selection.json` (commit `4082dcca9`) maps these paths to
`cmd/corvint-docs/rcf_qualification_test.go`:
- `cmd/corvint-docs/{docs,docs_corpus,main}.go`;
- the query-set directory;
- the `internal/doccorpus` artifact, compile, encoding, query, read, search and type files;
- `internal/mcp/corpusbridge/*.go`;
- `internal/mcp/docsserver/*.go`.

`corvint affected --path internal/doccorpus/search_rank.go` (rc.3 build 204) selects:
- `cmd/corvint-docs`, the runner package;
- `internal/mcp/corpusbridge`, which holds DCS-V1-404;
- `cmd/corvint-docs-mcp`.

Its scope verdict is `narrowed`.

Limit: a selected package runs `TestRCFV1007FrozenQuerySet` and the unit checks automatically. The
three-repository run needs `CORVINT_RCF_QUALIFY` and pinned clones, so an affected selection
signals the rerun but does not perform it. RCF-V1-010 keeps that rerun as a precondition of the
promotion commit.

## Promotion (RCF-V1-011, task 3.2)

The promotion commit changes labels only. Between the qualified commit `1f35e9f28` and the
promotion base, the only change under the RCF-V1-010 trigger paths is the task 3.1 contract work
in `internal/mcp/corpusbridge`: tests and `testdata/rcf-contract/` fixtures, with no ranking,
corpus build, corpus format, verb or tool code (`git diff --stat 1f35e9f28 97d5e2e4d --
cmd/corvint-docs internal/doccorpus internal/mcp`). The qualification therefore carries to the
promotion without a rerun.

- `docs/COMPATIBILITY.md`: `docs-search-mcp-schemas` and `evidence-corpus-format` (label
  `corvint-evidence-corpus/2`, read from `internal/doccorpus/capacity.go`) are `stable` under
  CMP-V1-003 item 6, with the task 3.1 contract tests. `mcp-schemas` stays `preview` for every
  other tool and for the three tools' result order.
- `README.md`: a `stable` Documentation search row. The Documentation corpus row stays
  `experimental` for drafting, generated pages, flows, claims and the other corpus verbs.
- `docs/CAPABILITIES.md`: the three tool rows read `stable (schemas; result order preview)`.
  The `corvint docs` and `corvint-docs` command rows stay `experimental`, because root help's
  Experimental group drives that cell.
- PROD-DOCS-009: Delivery `validated`, State `DELIVERED`, for the RCF-V1-006 scope.
- Spec delivery: the `documentation-corpus-v1.md` header is frozen
  (`docs/specs/INTENT-STATUS-FROZEN.tsv`). That spec and its search and reads companions still
  hold `standing` requirements, and SRG-V0-010 reports a whole-spec `validated` as
  NOT_PROMOTABLE. The scoped status is therefore recorded in the RCF-V1 delivery section and in
  PROD-DOCS-009.
- Independent external utility: `EXTERNAL_DEPENDENT`, `NOT_RUN`.

## Revert drill (RCF-V1-012, task 3.3)

On a scratch worktree at `e3f988b98`, `git revert e3f988b98 101d0951a` produced a tree with no
difference from the pre-promotion commit `97d5e2e4d` (`git diff --stat` empty). No data
migration was involved.
- `docs/COMPATIBILITY.md` and `surfaces.json` hold neither promoted row.
- PROD-DOCS-009 reads `Delivery: experimental. State: PARTIAL.`
- `README.md` has no Documentation search row.

`make compatibility-check`, `make capabilities-check`, `make openspec-check` and
`make spec-requirements-check` all exited 0 on the reverted tree. The scratch worktree was
removed afterwards.
