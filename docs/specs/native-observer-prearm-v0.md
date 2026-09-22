# Native observer pre-arm V0

Owner: Russell Lewis
Date: 2026-09-09
Intent status: proposed
Delivery status: experimental
Authoritative inputs: owner's full protected-runner task; `AGENTS.md` experimental prototype
allowance; `agent-harness-integration-v0.md`; `local-completion-policy-v0.md`.

## Agent digest
- Claim: An explicit idle pre-arm observer produces bounded setup and compared completed receipt projections without granting host authority.
- Status: proposed/experimental; fixture success does not qualify an actual native surface.
- Exists: explicit idle join, expected all-event inventory, READY and final completion binding.
- Blocked on: independent source review, final local evidence, native idle-join feasibility and actual qualification.
- Read next: Requirements; Inventory profile; Acceptance and rollback.

## User and measured job

An active-only observer cannot reliably attach before the first controlled native event. The optional
`corvint-native-hook-observer client ABSOLUTE_CWD --prearm-idle EXPECTED_INVENTORY_FILE [COMPARISON_GOLD_FILE]` mode joins a loaded idle task supplied
only on bounded stdin, then signals readiness. The existing inventory-only and `--subscribe` modes
retain their existing projection shapes. The task must already satisfy the installed implementation's
persisted-rollout precondition; the observer does not create, rename, fork, persist or start a task.

There is no atomic join-only API. Before/after metadata, membership and peer checks detect observed
races; they cannot prove no hidden race occurred. Inherited completed history is permitted, but
`includeTurns: false` and `excludeTurns: true` must suppress its return. These options do not prove
history never existed. Native idle join and delivered events remain separately measured feasibility.

## Requirements

- `NPO-V0-001`: The explicit idle mode MUST check loaded membership, metadata-only thread/read,
  thread/resume with excludeTurns true, metadata-only thread/read and loaded membership in that
  order. Every metadata result MUST match the input task and cwd, idle status and empty returned
  turns; resume MUST also have matching top-level cwd and no history page. Unknown/repeated/combined
  mode arguments MUST fail. Active mode MUST preserve its existing request sequence and projections.
- `NPO-V0-002`: Idle mode MUST compare an initial and final hooks/list inventory against the
  caller's single-read expected inventory. The expanded profile MUST preserve all schema-admitted
  command-hook metadata under the normalized contract below, ordered as received. Unknown fields,
  unsupported handlers/events/types, duplicate input keys, invalid hashes and mismatches MUST fail.
  The legacy stop-only profile MUST remain unchanged. This expectation is measurement configuration,
  not authority or proof that the currentHash field binds all other fields.
- `NPO-V0-003`: Idle mode MUST emit one bounded READY projection only after setup inventory,
  membership, metadata and unchanged peer identity succeed. Final projections MUST remain buffered
  until final membership, inventory and peer checks pass. A separate COMPLETE marker MUST be last
  and bind the count and SHA256 of every ordered projection including READY. Consumers MUST require
  the complete marker, matching count/hash and exit zero; READY or a partial write is not success.
- `NPO-V0-004`: Any observed hook before READY MUST refuse setup. Missing membership, unexpected
  metadata, changed peer image/birth/code identity and final inventory drift MUST refuse. Failure
  after READY MAY leave only READY or an incomplete output prefix, never a valid completed campaign.
  Raw task IDs, cwd, command, history, keys, plugin IDs and status text MUST NOT be emitted.
- `NPO-V0-005`: The existing total 30-second connection lifetime, final-check reserve, 65536-byte
  inbound frame limit, 256 frame count (inclusive: 256 frames then end of input is admitted, a 257th
  refuses), 32 observations and 8192-byte per-projection limit MUST
  remain. Expected inventory input MUST be a regular file read once with at most 65536 bytes and
  32 hooks; output size may impose a smaller admission set. Socket closure MUST hold on refusal,
  timeout, SIGINT and SIGTERM, with no descendants. No daemon, model turn, config override or
  notification allowlist expansion is included.

- `NPO-V0-006`: Optional comparison gold MUST be a single-read regular file, at most 65536 bytes
  and 32 ordered expected completed runs, with closed duplicate-free fields. It MAY contain the
  caller-authored fixed test task required by canonical validation; it MUST NOT capture native
  prompts or raw task IDs. Before entry text is discarded, the comparator MUST validate the exact
  Corvint envelope and canonical legacy dogfood or reviewed qualified lifecycle receipt, request
  hash, result digest, repository snapshot, selector sets and critical-missing expectations.
  Truncated/output-file references MUST refuse without opening the referenced path. Only hashes,
  byte counts, repository revision/dirty summary and enum/count outcomes may enter output.
- `NPO-V0-007`: Comparison mode MUST require every expected completed summary in order and refuse
  duplicates or conflicting observed starts. An independently complete summary with identity,
  event/source, entries/status and completed timing MUST NOT require a separately delivered start.
  Observed starts MUST match their completion; unfinished observed starts refuse final completion.
  SessionEnd's native empty output MUST remain NO_RECEIPT. Stop's exact reviewed status text and
  digest/decision MAY prove delivery, but continuation consumption MUST remain NOT_OBSERVED until
  separate actual native activity evidence exists. Default observation bytes MUST remain unchanged;
  comparison observations and READY MUST use explicit new profile discriminators. Before READY,
  every expected case MUST uniquely bind its complete normalized hook digest to an enabled
  synchronous inventory command; event/source-path and observed source/order/mode/scope MUST
  agree. Observed start/completion pairs MUST also agree on those stable metadata fields.

- `NPO-V0-008`: Optional native continuation evidence MUST use only typed turn/started,
  turn/completed, item/started and item/completed notifications for the owned thread. It MUST
  require the same turn's matched blocked Stop, a new agentMessage started and completed after
  that block, matched expected recursive release, then successful turn completion. Hook occurrence
  identity MUST include thread/nullable-turn/run/startedAt so recursive reuse of run ID is allowed
  while duplicate occurrences refuse. Only metadata, digests, receive sequence and enum outcomes
  may be retained. An optional fixed caller-authored expected result-text digest MUST compare to
  the last new completed agentMessage before release; mismatch refuses. Without that predicate,
  semantic consumption MUST remain NOT_OBSERVED for independent operator evidence. Neither native
  activity nor a Stop status digest alone proves that the model followed a correction. Existing
  32-observation, frame, output and connection limits MUST remain unchanged.
- `NPO-V0-009`: The `observe` mode's input read MUST be bounded by its stated lifetime on every
  input kind, including a blocking non-pollable fd such as a shell-pipeline stdin that ignores read
  deadlines. Such input is admitted, not refused: the read runs on a goroutine against the lifetime,
  and on expiry the mode MUST emit the existing `observer-input-unavailable` failure and return 1.
  The abandoned blocked read ends with process exit; no further read is issued after it.

## Inventory profile

`corvint-native-hook-inventory/1-experimental` has exactly profile, qualification (UNQUALIFIED) and
hooks. Each hook has exactly eventName, commandSHA256, sourcePathSHA256, currentHashSHA256,
matcherSHA256, enabled, async, trustStatus, timeoutSec, source, displayOrder, isManaged,
additionalContextLimit, keySHA256, pluginIdSHA256 and statusMessageSHA256. Handler type is fixed
command. Strings retained as digests use SHA256 of exact UTF-8 bytes; nullable digest values preserve
null distinctly from an empty string. List order is significant. Optional async defaults to false;
optional nullable metadata defaults to null. Numeric fields retain their schema integer domains:
timeoutSec/additionalContextLimit unsigned 64-bit, displayOrder signed 64-bit. The uint spill-limit
profile is the measured 64-bit installed runtime. Booleans cannot substitute for integers.

The pinned `tools/native-hook-observer/hook-inventory-schema.json` records the actual installed
Codex 0.153.4 engine SHA256/CDHash, generation argv and full HooksListResponse definition closure.
Its source schema SHA256 is 891dd10ef7f78e59631fce05fff2becddb8004b3c0338dbbbd6b4f17ef1fa64f.
Engine/schema pins establish examined bytes, not runtime authority. No currentHash completeness
assumption is made: independent review required source/order/spill/identity metadata preservation.

READY uses profile corvint-native-observer-ready/experimental, setup READY, qualification UNQUALIFIED,
mode idle, threadSHA256/cwdSHA256/peerSHA256/inventorySHA256 and maximumConnectionSeconds 30.
Peer hash input is ASCII decimal PID, a NUL separator, fixed native birth bytes and code digest.
Inventory and final projection hashes use UTF-8 JSON with sorted keys, compact separators and one
trailing newline per record. COMPLETE uses profile corvint-native-observer-complete/experimental,
qualification UNQUALIFIED, completion COMPLETE, projectionCount and projectionsSHA256. Its digest
covers ordered READY, inventory and observation records and excludes the COMPLETE record itself.
All final records are size-checked before the first final record is written. Completion does not
prove a turn ended, every event arrived or Stop continuation was consumed.

## Optional comparison gold

`corvint-native-comparison-gold/0-experimental` contains exactly profile, cwdSHA256 and ordered cases.
Every case contains kind, eventName, sourcePathSHA256, hookSHA256 and scope (thread or turn).
The hook digest binds the complete normalized inventory row, including command digest, matcher,
source/order and execution configuration. Missing or ambiguous mappings refuse before READY, including multiple enabled rows indistinguishable
by notification-visible event/source-path/source/order/mode despite different command digests;
disabled/asynchronous commands cannot be treated as measured synchronous hooks. Context cases additionally contain
receiptProfile (corvint-dogfood-event/0 or corvint-qualified-lifecycle/0), normalized fixed-test input,
repository, selectorsSHA256, criticalMissingSHA256 and unavailableSelectorsSHA256. Input contains
only optional sessionIdSha256 and startSource for SessionStart, or task for UserPromptSubmit.
The fixed task is caller-authored test configuration; it is read, never emitted or copied to logs.
Raw native session IDs are not admitted. Snapshot gold includes commit/tree/object format and dirty
state/count/digest; exact setup cwd hash binds root independently because receipts do not contain cwd.
Selectors hash the ordered governance/declared_scope/task_evidence object using adapter canonical JSON.
Critical-missing and unavailable-selector arrays are independently compared; matching nonzero arrays
never proves critical recall.

Stop cases contain entryKind, textSHA256, resultDigest, decision and boolean stopHookActive, alongside
common fields. The recursion flag is caller-authored expected receipt configuration; native status
text exposes its matched result digest, not the flag itself.
No-receipt cases admit only sessionEnd and common fields. Stop's expected text digest freezes the
complete reviewed grammar, and native status must agree with block/release. These outputs cannot
prove hidden receipt internals or native continuation. Complete summaries may have null turn IDs.

The qualified contract is reviewed commit d5ebd2b0f0d76d5a4908cd4f90800c91426e0b36,
`cmd/corvint/qualified_lifecycle.go` SHA256 5eeaf0c634a647e67650ba2d83070a135584eb4964da3eaeb3ca7dbe4370604c.
The native comparator rederives the reviewed canonical request/result hashes and validates the closed
receipt, repository, host/authority, context, and selector contracts in-process; it starts no validator
subprocess. Qualified top-level and host/authority fields remain closed.
Caller-asserted provenance and unattributed runtime origin remain explicit even for a FULL receipt.

Comparison READY uses corvint-native-observer-ready/1-experimental and adds comparisonGoldSHA256.
Comparison observations use corvint-native-hook-observation/1-experimental with comparison null for
starts and a bounded corvint-native-receipt-comparison/0-experimental result for completed runs.
The unchanged final count/hash binds these exact records. Missing expected completions refuse final
publication. Actual native activity/continuation observation is a separate witness, not inferred here.

## Optional native continuation witness

Gold profile corvint-native-comparison-gold/1-experimental adds exactly continuation with blockedCase,
releasedCase and nullable resultTextSHA256. Ordered case indices must select stopHookActive false/block
then true/release expectations. Expected receipt/status digests are independently frozen test gold;
the observer does not turn their caller assertions into authenticated native request provenance.

Only this explicit profile removes the four named turn/item lifecycle methods from the existing
notification opt-out set. Other modes retain the original set. Installed schema hashes and admitted
metadata fields are pinned by `tools/native-hook-observer/activity-schema.json`; nested bodies are
not interpreted except bounded completed agentMessage text compared in memory with the fixed digest.
Owned thread/turn/item IDs become hashes; raw messages, prompts, tools and turn histories are discarded.
Agent-message activity is the sole admitted correction-result predicate in this bounded slice; an
independent file-effect witness remains operator evidence and is not fabricated from metadata.

Each comparison-mode hook and activity projection carries its receive sequence. The final
corvint-native-continuation/0-experimental projection distinguishes activity OBSERVED, recursiveRelease
EXPECTED_RECEIPT_DIGEST_MATCH, turnCompletion SUCCEEDED and semanticResult MATCH or NOT_OBSERVED.
It retains caller-asserted request provenance and unattributed event surface. The final COMPLETE
record binds this witness with every prior projection. The original 32 total notification observations
remain; a fixture too large or slow for those limits refuses instead of claiming a partial campaign.

## Acceptance and rollback

| Requirement | Implementation | Executable evidence |
|---|---|---|
| `NPO-V0-001` | idle setup and unchanged active subscription | `TestIdlePrearmExactMetadataOnlySequence`; `TestActiveSubscribeUsesExistingIdentityAndNoHistory` |
| `NPO-V0-002` | closed normalized inventory and bounded expected file | `TestExpandedInventoryPreservesEveryAdmittedFieldByDigest`; `TestInventoryProjectionAndExpectedFileProtection` |
| `NPO-V0-003` | setup/final renderer and completion binding | `TestReadyCompleteBindsOrderedPrivacyProjections` |
| `NPO-V0-004` | pre-ready refusal and final guards | `TestNotificationsAreThreadScopedBufferedAndCapped` |
| `NPO-V0-005` | unchanged transport bounds and socket ownership | `TestWebSocketUpgradeFramesAndDeadlineCleanup`; `TestWebSocketOutboundIsMaskedAndInboundRejectsFragmentation`; `TestObserverFrameCountBoundaryIsInclusive`; `TestReceiveTimeoutMidFrameIsNotResumable` (a deadline after a frame's first byte is a hard failure, never the end of the observation window); observer deadline/interruption regressions |
| `NPO-V0-006` | canonical receipt and independent gold comparison | `TestContextComparisonBindsCanonicalReceiptAndRejectsTampering` |
| `NPO-V0-007` | completed-run comparison and explicit witness limits | `TestComparisonStopAndNoReceiptAreOrderedAndUnqualified` |
| `NPO-V0-008` | occurrence identity and bounded native continuation witness | `TestContinuationRequiresSameTurnNewCompletedActivity`; `TestContinuationRejectsMissingWrongAndPreblockActivity` |
| `NPO-V0-009` | observe-mode input read bounded by the lifetime on any fd (decision 0217) | `TestObserverBlockingPipeInputHonorsLifetime` (`syscall.Pipe` fd, 10x-lifetime hang detector); `TestObserverPartialFrameDeadline` (pollable pipe) |

These native Go fixture claims may remain unassessed where the existing OCM extractor cannot
admit them; actual execution receipts are retained separately. Never add fake Go anchors or broaden
claim extraction to make this slice appear complete. Local completion evidence and unassessed
obligations remain distinct from native qualification. Full AHI cases, receipt comparator, closure
schema, continuation consumption, protected principal separation, performance and critical recall
remain separate gates. Rollback removes the new mode, expanded profile and proposed spec while
preserving failed probes and the existing modes.
