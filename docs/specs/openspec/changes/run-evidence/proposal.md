# Change proposal: an append-only store of test run results

Change name: `run-evidence`
Ticket: none filed yet; to be filed in `.taskman/` before dispatch (not filed by this proposal).
Product requirement served: PROD-TEST-020 in `docs/specs/PRODUCT-PLAN-1.0.md` (after
`corvint test`, `corvint flows status` with no `--evidence` shows that run), and the run results
that PROD-TEST-018 and PROD-DOCS-012 reports join.
Owning capability spec: a new spec `docs/specs/run-evidence-v1.md` (prefix `RVD-V1`). It does not
exist yet; the lane creates it in its first commit.
Requirement IDs this change adds: RVD-V1-001 to RVD-V1-012. `RVD-` has no entry in
`docs/specs/REQUIREMENTS.tsv` and appears in no other folder under `docs/specs/openspec/changes/`
(checked 2026-11-09).
Intent status: proposed (written 2026-11-09; only the owner can accept it)
Delivery status: not-started
Written before dispatch: yes.

## Needs first

- `canonical-test-id`: records are keyed by the canonical test ID (CTI-V1-009).
- `test-inventory`: records name the runner instance (TIV-V1-003).

This change is third in the build order fixed by owner decision R38 (2026-11-09).
`per-test-durations` lands with it, so every record carries per-test durations where measured.

## Why

Owner decision R7 (2026-11-09) approves an append-only run result record whose receipt ID is the
SHA-256 of the file bytes. G24 (owner answer) removes `corvint runs` and `internal/playwrightruns`
and keeps `internal/testevidence` with `corvint test-evidence` and `flows test-evidence`, labelled
preview. So the record must be read by existing commands, not by a new listing verb.

Review finding testing T-B2 asks for an append-only, bounded, private store keyed by canonical ID and
tree, one write per test per run, reports that read it by default, retention, a size cap and a
defined failure for a corrupt store. Review finding NEW-testing-6 places it under
`<git-dir>/corvint/evidence/` and keeps about the last 20 results per test.

### What exists at integ 9ec3a29d0

- `corvint test` writes one receipt at `<git-dir>/corvint/test-run.json` (or `test-run.N.json` per
  shard) (`cmd/corvint/test.go`, lines 1018-1031, integ 9ec3a29d0). It writes a temporary file and
  renames it over the previous receipt, so each run replaces the last
  (`internal/sessiontest/manifest.go`, lines 253-272, integ 9ec3a29d0). Profile `corvint-test-run/0`,
  at most 1 MiB (`internal/sessiontest/manifest.go`, lines 26-29, integ 9ec3a29d0).
- `flows ingest` converts Playwright JSON, a Playwright receipt, JUnit XML or `go test -json` into
  `test-run-evidence/0` JSON lines printed to stdout; it writes no store
  (`internal/command/flows/flows.go`, lines 365-430, and `internal/appflows/runingest.go`, lines
  25-30, integ 9ec3a29d0).
- `flows map`, `flows gaps`, `flows stability` and `flows status` read run evidence only from files
  named with `--evidence` (`internal/command/flows/flows.go`, lines 239-328 and 470, integ
  9ec3a29d0).
- The top-level verb list still names `runs` (`cmd/corvint/main.go`, lines 644-646, integ
  9ec3a29d0); G24 removes it in change `source-removal`.

## What changes

- A private store at `<git-dir>/corvint/evidence/`: each record is the exact bytes of one run
  document, stored once at `records/<sha256>.json`, where the name is the SHA-256 of those bytes; an
  append-only index `log.jsonl` lists records in write order (RVD-V1-001, RVD-V1-002).
- The record ID is that SHA-256, the same digest `corvint-tasks receipt put FILE` uses for a
  witness receipt, so a run record can be attached to a ticket unchanged (RVD-V1-003).
- Writers: `corvint test` (and `corvint watch` runs, and the GitHub Action through `corvint test`)
  append one record per run; `flows ingest` appends a record for an external report when asked
  (RVD-V1-004, RVD-V1-005).
- Each record holds one outcome per test per run, keyed by canonical test ID, with the tree, the
  runner instance and the per-test duration when measured (RVD-V1-006).
- Readers by default: `corvint flows status`, `flows map`, `flows gaps`, `flows impact`,
  `corvint test-evidence` and the matching MCP tools. `--evidence FILE` still adds files
  (RVD-V1-007). No new listing verb; `corvint runs` stays removed (RVD-V1-008).
- Retention, size cap and corrupt-store behaviour (RVD-V1-009, RVD-V1-010).
- Run evidence never narrows selection (RVD-V1-011). Writes stay in writing verbs (RVD-V1-012).

## Non-goals

- No `corvint runs` verb and no other listing verb (G24).
- No shared or remote store; the store is local and private under the git directory.
- No change to `corvint-test-run/0` beyond add-only fields (settled D25).
- No use of run results to remove a test from a plan.
- No change to the Tasks witness-receipt format; this change only uses the same digest.

## Impact

- New spec `docs/specs/run-evidence-v1.md`; generated rows in `docs/specs/REQUIREMENTS.tsv`.
- `cmd/corvint/test.go` and `internal/sessiontest` append a record after writing the receipt.
- `internal/appflows` gains a store reader used by `flows status`, map, gaps and impact.
- `flows ingest` gains a store-append option.
- `corvint test-evidence` reads the store by default and stays labelled preview (G24).
- `per-test-durations` supplies `durationMs`; `flows-status-report` (AFL-V1-002) reads receipts and
  ingested evidence through this store.
