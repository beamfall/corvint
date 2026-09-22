# Retrieval benchmark diagnostics V0

Owner: Russell Lewis
Date: 2026-09-05
Requirement prefix: `RBD-V0`
Intent status: proposed
Delivery status: experimental
Authoritative inputs: `../DOGFOOD.md`, `task-context-packet-v0.md`, `../../ROADMAP.md` AT-05/08.

## Agent digest
- Claim: The development retrieval benchmark optionally retains exact bounded context output for failure diagnosis without changing ranking or scoring.
- Status: proposed/experimental
- Exists: `tools/retrieval-bench`; opt-in implementation candidate with independent review and fixed-case parity PASS.
- Blocked on: owner acceptance; retrieval and cost superiority remain unproven.
- Read next: Requirements; Evidence and rollback.

## Human intent and scope

The owner asked for baselines and a repeated failure, single-mechanism change, matched measurement,
independent review, retain-or-revert loop. The first selected known failure returned NO_CANDIDATES,
but bounded source inspection recovered relevant implementation details. The benchmark parser had
already discarded nearest handles and unsupported-term details. Retaining original context bytes
makes that diagnosis reproducible without rerunning a changed producer or reconstructing its output.
This private development artifact does not certify task success, source coverage or billed cost.

## Requirements

- `RBD-V0-001`: The benchmark MUST accept opt-in `--context-packets FILE` only with a context arm
  and ordinary pre-run registration (`--registration` or `--output`), and MUST refuse summarize.
  Default report/registration profiles, ranking, scorer, Corvint commands and index contracts stay unchanged.
- `RBD-V0-002`: Capture MUST create a new exclusive mode-0600 file, refuse existing paths and
  symlink components, and prevent snapshot/corpus input overlap or equality with samples, registration
  and report paths. Refusal preserves prior files. It MUST NOT delete, reuse or overwrite artifacts.
- `RBD-V0-003`: After ordinary registration and before retrieval, capture MUST write a JSONL header
  binding registration, producer and ordered sample/invocation identities. Actual call sites MUST bind
  sample ordinal, ID/repository/base, invocation ordinal and ordinary/cold/hit phase; task digest and
  byte count bind the actual input. Duplicate sample IDs remain distinct by ordinal.
- `RBD-V0-004`: Each invocation MUST retain byte-identical complete bounded stdout as base64 with
  byte count, SHA-256 and parse status, including successful empty/malformed output. Transport failure,
  timeout and stdout overflow MUST report NOT_PRODUCED without presenting a prefix/hash as full stdout.
  Bounded stderr MUST carry its own retained count/hash and COMPLETE/TRUNCATED state. Existing arm
  errors retain their meaning; stderr truncation alone does not imply stdout capture incompleteness.
- `RBD-V0-005`: Capture MUST bound the entire encoded file including footer to 64 MiB, header to
  1 MiB, each record's metadata excluding stream base64 to 64 KiB, and expected invocations to 1,024.
  Existing 8 MiB stdout/64 KiB stderr bounds remain. Each complete line is checked before writing.
  Capture integrity/write/budget failures MUST fail the benchmark before normal report output.
- `RBD-V0-006`: Capture disk writes MUST occur after timed retrieval calls, including error paths.
  Disabled capture MUST retain no raw diagnostic collection and perform no diagnostic writes.
  Observer bookkeeping remains overhead; capture-on/off latency is not a speed comparison.
- `RBD-V0-007`: Only after existing identity verification may capture emit its terminal footer,
  binding status, invocation count, entire encoded byte count and SHA-256 of exact preceding lines.
  COMPLETE requires every expected descriptor in sequence and every required stdout produced;
  unavailable stdout yields PARTIAL. Missing/invalid footer, partial line, digest/count mismatch or
  trailing bytes cannot establish completeness. Earlier failure/cancellation leaves an incomplete
  artifact. Capture is closed before report publication; a later report sink failure does not
  retroactively invalidate captured bytes or imply successful report/task execution.

## Artifact representation

JSONL uses `type: header`, `type: invocation`, and terminal `type: footer`. Header profile is
`corvint-retrieval-context-capture-v0`; `registration` preserves the ordinary registration and
`invocations` contains the expected zero-based descriptors. Each record has `invocation`, `stdout`,
`stderr`, `parse_status` (PARSED/MALFORMED/NOT_RUN), and optional bounded `error`. Available streams
carry `status`, `base64`, `bytes` and `sha256`; NOT_PRODUCED omits payload, byte count and hash.
The footer has `status`, `invocations`, `total_bytes` and `prior_sha256`. Ordinals and actual task
bytes/SHA-256 form part of each descriptor. This is a separate experimental artifact, not a change
to the existing report or registration profile.

## Non-goals and failure modes

No ranking change, packet reinterpretation, source extraction, corpus admission, new process wrapper,
host telemetry collection, pricing estimate, daemon or publication. Capture completeness concerns
retained bytes only. A syntactically malformed packet may be completely captured while failing its
parser; unavailable stdout cannot be recreated from stderr. Exceeding a bound refuses visibly.

## Evidence and rollback

Freeze the same selected development case and producer binary before the change. Retain a black-box
RED result for the unavailable flag, then verify exact packet replay and enabled/disabled semantic
report equality (excluding timing and registration identities). Cover malformed/empty output,
transport failure, stderr truncation, phase identity, duplicate/symlink/overlap refusal, budgets,
write failures, cancellation and incomplete finalization. Use the existing owned-process supervisor.
Run focused benchmark tests/vet, independent implementation review and the canonical gate.

The selected case is already observed development evidence, not held-out evaluation. Frozen full
retrieval results remain unchanged; this repair cannot establish a retrieval, task-success or cost
win. Retain the original failure and NOT_PRODUCED reasons. Rollback removes only this opt-in
benchmark recorder and leaves captured artifacts readable; no schema/index migration is involved.

The final development repeat retained the exact original 2,325-byte packet, matching registration
and footer digests, and identical semantic benchmark reports with capture enabled/disabled. Fresh
independent review is clean after regressions reproduced and repaired macOS path aliases and a
redirected held parent. These are diagnostic-integrity results; retrieval remains abstained and
complete cost remains NOT_OBSERVED. See `../reviews/improvement-loop-2026-09-05/`.
