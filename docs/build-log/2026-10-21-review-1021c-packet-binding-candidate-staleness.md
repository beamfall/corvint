# review-1021c: review packet checkpoint binding, history cursor and candidate staleness

Date: 2026-10-21
Tickets: V1-1921, V1-1922 (lane review-1021c). V1-1463 already delivered; V1-2132, V1-2107,
V1-1840 and V1-2036 skipped (below).
Requirements: ERG-V0-026, ERG-V0-027 (`docs/specs/corvint-tasks-external-reviews-v0.md`, with the
ERG-V0-021 wording amended) and TOL-V0-131 (`docs/specs/corvint-tasks-obligation-ledger-v0.md`, with
TOL-V0-077 and the non-goal amended). Accepted under the owner's standing auto-accept of lane
requirements (2026-10-11).

## Problem

- V1-1921: the `gate packet` checkpoint always reported `commitBinding: NOT_OBSERVED`, because the
  hand-off note had no way to name a commit, and review history past 4 x 50 events per gate was only
  named in `omitted`, with no command a reviewer could run to read the rest.
- V1-1922: `obligations show` judged reviewed-witness staleness at store `HEAD`, so a review could be
  counted fresh for a candidate that changed the cited example, or stale because `HEAD` moved while
  the candidate still held the reviewed bytes.

## Decisions

- **Optional note member, not a new profile version.** `taskman-dispatch-handoff-note/0` gains an
  optional `commit` (a full commit id, written last). A note without it keeps its exact bytes; no
  writer sets it yet, so no existing note changes. The packet resolves the commit in the repository
  that held the reviewed commit and reports `OBSERVED` with `REVIEWED_COMMIT | ANCESTOR |
  NOT_ANCESTOR`; an unresolved commit stays `NOT_OBSERVED`.
- **Paging names the command.** `readOnDemand.history` lists `gate history <ticket> --gate G
  --cursor DIGEST` per truncated gate. The bound is unchanged; the page count and size became
  package variables only so a test reaches the bound with four events.
- **Candidate basis without a new store field.** The candidate branch is the one `gate packet`
  already reviews: the newest attempt with a submitted candidate tree and a branch. It is read
  only when the ledger holds a reviewed entry, so `show` for other tickets reads nothing new. A
  candidate branch that no longer resolves falls back to `HEAD`, and `staleBasis` says so.
- **Plan statuses stay at HEAD.** `obligations plan` names its statuses `..._AT_HEAD`
  (TOL-V0-104); moving them is a separate contract change and stays out of scope.

## Evidence

Run through `heavy.sh`, one package per command: `TestERGV00` in `./internal/tasks/cli/` (all
packet tests, including the new `TestERGV0026_CheckpointCommitBinding` and
`TestERGV0027_HistoryBoundNamesNextCursor`), `TestERGV0026` in `./internal/tasks/dispatch/`, and
`TestTOLV0131|TestTOLV007|TestTOLV0100|TestTOLV010[4-6]` in `./internal/tasks/cli/`. All passed.
With the binding and cursor code disabled, both new packet tests failed. With the candidate lookup
disabled, `TestTOLV0131_StalenessJudgedAtCandidate` failed on the HEAD-only edit.

## NOT_RUN

- The dispatcher does not yet record a worker's commit in the note it writes, because it knows no
  worker checkout path. Live notes therefore stay `NOT_OBSERVED` until a writer sets `commit`.
- Staleness is still read-time only: no stored count, gate or fingerprint sees it.
- No live operator ledger reviewed across a diverging candidate.

## Skipped tickets

- V1-1463: already delivered by d6f12df67 (DCW-V0-110,
  `TestBatchCheckDCWV0110NamesTheMergedForwardBase`).
- V1-2132: the open batch PR #1430 (73d457f0a) adds `wire.Annotatef` and rewrites every site.
  Every site file overlaps that PR.
- V1-2107: needs a re-emission of the sealed CEM-V1-091 schema chain, and that regeneration has
  no mode for widening an enum.
- V1-1840 and V1-2036: larger closeout and live-enrollment work that did not fit this lane's
  budget.

## Rollback

Revert the change. ERG-V0-026 first needs any hand-off note carrying `commit` to drop it, because
the earlier closed reader refuses it. ERG-V0-027 and TOL-V0-131 store nothing.
