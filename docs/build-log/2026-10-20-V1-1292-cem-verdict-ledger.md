# 2026-10-20 V1-1292: CEM verdict ledger

Ticket: V1-1292. Requirements: `CEM-V1-271..279` in `docs/specs/cem-stable-v1.md`, accepted
under the owner's standing auto-accept of lane requirements (2026-10-11).

## Problem

`cem anchor` (`CEM-V1-067`) records which `cem/1.0` map describes a commit. It does not record
what a verifier concluded. The spec said "no verdict ledger" twice: in the `CEM-V1-065` row and in
the `cem/v2` non-goals. So a second verification that reached a different `verdictSha256`
(`CEM-V1-066`) for the same map and commit left no trace. The cause could be other artifacts or
another verifier release. Decision 0523 K6 sets the precondition for a ledger: the native and
portable consumers must agree on `verdictSha256` for every public stable case. That precondition
already holds through the `CEM-V1-066` tests.

## Decisions

- Ref: the ledger lives in `refs/notes/corvint-verdicts`. A ref below `refs/notes/corvint/`
  cannot coexist with the existing `refs/notes/corvint` ref (directory/file conflict). Adding
  lines to the anchor note would make it read `malformed`, because `CEM-V1-067` requires one
  pointer object per note.
- Layout: there is one note per target commit. It holds LF-terminated lines, each one canonical
  `corvint-cem-verdict/1` record, at most 256 per note. A record is keyed by `mapSha256` inside
  its target's note. Each record names the SHA-256 of the previous line, so an edit, removal or
  reorder inside a note reads `chain-broken`.
- Closed record: `schema`, `mapSha256`, `target{gitCommit,gitTree}`, `outcome`,
  `verdictSha256`, `consumer` (`native`), `runtime{goVersion}`, `recordedAt` and
  `previousRecordSha256`. A line must equal the compact re-encoding of its decoded record.
  `outcome` is an addition to the ticket's member list, so a reader can see ACCEPT or REJECT
  without recomputing the digest.
- Writer: opt-in through `cem anchor --artifacts DIR`. It runs `verify.Stable` in process on the
  exact committed map bytes, with the map's base as the expected base, so the digest is the one
  `cem verify` would report. Only ACCEPT and REJECT are recorded. Every refusal is decided before
  any write, whether of the pointer or of the verdict. The refusals are
  `verdict-ledger-conflict`, `-malformed`, `-unsupported-result` and `-full`.
- An identical repeat writes nothing and returns the first record, so `recordedAt` keeps the
  first verification time.
- Reader: `cem provenance --verdicts` adds a `verdicts` member. Records are listed only when the
  ledger is `verified`. No new `cem` action was added, because
  `TestAnchorActionsParseLikeTheirSiblings` pins the action list.
- Authority: none. Nothing outside `internal/gitnotes` reads the ref. Product invariant 4 holds.

## Evidence

- Focused tests, through `heavy.sh`, one package per command:
  - `./internal/gitnotes/`: full package, ok (35.9 s).
  - `./internal/cem/cli/`: full package, ok.
  - `./cmd/corvint/`: `-run 'TestCEMAnchor|TestCEMV1065|TestCEMCiteHelp|TestHelp|TestNativeVerbOption|TestRootHelp'`, ok.
- `go vet` on the three packages: clean.
- Fail-without-fix. Each guard was mutated in turn, and in every case the named test failed. The
  sources were then restored and checked with `cmp`.
  - Conflict check disabled: `TestCEMV1275DifferingVerdictIsRefused` failed.
  - Canonical-bytes check disabled: the non-canonical-spacing case of
    `TestCEMV1276TamperedLedgerIsNeitherListedNorExtended` failed.
  - Chain check disabled: the edited-earlier-record and removed-first-record cases of the same
    test failed.
  - Scope and repeated-key checks disabled: the record-names-another-commit and repeated-key
    cases failed.
  - Chain link not set on append: `TestCEMV1274AppendLinksToThePreviousRecord` failed.
  - Anchor ignoring `--artifacts`: `TestCEMV1273AnchorAppendsOneVerdictRecordAndRepeatsIdempotently`
    failed.
  - Idempotent repeat rewriting the note: `TestCEMV1273` failed.
  - Listing rewriting the note: `TestCEMV1277VerdictListingIsReadOnly` failed.
  - `--verdicts` inverted: `TestCEMV1277` failed.
  - Full bound off by one: `TestCEMV1275FullLedgerIsRefused` failed.
  - At the base commit none of the new tests compile, because `AnchorWithVerdict` is absent.
- The fixture's verdict is checked against an independent `wire.ParseStable` and `verify.Stable`
  run. The fixture reaches REJECT: it is the public receipt-authentication map rebound to the
  fixture patch.

## NOT_RUN

- `make gate`: not run, per the lane charter (focused tests only).
- The native/portable `verdictSha256` agreement (K6): not re-run here. The precondition rests on
  the existing `CEM-V1-066` tests in `internal/cem/verify` and `interop/cem01-go`, which this
  change does not touch.
- An ACCEPT record end to end: not run, because the fixture map verifies to REJECT. The
  record path does not depend on the outcome beyond the ACCEPT/REJECT domain check.
- Concurrent anchors of one commit: not tested. This is a stated non-goal, with no
  compare-and-swap.

## Rollback

Revert the V1-1292 commit. `--artifacts` and `--verdicts` become unrecognized arguments. Existing
ledger notes stay in the repository, unread. They can be removed with
`git update-ref -d refs/notes/corvint-verdicts`.
