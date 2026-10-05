# Decision 0434: the version tuple moves to 1.0.0-rc.2

Date: 2026-10-05. Status: accepted (owner answers 2026-10-05: "as recommended. get to rc2" and
"you have my explicit approval"). Tickets: V1-0019, V1-0020, V1-0834. Follows decision 0419.

## Context

`1.0.0-rc.1` is published (decision 0425) and every `HEAD` after its tag fails the pre-promotion
tag row until the tuple moves (decision 0419). Decision 0425 names `1.0.0-rc.2` as the next
candidate, decision 0432 selects urfave/cli for its untouched-repository run, and decision 0433
selects no signing for `1.0.0-rc.2` and `1.0.0`. The pinned toolchain `go1.27.1` is still the
newest Go release on 2026-10-05, so the toolchain review moves nothing.

## Decision

1. The `PUB-V0-001` version tuple moves from `1.0.0-rc.1` to `1.0.0-rc.2`: `VERSION`, the
   `cmd/corvint` version constant, the release-artifact manifest, and the VS Code extension's exact
   executable check with its conformance cases. Its tag `v1.0.0-rc.2` does not exist, so the tag
   row is `NOT_RUN` until the release-notes commit is tagged.
2. The stable readiness record fixes `policy/signing` for `1.0.0-rc.2` and `1.0.0` as `NOT_RUN`,
   decision 0433, "No signing: SHA256SUMS only; publisher identity NOT_VERIFIED" (`SRR-V1-009`).
   `1.0.0-rc.1` keeps decision 0420.
3. The N-1 compatibility baseline is the published `v1.0.0-rc.1` (`CCF-V1-007`). Evidence measured
   against rc.1, including host lifecycle results and the V1-0019 run-001 failure, keeps that
   identity and is not relabelled.
4. Retained receipts that bind `1.0.0-rc.1` by value (`cmd/corvint-behavior-falsify`, readiness and
   update fixtures, historical results) are not edited.
5. Nothing is tagged or published by this decision. The candidate is the merge commit of this
   change on `main`, gated by `docs/RELEASE-RUNBOOK.md` steps 1 to 9; tagging and publication
   follow only if every gate passes.

## Rollback

Revert this change. No tag, archive or store promotion depends on it until `v1.0.0-rc.2` is
tagged; after that, a defect is corrected by a later release, never by moving the tag.
