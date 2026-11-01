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
  file (a symlink), so no write, temporary sweep, or previous-row read leaves the worktree. An
  ignore file counts toward that coverage only when it is a regular file of at most 256 KiB,
  reached inside the repository without a symlink leaf, and read with a non-blocking open; any
  other ignore file skips the observation. Triage
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
  never input to ranking, evidence, or authority, nor to learning except through the operator-invoked
  `corvint eval --learn-slot-weights` step, whose output reaches ranking only after the frozen held-out
  gate admits it (`LTA-V0-009` to `LTA-V0-012`; decision 0368, ratified by decision 0373).
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
  (amended 2026-10-04, V1-0740) Triage MUST also re-apply `Append`'s writer contract, which refuses a
  negative count or latency, and count each bounded row it refuses by one closed reason without
  folding any of it: `malformed-json`, `unterminated-row` (a final row without a newline in a ledger
  under the cap), `contract-<field>` (the writer contract's closed field name, such as
  `contract-kind`, `contract-paths` or `contract-negative-count`), `unsafe-key`, and `proof-counts`
  (a proof row with an unknown verdict or an out-of-range count; a proof row with no counts, which
  `prove` writes for a document with no rows, judges nothing and is not rejected). It MUST report when the
  ledger exceeded 128 KiB and only its first 128 KiB were read; a row ending past that cap is
  neither folded nor rejected. The digest prints `REJECTED-ROWS reason=<reason> count=<n>` lines in reason
  order and `LEDGER-CUT cap-bytes=131072`.
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
  `ESV-V0-003`, which forbids ledger state, and every degraded `unsupported-hook-event` exit (`AHI-009`)
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
  from the hook input's `cwd` when it is absolute (decision 0169). An `opencode` `harness event` on
  `file-change` or `post-tool` whose input carries `adapterCodes` MUST attempt the same append once
  per code, with that event, after resolving `--root` and whether or not the event then succeeds
  (V1-0767), waiting for it no longer than a fixed 500 ms bound. The row MUST carry only `host` (`claude-code`, `codex` or `opencode`), `event` (the adapter event), `adapterCodes`, `window` (the UTC hour
  as `YYYY-MM-DDTHHZ`), and `corvintVersion`; never prompt text, paths, tool input, session identity,
  or repository content. `adapterCodes` is a sorted, duplicate-free set of one to eight codes from
  the closed registry in `internal/observations/observations.go`. A `corvint-event-rejected:<code>`
  reason keeps its code only when the `dogfood event`, dogfood error, or unsupported registry
  names it, and is otherwise recorded as `corvint-event-rejected`. The writer MUST NOT append a row
  byte-identical to one the ledger retains, so a host, event, code set and version is recorded at
  most once per hour window. Two silent abstentions attempt the same append, with the same
  bounds, while their output stays unchanged (V1-0746). A Codex hook event the adapter does not
  handle stays `{}` and, once its absolute `cwd` lies inside a Git repository, is recorded as event
  `unrecognised` with code `unsupported-hook-event`; the host's event name is never recorded. A
  Claude Code `post-tool` Edit, Write or NotebookEdit target that is not project-relative stays
  silent (`AHI-019`) and is recorded, without the path, as `SOL-V0-030` classifies it, unless its
  `harness event` call failed, which records that degradation instead. OpenCode's
  `adapterCodes` is a sorted, duplicate-free list of one or both of `changed-paths-truncated` and
  `post-tool-path-not-project-relative`; any other value, host or event refuses the whole event as
  `invalid-harness-input` before anything is written. Core removes the field before input
  validation, so it reaches neither the receipt basis, the response nor the trace, and the plugin's
  receipt check leaves it out of the basis too (`AHI-022`).
  `SOL-V0-001`'s ignore and symlink refusals and `SOL-V0-002`/`003`'s
  bounds apply unchanged. The append MUST NOT alter the hook output or exit status and waits no
  longer than the later of the invocation's work deadline and 50 ms, so a
  `dogfood-event-deadline` row, even one returned after that deadline has expired, is still attempted;
  the Claude Code `adapter-host-kill-deadline` row is attempted after the watchdog fires, waits at
  most 50 ms, and is abandoned past it. A reason
  returned before root resolution (Claude Code's `unsupported-hook-event`, `hook-input-too-large`,
  `malformed-hook-json`, `missing-cwd`, `project-root-unavailable`), `corvint-output-too-large` at
  emission, the Codex kill deadline, and `claude-source-handoff` and `source-view` (`ESV-V0-003`)
  record nothing. Triage prints one `ADAPTER-DEGRADATION windows=N key=HOST/EVENT/CODE
  latest=WINDOW` line per key in key order after the other ranked lines; `windows` counts retained
  hour windows, not occurrences. Adapter rows are not `events`, never enter `STANDING`, and never
  enter ranking, learning, evidence, or authority.

- `SOL-V0-011`: (added 2026-10-19, V1-1977) A `user-prompt` row whose receipt coverage carried a
  readable non-negative integer `authoritative_results` MUST record that count as
  `authoritativeResultCount` and mark itself `authorityCounted: true`, whether the count reached the
  writer as an integer or as the whole-number JSON number a decoded packet carries. No other row
  kind or event may carry the marker; triage rejects one that does as `contract-authorityCounted`.
  The digest's `zero-authoritative-rate` MUST count only marked `user-prompt` rows, as zero
  numerator over marked denominator, and MUST print `AUTHORITY-UNMEASURED count=<n>` after any
  `LEDGER-CUT` line for retained unmarked `user-prompt` rows, which carry no measured count and are
  never reported as zero authority.
- `SOL-V0-012`: (added 2026-10-19, V1-1977) A degradation that an accepted decision fixes on every
  receipt MUST print `DEGRADATION-BY-DESIGN count=<n> key=<code> decision=<path>` in place of its
  `DEGRADATION` line and is never labelled `STANDING` or given a DRAFT. The closed set is
  `frontier-authority-unavailable`, fixed by
  `docs/decisions/0519-w6-batch-acceptances-2026-10-11.md` (`AHI-054`): it reports the absent
  closing Frontier authority and says nothing about project authority such as `AGENTS.md` or
  accepted ADRs.
- `SOL-V0-013`: (added 2026-10-19, V1-1977) When retained adapter rows carry
  `corvint-event-rejected:dogfood-event-index-snapshot-stale` (`AHI-031`), triage MUST print one
  `SETUP key=index-snapshot-stale rows=<n> latest=<window> fix: corvint index --if-stale` line
  after the cut and unmeasured lines, where `rows` sums the retained adapter rows with that code
  across hosts and events and `latest` is their latest hour window. The line is a setup diagnosis
  read from rows already written; triage still writes nothing and never indexes.
- `SOL-V0-014`: (added 2026-10-28, V1-2992) Both private ledgers (`internal/observations` and
  `internal/unplannedread`) MUST reach `.corvint` through one shared validated directory handle
  (`internal/observations/ledgerdir.go`, `LedgerDirectory`). It opens the worktree root and then
  `.corvint` descriptor-relatively with `os.Root`, creating it there when a writer needs it, and
  refuses unless the opened directory is the same file as the no-follow `Lstat` of `.corvint`.
  Every ledger read, temporary sweep, exclusive temporary creation, rename replacement, marker write
  and marker removal MUST be relative to that handle, and each read and mutation MUST first re-verify
  that `.corvint` still names the validated directory, refusing otherwise. A `.corvint` swapped for a
  symlink after validation therefore leaves the symlink target byte-identical. A platform where
  `os.Root` cannot open the directory returns an error, so the writer fails closed. `Enabled` remains
  an existence check that writes nothing; the append it gates is the refusing step.
- `SOL-V0-030`: (added 2026-10-20, V1-1975) The Claude Code adapter MUST classify a `post-tool`
  Edit, Write or NotebookEdit target that `projectpath.Relative` does not admit. It is the exclusion
  code `post-tool-path-outside-project` only when the root resolves, the target resolves (itself, or
  for a not-yet-created file its parent directory, without cleaning `..` away first), and both the
  target's spelling and its resolved form lie outside both the root and the root's resolved form,
  compared case-insensitively. Every other target keeps the degradation code
  `post-tool-path-not-project-relative`: one that does not resolve (a missing parent, a `..` through
  a missing component), one whose spelling lies inside the root but resolves outside (a symlink that
  escapes the root), and one that differs from the root only by letter case or through a symlink
  alias of the root, so an exclusion is never established from spelling alone. An in-project
  absolute target is relativized and accepted,
  with no row. Triage MUST print each retained `adapter-degradation` key whose code is in the closed
  exclusion set (`post-tool-path-outside-project`) as `ADAPTER-EXCLUSION windows=<n> key=<key>
  latest=<window>` instead of `ADAPTER-DEGRADATION`: an out-of-project target is not a project
  change, so its count is a correct exclusion and not a fault. The row shape and writer contract are
  unchanged.

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
| malformed ledger row | triage rejects it under a closed reason, counts it, and keeps reading bounded rows |
| row outside the writer contract (unknown kind, escaping path, negative count) | triage rejects it as `contract-<field>`; it is never a count, miss path or label |
| ledger over 128 KiB | triage reads the first 128 KiB and reports `LEDGER-CUT` |
| unsupported code lacks owning spec | print an explicit DRAFT capability gap, never invent ownership |
| adapter degradation before root resolution, or Codex kill deadline | no row; the hook output is unchanged |
| unhandled Codex event or out-of-project Claude Code post-tool target | one row with its named code; the silent hook output is unchanged |
| adapter rejection code outside the closed registries | record bare `corvint-event-rejected`, never the unregistered code |
| adapter append exceeds its deadline or lock is held | abandon the append; the hook output is unchanged |
| same adapter row already retained in its hour window | no write |
| OpenCode `adapterCodes` with another host, event or code, or unsorted, duplicated or empty | refuse the event as `invalid-harness-input`; no row |
| OpenCode `adapterCodes` append fails (ledger not ignored, cap reached) | no row; the event and its receipt are unchanged |
| OpenCode `adapterCodes` append waits on a held ledger lock | the coded append stops being awaited after the 500 ms bound and the response is unchanged (a `file-change` event's own `SOL-V0-001` append still waits on the lock); the abandoned append lands once the lock frees or is lost with the process |
| `user-prompt` row without `authorityCounted` (written before SOL-V0-011, or with no readable count) | counted under `AUTHORITY-UNMEASURED`, never as zero authority; it ages out under the 128 KiB cap |
| `authorityCounted` on any row but a `user-prompt` event | triage rejects it as `contract-authorityCounted` |
| `.corvint` replaced by a symlink or another directory after the ledger handle is validated | the read or mutation is refused; nothing outside the worktree is read, created, renamed or removed |
| a by-design degradation's decision is superseded | the closed `SOL-V0-012` set is amended in the same change; until then the line still names the decision it cites |
| index snapshot missing or stale for the committed tree | hooks reject as `dogfood-event-index-snapshot-stale`; triage prints the `SETUP` line naming `corvint index --if-stale` |

## Acceptance evidence and traceability

`TestSelfObservationSpecEnumeratesEveryObligation` in `internal/lrfrepo/ocm_spec_test.go` checks
that OCM enumerates exactly the clauses `docs/specs/REQUIREMENTS.tsv` lists for this document, so a
new clause needs no count edit (V1-2153); it does not validate their runtime behavior.

| Requirement | Evidence |
|---|---|
| SOL-V0-001..004 | `internal/observations` append/rotation and ignore-coverage tests; `TestAppendRepairDiscardsUnterminatedRows`; `TestAppendRefusesSymlinkedLedgerDirectoryOrFile`; `TestReadRefusesSymlinkedLedgerDirectoryOrFile`; `TestAppendReadsOnlyBoundedRegularIgnoreFilesInsideTheRepository`; `TestIndexBootstrapsIgnoredObservationLedger`; harness tests |
| SOL-V0-005..006 | golden triage test, `TestRenderSkipsMalformedJSONLinesRows`, `TestRenderSkipsRowWhoseKeyCarriesALineBreak`, `TestReadRejectsContractViolationsAndReportsCut`, and stdout-only CLI test |
| SOL-V0-007 | unsupported aggregation test; `TestUnsupportedByDesignCodeIsReportedSeparately`; `TestOCMUnsupportedRefusalAppendsOneObservation`; `TestLRFCEMAndDogfoodOCMUnsupportedRefusalsAppendOneObservationEach`; `TestIndexBuildingCommandsDoNotRefuseAggregateSize_IDXSNAPV0910` (no aggregate refusal is left to record, `IDX-SNAP-V0-910`); `TestRunFrontierUnsupportedRefusalLeavesRepositoryUnchanged` (exclusion); `TestRecordUnsupportedVerifySyntaxAppendsOneObservation`; `TestRecordUnsupportedSkipsObservationBehindAnOversizedIgnoreFile`; `TestDogfoodRecordUnsupportedVerifySyntaxAppendsOneObservation`; `TestRefusalSnapshotExceptsOnlyTheIgnoredLedger` (conformance refusal snapshot); `TestHostAdapterUnsupportedHookEventRecordsNoObservation` (exclusion); `TestBatchRefusesWithoutSnapshot` (exclusion); `TestCLIReadVerbsLeaveTheRepositoryByteIdentical` (adapter codex/claude-code CLI-level exclusion) |
| SOL-V0-008 | bounded writer CLI test, report-row integration test, append concurrency tests, and `TestDogfoodReasonAdmitsEveryRegisteredCEMCode`; the admitted reasons read `cemcode.Registered()` through a Go import, so a registry change selects `internal/observations` (`TestRegisteredListsEveryCode`, `TestCEMCodeRegistryChangeSelectsObservations_V13215`; V1-3215) |
| SOL-V0-009 | `TestFalsificationRateCountsJudgedRowsOnly`, `TestProveObserveRecordsOnlyTheVerdictCounts`, `TestProveObserveRejectsWhatIsNotAProof` |
| SOL-V0-010 | `TestAdapterDegradationRowCarriesNoContentFields`, `TestAdapterDegradationAdmitsWorktreeChurn` (`dogfood-event-worktree-churn`, decision 0533), `TestAdapterDegradationAdmitsPromptBounds` (`dogfood-event-prompt-bounds`, LCP-V0-034, V1-1976), `TestAdapterDegradationAdmitsCompactionEventsAndCodes`, `TestAdapterDegradationDeduplicatesWithinWindow`, `TestAdapterDegradationRowsHonorLedgerCap`, `TestRenderTalliesAdapterDegradations`, `TestClaudeAdapterQuietDegradationIsLedgered`, `TestAdapterDegradationRecordedPastExpiredWorkDeadline`, `TestAdapterSilentAbstentionsAreLedgered`, `TestOpenCodeAdapterCodesAreLedgeredOutsideTheReceipt`, `TestAdapterCodesAreClosedToOpenCodePathEvents`, `TestOpenCodeAdapterCodesDoNotWaitOnAHeldLedgerLock`, and under `TestHostAdapterJavaScriptHosts` `SOL-V0-010 AHI-022 V1-0767 OpenCode path abstentions reach the self-observation ledger against the real binary`; `TestCLIReadVerbsLeaveTheRepositoryByteIdentical` (adapter paths touch nothing but the ledger) |
| SOL-V0-011 | `TestAuthorityRateCountsOnlyMeasuredUserPromptRows`; `TestAdopterUserPromptReturnsAndRecordsProjectAuthority` (V1-1977 external-adopter qualification: a Go module with `AGENTS.md` and accepted `docs/adr` decisions returns accepted-decision authority from a user-prompt hook event, records the measured count, and reports `zero-authoritative-rate=0/1`) |
| SOL-V0-012 | `TestFixedFrontierDegradationIsReportedByDesign`; `TestRenderGoldenAndReadOnly`; `TestAdopterUserPromptReturnsAndRecordsProjectAuthority` |
| SOL-V0-013 | `TestSetupLineNamesSnapshotRefresh` |
| SOL-V0-014 | `TestSOLV0014AppendRefusesSwappedLedgerDirectory`; `TestSOLV0014LedgerDirectoryRefusesEveryOperationAfterSwap`; `TestURE004AppendRefusesSwappedLedgerDirectory`; `TestAppendRefusesSymlinkedLedgerDirectoryOrFile`; `TestSymlinkedLedgerIsRefused` |
| SOL-V0-030 | `TestSOL030PostToolTargetAbstentionClasses` (resolved outside absolute and relative targets are exclusions, in-project absolute and relative targets are accepted; a symlink-escaping in-project target, a missing parent, a `..` through a missing component, a case-only spelling and a symlink alias of the root are never exclusions); `TestSOL030OutOfProjectPostToolIsExclusion` (triage prints `ADAPTER-EXCLUSION` for the exclusion code and `ADAPTER-DEGRADATION` for the unresolvable code); `TestAdapterSilentAbstentionsAreLedgered` (the out-of-project Write records `post-tool-path-outside-project`) |

Rollback is deletion of the gitignored ledger and removal of its post-receipt best-effort call; no
repository evidence or authority depends on it. `SOL-V0-010` rolls back by removing the adapter's
`recordAdapterDegradation` and `recordAdapterReason` calls and, for OpenCode, first the plugin's
`adapterCodes` field and then `takeAdapterCodes` in `internal/gokernel/harness.go` with
`recordAdapterCodes` in `internal/gokernel/adapter_codes.go` (in that order,
since Core without it refuses the field as unknown input); retained `adapter-degradation` rows then age out under the cap,
and triage ignores them once the reader is removed. `SOL-V0-011..013` roll back by reverting the
`authorityCounted` marker and its reader, the by-design map, and `setupLines`; retained marked
rows stay readable as plain rows. `SOL-V0-014` rolls back by restoring the pathname writers in both packages and deleting
`ledgerdir.go`; the ledger files and their format are unchanged. `SOL-V0-030` rolls back by recording
`post-tool-path-not-project-relative` for every non-relative target again and removing the
exclusion set; retained exclusion rows remain admitted and age out under the cap.
