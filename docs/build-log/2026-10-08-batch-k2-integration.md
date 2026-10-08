# 2026-10-08: Batch K2 integration (V1-0653, V1-0485, V1-0513)

## Intent

Integrate three finished, independently reviewed lanes as one batch branch,
`claude/batch-k2-2026-10-08`, based on `origin/main` `b5616037` (batch H). Each lane is merged with
`git merge --no-ff`, so its reviewed commits stay intact. This is the second half of batch K, which
was split by intent spec to stay under the change-evidence binder's 256-obligation cap. See
`2026-10-08-batch-k1-integration.md`. K2 binds the core-compatibility freeze (9), task-context packet
(63), MCP server (34) and CEM pilot kit (32) specs, 138 obligations.

## Merges

| Order | Ticket | Lane head | Conflicts |
| --- | --- | --- | --- |
| 1 | V1-0653 `governance_refused` register (decision 0468) | `4124c848` | decisions `README.md` |
| 2 | V1-0485 late cancellation (TCP-V0-064, MCPV0-034, decision 0470) | `666f2d1b` | decisions `README.md`, `core-compatibility-freeze-v1.md` |
| 3 | V1-0513 invalid OCM markdown | `29d6ed50` | none |

## Integration decisions

- In `core-compatibility-freeze-v1.md`, the V1-0653 register text is kept. V1-0485 shifted
  `internal/contextindex/taskcontext.go`, so its line citations are repinned to the verified lines
  2312, 285, 286 and 287.
- The decisions index keeps the union of rows (0470, 0468) in descending order.

## Evidence

Each lane retains its own build-log entry, focused tests and independent review. On the merged
tree, these all passed: the doc gates, `internal/specindex`, `go build ./...` and `go vet`. The
five-lane batch K tree, whose spec and Go content for these lanes matches this branch exactly,
also passed the focused tests:

- `internal/cem/workflow`, `internal/contextindex` and `internal/mcp/bridge`
- the `cmd/corvint` core-freeze and task-context tests

The batch CEM is bound against base `b5616037000a06e4719c759a555f7e393390e94f` and sealed.

## Rollback

Revert the batch merge commit on `main`. Each lane can also be reverted on its own through its
merge commit in the batch branch.

## Main merge (38fcf067, batch I and V1-1028)

origin/main 38fcf067 added V1-0859/V1-0431's record-data gate (TCP-V0-063) to
`internal/contextindex/taskcontext.go`, beside this batch's cancellation boundaries (TCP-V0-064,
V1-0485). The code merged cleanly. The spec keeps both requirements in number order. Line
citations into `taskcontext.go` in `core-compatibility-freeze-v1.md` (14) and decision 0377 (1)
were relocated to the unique span whose content anchor still matches, with no hash changed. Both
sides had re-pinned the analyzer audit digest at schema `corvint-analyzer/112` without a schema
bump, so the merged input set is the union of two audited changes; the pin is recomputed and the
schema stays /112. The earlier seal is dropped and the batch is rebound against the new base.
Focused checks: `internal/contextindex` (full), the `cmd/corvint` TaskContext, core, CEM,
governance and MCP subset, `internal/mcp/...` and the doc gates pass.
