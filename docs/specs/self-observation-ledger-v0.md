# Self-Observation Ledger V0

Owner: Russell Lewis
Amendment: `SOL-V0-008` ratified `accepted` 2026-09-01 by the repository owner; see
[`../decisions/0012-expert-panel-ratifications-2026-09-01.md`](../decisions/0012-expert-panel-ratifications-2026-09-01.md).
Ratification is per clause; every other clause retains the document's `proposed` status.
Intent status: proposed (except `SOL-V0-008`, accepted 2026-09-01)
Delivery status: experimental
Authoritative inputs: `docs/PRODUCT.md`, `docs/DOGFOOD.md`, `docs/specs/observation-corpus-authority-v0.md`

## Agent digest
- Claim: local, bounded observations expose recurring Corvint routing and capability gaps without claiming authority.
- Status: proposed (except `SOL-V0-008`, accepted 2026-09-01)/experimental
- Exists: `internal/observations`, `corvint observations`, harness event emission, `corvint dogfood-observe` and `corvint prove-observe` writers, host adapter degradation rows.
- Blocked on: owner review of DRAFT findings; observations cannot self-promote.
- Read next: Requirements; Non-goals and authority; Failure modes; Acceptance evidence and traceability.

## Intent and scope

SOL-V0 records only symptoms already computed by Corvint lifecycle handling: fallback degradations,
freshness, coverage counts, result omissions, task hashes, ranked/touched paths, and latency. It is
the runtime sibling of the proposed observation-corpus authority contract: that contract governs
what an independently usable corpus may establish; this one establishes nothing. The ledger is a
local append-only diagnostic proposal stream and `corvint observations` is its read-only triage.

## Requirements

- `SOL-V0-001`: Event handlers for `session-start`, `user-prompt`, and `file-change` MUST attempt
  one bounded JSON Lines append at `.corvint/self-observations.jsonl` after a successful receipt.
  Failure to observe MUST NOT alter a response, receipt, exit status, index, trace, or session. The
  writer MUST refuse when no ignore rule safely covers both the ledger and its atomic-write
  temporaries, and when `.corvint` is not a real directory or an existing ledger is not a regular
  file (a symlink), so no write, temporary sweep, or previous-row read leaves the worktree. Triage
  (`Read`) MUST open the ledger only under the same condition, verified against the opened file, and
  otherwise fail, so `corvint observations` exits 2 and `prove` omits its advisory `ledger` block
  (decision 0185's reader check). The mutating `corvint index` command MAY bootstrap the narrow parent ignore rule
  specified by `IDX-SNAP-V0-005`; event handlers and other read commands MUST NOT create it.
- `SOL-V0-002`: Rows MUST contain no task text, source body, command, credential, or agent-memory
  prose. Query identity is a SHA-256 task hash only. Rows may reuse a receipt ID and hashed session ID.
- `SOL-V0-003`: Each row is at most 2 KiB; the ledger is at most 128 KiB. On overflow, the writer
  drops complete oldest rows before appending. When an append repairs an externally oversized
  ledger, it MUST read at most half the file cap from the ledger's tail and align at the next row
  boundary exactly as oldest-first truncation does. Appends MUST discard an unterminated trailing row; a retained tail with no newline contributes no previous row, so malformed bytes cannot swallow the new event. The ledger is private local derived state,
  never input to ranking, learning, evidence, or authority.
- `SOL-V0-004`: Rows MAY report support, freshness, degradation codes, uncertainty, omitted and
  critical-missing counts, authoritative-result count, event latency, and file-change touched/ranked
  paths. A mixed-worktree file-change MUST report `MISS_DETECTION_NOT_OBSERVED`, not infer a miss.
- `SOL-V0-005`: Triage MUST be read-only and stdout-only. Its bounded digest reports degradation
  counts, routing misses, zero-authoritative and budget-omission rates, p50/p95 latency, and DRAFT
  backlog text; it MUST NOT create or modify `docs/agent-memory/`. Triage MUST accept only complete
  newline-terminated JSON Lines rows containing valid UTF-8 and one JSON object with no duplicate
  key at any depth; any other row is malformed and MUST be skipped. A row whose rendered key (code,
  query intent, degradation, touched or ranked path, or falsifier name) contains a control character
  or U+2028/U+2029 is malformed and MUST be skipped, so a hand-edited row cannot forge a triage line.
- `SOL-V0-006`: A degradation present in at least 90% of recorded events MUST be labelled
  `STANDING`; this is a candidate defect, not proof of defect cause or severity.
- `SOL-V0-007`: Every emitted `unsupported-*` CLI failure SHOULD be recorded with only
  code, classified intent, and task hash, except `query` and a command whose owning spec forbids a
  ledger write on its failure paths: `prove` (`FPK-V0-001`), `batch` (`SBQ-V0-005`), `affected`
  (`AFP-V0-001`), `dogfood event` and `dogfood status` (`LCP-V0-003`; its automatic surface owns no
  observations), `kernel` (`CKN-V0-007`: no ledger write), `necessity` (`NEC-V0-008`: no ledger or
  self-observation write), `answerability` (`RDS-V0-007`: no self-observation row), `surprise`
  (`TSS-V0-006`: no `.corvint/` state on any path), `context` and its lookup modes (`TCP-V0-001`: no
  ledger state on any path, including every error path; `TCP-V0-017`), and `init`/`adopt`
  (`GPK-V0-032`: the platform refusal precedes repository access and repository content stays
  byte-identical), and `frontier` (`CF-V0-033`: no repository or `.corvint` state on any path).
  The recording commands are exactly `impact`, `feature`, `docs`, `harness event`,
  `ocm`, `lrf`, `cem`, `dogfood-ocm`, `witness`, `index`, `calibrate`,
  `record`, and `dogfood-record`. A command whose every refusal leaves as one JSON stderr envelope
  (`ocm`, `lrf`, `cem`, `dogfood-ocm`, `witness`, `index`, `calibrate`, `record`,
  `dogfood-record`) is recorded once at its command
  boundary, not per call site. A command that emits no `unsupported-*` code on stderr has nothing
  to record. A code outside the writer's closed code set is not recorded. The row a refusing
  explicit mutator (`record`, `dogfood-record`, `cem`) appends is this ledger exception, not an artifact of that
  mutator, so it is outside `GPK-V0-008`'s byte-identical worktree rule; a refusing read (`lrf`) is
  outside `GPK-V0-007` in the same way, and the conformance refusal replay admits exactly that one
  path. `dogfood-record` records because it is `record`'s explicit coordinator surface: it refuses
  with the same `unsupported-verify-syntax` code, no owning spec forbids its ledger write, and
  `dogfood-change` already writes this ledger through `dogfood-observe` (`SOL-V0-008`; decision
  0121). The host `adapter` (and
  `native-hook`, which runs it) records no `unsupported` row, so no adapter code is admitted here
  (its post-root degradations are `SOL-V0-010` rows): its
  source-view and handoff refusals (`unsupported-text`, `unsupported-requirement`) fall under
  `ESV-V0-003`, which forbids ledger state, and every `unsupported-hook-event` exit (`AHI-009`)
  precedes repository-root resolution, so a row would need a ledger location guessed from the
  process environment (invariant 2). The code already reaches the host in the degraded
  `systemMessage`, and an append would wait on the ledger's blocking lock inside the two-second
  automatic-hook bound.
  Five or more matching hits become a `CAPABILITY-GAP` DRAFT naming the owning spec.
  Unsupported-by-design outcomes with a cited decision record are reported separately and MUST NOT
  become a capability gap: a code is by-design only when the closed registry `unsupportedByDesign`
  in `internal/observations/observations.go` maps it to an accepted decision record, and triage
  prints its key as `UNSUPPORTED-BY-DESIGN count=N key=CODE/INTENT decision=PATH`, never a DRAFT.
  The registry is empty until a decision record rules a specific code intended.
- `SOL-V0-008`: **Accepted 2026-09-01.** `corvint dogfood-observe` is the one additional bounded
  internal process writer surface. For every step rendered by `script/dogfood-change.sh`, the script
  MUST best-effort append one row with kind `dogfood-step` and only `step`, `status`, and `reason`.
  `step` and `reason` MUST match `[A-Za-z0-9_-]{1,96}`; `status` MUST be `PRODUCED` or
  `NOT_PRODUCED`. These fields MUST contain no path or command text, `corvint observations` MUST
  remain read-only, and an observation failure MUST NOT alter the script's exit status.
- `SOL-V0-009`: `corvint prove-observe` is the second bounded internal process writer surface.
  It appends one row with kind `proof` and only `counts`: the `proof.counts` of one `prove`
  document, falsifier then verdict then count, as FPK-V0-016 bounds it. Triage reports one
  `FALSIFICATION` line (proofs, judged, failed, rate) and one `FALSIFIER` line per falsifier; the
  rate is a number, never a DRAFT. `proof` rows are derived state under SOL-V0-003 and never
  enter ranking, learning, evidence, or authority; `prove` reads them into its advisory `ledger`
  block and never writes them.
- `SOL-V0-010`: The `codex` and `claude-code` host adapters (`corvint adapter`) MUST attempt one
  best-effort append with kind `adapter-degradation` whenever they return a degradation after
  resolving the project root: Claude Code from `CLAUDE_PROJECT_DIR` or the working directory, Codex
  from the hook input's `cwd` when it is absolute (decision 0169). The row MUST carry only `host`
  (`claude-code` or `codex`), `event` (the adapter event), `adapterCodes`, `window` (the UTC hour
  as `YYYY-MM-DDTHHZ`), and `corvintVersion`; never prompt text, paths, tool input, session identity,
  or repository content. `adapterCodes` is a sorted, duplicate-free set of one to eight codes from
  the closed registry in `internal/observations/observations.go`. A `corvint-event-rejected:<code>`
  reason keeps its code only when the `dogfood event`, dogfood error, or unsupported registry
  names it, and is otherwise recorded as `corvint-event-rejected`. The writer MUST NOT append a row
  byte-identical to one the ledger retains, so a host, event, code set and version is recorded at
  most once per hour window. `SOL-V0-001`'s ignore and symlink refusals and `SOL-V0-002`/`003`'s
  bounds apply unchanged. The append MUST NOT alter the hook output or exit status and waits no
  longer than the later of the invocation's work deadline and 50 ms, so a
  `dogfood-event-deadline` row, returned only once that deadline has expired, is still attempted;
  the Claude Code `adapter-host-kill-deadline` row is attempted after the watchdog fires, waits at
  most 50 ms, and is abandoned past it. A reason
  returned before root resolution (`unsupported-hook-event`, `hook-input-too-large`,
  `malformed-hook-json`, `missing-cwd`, `project-root-unavailable`), `corvint-output-too-large` at
  emission, the Codex kill deadline, and `claude-source-handoff` and `source-view` (`ESV-V0-003`)
  record nothing. Triage prints one `ADAPTER-DEGRADATION windows=N key=HOST/EVENT/CODE
  latest=WINDOW` line per key in key order after the other ranked lines; `windows` counts retained
  hour windows, not occurrences. Adapter rows are not `events`, never enter `STANDING`, and never
  enter ranking, learning, evidence, or authority.

## Non-goals and authority

V0 has no daemon, network, upload, auto-filing, automatic spec change, task-text storage, index
mutation, authority claim, or promotion path. It does not attest latency, establish a routing miss,
or close an obligation. A human or agent reviews printed drafts and files a selected finding through
the existing agent-memory convention.

## Failure modes

| Failure | Required behavior |
|---|---|
| ledger directory/file unavailable or not safely ignored | continue the lifecycle event without a row |
| `.corvint` or the ledger is a symlink or other non-regular entry at triage | `Read` fails without digesting it; `corvint observations` exits 2 with a JSON stderr error; `prove` omits the `ledger` block |
| row or file bound exceeded | drop oldest complete rows; never truncate a JSON row |
| mixed worktree | record `MISS_DETECTION_NOT_OBSERVED` |
| malformed ledger row | triage skips it and keeps reading bounded rows |
| unsupported code lacks owning spec | print an explicit DRAFT capability gap, never invent ownership |
| adapter degradation before root resolution, or Codex kill deadline | no row; the hook output is unchanged |
| adapter rejection code outside the closed registries | record bare `corvint-event-rejected`, never the unregistered code |
| adapter append exceeds its deadline or lock is held | abandon the append; the hook output is unchanged |
| same adapter row already retained in its hour window | no write |

## Acceptance evidence and traceability

`TestSelfObservationSpecEnumeratesEveryObligation` in `internal/lrfrepo/ocm_spec_test.go` checks
that OCM enumerates all ten clauses from this document; it does not validate their runtime behavior.

| Requirement | Evidence |
|---|---|
| SOL-V0-001..004 | `internal/observations` append/rotation and ignore-coverage tests; `TestAppendRepairDiscardsUnterminatedRows`; `TestAppendRefusesSymlinkedLedgerDirectoryOrFile`; `TestReadRefusesSymlinkedLedgerDirectoryOrFile`; `TestIndexBootstrapsIgnoredObservationLedger`; harness tests |
| SOL-V0-005..006 | golden triage test, `TestRenderSkipsMalformedJSONLinesRows`, `TestRenderSkipsRowWhoseKeyCarriesALineBreak`, and stdout-only CLI test |
| SOL-V0-007 | unsupported aggregation test; `TestUnsupportedByDesignCodeIsReportedSeparately`; `TestOCMUnsupportedRefusalAppendsOneObservation`; `TestLRFCEMAndDogfoodOCMUnsupportedRefusalsAppendOneObservationEach`; `TestIndexBuildingCommandsRecordUnsupportedRefusal`; `TestRunFrontierUnsupportedRefusalLeavesRepositoryUnchanged` (exclusion); `TestRecordUnsupportedVerifySyntaxAppendsOneObservation`; `TestDogfoodRecordUnsupportedVerifySyntaxAppendsOneObservation`; `TestRefusalSnapshotExceptsOnlyTheIgnoredLedger` (conformance refusal snapshot); `TestHostAdapterUnsupportedHookEventRecordsNoObservation` (exclusion); `TestBatchRefusesWithoutSnapshot` (exclusion); `TestCLIReadVerbsLeaveTheRepositoryByteIdentical` (adapter codex/claude-code CLI-level exclusion) |
| SOL-V0-008 | bounded writer CLI test, report-row integration test, append concurrency tests, and `TestDogfoodReasonAdmitsEveryRegisteredCEMCode` |
| SOL-V0-009 | `TestFalsificationRateCountsJudgedRowsOnly`, `TestProveObserveRecordsOnlyTheVerdictCounts`, `TestProveObserveRejectsWhatIsNotAProof` |
| SOL-V0-010 | `TestAdapterDegradationRowCarriesNoContentFields`, `TestAdapterDegradationDeduplicatesWithinWindow`, `TestAdapterDegradationRowsHonorLedgerCap`, `TestRenderTalliesAdapterDegradations`, `TestClaudeAdapterQuietDegradationIsLedgered`, `TestAdapterDegradationRecordedPastExpiredWorkDeadline`; `TestCLIReadVerbsLeaveTheRepositoryByteIdentical` (adapter paths touch nothing but the ledger) |

Rollback is deletion of the gitignored ledger and removal of its post-receipt best-effort call; no
repository evidence or authority depends on it. `SOL-V0-010` rolls back by removing the adapter's
`recordAdapterDegradation` calls; retained `adapter-degradation` rows then age out under the cap,
and triage ignores them once the reader is removed.
