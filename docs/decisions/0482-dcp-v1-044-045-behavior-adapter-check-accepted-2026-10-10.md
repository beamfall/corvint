# Decision 0482 — behavior-adapter `--check` report and anchor guide (DCP-V1-044..045) accepted

Date: 2026-10-10. Status: accepted by the owner. Authority: owner approval in chat, 2026-10-10
("accept").

## Context

GitHub #717 reported friction with `corvint docs corpus behavior-adapter`. The adapter-check lane
proposed two requirements in `docs/specs/documentation-corpus-v1.md`:

- `DCP-V1-044`: a `--check` mode that reports every refusal a build would make, item by item, in a
  bounded report, without emitting or writing an adapter result. It accepts exactly when the build
  would succeed.
- `DCP-V1-045`: the behavior-adapter guide states the input-anchor placement rule and gives a minimal
  schema-2 migration identity example, which a test builds into an accepted request.

Independent review raised findings in rounds 1 to 7, and round 8 passed. AGENTS.md invariant 8 keeps acceptance
human-owned.

## Decision

The owner accepts `DCP-V1-044` and `DCP-V1-045` as written. The requirement statuses, the
`docs/specs/README.md` row and the lane's build log (`docs/build-log/2026-10-10-gh717-adapter-check.md`)
record the acceptance. The spec's overall intent status stays proposed: its other requirements are
not accepted by this decision.

## Limits

This decision settles intent only. The evidence is focused tests, the doc gates and the retention
audit in the lane's build log. Live qualification is `NOT_RUN`.

- The check report does not list every independent refusal within one item, and a repaired
  refusal can reveal stages that were not evaluated before. The report says so.
- Build still retains an unbounded reconciliation frontier and still builds amplified key sets
  while running. The CLI then refuses at encoding. Check mode does neither. V1-1089 tracks the Build side.
- V1-1085 (the legacy-key alias) stays open. The frozen `/1` bytes are unchanged, and no alias is added.
- The producers half of #717 stays open. Native tickets V1-1084 and V1-1086 are not completed by this
  decision.

## Rollback

Revert this decision and return the DCP-V1-044 and DCP-V1-045 status text to proposed, pending owner
acceptance. To withdraw the behavior, also revert the adapter-check change: `CheckBehaviorAdapter`
and its report types, the check-mode skips in `internal/doccorpus/behavior_adapter.go`, the
`--check` flag in `cmd/corvint/docs_corpus.go` and the guide additions in
`docs/DOCUMENTATION-CORPUS.md`. Then regenerate `docs/specs/REQUIREMENTS.tsv`. No stored state
changes: `--check` writes nothing, and Build's bytes are unchanged.
