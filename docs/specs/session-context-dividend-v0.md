# Session Context Dividend V0

Owner: Russell Lewis
Frozen: 2026-08-22
Intent status: proposed
Delivery status: partial (handoff-receipt slice SESSION-V0-017..019 experimental; SESSION-V0-001..016 deferred)
Authoritative inputs: `docs/PRODUCT.md`, `docs/TECHNICAL-BRAIN.md`,
`docs/DOGFOOD.md`, `docs/specs/technical-brain-dogfood.md`

## Agent digest
- Claim: Exact-key local session receipts may reuse still-valid evidence without treating outcomes or producer metrics as truth.
- Status: proposed/partial (handoff-receipt slice SESSION-V0-017..019 experimental; SESSION-V0-001..016 deferred)
- Exists: the contract; the read-only `corvint dogfood handoff` receipt and re-resolution (SESSION-V0-017..019, ticket V1-0199).
- Blocked on: Genesis authority, access-context identity, CEM/OCM binding, SESSION-V0-001..016 implementation, and outcome trials.
- Read next: Verified current state; Requirements; Traceability.

## User and measurable job

An agent beginning a recurring engineering task should receive the exact, still-valid evidence that
a prior successful session already paid to discover, then widen only what is stale or missing. The
system must record whether this reduced complete billed input tokens, source opens, broad searches,
and latency without treating an agent belief, producer-reported metric, or successful exit label as
repository truth.

V0 proves the local receipt and exact-key reuse loop. It is an experimental bridge between today's
terminal-only `corvint record` trace and the broader Context Dividend contract; it is not a durable
knowledge updater.

Status: deferred after the independent 2026-08-22 technical-brain review. Corvint first strengthens
the CEM/OCM proof boundary and validates the Unknown Frontier on historical changes. This contract
remains design history and MUST NOT drive implementation until that flagship clears its outcome
gate.

The one exception is the handoff-receipt slice requested by owner ticket V1-0199 (2026-09-23):
`SESSION-V0-017..019` below, delivered experimentally as a read-only subverb of the existing
`dogfood` verb. It adds no `corvint session` verb, private session store, delta, capsule reuse, or
token-saving claim; `SESSION-V0-001..016` stay deferred and unimplemented.

## Verified current state

- `corvint record` writes one terminal local trace containing task text, opened/changed paths,
  verification command text, and outcome. It requires a clean tree and cannot begin before work or
  capture session measurements, evidence handles, decisions, hypotheses, or unknowns.
- `corvint query` and `corvint impact` already return compact revision-pinned packets and remain
  read-only. Dogfood still uses shell redirection and separate timing files because no typed session
  receipt exists.
- Local traces are bounded, content-digested, secret-screened, owner-only, and advisory. Their
  content digest is identity, not authorship, and successful trace labels do not establish product
  behavior.
- Together, `BRAIN-DOG-017..019` define the accepted direction. Their implementation and the paired
  context-substitution outcome gate remain `NOT_RUN`.

## Public workflow

```text
corvint session begin --task TEXT --task-key KEY [--requirement ID] [--budget-bytes N]
corvint session observe --id ID --event FILE
corvint session close --id ID --target FULL_COMMIT --outcome passed|failed|blocked|reverted
corvint session verify --delta FILE [--target FULL_COMMIT]
```

`--task` is used transiently to compile the initial query and is never persisted. `--task-key` is an
explicit stable identifier owned by the caller; V0 never infers task similarity. `--event -` reads
one bounded JSON object from standard input. All commands emit compact JSON. Only `begin`, `observe`,
and `close` mutate private session state.

## Requirements

- `SESSION-V0-001`: `session begin` MUST pin the full `HEAD` commit and tree, task-key, SHA-256 and
  UTF-8 length of the transient task, exact sorted requirement IDs, Corvint build identity, requested
  packet budget, and repository-local composition state. It MUST revalidate a non-empty eligible
  capsule before retrieval, then emit one budgeted, deduplicated minimum-witness packet and run the
  normal compact query only to widen missing, stale, or unsupported obligations. With no eligible
  capsule it runs the normal query. Any Corvint-measured initial-query latency is diagnostic
  `CORVINT_OBSERVED` data, not independent harness evidence.
- `SESSION-V0-002`: V0 MUST store receipts and deltas only beneath the actual `.git/corvint/sessions`
  directory of a standard checkout, with directories mode `0700`, files mode `0600`, descriptor-
  relative no-follow traversal, locking, append, flush, and `fsync`. It MUST reject a symlinked
  `.git`, gitfile/linked worktree, redirected object store, alternates, unsafe parent, wrong-UID
  artifact, or non-regular file before writing.
- `SESSION-V0-003`: a receipt MUST be canonical UTF-8 JSON Lines with one terminal LF per event.
  Events use contiguous zero-based sequence numbers, bind the previous event digest, and derive an
  event ID using domain-separated SHA-256 over the canonical event without its ID. Duplicate keys,
  noncanonical rows, chain discontinuities, digest mismatches, truncation, insertion, deletion,
  duplication, reordering, or bytes after a terminal event MUST fail verification against a
  separately trusted head/delta ID and emit no delta. A same-UID rewrite and full rehash creates a
  new untrusted identity; the hash chain is not an authenticity claim.
- `SESSION-V0-004`: the state machine MUST contain exactly one generated `begin`, zero or more
  `evidence`, `state`, `opened`, `broad-search`, `change`, `verification`, and `metric` observations,
  and exactly one generated `close`. Nothing may append after close. Concurrent append attempts MUST
  serialize to unique contiguous events, and at most one close may succeed.
- `SESSION-V0-005`: event input MUST use exact per-type fields and enumerations with no extension
  keys. V0 MUST reject keys or values shaped as prompts, messages, source/external bodies, command
  text or output, stdout/stderr, credentials, or arbitrary prose. A `state` observation carries only
  `hypothesis`, `decision`, or `unknown`, one typed requirement/evidence subject, and bounded
  typed references. It is a work-state pointer, not a persisted proposition, explanation, or claim
  that the state is true; free-form session memory remains outside V0.
- `SESSION-V0-006`: an `evidence` observation MUST accept a tracked repository-relative path plus a
  one-based inclusive line range and mechanically compile it at the session base commit into the
  CEM evidence identity: path, blob OID, byte span, and span digest. A caller-supplied blob, digest,
  span, authority, or evidence ID MUST be rejected rather than trusted.
- `SESSION-V0-007`: `opened`, `broad-search`, and `change` observations MUST be typed counters or
  normalized repository paths. Open and search totals are derived solely by summing their events,
  partitioned by `PRODUCER_REPORTED|HARNESS_OBSERVED`; no duplicate metric authority may report
  those values. V0 does not accept unbound hunk IDs.
  `verification` MUST store only a command reference derived
  from the initial packet or an independently supplied attestation digest, state
  `declared|passed|failed|NOT_RUN`, exit code when observed, revision, and observation source. It
  MUST NOT store argv or result bodies.
- `SESSION-V0-008`: the receipt MUST accept exactly one terminal observation for each external
  metric `inputTokens`, `outputTokens`, `agentVisibleCorvintBytes`, `automaticWidenings`,
  `reviewerCriticalMisses`, `initialPacketLatencyMs`, `capsuleLookupLatencyMs`,
  `taskWallLatencyMs`, and `correctness`; duplicates are invalid.
  Non-correctness values are non-negative integers and correctness is `PASS|FAIL`. Each carries
  source `PRODUCER_REPORTED|HARNESS_OBSERVED`, or the aggregate is exactly `NOT_OBSERVED`.
  `sourceOpens` and `callerBroadSearches` are deterministic event-derived totals with explicit
  observation-source partitions; `initialQueryLatencyMs` is a separate Corvint diagnostic. Packet bytes
  MUST never be converted into estimated tokens. Close MUST make every metric explicit rather than
  omit it.
- `SESSION-V0-009`: `session close` MUST require a full target commit, preserve outcome
  `passed|failed|blocked|reverted` as producer-reported, close the receipt, and compile a canonical
  content-addressed delta containing the base/target identities, task/spec key, head event digest,
  event count, typed evidence/state/change/verification aggregates, explicit metrics, outcome, and
  composition boundary. The delta ID MUST be domain separated and verifiable without source bodies.
- `SESSION-V0-010`: `session verify` MUST be read-only and independently reject a malformed receipt
  or delta, digest mismatch, repository mismatch, non-ancestor target, unavailable Git object,
  altered evidence, unsafe exact-span drift, or unsupported local boundary. Close verifies the
  receipt before compiling the delta; public `verify` verifies the delta and its bound local receipt,
  not an arbitrary receipt file. CEM/OCM binding is deferred. Verification MUST use sanitized,
  bounded, no-fetch Git reads and label producer outcomes and metrics as non-authoritative.
- `SESSION-V0-011`: every V0 delta MUST be same-repository `LOCAL_ONLY` and non-composable because
  Genesis authority and access-context identity are not implemented. V0 MUST NOT upload, disclose, or provide existence signals about
  local paths, identifiers, hashes, metrics, or deltas to another repository or access context.
  Hashes provide identity and integrity checking only; they are not signatures, anonymity, or
  confidentiality.
- `SESSION-V0-012`: a later `session begin` MAY reuse only a structurally verified, exact task-key
  and exact requirement-set delta whose producer outcome is `passed`, target is an ancestor of the
  current commit, evidence is stable or exactly uniquely relocated, and every required command
  reference has one `HARNESS_OBSERVED` passed observation with an attestation digest. The required
  set is mechanically the non-empty verification-command list in the initial packet; if Corvint cannot
  establish a non-empty set, replay is ineligible. Structural
  state, producer outcome, and replay eligibility MUST remain separate. Reuse is an
  `ADVISORY_ELIGIBLE` capsule view—not verified task success—MUST deduplicate handles already present
  in the widened packet, and MUST NOT promote accepted intent, a behavioral claim, test success, or
  durable product knowledge.
- `SESSION-V0-013`: failed, blocked, reverted, conflicting, stale, ambiguous, deleted, malformed,
  wrong-key, wrong-requirement, or non-ancestor deltas MUST never be successful capsule candidates.
  Begin MUST expose rejected-candidate counts and reasons, then return the fresh query as bounded
  widening rather than silently dropping uncertainty.
- `SESSION-V0-014`: V0 MUST bound an event to 16 KiB, a receipt or delta to 4 MiB, a session to 1,000
  events, references and paths to 200 per category, text identifiers to the byte limits below, ancestry
  to 10,000 commits, Git operations and blob bytes to the existing CEM budgets, and private artifact
  scans to 1,000 entries. An exceeded bound is a named failure; no event or metric is silently lost.
- `SESSION-V0-015`: existing `query`, `impact`, `feature`, `eval`, and `session verify` reads MUST NOT
  create or change session/trace state. Existing `corvint record` remains compatible but does not
  satisfy this spec. Session deltas MUST NOT enter the current trace ranker or become automatic
  learning data in V0.
- `SESSION-V0-016`: Corvint MUST dogfood the public lifecycle on the next substantive Corvint change,
  record unavailable fields as `NOT_OBSERVED`, retain any manual broad search as a product miss, and
  make no token-saving claim until a preregistered equal-tool paired baseline exists.

## Handoff receipt slice (V1-0199)

A handed-off enrollment keeps its original session key and root (`LCP-V0-003`), but without a
receipt the receiving session re-derives its context and may silently see different evidence. This
slice names what the receiver must re-resolve and makes any difference explicit. It reuses the
`corvint-dogfood-prompt/0` packet compiler (`LCP-V0-010/011`) unchanged.

- `SESSION-V0-017`: `corvint dogfood handoff --session-key KEY [--anchors TEXT]` MUST be read-only
  (`mutates=false`; no repository, enrollment, trace, ledger or index-snapshot write) and emit one
  `corvint-dogfood-handoff/0` receipt naming the session key, resolved root, bound revision (commit,
  tree, worktree state and dirty-path digest), enrollment lifecycle, base and plan digest, the
  sorted unique task anchors each with the SHA-256 of its own single-anchor resolution and task
  evidence, the SHA-256 and byte count of the LF-terminated canonical `corvint-dogfood-prompt/0`
  packet compiled from those anchors and the enrolled scope at budget 8000 and limit 10, and a
  sorted degradation list: always `frontier-authority-unavailable`, plus `enrollment-<lifecycle>`
  when the enrollment is neither active nor satisfied, `uncommitted-work` for a dirty worktree, and
  the packet resolution reason when it is not `none`. Anchors are whitespace-delimited tokens only,
  at most 32, each at most 512 bytes with no control characters; no other task text is retained.
- `SESSION-V0-018`: The receipt MUST carry `authority: none` and be emitted a second time framed by
  the untrusted repository-data envelope (`internal/repoenvelope`). It grants no authority,
  satisfies no local completion condition, and its digests are identity, not authenticity: a
  same-UID rewrite yields a different, equally untrusted receipt. A receiver MUST require its
  explicit session key to equal the receipt key (`handoff-session-key-mismatch`) and refuse unknown
  members, trailing data, a non-`none` authority, noncanonical anchors, malformed digests, or a
  foreign budget or limit as `invalid-handoff-receipt`.
- `SESSION-V0-019`: `corvint dogfood handoff --session-key KEY --receipt FILE` MUST be read-only,
  read that emitted document, and recompile the packet from its anchors and the current enrollment.
  When root, revision, enrollment, every anchor digest and the packet digest and bytes all match,
  it exits 0 with state `reresolved` and returns the byte-identical packet. Otherwise it exits 1
  with state `drifted`, lists each differing member in the fixed order root, revision, enrollment,
  anchor (by name), packet with both receipt and current values, and withholds the recompiled
  packet instead of silently substituting it.

Failure modes: a repository or snapshot change during the read fails
`dogfood-handoff-repository-drift` or `dogfood-handoff-context-drift`; an unreadable receipt fails
`handoff-receipt-unavailable`; malformed anchors fail `invalid-handoff-anchors`; `--anchors` with
`--receipt` fails `invalid-local-completion-option`; packet compiler refusals keep their
`LCP-V0-011` codes. Non-goals: persistence of receipts, transfer between repositories or access
contexts, automatic handoff by a host adapter, and any reuse decision. Rollback: remove the
`handoff` subverb (`cmd/corvint/dogfood_handoff.go` and its dispatch); it wrote no state, so nothing
else changes.

## Event shapes

The externally submitted event is one strict object. The writer adds session, sequence, previous
digest, pinned identities, and event digest.

| Type | Exact caller fields |
|---|---|
| `evidence` | `type`, `path`, `lines: {start, end}` |
| `state` | `type`, `state`, `subject`, `refs` |
| `opened` | `type`, `path`, `count`, `source` |
| `broad-search` | `type`, `scope: repository-wide`, `count`, `source` |
| `change` | `type`, `path` |
| `verification` | `type`, `commandRef`, `state`, `exitCode`, `attestationSha256`, `source` |
| `metric` | `type`, `name`, `value`, `source` |

Task keys match `[a-z0-9][a-z0-9._/-]{0,127}`. Requirement IDs match
`[A-Z][A-Z0-9-]{2,31}-[0-9]{3}`. Session IDs are 64 lowercase hexadecimal characters. Paths use
the CEM normalized repository-relative grammar and 512-byte limit. A state subject and each of at
most 64 refs is exactly one requirement ID or `evidence:sha256:` plus 64 lowercase hexadecimal
characters. Command refs are `command:sha256:` plus 64 lowercase hexadecimal characters.

Observation `source` is exactly `PRODUCER_REPORTED|HARNESS_OBSERVED`. Verification state is
`declared|passed|failed|NOT_RUN`. `exitCode` is a non-negative integer only for `passed|failed` and
otherwise null. `attestationSha256` is null or a lowercase SHA-256 digest and is required for a
`HARNESS_OBSERVED` pass. A metric event is either `{type,name,value:<non-negative integer or
PASS|FAIL>,source:PRODUCER_REPORTED|HARNESS_OBSERVED}` or
`{type,name,value:NOT_OBSERVED,source:null}`. Each identifier is UTF-8 and all non-path identifiers
not given a narrower rule are at most 256 bytes.

## Trust and authority

The agent/harness, local receipt writer, delta compiler, and future independent updater are separate
trust boundaries. The local hash chain detects mismatch against a known head but cannot stop the same
operating-system user from rewriting and rehashing a receipt. Producer-reported success and
measurements are observations, not attestations. `HARNESS_OBSERVED` plus an attestation digest makes
a capsule locally replay-eligible but does not prove authorship without an external trust mechanism.
A future updater must receive policy and authority configuration separately and re-derive Git, CEM,
OCM, intent, and test evidence before refreshing durable knowledge.

Canonical JSON is UTF-8 with keys sorted lexicographically, separators exactly `,` and `:`, no
insignificant whitespace, no non-finite values, no duplicate keys, Unicode emitted directly without
ASCII escaping, and exactly one terminal LF for stored objects. Event IDs use
`SHA-256(UTF8("corvint-session-event/0.1-experimental") || 0x00 || canonical-event-without-eventSha256)`;
delta IDs use
`SHA-256(UTF8("corvint-session-delta/0.1-experimental") || 0x00 || canonical-delta-without-deltaId)`.
Positive and negative digest vectors freeze both preimages before V0 can be called interoperable.

## Acceptance and adversarial matrix

| Requirement | Deterministic evidence |
|---|---|
| SESSION-V0-001, 012..013 | fresh query plus exact-key reuse, wrong-key, failed, stale, and non-ancestor fixtures |
| SESSION-V0-002..004, 014 | permissions, symlink/gitfile/object-store, race, limit, tamper, duplicate-key, reorder, truncate, and append-after-close fixtures |
| SESSION-V0-005..008 | strict event positives/negatives, forbidden marker scan over the private store, and explicit `NOT_OBSERVED` close fixture |
| SESSION-V0-009..011 | byte-identical delta, independent digest/evidence/ancestry verification, cross-repository rejection, and producer-authority labels |
| SESSION-V0-015 | read-only state snapshots and legacy-record regression |
| SESSION-V0-016 | dogfood receipt, full suite, CEM/OCM delivery evidence, and build-log outcome; process evidence, not a unit proof |

Unit/conformance tests prove only format, bounds, state transitions, deterministic compilation,
freshness, and authority separation. They do not prove token savings. Promotion requires at least ten
preregistered paired repeat-family tasks with at least 30% lower median complete input tokens, 25%
lower total tokens, non-inferior correctness, zero treatment-only critical misses, no increase in
unnecessary source opens, at least 80% without broad rescan, and warm p95 below one second.
One pair is eligible only when both arms contain `HARNESS_OBSERVED` input/output tokens for every
turn, tool result, and replayed-history charge, sealed-oracle correctness and critical-miss labels,
`HARNESS_OBSERVED` source-open/search counts and initial-packet/capsule-lookup/task latency. The
warm-p95 gate uses harness-observed initial-packet latency, never Corvint self-timing.
Any required `NOT_OBSERVED` value keeps that pair
`NOT_RUN` and outside the ten-pair denominator.

Immediate failure conditions are any persisted forbidden body/secret marker, fabricated pass or
authority promotion, failed-session replay, cross-context disclosure, or treatment-only critical
miss. After ten pairs, kill capsule reuse but retain measurement receipts if median input reduction
is below 15%, source opens do not fall, or treatment agents reread every capsule source in more than
20% of runs. Do not add fuzzy matching or embeddings unless exact-key safety passes and at least half
of otherwise eligible repeated tasks miss reuse only because keys differ.

## Non-goals and rollback

V0 does not include free-form memory, prompt/terminal interception, automatic activity collection,
background hooks, checkpoint/handoff/merge/revert deltas (the read-only receipt of
`SESSION-V0-017..019` is not a delta), signatures, encryption, linked-worktree
support, a daemon, database, service, upload, team synchronization, semantic task matching,
documentation generation, durable claim updates, Jira/Confluence ingestion, IDE integration, or
automatic editing. Delete the private session directory and remove the four CLI commands to roll
back; repository source, accepted intent, and existing traces remain unchanged.

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| SESSION-V0-001..016 | not started (deferred) | implementation, hostile fixtures, dogfood receipt, and paired outcome trial pending |
| SESSION-V0-017 | `cmd/corvint/dogfood_handoff.go` | `TestDogfoodHandoffReceiptReresolvesSamePacket` |
| SESSION-V0-018 | `cmd/corvint/dogfood_handoff.go` | `TestDogfoodHandoffReceiptReresolvesSamePacket`; `TestDogfoodHandoffReportsRevisionAndAnchorDrift` |
| SESSION-V0-019 | `cmd/corvint/dogfood_handoff.go` | `TestDogfoodHandoffReceiptReresolvesSamePacket`; `TestDogfoodHandoffReportsRevisionAndAnchorDrift` |
