# Receipt producer identity stays opt-in for 1.0 (V1-2064)

Date: 2026-10-21. Base: `5cf75daf0` (`origin/main`). Requirements: `PUB-V0-027`..`PUB-V0-030`
(`docs/specs/public-release-v0.md`), unchanged.

## Decision

V1-2064 asks whether `impact` and `test-validity` receipts carry the `PUB-V0-027` `producer`
member by default, or keep it behind `--producer`. The lane decided the fork under the delegated
in-task decision authority (owner, 2026-10-05). The owner can reverse it with a later decision.

The decision is to keep `producer` opt-in for 1.0, which is the conservative option. Default-on
would change bytes that three frozen contracts pin:

- the `impact` stdout that `CCF-V1-006` pins in `conformance/cli-parity-v0`;
- the closed top-level keys of the VS Code envelope (`VSC-V0`);
- the CLI/MCP test-validity equality in `MTV-V0-005`.

`PUB-V0-030` already states that default receipts stay byte-identical. It also says default-on
needs its own decision and divergence record. Opt-in loses nothing a caller needs: `--producer`
states the build on request, and MCP `serverInfo` already states the version. So the cost of
default-on is three re-pinned parity contracts, with no consumer that requires it.

Because of this decision, there is no new requirement, no fixture change and no divergence-register entry.

## Consequence for V1-2066

V1-2066 asks for producer members on the MCP impact and test-validity surfaces. Under this
decision, MCP keeps naming the producer through `serverInfo`, as the `PUB-V0` non-goals state.
Adding the member to MCP alone would split CLI/MCP equality under `MTV-V0-005`. The lane therefore
did not build V1-2066, and it recommends the same disposition: decline it for 1.0 unless default-on
is later accepted.

## Revisit when

Revisit this if an external consumer needs producer identity in default output, for example a
CEM producer field that requires it. Making it default-on then means re-pinning `CCF-V1-006`,
`VSC-V0` and `MTV-V0-005` in one change, with a divergence entry.

Rollback: none needed; no behavior changed.
