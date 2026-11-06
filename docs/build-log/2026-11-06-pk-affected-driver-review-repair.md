# pk-affected: independent driver review repairs

Owner intent: the four P1 findings supplied for the repair round on
`claude/pk-affected`, base `9db7a94169821d9b28701eaa88866aee2fec1083`.
Starting head: `f61a633b723762b80308ca06c5c54c73d09583c1`.
These are scoped repairs to AZU-V0-001/003, preserving AFP-V0-036's
containment of ordinary Go importers. No broader ticket closure is claimed.

## Decisions and regression evidence

1. A driver path import represents execution of a package, including its tests.
   Preserve the direct driver callers as traversal roots when a Go package is
   contained for a test-only change, then follow their ordinary caller edges.
   Do not restore the Go package as a root. Sort contained IDs before choosing
   witnesses and copy their chains. `TestDriverTestOnlyPackageChange_AZU_V0_001`
   checks the integration-environment target, a direct job, transitive make/CI
   callers, the actual test-path witness and excluded ordinary/test Go importers.
2. Shell globs cannot be exact path imports. Use the existing computed-invocation
   site fallback, rather than add a glob evaluator or change the path-import wire.
   `TestDriverGoGlobComputed_AZU_V0_001` checks `*`, `?`, bracket and bare patterns,
   both source and test-only changes, stored frontiers and transitive callers.
3. Consume the Go global `-C=DIR` option alongside `-C DIR` before command lookup.
   `TestDriverGoEqualsDirectory_AZU_V0_001` checks make and direct CI invocations,
   transitive callers and exclusion for a change to an unrelated package.
4. Inline non-shell interpreter programs are outside shell analysis. Preserve
   their invocation site as computed. `TestDriverInlineInterpreterComputed_AZU_V0_001`
   checks Node `-e`, Python `-c`, the stored frontier and make/CI caller propagation.

All four regressions fail against the starting source via Go's source overlay
and pass against the repairs. The failing overlay log SHA256 is
`453e04b2f2474db7b85e98155a9b36b30f2786b8250a8f43b902c15fd2c79bde`;
the passing focused log SHA256 is
`e3fb06a2e519bc5e281321ad52b2a21b761e45c838d429f6d5f77f3c4954cf0a`.
Existing assertions and committed goldens are unchanged. Rollback reverts this
repair and restores the four missing-driver risks, without a wire migration.

## Verification and limits

The installed and retained checkout planners both select the affected package
with `corvint affected --base` at the stated base. The installed binary warns
that it is stale; the retained checkout planner reports UNKNOWN, 390 selected
units and 83 unknowns. Neither is represented as a final-source zero-unknown run.
The existing pre-change capture is reused, without recapture.

All Go tests ran through `script/heavy.sh`, one package per command, with
GOTOOLCHAIN=local, private TMPDIR `/home/russell/lanes/pk-affected.tmp` (0700),
the configured shared lock directory and no exported commit identity.
The whole affected package passes before the deterministic-seed correction
(74.712 package seconds), and again on the final code (89.606 package seconds,
99.0 heavy runtime seconds). The latter runs every existing test without a
`-run` filter, including the existing containment and live-budget assertions.
Final whole-package log SHA256:
`cc44cdf80f8647cdd39d227f4dd2b72414d373febbf872ec5fe9c46437f19fa3`.
Windows vet passes on the final code; log SHA256:
`e16d9a682a77da9f776b0eb5cf7b50d0029bddf40c46000bc7d79010e9ee263b`.
The unchanged-content line citation moved by twelve lines and was repinned;
the focused spec-requirements and line-citation checks pass.

Raw plans, logs, overlay inputs and command digests remain outside the repository
under `/home/russell/lanes/pk-affected-evidence/independent-review-repair/`.
The final binding/check result and numbered friction reproductions are in
`/home/russell/lanes/pk-affected.handback.md`. Independent review of these repairs
belongs to the orchestrator. Whole-repository gates, zero-unknown replay,
cross-platform runtime qualification, native Tasks writes, publication and seal
are NOT_RUN by the scoped repair instructions. No equivalent coverage is claimed.
