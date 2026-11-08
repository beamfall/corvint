# Build trace and sibling pin self-review repair

RSP-V0-009 and RSP-V0-049: repair the two supplied P2 findings in the
runner evidence record slice (V1-5103), without extending the runner protocol.

Modeled build attribution now adds BuildLoaded while retaining every other
component, its state and its reason. Modeling does not authorize dropping trace
dependencies. TestRSPV0009Attribution requires exact records for both tests despite
qualified markers, retained reads/listings/lookups/execs after runtime union, and
rejection of an unknown trace state. Its former dropped-read expectation was
incorrect under RSP-V0-009; it is replaced with stronger preservation assertions.

Sibling pin freshness now covers every path-bearing component in the closed
record schema: executed, regions, loaded, reads, listings, lookups, writes and
execs. TestRSPV0049SiblingPinComponents exercises each component alone, an
unrelated changed pin, and a changed sibling pin, including a missing optional
probe with no blob. It also requires unknown states and unknown write kinds to
remain stale through the existing validator. The existing closed allowlists are
present/absent/incomplete states and create/replace/append/overwrite/truncate
write kinds; malformed or unknown forms cannot establish freshness. No new wire
shape, field, error code, stderr text or golden is introduced.

Validation: the whole internal/testevidence package passed through
script/heavy.sh with Go's local toolchain, without exported commit identity
(0.135 seconds). The affected receipt against
8b80e6020d8a78b65ca0b05d8f3d829a1ecb4734 selected that package and retained
80 unknowns plus the installed-binary-stale notice. The host owns independent
review and wider verification; those are not claimed here. Raw affected output
is outside the repository in /home/russell/lanes/rspt10-evidence/.

Rollback is a revert of this repair, which restores the two reported gaps.
No persisted format migration or qualification claim is part of this change.
