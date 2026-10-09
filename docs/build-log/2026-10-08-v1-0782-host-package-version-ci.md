# V1-0782: host package version gate in per-PR CI

## Finding

`script/check-host-package-versions.sh` (AHI-020) ran only inside `make gate`, which CI runs
solely in the nightly/owner-dispatched `release-gates.yml` `full-gate` job. No pull-request job
ran it, so a shipped-content change without a manifest bump merged unnoticed: the Gemini CLI
package (fixed by #590, 0.2.5 -> 0.2.6) and, on origin/main at f33ea8ef, the OpenCode package.

Observed on a clean origin/main f33ea8ef: rc=1, `opencode: integrations/opencode/package.json last
bumped at 1912550e (V1-0773, 0.7.8), older than shipped content changed at 62018f1d`
(`integrations/opencode/src/runtime.js`, the compact-plan `--full` consumer change). Gemini CLI
was already current.

## Change

- OpenCode adapter 0.7.8 -> 0.7.9 in every pinned location: `integrations/opencode/package.json`
  (`version`, `adapterVersion`), `integrations/opencode/src/runtime.js` `ADAPTER_VERSION`, and the
  opencode `adapterVersion` in `integrations/compatibility.json`.
- CI `doc-gates` job now checks out full history (`fetch-depth: 0`) and runs
  `make -s host-package-versions-check host-package-versions-test` first. It is the cheapest
  existing per-PR job; the only other full-history PR job (`docs-plan`) is the trusted planner and
  is left alone.
- AHI-020's trace row names the CI enforcement. The requirement text is unchanged.

## Rollback

Revert the commit; the gate stays available through `make gate`.

## Split (2026-10-09)

At the owner's direction, the `.github/workflows/ci.yml` wiring and the AHI-020 trace sentence that
names it moved out of batch J into a separate pull request. A `.github/` change needs an
admin-posted `ci-control-plane` status, which held the whole batch. Batch J keeps the OpenCode
0.7.9 bump. V1-0782 stays open until the CI pull request merges.
