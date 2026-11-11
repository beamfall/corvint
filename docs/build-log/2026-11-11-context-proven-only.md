# Context proven only: the cut (decision 0597, change `context-proven-only`)

Owner direction 2026-11-11: keep the context surfaces with evidence behind them and cut the rest.
This entry records what the change removed, what it kept, the evidence each step produced and
what it left. Base: `b55635262788687c3eea9a08a94378e4ab8856a4` (origin/main). The base tree is the
last one that holds every removed file; historical citations name it.

## Kept

`affected`, `test`, `watch`, `impact`, `index`, `cem`, the MCP impact and test tools, abstention
and the docs product. The instruction doctor is now `corvint instructions`; the
`context --instruction-*` spelling stays as an alias with a deprecation line (IID-V0-001).
`context --path` keeps three sections: `instructions`, `tests` and `requirements`, profile
`corvint-path-brief/1` (TBC-V1-009 to TBC-V1-011). `review` keeps affected advice, branch overlap
and authority labels (RGV-V0-001, RGV-V0-007).

## Removed, by step

| Step | Commit | Removed | Lines (+/-) |
|---|---|---|---|
| 1 | `60dd5419d` | `context --task`, `--worktree-overlay`, `feature`, `features`, `overview`, their help text, skills, commands and process-doc mentions, and the CCF-V1 `context --task` row and goldens | +666 / -2,988 |
| 2 | `b7f71667b` | path-brief `owners`, `flows` and `tickets` sections and their readers; doctor moved under `instructions` | +347 / -900 |
| 3 | `5b66b908e` | `tools/retrieval-bench`, `tools/cw-trial`, `tools/heading-nav-bench`, `tools/paired-trial`, the blind and case benchmark sets and their inventory rows | +30 / -25,391 |
| 4 | `2695ec5fa` | `internal/trace`, `internal/tracerecordrepo`, `internal/tracerepopaths`, `internal/unplannedread`, and the `internal/contextindex` ranking, packet and task-context code that no kept command reaches | +566 / -33,622 |
| 5 | this commit | spec retirement and amendments (no code) | docs only |

Reachability for step 4 used an RTA dead-code pass over `./cmd/...` before and after each cut: code
was removed only when it was live at base and dead once its last command was gone. Live helpers
that share files with the cut code stay (`pack_cochange.go`, `pack_termtable.go`; `taskcontext.go`
survives as `source_helpers.go`).

## Specs

- Retired (header, README row and digest): TCP-V0, CRK-V0, LTA-V0, URE-V0, RBD-V0, CWT-V0, HNE-V0,
  CEP-V0, TRJ-V0. Bodies stay as history and their requirement IDs stay for traceability.
- Amended: CCF-V1-002 (`context --task` leaves the frozen set; `context` stays a Core verb),
  IID-V0-001, TBC-V1-009 to TBC-V1-011, RGV-V0-001 and RGV-V0-007. Retired inside a live spec:
  RGV-V0-003 to RGV-V0-006, RGV-V0-955 and RGV-V0-956.
- `make spec-requirements` regenerated the index; a second run reported no changes.

## Evidence

- Each step: `go build ./...` ok, `go vet ./...` clean, `gofmt -l` empty.
- Focused tests through `script/heavy.sh`, one package per command: `internal/contextindex` ok
  (355 s), `internal/closureguard` ok, `tools/affected-edit-oracle` ok, `internal/extevidence` ok,
  `internal/pathbrief` ok, `conformance/use-cases-v0` ok, and `cmd/corvint` with
  `-run 'Context|Snapshot|Impact|Index|Freeze|Help|Instruction'` ok (53 s).
- Doc checks: line citations pass (11 dead citations rewritten to the historical form, the rest
  re-pinned), test-read-scopes check pass, use-case receipts current (11), capability catalog ok,
  agents invariants check and `openspec validate --all` as recorded in the lane handback.

## Not run and not done

- NOT_RUN: the full `cmd/corvint` package (about 591 s; the lane ran the focused subset above),
  `go test ./...` and `make gate` (not required for this lane).
- NOT_DONE: AGENTS.md invariants 4 and 5 still name the trace and unplanned-read ledgers and the
  learning step; the owner keeps that edit (decision H5).
- NOT_DONE: the index still builds and serializes the identifier graph, symbol windows and term
  table, which no kept command reads. Removing them changes the pack format and needs its own
  format gate.
- NOT_DONE: historical harnesses and fixtures that name the removed surfaces are left as history
  (the daily-loop corpus, preregistrations, `benchmarks/taskcontext-lookup`, `qualify-lsp.py`), as
  are negative tests that assert the removed verbs refuse.
- Removed coverage stays removed where its product code is dead: `readCleanFile`, the analyzer
  bounds and the test-link map equality test.
- The CI shard tables are edited on the separate branch `claude/ctxcut-cishards`
  (`.github/**` changes go in their own PR).

## Fix round 1 (review FIX)

- DELEGATED 1.0 CHOICE (orchestrator, not an owner acceptance): the `UC-TASK-ORIENTATION` Core job
  is retired by decision 0597, because every kept surface takes a path or a diff and none answers a
  task-text orientation question. `corvint-1.0-product-and-release-v1.md` and the
  `benchmarks/untouched-repository-v1` README say so. `harness.py` is not edited, since its sealed
  digest is bound by the preregistrations. No threshold or bar changed.
- The `context` alias accepts `--task`, `--limit` and `--worktree-overlay`, ignores them and warns on
  stderr (IID-V0-001); `TestContextAliasIgnoresTheRemovedPacketFlags` covers it.
- `LTA-V0-004` and `LTA-V0-014` to `-020` (except `-018`) moved to `secret-screen-writer-v0.md`, which
  governs the live `internal/secretscreen/prefilter.go`; the retired spec points to it.
- NOT_DONE: `corvint-path-brief/1` stays `/1`. `ticket-brief-v1.md` (amended 2026-11-11) states that
  the profile MUST be `corvint-path-brief/1`, so a bump needs its own spec change and owner call.

## Rollback

Revert the step commits in reverse order. The removed packages, tools and specs return unchanged
from the base tree; no stored state was migrated, and an existing `.corvint/unplanned-reads.jsonl`
or trace store is left in place untouched.
