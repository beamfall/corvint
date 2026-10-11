# 2026-10-11 CEM 0.3 runtime gate follows the admitted structural tuples (decision 0522)

Batch branch `claude/batch-2026-10-11-cem1-s123` (PR #808), integrating `origin/main` `645304a2f`
(#806, decision 0522) with the CEM 0.3 portable packet branch `claude/batch-2026-10-11-w10`.

## Finding

Decision 0522 admits `go1.27.2` beside `go1.27.1` as a structural tuple in `interop/cem01-go`,
including the `verify-03` runtime stage. The CEM 0.3 packet was written before that admission:
`CEM-SM-017`, `protocol/cem-0.3/README.md` and `OPERATION.json` required
`unsupported-structural-runtime` on every runtime but `go1.27.1`, and
`TestCEMSM016PortablePacketConformance` expected that refusal for the 44 structural cases. On the
new `go1.27.2` pin those cases accept, so the test failed after the merge (reproduced locally).

## Decision

The 0.3 runtime contract follows the admitted tuple set instead of a single release. `CEM-SM-017`,
the packet README, `OPERATION.json` and the `ALGORITHMS.md` runtime note now name `go1.27.1` and
`go1.27.2` (decision 0522, `CEM-V1-050`). The packet itself, its `qualifiedRuntime` `go1.27.1`,
its pinned results and its manifest digest are unchanged; the conformance test treats an admitted
tuple as qualified and compares the full result apart from `runtime.goVersion`, which CEM-SM-016
already excludes. Every unadmitted runtime still must refuse. Packet regeneration
(`TestCEMSM014PacketReproducible`) stays pinned to `go1.27.1` and is NOT_RUN on `go1.27.2`.
The requirement amendment falls under the owner's standing auto-accept of lane requirements
from 2026-10-11.

## Evidence

On `go1.27.2`: `TestCEMSM016PortablePacketConformance` and `TestCEMSM016PortableConfigCeiling`
pass in `interop/cem01-go`; `TestCEMSM016NativePortable03Vectors` passes in
`internal/cem/workflow`.

Rollback: revert this entry's commit and the test commit `9edb99dc5` together with decision 0522's
tuple admission; the go1.27.1-only gate then applies again.
