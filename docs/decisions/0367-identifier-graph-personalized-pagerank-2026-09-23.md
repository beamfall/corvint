# Decision 0367: Identifier graph with an opt-in personalized PageRank context slot

Date: 2026-09-23. Status: proposed (experimental delivery; ticket V1-0083).

This decision:

- adds `TCP-V0-030` through `TCP-V0-034` to `docs/specs/task-context-packet-v0.md`;
- bumps `analyzerSchemaID` to `corvint-analyzer/74` and adds the `vocab.identgraph` pack section;
- changes no default packet byte, no accepted wire and no `protocol/**` wire, and adds no root verb.

## Context

The release notes record that `context` is below RepoMap-class structure-aware retrieval at
recall@20 on three of the four Agent Retrieval Bench subsets (code2test, comment2context,
edit2ripple; decision 0070). Aider's RepoMap ranks files by PageRank over a graph whose edges join a
file that references an identifier to the file that defines it, personalized toward the files in
the chat. `context` already had the ingredients (indexed Symbols and case-sensitive Words postings)
but ranked only by named relations and lexical terms.

## Decision

1. The full compile derives an identifier graph from the index at the indexed revision
   (TCP-V0-030). Nodes are term-table source ids. An eligible name is an indexed symbol of 4..128
   ASCII identifier bytes that is not lowercase letters only, with at most 5 definers and at most
   50 naming sources. Edges are symmetric, weighted by the number of joining names and labelled by
   the rarest one with its direction. The encoding is fixed-width integers in sorted order, so it is
   byte-deterministic; a damaged encoding fails the load.
2. With `CORVINT_CONTEXT_GRAPH=on`, a `graph` slot runs after every other slot and the reserved
   rows (TCP-V0-031). It seeds personalized PageRank (restart 0.15, forward push to 1e-5) from the
   task anchors (subject, then `mentioned`, `pair`, `definition`, `reverse-import` and `reference`
   rows, at most 16) and admits at most 5 rows within 3 hops. They go after every relation row,
   within the limit, and displace only lexical and documentation tail rows.
3. Each row's reason names its seed and anchor and every hop (TCP-V0-032). Graph rows are `low`
   confidence and `syntax` authority, register no corroboration and never count as a TCP-V0-016
   relation.
4. The graph is bounded at 2^20 nodes, 2^21 arcs and 200,000 pushes. Past a bound, or with no seed,
   the slot abstains as `graph-bounded` or `no-seed` (TCP-V0-033).
5. Gating (TCP-V0-034): the slot stays an explicit opt-in. The frozen evaluation below regresses
   recall@20 on every subset, so the default-on condition (no recall@20 regression on any subset)
   fails, and so does the ticket's closing rule (recall@20 improves on the RepoMap-deficit
   subsets). The slot and its graph ship as experimental, off by default.

## Evaluation

`tools/retrieval-bench` built from this branch, `--arms context --max-samples 30` (the first 30
samples in file order of each release), flag unset versus `CORVINT_CONTEXT_GRAPH=on`, same binary
(sha256 in the report sidecars). The bound is recorded: unbounded runs measured about 0.5 samples a
minute per run at host load ~300 during the window, too slow for the full 345 samples twice.

| Subset | recall@5 off/on | recall@10 off/on | recall@20 off/on | recall@20 wins/losses |
| --- | --- | --- | --- | --- |
| code2test | 0.4056 / 0.4056 | 0.5222 / 0.5222 | 0.5856 / 0.5289 | 0 / 4 |
| comment2context | 0.3278 / 0.3278 | 0.4222 / 0.4222 | 0.5278 / 0.4389 | 0 / 6 |
| edit2ripple | 0.3833 / 0.3833 | 0.4694 / 0.4694 | 0.5667 / 0.5639 | 2 / 2 |
| trace2code | 0.4833 / 0.4833 | 0.5833 / 0.5833 | 0.8611 / 0.8278 | 0 / 1 |

Losing samples at recall@20: code2test `1dd2bdfdcf7092e1c69f50d6`, `3a9ae9c1d49cc6b262e3cab7`,
`8ab746ffa3dc3b8e4f8d8327`, `4efdeafc9e5c6cf5ef4334fc`; comment2context `01323034afde8983e978b8d2`,
`191c9a48dcea15ffd9b74793`, `41deac7db89b57cead1c85e0`, `7d5c2788e4c30bb773cb6643`,
`ba3c1cd24f66bb46eb30d57d`, `e1280404f66f39671f1939e5`; edit2ripple `fa3cd07539adf20ceb766dda`,
`4eb8d03198ff006b7a39c280`; trace2code `13dc98681c5ab25210a04f43`. In each code2test loss a
gold test file that a lexical or documentation tail row held (for example `server/storage/mvcc/kv_test.go`) was
displaced by a graph row that was not gold. recall@5 and recall@10 cannot move, because the slot
only places rows in the last five positions. One edit2ripple sample errored in both arms. The
full-sample runs are NOT_RUN.

The evidence says the identifier graph, as seeded here, ranks below the lexical tail it
displaces. A graph that competes for positions by score, a smaller cap, or a graph built with
test-file edges are the open follow-ups. None is taken here.

## Alternatives set aside

- Default-on without the frozen evaluation was rejected: the ticket's closing rule is recall@20 on
  the RepoMap-deficit subsets, and the gate is no regression on every subset.
- Ranking all files by global PageRank (RepoMap's non-personalized mode) was rejected. It returns
  hub files regardless of the task, and a reason could not name a seed.
- Seeding from lexical rows was rejected. It would rank the graph around term matches, which is
  what the slot is meant to supplement, and a lexical row is not an anchor.
- Letting graph rows count as relations for TCP-V0-016 was rejected. A name match is not evidence of
  a relation, so it cannot rescue an unsupported conjunction.
- A language-server or import-resolved call graph was rejected here. It needs a provider outside
  the one-binary boundary (V1-0099 owns that route).

## Consequences and rollback

- Unset, `off` or any other value leaves the packet byte-identical to the recipe golden
  (`TestContextGraphDefaultBytes`). The index gains one derived section, so every snapshot rebuilds
  once under schema `/74`.
- Rollback: unset `CORVINT_CONTEXT_GRAPH`. To remove the slot, delete
  `internal/contextindex/identgraph.go`, `ppr.go` and their tests, the `IdentGraph` member, the
  `vocab.identgraph` section and the compile, `compile` and `rowAction` hooks, and bump
  `analyzerSchemaID`.
