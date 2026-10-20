# V1-2043 and V1-2047: opt-in receipt producer identity and impact capability listing

## Problem

- GitHub #1300 (V1-2043): `impact` and `test-validity` receipts carry no producer identity,
  and the reporter's `corvint --version` printed build `0`.
- GitHub #1304 (V1-2047): the suffixes path `impact` admits, and its per-call limits, were
  available only as help prose or as an `unsupported-impact-path-suffix` refusal.

## Findings

- Build `0` comes from an unstamped source `go build` (README, `docs/AUTOMATION.md`). Release and
  candidate archives already pass `-X main.build=N` (`conformance/release-artifact-v0/build.go`
  `buildArguments`, `internal/companionrelease`), as `PUB-V0-021` requires. There is no published
  `v1.0.0-rc.3` release; the latest published release is `v1.0.0-rc.2`. No stamping fix was needed.
- Release archives are built with `-trimpath -buildvcs=false`, so they carry no `vcs.revision`.
  Their commit is identified by the release manifest plus the executable digest (`PUB-V0-021`).
- A default-on `producer` member conflicts with three frozen contracts:
  - `CCF-V1-006` byte-pins `impact` stdout against the python oracle (`conformance/cli-parity-v0`).
  - The VS Code envelope rejects unknown top-level keys (`extensions/vscode/src/model.ts` `exactKeys`).
  - `MTV-V0-005` requires the CLI and MCP test-validity outputs to be equal.

## Decisions

- `producer` is opt-in (`--producer`) on both commands. Default bytes are unchanged
  (`PUB-V0-027`, `PUB-V0-030`). A default-on member is left for an owner decision.
- An unknown member is an explicit marker (`unknown`; build `unstamped`), never a guess
  (`PUB-V0-028`, `PUB-V0-029`).
- `impact --capabilities` prints `corvint-impact-capabilities/0`, derived by probing
  `ImpactPathAdmitted` and `ImpactRuleNamed`, with an agreement test (`GPK-V0-084..086`).
- Admission and its refusal are unchanged.
- `analyzerSchemaID` is bumped to `corvint-analyzer/130` because the IDX-SNAP-V0-017 audit digests
  every `internal/contextindex` source. The new listing file changes no extraction or encoding.
- The test-validity change is limited to the JSON envelope in `cmd/corvint`.
  `internal/testvaliditydoc` and `internal/testsource` are untouched; the V1-2041 lane edits them.

## Acceptance

The owner's standing auto-accept of lane requirements (2026-10-11) accepts `PUB-V0-027..030`
and `GPK-V0-084..086`.

## Residuals

- A default-on `producer` member needs a `CCF-V1-006` decision with a divergence-register entry,
  a VS Code envelope update and an `MTV-V0-005` parity note.
- Release archives state commit `unknown`. Stamping the commit (for example with
  `-X main.commit`) would need its own `PUB-V0-021` amendment.
- MCP impact and test-validity results carry no producer member.
