# RSP read-context restart and implicit interpreter dependencies

Authority: **orchestrator-made under delegation**, owner 2026-11-02; lane restart
instruction 2026-11-07. The RSP-V0-051 paragraph records the supplied decision,
not a new inferred authority or a delivered-command claim. The status line and
shared traceability rows remain unchanged.

The read command must resolve the current host's planned-run identities without
executing runners, and consume only previously persisted and admitted inventory,
qualification and carry-forward artifacts. Version strings cannot establish
trust; expected executable digests must come from independently resolved bytes.
Missing context, inventory and qualification remain explicit unknown/FULL states.

The current checkout does not provide the required integration inputs.
`internal/linecov.Index` stores RSP evidence but no runner inventory, admitted
qualification or carry-forward artifact. `internal/testrunner.RunnerInventory`
is explicitly an in-process value, not a storage contract.
`cmd/corvint/affected_runner_shadow.go` supplies declaration-only inputs;
`internal/runnerselection` consumes supplied core values and is not an admission
API. `internal/testevidence.EnvironmentDigest` computes a keyed digest but does
not resolve a declared execution configuration or its key. The parallel T6
handback also leaves production driver and admission integration outstanding.
The restart settles the expected-identity policy but does not supply those
missing persisted inputs or a reader API. Adding an invented store or deriving
admission from a record would violate the lane's frozen-contract boundary.
RSP-V0-051 implementation therefore remains blocked on that integration; no
positive command, MCP, flow, golden or no-write evidence is claimed.

The separately requested PT_INTERP repair is implemented. The kernel's implicit
interpreter path now goes through the existing symlink dependency walker before
endpoint resolution. This retains intermediate links and applies the existing
40-hop and path caps. Tests use a synthetic ELF header without running a compiler
or tracee: `TestRSPV0021InterpreterMiddleLinkRetarget` requires both link dependencies
and the changed resolved endpoint after retargeting only the middle link;
`TestRSPV0022InterpreterLinkCaps` requires incomplete evidence for a 41-link chain,
a cycle and an exhausted path cap. Existing assertions and wire formats are
unchanged. Rollback is removal of this added walker call and its new regression
fixtures; prior interpreter and errno repairs remain intact.

Verification follows the build-only charter: package vet type-checks the new
tests; runtime tests and independent review belong to integration and remain
NOT_RUN. Final command results and the exact commit are retained in the external
lane handback. CEM binding and sealing are deferred by the same build-only rule.
