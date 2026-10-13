# Tasks criterion binding V0

Owner: Russell Lewis
Date: 2026-10-01
Intent status: proposed; TCB-V0-015..019 accepted under the owner's standing auto-accept (2026-10-11) (V1-1283)
Delivery status: experimental
Authoritative inputs: decision 0397, `docs/specs/cem-criterion-experiments-v0.md`, native V1-0575;
for TCB-V0-007..019, decision 0525 §1 step 8, `docs/specs/cem-stable-v1.md` and native V1-1283.

## Agent digest
- Claim: Native Tasks captures and verifies coherent claim-era criterion bindings; Core consumes only the Tasks wire contract.
- Status: proposed; TCB-V0-015..019 accepted under the owner's standing auto-accept (2026-10-11) (V1-1283)/experimental delivery; owner release intent remains accepted separately.
- Exists: `internal/tasks/criterionbinding`, compiled CLI read verbs and wire-owned projections; bounded independent repair review passed. Experimental sealed cem/1.0 verdict gate: library `internal/taskman/cem_verdict_gate.go` with a portable cross-check, `corvint cem gate-verdict`, Tasks argv placeholders and a policy entry (TCB-V0-007..019).
- Blocked on: full CEM1.0 live receipt/source join, integration and release qualification; installing the gate's two executables and registering its policy entry in the live store.
- Read next: Requirements; Acceptance and rollback; Sealed cem/1.0 verdict gate.

## Requirements

- `TCB-V0-001`: Core MUST import only Tasks wire. Full ticket/attempt/policy validation stays Tasks-side;
  existing TestImportDirection remains unchanged. No copied partial decoder or whitelist may substitute.
- `TCB-V0-002`: `criterion-binding capture --ticket ID --attempt ID` MUST be a bounded read with no
  repository/trace/Tasks mutation. Exact canonical ticket, queue, attempt and policy bytes MUST share
  audited head sequence/receipt, intent tree and primary identity, with no redo/barriers.
- `TCB-V0-003`: `criterion-binding verify` MUST consume a closed bounded capture on stdin with no
  repository resolution or mutation. Embedded native command identities, canonical records, coherent
  snapshots, acceptance/attempt/runtime/lease and raw/content policy digests MUST be revalidated.
- `TCB-V0-004`: Verify outer current snapshot MUST be null; carried snapshot MUST be explicitly
  HISTORICAL. Capture producer and current verifier identities MUST be distinct. Version/build labels
  cannot substitute for executable identity. All input bytes and results remain digest-bound.
- `TCB-V0-005`: The CEM companion MUST use only independently configured, pinned Tasks executable
  capture/verify calls. Plans bind executable digest and reported identities, never supplied executable
  path or argv. Historical verification runs only that trusted read verifier, never an experiment/test
  or plan-supplied command. Unsupported installed Tasks versions refuse visibly.
- `TCB-V0-006`: Unknown/repeated arguments, malformed/trailing/oversize payloads, nonregular executable,
  contradictory captures and missing authority MUST refuse. Calls cap stdout at 4 MiB, stderr at
  64 KiB and 30 seconds; cancellation/overflow retire the owned verifier process group. Admission
  and post-call checks bind the same executable. Same-user hostile mutation is not authenticated.
- `TCB-V0-007`: The sealed-verdict gate (`taskman-cem-verdict-gate/0`) MUST take only native facts:
  an absolute clean repository root, the attempt's full native base and candidate commit IDs (equal-length
  lowercase 40 or 64 hex), the claim-era `effects.touchPaths` keys and one predicate. Malformed input
  refuses `CEM_VERDICT_INPUT_INVALID` with no pinned inputs; an absent commit refuses it pinned.
- `TCB-V0-008`: The gate MUST read the sealed map from Git objects at the candidate commit, never from
  the worktree, an attachment or a Tasks store: exactly one `.corvint/changes/<target>.cem.json` added
  between base and candidate. None refuses `CEM_VERDICT_MAP_MISSING`; several refuse
  `CEM_VERDICT_MAP_AMBIGUOUS` and pin every sealed name.
- `TCB-V0-009`: Producer-written fields MUST agree with native recomputation or refuse
  `CEM_VERDICT_PRODUCER_MISMATCH`: the sealed name's target is a commit; the candidate tree is that
  target's tree with only `.corvint/change.cem.json` renamed byte for byte to the sealed path; the
  map's `baseRevision` equals the attempt base.
- `TCB-V0-010`: The gate MUST recompute the verdict in process through the stable consumer with the
  repository and artifact root at the root, the attempt base expected and the sealed target, and refuse
  `CEM_VERDICT_UNVERIFIABLE` unless the outcome is ACCEPT, the sidecar is EXACT and a citation block
  exists. An unreadable repository, Git read failures and hosts other than Darwin and Linux refuse the
  same code. Declared artifacts are read under the root and must match digests the sealed map pins.
- `TCB-V0-011`: Predicates are closed: `touched-anchored` governs hunks under a touch key (a `dir/` key
  is a prefix, any other key an exact path) and `all-anchored` governs every hunk. Tiers come only from
  the recomputed citation; `anchored`, `mechanical-proved` and `held` satisfy and nothing raises a tier.
  No governed hunk, or one below anchored, refuses `CEM_VERDICT_PREDICATE_FAILED`.
- `TCB-V0-012`: Every outcome MUST emit one canonical, deterministic, host-path-free document pinning
  the inputs, candidate tree, sealed names, map path/target/blob/`mapSha256`, consumer
  outcome/stage/code/issues/sidecar/`verdictSha256`, the recomputed citation and per-hunk governed rows.
  Only a fully satisfied predicate GRANTs.
- `TCB-V0-013`: Re-verification MUST recompute from native inputs and GRANT only when the recomputed
  document equals the pinned bytes exactly; other pinned bytes refuse `CEM_VERDICT_PRODUCER_MISMATCH`.
  Any consumer can recompute from the document's own inputs with no Tasks store present.
- `TCB-V0-014`: The gate MUST NOT mutate the repository, refs, objects, traces or a Tasks store. A GRANT
  is gate evidence only: it never completes, satisfies or edits a ticket, attempt, policy or tier, and
  native completion authority and TCB-V0-001..006 stay unchanged.
- `TCB-V0-015`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1283) A Tasks COMMAND
  gate MUST receive attempt facts only through three closed argv placeholders, replaced just before the
  gate runs when an argv entry equals one exactly: `{attempt.baseCommit}` is the attempt base,
  `{attempt.candidateCommit}` the worktree HEAD commit whose tree was just proven to be the candidate,
  and `{claim.touchPaths}` the claim-era ticket's `effects.touchPaths` as one canonical JSON string
  array. A ticket record whose digest differs from the claim's refuses `STALE_TICKET`; a
  multi-repository candidate refuses `UNSUPPORTED`. A substituted argv joins the result's inputs
  digest. Other entries, preflight and batch runs keep the declared argv.
- `TCB-V0-016`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1283)
  `corvint [--root PATH] cem gate-verdict --base OID --candidate OID --predicate PREDICATE
  --touch-paths JSON --portable-consumer PATH` MUST evaluate the verdict gate from a closed argv: each
  of the five flags exactly once with a value not starting `--`, `--touch-paths` a JSON string array,
  and nothing before `cem` except `--root` with an absolute clean path. Without `--root` the root is
  the working directory with symbolic links resolved. It prints the gate document and exits 0 on GRANT
  and 1 on REFUSE; an unreadable argv prints `cem-gate-verdict-invalid-arguments` to stderr, no
  stdout, and exits 2.
- `TCB-V0-017`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1283) The gate MUST
  run the independent portable consumer at the absolute operator-supplied path (an `interop/cem01-go`
  build) in the gate's process group on the same sealed map bytes through `verify-stable-canonical`,
  and pin its route, outcome, claimed `verdictSha256` and citation digest as `portable`. The portable
  outcome, claimed digest, recomputed digest and canonical citation block MUST each equal the native
  ones; any difference refuses `CEM_VERDICT_VERIFIER_DISAGREEMENT` (`verifier-disagreement`). A
  consumer that cannot run, emits no readable result, ends on a signal, or exits other than 0 on
  ACCEPT and 1 or 2 otherwise refuses `CEM_VERDICT_UNVERIFIABLE` (`portable-consumer-failed`).
- `TCB-V0-018`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1283) The document MUST
  carry `paths`: each governed path once, in byte order, with the minimum recomputed citation tier of
  its governed hunks (`held` over `mechanical-proved` over `anchored` over `cited` over `unknown`) and
  whether it satisfies. A tier a producer writes into the sealed map is never admitted: such a map is
  not accepted by the stable consumers and refuses before any predicate runs.
- `TCB-V0-019`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1283) The gate MUST be
  registered as the `cem-verdict` policy gate below: kind `COMMAND`, cwd `WORKTREE`, env `PATH`, a
  1800-second timeout, expected exit 0, `output` evidence, not reusable and `required: false`, so only a
  ticket that names it in `requiredGates` must pass it. A Tasks PASS records the gate document's digest
  as its output evidence; a FAIL records exit 1 and the refusal document, or exit 2.

## Acceptance and rollback

Prove canonical capture, tamper/command/policy/snapshot negatives, verify with a missing repository,
read-only filesystem behavior, duplicate-flag refusal and current versus historical identity. Existing
criterion behavior, focused tests/vet, actual compiled nonfixture native Tasks lifecycle and unchanged
import-direction test MUST pass. Rollback removes optional verbs/adapter and preserves prior receipts;
existing Tasks schemas/policy and frozen CEM bytes do not change. TCB-V0-001..019 have experimental
implementation and scoped acceptance evidence below. TCB-V0-015..019 are accepted under the owner's
standing auto-accept; the rest of this proposed technical contract is not thereby accepted or
promoted. No stable release or execution cutover is implied.

Capture includes canonical claim-era ticket bytes whose digest equals the attempt claim record, and compares the ordered criteria with the current ticket at the same acceptance revision. Body-only refinements remain admissible. Attempt phase and lease sequence numbers may not exceed the retained snapshot head. Historical audit provides bounded structural journal coherence; semantic authority and authentication remain NOT_OBSERVED. Earlier proposed captures without claim-era bytes remain unsupported by this verifier rather than being silently upgraded.

## Traceability and retained evidence

| Requirement | Source and focused tests | Observed result / limit |
| --- | --- | --- |
| TCB-V0-001 | `internal/tasks/wire/criterion_binding.go`, `internal/tasks/criterionbinding/verify.go`; unchanged `internal/tasks/boundary_test.go` `TestImportDirection` | Core consumes wire, while native Tasks validates full records; no import whitelist |
| TCB-V0-002 | `internal/tasks/cli/criterion_binding.go`; `TestCriterionCaptureRetainsClaimAcrossNativeBodyEdit`, `TestClaimedAcceptanceAndSequenceBindings` | Claim afterimage and current criteria coherence retained; body-only refinement admitted, future attempt/lease sequences refused |
| TCB-V0-003..004 | `TestNativeCaptureVerification`, CLI `TestCriterionBindingOfflineVerify` | Closed native capture, offline historical verification and producer/verifier identities; no current-authority inference |
| TCB-V0-005..006 | `internal/criterionexperiment/tasks_unix.go`; `TestTasksExecutableIndependentPin`, `TestTasksTransportBoundsAndCancellation`, `TestTasksCancellationRetiresDescendant` | Independently pinned bounded read transport and ordinary descendant retirement; same-user races remain outside authentication |
| TCB-V0-007 | `internal/taskman/cem_verdict_gate.go`; `TestTCBV0007GateInputsAreNativeAndClosed` | Malformed roots, IDs, predicates and touch keys refuse unpinned; touch keys pin sorted and unique |
| TCB-V0-008 | `TestTCBV0008SealedMapIsReadFromCandidateGit` | Change and bind candidates refuse missing, a worktree-only copy is ignored, two sealed maps refuse ambiguous |
| TCB-V0-009 | `TestTCBV0009ProducerFieldsDisagreeingRefuse` | Base mismatch, a post-seal edit, edited sealed bytes and a non-commit target each refuse before the consumer |
| TCB-V0-010 | `TestTCBV0010UnverifiableMapRefuses` | A non-repository root refuses as a read failure; a wrong producer `patchSha256` sealed byte for byte refuses with the consumer's non-ACCEPT verdict pinned |
| TCB-V0-011 | `TestTCBV0011PredicateIsClosedAndNeverRaisesATier` | A cited hunk stays cited and fails; governed tiers equal the recomputed citation; exact and prefix keys |
| TCB-V0-012 | `TestTCBV0012GrantPinsVerdictAndCitation` | GRANT pins the same `verdictSha256` and citation as an independent stable consumer run; deterministic bytes |
| TCB-V0-013 | `TestTCBV0013TamperedEvidenceRefusesOnReverify` | Store-less re-verification grants; edited digest, raised tier, dropped citation and other inputs refuse |
| TCB-V0-014 | `TestTCBV0014GateIsReadOnly` | Status, refs, HEAD and object counts are unchanged across GRANT and REFUSE evaluations |
| TCB-V0-015 | `internal/tasks/store/lease_gate.go`; `TestTCBV0015_GateRunSubstitutesAttemptFacts` | Base, candidate HEAD and claim-era touch paths replace only whole entries; substituted argv binds the inputs digest; an edited ticket refuses `STALE_TICKET` |
| TCB-V0-016 | `cmd/corvint/cem_gate_verdict.go`; `TestTCBV0016GateVerdictArgvIsClosed`, `TestTCBV0016GateVerdictRunsBothConsumers` | Twelve unreadable argv shapes exit 2 with one stderr code; on a sealed fixture the command, run with a built `interop/cem01-go`, prints the library's bytes, grants, and refuses a patched portable result |
| TCB-V0-017 | `TestTCBV0017PortableConsumerMustRecomputeTheSameVerdict`, `TestTCBV0017PortableConsumerSharesTheGateProcessGroup` | Agreement pins equal digests; a changed claim, raised tier or outcome disagrees; a missing, unreadable, other-profile or digest-less result, or an ACCEPT that exits 1 or 2 or is killed, is unverifiable; the consumer runs in the gate's process group |
| TCB-V0-018 | `TestTCBV0018PathsCarryTheMinimumRecomputedTier`, `TestTCBV0018ProducerEditedTierIsNeverAdmitted` | A path takes its lowest hunk tier; a sealed map with producer-written `held` tiers refuses `stable-not-accepted` with no predicate rows |
| TCB-V0-019 | `TestTCBV0019_CEMVerdictPolicyEntryRunsAsDeclared` | The entry registers in a fixture policy; stand-in commands with its argv record PASSED and FAILED with the substituted argv as output evidence |

Independent repair review `/private/tmp/cem10-build/tasks-rereview.json` passed the original
claim-digest and future-sequence counterexamples plus positive native lifecycle tests. The compiled
nonfixture disposable lifecycle in `live-demo-repair1-evidence.json` retained eight scenarios,
three killed controls, native completion, a consistent final audit and historical replay after an
acceptance change. Its qualification admission input was explicitly synthetic feasibility input,
not deployment qualification. Source/binary/result digests remain in that manifest. This proves the
specific Tasks/CEX boundary; it does not transfer historical satisfaction, source Git authority or
completion authority to opaque `cem/1.0-experimental.1` references.

## Sealed cem/1.0 verdict gate

Decision 0525 §1 step 8 asks a Tasks gate to consume the sealed cem/1.0 verdict. The gate reads the
committed sealed map from Git at the attempt's candidate commit and recomputes the verdict; an
attached-evidence receipt was rejected because it carries a producer-written digest bound to the ticket
rather than the candidate. The decision and its rejected alternative are recorded in
`docs/build-log/2026-10-13-v1-1283-tasks-gate-sealed-verdict.md`.

Non-goals: Tasks never raises a tier, never trusts a producer-written field, never reads the map from
the worktree, an attachment or the store, and never completes a ticket from a GRANT. Artifacts a map
declares are the one worktree input: the stable consumer reads them under the root and refuses unless
their bytes match the digests the sealed map pins, so a re-verifier needs those artifact bytes. The `held`
count predicate waits for V1-1266. Verifying a chain of stacked seals is follow-up work. The gate never
takes the portable consumer's path from the map or the store, and a portable result never raises
or replaces a native tier.

Failure modes. Every refusal pins one closed code and one closed reason:

| Code | Reason | Cause |
| --- | --- | --- |
| `CEM_VERDICT_INPUT_INVALID` | `invalid-input` | Malformed root, commit ID, predicate, touch key or portable consumer path |
| `CEM_VERDICT_INPUT_INVALID` | `commit-unavailable` | The base or candidate commit is not in the repository |
| `CEM_VERDICT_MAP_MISSING` | none | No sealed map is added between base and candidate |
| `CEM_VERDICT_MAP_AMBIGUOUS` | none | More than one sealed map is added |
| `CEM_VERDICT_PRODUCER_MISMATCH` | `sealed-target-unavailable` | The sealed name's target is not a commit |
| `CEM_VERDICT_PRODUCER_MISMATCH` | `sealed-tree-mismatch` | The candidate is not the target plus only the renamed sidecar |
| `CEM_VERDICT_PRODUCER_MISMATCH` | `sealed-base-mismatch` | The map's `baseRevision` is not the attempt base |
| `CEM_VERDICT_PRODUCER_MISMATCH` | none | Re-verified pinned bytes differ from the recomputation |
| `CEM_VERDICT_UNVERIFIABLE` | `unsupported-platform` | The host is not Darwin or Linux |
| `CEM_VERDICT_UNVERIFIABLE` | `git-read-failed` | The root is no readable repository or a bounded Git read failed |
| `CEM_VERDICT_UNVERIFIABLE` | `portable-consumer-failed` | The portable consumer did not run, exceeded its bound, emitted no readable `cem-stable-verification/1` result, ended on a signal or exited with a status its outcome does not explain |
| `CEM_VERDICT_UNVERIFIABLE` | `stable-not-accepted` | The stable consumer did not ACCEPT with an EXACT sidecar and citation |
| `CEM_VERDICT_VERIFIER_DISAGREEMENT` | `verifier-disagreement` | The portable outcome, claimed or recomputed `verdictSha256`, or citation block differs from the native one |
| `CEM_VERDICT_PREDICATE_FAILED` | `no-governed-hunk` | No hunk lies under the touch keys |
| `CEM_VERDICT_PREDICATE_FAILED` | `tier-below-anchored` | A governed hunk is `cited` or `unknown` |

The command adds one closed stderr code, `cem-gate-verdict-invalid-arguments` (exit 2), for an argv it
cannot read. Every other outcome is a gate document.

Tasks gate wiring. TCB-V0-015..019 let a native Tasks COMMAND gate run this library. The decisions, the rejected
alternative (a gate that reads the attempt from a pinned Tasks read) and the rehearsals are recorded in
`docs/build-log/2026-10-13-v1-1283-gate-wiring.md`. The runner substitutes the attempt facts, so the
command takes them as argv. The working directory is the attempt worktree, so the command needs no
`--root`. The registration, with this example's two executable paths replaced by the installed
absolute ones, is:

```json
{"gateId":"cem-verdict","kind":"COMMAND","argv":["/usr/local/bin/corvint","cem","gate-verdict","--base","{attempt.baseCommit}","--candidate","{attempt.candidateCommit}","--predicate","touched-anchored","--touch-paths","{claim.touchPaths}","--portable-consumer","/usr/local/libexec/cem01-go"],"cwd":"WORKTREE","env":["PATH"],"timeoutSeconds":"1800","expected":{"exitCode":"0","reducer":null},"evidence":["output"],"inputs":[],"sharedResource":null,"reusable":false,"required":false}
```

Both executables must be built from a commit that contains TCB-V0-015..019. `PATH` must be in the
policy's `environment.allowedEnvKeys`. The portable consumer runs with `PATH=/usr/bin:/bin` whatever
the gate's environment, and needs `git` there. The gate must run on the seal commit itself
(`{attempt.candidateCommit}` is the worktree HEAD), because a later commit fails `sealed-tree-mismatch`.
Preflight and batch runs keep placeholders literal, and the command then refuses
`CEM_VERDICT_INPUT_INVALID`.

Acceptance evidence: the TCB-V0-007..019 tests above, run on a dogfood-shaped fixture (base, change,
bind and seal commits produced by the CEM workflow), plus the unchanged Tasks import-direction test.
The TCB-V0-016 test runs a freshly built `interop/cem01-go` on darwin, the only host where that
consumer's process containment is qualified; elsewhere it skips `NOT_RUN`. The `internal/taskman`
tests use their own test binary as a stand-in portable consumer that answers with the in-process
stable verifier, because those tests may not read the interop module; they run on darwin and linux.

Rollback: delete `cmd/corvint/cem_gate_verdict.go` and its test, the `cem gate-verdict` dispatch and
help, the placeholder substitution in `internal/tasks/store/lease_gate.go` and its tests, and
`internal/taskman/cem_verdict_gate.go` with its tests, then remove any `cem-verdict` policy entry from
a live store. No Tasks schema, wire contract or CEM byte changed. A gate result recorded with a
substituted argv keeps its inputs digest; a declared argv's digest is unchanged.
