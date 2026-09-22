# Decision 0171 — Learned-trace reads keep the untracked-path block

Date: 2026-09-12. Status: accepted, delegated coordinator call. Authority: repository owner
delegated the A/B choice on the `docs/agent-memory/ideas.md` entry "learned traces: a
revision-bounded untracked allowance" to the coordinator and this leaf.

## Outcome: B — decline the tree-bounded allowance

`GPK-V0-061` is unchanged in behavior. Any path Git status lists, untracked ones included, keeps
`learning.local_trace_state: "blocked-mixed-worktree"` and the trace store is not read
(`internal/tracerecordrepo/read.go:95@241a6e10`, `internal/contextindex/history.go:29@241a6e10`). No requirement
ID, wire field, packet byte, or analyzer schema changes. The spec clause gains one sentence citing
this decision.

## The variant considered (A)

Admit an untracked path when it is absent from every tree in the replay window and outside every
tracked directory, bind the admitted set into the query packet, and adjudicate the packet change
under `GPK-V0-033`. The soundness premise holds: a stored record's `opened_paths` and
`changed_paths` must be tracked at the record's own revision (`internal/trace/record.go:105-112@df31a948`),
so a path absent from every such tree cannot be named by a valid record.

## Why it is declined

1. **The window is defined by the forbidden read.** The trees a record can name come from the trace
   store's revision filenames (`trace.CandidateRevisions`, `internal/trace/store.go:251@a5b8bf2c`, called
   from `internal/tracerecordrepo/adapter.go:248@a73ccead`). Enumerating them is a read of
   `.context-corvint/traces`, which `GPK-V0-061` forbids in the blocked state. The only store-free
   window is the whole HEAD ancestry the adapter walks, bounded at `maximumAncestry = 10_000`
   commits (`internal/tracerecordrepo/adapter.go:23@159644e6`). Proving absence there means a per-path
   history lookup over up to 10,000 trees, a new unbounded-cost Git read on a path that today
   returns without touching history. Either reading replaces the refusal with a heavier read than
   the one it is meant to avoid.
2. **The receipt cannot bind it without widening the oracle surface.** `GPK-V0-044` requires the
   repository-intent query to consume traces "with the same learning semantics as" the Python-oracle
   `user-prompt` path, and `conformance/cli-parity-v0/manifest.json` pins the
   `query-repository-trace-*` rows with `expectationSource: "python-oracle"`. The oracle blocks on
   any dirty path. An untracked-only worktree would make Go emit `ready`, a trace count, learned
   candidates, and a new allowance binding field that the oracle never emits. Under `GPK-V0-033`
   that disagreement is BLOCKED; it cannot be adjudicated `python-defect`, because the spec text
   (`GPK-V0-061`) states the oracle's behavior. Admitting it would require authoring a spec
   divergence in order to create a known-divergent register entry: a Go-originated change to a
   Python-oracle packet surface, which this call does not accept for this benefit.
3. **The benefit is small and has an existing route.** Learned candidates are restricted to
   currently indexed paths (`GPK-V0-044`), so an untracked file never becomes a candidate either
   way. The loss is only that an agent's scratch file suppresses learned traces. A path Git status
   does not list (ignored under the caller's ignore policy, decision 0158) does not trigger the
   block today.

## Consequences

The ideas.md entry is removed. Range impact keeps its `GPK-V0-060` allowance; the two consumers
intentionally differ. A future reopening needs, at minimum, a store-free bounded window and an
accepted `GPK-V0-025` retirement of the Python oracle for the query packet, after which the packet
is no longer a cross-checked surface.

Rollback: revert this decision's commit, which restores the ideas.md entry and removes the
`GPK-V0-061` sentence. No code or data changes.
