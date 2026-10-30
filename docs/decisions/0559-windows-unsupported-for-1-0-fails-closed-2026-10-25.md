# Decision 0559 — windows/amd64 is UNSUPPORTED for 1.0 and fails closed

Date: 2026-10-25. Status: accepted. Authority: repository owner, in chat on 2026-10-25: windows/amd64
stays UNSUPPORTED for 1.0 and must fail closed; every subsystem that cannot run on native Windows
refuses with a typed, documented reason instead of a partial or wrong result; native-Windows
qualification (V1-2441) is post-1.0; the CI windows-vet step stays. This answers V1-2485. It keeps
decision 0061 (Windows is a deferred target) and `PRS-V1-004` ("Windows MUST stay UNSUPPORTED") and
adds the fail-closed rule they did not state.

## Context

The release spec already labels windows/amd64 UNSUPPORTED, yet V1-2441 and V1-2442 were filed on
the premise that 1.0 must work on Windows. The V1-2441 native run (origin/main `0641ad2e`, then
`368b80d4`) failed 173, then 162, of 362 packages. Most first failures are deliberate refusals
(procgroup, gitstatus no-follow, Tasks safe open, CEM publish and lock, contextindex shards,
analyzer execution), which `internal/testplatform.PlatformRefusals` now lists. Others are not typed:
the docs corpus reported `corpus-input-unavailable` for every read (V1-2484), appmap reports
`evaluated revision is not a commit` where procgroup refused (V1-2487), and two packages hang
(V1-2492). An operator on Windows could not tell an unsupported platform from a broken input.

## Decision

1. windows/amd64 is UNSUPPORTED for 1.0 (`PRS-V1-004`, unchanged). No 1.0 release note, help text or
   product doc may describe any Windows behavior as supported or experimental.
2. Fail closed (`PRS-V1-021`). A subsystem whose qualified implementation is not built for the
   running platform refuses before it opens, reads or writes anything, with a stable reason that
   names the platform limit. It never returns a partial, guessed or misattributed result, and never
   hangs. Each such refusal is listed in `internal/testplatform.PlatformRefusals` with its source,
   and a test on that platform skips only on that exact refusal (V1-2486 helper).
3. Docs corpus reads (`PRS-V1-022`, V1-2484). Off darwin and linux every docs corpus input read
   refuses up front with `corpus-platform-unsupported` and the message `docs corpus reads are
   unsupported on this platform`. The receipt reader refuses every open with the sentinel
   `testvaliditydoc.ErrPlatformUnsupported` (`receipt reads are unsupported on this platform`),
   which retained-evidence discovery reports as its location reason.
4. Qualification of native Windows is post-1.0: V1-2441 and the test-portability and Windows-host
   rerun work from it move to milestone `post-1-0`. A Windows defect that leaves a 1.0 surface
   returning an untyped, misattributed or unbounded result stays at milestone v1-0 under rule 2.
5. The CI windows-vet step and `make windows-vet` stay; they type-check the Windows build and the
   Windows-tagged refusal tests. They do not qualify the platform.

### Disposition of the V1-2441 follow-ups

The orchestrator applies these in the task store; this lane writes no ticket state.

| Ticket | Disposition | Reason |
|---|---|---|
| V1-2441 | post-1-0 | Native Windows qualification (owner). |
| V1-2442 | unchanged (v1-0) | WSL2 is a Linux benchmark host, not Windows support. |
| V1-2484 | v1-0, criterion 1 superseded | A native Windows corpus read now refuses with `corpus-platform-unsupported` (rule 3) instead of exiting 0. |
| V1-2486 | post-1-0 | Test-skip wiring and the Windows-host before/after rerun are qualification. |
| V1-2487 | v1-0 | A procgroup refusal reported as `not a commit` violates rule 2. |
| V1-2488 | post-1-0 | Remaining Windows path work and the host rerun are qualification. |
| V1-2489 | post-1-0 | Lease contention on Windows refuses with an error; only qualification needs it mapped. |
| V1-2490 | post-1-0 | Test-helper portability. |
| V1-2491 | post-1-0 | Test POSIX assumptions. |
| V1-2492 | v1-0 | A hang is not a refusal (rule 2): refuse within a bound or up front. |
| V1-2522 | post-1-0 | The lock now refuses up front with a listed refusal; the host rerun is qualification. |
| V1-2524 | post-1-0 | Windows path separators in two providers; qualification. |

## Consequences

The release spec's platform table and Agent digest state the rule. Windows-only defects are no
longer 1.0 blockers unless they break rule 2. New Windows-host runs are optional evidence, labelled
native Windows and never compared with darwin or linux numbers.

## Rollback

Supporting Windows for 1.0 needs a new owner decision that supersedes this one, re-targets the
post-1-0 tickets above and replaces each listed refusal with a qualified implementation. The
`corpus-platform-unsupported` refusal is removed by deleting `platformRefusal` in
`internal/doccorpus/encoding.go` and the sentinel in `internal/testvaliditydoc`; darwin and linux
behavior does not depend on either.
