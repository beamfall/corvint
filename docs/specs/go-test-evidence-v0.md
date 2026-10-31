# Go Test Evidence V0

Owner: Russell Lewis
Date: 2026-10-31
Requirement prefix: `GTE-V0`
Intent status: proposed
Delivery status: experimental
Authoritative inputs: ticket V1-0819 (idea reviewed 2026-10-05; clean-room, no third-party code, text or tests); `AGENTS.md` invariants 2, 4, 5 and 6; `gate-ledger-v0.md` (V1-0245) for the gate's own per-package pass memory, which this ledger does not replace.

## Agent digest
- Claim: An operator tool derives test records from `go test -json` under a stable per-test identity, so flaky tests gain a measured cross-run history.
- Status: proposed/experimental
- Exists: `tools/gotest-evidence` (`record`, `summary`), its tests and two captured `go test -json` fixtures.
- Blocked on: owner acceptance of the intent; nothing records the gate's or CI's streams into a ledger yet.
- Read next: Requirements; Identity rules; Non-goals; Rollback.

## User and measurable job

Flaky tests (V1-0768, V1-0771, V1-0815, V1-0724, V1-0779, V1-0796) are remembered, not measured:
each report names one failing run, and nothing ties it to the earlier runs of the same test. The Go
test suite is already the source of truth for what was tested. This slice keeps it so (invariant
6): evaluation records are derived from the `go test -json` output the suite already prints,
never hand-authored. Tests are not uniform (packages, subtests and repeat counts differ), and no
aggregate gate is computed. Identity is held strict instead. Measurable job: given the streams of
several runs, say per test how often it passed, failed, skipped or was not observed, on which
trees, and whether it flaked, which here means it passed and failed on the same tree, toolchain
and platform.

## Requirements

- `GTE-V0-820`: `gotest-evidence record -ledger FILE -tree SHA` MUST read one `go test -json`
  stream from stdin and append to FILE one record per terminal outcome (`pass`, `fail`, `skip`)
  of a test, subtest or package. `--help` names the usage, the required inputs and an example.
  The tool never runs tests and is never invoked by a read command.
- `GTE-V0-821`: Each record MUST name its package, its test (empty for the package outcome) and
  the subtest path split on `/`, its ordinal among that test's outcomes in the stream (so
  `-count=N` gives N records, never a merge), the tree, the toolchain, the platform and the
  command. `-tree` MUST be a full lowercase Git SHA. Toolchain, platform and command the caller
  does not declare are `NOT_OBSERVED`; the tool does not infer them. A declared platform MUST be
  `GOOS/GOARCH`.
- `GTE-V0-822`: Writes MUST be idempotent. A record's `id` is the SHA-256 of its schema, the
  stream's own SHA-256, the declared provenance, the package, the test and the ordinal. Recording
  a stream adds only records whose id the ledger does not hold, so recording the same stream
  twice adds nothing and leaves the ledger byte-identical. The ledger is rewritten through a
  temporary file and a rename.
- `GTE-V0-823`: Identity is the package and the full test path. A renamed test is a new identity
  and is never merged with the old one; nothing maps names across a rename.
- `GTE-V0-824`: A test that started (`run`) and never reached a terminal event, as after a panic,
  a timeout or a killed process, MUST be recorded `NOT_OBSERVED` with no elapsed time, never as a
  pass. A stream with any line that is not a test2json event, an unknown action, an event without a
  bounded package, or no events at all MUST be refused with no write. A ledger line that does not
  decode, has an unknown field, or whose `id` or `sha256` does not match its content MUST make the
  tool refuse the ledger.
- `GTE-V0-825`: `gotest-evidence summary -ledger FILE` MUST print, per package and test, the
  outcome counts, the number of distinct runs and `flaky`, which is true only when the test both
  passed and failed on the same tree, toolchain and platform. A pass on one tree and a fail on
  another is a change, not a flake.
- `GTE-V0-826`: Nothing in Corvint reads the ledger for ranking, evidence, authority or learning.
  The ledger lives where the operator names it; the tool writes nothing else.

## Identity rules

| Field | Source | Rule |
| --- | --- | --- |
| `package` | event `Package` | exact import path |
| `test`, `path` | event `Test` | exact name; `path` is the split on `/` |
| `ordinal` | count of terminal events for that package and test in the stream | starts at 1 |
| `run` | SHA-256 of the whole stream | two different runs differ in timings, so they are two runs; identical bytes are one recording |
| `tree`, `toolchain`, `platform`, `command` | caller flags | declared, not observed; undeclared is `NOT_OBSERVED` |

## Non-goals

No aggregate pass rate, threshold or gate; no automatic recording from `make gate`, CI or any
read command; no output text, log or source content stored (only identity, outcome and elapsed
time); no merge of renamed or moved tests; no ranking, authority or learning input; no other test
framework than `go test -json`.

## Failure modes

| Condition | Observation |
| --- | --- |
| Stream cut mid-test | Started tests without a terminal event are `NOT_OBSERVED` |
| Prefixed or foreign log, unknown action, empty stream | Refused, ledger untouched |
| Ledger edited by hand | Refused at read: `id` or `sha256` mismatch |
| Two writers at once | Not excluded: the last rename wins and the other's new records are lost; rerecording that stream restores them |
| A stream from a dirty worktree | Recorded under the declared tree; the tool cannot see the dirt (the declared tree is the caller's claim) |

## Acceptance evidence

| Requirements | Source/tests | Evidence |
| --- | --- | --- |
| GTE-V0-820 | `tools/gotest-evidence/main.go` `run`, `record`; `TestGTEV0820HelpNamesUsageInputsAndExample` | help and flag refusal, 2026-10-31 |
| GTE-V0-821, GTE-V0-823 | `Derive`; `TestGTEV0821RecordsNameIdentityAndProvenance` | actual go1.27.3 `-count=2` stream: two ordinals per test, subtest path split, renamed test a new identity, undeclared provenance `NOT_OBSERVED` |
| GTE-V0-822 | `record`, `recordID`, `replace`; `TestGTEV0822RecordingTwiceIsByteIdentical` | second recording adds 0 and the bytes are equal; 13 distinct ids for 13 outcomes |
| GTE-V0-824 | `Derive`, `readLedger`; `TestGTEV0824UnfinishedTestIsNotObserved` | a stream cut before `TestPass` ended gives one `NOT_OBSERVED`; three bad streams and three bad provenances refused with no write; an edited outcome refused |
| GTE-V0-825 | `summary`; `TestGTEV0825SummaryMeasuresFlakes` | a subtest that passed twice and failed once on one tree is flaky; the same outcomes split across two trees are not |
| GTE-V0-826 | no reader outside `tools/gotest-evidence` | `git grep gotest-evidence` names only this tool, this spec and its index rows |

Recorded limits. The fixtures are one module captured on one host; no CI or gate stream has been
recorded yet, so no flake rate is measured for the tickets above (NOT_RUN). Concurrent writers are
not locked.

## Rollback

Delete `tools/gotest-evidence` and this spec. No other code reads a ledger, so nothing else
changes; any ledger files the operator created stay where they were put and can be deleted.
