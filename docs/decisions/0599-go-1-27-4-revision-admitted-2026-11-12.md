# Decision 0599 — go1.27.4 is admitted as a structural tuple

Date: 2026-11-12. Status: accepted. Authority: repository owner, in chat on 2026-11-11: "go is on
1.27.4. use that going forward". Ticket: V1-5408. This decision amends decisions 0522, 0547 and
0557 without superseding them. go1.27.1, go1.27.2 and go1.27.3 stay admitted, and go1.27.4 joins
them as an equally supported patch revision.

## Context

V1-5408 moves the repository's Go pin from go1.27.3 to go1.27.4. Under go1.27.4 the `CEM-V1-050`
structural runtime gate refused with `unsupported-structural-runtime`, and the tests that need a
structural mark failed:

- `TestCEMV1144StableStructuralMark` (`internal/cem/workflow`).
- `TestCEMV1145StatusCeilings`, `TestCCPV0010StatusAndReportAlwaysEmitCounts`,
  `TestCEMV1636StatusNamesThePolicyVerdict`, `TestCEMV1637ReportAppliesCeilings`,
  `TestCEMV17400AbsoluteAliases` and `TestCEMV17400MapForms` (`internal/cem/cli`).
- `TestCEMV1610FailingBinaryEnvelopeRunIsRefused` (`tools/cem-release-suites`).

## Decision

- `CEM-V1-050`: `go1.27.4` joins `structuralTuples` in both `internal/cem/verify/stable.go` and
  the portable `interop/cem01-go/cem.go`. It is admitted on the sealed packet
  `protocol/cem-1.0/structural-tuples/go1.27.4/`, made of `TUPLE.json` plus `manifest.json`.
- The packet is shaped like the go1.27.3 packet. It binds the frozen S0E packet manifest and its two
  result files by SHA-256, and declares a single substitution of `expectedResult.runtime.goVersion`
  from `go1.27.1` to `go1.27.4`.
- The packet records this qualification evidence, gathered on darwin/arm64 with the official
  module-cache toolchains:
  - The parser closure (`tools/cem-structural-closure`) is `83e19e1d…` with 35 packages and 323
    files. That value is identical for go1.27.1, go1.27.2, go1.27.3 and go1.27.4, so `CEM-V1-370`
    applies.
  - The 70-package closure of the five structural stdlib packages has 2323 files. Compared with
    go1.27.3, 69 packages are byte-identical. Only six `runtime` files differ, and `runtime` is
    outside the parser closure.
  - The go/ast, go/format, go/parser, go/printer, go/scanner, go/token, go/build/constraint and
    cmd/gofmt trees are identical to go1.27.3 and to go1.27.1.
  - The decision 0551 structural corpus covers 4836 repository files. Its portable observation under
    go1.27.4 is byte-identical to the qualified go1.27.1 observation, and so is the case observation.
    Native parity holds: 19190 repository pairs and 85 case pairs give verdicts equal to the
    portable ones.
  - The native S0E public cases (54 of 56) and the linked control pass through the binding. The two
    go1.24.13 cases stay NOT_RUN.
- `protocol/cem-1.0/structural-corpus/QUALIFICATION.json` admits go1.27.4 and records these runs as
  an amendment. Its manifest entry is resealed.
- NOT_RUN:
  - The go1.27.3 packet's ad hoc 4621-file corpus, because its harness is not in the repository.
  - Linux and Windows hosts.
  - CI-hosted runs.

## Consequences

- A go1.27.4 host runs the structural proof.
- A go1.27.5 or later host still refuses. Admitting it needs its own decision and its own sealed
  packet.

## Rollback

Revert this commit. go1.27.4 then refuses the structural proof with
`unsupported-structural-runtime`, and the earlier tuples apply unchanged.
