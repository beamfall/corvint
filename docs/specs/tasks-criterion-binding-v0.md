# Tasks criterion binding V0

Owner: Russell Lewis
Date: 2026-10-01
Intent status: proposed
Delivery status: experimental
Authoritative inputs: decision 0397, `docs/specs/cem-criterion-experiments-v0.md`, native V1-0575.

## Agent digest
- Claim: Native Tasks captures and verifies coherent claim-era criterion bindings; Core consumes only the Tasks wire contract.
- Status: proposed technical contract/experimental delivery; owner release intent remains accepted separately.
- Exists: `internal/tasks/criterionbinding`, compiled CLI read verbs and wire-owned projections; bounded independent repair review passed.
- Blocked on: full CEM1.0 live receipt/source join, integration and release qualification.
- Read next: Requirements; Acceptance and rollback.

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

## Acceptance and rollback

Prove canonical capture, tamper/command/policy/snapshot negatives, verify with a missing repository,
read-only filesystem behavior, duplicate-flag refusal and current versus historical identity. Existing
criterion behavior, focused tests/vet, actual compiled nonfixture native Tasks lifecycle and unchanged
import-direction test MUST pass. Rollback removes optional verbs/adapter and preserves prior receipts;
existing Tasks schemas/policy and frozen CEM bytes do not change. TCB-V0-001..006 have experimental
implementation and scoped acceptance evidence below; this proposed technical contract is not thereby
accepted or promoted. No stable release or execution cutover is implied.

Capture includes canonical claim-era ticket bytes whose digest equals the attempt claim record, and compares the ordered criteria with the current ticket at the same acceptance revision. Body-only refinements remain admissible. Attempt phase and lease sequence numbers may not exceed the retained snapshot head. Historical audit provides bounded structural journal coherence; semantic authority and authentication remain NOT_OBSERVED. Earlier proposed captures without claim-era bytes remain unsupported by this verifier rather than being silently upgraded.

## Traceability and retained evidence

| Requirement | Source and focused tests | Observed result / limit |
| --- | --- | --- |
| TCB-V0-001 | `internal/tasks/wire/criterion_binding.go`, `internal/tasks/criterionbinding/verify.go`; unchanged `internal/tasks/boundary_test.go` `TestImportDirection` | Core consumes wire, while native Tasks validates full records; no import whitelist |
| TCB-V0-002 | `internal/tasks/cli/criterion_binding.go`; `TestCriterionCaptureRetainsClaimAcrossNativeBodyEdit`, `TestClaimedAcceptanceAndSequenceBindings` | Claim afterimage and current criteria coherence retained; body-only refinement admitted, future attempt/lease sequences refused |
| TCB-V0-003..004 | `TestNativeCaptureVerification`, CLI `TestCriterionBindingOfflineVerify` | Closed native capture, offline historical verification and producer/verifier identities; no current-authority inference |
| TCB-V0-005..006 | `internal/criterionexperiment/tasks_unix.go`; `TestTasksExecutableIndependentPin`, `TestTasksTransportBoundsAndCancellation`, `TestTasksCancellationRetiresDescendant` | Independently pinned bounded read transport and ordinary descendant retirement; same-user races remain outside authentication |

Independent repair review `/private/tmp/cem10-build/tasks-rereview.json` passed the original
claim-digest and future-sequence counterexamples plus positive native lifecycle tests. The compiled
nonfixture disposable lifecycle in `live-demo-repair1-evidence.json` retained eight scenarios,
three killed controls, native completion, a consistent final audit and historical replay after an
acceptance change. Its qualification admission input was explicitly synthetic feasibility input,
not deployment qualification. Source/binary/result digests remain in that manifest. This proves the
specific Tasks/CEX boundary; it does not transfer historical satisfaction, source Git authority or
completion authority to opaque `cem/1.0-experimental.1` references.
