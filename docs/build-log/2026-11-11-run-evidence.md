# 2026-11-11: run evidence store (V1-5448)

Change `run-evidence`, tasks 1, 2, 3, 5, 6, 7 and 8; task 4 stays open (see 2). Requirements
RVD-V1-001 to RVD-V1-012 in `docs/specs/run-evidence-v1.md`. Third change of the R38 testing
order; serves PROD-TEST-020 and the run results PROD-TEST-018 and PROD-DOCS-012 join.

## What changed

- `internal/runevidence` (shared layer, standard library only; one prefix line in
  `tools/import-ratchet/areas.json`): the private store at `<git-dir>/corvint/evidence/`, records
  named by the SHA-256 of their bytes, `log.jsonl` appended under `flock`, retention of 20 records
  per test and a 64 MiB cap applied only by writers, corrupt-record and unreadable-log handling,
  and the normalisation of both run documents to rows.
- `corvint test` appends each shard receipt after writing it; `flows ingest --record` appends its
  document. `flows status`, `map`, `gaps` and `impact`, `corvint test-evidence` (and `flows
  test-evidence`) and the MCP flows tools read the store by default; `--no-store` opts out.
- Acceptance-log row `run-evidence` (kind `change`, mode `auto`); the spec marks each requirement
  with the standing auto-accept marker (G14, decision 0596).

## Differences from the proposal, followed by intent

1. `canonical-test-id` and `test-inventory` are not merged on this tree (lanes ctid and tiv). A
   receipt row is keyed by the outcome's `testId` when a receipt carries it and otherwise by its
   unit, which CTI-V1-012 reads at the unit's granularity; so the rows join ctid's IDs once it
   lands, with no record rewritten. The runner instance is the adapter and runner
   (`go`, `declared:<command>`, `provider:<runner>`) until TIV-V1-003 instance IDs exist.
2. `corvint watch` does not run tests through `corvint test`: it launches `go test` itself and
   writes no receipt, so it has no run document to append. Inventing a receipt for it would bind a
   manifest digest no manifest has. Its append is not yet built and task 4 stays unticked; the
   `corvint test` half and the task's stated proof are delivered. No GitHub Action is on this tree.
3. `corvint test` units are packages and files on this tree, so every receipt row is granularity
   `file`. Per-test rows come only from ingested documents.
4. The log line adds `tests` and `bytes` to the design's members. `tests` lets a reader name the
   tests of a record that no longer hashes to its name (RVD-V1-010 requires its tests be shown);
   `bytes` lets the writer apply the cap without reading every record.
5. `--evidence FILE` exists on the flows readers only. `corvint test-evidence` has no run-evidence
   file input, so it gains `--no-store` alone; its MCP tool and the MCP flows tools read the store
   with no new argument, so the frozen tool schemas (lane mtt's table) keep their members.
6. Report additions are add-only and keep existing bytes when the store is empty or not read:
   `source_tree` and `reason` on a status test (omitempty), `run_evidence` on the status report,
   `last_results` per impact flow only when the store holds a record, and `runs` on the
   test-evidence document. The one existing whole-struct status assertion
   (`TestV15257StatusReportsAttachedTestsAndLastResults`) is extended with the record's tree.
7. A failed `corvint test` append keeps the exit code (Open 2 left as proposed). `flows ingest
   --record` exits 2 on a failed append because ingest is not a test run and its only job with
   `--record` is the write. The cap is 64 MiB (Open 1 as proposed); ingest appends only with
   `--record` (Open 3 as proposed).
8. `corvint runs` still exists until change `source-removal` deletes it, so the RVD-V1-008
   scenario "the command is unknown" is not reachable here. The test proves what this change owns:
   no verb besides the named writers and readers mentions the store, and `runs` does not read it.
9. STR-V0-011 named `--fail-fast-order` as the one receipt reader besides `verify-shards`, and its
   guard `TestSTRV0011ReceiptsHaveNoOtherReader` refuses a new reader without an accepted
   requirement. RVD-V1-006 and RVD-V1-007 are that requirement; STR-V0-011's text and the guard's
   site list now name the store, for reports only.
10. The store follows `git rev-parse --absolute-git-dir`, as the receipt does, so each linked
    worktree has its own store.

## Evidence

- `script/heavy.sh go test -count=1 ./internal/runevidence/`: ok.
- `./internal/command/flows/`, `./internal/appflows/`, `./internal/testevidencedoc/`,
  `./internal/mcp/bridge/`, `./internal/mcp/toolgroup/`, `./internal/capcatalog/`,
  `./tools/import-ratchet/`: ok.
- `./cmd/corvint/` with `-run 'Help|Catalog|Capab|TestEvidence|CMSV09|STRV0|CommandTree|FlagAlias|RSPV0051|RVDV1'`: ok.
- `make traceability-tests-check`, `make capabilities-check`: ok.

## NOT_RUN

- RVD-V1-003's `corvint-tasks receipt put` digest check: `receipt put` is a Tasks write command,
  which the delivering lane may not run. The digest rule is the plain SHA-256 of the file bytes,
  checked by `TestRVDV1001HashNamedRecordsAndLog`.
- `openspec validate run-evidence`: the OpenSpec CLI is not installed on this host.
