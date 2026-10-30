# Decision 0102 — the learned-trace path screen is the index snapshot screen

Date: 2026-09-12. Status: accepted. Authority: repository owner delegation to make owner calls
and record them (orchestration thread, 2026-09-12).

`internal/trace/record.go` and `internal/dashboard/adapters/trace.go` each held a copy of the path
screen the index used before `.claude` joined it. Decision 0095 made the index screen spec-owned as
`IDX-SNAP-V0-018` and left the trace copies ungoverned, so the two could drift and a trace path
screen admitted `.claude/` paths the index refuses.

Call: learned-trace path admission adopts `IDX-SNAP-V0-018` exactly. `internal/contextindex`
exports `ForbiddenPathReason`, a thin wrapper over the index screen, and the trace record, stored-row
read, and dashboard trace adapter consume it; both copies are deleted. The export is an
`internal/contextindex` production change, so `analyzerSchemaID` moves to `corvint-analyzer/27`
under `IDX-SNAP-V0-017`; analyzer facts and encoding are otherwise unchanged. `LTPM-V0-011` and
`IDX-SNAP-V0-018` both state the shared screen.

Alternative set aside: keeping a separate trace set would need its own clause and a stated reason
for admitting a path the index refuses; none exists, since learned-path candidates can only rank
index sources (`GPK-V0-044`).

Consequences:

- `record` outcomes are unchanged for every path except a tracked `.gitignore` under a screened
  directory (for example `.claude/.gitignore`), which is now refused. A supplied `.claude/` source
  path was already refused as not an index source; it is now refused as `forbidden path`.
- A stored schema-version-1 row naming a screened `.claude/` path now fails closed on read. Only a
  repository that tracks `.claude/` content and recorded such a path is affected; Corvint tracks none
  (`git ls-files .claude` is empty).
- A future amendment to `IDX-SNAP-V0-018` changes learned-trace admission in the same commit.

Rollback: revert this decision's commit. That restores the two local path lists, the
`corvint-analyzer/26` pins, and the backlog entry, and removes the export, the pinning test, and the
two spec paragraphs.
