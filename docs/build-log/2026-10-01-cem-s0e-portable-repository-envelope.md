# CEM stable S0E: public repository-envelope packet and portable verifier

Status: experimental proposal. Portable implementation only; native integration,
stable promotion, external adoption and Linux lifecycle qualification remain open.

## What landed

- `protocol/cem-1.0/stable/repository-envelope-packet/`: the public S0E packet
  (revision "repair 6", manifest SHA256
  `21655c5d73a4b9b9af9f91e6dacbf702a8d2f95cdc9b40368a4cc240dedbd535`): proposed
  rules, fixture packs, 56 full-result cases, envelope cells and ledger boundaries.
- `interop/cem01-go`: an independently authored portable `verify-stable`
  repository path (`stable_repository.go`, `stable_process_darwin.go`,
  `stable_process_other.go`) written from the public packet only. The author had
  no access to native source or native plans.

## Decisions (root, overridable by the owner)

- The wire contract is unchanged: every evidence is cited by a hunk basis and a
  basis holds at most 32 items. The earlier ledger-boundary fixture could not
  satisfy that (2 hunks cannot cite 336 evidence), so the fixture was rebuilt with
  a second pack (`FIXTURES-ledger-boundary.pack.json`) instead of relaxing the
  contract.
- The canonical ledger is an attempted-call trace that ends at the reservation
  refused before spawn. Drift is processed and emitted in bytewise ascending
  evidence-ID order. The fixed prefix is the enumerated 15 calls; the ceiling is
  1024 calls. 336 evidence yields 15 + 3·336 = 1023 accepted calls; 337 is refused
  at reservation 1025 as `unsupported-resource-limit`, exit 2.
- After the leader is reaped, group emptiness is polled with signal 0 only, until
  `ESRCH` or a fixed 2-second deadline. A deadline or probe error is a HOLD that
  surfaces as `repository/unsupported-process-containment`, exit 2, with no retry.
  A HOLD in a short-lived CLI leaves the group to the operating system at process
  exit; that is not observed cleanup.
- Canonical proof blobs use the separate 128 MiB proof batch bound and are not
  charged to the normal blob ledger.

## Evidence

- Public packet: independent review of repair 5 rejected it (ledger row after
  refusal, ambiguous drift order, stale prose); repair 6 was approved with no
  findings. Not checked by that review: runtime behaviour and a full JSON-Schema
  validator pass.
- Portable implementation: the first independent review rejected it (proof blobs
  bounded and charged as normal blobs; descriptor leak on pipe/socketpair setup
  failure; packet tests skipped when no packet path was set). All three were
  repaired with tests, and packet tests now default to the in-tree packet and fail
  when it is missing or does not match its manifest. A confirming independent
  review approved the repaired implementation with no new findings; it did not
  synthesize a proof blob above 64 MiB (code inspection and the normal-ledger
  regression test only) and ran no non-Darwin tests.
- Darwin arm64, Go 1.27.1: `go test -count=1 ./...` in `interop/cem01-go` passes;
  54 of 56 full-result cases run and pass, linked control 1, ledgers 4, envelope
  cells 30; lifecycle tests pass at `-count=20`; `go vet` passes natively and for
  `GOOS=linux` and `GOOS=windows`.

## Not produced / not run

- Full-result cases 21 and 22 require Go 1.24.13 and are `NOT_RUN`.
- Linux lifecycle, cross-device topology and escaped-session behaviour are
  `NOT_RUN`; cross-vet is compile evidence only.
- Native verifier agreement with this packet is not part of this change.
- No external consumer exists; external acceptance criteria are not waived.

## Rollback

Remove the packet directory and the three portable source files with their tests
and revert the `main.go` dispatch lines. No stored state or wire format depends on
them.
