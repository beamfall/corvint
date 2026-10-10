# rc.3 version tuple

Human-owned intent: on 2026-10-09 the owner asked to "bump the version label off rc.2".

## Decision

- Decision 0486 moves the `PUB-V0-001` tuple to `1.0.0-rc.3` (VERSION, `cmd/corvint` version
  constant, release-artifact manifest, VS Code exact admission, its README, tests and conformance
  cases) and amends `PUB-V0-001` and the `vscode-extension-v0` admitted-token sentence.
- The README now says source builds of `main` report `1.0.0-rc.3`, an unreleased candidate label;
  `1.0.0-rc.2` remains the published prerelease.

## Limits

- rc.3 is unpublished: no tag, archive, readiness record, signing selection or qualification exists
  for it. Historical rc.2 evidence (benchmarks, conformance results, release notes, earlier
  decisions and build-log entries) is not relabelled.
