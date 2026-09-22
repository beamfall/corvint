# Qualified lifecycle V0

Owner: Russell Lewis
Date: 2026-09-09
Requirement prefix: `QLF-V0`
Intent status: proposed
Delivery status: experimental
Authoritative inputs: `docs/specs/agent-harness-integration-v0.md`, `docs/specs/protected-local-execution-v0.md`, `docs/specs/local-completion-policy-v0.md`, `docs/decisions/0009-harness-authority-boundary.md`

## Agent digest
- Claim: An optional protected adapter can deliver bounded lifecycle context with current qualified capability and separately computed Stop authority.
- Status: proposed/experimental; source and fixture tests are not native qualification or root admission.
- Exists: closed qualified-event consumer, protected scope bracketing, uninstalled adapter mode and candidate lifecycle campaign path.
- Blocked on: independent source review, accepted optional profile/root, eleven actual AHI cases, exact native latency/equal-critical-recall evidence and completed qualification admission.
- Read next: Requirements; Wire and trust boundary; Acceptance and rollback.

## Human intent and verified starting point

The owner renewed the goal of FULL native Corvint participation. At source
`871423898a5d413bebada15ebe4a7bf2b93df970`, ordinary startup and prompt hooks still emit the
legacy FALLBACK profile, while the separate protected adapter handles Stop only. Installing a
completed protected HostQualification alone cannot promote ordinary context delivery. This proposal
adds the missing route while preserving accepted local workflow and legacy wire meanings. It does
not accept itself, the optional execution root, or a completed native qualification.

The simpler baseline remains the unprivileged `corvint-dogfood-event/0` adapter. This proposal is
useful only when a separately reviewed optional protected installation supplies the actual immutable
consumer/adapter/runtime scope; default local use continues to need no account or service.

## Requirements

- `QLF-V0-001`: The additive `corvint-qualified-lifecycle/0` request MUST be a closed object containing only profile, one of the four existing event names and the existing closed normalized event input. Caller root, host, qualification, authority, enrollment, target and permission fields MUST refuse. Request provenance MUST remain caller-asserted; same-engine invocation is not independent evidence that a native event occurred.
- `QLF-V0-002`: Completed qualified non-Stop events MUST derive repository scope exclusively from the held actual cwd and fixed protected root/floor/policy/runtime. They MUST recheck those identities and actual directory/configuration after local context reads, never open protected enrollment/publication/evidence/private state, and report authority NONE with Frontier NOT_EVALUATED.
- `QLF-V0-003`: Qualified Stop MUST derive the active handle inside the same protected scope, compute the real protected Frontier exactly once, perform read-only local-policy observation before final guards, and recheck the active pointer, publication, view, runtime, root/floor/policy and actual cwd. Handle replacement or missing evidence MUST refuse rather than synthesize EMPTY.
- `QLF-V0-004`: The local completion result and protected Frontier result MUST remain separate. The native decision may request at most one remediation when either enrolled local policy or protected root permission requires it. Recursive Stop MUST release unresolved; local policy cannot grant execution authority and EMPTY cannot erase an unmet local policy.
- `QLF-V0-005`: FULL MUST mean current independently completed exact native tuple support, with truthful image identities, qualification/evidence digests and support scope. Shared runtime MUST list both qualified Desktop and CLI surfaces and keep each caller event unattributed. Candidate success MUST remain FALLBACK/UNQUALIFIED; no installed flag, caller value, fixture, incomplete qualification or invalid present qualification may promote it. `degradations` MUST be the support codes (none for FULL, exactly `native-tuple-unqualified` first for FALLBACK) followed by exactly the `compaction-*` codes that a session-start context's own `compaction` block raises, in `compaction-dirty-set-over-budget`, `compaction-untracked-paths-not-rehydratable`, `compaction-critical-evidence-overflow` order (`AHI-003`, decision 0241); a compaction code never changes support or qualification, and the renderer MUST refuse any other code, order, or a `compaction` key outside session-start.
- `QLF-V0-006`: The request digest MUST bind the normalized request; the domain-separated result digest MUST bind the complete response except itself. The native renderer MUST validate the closed response and qualification/authority semantics, preserve the existing untrusted repository framing and enforce the complete 8000-byte native output bound. Startup/prompt fitting MUST include final tuple and escaped native envelope costs, preserving explicit context omissions; Stop carries no context packet.
- `QLF-V0-007`: The optional immutable `authority_hook.py --qualified-lifecycle` mode MUST normalize only immediate bounded native input, hash session identity, avoid transcripts/raw IDs/environment authority, and use pinned process replacement without a child writer or persistent prompt file. The template MUST remain uninstalled/unadmitted and named fallback MUST keep unrelated coding available. Legacy authority-hook mode and existing harness/local-completion profiles MUST retain their behavior.
- `QLF-V0-008`: Candidate bootstrap MUST use only the new explicit root-owned campaign scope ALL_NATIVE_LIFECYCLE_EVENTS_IN_EXACT_REPOSITORY on qualified-event. It MUST retain the finite canonical <=900-second root/floor/policy/release/process binding and no self-renewal rules. Existing authority-event MUST continue accepting only its original all-native-Stops scope. A present invalid completed qualification MUST NOT fall back to a campaign.
- `QLF-V0-009`: Candidate non-Stop MUST execute the same context/compiler/renderer path, using campaign target/handle only as root-owned bindings, not enrollment evidence. It MUST bind actual held HEAD to target before/after and recheck campaign bytes/expiry, runtime, root/floor/policy and cwd/configuration, without protected enrollment/publication reads. Candidate Stop MUST retain actual enrollment and single protected computation. Candidate output MUST include actual image identities, no completed qualification/evidence digest or qualified surfaces, and explicit candidate support scope.
- `QLF-V0-010`: Promotion MUST retain exact native AHI event, privacy, cleanup, installation/revocation and latency/equal-critical-recall evidence. Candidate admission overhead MUST be disclosed; its smaller receipt cannot substitute for the final FULL budget. Serialization-only final-shape tests carry no admission or native timing evidence. Source tests or temporary campaign success MUST NOT self-promote.

## Wire and trust boundary

The only command is `corvint qualified-event --input - [--native-output]`; it rejects global root
routing. Native input is at most 131072 bytes and normalized task text uses the existing 2000-codepoint,
16384-byte LCP query bounds. Events are session-start, user-prompt, stop and session-end. Stop requires
an explicit boolean stopHookActive. Other normalized event members retain the existing local-completion
profile. Raw session identity is never forwarded or persisted; its domain-separated SHA256 is only a
local workflow lookup hint. The adapter does not read CODEX environment identity for this new mode.

The complete response has these required top-level members:

| Member | Meaning |
|---|---|
| profile, ok, mutates | corvint-qualified-lifecycle/0, true, false |
| event, requestProvenance, requestSha256 | Closed event, caller-asserted, SHA256 of canonical normalized request |
| support, qualification, degradations | FULL/QUALIFIED/empty array or FALLBACK/UNQUALIFIED/native-tuple-unqualified; a compact session-start then appends its compaction block's `compaction-*` codes (QLF-V0-005) |
| qualifiedHost | host, digest, evidenceSHA256, appSHA256, engineSHA256, adapterSHA256, osBuild, architecture, supportScope, eventSurface, qualifiedSurfaces |
| repository | Existing local event commit/tree/object format and dirty-state envelope |
| policy, completion | Existing read-only local workflow projection and local decision/reason |
| decision | Composed native decision/reason; separate from local completion |
| authority, frontier | NONE/NOT_EVALUATED for non-Stop; VERIFIED and actual OPEN/EMPTY plus universeSHA256, decision and reason for Stop |
| resultDigest | qualified-lifecycle:sha256: followed by SHA256 of profile, one NUL byte and canonical complete response excluding this field |

Only startup and prompt additionally require context, with the unchanged corvint-dogfood-prompt/0
packet; a compact session start over a dirty worktree adds the `AHI-003` compaction block under its `compaction` key, compiled from the same index with at most half the 8000-byte budget. Counts retain the local context numeric JSON encoding; this integrity receipt is not a WP3
signed execution attestation. All new top-level and authority-bearing nested objects are closed and
complete. A digest is an integrity binding, not a signature or event witness. The renderer uses only
its trusted compiler's local context packet; repository free text remains inside the existing
BEGIN/END CORVINT REPOSITORY DATA envelope.

Completed support scope is qualified-native-runtime or qualified-shared-runtime. Completed digests
and image SHA256 values are lowercase 64-hex; shared surfaces are exactly codex-desktop and codex-cli
with their evidence SHA256 values. eventSurface is always unattributed. Candidate scope is
candidate-native-runtime or candidate-shared-runtime; its digest/evidenceSHA256 are empty strings and
qualifiedSurfaces is empty. Candidate App/engine/adapter hashes, OS and architecture are actual
campaign pins, never placeholder qualification. Runtime verifies the actual protected release and
kernel process/image identities before/after each event. No caller host version is trusted or invented;
the independently reviewed qualification matrix binds human version names to these exact images.

A candidate context packet reserves 512 extra native output bytes for the completed qualification
hashes and at most two surface entries. The same packet is checked against the actual FULL-shaped
serializer in deterministic tests, including escaped data: the maximal one/two-surface expansions are 213/329 native bytes. This reservation is only byte accounting;
it does not create a qualification. Candidate and FULL admission costs are different: candidate adds
campaign checks, while completed qualification adds its validation, digest and surface projection.
Candidate target equality uses the same before/after repository probes as completed qualification,
checked before local evaluation/compiler and again before return; it adds no Git invocation or
candidate-only prewarming. The protected scope alone supplies expectedTarget internally; it is not
an event/request field. Complete normalized Git argv multiplicity is compared in tests, preserving
concurrent independent probe scheduling and random private status directories. Native measurement
must report remaining admission differences explicitly.

The adapter reads bounded native input, verifies its fixed consumer image and replaces itself with
that consumer. Its nonblocking in-memory socket is sized for the bounded escaped request; failure to
fit refuses visibly. It creates no child or prompt file. The consumer owns its 1600-ms deadline and
its existing bounded Git subprocess cleanup. Native host timeout remains an independent outer bound. The new mode reports corvint-invocation-timeout
when its 1600 ms adapter or consumer deadline is exceeded, explicitly identifying a time bound rather
than a diagnosed fault; consumer cancellation retains a distinct cancellation label.

## Failure modes and non-goals

Missing/unqualified/revoked/drifting root, runtime, cwd, configuration, campaign, pointer, publication
or output emits a fixed named FALLBACK without authority or a blocking native decision. A failed
protected check cannot be masked by a successful local workflow. Context failures preserve explicit
refusal; unsupported budgets never truncate an integrity receipt. Candidate non-Stop never claims that
its declared campaign enrollment was actually validated. Default legacy adapters stay unchanged.

This proposal does not add arbitrary native execution, a daemon, cache, signed event provenance,
new host events, root installation, principal creation, native qualification evidence, broader PLE
worker drivers or automatic persistent learning. Closing all unrelated Corvint OCM unknowns is not a
predicate for exact tuple support; selected protected obligations still determine actual OPEN/EMPTY.

### Named codes and reasons

The `qualified-event` command emits the kebab-case codes below (decision 0100). Each row cites the
first emitting site and states only the condition checked there.

| Code | First emitting site | At the cited site |
|---|---|---|
| `continuation-limit` | `cmd/corvint/qualified_lifecycle.go:205` | on Stop, `stopHookActive` is true; the envelope `decision` becomes `release` with this reason |
| `invalid-qualified-lifecycle-arguments` | `cmd/corvint/qualified_lifecycle.go:95` | the arguments are not exactly `--input -`, optionally followed by `--native-output` |
| `invalid-qualified-lifecycle-input` | `cmd/corvint/qualified_lifecycle.go:108` | `parseQualifiedRequest` refused the input (wrong profile, unreadable event, refused event input, or a Stop input without `stopHookActive`) |
| `not-stop-event` | `cmd/corvint/qualified_lifecycle.go:188` | the `frontier.reason` of every envelope before Stop handling, with state `NOT_EVALUATED` and decision `release`; Stop handling replaces the frontier object |
| `protected-frontier-open` | `cmd/corvint/qualified_lifecycle.go:207` | on Stop with `stopHookActive` false, the protected frontier decision is `block` and the local completion decision is not `block`; the envelope `decision` becomes `block` with this reason |
| `qualified-lifecycle-target-drift` | `cmd/corvint/local_completion_event.go:254` | an error from `localEventRead` when the held protected scope supplies an expected target and the first repository probe reports a different commit revision; `qualifiedLifecycle` does not pass it through and reports `qualified-lifecycle-unavailable` |
| `qualified-lifecycle-unavailable` | `cmd/corvint/qualified_lifecycle.go:23` | the text of `errQualifiedLifecycle`; the command reports it when `qualifiedLifecycle` fails and the context is neither past its deadline nor cancelled, which covers a failed or repeated observer call, an empty scope root, a missing result, and a Stop resolution whose root is not current, whose state is neither `OPEN` nor `EMPTY`, or whose host or campaign digest does not match |

## Acceptance and rollback

Source acceptance requires the tests below, existing legacy wire regressions, focused vet, the
canonical repository gates and independent source review. Interruption must leave no descendant.
Actual native support remains unqualified until the accepted AHI/PLE promotion conditions pass for
this exact adapter, release and host scope. Failed and absent evidence remains visible.

Installation is an explicit separately reviewed operator action using immutable release pins. The
source template has no consumer pins and the proposed hook manifest is never installed by a build.
Rollback removes the optional qualified hook mode or revokes its protected admission, then restores
the existing legacy hooks. Reverting this additive source must not rewrite older receipts, local
workflow state or accepted qualification evidence. Any runtime image/OS/scope drift requires renewed
qualification; temporary campaigns never renew themselves.

## Traceability

| Requirement | Implementation | Executable source evidence |
|---|---|---|
| QLF-V0-001 | cmd/corvint/qualified_lifecycle.go | TestQualifiedLifecycleClosedInputAndUnadmittedNative; QualifiedLifecycleHookTests |
| QLF-V0-002 | internal/authoritystore/lifecycle.go | TestLifecycleContextHeldScopeWithoutEnrollment; TestLifecycleContextDriftAndUnqualifiedRefuse |
| QLF-V0-003 | internal/authoritystore/store.go | TestLifecycleActivePointerClosedAndPinned; existing protected view/cwd/currentness tests; actual protected Compute integration remains native evidence |
| QLF-V0-004 | cmd/corvint/qualified_lifecycle.go | TestQualifiedLifecycleStopComposition |
| QLF-V0-005 | cmd/corvint/qualified_lifecycle.go | TestQualifiedLifecycleResultDigestAndClosedRenderer; TestQualifiedLifecycleCandidateNeverFullAndReservesFinalBudget; TestQualifiedLifecycleCompactSessionStartRehydratesDirtyPaths |
| QLF-V0-006 | cmd/corvint/local_completion_event.go | TestQualifiedLifecycleContextBudgetAndReadOnly; TestQualifiedLifecycleMaximumStopAndCandidateReserve |
| QLF-V0-007 | integrations/codex/plugins/corvint/scripts/authority_hook.py | QualifiedLifecycleHookTests; TestDogfoodEventGoPythonWireAndSession; TestQualifiedHooksPrepareExactSidecar; TestQualifiedHooksRejectTemplateAndBundleDrift; TestQualifiedHooksPartialFailureLeavesNoReceipt |
| QLF-V0-008 | internal/authoritystore/campaign.go | TestLifecycleCampaignScopeDoesNotBroadenLegacy; TestCampaignClosedSchemaAndBinding |
| QLF-V0-009 | internal/authoritystore/lifecycle.go | TestLifecycleCandidateContextUsesSameReadAndRejectsDrift; TestQualifiedLifecycleSharedTargetProbesAndGitParity |
| QLF-V0-010 | This proposal and AHI/PLE owning contracts | Actual native event/performance/admission evidence NOT_RUN for this source candidate |

## Prospective direct native CLI profile (DCLI-V0)

The separately reviewed [direct native CLI profile](direct-native-cli-authority-v0.md) adds closed
root/2, campaign/1 and QLF/1 for one actual Codex CLI process. PLE-V0-009..012 and QLF-V0-001..010
retain their currentness, protected computation, attribution, native qualification and separate
operator admission obligations. The direct process pin replaces only the app/engine topology in
that new version; root/1, campaign/0, authority-event/0 and QLF/0 keep their existing wire meanings.
Legacy authority-event refuses root/2. A completed direct qualification cannot inherit Desktop
support or recover through a candidate if invalid. This prospective source profile does not accept
an execution root or weaken decision 0009.

## Prospective protected Pi SDK profile (PPI-V0)

The separately scoped [protected Pi runtime](protected-pi-runtime-v0.md) adds root/3,
campaign/2 and QLF/2 for the closed Pi SDK image. Its immediate-host topology is
`protected-pi-sdk`, and completed qualification requires both `pi-tui` and `pi-rpc`
evidence for that image. The four-event input contract, caller-asserted provenance,
unattributed event surface, protected currentness, non-Stop privacy and independent
admission obligations are unchanged. Earlier profiles cannot consume Pi admission.
PPI source tests and sealed images do not grant FULL or accept an execution root.
