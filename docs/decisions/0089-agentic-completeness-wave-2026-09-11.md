# Decision 0089 — Agentic-completeness wave: eight proposed experimental slices

Date: 2026-09-11. Status: accepted direction; every slice below stays `proposed` intent and
`experimental` delivery until its own gate runs. Authority: repository owner, "you should build it
all out" and "build those too", in answer to a gap assessment of Corvint as an agentic tool.

The owner accepts building, as separately specified experimental verbs, the systems the
assessment found unspecified and the five directions it proposed as candidate breakthroughs.
The assessment's first item, a symbol-level query, was withdrawn during the wave: `corvint
context defs|refs|grep` already answers it with blob-pinned evidence (`internal/contextindex/lookup.go`),
so no `symbol` verb ships; per-occurrence columns and excerpts for `context refs` are recorded as an
idea, not built.

| Slice | Contract | Verb | Kind |
|---|---|---|---|
| Dependency source evidence | `docs/specs/dependency-source-evidence-v0.md` | `depsource` | read-only |
| Scope leases | `docs/specs/scope-lease-v0.md` | `lease` | explicit local mutation |
| Outcome calibration | `docs/specs/outcome-calibration-v0.md` | `calibrate` | read-only |
| Necessity labels | `docs/specs/necessity-labels-v0.md` | `necessity` | read-only |
| Retriever disagreement | `docs/specs/retriever-disagreement-v0.md` | `answerability` | read-only |
| Touch-set surprise | `docs/specs/touch-set-surprise-v0.md` | `surprise` | read-only |
| Unplanned-read events | `docs/specs/unplanned-read-events-v0.md` | `reads` | opt-in private ledger |
| Compaction kernel | `docs/specs/compaction-kernel-v0.md` | `kernel` | read-only |

Boundaries this decision does not move:

- Invariant 4 is unchanged. `lease` and `reads enable` are explicit mutations of private derived
  state under `.corvint/`, in the same class as `record`; `.corvint/leases/` and the opt-in
  `.corvint/unplanned-reads.jsonl` are never inputs to ranking, learning, evidence, or authority.
  The unplanned-read spec proposes the exact invariant-4 sentence; it is not applied here.
- Invariant 1 is extended only as a proposal: dependency evidence pins content by the repository's
  own `go.sum` `h1:` hash. Acceptance of that extension is a separate decision.
- Invariant 5 is unchanged. `calibrate` and `answerability` emit proposals labelled
  `applied: false`; no threshold moves without the learned-trace admission gate.
- Invariant 7 is unchanged: no daemon, network, embeddings, database, or UI.

Two harness call sites are deliberately left unwired, so the unplanned-read and compaction-kernel
slices ship as a package plus a verb only:

- `unplannedread.HookPostTool` has no caller in `cmd/corvint/host_adapter.go`. The post-tool branch
  holds no delivered packet, so the planned-path denominator would be empty and every read would
  score unplanned. Supplying it needs a persisted planned set, a second write that waits on the
  invariant-4 amendment this decision only proposes.
- `compactionkernel.InjectionBlock` is not called from `compactionContext` in
  `cmd/corvint/harness_context.go`. Injecting into the default session-start and user-prompt context
  would promote an experimental slice into the shipped agent path, against invariant 8.

The roadmapped programs named in the same assessment (MCP verb parity, runtime evidence, sandbox
containment, additional language providers, in-process Git) are not covered by this decision. The
owner's "build it all out" lifts the outside-demand trigger on MCP verb parity as a direction; its
own specification and gate follow in a later wave and are not delivered or advertised by this one.

Rollback: delete the eight verbs, packages, and specs; no persisted state outside `.corvint/` exists.
