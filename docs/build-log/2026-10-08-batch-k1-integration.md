# 2026-10-08: Batch K1 integration (V1-0827, V1-0759)

## Intent

Integrate two finished, independently reviewed Tasks lanes as one batch branch,
`claude/batch-k1-2026-10-08`, based on `origin/main` `b5616037` (batch H). Each lane is merged with
`git merge --no-ff`, so its reviewed commits stay intact.

A first batch K merged five lanes, which put 330 obligations across six intent specs. That is over
the change-evidence binder's 256 cap. The batch was split by intent spec: K1 carries the agent-leases
and store-init specs (185 + 7 = 192 obligations). K2 (`claude/batch-k2-2026-10-08`) carries V1-0653,
V1-0485 and V1-0513.

## Merges

| Order | Ticket | Lane head | Conflicts |
| --- | --- | --- | --- |
| 1 | V1-0827 role-stage selection (CAL-V0-197, decision 0467) | `9cf9d6ce` | leases spec, `INDEX.json`, `README.md`, decisions `README.md`, `REQUIREMENTS.tsv` (generated) |
| 2 | V1-0759 re-import keeps refined pool and roles (CTS-V0-007) | `e08da0db` | none |

## Integration decisions

- The V1-0827 lane still described CAL-V0-197 as proposed, but decision 0467 accepted it. The
  merge records it as accepted (decision 0467) in the spec header, its digest, `README.md` and
  `INDEX.json`.
- CTS-V0-007 stays proposed. Owner acceptance is human-owned and has not been given in this batch.

## Evidence

Each lane retains its own build-log entry, focused tests and independent review. On the merged
tree, these all passed: the doc gates, `internal/specindex`, `go build ./...` and `go vet`. The
five-lane batch K tree, whose spec and Go content for these lanes matches this branch exactly,
also passed the focused `internal/tasks` packages, including the `internal/tasks/store`
role-stage, import and workflow tests. The batch CEM is bound against base
`b5616037000a06e4719c759a555f7e393390e94f` and sealed.

## Rollback

Revert the batch merge commit on `main`. Either lane can also be reverted on its own through its
merge commit in the batch branch.
