# Decision 0151 — The change-witness symbol resolver is Go's standard-library parser over Git objects

Date: 2026-09-12. Status: accepted. Authority: repository owner instruction, 2026-09-12, delivered
through the coordinator under the owner's delegation of owner calls.

`docs/specs/change-witness-relation-v0.md` (accepted by
`0047-batch-acceptance-eight-proposed-specs-2026-09-04.md`) was blocked on its Q1: whether the
Frontier verifier may depend on a Corvint-built symbol resolver, given `CF-V0-026`'s prohibition on
Corvint retrieval.

The owner call: the resolver for `ocm-change-witnessed-v0` is the in-repo structural analysis only.
For Go it is `go/parser` plus `go/ast` top-level definitions at the pinned target revision, read
from Git objects and never the worktree. It performs no type checking, reaches no network, runs no
external tool, and reads no persisted Corvint index, so it sits on the "resolver built from Git
objects alone" side of Q1 and `CF-V0-026` needs no narrowing. Other languages have no resolver and
abstain with an explicit reason. An identifier that resolves to zero or to more than one definition
abstains and never witnesses (invariant 2); a Go blob the grammar refuses makes every resolution
uncertain, so the obligation abstains rather than resolving against a partial table.

Consequences: the spec gains `CWR-V0-014` (the resolver boundary) and `CWR-V0-015` (the evaluator's
outcome and reason vocabulary), its Q1 is marked answered, and its delivery moves to experimental
with the pure evaluator `internal/changewitness`. It does not import `internal/contextindex`, whose
`goSymbols` falls back to a lossy line scanner on parse failure. Nothing consumes the evaluator:
`frontier/0` output and every frozen conformance corpus are unchanged, and `frontier/1` remains
not-started.

Rollback: delete `internal/changewitness`, remove `CWR-V0-014` and `CWR-V0-015`, restore Q1 as
open and the spec's delivery to not-started, and mark this decision superseded.
