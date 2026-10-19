# 2026-10-19 V1-1973: typed flow-level relations in the documentation corpus

Ticket: V1-1973 (GitHub #1225).

## Decision

`DCP-V1-093` to `DCP-V1-099` are added to `docs/specs/documentation-corpus-v1.md` and accepted under
the owner's standing auto-accept of lane requirements (2026-10-11). No accepted requirement is
amended.

- Separate provider schema and artifact member. Flow relations use their own closed
  `corvint-corpus-flow-relations/1` record and a separate `flow_relations` artifact member, as
  test-knowledge records do. Generic `Relation` records were not reused: they need evidence on every
  edge and have no review state or event/job/system target, so an unreviewed declaration could not
  be stored without being traversable.
- Traversal at read time. The artifact keeps only the joins. `impact`, `affected` and `trace` walk
  them with a shared breadth-first function, so the depth is a read argument and the artifact bytes
  do not depend on it.
- Produces/consumes as one hop. `A produces J` with `B consumes J` is one flow hop from A to B whose
  path names J (`A -> J -> B`). Counting J as its own hop would halve the useful depth.
- Fail closed on review. Only a joined, reviewed record is traversed. Any other record that would
  extend the walk is listed in `withheld` with its reason and adds an impact unknown. It is never
  traversed.
- Authority. The trace is labelled `project-declared-corpus` with a limitation that it ranks below
  project authority. Selection keeps `narrowing_allowed: false`.
- MCP. The existing `corvint.docs_trace` tool gains an optional `depth` argument; no new tool name
  was added. `corvint docs corpus trace` has no depth flag and uses the default (recorded limit).

## Evidence

Focused tests: `go test ./internal/doccorpus/ ./internal/mcp/corpusbridge/ ./internal/corpusrepublish/`
and the `cmd/corvint` corpus tests selected by `corvint affected`. The acceptance case,
`TestDCPV1096FlowRelationImpactAttribution`, builds a corpus where flow A produces `job:J` from an
anchored source and flow B consumes it. A change on A's anchored path reports B at depth 1 with
path `adapter:flow -> job:J -> adapter:flow-b`, then C through `depends-on` and an external system
at depth 3. `TestDCPV1097FlowRelationUnreviewedAndDepth` shows that the unreviewed `depends-on`
from D is withheld, never reached, and kept as an unknown. No live qualification on a real
repository corpus was run (NOT_RUN).
