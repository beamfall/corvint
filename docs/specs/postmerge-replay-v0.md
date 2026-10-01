# Post-merge Replay V0

Owner: Russell Lewis
Date: 2026-10-01
Intent status: proposed
Delivery status: experimental
Authoritative inputs: human issue https://github.com/beamfall/corvint/issues/395,
native V1-0542 revision 5; decision 0373; `AGENTS.md`; contracts `postmerge-connectors-v0.md`
(issue #392) and the documented `corvint delta` record of issue #389.

## Agent digest
- Claim: An optional companion runs a pinned local adapter twice and compares typed historical expectations through the actual recording connector.
- Status: proposed/experimental harness; whole-workflow qualification `NOT_OBSERVED`.
- Exists: `internal/postmergeworkflow/`, `cmd/corvint-postmerge-workflow/`.
- Blocked on: actual delta/author/scope/validation stage integration and CI host containment evidence.
- Read next: Requirements; Trust and limits; Acceptance and rollback.

## User and measurable job

A maintainer replays a historical merge without tracker or forge credentials and gets identical
recorded requests on repeated execution, plus mismatches separated by expectation basis. The
human issue requires the whole workflow. This first experimental slice proves execution of the
adapter protocol and real local recording connector only. Synthetic adapters are not delivery
or qualification of the whole workflow, and passing expectations do not accept generated intent.

## Requirements

- `PMR-V0-001`: Decode a bounded closed fixture containing immutable connector identity and
  expectations for affected flows, follow-up, test gaps and known defects. Every field has a
  generated or human-verified basis and evidence digest. Reject duplicate keys, unknown fields,
  case aliases, trailing JSON, excessive nesting, unsafe paths, duplicate outcomes and malformed enums.
- `PMR-V0-002`: Use a separate host-owned policy to pin the connector binding, executable bytes,
  bounded arguments and runtime configuration digest. Verify actual commits, ancestry and changed
  paths through the existing connector. Fixture content cannot choose commands or credentials.
- `PMR-V0-003`: Admit human labels only when one separate pinned registry entry matches full immutable binding,
  field, exact value digest, evidence digest, human ID and approval ID. Generated labels cannot
  self-promote. Registry pinning records a host claim, not authentication or independent truth.
- `PMR-V0-004`: Execute the pinned adapter bytes in two fresh private directories with bounded
  stdin/stdout/stderr, timeout, cancellation and process-group cleanup. Use explicit minimal
  environment, clean HOME/TMPDIR and fixed system PATH; do not forward raw title/body or inherited
  credential environment variables. Retain the limits of bounded descendant observation.
- `PMR-V0-005`: Require a result bound to the complete fixture digest, runtime digest and change
  identity. Code fixes stage applicability: trigger, intake, delta, follow-up, findings and metrics
  are always required and observed. Documentation author/scope/validation stages are `deferred`
  when documentation targets exist, test author/scope/validation stages when test gaps exist, and
  draft requests when either exists; otherwise each is `not-applicable`. No driver may report a
  deferred stage as observed, and every connector draft blocks, until the authored-content verifier
  is integrated. Missing, duplicate, blocked or mislabelled stages block. Counts must agree with
  outcomes, corpus count is zero and findings must match connector findings.
- `PMR-V0-006`: Build and validate the actual connector plan, record it twice in each run and prove
  that repeats add no ledger events. Compare canonical outcomes, stage digests and full recorded
  bytes across fresh runs. Reject nondeterminism. Derive follow-up from actual plan requests.
- `PMR-V0-007`: Emit a separate report with fixture/policy/binding identities, canonical JSONL,
  recording digest and deferred stage names. Report mismatches against human-verified expectations
  and mismatches against generated expectations in two separate lists, each entry naming its field
  and expected/observed digests. MATCH means harness outcomes match expectations;
  `workflow_qualification` remains `NOT_OBSERVED` for all outcomes. Error reports omit adapter
  stdout/stderr and retain bounded machine-readable reasons.
- `PMR-V0-008`: Provide local/CI invocation `corvint-postmerge-workflow replay --change ID --dry-run`
  with explicit fixture, policy and product root. No live writer, automatic merge, Core registration
  or credential lookup. Exit 0 is MATCH, 1 is MISMATCH and 2 is BLOCKED or invalid invocation.
  Full issue completion still requires executing actual child stages and their validation,
  historical expectations with generated or human-verified basis, qualified host containment,
  integration and native completion evidence. Human labels are optional; human-verified coverage
  remains `NOT_OBSERVED`.
- `PMR-V0-009`: Replay does not parse or reimplement the issue #389 delta record. The adapter
  result is the narrow interface: `affected_flows` carries flow identities whose documentation spans
  the delta record reports stale, retired or added; `test_gaps` carries its uncovered-behaviour gap
  identities; the `delta` stage digest is the SHA-256 of the canonical record bytes. Until that
  command lands on the base, an adapter supplies these values from its own runtime, bound by
  `runtime_sha256`, and the report gains no delta qualification.

## Trust and limits

The operator supplies trusted policy, runtime/configuration binding and adapter implementation.
The adapter may read host files or access the network: sanitized environment is not OS isolation.
Copying pinned executable bytes proves what was launched, not its semantics, interpreter, shared
libraries or external inputs. The host must bind those to `runtime_sha256` and provide actual
containment. A driver-supplied stage digest is an observation, not verification of the child stage.
No positive whole-workflow claim follows from matching digests or synthetic protocol tests.

JSON input/output is at most 4 MiB and 64 nesting levels. Executable bytes are at most 128 MiB;
arguments at most 32 of 2048 bytes; IDs at most 128 bytes; paths at most 512 bytes; flow/target/gap
sets at most 256; findings at most 64; registry entries at most 1024. Process timeout is 30 seconds,
shutdown 2 seconds, stderr 64 KiB. Private scratch is removed after success, blocking or cancellation.
The existing process supervisor observes descendants, including sampled detached descendants, but
cannot prove absence of every fast escape; this remains an explicit qualification limitation.

Fixtures and policies are read into bounded snapshots. Input paths are resolved for connector
destination protection. Recordings use a private scratch ledger outside the product, author and
input paths and the connector's destination guard; the report retains the ledger after cleanup.
Host policy and its arguments must not contain credentials. No adversarial host or concurrent
filesystem-replacement qualification is claimed. Fixture title/body remains in the parent intake
snapshot and its digest; adapter stdin contains only identity and digests.

## Acceptance and traceability

Run `GOTOOLCHAIN=local go test -count=1 -timeout 30m ./internal/postmergeworkflow ./cmd/corvint-postmerge-workflow`
and focused vet. The conformance helper is explicitly synthetic and never a historical human label.

| Requirements | Evidence |
|---|---|
| PMR-V0-001 | `TestClosedJSON`, `TestPinsAndSelector`, `TestReplayBlocks` |
| PMR-V0-002 | `TestPinsAndSelector`, actual disposable Git commits in `setup` |
| PMR-V0-003 | `TestHumanRegistryCannotSelfPromote`, `TestReplayMismatchBasis` |
| PMR-V0-004 | `TestReplayActualRecording`, `TestCancellationRetiresDescendant` (Darwin/Linux) |
| PMR-V0-005 | `TestReplayBlocks`, `TestReplayDeferredAuthoringStages`; positive author/scope/validation NOT_PRODUCED |
| PMR-V0-006 | `TestReplayActualRecording`, `TestReplayTwiceIdenticalRecording`, nondeterministic case of `TestReplayBlocks` |
| PMR-V0-007 | `TestReplayMismatchBasis`, `TestReplaySeparatesHumanVerifiedMismatches`, `TestReplayDeferredAuthoringStages` |
| PMR-V0-008 | `TestCLIReplay`; full integration and whole-workflow acceptance NOT_PRODUCED |
| PMR-V0-009 | Adapter protocol in `TestAdapterHelper`; actual `corvint delta` integration NOT_PRODUCED (issue #389 not on base) |

## Non-goals and rollback

No remote adapters, live writes, label authority inference, credential discovery, unverified draft
publication or host sandbox provisioning. This partial slice cannot close #395 or parent #388.
Rollback removes the optional companion and its private scratch only. Preserve historical fixtures,
policy/label registry and retained reports; no tracker, forge, source or queue migration is performed.
