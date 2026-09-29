# Bounded LSP task-subject projection

Owner intent: V1-0477/V1-0482, improve both agent semantic enrichment and the optional editor direction. On 2026-09-29 the owner selected Go/gopls agents plus VS Code and Neovim editors for initial qualification. This accepts that scope, not exact client tuples, quantitative floors or promotion. This entry records an experimental agent repair; no editor server is delivered.

## Defect and change contract

The public baseline retained by draft PR #360 resolved both definitions directly with gopls but included only 3/6 expected combined relations. Its matched `--limit 20`/`32` repro pinned the miss to final lexical path-relation selection: the expected edge existed in the 32-row provider record and was dropped at 20. The 64-query stop is a separate remaining limit.

TCP-V0-054 now specifies subject-first selection for opt-in task LSP evidence. `lspevidence.Attach` passes the explicit subject to `extevidence.InlineTaskSection`; after repository binding, exact root-repository/path matches on either endpoint precede other admitted edges. Each group retains the existing total key, then the existing limit and omission count apply. Ordinary provider and no-subject inline projections keep their existing ordering. Core rows, authority, endpoint validation, seeds and query/resource bounds are not changed by this selection. Subject edges can still exceed the bound or be absent from the upstream record.

## Immutable development evidence

Base: `29274889ac6cf78a2c2029d64b1a6601f537441d`. Measured clean candidate: `8437bed30ce1a3ce1d9af719ff9d610a4ada0e3d`. Runtime: Go 1.27.1, gopls 0.23.0, darwin/arm64. Public candidate binary SHA-256: `d3091ac21af6f1007d082f8d7aed242a4e5ce2e7b8f8c42460353ab434e88cf6`; gopls executable SHA-256: `39431a5b273a5ac124a98521d1ec3e38af4bc5eec783e24a23344145eef24836`.

The unchanged PR #360 measurement harness ran the same two public labels and positions, three repeats, upstream/Core/combined arms and `--limit 20`. The private candidate manifest changes only `sourceBase` to the candidate above so the harness admits changed Go sources; it does not rewrite gold labels. Manifest SHA-256: `25c62d03f9961a49abb9e2d90e20e8c460a189b91c72e6699448f724e5aef4c4`. Raw private report `/tmp/corvint-lsp-integration/projection-public-result.json`, SHA-256 `c85eb51cc45172e6170b453695b434182d0cf9f34829e1fb6454f8995e0b5c0e`.

| Public observations | Prior baseline | Candidate |
|---|---|---|
| Direct gopls expected definition | 6/6 | 6/6 |
| Core expected path present | 6/6 | 6/6 |
| Combined expected relation | 3/6 | 6/6 |
| Core/combined non-external packet parity | 6/6 | 6/6 |

All candidate commands and gold checks passed. Each combined result kept 20 path relations. The first witness still issued 64 queries and stopped at `query-budget`; it now retains the expected subject definition. This is a development regression witness across changed source revisions, not a held-out distribution or a controlled latency comparison. Three repeats are not six independent tasks. Cache topology differs between upstream and combined arms; timings remain descriptive and p95 of three samples is only the maximum. Representative recall, precision, memory/CPU, comparable cold/warm latency, task-outcome improvement and editor interoperability remain `NOT_RUN`.

Required `script/qualify-lsp.py` passed on the clean candidate using dependency-free committed module and go.work fixtures: five/six definition/reference witnesses, exact CLI/MCP packet parity, default-off behavior, dirty-file omission and SIGTERM descendant retirement/private-cache removal. Raw private report SHA-256: `224de6d5f6b6310d9351bd069b6d804b3f8fdf20c86b8d232930e910bede2a8f`. Its separate compiled CLI/MCP binary digests and fixture commits are retained in the report. These are synthetic semantics/transport observations, not productivity evidence.

## Checks, review and dogfood boundaries

Pre-change `query` and path `impact` receipts are retained privately. Query selected the transport spec, not the target task-context clause, and omitted four ranked rows; original task-context/source evidence was read directly. Impact found both changed Go paths but omitted 27 rows and named omitted CLI/MCP callers. Candidate `affected --base` retains language-frontier and non-Go path unknowns; it does not attest test completeness. Corvint suggestions do not override the owner's scoped focused-test preference. Broad repository `make gate` is `NOT_RUN` for this slice.

The pre-change external-evidence/attachment units passed after admitting local TLS fixture listeners outside the sandbox; the first sandbox run failed because those listeners were denied. The new regression witnesses passed: subject on either endpoint, foreign same-path rejection, reversed record order, zero/positive bounds, exact omissions, verified provenance, unchanged other external members and empty-subject parity. Spec-index checks passed. The frozen enrolled final checks are external-evidence/attachment units and vet, CLI LSP parity/degradation tests, and spec-index tests. Their exact committed observations and CEM/OCM reports are retained by the normal dogfood binding/check/seal workflow; this entry does not infer completion from enrollment alone.

Independent Astra/low review of candidate `8437bed3` found no actionable code findings, including root identity, before-bound ordering and preserved generic/no-subject behavior. The reviewer also inspected the live module/workspace report; final public evidence inspection is retained separately. Model choice was a bounded review of a small ordering/trust boundary; builder live model/effort controls were not inspectable. No held-out corpus was opened. Mutation and learning routes are not applicable because this changes external projection rather than Core ranking/learning. Default read behavior remains terminating and no test/model/edit is launched by the read.

## Rollback and remaining integration

Revert this slice to restore lexical task relation selection, or use `--lsp off`/the existing MCP `task-review` profile to disable semantics. Default Core operation remains available. V1-0482 stays open for its broader profile and projection acceptance, dependencies and native completion; V1-0478 and editor qualification remain open. PR #361 redaction and PR #362 frozen-corpus integrity interpretation remain separate pending integrations. No support/promotion or new merge approval is inferred from these observations.
