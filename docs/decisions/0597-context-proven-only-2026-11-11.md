# Decision 0597: context keeps only the proven parts

- Date: 2026-11-11
- Status: accepted as owner direction (owner chat, 2026-11-11, relayed by the orchestrator). No
  owner acceptance row is written by the lane that carries this file; the change
  `docs/specs/openspec/changes/context-proven-only/` stays `proposed` until the owner accepts it.
- Authority: owner direction, 2026-11-11, quoted below.
- Completes: decision 0594 item 1 (CUT-CONTEXT), which removed the context, query and review-pack
  families but kept the verbs `context`, `feature`, `features`, `overview` and `review`.
- Amends: `docs/specs/core-compatibility-freeze-v1.md` (CCF-V1-001 and the `context --task` wire
  rows; owner-directed), `docs/specs/ticket-brief-v1.md` (TBC-V1-009 to TBC-V1-011, the path brief),
  `docs/specs/agent-instruction-doctor-v0.md` (IID-V0-001, the verb), and
  `docs/specs/repository-guidance-v0.md` (the `features` and `overview` requirements).
- Retires: `task-context-packet-v0` (TCP-V0), `context-ranking-v0` (CRK-V0),
  `learned-trace-admission-v0` (LTA-V0), `unplanned-read-events-v0` (URE-V0),
  `retrieval-bench-diagnostics-v0` (RBD-V0), `confidently-wrong-trial-v0` (CWT-V0),
  `heading-navigation-experiment-v0` (HNE-V0), `context-evolution-program-v0` (CEP-V0) and
  `trial-replay-journal-v0` (TRJ-V0).
- Resolves: V1-5246 (keep or cut `context`, `feature`, `features`, `overview`, `review`) and the
  open path-brief questions of V1-5299, toward the reduced forms below.
- Read with: decisions 0594, 0595 and 0596; `docs/build-log/2026-11-11-context-proven-only.md`.

## Context

Decision 0594 cut context injection and kept a handful of context verbs while the product plan was
settled. Decision 0596 then revived `impact`, `review` and `context` "as proposed" and folded
`feature`, `features` and `overview` into flows (G25). What stayed on `origin/main` after both is
still a mix: a deterministic path brief and an instruction diagnostic beside a ranked task packet,
a regex feature inventory, a ranking stack in `internal/contextindex`, a learned-trace pipeline with
no producer, and the development benchmarks that only scored the ranking.

## Owner direction

Owner, 2026-11-11, verbatim: "for context we should focus on the parts that are proven to work, and
scrap the rest".

## Evidence

Outside evidence. The following is the orchestrator's research summary as relayed to the lane; the
lane could not confirm these figures or URLs through citations already in this repository, so each
is labelled as that summary, not as a verified quote.

- Supports test selection from a diff: Meta predictive test selection; Google TAP (orchestrator's
  research summary).
- Supports giving the agent a runnable reproduction or a failing test: ORACLE-SWE, about +44 points
  (orchestrator's research summary).
- Supports giving the agent the exact edit locations: about +14 points (orchestrator's research
  summary).
- Supports project-owned rule and instruction files the agent must follow (orchestrator's research
  summary).
- Against broad precomputed context: the ETH Zurich `AGENTS.md` study, arXiv 2602.11988, found no
  gain, about +20% cost, and -0.5 to -2 points for LLM-written context files; SWE-Explore found
  BM25 or RAG retrieval near random for locating edits; Chroma "Context Rot" found quality falls as
  context grows; summaries in place of source scored 4/45 against 27/45. arXiv 2604.07789 and
  2606.07297 are cited in the same summary (orchestrator's research summary).

Corvint's own evidence, in this repository:

- The value audit of 2026-10-26 found that context injection cannot pay for itself; the V1-2757
  replay was killed (cited by `docs/build-log/2026-11-10-context-replay-and-link-coverage.md`).
- The replay of 2026-11-10 in that build-log entry: on 16 merged pull requests the ranked
  `corvint context --task` named 9 of 39 changed files and a plain term grep named 13.
- The link-coverage check of 2026-11-10
  (`docs/build-log/2026-11-10-path-brief-link-coverage-check.md`) met the owner's bar for the
  `tests` and `requirements` sections of the path brief (16 of 20). No measurement exists for the
  `owners`, `flows` or `tickets` sections.
- Decision 0594 CUT-CONTEXT (2026-11-09) already removed the context, query and review-pack families.

Reading: what is proven is a diff-to-tests answer, the tests and requirements a path is linked to,
and the instruction files a host loads. Ranked, inferred or summarised context is not proven and,
on this repository, does not beat grep.

## Decision

### Disposition

| Surface | Disposition | What stays | What goes |
| --- | --- | --- | --- |
| `affected`, `test`, `watch`, `impact`, `index`, `cem` | KEEP unchanged | everything | nothing |
| MCP `impact`, test-evidence and test-validity tools | KEEP unchanged | everything | nothing |
| Abstention, `unknown`, `NOT_OBSERVED` states | KEEP unchanged | everywhere they occur | nothing |
| Instruction doctor (`context --instruction-*`) | KEEP, re-homed | the load-set prediction under its own verb `corvint instructions` | the `--task` prerequisite; `context --instruction-*` stays one release as an alias with a stderr deprecation line |
| `context --path` (path brief) | SPLIT | `instructions` (authority), `tests` (linked tests and plan verdict), `requirements` (kept links), each capped at 64 with state, reason and unknowns | `owners`, `flows`, `tickets`, and the completed-ticket and claim-scope prefix matches |
| `review --base` | SPLIT | the affected compact receipt and its digest, the declared repository gate, the deterministic branch overlap and skipped refs | the inferred feature list and `changedFeatures` |
| `context --task`, `context --worktree-overlay` | CUT | nothing | the ranked task packet and its flags |
| `feature`, `features`, `overview` | CUT | nothing (0596 G25 folds them into flows) | the verbs, the regex feature inventory, `internal/genesis/guidance.go` where only they read it |
| Ranking code in `internal/contextindex` | CUT | every file and symbol a KEEP user still reaches | the task-packet compiler and whatever only it reaches |
| `internal/trace`, `tracerecordrepo`, `tracerepopaths`, `unplannedread` | CUT | nothing | the packages |
| `tools/retrieval-bench`, `cw-trial`, `heading-nav-bench`, `paired-trial` | CUT | nothing | the tools |
| `benchmarks/blind-v2` to `blind-v7`, `benchmarks/cases`, the frozen ranking summary, `testing/context-retrieval-goldens.json`, `script/measure-context-packet.py` | CUT | nothing | the files |
| Docs search, `internal/mcp/corpusbridge`, `corvint-docs mcp`, `flows`, `how` | KEEP (docs product) | everything | nothing; decision 0594 item 3, decision 0595 and decision 0596 R30 assign them to the docs product |
| `internal/contextindex` as a substrate | KEEP | snapshot and pack format, impact, index, watch, setup selection, adopt, work-queue collision, doc corpus, external evidence, app map | only code that no KEEP user reaches |

Rule for the shared substrate: a file named for cutting that a KEEP user still reaches stays, and
the build log names it and the user. In particular a section of the persisted snapshot pack is
not removed by this decision: changing the pack encoding is a format change that needs its own
benchmark and format gate (invariant 7).

### Pending changes this agrees with

The proposed changes `review-evidence-check` (review as a deterministic evidence check),
`ticket-brief-context` (TBC-V1-005, ranking code removed), `feature-map-from-flows` (the feature
map from flows) and `source-removal` (B6 and B7 remove the benchmarks, the bench tools and the
trace packages) point the same way. This decision does the removals now; those changes keep their
other tasks.

### Not changed

- The ticket form of the brief (`corvint context --ticket ID`, PROD-INT-030, TBC-V1-001 to
  TBC-V1-008) is an owner-accepted product requirement of decision 0596 and is not built. This
  decision neither builds nor retires it; whether the owner direction of 2026-11-11 retires it is
  an open owner question.
- `AGENTS.md` product invariants 4 and 5 already carry the wording of decision 0596 (no trace or
  learning ledger, "Corvint 1.0 learns nothing from use"), made by change `acceptance-record`
  (ACR-V1-012). Owner decision H5 (2026-11-10) makes that change their only editor, so this
  decision does not edit them.

## Wire compatibility and the CCF-V1 freeze

`docs/specs/core-compatibility-freeze-v1.md` froze `context --task` (`task-context-packet-v0`) and
`feature`. Their removal is an owner-directed contract change under the quote above, not a
regression: CCF-V1-001 loses `feature` and the `context --task` row, and the frozen goldens under
`cmd/corvint/testdata/core-freeze/` for `context --task` are removed with the surface. The
`impact` rows and goldens are unchanged. A caller of a removed verb or flag gets the existing typed
usage refusal and exit 2. `context --instruction-*` keeps its output for one release and prints one
deprecation line to stderr; `corvint instructions` prints the same `instruction_load_set` object.
The path brief keeps its JSON shape for the three sections it keeps; the three cut sections are
absent, and the profile becomes `corvint-path-brief/1` so a reader can tell the forms apart.

## Rollback

Revert the merge commit that carries the change. Nothing is migrated: no store, ledger, snapshot,
ticket or `.corvint/` file is rewritten or deleted by this change, and the snapshot pack format is
unchanged, so an index written before or after reads the same.

## Non-goals

- No new context feature, ranking or replacement retrieval.
- No change to the docs product, Tasks, flows or `how`.
- No change to the snapshot or pack format.
- No benefit claim for the kept path brief beyond the recorded link-coverage check.
- No edit of `AGENTS.md` product invariants (owner decision H5).
- No `.github` change in the same branch; the CI shard tables follow in their own branch
  (decision 0594 item 7).
