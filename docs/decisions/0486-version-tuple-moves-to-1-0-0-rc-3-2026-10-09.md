# Decision 0486: the version tuple moves to 1.0.0-rc.3

Date: 2026-10-09. Status: accepted (owner request in chat 2026-10-09: "bump the version label off
rc.2"; the orchestrator chose `1.0.0-rc.3` following the decision 0419 and 0434 tuple-move
precedent). Follows decision 0434.

## Context

`1.0.0-rc.2` is published (`docs/RELEASE-NOTES.md`). Source builds of `main` after the `v1.0.0-rc.2`
tag still report `1.0.0-rc.2`, so a build carrying later fixes is indistinguishable by version
from the published prerelease.

## Decision

1. The `PUB-V0-001` version tuple moves from `1.0.0-rc.2` to `1.0.0-rc.3`: `VERSION`, the
   `cmd/corvint` version constant, the release-artifact manifest's expected version, and the VS
   Code extension's exact executable check with its README, tests and conformance cases.
2. `1.0.0-rc.3` is an unpublished candidate label. No tag, archive, release, readiness record or
   qualification exists or is claimed for it; its tag row stays `NOT_RUN` until a later release
   decision builds and gates a candidate under `docs/RELEASE-RUNBOOK.md`.
3. No signing selection is made for `1.0.0-rc.3`. Decision 0433 covers `1.0.0-rc.2` and `1.0.0`
   only, so the readiness record's `policy/signing` row for rc.3 stays operator-owned
   (`SRR-V1-009`) and `internal/releasecandidate/readiness.go` is unchanged.
4. Historical benchmark and release evidence keeps its original identity (`PUB-V0-001`): sealed
   preregistrations and runs under `benchmarks/`, conformance results, the rc.2 release notes, and
   earlier decisions and build-log entries still name `1.0.0-rc.2`. The README continues to name
   `1.0.0-rc.2` as the published prerelease.

## Rollback

Revert this change. Nothing is tagged or published by it, so no artifact or store depends on the
rc.3 label.
