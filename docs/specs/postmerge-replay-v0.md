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
- Blocked on: actual delta/scope/validation stage integration and CI host containment evidence; the author and draft stages replay with outward writes recorded, not performed (`PMR-V0-940`..`943`, V1-0805).
- Read next: Requirements; Trust and limits; Acceptance and rollback.

On 2026-10-05 the owner accepted the delivered scope of #395 and closed it (and parent #388) with the
remaining qualification items recorded as `NOT_RUN`: no historical replay set exists, the
qualification campaign is `NOT_RUN`, and the author and draft stages are owner-deferred, not delivered.
The owner also directed that V1-0542 and V1-0535 complete on this basis. Follow-up V1-0805 tracks the
author and draft stages, the replay set and the campaign; V1-0807 tracks the final rerun of gap checks
(a) to (d) for #388, which is `NOT_RUN`. This decision accepts no unrun witness; whole-workflow qualification stays
`NOT_OBSERVED`. See `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-05-postmerge-issues-accepted-not-run.md`.

V1-0805 (2026-10-31, proposed under the standing lane auto-accept of 2026-10-11) adds the author and
draft stages (`PMR-V0-940`..`942`) and a labelled historical replay set with its campaign
(`PMR-V0-943`). Scope and validation of authored content stay deferred, and whole-workflow
qualification stays `NOT_OBSERVED`.

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
  deferred stage as observed, and every adapter-supplied connector draft blocks, until the
  authored-content verifier is integrated (harness-authored drafts: `PMR-V0-940`..`942`). Missing, duplicate, blocked or mislabelled stages block. Counts must agree with
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
- `PMR-V0-960`: (experimental under the standing lane auto-accept, 2026-10-31; V1-3662) Free text
  that becomes a workflow identifier (grammar `^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`), such as a
  branch name with `/`, MUST be encoded by the one shared helper `postmergeworkflow.EncodeID`, or by
  `corvint-postmerge-workflow encode-id VALUE` for a script: every byte outside `[A-Za-z0-9._-]`,
  `:` included, becomes `:` and two uppercase hex digits (`claude/lane-a` is `claude:2Flane-a`).
  A value that does not begin with a letter or digit, or that would encode to more than 128 bytes,
  is refused. `DecodeID` reverses it and refuses any identifier `EncodeID` would not produce, so
  the round trip is exact. The grammar itself is unchanged.
- `PMR-V0-940`: Author stage. A result may carry `authored`: at most one entry per kind `docs` or
  `tests`, each with 1 to 64 unique files `{path, content}` (clean relative paths, content at most
  1 MiB). A `docs` entry needs documentation targets and every path must be one of them; a `tests`
  entry needs test gaps and every path must be a test file (`_test.go`, `.test.mjs`, `.test.js`,
  `_test.sh`) or lie under a `testdata/` directory. Otherwise the replay blocks
  (`authored-scope-invalid`). The harness, never the adapter, creates one authored commit per entry
  in a private Git directory inside its scratch that reads the product's objects through alternates:
  the merge tree plus the files, parent the merge, fixed author/committer `corvint-replay`, date the
  change's `merged_at` and a fixed message, so both runs produce the same revision. The product
  root's worktree, index, refs and object store contents are never written; Git may freshen the
  modification time of a shared object it reuses, which the report keeps as a limit.
- `PMR-V0-941`: Draft stage. For each authored commit the harness adds a connector draft of that
  kind, revision and the host policy's `draft_url` to the connector input, then builds, validates
  and records the plan against the private Git directory exactly as `PMR-V0-006` requires.
  `upsert-draft-change` requests are recorded outward writes: no writer is constructed, no ref,
  branch, push or draft is created anywhere. Authored content without a valid `draft_url` blocks
  (`draft-url-unpinned`, or the connector's `draft-invalid`). A connector draft supplied by the
  adapter itself still blocks (`draft-scope-validation-unavailable`).
- `PMR-V0-942`: The adapter still reports author and draft stages as `deferred` under `PMR-V0-005`.
  The report lists the stages the harness replayed in `replayed_stages` (`docs-author`,
  `tests-author` with the SHA-256 of the canonical authored entry, `draft-requests` with the SHA-256
  of the recorded draft request lines) and drops them from `deferred_stages`. When anything was
  authored it adds the limits `authored-content-scope-validation-not-verified` and
  `draft-requests-recorded-not-performed`. Follow-up is derived from the recorded plan.
  `workflow_qualification` stays `NOT_OBSERVED`.
- `PMR-V0-943`: Historical replay set and campaign. `internal/postmergeworkflow/testdata/historical/`
  holds a manifest of real commit-level changes from this repository's `origin/main`, each with its
  base, change commit, expected outcomes with basis `generated` and an evidence digest of its
  provenance record (the later commit that shows the historical follow-up, or none), plus the
  expected and last recorded campaign outcome of the changed-path reference adapter. The campaign
  replays every entry through `Replay` with that adapter and reports each entry's status and
  mismatching fields; an entry whose commits are absent reads `NOT_RUN`, never `MATCH`. The
  reference adapter is a stated changed-path rule, not the `#389` delta detector, so a campaign
  `MATCH` qualifies the harness path and that rule only.

The explicit native `postmerge-replay/1` route is governed by `postmerge-runtime-v1.md`.
Its experimental connector/intake refusal slice always blocks and does not change this `/0`
adapter contract or add positive whole-workflow qualification.

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
| PMR-V0-940 | `TestReplayAuthorAndDraftStages`, `TestReplayAuthoredBlocks` |
| PMR-V0-941 | `TestReplayAuthorAndDraftStages` (recording holds `upsert-draft-change`, product unchanged), `TestReplayAuthoredBlocks` |
| PMR-V0-942 | `TestReplayAuthorAndDraftStages` |
| PMR-V0-943 | `TestHistoricalReplayCampaign`; results recorded by V1-0805, mismatches retained |
| PMR-V0-009 | Adapter protocol in `TestAdapterHelper`; actual `corvint delta` integration NOT_PRODUCED (issue #389 not on base) |
| PMR-V0-960 | `TestPMRV0960EncodeIDRoundTrip` (`internal/postmergeworkflow/id.go`); `corvint-postmerge-workflow encode-id` |

## Non-goals and rollback

No remote adapters, live writes, label authority inference, credential discovery, unverified draft
publication or host sandbox provisioning. This partial slice cannot close #395 or parent #388.
Rollback removes the optional companion and its private scratch only. Preserve historical fixtures,
policy/label registry and retained reports; no tracker, forge, source or queue migration is performed.
