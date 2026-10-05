# PMR-V2-006 host collector review fixes (#395, V1-0542)

An independent Codex review of the collector slice (`07b783bc..7d2918ae`) returned CHANGES_REQUIRED
with four findings and two hypotheses. Each was checked against the source before it was changed.

Findings and fixes:

- P1-1, reused parent PID. Confirmed: `AwaitChild` resolved the parent only through its cached PID.
  It now refuses a parent launch the collector already reaped and a retirement-phase capture
  (`process-collection-refused`). It also requires the parent to still hold its captured birth (PID,
  stat start time, not Z or X) before discovery and again after the child capture, and both child
  stat brackets must name that parent (`process-collection-failed`). A PID cannot be reused while its
  holder exists, so an unchanged birth after the capture proves continuity. Test:
  `TestProcessCollectorV2AwaitChildRefusesReusedParent`, which includes a controlled reuse case with
  a substituted start time.
- P1-2, descendants survive a failed launch. Confirmed: only the leader was killed and reaped. Every
  failure after a process starts now goes through an owned cleanup boundary. The boundary stops the
  leader and walks descendants through PPID links, reading a member's children only after that
  member shows a stopped or dead state. A stopped process cannot fork, and its exited children stay
  unreaped, so their PIDs are stable. It then kills all matching births and waits, within the 10s
  settle bound, until none is still running. Only after that is the leader reaped. Test:
  `TestProcessCollectorV2FailedLaunchRetiresDescendants`, where retention fails after the child has
  forked.
- P2-3, uncancellable input. Confirmed: the stdin write and both observer reads ignored cancellation.
  Delivery now closes the pipe when the context is done and abandons the launch through the cleanup
  boundary. The observer reads its input against a context that the launcher derives from SIGTERM
  and interrupt, and refuses `process-observer-refused` when cancelled. Tests:
  `TestProcessCollectorV2CancelledDeliveryRetires`, `TestProcessObserverV2CancelledInput`, and
  `TestInternalProcessObserverStopsOnSIGTERM`, which runs the real launcher main.
- P2-4, proof after a failed sweep. Confirmed: `sealed` was set before the sweep completed. The
  collector now retains the first failure, tracks sweep success separately, and refuses `Proof` with
  `process-collection-failed` after any failed step. Test: `TestHostCollectorV2FailedSweepEmitsNoProof`.

Hypotheses:

- (a) The declared environment was not checked against the launched process. Confirmed in the Linux
  container: dash exports `PWD` to its children, so a child's real environment differed from its
  declared preimage. `Start` and `AwaitChild` now compare `/proc/PID/environ`, sorted, with the
  declared environment. The end-to-end test root uses `env -i PATH="$PATH"` for its child. Retained
  limit: the check is a collector-side guard, not retained proof evidence. The verifier trusts the
  invocation preimage, so producer wiring must establish that the preimages describe the launch.
- (b) Observer routing from argv. No exploit path was found. The observer mode is routed only from
  the first argument, and the collector copies its observer command so a caller cannot change it
  later. Tests: `TestInternalProcessObserverRoutingBoundary` and
  `TestProcessCollectorV2OwnsObserverCommand`.

Owner decisions 2026-10-05, recorded in the spec Authority section:

- The pinned observer is accepted as a mode of the host launcher binary. This closes that open
  decision.
- The qualification campaign is authorized in local disposable Linux containers, prerequisites
  first. Admission is a separate later step on the actual evidence. The campaign was not run here.

Evidence:

- darwin: `go test` passes for `internal/postmergehost`, `internal/postmergeproof/...` and
  `cmd/corvint-postmerge-host-launcher`.
- Linux arm64 container (go1.27.1): the new and related tests pass under `-race -v`, and the three
  packages pass in a full run without race.
- `go vet` is clean for darwin, linux/arm64 and linux/amd64.
- NOT_RUN: Linux amd64 runtime.

Limits retained:

- A descendant that double-forked and was reparented before the cleanup walk is outside the cleanup
  boundary, as it is outside the final sweep.
- The environ check is not proof evidence.

Rollback: revert this change. Nothing consumes the collector yet.
