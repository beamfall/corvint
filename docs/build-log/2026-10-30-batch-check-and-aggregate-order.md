# Batch-check merge recovery and aggregate verify order

Lane friction-dogfood-w5, base `c5d5f0829c28756ee7b202caf80a3f3fc37fa85b`. Tickets `V1-3104`
and `V1-3166`; requirements `DCW-V0-443` and `DCW-V0-445`.

## V1-3104: one merge-resolution-unbound merge per run (suspected)

The suspected early stop does not reproduce. `internal/dogfoodflow/batch.go` is byte-identical
between batch 1029a's BASE `c412f4f10` and this lane's base, and the installed build of this base
run on the batch's head `b0de0ac91` (detached scratch worktree, `--verifier` the same binary)
prints `FAIL merge-resolution-unbound count=2`, listing both `521287dca` (documentation-corpus
spec) and `b0de0ac91`. `verifyCoverage` walks the whole range and `failure` appends each merge;
nothing stops at the first failure or dedupes by code. The second rebind round came from acting on
the head merge only: the single recovery line named neither merge's own BASE nor that every
listed merge needs the same recovery.

Decision: keep the verdict and add one `fix:` line per listed merge, naming the merge, the change
it merged (second parent) and its rebind BASE (first parent), plus, for more than one merge, a line
saying all are recovered in one round in the listed order, because redoing a merge changes the
first parent of every later merge. `TestBatchCheckDCWV0443ListsEveryHandResolvedMerge` pins two
hand-resolved merges, `count=2` and both lines.

## V1-3166: unsorted VERIFY file on the aggregate path

`aggregateExpectedValid` requires the verification list to equal `trace.NewRecord`'s sorted
unique normalization, and `AggregateExpectedFor` passed the author's file order straight in, so
an unsorted file failed `aggregate-schema-invalid`. The learning trace path normalizes the same
file silently. Order binds nothing beyond the receipt: the receipt array, envelope and logical
binding all hash the normalized list (`ALO-V0-005`, `ALO-V0-006`), and `dogfood-check` reproduces
from the receipt's own list. Decision: normalize (byte-order sort, drop duplicates) at the author
entry point `AggregateExpectedFor`; receipts stay strict and a reordered receipt still refuses.
Accepting the order removes the refusal entirely, which is preferred to an early hint.
`TestDCWV0445AggregateAcceptsAnyVerifyOrder` fails on BASE with `aggregate-schema-invalid`.

## Limits

No `make gate`; focused tests only. The spec status lines at the top of
`docs/specs/daily-change-evidence-workflow-v0.md` and `docs/specs/REQUIREMENTS.tsv` are left for
the lane's single `make spec-requirements` pass.
