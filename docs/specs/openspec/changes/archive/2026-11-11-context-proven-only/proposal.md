# Change proposal: context keeps only the proven parts

Change name: `context-proven-only`
Tickets: V1-5246 (keep or cut the context verbs) and V1-5299 (the path brief owner questions). No
new ticket is filed by this change.
Owner direction: 2026-11-11, verbatim: "for context we should focus on the parts that are proven to
work, and scrap the rest".
Companion decision: `docs/decisions/0597-context-proven-only-2026-11-11.md`.
Owning capability specs: `docs/specs/ticket-brief-v1.md` (TBC-V1-009 to TBC-V1-011, modified),
`docs/specs/agent-instruction-doctor-v0.md` (IID-V0-001, modified),
`docs/specs/repository-guidance-v0.md` (RGV-V0-001, modified; RGV-V0-003 to RGV-V0-006, RGV-V0-955
and RGV-V0-956, removed), `docs/specs/core-compatibility-freeze-v1.md` (CCF-V1-002, modified), and
the retired families TCP-V0, CRK-V0, LTA-V0, URE-V0, RBD-V0, CWT-V0, HNE-V0, CEP-V0 and TRJ-V0.
Requirement IDs this change adds: none. Every ID it touches keeps its number; a removed ID is
listed as retired and never reused.
Intent status: proposed. Delivery status: experimental until the owner accepts the change.

## Why

Decision 0594 cut context injection but kept `context`, `feature`, `features`, `overview` and
`review` while the plan settled. The surviving code mixes surfaces with evidence behind them (the
path brief's `tests` and `requirements` sections, the instruction load set, `review`'s affected
receipt) with surfaces that have none or have evidence against them (the ranked task packet, the
regex feature inventory, the ranking stack, the learned-trace pipeline and its benchmarks). The
owner direction above asks for the second group to go. Decision 0597 records the evidence.

## What changes

- `corvint context --task` and `--worktree-overlay` are removed. `corvint context --path` keeps
  only the `instructions`, `tests` and `requirements` sections (profile `corvint-path-brief/1`).
- `corvint instructions` prints the instruction load set without a task. `corvint context
  --instruction-*` stays one release as an alias and prints a deprecation line on stderr.
- `corvint feature`, `features` and `overview` are removed. `corvint review --base` keeps the
  affected receipt, its digest, the repository gate and the branch overlap, and drops the inferred
  feature list.
- The ranking code in `internal/contextindex` that only the task packet reached, the trace and
  unplanned-read packages, the context bench tools and the context benchmarks are deleted.
- The specs of the removed families are marked retired; CCF-V1 records the removal of the
  `context --task` mode as owner-directed.

## Non-goals

- No new context feature, ranking or retrieval. No change to the docs product (docs search,
  `corvint-docs mcp`, `flows`, `how`) or to Tasks.
- No change to the snapshot or pack encoding: a pack section that the index still writes or reads
  stays even when the charter named its file for cutting.
- No edit of `AGENTS.md` product invariants: change `acceptance-record` is their only editor (owner
  decision H5), and invariants 4 and 5 already carry the decision 0596 wording.
- The ticket form of the brief (PROD-INT-030) is neither built nor retired here.
- No `.github` change in the code branch; the CI shard tables change in their own branch.

## Failure modes

- A KEEP user (impact, index, watch, setup selection, adopt, work-queue collision, doc corpus,
  external evidence, app map, MCP impact) loses a symbol: the build fails. The lane keeps the file
  and records it.
- A caller still runs a removed verb or flag: it gets the existing typed usage refusal, exit 2.
- A reader of the path brief expects six sections: the profile change to `corvint-path-brief/1`
  tells it the form changed.

## Acceptance evidence

`go build ./...`, `go vet ./...` and `gofmt -l` clean after each step; focused tests of each edited
package that still exists; `make windows-vet`; `make spec-requirements` with no diff after;
the line-citations check, the `AGENTS.md` invariant check and `openspec validate --all`.

## Rollback

Revert the merge. Nothing is migrated or deleted from any store, ledger, snapshot or ticket.
