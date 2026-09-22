# Reproducible release and recovery runbook

This runbook uses the existing native archive, companion, candidate and installer boundaries.
It does not authorize a version change, tag, push, signing, upload or publication. See
[Public Release V0](specs/public-release-v0.md), [archive integrity](specs/release-artifact-integrity-v0.md),
the [changelog](RELEASE-NOTES.md) and [security/support policy](SECURITY.md).

1. Select clean immutable Corvint and Corvint Tasks commits with their accepted version policy.
   Record full commits/trees, first-parent Core build number, exact Go 1.27.1 and Git identities.
   Preserve predecessor evidence and explicit NOT_RUN rows. Do not build from a dirty primary.
2. Enroll and finish source verification under [DOGFOOD](DOGFOOD.md). Commit source and required
   evidence before the frozen gate; do not commit during a running gate. Coordinate a single
   `make gate` with the release owner. Its archive gate compares two builds and two assemblies
   per declared target from immutable exports; a pass is specific to that toolchain/profile.
3. Retain the existing seven-file core artifact set in a new private directory outside the
   checkout. For an approved frozen source revision, the existing producer is:

   ```sh
   GOTOOLCHAIN=local go run ./conformance/release-artifact-v0 archive \
     --revision FULL_COMMIT --output /absolute/new-core-output
   ```

   Retain `verification-report.json` and `SHA256SUMS`. The normal `make gate` archive scratch is
   removed on exit, so its temporary path is not a distributable backup. Windows remains outside
   the qualified candidate despite the core producer's Windows cross-build.
4. Use `script/corvint-companion-release-gate` for the separately qualified companion tuple,
   following its declared inputs and pinned Tasks checkout. Retain its three-file archive,
   checksum and smoke set. Run required installed qualification against those exact bytes via
   `script/public-release-check`; a cross-build does not supply a native installed-platform pass.
   Missing tools, authorization or platform evidence remain NOT_RUN/NOT_PRODUCED.
5. Assemble with the existing `corvint-release-candidate` invocation in [INSTALL](INSTALL.md).
   It requires matching Core source/toolchain and source archives across both retained gates.
   Preserve the whole closed candidate and independently obtained digest. Do not edit a receipt
   to claim qualification; any changed evidence requires requalification and a fresh candidate.
6. Exercise the installer in an exclusively owned canonical temporary store on each actual
   supported OS/CPU. Verify the complete candidate, install and run the host binary, install a
   second qualified version beside it, switch explicit paths back to prove rollback, and restore
   a copied candidate into a fresh store. Test corruption and uninstall only in those fixtures.
   Retain exact candidate/archive digests, native OS/CPU, command results and remaining unknowns.
7. Follow the operational backup/recovery and removal procedure in [INSTALL](INSTALL.md). Keep
   retained candidates immutable. A missing or incompatible data backup is a recovery blocker;
   the installer does not repair ticket state, traces or evidence. Human approval of the exact
   reviewable release and support policy is still required for outward publication/promotion.

## Existing security evidence and remaining qualification

| Boundary | Existing regression/evidence | Remaining release obligation |
|---|---|---|
| Candidate integrity and inventory | `internal/releasecandidate` checksum/source/version/inventory tests | Exact future closed artifact verification |
| Upgrade, rollback, reinstall, removal | `TestPUBV0025RecoveryLifecycle`, temporary shell/archive fixtures | Native lifecycle using each released executable and compatible data backup |
| Store symlinks, case aliases and overlap | `TestPUBV0025HostileStore`, no-replace promotion tests | Exclusive store ownership; concurrent malicious renames unqualified |
| Probe output, cancellation and descendants | `TestPUBV0026ProbeFailureCleansInstall`, `TestPUBV0026InterruptedInstallReapsDescendant`, `internal/procgroup` | Escaped descendants remain outside owned-group proof |
| Hostile archives and repository exports | `internal/companionrelease` unsafe path, casefold, source count/byte tests; `go-archive-gate-injection-test` | Capability-specific exact artifact/security gates; injection gate is opt-in |
| Secrets and evidence | `internal/secretscreen`, `internal/trace`, local-completion screening | Review retained release evidence before sharing; fixtures are not a comprehensive audit |

The V1-0017 native planning record, runtime qualification and promotion axes remain distinct.
Neither this runbook nor source fixture tests complete its V1-0008/V1-0015 dependencies or establish
support policy. The ticket's full-gate requirement remains mandatory at completion.
