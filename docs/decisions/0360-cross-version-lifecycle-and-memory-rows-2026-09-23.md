# Decision 0360 — Cross-version upgrade reference and memory/case-fold hostile rows

Date: 2026-09-23. Status: accepted technical amendment under ticket V1-0017 (release v0-8); the
ticket's intent is unchanged. Amends decision 0341 items 1 and 2 and `SOP-V0-003` / `SOP-V0-009` in
`docs/specs/stable-operations-v0.md`.

## Context

Decision 0341 made packet-byte identity the invariant for every lifecycle step, including the
upgrade into a second store. Run with the real N-1 release, `CORVINT_LIFECYCLE_BINARY` set to 0.6.0
(build 90) and `CORVINT_LIFECYCLE_UPGRADE_BINARY` to 0.7.0, `script/check-install-lifecycle.sh`
fails at `upgrade-b` with "packet bytes changed across the upgrade" on darwin arm64. The diff is two
additive packet fields 0.7.0 introduced (`coverage.governance_refused`, evidence `trust` from
decision 0346). The rule could pass only same-version upgrades, so runbook step 8's cross-version
upgrade could never pass for a release that changes the packet wire.

Decision 0341 also left `memory` and `case-folds-context-index` NOT_COVERED.

## Decision

1. A distinct upgrade (`CORVINT_LIFECYCLE_UPGRADE_BINARY` set) is compared to the packet the upgrade
   binary itself builds from a cold `corvint index` of a clone of the fixture at the same commit.
   This still catches an upgrade that misreads the previous release's retained `.corvint` state. The
   step reports `packet=identical` or `packet=changed` against the first packet so a wire change
   stays visible. A same-bytes upgrade keeps the byte-identity rule. Rollback, which is also the
   downgrade path, still requires the first packet's bytes after the newer binary wrote its snapshot.
2. `memory` is covered by a Go-heap allocation bound on one index build over a tracked source 64
   times larger than `maxSourceBytes`. `case-folds-context-index` is covered by a build over two
   tracked paths differing only by case, which must pin each to its own blob. That row skips (NOT_RUN)
   on a case-sensitive filesystem. Whole-process resident memory, including git children, stays
   NOT_COVERED as `memory-resident`.

## Consequences

The lifecycle check can qualify a real N-1 upgrade. It no longer proves that a newer release's
packet equals the older one's, which was never a product promise. Reverting item 1 restores the
0341 rule and fails every wire-changing release at `upgrade-b`. The two new tests are test-only Go.
Linux amd64 remains NOT_RUN, and no service, daemon or root verb is added.
