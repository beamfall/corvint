# V1-4701: preserve linecov windows with inherited PWD

Date: 2026-11-06. Repair base: `8fc0cf37642e7012f4c26671c9ed97a8706d6e09`.
Independent review found that resolving the collection root alone still lets Go
preserve a valid symlinked `PWD`, causing source paths to miss physical overlay
keys. This repairs the implementation of LPCV-V0-7600 and LPCV-V0-954 without
changing accepted intent or any wire contract.

Decision: use an absolute physical collection root and explicitly set the build
subprocess's `PWD` to it. Keep the override local to the build; the collection
environment and each test's working directory retain their existing behavior.
The environment slice is capacity-clipped before the build appends its override.

`TestCollectSymlinkedRootPWD` builds and executes the existing two-test fixture
through a symlinked absolute root with both matching symlinked and stale inherited
`PWD`. It requires all three outcomes, complete records, no divergence and no
window fallback. Before the repair the matching case failed with
`window-order-unavailable`; the stale case passed. After the repair the entire
`internal/linecov` package passed through `script/heavy.sh` with `-count=1
-timeout 30m` (14.086 s), including both regression cases and existing assertions.
No golden or existing assertion changed.

Raw logs and affected-plan receipts stay outside the repository under
`/home/russell/lanes/darwin-shards-evidence/`. Log SHA-256 bindings:

- `pwd-regression-before.log`: `b9b27ef02936a78738ddecc97375c3ad732ec6cf34f238e62ff77fd69c702a53`
- `linecov-whole.log`: `de326b5c753021655e713fb8342cd3850bfca661714c6ce079aab97352d11c37`

This is Linux symlink qualification;
native macOS execution is NOT_RUN on this host. The supplied independent review
is the repair input; a fresh independent review belongs to the orchestrator.
Pre-change capture is reused as instructed; no new capture is claimed.

Rollback: revert this repair's code and regression test together. The original
root-resolution change remains, including its known matching-PWD limitation.
V1-4761 and CAL-V0-026 / ATR-V0-7203 have no additional implementation changes in
this repair round. Task completion writes, publication and sealing are excluded
by the repair instructions.
