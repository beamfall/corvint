# Corvint Tasks defect register V0

Owner: Russell Lewis
Date: 2026-10-17
Intent status: accepted under the owner's standing auto-accept (2026-10-11) (V1-1593; GitHub #1036)
Delivery status: experimental
Authoritative inputs: GitHub issue beamfall/corvint#1036 and native ticket V1-1593 ("defects register
for suspected product and environment faults found during test work"), the
[obligation ledger contract](corvint-tasks-obligation-ledger-v0.md) (DEFECT and BLOCKED obligation
states, expected-fail annotations, `preflight`), decision 0397, and the in-tree sources under
`internal/tasks/defects`, `internal/tasks/intent` and `internal/tasks/cli`.

## Agent digest
- Claim: A local append-only register records product and environment defects; preflight checks that obligations and expected failures cite known, live product defects.
- Status: TDR-V0-001..010 accepted under the owner's standing auto-accept (2026-10-11) (V1-1593; GitHub #1036); experimental; implemented with focused tests.
- Exists: the `internal/tasks/defects` register, the optional policy `defects {prefix, path}`, `corvint-tasks defects add|update|list`, and the preflight `DEFECT_UNKNOWN`, `DEFECT_NOT_A_BUG` and `DEFECT_ENVIRONMENT` findings.
- Blocked on: nothing for V0; harvesting defects from test reports is a residual (see Non-goals).
- Read next: Requirements; Failure modes; Traceability.

## User and boundary

Test work finds faults that are not yet tickets: a product bug an expected-fail test documents, or
an environment fault (a flaky proxy, a broken runner) that blocks a run. Without a register, the
ids cited in obligation reasons and `test.fail` titles are free text: nothing says whether they
exist, were closed as not-a-bug, or describe the environment rather than the product. The register
is a local file, separate from the Tasks journal; it is never an input to ranking, evidence or
authority, and preflight only reads it.

## Requirements

- `TDR-V0-001`: Policy MAY declare `defects` as a closed object with optional `prefix` (matching
  `[A-Z][A-Z0-9]{0,15}`, default `DEF`) and optional `path` (a repository-relative path
  without `..`). Any other shape refuses `MALFORMED`. Without the key the register still works at
  its default location with the default prefix.
- `TDR-V0-002`: Each entry has `id` (`PREFIX-N`), `title`, `location`, `evidence`, `severity`
  (`LOW`, `MEDIUM`, `HIGH`, `CRITICAL`; default `MEDIUM`), `status`, `category`, `duplicateOf`,
  `recordedAt` and a derived `revision`. `add` requires title, location and evidence, each one
  line of 1 to 2048 bytes, and assigns the next id after the highest of its prefix. A missing,
  empty, multi-line or oversized field or an unknown enum refuses `MALFORMED` and writes nothing.
- `TDR-V0-003`: `status` is `SUSPECTED` (default), `CONFIRMED`, `NOT_A_BUG` or `DUPLICATE`;
  `category` is `PRODUCT` (default) or `ENVIRONMENT`. `DUPLICATE` requires `duplicateOf` naming
  another existing entry, without forming a cycle; `duplicateOf` without `DUPLICATE` refuses
  `MALFORMED`, and leaving `DUPLICATE` clears it.
- `TDR-V0-004`: The register is JSONL with profile `taskman-defect/0`. Each `add` or `update`
  appends one line holding the entry's full new state; no line is ever rewritten or removed, and
  the fold takes the last line per id. An `update` of an unknown id refuses `MISSING_EVIDENCE`.
  A second `ADD` of an id or an `UPDATE` before its `ADD` is `MALFORMED` on read, as is a line of
  another profile; a line of a later `taskman-defect` version is `UNSUPPORTED_VERSION`.
- `TDR-V0-005`: Writers hold an exclusive `flock` on `<register>.lock` across read, validation and
  the `O_APPEND` write, then `fsync`, so concurrent appends receive distinct ids and never tear a
  line. Readers take no lock and create nothing: a missing register is empty, bytes after the last
  LF (an interrupted write) are ignored, and the next writer seals them with an LF so the torn
  bytes become a complete line that a later read reports `MALFORMED` rather than skips. An append
  that would grow the register past the 8 MiB read bound refuses `LIMIT_EXCEEDED` and writes
  nothing, so the register stays readable.
- `TDR-V0-006`: Every text field is checked against the shared credential screen; a match refuses
  `SECRET_DETECTED` with the detail prefix `DEFECT_SECRET_SCREENED`, naming the flag and no part
  of the value, and writes nothing.
- `TDR-V0-007`: `preflight` reads the register at its location when it exists or when policy
  declares `defects`. Register-prefix ids cited in an obligation's title or reason, and on a spec
  file line that declares an expected failure (an expected-fail title or a `test.fail(` call),
  produce `DEFECT_UNKNOWN` when the register does not hold them and `DEFECT_NOT_A_BUG` when the
  register closed them `NOT_A_BUG`. Ids of other prefixes are left to the existing checks.
- `TDR-V0-008`: A `DEFECT` or `BLOCKED` obligation whose cited register ids all resolve (through
  `DUPLICATE` links) to `ENVIRONMENT` defects, and that cites no other defect id, produces
  `DEFECT_ENVIRONMENT`: an environment defect does not satisfy a blocked-by-defect obligation. A
  `PRODUCT` defect or a foreign tracker id satisfies it.
- `TDR-V0-009`: The default register is `<git-common-dir>/taskman-defects/register.jsonl`
  (directory mode 0700): outside every worktree, the Tasks state directory and therefore
  `archive export`, and never pushed. Declaring `defects.path` places it in the primary worktree
  instead, where the adopter decides whether it is committed or published. A writer refuses
  `UNSUPPORTED_FILESYSTEM`, as readers do, when the existing part of the register directory
  resolves through a symbolic link, before and after creating it, so a symlinked parent never
  places the register outside its root.
- `TDR-V0-010`: `corvint-tasks defects add|update <id>|list [--status S] [--category C]` return the
  entries as untrusted data. `add` and `update` append exactly one line per invocation, even when
  the Tasks state read behind them retries, and write only the register and its lock file; `list`
  writes nothing, and none of them writes the Tasks journal, receipts or intent.

## Non-goals and simpler baseline

The simpler baseline is free-text defect ids in obligation reasons; it was rejected because nothing
checks them. Out of scope for V0: harvesting defects from Playwright or CI reports (residual of
V1-1593), a Core `corvint defects` verb (the Core verb surface is frozen; the verbs live in
`corvint-tasks`), linking a defect to a filed ticket automatically, and deleting entries.

## Failure modes

| Situation | Behavior |
| --- | --- |
| Two writers append at once | Serialized by the lock; distinct ids; no torn line (`TDR-V0-005`). |
| Writer killed mid-line | Readers ignore the tail; the next write seals it; a later read reports `MALFORMED` (`TDR-V0-005`). |
| Hand-edited or corrupt line | Read refuses `MALFORMED`; preflight surfaces the error rather than passing (`TDR-V0-004`). |
| Credential in a field | `SECRET_DETECTED`, value not echoed, nothing written (`TDR-V0-006`). |
| Obligation cites a closed or unknown defect | `DEFECT_NOT_A_BUG` or `DEFECT_UNKNOWN` finding (`TDR-V0-007`). |
| Blocked obligation rests on an environment fault | `DEFECT_ENVIRONMENT` finding (`TDR-V0-008`). |
| Platform without `flock` | Writes refuse `UNSUPPORTED`; reads still work (`TDR-V0-005`). |
| Append would cross the 8 MiB read bound | `LIMIT_EXCEEDED`; nothing written (`TDR-V0-005`). |
| Declared register parent is a symlink | `UNSUPPORTED_FILESYSTEM`; nothing created at its target (`TDR-V0-009`). |
| Tasks state moves during `defects add` | The read retries; the register still gains one line (`TDR-V0-010`). |

## Acceptance evidence

Focused `internal/tasks/defects`, `internal/tasks/intent` and `internal/tasks/cli` tests on Darwin,
including an 8-writer by 5-append concurrency test. Linux and multi-process qualification are
`NOT_RUN`.

## Rollback

Revert the V1-1593 change and remove `defects` from policy. The register is a separate local file
that no Tasks record, receipt or archive references; deleting `taskman-defects/` (or the declared
path) removes it. An older binary refuses a policy carrying `defects` closed and ignores the file.

## Traceability

| Requirement | Evidence |
| --- | --- |
| `TDR-V0-001` | `TestTDRV0001_PolicyDefects` |
| `TDR-V0-002` | `TestTDRV0002_AddAssignsIDsAndDefaults`; `TestTDRV0010_DefectsVerbsThroughTheCLI` |
| `TDR-V0-003` | `TestTDRV0002_AddAssignsIDsAndDefaults`; `TestTDRV0004_UpdatesAppendAndLaterLinesOverride` |
| `TDR-V0-004` | `TestTDRV0004_UpdatesAppendAndLaterLinesOverride`; `TestTDRV0004_LineProfileIsChecked`; `TestCALV0131_EveryLiveFormatRefusesANewerVersion` |
| `TDR-V0-005` | `TestTDRV0005_ConcurrentAddsSerializeAndReadsIgnoreAPartialTail`; `TestTDRV0005_AppendNeverCrossesTheReadBound` |
| `TDR-V0-006` | `TestTDRV0006_TextIsSecretScreenedWithoutEcho`; `TestTDRV0010_DefectsVerbsThroughTheCLI` |
| `TDR-V0-007` | `TestTDRV0007_TDRV0008_Findings`; `TestTDRV0007_TDRV0008_PreflightDefectFindings` |
| `TDR-V0-008` | `TestTDRV0007_TDRV0008_Findings`; `TestTDRV0007_TDRV0008_PreflightDefectFindings` |
| `TDR-V0-009` | `TestTDRV0009_PrivateDefaultAndDeclaredPath`; `TestTDRV0009_SymlinkedParentRefusesBeforeWriting`; `TestTDRV0010_DefectsVerbsThroughTheCLI` |
| `TDR-V0-010` | `TestTDRV0010_DefectsVerbsThroughTheCLI`; `TestTDRV0010_DefectWriteAppendsOnceAcrossStoreRetries` |
