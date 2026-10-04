# Post-merge /1 native delta integration

Date: 2026-10-04. Owning ticket: V1-0542 (#395), still OPEN. Base: origin/main
`cd70ba0ab538a612a13d146934cf95ef356c333d`.

## Correction to the /2 registration entry

`2026-10-04-postmerge-v2-contract-registration.md` says #389 `corvint delta` is "OPEN and absent".
That was true at its base `4b10a021`, where `internal/delta` does not exist. It is now stale:

- PR #525 (`dd485091`) merged `cmd/corvint/delta.go` and `internal/delta` into main.
- #389 closed at 2026-10-04T21:27:01Z.

The earlier entry stays immutable, and this entry supersedes that one line.

## Decision

Wire actual `delta.Compile` into the experimental `/1` delta stage. This is PMR-V1-002, in the new
"Delta slice" section of `postmerge-runtime-v1.md`.

- Options are fixed in code:
  - base and head come from the admitted product binding;
  - the build label is the operator-pinned implementation source commit;
  - there is no previous generation, work-key pattern, provider or checkout.
- The resulting unknowns and full-suite obligations stay in the retained record. No fixture or
  label input selects an option.
- A record whose base, head or tree differs from the admitted binding is refused as
  `native-delta-failed`. It is not observed.
- After an observed delta, follow-up blocks with the new fixed reason `follow-up-not-integrated`.
  Later stages are `prior-stage-blocked`.
- A delta refusal leaves later stages `dependency-delta-blocked`.
- `actual-delta-unavailable` is no longer emitted. `delta-not-integrated` stays reserved.
- Every `/1` report remains BLOCKED with CLI exit 2. No connector Build, ValidatePlan or Record call
  becomes reachable.

The simpler alternative was to pass `delta-not-integrated`, as the first slice anticipated. It was
rejected because the issue asks for the actual delta observation, and the native call adds no
writer, network or credential surface.

## #394 producer decision emitter: not delivered

Every added line of patch `da35448e…` (source `e5d777cb…`, which is unreachable here) is already
present on main. It is a test-acceptance freshness change and contains no `/2` decision emitter.

A conforming PMR-V2-002 emitter needs the following, and none of it exists:

- a verified `processes` member (required by `decision.schema.json`);
- `ProcessAdmissionV2` and `VerifyProcessV2` from `internal/postmergeproof`, which is absent;
- a Linux procfs host for qualification.

Partial emitter code would be speculative, so none was added. The emitter waits on PMR-V2-006.

## Evidence and limits

- `go test ./internal/postmergeworkflow/ ./cmd/corvint-postmerge-workflow/` passed on Darwin with a
  disposable Git fixture.
- The tests cover:
  - observed delta bytes equal to an independent `delta.Compile`;
  - byte-identical repeat runs;
  - a 129-level path refused by delta with dependents blocked.
- These tests show native conformance only. Historical replay, provider-backed delta, host
  qualification and `/2` remain `NOT_PRODUCED`.
